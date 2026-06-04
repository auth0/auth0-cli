package skills

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// CreateSkillLink installs skillName from sourceSkillDir into agentSkillsDir.
// useCopy=true copies files recursively; useCopy=false creates a symlink.
// The operation is idempotent: a correct existing symlink or copy is left unchanged.
func CreateSkillLink(sourceSkillDir, agentSkillsDir, skillName string, useCopy bool) error {
	if err := os.MkdirAll(agentSkillsDir, 0o755); err != nil {
		return fmt.Errorf("create agent skills dir: %w", err)
	}

	linkPath := filepath.Join(agentSkillsDir, skillName)

	info, err := os.Lstat(linkPath)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			if isSymlinkCorrect(linkPath, agentSkillsDir, sourceSkillDir) {
				return nil
			}
			if rmErr := os.Remove(linkPath); rmErr != nil {
				return fmt.Errorf("remove existing symlink %s: %w", linkPath, rmErr)
			}
		} else if info.IsDir() {
			// Prior --copy install. Skip: replacing a real dir silently is unsafe.
			return nil
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("lstat %s: %w", linkPath, err)
	}

	if useCopy {
		return copyDir(sourceSkillDir, linkPath)
	}
	return createSymlink(sourceSkillDir, agentSkillsDir, linkPath)
}

// isSymlinkCorrect returns true if linkPath is a non-broken symlink resolving to sourceSkillDir.
func isSymlinkCorrect(linkPath, agentSkillsDir, sourceSkillDir string) bool {
	target, err := os.Readlink(linkPath)
	if err != nil {
		return false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(agentSkillsDir, target)
	}
	if filepath.Clean(target) != filepath.Clean(sourceSkillDir) {
		return false
	}
	_, err = os.Stat(linkPath)
	return err == nil
}

// createSymlink creates a symlink at linkPath pointing to sourceSkillDir.
// On Unix a relative path is used. On Windows an absolute symlink is tried first,
// then a directory junction, then a file copy with a warning written to stderr.
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
	fmt.Fprintf(os.Stderr, "warning: symlink and junction unavailable; copying %s to %s\n", sourceSkillDir, linkPath)
	return copyDir(sourceSkillDir, linkPath)
}

// copyDir recursively copies src into a newly created dst directory.
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("create copy dir: %w", err)
	}
	return mergeDir(src, dst)
}

// RemoveSkillLink removes the skill entry (symlink or copied directory) at agentSkillsDir/skillName.
// Returns nil if the entry does not exist.
func RemoveSkillLink(agentSkillsDir, skillName string) error {
	linkPath := filepath.Join(agentSkillsDir, skillName)
	info, err := os.Lstat(linkPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lstat %s: %w", linkPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return os.Remove(linkPath)
	}
	return os.RemoveAll(linkPath)
}

// CheckSkillLink reports the installation state of agentSkillsDir/skillName.
// Returns: "ok", "missing", "broken", "wrong_target", or "copy".
func CheckSkillLink(agentSkillsDir, skillName, expectedSourceDir string) string {
	linkPath := filepath.Join(agentSkillsDir, skillName)
	info, err := os.Lstat(linkPath)
	if err != nil {
		return "missing"
	}

	if info.Mode()&os.ModeSymlink == 0 {
		return "copy"
	}

	// It's a symlink. Verify the target exists.
	if _, err := os.Stat(linkPath); err != nil {
		return "broken"
	}

	// Target exists. Check if it points to the expected place.
	target, err := os.Readlink(linkPath)
	if err != nil {
		return "broken"
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(agentSkillsDir, target)
	}
	if filepath.Clean(target) == filepath.Clean(expectedSourceDir) {
		return "ok"
	}
	return "wrong_target"
}
