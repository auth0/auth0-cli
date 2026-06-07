package skills

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests skipped on windows")
	}

	t.Run("copies regular files", func(t *testing.T) {
		src := t.TempDir()
		dst := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(src, "file.txt"), []byte("hello"), 0o644))

		require.NoError(t, mergeDir(src, dst))

		data, err := os.ReadFile(filepath.Join(dst, "file.txt"))
		require.NoError(t, err)
		assert.Equal(t, "hello", string(data))
	})

	t.Run("preserves symlinks", func(t *testing.T) {
		src := t.TempDir()
		dst := t.TempDir()
		target := filepath.Join(src, "target.txt")
		require.NoError(t, os.WriteFile(target, []byte("target"), 0o644))
		require.NoError(t, os.Symlink(target, filepath.Join(src, "link")))

		require.NoError(t, mergeDir(src, dst))

		linkDst := filepath.Join(dst, "link")
		info, err := os.Lstat(linkDst)
		require.NoError(t, err)
		assert.NotZero(t, info.Mode()&os.ModeSymlink, "should be a symlink")
	})

	t.Run("symlink overwrite is idempotent (no EEXIST)", func(t *testing.T) {
		src := t.TempDir()
		dst := t.TempDir()
		target := filepath.Join(src, "target.txt")
		require.NoError(t, os.WriteFile(target, []byte("target"), 0o644))
		require.NoError(t, os.Symlink(target, filepath.Join(src, "link")))

		// First merge creates the symlink.
		require.NoError(t, mergeDir(src, dst))
		// Second merge must not fail with EEXIST.
		require.NoError(t, mergeDir(src, dst))

		linkDst := filepath.Join(dst, "link")
		info, err := os.Lstat(linkDst)
		require.NoError(t, err)
		assert.NotZero(t, info.Mode()&os.ModeSymlink)
	})

	t.Run("recurses into subdirectories", func(t *testing.T) {
		src := t.TempDir()
		dst := t.TempDir()
		sub := filepath.Join(src, "sub")
		require.NoError(t, os.MkdirAll(sub, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(sub, "nested.txt"), []byte("nested"), 0o644))

		require.NoError(t, mergeDir(src, dst))

		data, err := os.ReadFile(filepath.Join(dst, "sub", "nested.txt"))
		require.NoError(t, err)
		assert.Equal(t, "nested", string(data))
	})
}
