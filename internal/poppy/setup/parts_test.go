package setup

import (
	"slices"
	"testing"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/auth0/auth0-cli/internal/poppy/config"
)

func TestFlagsParseParts(t *testing.T) {
	tests := []struct {
		name    string
		flags   Flags
		wantErr string
		wantFix string
		noAPI   bool
		noWeb   bool
	}{
		{name: "--no-api alone", flags: Flags{NoAPI: true}, noAPI: true},
		{name: "--no-web alone", flags: Flags{NoWeb: true}, noWeb: true},
		{name: "--no-api with the web flags", flags: Flags{NoAPI: true, App: "Shop Web", LoginURI: "https://shop.example/login", WebUnscoped: ptr(true)}, noAPI: true},
		{name: "--no-web with the API flags", flags: Flags{NoWeb: true, APIs: []string{shopAPI}, Scopes: []string{"checkout=Place orders."}}, noWeb: true},
		{
			name:    "both",
			flags:   Flags{NoAPI: true, NoWeb: true},
			wantErr: "--no-api: it cannot be used with --no-web: there would be nothing to set up",
			wantFix: "Drop --no-api or --no-web.",
		},
		{name: "--no-api with --api", flags: Flags{NoAPI: true, APIs: []string{shopAPI}}, wantErr: "--no-api: it turns that part off, so it cannot be used with --api", wantFix: "Drop --no-api, or drop --api."},
		{name: "--no-api with --api-url", flags: Flags{NoAPI: true, APIURLs: []string{shopAPI + "=" + shopAPI + "/openapi.json"}}, wantErr: "cannot be used with --api-url"},
		{name: "--no-api with --scope", flags: Flags{NoAPI: true, Scopes: []string{"checkout=Place orders."}}, wantErr: "cannot be used with --scope"},
		{name: "--no-api with --hint", flags: Flags{NoAPI: true, Hints: []string{"checkout=changes:none,personal_data:false"}}, wantErr: "cannot be used with --hint"},
		{name: "--no-web with --app", flags: Flags{NoWeb: true, App: "Shop Web"}, wantErr: "--no-web: it turns that part off, so it cannot be used with --app", wantFix: "Drop --no-web, or drop --app."},
		{name: "--no-web with --login-uri", flags: Flags{NoWeb: true, LoginURI: "https://shop.example/login"}, wantErr: "cannot be used with --login-uri"},
		{name: "--no-web with --web-unscoped", flags: Flags{NoWeb: true, WebUnscoped: ptr(false)}, wantErr: "cannot be used with --web-unscoped"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			in, err := test.flags.Parse()

			if test.wantErr == "" {
				require.NoError(t, err)
				assert.Equal(t, test.noAPI, in.NoAPI)
				assert.Equal(t, test.noWeb, in.NoWeb)

				return
			}

			var input *InputError
			require.ErrorAs(t, err, &input)
			assert.Equal(t, "invalid_flag", input.Reason)
			assert.Contains(t, input.Message, test.wantErr)

			if test.wantFix != "" {
				assert.Equal(t, test.wantFix, input.Fix)
			} else {
				assert.NotEmpty(t, input.Fix, "every rejection has a FIX line")
			}
		})
	}
}

func TestRequiredQuestionsParts(t *testing.T) {
	type asked struct{ flag, part string }

	tests := []struct {
		name string
		in   Inputs
		want []asked
	}{
		{name: "both parts on", want: []asked{{"--company-domain", ""}, {"--api", PartAPI}, {"--app", PartWeb}}},
		{name: "no API part", in: Inputs{NoAPI: true}, want: []asked{{"--company-domain", ""}, {"--app", PartWeb}}},
		{name: "no web part", in: Inputs{NoWeb: true}, want: []asked{{"--company-domain", ""}, {"--api", PartAPI}}},
		{name: "no API part, everything else given", in: Inputs{NoAPI: true, CompanyDomain: "shop.example", App: "Shop Web"}},
		{name: "no web part, everything else given", in: Inputs{NoWeb: true, CompanyDomain: "shop.example", APIs: []string{shopAPI}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got []asked
			for _, q := range RequiredQuestions(test.in) {
				got = append(got, asked{q.Flag, q.Part})
			}

			assert.Equal(t, test.want, got)
		})
	}
}

