package cli

import (
	"errors"
	"io"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/display"
)

func TestCommandSuggestions(t *testing.T) {
	root := &cobra.Command{Use: "auth0"}
	apps := &cobra.Command{Use: "apps"}
	apps.AddCommand(&cobra.Command{Use: "list", RunE: func(*cobra.Command, []string) error { return nil }})
	grants := &cobra.Command{Use: "client-grants", Aliases: []string{"grants"}, SuggestFor: []string{"grant"}}
	grants.AddCommand(&cobra.Command{Use: "create", RunE: func(*cobra.Command, []string) error { return nil }})
	hidden := &cobra.Command{Use: "secret-grants", Hidden: true}
	login := &cobra.Command{Use: "login"}
	root.AddCommand(apps, grants, hidden, login)

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{name: "alias under the wrong parent", args: []string{"grants"}, want: []string{"auth0 client-grants"}},
		{name: "alias is case insensitive", args: []string{"GRANTS"}, want: []string{"auth0 client-grants"}},
		{name: "suggest for entry", args: []string{"grant"}, want: []string{"auth0 client-grants"}},
		{name: "hyphenated name typed in full", args: []string{"client-grants"}, want: []string{"auth0 client-grants"}},
		{name: "deeper command when the rest resolves", args: []string{"grants", "create"}, want: []string{"auth0 client-grants create"}},
		{name: "top command when the rest does not resolve", args: []string{"grants", "bogus"}, want: []string{"auth0 client-grants"}},
		{name: "typo keeps the sibling as a full path", args: []string{"lst"}, want: []string{"auth0 apps list"}},
		{name: "plain words are not matched across parents", args: []string{"login"}, want: nil},
		{name: "last word of a hyphenated name is not matched", args: []string{"secret"}, want: nil},
		{name: "no match", args: []string{"zzz"}, want: nil},
		{name: "hidden commands are skipped", args: []string{"secret-grants"}, want: nil},
		{name: "no args", args: nil, want: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.ElementsMatch(t, test.want, commandSuggestions(apps, test.args))
		})
	}
}

// TestUnknownSubcommandSuggestionsOnRealTree runs the actual command tree, so
// renaming or re-aliasing client-grants fails here rather than silently
// dropping the hint.
func TestUnknownSubcommandSuggestionsOnRealTree(t *testing.T) {
	testCLI := &cli{renderer: display.NewRenderer()}
	root := buildRootCmd(testCLI)
	addSubCommands(root, testCLI)
	enforceUnknownSubcommand(root)

	tests := []struct {
		name    string
		path    []string
		args    []string
		want    []string
		notWant []string
	}{
		{name: "apps grants", path: []string{"apps"}, args: []string{"grants"}, want: []string{"auth0 client-grants"}},
		{name: "apis grants", path: []string{"apis"}, args: []string{"grants"}, want: []string{"auth0 client-grants"}},
		{name: "singular form", path: []string{"apps"}, args: []string{"grant"}, want: []string{"auth0 client-grants"}},
		{name: "apps client-grants", path: []string{"apps"}, args: []string{"client-grants"}, want: []string{"auth0 client-grants"}},
		{name: "deeper command", path: []string{"apps"}, args: []string{"grants", "create"}, want: []string{"auth0 client-grants create"}},
		{name: "apps typo keeps the sibling suggestion", path: []string{"apps"}, args: []string{"lst"}, want: []string{"auth0 apps list"}, notWant: []string{"auth0 client-grants"}},
		{name: "generic words are not suggested", path: []string{"apps"}, args: []string{"login"}, notWant: []string{"auth0 login", "auth0 universal-login"}},
		{name: "settings is not tenant-settings", path: []string{"apps"}, args: []string{"settings"}, notWant: []string{"auth0 tenant-settings"}},
		{name: "help is not suggested", path: []string{"apps"}, args: []string{"help"}, notWant: []string{"auth0 help"}},
		{name: "unrelated token suggests nothing", path: []string{"apps"}, args: []string{"zzzzzz"}, notWant: []string{"auth0 client-grants"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd, _, err := root.Find(test.path)
			require.NoError(t, err)
			require.NotNil(t, cmd.Args, "namespace must reject unknown subcommands")

			err = cmd.Args(cmd, test.args)

			var unknownCmd unknownCommandError
			require.True(t, errors.As(err, &unknownCmd))

			for _, want := range test.want {
				assert.Contains(t, unknownCmd.suggestions, want)
			}
			for _, notWant := range test.notWant {
				assert.NotContains(t, unknownCmd.suggestions, notWant)
			}

			// The suggestion must reach the agent-facing JSON envelope.
			if len(test.want) > 0 {
				envelope := buildErrorEnvelope(err)
				require.NotNil(t, envelope.Error.Details)
				for _, want := range test.want {
					assert.Contains(t, string(envelope.Error.Details), want)
				}
			}
		})
	}
}

// TestUnknownSubcommandWithFlagsAndHelp covers the paths that fail before the
// Args validator runs: an unknown flag next to the mistyped token, and --help.
func TestUnknownSubcommandWithFlagsAndHelp(t *testing.T) {
	newRoot := func() (*cobra.Command, *cli) {
		testCLI := &cli{renderer: display.NewRenderer()}
		root := buildRootCmd(testCLI)
		addSubCommands(root, testCLI)
		enforceUnknownSubcommand(root)
		root.SilenceUsage = true
		root.SilenceErrors = true
		root.SetFlagErrorFunc(wrapFlagError)
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		return root, testCLI
	}

	t.Run("unknown flag keeps the suggestion", func(t *testing.T) {
		root, _ := newRoot()
		root.SetArgs([]string{"apps", "grants", "create", "--client-id", "abc", "--audience", "https://x"})

		err := root.Execute()

		var unknownCmd unknownCommandError
		require.True(t, errors.As(err, &unknownCmd))
		assert.Equal(t, "grants", unknownCmd.token)
		assert.Contains(t, unknownCmd.suggestions, "auth0 client-grants create")
		assert.Contains(t, err.Error(), "unknown flag: --client-id")
		assert.Contains(t, string(buildErrorEnvelope(err).Error.Details), "auth0 client-grants create")
	})

	t.Run("help on an unknown subcommand is reported", func(t *testing.T) {
		root, testCLI := newRoot()
		defaultHelp := root.HelpFunc()
		root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
			if unknownCmd, ok := unknownSubcommandFromHelp(cmd); ok {
				testCLI.helpErr = unknownCmd
				return
			}
			defaultHelp(cmd, args)
		})
		root.SetArgs([]string{"apps", "grants", "--help"})

		require.NoError(t, root.Execute())

		var unknownCmd unknownCommandError
		require.True(t, errors.As(testCLI.helpErr, &unknownCmd))
		assert.Equal(t, "grants", unknownCmd.token)
		assert.Contains(t, unknownCmd.suggestions, "auth0 client-grants")
	})

	t.Run("help on a namespace or leaf command is untouched", func(t *testing.T) {
		for _, args := range [][]string{{"apps", "--help"}, {"apps", "list", "--help"}} {
			root, testCLI := newRoot()
			root.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
				_, found := unknownSubcommandFromHelp(cmd)
				assert.False(t, found)
			})
			root.SetArgs(args)

			require.NoError(t, root.Execute())
			assert.NoError(t, testCLI.helpErr)
		}
	})
}
