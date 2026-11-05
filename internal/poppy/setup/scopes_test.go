package setup

import (
	"encoding/json"
	"path/filepath"
	"testing"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/poppy/config"
)

const otherAPI = "https://other.example"

// scopeChange returns the scopes field of the plan change of an API.
func scopeChange(t *testing.T, plan Plan, identifier string) (Field, bool) {
	t.Helper()

	for _, change := range plan.Changes {
		if change.Kind != KindAPI || change.target != identifier {
			continue
		}

		for _, field := range change.Fields {
			if field.Name == fieldScopes {
				return field, true
			}
		}
	}

	return Field{}, false
}

func TestBuildBrowserSignInScope(t *testing.T) {
	tests := []struct {
		name   string
		change func(state *State, in *Inputs)
		// WantScopes is the value of the scopes field of the shop API, empty when it
		// is not in the plan.
		wantScopes string
		// WantDescription is the description the scope gets, empty when it is left alone.
		wantDescription string
		wantErrReason   string
	}{
		{
			name:            "the scope is missing, so it is added with the default description",
			change:          func(state *State, _ *Inputs) { state.APIs[0].Scopes = state.APIs[0].Scopes[:2] },
			wantScopes:      "poppy:browser-sign-in (new)",
			wantDescription: BrowserSignInDescription,
		},
		{
			name:   "the scope exists with a description, so it is left alone",
			change: func(state *State, _ *Inputs) { state.APIs[0].Scopes[2].Description = "Sign in on the website." },
		},
		{
			name:            "the scope exists without a description, so the default fills it in",
			change:          func(state *State, _ *Inputs) { state.APIs[0].Scopes[2].Description = "" },
			wantScopes:      "poppy:browser-sign-in (description)",
			wantDescription: BrowserSignInDescription,
		},
		{
			name: "--scope wins over the default for a missing scope",
			change: func(state *State, in *Inputs) {
				state.APIs[0].Scopes = state.APIs[0].Scopes[:2]
				in.Scopes = map[string]string{BrowserSignInScope: "Their text"}
			},
			wantScopes:      "poppy:browser-sign-in (new)",
			wantDescription: "Their text",
		},
		{
			name: "--scope wins over the default for a missing description",
			change: func(state *State, in *Inputs) {
				state.APIs[0].Scopes[2].Description = ""
				in.Scopes = map[string]string{BrowserSignInScope: "Their text"}
			},
			wantScopes:      "poppy:browser-sign-in (description)",
			wantDescription: "Their text",
		},
		{
			name: "--scope does not edit an existing description",
			change: func(state *State, in *Inputs) {
				state.APIs[0].Scopes[2].Description = "Sign in on the website."
				in.Scopes = map[string]string{BrowserSignInScope: "Their text"}
			},
			wantErrReason: "scope_description_exists",
		},
		{
			name:   "the rerun after it exists changes nothing",
			change: func(*State, *Inputs) {},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state, in := configuredState(t), configuredInputs()
			test.change(state, &in)

			plan, err := Build(state, in)

			if test.wantErrReason != "" {
				var input *InputError
				require.ErrorAs(t, err, &input)
				assert.Equal(t, test.wantErrReason, input.Reason)

				return
			}

			require.NoError(t, err)

			field, found := scopeChange(t, plan, shopAPI)
			assert.Equal(t, test.wantScopes != "", found)
			assert.Equal(t, test.wantScopes, field.Value)

			scopes, err := resolveScopes(state, in)
			require.NoError(t, err)

			var description string
			for _, scope := range scopes[shopAPI].All {
				if scope.Name == BrowserSignInScope {
					description = scope.Description
				}
			}

			if test.wantDescription == "" {
				assert.Equal(t, state.APIs[0].Scopes[len(state.APIs[0].Scopes)-1].Description, description)

				return
			}

			assert.Equal(t, test.wantDescription, description)
		})
	}
}

