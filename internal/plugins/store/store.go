// Package store installs, verifies, updates and removes plugins on disk.
//
// A plugin is downloaded from the host that the signed registry index names,
// checked against the hash in the index, unpacked into a staging directory,
// compared with the registry record, and only then moved into place and
// recorded in the lockfile. What runs later is the binary that was installed:
// Verify hashes the binary and the manifest against the lockfile before every
// launch.
//
// The layout under the plugins directory is:
//
//	plugins.lock.json                        the lockfile, with the active version
//	.lock                                    the lock that serializes changes
//	store/<registry>/<name>/<version>/       the binary and plugin.json
//	tmp/                                     downloads and unpacking in progress
package store

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/filelock"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
)

// The names of the files and directories under the plugins directory.
const (
	lockfileName     = "plugins.lock.json"
	lockName         = ".lock"
	storeDirName     = "store"
	tmpDirName       = "tmp"
	manifestFileName = "plugin.json"
)

// The permissions of what the store writes. Directories and the files that only
// the CLI reads are private, and the executable can be run.
const (
	dirPerm    os.FileMode = 0o700
	filePerm   os.FileMode = 0o600
	binaryPerm os.FileMode = 0o755
)

// idPattern is the form of a plugin ID, registry/name. It is the same pattern
// as the index and the manifest schemas use, and what makes an ID safe to use
// as a path.
var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*/[a-z0-9][a-z0-9-]*$`)

// Approvals removes what the user approved for a plugin.
type Approvals interface {
	// RemoveAll deletes every approval of the plugin with the given ID.
	RemoveAll(pluginID string) error
}

// VersionSource lists the versions of a plugin. A *registry.Client does, and
// it also reads the versions of a plugin whose versions are in a separate file.
type VersionSource interface {
	Versions(ctx context.Context, plugin registry.Plugin) ([]registry.VersionRecord, error)
}

// Store is the set of installed plugins under one plugins directory.
type Store struct {
	// Dir is the plugins directory, <cli config dir>/plugins.
	Dir string

	// CLIVersion is the version of this CLI, which the engines constraint of a
	// plugin is checked against. A version that is not a semantic version, such
	// as the "dev" of a local build, skips that check.
	CLIVersion string

	// HTTP is the client to download with. When it is nil, a client that honors
	// HTTPS_PROXY is used. Whatever the client is, every request and every
	// redirect must go to an allowed download host over https.
	HTTP *http.Client

	// Approvals is called when a plugin is removed. When it is nil, there are no
	// approvals to remove.
	Approvals Approvals

	// Versions lists the versions of a plugin. When it is nil, only plugins whose
	// versions are in the index itself can be installed.
	Versions VersionSource
}

// Change describes what installing a version changes for the user, so that the
// Consent callback can ask for approval.
type Change struct {
	ID string

	// OldVersion is the active version, or empty when the plugin is not
	// installed.
	OldVersion string
	NewVersion string

	// Old holds the permissions and commands of the active version, read from
	// its installed manifest. They are empty when the plugin is not installed, or
	// the manifest cannot be read or does not match the hash in the lockfile.
	Old         plugins.Permissions
	OldCommands []plugins.ConsentCommand

	// New holds the permissions and commands of the new version, as the signed
	// index lists them.
	New         plugins.Permissions
	NewCommands []plugins.ConsentCommand
}

// ConsentFunc is called before anything is downloaded when the plugin is new or
// the permissions of the new version differ from the active one. Returning an
// error stops the change.
type ConsentFunc func(ctx context.Context, change Change) error

// Options are the optional parts of installing and updating.
type Options struct {
	// Consent is called as described at ConsentFunc. A nil Consent proceeds
	// without asking, which records no approval: approving is a separate step.
	Consent ConsentFunc
}

// Action says what an install did.
type Action string

// The things that an install can do.
const (
	// ActionInstalled means the version was downloaded and installed.
	ActionInstalled Action = "installed"

	// ActionSwitched means a version that was already on disk and verified was
	// made the active one, without a download.
	ActionSwitched Action = "switched"

	// ActionUnchanged means the version was already the active one.
	ActionUnchanged Action = "unchanged"
)

// Result is what an install or an update did.
type Result struct {
	ID string

	// Version is the active version afterwards.
	Version string

	// Previous is the version that was active before, which is empty when the
	// plugin was not installed.
	Previous string

	Action Action
	Pinned bool
}

// The functions that the store uses to move files into place, which tests
// replace to make them fail.
var renameFile = os.Rename

// The functions that tests replace to see what the store reads: readDir lists a
// version directory when the size of a plugin is added up, and currentPlatform
// tells which platform this machine is.
var (
	readDir         = os.ReadDir
	currentPlatform = plugins.CurrentPlatform
)

// pluginPath returns where the directory of a plugin is, or an error when the
// ID is not safe to use as a path.
func (s *Store) pluginPath(id string) (string, error) {
	if !idPattern.MatchString(id) {
		return "", fmt.Errorf("%q is not a plugin ID in the form registry/name", id)
	}

	return filepath.Join(s.Dir, storeDirName, filepath.FromSlash(id)), nil
}

// versionPath returns where the files of one version of a plugin are. The ID and
// the version must have been checked by the caller.
func (s *Store) versionPath(id, version string) string {
	return filepath.Join(s.Dir, storeDirName, filepath.FromSlash(id), version)
}

// isCanonicalVersion reports whether version is MAJOR.MINOR.PATCH written the
// one way that makes it safe to use as a directory name.
func isCanonicalVersion(version string) bool {
	parsed, ok := plugins.ParseVersion(version)

	return ok && parsed.String() == version
}

// compareVersions compares two versions. A string that is not a version sorts
// before every version.
func compareVersions(a, b string) int {
	left, leftOK := plugins.ParseVersion(a)
	right, rightOK := plugins.ParseVersion(b)

	switch {
	case !leftOK && !rightOK:
		return 0
	case !leftOK:
		return -1
	case !rightOK:
		return 1
	default:
		return left.Compare(right)
	}
}

// lock takes the store lock, creating the plugins directory when it does not
// exist, and returns the function that frees it. A lock that stays busy is
// reported as ErrBusy.
func (s *Store) lock(ctx context.Context) (release func(), err error) {
	if err := os.MkdirAll(s.Dir, dirPerm); err != nil {
		return nil, fmt.Errorf("failed to create the plugins directory: %w", err)
	}

	if err := os.Chmod(s.Dir, dirPerm); err != nil {
		return nil, fmt.Errorf("failed to restrict the plugins directory: %w", err)
	}

	return filelock.Acquire(ctx, filepath.Join(s.Dir, lockName), ErrBusy)
}

// resetTmp removes whatever an earlier install left in the tmp directory and
// creates it again. The caller must hold the store lock, which is what makes it
// safe: nothing else is installing.
func (s *Store) resetTmp() (string, error) {
	tmp := filepath.Join(s.Dir, tmpDirName)

	if err := os.RemoveAll(tmp); err != nil {
		return "", fmt.Errorf("failed to clear %s: %w", tmp, err)
	}

	if err := os.MkdirAll(tmp, dirPerm); err != nil {
		return "", fmt.Errorf("failed to create %s: %w", tmp, err)
	}

	return tmp, nil
}

// machineName names this machine for messages: its platform, such as
// linux-x64-musl, or only the system and the architecture when the machine is not
// a plugin platform.
func machineName(platform plugins.Platform) string {
	if platform != "" {
		return string(platform)
	}

	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x64"
	}

	return runtime.GOOS + "-" + arch
}
