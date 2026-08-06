package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/auth0/go-auth0/management"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
)

// Scopes granted to the portal client per audience.
var (
	myAccountAPIScopes = []string{
		"read:me:authentication_methods",
		"delete:me:authentication_methods",
		"update:me:authentication_methods",
		"read:me:factors",
		"create:me:authentication_methods",
	}

	myOrgAPIScopes = []string{
		"read:my_org:configuration",
		"read:my_org:details",
		"update:my_org:details",
	}

	managementAPIScopes = []string{
		"read:branding",
		"read:organizations_summary",
		"read:organizations",
		"update:users",
		"update:users_app_metadata",
	}

	portalGrantTypes = []string{
		"authorization_code",
		"refresh_token",
		"client_credentials",
		"http://auth0.com/oauth/grant-type/mfa-oob",
		"http://auth0.com/oauth/grant-type/mfa-otp",
		"http://auth0.com/oauth/grant-type/mfa-recovery-code",
	}
)

var (
	upPortalName = Flag{
		Name:      "Name",
		LongForm:  "name",
		ShortForm: "n",
		Help:      "Display name of the portal.",
	}
	upPortalSlug = Flag{
		Name:      "Slug",
		LongForm:  "slug",
		ShortForm: "s",
		Help:      "URL-friendly identifier for the portal (e.g. my-portal).",
	}
)

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
		PortalName string
		Slug       string
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
  auth0 universal-portals setup --name "Acme" --slug "acme"
  auth0 up setup -n "Acme" -s "acme"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUniversalPortalsSetup(cmd, cli, inputs.PortalName, inputs.Slug)
		},
	}

	upPortalName.RegisterString(cmd, &inputs.PortalName, "")
	upPortalSlug.RegisterString(cmd, &inputs.Slug, "")

	return cmd
}