func TestBuildBrowserSignInScopeIsShownInThePlan(t *testing.T) {
	state, in := configuredState(t), configuredInputs()
	state.APIs[0].Scopes = state.APIs[0].Scopes[:2]
	delete(state.File.Scopes, BrowserSignInScope)

	plan, err := Build(state, in)
	require.NoError(t, err)
	require.Equal(t, []Kind{KindAPI, KindFile}, kinds(plan))

	api := plan.Changes[0]
	assert.Equal(t, []Field{{Name: "scopes", Value: "poppy:browser-sign-in (new)"}}, api.Fields)
	assert.Equal(t, shopAPI+": scopes [poppy:browser-sign-in (new)]", api.Description)
	assert.Equal(t, []string{"read:clients", "read:client_grants", "read:custom_domains", "read:resource_servers", "read:tenant_settings", "update:resource_servers"}, unsorted(plan.Scopes()),
		"no new Management API scope")

	asJSON, err := json.Marshal(api)
	require.NoError(t, err)
	assert.JSONEq(t, `{"kind":"api","description":"`+shopAPI+`: scopes [poppy:browser-sign-in (new)]","impact":"api-wide","fields":[{"name":"scopes","value":"poppy:browser-sign-in (new)"}]}`, string(asJSON))
}

// unsorted returns the scopes in the order the test lists them, from the sorted plan scopes.
func unsorted(scopes []string) []string {
	order := []string{"read:clients", "read:client_grants", "read:custom_domains", "read:resource_servers", "read:tenant_settings", "update:resource_servers"}
	out := make([]string, 0, len(scopes))

	for _, want := range order {
		for _, scope := range scopes {
			if scope == want {
				out = append(out, scope)
			}
		}
	}

	return out
}

func TestBuildBrowserSignInScopeOnSeveralAPIs(t *testing.T) {
	state := configuredState(t)
	state.APIs = append(state.APIs, API{
		ID: "rs_other", Identifier: otherAPI,
		Scopes:             []Scope{{Name: "reports", Description: "See reports."}},
		AllowOfflineAccess: true, AgentSubjectClaims: "auth0-v1", Poppy: true, GuestPolicy: "require_client_grant",
	})
	state.HasGuestGrant[otherAPI] = true
	state.File.Scopes["reports"] = config.Scope{Description: "See reports."}
	state.File.APIs = append(state.File.APIs, config.API{Type: "openapi", URL: otherAPI + "/openapi.json", Description: "Other"})

	in := configuredInputs()
	in.APIs = append(in.APIs, otherAPI)

	plan, err := Build(state, in)

	require.NoError(t, err)

	_, shop := scopeChange(t, plan, shopAPI)
	assert.False(t, shop, "the API that has the scope is untouched")

	field, other := scopeChange(t, plan, otherAPI)
	require.True(t, other, "the API that lacks the scope gets it")
	assert.Equal(t, "poppy:browser-sign-in (new)", field.Value)

	assert.Equal(t, "See reports.", state.APIs[1].Scopes[0].Description)

	request := serverRequest(plan.Changes[0], state.APIs[1], mustResolve(t, state, in)[otherAPI])
	require.Len(t, request.Scopes, 2, "the existing scope is kept")
	assert.Equal(t, "reports", request.Scopes[0].Value)
	assert.Equal(t, BrowserSignInScope, request.Scopes[1].Value)
	assert.Equal(t, managementv3.String(BrowserSignInDescription), request.Scopes[1].Description)
}

func mustResolve(t *testing.T, state *State, in Inputs) map[string]apiScopes {
	t.Helper()

	scopes, err := resolveScopes(state, in)
	require.NoError(t, err)

	return scopes
}

func TestBrowserSignInScopeOnASecondAPIWithScopeFlag(t *testing.T) {
	state := twoAPIState(t)
	state.APIs[0].Scopes = append(state.APIs[0].Scopes, Scope{Name: BrowserSignInScope, Description: "Sign in."})
	_, in := freshState(t)
	in.APIs = append(in.APIs, otherAPI)
	in.Scopes = map[string]string{BrowserSignInScope: "Sign in."}

	scopes, err := resolveScopes(state, in)

	require.NoError(t, err, "a --scope that matches the description it already has is fine")
	assert.Equal(t, []string{"poppy:browser-sign-in (new)"}, scopes[otherAPI].Changed)
	assert.Empty(t, scopes[shopAPI].Changed)
}

