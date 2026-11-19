package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/poppy/setup"
)

func TestPoppyWebUnscopedFlagIsOnSetupAndConfigOnly(t *testing.T) {
	root := newPoppyTestRoot()

	for _, path := range [][]string{{"poppy", "setup"}, {"poppy", "config"}} {
		cmd, _, err := root.Find(path)
		require.NoError(t, err)

		flag := cmd.Flags().Lookup("web-unscoped")
		require.NotNil(t, flag, strings.Join(path, " "))
		assert.Contains(t, flag.Usage, "poppy:browser-sign-in")
	}
}

func TestAskPoppyWebUnscoped(t *testing.T) {
	fileWith := func(unscoped, set bool) *setup.State {
		return &setup.State{File: &config.Config{Web: config.Web{Unscoped: unscoped, UnscopedSet: set}}}
	}

	tests := []struct {
		name        string
		interactive bool
		state       *setup.State
		in          setup.Inputs
		answer      bool
		wantAsked   bool
		wantNote    bool
		want        *bool
	}{
		{name: "no terminal asks nothing", interactive: false, state: &setup.State{}, want: nil},
		{name: "yes", interactive: true, state: &setup.State{}, answer: true, wantAsked: true, wantNote: true, want: testPtr(true)},
		{name: "no, which is then written as false", interactive: true, state: &setup.State{}, answer: false, wantAsked: true, want: testPtr(false)},
		{name: "the file has no value", interactive: true, state: fileWith(false, false), answer: true, wantAsked: true, wantNote: true, want: testPtr(true)},
		{name: "the file says false", interactive: true, state: fileWith(false, true), want: nil},
		{name: "the file says true", interactive: true, state: fileWith(true, true), want: nil},
		{name: "the flag was passed as true", interactive: true, state: &setup.State{}, in: setup.Inputs{WebUnscoped: testPtr(true)}, want: testPtr(true)},
		{name: "the flag was passed as false", interactive: true, state: &setup.State{}, in: setup.Inputs{WebUnscoped: testPtr(false)}, want: testPtr(false)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var asked bool

			screen, out := newTestPromptScreen(t, promptAsker{
				confirm: func(message string, _ bool) (bool, error) {
					asked = true

					assert.Equal(t, poppyWebUnscopedText, message)

					return test.answer, nil
				},
			})

			in := test.in

			require.NoError(t, askPoppyWebUnscoped(screen, test.interactive, test.state, &in))
			assert.Equal(t, test.wantAsked, asked, "asked")
			assert.Equal(t, test.wantNote, strings.Contains(out.String(), poppyWebUnscopedNote), "note after a yes only")
			assert.Equal(t, test.want, in.WebUnscoped)
		})
	}

	t.Run("a failure of the question ends it and sets nothing", func(t *testing.T) {
		screen, out := newTestPromptScreen(t, promptAsker{
			confirm: func(string, bool) (bool, error) { return false, terminal.InterruptErr },
		})

		var in setup.Inputs

		require.ErrorIs(t, askPoppyWebUnscoped(screen, true, &setup.State{}, &in), terminal.InterruptErr)
		assert.Nil(t, in.WebUnscoped)
		assert.NotContains(t, out.String(), poppyWebUnscopedNote)
	})
}

func TestPoppyWebUnscopedWording(t *testing.T) {
	assert.Equal(t, "Web Browsing permissions", poppyWebUnscopedTitle)
	assert.Contains(t, poppyWebUnscopedText, "Answer no to limit access to the permissions the end user approves.")
	assert.Contains(t, poppyWebUnscopedText, "poppy:browser-sign-in")
	assert.Contains(t, poppyWebUnscopedText, "web.unscoped is set to true")
}

// webAnswer is what a person does at the question about web.unscoped: a bool
// answers it, an error is Ctrl-C.
type webAnswer any

