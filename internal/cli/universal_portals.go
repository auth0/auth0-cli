package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/auth0/go-auth0/management"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
)

// myAccountAPIScopes are the scopes granted to the portal client for the My Account API.
var myAccountAPIScopes = []string{
	"read:me:authentication_methods",
	"delete:me:authentication_methods",
	"update:me:authentication_methods",
	"read:me:factors",
	"create:me:authentication_methods",
}

// myOrgAPIScopes are the scopes granted to the portal client for the My Organization API.
var myOrgAPIScopes = []string{
	"read:my_org:configuration",
	"read:my_org:details",
	"update:my_org:details",
}

// managementAPIScopes are the scopes granted to the portal client for the Management API.
var managementAPIScopes = []string{
	"read:branding",
	"read:organizations_summary",
	"read:organizations",
}

// portalGrantTypes are the grant types enabled on the portal client.
var portalGrantTypes = []string{
	"authorization_code",
	"refresh_token",
	"client_credentials",
	"http://auth0.com/oauth/grant-type/mfa-oob",
	"http://auth0.com/oauth/grant-type/mfa-otp",
	"http://auth0.com/oauth/grant-type/mfa-recovery-code",
}

var upName = Flag{
	Name:     "Name",
	LongForm: "name",
	ShortForm: "n",
	Help:     "Name of the Universal Portals application.",
}

// universalPortalsCmd groups Universal Portals management commands.
func universalPortalsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "universal-portals",
		Aliases: []string{"up"},
		Short:   "Manage Universal Portals resources",
		Long:    "Manage Auth0 Universal Portals resources.",
	}

	cmd.SetUsageTemplate(namespaceUsageTemplate())
	cmd.AddCommand(universalPortalsSetupCmd(cli))

	return cmd
}

