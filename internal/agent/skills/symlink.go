package skills

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// stderrWriter is the target for diagnostic output. Replaced in tests.
var stderrWriter io.Writer = os.Stderr

// CreateSkillLink installs skillName from sourceSkillDir into agentSkillsDir as a symlink.
// It is idempotent: a correct existing symlink is left unchanged.
func CreateSkillLink(sourceSkillDir, agentSkillsDir, skillName string) error {
	if err := os.MkdirAll(agentSkillsDir, 0o755); err != nil {
		return fmt.Errorf("create agent skills dir: %w", err)
	}

	linkPath := filepath.Join(agentSkillsDir, skillName)

	info, err := os.Lstat(linkPath)
	if err == nil {
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			if isSymlinkCorrect(linkPath, sourceSkillDir) {
				return nil
			}
			if rmErr := os.Remove(linkPath); rmErr != nil {
				return fmt.Errorf("remove existing symlink %s: %w", linkPath, rmErr)
			}
		case info.IsDir():
			// A real directory here is a prior copy (e.g. from the Windows fallback);
			// leave it untouched rather than destroy it.
			fmt.Fprintf(stderrWriter, "warning: %s is a copied directory; remove it manually to switch to a symlink\n", linkPath)
			return nil
		default:
			return fmt.Errorf("%s exists as a regular file; remove it before installing skill %q", linkPath, skillName)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("lstat %s: %w", linkPath, err)
	}

	return createSymlink(sourceSkillDir, agentSkillsDir, linkPath)
}

// isSymlinkCorrect reports whether linkPath is a non-broken symlink resolving to sourceSkillDir.
// It uses os.SameFile to stay correct on case-insensitive filesystems (e.g. macOS APFS).
func isSymlinkCorrect(linkPath, sourceSkillDir string) bool {
	linkInfo, err := os.Stat(linkPath)
	if err != nil {
		return false
	}
	srcInfo, err := os.Stat(sourceSkillDir)
	if err != nil {
		return false
	}
	return os.SameFile(linkInfo, srcInfo)
}

// createSymlink links linkPath to sourceSkillDir: a relative symlink on Unix; on Windows
// it falls back symlink → junction → copy.
func createSymlink(sourceSkillDir, agentSkillsDir, linkPath string) error {
	if runtime.GOOS != "windows" {
		rel, err := filepath.Rel(agentSkillsDir, sourceSkillDir)
		if err != nil {
			rel = sourceSkillDir
		}
		return os.Symlink(rel, linkPath)
	}

	// Windows: absolute symlink → junction → copy fallback.
	if err := os.Symlink(sourceSkillDir, linkPath); err == nil {
		return nil
	}
	if err := exec.Command("cmd", "/C", "mklink", "/J", linkPath, sourceSkillDir).Run(); err == nil {
		return nil
	}
	fmt.Fprintf(stderrWriter, "warning: symlink and junction unavailable; copying %s to %s\n", sourceSkillDir, linkPath)
	return copyDir(sourceSkillDir, linkPath)
}

// copyDir replaces dst with a copy of src, staged in a sibling temp dir and swapped in
// with an atomic rename so an interrupted copy cannot corrupt an existing dst.
func copyDir(src, dst string) error {
	// A sibling of dst shares its filesystem, so the final rename is atomic.
	tmp := dst + ".tmp"
	if err := os.RemoveAll(tmp); err != nil {
		return fmt.Errorf("clear temp copy dir: %w", err)
	}
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return fmt.Errorf("create temp copy dir: %w", err)
	}
	if err := copyTree(src, tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	if err := os.RemoveAll(dst); err != nil {
		_ = os.RemoveAll(tmp)
		return fmt.Errorf("remove stale copy dir: %w", err)
	}
	return os.Rename(tmp, dst)
}