// runUniversalPortalsSetup orchestrates the provisioning flow.
// It owns all display logic; business functions only return (value, error).
func runUniversalPortalsSetup(cmd *cobra.Command, cli *cli, portalName, slug string) error {
	if err := cli.setupWithAuthentication(cmd.Context()); err != nil {
		return fmt.Errorf("authentication required: %w", err)
	}

	// Verify the token carries the scopes required by the EA setup endpoints.
	// These are intentionally not in RequiredScopes to avoid breaking login for
	// tenants without the feature flag.
	//   create:flows_vault_connections — POST /api/v2/flows/vault/connections
	//   create:forms, create:flows     — POST /api/v2/forms/import
	//   create:portals                 — POST /api/v2/portals
	//
	// Scopes are read from the JWT directly so this works regardless of auth
	// method (device code, client secret, etc.).
	upRequiredScopes := []string{
		"create:flows_vault_connections",
		"create:forms",
		"create:flows",
		"create:portals",
	}
	if tenant, err := cli.Config.GetTenant(cli.tenant); err == nil {
		if granted := scopesFromToken(tenant.GetAccessToken()); granted != nil {
			if missing := missingScopes(granted, upRequiredScopes); len(missing) > 0 {
				return fmt.Errorf(
					"insufficient scopes to provision Universal Portals\nMissing: %s\nRe-authenticate to continue: auth0 login",
					strings.Join(missing, ", "),
				)
			}
		}
	}

	ctx := cmd.Context()

	// Resolve the portal domain: default custom domain, or fall back to the tenant domain.
	domain, isCustom := resolvePortalDomain(ctx, cli.api.CustomDomain, cli.tenant)
	if isCustom {
		cli.renderer.Infof("Domain: %s  (custom domain)", ansi.Cyan(domain))
	} else {
		cli.renderer.Infof("Domain: %s", ansi.Cyan(domain))
	}
	cli.renderer.Newline()

	// Collect portal name.
	defaultPortalName := portalName
	if defaultPortalName == "" {
		defaultPortalName = "My Portal"
	}
	if err := upPortalName.Ask(cmd, &portalName, &defaultPortalName); err != nil {
		return fmt.Errorf("failed to enter portal name: %w", err)
	}
	if portalName == "" {
		return fmt.Errorf("portal name cannot be empty")
	}

	// Collect portal slug, defaulting to a slugified version of the portal name.
	if slug == "" {
		slug = toPortalSlug(portalName)
	}
	if err := upPortalSlug.Ask(cmd, &slug, &slug); err != nil {
		return fmt.Errorf("failed to enter portal slug: %w", err)
	}
	if slug == "" {
		return fmt.Errorf("portal slug cannot be empty")
	}

	// The application name is derived automatically from the portal name.
	appName := portalName + " (Universal Portals)"

	tenant := cli.tenant

	// Ensure the resource servers required by Universal Portals exist.
	type rsResult struct {
		name    string
		existed bool
	}
	var rsResults []rsResult
	for _, rs := range portalResourceServers(tenant) {
		existed, err := ensurePortalResourceServer(ctx, cli.api.ResourceServer, rs.name, rs.identifier)
		if err != nil {
			return err
		}
		rsResults = append(rsResults, rsResult{name: rs.name, existed: existed})
	}
	cli.renderer.Successf("Resource servers ready")
	for _, rs := range rsResults {
		if rs.existed {
			cli.renderer.Detailf("%s", ansi.Faint(rs.name+" (already exists)"))
		} else {
			cli.renderer.Detailf("%s", ansi.Faint(rs.name))
		}
	}

	// Create the portal application client.
	var client portalClientResult
	if err := ansi.Waiting(func() error {
		var err error
		client, err = createPortalClient(ctx, cli.api.HTTPClient, appName, domain, tenant, isCustom)
		return err
	}); err != nil {
		return fmt.Errorf("failed to create application: %w", err)
	}

	clientURL := portalManageClientURL(cli.tenant, cli.Config.Tenants[cli.tenant].Name, client.ClientID)
	maskedSecret := client.ClientSecret[:4] + strings.Repeat("•", len(client.ClientSecret)-4)

	cli.renderer.Successf("Application %q created", appName)
	cli.renderer.Detailf("Client ID:     %s", ansi.Hyperlink(clientURL, ansi.Magenta(client.ClientID)))
	cli.renderer.Detailf("Client secret: %s", ansi.Faint(maskedSecret))
	cli.renderer.Newline()

	// Create the three client grants.
	grants := buildPortalGrants(client.ClientID, tenant)
	for _, g := range grants {
		g := g
		if err := ansi.Waiting(func() error {
			return createPortalGrant(ctx, cli.api.HTTPClient, g)
		}); err != nil {
			return fmt.Errorf("failed to create client grant for %q: %w", g.Audience, err)
		}
	}
	cli.renderer.Successf("Client grants created")
	for _, g := range grants {
		cli.renderer.Detailf("%s", ansi.Faint(g.Audience))
	}
	cli.renderer.Newline()

	// Create the vault connection used by the portal forms to call the Management API.
	var vaultConnID string
	if err := ansi.Waiting(func() error {
		setup := map[string]interface{}{
			"domain":        tenant,
			"client_id":     client.ClientID,
			"client_secret": client.ClientSecret,
			"type":          "OAUTH_APP",
		}
		appID := "AUTH0"
		connName := appName
		conn := &management.FlowVaultConnection{
			AppID: &appID,
			Name:  &connName,
			Setup: &setup,
		}
		if err := cli.api.FlowVaultConnection.CreateConnection(ctx, conn); err != nil {
			return err
		}
		if conn.ID != nil {
			vaultConnID = *conn.ID
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to create vault connection: %w", err)
	}
	cli.renderer.Successf("Vault connection created")
	cli.renderer.Detailf("%s", ansi.Faint(appName))
	cli.renderer.Newline()

	// Import the portal forms.
	var forms portalFormIDs
	if err := ansi.Waiting(func() error {
		var err error
		forms, err = createPortalForms(ctx, cli.api.HTTPClient, vaultConnID)
		return err
	}); err != nil {
		return fmt.Errorf("failed to create forms: %w", err)
	}
	cli.renderer.Successf("Forms created")
	cli.renderer.Detailf("%s", ansi.Faint("Profile update (Universal Portals)"))
	cli.renderer.Detailf("%s", ansi.Faint("Marketing communication preferences (Universal Portals)"))
	cli.renderer.Detailf("%s", ansi.Faint("Privacy settings (Universal Portals)"))
	cli.renderer.Newline()

	// Create the portal. On slug conflict, re-prompt and retry this step only.
	var portal portalResult
	for {
		err := ansi.Waiting(func() error {
			var err error
			portal, err = createPortal(ctx, cli.api.HTTPClient, slug, portalName, client.ClientID, client.ClientSecret, forms)
			return err
		})
		if err == nil {
			break
		}
		var conflict *errAPIConflict
		if !errors.As(err, &conflict) {
			return fmt.Errorf("failed to create portal: %w", err)
		}
		cli.renderer.Warnf("Slug %q is already taken.", slug)
		prevSlug := slug
		slug = ""
		if err := upPortalSlug.Ask(cmd, &slug, &prevSlug); err != nil {
			return fmt.Errorf("failed to enter portal slug: %w", err)
		}
	}
	cli.renderer.Successf("Portal %q created", portal.Name)

	portalURL := "https://" + domain + "/portals/" + portal.Slug

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

// portalResourceServer describes a resource server that Universal Portals requires.
type portalResourceServer struct {
	name       string
	identifier string
}

// portalResourceServers returns the two resource servers required by Universal Portals.
func portalResourceServers(tenant string) []portalResourceServer {
	return []portalResourceServer{
		{name: "Auth0 My Account API", identifier: "https://" + tenant + "/me/"},
		{name: "Auth0 My Organization API", identifier: "https://" + tenant + "/my-org/"},
	}
}

// resolvePortalDomain returns the domain for portal URLs.
// Uses the default custom domain if active; falls back to tenantDomain.
func resolvePortalDomain(ctx context.Context, api auth0.CustomDomainAPI, tenantDomain string) (domain string, isCustom bool) {
	cd, err := api.ReadDefault(ctx)
	if err == nil && cd.GetStatus() == "ready" {
		return cd.GetDomain(), true
	}
	return tenantDomain, false
}

// ensurePortalResourceServer creates a resource server idempotently.
// Returns (true, nil) when the server already existed, (false, nil) when created.
func ensurePortalResourceServer(ctx context.Context, api auth0.ResourceServerAPI, name, identifier string) (alreadyExisted bool, err error) {
	skipConsent := true
	tokenDialect := "rfc9068_profile"

	err = api.Create(ctx, &management.ResourceServer{
		Name:                                      &name,
		Identifier:                                &identifier,
		SkipConsentForVerifiableFirstPartyClients: &skipConsent,
		TokenDialect:                              &tokenDialect,
	})
	if err == nil {
		return false, nil
	}
	if mErr, ok := err.(management.Error); ok && mErr.Status() == http.StatusConflict {
		return true, nil
	}
	return false, fmt.Errorf("failed to ensure resource server %q: %w", name, err)
}

// ---- Client payload types ----
// Local structs are used because session_transfer and refresh_token.policies
// are absent from the vendored go-auth0 management.Client.

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
	OIDCBackchannelLogout       *portalBackchannelLogout `json:"oidc_backchannel_logout,omitempty"`
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

type portalBackchannelLogout struct {
	BackchannelLogoutInitiators portalBackchannelInitiators `json:"backchannel_logout_initiators"`
	BackchannelLogoutURLs       []string                    `json:"backchannel_logout_urls"`
}

type portalBackchannelInitiators struct {
	Mode               string   `json:"mode"`
	SelectedInitiators []string `json:"selected_initiators"`
}

type portalClientResult struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// createPortalClient builds and POSTs the portal application client.
// Takes only what it needs: an HTTP client, not the full *cli.
// Backchannel logout is omitted when isCustomDomain is false because
// Auth0 domains are rejected by the payload validation.
func createPortalClient(ctx context.Context, h auth0.HTTPClientAPI, name, domain, tenant string, isCustomDomain bool) (portalClientResult, error) {
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
				{Audience: "https://" + tenant + "/me/", Scope: myAccountAPIScopes},
				{Audience: "https://" + tenant + "/my-org/", Scope: myOrgAPIScopes},
			},
		},
	}

	if isCustomDomain {
		payload.OIDCBackchannelLogout = &portalBackchannelLogout{
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
		}
	}

	var result portalClientResult
	if err := rawAPIPost(ctx, h, payload, &result, "clients"); err != nil {
		return portalClientResult{}, err
	}
	return result, nil
}

