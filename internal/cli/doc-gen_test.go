package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readDocs generates the docs of the tree into a new directory and returns the
// content of every page by file name.
func readDocs(t *testing.T, root *cobra.Command) map[string]string {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, GenMarkdownTree(root, dir))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	pages := make(map[string]string, len(entries))

	for _, entry := range entries {
		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		require.NoError(t, err)

		pages[entry.Name()] = string(content)
	}

	return pages
}

func TestGenMarkdownTree_LeavesOutThePluginCommands(t *testing.T) {
	root := newDocsRoot()

	group, _, err := root.Find([]string{"plugin"})
	require.NoError(t, err)
	require.NotSame(t, root, group, "the plugin group must be in the tree, or this test proves nothing")

	pages := readDocs(t, root)

	assert.Contains(t, pages, "auth0.md")
	assert.Contains(t, pages, "auth0_apps.md")

	for name, content := range pages {
		assert.False(t, strings.HasPrefix(name, "auth0_plugin"), "page %s", name)
		assert.NotContains(t, content, "auth0_plugin", "link in %s", name)
		assert.NotContains(t, content, "auth0 plugin", "mention in %s", name)
	}
}

func TestGenMarkdownTree_SkipsAnnotatedCommands(t *testing.T) {
	newTree := func() *cobra.Command {
		run := func(*cobra.Command, []string) {}
		skip := map[string]string{skipDocsAnnotation: "true"}

		root := &cobra.Command{Use: "auth0"}

		things := &cobra.Command{Use: "things", Long: "Things."}
		things.AddCommand(
			&cobra.Command{Use: "keep", Short: "Keep it", Run: run},
			&cobra.Command{Use: "hide", Short: "Hide it", Run: run, Annotations: skip},
		)

		onlyHidden := &cobra.Command{Use: "only-hidden", Long: "Only hidden."}
		onlyHidden.AddCommand(&cobra.Command{Use: "hide", Short: "Hide it", Run: run, Annotations: skip})

		hidden := &cobra.Command{Use: "secret", Short: "Secret group", Annotations: skip}
		hidden.AddCommand(&cobra.Command{Use: "child", Short: "Child", Run: run})

		root.AddCommand(things, onlyHidden, hidden)

		return root
	}

	pages := readDocs(t, newTree())

	assert.Contains(t, pages, "auth0_things_keep.md")
	assert.NotContains(t, pages, "auth0_things_hide.md")
	assert.NotContains(t, pages, "auth0_only-hidden_hide.md")
	assert.NotContains(t, pages, "auth0_secret.md")
	assert.NotContains(t, pages, "auth0_secret_child.md")

	assert.NotContains(t, pages["auth0.md"], "auth0_secret")
	assert.Contains(t, pages["auth0.md"], "auth0_things.md")

	assert.Contains(t, pages["auth0_things.md"], "auth0_things_keep.md")
	assert.NotContains(t, pages["auth0_things.md"], "auth0_things_hide")
	assert.NotContains(t, pages["auth0_things_keep.md"], "auth0_things_hide")

	assert.NotContains(t, pages["auth0_only-hidden.md"], "has_children")
	assert.NotContains(t, pages["auth0_only-hidden.md"], "auth0_only-hidden_hide")
}