// universalPortalsSetupCmd provisions all Auth0 resources required by a Universal Portals application.
func universalPortalsSetupCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Name string
	}

	cmd := &cobra.Command{
		Use:   "setup",
		Args:  cobra.NoArgs,
		Short: "Set up Auth0 resources for a Universal Portals application",
		Long: `Provisions the Auth0 resources required by a Universal Portals application:

  - Auth0 My Account API and My Organization API (resource servers)
  - A Regular Web App client with the required configuration
  - Client grants for My Account API, My Organization API, and the Management API`,
		Example: `  auth0 universal-portals setup
  auth0 universal-portals setup --name "My Portal"
  auth0 up setup -n "My Portal"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUniversalPortalsSetup(cmd, cli, inputs.Name)
		},
	}

	upName.RegisterString(cmd, &inputs.Name, "")

	return cmd
}

// runUniversalPortalsSetup is the orchestration entry point for `universal-portals setup`.
func runUniversalPortalsSetup(cmd *cobra.Command, cli *cli, name string) error {
	if err := cli.setupWithAuthentication(cmd.Context()); err != nil {
		return fmt.Errorf("authentication required: %w", err)
	}

	// Resolve the portal domain: default custom domain, or fall back to the tenant domain.
	domain, isCustom := resolvePortalDomain(cmd, cli)
	if isCustom {
		cli.renderer.Infof("Domain: %s  (custom domain)", ansi.Cyan(domain))
	} else {
		cli.renderer.Infof("Domain: %s", ansi.Cyan(domain))
	}
	cli.renderer.Newline()

	// Collect the application name.
	defaultName := name
	if defaultName == "" {
		defaultName = "Universal Portals"
	}
	if err := upName.Ask(cmd, &name, &defaultName); err != nil {
		return fmt.Errorf("failed to enter application name: %w", err)
	}
	if name == "" {
		return fmt.Errorf("application name cannot be empty")
	}

	tenant := cli.tenant

	// Ensure the resource servers required by Universal Portals exist.
	if err := ensurePortalResourceServer(cmd, cli, "Auth0 My Account API", "https://"+tenant+"/me/"); err != nil {
		return err
	}
	if err := ensurePortalResourceServer(cmd, cli, "Auth0 My Organization API", "https://"+tenant+"/my-org/"); err != nil {
		return err
	}

	// Create the portal application client.
	client, err := createPortalClient(cmd, cli, name, domain, tenant)
	if err != nil {
		return err
	}

	clientURL := portalManageClientURL(cli, client.ClientID)
	maskedSecret := client.ClientSecret[:4] + strings.Repeat("•", len(client.ClientSecret)-4)

	cli.renderer.Successf("Application %q created", name)
	cli.renderer.Detailf("Client ID:     %s", ansi.Hyperlink(clientURL, ansi.Magenta(client.ClientID)))
	cli.renderer.Detailf("Client secret: %s", ansi.Faint(maskedSecret))
	cli.renderer.Newline()

	// Create the three client grants.
	if err := createPortalClientGrants(cmd, cli, client.ClientID, tenant); err != nil {
		return err
	}

	// TODO: Create Form (payload TBD).
	// TODO: Create Portal (payload TBD — needs client.ClientID and form ID).

	portalURL := "https://" + domain + "/portals"

	cli.renderer.Newline()
	cli.renderer.Infof("Portal: %s", ansi.Cyan(portalURL))
	cli.renderer.Newline()

	if cli.noInput {
		return nil
	}

	cli.renderer.Infof("%s to open the portal or %s to quit...", ansi.Green("Press Enter"), ansi.Red("^C"))
	if _, err := fmt.Scanln(); err != nil {
		return nil
	}

	if err := browser.OpenURL(portalURL); err != nil {
		cli.renderer.Warnf("Couldn't open the URL, please do it manually: %s", portalURL)
	}

	return nil
}

// resolvePortalDomain returns the domain to use for portal URLs.
// Uses the default custom domain if one is active; falls back to the tenant domain.
func resolvePortalDomain(cmd *cobra.Command, cli *cli) (domain string, isCustom bool) {
	cd, err := cli.api.CustomDomain.ReadDefault(cmd.Context())
	if err == nil && cd.GetStatus() == "ready" {
		return cd.GetDomain(), true
	}
	return cli.tenant, false
}

// ensurePortalResourceServer creates a resource server idempotently.
// A 409 Conflict (already exists) is treated as success.
func ensurePortalResourceServer(cmd *cobra.Command, cli *cli, name, identifier string) error {
	skipConsent := true
	tokenDialect := "rfc9068_profile"

	var alreadyExists bool

	if err := ansi.Waiting(func() error {
		err := cli.api.ResourceServer.Create(cmd.Context(), &management.ResourceServer{
			Name:                                      &name,
			Identifier:                                &identifier,
			SkipConsentForVerifiableFirstPartyClients: &skipConsent,
			TokenDialect:                              &tokenDialect,
		})
		if err == nil {
			return nil
		}
		if mErr, ok := err.(management.Error); ok && mErr.Status() == http.StatusConflict {
			alreadyExists = true
			return nil
		}
		return err
	}); err != nil {
		return fmt.Errorf("failed to ensure resource server %q: %w", name, err)
	}

	if alreadyExists {
		cli.renderer.Infof("%s %s", name, ansi.Faint("(already exists)"))
	} else {
		cli.renderer.Successf(name)
	}

	return nil
}

// portalClientPayload is the full request body for creating the portal application.
// Uses a local struct because several fields (session_transfer, refresh_token.policies)
// are not present in the vendored go-auth0 management.Client.
type portalClientPayload struct {
	Name                        string                  `json:"name"`
	IsFirstParty                bool                    `json:"is_first_party"`
	AppType                     string                  `json:"app_type"`
	TokenEndpointAuthMethod     string                  `json:"token_endpoint_auth_method"`
	Callbacks                   []string                `json:"callbacks"`
	AllowedLogoutURLs           []string                `json:"allowed_logout_urls"`
	OrganizationRequireBehavior string                  `json:"organization_require_behavior"`
	OrganizationUsage           string                  `json:"organization_usage"`
	GrantTypes                  []string                `json:"grant_types"`
	RefreshToken                portalRefreshToken      `json:"refresh_token"`
	SessionTransfer             portalSessionTransfer   `json:"session_transfer"`
	OIDCBackchannelLogout       portalBackchannelLogout `json:"oidc_backchannel_logout"`
}

type portalRefreshToken struct {
	ExpirationType            string                     `json:"expiration_type"`
	Leeway                    int                        `json:"leeway"`
	InfiniteTokenLifetime     bool                       `json:"infinite_token_lifetime"`
	InfiniteIdleTokenLifetime bool                       `json:"infinite_idle_token_lifetime"`
	TokenLifetime             int                        `json:"token_lifetime"`
	IdleTokenLifetime         int                        `json:"idle_token_lifetime"`
	RotationType              string                     `json:"rotation_type"`
	Policies                  []portalRefreshTokenPolicy `json:"policies"`
}

type portalRefreshTokenPolicy struct {
	Audience string   `json:"audience"`
	Scope    []string `json:"scope"`
}

type portalSessionTransfer struct {
	AllowRefreshToken             bool     `json:"allow_refresh_token"`
	AllowedAuthenticationMethods  []string `json:"allowed_authentication_methods"`
	CanCreateSessionTransferToken bool     `json:"can_create_session_transfer_token"`
	EnforceCascadeRevocation      bool     `json:"enforce_cascade_revocation"`
	EnforceDeviceBinding          string   `json:"enforce_device_binding"`
	EnforceOnlineRefreshTokens    bool     `json:"enforce_online_refresh_tokens"`
}

type portalBackchannelLogout struct {
	BackchannelLogoutInitiators portalBackchannelInitiators `json:"backchannel_logout_initiators"`
	BackchannelLogoutURLs       []string                    `json:"backchannel_logout_urls"`
}

type portalBackchannelInitiators struct {
	Mode               string   `json:"mode"`
	SelectedInitiators []string `json:"selected_initiators"`
}

// portalClientResult holds the fields read back from the create-client response.
type portalClientResult struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// createPortalClient creates the Universal Portals application client.
// Uses the raw HTTP client because session_transfer and refresh_token.policies
// are not present in the vendored management.Client struct.
func createPortalClient(cmd *cobra.Command, cli *cli, name, domain, tenant string) (*portalClientResult, error) {
	payload := portalClientPayload{
		Name:                        name,
		IsFirstParty:                true,
		AppType:                     "regular_web",
		TokenEndpointAuthMethod:     "client_secret_post",
		Callbacks:                   []string{"https://" + domain + "/portals/auth/callback"},
		AllowedLogoutURLs:           []string{"https://" + domain},
		OrganizationRequireBehavior: "no_prompt",
		OrganizationUsage:           "allow",
		GrantTypes:                  portalGrantTypes,
		RefreshToken: portalRefreshToken{
			ExpirationType:            "expiring",
			Leeway:                    0,
			InfiniteTokenLifetime:     false,
			InfiniteIdleTokenLifetime: true,
			TokenLifetime:             86400,
			IdleTokenLifetime:         86399,
			RotationType:              "non-rotating",
			Policies: []portalRefreshTokenPolicy{
				{
					Audience: "https://" + tenant + "/me/",
					Scope:    myAccountAPIScopes,
				},
				{
					Audience: "https://" + tenant + "/my-org/",
					Scope:    myOrgAPIScopes,
				},
			},
		},
		SessionTransfer: portalSessionTransfer{
			AllowRefreshToken:             true,
			AllowedAuthenticationMethods:  []string{"query"},
			CanCreateSessionTransferToken: false,
			EnforceCascadeRevocation:      true,
			EnforceDeviceBinding:          "ip",
			EnforceOnlineRefreshTokens:    true,
		},
		OIDCBackchannelLogout: portalBackchannelLogout{
			BackchannelLogoutInitiators: portalBackchannelInitiators{
				Mode: "custom",
				SelectedInitiators: []string{
					"idp-logout",
					"rp-logout",
					"session-expired",
					"session-revoked",
					"account-deleted",
					"account-deactivated",
				},
			},
			BackchannelLogoutURLs: []string{"https://" + domain + "/portals/auth/backchannel-logout"},
		},
	}

	var result portalClientResult

	if err := ansi.Waiting(func() error {
		req, err := cli.api.HTTPClient.NewRequest(
			cmd.Context(),
			http.MethodPost,
			cli.api.HTTPClient.URI("clients"),
			payload,
		)
		if err != nil {
			return err
		}

		resp, err := cli.api.HTTPClient.Do(req)
		if err != nil {
			return err
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode >= http.StatusBadRequest {
			var apiErr struct {
				StatusCode int    `json:"statusCode"`
				Message    string `json:"message"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&apiErr)
			return fmt.Errorf("API error %d: %s", resp.StatusCode, apiErr.Message)
		}

		return json.NewDecoder(resp.Body).Decode(&result)
	}); err != nil {
		return nil, fmt.Errorf("failed to create application: %w", err)
	}

	return &result, nil
}