// ---- Grant payload types ----
// subject_type is absent from the vendored management.ClientGrant struct.

type portalGrantPayload struct {
	ClientID    string   `json:"client_id"`
	Audience    string   `json:"audience"`
	SubjectType string   `json:"subject_type"`
	Scope       []string `json:"scope"`
}

// buildPortalGrants returns the three grants required by Universal Portals.
// Pure function: no I/O, fully testable.
func buildPortalGrants(clientID, tenant string) []portalGrantPayload {
	return []portalGrantPayload{
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
}

// createPortalGrant POSTs a single client grant.
func createPortalGrant(ctx context.Context, h auth0.HTTPClientAPI, grant portalGrantPayload) error {
	return rawAPIPost(ctx, h, grant, nil, "client-grants")
}

// errAPIConflict is returned by rawAPIPost when the server responds 409.
// Callers that want to handle conflicts (e.g. slug already taken) can use
// errors.As to detect and recover from this case.
type errAPIConflict struct{ message string }

func (e *errAPIConflict) Error() string { return e.message }

// rawAPIPost is a deep helper that hides the New/Do/decode HTTP pattern.
// Pass path as one or more segments — they are joined by the URI builder so
// slashes are not URL-encoded. Pass a non-nil result to decode the response.
func rawAPIPost(ctx context.Context, h auth0.HTTPClientAPI, payload, result any, path ...string) error {
	req, err := h.NewRequest(ctx, http.MethodPost, h.URI(path...), payload)
	if err != nil {
		return err
	}

	resp, err := h.Do(req)
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
		if resp.StatusCode == http.StatusConflict {
			return &errAPIConflict{message: apiErr.Message}
		}
		return fmt.Errorf("API error %d: %s", resp.StatusCode, apiErr.Message)
	}

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}

