package skills

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeSkillSource creates a temporary directory with a SKILL.md file inside.
func makeSkillSource(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# skill"), 0o644))
	return dir
}

// --- CheckSkillLink ---

func TestCheckSkillLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests skipped on windows")
	}

	t.Run("missing when nothing exists", func(t *testing.T) {
		agentDir := t.TempDir()
		assert.Equal(t, "missing", CheckSkillLink(agentDir, "my-skill", "/some/source"))
	})

	t.Run("ok for correct relative symlink", func(t *testing.T) {
		src := makeSkillSource(t)
		agentDir := t.TempDir()
		rel, err := filepath.Rel(agentDir, src)
		require.NoError(t, err)
		require.NoError(t, os.Symlink(rel, filepath.Join(agentDir, "my-skill")))

		assert.Equal(t, "ok", CheckSkillLink(agentDir, "my-skill", src))
	})

	t.Run("ok for correct absolute symlink", func(t *testing.T) {
		src := makeSkillSource(t)
		agentDir := t.TempDir()
		require.NoError(t, os.Symlink(src, filepath.Join(agentDir, "my-skill")))

		assert.Equal(t, "ok", CheckSkillLink(agentDir, "my-skill", src))
	})

	t.Run("broken for dangling symlink", func(t *testing.T) {
		agentDir := t.TempDir()
		require.NoError(t, os.Symlink("/nonexistent/path/does/not/exist", filepath.Join(agentDir, "my-skill")))

		assert.Equal(t, "broken", CheckSkillLink(agentDir, "my-skill", "/nonexistent/path/does/not/exist"))
	})

	t.Run("wrong_target for symlink pointing elsewhere", func(t *testing.T) {
		src1 := makeSkillSource(t)
		src2 := makeSkillSource(t)
		agentDir := t.TempDir()
		rel, err := filepath.Rel(agentDir, src1)
		require.NoError(t, err)
		require.NoError(t, os.Symlink(rel, filepath.Join(agentDir, "my-skill")))

		assert.Equal(t, "wrong_target", CheckSkillLink(agentDir, "my-skill", src2))
	})

	t.Run("copy for real directory", func(t *testing.T) {
		agentDir := t.TempDir()
		linkPath := filepath.Join(agentDir, "my-skill")
		require.NoError(t, os.MkdirAll(linkPath, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(linkPath, "SKILL.md"), []byte("# skill"), 0o644))

		assert.Equal(t, "copy", CheckSkillLink(agentDir, "my-skill", "/any/source"))
	})

	t.Run("broken on permission error (not missing)", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("root bypasses permission checks")
		}
		parent := t.TempDir()
		agentDir := filepath.Join(parent, "locked")
		require.NoError(t, os.MkdirAll(filepath.Join(agentDir, "my-skill"), 0o755))
		require.NoError(t, os.Chmod(agentDir, 0o000))
		t.Cleanup(func() { _ = os.Chmod(agentDir, 0o755) })

		result := CheckSkillLink(agentDir, "my-skill", "/any/source")
		assert.Equal(t, "broken", result)
	})
}

// --- CreateSkillLink ---

func TestCreateSkillLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests skipped on windows")
	}

	t.Run("creates symlink for new install", func(t *testing.T) {
		src := makeSkillSource(t)
		agentDir := t.TempDir()

		require.NoError(t, CreateSkillLink(src, agentDir, "my-skill", false))

		assert.Equal(t, "ok", CheckSkillLink(agentDir, "my-skill", src))
		info, err := os.Lstat(filepath.Join(agentDir, "my-skill"))
		require.NoError(t, err)
		assert.NotZero(t, info.Mode()&os.ModeSymlink, "entry should be a symlink")
	})

	t.Run("uses relative symlink target", func(t *testing.T) {
		src := makeSkillSource(t)
		agentDir := t.TempDir()

		require.NoError(t, CreateSkillLink(src, agentDir, "my-skill", false))

		target, err := os.Readlink(filepath.Join(agentDir, "my-skill"))
		require.NoError(t, err)
		assert.False(t, filepath.IsAbs(target), "symlink target should be relative, got: %s", target)
	})

	t.Run("idempotent when correct symlink already exists", func(t *testing.T) {
		src := makeSkillSource(t)
		agentDir := t.TempDir()

		require.NoError(t, CreateSkillLink(src, agentDir, "my-skill", false))
		require.NoError(t, CreateSkillLink(src, agentDir, "my-skill", false))

		assert.Equal(t, "ok", CheckSkillLink(agentDir, "my-skill", src))
	})

	t.Run("replaces broken symlink", func(t *testing.T) {
		src := makeSkillSource(t)
		agentDir := t.TempDir()
		require.NoError(t, os.Symlink("/nonexistent/path/does/not/exist", filepath.Join(agentDir, "my-skill")))

		require.NoError(t, CreateSkillLink(src, agentDir, "my-skill", false))

		assert.Equal(t, "ok", CheckSkillLink(agentDir, "my-skill", src))
	})

	t.Run("replaces wrong-target symlink", func(t *testing.T) {
		src1 := makeSkillSource(t)
		src2 := makeSkillSource(t)
		agentDir := t.TempDir()

		require.NoError(t, CreateSkillLink(src1, agentDir, "my-skill", false))
		require.NoError(t, CreateSkillLink(src2, agentDir, "my-skill", false))

		assert.Equal(t, "ok", CheckSkillLink(agentDir, "my-skill", src2))
	})

	t.Run("creates agent skills dir when missing", func(t *testing.T) {
		src := makeSkillSource(t)
		agentDir := filepath.Join(t.TempDir(), "deep", "nested", "agent")

		require.NoError(t, CreateSkillLink(src, agentDir, "my-skill", false))

		assert.Equal(t, "ok", CheckSkillLink(agentDir, "my-skill", src))
	})

	t.Run("copies directory when useCopy is true", func(t *testing.T) {
		src := makeSkillSource(t)
		agentDir := t.TempDir()

		require.NoError(t, CreateSkillLink(src, agentDir, "my-skill", true))

		assert.Equal(t, "copy", CheckSkillLink(agentDir, "my-skill", src))
		data, err := os.ReadFile(filepath.Join(agentDir, "my-skill", "SKILL.md"))
		require.NoError(t, err)
		assert.Equal(t, "# skill", string(data))
	})

	t.Run("idempotent when copy already exists and useCopy is true", func(t *testing.T) {
		src := makeSkillSource(t)
		agentDir := t.TempDir()

		require.NoError(t, CreateSkillLink(src, agentDir, "my-skill", true))
		require.NoError(t, CreateSkillLink(src, agentDir, "my-skill", true))

		assert.Equal(t, "copy", CheckSkillLink(agentDir, "my-skill", src))
	})

	t.Run("warns and skips real directory when useCopy is false", func(t *testing.T) {
		agentDir := t.TempDir()
		linkPath := filepath.Join(agentDir, "my-skill")
		require.NoError(t, os.MkdirAll(linkPath, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(linkPath, "SKILL.md"), []byte("original"), 0o644))

		src := makeSkillSource(t)
		// Should succeed (skip) but warn to stderr.
		require.NoError(t, CreateSkillLink(src, agentDir, "my-skill", false))

		// Original directory content must be preserved.
		data, err := os.ReadFile(filepath.Join(linkPath, "SKILL.md"))
		require.NoError(t, err)
		assert.Equal(t, "original", string(data), "original directory should be preserved")
		// Entry must still be a real directory, not a symlink.
		info, err := os.Lstat(linkPath)
		require.NoError(t, err)
		assert.Zero(t, info.Mode()&os.ModeSymlink, "entry should remain a directory")
	})

	t.Run("errors on regular file at linkPath", func(t *testing.T) {
		agentDir := t.TempDir()
		linkPath := filepath.Join(agentDir, "my-skill")
		require.NoError(t, os.WriteFile(linkPath, []byte("not a dir"), 0o644))

		src := makeSkillSource(t)
		err := CreateSkillLink(src, agentDir, "my-skill", false)
		assert.Error(t, err)
	})

	t.Run("copy is replaced on re-install (replace semantics)", func(t *testing.T) {
		src := makeSkillSource(t)
		agentDir := t.TempDir()

		require.NoError(t, CreateSkillLink(src, agentDir, "my-skill", true))

		// Add a stale file directly into the copy.
		staleFile := filepath.Join(agentDir, "my-skill", "stale.txt")
		require.NoError(t, os.WriteFile(staleFile, []byte("stale"), 0o644))

		// Re-run copy install; the stale file should be gone.
		require.NoError(t, CreateSkillLink(src, agentDir, "my-skill", true))

		_, err := os.Stat(staleFile)
		assert.True(t, os.IsNotExist(err), "stale file should be removed after re-install")
	})
}

// --- RemoveSkillLink ---

func TestRemoveSkillLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests skipped on windows")
	}

	t.Run("removes symlink", func(t *testing.T) {
		src := makeSkillSource(t)
		agentDir := t.TempDir()
		require.NoError(t, CreateSkillLink(src, agentDir, "my-skill", false))

		require.NoError(t, RemoveSkillLink(agentDir, "my-skill"))

		assert.Equal(t, "missing", CheckSkillLink(agentDir, "my-skill", src))
	})

	t.Run("removes copied directory", func(t *testing.T) {
		src := makeSkillSource(t)
		agentDir := t.TempDir()
		require.NoError(t, CreateSkillLink(src, agentDir, "my-skill", true))

		require.NoError(t, RemoveSkillLink(agentDir, "my-skill"))

		assert.Equal(t, "missing", CheckSkillLink(agentDir, "my-skill", src))
	})

	t.Run("returns nil for non-existent entry", func(t *testing.T) {
		agentDir := t.TempDir()
		require.NoError(t, RemoveSkillLink(agentDir, "nonexistent"))
	})

	t.Run("removes nested copied directory recursively", func(t *testing.T) {
		src := t.TempDir()
		nested := filepath.Join(src, "sub")
		require.NoError(t, os.MkdirAll(nested, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(nested, "file.txt"), []byte("x"), 0o644))

		agentDir := t.TempDir()
		require.NoError(t, CreateSkillLink(src, agentDir, "my-skill", true))
		require.NoError(t, RemoveSkillLink(agentDir, "my-skill"))

		_, err := os.Lstat(filepath.Join(agentDir, "my-skill"))
		assert.True(t, os.IsNotExist(err))
	})
}
