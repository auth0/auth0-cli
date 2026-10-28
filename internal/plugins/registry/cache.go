package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/auth0/auth0-cli/internal/plugins"
)

// The files in the cache directory of a registry.
const (
	indexFileName     = "index.json"
	signatureFileName = "index.json.sig"
	stateFileName     = "state.json"
)

// The permissions of the cache directory, which only the current user needs.
const cacheDirPerm os.FileMode = 0o700

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
	file, err := os.Open(filepath.Join(c.Dir, name))
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

// persist stores a verified index, its signature and the new state. The cache
// directory must exist, which lock makes sure of, and the caller must hold the
// lock.
//
// All three files are first written to temporary files and synced, so that a
// failure while writing, such as a full disk, happens before any file in the
// cache was replaced. The temporary files are then renamed in the order state,
// signature, index. If a rename fails, the files that were already replaced are
// put back as they were, in the opposite order, as far as that is possible. The
// directory is synced at the end so that the renames survive a crash.
func (c *Client) persist(indexData, signatureData []byte, current state) error {
	encoded, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode the registry state: %w", err)
	}

	// The files are listed in the order they are renamed in, which is the state,
	// the signature, and the index last. A crash between two renames leaves a
	// cache that does not verify, which fails closed and never weakens the
	// protection against an older index: with only the new state in place, the
	// index is older than the highest version, and with the new signature as
	// well, the signature does not match the index.
	files := []struct {
		name  string
		data  []byte
		limit int64
	}{
		{stateFileName, encoded, maxStateBytes},
		{signatureFileName, signatureData, maxSignatureBytes},
		{indexFileName, indexData, maxIndexBytes},
	}

	staged := make([]string, 0, len(files))

	// Whatever is still staged when this returns was not renamed, and goes away.
	defer func() {
		for _, temp := range staged {
			_ = os.Remove(temp)
		}
	}()

	for _, file := range files {
		temp, err := stageFile(c.Dir, file.name, file.data)
		if err != nil {
			return fmt.Errorf("failed to store the registry index: %w", err)
		}

		staged = append(staged, temp)
	}

	previous := make([]previousFile, len(files))
	for n, file := range files {
		previous[n] = c.readPrevious(file.name, file.limit)
	}

	for n, file := range files {
		if err := renameFile(staged[n], filepath.Join(c.Dir, file.name)); err != nil {
			for m := n - 1; m >= 0; m-- {
				c.restore(files[m].name, previous[m])
			}

			return fmt.Errorf("failed to store the registry index: %w", err)
		}
	}

	plugins.SyncDir(c.Dir)

	return nil
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
	path := filepath.Join(c.Dir, name)

	switch {
	case !previous.readable:
	case previous.exists:
		_ = writeFileAtomic(path, previous.data)
	default:
		_ = os.Remove(path)
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
