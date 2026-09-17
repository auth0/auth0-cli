package cli

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/iostream"
)

// findSubcommand returns the node with the given name from a slice of nodes.
func findSubcommand(nodes []commandNode, name string) (commandNode, bool) {
	for _, n := range nodes {
		if n.Name == name {
			return n, true
		}
	}
	return commandNode{}, false
}

func newTestCommandTree() *cobra.Command {
	root := &cobra.Command{Use: "auth0"}
	// Mirror the real root's persistent flags so tests can tell a known global
	// flag (which still counts as implicit namespace help) apart from an unknown one.
	root.PersistentFlags().String("tenant", "", "Specific tenant to use.")
	root.PersistentFlags().Bool("debug", false, "Enable debug mode.")
	root.PersistentFlags().Bool("no-input", false, "Disable interactivity.")
	root.PersistentFlags().Bool("no-color", false, "Disable colors.")
	root.PersistentFlags().Bool("agent-mode", false, "Output JSON, disable prompts and colors.")

	apps := &cobra.Command{
		Use:   "apps",
		Short: "Manage resources for applications",
	}

	show := &cobra.Command{
		Use:   "show",
		Short: "Show an application",
		Long:  "Display the name, description, app type, and other information about an application.",
		Example: `  auth0 apps show
  auth0 apps show <app-id>
  auth0 apps show <app-id> --reveal-secrets`,
		Run: func(*cobra.Command, []string) {},
	}
	show.Flags().Bool("reveal-secrets", false, "Display the application secrets.")

	create := &cobra.Command{
		Use:   "create",
		Short: "Create a new application",
		Example: `  auth0 apps create
  auth0 apps create --name myapp --description <description>`,
		Run: func(*cobra.Command, []string) {},
	}
	create.Flags().String("name", "", "Name of the application.")
	create.Flags().String("description", "", "Description of the application.")

	apps.AddCommand(show)
	apps.AddCommand(create)
	root.AddCommand(apps)

	return root
}

func TestBuildCommandTree(t *testing.T) {
	root := newTestCommandTree()

	tree := buildCommandTree(root, 0, false)

	assert.Len(t, tree, 1)
	assert.Equal(t, "auth0 apps", tree[0].Path)
	assert.Equal(t, "apps", tree[0].Name)
	assert.False(t, tree[0].Runnable)
	assert.Len(t, tree[0].Subcommands, 2)

	show, ok := findSubcommand(tree[0].Subcommands, "show")
	assert.True(t, ok)
	assert.True(t, show.Runnable)
}

func TestBuildCommandTreeRespectsDepth(t *testing.T) {
	root := newTestCommandTree()

	tree := buildCommandTree(root, 1, false)

	assert.Len(t, tree, 1)
	assert.Empty(t, tree[0].Subcommands, "depth 1 should not include grandchildren")
}

func TestBuildCommandTreeDetailed(t *testing.T) {
	root := newTestCommandTree()

	tree := buildCommandTree(root, 0, true)
	show, ok := findSubcommand(tree[0].Subcommands, "show")
	assert.True(t, ok)

	assert.Equal(t, "Display the name, description, app type, and other information about an application.", show.Description)
	assert.Equal(t, []string{"<app-id>"}, show.Arguments)

	var flagNames []string
	for _, f := range show.Flags {
		flagNames = append(flagNames, f.Name)
	}
	assert.Contains(t, flagNames, "reveal-secrets")
	assert.NotContains(t, flagNames, "help", "the --help flag should be filtered out")
}

func TestFlattenCommands(t *testing.T) {
	root := newTestCommandTree()

	// Unscoped: only the runnable leaf commands, no group nodes.
	nodes := flattenCommands(root, false, false)

	var paths []string
	for _, n := range nodes {
		paths = append(paths, n.Path)
		assert.True(t, n.Runnable, "flat mode should only include runnable commands")
		assert.Empty(t, n.Subcommands, "flat nodes should not nest")
	}

	assert.ElementsMatch(t, []string{"auth0 apps show", "auth0 apps create"}, paths)
	assert.NotContains(t, paths, "auth0 apps", "the non-runnable group should be excluded")
}

