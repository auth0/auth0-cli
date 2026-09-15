package plugins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(filepath.Join(t.TempDir(), "plugins.json"))
}

func TestStoreListEmptyWhenMissing(t *testing.T) {
	store := newTestStore(t)

	plugins, err := store.List()
	require.NoError(t, err)
	assert.Empty(t, plugins)
}

func TestStoreListEmptyWhenFileEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plugins.json")
	require.NoError(t, os.WriteFile(path, []byte(""), 0o600))

	plugins, err := NewStore(path).List()
	require.NoError(t, err)
	assert.Empty(t, plugins)
}

func TestStoreAddGetRemove(t *testing.T) {
	store := newTestStore(t)

	checkmate := InstalledPlugin{
		Name:        "checkmate",
		Version:     "1.2.3",
		InstallType: InstallNPM,
		Package:     "checkmate@1.2.3",
	}
	require.NoError(t, store.Add(checkmate))

	got, ok, err := store.Get("checkmate")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, checkmate, got)

	_, ok, err = store.Get("missing")
	require.NoError(t, err)
	assert.False(t, ok)

	require.NoError(t, store.Remove("checkmate"))
	_, ok, err = store.Get("checkmate")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestStoreAddReplacesByName(t *testing.T) {
	store := newTestStore(t)

	require.NoError(t, store.Add(InstalledPlugin{Name: "checkmate", Version: "1.0.0", InstallType: InstallNPM}))
	require.NoError(t, store.Add(InstalledPlugin{Name: "checkmate", Version: "2.0.0", InstallType: InstallNPM}))

	plugins, err := store.List()
	require.NoError(t, err)
	require.Len(t, plugins, 1)
	assert.Equal(t, "2.0.0", plugins[0].Version)
}

func TestStoreListSortedByName(t *testing.T) {
	store := newTestStore(t)

	require.NoError(t, store.Add(InstalledPlugin{Name: "widget", InstallType: InstallGitHubRelease}))
	require.NoError(t, store.Add(InstalledPlugin{Name: "checkmate", InstallType: InstallNPM}))

	plugins, err := store.List()
	require.NoError(t, err)
	require.Len(t, plugins, 2)
	assert.Equal(t, "checkmate", plugins[0].Name)
	assert.Equal(t, "widget", plugins[1].Name)
}

func TestStorePersistsAcrossInstances(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plugins.json")

	require.NoError(t, NewStore(path).Add(InstalledPlugin{Name: "checkmate", InstallType: InstallNPM}))

	plugins, err := NewStore(path).List()
	require.NoError(t, err)
	require.Len(t, plugins, 1)
	assert.Equal(t, "checkmate", plugins[0].Name)
}