// ---- Form provisioning ----

type portalFormResult struct {
	Form struct {
		ID string `json:"id"`
	} `json:"form"`
}

// buildCommunicationPreferencesFormPayload returns the form document for the
// "Marketing communication preferences" form with connID injected.
func buildCommunicationPreferencesFormPayload(connID string) map[string]any {
	return map[string]any{
		"version": "4.0.0",
		"form": map[string]any{
			"name":      "Marketing communication preferences (Universal Portals)",
			"languages": map[string]any{"primary": "en"},
			"nodes": []any{
				map[string]any{
					"id":   "step_4Td2",
					"type": "STEP",
					"coordinates": map[string]any{"x": 217, "y": -218},
					"config": map[string]any{
						"components": []any{
							map[string]any{
								"id":        "communication_preferences",
								"category":  "FIELD",
								"type":      "CHOICE",
								"required":  false,
								"sensitive": false,
								"config": map[string]any{
									"multiple": true,
									"options": []any{
										map[string]any{"label": "Product Announcements", "value": "Product Announcements"},
										map[string]any{"label": "Featured Content", "value": "Featured Content"},
										map[string]any{"label": "Digest News", "value": "Digest News"},
										map[string]any{"label": "Events", "value": "Events"},
									},
								},
							},
							map[string]any{
								"id":       "next_button_US5N",
								"category": "BLOCK",
								"type":     "NEXT_BUTTON",
								"config":   map[string]any{"text": "Update"},
							},
						},
						"next_node": "flow_etvH",
					},
				},
				map[string]any{
					"id":          "flow_etvH",
					"type":        "FLOW",
					"coordinates": map[string]any{"x": 791, "y": -81},
					"config":      map[string]any{"flow_id": "#FLOW-1#", "next_node": "$ending"},
				},
			},
			"start":  map[string]any{"next_node": "step_4Td2", "coordinates": map[string]any{"x": -25, "y": -98}},
			"ending": map[string]any{"resume_flow": true, "coordinates": map[string]any{"x": 1194, "y": -59}},
		},
		"flows": map[string]any{
			"#FLOW-1#": map[string]any{
				"name": "Update preferences",
				"actions": []any{
					map[string]any{
						"id":            "update_user_HitP",
						"type":          "AUTH0",
						"action":        "UPDATE_USER",
						"allow_failure": false,
						"mask_output":   false,
						"params": map[string]any{
							"connection_id": "#CONN-1#",
							"user_id":       "{{context.user.user_id}}",
							"changes": map[string]any{
								"app_metadata": map[string]any{
									"communication_preferences": "{{fields.communication_preferences}}",
								},
							},
						},
					},
				},
			},
		},
		"connections": map[string]any{
			"#CONN-1#": map[string]any{"id": connID},
		},
	}
}