func TestFlattenCommandsScopedIncludesRunnableStart(t *testing.T) {
	root := newTestCommandTree()

	apps, _, err := root.Find([]string{"apps", "show"})
	assert.NoError(t, err)

	// Scoped to a runnable leaf: it should include itself.
	nodes := flattenCommands(apps, true, false)
	assert.Len(t, nodes, 1)
	assert.Equal(t, "auth0 apps show", nodes[0].Path)
}

func TestExtractArgumentsIgnoresFlagValues(t *testing.T) {
	root := newTestCommandTree()

	tree := buildCommandTree(root, 0, true)
	create, ok := findSubcommand(tree[0].Subcommands, "create")
	assert.True(t, ok)

	// <description> is the value of the --description flag, not a positional
	// argument, so it must not appear in the arguments list.
	assert.Empty(t, create.Arguments)
}

func TestHasJSONRequest(t *testing.T) {
	assert.True(t, hasJSONRequest([]string{"apps", "list", "--json"}))
	assert.True(t, hasJSONRequest([]string{"apps", "list", "--json-compact"}))
	assert.False(t, hasJSONRequest([]string{"apps", "list", "--flat"}))
	assert.False(t, hasJSONRequest(nil))
}

func TestAnnotateWithRawAPINote(t *testing.T) {
	nodes := annotateWithRawAPINote([]commandNode{{Path: "auth0 apps"}, {Path: "auth0 users"}})

	for _, n := range nodes {
		assert.Equal(t, rawAPIFallbackNote, n.Note)
	}
}

// captureOutput redirects iostream.Output for the duration of fn and returns
// what was written.
func captureOutput(t *testing.T, fn func()) string {
	t.Helper()

	original := iostream.Output
	r, w, err := os.Pipe()
	assert.NoError(t, err)

	iostream.Output = w
	defer func() { iostream.Output = original }()

	fn()
	assert.NoError(t, w.Close())

	out, err := io.ReadAll(r)
	assert.NoError(t, err)

	return string(out)
}

func TestRenderNamespaceJSONHelp(t *testing.T) {
	// RenderNamespaceJSONHelp only handles the case Cobra's help func cannot reach:
	// an explicit --json/--json-compact on a namespace, which defines no such flag.
	// It resolves the target with Cobra's own parser rather than re-implementing pflag.
	fireTests := []struct {
		name     string
		args     []string
		wantPath string
		wantNote bool
	}{
		{"json on the root namespace", []string{"--json"}, "auth0", false},
		{"json on a group namespace", []string{"apps", "--json"}, "auth0 apps", true},
		{"json-compact on a group namespace", []string{"apps", "--json-compact"}, "auth0 apps", true},
		{"help and json together on a namespace", []string{"apps", "--help", "--json"}, "auth0 apps", true},
		{"space-separated global flag value is skipped", []string{"apps", "--tenant", "x.auth0.com", "--json"}, "auth0 apps", true},
		{"equals form global flag value is skipped", []string{"apps", "--tenant=x.auth0.com", "--json"}, "auth0 apps", true},
		{"known global flag with json", []string{"apps", "--debug", "--json"}, "auth0 apps", true},
	}

	for _, test := range fireTests {
		t.Run("fires: "+test.name, func(t *testing.T) {
			var fired bool
			out := captureOutput(t, func() {
				fired = renderNamespaceJSONHelp(newTestCommandTree(), test.args)
			})
			assert.True(t, fired)

			var nodes []commandNode
			assert.NoError(t, json.Unmarshal([]byte(out), &nodes))
			assert.Len(t, nodes, 1)
			assert.Equal(t, test.wantPath, nodes[0].Path)
			if test.wantNote {
				assert.Equal(t, rawAPIFallbackNote, nodes[0].Note, "a namespace's help is detailed")
			} else {
				assert.Empty(t, nodes[0].Note, "the root overview does not carry the note")
			}
		})
	}

	skipTests := []struct {
		name string
		args []string
	}{
		{"no json flag at all", []string{"apps"}},
		{"json on a leaf command falls through to Cobra", []string{"apps", "create", "--json"}},
		{"help and json on a leaf falls through to Cobra", []string{"apps", "create", "--help", "--json"}},
		{"mistyped subcommand leaves a positional", []string{"apps", "lst", "--json"}},
		{"positional after a space-separated flag value", []string{"apps", "--tenant", "foo", "lst", "--json"}},
		{"unknown flag on the namespace errors in parsing", []string{"apps", "--bogus", "--json"}},
		{"version wins over json", []string{"--version", "--json"}},
	}

	for _, test := range skipTests {
		t.Run("skips: "+test.name, func(t *testing.T) {
			var fired bool
			out := captureOutput(t, func() {
				fired = renderNamespaceJSONHelp(newTestCommandTree(), test.args)
			})
			assert.False(t, fired)
			assert.Empty(t, out)
		})
	}
}

