package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/plugins"
)

func TestMissingScopes(t *testing.T) {
	tests := []struct {
		name     string
		granted  []string
		required []string
		want     []string
	}{
		{
			name:     "all required scopes granted",
			granted:  []string{"read:clients", "read:connections"},
			required: []string{"read:clients"},
			want:     nil,
		},
		{
			name:     "some scopes missing, order preserved",
			granted:  []string{"read:clients"},
			required: []string{"read:connections", "read:clients", "read:logs"},
			want:     []string{"read:connections", "read:logs"},
		},
		{
			name:     "no required scopes",
			granted:  []string{"read:clients"},
			required: nil,
			want:     nil,
		},
		{
			name:     "nothing granted",
			granted:  nil,
			required: []string{"read:clients"},
			want:     []string{"read:clients"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, missingScopes(tt.granted, tt.required))
		})
	}
}

func TestPluginShort(t *testing.T) {
	assert.Equal(t,
		"Audit an Auth0 tenant against security best practices",
		pluginShort(plugins.InstalledPlugin{Name: "checkmate", Description: "Audit an Auth0 tenant against security best practices"}),
	)

	// Plugins installed before descriptions were persisted fall back to a
	// generic line.
	assert.Equal(t, "Run the checkmate plugin", pluginShort(plugins.InstalledPlugin{Name: "checkmate"}))
}

func TestWantsHelp(t *testing.T) {
	assert.True(t, wantsHelp([]string{"scan", "--help"}))
	assert.True(t, wantsHelp([]string{"-h"}))
	assert.False(t, wantsHelp([]string{"scan", "--output", "json"}))
	assert.False(t, wantsHelp(nil))
}

func TestVersionIsNewer(t *testing.T) {
	tests := []struct {
		name      string
		registry  string
		installed string
		want      bool
	}{
		{name: "newer patch", registry: "1.8.6", installed: "1.8.5", want: true},
		{name: "newer minor", registry: "1.9.0", installed: "1.8.5", want: true},
		{name: "same version", registry: "1.8.5", installed: "1.8.5", want: false},
		{name: "older version", registry: "1.8.4", installed: "1.8.5", want: false},
		{name: "v prefix tolerated on both sides", registry: "v2.0.0", installed: "1.9.9", want: true},
		{name: "mixed v prefix equal", registry: "v1.8.5", installed: "1.8.5", want: false},
		{name: "non-semver differing treated as newer", registry: "2024-01-02", installed: "2024-01-01", want: true},
		{name: "non-semver equal not newer", registry: "latest", installed: "latest", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, versionIsNewer(tt.registry, tt.installed))
		})
	}
}

func TestPluginsToUpdate(t *testing.T) {
	installed := []plugins.InstalledPlugin{
		{Name: "checkmate", Version: "1.8.5", InstallType: plugins.InstallNPM},
		{Name: "widgets", Version: "0.2.0", InstallType: plugins.InstallNPM},
	}

	t.Run("no plugins installed errors", func(t *testing.T) {
		_, err := pluginsToUpdate(nil, nil, nil, false)
		assert.Error(t, err)
	})

	t.Run("named plugin selects that record", func(t *testing.T) {
		got, err := pluginsToUpdate(nil, installed, []string{"widgets"}, false)
		assert.NoError(t, err)
		assert.Equal(t, []plugins.InstalledPlugin{installed[1]}, got)
	})

	t.Run("unknown named plugin errors", func(t *testing.T) {
		_, err := pluginsToUpdate(nil, installed, []string{"nope"}, false)
		assert.Error(t, err)
	})

	t.Run("--all selects every installed plugin", func(t *testing.T) {
		got, err := pluginsToUpdate(nil, installed, nil, true)
		assert.NoError(t, err)
		assert.Equal(t, installed, got)
	})
}
