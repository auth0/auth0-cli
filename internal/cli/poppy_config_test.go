package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/poppy/config"
)

func TestPoppyConfig(t *testing.T) {
	t.Run("writes the file for a tenant that is set up", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		out := filepath.Join(t.TempDir(), "poppy.json")

		err := h.run("config", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID,
			"--api-url", poppyTestAPI+"="+poppyTestAPI+"/openapi.json", "--out", out)

		require.NoError(t, err)
		assert.Contains(t, h.out.String(), out+"  written")
		assert.Empty(t, h.messages.String(), "the result is on stdout")

		cfg, err := config.Read(out)
		require.NoError(t, err)
		assert.Equal(t, "shop.example", cfg.Organization.Domain)
		assert.Equal(t, "https://auth.shop.example/", cfg.Issuer)
		assert.Equal(t, "https://auth.shop.example/poppy/"+poppySetupAppID+"/browser-session", cfg.Web.BrowserSessionEndpoint)
		assert.Equal(t, "Shop API", cfg.APIs[0].Description)
		assert.Equal(t, poppyTestAPI+"/openapi.json", cfg.APIs[0].URL)
	})

	t.Run("a file that is already right is said to be unchanged", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "poppy.json")
		args := []string{"config", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID,
			"--api-url", poppyTestAPI + "=" + poppyTestAPI + "/openapi.json", "--out", out}

		first := newPoppySetupHarness(t, configuredPoppyTenant())
		require.NoError(t, first.run(args...))
		assert.Contains(t, first.out.String(), out+"  written")

		second := newPoppySetupHarness(t, configuredPoppyTenant())
		require.NoError(t, second.run(args...))
		assert.Contains(t, second.out.String(), out+"  unchanged")
		assert.NotContains(t, second.out.String(), "written")
	})

	t.Run("the API is told by the file and the web app by the tenant", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		path := goldenPoppyConfig(t)
		before, err := os.ReadFile(path)
		require.NoError(t, err)

		require.NoError(t, h.run("config", "--out", path))

		after, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.JSONEq(t, string(before), string(after), "rebuilding from a matching tenant changes nothing")
	})

	t.Run("--api alone is enough when the tenant has one app with the browser join", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		out := filepath.Join(t.TempDir(), "poppy.json")

		require.NoError(t, h.run("config", "--company-domain", "shop.example", "--api", poppyTestAPI,
			"--api-url", poppyTestAPI+"="+poppyTestAPI+"/openapi.json", "--out", out))

		cfg, err := config.Read(out)
		require.NoError(t, err)
		assert.Equal(t, "https://auth.shop.example/poppy/"+poppySetupAppID+"/browser-session", cfg.Web.BrowserSessionEndpoint)
	})

	t.Run("prints the file as JSON", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		path := goldenPoppyConfig(t)

		require.NoError(t, h.run("config", "--out", path, "--json"))

		var cfg config.Config
		require.NoError(t, json.Unmarshal(h.out.Bytes(), &cfg))
		assert.Equal(t, "https://auth.shop.example/", cfg.Issuer)
		assert.Empty(t, h.messages.String())

		var doc map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(h.out.Bytes(), &doc))
		assert.Contains(t, doc, "auth", "the JSON has the shape of the file")
		assert.NotContains(t, doc, "auth0", "the CLI adds no member of its own")
	})

	t.Run("members of an existing file that the CLI does not use are kept", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		path := goldenPoppyConfig(t)

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, []byte(strings.Replace(string(data), `"apis": [`, `"feedback": {"endpoint": "https://shop.example/feedback"},
  "apis": [`, 1)), 0o600))

		require.NoError(t, h.run("config", "--out", path))

		after, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Contains(t, string(after), `"endpoint": "https://shop.example/feedback"`)
	})

	t.Run("a file in the old layout is replaced by the new layout", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		path := filepath.Join(t.TempDir(), "poppy.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"protocol_version":"0.4","issuer":"https://auth.shop.example/","scopes":{},"apis":[],"web":{"client_id":"x"},"agents":[]}`), 0o600))

		require.NoError(t, h.run("config", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID,
			"--api-url", poppyTestAPI+"="+poppyTestAPI+"/openapi.json", "--out", path))

		cfg, err := config.Read(path)
		require.NoError(t, err)
		assert.Equal(t, "https://auth.shop.example/", cfg.Issuer)
	})

	t.Run("the company domain is picked from several without a terminal", func(t *testing.T) {
		tenant := configuredPoppyTenant()
		tenant.settings.PersonalAgentProtocolCompanyDomains = []string{"shop.example", "https://shop.test"}
		h := newPoppySetupHarness(t, tenant)

		out := filepath.Join(t.TempDir(), "none.json")
		err := h.run("config", "--api", poppyTestAPI, "--app", poppySetupAppID,
			"--api-url", poppyTestAPI+"="+poppyTestAPI+"/openapi.json", "--out", out)

		reason, details := decodeCLIError(t, err)
		assert.Equal(t, "missing_required_flags", reason)
		assert.Equal(t, "usage", errorClass(err))
		assert.EqualError(t, err, "missing required flags: --company-domain")
		assert.Contains(t, details["flags"].([]any)[0].(map[string]any)["reason"], "shop.example, shop.test")
		assert.Equal(t, "auth0 poppy config --tenant "+poppySetupTenant+" --api "+poppyTestAPI+" --api-url "+poppyTestAPI+"="+poppyTestAPI+"/openapi.json --app "+poppySetupAppID+" --out "+out+" --company-domain <domain>", details["example"])
	})
}

