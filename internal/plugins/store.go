package plugins

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// storeVersion is the schema version of the on-disk plugins.json file.
const storeVersion = 1

// InstalledPlugin is the on-disk record of a plugin the user has installed.
type InstalledPlugin struct {
	Name        string      `json:"name"`
	Version     string      `json:"version"`
	InstallType InstallType `json:"install_type"`
	// Description is the plugin's one-line summary from the registry. It is
	// surfaced as the command's help text in `auth0 --help`.
	Description string `json:"description,omitempty"`
	// Package is the pinned npm spec for npm plugins (e.g. "checkmate@1.2.3").
	Package string `json:"package,omitempty"`
	// BinaryPath is the absolute path to the downloaded binary for
	// github-release plugins.
	BinaryPath string `json:"binary_path,omitempty"`
	// RequiredScopes are the Management API scopes the plugin declared. They
	// bound the plugin at the proxy and drive scope-gap re-authorization before
	// the plugin runs.
	RequiredScopes []string `json:"required_scopes,omitempty"`
}

// Store persists the set of installed plugins to plugins.json, next to the CLI
// config file (~/.config/auth0/plugins.json).
type Store struct {
	path string
}

// storeFile is the JSON envelope written to disk.
type storeFile struct {
	Version int               `json:"version"`
	Plugins []InstalledPlugin `json:"plugins"`
}

// NewStore returns a Store backed by the given file path. Callers typically pass
// DefaultStorePath.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// storePathEnvVar overrides the plugins.json location. It lets tests and local
// experiments keep their installed-plugin state out of the real user config.
const storePathEnvVar = "AUTH0_CLI_PLUGINS_STORE"

// DefaultStorePath returns the standard plugins.json location, alongside the CLI
// config directory. AUTH0_CLI_PLUGINS_STORE overrides it when set.
func DefaultStorePath() string {
	if override := os.Getenv(storePathEnvVar); override != "" {
		return override
	}
	return filepath.Join(os.Getenv("HOME"), ".config", "auth0", "plugins.json")
}

// List returns every installed plugin, sorted by name. A missing store file is
// treated as an empty list, not an error.
func (s *Store) List() ([]InstalledPlugin, error) {
	file, err := s.load()
	if err != nil {
		return nil, err
	}

	sort.Slice(file.Plugins, func(i, j int) bool {
		return file.Plugins[i].Name < file.Plugins[j].Name
	})

	return file.Plugins, nil
}

// Get returns the installed record for the named plugin, if present.
func (s *Store) Get(name string) (InstalledPlugin, bool, error) {
	file, err := s.load()
	if err != nil {
		return InstalledPlugin{}, false, err
	}

	for _, p := range file.Plugins {
		if p.Name == name {
			return p, true, nil
		}
	}

	return InstalledPlugin{}, false, nil
}

// Add inserts or replaces the record for a plugin (keyed by name) and persists
// the store.
func (s *Store) Add(plugin InstalledPlugin) error {
	file, err := s.load()
	if err != nil {
		return err
	}

	replaced := false
	for i := range file.Plugins {
		if file.Plugins[i].Name == plugin.Name {
			file.Plugins[i] = plugin
			replaced = true
			break
		}
	}
	if !replaced {
		file.Plugins = append(file.Plugins, plugin)
	}

	return s.save(file)
}

// Remove deletes the named plugin from the store. Removing a plugin that is not
// present is a no-op.
func (s *Store) Remove(name string) error {
	file, err := s.load()
	if err != nil {
		return err
	}

	filtered := file.Plugins[:0]
	for _, p := range file.Plugins {
		if p.Name != name {
			filtered = append(filtered, p)
		}
	}
	file.Plugins = filtered

	return s.save(file)
}

// load reads and parses the store file, returning an empty store if the file
// does not exist yet.
func (s *Store) load() (*storeFile, error) {
	buffer, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &storeFile{Version: storeVersion}, nil
		}
		return nil, fmt.Errorf("failed to read plugins store: %w", err)
	}

	// An empty file (e.g. never written, or an interrupted write) is an empty
	// store, not a parse error.
	if len(bytes.TrimSpace(buffer)) == 0 {
		return &storeFile{Version: storeVersion}, nil
	}

	var file storeFile
	if err := json.Unmarshal(buffer, &file); err != nil {
		return nil, fmt.Errorf("failed to parse plugins store: %w", err)
	}
	if file.Version == 0 {
		file.Version = storeVersion
	}

	return &file, nil
}

// save writes the store file with owner-only permissions, creating the parent
// directory if needed.
func (s *Store) save(file *storeFile) error {
	file.Version = storeVersion

	dir := filepath.Dir(s.path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		const dirPerm os.FileMode = 0o700
		if err := os.MkdirAll(dir, dirPerm); err != nil {
			return err
		}
	}

	buffer, err := json.MarshalIndent(file, "", "    ")
	if err != nil {
		return err
	}

	const filePerm os.FileMode = 0o600
	return os.WriteFile(s.path, buffer, filePerm)
}
