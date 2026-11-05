package setup

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/poppy/config"
)

func fieldNames(change Change) []string {
	names := make([]string, len(change.Fields))
	for i, field := range change.Fields {
		names[i] = field.Name
	}

	return names
}

func kinds(plan Plan) []Kind {
	out := make([]Kind, len(plan.Changes))
	for i, change := range plan.Changes {
		out[i] = change.Kind
	}

	return out
}

// freshState is a tenant that has never been set up for Poppy.
func freshState(t *testing.T) (*State, Inputs) {
	t.Helper()

	state := configuredState(t)
	state.Tenant = Tenant{ResourceParameterProfile: "audience", FriendlyName: "Shop"}
	state.CustomDomain = ""
	state.APIs[0] = API{
		ID:         "rs_shop",
		Identifier: shopAPI,
		Name:       "Shop API",
		Scopes: []Scope{
			{Name: "orders:read", Description: "See your orders and their status. Read only."},
			{Name: "checkout", Description: "Place orders and pay with your saved card. Can't be undone."},
		},
		AgentSubjectClaims: "off",
	}
	state.App.BrowserJoin = false
	state.HasGuestGrant = map[string]bool{}
	state.File = nil

	in := configuredInputs()
	in.APIURLs = map[string]string{shopAPI: shopAPI + "/openapi.json"}

	return state, in
}

func TestBuildFreshTenant(t *testing.T) {
	state, in := freshState(t)

	plan, err := Build(state, in)

	require.NoError(t, err)
	assert.Equal(t, tenantURL, plan.Tenant)
	assert.Equal(t, "https://"+tenantURL+"/", plan.Issuer)
	require.Len(t, plan.Warnings, 1)
	assert.Contains(t, plan.Warnings[0], "agents will see the Auth0 domain "+tenantURL)
	assert.Contains(t, plan.Warnings[0], "signs every user out")

	assert.Equal(t, []Kind{KindTenant, KindAPI, KindApp, KindGrant, KindFile}, kinds(plan), "in apply order")

	tenant := plan.Changes[0]
	assert.Equal(t, ImpactTenantWide, tenant.Impact)
	assert.Equal(t, []Field{
		{Name: "client_id_metadata_document_supported", Value: "true", Before: "false"},
		{Name: "authorization_response_iss_parameter_supported", Value: "true", Before: "false", Effect: "every app's redirects"},
		{Name: "resource_parameter_profile", Value: "compatibility", Before: "audience"},
		{Name: "default_audience", Value: shopAPI, Before: "none", Effect: "apps that send no audience"},
		{Name: "personal_agent_protocol_company_domains", Value: "https://shop.example", Before: "none"},
	}, tenant.Fields)

	api := plan.Changes[1]
	assert.Equal(t, ImpactAPIWide, api.Impact)
	assert.Equal(t, []Field{
		{Name: "scopes", Value: "poppy:browser-sign-in (new)"},
		{Name: "allow_offline_access", Value: "true", Before: "false"},
		{Name: "agent_subject_claims", Value: "auth0-v1", Before: "off"},
		{Name: "use_for_personal_agent_protocol", Value: "true", Before: "false"},
		{Name: "subject_type_authorization.anonymous_user.policy", Value: "require_client_grant", Before: "none"},
	}, api.Fields)
	assert.Equal(t, shopAPI+": scopes [poppy:browser-sign-in (new)], allow_offline_access, agent_subject_claims, use_for_personal_agent_protocol, subject_type_authorization.anonymous_user.policy", api.Description)

	app := plan.Changes[2]
	assert.Equal(t, ImpactApp, app.Impact)
	assert.Equal(t, []Field{{Name: "allow_personal_agent_browser_join", Value: "true", Before: "false"}}, app.Fields)

	grant := plan.Changes[3]
	assert.Equal(t, ImpactGrant, grant.Impact)
	assert.Equal(t, "Shop Web → "+shopAPI+" for guests (scopes: orders:read, checkout, poppy:browser-sign-in)", grant.Description)
	assert.Equal(t, []Field{
		{Name: "client_id", Value: shopApp},
		{Name: "audience", Value: shopAPI},
		{Name: "scope", Value: "orders:read checkout poppy:browser-sign-in"},
		{Name: "subject_type", Value: "anonymous_user"},
	}, grant.Fields)

	file := plan.Changes[4]
	assert.Equal(t, ImpactFile, file.Impact)
	assert.Equal(t, "write auth0-poppy.json", file.Description)

	assert.Equal(t, []string{
		"create:client_grants", "read:client_grants", "read:clients", "read:custom_domains", "read:resource_servers",
		"read:tenant_settings", "update:clients", "update:resource_servers", "update:tenant_settings",
	}, plan.Scopes())
}

