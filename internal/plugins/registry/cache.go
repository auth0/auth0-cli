package registry

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/auth0/auth0-cli/internal/plugins"
)

// The files and directories in the cache directory of a registry. The
// directory metadata holds the latest root, timestamp, snapshot and targets
// metadata, and the directory roots holds every root after the one that is
// embedded in the CLI, as <version>.root.json.
const (
	indexFileName   = "index.json"
	stateFileName   = "state.json"
	metadataDirName = "metadata"
	rootsDirName    = "roots"
)

// The names of the roles of the signed metadata, and of their files.
const (
	rootFileName      = "root.json"
	timestampFileName = "timestamp.json"
	snapshotFileName  = "snapshot.json"
	targetsFileName   = "targets.json"
)

// The permissions of the cache directory and of the scratch files that the
// refresh seeds go-tuf with, which only the current user needs.
const (
	cacheDirPerm  os.FileMode = 0o700
	cacheFilePerm os.FileMode = 0o600
)

// state is the content of state.json. It records what the CLI has seen of a
// registry, so that it can refuse to go back to an older index.
type state struct {
	// HighestVersion is the highest index version that was accepted.
	HighestVersion int `json:"highestVersion"`

	// LastRefresh is when the cached index was last replaced by a verified one.
	LastRefresh time.Time `json:"lastRefresh"`
}

// readCacheFile reads a file of the cache directory.
func (c *Client) readCacheFile(name string, limit int64) ([]byte, error) {
	file, err := os.Open(c.cachePath(name))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := plugins.ReadCapped(file, limit)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file.Name(), err)
	}

	return data, nil
}

// readState reads state.json. The error wraps fs.ErrNotExist when the file does
// not exist.
func (c *Client) readState() (state, error) {
	data, err := c.readCacheFile(stateFileName, maxStateBytes)
	if err != nil {
		return state{}, err
	}

	var current state

	if err := plugins.UnmarshalStrict(data, &current); err != nil {
		return state{}, fmt.Errorf("the registry state file in %s is damaged, remove that directory to start over: %w", c.Dir, err)
	}

	return current, nil
}

// previousFile is what a file of the cache directory held before it was
// replaced.
type previousFile struct {
	data   []byte
	exists bool

	// Whether the file could be read. It is false when the file exists but could
	// not be read, in which case it cannot be restored.
	readable bool
}

// The functions that persist uses to create and replace files, which tests
// replace to make them fail.
var (
	stageFile  = plugins.StageTempFile
	renameFile = plugins.Rename
)

// verified is what a refresh accepted, and persist stores.
type verified struct {
	// Roots are the roots that were walked and are not in the cache yet, by
	// version.
	Roots map[int64][]byte

	// FinalRoot is the version of the last root, which is Root. The roots in the
	// cache above it are removed, because they would be walked as part of the
	// chain by whoever reads the cache. It is 0 when persist must not remove any.
	FinalRoot int64

	// The latest metadata of each role, and the index that the targets metadata
	// vouches for.
	Root, Timestamp, Snapshot, Targets, Index []byte
}

// cacheFile is a file of the cache directory that persist writes, with the path
// relative to the directory, written with forward slashes.
type cacheFile struct {
	name  string
	data  []byte
	limit int64

	// Remove is set for a file that persist deletes instead of writing.
	remove bool
}

// cachePath returns the path in the cache directory of a file of the cache.
func (c *Client) cachePath(name string) string {
	return filepath.Join(c.Dir, filepath.FromSlash(name))
}

// rootFilePath returns the path, relative to the cache directory, of the cached
// root of a version.
func rootFilePath(version int64) string {
	return path.Join(rootsDirName, strconv.FormatInt(version, 10)+".root.json")
}

// metadataFilePath returns the path, relative to the cache directory, of the
// cached metadata of a role.
func metadataFilePath(fileName string) string {
	return path.Join(metadataDirName, fileName)
}

