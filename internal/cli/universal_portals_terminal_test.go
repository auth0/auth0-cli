package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// markedPortalsStyle wraps what it paints in a tag named after the style, so a test can see what was painted.
func markedPortalsStyle() portalsStyle {
	mark := func(name string) func(string) string {
		return func(text string) string { return "<" + name + ">" + text + "</" + name + ">" }
	}

	return portalsStyle{
		bold: mark("bold"), faint: mark("faint"), green: mark("green"), yellow: mark("yellow"),
		red: mark("red"), blue: mark("blue"), cyan: mark("cyan"),
		link: func(url, text string) string { return "<link " + url + ">" + text + "</link>" },
	}
}

func newTestTerminal(style portalsStyle, silent bool) (*portalsTerminal, *bytes.Buffer) {
	var out bytes.Buffer

	return newPortalsTerminal(&out, style, silent, false), &out
}

// drawEverything makes every kind of line, in the order of a setup.
func drawEverything(terminal *portalsTerminal) {
	terminal.Banner("Universal Portals setup", "acme.us.auth0.com")
	terminal.Section("Permissions")
	terminal.Result("The CLI session has every permission setup needs")
	terminal.Section("Resource servers")
	terminal.Item(`"Auth0 My Account API"`, "already exists")
	terminal.Item(`"Auth0 My Organization API"`, "created")
	terminal.Result("Resource servers ready")
	terminal.Section("Application")
	terminal.Item(`"Acme (Universal Portals)"`, "created")
	terminal.Reference("Application", "000000000000000000000000000003", "https://manage.auth0.com/x")
	terminal.Warn(`The slug "acme" is already taken.`)
	terminal.Info("Replace <ORGANIZATION_ID> in the link with the organization ID to test with.")
	terminal.Error("Missing scope")
	terminal.Detail("Run the same command again to resume.")
	terminal.Fixed("P003 legacy sidebar links", "converted to v2")
	terminal.Table(
		[]string{"", "ID", "Check", "Fix"},
		[][]string{
			{"✖", "P001", "No home page", "Set the home page"},
			{"▲", "P006", "Empty page", "Add components"},
		},
	)
	terminal.Ready("https://acme.us.auth0.com/portals/acme")
	terminal.Cancelled("Setup cancelled.")
}

// The expected output is what the Clack based tool printed for the same calls, captured from it.
func TestPortalsTerminal_LooksLikeTheToolItReplaces(t *testing.T) {
	t.Parallel()

	terminal, out := newTestTerminal(plainPortalsStyle(), false)

	drawEverything(terminal)

	expected := []string{
		"│ ╭─Universal Portals setup──╮",
		"│ │    acme.us.auth0.com     │",
		"│ ╰──────────────────────────╯",
		"│",
		"◇  Permissions",
		"│",
		"◆  The CLI session has every permission setup needs",
		"│",
		"◇  Resource servers",
		"│",
		"│  \"Auth0 My Account API\" already exists",
		"│",
		"│  \"Auth0 My Organization API\" created",
		"│",
		"◆  Resource servers ready",
		"│",
		"◇  Application",
		"│",
		"│  \"Acme (Universal Portals)\" created",
		"│  Application: 000000000000000000000000000003",
		"▲  The slug \"acme\" is already taken.",
		"●  Replace <ORGANIZATION_ID> in the link with the organization ID to test with.",
		"■  Missing scope",
		"│  Run the same command again to resume.",
		"│",
		"│  ✓ P003 legacy sidebar links converted to v2",
		"│     ID    Check         Fix              ",
		"│  ✖  P001  No home page  Set the home page",
		"│  ▲  P006  Empty page    Add components   ",
		"│",
		"│",
		"◇  Portal URL ──────────────────────────────────────────╮",
		"│                                                       │",
		"│  Open portal: https://acme.us.auth0.com/portals/acme  │",
		"│                                                       │",
		"├───────────────────────────────────────────────────────╯",
		"└  Setup cancelled.",
		"",
		"",
	}
	assert.Equal(t, strings.Join(expected, "\n"), out.String())
}

