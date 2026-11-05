package setup

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/poppy/config"
)

// configuredState is a tenant that setup has already been run against, with the
// config file it wrote.
func configuredState(t *testing.T) *State {
	t.Helper()

	file, err := config.Read(filepath.Join("..", "config", "testdata", "auth0-poppy.golden.json"))
	require.NoError(t, err)

	// Setup has added the reserved scope to the API, and the file lists it with
	// its default hints.
	return &State{
		Tenant: Tenant{
			CIMD:                     true,
			IssParameter:             true,
			ResourceParameterProfile: "compatibility",
			DefaultAudience:          shopAPI,
			FriendlyName:             "Shop",
			CompanyDomains:           []string{"https://shop.example"},
		},
		Domain:       tenantURL,
		CustomDomain: "auth.shop.example",
		APIs: []API{{
			ID:         "rs_shop",
			Identifier: shopAPI,
			Name:       "Shop API",
			Scopes: []Scope{
				{Name: "orders:read", Description: "See your orders and their status. Read only."},
				{Name: "checkout", Description: "Place orders and pay with your saved card. Can't be undone."},
				{Name: BrowserSignInScope, Description: BrowserSignInDescription},
			},
			AllowOfflineAccess: true,
			AgentSubjectClaims: "auth0-v1",
			Poppy:              true,
			GuestPolicy:        "require_client_grant",
		}},
		App:           App{ClientID: shopApp, Name: "Shop Web", FirstParty: true, LoginURI: "https://shop.example/login", BrowserJoin: true},
		HasGuestGrant: map[string]bool{shopAPI: true},
		File:          file,
	}
}

func configuredInputs() Inputs {
	return Inputs{
		CompanyDomain:  "shop.example",
		APIs:           []string{shopAPI},
		App:            "Shop Web",
		Out:            config.DefaultFileName,
		GuestAPIAccess: true,
		GuestAllScopes: true,
	}
}

