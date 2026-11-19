package doctor

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"slices"
	"strings"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"

	coreagents "github.com/auth0/auth0-cli/internal/agents"
	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/poppy/setup"
)

const (
	specTenantSettings = "4"
	specWebApp         = "design 5.5"
	specAgentApp       = "4.1"
	specAgentKeys      = "5.2"

	// NoLogin is why the tenant checks are skipped.
	noLogin = "no usable tenant login, so the tenant checks did not run"

	// Unavailable is what a 403 that is not a missing scope means.
	unavailable = "the tenant may lack Agent as Principal or the Poppy beta"

	// The metadata statuses of an agent app, as the agents package reports them.
	metadataAvailable = "available"
	metadataUnknown   = "unknown"
)

// The Management API scopes the tenant checks need, for the hint of a skipped check.
// The checks declare their own list and do not follow the scopes of the setup
// command, because the two read different things.
var (
	scopeTenantSettings = []string{"read:tenant_settings"}
	scopeClients        = []string{"read:clients"}
	scopeGrants         = []string{"read:client_grants"}
	scopeMFAPolicies    = []string{"read:mfa_policies"}
	scopeAgents         = []string{"read:agents", "read:clients", "read:client_grants", "read:resource_servers"}
	scopeAgentKeys      = []string{"read:clients", "read:client_credentials"}
	scopeConfigFile     = []string{"read:tenant_settings", "read:custom_domains", "read:resource_servers", "read:clients"}
)

// tenantChecks lists the checks that read the tenant, in order, with their spec
// sections.
var tenantChecks = []struct{ id, spec string }{
	{"D8", specResource}, {"D9", specWebApp}, {"D10", ""},
	{"D11", specAgentApp}, {"D12", specAgentApp}, {"D13", ""}, {"D14", ""},
}

// StateReader is the part of setup.Reader that the tenant checks use.
type StateReader interface {
	Settings(ctx context.Context) (setup.Tenant, error)
	TenantHosts(ctx context.Context) ([]string, error)
	BrowserJoinApps(ctx context.Context) ([]setup.App, error)
	Read(ctx context.Context, apis []string, app string) (*setup.State, error)
	ListPoppyAPIs(ctx context.Context) ([]setup.API, error)
	ListAPIs(ctx context.Context) ([]setup.API, error)
	GrantedScopes(ctx context.Context, clientID, audience string) ([]string, error)
}

// AgentService is the part of poppyagents.Service that the tenant checks use.
type AgentService interface {
	PoppyAPIs(ctx context.Context, picked []string) ([]coreagents.API, error)
	List(ctx context.Context, all bool) ([]coreagents.GrantedAgent, error)
	Show(ctx context.Context, ref string) (*coreagents.GrantedDetail, error)
}

// PolicyAPI is the part of the guardian policies API that the tenant checks use.
type PolicyAPI interface {
	List(ctx context.Context, opts ...option.RequestOption) (managementv3.ListGuardianPoliciesResponseContent, error)
}

// Tenant gives checks D8 to D14 read access to the tenant.
type Tenant struct {
	// Domain is the domain of the tenant, which names it in the messages.
	Domain   string
	State    StateReader
	Agents   AgentService
	Policies PolicyAPI
}

// skipTenantChecks records every check that reads the tenant as skipped, for the
// same reason. The code of each result is the ID of the check and the suffix.
func skipTenantChecks(suffix string, reason SkipReason, message string, f fix) []Result {
	var skipped collector
	for _, c := range tenantChecks {
		skipped.skipWith(c.id+"."+suffix, c.spec, reason, message, f)
	}

	return skipped.results
}

// checkTenant runs D8 to D14. Without access to the tenant it skips them. The
// issuer is the one discovery found, empty when it is not known: when the logged-in
// tenant does not serve the domain the checks do not apply, and each one is
// skipped. It also tells whether the checks ran.
func (r *runner) checkTenant(ctx context.Context, opts Options, issuer string, file *config.Config, fileErr error) ([]Result, bool) {
	if opts.Tenant == nil {
		return skipTenantChecks("no_login", SkipNoLogin, noLogin, command("auth0 login")), false
	}

	t := opts.Tenant
	if serves, name := r.tenantServesDomain(ctx, t, issuer); !serves {
		message := fmt.Sprintf("you are logged in to %s, which does not list %s as a company domain, so the tenant checks do not apply", name, r.domain)

		return skipTenantChecks("different_tenant", SkipDifferentTenant, message,
			hint("Log in to the tenant that serves "+r.domain+", or choose it with --tenant.")), false
	}

	// A part that is off has no checks, whether the config file or the website's
	// poppy.json says so.
	webOff := r.liveWebOff || (file != nil && file.Web.Disabled)
	apiOff := r.liveAPIOff || (file != nil && file.Auth.Disabled)

	var (
		apis    []coreagents.API
		apisErr error
	)
	if !apiOff {
		apis, apisErr = t.Agents.PoppyAPIs(ctx, nil)
	}

	var d8, d9, d10, d14 collector
	r.checkSettings(ctx, t, apis, apisErr, &d8)

	var tenantAPIs []setup.API

	if apiOff {
		d8.skip("D8.api_off", specResource, SkipAPIOff, messageAPIOff)
	} else {
		var tenantAPIsErr error

		tenantAPIs, tenantAPIsErr = t.State.ListAPIs(ctx)
		r.checkAPISettings(tenantAPIs, tenantAPIsErr, file, &d8)
	}

	if webOff {
		d9.skip("D9.web_off", specWebApp, SkipWebOff, messageWebOff)
	} else {
		r.checkWebApps(ctx, t, tenantAPIs, &d9)
	}

	r.checkMFA(ctx, t, &d10)
	d11, d12, d13 := r.checkAgents(ctx, t, apis, apisErr, apiOff)

	r.checkConfig(ctx, t, opts.ConfigFileExplicit, file, fileErr, &d14)

	var results []Result
	for _, c := range []collector{d8, d9, d10, d11, d12, d13, d14} {
		results = append(results, c.results...)
	}

	return results, true
}