// TestBuildIsIdempotent is the idempotency test: a tenant that already matches
// gets an empty plan.
func TestBuildIsIdempotent(t *testing.T) {
	plan, err := Build(configuredState(t), configuredInputs())

	require.NoError(t, err)
	assert.True(t, plan.Empty())
	assert.Empty(t, plan.Warnings)
	assert.NotNil(t, plan.Changes)
	assert.ElementsMatch(t, ReadScopes, plan.Scopes(), "an empty plan needs only the read scopes")
	assert.Len(t, plan.Unchanged, 10, "every setting is already right")
}

func TestBuildPartiallyConfigured(t *testing.T) {
	tests := []struct {
		name       string
		change     func(state *State, in *Inputs)
		wantKinds  []Kind
		wantFields []string
	}{
		{
			name:       "client ID metadata documents are off",
			change:     func(state *State, _ *Inputs) { state.Tenant.CIMD = false },
			wantKinds:  []Kind{KindTenant},
			wantFields: []string{"client_id_metadata_document_supported"},
		},
		{
			name:       "the iss parameter is off",
			change:     func(state *State, _ *Inputs) { state.Tenant.IssParameter = false },
			wantKinds:  []Kind{KindTenant},
			wantFields: []string{"authorization_response_iss_parameter_supported"},
		},
		{
			name:       "the resource parameter profile is audience",
			change:     func(state *State, _ *Inputs) { state.Tenant.ResourceParameterProfile = "audience" },
			wantKinds:  []Kind{KindTenant},
			wantFields: []string{"resource_parameter_profile"},
		},
		{
			name:       "there is no default audience",
			change:     func(state *State, _ *Inputs) { state.Tenant.DefaultAudience = "" },
			wantKinds:  []Kind{KindTenant},
			wantFields: []string{"default_audience"},
		},
		{
			name:       "another default audience is kept",
			change:     func(state *State, _ *Inputs) { state.Tenant.DefaultAudience = "https://other.example" },
			wantKinds:  nil,
			wantFields: nil,
		},
		{
			name:       "the company domain is not listed",
			change:     func(state *State, _ *Inputs) { state.Tenant.CompanyDomains = nil },
			wantKinds:  []Kind{KindTenant},
			wantFields: []string{"personal_agent_protocol_company_domains"},
		},
		{
			name:       "offline access is off",
			change:     func(state *State, _ *Inputs) { state.APIs[0].AllowOfflineAccess = false },
			wantKinds:  []Kind{KindAPI},
			wantFields: []string{"allow_offline_access"},
		},
		{
			name:       "agent subject claims are off",
			change:     func(state *State, _ *Inputs) { state.APIs[0].AgentSubjectClaims = "off" },
			wantKinds:  []Kind{KindAPI},
			wantFields: []string{"agent_subject_claims"},
		},
		{
			name:       "the API does not use the protocol",
			change:     func(state *State, _ *Inputs) { state.APIs[0].Poppy = false },
			wantKinds:  []Kind{KindAPI},
			wantFields: []string{"use_for_personal_agent_protocol"},
		},
		{
			name:       "the guest policy is not set",
			change:     func(state *State, _ *Inputs) { state.APIs[0].GuestPolicy = "" },
			wantKinds:  []Kind{KindAPI},
			wantFields: []string{"subject_type_authorization.anonymous_user.policy"},
		},
		{
			name: "a scope gets a description",
			change: func(state *State, in *Inputs) {
				state.APIs[0].Scopes[1].Description = ""
				in.Scopes = map[string]string{"checkout": "Place orders and pay with your saved card. Can't be undone."}
			},
			wantKinds:  []Kind{KindAPI},
			wantFields: []string{"scopes"},
		},
		{
			name:       "a scope is added",
			change:     func(_ *State, in *Inputs) { in.Scopes = map[string]string{"refunds": "Refund an order."} },
			wantKinds:  []Kind{KindAPI, KindFile},
			wantFields: []string{"scopes"},
		},
		{
			name:       "the browser join is off",
			change:     func(state *State, _ *Inputs) { state.App.BrowserJoin = false },
			wantKinds:  []Kind{KindApp},
			wantFields: []string{"allow_personal_agent_browser_join"},
		},
		{
			name:       "--login-uri differs from the current route",
			change:     func(_ *State, in *Inputs) { in.LoginURI = "https://shop.example/auth/login" },
			wantKinds:  []Kind{KindApp},
			wantFields: []string{"initiate_login_uri"},
		},
		{
			name:       "--login-uri equals the current route",
			change:     func(_ *State, in *Inputs) { in.LoginURI = "https://shop.example/login" },
			wantKinds:  nil,
			wantFields: nil,
		},
		{
			name:       "the guest grant is missing",
			change:     func(state *State, _ *Inputs) { state.HasGuestGrant = map[string]bool{} },
			wantKinds:  []Kind{KindGrant},
			wantFields: []string{"client_id", "audience", "scope", "subject_type"},
		},
		{
			name: "the guest grant is missing but not asked for",
			change: func(state *State, in *Inputs) {
				state.HasGuestGrant = map[string]bool{}
				in.GuestAPIAccess = false
			},
			wantKinds:  nil,
			wantFields: nil,
		},
		{
			name: "there is no config file",
			change: func(state *State, in *Inputs) {
				state.File = nil
				in.APIURLs = map[string]string{shopAPI: shopAPI + "/openapi.json"}
			},
			wantKinds: []Kind{KindFile},
		},
		{
			name:      "--name differs from the file",
			change:    func(_ *State, in *Inputs) { in.Name = "Shop Ltd" },
			wantKinds: []Kind{KindFile},
		},
		{
			name:      "--hint differs from the file",
			change:    func(_ *State, in *Inputs) { in.Hints = map[string]config.Hints{"checkout": {Changes: "reversible"}} },
			wantKinds: []Kind{KindFile},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state, in := configuredState(t), configuredInputs()
			test.change(state, &in)

			plan, err := Build(state, in)

			require.NoError(t, err)
			require.Len(t, plan.Changes, len(test.wantKinds), "only the differences are planned")
			if len(test.wantKinds) > 0 {
				assert.Equal(t, test.wantKinds, kinds(plan))
			}

			if test.wantFields != nil {
				assert.Equal(t, test.wantFields, fieldNames(plan.Changes[0]))
			}
		})
	}
}