func TestPoppyConfigErrors(t *testing.T) {
	missingURL := []string{"config", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID, "--out"}

	tests := []struct {
		name       string
		tenant     poppyTenantState
		args       func(t *testing.T) []string
		wantReason string
		wantText   string
		// The fix in the details of the error, empty when the message holds it.
		wantFix string
	}{
		{
			name: "no API anywhere",
			tenant: func() poppyTenantState {
				state := configuredPoppyTenant()
				state.server.UseForPersonalAgentProtocol = nil
				return state
			}(),
			args: func(t *testing.T) []string {
				return []string{"config", "--app", poppySetupAppID, "--out", filepath.Join(t.TempDir(), "none.json")}
			},
			wantReason: "api_not_found",
			wantText:   "There is no config file, so no API of the tenant is known.",
			wantFix:    "Pass --api <identifier> to name the API, or create the file with auth0 poppy setup.",
		},
		{
			name: "no app anywhere",
			tenant: func() poppyTenantState {
				state := configuredPoppyTenant()
				state.app.AllowPersonalAgentBrowserJoin = nil
				return state
			}(),
			args: func(t *testing.T) []string {
				return []string{"config", "--api", poppyTestAPI, "--out", filepath.Join(t.TempDir(), "none.json")}
			},
			wantReason: "app_not_found",
			wantText:   "No app of the tenant has allow_personal_agent_browser_join on.",
			wantFix:    "Pass --app <client ID or name> to name the web app.",
		},
		{
			name:   "the OpenAPI URL is not stored anywhere",
			tenant: configuredPoppyTenant(),
			args: func(t *testing.T) []string {
				return append(missingURL, filepath.Join(t.TempDir(), "none.json"))
			},
			wantReason: "missing_required_flags",
			wantText:   "missing required flags: --api-url " + poppyTestAPI + "=<openapi url>",
		},
		{
			name: "the company domain is not listed",
			tenant: func() poppyTenantState {
				state := configuredPoppyTenant()
				state.settings.PersonalAgentProtocolCompanyDomains = nil
				return state
			}(),
			args: func(t *testing.T) []string {
				return append(missingURL, goldenPoppyConfig(t))
			},
			wantReason: "poppy_not_ready",
			wantText:   "tenant setting personal_agent_protocol_company_domains does not list shop.example",
		},
		{
			name: "the API does not use the protocol",
			tenant: func() poppyTenantState {
				state := configuredPoppyTenant()
				state.server.UseForPersonalAgentProtocol = nil
				return state
			}(),
			args: func(t *testing.T) []string {
				return append(missingURL, goldenPoppyConfig(t))
			},
			wantReason: "poppy_not_ready",
			wantText:   "API " + poppyTestAPI + ": use_for_personal_agent_protocol is off",
		},
		{
			name: "the browser join is off",
			tenant: func() poppyTenantState {
				state := configuredPoppyTenant()
				state.app.AllowPersonalAgentBrowserJoin = nil
				return state
			}(),
			args: func(t *testing.T) []string {
				return append(missingURL, goldenPoppyConfig(t))
			},
			wantReason: "poppy_not_ready",
			wantText:   "app Shop Web: allow_personal_agent_browser_join is off\n  FIX  Run: auth0 poppy setup --company-domain shop.example",
		},
		{
			name:   "the file belongs to another issuer",
			tenant: func() poppyTenantState { state := configuredPoppyTenant(); state.domains = nil; return state }(),
			args: func(t *testing.T) []string {
				return append(missingURL, goldenPoppyConfig(t))
			},
			wantReason: "issuer_mismatch",
			wantText:   "Pass --overwrite-config to overwrite it",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := newPoppySetupHarness(t, test.tenant)
			args := test.args(t)

			before := map[string][]byte{}
			if path := args[len(args)-1]; filepath.Ext(path) == ".json" {
				if data, err := os.ReadFile(path); err == nil {
					before[path] = data
				}
			}

			err := h.run(args...)

			reason, details := decodeCLIError(t, err)
			assert.Equal(t, test.wantReason, reason)
			assert.Contains(t, err.Error(), test.wantText)
			assert.Empty(t, h.out.String())

			if test.wantFix != "" {
				assert.Equal(t, test.wantFix, details["fix"])
				assert.NotContains(t, err.Error(), "FIX", "the message is one line")
			}

			for path, data := range before {
				after, readErr := os.ReadFile(path)
				require.NoError(t, readErr)
				assert.Equal(t, data, after, "a refused command leaves the file alone")
			}
		})
	}

	t.Run("--overwrite-config overwrites a file of another issuer", func(t *testing.T) {
		tenant := configuredPoppyTenant()
		tenant.domains = nil
		h := newPoppySetupHarness(t, tenant)
		path := goldenPoppyConfig(t)

		require.NoError(t, h.run(append(missingURL, path, "--overwrite-config")...))

		cfg, err := config.Read(path)
		require.NoError(t, err)
		assert.Equal(t, "https://"+poppySetupTenant+"/", cfg.Issuer)
	})
}

