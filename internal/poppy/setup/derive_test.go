package setup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/poppy/config"
)

func TestDeriveRebuildsTheGoldenFile(t *testing.T) {
	state, in := configuredState(t), configuredInputs()

	cfg, err := Derive(state, in)

	require.NoError(t, err)
	assert.Empty(t, config.Diff(state.File, cfg), "the file setup wrote derives to itself")
	assert.Empty(t, cfg.Validate())
}

func TestDeriveWithoutAFile(t *testing.T) {
	state, in := freshState(t)
	state.CustomDomain = "auth.shop.example"
	in.APIURLs = map[string]string{shopAPI: shopAPI + "/openapi.json"}
	in.Name = "Shop Ltd"
	in.Hints = map[string]config.Hints{"checkout": {Changes: "irreversible", PersonalData: true}}

	cfg, err := Derive(state, in)

	require.NoError(t, err)
	assert.Equal(t, "Shop Ltd", cfg.Organization.Name)
	assert.Equal(t, "shop.example", cfg.Organization.Domain)
	assert.Equal(t, "https://auth.shop.example/", cfg.Issuer)
	assert.Equal(t, "https://auth.shop.example/poppy/"+shopApp+"/browser-session", cfg.Web.BrowserSessionEndpoint)
	assert.Equal(t, []config.API{{Type: "openapi", URL: shopAPI + "/openapi.json", Description: "Shop API"}}, cfg.APIs)
	assert.Equal(t, "Place orders and pay with your saved card. Can't be undone.", cfg.Scopes["checkout"].Description)
	assert.Equal(t, &config.Hints{Changes: "irreversible", PersonalData: true}, cfg.Scopes["checkout"].Hints)
	assert.Empty(t, cfg.Validate())
}

func TestDeriveUsesTheScopesThatSetupAdds(t *testing.T) {
	state, in := configuredState(t), configuredInputs()
	in.Scopes = map[string]string{"refunds": "Refund an order."}

	cfg, err := Derive(state, in)

	require.NoError(t, err)
	assert.Equal(t, "Refund an order.", cfg.Scopes["refunds"].Description)
	assert.Contains(t, cfg.Scopes, "checkout")
}

func TestDeriveErrors(t *testing.T) {
	t.Run("a value Auth0 does not store", func(t *testing.T) {
		state, in := freshState(t)
		in.APIURLs = nil

		_, err := Derive(state, in)

		var missing *config.MissingFieldsError
		require.ErrorAs(t, err, &missing)
		assert.Equal(t, []string{"apis[" + shopAPI + "].url"}, missing.Fields)
	})

	t.Run("a scope without a description", func(t *testing.T) {
		state, in := configuredState(t), configuredInputs()
		state.APIs[0].Scopes[0].Description = ""

		_, err := Derive(state, in)

		var input *InputError
		require.ErrorAs(t, err, &input)
		assert.Equal(t, "scope_description_required", input.Reason)
	})

	t.Run("an issuer of another tenant", func(t *testing.T) {
		state, in := configuredState(t), configuredInputs()
		state.CustomDomain = "login.other.example"

		_, err := Derive(state, in)

		assert.ErrorIs(t, err, config.ErrIssuerMismatch)

		in.ForceConfig = true
		_, err = Derive(state, in)
		assert.NoError(t, err)
	})
}

func TestCheckIssuer(t *testing.T) {
	tests := []struct {
		name    string
		change  func(state *State, in *Inputs)
		wantErr bool
	}{
		{name: "the file has the issuer of the tenant", change: func(*State, *Inputs) {}},
		{name: "there is no file", change: func(state *State, _ *Inputs) { state.File = nil }},
		{name: "the file has no issuer", change: func(state *State, _ *Inputs) { state.File.Issuer = "" }},
		{name: "the file has another issuer", change: func(state *State, _ *Inputs) { state.CustomDomain = "login.other.example" }, wantErr: true},
		{
			name: "another issuer is allowed with force",
			change: func(state *State, in *Inputs) {
				state.CustomDomain = "login.other.example"
				in.ForceConfig = true
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state, in := configuredState(t), configuredInputs()
			test.change(state, &in)

			err := CheckIssuer(state, in)

			if !test.wantErr {
				assert.NoError(t, err)

				return
			}

			require.ErrorIs(t, err, config.ErrIssuerMismatch)

			_, deriveErr := Derive(state, in)
			require.Error(t, deriveErr)
			assert.Equal(t, deriveErr.Error(), err.Error(), "the same error as Derive")
		})
	}
}

