//go:build !windows

package config

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const ErrFileIsADirectory = "is a directory"

func FailsToSaveToReadOnlyDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		err := os.RemoveAll(tmpDir)
		require.NoError(t, err)
	})

	err = os.Chmod(tmpDir, 0555)
	require.NoError(t, err)

	config := &Config{path: path.Join(tmpDir, "auth0", "config.json")}

	err = config.saveToDisk()
	assert.EqualError(t, err, fmt.Sprintf("mkdir %s/auth0: permission denied", tmpDir))
}