// buildPersonalInfoFormPayload returns the form document for the
// "Profile update" form with connID injected.
func buildPersonalInfoFormPayload(connID string) map[string]any {
	textField := func(id, label string) map[string]any {
		return map[string]any{
			"id":        id,
			"category":  "FIELD",
			"type":      "TEXT",
			"label":     label,
			"required":  false,
			"sensitive": false,
			"config":    map[string]any{"multiline": false},
		}
	}
	return map[string]any{
		"version": "4.0.0",
		"form": map[string]any{
			"name":      "Profile update (Universal Portals)",
			"languages": map[string]any{"primary": "en"},
			"nodes": []any{
				map[string]any{
					"id":          "step_BL3V",
					"type":        "STEP",
					"coordinates": map[string]any{"x": 346, "y": -221},
					"config": map[string]any{
						"components": []any{
							textField("full_name", "Full name"),
							textField("job_title", "Job title"),
							map[string]any{
								"id":        "mobile_number",
								"category":  "FIELD",
								"type":      "TEL",
								"label":     "Mobile number",
								"required":  false,
								"sensitive": false,
								"config":    map[string]any{"country_picker": true},
							},
							map[string]any{
								"id":        "date_of_birth",
								"category":  "FIELD",
								"type":      "DATE",
								"label":     "Date of birth",
								"required":  false,
								"sensitive": false,
								"config":    map[string]any{"format": "DATE"},
							},
							map[string]any{
								"id":        "linkedin",
								"category":  "FIELD",
								"type":      "URL",
								"label":     "LinkedIn",
								"required":  false,
								"sensitive": false,
							},
							map[string]any{
								"id":       "next_button_UmEY",
								"category": "BLOCK",
								"type":     "NEXT_BUTTON",
								"config":   map[string]any{"text": "Update"},
							},
						},
						"next_node": "flow_DcHK",
					},
				},
				map[string]any{
					"id":          "flow_DcHK",
					"type":        "FLOW",
					"coordinates": map[string]any{"x": 988, "y": 9},
					"config":      map[string]any{"flow_id": "#FLOW-1#", "next_node": "$ending"},
				},
			},
			"start":  map[string]any{"next_node": "step_BL3V", "coordinates": map[string]any{"x": 0, "y": 0}},
			"ending": map[string]any{"resume_flow": true, "coordinates": map[string]any{"x": 1367, "y": -7}},
		},
		"flows": map[string]any{
			"#FLOW-1#": map[string]any{
				"name": "Update profile",
				"actions": []any{
					map[string]any{
						"id":            "update_user_v4C0",
						"type":          "AUTH0",
						"action":        "UPDATE_USER",
						"allow_failure": false,
						"mask_output":   false,
						"params": map[string]any{
							"connection_id": "#CONN-1#",
							"user_id":       "{{context.user.user_id}}",
							"changes": map[string]any{
								"user_metadata": map[string]any{
									"linkedin":       "{{fields.linkedin}}",
									"full_name":      "{{fields.full_name}}",
									"job_title":      "{{fields.job_title}}",
									"date_of_birth":  "{{fields.date_of_birth}}",
									"mobile_number":  "{{fields.mobile_number.international_number}}",
								},
							},
						},
					},
				},
			},
		},
		"connections": map[string]any{
			"#CONN-1#": map[string]any{"id": connID},
		},
	}
}

