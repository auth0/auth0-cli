package doctor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/auth0/auth0-cli/internal/auth0/mock"
	"github.com/auth0/auth0-cli/internal/poppy/agents"
	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/poppy/setup"
)

const (
	shopAPI = "https://api.shop.example"
	shopApp = "W1x9aBcD2eFgH3iJ"
	museURL = "https://muse.example/agent.json"
)

var errBoom = errors.New("boom")

// fakeTenant serves a tenant to the checks. The zero value of a field is what
// the tenant answers.
type fakeTenant struct {
	settings    setup.Tenant
	settingsErr error
	hosts       []string
	hostsErr    error
	joinApps    []setup.App
	joinErr     error
	state       *setup.State
	readErr     error
	poppyAPIs   []setup.API
	poppyErr    error

	apis    []agents.API
	apisErr error
	list    []agents.Agent
	listErr error
	detail  *agents.Detail
	showErr error

	policies    managementv3.ListGuardianPoliciesResponseContent
	policiesErr error

	// Shown records the references given to Show.
	shown []string
}

func (f *fakeTenant) Settings(context.Context) (setup.Tenant, error) {
	return f.settings, f.settingsErr
}

func (f *fakeTenant) TenantHosts(context.Context) ([]string, error) { return f.hosts, f.hostsErr }

func (f *fakeTenant) BrowserJoinApps(context.Context) ([]setup.App, error) {
	return f.joinApps, f.joinErr
}

func (f *fakeTenant) Read(context.Context, []string, string) (*setup.State, error) {
	return f.state, f.readErr
}

func (f *fakeTenant) ListPoppyAPIs(context.Context) ([]setup.API, error) {
	return f.poppyAPIs, f.poppyErr
}

func (f *fakeTenant) PoppyAPIs(context.Context, []string) ([]agents.API, error) {
	return f.apis, f.apisErr
}

func (f *fakeTenant) List(context.Context, bool) ([]agents.Agent, error) { return f.list, f.listErr }

func (f *fakeTenant) Show(_ context.Context, ref string) (*agents.Detail, error) {
	f.shown = append(f.shown, ref)

	return f.detail, f.showErr
}

func known[T any](value T) agents.Maybe[T] { return agents.Maybe[T]{Value: value, Known: true} }

// healthyTenant is a tenant that passes every check, with the config file that
// matches it.
func healthyTenant() *fakeTenant {
	agent := agents.Agent{
		AgentID:        "agt_muse",
		Name:           "Meta Muse",
		URL:            museURL,
		RecordURL:      museURL,
		ClientID:       "client_muse",
		MetadataStatus: "available",
		DPoPRequired:   known(false),
		APIs:           known([]agents.Grant{{ID: "cgr_shop", Audience: shopAPI, Scopes: []string{"orders:read"}, SubjectType: "user"}}),
	}

	return &fakeTenant{
		hosts: []string{"tenant.us.auth0.com", "auth.shop.example"},
		settings: setup.Tenant{
			CIMD:                     true,
			IssParameter:             true,
			ResourceParameterProfile: "compatibility",
			DefaultAudience:          shopAPI,
			FriendlyName:             "Shop",
			CompanyDomains:           []string{"https://shop.example"},
		},
		joinApps:  []setup.App{{ClientID: shopApp, Name: "Shop Web", FirstParty: true, LoginURI: "https://shop.example/login", BrowserJoin: true}},
		poppyAPIs: []setup.API{{ID: "rs_shop", Identifier: shopAPI, Poppy: true}},
		state: &setup.State{
			Tenant:       setup.Tenant{FriendlyName: "Shop"},
			Domain:       "tenant.us.auth0.com",
			CustomDomain: "auth.shop.example",
			APIs: []setup.API{{
				ID:         "rs_shop",
				Identifier: shopAPI,
				Poppy:      true,
				Scopes: []setup.Scope{
					{Name: "checkout", Description: "Place orders and pay with your saved card. Can't be undone."},
					{Name: "orders:read", Description: "See your orders and their status. Read only."},
					{Name: "poppy:browser-sign-in", Description: "Use the website signed in, with the other access you approve"},
				},
			}},
			App: setup.App{ClientID: shopApp, Name: "Shop Web", FirstParty: true, LoginURI: "https://shop.example/login", BrowserJoin: true},
		},
		apis: []agents.API{{ID: "rs_shop", Identifier: shopAPI}},
		list: []agents.Agent{agent},
		detail: &agents.Detail{
			Agent:         agent,
			MetadataCheck: agents.MetadataCheck{Valid: known(true)},
			KeysCurrent:   known(true),
		},
	}
}

// goldenConfigFile writes the config file that matches the healthy tenant, after
// the edit, and returns its path.
func goldenConfigFile(t *testing.T, edit func(*config.Config)) string {
	t.Helper()

	cfg, err := config.Read(filepath.Join("..", "config", "testdata", "auth0-poppy.golden.json"))
	require.NoError(t, err)

	if edit != nil {
		edit(cfg)
	}

	data, err := json.Marshal(cfg)
	require.NoError(t, err)

	return writeFile(t, string(data))
}

func writeFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "auth0-poppy.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

// runTenant runs the tenant checks against the fake, with the Management API
// policies endpoint mocked.
func runTenant(t *testing.T, f *fakeTenant, configFile string) []Result {
	t.Helper()

	policies := mock.NewMockGuardianPolicyAPIV3(gomock.NewController(t))
	policies.EXPECT().List(gomock.Any()).Return(f.policies, f.policiesErr)

	r := &runner{domain: testDomain, origin: "https://" + testDomain}

	return r.checkTenant(t.Context(), Options{
		Tenant:     &Tenant{State: f, Agents: f, Policies: policies},
		ConfigFile: configFile,
	}, "")
}

func forbidden(body string) error {
	return core.NewAPIError(http.StatusForbidden, nil, errors.New(body))
}

func TestTenantChecksHealthy(t *testing.T) {
	results := runTenant(t, healthyTenant(), goldenConfigFile(t, nil))

	assert.Empty(t, nonPass(Report{Results: results}))

	var ids []string
	for _, res := range results {
		if len(ids) == 0 || ids[len(ids)-1] != res.ID {
			ids = append(ids, res.ID)
		}
	}
	assert.Equal(t, []string{"D8", "D9", "D10", "D11", "D12", "D13", "D14"}, ids, "results come in check order, once per check")

	messages := make([]string, len(results))
	for i, res := range results {
		messages[i] = res.Message
	}
	assert.Contains(t, messages, museURL+": DPoP not required (opt-in)", "DPoP is only reported")
}

