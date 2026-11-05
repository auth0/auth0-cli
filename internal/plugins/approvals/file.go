package approvals

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"time"

	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/filelock"
)

const (
	// The name of the approvals file in the plugins directory.
	fileName = "approvals.json"

	// The name of the lock file that writers take. It is not the lock of the
	// store, because the store holds that one while it removes a plugin, which
	// calls RemoveAll.
	lockName = ".approvals.lock"

	// The only format of the approvals file this CLI reads.
	formatVersion = 1

	// The largest approvals file that is read or written.
	maxFileBytes = 1 << 20

	// The permissions of the plugins directory, which only the current user
	// needs.
	dirPerm os.FileMode = 0o700
)

var (
	// The form of a tenant domain before it is folded to lower case. It only
	// matches ASCII, so that folding cannot map another character onto a letter.
	tenantPattern = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$`)

	// The form of a plugin ID, registry/name. It is the pattern that the store
	// uses.
	idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*/[a-z0-9][a-z0-9-]*$`)

	hashPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

// renameFile replaces a file, which tests replace to make it fail.
var renameFile = os.Rename

// approval is what is stored for one plugin on one tenant.
type approval struct {
	PermissionsHash string    `json:"permissionsHash"`
	ApprovedAt      time.Time `json:"approvedAt"`
}

// table maps a tenant to a plugin ID to what was approved.
type table map[string]map[string]approval

// document is the content of approvals.json.
type document struct {
	FormatVersion int   `json:"formatVersion"`
	Tenants       table `json:"tenants"`
}

// path returns the location of the approvals file.
func (a *Approvals) path() string {
	return filepath.Join(a.Dir, fileName)
}

// checkDir returns an error unless the plugins directory is an absolute path. A
// relative or empty one would read and write approvals.json in whatever
// directory the command happens to run in.
func (a *Approvals) checkDir() error {
	if !filepath.IsAbs(a.Dir) {
		return fmt.Errorf("the plugins directory %q is not an absolute path", a.Dir)
	}

	return nil
}

// load reads the approvals. A missing file is an empty table. Anything else that
// is not a valid approvals file is an error that wraps ErrDamaged, and the file
// is left alone.
//
// It takes no lock, because the file is always replaced by a rename.
func (a *Approvals) load() (table, error) {
	if err := a.checkDir(); err != nil {
		return nil, err
	}

	data, err := readRegularFile(a.path())
	if errors.Is(err, fs.ErrNotExist) {
		return table{}, nil
	}

	if err != nil {
		return nil, a.damaged(err)
	}

	loaded, err := parse(data)
	if err != nil {
		return nil, a.damaged(err)
	}

	return loaded, nil
}

// damaged wraps the reason that the file cannot be used.
func (a *Approvals) damaged(reason error) error {
	return fmt.Errorf("%w (%s: %w), remove the file to start over, which asks for approval again", ErrDamaged, a.path(), reason)
}

// readRegularFile reads a file that must be a regular file and not a link, that
// other users cannot write to, and that is not larger than maxFileBytes. It
// looks at the path first and compares it with what was opened, so a file
// swapped for a link in between is refused too.
func readRegularFile(path string) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}

	if !before.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	after, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if !os.SameFile(before, after) {
		return nil, errors.New("changed while it was opened")
	}

	if runtime.GOOS != "windows" && after.Mode().Perm()&0o022 != 0 {
		return nil, errors.New("writable by other users")
	}

	return plugins.ReadCapped(file, maxFileBytes)
}

