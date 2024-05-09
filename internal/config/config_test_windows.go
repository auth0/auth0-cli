//go:build windows

package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

const ErrFileIsADirectory = "Incorrect function."

func FailsToSaveToReadOnlyDirectory(t *testing.T) {
	t.SkipNow()
}

func PlatformTestConfigFileDefaultPath(t *testing.T) {
	os.Unsetenv("AUTH0_CONFIG_FILE")
	os.Unsetenv("XDG_CONFIG_HOME")
	os.Unsetenv("HOME")
	t.Setenv("APPDATA", "C:\\Users\\test\\AppData\\Roaming")
	expected := "C:\\Users\\test\\AppData\\Roaming/auth0/config.json"
	actual := defaultPath()
	assert.Equal(t, expected, actual)
}

func PlatformTestXDGConfigHome(t *testing.T) {
	os.Unsetenv("AUTH0_CONFIG_FILE")
	t.Setenv("XDG_CONFIG_HOME", "/path/to/xdg_config_home")
	t.Setenv("HOME", "/path/to/home")
	t.Setenv("APPDATA", "/path/to/appdata")
	expectedPath := "/path/to/appdata/auth0/config.json"
	actualPath := defaultPath()
	assert.Equal(t, expectedPath, actualPath)
}