func TestUseConfigFile(t *testing.T) {
	file := func(authOff, webOff bool) *config.Config {
		return &config.Config{Auth: config.Auth{Disabled: authOff}, Web: config.Web{Disabled: webOff}}
	}

	tests := []struct {
		name      string
		in        Inputs
		file      *config.Config
		wantNoAPI bool
		wantNoWeb bool
		wantErr   bool
		asksAPI   bool
		asksWeb   bool
	}{
		{name: "no file", asksAPI: true, asksWeb: true},
		{name: "a file that turns nothing off", file: file(false, false), asksAPI: true, asksWeb: true},
		{name: "auth.disabled answers the API question", file: file(true, false), wantNoAPI: true, asksWeb: true},
		{name: "web.disabled answers the web question", file: file(false, true), wantNoWeb: true, asksAPI: true},
		{name: "--api wins over auth.disabled", in: Inputs{APIs: []string{shopAPI}}, file: file(true, false), asksWeb: true},
		{name: "--scope wins over auth.disabled", in: Inputs{Scopes: map[string]string{"a": "b"}}, file: file(true, false), asksWeb: true},
		{name: "--app wins over web.disabled", in: Inputs{App: "Shop Web"}, file: file(false, true), asksAPI: true},
		{name: "--login-uri wins over web.disabled", in: Inputs{LoginURI: "https://shop.example/login"}, file: file(false, true), asksAPI: true},
		{name: "both off in the file", file: file(true, true), wantNoAPI: true, wantNoWeb: true, wantErr: true},
		{name: "a file that lists APIs answers the API question", file: &config.Config{APIs: []config.API{{Type: "openapi", URL: "https://api.shop.example/openapi.json"}}}, asksWeb: true},
		{name: "a file with the join address answers the web question", file: &config.Config{Web: config.Web{BrowserSessionEndpoint: "https://auth.shop.example/poppy/c/browser-session"}}, asksAPI: true},
		{name: "a file with both answers both questions", file: &config.Config{APIs: []config.API{{URL: "https://api.shop.example/openapi.json"}}, Web: config.Web{BrowserSessionEndpoint: "https://auth.shop.example/poppy/c/browser-session"}}},
		{name: "auth.disabled still answers no when the file lists APIs", file: &config.Config{Auth: config.Auth{Disabled: true}, APIs: []config.API{{URL: "https://api.shop.example/openapi.json"}}}, wantNoAPI: true, asksWeb: true},
		{name: "web.disabled still answers no when the file has the join address", file: &config.Config{Web: config.Web{Disabled: true, BrowserSessionEndpoint: "https://auth.shop.example/poppy/c/browser-session"}}, wantNoWeb: true, asksAPI: true},
		{name: "both off in the file, --app turns the web part on", in: Inputs{App: "Shop Web"}, file: file(true, true), wantNoAPI: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			in := test.in

			err := in.UseConfigFile(test.file)

			if test.wantErr {
				var input *InputError
				require.ErrorAs(t, err, &input)
				assert.Equal(t, "nothing_to_set_up", input.Reason)
				assert.NotEmpty(t, input.Fix)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, test.wantNoAPI, in.NoAPI, "NoAPI")
			assert.Equal(t, test.wantNoWeb, in.NoWeb, "NoWeb")
			assert.Equal(t, test.asksAPI, in.AsksAPI(), "AsksAPI")
			assert.Equal(t, test.asksWeb, in.AsksWeb(), "AsksWeb")
		})
	}
}

func TestQuestionsWithTheWebPartOff(t *testing.T) {
	state, in := freshState(t)
	state.App.LoginURI = ""
	in.APIURLs = map[string]string{shopAPI: shopAPI + "/openapi.json"}

	require.Len(t, Questions(state, in), 1, "the Application Login URI is asked with the web part on")

	in.NoWeb, in.App = true, ""

	assert.Empty(t, Questions(state, in), "and not with it off")
}