// buildPrivacySettingsFormPayload returns the form document for the
// "Privacy settings" form with connID injected.
func buildPrivacySettingsFormPayload(connID string) map[string]any {
	boolField := func(id, label, hint string) map[string]any {
		return map[string]any{
			"id":        id,
			"category":  "FIELD",
			"type":      "BOOLEAN",
			"label":     label,
			"hint":      hint,
			"required":  true,
			"sensitive": false,
			"config":    map[string]any{"default_value": false},
		}
	}
	return map[string]any{
		"version": "4.0.0",
		"form": map[string]any{
			"name":      "Privacy settings (Universal Portals)",
			"languages": map[string]any{"primary": "en"},
			"nodes": []any{
				map[string]any{
					"id":          "step_y0Ri",
					"type":        "STEP",
					"coordinates": map[string]any{"x": 500, "y": 0},
					"config": map[string]any{
						"components": []any{
							boolField("data_sharing", "Data sharing", `<p>Allow app usage data to improve features. <a href="https://auth0.com" target="_blank">Learn more.</a></p>`),
							boolField("profile_visibility", "Profile visibility", `<p>Show my profile in search results. <a href="https://auth0.com" target="_blank">Learn more.</a></p>`),
							boolField("location_tracking", "Location tracking", `<p>Allow location tracking for personalized recommendations. <a href="https://auth0.com" target="_blank">Learn more.</a></p>`),
							boolField("ad_personalization", "Ad personalization", `<p>Use my data to personalize ads. <a href="https://auth0.com" target="_blank">Learn more.</a></p>`),
							map[string]any{
								"id":       "next_button_aB8e",
								"category": "BLOCK",
								"type":     "NEXT_BUTTON",
								"config":   map[string]any{"text": "Update"},
							},
						},
						"next_node": "flow_AB1r",
					},
				},
				map[string]any{
					"id":          "flow_AB1r",
					"type":        "FLOW",
					"coordinates": map[string]any{"x": 1043, "y": 261},
					"config":      map[string]any{"flow_id": "#FLOW-1#", "next_node": "$ending"},
				},
			},
			"start":  map[string]any{"next_node": "step_y0Ri", "coordinates": map[string]any{"x": 202, "y": 247}},
			"ending": map[string]any{"resume_flow": true, "coordinates": map[string]any{"x": 1445, "y": 247}},
		},
		"flows": map[string]any{
			"#FLOW-1#": map[string]any{
				"name": "Update privacy settings",
				"actions": []any{
					map[string]any{
						"id":            "update_user_uOZW",
						"type":          "AUTH0",
						"action":        "UPDATE_USER",
						"allow_failure": false,
						"mask_output":   false,
						"params": map[string]any{
							"connection_id": "#CONN-1#",
							"user_id":       "{{context.user.user_id}}",
							"changes": map[string]any{
								"app_metadata": map[string]any{
									"data_sharing":       "{{fields.data_sharing}}",
									"location_tracking":  "{{fields.location_tracking}}",
									"ad_personalization": "{{fields.ad_personalization}}",
									"profile_visibility": "{{fields.profile_visibility}}",
								},
							},
						},
					},
				},
			},
		},
		"connections": map[string]any{
			"#CONN-1#": map[string]any{"id": connID},
		},
	}
}