func TestReady(t *testing.T) {
	tests := []struct {
		name   string
		change func(state *State, in *Inputs)
		want   []string
	}{
		{name: "set up", change: func(*State, *Inputs) {}},
		{
			name:   "no company domain is known",
			change: func(_ *State, in *Inputs) { in.CompanyDomain = "" },
			want:   []string{"tenant setting personal_agent_protocol_company_domains is empty"},
		},
		{
			name:   "the company domain is not listed",
			change: func(state *State, _ *Inputs) { state.Tenant.CompanyDomains = []string{"https://other.example"} },
			want:   []string{"tenant setting personal_agent_protocol_company_domains does not list https://shop.example"},
		},
		{
			name:   "the company domain is listed in another case",
			change: func(state *State, _ *Inputs) { state.Tenant.CompanyDomains = []string{"https://Shop.Example"} },
		},
		{
			name:   "an API does not use the protocol",
			change: func(state *State, _ *Inputs) { state.APIs[0].Poppy = false },
			want:   []string{"API " + shopAPI + ": use_for_personal_agent_protocol is off"},
		},
		{
			name:   "the browser join is off",
			change: func(state *State, _ *Inputs) { state.App.BrowserJoin = false },
			want:   []string{"app Shop Web: allow_personal_agent_browser_join is off"},
		},
		{
			name: "everything is off",
			change: func(state *State, _ *Inputs) {
				state.Tenant.CompanyDomains = nil
				state.APIs[0].Poppy = false
				state.App.BrowserJoin = false
			},
			want: []string{
				"tenant setting personal_agent_protocol_company_domains does not list https://shop.example",
				"API " + shopAPI + ": use_for_personal_agent_protocol is off",
				"app Shop Web: allow_personal_agent_browser_join is off",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state, in := configuredState(t), configuredInputs()
			test.change(state, &in)

			err := Ready(state, in)

			if test.want == nil {
				require.NoError(t, err)
				return
			}

			var notReady *NotReadyError
			require.ErrorAs(t, err, &notReady)
			assert.Equal(t, test.want, notReady.Problems)
		})
	}
}

func TestNotReadyErrorText(t *testing.T) {
	state, in := configuredState(t), configuredInputs()
	state.APIs[0].Poppy = false

	err := Ready(state, in)

	assert.EqualError(t, err, "the tenant is not set up for Poppy:\n"+
		"  - API "+shopAPI+": use_for_personal_agent_protocol is off\n"+
		"  FIX  Run: auth0 poppy setup --company-domain shop.example")

	in.CompanyDomain = ""
	state.Tenant.CompanyDomains = nil

	assert.ErrorContains(t, Ready(state, in), "FIX  Run: auth0 poppy setup --company-domain <host>")
}

func TestStateCompanyDomain(t *testing.T) {
	tests := []struct {
		name        string
		change      func(state *State, in *Inputs)
		wantDomain  string
		wantChoices []string
	}{
		{name: "given", change: func(*State, *Inputs) {}, wantDomain: "shop.example"},
		{
			name: "from the existing file",
			change: func(_ *State, in *Inputs) {
				in.CompanyDomain = ""
			},
			wantDomain: "shop.example",
		},
		{
			name: "the only one the tenant lists",
			change: func(state *State, in *Inputs) {
				in.CompanyDomain = ""
				state.File = nil
				state.Tenant.CompanyDomains = []string{"https://Only.Example"}
			},
			wantDomain: "only.example",
		},
		{
			name: "several listed and none picked",
			change: func(state *State, in *Inputs) {
				in.CompanyDomain = ""
				state.File = nil
				state.Tenant.CompanyDomains = []string{"https://a.example", "https://B.example"}
			},
			wantChoices: []string{"a.example", "b.example"},
		},
		{
			name: "none listed",
			change: func(state *State, in *Inputs) {
				in.CompanyDomain = ""
				state.File = nil
				state.Tenant.CompanyDomains = nil
			},
			wantChoices: []string{},
		},
		{
			name: "the flag wins over the file",
			change: func(_ *State, in *Inputs) {
				in.CompanyDomain = "other.example"
			},
			wantDomain: "other.example",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state, in := configuredState(t), configuredInputs()
			test.change(state, &in)

			domain, choices := state.CompanyDomain(in)

			assert.Equal(t, test.wantDomain, domain)
			assert.Equal(t, test.wantChoices, choices)
		})
	}
}

