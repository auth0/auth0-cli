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

	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/poppy/setup"
)

func TestPoppyPartFlagsAreOnSetupAndConfigOnly(t *testing.T) {
	root := newPoppyTestRoot()

	for _, path := range [][]string{{"poppy", "setup"}, {"poppy", "config"}} {
		cmd, _, err := root.Find(path)
		require.NoError(t, err)

		for _, name := range []string{"no-api", "no-web"} {
			flag := cmd.Flags().Lookup(name)
			require.NotNil(t, flag, strings.Join(path, " ")+" --"+name)
			assert.Equal(t, "false", flag.DefValue)
		}
	}

	for _, path := range [][]string{{"poppy", "doctor"}, {"agents", "register"}} {
		cmd, _, err := root.Find(path)
		require.NoError(t, err)

		assert.Nil(t, cmd.Flags().Lookup("no-api"), strings.Join(path, " "))
		assert.Nil(t, cmd.Flags().Lookup("no-web"), strings.Join(path, " "))
	}
}

func TestAskPoppyPart(t *testing.T) {
	fileOff := func(authOff, webOff bool) setup.Inputs {
		in := setup.Inputs{}
		require.NoError(t, in.UseConfigFile(&config.Config{Auth: config.Auth{Disabled: authOff}, Web: config.Web{Disabled: webOff}}))

		return in
	}

	fileWith := func(file *config.Config) setup.Inputs {
		in := setup.Inputs{}
		require.NoError(t, in.UseConfigFile(file))

		return in
	}

	tests := []struct {
		name        string
		interactive bool
		part        string
		in          setup.Inputs
		answer      bool
		wantAsked   bool
		wantNoAPI   bool
		wantNoWeb   bool
		wantReason  string
	}{
		{name: "no terminal asks nothing", part: setup.PartAPI, answer: false},
		{name: "API, yes", interactive: true, part: setup.PartAPI, answer: true, wantAsked: true},
		{name: "API, no", interactive: true, part: setup.PartAPI, answer: false, wantAsked: true, wantNoAPI: true},
		{name: "web, yes", interactive: true, part: setup.PartWeb, answer: true, wantAsked: true},
		{name: "web, no", interactive: true, part: setup.PartWeb, answer: false, wantAsked: true, wantNoWeb: true},
		{name: "--api answers the API question", interactive: true, part: setup.PartAPI, in: setup.Inputs{APIs: []string{"https://api.shop.example"}}, answer: false},
		{name: "--app answers the web question", interactive: true, part: setup.PartWeb, in: setup.Inputs{App: "app1"}, answer: false},
		{name: "--login-uri answers the web question", interactive: true, part: setup.PartWeb, in: setup.Inputs{LoginURI: "https://shop.example/login"}, answer: false},
		{name: "auth.disabled in the file answers the API question", interactive: true, part: setup.PartAPI, in: fileOff(true, false), answer: true, wantNoAPI: true},
		{name: "web.disabled in the file answers the web question", interactive: true, part: setup.PartWeb, in: fileOff(false, true), answer: true, wantNoWeb: true},
		{name: "apis in the file answer the API question", interactive: true, part: setup.PartAPI, in: fileWith(&config.Config{APIs: []config.API{{URL: "https://api.shop.example/openapi.json"}}}), answer: false},
		{name: "the join address in the file answers the web question", interactive: true, part: setup.PartWeb, in: fileWith(&config.Config{Web: config.Web{BrowserSessionEndpoint: "https://auth.shop.example/poppy/c/browser-session"}}), answer: false},
		{name: "apis in the file do not answer the web question", interactive: true, part: setup.PartWeb, in: fileWith(&config.Config{APIs: []config.API{{URL: "https://api.shop.example/openapi.json"}}}), answer: false, wantAsked: true, wantNoWeb: true},
		{name: "--no-api is not asked again", interactive: true, part: setup.PartAPI, in: setup.Inputs{NoAPI: true}, answer: true, wantNoAPI: true},
		{name: "no to both ends the run", interactive: true, part: setup.PartWeb, in: setup.Inputs{NoAPI: true}, answer: false, wantAsked: true, wantNoAPI: true, wantNoWeb: true, wantReason: "nothing_to_set_up"},
		{name: "the file turns the API off and the answer turns the web off", interactive: true, part: setup.PartWeb, in: fileOff(true, false), answer: false, wantAsked: true, wantNoAPI: true, wantNoWeb: true, wantReason: "nothing_to_set_up"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var asked bool

			screen, _ := newTestPromptScreen(t, promptAsker{
				confirm: func(message string, _ bool) (bool, error) {
					asked = true

					assert.Contains(t, []string{poppyAPIPartText, poppyWebPartText}, message)

					return test.answer, nil
				},
			})

			in := test.in

			err := askPoppyPart(screen, test.interactive, test.part, &in)

			if test.wantReason == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Equal(t, test.wantReason, errorReason(err))
			}

			assert.Equal(t, test.wantAsked, asked, "asked")
			assert.Equal(t, test.wantNoAPI, in.NoAPI, "NoAPI")
			assert.Equal(t, test.wantNoWeb, in.NoWeb, "NoWeb")
		})
	}

	t.Run("Ctrl-C ends the question and turns nothing off", func(t *testing.T) {
		screen, _ := newTestPromptScreen(t, promptAsker{
			confirm: func(string, bool) (bool, error) { return false, terminal.InterruptErr },
		})

		for _, part := range []string{setup.PartAPI, setup.PartWeb} {
			var in setup.Inputs

			require.ErrorIs(t, askPoppyPart(screen, true, part, &in), terminal.InterruptErr)
			assert.False(t, in.NoAPI)
			assert.False(t, in.NoWeb)
		}
	})
}