func TestFlagsParse(t *testing.T) {
	tests := []struct {
		name    string
		flags   Flags
		want    func(t *testing.T, in Inputs)
		wantErr string
	}{
		{
			name: "nothing given",
			want: func(t *testing.T, in Inputs) {
				assert.Empty(t, in.CompanyDomain)
				assert.Empty(t, in.APIs)
				assert.Empty(t, in.Scopes)
				assert.Empty(t, in.Hints)
			},
		},
		{
			name: "every flag",
			flags: Flags{
				CompanyDomain:  "Shop.Example",
				APIs:           []string{shopAPI, " ", shopAPI, "rs_other"},
				APIURLs:        []string{shopAPI + "=" + shopAPI + "/openapi.json"},
				App:            " Shop Web ",
				Scopes:         []string{`checkout=Place orders. Can't be undone. Use a=b if you must.`},
				Hints:          []string{"checkout=changes:irreversible,personal_data:true"},
				Name:           "Shop",
				LoginURI:       "https://shop.example/login",
				Out:            "out.json",
				GuestAPIAccess: true,
				ForceConfig:    true,
			},
			want: func(t *testing.T, in Inputs) {
				assert.Equal(t, Inputs{
					CompanyDomain:  "shop.example",
					APIs:           []string{shopAPI, "rs_other"},
					APIURLs:        map[string]string{shopAPI: shopAPI + "/openapi.json"},
					App:            "Shop Web",
					Scopes:         map[string]string{"checkout": "Place orders. Can't be undone. Use a=b if you must."},
					Hints:          map[string]config.Hints{"checkout": {Changes: "irreversible", PersonalData: true}},
					Name:           "Shop",
					LoginURI:       "https://shop.example/login",
					Out:            "out.json",
					GuestAPIAccess: true,
					ForceConfig:    true,
				}, in)
			},
		},
		{name: "company domain with a scheme", flags: Flags{CompanyDomain: "https://shop.example"}, wantErr: "--company-domain: \"https://shop.example\" must be a host name such as shop.example, without a scheme"},
		{name: "company domain with an http scheme", flags: Flags{CompanyDomain: "http://shop.example"}, wantErr: "without a scheme"},
		{name: "company domain with a path", flags: Flags{CompanyDomain: "shop.example/orders"}, wantErr: "--company-domain: the domain \"shop.example/orders\" must not include a path"},
		{name: "company domain with a port", flags: Flags{CompanyDomain: "shop.example:8443"}, wantErr: "must not include a port"},
		{name: "company domain that is not a host", flags: Flags{CompanyDomain: "shop_example"}, wantErr: "not a valid host name"},
		{name: "login uri over http", flags: Flags{LoginURI: "http://shop.example/login"}, wantErr: "--login-uri: \"http://shop.example/login\" must be an https URL"},
		{name: "api url without a name", flags: Flags{APIURLs: []string{"https://api.shop.example/openapi.json"}}, wantErr: "--api-url: \"https://api.shop.example/openapi.json\" must be written <name>=<value>"},
		{name: "api url that is not https", flags: Flags{APIURLs: []string{shopAPI + "=http://api.shop.example/openapi.json"}}, wantErr: "--api-url"},
		{name: "scope without a description", flags: Flags{Scopes: []string{"checkout="}}, wantErr: "--scope: the description must not be empty"},
		{name: "scope without an equals sign", flags: Flags{Scopes: []string{"checkout"}}, wantErr: "--scope: \"checkout\" must be written <name>=<value>"},
		{name: "hint without an equals sign", flags: Flags{Hints: []string{"checkout"}}, wantErr: "--hint"},
		{name: "hint with an unknown key", flags: Flags{Hints: []string{"checkout=reach:wide"}}, wantErr: "unknown hint \"reach\""},
		{name: "hint with a bad boolean", flags: Flags{Hints: []string{"checkout=changes:none,personal_data:maybe"}}, wantErr: "personal_data of checkout must be true or false"},
		{name: "hint without changes", flags: Flags{Hints: []string{"checkout=personal_data:true"}}, wantErr: "checkout needs both changes and personal_data"},
		{name: "hint with an unknown changes value", flags: Flags{Hints: []string{"checkout=changes:dangerous,personal_data:true"}}, wantErr: "changes of checkout must be one of none, reversible, irreversible, got \"dangerous\""},
		{name: "hint without personal data", flags: Flags{Hints: []string{"checkout=changes:none"}}, wantErr: "checkout needs both changes and personal_data"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			in, err := test.flags.Parse()

			if test.wantErr != "" {
				var input *InputError
				require.ErrorAs(t, err, &input)
				assert.Equal(t, "invalid_flag", input.Reason)
				assert.Contains(t, err.Error(), test.wantErr)
				return
			}

			require.NoError(t, err)
			test.want(t, in)
		})
	}
}

func TestHintWithUnknownChangesHasAFix(t *testing.T) {
	_, err := Flags{Hints: []string{"checkout=changes:dangerous,personal_data:true"}}.Parse()

	var input *InputError
	require.ErrorAs(t, err, &input)
	assert.Equal(t, "Use --hint checkout=changes:<none|reversible|irreversible>,personal_data:<true|false>", input.Fix)
	assert.Contains(t, err.Error(), "\n  FIX  Use --hint checkout=")
}

func TestInputErrorText(t *testing.T) {
	assert.Equal(t, "boom", (&InputError{Message: "boom"}).Error())
	assert.Equal(t, "boom\n  FIX  do this", (&InputError{Message: "boom", Fix: "do this"}).Error())
}

func TestRequiredQuestions(t *testing.T) {
	t.Run("everything given", func(t *testing.T) {
		assert.Empty(t, RequiredQuestions(configuredInputs()))
	})

	t.Run("nothing given", func(t *testing.T) {
		var in Inputs

		questions := RequiredQuestions(in)

		flags := make([]string, len(questions))
		for i, q := range questions {
			flags[i] = q.Flag
		}
		assert.Equal(t, []string{"--company-domain", "--api", "--app"}, flags)

		require.NoError(t, questions[0].Answer(&in, " Shop.Example "))
		require.NoError(t, questions[1].Answer(&in, shopAPI))
		require.NoError(t, questions[2].Answer(&in, " Shop Web "))
		assert.Equal(t, Inputs{CompanyDomain: "shop.example", APIs: []string{shopAPI}, App: "Shop Web"}, in)

		assert.Empty(t, RequiredQuestions(in))
	})

	t.Run("a bad company domain is refused", func(t *testing.T) {
		var in Inputs

		err := RequiredQuestions(in)[0].Answer(&in, "shop.example:8443")

		assert.ErrorContains(t, err, "must not include a port")
		assert.Empty(t, in.CompanyDomain)
	})
}