func TestApplyBrowserSignInScope(t *testing.T) {
	tests := []struct {
		name            string
		scopes          map[string]string
		wantDescription string
	}{
		{name: "the default description", wantDescription: BrowserSignInDescription},
		{name: "the description from --scope", scopes: map[string]string{BrowserSignInScope: "Their text"}, wantDescription: "Their text"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state, in := configuredState(t), configuredInputs()
			state.APIs[0].Scopes = state.APIs[0].Scopes[:2]
			delete(state.File.Scopes, BrowserSignInScope)
			in.Scopes = test.scopes
			in.Out = filepath.Join(t.TempDir(), config.DefaultFileName)

			plan, err := Build(state, in)
			require.NoError(t, err)

			h := newApplyHarness(t)
			_, err = h.applier.Apply(t.Context(), state, in, plan)
			require.NoError(t, err)

			assert.Equal(t, []string{"api"}, h.calls, "one API call, and the file is not a call")
			assert.Equal(t, "rs_shop", h.serverID)
			assert.Equal(t, []*managementv3.ResourceServerScope{
				{Value: "orders:read", Description: managementv3.String("See your orders and their status. Read only.")},
				{Value: "checkout", Description: managementv3.String("Place orders and pay with your saved card. Can't be undone.")},
				{Value: BrowserSignInScope, Description: managementv3.String(test.wantDescription)},
			}, h.server.Scopes, "the existing scopes are kept and the new one carries its description")

			written, err := config.Read(in.Out)
			require.NoError(t, err)
			assert.Equal(t, test.wantDescription, written.Scopes[BrowserSignInScope].Description, "it is an ordinary scope in the config file")

			after := *state
			after.APIs = []API{state.APIs[0]}
			after.APIs[0].Scopes = append(append([]Scope{}, state.APIs[0].Scopes...), Scope{Name: BrowserSignInScope, Description: test.wantDescription})
			after.File = written

			again, err := Build(&after, Inputs{
				CompanyDomain: in.CompanyDomain, APIs: in.APIs, App: in.App, Out: in.Out,
				GuestAPIAccess: in.GuestAPIAccess, GuestAllScopes: in.GuestAllScopes, Scopes: test.scopes,
			})
			require.NoError(t, err)
			assert.True(t, again.Empty(), "a rerun reports nothing to change: %v", again.Changes)
		})
	}
}

func TestDeriveAddsNoScope(t *testing.T) {
	state, in := configuredState(t), configuredInputs()
	state.APIs[0].Scopes = state.APIs[0].Scopes[:2]
	delete(state.File.Scopes, BrowserSignInScope)

	cfg, err := Derive(state, in)

	require.NoError(t, err)
	assert.NotContains(t, cfg.Scopes, BrowserSignInScope, "Derive describes the tenant as it is")
}

func TestGuestGrantDoesNotCarryTheBrowserSignInScopeUnlessAsked(t *testing.T) {
	state, in := freshState(t)
	in.GuestAllScopes, in.GuestScopes = false, map[string][]string{shopAPI: {"orders:read"}}

	plan, err := Build(state, in)

	require.NoError(t, err)
	scope, found := grantScope(t, plan, shopAPI)
	assert.True(t, found)
	assert.Equal(t, "orders:read", scope)
}

func TestScopeDescriptionRequiredFix(t *testing.T) {
	scope := func(name string) Scope { return Scope{Name: name} }
	names := func(n int) []Scope {
		all := []Scope{}
		for _, name := range []string{"a:read", "b:read", "c:read", "d:read", "e:read", "f:read", "g:read"}[:n] {
			all = append(all, scope(name))
		}

		return all
	}

	tests := []struct {
		name     string
		apis     []API
		wantText string
	}{
		{
			name: "one scope",
			apis: []API{{Identifier: shopAPI, Scopes: names(1)}},
			wantText: "Every scope offered to agents needs a description. These have none: a:read (" + shopAPI + ").\n" +
				"  FIX  Pass --scope 'a:read=<what this lets an agent do>'.",
		},
		{
			name: "up to five scopes get a line each",
			apis: []API{{Identifier: shopAPI, Scopes: names(2)}},
			wantText: "Every scope offered to agents needs a description. These have none: a:read (" + shopAPI + "), b:read (" + shopAPI + ").\n" +
				"  FIX  Pass --scope for each of them:\n" +
				"         --scope 'a:read=<what this lets an agent do>'\n" +
				"         --scope 'b:read=<what this lets an agent do>'",
		},
		{
			name: "more than five end with a count",
			apis: []API{{Identifier: shopAPI, Scopes: names(7)}},
			wantText: "Every scope offered to agents needs a description. These have none: a:read (" + shopAPI + "), b:read (" + shopAPI + "), c:read (" + shopAPI + "), d:read (" + shopAPI + "), e:read (" + shopAPI + "), f:read (" + shopAPI + "), g:read (" + shopAPI + ").\n" +
				"  FIX  Pass --scope for each of them:\n" +
				"         --scope 'a:read=<what this lets an agent do>'\n" +
				"         --scope 'b:read=<what this lets an agent do>'\n" +
				"         --scope 'c:read=<what this lets an agent do>'\n" +
				"         --scope 'd:read=<what this lets an agent do>'\n" +
				"         --scope 'e:read=<what this lets an agent do>'\n" +
				"         and 2 more",
		},
		{
			name: "a scope on two APIs is listed once in the FIX line",
			apis: []API{{Identifier: shopAPI, Scopes: names(1)}, {Identifier: otherAPI, Scopes: names(1)}},
			wantText: "Every scope offered to agents needs a description. These have none: a:read (" + shopAPI + "), a:read (" + otherAPI + ").\n" +
				"  FIX  Pass --scope 'a:read=<what this lets an agent do>'.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state, in := configuredState(t), configuredInputs()
			state.APIs = test.apis

			err := Check(state, in)

			var input *InputError
			require.ErrorAs(t, err, &input)
			assert.Equal(t, ReasonScopeDescriptionRequired, input.Reason)
			assert.Equal(t, test.wantText, err.Error())
		})
	}
}