func TestBuildDefaultAudience(t *testing.T) {
	tests := []struct {
		name   string
		change func(state *State)
		want   bool
	}{
		{name: "one API", change: func(*State) {}, want: true},
		{
			name: "several APIs",
			change: func(state *State) {
				state.APIs = append(state.APIs, API{
					ID: "rs_other", Identifier: "https://other.example", Name: "Other API",
					Scopes: []Scope{{Name: "read", Description: "Read."}}, AllowOfflineAccess: true, AgentSubjectClaims: "auth0-v1",
					Poppy: true, GuestPolicy: "require_client_grant",
				})
				state.HasGuestGrant["https://other.example"] = true
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state, in := configuredState(t), configuredInputs()
			state.Tenant.DefaultAudience = ""
			test.change(state)
			in.APIURLs = map[string]string{"https://other.example": "https://other.example/openapi.json"}

			plan, err := Build(state, in)

			require.NoError(t, err)
			tenantChanged := len(plan.Changes) > 0 && plan.Changes[0].Kind == KindTenant
			assert.Equal(t, test.want, tenantChanged, "default_audience is set only with exactly one API")
		})
	}
}

func TestBuildSeveralAPIsAndGuests(t *testing.T) {
	state, in := freshState(t)
	state.APIs = append(state.APIs, API{
		ID: "rs_other", Identifier: "https://other.example", Name: "Other API",
		Scopes: []Scope{{Name: "read", Description: "Read."}},
	})
	state.HasGuestGrant = map[string]bool{"https://other.example": true}
	in.APIs = append(in.APIs, "https://other.example")
	in.APIURLs["https://other.example"] = "https://other.example/openapi.json"

	plan, err := Build(state, in)

	require.NoError(t, err)
	assert.Equal(t, []Kind{KindTenant, KindAPI, KindAPI, KindApp, KindGrant, KindFile}, kinds(plan))
	assert.NotContains(t, fieldNames(plan.Changes[0]), "default_audience")
	assert.Equal(t, shopAPI+": ", plan.Changes[1].Description[:len(shopAPI)+2])
	assert.Equal(t, "https://other.example: ", plan.Changes[2].Description[:len("https://other.example")+2])
	assert.Equal(t, "Shop Web → "+shopAPI+" for guests (scopes: orders:read, checkout, poppy:browser-sign-in)", plan.Changes[4].Description, "the other API already has its guest grant")
}