func TestQuestions(t *testing.T) {
	unconfigured := func(t *testing.T) (*State, Inputs) {
		t.Helper()

		state := configuredState(t)
		state.File = nil
		state.App.LoginURI = ""

		in := configuredInputs()

		return state, in
	}

	t.Run("nothing is missing", func(t *testing.T) {
		assert.Empty(t, Questions(configuredState(t), configuredInputs()))
	})

	t.Run("the OpenAPI URL defaults to the origin and the login route follows", func(t *testing.T) {
		state, in := unconfigured(t)

		questions := Questions(state, in)

		require.Len(t, questions, 1)
		assert.Equal(t, "--api-url "+shopAPI+"=<openapi url>", questions[0].Flag)
		assert.Equal(t, shopAPI+"/openapi.json", questions[0].Default)

		require.NoError(t, questions[0].Answer(&in, shopAPI+"/docs/openapi.json"))
		assert.Equal(t, map[string]string{shopAPI: shopAPI + "/docs/openapi.json"}, in.APIURLs)

		questions = Questions(state, in)
		require.Len(t, questions, 1)
		assert.Equal(t, "--login-uri", questions[0].Flag)

		require.NoError(t, questions[0].Answer(&in, "https://shop.example/auth/login"))
		assert.Equal(t, "https://shop.example/auth/login", in.LoginURI)
		assert.Empty(t, Questions(state, in))
	})

	t.Run("the existing config file answers for the URL and the login route is kept when https", func(t *testing.T) {
		state, in := unconfigured(t)
		state.File = configuredState(t).File
		state.App.LoginURI = "https://shop.example/login"

		assert.Empty(t, Questions(state, in))
	})

	t.Run("a login route that is not https is asked for", func(t *testing.T) {
		state := configuredState(t)
		state.App.LoginURI = "http://shop.example/login"

		questions := Questions(state, configuredInputs())

		require.Len(t, questions, 1)
		assert.Equal(t, "--login-uri", questions[0].Flag)

		in := configuredInputs()
		assert.ErrorContains(t, questions[0].Answer(&in, "http://shop.example/login"), "must be an https URL")
	})

	t.Run("--login-uri answers the question", func(t *testing.T) {
		state := configuredState(t)
		state.App.LoginURI = ""
		in := configuredInputs()
		in.LoginURI = "https://shop.example/login"

		assert.Empty(t, Questions(state, in))
	})

	t.Run("an identifier that is not a URL has no default for the URL", func(t *testing.T) {
		state, in := unconfigured(t)
		state.APIs[0].Identifier = "urn:shop:api"

		questions := Questions(state, in)

		require.Len(t, questions, 1)
		assert.Equal(t, "--api-url urn:shop:api=<openapi url>", questions[0].Flag)
		assert.Empty(t, questions[0].Default)

		require.NoError(t, questions[0].Answer(&in, shopAPI+"/openapi.json"))
		assert.Equal(t, map[string]string{"urn:shop:api": shopAPI + "/openapi.json"}, in.APIURLs)
		assert.ErrorContains(t, questions[0].Answer(&in, "http://api.shop.example/openapi.json"), "must be an https URL")
	})
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name       string
		change     func(state *State, in *Inputs)
		wantReason string
		wantText   string
	}{
		{name: "a configured tenant passes", change: func(*State, *Inputs) {}},
		{
			name:       "the web app is not first-party",
			change:     func(state *State, _ *Inputs) { state.App.FirstParty = false },
			wantReason: "app_not_first_party",
			wantText:   "The browser join needs a first-party web app. Shop Web is not first-party.",
		},
		{
			name: "--api-url names an unknown API",
			change: func(_ *State, in *Inputs) {
				in.APIURLs = map[string]string{"https://other.example": "https://other.example/o.json"}
			},
			wantReason: "invalid_flag",
			wantText:   "--api-url: https://other.example is not one of the APIs. The APIs are: " + shopAPI,
		},
		{
			name: "a scope has no description",
			change: func(state *State, _ *Inputs) {
				state.APIs[0].Scopes[1].Description = ""
			},
			wantReason: "scope_description_required",
			wantText:   "These have none: checkout (" + shopAPI + ").\n  FIX  Pass --scope 'checkout=<what this lets an agent do>'.",
		},
		{
			name: "--scope describes a scope that has none",
			change: func(state *State, in *Inputs) {
				state.APIs[0].Scopes[1].Description = ""
				in.Scopes = map[string]string{"checkout": "Place orders."}
			},
		},
		{
			name: "--scope repeats the description a scope has",
			change: func(_ *State, in *Inputs) {
				in.Scopes = map[string]string{"checkout": "Place orders and pay with your saved card. Can't be undone."}
			},
		},
		{
			name:       "--scope would edit a description",
			change:     func(_ *State, in *Inputs) { in.Scopes = map[string]string{"checkout": "Pay."} },
			wantReason: "scope_description_exists",
			wantText:   "Scope checkout of " + shopAPI + " already has a description, and setup never edits one.\n  FIX  Remove --scope checkout",
		},
		{
			name:     "--scope adds a scope to the only API",
			change:   func(_ *State, in *Inputs) { in.Scopes = map[string]string{"refunds": "Refund an order."} },
			wantText: "",
		},
		{
			name: "--scope names a scope that no API has, with several APIs",
			change: func(state *State, in *Inputs) {
				state.APIs = append(state.APIs, API{Identifier: "https://other.example"})
				in.Scopes = map[string]string{"refunds": "Refund an order."}
			},
			wantReason: "scope_not_found",
			wantText:   "Scope refunds is not a scope of any of the APIs",
		},
		{
			name:       "--hint names a scope that no API has",
			change:     func(_ *State, in *Inputs) { in.Hints = map[string]config.Hints{"refunds": {Changes: "none"}} },
			wantReason: "invalid_flag",
			wantText:   "--hint: refunds is not a scope of the APIs",
		},
		{
			name:     "--hint names a scope added by --scope",
			wantText: "",
			change: func(_ *State, in *Inputs) {
				in.Scopes = map[string]string{"refunds": "Refund an order."}
				in.Hints = map[string]config.Hints{"refunds": {Changes: "irreversible"}}
			},
		},
		{
			name: "the tenant lists the most company domains and the domain is one of them",
			change: func(state *State, _ *Inputs) {
				state.Tenant.CompanyDomains = []string{"https://a.example", "https://b.example", "https://c.example", "https://d.example", "https://Shop.Example"}
			},
		},
		{
			name: "the tenant lists the most company domains",
			change: func(state *State, _ *Inputs) {
				state.Tenant.CompanyDomains = []string{"https://a.example", "https://b.example", "https://c.example", "https://d.example", "https://e.example"}
			},
			wantReason: "company_domains_full",
			wantText:   "The tenant already lists 5 company domains, which is the most it can hold, so https://shop.example cannot be added.",
		},
		{
			name: "the tenant lists four company domains",
			change: func(state *State, _ *Inputs) {
				state.Tenant.CompanyDomains = []string{"https://a.example", "https://b.example", "https://c.example", "https://d.example"}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state, in := configuredState(t), configuredInputs()
			test.change(state, &in)

			err := Check(state, in)

			if test.wantReason == "" {
				require.NoError(t, err)
				return
			}

			var input *InputError
			require.ErrorAs(t, err, &input)
			assert.Equal(t, test.wantReason, input.Reason)
			assert.Contains(t, err.Error(), test.wantText)
		})
	}
}

