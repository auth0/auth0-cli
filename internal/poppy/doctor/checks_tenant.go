package doctor

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	managementv3 "github.com/auth0/go-auth0/v3/management"

	coreagents "github.com/auth0/auth0-cli/internal/agents"
	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/poppy/setup"
)

// checkSettings runs D8.
func (r *runner) checkSettings(ctx context.Context, t *Tenant, apis []coreagents.API, apisErr error, c *collector) {
	settings, err := t.State.Settings(ctx)
	if err != nil {
		r.unreadable(c, "D8", specResource, "the tenant settings", scopeTenantSettings, err)
		return
	}

	fix := r.setupFix()
	switches := []struct {
		code    string
		setting string
		passed  string
		on      bool
	}{
		{"D8.cimd", "client_id_metadata_document_supported", "the tenant supports client ID metadata documents", settings.CIMD},
		{"D8.iss_parameter", "authorization_response_iss_parameter_supported", "the tenant returns iss in sign-in responses", settings.IssParameter},
	}
	for _, s := range switches {
		if s.on {
			c.pass(s.code+"_on", specTenantSettings, s.passed)
		} else {
			c.fail(s.code+"_off", specTenantSettings, s.setting+" is off", fix)
		}
	}

	if settings.ResourceParameterProfile == string(managementv3.TenantSettingsResourceParameterProfileCompatibility) {
		c.pass("D8.resource_parameter_profile", specTenantSettings, "the resource parameter profile is compatibility")
	} else {
		c.fail("D8.resource_parameter_profile_wrong", specTenantSettings,
			fmt.Sprintf("resource_parameter_profile is %q, and it must be compatibility", settings.ResourceParameterProfile), fix)
	}

	if apisErr == nil && len(apis) == 1 {
		if settings.DefaultAudience != "" {
			c.pass("D8.default_audience", specTenantSettings, "the default audience is set")
		} else {
			c.fail("D8.default_audience_missing", specTenantSettings, "default_audience is not set, and the tenant has one Poppy API ("+apis[0].Identifier+")", fix)
		}
	}

	want := strings.ToLower(r.domain)
	if setup.HasCompanyDomain(settings.CompanyDomains, want) {
		c.pass("D8.company_domain", specResource, "the tenant lists "+want+" as a company domain")
	} else {
		c.fail("D8.company_domain_missing", specResource, "personal_agent_protocol_company_domains does not list "+want, fix)
	}

	for _, entry := range settings.CompanyDomains {
		if !setup.IsBareCompanyDomain(entry) {
			c.warn("D8.company_domain_malformed", specResource, fmt.Sprintf("personal_agent_protocol_company_domains has %q, which is not a bare lower-case hostname", entry),
				hint("Replace it with a bare lower-case host such as shop.example, without a scheme, port, path or trailing slash."))
		}
	}
}

// checkWebApps runs D9 on every app that has the browser join on. The config
// file does not name the web app, so the tenant decides which apps count.
func (r *runner) checkWebApps(ctx context.Context, t *Tenant, tenantAPIs []setup.API, c *collector) {
	apps, err := t.State.BrowserJoinApps(ctx)
	if err != nil {
		r.unreadable(c, "D9", specWebApp, "the web apps", scopeClients, err)
		return
	}

	if len(apps) == 0 {
		c.skipWith("D9.no_browser_join_app", specWebApp, SkipNotAvailable, "no app has allow_personal_agent_browser_join on", r.setupFix())
		return
	}

	for _, app := range apps {
		r.checkWebApp(app, c)
		r.checkWebAppAccess(ctx, t, app, tenantAPIs, c)
	}
}