// runWebMode runs a command in a mode with the question about web.unscoped
// scripted through the confirm seam, so that the real prompts and the screen
// handle it. It returns the run and how many times the question was asked.
func (h *poppySetupHarness) runWebMode(t *testing.T, mode poppyPromptMode, args []string, answer webAnswer) (poppyPromptRun, int) {
	t.Helper()

	installPoppyAsker(t, mode)

	asked := 0
	previous := promptAsk.confirm

	promptAsk.interrupt = func(err error) error { return err }
	promptAsk.confirm = func(message string, def bool) (bool, error) {
		if message != poppyWebUnscopedText {
			return previous(message, def)
		}

		asked++

		if err, ok := answer.(error); ok {
			return false, err
		}

		return answer.(bool), nil
	}

	h.agentMode = mode.agentMode

	err := h.run(append(args, mode.flags...)...)

	return poppyPromptRun{out: h.out.String(), errOut: h.messages.String(), err: err}, asked
}

// poppyConfigWithWeb returns a copy of the golden config file for the configured
// tenant with the web member written as given, or untouched when member is empty.
func poppyConfigWithWeb(t *testing.T, member string) string {
	t.Helper()

	path := goldenPoppyConfig(t)
	if member == "" {
		return path
	}

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	raw := strings.Replace(string(data), `"browser_session_endpoint"`, member+`, "browser_session_endpoint"`, 1)
	require.NotEqual(t, string(data), raw)
	require.NoError(t, os.WriteFile(path, []byte(raw), 0o600))

	return path
}

// unscopedIn reads the web.unscoped member of a config file: its value and
// whether the file has it.
func unscopedIn(t *testing.T, path string) (value, present bool) {
	t.Helper()

	cfg, err := config.Read(path)
	require.NoError(t, err)

	return cfg.Web.Unscoped, cfg.Web.UnscopedSet
}

func TestPoppySetupWebUnscoped(t *testing.T) {
	// Fresh tenant, no file: the question is asked whenever a terminal is there and
	// no flag answers it.
	tests := []struct {
		name      string
		mode      poppyPromptMode
		flags     []string
		answer    webAnswer
		wantAsked int
		// The member in the written file: its value, and whether it is there.
		wantValue   bool
		wantPresent bool
	}{
		{name: "terminal, yes", mode: modeTerminal, answer: true, wantAsked: 1, wantValue: true, wantPresent: true},
		{name: "terminal, no is written as false", mode: modeTerminal, answer: false, wantAsked: 1, wantPresent: true},
		{name: "terminal with --json still asks", mode: modeTerminalJSON, answer: true, wantAsked: 1, wantValue: true, wantPresent: true},
		{name: "the flag answers it, true", mode: modeTerminal, flags: []string{"--web-unscoped"}, answer: false, wantValue: true, wantPresent: true},
		{name: "the flag answers it, explicit true", mode: modeTerminal, flags: []string{"--web-unscoped=true"}, answer: false, wantValue: true, wantPresent: true},
		{name: "the flag answers it, false", mode: modeTerminal, flags: []string{"--web-unscoped=false"}, answer: true, wantPresent: true},
		{name: "terminal with --no-input asks nothing and leaves it absent", mode: modeTerminalNoInput, answer: true},
		{name: "piped asks nothing and leaves it absent", mode: modePiped, answer: true},
		{name: "piped with --json asks nothing", mode: modePipedJSON, answer: true},
		{name: "agent mode asks nothing", mode: modeAgent, answer: true},
		{name: "piped uses the flag", mode: modePiped, flags: []string{"--web-unscoped"}, answer: true, wantValue: true, wantPresent: true},
		{name: "piped uses the flag set to false", mode: modePiped, flags: []string{"--web-unscoped=false"}, answer: true, wantPresent: true},
		{name: "agent mode uses the flag", mode: modeAgent, flags: []string{"--web-unscoped"}, answer: true, wantValue: true, wantPresent: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := newPoppySetupHarness(t, freshPoppyTenant())
			h.allowWrites()

			args := freshSetupArgs(t, append([]string{"--force"}, test.flags...)...)

			run, asked := h.runWebMode(t, test.mode, args, test.answer)

			require.NoError(t, run.err)
			assert.Equal(t, test.wantAsked, asked, "times asked")

			value, present := unscopedIn(t, outOf(args))
			assert.Equal(t, test.wantValue, value, "web.unscoped")
			assert.Equal(t, test.wantPresent, present, "web.unscoped is in the file")

			if test.wantAsked > 0 && test.answer == true {
				assert.Contains(t, run.errOut, poppyWebUnscopedNote, "the note after a yes")
			} else {
				assert.NotContains(t, run.errOut, poppyWebUnscopedNote)
			}
		})
	}
}

