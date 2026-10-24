// Package devlink records the local builds of plugins that a plugin author runs
// as auth0 <name> without publishing them, and reads such a build from disk.
//
// The links are kept in dev-links.json in the plugins directory, next to the
// lockfile. A link names the directory of the build and its executable, both as
// absolute paths. Nothing here decides whether a linked plugin may run: that is
// the job of the caller, which must also keep linked plugins out of reach unless
// dev mode is on.
package devlink

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"

	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/filelock"
)

const (
	// FileName is the name of the file that holds the links, in the plugins
	// directory.
	FileName = "dev-links.json"

	// ManifestName is the name of the manifest in the directory of a build.
	ManifestName = "plugin.json"

	// The lock that serializes changes to the links. It is not the lock of the
	// store, so that a long download never holds up a link.
	lockName = ".dev-links.lock"

	// The only format of the file this CLI reads.
	formatVersion = 1

	// The largest file, manifest or dev-links.json, that is read, in bytes.
	maxFileBytes = 1 << 20

	dirPerm os.FileMode = 0o700
)

var (
	// ErrDamaged means dev-links.json cannot be trusted. The file is left alone
	// and never replaced automatically.
	ErrDamaged = errors.New("the dev links file is damaged")

	// Another auth0 process held the lock of the links for too long.
	errBusy = errors.New("another auth0 process is changing the dev links, try again in a moment")

	// ErrNotLinked means no plugin with the ID is linked.
	ErrNotLinked = errors.New("the plugin is not linked")
)

// The function that moves the new file into place, which tests replace to make
// it fail.
var renameFile = plugins.Rename

// Link is one linked build: the directory that holds its plugin.json and the
// executable to run. Both are absolute paths.
type Link struct {
	Path string `json:"path"`
	Bin  string `json:"bin"`
}

// Build is a linked plugin as it is on disk now.
type Build struct {
	// Dir is the directory of the build, ManifestPath its plugin.json and Bin its
	// executable.
	Dir          string
	ManifestPath string
	Bin          string

	// Manifest is the plugin.json as it was read and validated just now.
	Manifest *plugins.Manifest
}

// document is the content of dev-links.json.
type document struct {
	FormatVersion int             `json:"formatVersion"`
	Links         map[string]Link `json:"links"`
}

// Links is the set of links kept in one plugins directory.
type Links struct {
	// Dir is the plugins directory.
	Dir string
}

func (l Links) path() string {
	return filepath.Join(l.Dir, FileName)
}

// Load returns the links by plugin ID. A missing file is no links. A file that
// cannot be read, is too large, has a duplicate key, a field this CLI does not
// know, a format version it does not read or a path that is not absolute is an
// error that wraps ErrDamaged.
//
// It takes no lock, because the file is always replaced by a rename.
func (l Links) Load() (map[string]Link, error) {
	path := l.path()

	data, err := readRegularFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]Link{}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("%w (%w)", ErrDamaged, err)
	}

	doc, err := decode(data)
	if err != nil {
		return nil, fmt.Errorf("%w (%s: %w)", ErrDamaged, path, err)
	}

	return doc.Links, nil
}

// Add links the plugin, replacing the link it had.
func (l Links) Add(ctx context.Context, id string, link Link) error {
	if id == "" || !filepath.IsAbs(link.Path) || !filepath.IsAbs(link.Bin) {
		return fmt.Errorf("a link needs a plugin ID and absolute paths, got %q, %q and %q", id, link.Path, link.Bin)
	}

	return l.change(ctx, func(links map[string]Link) error {
		links[id] = link

		return nil
	})
}

// Remove drops the link of the plugin. It fails with ErrNotLinked when the
// plugin is not linked.
func (l Links) Remove(ctx context.Context, id string) error {
	return l.change(ctx, func(links map[string]Link) error {
		if _, found := links[id]; !found {
			return fmt.Errorf("%w: %s", ErrNotLinked, id)
		}

		delete(links, id)

		return nil
	})
}

