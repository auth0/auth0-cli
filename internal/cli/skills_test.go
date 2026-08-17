package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadSkillConfig(t *testing.T) {
	t.Run("returns nil nil when file does not exist", func(t *testing.T) {
		cfg, err := readSkillConfig(filepath.Join(t.TempDir(), skillConfigFileName))
		require.NoError(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("returns parsed config for valid file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), skillConfigFileName)
		content := `{
  "etag": "\"abc123\"",
  "installedAt": "2026-05-12T10:00:00Z",
  "updatedAt": "2026-05-12T10:00:00Z",
  "agents": ["claude-code"],
  "scope": "global"
}`
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		cfg, err := readSkillConfig(path)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, `"abc123"`, cfg.ETag)
		assert.Equal(t, []string{"claude-code"}, cfg.Agents)
		assert.Equal(t, skillsScopeGlobal, cfg.Scope)
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), skillConfigFileName)
		require.NoError(t, os.WriteFile(path, []byte("not json"), 0o644))

		_, err := readSkillConfig(path)
		require.Error(t, err)
	})
}

func TestWriteSkillConfig(t *testing.T) {
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)

	t.Run("roundtrip preserves fields", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), skillConfigFileName)

		original := &skillConfig{
			ETag:        `"etag-v1"`,
			Skills:      []string{"auth0"},
			InstalledAt: now,
			UpdatedAt:   now.Add(time.Hour),
			Agents:      []string{"claude-code", "cursor"},
			Scope:       skillsScopeGlobal,
		}
		require.NoError(t, writeSkillConfig(path, original))

		got, err := readSkillConfig(path)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, original.ETag, got.ETag)
		assert.Equal(t, original.Skills, got.Skills)
		assert.Equal(t, original.InstalledAt.UTC(), got.InstalledAt.UTC())
		assert.Equal(t, original.UpdatedAt.UTC(), got.UpdatedAt.UTC())
		assert.Equal(t, original.Agents, got.Agents)
		assert.Equal(t, original.Scope, got.Scope)
	})

	t.Run("creates parent directories when they do not exist", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nested", "deep", skillConfigFileName)

		require.NoError(t, writeSkillConfig(path, &skillConfig{Scope: skillsScopeGlobal}))

		got, err := readSkillConfig(path)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, skillsScopeGlobal, got.Scope)
	})

	t.Run("overwrites existing skill config file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), skillConfigFileName)

		require.NoError(t, writeSkillConfig(path, &skillConfig{ETag: `"first"`, Scope: skillsScopeGlobal}))
		require.NoError(t, writeSkillConfig(path, &skillConfig{ETag: `"second"`, Scope: skillsScopeGlobal}))

		got, err := readSkillConfig(path)
		require.NoError(t, err)
		assert.Equal(t, `"second"`, got.ETag)
	})
}