func TestCheckWithThePartsOff(t *testing.T) {
	t.Run("the app is not checked without the web part", func(t *testing.T) {
		state, in := configuredState(t), configuredInputs()
		state.App = App{}
		in.NoWeb, in.App = true, ""

		require.NoError(t, Check(state, in))

		in.NoWeb = false
		assert.ErrorContains(t, Check(state, in), "first-party")
	})
}

func TestReadyWithThePartsOff(t *testing.T) {
	state, in := configuredState(t), configuredInputs()
	state.App.BrowserJoin = false
	state.APIs[0].Poppy = false

	err := Ready(state, in)
	var notReady *NotReadyError
	require.ErrorAs(t, err, &notReady)
	assert.Len(t, notReady.Problems, 2)

	in.NoWeb = true
	require.ErrorAs(t, Ready(state, in), &notReady)
	assert.Equal(t, []string{"API " + shopAPI + ": use_for_personal_agent_protocol is off"}, notReady.Problems)

	state.APIs = nil
	in.NoAPI, in.NoWeb = true, false
	require.ErrorAs(t, Ready(state, in), &notReady)
	assert.Equal(t, []string{"app Shop Web: allow_personal_agent_browser_join is off"}, notReady.Problems)

	state.App.BrowserJoin = true
	require.NoError(t, Ready(state, in))
}

func TestDeriveWithThePartsOff(t *testing.T) {
	t.Run("no web part", func(t *testing.T) {
		state, in := configuredState(t), configuredInputs()
		state.App = App{}
		in.NoWeb, in.App = true, ""
		state.File.Web.Unscoped, state.File.Web.UnscopedSet = true, true

		cfg, err := Derive(state, in)

		require.NoError(t, err)
		assert.True(t, cfg.Web.Disabled, "web.disabled")
		assert.Empty(t, cfg.Web.BrowserSessionEndpoint, "the browser join address is removed")
		assert.False(t, cfg.Web.Unscoped)
		assert.False(t, cfg.Web.UnscopedSet, "web.unscoped is removed")
		assert.False(t, cfg.Auth.Disabled)
		assert.Equal(t, "https://auth.shop.example/", cfg.Issuer)
		assert.Equal(t, state.File.APIs, cfg.APIs)
		assert.Empty(t, cfg.Validate())

		changes := config.Diff(state.File, cfg)
		var paths []string
		for _, change := range changes {
			paths = append(paths, change.Path)
		}
		assert.ElementsMatch(t, []string{"web.browser_session_endpoint", "web.unscoped", "web.disabled"}, paths)
	})

	t.Run("no API part", func(t *testing.T) {
		state, in := configuredState(t), configuredInputs()
		state.APIs = nil
		in.NoAPI, in.APIs = true, nil

		cfg, err := Derive(state, in)

		require.NoError(t, err)
		assert.True(t, cfg.Auth.Disabled, "auth.disabled")
		assert.True(t, cfg.Auth.DisabledSet)
		assert.Equal(t, "https://auth.shop.example/", cfg.Issuer, "the issuer comes from the tenant")
		assert.Equal(t, state.File.APIs, cfg.APIs, "the apis entries stay as the file has them")
		assert.Equal(t, state.File.Scopes, cfg.Scopes, "so do the scopes")
		assert.False(t, cfg.Web.Disabled)
		assert.NotEmpty(t, cfg.Web.BrowserSessionEndpoint)
		assert.Empty(t, cfg.Validate())

		changes := config.Diff(state.File, cfg)
		require.Len(t, changes, 1)
		assert.Equal(t, config.Change{Path: "auth.disabled", Kind: config.ChangeAdded, New: "true"}, changes[0])
	})

	t.Run("no file and no API part", func(t *testing.T) {
		state, in := freshState(t)
		state.APIs = nil
		in.NoAPI, in.APIs = true, nil

		cfg, err := Derive(state, in)

		require.NoError(t, err)
		assert.True(t, cfg.Auth.Disabled)
		assert.Empty(t, cfg.APIs)
		assert.NotEmpty(t, cfg.Web.BrowserSessionEndpoint)
		assert.Empty(t, cfg.Validate())
	})

	t.Run("no file and no web part", func(t *testing.T) {
		state, in := freshState(t)
		state.App = App{}
		in.NoWeb, in.App = true, ""

		cfg, err := Derive(state, in)

		require.NoError(t, err)
		assert.Equal(t, config.Web{Disabled: true}, cfg.Web)
		assert.NotEmpty(t, cfg.APIs)
		assert.Empty(t, cfg.Validate())
	})
}