func TestDeriveGivesOnlyTheBrowserSignInScopeDefaultHints(t *testing.T) {
	state, in := freshState(t)
	state.CustomDomain = "auth.shop.example"
	state.APIs[0].Scopes = append(state.APIs[0].Scopes, Scope{Name: BrowserSignInScope, Description: BrowserSignInDescription})
	in.Hints = nil

	cfg, err := Derive(state, in)

	require.NoError(t, err)
	assert.Equal(t, &config.Hints{Changes: "none", PersonalData: false}, cfg.Scopes[BrowserSignInScope].Hints)
	assert.Nil(t, cfg.Scopes["checkout"].Hints)
	assert.Nil(t, cfg.Scopes["orders:read"].Hints)
}

func TestDeriveDefaultHintsGiveWayToTheFileAndTheInputs(t *testing.T) {
	state, in := configuredState(t), configuredInputs()
	state.File.Scopes[BrowserSignInScope] = config.Scope{
		Description: BrowserSignInDescription,
		Hints:       &config.Hints{Changes: "reversible", PersonalData: true},
	}

	cfg, err := Derive(state, in)

	require.NoError(t, err)
	assert.Equal(t, &config.Hints{Changes: "reversible", PersonalData: true}, cfg.Scopes[BrowserSignInScope].Hints)

	in.Hints = map[string]config.Hints{BrowserSignInScope: {Changes: "none", PersonalData: true}}

	cfg, err = Derive(state, in)

	require.NoError(t, err)
	assert.Equal(t, &config.Hints{Changes: "none", PersonalData: true}, cfg.Scopes[BrowserSignInScope].Hints)
}

func TestDeriveKeepsWhatTheFileHoldsBeyondTheTenant(t *testing.T) {
	file, err := config.Read(filepath.Join("..", "config", "testdata", "auth0-poppy-full.golden.json"))
	require.NoError(t, err)

	state, in := configuredState(t), configuredInputs()
	state.File = file

	cfg, err := Derive(state, in)

	require.NoError(t, err)
	assert.Empty(t, config.Diff(file, cfg))
	assert.True(t, cfg.Web.Unscoped)
	assert.Equal(t, "https://shop.example/logo.png", cfg.Organization.LogoURI)
	require.Len(t, cfg.APIs, 2)
	assert.Equal(t, "mcp", cfg.APIs[1].Type)

	out := filepath.Join(t.TempDir(), config.DefaultFileName)
	require.NoError(t, config.Write(out, cfg))

	want, err := os.ReadFile(filepath.Join("..", "config", "testdata", "auth0-poppy-full.golden.json"))
	require.NoError(t, err)

	got, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Equal(t, string(want), string(got))
}

func TestDeriveKeepsTheScopesOfApisOutsideTheRun(t *testing.T) {
	state, in := configuredState(t), configuredInputs()
	state.File.Scopes["users:write"] = config.Scope{
		Description: "Change users.",
		Hints:       &config.Hints{Changes: "reversible", PersonalData: true},
	}

	cfg, err := Derive(state, in)

	require.NoError(t, err)
	assert.Equal(t, state.File.Scopes["users:write"], cfg.Scopes["users:write"])
	assert.Contains(t, cfg.Scopes, "checkout")
	assert.Empty(t, config.Diff(state.File, cfg))
}

func TestDeriveUpdatesEveryEntryAtTheOriginOfTheApi(t *testing.T) {
	state, in := configuredState(t), configuredInputs()
	in.APIURLs = nil
	state.File.APIs = []config.API{
		{Type: "mcp", URL: shopAPI + "/mcp", Description: "Search"},
		{Type: "openapi", URL: shopAPI + "/openapi.json", Description: "Orders"},
	}

	cfg, err := Derive(state, in)

	require.NoError(t, err)
	assert.Equal(t, state.File.APIs, cfg.APIs)
}

func TestDeriveNeedsAnOrganizationName(t *testing.T) {
	state, in := freshState(t)
	state.Tenant.FriendlyName = ""
	in.APIURLs = map[string]string{shopAPI: shopAPI + "/openapi.json"}

	_, err := Derive(state, in)

	var missing *config.MissingFieldsError
	require.ErrorAs(t, err, &missing)
	assert.Equal(t, []string{"organization.name"}, missing.Fields)

	in.Name = "Shop Ltd"

	cfg, err := Derive(state, in)

	require.NoError(t, err)
	assert.Equal(t, "Shop Ltd", cfg.Organization.Name)
}