// portalGrantPayload is the request body for creating a client grant with subject_type,
// which is absent from the vendored management.ClientGrant struct.
type portalGrantPayload struct {
	ClientID    string   `json:"client_id"`
	Audience    string   `json:"audience"`
	SubjectType string   `json:"subject_type"`
	Scope       []string `json:"scope"`
}

// createPortalClientGrants creates the three client grants required by the portal.
// Uses the raw HTTP client because subject_type is not in the vendored ClientGrant struct.
func createPortalClientGrants(cmd *cobra.Command, cli *cli, clientID, tenant string) error {
	grants := []portalGrantPayload{
		{
			ClientID:    clientID,
			Audience:    "https://" + tenant + "/me/",
			SubjectType: "user",
			Scope:       myAccountAPIScopes,
		},
		{
			ClientID:    clientID,
			Audience:    "https://" + tenant + "/my-org/",
			SubjectType: "user",
			Scope:       myOrgAPIScopes,
		},
		{
			ClientID:    clientID,
			Audience:    "https://" + tenant + "/api/v2/",
			SubjectType: "client",
			Scope:       managementAPIScopes,
		},
	}

	for _, grant := range grants {
		g := grant
		if err := ansi.Waiting(func() error {
			req, err := cli.api.HTTPClient.NewRequest(
				cmd.Context(),
				http.MethodPost,
				cli.api.HTTPClient.URI("client-grants"),
				g,
			)
			if err != nil {
				return err
			}

			resp, err := cli.api.HTTPClient.Do(req)
			if err != nil {
				return err
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode >= http.StatusBadRequest {
				var apiErr struct {
					StatusCode int    `json:"statusCode"`
					Message    string `json:"message"`
				}
				_ = json.NewDecoder(resp.Body).Decode(&apiErr)
				return fmt.Errorf("API error %d: %s", resp.StatusCode, apiErr.Message)
			}

			return nil
		}); err != nil {
			return fmt.Errorf("failed to create client grant for %q: %w", g.Audience, err)
		}

		cli.renderer.Successf("Client grant  %s", g.Audience)
	}

	return nil
}

// portalManageClientURL returns the Management Dashboard URL for the given client.
func portalManageClientURL(cli *cli, clientID string) string {
	parts := strings.Split(cli.tenant, ".")

	var region string
	if len(parts) == 3 {
		region = "us"
	} else {
		region = parts[1]
	}

	tenantName := cli.Config.Tenants[cli.tenant].Name
	base := deriveServiceURL("manage", cli.tenant)

	return fmt.Sprintf("%s/dashboard/%s/%s/applications/%s/settings",
		base, region, tenantName, clientID)
}