func TestTenantChecksMutations(t *testing.T) {
	scopeBody := `{"errorCode":"insufficient_scope","message":"Insufficient scope, expected any of: read:mfa_policies"}`
	noPoppyAPI := fmt.Errorf("%w. Run: auth0 poppy setup", agents.ErrNoPoppyAPI)

	tests := []struct {
		name   string
		mutate func(*fakeTenant)
		// File is the config file the check reads. Nil means the matching file.
		file func(*testing.T) string
		want []want
		// Absent are messages that no result may contain.
		absent []string
	}{
		{
			name:   "client ID metadata documents are off",
			mutate: func(f *fakeTenant) { f.settings.CIMD = false },
			want:   []want{{id: "D8", level: Fail, spec: "4", contains: "client_id_metadata_document_supported is off"}},
		},
		{
			name:   "the iss parameter is off",
			mutate: func(f *fakeTenant) { f.settings.IssParameter = false },
			want:   []want{{id: "D8", level: Fail, spec: "4", contains: "authorization_response_iss_parameter_supported is off"}},
		},
		{
			name:   "the resource parameter profile is not compatibility",
			mutate: func(f *fakeTenant) { f.settings.ResourceParameterProfile = "audience" },
			want:   []want{{id: "D8", level: Fail, spec: "4", contains: `resource_parameter_profile is "audience"`}},
		},
		{
			name:   "no default audience with one Poppy API",
			mutate: func(f *fakeTenant) { f.settings.DefaultAudience = "" },
			want:   []want{{id: "D8", level: Fail, spec: "4", contains: "default_audience is not set"}},
		},
		{
			name: "no default audience is fine with several Poppy APIs",
			mutate: func(f *fakeTenant) {
				f.settings.DefaultAudience = ""
				f.apis = append(f.apis, agents.API{ID: "rs_other", Identifier: "https://api.other.example"})
				f.list[0].APIs = known(append(f.list[0].APIs.Value, agents.Grant{Audience: "https://api.other.example", SubjectType: "user"}))
			},
			absent: []string{"default_audience is"},
		},
		{
			name:   "no default audience is fine without a Poppy API",
			mutate: func(f *fakeTenant) { f.settings.DefaultAudience, f.apis, f.apisErr = "", nil, noPoppyAPI },
			absent: []string{"default_audience is"},
			want: []want{
				{id: "D11", level: Skipped, spec: "4.1", contains: "no Poppy API"},
				{id: "D12", level: Skipped, spec: "4.1", contains: "no Poppy API"},
				{id: "D13", level: Skipped, contains: "no Poppy API"},
			},
		},
		{
			name:   "the company domain is not listed",
			mutate: func(f *fakeTenant) { f.settings.CompanyDomains = []string{"https://other.example"} },
			want:   []want{{id: "D8", level: Fail, spec: "3.2", contains: "does not list https://shop.example"}},
		},
		{
			name:   "the company domain is listed in another case",
			mutate: func(f *fakeTenant) { f.settings.CompanyDomains = []string{"https://SHOP.example"} },
			want:   []want{{id: "D8", level: Pass, spec: "3.2", contains: "lists https://shop.example"}},
		},
		{
			name: "a company domain entry that is not an origin",
			mutate: func(f *fakeTenant) {
				f.settings.CompanyDomains = []string{"https://shop.example", "https://shop.example/store/"}
			},
			want: []want{{id: "D8", level: Warn, spec: "3.2", contains: `"https://shop.example/store/", which is not an https://host origin`}},
		},
		{
			name:   "the web app is not first-party",
			mutate: func(f *fakeTenant) { f.joinApps[0].FirstParty = false },
			want:   []want{{id: "D9", level: Fail, spec: "design 5.5", contains: "Shop Web is not first-party"}},
		},
		{
			name:   "the web app has an http login route",
			mutate: func(f *fakeTenant) { f.joinApps[0].LoginURI = "http://shop.example/login" },
			want:   []want{{id: "D9", level: Fail, spec: "design 5.5", contains: "Shop Web has no https initiate_login_uri"}},
		},
		{
			name:   "the web app has no login route",
			mutate: func(f *fakeTenant) { f.joinApps[0].LoginURI = "" },
			want:   []want{{id: "D9", level: Fail, spec: "design 5.5", contains: "Shop Web has no https initiate_login_uri"}},
		},
		{
			name:   "the browser join is off",
			mutate: func(f *fakeTenant) { f.joinApps[0].BrowserJoin = false },
			want:   []want{{id: "D9", level: Fail, spec: "design 5.5", contains: "allow_personal_agent_browser_join off"}},
		},
		{
			name:   "no config file and no app with the browser join",
			mutate: func(f *fakeTenant) { f.joinApps = nil },
			file:   func(t *testing.T) string { return filepath.Join(t.TempDir(), "missing.json") },
			want:   []want{{id: "D9", level: Skipped, spec: "design 5.5", contains: "no app has allow_personal_agent_browser_join on"}},
		},
		{
			name: "several apps with the browser join are each checked",
			mutate: func(f *fakeTenant) {
				f.joinApps = []setup.App{
					{ClientID: "app_a", Name: "App A", FirstParty: true, LoginURI: "https://a.example/login", BrowserJoin: true},
					{ClientID: "app_b", Name: "App B", FirstParty: false, LoginURI: "https://b.example/login", BrowserJoin: true},
				}
			},
			file: func(t *testing.T) string { return filepath.Join(t.TempDir(), "missing.json") },
			want: []want{
				{id: "D9", level: Pass, spec: "design 5.5", contains: "App A is first-party"},
				{id: "D9", level: Pass, spec: "design 5.5", contains: "App B has an https initiate_login_uri"},
				{id: "D9", level: Fail, spec: "design 5.5", contains: "App B is not first-party"},
			},
		},
		{
			name: "MFA is required for all logins",
			mutate: func(f *fakeTenant) {
				f.policies = managementv3.ListGuardianPoliciesResponseContent{managementv3.MfaPolicyEnumAllApplications}
			},
			want: []want{{id: "D10", level: Warn, contains: "browser joins cannot show MFA prompts, and post-login Actions can also ask for MFA"}},
		},
		{
			name: "the confidence score policy does not block browser joins",
			mutate: func(f *fakeTenant) {
				f.policies = managementv3.ListGuardianPoliciesResponseContent{managementv3.MfaPolicyEnumConfidenceScore}
			},
			want: []want{{id: "D10", level: Pass, contains: "MFA is not required for all logins"}},
		},
		{
			name:   "the policies need a scope",
			mutate: func(f *fakeTenant) { f.policiesErr = forbidden(scopeBody) },
			want:   []want{{id: "D10", level: Skipped, contains: "the login lacks read:mfa_policies"}},
		},
		{
			name: "the agent metadata is unavailable",
			mutate: func(f *fakeTenant) {
				f.list[0].MetadataStatus = "unavailable"
			},
			want: []want{{id: "D11", level: Fail, spec: "4.1", contains: museURL + `: metadata status is "unavailable"`}},
		},
		{
			name:   "the agent metadata status could not be read",
			mutate: func(f *fakeTenant) { f.list[0].MetadataStatus = "unknown" },
			want:   []want{{id: "D11", level: Warn, spec: "4.1", contains: "the metadata status could not be read"}},
		},
		{
			name:   "the agent has no record",
			mutate: func(f *fakeTenant) { f.list[0].AgentID, f.list[0].RecordURL = "", "" },
			want:   []want{{id: "D11", level: Warn, spec: "4.1", contains: "no agent record is linked to the app"}},
		},
		{
			name:   "the agent record names another URL",
			mutate: func(f *fakeTenant) { f.list[0].RecordURL = "https://other.example/agent.json" },
			want:   []want{{id: "D11", level: Warn, spec: "4.1", contains: `external_agent_id "https://other.example/agent.json"`}},
		},
		{
			name:   "DPoP is required",
			mutate: func(f *fakeTenant) { f.list[0].DPoPRequired = known(true) },
			want:   []want{{id: "D11", level: Pass, spec: "4.1", contains: museURL + ": DPoP required"}},
		},
		{
			name:   "DPoP is unknown",
			mutate: func(f *fakeTenant) { f.list[0].DPoPRequired = agents.Maybe[bool]{} },
			absent: []string{"DPoP required", "DPoP not"},
		},
		{
			name: "the metadata document is invalid",
			mutate: func(f *fakeTenant) {
				f.detail.MetadataCheck = agents.MetadataCheck{Valid: known(false), Violations: []string{"jwks is missing", "name is empty"}}
				f.detail.KeysCurrent = agents.Maybe[bool]{}
			},
			want: []want{{id: "D12", level: Fail, spec: "4.1", contains: "the metadata document is invalid: jwks is missing; name is empty"}},
		},
		{
			name:   "the keys changed",
			mutate: func(f *fakeTenant) { f.detail.KeysCurrent = known(false) },
			want:   []want{{id: "D12", level: Fail, spec: "5.2", contains: "other keys than the ones Auth0 stored"}},
		},
		{
			name: "the preview failed",
			mutate: func(f *fakeTenant) {
				f.detail.MetadataCheck = agents.MetadataCheck{}
				f.detail.KeysCurrent = agents.Maybe[bool]{}
				f.detail.Notes = []string{"The metadata document could not be checked: boom."}
			},
			want: []want{{id: "D12", level: Warn, spec: "4.1", contains: "cannot check " + museURL + ": The metadata document could not be checked: boom"}},
		},
		{
			name:   "the agent cannot be shown",
			mutate: func(f *fakeTenant) { f.showErr = errBoom },
			want:   []want{{id: "D12", level: Warn, spec: "4.1", contains: "cannot check the metadata of " + museURL + ": boom"}},
		},
		{
			name:   "the keys need a scope",
			mutate: func(f *fakeTenant) { f.showErr = forbidden(scopeBody) },
			want:   []want{{id: "D12", level: Skipped, spec: "4.1", contains: "the login lacks read:clients, read:client_credentials"}},
		},
		{
			name: "the agent lacks a grant",
			mutate: func(f *fakeTenant) {
				f.apis = append(f.apis, agents.API{ID: "rs_other", Identifier: "https://api.other.example"})
				f.settings.DefaultAudience = ""
			},
			want: []want{{id: "D13", level: Fail, contains: museURL + " is not granted https://api.other.example"}},
		},
		{
			name: "a grant of another subject type does not count",
			mutate: func(f *fakeTenant) {
				f.list[0].APIs = known([]agents.Grant{{ID: "cgr_m2m", Audience: shopAPI, Scopes: []string{"orders:read"}, SubjectType: "client"}})
			},
			want: []want{{id: "D13", level: Fail, contains: museURL + " is not granted " + shopAPI}},
		},
		{
			name:   "the grants of the agent could not be read",
			mutate: func(f *fakeTenant) { f.list[0].APIs = agents.Maybe[[]agents.Grant]{} },
			want:   []want{{id: "D13", level: Warn, contains: "the grants of the agent could not be read"}},
		},
		{
			name:   "no agent has a Poppy grant",
			mutate: func(f *fakeTenant) { f.list = nil },
			want: []want{
				{id: "D11", level: Skipped, spec: "4.1", contains: "no agent has a grant"},
				{id: "D12", level: Skipped, spec: "4.1", contains: "no agent has a grant"},
				{id: "D13", level: Skipped, contains: "no agent has a grant"},
			},
		},
		{
			name:   "the agents need a scope",
			mutate: func(f *fakeTenant) { f.listErr = forbidden(scopeBody) },
			want: []want{
				{id: "D11", level: Skipped, spec: "4.1", contains: "the login lacks read:agents, read:clients, read:client_grants, read:resource_servers"},
				{id: "D12", level: Skipped, spec: "4.1", contains: "cannot check the agents"},
				{id: "D13", level: Skipped, contains: "cannot check the agents"},
			},
		},
		{
			name:   "the agents are not available to the tenant",
			mutate: func(f *fakeTenant) { f.listErr = forbidden(`{"errorCode":"forbidden","message":"not allowed"}`) },
			want: []want{
				{id: "D11", level: Warn, spec: "4.1", contains: "the tenant may lack Agent as Principal or the Poppy beta"},
				{id: "D12", level: Warn, spec: "4.1", contains: "the tenant may lack Agent as Principal or the Poppy beta"},
				{id: "D13", level: Warn, contains: "the tenant may lack Agent as Principal or the Poppy beta"},
			},
		},
		{
			name:   "the Poppy APIs cannot be listed",
			mutate: func(f *fakeTenant) { f.apisErr = errBoom },
			want: []want{
				{id: "D11", level: Warn, spec: "4.1", contains: "cannot check the agents: boom"},
				{id: "D12", level: Warn, spec: "4.1", contains: "cannot check the agents: boom"},
				{id: "D13", level: Warn, contains: "cannot check the agents: boom"},
			},
		},
		{
			name:   "the tenant settings need a scope",
			mutate: func(f *fakeTenant) { f.settingsErr = forbidden(scopeBody) },
			want:   []want{{id: "D8", level: Skipped, spec: "3.2", contains: "cannot check the tenant settings: the login lacks read:tenant_settings"}},
		},
		{
			name:   "the tenant settings are not available",
			mutate: func(f *fakeTenant) { f.settingsErr = forbidden("not json") },
			want:   []want{{id: "D8", level: Warn, spec: "3.2", contains: "cannot check the tenant settings: the tenant may lack"}},
		},
		{
			name:   "the tenant settings fail",
			mutate: func(f *fakeTenant) { f.settingsErr = errBoom },
			want:   []want{{id: "D8", level: Warn, spec: "3.2", contains: "cannot check the tenant settings: boom"}},
		},
		{
			name:   "the web apps cannot be listed",
			mutate: func(f *fakeTenant) { f.joinErr = errBoom },
			file:   func(t *testing.T) string { return filepath.Join(t.TempDir(), "missing.json") },
			want:   []want{{id: "D9", level: Warn, spec: "design 5.5", contains: "cannot check the web apps: boom"}},
		},
		{
			name:   "the config file lists no API, so the tenant APIs cannot be linked to it",
			mutate: func(*fakeTenant) {},
			file: func(t *testing.T) string {
				return writeFile(t, `{"protocol_version":"0.3","organization":{"name":"Shop","domain":"shop.example"}}`)
			},
			want: []want{
				{id: "D9", level: Pass, spec: "design 5.5", contains: "Shop Web is first-party"},
				{id: "D14", level: Warn, contains: "No API of the tenant is known"},
			},
		},
		{
			name:   "an apis entry no Poppy API of the tenant is at is skipped",
			mutate: func(*fakeTenant) {},
			file: func(t *testing.T) string {
				return goldenConfigFile(t, func(c *config.Config) {
					c.APIs = append(c.APIs, config.API{Type: "mcp", URL: "https://mcp.vendor.example/mcp", Description: "Vendor"})
				})
			},
			want: []want{{id: "D14", level: Pass, contains: "matches the tenant"}},
		},
		{
			name:   "the scopes of an API the file describes but the run does not resolve are not a mismatch",
			mutate: func(*fakeTenant) {},
			file: func(t *testing.T) string {
				return goldenConfigFile(t, func(c *config.Config) {
					c.Scopes["users:write"] = config.Scope{Description: "Change users.", Hints: &config.Hints{Changes: "reversible", PersonalData: true}}
				})
			},
			want: []want{{id: "D14", level: Pass, contains: "matches the tenant"}},
		},
		{
			name:   "one API published in two forms at one origin is compared without flags",
			mutate: func(*fakeTenant) {},
			file: func(t *testing.T) string {
				return goldenConfigFile(t, func(c *config.Config) {
					c.APIs = []config.API{
						{Type: "mcp", URL: shopAPI + "/mcp", Description: "Search"},
						{Type: "openapi", URL: shopAPI + "/openapi.json", Description: "Orders"},
					}
				})
			},
			want: []want{{id: "D14", level: Pass, contains: "matches the tenant"}},
		},
		{
			name: "two Poppy APIs of the tenant are at the origin of one entry",
			mutate: func(f *fakeTenant) {
				f.poppyAPIs = append(f.poppyAPIs, setup.API{Identifier: shopAPI + "/v2", Poppy: true})
			},
			file: func(t *testing.T) string {
				return goldenConfigFile(t, func(c *config.Config) { c.APIs[0].URL = shopAPI + "/openapi.json" })
			},
			want: []want{{id: "D14", level: Warn, contains: "could be any of these APIs"}},
		},
		{
			name:   "no Poppy API of the tenant is at the origin of an entry",
			mutate: func(f *fakeTenant) { f.poppyAPIs = nil },
			want:   []want{{id: "D14", level: Warn, contains: "No API of the tenant is known"}},
		},
		{
			name: "several apps have the browser join on, so the app cannot be told",
			mutate: func(f *fakeTenant) {
				f.joinApps = append(f.joinApps, setup.App{ClientID: "app_b", Name: "App B", FirstParty: true, LoginURI: "https://b.example/login", BrowserJoin: true})
			},
			want: []want{
				{id: "D9", level: Pass, spec: "design 5.5", contains: "Shop Web is first-party"},
				{id: "D9", level: Pass, spec: "design 5.5", contains: "App B is first-party"},
				{id: "D14", level: Warn, contains: "Several apps of the tenant have allow_personal_agent_browser_join on"},
			},
		},
		{
			name:   "no app has the browser join on",
			mutate: func(f *fakeTenant) { f.joinApps = nil },
			want: []want{
				{id: "D9", level: Skipped, spec: "design 5.5", contains: "no app has allow_personal_agent_browser_join on"},
				{id: "D14", level: Warn, contains: "No app of the tenant has allow_personal_agent_browser_join on"},
			},
		},
		{
			name:   "the config file is a protocol 0.3 file",
			mutate: func(*fakeTenant) {},
			file: func(t *testing.T) string {
				return goldenConfigFile(t, func(c *config.Config) { c.ProtocolVersion = "0.3" })
			},
			want: []want{{id: "D14", level: Warn, contains: "is a protocol 0.3 file, which is stale: this CLI writes 0.4"}},
		},
		{
			name:   "the config file has another organization name",
			mutate: func(*fakeTenant) {},
			file: func(t *testing.T) string {
				return goldenConfigFile(t, func(c *config.Config) { c.Organization.Name = "Old Shop" })
			},
			want: []want{{id: "D14", level: Warn, contains: `organization.name is "Old Shop" in `}},
		},
		{
			name:   "the config file belongs to another issuer",
			mutate: func(*fakeTenant) {},
			file: func(t *testing.T) string {
				return goldenConfigFile(t, func(c *config.Config) { c.Issuer = "https://old.example/" })
			},
			want: []want{{id: "D14", level: Warn, contains: `auth.issuer is "https://old.example/" in `}},
		},
		{
			name:   "the config file keeps values the tenant does not store",
			mutate: func(*fakeTenant) {},
			file: func(t *testing.T) string {
				return goldenConfigFile(t, func(c *config.Config) {
					c.APIs[0].URL = "https://api.shop.example/v2/openapi.json"
					c.Scopes["checkout"] = config.Scope{Description: c.Scopes["checkout"].Description, Hints: &config.Hints{Changes: "none"}}
				})
			},
			want: []want{{id: "D14", level: Pass, contains: "matches the tenant"}},
		},
		{
			name:   "the config file is in the old layout",
			mutate: func(*fakeTenant) {},
			file: func(t *testing.T) string {
				return writeFile(t, `{"protocol_version":"0.4","organization":{"name":"Shop","domain":"shop.example"},"issuer":"https://auth.shop.example/","scopes":{},"apis":[],"web":{"client_id":"`+shopApp+`"},"agents":[]}`)
			},
			want: []want{
				{id: "D9", level: Pass, spec: "design 5.5", contains: "Shop Web is first-party"},
				{id: "D14", level: Warn, contains: "it uses the old layout"},
			},
		},
		{
			name:   "the config file has no hints for the browser sign-in scope",
			mutate: func(*fakeTenant) {},
			file: func(t *testing.T) string {
				return goldenConfigFile(t, func(c *config.Config) {
					c.Scopes["poppy:browser-sign-in"] = config.Scope{Description: c.Scopes["poppy:browser-sign-in"].Description}
				})
			},
			want: []want{
				{id: "D14", level: Warn, contains: "auth.scope_hints.poppy:browser-sign-in.changes is missing from "},
				{id: "D14", level: Warn, contains: "auth.scope_hints.poppy:browser-sign-in.personal_data is missing from "},
			},
		},
		{
			name:   "the config file cannot be parsed",
			mutate: func(*fakeTenant) {},
			file:   func(t *testing.T) string { return writeFile(t, "{") },
			want: []want{
				{id: "D9", level: Pass, spec: "design 5.5", contains: "Shop Web is first-party"},
				{id: "D14", level: Warn, contains: "cannot compare "},
			},
		},
		{
			name:   "the config file cannot be compared because the tenant cannot be read",
			mutate: func(f *fakeTenant) { f.readErr = forbidden(scopeBody) },
			want:   []want{{id: "D14", level: Skipped, contains: "cannot check the config file "}},
		},
		{
			name:   "the config file cannot be compared because the Poppy APIs cannot be listed",
			mutate: func(f *fakeTenant) { f.poppyErr = forbidden(scopeBody) },
			want:   []want{{id: "D14", level: Skipped, contains: "cannot check the config file "}},
		},
		{
			name: "the config file cannot be derived",
			mutate: func(f *fakeTenant) {
				f.state.APIs[0].Scopes = append(f.state.APIs[0].Scopes, setup.Scope{Name: "refund"})
			},
			want: []want{{id: "D14", level: Warn, contains: "refund (" + shopAPI + ")"}},
		},
		{
			name:   "no config file gives no D14 result",
			mutate: func(*fakeTenant) {},
			file:   func(t *testing.T) string { return filepath.Join(t.TempDir(), "missing.json") },
			absent: []string{"matches the tenant", "cannot compare"},
			want:   []want{{id: "D9", level: Pass, spec: "design 5.5", contains: "Shop Web is first-party"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := healthyTenant()
			test.mutate(f)
			if test.file == nil {
				test.file = func(t *testing.T) string { return goldenConfigFile(t, nil) }
			}

			results := runTenant(t, f, test.file(t))

			for _, w := range test.want {
				i := slices.IndexFunc(results, func(res Result) bool {
					return res.ID == w.id && res.Level == w.level && res.Spec == w.spec && strings.Contains(res.Message, w.contains)
				})
				if !assert.GreaterOrEqual(t, i, 0, "missing %+v in %+v", w, results) {
					continue
				}

				if res := results[i]; res.Level == Fail || (res.Level == Skipped && strings.Contains(res.Message, "lacks")) {
					assert.NotEmpty(t, res.Fix, res.Message)
				}
			}

			wantedNonPass := 0
			for _, w := range test.want {
				if w.level != Pass {
					wantedNonPass++
				}
			}
			assert.Len(t, nonPass(Report{Results: results}), wantedNonPass, "unexpected extra results: %+v", nonPass(Report{Results: results}))

			for _, text := range test.absent {
				assert.False(t, slices.ContainsFunc(results, func(res Result) bool { return strings.Contains(res.Message, text) }), "%q in %+v", text, results)
			}
		})
	}
}

func TestTenantChecksWithoutLogin(t *testing.T) {
	tests := []struct {
		name string
		file func(*testing.T) string
		want []string
	}{
		{
			name: "no config file",
			file: func(t *testing.T) string { return filepath.Join(t.TempDir(), "missing.json") },
			want: []string{"D8", "D9", "D10", "D11", "D12", "D13"},
		},
		{
			name: "a config file",
			file: func(t *testing.T) string { return goldenConfigFile(t, nil) },
			want: []string{"D8", "D9", "D10", "D11", "D12", "D13", "D14"},
		},
		{
			name: "a config file that cannot be read",
			file: func(t *testing.T) string { return writeFile(t, "{") },
			want: []string{"D8", "D9", "D10", "D11", "D12", "D13", "D14"},
		},
		{
			name: "no config file name",
			file: func(*testing.T) string { return "" },
			want: []string{"D8", "D9", "D10", "D11", "D12", "D13"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := &runner{domain: testDomain, origin: "https://" + testDomain}

			results := r.checkTenant(t.Context(), Options{ConfigFile: test.file(t)}, "")

			var ids []string
			for _, res := range results {
				ids = append(ids, res.ID)
				assert.Equal(t, Skipped, res.Level)
				assert.Equal(t, noLogin, res.Message)
			}
			assert.Equal(t, test.want, ids)
			assert.Zero(t, Report{Results: results}.ExitCode(), "a skipped check never fails")
		})
	}
}

func TestRunTenantAfterDiscovery(t *testing.T) {
	policies := mock.NewMockGuardianPolicyAPIV3(gomock.NewController(t))
	policies.EXPECT().List(gomock.Any()).Return(managementv3.ListGuardianPoliciesResponseContent{}, nil)

	f := healthyTenant()
	opts := Options{Tenant: &Tenant{State: f, Agents: f, Policies: policies}, ConfigFile: goldenConfigFile(t, nil)}

	report := validSite().runWith(t, opts)

	var ids []string
	for _, res := range report.Results {
		assert.Equal(t, Pass, res.Level, res.Message)
		if len(ids) == 0 || ids[len(ids)-1] != res.ID {
			ids = append(ids, res.ID)
		}
	}
	assert.Equal(t, []string{"D1", "D2", "D3", "D4", "D5", "D6", "D7", "D15", "D16", "D8", "D9", "D10", "D11", "D12", "D13", "D14"}, ids)
	assert.Equal(t, []string{museURL}, f.shown, "the agent is shown by its URL")
}

func TestRunTenantChecksRunWhenDiscoveryFails(t *testing.T) {
	policies := mock.NewMockGuardianPolicyAPIV3(gomock.NewController(t))
	policies.EXPECT().List(gomock.Any()).Return(managementv3.ListGuardianPoliciesResponseContent{}, nil)

	s := validSite()
	s.routes[testDomain+"/.well-known/poppy.json"] = func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }

	f := healthyTenant()
	report := s.runWith(t, Options{Tenant: &Tenant{State: f, Agents: f, Policies: policies}})

	assert.Equal(t, Fail, report.Results[0].Level)
	assert.Equal(t, 1, report.ExitCode())
	assert.True(t, slices.ContainsFunc(report.Results, func(res Result) bool { return res.ID == "D8" && res.Level == Pass }), "D8 ran")
}

