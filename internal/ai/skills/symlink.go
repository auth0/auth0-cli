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

// CreateSkillLink installs skillName from sourceSkillDir into agentSkillsDir.
// When useCopy is true the directory is copied recursively; otherwise a symlink is created.
// The operation is idempotent: a correct existing symlink or copy is left unchanged.
func CreateSkillLink(sourceSkillDir, agentSkillsDir, skillName string, useCopy bool) error {
	if err := os.MkdirAll(agentSkillsDir, 0o755); err != nil {
		return fmt.Errorf("create agent skills dir: %w", err)
	}

	linkPath := filepath.Join(agentSkillsDir, skillName)

	info, err := os.Lstat(linkPath)
	if err == nil {
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			// For useCopy=false: skip if already pointing to the right place.
			// For useCopy=true: remove the symlink so we can replace it with a copy.
			if !useCopy && isSymlinkCorrect(linkPath, sourceSkillDir) {
				return nil
			}
			if rmErr := os.Remove(linkPath); rmErr != nil {
				return fmt.Errorf("remove existing symlink %s: %w", linkPath, rmErr)
			}
		case info.IsDir():
			if !useCopy {
				fmt.Fprintf(stderrWriter,
					"warning: %s is a copied directory; remove it manually to switch to symlink mode\n",
					linkPath)
				return nil
			}
			// UseCopy=true: fall through to re-copy with replace semantics.
		default:
			return fmt.Errorf("%s exists as a regular file; remove it before installing skill %q", linkPath, skillName)
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
// Uses os.SameFile instead of string comparison to handle case-insensitive filesystems (e.g. macOS APFS).
func isSymlinkCorrect(linkPath, sourceSkillDir string) bool {
	linkInfo, err := os.Stat(linkPath)
	if err != nil {
		return false // Broken symlink.
	}
	srcInfo, err := os.Stat(sourceSkillDir)
	if err != nil {
		return false
	}
	return os.SameFile(linkInfo, srcInfo)
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
	fmt.Fprintf(stderrWriter, "warning: symlink and junction unavailable; copying %s to %s\n", sourceSkillDir, linkPath)
	return copyDir(sourceSkillDir, linkPath)
}

// copyDir replaces dst with an exact copy of src.
// Any files in dst that no longer exist in src are removed, so the installed copy
// stays in sync with the canonical source on skill updates.
func copyDir(src, dst string) error {
	tmpDst, err := os.MkdirTemp(filepath.Dir(dst), ".skill-copy-*")
	if err != nil {
		return fmt.Errorf("create temp copy dir: %w", err)
	}
	// Always clean up the temp dir so it is never left as an orphan in agentSkillsDir.
	tmpRemoved := false
	defer func() {
		if !tmpRemoved {
			_ = os.RemoveAll(tmpDst)
		}
	}()

	if err := mergeDir(src, tmpDst); err != nil {
		return err
	}
	if err := os.RemoveAll(dst); err != nil {
		return fmt.Errorf("remove stale copy dir: %w", err)
	}
	if err := os.Rename(tmpDst, dst); err != nil {
		// Cross-filesystem fallback: re-create dst from the temp copy.
		fmt.Fprintf(stderrWriter, "warning: rename %s → %s failed (%v); falling back to copy\n", tmpDst, dst, err)
		if mkErr := os.MkdirAll(dst, 0o755); mkErr != nil {
			return fmt.Errorf("create copy dir: %w", mkErr)
		}
		if mergeErr := mergeDir(tmpDst, dst); mergeErr != nil {
			return mergeErr
		}
	} else {
		tmpRemoved = true // Rename succeeded; temp dir is now dst.
	}
	return nil
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