// checkAPISettings runs D8 on the settings of each Poppy API: the ones that have
// the Personal Agent Protocol on, and the ones the config file points at, which
// may have it off. It reports nothing when the APIs cannot be read, because the
// other checks already say what is missing.
func (r *runner) checkAPISettings(apis []setup.API, apisErr error, file *config.Config, c *collector) {
	if apisErr != nil {
		return
	}

	var origins []string
	if file != nil {
		for _, entry := range file.APIs {
			if origin := config.Origin(entry.URL); origin != "" {
				origins = append(origins, origin)
			}
		}
	}

	for _, api := range apis {
		if !api.Poppy && !slices.Contains(origins, config.Origin(api.Identifier)) {
			continue
		}

		fix := r.setupFix("--api", quoteArg(api.Identifier))

		if api.Poppy {
			c.pass("D8.api_poppy", specResource, api.Identifier+" has use_for_personal_agent_protocol on")
		} else {
			c.fail("D8.api_poppy_off", specResource, api.Identifier+" has use_for_personal_agent_protocol off", fix)

			continue
		}

		if api.AgentSubjectClaims == "auth0-v1" {
			c.pass("D8.api_subject_claims", specResource, api.Identifier+" has agent_subject_claims auth0-v1")
		} else {
			c.fail("D8.api_subject_claims_wrong", specResource, fmt.Sprintf("%s has agent_subject_claims %q, and it must be auth0-v1", api.Identifier, api.AgentSubjectClaims), fix)
		}

		if api.GuestPolicy == "require_client_grant" {
			c.pass("D8.api_guest_policy", specResource, api.Identifier+" has subject_type_authorization.anonymous_user.policy require_client_grant")
		} else {
			c.fail("D8.api_guest_policy_wrong", specResource,
				fmt.Sprintf("%s has subject_type_authorization.anonymous_user.policy %q, and it must be require_client_grant", api.Identifier, api.GuestPolicy), fix)
		}
	}
}

// checkWebAppAccess runs D9 on the access of a web app: it should hold at least
// one scope on each Poppy API, for signed-in users or for guests. A web app with
// none can still be used for the browser join, so this only warns.
func (r *runner) checkWebAppAccess(ctx context.Context, t *Tenant, app setup.App, tenantAPIs []setup.API, c *collector) {
	name := cmp.Or(app.Name, app.ClientID)

	for _, api := range tenantAPIs {
		if !api.Poppy {
			continue
		}

		scopes, err := t.State.GrantedScopes(ctx, app.ClientID, api.Identifier)
		if err != nil {
			r.unreadable(c, "D9", specWebApp, "the grants of "+name, scopeGrants, err)

			return
		}

		if len(scopes) > 0 {
			c.pass("D9.app_access", specWebApp, fmt.Sprintf("%s can use %s with %d scope(s)", name, api.Identifier, len(scopes)))
		} else {
			c.warn("D9.app_access_none", specWebApp, fmt.Sprintf("%s has no scopes on %s", name, api.Identifier),
				hint(fmt.Sprintf("Grant the app scopes on the API: auth0 client-grants create --client-id %s --audience %s --subject-type user --scopes \"<scope1,scope2>\"", app.ClientID, api.Identifier)))
		}
	}
}

func (r *runner) checkWebApp(app setup.App, c *collector) {
	name := cmp.Or(app.Name, app.ClientID)
	appFix := r.setupFix("--app", quoteArg(app.ClientID))

	if app.FirstParty {
		c.pass("D9.first_party", specWebApp, name+" is first-party")
	} else {
		c.fail("D9.first_party_off", specWebApp, name+" is not first-party",
			fix{command: appFix.command, hint: "The browser join works only for first-party apps. Pick another one."})
	}

	if isHTTPS(app.LoginURI) {
		c.pass("D9.login_uri", specWebApp, name+" has an https initiate_login_uri")
	} else {
		c.fail("D9.login_uri_missing", specWebApp, fmt.Sprintf("%s has no https initiate_login_uri (it is %q)", name, app.LoginURI),
			hint("Set it to the page of your site that starts a sign-in: "+r.setupCommand("--app", quoteArg(app.ClientID), "--login-uri", "https://<host>/<login path>")))
	}

	if app.BrowserJoin {
		c.pass("D9.browser_join", specWebApp, name+" has allow_personal_agent_browser_join on")
	} else {
		c.fail("D9.browser_join_off", specWebApp, name+" has allow_personal_agent_browser_join off", appFix)
	}
}

// checkMFA runs D10.
func (r *runner) checkMFA(ctx context.Context, t *Tenant, c *collector) {
	policies, err := t.Policies.List(ctx)
	if err != nil {
		r.unreadable(c, "D10", "", "the MFA policies", scopeMFAPolicies, err)
		return
	}

	if slices.Contains(policies, managementv3.MfaPolicyEnumAllApplications) {
		c.warn("D10.mfa_all", "", "MFA is required for all logins; browser joins cannot show MFA prompts, and post-login Actions can also ask for MFA, which doctor cannot see",
			hint("Do not require MFA for all logins, or skip it for the browser join in a post-login Action."))
		return
	}

	c.pass("D10.mfa", "", "MFA is not required for all logins (post-login Actions are not checked)")
}