func TestPoppyPartWording(t *testing.T) {
	assert.Equal(t, "Do you want agents to use an API with Poppy?", poppyAPIPartText)
	assert.Equal(t, "Do you want agents to browse your website signed in (browser sign-in)?", poppyWebPartText)
	assert.Equal(t, "Web Browsing", poppyWebPartTitle)
	assert.Equal(t, "Agents browse your website on behalf of the end user, either as guests or signed in. Signed-in access is approved by end users.", poppyWebPartReason)
}

func TestPoppySetupWithoutTheWebPart(t *testing.T) {
	args := func(t *testing.T, extra ...string) []string {
		t.Helper()

		return append([]string{
			"setup", "--company-domain", "shop.example", "--api", poppyTestAPI, "--no-web",
			"--api-url", poppyTestAPI + "=" + poppyTestAPI + "/openapi.json",
			"--out", filepath.Join(t.TempDir(), "poppy.json"),
		}, extra...)
	}

	t.Run("the plan has no app and no browser sign-in scope", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())

		require.NoError(t, h.run(args(t, "--dry-run")...))

		text := h.out.String()
		assert.Contains(t, text, "API "+poppyTestAPI+"\n")
		assert.NotContains(t, text, "App Shop Web")
		assert.NotContains(t, text, "browser join")
		assert.NotContains(t, text, setup.BrowserSignInScope)
		assert.Contains(t, text, "off (web.disabled)")
		assert.Contains(t, text, "--no-web", "the command to apply repeats the choice")
		assert.NotContains(t, text, "--app")
	})

	t.Run("the plan as JSON has no app", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())

		require.NoError(t, h.run(args(t, "--dry-run", "--json")...))

		var plan setup.Plan
		require.NoError(t, json.Unmarshal(h.out.Bytes(), &plan))
		assert.Empty(t, plan.App)

		for _, change := range plan.Changes {
			assert.NotEqual(t, setup.KindApp, change.Kind)
		}
	})

	t.Run("applying changes the tenant and the API, never the app, and writes web.disabled", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())
		h.allowWrites()
		a := args(t, "--force")

		require.NoError(t, h.run(a...))

		assert.Equal(t, []string{"tenant", "api"}, h.calls)

		cfg, err := config.Read(outOfArgs(a))
		require.NoError(t, err)
		assert.Equal(t, config.Web{Disabled: true}, cfg.Web)
		assert.False(t, cfg.Auth.Disabled)
		assert.NotEmpty(t, cfg.APIs)
		assert.NotContains(t, cfg.Scopes, setup.BrowserSignInScope)
	})

	t.Run("a tenant that is set up for the API needs nothing about the app", func(t *testing.T) {
		state := configuredPoppyTenant()
		state.app.AllowPersonalAgentBrowserJoin = nil
		h := newPoppySetupHarness(t, state)

		err := h.run("setup", "--company-domain", "shop.example", "--api", poppyTestAPI, "--no-web", "--out", filepath.Join(t.TempDir(), "none.json"),
			"--api-url", poppyTestAPI+"="+poppyTestAPI+"/openapi.json", "--dry-run")

		require.NoError(t, err)
		assert.NotContains(t, h.out.String(), "browser join")
	})
}

// outOfArgs returns the value of --out.
func outOfArgs(args []string) string {
	for i, arg := range args {
		if arg == "--out" {
			return args[i+1]
		}
	}

	return ""
}

