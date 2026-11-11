package portals

import "strings"

// The scopes the portal application needs on each API, as listed in the setup guide.
var (
	myAccountScopes = []string{
		"read:me:factors",
		"read:me:authentication_methods",
		"create:me:authentication_methods",
		"delete:me:authentication_methods",
	}

	// The scopes the organization components of the business template use: the details of the
	// organization (read and update), and its verified domains and identity provider domains.
	myOrganizationScopes = []string{
		"read:my_org:details",
		"update:my_org:details",
		"read:my_org:configuration",
		"read:my_org:domains",
		"create:my_org:domains",
		"update:my_org:domains",
		"delete:my_org:domains",
		"read:my_org:identity_providers",
		"create:my_org:identity_providers_domains",
		"delete:my_org:identity_providers_domains",
	}

	// The scopes of the machine-to-machine application behind the vault connection of the forms.
	formsVaultScopes = []string{"update:users", "update:users_app_metadata"}

	portalGrantTypes = []string{
		"authorization_code",
		"refresh_token",
		"http://auth0.com/oauth/grant-type/mfa-oob",
		"http://auth0.com/oauth/grant-type/mfa-otp",
		"http://auth0.com/oauth/grant-type/mfa-recovery-code",
	}

	backchannelLogoutInitiators = []string{
		"idp-logout",
		"rp-logout",
		"session-expired",
		"session-revoked",
		"account-deleted",
		"account-deactivated",
	}
)

const (
	// The suffix of the name of the portal application, after the name of the portal.
	portalApplicationSuffix = "(Universal Portals)"
	// The name of the application and of the vault connection that serve the forms of every portal.
	formsVaultName = "Universal Portals forms vault"
	// The metadata that tells which setup created an application.
	portalSlugMetadata = "portals_setup_slug"
	formsVaultMetadata = "auth0_portals_role"
	formsVaultRole     = "forms-vault"

	refreshTokenIdleSeconds     = 3600
	refreshTokenAbsoluteSeconds = 86400
)

// MyAccountScopes returns the scopes the portal application needs on the My Account API.
func MyAccountScopes() []string { return append([]string(nil), myAccountScopes...) }

// MyOrganizationScopes returns the scopes the portal application needs on the My Organization API.
func MyOrganizationScopes() []string { return append([]string(nil), myOrganizationScopes...) }

// FormsVaultScopes returns the scopes of the application behind the vault connection.
func FormsVaultScopes() []string { return append([]string(nil), formsVaultScopes...) }

func myAccountAudience(tenantDomain string) string { return "https://" + tenantDomain + "/me/" }

func myOrganizationAudience(tenantDomain string) string {
	return "https://" + tenantDomain + "/my-org/"
}

func managementAudience(tenantDomain string) string { return "https://" + tenantDomain + "/api/v2/" }

// apiAccess is an API the portal application calls, with the scopes it needs there.
type apiAccess struct {
	audience string
	scope    []string
}

// audiencesFor returns the APIs a template calls. Setup creates a client grant and a refresh token
// policy for each.
func audiencesFor(template Template, tenantDomain string) []apiAccess {
	account := apiAccess{audience: myAccountAudience(tenantDomain), scope: myAccountScopes}
	if template == TemplateConsumer {
		return []apiAccess{account}
	}

	return []apiAccess{{audience: myOrganizationAudience(tenantDomain), scope: myOrganizationScopes}, account}
}

// apiLabel is the name of an API in what setup shows.
func apiLabel(audience, tenantDomain string) string {
	switch audience {
	case myAccountAudience(tenantDomain):
		return "My Account API"
	case myOrganizationAudience(tenantDomain):
		return "My Organization API"
	default:
		return "Management API"
	}
}

// portalApplication is what the settings of the portal application are built from.
type portalApplication struct {
	name         string
	slug         string
	template     Template
	tenantDomain string
	// The host the portal is served from: the custom domain, or the tenant domain.
	host           string
	isCustomDomain bool
}

// The type of the application of a portal, and how it proves who it is at the token endpoint.
const (
	portalAppType         = "regular_web"
	portalTokenAuthMethod = "client_secret_post"
)

func boolPointer(value bool) *bool { return &value }

func intPointer(value int) *int { return &value }

// portalClientSettings are the settings of the application a portal signs users in with.
func portalClientSettings(app portalApplication) ClientSettings {
	settings := ClientSettings{
		Name:                    app.name,
		AppType:                 portalAppType,
		IsFirstParty:            boolPointer(true),
		OIDCConformant:          boolPointer(true),
		TokenEndpointAuthMethod: portalTokenAuthMethod,
		IPHeaderTrusted:         boolPointer(true),
		Callbacks:               []string{"https://" + app.host + "/portals/auth/callback"},
		// Users land on the host of the portal after they log out.
		AllowedLogoutURLs:           []string{"https://" + app.host},
		GrantTypes:                  append([]string(nil), portalGrantTypes...),
		OrganizationUsage:           "allow",
		OrganizationRequireBehavior: "no_prompt",
		RefreshToken:                refreshTokenSettings(app.template, app.tenantDomain),
		Metadata:                    map[string]string{portalSlugMetadata: app.slug},
		// Lets a native application open the portal with a session transfer token.
		SessionTransfer: &SessionTransfer{AllowedAuthenticationMethods: []string{"query"}, EnforceDeviceBinding: "ip"},
	}

	if app.isCustomDomain {
		settings.OIDCLogout = &OIDCLogout{
			BackchannelLogoutURLs: []string{"https://" + app.host + "/portals/auth/backchannel-logout"},
			Mode:                  "custom",
			SelectedInitiators:    append([]string(nil), backchannelLogoutInitiators...),
		}
	}

	return settings
}

func refreshTokenSettings(template Template, tenantDomain string) *RefreshToken {
	token := &RefreshToken{
		RotationType:      "non-rotating",
		ExpirationType:    "expiring",
		TokenLifetime:     intPointer(refreshTokenAbsoluteSeconds),
		IdleTokenLifetime: intPointer(refreshTokenIdleSeconds),
		// The API keeps a lifetime infinite and ignores its seconds unless the flag is sent as false.
		InfiniteTokenLifetime:     boolPointer(false),
		InfiniteIdleTokenLifetime: boolPointer(false),
	}

	for _, access := range audiencesFor(template, tenantDomain) {
		token.Policies = append(token.Policies, RefreshTokenPolicy{
			Audience: access.audience,
			Scope:    append([]string(nil), access.scope...),
		})
	}

	return token
}

// formsVaultClientSettings are the settings of the application behind the vault connection.
func formsVaultClientSettings(name string) ClientSettings {
	return ClientSettings{
		Name:     name,
		AppType:  "non_interactive",
		Metadata: map[string]string{formsVaultMetadata: formsVaultRole},
	}
}

// portalName is the name of the application of a portal.
func portalName(name string) string {
	return strings.TrimSpace(name) + " " + portalApplicationSuffix
}