func TestBuildAppendsToTheCompanyDomains(t *testing.T) {
	state, in := configuredState(t), configuredInputs()
	state.Tenant.CompanyDomains = []string{"https://a.example", "https://b.example"}

	plan, err := Build(state, in)

	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	assert.Equal(t, []Field{{Name: "personal_agent_protocol_company_domains", Value: "https://a.example, https://b.example, https://shop.example", Before: "https://a.example, https://b.example"}}, plan.Changes[0].Fields)
}

func TestBuildWarnings(t *testing.T) {
	state, in := configuredState(t), configuredInputs()
	state.APIs[0].Identifier = "urn:shop:api"
	in.APIURLs = map[string]string{"urn:shop:api": shopAPI + "/openapi.json"}

	plan, err := Build(state, in)

	require.NoError(t, err)
	assert.Equal(t, []string{"urn:shop:api is not an https URL, and Poppy APIs normally need one."}, plan.Warnings)
}

func TestBuildErrors(t *testing.T) {
	t.Run("a scope without a description", func(t *testing.T) {
		state, in := configuredState(t), configuredInputs()
		state.APIs[0].Scopes[0].Description = ""

		_, err := Build(state, in)

		var input *InputError
		require.ErrorAs(t, err, &input)
		assert.Equal(t, "scope_description_required", input.Reason)
	})

	t.Run("a value Auth0 does not store", func(t *testing.T) {
		state, in := freshState(t)
		in.APIURLs = nil

		_, err := Build(state, in)

		var missing *config.MissingFieldsError
		require.ErrorAs(t, err, &missing)
		assert.Equal(t, []string{"apis[" + shopAPI + "].url"}, missing.Fields)
	})

	t.Run("a config file of another issuer", func(t *testing.T) {
		state, in := configuredState(t), configuredInputs()
		state.File.Issuer = "https://other.example/"

		_, err := Build(state, in)

		assert.ErrorIs(t, err, config.ErrIssuerMismatch)

		in.ForceConfig = true
		plan, err := Build(state, in)
		require.NoError(t, err)
		assert.Equal(t, []Kind{KindFile}, kinds(plan))
	})
}