func TestPoppySetupWithoutTheAPIPart(t *testing.T) {
	args := func(t *testing.T, extra ...string) []string {
		t.Helper()

		return append([]string{
			"setup", "--company-domain", "shop.example", "--app", poppySetupAppID, "--no-api",
			"--out", filepath.Join(t.TempDir(), "poppy.json"),
		}, extra...)
	}

	t.Run("the plan has no API and asks for no OpenAPI URL", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())

		require.NoError(t, h.run(args(t, "--dry-run")...))

		text := h.out.String()
		assert.NotContains(t, text, "API "+poppyTestAPI)
		assert.NotContains(t, text, "default audience")
		assert.NotContains(t, text, "Scopes")
		assert.Contains(t, text, "App Shop Web\n")
		assert.Contains(t, text, "CIMD support")
		assert.Contains(t, text, "off (auth.disabled)")
		assert.Contains(t, text, "--no-api", "the command to apply repeats the choice")
		assert.NotContains(t, text, "--api ")
	})

	t.Run("applying changes the tenant and the app, never an API, and writes auth.disabled", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())
		h.allowWrites()
		a := args(t, "--force")

		require.NoError(t, h.run(a...))

		assert.Equal(t, []string{"tenant", "app"}, h.calls)

		cfg, err := config.Read(outOfArgs(a))
		require.NoError(t, err)
		assert.True(t, cfg.Auth.Disabled)
		assert.False(t, cfg.Web.Disabled)
		assert.NotEmpty(t, cfg.Web.BrowserSessionEndpoint)
		assert.Empty(t, cfg.APIs)
		assert.NotEmpty(t, cfg.Issuer)
	})
}

func TestPoppySetupWithBothPartsOff(t *testing.T) {
	h := newPoppySetupHarness(t, freshPoppyTenant())

	err := h.run("setup", "--no-api", "--no-web", "--company-domain", "shop.example", "--dry-run")

	reason, details := decodeCLIError(t, err)
	assert.Equal(t, "invalid_flag", reason)
	assert.Contains(t, err.Error(), "there would be nothing to set up")
	assert.Equal(t, "Drop --no-api or --no-web.", details["fix"])
	assert.Empty(t, h.calls)
}

func TestPoppyPartsFlagsExcludeTheirOwnFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		with string
		// Only is the command that has the flag, when it is not both.
		only string
	}{
		{name: "--no-api with --api", args: []string{"--no-api", "--api", poppyTestAPI}, with: "--api"},
		{name: "--no-api with --scope", args: []string{"--no-api", "--scope", "checkout=Place orders."}, with: "--scope", only: "setup"},
		{name: "--no-web with --app", args: []string{"--no-web", "--app", poppySetupAppID}, with: "--app"},
		{name: "--no-web with --login-uri", args: []string{"--no-web", "--login-uri", "https://shop.example/login"}, with: "--login-uri", only: "setup"},
		{name: "--no-web with --web-unscoped", args: []string{"--no-web", "--web-unscoped"}, with: "--web-unscoped"},
	}

	for _, command := range []string{"setup", "config"} {
		for _, test := range tests {
			if test.only != "" && test.only != command {
				continue
			}

			t.Run(command+": "+test.name, func(t *testing.T) {
				h := newPoppySetupHarness(t, freshPoppyTenant())

				err := h.run(append([]string{command, "--company-domain", "shop.example"}, test.args...)...)

				reason, _ := decodeCLIError(t, err)
				assert.Equal(t, "invalid_flag", reason)
				assert.Contains(t, err.Error(), "cannot be used with "+test.with)
			})
		}
	}
}

