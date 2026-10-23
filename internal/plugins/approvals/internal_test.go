package approvals

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFailedRenameKeepsTheOldFile(t *testing.T) {
	const (
		tenant = "acme.us.auth0.com"
		hashA  = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		hashB  = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	)

	a := &Approvals{Dir: filepath.Join(t.TempDir(), "plugins")}
	require.NoError(t, a.Approve(t.Context(), tenant, "auth0/checkmate", hashA))

	before, err := os.ReadFile(a.path())
	require.NoError(t, err)

	namesBefore := dirEntries(t, a.Dir)

	renameFile = func(string, string) error { return errors.New("disk is read only") }

	t.Cleanup(func() { renameFile = os.Rename })

	require.ErrorContains(t, a.Approve(t.Context(), tenant, "auth0/checkmate", hashB), "disk is read only")

	after, err := os.ReadFile(a.path())
	require.NoError(t, err)

	assert.Equal(t, string(before), string(after))
	assert.Equal(t, namesBefore, dirEntries(t, a.Dir), "no temporary file is left behind")
}

// dirEntries lists the names in a directory.
func dirEntries(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	return names
}