func TestPlanScopes(t *testing.T) {
	tests := []struct {
		name  string
		kinds []Kind
		want  []string
	}{
		{name: "no changes", want: ReadScopes},
		{name: "file only", kinds: []Kind{KindFile}, want: ReadScopes},
		{name: "tenant", kinds: []Kind{KindTenant}, want: append([]string{"update:tenant_settings"}, ReadScopes...)},
		{name: "api twice", kinds: []Kind{KindAPI, KindAPI}, want: append([]string{"update:resource_servers"}, ReadScopes...)},
		{name: "app", kinds: []Kind{KindApp}, want: append([]string{"update:clients"}, ReadScopes...)},
		{name: "grant", kinds: []Kind{KindGrant}, want: append([]string{"create:client_grants"}, ReadScopes...)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var plan Plan
			for _, kind := range test.kinds {
				plan.Changes = append(plan.Changes, Change{Kind: kind})
			}

			assert.ElementsMatch(t, test.want, plan.Scopes())
			assert.IsIncreasing(t, plan.Scopes(), "sorted and without duplicates")
		})
	}
}

func TestPlanJSON(t *testing.T) {
	state, in := freshState(t)
	plan, err := Build(state, in)
	require.NoError(t, err)

	data, err := json.Marshal(plan)
	require.NoError(t, err)

	var decoded struct {
		Tenant  string `json:"tenant"`
		Changes []struct {
			Kind   string `json:"kind"`
			Impact string `json:"impact"`
			Fields []struct {
				Name string `json:"name"`
			} `json:"fields"`
		} `json:"changes"`
		Warnings []string `json:"warnings"`
	}
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, tenantURL, decoded.Tenant)
	assert.Equal(t, "tenant", decoded.Changes[0].Kind)
	assert.Equal(t, "tenant-wide", decoded.Changes[0].Impact)
	assert.Equal(t, "client_id_metadata_document_supported", decoded.Changes[0].Fields[0].Name)
	assert.Len(t, decoded.Warnings, 1)

	data, err = json.Marshal(Plan{Changes: []Change{}, Warnings: []string{}})
	require.NoError(t, err)
	assert.JSONEq(t, `{"tenant":"","issuer":"","warnings":[],"changes":[]}`, string(data))
}

func grantScope(t *testing.T, plan Plan, audience string) (string, bool) {
	t.Helper()

	for _, change := range plan.Changes {
		if change.Kind != KindGrant || change.target != audience {
			continue
		}

		for _, field := range change.Fields {
			if field.Name == "scope" {
				return field.Value, true
			}
		}
	}

	return "", false
}

func TestBuildGuestGrantScopes(t *testing.T) {
	tests := []struct {
		name      string
		change    func(in *Inputs)
		wantScope string
		wantGrant bool
	}{
		{name: "every scope", wantScope: "orders:read checkout poppy:browser-sign-in", wantGrant: true},
		{
			name: "the chosen scopes only",
			change: func(in *Inputs) {
				in.GuestAllScopes, in.GuestScopes = false, map[string][]string{shopAPI: {"orders:read"}}
			},
			wantScope: "orders:read", wantGrant: true,
		},
		{
			name:   "no scope chosen for the API",
			change: func(in *Inputs) { in.GuestAllScopes, in.GuestScopes = false, map[string][]string{shopAPI: nil} },
		},
		{
			name:   "no choice at all",
			change: func(in *Inputs) { in.GuestAllScopes = false },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state, in := freshState(t)
			if test.change != nil {
				test.change(&in)
			}

			plan, err := Build(state, in)

			require.NoError(t, err)
			scope, found := grantScope(t, plan, shopAPI)
			assert.Equal(t, test.wantGrant, found)
			assert.Equal(t, test.wantScope, scope)
		})
	}

	t.Run("an API that has a guest grant stays out of the plan", func(t *testing.T) {
		state, in := freshState(t)
		state.HasGuestGrant = map[string]bool{shopAPI: true}
		in.GuestAllScopes, in.GuestScopes = false, map[string][]string{shopAPI: {"orders:read"}}

		plan, err := Build(state, in)

		require.NoError(t, err)
		_, found := grantScope(t, plan, shopAPI)
		assert.False(t, found)
	})
}