func TestPoppySetupAsksTheParts(t *testing.T) {
	type answers struct{ api, web bool }

	run := func(t *testing.T, state poppyTenantState, a answers, args ...string) (*poppySetupHarness, []string, error) {
		t.Helper()

		h := newPoppySetupHarness(t, state)
		asked := installPoppyAsker(t, modeTerminal)

		previous := promptAsk.confirm
		promptAsk.confirm = func(message string, def bool) (bool, error) {
			switch message {
			case poppyAPIPartText:
				*asked = append(*asked, "API part")

				return a.api, nil
			case poppyWebPartText:
				*asked = append(*asked, "web part")

				return a.web, nil
			}

			return previous(message, def)
		}

		err := h.run(args...)

		// Only the part questions are of interest here: the others are covered by their own tests.
		var parts []string
		for _, question := range *asked {
			if strings.HasSuffix(question, " part") {
				parts = append(parts, question)
			}
		}

		return h, parts, err
	}

	base := func(t *testing.T) []string {
		return []string{"--company-domain", "shop.example", "--out", filepath.Join(t.TempDir(), "poppy.json"), "--dry-run"}
	}

	t.Run("a no to the web question skips the app question", func(t *testing.T) {
		h, asked, err := run(t, freshPoppyTenant(), answers{api: true, web: false},
			append([]string{"setup", "--api", poppyTestAPI, "--api-url", poppyTestAPI + "=" + poppyTestAPI + "/openapi.json"}, base(t)...)...)

		require.NoError(t, err)
		assert.Equal(t, []string{"web part"}, asked)
		assert.NotContains(t, h.out.String(), "App Shop Web")
		assert.Contains(t, h.out.String(), "--no-web")
	})

	t.Run("a no to the API question skips the API question and the OpenAPI URL", func(t *testing.T) {
		h, asked, err := run(t, freshPoppyTenant(), answers{api: false, web: true},
			append([]string{"setup", "--app", poppySetupAppID}, base(t)...)...)

		require.NoError(t, err)
		assert.Equal(t, []string{"API part"}, asked)
		assert.NotContains(t, h.out.String(), "API "+poppyTestAPI)
		assert.Contains(t, h.out.String(), "--no-api")
	})

	t.Run("a flag answers its question", func(t *testing.T) {
		_, asked, err := run(t, freshPoppyTenant(), answers{api: true, web: true},
			append([]string{"setup", "--no-web", "--api", poppyTestAPI, "--api-url", poppyTestAPI + "=" + poppyTestAPI + "/openapi.json"}, base(t)...)...)

		require.NoError(t, err)
		assert.Empty(t, asked)
	})

	t.Run("no to both is an error", func(t *testing.T) {
		_, asked, err := run(t, freshPoppyTenant(), answers{api: false, web: false}, append([]string{"setup"}, base(t)...)...)

		reason, _ := decodeCLIError(t, err)
		assert.Equal(t, "nothing_to_set_up", reason)
		assert.Equal(t, []string{"API part", "web part"}, asked)
	})

	t.Run("the config file answers the questions, and a flag wins over it", func(t *testing.T) {
		path := configFileWith(t, func(c *config.Config) { c.Web.Disabled = true })
		args := []string{"--company-domain", "shop.example", "--out", path, "--dry-run"}

		_, asked, err := run(t, configuredPoppyTenant(), answers{api: true, web: true},
			append([]string{"setup", "--api", poppyTestAPI}, args...)...)
		require.NoError(t, err)
		assert.Empty(t, asked, "web.disabled answers the web question, and --api the API one")

		_, asked, err = run(t, configuredPoppyTenant(), answers{api: true, web: true},
			append([]string{"setup", "--api", poppyTestAPI, "--app", poppySetupAppID}, args...)...)
		require.NoError(t, err)
		assert.Empty(t, asked, "--app turns the web part on, so web.disabled is not used")
	})

	t.Run("auth.disabled in the file answers the API question", func(t *testing.T) {
		path := configFileWith(t, func(c *config.Config) { c.Auth = config.Auth{Disabled: true, DisabledSet: true}; c.APIs = nil })

		_, asked, err := run(t, configuredPoppyTenant(), answers{api: true, web: true},
			"setup", "--company-domain", "shop.example", "--app", poppySetupAppID, "--out", path, "--dry-run")

		require.NoError(t, err)
		assert.Empty(t, asked)
	})

	t.Run("config asks the parts too", func(t *testing.T) {
		_, asked, err := run(t, configuredPoppyTenant(), answers{api: false, web: true},
			"config", "--company-domain", "shop.example", "--app", poppySetupAppID, "--out", filepath.Join(t.TempDir(), "poppy.json"))

		require.NoError(t, err)
		assert.Equal(t, []string{"API part"}, asked)
	})
}

func TestPoppySetupAsksNothingWithoutATerminal(t *testing.T) {
	h := newPoppySetupHarness(t, freshPoppyTenant())
	asked := installPoppyAsker(t, modePiped)

	err := h.run("setup", "--company-domain", "shop.example", "--api", poppyTestAPI, "--dry-run")

	reason, _ := decodeCLIError(t, err)
	assert.Equal(t, "missing_required_flags", reason, "a missing --app is still a missing value")
	assert.EqualError(t, err, "missing required flags: --app")
	assert.Empty(t, *asked)
}

// configFileWith writes the golden config file after the edit and returns its path.
func configFileWith(t *testing.T, edit func(*config.Config)) string {
	t.Helper()

	cfg, err := config.Read(goldenPoppyConfig(t))
	require.NoError(t, err)

	edit(cfg)

	path := filepath.Join(t.TempDir(), "poppy.json")
	require.NoError(t, config.Write(path, cfg))

	_, err = os.Stat(path)
	require.NoError(t, err)

	return path
}
