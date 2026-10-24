package store_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// junction makes a directory junction, which any account can do.
func junction(t *testing.T, target, link string) {
	t.Helper()

	output, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()

	require.NoError(t, err, string(output))
}

func TestVerify_RefusesADirectoryThatIsAJunction(t *testing.T) {
	t.Run("the version directory", func(t *testing.T) {
		f := installed(t)

		moved := f.versionDir("1.0.0") + ".real"
		require.NoError(t, os.Rename(f.versionDir("1.0.0"), moved))
		junction(t, moved, f.versionDir("1.0.0"))

		requireRefused(t, f, f.versionDir("1.0.0")+" is not a directory")
	})

	t.Run("the plugin directory", func(t *testing.T) {
		f := installed(t)

		pluginDir := filepath.Dir(f.versionDir("1.0.0"))
		moved := pluginDir + ".real"
		require.NoError(t, os.Rename(pluginDir, moved))
		junction(t, moved, pluginDir)

		requireRefused(t, f, pluginDir+" is not a directory")
	})
}

// The link count is read from the file that was opened, so it neither fails nor
// is failed by a process that has the file open without sharing it for deletion,
// as a running plugin or a virus scanner does.
func TestVerify_ReadsTheLinkCountOfAFileThatIsOpen(t *testing.T) {
	f := installed(t)

	for _, name := range []string{testBin, "plugin.json"} {
		holder, err := os.Open(filepath.Join(f.versionDir("1.0.0"), name))
		require.NoError(t, err)

		t.Cleanup(func() { _ = holder.Close() })
	}

	_, err := f.store.Verify(testID)

	require.NoError(t, err)
}
