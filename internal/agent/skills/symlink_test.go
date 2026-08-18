package skills

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureStderr replaces stderrWriter with a buffer for the duration of the test.
func captureStderr(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	orig := stderrWriter
	stderrWriter = buf
	t.Cleanup(func() { stderrWriter = orig })
	return buf
}

// makeSkillSource creates a temporary directory with a SKILL.md file inside.
func makeSkillSource(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# skill"), 0o644))
	return dir
}

func TestCheckSkillLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests skipped on windows")
	}

	t.Run("missing when nothing exists", func(t *testing.T) {
		agentDir := t.TempDir()
		assert.Equal(t, "missing", checkSkillLink(agentDir, "my-skill", "/some/source"))
	})

	t.Run("ok for correct relative symlink", func(t *testing.T) {
		src := makeSkillSource(t)
		agentDir := t.TempDir()
		rel, err := filepath.Rel(agentDir, src)
		require.NoError(t, err)
		require.NoError(t, os.Symlink(rel, filepath.Join(agentDir, "my-skill")))

		assert.Equal(t, "ok", checkSkillLink(agentDir, "my-skill", src))
	})

	t.Run("ok for correct absolute symlink", func(t *testing.T) {
		src := makeSkillSource(t)
		agentDir := t.TempDir()
		require.NoError(t, os.Symlink(src, filepath.Join(agentDir, "my-skill")))

		assert.Equal(t, "ok", checkSkillLink(agentDir, "my-skill", src))
	})

	t.Run("broken for dangling symlink", func(t *testing.T) {
		agentDir := t.TempDir()
		require.NoError(t, os.Symlink("/nonexistent/path/does/not/exist", filepath.Join(agentDir, "my-skill")))

		assert.Equal(t, "broken", checkSkillLink(agentDir, "my-skill", "/nonexistent/path/does/not/exist"))
	})

	t.Run("wrong_target for symlink pointing elsewhere", func(t *testing.T) {
		src1 := makeSkillSource(t)
		src2 := makeSkillSource(t)
		agentDir := t.TempDir()
		rel, err := filepath.Rel(agentDir, src1)
		require.NoError(t, err)
		require.NoError(t, os.Symlink(rel, filepath.Join(agentDir, "my-skill")))

		assert.Equal(t, "wrong_target", checkSkillLink(agentDir, "my-skill", src2))
	})

	t.Run("copy for real directory", func(t *testing.T) {
		agentDir := t.TempDir()
		linkPath := filepath.Join(agentDir, "my-skill")
		require.NoError(t, os.MkdirAll(linkPath, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(linkPath, "SKILL.md"), []byte("# skill"), 0o644))

		assert.Equal(t, "copy", checkSkillLink(agentDir, "my-skill", "/any/source"))
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

		result := checkSkillLink(agentDir, "my-skill", "/any/source")
		assert.Equal(t, "broken", result)
	})
}

func TestCreateSkillLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests skipped on windows")
	}

	t.Run("creates symlink for new install", func(t *testing.T) {
		src := makeSkillSource(t)
		agentDir := t.TempDir()

		require.NoError(t, CreateSkillLink(src, agentDir, "my-skill"))

		assert.Equal(t, "ok", checkSkillLink(agentDir, "my-skill", src))
		info, err := os.Lstat(filepath.Join(agentDir, "my-skill"))
		require.NoError(t, err)
		assert.NotZero(t, info.Mode()&os.ModeSymlink, "entry should be a symlink")
	})

	t.Run("uses relative symlink target", func(t *testing.T) {
		src := makeSkillSource(t)
		agentDir := t.TempDir()

		require.NoError(t, CreateSkillLink(src, agentDir, "my-skill"))

		target, err := os.Readlink(filepath.Join(agentDir, "my-skill"))
		require.NoError(t, err)
		assert.False(t, filepath.IsAbs(target), "symlink target should be relative, got: %s", target)
	})

	t.Run("idempotent when correct symlink already exists", func(t *testing.T) {
		src := makeSkillSource(t)
		agentDir := t.TempDir()

		require.NoError(t, CreateSkillLink(src, agentDir, "my-skill"))
		require.NoError(t, CreateSkillLink(src, agentDir, "my-skill"))

		assert.Equal(t, "ok", checkSkillLink(agentDir, "my-skill", src))
	})

	t.Run("replaces broken symlink", func(t *testing.T) {
		src := makeSkillSource(t)
		agentDir := t.TempDir()
		require.NoError(t, os.Symlink("/nonexistent/path/does/not/exist", filepath.Join(agentDir, "my-skill")))

		require.NoError(t, CreateSkillLink(src, agentDir, "my-skill"))

		assert.Equal(t, "ok", checkSkillLink(agentDir, "my-skill", src))
	})

	t.Run("replaces wrong-target symlink", func(t *testing.T) {
		src1 := makeSkillSource(t)
		src2 := makeSkillSource(t)
		agentDir := t.TempDir()

		require.NoError(t, CreateSkillLink(src1, agentDir, "my-skill"))
		require.NoError(t, CreateSkillLink(src2, agentDir, "my-skill"))

		assert.Equal(t, "ok", checkSkillLink(agentDir, "my-skill", src2))
	})

	t.Run("creates agent skills dir when missing", func(t *testing.T) {
		src := makeSkillSource(t)
		agentDir := filepath.Join(t.TempDir(), "deep", "nested", "agent")

		require.NoError(t, CreateSkillLink(src, agentDir, "my-skill"))

		assert.Equal(t, "ok", checkSkillLink(agentDir, "my-skill", src))
	})

	t.Run("warns and skips a real directory", func(t *testing.T) {
		buf := captureStderr(t)
		agentDir := t.TempDir()
		linkPath := filepath.Join(agentDir, "my-skill")
		require.NoError(t, os.MkdirAll(linkPath, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(linkPath, "SKILL.md"), []byte("original"), 0o644))

		src := makeSkillSource(t)
		require.NoError(t, CreateSkillLink(src, agentDir, "my-skill"))

		data, err := os.ReadFile(filepath.Join(linkPath, "SKILL.md"))
		require.NoError(t, err)
		assert.Equal(t, "original", string(data), "original directory should be preserved")
		info, err := os.Lstat(linkPath)
		require.NoError(t, err)
		assert.Zero(t, info.Mode()&os.ModeSymlink, "entry should remain a directory")
		assert.True(t, strings.Contains(buf.String(), "warning:"), "expected warning on stderr, got: %q", buf.String())
	})

	t.Run("errors on regular file at linkPath", func(t *testing.T) {
		agentDir := t.TempDir()
		linkPath := filepath.Join(agentDir, "my-skill")
		require.NoError(t, os.WriteFile(linkPath, []byte("not a dir"), 0o644))

		src := makeSkillSource(t)
		err := CreateSkillLink(src, agentDir, "my-skill")
		assert.Error(t, err)
	})
}

func TestCopyDir(t *testing.T) {
	t.Run("copies the source directory contents", func(t *testing.T) {
		src := makeSkillSource(t)
		dst := filepath.Join(t.TempDir(), "my-skill")

		require.NoError(t, copyDir(src, dst))

		data, err := os.ReadFile(filepath.Join(dst, "SKILL.md"))
		require.NoError(t, err)
		assert.Equal(t, "# skill", string(data))
	})

	t.Run("replaces dst, removing stale files", func(t *testing.T) {
		src := makeSkillSource(t)
		dst := filepath.Join(t.TempDir(), "my-skill")

		require.NoError(t, copyDir(src, dst))
		staleFile := filepath.Join(dst, "stale.txt")
		require.NoError(t, os.WriteFile(staleFile, []byte("stale"), 0o644))

		require.NoError(t, copyDir(src, dst))

		_, err := os.Stat(staleFile)
		assert.True(t, os.IsNotExist(err), "stale file should be removed after re-copy")
	})
}

// checkSkillLink reports the installation state of agentSkillsDir/skillName, used by tests
// to assert link state. Returns: "ok", "missing", "broken", "wrong_target", or "copy".
func checkSkillLink(agentSkillsDir, skillName, expectedSourceDir string) string {
	linkPath := filepath.Join(agentSkillsDir, skillName)
	info, err := os.Lstat(linkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "missing"
		}
		return "broken"
	}

	if info.Mode()&os.ModeSymlink == 0 {
		return "copy"
	}

	// It's a symlink. Verify the target exists by following the link.
	resolvedInfo, err := os.Stat(linkPath)
	if err != nil {
		return "broken"
	}

	// Use os.SameFile to handle case-insensitive filesystems (e.g. macOS APFS).
	srcInfo, err := os.Stat(expectedSourceDir)
	if err != nil {
		return "wrong_target"
	}
	if os.SameFile(resolvedInfo, srcInfo) {
		return "ok"
	}
	return "wrong_target"
}