func TestTenantChecksBelongToTheCheckedDomain(t *testing.T) {
	const own = "https://auth.shop.example/"

	tests := []struct {
		name    string
		issuer  string
		mutate  func(*fakeTenant)
		wantRun bool
	}{
		{
			name:   "another company's domain",
			issuer: "https://auth.other.example/",
			mutate: func(f *fakeTenant) { f.settings.CompanyDomains = []string{"https://other.example"} },
		},
		{
			name:    "the issuer is a custom domain of the tenant",
			issuer:  own,
			wantRun: true,
		},
		{
			name:    "the issuer is the canonical domain of the tenant",
			issuer:  "https://tenant.us.auth0.com/",
			wantRun: true,
		},
		{
			name:   "the issuer host is compared without case",
			issuer: "https://AUTH.Shop.Example/",
			mutate: func(f *fakeTenant) {
				f.hosts = []string{"auth.shop.example"}
			},
			wantRun: true,
		},
		{
			name:   "the tenant already lists the domain",
			issuer: "https://auth.elsewhere.example/",
			mutate: func(f *fakeTenant) {
				f.hosts = []string{"tenant.us.auth0.com"}
			},
			wantRun: true,
		},
		{
			name:    "the issuer is not known",
			issuer:  "",
			mutate:  func(f *fakeTenant) { f.settings.CompanyDomains = nil },
			wantRun: true,
		},
		{
			name:   "the domains of the tenant cannot be read",
			issuer: "https://auth.other.example/",
			mutate: func(f *fakeTenant) {
				f.hostsErr = errBoom
				f.settings.CompanyDomains = nil
			},
			wantRun: true,
		},
		{
			name:   "the settings cannot be read",
			issuer: "https://auth.other.example/",
			mutate: func(f *fakeTenant) {
				f.hosts = []string{"tenant.us.auth0.com"}
				f.settingsErr = errBoom
			},
			wantRun: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := healthyTenant()
			if test.mutate != nil {
				test.mutate(f)
			}

			policies := mock.NewMockGuardianPolicyAPIV3(gomock.NewController(t))
			if test.wantRun {
				policies.EXPECT().List(gomock.Any()).Return(f.policies, f.policiesErr)
			}

			r := &runner{domain: testDomain, origin: "https://" + testDomain}
			results := r.checkTenant(t.Context(), Options{Tenant: &Tenant{State: f, Agents: f, Policies: policies}}, test.issuer)

			if test.wantRun {
				assert.Greater(t, len(results), 1)
				assert.NotEqual(t, Skipped, results[0].Level)
				return
			}

			require.Len(t, results, 1)
			assert.Equal(t, Skipped, results[0].Level)
			assert.Equal(t, "you are logged in to a different tenant than the one that serves shop.example. Log in to that tenant to run these checks.", results[0].Message)
			assert.Zero(t, Report{Results: results}.ExitCode())
		})
	}
}