// persist stores a verified refresh and the new state. The cache directory must
// exist, which lock makes sure of, and the caller must hold the lock.
//
// The files that change are first written to temporary files and synced, so
// that a failure while writing, such as a full disk, happens before any file in
// the cache was replaced. The temporary files are then renamed one by one. If a
// rename fails, the files that were already replaced are put back as they were,
// in the opposite order, as far as that is possible. The directories are synced
// at the end so that the renames survive a crash.
//
// Every state that a crash between two renames can leave behind must fail
// closed, which is why the order is: the state, then the roots from the lowest
// version up, then the removal of the cached roots above the last one, then the
// root, timestamp, snapshot and targets metadata, and the index last. The roots
// above the last one only exist when a refresh replaced a cached chain that did
// not verify, and they have to go because a reader walks every root it finds.
// With only the new state in place, the index is older than the highest version.
// With new roots in place the metadata and the index are checked against the
// keys of the newest root, and metadata or an index that those keys no longer
// vouch for do not verify. New targets metadata next to the old index does not
// verify, because the index no longer has the length and the hash that the
// metadata gives. The index is replaced last, so that the cache is complete only
// once the new index is there. At no point does the cache accept something older
// than what the state remembers, so protection against an older index never
// weakens.
func (c *Client) persist(accepted verified, current state) error {
	encoded, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode the registry state: %w", err)
	}

	files := []cacheFile{{name: stateFileName, data: encoded, limit: maxStateBytes}}

	for _, version := range slices.Sorted(maps.Keys(accepted.Roots)) {
		files = append(files, cacheFile{name: rootFilePath(version), data: accepted.Roots[version], limit: maxMetadataBytes})
	}

	if accepted.FinalRoot > 0 {
		for _, version := range c.cachedRootsAbove(accepted.FinalRoot) {
			files = append(files, cacheFile{name: rootFilePath(version), limit: maxMetadataBytes, remove: true})
		}
	}

	files = append(files,
		cacheFile{name: metadataFilePath(rootFileName), data: accepted.Root, limit: maxMetadataBytes},
		cacheFile{name: metadataFilePath(timestampFileName), data: accepted.Timestamp, limit: maxMetadataBytes},
		cacheFile{name: metadataFilePath(snapshotFileName), data: accepted.Snapshot, limit: maxMetadataBytes},
		cacheFile{name: metadataFilePath(targetsFileName), data: accepted.Targets, limit: maxMetadataBytes},
		cacheFile{name: indexFileName, data: accepted.Index, limit: maxIndexBytes},
	)

	// A file that already has the content it should have is left alone.
	changed := make([]cacheFile, 0, len(files))
	previous := make([]previousFile, 0, len(files))

	for _, file := range files {
		before := c.readPrevious(file.name, file.limit)
		if !file.remove && before.readable && before.exists && bytes.Equal(before.data, file.data) {
			continue
		}

		changed = append(changed, file)
		previous = append(previous, before)
	}

	for _, dir := range []string{metadataDirName, rootsDirName} {
		if err := os.MkdirAll(c.cachePath(dir), cacheDirPerm); err != nil {
			return fmt.Errorf("failed to store the registry index: %w", err)
		}
	}

	staged := make([]string, 0, len(changed))

	// Whatever is still staged when this returns was not renamed, and goes away.
	defer func() {
		for _, temp := range staged {
			if temp != "" {
				_ = os.Remove(temp)
			}
		}
	}()

	for _, file := range changed {
		if file.remove {
			staged = append(staged, "")

			continue
		}

		temp, err := stageFile(filepath.Dir(c.cachePath(file.name)), path.Base(file.name), file.data)
		if err != nil {
			return fmt.Errorf("failed to store the registry index: %w", err)
		}

		staged = append(staged, temp)
	}

	for n, file := range changed {
		var err error

		if file.remove {
			err = os.Remove(c.cachePath(file.name))
		} else {
			err = renameFile(staged[n], c.cachePath(file.name))
		}

		if err != nil {
			for m := n - 1; m >= 0; m-- {
				c.restore(changed[m].name, previous[m])
			}

			return fmt.Errorf("failed to store the registry index: %w", err)
		}
	}

	for _, dir := range []string{"", metadataDirName, rootsDirName} {
		plugins.SyncDir(c.cachePath(dir))
	}

	return nil
}

// cachedRootsAbove returns the versions of the cached roots that are higher than
// version, in ascending order.
func (c *Client) cachedRootsAbove(version int64) []int64 {
	entries, err := os.ReadDir(c.cachePath(rootsDirName))
	if err != nil {
		return nil
	}

	var versions []int64

	for _, entry := range entries {
		number, found := strings.CutSuffix(entry.Name(), ".root.json")
		if !found {
			continue
		}

		parsed, err := strconv.ParseInt(number, 10, 64)
		if err != nil || parsed <= version || rootFilePath(parsed) != path.Join(rootsDirName, entry.Name()) {
			continue
		}

		versions = append(versions, parsed)
	}

	slices.Sort(versions)

	return versions
}

// readPrevious reads a file of the cache directory before it is replaced.
func (c *Client) readPrevious(name string, limit int64) previousFile {
	data, err := c.readCacheFile(name, limit)

	switch {
	case err == nil:
		return previousFile{data: data, exists: true, readable: true}
	case errors.Is(err, fs.ErrNotExist):
		return previousFile{readable: true}
	default:
		return previousFile{exists: true}
	}
}

// restore puts a file of the cache directory back as it was. It does what it
// can and reports nothing, because it only runs after a failure that is already
// being reported.
func (c *Client) restore(name string, previous previousFile) {
	target := c.cachePath(name)

	switch {
	case !previous.readable:
	case previous.exists:
		_ = writeFileAtomic(target, previous.data)
	default:
		_ = os.Remove(target)
	}
}

// writeFileAtomic writes data to path through a temporary file in the same
// directory that is synced and then renamed, so that path holds either the old
// content or the new one, never a partial file.
func writeFileAtomic(path string, data []byte) error {
	temp, err := stageFile(filepath.Dir(path), filepath.Base(path), data)
	if err != nil {
		return err
	}

	if err := renameFile(temp, path); err != nil {
		_ = os.Remove(temp)

		return err
	}

	return nil
}