// createPortalForms imports the portal forms and returns their IDs.
// Uses POST /api/v2/forms/import which accepts the full form document
// (version, form, flows, connections). The URI is built with two segments
// to avoid h.URI encoding the slash as %2F.
func createPortalForms(ctx context.Context, h auth0.HTTPClientAPI, vaultConnID string) (portalFormIDs, error) {
	importForm := func(payload map[string]any) (string, error) {
		var result portalFormResult
		if err := rawAPIPost(ctx, h, payload, &result, "forms", "import"); err != nil {
			return "", err
		}
		return result.Form.ID, nil
	}

	personalInfoID, err := importForm(buildPersonalInfoFormPayload(vaultConnID))
	if err != nil {
		return portalFormIDs{}, fmt.Errorf("failed to create personal info form: %w", err)
	}

	commPrefID, err := importForm(buildCommunicationPreferencesFormPayload(vaultConnID))
	if err != nil {
		return portalFormIDs{}, fmt.Errorf("failed to create communication preferences form: %w", err)
	}

	privacyID, err := importForm(buildPrivacySettingsFormPayload(vaultConnID))
	if err != nil {
		return portalFormIDs{}, fmt.Errorf("failed to create privacy settings form: %w", err)
	}

	return portalFormIDs{
		PersonalInfo:             personalInfoID,
		CommunicationPreferences: commPrefID,
		PrivacyConsent:           privacyID,
	}, nil
}

// ---- Portal payload types ----

type portalPayload struct {
	Slug       string            `json:"slug"`
	Name       string            `json:"name"`
	Client     portalClientRef   `json:"client"`
	Navigation *portalNavigation `json:"navigation,omitempty"`
	Pages      *portalPages      `json:"pages,omitempty"`
}

type portalClientRef struct {
	TokenEndpointAuthMethod string `json:"token_endpoint_auth_method"`
	ClientID                string `json:"client_id"`
	ClientSecret            string `json:"client_secret"`
}

type portalNavigation struct {
	Sidebar portalSidebar `json:"sidebar"`
}

type portalSidebar struct {
	Components []portalComponent `json:"components"`
}

type portalPages struct {
	Default string       `json:"default,omitempty"`
	Content []portalPage `json:"content"`
}

type portalPage struct {
	Title      string            `json:"title"`
	Slug       string            `json:"slug"`
	Components []portalComponent `json:"components,omitempty"`
}

// portalComponent covers both sidebar and page components.
// Config is map[string]any because component types have heterogeneous shapes.
type portalComponent struct {
	Type   string         `json:"type"`
	Config map[string]any `json:"config,omitempty"`
}

// portalResult holds the fields read back from the create-portal response.
type portalResult struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// toPortalSlug derives a URL-safe kebab-case slug from a portal name.
// Pure function: no I/O.
func toPortalSlug(name string) string {
	var b strings.Builder
	prevHyphen := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevHyphen = false
		case b.Len() > 0 && !prevHyphen:
			b.WriteByte('-')
			prevHyphen = true
		}
	}
	slug := strings.TrimRight(b.String(), "-")
	if slug == "" {
		return "my-portal"
	}
	return slug
}

// portalFormIDs holds the IDs of the three Forms required by the portal.
// All three will be populated once Form provisioning is implemented.
type portalFormIDs struct {
	PersonalInfo             string // form_id for the "Personal information" section
	PrivacyConsent           string // form_id for the "Privacy & data consent" section
	CommunicationPreferences string // form_id for the "Communication preferences" section
}