func TestPoppySetupWebUnscopedWithAFile(t *testing.T) {
	tests := []struct {
		name        string
		member      string
		flags       []string
		answer      webAnswer
		wantAsked   int
		wantValue   bool
		wantPresent bool
	}{
		{name: "the file has no value: asked, yes", member: "", answer: true, wantAsked: 1, wantValue: true, wantPresent: true},
		{name: "the file has no value: asked, no", member: "", answer: false, wantAsked: 1, wantPresent: true},
		{name: "the file says false: not asked, kept", member: `"unscoped": false`, answer: true, wantPresent: true},
		{name: "the file says true: not asked, kept", member: `"unscoped": true`, answer: false, wantValue: true, wantPresent: true},
		{name: "the file says false, the flag says true", member: `"unscoped": false`, flags: []string{"--web-unscoped"}, answer: false, wantValue: true, wantPresent: true},
		{name: "the file says true, the flag says false", member: `"unscoped": true`, flags: []string{"--web-unscoped=false"}, answer: true, wantPresent: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := newPoppySetupHarness(t, configuredPoppyTenant())
			h.allowWrites()

			path := poppyConfigWithWeb(t, test.member)
			args := []string{"setup", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID,
				"--out", path, "--force"}

			run, asked := h.runWebMode(t, modeTerminal, append(args, test.flags...), test.answer)

			require.NoError(t, run.err)
			assert.Equal(t, test.wantAsked, asked, "times asked")

			value, present := unscopedIn(t, path)
			assert.Equal(t, test.wantValue, value, "web.unscoped")
			assert.Equal(t, test.wantPresent, present, "web.unscoped is in the file")
		})
	}
}

func TestPoppySetupWebUnscopedAsksAfterTheHints(t *testing.T) {
	h := newPoppySetupHarness(t, freshPoppyTenant())
	h.allowWrites()

	asked := installPoppyAsker(t, modeTerminal)

	require.NoError(t, h.run(freshSetupArgs(t, "--force")...))

	assert.Equal(t, []string{"hints", "web access"}, *asked)
}

func TestPoppySetupWebUnscopedDryRun(t *testing.T) {
	h := newPoppySetupHarness(t, freshPoppyTenant())
	args := freshSetupArgs(t, "--dry-run")

	run, asked := h.runWebMode(t, modeTerminal, args, true)

	require.NoError(t, run.err)
	assert.Equal(t, 1, asked, "a dry run in a terminal asks as a real run does")
	assert.Contains(t, run.out, "Your answers")
	assert.Contains(t, run.out, "Web access    web.unscoped true")
	assert.Contains(t, run.out, "--web-unscoped=true", "the command to apply carries the answer")
	assert.NoFileExists(t, outOf(args), "a dry run writes nothing")
}

func TestPoppySetupWebUnscopedDryRunFromTheFlag(t *testing.T) {
	h := newPoppySetupHarness(t, configuredPoppyTenant())
	path := poppyConfigWithWeb(t, "")
	before, err := os.ReadFile(path)
	require.NoError(t, err)

	args := []string{"setup", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID,
		"--out", path, "--dry-run", "--web-unscoped"}

	run, asked := h.runWebMode(t, modePiped, args, true)

	require.NoError(t, run.err)
	assert.Zero(t, asked)
	assert.Contains(t, run.out, "Web access    web.unscoped true")
	assert.Contains(t, run.out, "updated", "the plan shows the file as changed, because only web.unscoped differs")

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after))
}

func TestPoppySetupWebUnscopedCtrlC(t *testing.T) {
	h := newPoppySetupHarness(t, freshPoppyTenant())
	h.allowWrites()

	args := freshSetupArgs(t, "--force")

	run, asked := h.runWebMode(t, modeTerminal, args, terminal.InterruptErr)

	require.ErrorIs(t, run.err, terminal.InterruptErr)
	assert.Equal(t, 1, asked)
	assert.Contains(t, run.errOut, display.PromptCancelled)
	assert.Empty(t, h.calls, "the tenant was not changed")
	assert.NoFileExists(t, outOf(args))
}