func TestRenderCommandHelpJSON(t *testing.T) {
	root := newTestCommandTree()

	t.Run("root renders a compact overview with agent-mode prose and global flags", func(t *testing.T) {
		out := captureOutput(t, func() { renderCommandHelpJSON(root) })

		var nodes []commandNode
		assert.NoError(t, json.Unmarshal([]byte(out), &nodes))
		assert.Len(t, nodes, 1)
		assert.Equal(t, "auth0", nodes[0].Path)

		// The root help is the one place an agent learns the mode's output contract.
		assert.Contains(t, nodes[0].Description, agentModeHelp)
		assert.Contains(t, nodes[0].Description, "For Agents and Automation")

		_, hasAgentModeFlag := findFlag(nodes[0].Flags, "agent-mode")
		assert.True(t, hasAgentModeFlag, "the root help should list the global agent-mode flag")

		// The raw-API fallback note is for a specific command, not the overview.
		assert.Empty(t, nodes[0].Note, "the root overview should not carry the note")

		// The child tree stays compact (no per-command flags dumped).
		assert.NotEmpty(t, nodes[0].Subcommands)
		apps, ok := findSubcommand(nodes[0].Subcommands, "apps")
		assert.True(t, ok)
		assert.Empty(t, apps.Flags, "subcommands in the overview should not be detailed")
	})

	t.Run("a specific command renders detailed help carrying the note", func(t *testing.T) {
		create, _, err := root.Find([]string{"apps", "create"})
		assert.NoError(t, err)

		out := captureOutput(t, func() { renderCommandHelpJSON(create) })

		var nodes []commandNode
		assert.NoError(t, json.Unmarshal([]byte(out), &nodes))
		assert.Len(t, nodes, 1)
		assert.Equal(t, "auth0 apps create", nodes[0].Path)
		assert.NotEmpty(t, nodes[0].Flags, "a specific command's help should be detailed")
		assert.Equal(t, rawAPIFallbackNote, nodes[0].Note)
	})
}

// runWiredHelp mirrors Execute's help wiring over a test tree: it resolves agent mode,
// installs the JSON-aware help func, runs the namespace-json guard, then executes the
// command. It returns whatever was written to iostream.Output and the execution error,
// so tests can drive the real --help/bare-namespace paths end to end.
func runWiredHelp(t *testing.T, c *cli, args []string) (string, error) {
	t.Helper()

	root := newTestCommandTree()
	root.SilenceUsage = true
	root.SilenceErrors = true
	enforceUnknownSubcommand(root)
	root.SetFlagErrorFunc(wrapFlagError)

	c.agentMode = resolveAgentMode(c.agentClientName(), args)

	defaultHelpFunc := root.HelpFunc()
	root.SetHelpFunc(func(cmd *cobra.Command, a []string) {
		if !c.wantsJSONHelp() {
			defaultHelpFunc(cmd, a)
			return
		}
		renderCommandHelpJSON(cmd)
	})

	var execErr error
	out := captureOutput(t, func() {
		// Route Cobra's own (human) help to the captured stream too, so tests can
		// assert on it; the JSON path already writes to iostream.Output.
		root.SetOut(iostream.Output)
		root.SetErr(iostream.Output)
		if renderNamespaceJSONHelp(root, args) {
			return
		}
		root.SetArgs(args)
		execErr = root.Execute()
	})
	return out, execErr
}