func TestPoppyConfigPromptRule(t *testing.T) {
	// A terminal is asked about the hints of the scopes, and the test terminal
	// cannot take the question back, so it stays on the screen. Then it is asked
	// about web.unscoped, as the file has no value for it.
	const question = "Hints for your scopes"

	asks := []string{"hints", "web access"}

	tests := []poppyPromptRow{
		{name: "terminal", mode: modeTerminal, asked: asks, text: "written"},
		{name: "terminal with --json", mode: modeTerminalJSON, asked: asks, message: question, json: true},
		{name: "terminal with --no-input", mode: modeTerminalNoInput, text: "written"},
		{name: "piped", mode: modePiped, text: "written"},
		{name: "piped with --json", mode: modePipedJSON, json: true},
		{name: "agent mode", mode: modeAgent, json: true},
		{name: "agent mode with --no-input=false", mode: modeAgentPrompting, asked: asks, message: question, json: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newPoppySetupHarness(t, configuredPoppyTenant())
			args := []string{"config", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID,
				"--api-url", poppyTestAPI + "=" + poppyTestAPI + "/openapi.json", "--out", filepath.Join(t.TempDir(), "poppy.json")}

			assertPoppyPromptRun(t, tc, h.runPromptMode(t, tc.mode, args))
		})
	}
}

func TestPoppyConfigPromptRuleCompanyDomain(t *testing.T) {
	const missing = "missing required flags: --company-domain"

	// The question a terminal gets here is a survey prompt that no test can answer,
	// so the modes that ask are left out.
	tests := []poppyPromptRow{
		{name: "terminal with --no-input", mode: modeTerminalNoInput, reason: "missing_required_flags", errText: missing},
		{name: "piped", mode: modePiped, reason: "missing_required_flags", errText: missing},
		{name: "piped with --json", mode: modePipedJSON, reason: "missing_required_flags", errText: missing},
		{name: "agent mode", mode: modeAgent, reason: "missing_required_flags", errText: missing},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tenant := configuredPoppyTenant()
			tenant.settings.PersonalAgentProtocolCompanyDomains = []string{"shop.example", "https://shop.test"}

			h := newPoppySetupHarness(t, tenant)
			args := []string{"config", "--api", poppyTestAPI, "--app", poppySetupAppID, "--out", filepath.Join(t.TempDir(), "poppy.json")}

			assertPoppyPromptRun(t, tc, h.runPromptMode(t, tc.mode, args))
		})
	}
}

