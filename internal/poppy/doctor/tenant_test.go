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

	coreagents "github.com/auth0/auth0-cli/internal/agents"
	poppyagents "github.com/auth0/auth0-cli/internal/agents/poppy"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/poppy/setup"
)

const (
	// TenantIssuer is an issuer on the canonical domain of the healthy tenant.
	tenantIssuer = "https://tenant.us.auth0.com/"

	shopAPI  = "https://api.shop.example"
	shopApp  = "W1x9aBcD2eFgH3iJ"
	museURL  = "https://muse.example/agent.json"
	museName = "Meta Muse"
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
	allAPIs     []setup.API
	allAPIsErr  error
	granted     []string
	grantedErr  error

	apis    []coreagents.API
	apisErr error
	list    []coreagents.GrantedAgent
	listErr error
	detail  *coreagents.GrantedDetail
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

func (f *fakeTenant) ListAPIs(context.Context) ([]setup.API, error) {
	return f.allAPIs, f.allAPIsErr
}

func (f *fakeTenant) GrantedScopes(context.Context, string, string) ([]string, error) {
	return f.granted, f.grantedErr
}

func (f *fakeTenant) PoppyAPIs(context.Context, []string) ([]coreagents.API, error) {
	return f.apis, f.apisErr
}

func (f *fakeTenant) List(context.Context, bool) ([]coreagents.GrantedAgent, error) {
	return f.list, f.listErr
}

func (f *fakeTenant) Show(_ context.Context, ref string) (*coreagents.GrantedDetail, error) {
	f.shown = append(f.shown, ref)

	return f.detail, f.showErr
}

func known[T any](value T) coreagents.Maybe[T] { return coreagents.Known(value) }

// healthyTenant is a tenant that passes every check, with the config file that
// matches it.
func healthyTenant() *fakeTenant {
	agent := coreagents.GrantedAgent{
		Agent: coreagents.Agent{
			AgentID:        "agt_muse",
			Name:           museName,
			URL:            museURL,
			RecordURL:      museURL,
			ClientID:       "client_muse",
			MetadataStatus: "available",
			DPoPRequired:   known(false),
		},
		APIs: known([]coreagents.Grant{{ID: "cgr_shop", Audience: shopAPI, Scopes: []string{"orders:read"}, SubjectType: "user"}}),
	}

	return &fakeTenant{
		hosts: []string{"tenant.us.auth0.com", "auth.shop.example"},
		settings: setup.Tenant{
			CIMD:                     true,
			IssParameter:             true,
			ResourceParameterProfile: "compatibility",
			DefaultAudience:          shopAPI,
			FriendlyName:             "Shop",
			CompanyDomains:           []string{"shop.example"},
		},
		joinApps:  []setup.App{{ClientID: shopApp, Name: "Shop Web", FirstParty: true, LoginURI: "https://shop.example/login", BrowserJoin: true}},
		poppyAPIs: []setup.API{{ID: "rs_shop", Identifier: shopAPI, Poppy: true}},
		allAPIs: []setup.API{{
			ID: "rs_shop", Identifier: shopAPI, Poppy: true, AgentSubjectClaims: "auth0-v1", GuestPolicy: "require_client_grant",
		}},
		granted: []string{"orders:read"},
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
		apis: []coreagents.API{{ID: "rs_shop", Identifier: shopAPI}},
		list: []coreagents.GrantedAgent{agent},
		detail: &coreagents.GrantedDetail{
			GrantedAgent:  agent,
			MetadataCheck: coreagents.MetadataCheck{Valid: known(true), UsableKeys: 1},
			KeysCurrent:   known(true),
		},
	}
}

// goldenConfigFile writes the config file that matches the healthy tenant, after
// the edit, and returns its path.
func goldenConfigFile(t *testing.T, edit func(*config.Config)) string {
	t.Helper()

	cfg, err := config.Read(filepath.Join("..", "config", "testdata", "poppy.golden.json"))
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

	path := filepath.Join(t.TempDir(), "poppy.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

// runTenant runs the tenant checks against the fake, with the Management API
// policies endpoint mocked.
func runTenant(t *testing.T, f *fakeTenant, configFile string) []Result {
	t.Helper()

	policies := mock.NewMockGuardianPolicyAPIV3(gomock.NewController(t))
	policies.EXPECT().List(gomock.Any()).Return(f.policies, f.policiesErr)

	r := &runner{domain: testDomain, origin: "https://" + testDomain, configFile: configFile}
	file, fileErr := readConfigFile(configFile)

	results, _ := r.checkTenant(t.Context(), Options{
		Tenant:     &Tenant{State: f, Agents: f, Policies: policies},
		ConfigFile: configFile,
	}, tenantIssuer, file, fileErr)

	return results
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
	assert.Contains(t, messages, museName+": DPoP not required (opt-in)", "DPoP is only reported")
}

func TestTenantChecksNameTheAgent(t *testing.T) {
	results := runTenant(t, healthyTenant(), goldenConfigFile(t, nil))

	var about []Result
	for _, res := range results {
		if res.Agent != nil {
			about = append(about, res)
		}
	}

	require.NotEmpty(t, about)

	for _, res := range about {
		assert.Equal(t, &AgentRef{ID: "agt_muse", URL: museURL, Name: museName}, res.Agent, res.Code)
		assert.True(t, strings.HasPrefix(res.Message, museName), "the message names the agent, the URL is in the agent field: %s", res.Message)
		assert.Contains(t, []string{"D11", "D12", "D13"}, res.ID)
	}

	raw, err := json.Marshal(about[0])
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"agent":{"id":"agt_muse","url":"`+museURL+`","name":"`+museName+`"}`)
}

func TestTenantChecksMutations(t *testing.T) {
	scopeBody := `{"errorCode":"insufficient_scope","message":"Insufficient scope, expected any of: read:mfa_policies"}`
	noPoppyAPI := fmt.Errorf("%w. Run: auth0 poppy setup", poppyagents.ErrNoPoppyAPI)

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
				f.apis = append(f.apis, coreagents.API{ID: "rs_other", Identifier: "https://api.other.example"})
				f.list[0].APIs = known(append(f.list[0].APIs.Value, coreagents.Grant{Audience: "https://api.other.example", SubjectType: "user"}))
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
			name:   "the API has the Personal Agent Protocol off",
			mutate: func(f *fakeTenant) { f.allAPIs[0].Poppy = false },
			want:   []want{{id: "D8", level: Fail, spec: "3.2", contains: shopAPI + " has use_for_personal_agent_protocol off"}},
		},
		{
			name:   "the API has other agent subject claims",
			mutate: func(f *fakeTenant) { f.allAPIs[0].AgentSubjectClaims = "" },
			want:   []want{{id: "D8", level: Fail, spec: "3.2", contains: `agent_subject_claims "", and it must be auth0-v1`}},
		},
		{
			name:   "the API has another guest policy",
			mutate: func(f *fakeTenant) { f.allAPIs[0].GuestPolicy = "allow_all" },
			want:   []want{{id: "D8", level: Fail, spec: "3.2", contains: `anonymous_user.policy "allow_all", and it must be require_client_grant`}},
		},
		{
			name:   "an API that has the protocol off and is not in the config file is not checked",
			mutate: func(f *fakeTenant) { f.allAPIs = append(f.allAPIs, setup.API{Identifier: "https://other.example"}) },
			absent: []string{"https://other.example"},
		},
		{
			name:   "the web app has no scopes on the API",
			mutate: func(f *fakeTenant) { f.granted = nil },
			want:   []want{{id: "D9", level: Warn, spec: "design 5.5", contains: "Shop Web has no scopes on " + shopAPI}},
		},
		{
			name:   "the web app has scopes on the API",
			mutate: func(f *fakeTenant) { f.granted = []string{"orders:read", "checkout"} },
			want:   []want{{id: "D9", level: Pass, spec: "design 5.5", contains: "Shop Web can use " + shopAPI + " with 2 scope(s)"}},
		},
		{
			name:   "the grants of the web app cannot be read",
			mutate: func(f *fakeTenant) { f.grantedErr = forbidden(`{"errorCode":"insufficient_scope"}`) },
			want:   []want{{id: "D9", level: Skipped, spec: "design 5.5", contains: "read:client_grants"}},
		},
		{
			name:   "the company domain is not listed",
			mutate: func(f *fakeTenant) { f.settings.CompanyDomains = []string{"other.example"} },
			want:   []want{{id: "D8", level: Fail, spec: "3.2", contains: "does not list shop.example"}},
		},
		{
			name:   "the company domain is listed as an origin in another case",
			mutate: func(f *fakeTenant) { f.settings.CompanyDomains = []string{"https://SHOP.example"} },
			want: []want{
				{id: "D8", level: Pass, spec: "3.2", contains: "lists shop.example"},
				{id: "D8", level: Warn, spec: "3.2", contains: `"https://SHOP.example", which is not a bare lower-case hostname`},
			},
		},
		{
			name:   "the company domain is listed with capitals",
			mutate: func(f *fakeTenant) { f.settings.CompanyDomains = []string{"Shop.example"} },
			want:   []want{{id: "D8", level: Warn, spec: "3.2", contains: `"Shop.example", which is not a bare lower-case hostname`}},
		},
		{
			name: "a company domain entry that is not a bare hostname",
			mutate: func(f *fakeTenant) {
				f.settings.CompanyDomains = []string{"shop.example", "https://shop.example/store/"}
			},
			want: []want{{id: "D8", level: Warn, spec: "3.2", contains: `"https://shop.example/store/", which is not a bare lower-case hostname`}},
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
			want: []want{{id: "D11", level: Fail, spec: "4.1", contains: museName + `: metadata status is "unavailable"`}},
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
			want:   []want{{id: "D11", level: Pass, spec: "4.1", contains: museName + ": DPoP required"}},
		},
		{
			name:   "DPoP is unknown",
			mutate: func(f *fakeTenant) { f.list[0].DPoPRequired = coreagents.Maybe[bool]{} },
			absent: []string{"DPoP required", "DPoP not"},
		},
		{
			name: "the metadata document is invalid",
			mutate: func(f *fakeTenant) {
				f.detail.MetadataCheck = coreagents.MetadataCheck{Valid: known(false), Violations: []string{"jwks is missing", "name is empty"}}
				f.detail.KeysCurrent = coreagents.Maybe[bool]{}
			},
			want: []want{{id: "D12", level: Fail, spec: "4.1", contains: "the metadata document is invalid: jwks is missing; name is empty"}},
		},
		{
			name:   "the keys changed",
			mutate: func(f *fakeTenant) { f.detail.KeysCurrent = known(false) },
			want:   []want{{id: "D12", level: Fail, spec: "5.2", contains: "other keys than the ones Auth0 stored"}},
		},
		{
			name: "the document signs with ES256 and Auth0 stored its key",
			mutate: func(f *fakeTenant) {
				f.detail.MetadataCheck = coreagents.MetadataCheck{Valid: known(true), UsableKeys: 1}
				f.detail.KeysCurrent = known(true)
			},
			want:   []want{{id: "D12", level: Pass, spec: "5.2", contains: museName + ": the keys Auth0 stored match the document"}},
			absent: []string{"cannot check", "other keys than"},
		},
		{
			name: "the document has no key that Auth0 stores",
			mutate: func(f *fakeTenant) {
				f.detail.MetadataCheck = coreagents.MetadataCheck{Valid: known(true)}
				f.detail.KeysCurrent = known(true)
			},
			want:   []want{{id: "D12", level: Warn, spec: "5.2", contains: museName + ": Auth0 stored no key for this agent, because no key of the document matches its token_endpoint_auth_signing_alg"}},
			absent: []string{"cannot check", "the keys Auth0 stored match"},
		},
		{
			name: "the preview failed",
			mutate: func(f *fakeTenant) {
				f.detail.MetadataCheck = coreagents.MetadataCheck{}
				f.detail.KeysCurrent = coreagents.Maybe[bool]{}
				f.detail.Notes = []string{"The metadata document could not be checked: boom."}
			},
			want: []want{{id: "D12", level: Warn, spec: "4.1", contains: "cannot check " + museName + ": The metadata document could not be checked: boom"}},
		},
		{
			name:   "the agent cannot be shown",
			mutate: func(f *fakeTenant) { f.showErr = errBoom },
			want:   []want{{id: "D12", level: Warn, spec: "4.1", contains: "cannot check the metadata of " + museName + ": boom"}},
		},
		{
			name:   "the keys need a scope",
			mutate: func(f *fakeTenant) { f.showErr = forbidden(scopeBody) },
			want:   []want{{id: "D12", level: Skipped, spec: "4.1", contains: "the login lacks read:clients, read:client_credentials"}},
		},
		{
			name: "the agent is granted only some of the Poppy APIs",
			mutate: func(f *fakeTenant) {
				f.apis = append(f.apis, coreagents.API{ID: "rs_other", Identifier: "https://api.other.example"})
				f.settings.DefaultAudience = ""
			},
			want: []want{{id: "D13", level: Warn, contains: museName + " is granted 1 of 2 Poppy APIs (missing: https://api.other.example)"}},
		},
		{
			name: "the agent is granted none of the Poppy APIs",
			mutate: func(f *fakeTenant) {
				f.apis = append(f.apis, coreagents.API{ID: "rs_other", Identifier: "https://api.other.example"})
				f.settings.DefaultAudience = ""
				f.list[0].APIs = known([]coreagents.Grant{{ID: "cgr_m2m", Audience: shopAPI, SubjectType: "client"}})
			},
			want: []want{
				{id: "D13", level: Fail, contains: museName + " is not granted " + shopAPI},
				{id: "D13", level: Fail, contains: museName + " is not granted https://api.other.example"},
			},
		},
		{
			name: "an agent has no grant at all",
			mutate: func(f *fakeTenant) {
				f.list = append(f.list, coreagents.GrantedAgent{
					Agent: coreagents.Agent{AgentID: "agt_lost", Name: "Lost", URL: "https://lost.example/agent.json", MetadataStatus: "available"},
					APIs:  known([]coreagents.Grant{}),
				})
			},
			want: []want{{id: "D13", level: Warn, contains: "Lost has no grant to a Poppy API"}},
		},
		{
			name: "a grant of another subject type does not count",
			mutate: func(f *fakeTenant) {
				f.list[0].APIs = known([]coreagents.Grant{{ID: "cgr_m2m", Audience: shopAPI, Scopes: []string{"orders:read"}, SubjectType: "client"}})
			},
			want: []want{{id: "D13", level: Fail, contains: museName + " is not granted " + shopAPI}},
		},
		{
			name:   "the grants of the agent could not be read",
			mutate: func(f *fakeTenant) { f.list[0].APIs = coreagents.Maybe[[]coreagents.Grant]{} },
			want:   []want{{id: "D13", level: Warn, contains: "the grants of the agent could not be read"}},
		},
		{
			name:   "the tenant has no agent",
			mutate: func(f *fakeTenant) { f.list = nil },
			want: []want{
				{id: "D11", level: Skipped, spec: "4.1", contains: "the tenant has no agent"},
				{id: "D12", level: Skipped, spec: "4.1", contains: "the tenant has no agent"},
				{id: "D13", level: Skipped, contains: "the tenant has no agent"},
			},
		},
		{
			name: "no agent has a Poppy grant",
			mutate: func(f *fakeTenant) {
				f.list[0].APIs = known([]coreagents.Grant{})
			},
			want: []want{
				{id: "D11", level: Skipped, spec: "4.1", contains: "no agent has a grant"},
				{id: "D12", level: Skipped, spec: "4.1", contains: "no agent has a grant"},
				{id: "D13", level: Warn, contains: museName + " has no grant to a Poppy API"},
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
			name:   "the entry of the Poppy API is on another host",
			mutate: func(*fakeTenant) {},
			file: func(t *testing.T) string {
				return goldenConfigFile(t, func(c *config.Config) { c.APIs[0].URL = "https://docs.example/openapi.json" })
			},
			want: []want{{id: "D14", level: Warn, contains: "no apis entry is on the host of a Poppy API"}},
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
			want: []want{{id: "D14", level: Warn, contains: "is a protocol 0.3 file, which is stale: this CLI writes 1.0"}},
		},
		{
			name:   "the config file is a protocol 0.4 file",
			mutate: func(*fakeTenant) {},
			file: func(t *testing.T) string {
				return goldenConfigFile(t, func(c *config.Config) { c.ProtocolVersion = "0.4" })
			},
			want: []want{{id: "D14", level: Warn, contains: "is a protocol 0.4 file, which is stale: this CLI writes 1.0"}},
		},
		{
			name:   "the config file is a protocol 1.1 file, which is not stale",
			mutate: func(*fakeTenant) {},
			file: func(t *testing.T) string {
				return goldenConfigFile(t, func(c *config.Config) { c.ProtocolVersion = "1.1" })
			},
			want: []want{{id: "D14", level: Pass, contains: "matches the tenant"}},
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
			name:   "the config file has web.disabled together with the join address",
			mutate: func(f *fakeTenant) { f.state.App, f.joinApps = setup.App{}, nil },
			file: func(t *testing.T) string {
				return goldenConfigFile(t, func(c *config.Config) { c.Web.Disabled = true })
			},
			want: []want{
				{id: "D9", level: Skipped, spec: "design 5.5", contains: messageWebOff},
				{id: "D14", level: Warn, contains: "web.browser_session_endpoint is "},
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
			name:   "no config file skips D14",
			mutate: func(*fakeTenant) {},
			file:   func(t *testing.T) string { return filepath.Join(t.TempDir(), "missing.json") },
			absent: []string{"matches the tenant", "cannot compare"},
			want: []want{
				{id: "D9", level: Pass, spec: "design 5.5", contains: "Shop Web is first-party"},
				{id: "D14", level: Skipped, contains: "there is no config file at"},
			},
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
					assert.NotEmpty(t, res.FixCommand+res.FixHint, res.Message)
				}
			}

			wantedNonPass := 0
			for _, w := range test.want {
				if w.level != Pass {
					wantedNonPass++
				}
			}
			assert.Len(t, withoutNoConfigFile(results, test.want), wantedNonPass, "unexpected extra results: %+v", withoutNoConfigFile(results, test.want))

			for _, text := range test.absent {
				assert.False(t, slices.ContainsFunc(results, func(res Result) bool { return strings.Contains(res.Message, text) }), "%q in %+v", text, results)
			}
		})
	}
}

// withoutNoConfigFile returns the results that are not a pass, and leaves out the
// D14 that is skipped because the file is missing unless a case wants it. Most
// cases run without a file.
func withoutNoConfigFile(results []Result, wants []want) []Result {
	if slices.ContainsFunc(wants, func(w want) bool { return w.id == "D14" }) {
		return nonPass(Report{Results: results})
	}

	return slices.DeleteFunc(nonPass(Report{Results: results}), func(res Result) bool { return res.Code == "D14.no_config_file" })
}

// goldenDocument is the config file that matches the healthy tenant, as the
// poppy.json a website would serve.
func goldenDocument(t *testing.T) map[string]any {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "config", "testdata", "poppy.golden.json"))
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(data, &doc))

	return doc
}

func TestConfigSourceOfD14(t *testing.T) {
	const liveName = "the live poppy.json at https://" + testDomain + wellKnownPoppy

	notFound := func(s *site) {
		s.routes[testDomain+wellKnownPoppy] = func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }
	}

	tests := []struct {
		name     string
		site     func(*site)
		file     func(*testing.T) string
		explicit bool
		tenant   bool
		level    Level
		code     string
		contains string
		absent   string
		command  string
		hint     string
	}{
		{
			name:     "a local file is compared, whatever the website serves",
			site:     func(s *site) { s.poppy["organization"].(map[string]any)["name"] = "Another shop" },
			file:     func(t *testing.T) string { return goldenConfigFile(t, nil) },
			level:    Pass,
			code:     "D14.matches",
			contains: "the config file ",
			absent:   "live",
		},
		{
			name: "a local file that differs is named as the config file",
			file: func(t *testing.T) string {
				return goldenConfigFile(t, func(c *config.Config) { c.Organization.Name = "Old Shop" })
			},
			level:    Warn,
			code:     "D14.value_differs",
			contains: "in the config file ",
			absent:   "live",
			command:  "auth0 poppy config --company-domain " + testDomain,
		},
		{
			name:     "the live poppy.json is compared when there is no local file",
			file:     func(t *testing.T) string { return filepath.Join(t.TempDir(), "missing.json") },
			level:    Pass,
			code:     "D14.matches",
			contains: liveName + " matches the tenant",
		},
		{
			name:     "a live poppy.json that differs from the tenant is named as live",
			site:     func(s *site) { s.poppy["organization"].(map[string]any)["name"] = "Another shop" },
			file:     func(t *testing.T) string { return filepath.Join(t.TempDir(), "missing.json") },
			level:    Warn,
			code:     "D14.value_differs",
			contains: `"Another shop" in ` + liveName,
			command:  "auth0 poppy config --company-domain " + testDomain,
			hint:     "Deploy the file it writes to https://" + testDomain + wellKnownPoppy,
		},
		{
			name:     "a live poppy.json that is not a config file skips D14",
			site:     func(s *site) { s.poppy["web"] = map[string]any{"disabled": "yes"} },
			file:     func(t *testing.T) string { return filepath.Join(t.TempDir(), "missing.json") },
			level:    Skipped,
			code:     "D14.live_unreadable",
			contains: "cannot be read as one",
		},
		{
			name:     "a config file that was named and is missing does not fall back to the live poppy.json",
			file:     func(t *testing.T) string { return filepath.Join(t.TempDir(), "missing.json") },
			explicit: true,
			level:    Skipped,
			code:     "D14.no_config_file",
			contains: "there is no config file at ",
			command:  "auth0 poppy config --company-domain " + testDomain,
		},
		{
			name:     "no local file and no live poppy.json skips D14",
			site:     notFound,
			file:     func(t *testing.T) string { return filepath.Join(t.TempDir(), "missing.json") },
			level:    Skipped,
			code:     "D14.no_config_file",
			contains: "there is no config file at ",
			command:  "auth0 poppy config --company-domain " + testDomain,
		},
		{
			name:     "the tenant-only run does not fetch the live poppy.json",
			file:     func(t *testing.T) string { return filepath.Join(t.TempDir(), "missing.json") },
			tenant:   true,
			level:    Skipped,
			code:     "D14.no_config_file",
			contains: "there is no config file at ",
			command:  "auth0 poppy config --company-domain " + testDomain,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policies := mock.NewMockGuardianPolicyAPIV3(gomock.NewController(t))
			policies.EXPECT().List(gomock.Any()).Return(managementv3.ListGuardianPoliciesResponseContent{}, nil)

			s := validSite()
			s.poppy = goldenDocument(t)

			if test.site != nil {
				test.site(s)
			}

			f := healthyTenant()
			opts := Options{
				Tenant:             &Tenant{State: f, Agents: f, Policies: policies},
				ConfigFile:         test.file(t),
				ConfigFileExplicit: test.explicit,
				TenantOnly:         test.tenant,
			}

			report := s.runWith(t, opts)

			i := slices.IndexFunc(report.Results, func(res Result) bool { return res.Code == test.code })
			require.GreaterOrEqual(t, i, 0, "missing %s in %+v", test.code, report.Results)

			res := report.Results[i]
			assert.Equal(t, "D14", res.ID)
			assert.Equal(t, test.level, res.Level)
			assert.Contains(t, res.Message, test.contains)

			if test.absent != "" {
				assert.NotContains(t, res.Message, test.absent)
			}

			if test.command == "" {
				assert.Empty(t, res.FixCommand)
			} else {
				assert.True(t, strings.HasPrefix(res.FixCommand, test.command), res.FixCommand)
			}

			if test.hint == "" {
				assert.Empty(t, res.FixHint)
			} else {
				assert.Contains(t, res.FixHint, test.hint)
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
			want: []string{"D8", "D9", "D10", "D11", "D12", "D13", "D14"},
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
			want: []string{"D8", "D9", "D10", "D11", "D12", "D13", "D14"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := &runner{domain: testDomain, origin: "https://" + testDomain}
			file, fileErr := readConfigFile(test.file(t))

			results, ran := r.checkTenant(t.Context(), Options{}, "", file, fileErr)

			assert.False(t, ran)

			var ids []string
			for _, res := range results {
				ids = append(ids, res.ID)
				assert.Equal(t, Skipped, res.Level)
				assert.Equal(t, SkipNoLogin, res.SkipReason)
				assert.Equal(t, res.ID+".no_login", res.Code)
				assert.Equal(t, noLogin, res.Message)
				assert.Equal(t, "auth0 login", res.FixCommand)
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
	assert.Equal(t, []string{"D1", "D2", "D3", "D4", "D5", "D6", "D7", "D15", "D16", "D17", "D8", "D9", "D10", "D11", "D12", "D13", "D14"}, ids)
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
			mutate: func(f *fakeTenant) { f.settings.CompanyDomains = []string{"other.example"} },
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
			name:    "the issuer is not known and the tenant lists the domain",
			issuer:  "",
			wantRun: true,
		},
		{
			name:   "the issuer is not known and the tenant does not list the domain",
			issuer: "",
			mutate: func(f *fakeTenant) { f.settings.CompanyDomains = []string{"other.example"} },
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
			results, ran := r.checkTenant(t.Context(), Options{Tenant: &Tenant{Domain: "tenant.us.auth0.com", State: f, Agents: f, Policies: policies}}, test.issuer, nil, nil)

			assert.Equal(t, test.wantRun, ran)

			if test.wantRun {
				assert.Greater(t, len(results), 1)
				assert.NotEqual(t, Skipped, results[0].Level)
				return
			}

			require.Len(t, results, len(tenantChecks), "one skipped result for each check")
			for i, res := range results {
				assert.Equal(t, tenantChecks[i].id, res.ID)
				assert.Equal(t, tenantChecks[i].id+".different_tenant", res.Code)
				assert.Equal(t, Skipped, res.Level)
				assert.Equal(t, SkipDifferentTenant, res.SkipReason)
				assert.Equal(t, "you are logged in to tenant.us.auth0.com, which does not list shop.example as a company domain, so the tenant checks do not apply", res.Message)
				assert.NotEmpty(t, res.FixHint)
			}
			assert.Zero(t, Report{Results: results}.ExitCode())
		})
	}
}

func TestRunOtherCompanysDomainIsNotFailedByTheTenant(t *testing.T) {
	f := healthyTenant()
	f.hosts = []string{"tenant.us.auth0.com", "login.other.example"}
	f.settings.CompanyDomains = []string{"other.example"}

	// The mock has no expectation, so any tenant check that reaches it fails the test.
	policies := mock.NewMockGuardianPolicyAPIV3(gomock.NewController(t))

	report := validSite().runWith(t, Options{Tenant: &Tenant{State: f, Agents: f, Policies: policies}, ConfigFile: goldenConfigFile(t, nil)})

	assert.Zero(t, report.ExitCode(), "%+v", nonPass(report))
	require.NotEmpty(t, report.Results)

	assert.False(t, report.TenantChecked)
	assert.Empty(t, f.shown)

	for _, res := range report.Results {
		if res.Level == Skipped && res.SkipReason == SkipDifferentTenant {
			assert.Contains(t, res.Message, "which does not list shop.example as a company domain")
			continue
		}

		assert.Contains(t, []string{"D1", "D2", "D3", "D4", "D5", "D6", "D7", "D15", "D16", "D17"}, res.ID)
		assert.NotEqual(t, Fail, res.Level, res.Message)
	}
}

func TestRunWithoutLoginKeepsDiscoveryResults(t *testing.T) {
	s := validSite()

	withLogin := s.run(t)
	full := s.runWith(t, Options{})

	assert.Equal(t, withLogin.Results, full.Results[:len(withLogin.Results)], "D1 to D7 are unchanged")

	tail := full.Results[len(withLogin.Results):]
	require.Len(t, tail, 7)
	for _, res := range tail {
		assert.Equal(t, Skipped, res.Level)
		assert.Equal(t, noLogin, res.Message)
	}
	assert.Zero(t, full.ExitCode())
}
