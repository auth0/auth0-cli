package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A file that is open cannot be deleted on Windows, which is the case for the
// executable of a plugin that is running. Go opens a file for reading in a way
// that does not let it be deleted, as the system does for a running program.
func TestRemove_ExplainsAPluginThatIsInUse(t *testing.T) {
	f := installed(t)

	holder, err := os.Open(filepath.Join(f.versionDir("1.0.0"), testBin))
	require.NoError(t, err)

	t.Cleanup(func() { _ = holder.Close() })

	_, err = f.store.Remove(t.Context(), testID)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "checkmate was uninstalled, but not everything was removed")
	assert.Contains(t, err.Error(), "failed to delete "+filepath.Dir(f.versionDir("1.0.0")))
	assert.Contains(t, err.Error(), "the plugin is running or in use")

	require.NoError(t, holder.Close())

	// The plugin is no longer installed, and its files are left for the person to
	// delete.
	assert.DirExists(t, f.versionDir("1.0.0"))
}