func TestFlagsParseGuestScopes(t *testing.T) {
	tests := []struct {
		name      string
		flags     Flags
		wantNames []string
		wantAll   bool
		wantFlag  string
	}{
		{name: "names are trimmed and listed once", flags: Flags{GuestAPIAccess: true, GuestScopes: []string{" orders:read", "", "checkout", "orders:read"}}, wantNames: []string{"orders:read", "checkout"}},
		{name: "every scope", flags: Flags{GuestAPIAccess: true, GuestAllScopes: true}, wantAll: true},
		{name: "neither flag", flags: Flags{GuestAPIAccess: true}},
		{name: "names without the guest grant", flags: Flags{GuestScopes: []string{"orders:read"}}, wantFlag: "--guest-scopes"},
		{name: "every scope without the guest grant", flags: Flags{GuestAllScopes: true}, wantFlag: "--guest-all-scopes"},
		{name: "only blank names without the guest grant", flags: Flags{GuestScopes: []string{" ", ""}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			in, err := test.flags.Parse()

			if test.wantFlag != "" {
				var input *InputError
				require.ErrorAs(t, err, &input)
				assert.Equal(t, "invalid_flag", input.Reason)
				assert.Contains(t, input.Message, test.wantFlag+": ")
				assert.Equal(t, "Add --guest-api-access.", input.Fix)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.wantNames, in.GuestScopeNames)
			assert.Equal(t, test.wantAll, in.GuestAllScopes)
			assert.Nil(t, in.GuestScopes, "the scopes per API are chosen later")
		})
	}
}

