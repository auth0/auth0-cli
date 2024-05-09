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

func PlatformTestConfigFileDefaultPath(t *testing.T) {
	os.Unsetenv("AUTH0_CONFIG_FILE")
	os.Unsetenv("XDG_CONFIG_HOME")
	t.Setenv("HOME", "/home/test")
	os.Unsetenv("USERPROFILE")
	expected := "/home/test/.config/auth0/config.json"
	actual := defaultPath()
	assert.Equal(t, expected, actual)
}

func PlatformTestXDGConfigHome(t *testing.T) {
	os.Unsetenv("AUTH0_CONFIG_FILE")
	t.Setenv("XDG_CONFIG_HOME", "/path/to/xdg_config_home")
	t.Setenv("HOME", "/path/to/home")
	t.Setenv("APPDATA", "/path/to/appdata")
	expectedPath := "/path/to/xdg_config_home/auth0/config.json"
	actualPath := defaultPath()
	assert.Equal(t, expectedPath, actualPath)
}
