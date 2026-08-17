package cli

import (
	"encoding/json"
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

func TestHasHelpRequest(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"long help flag", []string{"apps", "create", "--help"}, true},
		{"short help flag", []string{"apps", "-h"}, true},
		{"help subcommand in command position", []string{"help", "apps"}, true},
		{"no help request", []string{"apps", "create", "--name", "x"}, false},
		{"help as a flag value is not a request", []string{"apps", "create", "--name", "help"}, false},
		{"help not in command position is not the subcommand", []string{"apps", "help"}, false},
		{"empty args", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, hasHelpRequest(tt.args))
		})
	}
}

func TestHasJSONRequest(t *testing.T) {
	assert.True(t, hasJSONRequest([]string{"apps", "list", "--json"}))
	assert.False(t, hasJSONRequest([]string{"apps", "list", "--flat"}))
	assert.False(t, hasJSONRequest(nil))
}

func TestAgentModeEnabled(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"1", true},
		{"true", true},
		{"  true  ", true},
		{"0", false},
		{"false", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Setenv(agentModeEnvVar, tt.value)
			assert.Equal(t, tt.want, agentModeEnabled())
		})
	}
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

func TestRenderJSONHelpIfRequested(t *testing.T) {
	root := newTestCommandTree()

	t.Run("does not fire without a help request", func(t *testing.T) {
		t.Setenv(agentModeEnvVar, "")
		var fired bool
		out := captureOutput(t, func() {
			fired = renderJSONHelpIfRequested(&cli{}, root, []string{"apps", "create"})
		})
		assert.False(t, fired)
		assert.Empty(t, out)
	})

	t.Run("does not fire for help without json or env", func(t *testing.T) {
		t.Setenv(agentModeEnvVar, "")
		var fired bool
		out := captureOutput(t, func() {
			fired = renderJSONHelpIfRequested(&cli{}, root, []string{"apps", "create", "--help"})
		})
		assert.False(t, fired)
		assert.Empty(t, out)
	})

	t.Run("explicit --help --json on a leaf is detailed and carries the note", func(t *testing.T) {
		t.Setenv(agentModeEnvVar, "")
		var fired bool
		out := captureOutput(t, func() {
			fired = renderJSONHelpIfRequested(&cli{}, root, []string{"apps", "create", "--help", "--json"})
		})
		assert.True(t, fired)

		var nodes []commandNode
		assert.NoError(t, json.Unmarshal([]byte(out), &nodes))
		assert.Len(t, nodes, 1)
		assert.Equal(t, "auth0 apps create", nodes[0].Path)
		assert.NotEmpty(t, nodes[0].Flags, "a specific command's help should be detailed")
		assert.Equal(t, rawAPIFallbackNote, nodes[0].Note)
	})

	t.Run("agent mode help needs no --json flag", func(t *testing.T) {
		var fired bool
		out := captureOutput(t, func() {
			fired = renderJSONHelpIfRequested(&cli{agentMode: true}, root, []string{"apps", "create", "--help"})
		})
		assert.True(t, fired)

		var nodes []commandNode
		assert.NoError(t, json.Unmarshal([]byte(out), &nodes))
		assert.Equal(t, "auth0 apps create", nodes[0].Path)
	})

	t.Run("root help is a compact overview without the note", func(t *testing.T) {
		t.Setenv(agentModeEnvVar, "")
		var fired bool
		out := captureOutput(t, func() {
			fired = renderJSONHelpIfRequested(&cli{}, root, []string{"--help", "--json"})
		})
		assert.True(t, fired)

		var nodes []commandNode
		assert.NoError(t, json.Unmarshal([]byte(out), &nodes))
		assert.Len(t, nodes, 1)
		assert.Equal(t, "auth0", nodes[0].Path)
		assert.Empty(t, nodes[0].Flags, "the root overview should not be detailed")
		assert.Empty(t, nodes[0].Note, "the root overview should not carry the note")
	})
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