// checkConfig runs D14 on the config file. When the user did not name a file and
// the default one does not exist, it compares the poppy.json that D1 fetched
// instead, since the site already serves what the file would hold. Without
// either it is skipped.
func (r *runner) checkConfig(ctx context.Context, t *Tenant, explicit bool, file *config.Config, fileErr error, c *collector) {
	local := configSource{name: "the config file " + cmp.Or(r.configFile, config.DefaultFileName), repair: command(r.configCommand())}

	switch {
	case file != nil || fileErr != nil:
		r.checkConfigFile(ctx, t, local, file, fileErr, c)
	case !explicit && r.liveConfigErr != nil:
		c.skip("D14.live_unreadable", "", SkipNotAvailable,
			fmt.Sprintf("there is no config file at %s, and %s cannot be read as one: %v", cmp.Or(r.configFile, config.DefaultFileName), r.liveName(), r.liveConfigErr))
	case !explicit && r.liveConfig != nil:
		live := configSource{name: r.liveName(), repair: fix{command: r.configCommand(), hint: "Deploy the file it writes to " + r.origin + wellKnownPoppy + "."}}
		r.checkConfigFile(ctx, t, live, r.liveConfig, nil, c)
	default:
		c.skipWith("D14.no_config_file", "", SkipNoConfigFile, "there is no config file at "+cmp.Or(r.configFile, config.DefaultFileName)+" to compare with the tenant",
			command(r.configCommand()))
	}
}

// liveName names the poppy.json the website serves, for the messages of D14.
func (r *runner) liveName() string { return "the live poppy.json at " + r.origin + wellKnownPoppy }

// tenantServesDomain tells whether the tenant checks apply to the domain: the
// tenant lists the domain as a company domain, or the issuer is the canonical
// domain or a custom domain of the tenant (so that a tenant that serves the site
// but is missing the company domain is still diagnosed). It answers true when it
// cannot tell, because the tenant cannot be read. Otherwise it returns the name
// of the tenant, for the message of the skipped checks.
func (r *runner) tenantServesDomain(ctx context.Context, t *Tenant, issuer string) (bool, string) {
	name := t.Domain
	if name == "" {
		name = "another tenant"
	}

	settings, err := t.State.Settings(ctx)
	if err != nil {
		return true, name
	}

	if setup.HasCompanyDomain(settings.CompanyDomains, r.domain) {
		return true, name
	}

	u, err := url.Parse(issuer)
	if issuer == "" || err != nil || u.Hostname() == "" {
		return false, name
	}

	hosts, err := t.State.TenantHosts(ctx)
	if err != nil {
		return true, name
	}

	return slices.ContainsFunc(hosts, func(host string) bool { return strings.EqualFold(host, u.Hostname()) }), name
}

// readConfigFile reads the config file without validating it. It returns nil
// and no error when there is no file.
func readConfigFile(path string) (*config.Config, error) {
	cfg, err := config.ReadLenient(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	return cfg, err
}

// unreadable records that a check could not read the tenant. A missing scope
// skips the check with the login command that adds it; any other failure is a
// warning, because the tenant may only lack a feature.
func (r *runner) unreadable(c *collector, id, spec, thing string, scopes []string, err error) {
	var apiErr *core.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusForbidden {
		c.warn(id+".unreadable", spec, fmt.Sprintf("cannot check %s: %v", thing, err), fix{})
		return
	}

	if isInsufficientScope(apiErr) {
		c.skipWith(id+".missing_scope", spec, SkipMissingScope, fmt.Sprintf("cannot check %s: the login lacks %s", thing, strings.Join(scopes, ", ")),
			command("auth0 login --scopes "+strings.Join(scopes, ",")))
		return
	}

	c.warn(id+".unavailable", spec, fmt.Sprintf("cannot check %s: %s", thing, unavailable), hint("Contact support."))
}

// isInsufficientScope reports whether a 403 says the token lacks a scope.
func isInsufficientScope(apiErr *core.APIError) bool {
	inner := apiErr.Unwrap()
	if inner == nil {
		return false
	}

	var body struct {
		ErrorCode string `json:"errorCode"`
	}

	return json.Unmarshal([]byte(inner.Error()), &body) == nil && body.ErrorCode == "insufficient_scope"
}
