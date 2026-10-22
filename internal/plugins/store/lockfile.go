package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/auth0/auth0-cli/internal/plugins"
)

const (
	// LockfileFormatVersion is the only format of the lockfile this CLI reads.
	lockfileFormatVersion = 1

	// MaxLockfileBytes is the largest lockfile that is read.
	maxLockfileBytes = 1 << 20
)

var (
	sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	hashPattern   = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	binPattern    = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]*$`)
)

// lockfile is the content of plugins.lock.json: which plugins are installed,
// which version of each is active, and the hashes that the files of each
// version must have.
type lockfile struct {
	FormatVersion int               `json:"formatVersion"`
	Plugins       map[string]*entry `json:"plugins"`
}

// entry is one installed plugin.
type entry struct {
	Registry string `json:"registry"`
	Name     string `json:"name"`

	// Active is the version that runs. It is a field and not a symlink, because
	// symlinks need extra privileges on Windows.
	Active string `json:"active"`

	// Previous is the version that was active before, which stays on disk so that
	// going back needs no download.
	Previous string `json:"previous,omitempty"`

	// Pinned is true when an exact version was installed, which keeps update
	// --all from moving the plugin.
	Pinned bool `json:"pinned"`

	Versions map[string]*installedVersion `json:"versions"`
}

// installedVersion is what was verified when one version was installed.
type installedVersion struct {
	// Bin is the file name of the executable in the version directory.
	Bin string `json:"bin"`

	// BinSHA256 and ManifestSHA256 are the hashes of the executable and of
	// plugin.json as they were unpacked. Verify checks them before every run.
	BinSHA256      string `json:"binSha256"`
	ManifestSHA256 string `json:"manifestSha256"`

	// ArtifactSHA256 is the hash of the downloaded archive, as the index lists it.
	ArtifactSHA256 string `json:"artifactSha256"`

	// PermissionsHash is the consent hash of the manifest.
	PermissionsHash string `json:"permissionsHash"`

	// Size is the size of the executable in bytes.
	Size        int64     `json:"size"`
	InstalledAt time.Time `json:"installedAt"`
}

// readLockfile reads the lockfile. A missing file is an empty lockfile. A file
// that cannot be read, is too large, has duplicate keys, has a format version
// this CLI does not know, or is not consistent is an error that wraps
// ErrLockfileDamaged, and the file is left alone: it is never replaced
// automatically.
//
// It takes no lock, because the file is always replaced by a rename.
func (s *Store) readLockfile() (*lockfile, error) {
	path := filepath.Join(s.Dir, lockfileName)

	data, err := readRegularFile(path, maxLockfileBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return &lockfile{FormatVersion: lockfileFormatVersion, Plugins: map[string]*entry{}}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("%w (%w)", ErrLockfileDamaged, err)
	}

	var current lockfile

	if err := plugins.UnmarshalStrict(data, &current); err != nil {
		return nil, fmt.Errorf("%w (%s: %w)", ErrLockfileDamaged, path, err)
	}

	if err := current.validate(); err != nil {
		return nil, fmt.Errorf("%w (%s: %w)", ErrLockfileDamaged, path, err)
	}

	return &current, nil
}

// validate checks that the lockfile follows the format and agrees with itself,
// so that everything the store builds a path from has a safe form.
func (l *lockfile) validate() error {
	if l.FormatVersion != lockfileFormatVersion {
		return fmt.Errorf("format version %d is not supported, this CLI reads version %d", l.FormatVersion, lockfileFormatVersion)
	}

	if l.Plugins == nil {
		l.Plugins = map[string]*entry{}
	}

	for id, e := range l.Plugins {
		if err := e.validate(id); err != nil {
			return fmt.Errorf("plugins[%q]: %w", id, err)
		}
	}

	return nil
}

// validate checks one entry, which is stored under id.
func (e *entry) validate(id string) error {
	switch {
	case e == nil:
		return errors.New("the entry is empty")
	case !idPattern.MatchString(id):
		return errors.New("the key is not a plugin ID in the form registry/name")
	case e.Registry+"/"+e.Name != id:
		return errors.New("registry and name do not match the key")
	case e.Versions[e.Active] == nil:
		return fmt.Errorf("the active version %q is not among the versions", e.Active)
	case e.Previous != "" && (e.Previous == e.Active || e.Versions[e.Previous] == nil):
		return fmt.Errorf("the previous version %q is not another one of the versions", e.Previous)
	}

	for version, installed := range e.Versions {
		if !isCanonicalVersion(version) {
			return fmt.Errorf("versions[%q]: not a version in the form MAJOR.MINOR.PATCH", version)
		}

		if err := installed.validate(); err != nil {
			return fmt.Errorf("versions[%q]: %w", version, err)
		}
	}

	return nil
}

// validate checks one installed version.
func (v *installedVersion) validate() error {
	switch {
	case v == nil:
		return errors.New("the entry is empty")
	case !binPattern.MatchString(v.Bin) || v.Bin == manifestFileName:
		return fmt.Errorf("bin %q is not a plain file name", v.Bin)
	case !sha256Pattern.MatchString(v.BinSHA256):
		return errors.New("binSha256 is not a lowercase hex SHA-256")
	case !sha256Pattern.MatchString(v.ManifestSHA256):
		return errors.New("manifestSha256 is not a lowercase hex SHA-256")
	case !sha256Pattern.MatchString(v.ArtifactSHA256):
		return errors.New("artifactSha256 is not a lowercase hex SHA-256")
	case !hashPattern.MatchString(v.PermissionsHash):
		return errors.New("permissionsHash is not sha256: followed by a lowercase hex SHA-256")
	}

	return nil
}

// writeLockfile replaces the lockfile with l. The caller must hold the store
// lock, and must have read the lockfile it changed under that lock. Readers see
// either the old file or the new one.
func (s *Store) writeLockfile(l *lockfile) error {
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode the plugin lockfile: %w", err)
	}

	if err := writeFileAtomic(filepath.Join(s.Dir, lockfileName), data); err != nil {
		return fmt.Errorf("failed to write the plugin lockfile: %w", err)
	}

	return nil
}

// writeFileAtomic writes data to path through a temporary file in the same
// directory that is synced and then renamed, so that path holds either the old
// content or the new one, never a partial file. The file is private.
func writeFileAtomic(path string, data []byte) error {
	temp, err := plugins.StageTempFile(filepath.Dir(path), filepath.Base(path), data)
	if err != nil {
		return err
	}

	if err := renameFile(temp, path); err != nil {
		_ = os.Remove(temp)

		return err
	}

	plugins.SyncDir(filepath.Dir(path))

	return nil
}
