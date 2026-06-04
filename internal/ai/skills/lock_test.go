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
		lock, err := ReadLock(filepath.Join(t.TempDir(), "skills-lock.json"))
		require.NoError(t, err)
		assert.Nil(t, lock)
	})

	t.Run("returns parsed lock for valid file", func(t *testing.T) {
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

		lock, err := ReadLock(path)
		require.NoError(t, err)
		require.NotNil(t, lock)
		assert.Equal(t, "https://github.com/auth0/agent-skills.git", lock.Repo)
		assert.Equal(t, "main", lock.Ref)
		assert.Equal(t, "abc123", lock.CommitSHA)
		assert.Equal(t, []string{"auth0-react", "auth0-nextjs"}, lock.Skills)
		assert.Equal(t, []string{"claude-code"}, lock.Agents)
		assert.Equal(t, ScopeGlobal, lock.Scope)
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

		lock := &Lock{
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
		require.NoError(t, WriteLock(path, lock))

		got, err := ReadLock(path)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, lock.Repo, got.Repo)
		assert.Equal(t, lock.CommitSHA, got.CommitSHA)
		assert.Equal(t, lock.Skills, got.Skills)
		assert.Equal(t, lock.Scope, got.Scope)
		assert.Equal(t, lock.InstalledAt.UTC(), got.InstalledAt.UTC())
		assert.Equal(t, lock.LastCheckedAt.UTC(), got.LastCheckedAt.UTC())
	})

	t.Run("creates parent directories when they do not exist", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "nested", "deep", "skills-lock.json")

		require.NoError(t, WriteLock(path, &Lock{Scope: ScopeGlobal}))

		got, err := ReadLock(path)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, ScopeGlobal, got.Scope)
	})

	t.Run("overwrites existing lock file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "skills-lock.json")

		require.NoError(t, WriteLock(path, &Lock{CommitSHA: "first", Scope: ScopeGlobal}))
		require.NoError(t, WriteLock(path, &Lock{CommitSHA: "second", Scope: ScopeGlobal}))

		got, err := ReadLock(path)
		require.NoError(t, err)
		assert.Equal(t, "second", got.CommitSHA)
	})

	t.Run("roundtrip preserves all fields", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "skills-lock.json")

		original := &Lock{
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
}