func TestDeriveClearsDisabledOfAPartThatIsOn(t *testing.T) {
	tests := []struct {
		name       string
		prepare    func(*State, *Inputs)
		wantAuth   config.Auth
		wantWebOff bool
		wantJoin   bool
	}{
		{
			name:     "web.disabled in the file and the web app given",
			prepare:  func(s *State, _ *Inputs) { s.File.Web.Disabled = true },
			wantJoin: true,
		},
		{
			name:     "auth.disabled in the file and the APIs given",
			prepare:  func(s *State, _ *Inputs) { s.File.Auth = config.Auth{Disabled: true, DisabledSet: true} },
			wantJoin: true,
		},
		{
			name: "both disabled in the file, both parts given",
			prepare: func(s *State, _ *Inputs) {
				s.File.Auth = config.Auth{Disabled: true, DisabledSet: true}
				s.File.Web.Disabled = true
			},
			wantJoin: true,
		},
		{
			name: "web.disabled in the file kept while only the API part is on",
			prepare: func(s *State, in *Inputs) {
				s.File.Web.Disabled = true
				s.App = App{}
				in.NoWeb, in.App = true, ""
			},
			wantWebOff: true,
		},
		{
			name: "auth.disabled in the file kept while only the web part is on",
			prepare: func(s *State, in *Inputs) {
				s.File.Auth = config.Auth{Disabled: true, DisabledSet: true}
				s.APIs = nil
				in.NoAPI, in.APIs = true, nil
			},
			wantAuth: config.Auth{Disabled: true, DisabledSet: true},
			wantJoin: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state, in := configuredState(t), configuredInputs()
			test.prepare(state, &in)

			cfg, err := Derive(state, in)

			require.NoError(t, err)
			assert.Equal(t, test.wantAuth, cfg.Auth)
			assert.Equal(t, test.wantWebOff, cfg.Web.Disabled, "web.disabled")
			assert.Equal(t, test.wantJoin, cfg.Web.BrowserSessionEndpoint != "", "the browser join address")
			assert.Empty(t, cfg.Validate())
		})
	}
}

func TestBuildWithThePartsOff(t *testing.T) {
	t.Run("no web part", func(t *testing.T) {
		state, in := freshState(t)
		state.App = App{}
		in.NoWeb, in.App = true, ""

		plan := buildPlan(t, state, in)

		assert.Equal(t, []Kind{KindTenant, KindAPI, KindFile}, kinds(plan), "no app change")
		assert.Empty(t, plan.App)

		for _, change := range plan.Changes {
			assert.NotContains(t, fieldNames(change), fieldBrowserJoin)
			assert.NotContains(t, fieldNames(change), fieldLoginURI)
			assert.Empty(t, change.ScopeChanges, "the browser sign-in scope is not added")
		}

		assert.NotContains(t, plan.Scopes(), "update:clients")
	})

	t.Run("no API part", func(t *testing.T) {
		state, in := freshState(t)
		state.APIs = nil
		in.NoAPI, in.APIs = true, nil

		plan := buildPlan(t, state, in)

		assert.Equal(t, []Kind{KindTenant, KindApp, KindFile}, kinds(plan), "no API change")
		assert.Empty(t, plan.APIs)
		assert.Equal(t, "Shop Web", plan.App)
		assert.NotContains(t, fieldNames(plan.Changes[0]), fieldDefaultAudience, "the default audience needs an API")
		assert.Contains(t, fieldNames(plan.Changes[0]), FieldCompanyDomains, "the company domain is still planned")
		assert.Contains(t, fieldNames(plan.Changes[0]), fieldCIMD)
	})

	t.Run("no API part, a tenant that is set up has the file only to write", func(t *testing.T) {
		state, in := configuredState(t), configuredInputs()
		state.APIs = nil
		in.NoAPI, in.APIs = true, nil

		plan := buildPlan(t, state, in)

		assert.Equal(t, []Kind{KindFile}, kinds(plan), "auth.disabled is the only change")
	})
}

