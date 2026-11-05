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

	"github.com/auth0/auth0-cli/internal/poppy/agents"
	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/poppy/setup"
)

const (
	specTenantSettings = "4"
	specWebApp         = "design 5.5"
	specAgentApp       = "4.1"
	specAgentKeys      = "5.2"

	// NoLogin is why the tenant checks are skipped.
	noLogin = "no usable tenant login; run auth0 login to check the tenant"

	// Unavailable is what a 403 that is not a missing scope means.
	unavailable = "the tenant may lack Agent as Principal or the Poppy beta"

	// The metadata statuses of an agent app, as the agents package reports them.
	metadataAvailable = "available"
	metadataUnknown   = "unknown"
)

// The Management API scopes the tenant checks need, for the hint of a skipped check.
var (
	scopeTenantSettings = []string{"read:tenant_settings"}
	scopeClients        = []string{"read:clients"}
	scopeMFAPolicies    = []string{"read:mfa_policies"}
	scopeAgents         = []string{"read:agents", "read:clients", "read:client_grants", "read:resource_servers"}
	scopeAgentKeys      = []string{"read:clients", "read:client_credentials"}
)

// StateReader is the part of setup.Reader that the tenant checks use.
type StateReader interface {
	Settings(ctx context.Context) (setup.Tenant, error)
	TenantHosts(ctx context.Context) ([]string, error)
	BrowserJoinApps(ctx context.Context) ([]setup.App, error)
	Read(ctx context.Context, apis []string, app string) (*setup.State, error)
	ListPoppyAPIs(ctx context.Context) ([]setup.API, error)
}

// AgentService is the part of agents.Service that the tenant checks use.
type AgentService interface {
	PoppyAPIs(ctx context.Context, picked []string) ([]agents.API, error)
	List(ctx context.Context, all bool) ([]agents.Agent, error)
	Show(ctx context.Context, ref string) (*agents.Detail, error)
}

// PolicyAPI is the part of the guardian policies API that the tenant checks use.
type PolicyAPI interface {
	List(ctx context.Context, opts ...option.RequestOption) (managementv3.ListGuardianPoliciesResponseContent, error)
}

// Tenant gives checks D8 to D14 read access to the tenant.
type Tenant struct {
	State    StateReader
	Agents   AgentService
	Policies PolicyAPI
}

// checkTenant runs D8 to D14. Without access to the tenant it skips them. The
// issuer is the one discovery found, empty when it is not known: when it is
// known and the logged-in tenant does not serve the domain, the checks do not
// apply and a single skipped result says so.
func (r *runner) checkTenant(ctx context.Context, opts Options, issuer string) []Result {
	file, fileErr := readConfigFile(opts.ConfigFile)
	hasFile := file != nil || fileErr != nil

	if opts.Tenant == nil {
		var skipped collector
		for _, c := range []struct{ id, spec string }{
			{"D8", specResource}, {"D9", specWebApp}, {"D10", ""},
			{"D11", specAgentApp}, {"D12", specAgentApp}, {"D13", ""},
		} {
			skipped.skip(c.id, c.spec, noLogin)
		}
		if hasFile {
			skipped.skip("D14", "", noLogin)
		}

		return skipped.results
	}

	t := opts.Tenant
	if !r.tenantServesDomain(ctx, t, issuer) {
		var skipped collector
		skipped.skip("D8", "", fmt.Sprintf("you are logged in to a different tenant than the one that serves %s. Log in to that tenant to run these checks.", r.domain))

		return skipped.results
	}

	apis, apisErr := t.Agents.PoppyAPIs(ctx, nil)

	var d8, d9, d10, d14 collector
	r.checkSettings(ctx, t, apis, apisErr, &d8)
	r.checkWebApps(ctx, t, &d9)
	checkMFA(ctx, t, &d10)
	d11, d12, d13 := r.checkAgents(ctx, t, apis, apisErr)
	if hasFile {
		r.checkConfigFile(ctx, t, opts.ConfigFile, file, fileErr, &d14)
	}

	var results []Result
	for _, c := range []collector{d8, d9, d10, d11, d12, d13, d14} {
		results = append(results, c.results...)
	}

	return results
}

