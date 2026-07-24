package skills

import (
	"os"
	"path/filepath"

	"github.com/auth0/auth0-cli/internal/utils"
)

// mergeDir recursively copies the contents of src into dst. Symlinks are preserved
// (not dereferenced) so the layout matches what git sparse-checkout produces.
func mergeDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		switch {
		case entry.Type()&os.ModeSymlink != 0:
			target, err := os.Readlink(srcPath)
			if err != nil {
				return err
			}
			// Os.Symlink is not idempotent (returns EEXIST). Remove any existing
			// entry so the call is safe under concurrent writes or repeated merges.
			_ = os.Remove(dstPath)
			if err := os.Symlink(target, dstPath); err != nil {
				return err
			}
		case entry.IsDir():
			if err := os.MkdirAll(dstPath, 0o755); err != nil {
				return err
			}
			if err := mergeDir(srcPath, dstPath); err != nil {
				return err
			}
		default:
			if err := utils.CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}