func TestCommandWithThePartsOff(t *testing.T) {
	assert.Equal(t,
		[]string{"auth0 poppy setup", "--company-domain shop.example", "--no-api", "--app app1"},
		Command(Inputs{CompanyDomain: "shop.example", NoAPI: true, App: "app1"}, CommandOptions{}))
	assert.Equal(t,
		[]string{"auth0 poppy setup", "--company-domain shop.example", "--api " + shopAPI, "--no-web"},
		Command(Inputs{CompanyDomain: "shop.example", NoWeb: true, APIs: []string{shopAPI}}, CommandOptions{}))
}

func TestCommandOfTheConfigCommand(t *testing.T) {
	in := Inputs{
		CompanyDomain: "shop.example", APIs: []string{shopAPI}, App: "app1", Name: "Shop Inc", LoginURI: "https://shop.example/login",
		Scopes: map[string]string{"a": "b"}, Out: "x.json",
	}

	assert.Equal(t,
		[]string{"auth0 poppy config", "--tenant t.auth0.com", "--company-domain shop.example", "--api " + shopAPI, "--app app1", `--company-name "Shop Inc"`, "--out x.json"},
		Command(in, CommandOptions{Tenant: "t.auth0.com", Config: true, Yes: true}),
		"the config command has no --scope, --login-uri or --force")
}

func TestQuestionsForMissing(t *testing.T) {
	// The fields are sorted, which puts the OpenAPI URL before the domain and the name.
	questions, unset := QuestionsForMissing(&config.MissingFieldsError{Fields: []string{
		"apis[" + shopAPI + "].description", "apis[" + shopAPI + "].url", "organization.domain", "organization.name",
	}})

	var flags []string
	for _, question := range questions {
		flags = append(flags, question.Flag)
	}

	assert.Equal(t, []string{"--company-domain", "--company-name", "--api-url " + shopAPI + "=<openapi url>"}, flags, "the domain, then the name, then the OpenAPI URLs")
	assert.Equal(t, []string{"apis[" + shopAPI + "].description"}, unset)
}

func TestReadWithThePartsOff(t *testing.T) {
	t.Run("no app is read without a reference", func(t *testing.T) {
		f := newReaderFixture(t)
		f.expectTenant()
		f.domains.EXPECT().List(gomock.Any()).Return(nil, nil)
		f.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(serverPages([]*managementv3.ResourceServer{shopServer()}), nil)

		state, err := f.reader.Read(t.Context(), []string{shopAPI}, "")

		require.NoError(t, err)
		assert.Equal(t, App{}, state.App)
		assert.Len(t, state.APIs, 1)
	})

	t.Run("no API is read without a reference", func(t *testing.T) {
		f := newReaderFixture(t)
		f.expectTenant()
		f.domains.EXPECT().List(gomock.Any()).Return(nil, nil)
		f.clients.EXPECT().Get(gomock.Any(), shopApp, gomock.Any()).Return(shopClient(), nil)

		state, err := f.reader.Read(t.Context(), nil, shopApp)

		require.NoError(t, err)
		assert.Empty(t, state.APIs)
		assert.Equal(t, shopApp, state.App.ClientID)
	})
}

func TestPartFlagListsAreDisjointFromTheOtherPart(t *testing.T) {
	// The flags named in the rules of the two parts, so that a new flag is placed on purpose.
	var api, web []string

	for _, flag := range (Inputs{}).apiFlags() {
		api = append(api, flag.name)
	}

	for _, flag := range (Inputs{}).webFlags() {
		web = append(web, flag.name)
	}

	assert.ElementsMatch(t, []string{"--api", "--api-url", "--scope", "--hint"}, api)
	assert.ElementsMatch(t, []string{"--app", "--login-uri", "--web-unscoped"}, web)

	shared := slices.DeleteFunc(slices.Clone(api), func(name string) bool { return !slices.Contains(web, name) })
	assert.Empty(t, shared, "no flag belongs to both")
}