func TestPoppyConfigAsksTheHints(t *testing.T) {
	newArgs := func(t *testing.T) ([]string, string) {
		out := filepath.Join(t.TempDir(), "poppy.json")

		return []string{"config", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID,
			"--api-url", poppyTestAPI + "=" + poppyTestAPI + "/openapi.json", "--out", out}, out
	}

	t.Run("the answers go to the file and are listed", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		args, out := newArgs(t)

		run := h.runHintMode(t, &poppyScript{answers: []any{nil, 0, false, false, nil, nil}}, args)

		require.NoError(t, run.err)

		data, err := os.ReadFile(out)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"orders:read": {`)
		assert.Regexp(t, `"changes": "none",\s+"personal_data": false`, string(data))
		assert.Contains(t, run.out, "Your answers")
		assert.Contains(t, run.out, "orders:read: none")
		assert.Contains(t, run.out, "checkout: irreversible, personal data")
		assert.Equal(t, 1, strings.Count(run.out, "Poppy config for shop.example"), "the title is printed once")
	})

	t.Run("skipping them names the flag", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		args, _ := newArgs(t)

		run := h.runHintMode(t, &poppyScript{answers: []any{false}}, args)

		require.NoError(t, run.err)
		assert.Contains(t, run.out, "auth0 poppy config --hint <scope>=changes:<none|reversible|irreversible>,personal_data:<true|false>")
		assert.Contains(t, run.out, "to set the hints of 2 scopes.")
	})

	t.Run("Ctrl-C writes no file", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		args, out := newArgs(t)

		run := h.runHintMode(t, &poppyScript{answers: []any{terminal.InterruptErr}}, args)

		require.ErrorIs(t, run.err, terminal.InterruptErr)
		assert.Contains(t, run.errOut, display.PromptCancelled)
		assert.NoFileExists(t, out)
	})
}

func TestPoppyConfigWithThePartsOff(t *testing.T) {
	t.Run("--no-web writes web.disabled", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		out := filepath.Join(t.TempDir(), "poppy.json")

		require.NoError(t, h.run("config", "--company-domain", "shop.example", "--api", poppyTestAPI, "--no-web",
			"--api-url", poppyTestAPI+"="+poppyTestAPI+"/openapi.json", "--out", out))

		cfg, err := config.Read(out)
		require.NoError(t, err)
		assert.Equal(t, config.Web{Disabled: true}, cfg.Web)
		assert.NotEmpty(t, cfg.APIs)
	})

	t.Run("--no-api writes auth.disabled", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		out := filepath.Join(t.TempDir(), "poppy.json")

		require.NoError(t, h.run("config", "--company-domain", "shop.example", "--app", poppySetupAppID, "--no-api", "--out", out))

		cfg, err := config.Read(out)
		require.NoError(t, err)
		assert.True(t, cfg.Auth.Disabled)
		assert.Empty(t, cfg.APIs)
		assert.NotEmpty(t, cfg.Web.BrowserSessionEndpoint)
	})

	t.Run("a file that turns the API off is kept by config without the flag", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		out := filepath.Join(t.TempDir(), "poppy.json")

		require.NoError(t, h.run("config", "--company-domain", "shop.example", "--app", poppySetupAppID, "--no-api", "--out", out))

		again := newPoppySetupHarness(t, configuredPoppyTenant())
		require.NoError(t, again.run("config", "--company-domain", "shop.example", "--app", poppySetupAppID, "--out", out))

		cfg, err := config.Read(out)
		require.NoError(t, err)
		assert.True(t, cfg.Auth.Disabled)
		assert.Contains(t, again.out.String(), out+"  unchanged")
	})
}

func TestPoppyConfigWebUnscoped(t *testing.T) {
	tests := []struct {
		name        string
		mode        poppyPromptMode
		member      string
		flags       []string
		answer      webAnswer
		wantAsked   int
		wantValue   bool
		wantPresent bool
	}{
		{name: "terminal, no value in the file, yes", mode: modeTerminal, answer: true, wantAsked: 1, wantValue: true, wantPresent: true},
		{name: "terminal, no value in the file, no", mode: modeTerminal, answer: false, wantAsked: 1, wantPresent: true},
		{name: "terminal with --json still asks", mode: modeTerminalJSON, answer: true, wantAsked: 1, wantValue: true, wantPresent: true},
		{name: "the file says false: not asked", mode: modeTerminal, member: `"unscoped": false`, answer: true, wantPresent: true},
		{name: "the file says true: not asked", mode: modeTerminal, member: `"unscoped": true`, answer: false, wantValue: true, wantPresent: true},
		{name: "the flag over a true file", mode: modeTerminal, member: `"unscoped": true`, flags: []string{"--web-unscoped=false"}, answer: true, wantPresent: true},
		{name: "the flag over an empty file", mode: modeTerminal, flags: []string{"--web-unscoped"}, answer: false, wantValue: true, wantPresent: true},
		{name: "terminal with --no-input leaves it absent", mode: modeTerminalNoInput, answer: true},
		{name: "piped leaves it absent", mode: modePiped, answer: true},
		{name: "piped keeps the file's value", mode: modePiped, member: `"unscoped": true`, answer: false, wantValue: true, wantPresent: true},
		{name: "piped uses the flag", mode: modePiped, flags: []string{"--web-unscoped"}, answer: false, wantValue: true, wantPresent: true},
		{name: "agent mode leaves it absent", mode: modeAgent, answer: true},
		{name: "agent mode uses the flag set to false", mode: modeAgent, flags: []string{"--web-unscoped=false"}, answer: true, wantPresent: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := newPoppySetupHarness(t, configuredPoppyTenant())
			path := poppyConfigWithWeb(t, test.member)

			args := append([]string{"config", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID, "--out", path}, test.flags...)

			run, asked := h.runWebMode(t, test.mode, args, test.answer)

			require.NoError(t, run.err)
			assert.Equal(t, test.wantAsked, asked, "times asked")

			value, present := unscopedIn(t, path)
			assert.Equal(t, test.wantValue, value, "web.unscoped")
			assert.Equal(t, test.wantPresent, present, "web.unscoped is in the file")
		})
	}
}

func TestPoppyConfigWebUnscopedShowsInTheAnswers(t *testing.T) {
	h := newPoppySetupHarness(t, configuredPoppyTenant())
	path := poppyConfigWithWeb(t, "")

	run, asked := h.runWebMode(t, modeTerminal, []string{"config", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID, "--out", path}, true)

	require.NoError(t, run.err)
	assert.Equal(t, 1, asked)
	assert.Contains(t, run.out, "Your answers")
	assert.Contains(t, run.out, "Web access    web.unscoped true")
	assert.Contains(t, run.errOut, poppyWebUnscopedNote)
}

func TestPoppyConfigWebUnscopedCtrlC(t *testing.T) {
	h := newPoppySetupHarness(t, configuredPoppyTenant())
	path := poppyConfigWithWeb(t, "")

	before, err := os.ReadFile(path)
	require.NoError(t, err)

	run, asked := h.runWebMode(t, modeTerminal, []string{"config", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID, "--out", path}, terminal.InterruptErr)

	require.ErrorIs(t, run.err, terminal.InterruptErr)
	assert.Equal(t, 1, asked)
	assert.Contains(t, run.errOut, display.PromptCancelled)

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "the file was not changed")
}

func TestPoppyConfigClearsWebDisabledWhenTheAppIsGivenAndNeverAsksForIt(t *testing.T) {
	h := newPoppySetupHarness(t, configuredPoppyTenant())
	path := poppyConfigWithWeb(t, `"disabled": true`)

	run, asked := h.runWebMode(t, modeTerminal, []string{"config", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID, "--out", path}, true)

	require.NoError(t, run.err)
	assert.Equal(t, 1, asked, "only web.unscoped is asked")

	cfg, err := config.Read(path)
	require.NoError(t, err)
	assert.False(t, cfg.Web.Disabled, "--app turns the web part on, so web.disabled goes")
	assert.NotEmpty(t, cfg.Web.BrowserSessionEndpoint)
	assert.True(t, cfg.Web.Unscoped)
}

func TestPoppyConfigAsksTheMissingValues(t *testing.T) {
	t.Run("in a terminal the OpenAPI URL is asked instead of failing", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		out := filepath.Join(t.TempDir(), "poppy.json")

		asked := installPoppyAsker(t, modeTerminal)
		promptAsk.text = func(label, _ string) (string, error) {
			*asked = append(*asked, "text "+label)

			return poppyTestAPI + "/docs/openapi.json", nil
		}

		err := h.run("config", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID, "--out", out)

		require.NoError(t, err)

		var texts []string
		for _, question := range *asked {
			if strings.HasPrefix(question, "text ") {
				texts = append(texts, question)
			}
		}
		assert.Len(t, texts, 1, "the URL is asked once: %v", *asked)

		cfg, err := config.Read(out)
		require.NoError(t, err)
		assert.Equal(t, poppyTestAPI+"/docs/openapi.json", cfg.APIs[0].URL)
	})

	t.Run("Ctrl-C at the question writes no file", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		out := filepath.Join(t.TempDir(), "poppy.json")

		installPoppyAsker(t, modeTerminal)
		promptAsk.text = func(string, string) (string, error) { return "", terminal.InterruptErr }
		promptAsk.interrupt = func(err error) error { return err }

		err := h.run("config", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID, "--out", out)

		require.ErrorIs(t, err, terminal.InterruptErr)
		assert.Contains(t, h.messages.String(), display.PromptCancelled)
		assert.NoFileExists(t, out)
	})

	t.Run("without a terminal the missing value names its flag", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		out := filepath.Join(t.TempDir(), "poppy.json")

		installPoppyAsker(t, modePiped)

		err := h.run("config", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID, "--out", out)

		require.ErrorContains(t, err, "--api-url")
		assert.NoFileExists(t, out)
	})
}
