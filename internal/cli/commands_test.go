package cli

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
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