// buildDefaultPortal returns the full portal payload matching the Universal Portals
// template: four pages with all sections, including form-backed ones.
// Pure function: no I/O.
func buildDefaultPortal(slug, name, clientID, clientSecret string, forms portalFormIDs) portalPayload {
	section := func(title, description string, children ...portalComponent) portalComponent {
		return portalComponent{
			Type: "page:component:auth0:structure:section",
			Config: map[string]any{
				"title":       title,
				"description": description,
				"variant":     "card",
				"children":    children,
			},
		}
	}
	builtin := func(componentType string) portalComponent {
		return portalComponent{Type: componentType}
	}
	form := func(formID, completionMessage string) portalComponent {
		return portalComponent{
			Type: "page:component:auth0:form",
			Config: map[string]any{
				"form_id":            formID,
				"completion_message": completionMessage,
			},
		}
	}
	navLink := func(label, to, icon string) portalComponent {
		return portalComponent{
			Type:   "sidebar:component:auth0:internal_link",
			Config: map[string]any{"label": label, "to": to, "icon": icon},
		}
	}

	return portalPayload{
		Slug: slug,
		Name: name,
		Client: portalClientRef{
			TokenEndpointAuthMethod: "client_secret_post",
			ClientID:                clientID,
			ClientSecret:            clientSecret,
		},
		Navigation: &portalNavigation{
			Sidebar: portalSidebar{
				Components: []portalComponent{
					navLink("Profile", "profile", "user"),
					navLink("Security", "security", "shield"),
					navLink("Organization", "organization", "building"),
					navLink("Legal & privacy", "legal-privacy", "file-text"),
				},
			},
		},
		Pages: &portalPages{
			Default: "profile",
			Content: []portalPage{
				{
					Title: "Profile",
					Slug:  "profile",
					Components: []portalComponent{
						section(
							"Personal information",
							"Basic info about you, like your name and contact details, that you use across services.",
							form(forms.PersonalInfo, "Your personal information has been updated."),
						),
						section(
							"Passkeys",
							"Use your fingerprint, face, or screen lock instead of a password to sign in quickly and more securely.",
							builtin("page:component:auth0:my_account:passkey_management"),
						),
					},
				},
				{
					Title: "Security",
					Slug:  "security",
					Components: []portalComponent{
						section(
							"Multi-factor authentication",
							"Add an extra layer of protection to your account by requiring a second verification step each time you sign in.",
							builtin("page:component:auth0:my_account:mfa_management"),
						),
						section(
							"Sessions & devices",
							"Review the devices and sessions that are currently signed in to your account.",
							portalComponent{
								Type:   "page:component:auth0:typography:rich_text",
								Config: map[string]any{"content": "<p><em>Sessions &amp; devices management coming soon.</em></p>"},
							},
						),
					},
				},
				{
					Title: "Organization",
					Slug:  "organization",
					Components: []portalComponent{
						section(
							"Organization details",
							"Update your organization's name and other details visible to its members.",
							builtin("page:component:auth0:my_organization:details_edit"),
						),
					},
				},
				{
					Title: "Legal & privacy",
					Slug:  "legal-privacy",
					Components: []portalComponent{
						section(
							"Privacy & data consent",
							"Control how your personal data is collected and used across our services.",
							form(forms.PrivacyConsent, "Your privacy preferences have been saved."),
						),
						section(
							"Communication preferences",
							"Choose which emails and notifications you'd like to receive from us.",
							form(forms.CommunicationPreferences, "Your communication preferences have been updated."),
						),
					},
				},
			},
		},
	}
}

// createPortal POSTs a new portal with the full default page structure.
func createPortal(ctx context.Context, h auth0.HTTPClientAPI, slug, name, clientID, clientSecret string, forms portalFormIDs) (portalResult, error) {
	payload := buildDefaultPortal(slug, name, clientID, clientSecret, forms)
	var result portalResult
	if err := rawAPIPost(ctx, h, payload, &result, "portals"); err != nil {
		return portalResult{}, err
	}
	return result, nil
}

// portalManageClientURL builds the Management Dashboard URL for the given client.
// Pure function: takes only the values it needs, no struct access.
func portalManageClientURL(tenantDomain, tenantName, clientID string) string {
	parts := strings.Split(tenantDomain, ".")
	region := "us"
	if len(parts) > 3 {
		region = parts[1]
	}
	return fmt.Sprintf("%s/dashboard/%s/%s/applications/%s/settings",
		deriveServiceURL("manage", tenantDomain), region, tenantName, clientID)
}