func TestJSONHelpFunc(t *testing.T) {
	agent := func() *cli { return &cli{detectedAgent: "claude-code"} }
	human := func() *cli { return &cli{detectedAgent: "human"} }

	jsonPathTests := []struct {
		name     string
		cli      func() *cli
		args     []string
		wantPath string
	}{
		{"agent-mode --help on the root", agent, []string{"--help"}, "auth0"},
		{"agent-mode --help on a namespace", agent, []string{"apps", "--help"}, "auth0 apps"},
		{"agent-mode --help on a leaf", agent, []string{"apps", "create", "--help"}, "auth0 apps create"},
		{"agent-mode bare namespace via RunE", agent, []string{"apps"}, "auth0 apps"},
		{"agent-mode bare root via RunE", agent, []string{}, "auth0"},
		{"agent-mode help subcommand", agent, []string{"help", "apps"}, "auth0 apps"},
	}

	for _, test := range jsonPathTests {
		t.Run("json help: "+test.name, func(t *testing.T) {
			t.Setenv(agentModeEnvVar, "")
			out, err := runWiredHelp(t, test.cli(), test.args)
			assert.NoError(t, err)

			var nodes []commandNode
			assert.NoError(t, json.Unmarshal([]byte(out), &nodes))
			assert.Len(t, nodes, 1)
			assert.Equal(t, test.wantPath, nodes[0].Path)
		})
	}

	t.Run("human --help falls through to Cobra's help, not JSON", func(t *testing.T) {
		t.Setenv(agentModeEnvVar, "")
		out, err := runWiredHelp(t, human(), []string{"apps", "--help"})
		assert.NoError(t, err)

		var nodes []commandNode
		assert.Error(t, json.Unmarshal([]byte(out), &nodes), "human help is not JSON")
		assert.Contains(t, out, "Manage resources for applications")
	})

	usageErrTests := []struct {
		name string
		args []string
	}{
		{"unknown flag on a namespace", []string{"apps", "--bogus"}},
		{"mistyped subcommand", []string{"apps", "lst"}},
	}

	for _, test := range usageErrTests {
		t.Run("usage error: "+test.name, func(t *testing.T) {
			t.Setenv(agentModeEnvVar, "")
			_, err := runWiredHelp(t, agent(), test.args)
			assert.Error(t, err)

			var usageErr usageError
			assert.True(t, errors.As(err, &usageErr))
			assert.Equal(t, "usage", errorClass(err))
		})
	}
}

// findFlag returns the flag with the given name from a slice of flags.
func findFlag(flags []commandFlag, name string) (commandFlag, bool) {
	for _, f := range flags {
		if f.Name == name {
			return f, true
		}
	}
	return commandFlag{}, false
}

func TestRenderCommandTreeTextDetailed(t *testing.T) {
	root := newTestCommandTree()

	t.Run("without --detailed the tree omits invocation detail", func(t *testing.T) {
		out := captureOutput(t, func() {
			renderCommandTreeText(root, 0, false)
		})
		assert.Contains(t, out, "show")
		assert.NotContains(t, out, "usage:")
		assert.NotContains(t, out, "reveal-secrets")
	})

	t.Run("with --detailed each runnable command shows usage, args, auth and flags", func(t *testing.T) {
		out := captureOutput(t, func() {
			renderCommandTreeText(root, 0, true)
		})
		assert.Contains(t, out, "usage: auth0 apps show [flags]")
		assert.Contains(t, out, "args:  <app-id>")
		assert.Contains(t, out, "auth:  required")
		assert.Contains(t, out, "--reveal-secrets")
	})
}

func TestRenderCommandsFlatTextDetailed(t *testing.T) {
	root := newTestCommandTree()
	nodes := flattenCommands(root, false, true)

	t.Run("without --detailed only path and short are printed", func(t *testing.T) {
		out := captureOutput(t, func() {
			renderCommandsFlatText(nodes, false)
		})
		assert.Contains(t, out, "auth0 apps show")
		assert.NotContains(t, out, "usage:")
	})

	t.Run("with --detailed the invocation detail is printed under each command", func(t *testing.T) {
		out := captureOutput(t, func() {
			renderCommandsFlatText(nodes, true)
		})
		assert.Contains(t, out, "auth0 apps show")
		assert.Contains(t, out, "usage: auth0 apps show [flags]")
		assert.Contains(t, out, "--reveal-secrets")
	})
}
