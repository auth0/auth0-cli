package skills

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadLock(t *testing.T) {
	t.Run("returns nil nil when file does not exist", func(t *testing.T) {
		cfg, err := ReadLock(filepath.Join(t.TempDir(), "skills-lock.json"))
		require.NoError(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("returns parsed config for valid file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "skills-lock.json")
		content := `{
  "repo": "https://github.com/auth0/agent-skills.git",
  "ref": "main",
  "commitSHA": "abc123",
  "installedAt": "2026-05-12T10:00:00Z",
  "updatedAt": "2026-05-12T10:00:00Z",
  "lastCheckedAt": "2026-05-12T11:00:00Z",
  "skills": ["auth0-react", "auth0-nextjs"],
  "agents": ["claude-code"],
  "scope": "global"
}`
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		cfg, err := ReadLock(path)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, "https://github.com/auth0/agent-skills.git", cfg.Repo)
		assert.Equal(t, "main", cfg.Ref)
		assert.Equal(t, "abc123", cfg.CommitSHA)
		assert.Equal(t, []string{"auth0-react", "auth0-nextjs"}, cfg.Skills)
		assert.Equal(t, []string{"claude-code"}, cfg.Agents)
		assert.Equal(t, ScopeGlobal, cfg.Scope)
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "skills-lock.json")
		require.NoError(t, os.WriteFile(path, []byte("not json"), 0o644))

		_, err := ReadLock(path)
		require.Error(t, err)
	})

	t.Run("returns error on unreadable file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "skills-lock.json")
		require.NoError(t, os.WriteFile(path, []byte("{}"), 0o000))
		t.Cleanup(func() { os.Chmod(path, 0o644) })

		if os.Getuid() == 0 {
			t.Skip("root bypasses file permissions")
		}
		_, err := ReadLock(path)
		require.Error(t, err)
	})
}

func TestWriteLock(t *testing.T) {
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)

	t.Run("creates file with correct content", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "skills-lock.json")

		cfg := &SkillsVersionConfig{
			Repo:          "https://github.com/auth0/agent-skills.git",
			Ref:           "main",
			CommitSHA:     "deadbeef",
			InstalledAt:   now,
			UpdatedAt:     now,
			LastCheckedAt: now,
			Skills:        []string{"auth0-react"},
			Agents:        []string{"cursor"},
			Scope:         ScopeLocal,
		}
		require.NoError(t, WriteLock(path, cfg))

		got, err := ReadLock(path)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, cfg.Repo, got.Repo)
		assert.Equal(t, cfg.CommitSHA, got.CommitSHA)
		assert.Equal(t, cfg.Skills, got.Skills)
		assert.Equal(t, cfg.Scope, got.Scope)
		assert.Equal(t, cfg.InstalledAt.UTC(), got.InstalledAt.UTC())
		assert.Equal(t, cfg.LastCheckedAt.UTC(), got.LastCheckedAt.UTC())
	})

	t.Run("creates parent directories when they do not exist", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "nested", "deep", "skills-lock.json")

		require.NoError(t, WriteLock(path, &SkillsVersionConfig{Scope: ScopeGlobal}))

		got, err := ReadLock(path)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, ScopeGlobal, got.Scope)
	})

	t.Run("overwrites existing lock file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "skills-lock.json")

		require.NoError(t, WriteLock(path, &SkillsVersionConfig{CommitSHA: "first", Scope: ScopeGlobal}))
		require.NoError(t, WriteLock(path, &SkillsVersionConfig{CommitSHA: "second", Scope: ScopeGlobal}))

		got, err := ReadLock(path)
		require.NoError(t, err)
		assert.Equal(t, "second", got.CommitSHA)
	})

	t.Run("roundtrip preserves all fields", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "skills-lock.json")

		original := &SkillsVersionConfig{
			Repo:          "https://github.com/auth0/agent-skills.git",
			Ref:           "v1.2.3",
			CommitSHA:     "cafebabe",
			InstalledAt:   now,
			UpdatedAt:     now.Add(time.Hour),
			LastCheckedAt: now.Add(2 * time.Hour),
			Skills:        []string{"auth0-react", "auth0-nextjs", "auth0-vue"},
			Agents:        []string{"claude-code", "cursor", "gemini-cli"},
			Scope:         ScopeGlobal,
		}

		require.NoError(t, WriteLock(path, original))
		got, err := ReadLock(path)
		require.NoError(t, err)
		require.NotNil(t, got)

		assert.Equal(t, original.Repo, got.Repo)
		assert.Equal(t, original.Ref, got.Ref)
		assert.Equal(t, original.CommitSHA, got.CommitSHA)
		assert.Equal(t, original.InstalledAt.UTC(), got.InstalledAt.UTC())
		assert.Equal(t, original.UpdatedAt.UTC(), got.UpdatedAt.UTC())
		assert.Equal(t, original.LastCheckedAt.UTC(), got.LastCheckedAt.UTC())
		assert.Equal(t, original.Skills, got.Skills)
		assert.Equal(t, original.Agents, got.Agents)
		assert.Equal(t, original.Scope, got.Scope)
	})

	t.Run("returns error for invalid scope", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "skills-lock.json")
		err := WriteLock(path, &SkillsVersionConfig{Scope: "invalid"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid scope")
	})
}
