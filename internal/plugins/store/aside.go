package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// asidePrefix starts the name of a version directory that an install moved aside
// while it replaced it.
const asidePrefix = ".aside-"

// moveAside renames the version directory to a name next to it, so that it can
// be put back, and returns that name. It returns an empty string when there is
// no such directory. The caller holds the store lock and has removed the
// leftovers of earlier installs, so the name is free.
func moveAside(dir, version string) (string, error) {
	if _, err := os.Lstat(dir); errors.Is(err, fs.ErrNotExist) {
		return "", nil
	} else if err != nil {
		return "", fmt.Errorf("failed to look at %s: %w", dir, err)
	}

	aside := filepath.Join(filepath.Dir(dir), asidePrefix+version)

	if err := renameFile(dir, aside); err != nil {
		return "", fmt.Errorf("failed to move %s aside: %w", dir, err)
	}

	return aside, nil
}

// putBack undoes the move of a new directory to final: it removes whatever is at
// final and moves the directory that was set aside back. It returns cause, with
// a note when the old state could not be restored.
func putBack(cause error, final, aside string) error {
	if err := os.RemoveAll(final); err != nil {
		return fmt.Errorf("%w (and %s could not be removed: %w)", cause, final, err)
	}

	if aside == "" {
		return cause
	}

	if err := renameFile(aside, final); err != nil {
		return fmt.Errorf("%w (and the previous copy could not be put back from %s: %w)", cause, aside, err)
	}

	return cause
}

// removeAsides cleans up the directories that an install moved aside and did not
// delete, because the process ended before it was done. When the version is one
// the lockfile still lists and its directory is gone, the process ended between
// moving the good copy aside and putting the new one in, so the copy is moved
// back and Verify judges it as usual. Otherwise it is deleted, and a version
// directory that exists is never touched. The entry is the plugin as the
// lockfile has it, or nil, and the caller holds the store lock.
func (s *Store) removeAsides(id string, e *entry) {
	dir, err := s.pluginPath(id)
	if err != nil {
		return
	}

	entries, _ := os.ReadDir(dir)
	for _, item := range entries {
		version, ok := strings.CutPrefix(item.Name(), asidePrefix)
		if !ok {
			continue
		}

		aside := filepath.Join(dir, item.Name())
		final := s.versionPath(id, version)

		if e != nil && e.Versions[version] != nil && isCanonicalVersion(version) {
			if _, err := os.Lstat(final); errors.Is(err, fs.ErrNotExist) && renameFile(aside, final) == nil {
				continue
			}
		}

		_ = os.RemoveAll(aside)
	}
}