func TestRunOtherCompanysDomainIsNotFailedByTheTenant(t *testing.T) {
	f := healthyTenant()
	f.hosts = []string{"tenant.us.auth0.com", "login.other.example"}
	f.settings.CompanyDomains = []string{"https://other.example"}

	// The mock has no expectation, so any tenant check that reaches it fails the test.
	policies := mock.NewMockGuardianPolicyAPIV3(gomock.NewController(t))

	report := validSite().runWith(t, Options{Tenant: &Tenant{State: f, Agents: f, Policies: policies}, ConfigFile: goldenConfigFile(t, nil)})

	assert.Zero(t, report.ExitCode(), "%+v", nonPass(report))
	require.NotEmpty(t, report.Results)

	last := report.Results[len(report.Results)-1]
	assert.Equal(t, Skipped, last.Level)
	assert.Contains(t, last.Message, "different tenant than the one that serves shop.example")
	assert.Empty(t, f.shown)

	for _, res := range report.Results[:len(report.Results)-1] {
		assert.Equal(t, Pass, res.Level, res.Message)
		assert.Contains(t, []string{"D1", "D2", "D3", "D4", "D5", "D6", "D7", "D15", "D16"}, res.ID)
	}
}

func TestRunWithoutLoginKeepsDiscoveryResults(t *testing.T) {
	s := validSite()

	withLogin := s.run(t)
	full := s.runWith(t, Options{})

	assert.Equal(t, withLogin.Results, full.Results[:len(withLogin.Results)], "D1 to D7 are unchanged")

	tail := full.Results[len(withLogin.Results):]
	require.Len(t, tail, 6)
	for _, res := range tail {
		assert.Equal(t, Skipped, res.Level)
		assert.Equal(t, noLogin, res.Message)
	}
	assert.Zero(t, full.ExitCode())
}

func TestIsOrigin(t *testing.T) {
	tests := map[string]bool{
		"https://shop.example":       true,
		"https://shop.example/":      false,
		"https://shop.example/store": false,
		"https://shop.example:8443":  false,
		"https://user@shop.example":  false,
		"https://shop.example?x=1":   false,
		"https://shop.example#x":     false,
		"http://shop.example":        false,
		"shop.example":               false,
		"https://":                   false,
		"://":                        false,
	}

	for value, want := range tests {
		t.Run(value, func(t *testing.T) {
			assert.Equal(t, want, isOrigin(value))
		})
	}
}
