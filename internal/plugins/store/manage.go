package store

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
)

// Installed is an installed plugin, as List reports it.
type Installed struct {
	ID string

	// Active is the version that runs, and Previous the version kept for going
	// back, or empty.
	Active   string
	Previous string
	Pinned   bool
}

// List returns the installed plugins, sorted by ID. It reads only the lockfile:
// it does not verify the plugins, does not look at their files, and takes no
// lock. Sizes gives the space they use.
func (s *Store) List() ([]Installed, error) {
	current, err := s.readLockfile()
	if err != nil {
		return nil, err
	}

	ids := slices.Sorted(maps.Keys(current.Plugins))

	list := make([]Installed, 0, len(ids))

	for _, id := range ids {
		e := current.Plugins[id]
		list = append(list, Installed{ID: id, Active: e.Active, Previous: e.Previous, Pinned: e.Pinned})
	}

	return list, nil
}

// Sizes returns the number of bytes that each installed plugin uses on disk, all
// of its versions together, by plugin ID. Like List, it takes no lock.
func (s *Store) Sizes() (map[string]int64, error) {
	current, err := s.readLockfile()
	if err != nil {
		return nil, err
	}

	sizes := make(map[string]int64, len(current.Plugins))

	for id, e := range current.Plugins {
		sizes[id] = s.sizeOnDisk(id, e)
	}

	return sizes, nil
}

// sizeOnDisk adds up the files of every version of the entry. A version
// directory only holds files, so it does not look any deeper.
func (s *Store) sizeOnDisk(id string, e *entry) int64 {
	var total int64

	for version := range e.Versions {
		entries, err := readDir(s.versionPath(id, version))
		if err != nil {
			continue
		}

		for _, item := range entries {
			if info, err := item.Info(); err == nil && info.Mode().IsRegular() {
				total += info.Size()
			}
		}
	}

	return total
}

// Remove uninstalls a plugin and returns the number of bytes it freed.
//
// The lockfile entry goes first, so that the plugin can no longer run, then the
// version directories, and then the approvals through the Approvals of the
// store. When deleting a directory or the approvals fails, the plugin is already
// uninstalled and the error says what was left behind.
func (s *Store) Remove(ctx context.Context, id string) (int64, error) {
	dir, err := s.pluginPath(id)
	if err != nil {
		return 0, err
	}

	release, err := s.lock(ctx)
	if err != nil {
		return 0, err
	}
	defer release()

	current, err := s.readLockfile()
	if err != nil {
		return 0, err
	}

	e := current.Plugins[id]
	if e == nil {
		return 0, &NotInstalledError{ID: id}
	}

	freed := s.sizeOnDisk(id, e)

	delete(current.Plugins, id)

	if err := s.writeLockfile(current); err != nil {
		return 0, err
	}

	var problems []error

	if err := os.RemoveAll(dir); err != nil {
		problems = append(problems, fmt.Errorf("failed to delete %s: %w", dir, err))
	}

	// The registry directory goes too when this was its last plugin. Deleting it
	// fails when it is not empty, which is what is wanted.
	_ = os.Remove(filepath.Dir(dir))

	if s.Approvals != nil {
		if err := s.Approvals.RemoveAll(id); err != nil {
			problems = append(problems, fmt.Errorf("failed to remove the approvals of %s: %w", id, err))
		}
	}

	if len(problems) > 0 {
		return freed, fmt.Errorf("%s was uninstalled, but not everything was removed: %w", plugins.NameOf(id), errors.Join(problems...))
	}

	return freed, nil
}

// UpdateResult is what UpdateAll did for one plugin.
type UpdateResult struct {
	ID string

	// Result is what the update did. It is empty when the plugin was skipped or
	// the update failed.
	Result Result

	// Active is the version that was active before the update.
	Active string

	// Skipped is true for a pinned plugin, which UpdateAll leaves alone.
	// Available is then the newer version the registry has, or empty when the
	// pinned version is the newest one.
	Skipped   bool
	Available string

	// Err is why the update failed, which is a *DeclinedError when opts.Consent
	// refused it.
	Err error
}

// UpdateAll updates every installed plugin that is not pinned to the latest
// version in the index, as Update does, and reports every plugin in the order of
// their IDs. A plugin that fails does not stop the others. A pinned plugin is
// skipped, and the report names the newer version that is available for it.
//
// It returns an error only when the lockfile cannot be read or the context ends;
// the results so far are returned with it.
func (s *Store) UpdateAll(ctx context.Context, index *registry.Index, opts Options) ([]UpdateResult, error) {
	current, err := s.readLockfile()
	if err != nil {
		return nil, err
	}

	ids := slices.Sorted(maps.Keys(current.Plugins))
	results := make([]UpdateResult, 0, len(ids))

	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return results, err
		}

		active := current.Plugins[id].Active

		// Whether the plugin is pinned is decided under the store lock, so that a
		// pin set by another process meanwhile is kept.
		result, err := s.install(ctx, index, id, "", opts, modeUpdateAll)
		if errors.Is(err, errPinned) {
			results = append(results, UpdateResult{ID: id, Skipped: true, Active: active, Available: newerVersion(index, id, active)})

			continue
		}

		results = append(results, UpdateResult{ID: id, Result: result, Active: active, Err: err})
	}

	return results, nil
}

// newerVersion returns the latest version of the plugin in the index when it is
// newer than the active one, and an empty string otherwise.
func newerVersion(index *registry.Index, id, active string) string {
	listed, ok := index.FindPlugin(id)
	if !ok || compareVersions(listed.Latest, active) <= 0 {
		return ""
	}

	return listed.Latest
}