// twoAPIState is a fresh tenant with a second API that has one scope in common
// with the first.
func twoAPIState(t *testing.T) *State {
	t.Helper()

	state, _ := freshState(t)
	state.APIs = append(state.APIs, API{
		ID:         "rs_other",
		Identifier: "https://other.example",
		Scopes:     []Scope{{Name: "orders:read", Description: "Read."}, {Name: "reports", Description: "See reports."}},
	})

	return state
}

func TestChooseGuestScopes(t *testing.T) {
	tests := []struct {
		name       string
		state      func(t *testing.T) *State
		change     func(in *Inputs)
		want       map[string][]string
		wantErr    error
		wantReason string
		wantText   string
	}{
		{
			name:   "the guest grant is not asked for",
			state:  func(t *testing.T) *State { s, _ := freshState(t); return s },
			change: func(in *Inputs) { in.GuestAPIAccess, in.GuestAllScopes = false, false },
		},
		{
			name:    "no scope flag",
			state:   func(t *testing.T) *State { s, _ := freshState(t); return s },
			change:  func(in *Inputs) { in.GuestAllScopes = false },
			wantErr: ErrGuestScopesRequired,
		},
		{
			name: "no scope flag, but every API has a guest grant",
			state: func(t *testing.T) *State {
				s, _ := freshState(t)
				s.HasGuestGrant = map[string]bool{shopAPI: true}

				return s
			},
			change: func(in *Inputs) { in.GuestAllScopes = false },
		},
		{
			name: "no scope flag, and one of two APIs lacks a guest grant",
			state: func(t *testing.T) *State {
				s := twoAPIState(t)
				s.HasGuestGrant = map[string]bool{shopAPI: true}

				return s
			},
			change:  func(in *Inputs) { in.GuestAllScopes = false },
			wantErr: ErrGuestScopesRequired,
		},
		{
			name:  "every scope leaves the choice to the plan",
			state: func(t *testing.T) *State { s, _ := freshState(t); return s },
		},
		{
			name:   "scopes named on one API",
			state:  func(t *testing.T) *State { s, _ := freshState(t); return s },
			change: func(in *Inputs) { in.GuestAllScopes, in.GuestScopeNames = false, []string{"orders:read"} },
			want:   map[string][]string{shopAPI: {"orders:read"}},
		},
		{
			name:   "scopes named on two APIs that both define them",
			state:  twoAPIState,
			change: func(in *Inputs) { in.GuestAllScopes, in.GuestScopeNames = false, []string{"orders:read"} },
			want:   map[string][]string{shopAPI: {"orders:read"}, "https://other.example": {"orders:read"}},
		},
		{
			name:       "a scope one of the APIs lacks names that API",
			state:      twoAPIState,
			change:     func(in *Inputs) { in.GuestAllScopes, in.GuestScopeNames = false, []string{"orders:read", "checkout"} },
			wantReason: "unknown_scope",
			wantText:   `scope "checkout" is not defined on https://other.example`,
		},
		{
			name:       "a scope no API defines",
			state:      func(t *testing.T) *State { s, _ := freshState(t); return s },
			change:     func(in *Inputs) { in.GuestAllScopes, in.GuestScopeNames = false, []string{"nope"} },
			wantReason: "unknown_scope",
			wantText:   `scope "nope" is not defined on ` + shopAPI,
		},
		{
			name:  "a scope that --scope adds to the only API",
			state: func(t *testing.T) *State { s, _ := freshState(t); return s },
			change: func(in *Inputs) {
				in.GuestAllScopes, in.GuestScopeNames, in.Scopes = false, []string{"refunds"}, map[string]string{"refunds": "Return an order."}
			},
			want: map[string][]string{shopAPI: {"refunds"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := test.state(t)
			_, in := freshState(t)

			if test.change != nil {
				test.change(&in)
			}

			got, err := ChooseGuestScopes(state, in)

			switch {
			case test.wantErr != nil:
				assert.ErrorIs(t, err, test.wantErr)
			case test.wantReason != "":
				var input *InputError
				require.ErrorAs(t, err, &input)
				assert.Equal(t, test.wantReason, input.Reason)
				assert.Contains(t, input.Message, test.wantText)
				assert.NotEmpty(t, input.Fix)
			default:
				require.NoError(t, err)
				assert.Equal(t, test.want, got)
			}
		})
	}
}

func TestGuestOffers(t *testing.T) {
	t.Run("lists the scopes of the APIs without a guest grant, with the hints", func(t *testing.T) {
		state := twoAPIState(t)
		state.HasGuestGrant = map[string]bool{"https://other.example": true}
		_, in := freshState(t)
		in.Hints = map[string]config.Hints{"checkout": {Changes: "irreversible", PersonalData: true}}

		offers, err := GuestOffers(state, in)

		require.NoError(t, err)
		require.Len(t, offers, 1)
		assert.Equal(t, shopAPI, offers[0].Audience)
		assert.Equal(t, []GuestScope{
			{Name: "orders:read", Description: "See your orders and their status. Read only."},
			{Name: "checkout", Description: "Place orders and pay with your saved card. Can't be undone.", Hints: &config.Hints{Changes: "irreversible", PersonalData: true}},
			{Name: BrowserSignInScope, Description: BrowserSignInDescription},
		}, offers[0].Scopes, "the reserved scope is offered like any other")
	})

	t.Run("hints come from the config file when none is given", func(t *testing.T) {
		state := configuredState(t)
		state.HasGuestGrant = map[string]bool{}
		hints := state.File.Scopes["checkout"].Hints
		require.NotNil(t, hints, "the golden file holds hints for checkout")

		offers, err := GuestOffers(state, configuredInputs())

		require.NoError(t, err)
		require.Len(t, offers, 1)
		assert.Equal(t, hints, offers[0].Scopes[1].Hints)
	})

	t.Run("a scope without a description stops first", func(t *testing.T) {
		state, in := freshState(t)
		state.APIs[0].Scopes[1].Description = ""

		_, err := GuestOffers(state, in)

		var input *InputError
		require.ErrorAs(t, err, &input)
		assert.Equal(t, "scope_description_required", input.Reason)
	})
}

func TestAllGuestGrants(t *testing.T) {
	state := twoAPIState(t)
	assert.False(t, state.AllGuestGrants())

	state.HasGuestGrant = map[string]bool{"https://other.example": true}
	assert.False(t, state.AllGuestGrants())

	for _, api := range state.APIs {
		state.HasGuestGrant[api.Identifier] = true
	}
	assert.True(t, state.AllGuestGrants())

	assert.False(t, (&State{}).AllGuestGrants())
}

func TestHintScopes(t *testing.T) {
	// The file of the configured state hints orders:read and checkout.
	withoutFile := func(state *State, _ *Inputs) { state.File = nil }

	tests := []struct {
		name        string
		change      func(state *State, in *Inputs)
		addReserved bool
		want        []string
	}{
		{name: "the file hints every scope", change: func(*State, *Inputs) {}},
		{name: "no file, every scope is asked about but the reserved one", change: withoutFile, want: []string{"orders:read", "checkout"}},
		{
			name: "a hint from the file is not asked again",
			change: func(state *State, in *Inputs) {
				withoutFile(state, in)
				state.File = &config.Config{Scopes: map[string]config.Scope{"orders:read": {Hints: &config.Hints{Changes: "none"}}}}
			},
			want: []string{"checkout"},
		},
		{
			name: "--hint answers it already",
			change: func(state *State, in *Inputs) {
				withoutFile(state, in)
				in.Hints = map[string]config.Hints{"checkout": {Changes: "irreversible"}}
			},
			want: []string{"orders:read"},
		},
		{
			name: "--hint on every scope leaves nothing to ask",
			change: func(state *State, in *Inputs) {
				withoutFile(state, in)
				in.Hints = map[string]config.Hints{"checkout": {Changes: "none"}, "orders:read": {Changes: "none"}}
			},
		},
		{
			name: "a scope on two APIs is asked once, naming both",
			change: func(state *State, in *Inputs) {
				withoutFile(state, in)
				state.APIs = append(state.APIs, API{Identifier: otherAPI, Scopes: []Scope{{Name: "checkout", Description: "Place orders."}}})
				in.APIs = append(in.APIs, otherAPI)
			},
			want: []string{"orders:read", "checkout"},
		},
		{
			name: "a scope the run adds is asked about",
			change: func(state *State, in *Inputs) {
				withoutFile(state, in)
				in.Scopes = map[string]string{"refunds": "Refund an order you placed."}
			},
			addReserved: true,
			want:        []string{"orders:read", "checkout", "refunds"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state, in := configuredState(t), configuredInputs()
			test.change(state, &in)

			var names []string
			for _, scope := range HintScopes(state, in, test.addReserved) {
				names = append(names, scope.Name)
				assert.NotEqual(t, BrowserSignInScope, scope.Name, "the reserved scope has a default")
			}

			assert.Equal(t, test.want, names)
		})
	}

	t.Run("a scope on two APIs lists both", func(t *testing.T) {
		state, in := configuredState(t), configuredInputs()
		state.File = nil
		state.APIs = append(state.APIs, API{Identifier: otherAPI, Scopes: []Scope{{Name: "checkout", Description: "Place orders."}}})
		in.APIs = append(in.APIs, otherAPI)

		for _, scope := range HintScopes(state, in, false) {
			if scope.Name == "checkout" {
				assert.Equal(t, []string{shopAPI, otherAPI}, scope.APIs)
				assert.NotEmpty(t, scope.Description)
			}
		}
	})
}

func TestQuestionsAreFullyWorded(t *testing.T) {
	undescribed := configuredState(t)
	undescribed.APIs[0].Scopes[0].Description = ""

	bare := configuredState(t)
	bare.File = nil
	bare.App.LoginURI = ""

	tests := []struct {
		name      string
		questions []Question
		wantTitle []string
	}{
		{name: "required", questions: RequiredQuestions(Inputs{}), wantTitle: []string{"Company domain", "API", "Web app"}},
		{name: "OpenAPI URL", questions: Questions(bare, Inputs{}), wantTitle: []string{"OpenAPI document of " + shopAPI}},
		{
			name:      "login route",
			questions: Questions(bare, Inputs{APIURLs: map[string]string{shopAPI: shopAPI + "/openapi.json"}}),
			wantTitle: []string{"Login route of " + bare.App.Name},
		},
		{name: "scope description", questions: ScopeQuestions(undescribed, configuredInputs()), wantTitle: []string{"Description of orders:read"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			titles := make([]string, len(test.questions))

			for i, question := range test.questions {
				titles[i] = question.Title

				assert.NotEmpty(t, question.Flag, "the flag that gives the answer")
				assert.NotEmpty(t, question.Reason, "one sentence on why it is asked")
				assert.NotEmpty(t, question.Prompt, "the label of the place to type")
				assert.True(t, strings.HasSuffix(question.Reason, "."), question.Reason)
			}

			assert.Equal(t, test.wantTitle, titles)
		})
	}
}