// tenantServesDomain tells whether the tenant checks apply to the domain: the
// issuer is the canonical domain or a custom domain of the tenant, or the tenant
// already lists the domain as a company domain. It answers true when it cannot
// tell, because the issuer is unknown or the tenant cannot be read, so that a
// broken setup is still diagnosed.
func (r *runner) tenantServesDomain(ctx context.Context, t *Tenant, issuer string) bool {
	u, err := url.Parse(issuer)
	if issuer == "" || err != nil || u.Hostname() == "" {
		return true
	}

	known := true

	hosts, err := t.State.TenantHosts(ctx)
	if err != nil {
		known = false
	}
	if slices.ContainsFunc(hosts, func(host string) bool { return strings.EqualFold(host, u.Hostname()) }) {
		return true
	}

	settings, err := t.State.Settings(ctx)
	if err != nil {
		known = false
	}
	want := "https://" + r.domain
	if slices.ContainsFunc(settings.CompanyDomains, func(entry string) bool { return strings.EqualFold(entry, want) }) {
		return true
	}

	return !known
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
func unreadable(c *collector, id, spec, thing string, scopes []string, err error) {
	var apiErr *core.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusForbidden {
		c.warn(id, spec, fmt.Sprintf("cannot check %s: %v", thing, err), "")
		return
	}

	if isInsufficientScope(apiErr) {
		c.add(Skipped, id, spec, fmt.Sprintf("cannot check %s: the login lacks %s", thing, strings.Join(scopes, ", ")),
			"auth0 login --scopes "+strings.Join(scopes, ","))
		return
	}

	c.warn(id, spec, fmt.Sprintf("cannot check %s: %s", thing, unavailable), "Contact support.")
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

// checkSettings runs D8.
func (r *runner) checkSettings(ctx context.Context, t *Tenant, apis []agents.API, apisErr error, c *collector) {
	settings, err := t.State.Settings(ctx)
	if err != nil {
		unreadable(c, "D8", specResource, "the tenant settings", scopeTenantSettings, err)
		return
	}

	fix := "auth0 poppy setup --company-domain " + r.domain
	switches := []struct {
		setting string
		passed  string
		on      bool
	}{
		{"client_id_metadata_document_supported", "the tenant supports client ID metadata documents", settings.CIMD},
		{"authorization_response_iss_parameter_supported", "the tenant returns iss in sign-in responses", settings.IssParameter},
	}
	for _, s := range switches {
		if s.on {
			c.pass("D8", specTenantSettings, s.passed)
		} else {
			c.fail("D8", specTenantSettings, s.setting+" is off", fix)
		}
	}

	if settings.ResourceParameterProfile == string(managementv3.TenantSettingsResourceParameterProfileCompatibility) {
		c.pass("D8", specTenantSettings, "the resource parameter profile is compatibility")
	} else {
		c.fail("D8", specTenantSettings,
			fmt.Sprintf("resource_parameter_profile is %q, and it must be compatibility", settings.ResourceParameterProfile), fix)
	}

	if apisErr == nil && len(apis) == 1 {
		if settings.DefaultAudience != "" {
			c.pass("D8", specTenantSettings, "the default audience is set")
		} else {
			c.fail("D8", specTenantSettings, "default_audience is not set, and the tenant has one Poppy API ("+apis[0].Identifier+")", fix)
		}
	}

	want := "https://" + r.domain
	if slices.ContainsFunc(settings.CompanyDomains, func(entry string) bool { return strings.EqualFold(entry, want) }) {
		c.pass("D8", specResource, "the tenant lists "+want+" as a company domain")
	} else {
		c.fail("D8", specResource, "personal_agent_protocol_company_domains does not list "+want, fix)
	}

	for _, entry := range settings.CompanyDomains {
		if !isOrigin(entry) {
			c.warn("D8", specResource, fmt.Sprintf("personal_agent_protocol_company_domains has %q, which is not an https://host origin", entry),
				"Replace it with an https://<host> entry, without a path, port or trailing slash.")
		}
	}
}

// isOrigin reports whether a value is an https origin: a host with no path,
// port, user, query or fragment.
func isOrigin(value string) bool {
	u, err := url.Parse(value)

	return err == nil && u.Scheme == "https" && u.Host != "" && u.Host == u.Hostname() &&
		u.Path == "" && u.RawQuery == "" && u.Fragment == "" && u.User == nil && !u.ForceQuery
}

// checkWebApps runs D9 on every app that has the browser join on. The config
// file does not name the web app, so the tenant decides which apps count.
func (r *runner) checkWebApps(ctx context.Context, t *Tenant, c *collector) {
	apps, err := t.State.BrowserJoinApps(ctx)
	if err != nil {
		unreadable(c, "D9", specWebApp, "the web apps", scopeClients, err)
		return
	}

	if len(apps) == 0 {
		c.skip("D9", specWebApp, "no app has allow_personal_agent_browser_join on; run auth0 poppy setup")
		return
	}

	for _, app := range apps {
		r.checkWebApp(app, c)
	}
}

func (r *runner) checkWebApp(app setup.App, c *collector) {
	name := cmp.Or(app.Name, app.ClientID)
	fix := fmt.Sprintf("auth0 poppy setup --company-domain %s --app %s", r.domain, app.ClientID)

	if app.FirstParty {
		c.pass("D9", specWebApp, name+" is first-party")
	} else {
		c.fail("D9", specWebApp, name+" is not first-party", "The browser join works only for first-party apps. Pick another with "+fix)
	}

	if isHTTPS(app.LoginURI) {
		c.pass("D9", specWebApp, name+" has an https initiate_login_uri")
	} else {
		c.fail("D9", specWebApp, fmt.Sprintf("%s has no https initiate_login_uri (it is %q)", name, app.LoginURI), fix+" --login-uri https://<host>/<login route>")
	}

	if app.BrowserJoin {
		c.pass("D9", specWebApp, name+" has allow_personal_agent_browser_join on")
	} else {
		c.fail("D9", specWebApp, name+" has allow_personal_agent_browser_join off", fix)
	}
}

// checkMFA runs D10.
func checkMFA(ctx context.Context, t *Tenant, c *collector) {
	policies, err := t.Policies.List(ctx)
	if err != nil {
		unreadable(c, "D10", "", "the MFA policies", scopeMFAPolicies, err)
		return
	}

	if slices.Contains(policies, managementv3.MfaPolicyEnumAllApplications) {
		c.warn("D10", "", "MFA is required for all logins; browser joins cannot show MFA prompts, and post-login Actions can also ask for MFA, which doctor cannot see",
			"Do not require MFA for all logins, or skip it for the browser join in a post-login Action.")
		return
	}

	c.pass("D10", "", "MFA is not required for all logins (post-login Actions are not checked)")
}

// checkAgents runs D11 to D13 on every Poppy agent.
func (r *runner) checkAgents(ctx context.Context, t *Tenant, apis []agents.API, apisErr error) (d11, d12, d13 collector) {
	ids := []struct {
		id   string
		spec string
		col  *collector
	}{{"D11", specAgentApp, &d11}, {"D12", specAgentApp, &d12}, {"D13", "", &d13}}

	each := func(record func(id, spec string, c *collector)) {
		for _, check := range ids {
			record(check.id, check.spec, check.col)
		}
	}

	if errors.Is(apisErr, agents.ErrNoPoppyAPI) {
		each(func(id, spec string, c *collector) { c.skip(id, spec, "no Poppy API") })
		return d11, d12, d13
	}

	var list []agents.Agent
	err := apisErr
	if err == nil {
		list, err = t.Agents.List(ctx, false)
	}
	if err != nil {
		each(func(id, spec string, c *collector) { unreadable(c, id, spec, "the agents", scopeAgents, err) })
		return d11, d12, d13
	}

	if len(list) == 0 {
		each(func(id, spec string, c *collector) { c.skip(id, spec, "no agent has a grant to a Poppy API") })
		return d11, d12, d13
	}

	for _, agent := range list {
		checkAgentApp(agent, &d11)
		r.checkAgentKeys(ctx, t, agent, &d12)
		checkAgentGrants(agent, apis, &d13)
	}

	return d11, d12, d13
}

// agentName is how the results name an agent.
func agentName(agent agents.Agent) string {
	return cmp.Or(agent.URL, agent.AgentID, agent.Name)
}

// registerFix is the command that registers an agent again.
func registerFix(agent agents.Agent) string {
	return "auth0 poppy agents register " + agent.URL
}

// checkAgentApp runs D11 on one agent.
func checkAgentApp(agent agents.Agent, c *collector) {
	name := agentName(agent)

	switch agent.MetadataStatus {
	case metadataAvailable:
		c.pass("D11", specAgentApp, name+": metadata is available")
	case metadataUnknown:
		c.warn("D11", specAgentApp, name+": the metadata status could not be read", "")
	default:
		c.fail("D11", specAgentApp, fmt.Sprintf("%s: metadata status is %q", name, agent.MetadataStatus),
			"Make the metadata document reachable and valid, then run "+registerFix(agent))
	}

	switch {
	case agent.RecordURL == "":
		c.warn("D11", specAgentApp, name+": no agent record is linked to the app", registerFix(agent))
	case agent.RecordURL != agent.URL:
		c.warn("D11", specAgentApp, fmt.Sprintf("%s: the agent record has external_agent_id %q", name, agent.RecordURL), registerFix(agent))
	default:
		c.pass("D11", specAgentApp, name+": the agent record is linked with external_agent_id "+agent.RecordURL)
	}

	// DPoP is opt-in, so it is reported but never a problem.
	switch {
	case !agent.DPoPRequired.Known:
	case agent.DPoPRequired.Value:
		c.pass("D11", specAgentApp, name+": DPoP required")
	default:
		c.pass("D11", specAgentApp, name+": DPoP not required (opt-in)")
	}
}

// checkAgentKeys runs D12 on one agent.
func (r *runner) checkAgentKeys(ctx context.Context, t *Tenant, agent agents.Agent, c *collector) {
	name := agentName(agent)

	detail, err := t.Agents.Show(ctx, cmp.Or(agent.URL, agent.AgentID))
	if err != nil {
		unreadable(c, "D12", specAgentApp, "the metadata of "+name, scopeAgentKeys, err)
		return
	}

	switch check := detail.MetadataCheck; {
	case !check.Valid.Known:
	case check.Valid.Value:
		c.pass("D12", specAgentApp, name+": the metadata document is valid")
	default:
		c.fail("D12", specAgentApp, fmt.Sprintf("%s: the metadata document is invalid: %s", name, strings.Join(check.Violations, "; ")),
			"Fix the document, then run "+registerFix(agent))
	}

	switch keys := detail.KeysCurrent; {
	case !keys.Known:
	case keys.Value:
		c.pass("D12", specAgentKeys, name+": the keys Auth0 stored match the document")
	default:
		c.fail("D12", specAgentKeys, name+": the document has other keys than the ones Auth0 stored", registerFix(agent))
	}

	for _, note := range detail.Notes {
		c.warn("D12", specAgentApp, fmt.Sprintf("cannot check %s: %s", name, strings.TrimSuffix(note, ".")), "")
	}
}

// checkAgentGrants runs D13 on one agent.
func checkAgentGrants(agent agents.Agent, apis []agents.API, c *collector) {
	name := agentName(agent)

	if !agent.APIs.Known {
		c.warn("D13", "", fmt.Sprintf("cannot check %s: the grants of the agent could not be read", name), "")
		return
	}

	missing := false
	for _, api := range apis {
		// Only a grant for users lets the agent act for a person, so a grant of the
		// app itself, such as a machine-to-machine one, does not count.
		if slices.ContainsFunc(agent.APIs.Value, func(grant agents.Grant) bool {
			return grant.Audience == api.Identifier && grant.IsUserGrant()
		}) {
			continue
		}

		missing = true
		c.fail("D13", "", fmt.Sprintf("%s is not granted %s", name, api.Identifier),
			fmt.Sprintf("%s --api %s", registerFix(agent), api.Identifier))
	}

	if !missing {
		c.pass("D13", "", name+" is granted every Poppy API")
	}
}

// checkConfigFile runs D14: every protocol value of the config file must match
// the one derived from the tenant. The file names no API identifier and no web
// app, so the APIs are the Poppy APIs of the tenant that the apis entries are at,
// and the app is the only one that has the browser join on.
func (r *runner) checkConfigFile(ctx context.Context, t *Tenant, path string, file *config.Config, fileErr error, c *collector) {
	const fix = "auth0 poppy config"

	if fileErr != nil {
		c.warn("D14", "", fmt.Sprintf("cannot compare %s with the tenant: %v", path, fileErr), fix)
		return
	}

	if file.OldLayout() {
		c.warn("D14", "", fmt.Sprintf("cannot compare %s with the tenant: it uses the old layout, which was never released", path), fix)
		return
	}

	thing := "the config file " + path

	poppy, err := t.State.ListPoppyAPIs(ctx)
	if err != nil {
		unreadable(c, "D14", "", thing, setup.ReadScopes, err)
		return
	}

	identifiers, err := setup.ResolveAPIs(file, poppy)
	if err != nil {
		warnCannotCompare(c, path, err, fix)
		return
	}

	apps, err := t.State.BrowserJoinApps(ctx)
	if err != nil {
		unreadable(c, "D14", "", thing, setup.ReadScopes, err)
		return
	}

	app, err := setup.ResolveApp(apps)
	if err != nil {
		warnCannotCompare(c, path, err, fix)
		return
	}

	state, err := t.State.Read(ctx, identifiers, app)
	if err != nil {
		unreadable(c, "D14", "", thing, setup.ReadScopes, err)
		return
	}

	state.File = file

	// The issuer is compared, not enforced, so a file of another issuer shows up as a difference.
	derived, err := setup.Derive(state, setup.Inputs{ForceConfig: true})
	if err != nil {
		warnCannotCompare(c, path, err, fix)
		return
	}

	changes := config.Diff(file, derived)
	for _, change := range changes {
		// An older version is understood, so it is only stale, not a mismatch.
		if change.Path == "protocol_version" && change.Kind == config.ChangeChanged && slices.Contains(config.SupportedVersions, change.Old) {
			c.warn("D14", "", fmt.Sprintf("%s is a protocol %s file, which is stale: this CLI writes %s", path, change.Old, change.New), fix)
			continue
		}

		switch change.Kind {
		case config.ChangeChanged:
			c.warn("D14", "", fmt.Sprintf("%s is %q in %s, and the tenant has %q", change.Path, change.Old, path, change.New), fix)
		case config.ChangeAdded:
			c.warn("D14", "", fmt.Sprintf("%s is missing from %s, and the tenant has %q", change.Path, path, change.New), fix)
		case config.ChangeRemoved:
			c.warn("D14", "", fmt.Sprintf("%s is %q in %s, and the tenant has no such value", change.Path, change.Old, path), fix)
		}
	}

	if len(changes) == 0 {
		c.pass("D14", "", path+" matches the tenant")
	}
}

// warnCannotCompare reports that the file cannot be compared with the tenant. An
// error that carries its own FIX line (the API or the web app is not known) gives
// that line next to the command, since it says how to name the missing one.
func warnCannotCompare(c *collector, path string, err error, fix string) {
	var input *setup.InputError
	if errors.As(err, &input) {
		c.warn("D14", "", fmt.Sprintf("cannot compare %s with the tenant: %s", path, input.Message), fix+" ("+strings.TrimSuffix(input.Fix, ".")+")")
		return
	}

	c.warn("D14", "", fmt.Sprintf("cannot compare %s with the tenant: %v", path, err), fix)
}