// change reads the links under the lock, applies the change and writes them
// back, so that two processes never lose each other's change.
func (l Links) change(ctx context.Context, apply func(links map[string]Link) error) error {
	if err := os.MkdirAll(l.Dir, dirPerm); err != nil {
		return fmt.Errorf("failed to create the plugins directory: %w", err)
	}

	if err := os.Chmod(l.Dir, dirPerm); err != nil {
		return fmt.Errorf("failed to restrict the plugins directory: %w", err)
	}

	release, err := filelock.Acquire(ctx, filepath.Join(l.Dir, lockName), errBusy)
	if err != nil {
		return err
	}
	defer release()

	links, err := l.Load()
	if err != nil {
		return err
	}

	if err := apply(links); err != nil {
		return err
	}

	return l.write(links)
}

// write replaces dev-links.json through a temporary file that is synced and then
// renamed, so that readers see either the old file or the new one. The caller
// holds the lock.
func (l Links) write(links map[string]Link) error {
	data, err := json.MarshalIndent(document{FormatVersion: formatVersion, Links: links}, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode the dev links: %w", err)
	}

	temp, err := plugins.StageTempFile(l.Dir, FileName, append(data, '\n'))
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", l.path(), err)
	}

	if err := renameFile(temp, l.path()); err != nil {
		_ = os.Remove(temp)

		return fmt.Errorf("failed to write %s: %w", l.path(), err)
	}

	plugins.SyncDir(l.Dir)

	return nil
}

// decode reads and checks the content of dev-links.json.
func decode(data []byte) (*document, error) {
	var doc document

	if err := plugins.UnmarshalStrict(data, &doc); err != nil {
		return nil, err
	}

	// Decoding ignores a field it does not know, and matches the name of a field
	// without regard to case, so that "Links" would be read as "links". The file
	// is accepted only if writing what was read gives the same document back.
	canonical, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}

	var got, want any

	if err := json.Unmarshal(data, &got); err != nil {
		return nil, err
	}

	if err := json.Unmarshal(canonical, &want); err != nil {
		return nil, err
	}

	if !reflect.DeepEqual(got, want) {
		return nil, errors.New("it has a field this CLI does not know, a field that is not written in the exact case, or a field that is missing")
	}

	if doc.FormatVersion != formatVersion {
		return nil, fmt.Errorf("format version %d is not supported, this CLI reads version %d", doc.FormatVersion, formatVersion)
	}

	if doc.Links == nil {
		doc.Links = map[string]Link{}
	}

	for id, link := range doc.Links {
		if id == "" || !filepath.IsAbs(link.Path) || !filepath.IsAbs(link.Bin) {
			return nil, fmt.Errorf("links[%q]: the plugin ID is empty or a path is not absolute", id)
		}
	}

	return &doc, nil
}

// Open reads the build of the link as it is on disk now: the manifest is read
// and validated again, it must be for the plugin with the ID, and the executable
// must be usable. Nothing is cached, so an edit shows at the next call.
func (l Link) Open(id string) (*Build, error) {
	manifestPath := filepath.Join(l.Path, ManifestName)

	manifest, err := ReadManifest(manifestPath)
	if err != nil {
		return nil, err
	}

	if manifest.ID != id {
		return nil, fmt.Errorf("%s is for the plugin %s, not %s", manifestPath, manifest.ID, id)
	}

	if err := CheckExecutable(l.Bin); err != nil {
		return nil, err
	}

	return &Build{Dir: l.Path, ManifestPath: manifestPath, Bin: l.Bin, Manifest: manifest}, nil
}

// ReadManifest reads and validates the plugin.json at the path, as the manifest
// of a plugin that is being published is. A manifest with problems is a
// *plugins.ValidationError.
func ReadManifest(path string) (*plugins.Manifest, error) {
	data, err := readRegularFile(path)
	if err != nil {
		return nil, err
	}

	return plugins.ParseManifest(data)
}

// DefaultBin returns where the executable of the plugin with the name is by
// convention: a file of that name in the directory of the build, with .exe added
// on Windows.
func DefaultBin(dir, name string) string {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}

	return filepath.Join(dir, name)
}

// CheckExecutable fails unless the path is a regular file once links are
// followed, and on systems that have an executable bit, one that can be run.
func CheckExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}

	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("%s is not executable, make it executable with: chmod +x %s", path, path)
	}

	return nil
}

// readRegularFile reads a file of at most maxFileBytes that is a regular file
// once links are followed.
func readRegularFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}

	data, err := plugins.ReadCapped(file, maxFileBytes)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return data, nil
}