// richTenant is a tenant with something to change in every group: scopes to
// describe and to add, a login route, and a guest grant.
func richTenant(t *testing.T) (*State, Inputs) {
	t.Helper()

	state, in := freshState(t)
	state.Tenant.CompanyDomains = []string{"https://old.example"}
	state.APIs[0].Scopes = append(state.APIs[0].Scopes, Scope{Name: "refunds"})
	state.App.LoginURI = ""

	in.Scopes = map[string]string{"refunds": "Refund an order you placed.", "wishlist": "Keep a list of things you want."}
	in.LoginURI = "https://shop.example/login"
	in.GuestAllScopes, in.GuestScopes = false, map[string][]string{shopAPI: {"orders:read", "checkout"}}

	return state, in
}

func buildPlan(t *testing.T, state *State, in Inputs) Plan {
	t.Helper()

	plan, err := Build(state, in)
	require.NoError(t, err)

	return plan
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()

	data, err := json.Marshal(v)
	require.NoError(t, err)

	return data
}

func TestChangeCount(t *testing.T) {
	state, in := richTenant(t)
	plan := buildPlan(t, state, in)

	counts := map[Kind]int{}
	for _, change := range plan.Changes {
		counts[change.Kind] += change.Count()
	}

	assert.Equal(t, map[Kind]int{KindTenant: 5, KindAPI: 7, KindApp: 2, KindGrant: 1, KindFile: 1}, counts)
	assert.Equal(t, 16, plan.Count())
}

func TestPlanUnchanged(t *testing.T) {
	t.Run("a tenant that matches lists every setting", func(t *testing.T) {
		plan := buildPlan(t, configuredState(t), configuredInputs())

		assert.Equal(t, []string{
			"CIMD support", "iss in sign-in redirects", "resource parameter profile", "default audience", "company domain",
			"offline access", "agent subject claims", "Poppy API", "guest policy", "browser join",
		}, plan.Unchanged)
	})

	t.Run("a setting that changes is not listed", func(t *testing.T) {
		state := configuredState(t)
		state.Tenant.CIMD = false

		plan := buildPlan(t, state, configuredInputs())

		assert.NotContains(t, plan.Unchanged, "CIMD support")
		assert.Contains(t, plan.Unchanged, "iss in sign-in redirects")
	})

	t.Run("a login route that is given and right is listed", func(t *testing.T) {
		in := configuredInputs()
		in.LoginURI = "https://shop.example/login"

		assert.Contains(t, buildPlan(t, configuredState(t), in).Unchanged, "login route")
	})

	t.Run("several APIs are told apart", func(t *testing.T) {
		state, in := freshState(t)
		state.APIs = append(state.APIs, API{ID: "rs_pay", Identifier: "https://pay.shop.example", Name: "Pay", Poppy: true,
			Scopes: []Scope{{Name: "pay", Description: "Pay."}}})
		in.APIs = append(in.APIs, "https://pay.shop.example")
		in.APIURLs["https://pay.shop.example"] = "https://pay.shop.example/openapi.json"

		assert.Contains(t, buildPlan(t, state, in).Unchanged, "Poppy API on https://pay.shop.example")
	})

	t.Run("it stays out of the JSON when empty", func(t *testing.T) {
		assert.NotContains(t, string(mustJSON(t, Plan{Changes: []Change{}, Warnings: []string{}})), "unchanged")
	})
}