func TestScopeQuestions(t *testing.T) {
	undescribed := func(state *State, in *Inputs) {
		state.APIs[0].Scopes[1].Description = ""
	}

	tests := []struct {
		name       string
		change     func(state *State, in *Inputs)
		wantTitles []string
		wantReason string
		answers    []string
		wantScopes map[string]string
	}{
		{
			name:   "nothing is missing",
			change: func(*State, *Inputs) {},
		},
		{
			name:       "a scope with no description is asked about",
			change:     undescribed,
			wantTitles: []string{`Description of checkout`},
			answers:    []string{"Place orders"},
			wantScopes: map[string]string{"checkout": "Place orders"},
		},
		{
			name: "--scope answers it already",
			change: func(state *State, in *Inputs) {
				undescribed(state, in)
				in.Scopes = map[string]string{"checkout": "Place orders"}
			},
		},
		{
			name: "a scope on two APIs is asked about once, naming both",
			change: func(state *State, in *Inputs) {
				undescribed(state, in)
				state.APIs = append(state.APIs, API{Identifier: otherAPI, Scopes: []Scope{{Name: "checkout"}, {Name: BrowserSignInScope}}})
			},
			wantTitles: []string{`Description of checkout`},
			answers:    []string{"Place orders"},
			wantScopes: map[string]string{"checkout": "Place orders"},
		},
		{
			name: "the reserved scope is never asked about",
			change: func(state *State, _ *Inputs) {
				state.APIs[0].Scopes[2].Description = ""
			},
		},
		{
			name: "several scopes are asked about in order",
			change: func(state *State, _ *Inputs) {
				state.APIs[0].Scopes[0].Description = ""
				state.APIs[0].Scopes[1].Description = ""
			},
			wantTitles: []string{`Description of orders:read`, `Description of checkout`},
			answers:    []string{"See orders", "Place orders"},
			wantScopes: map[string]string{"orders:read": "See orders", "checkout": "Place orders"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state, in := configuredState(t), configuredInputs()
			test.change(state, &in)

			questions := ScopeQuestions(state, in)

			titles := make([]string, len(questions))
			for i, question := range questions {
				titles[i] = question.Title
			}

			if len(test.wantTitles) == 0 {
				assert.Empty(t, questions)

				return
			}

			assert.Equal(t, test.wantTitles, titles)
			assert.Contains(t, questions[0].Reason, test.wantReason)

			for i, question := range questions {
				require.NoError(t, question.Answer(&in, test.answers[i]))
			}

			assert.Equal(t, test.wantScopes, in.Scopes, "the answers are stored as if --scope had been passed")
			require.NoError(t, Check(state, in), "with the answers, nothing is missing")
			assert.Empty(t, ScopeQuestions(state, in))
		})
	}

	t.Run("an empty answer is refused", func(t *testing.T) {
		state, in := configuredState(t), configuredInputs()
		undescribed(state, &in)

		questions := ScopeQuestions(state, in)
		require.Len(t, questions, 1)

		assert.Error(t, questions[0].Answer(&in, "   "))
		assert.Empty(t, in.Scopes)
	})
}