func TestPortalsTerminal_Colors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		draw func(terminal *portalsTerminal)
		want string
	}{
		{
			name: "the title is bold and the tenant is cyan, inside a faint box",
			draw: func(terminal *portalsTerminal) { terminal.Banner("Setup", "acme.us.auth0.com") },
			want: "<faint>│</faint> <faint>╭────────</faint><bold>Setup</bold><faint>─────────╮</faint>\n" +
				"<faint>│</faint> <faint>│</faint>  <cyan>acme.us.auth0.com</cyan>   <faint>│</faint>\n" +
				"<faint>│</faint> <faint>╰──────────────────────╯</faint>\n",
		},
		{
			name: "a section is a green step after a faint rail",
			draw: func(terminal *portalsTerminal) { terminal.Section("Forms") },
			want: "<faint>│</faint>\n<green>◇</green>  Forms\n",
		},
		{
			name: "an outcome is faint",
			draw: func(terminal *portalsTerminal) { terminal.Item(`"API"`, "created") },
			want: "<faint>│</faint>\n<faint>│</faint>  \"API\" <faint>created</faint>\n",
		},
		{
			name: "a line that needs attention has its own color",
			draw: func(terminal *portalsTerminal) {
				terminal.Info("i")
				terminal.Warn("w")
				terminal.Error("e")
			},
			want: "<blue>●</blue>  i\n<yellow>▲</yellow>  w\n<red>■</red>  e\n",
		},
		{
			name: "a fix has a green check",
			draw: func(terminal *portalsTerminal) { terminal.Fixed("P003 links", "converted") },
			want: "<faint>│</faint>\n<faint>│</faint>  <green>✓</green> P003 links <faint>converted</faint>\n",
		},
		{
			name: "an ID links to its page",
			draw: func(terminal *portalsTerminal) { terminal.Reference("Application", "abc", "https://x/y") },
			want: "<faint>│</faint>  <faint>Application:</faint> <link https://x/y><cyan>abc</cyan></link>\n",
		},
		{
			name: "an error row is red and a warning row is yellow",
			draw: func(terminal *portalsTerminal) {
				terminal.Table([]string{"", "ID"}, [][]string{{"✖", "P1"}, {"▲", "P2"}})
			},
			want: "<faint>│</faint>  <faint>   ID</faint>\n" +
				"<faint>│</faint>  <red>✖</red>  P1\n" +
				"<faint>│</faint>  <yellow>▲</yellow>  P2\n",
		},
		{
			name: "the closing line is red after a faint end mark",
			draw: func(terminal *portalsTerminal) { terminal.Cancelled("Setup cancelled.") },
			want: "<faint>└</faint>  <red>Setup cancelled.</red>\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			terminal, out := newTestTerminal(markedPortalsStyle(), false)

			tt.draw(terminal)

			assert.Equal(t, tt.want, out.String())
		})
	}
}

func TestPortalsTerminal_LinesAfterTheFirstStayOnTheRail(t *testing.T) {
	t.Parallel()

	terminal, out := newTestTerminal(plainPortalsStyle(), false)

	terminal.Warn("first line\nsecond line")
	terminal.Detail("one\ntwo")

	assert.Equal(t, "▲  first line\n│  second line\n│  one\n│  two\n", out.String())
}

func TestPortalsTerminal_SilentDrawsNothing(t *testing.T) {
	t.Parallel()

	terminal, out := newTestTerminal(plainPortalsStyle(), true)

	drawEverything(terminal)

	assert.Empty(t, out.String())
}

func TestPortalsTerminal_Task(t *testing.T) {
	t.Parallel()

	t.Run("without a terminal it runs the work and draws nothing", func(t *testing.T) {
		t.Parallel()

		terminal, out := newTestTerminal(plainPortalsStyle(), false)
		ran := false

		err := terminal.Task("Creating application", func() error {
			ran = true
			return nil
		})

		require.NoError(t, err)
		assert.True(t, ran)
		assert.Empty(t, out.String())

		terminal.Result("Done")
		assert.Equal(t, "│\n◆  Done\n", out.String(), "the blank line before a result is still drawn")
	})

	t.Run("it returns the error of the work", func(t *testing.T) {
		t.Parallel()

		terminal, _ := newTestTerminal(plainPortalsStyle(), false)
		failure := errors.New("boom")

		assert.ErrorIs(t, terminal.Task("Creating portal", func() error { return failure }), failure)
	})
}

func TestPortalsTerminal_ASpinnerLeavesTheBlankLineBeforeTheNextLine(t *testing.T) {
	t.Parallel()

	terminal, out := newTestTerminal(plainPortalsStyle(), false)
	terminal.spinnerGap = true

	terminal.Result("Done")
	terminal.Result("Done again")

	assert.Equal(t, "◆  Done\n│\n◆  Done again\n", out.String(), "only the first line skips the blank line")
}

func TestAnsiPortalsStyle_KeepsTheText(t *testing.T) {
	t.Parallel()

	style := ansiPortalsStyle()

	// Whether the text is painted depends on the terminal and on --no-color, but it is always there.
	for name, paint := range map[string]func(string) string{
		"bold": style.bold, "faint": style.faint, "green": style.green, "yellow": style.yellow,
		"red": style.red, "blue": style.blue, "cyan": style.cyan,
	} {
		assert.Contains(t, paint("text"), "text", name)
	}

	assert.Contains(t, style.link("https://example.com", "text"), "text")
}