// parse decodes and validates the content of the approvals file. Every key must
// be exactly one that the format has, in exactly that case, because the decoder
// of encoding/json would accept other cases and the keys of the file decide
// what is approved.
func parse(data []byte) (table, error) {
	var top map[string]json.RawMessage

	if err := plugins.UnmarshalStrict(data, &top); err != nil {
		return nil, err
	}

	if err := checkKeys(top, "formatVersion", "tenants"); err != nil {
		return nil, err
	}

	var version int

	if err := json.Unmarshal(top["formatVersion"], &version); err != nil || version != formatVersion {
		return nil, fmt.Errorf("the format version is not %d, which is the only one this CLI reads", formatVersion)
	}

	var tenants map[string]map[string]json.RawMessage

	if err := json.Unmarshal(top["tenants"], &tenants); err != nil {
		return nil, fmt.Errorf("tenants: %w", err)
	}

	// A null decodes without an error, to a map that is nil.
	if tenants == nil {
		return nil, errors.New("tenants: not an object")
	}

	loaded := table{}

	for tenant, plugs := range tenants {
		if folded, err := foldTenant(tenant); err != nil || folded != tenant {
			return nil, fmt.Errorf("tenants[%q]: not a lower case tenant domain", tenant)
		}

		if plugs == nil {
			return nil, fmt.Errorf("tenants[%q]: not an object", tenant)
		}

		loaded[tenant] = map[string]approval{}

		for id, raw := range plugs {
			stored, err := parseApproval(id, raw)
			if err != nil {
				return nil, fmt.Errorf("tenants[%q][%q]: %w", tenant, id, err)
			}

			loaded[tenant][id] = stored
		}
	}

	return loaded, nil
}

// parseApproval decodes and validates one approval, which is stored under id.
func parseApproval(id string, raw json.RawMessage) (approval, error) {
	if !idPattern.MatchString(id) {
		return approval{}, errors.New("the key is not a plugin ID in the form registry/name")
	}

	var fields map[string]json.RawMessage

	if err := json.Unmarshal(raw, &fields); err != nil {
		return approval{}, err
	}

	if err := checkKeys(fields, "permissionsHash", "approvedAt"); err != nil {
		return approval{}, err
	}

	var stored approval

	if err := json.Unmarshal(fields["permissionsHash"], &stored.PermissionsHash); err != nil || !hashPattern.MatchString(stored.PermissionsHash) {
		return approval{}, errors.New("permissionsHash is not sha256: followed by a lowercase hex SHA-256")
	}

	if err := json.Unmarshal(fields["approvedAt"], &stored.ApprovedAt); err != nil {
		return approval{}, fmt.Errorf("approvedAt: %w", err)
	}

	return stored, nil
}

// checkKeys returns an error unless the object has the named keys, in exactly
// that case, and no other key.
func checkKeys(fields map[string]json.RawMessage, names ...string) error {
	for _, name := range names {
		if _, found := fields[name]; !found {
			return fmt.Errorf("the key %q is missing, or is written in another case", name)
		}
	}

	if len(fields) != len(names) {
		return fmt.Errorf("the object has keys other than %q", names)
	}

	return nil
}

// lock creates the plugins directory when it does not exist, takes the lock that
// writers share, and returns the function that frees it.
func (a *Approvals) lock(ctx context.Context) (release func(), err error) {
	if err := a.checkDir(); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(a.Dir, dirPerm); err != nil {
		return nil, fmt.Errorf("failed to create the plugins directory: %w", err)
	}

	return filelock.Acquire(ctx, filepath.Join(a.Dir, lockName), ErrBusy)
}

// write replaces the approvals file with t, through a temporary file that is
// synced and then renamed, so that readers see either the old file or the new
// one, never a partial file. The caller must hold the lock, and must have loaded
// the table it changed under that lock.
func (a *Approvals) write(t table) error {
	data, err := json.MarshalIndent(document{FormatVersion: formatVersion, Tenants: t}, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode the plugin approvals: %w", err)
	}

	if len(data) > maxFileBytes {
		return fmt.Errorf("failed to write the plugin approvals: %w", plugins.ErrTooLarge)
	}

	temp, err := plugins.StageTempFile(a.Dir, fileName, data)
	if err != nil {
		return fmt.Errorf("failed to write the plugin approvals: %w", err)
	}

	if err := renameFile(temp, a.path()); err != nil {
		_ = os.Remove(temp)

		return fmt.Errorf("failed to write the plugin approvals: %w", err)
	}

	plugins.SyncDir(a.Dir)

	return nil
}
