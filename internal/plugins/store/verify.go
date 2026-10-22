package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/auth0/auth0-cli/internal/plugins"
)

// maxManifestBytes is the largest plugin.json that is unpacked or read.
const maxManifestBytes = 1 << 20

// Verified is an installed plugin whose files were checked against the lockfile.
type Verified struct {
	ID      string
	Version string

	// Dir is the directory of the version, Bin the executable in it and
	// ManifestPath the plugin.json next to it.
	Dir          string
	Bin          string
	ManifestPath string

	// Manifest is the parsed plugin.json that was checked, and PermissionsHash
	// its consent hash.
	Manifest        *plugins.Manifest
	PermissionsHash string
}

// Verify checks that the active version of a plugin is the one that was
// installed, and returns where it is. It is the check to run before every
// launch.
//
// It reads the lockfile and then requires that the version directory holds
// exactly the executable and plugin.json, both regular files and not links; that
// the SHA-256 of each equals the one recorded at install time; that the manifest
// parses; and that its consent hash equals the recorded one. It also refuses a
// version directory, executable or plugin.json that is writable by the group or
// by everyone, and an executable or plugin.json with more than one hard link,
// because another account or process could then rewrite the file after it was
// hashed. The plugins directory and the directories between it and the version
// directory must be real directories and not links, and not writable by the
// group or by everyone either. The checks of write access and hard links are
// skipped on Windows, which controls access with ACLs. Any difference is a *VerificationError, and nothing falls back to
// unverified files. The whole executable is hashed every time, without a
// shortcut based on its size or modification time, because those can be set to
// anything.
//
// Verify cannot close the gap between hashing the executable and the caller
// starting it: a process that can write to the file in that time can still
// replace it, and only the checks above make that harder.
//
// Revocation and the freshness of the registry index are not checked here. A
// caller checks them with registry.Index.Revoked and registry.Status.Runnable.
//
// Verify takes no lock. The files are in a directory only the user can write
// to, and a process that can change them can also change the lockfile.
func (s *Store) Verify(id string) (Verified, error) {
	if _, err := s.pluginPath(id); err != nil {
		return Verified{}, err
	}

	current, err := s.readLockfile()
	if err != nil {
		return Verified{}, err
	}

	e := current.Plugins[id]
	if e == nil {
		return Verified{}, &NotInstalledError{ID: id}
	}

	return s.verifyVersion(id, e, e.Active)
}

// verifyVersion is Verify for one version of an entry that came out of a
// validated lockfile.
func (s *Store) verifyVersion(id string, e *entry, version string) (Verified, error) {
	installed := e.Versions[version]
	dir := s.versionPath(id, version)

	fail := func(format string, args ...any) (Verified, error) {
		return Verified{}, &VerificationError{ID: id, Reason: fmt.Sprintf(format, args...)}
	}

	// Every directory from the plugins directory down to the version directory
	// must be a real directory that only the user can write to. Otherwise
	// another account could rename the version directory and put another one in
	// its place between this check and the launch.
	pluginDir := filepath.Dir(dir)
	registryDir := filepath.Dir(pluginDir)

	for _, path := range []string{s.Dir, filepath.Join(s.Dir, storeDirName), registryDir, pluginDir, dir} {
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() {
			return fail("%s is not a directory", path)
		}

		if reason := unsafeFile(info); reason != "" {
			return fail("%s %s", path, reason)
		}
	}

	if err := requireOnly(dir, installed.Bin, manifestFileName); err != nil {
		return fail("%v", err)
	}

	binPath := filepath.Join(dir, installed.Bin)
	manifestPath := filepath.Join(dir, manifestFileName)

	binSum, err := hashFile(binPath)
	if err != nil {
		return fail("%v", err)
	}

	if binSum != installed.BinSHA256 {
		return fail("the hash of %s changed", installed.Bin)
	}

	manifestData, err := readRegularFile(manifestPath, maxManifestBytes)
	if err != nil {
		return fail("%v", err)
	}

	if sum := sha256.Sum256(manifestData); hex.EncodeToString(sum[:]) != installed.ManifestSHA256 {
		return fail("the hash of %s changed", manifestFileName)
	}

	manifest, err := plugins.ParseInstalledManifest(manifestData)
	if err != nil {
		return fail("%s is not valid: %v", manifestFileName, err)
	}

	if manifest.ID != id || manifest.Version != version {
		return fail("%s is for %s %s", manifestFileName, manifest.ID, manifest.Version)
	}

	if manifest.PermissionsHash() != installed.PermissionsHash {
		return fail("the permissions of %s changed", manifestFileName)
	}

	return Verified{
		ID: id, Version: version, Dir: dir, Bin: binPath, ManifestPath: manifestPath,
		Manifest: manifest, PermissionsHash: installed.PermissionsHash,
	}, nil
}

// requireOnly checks that dir holds exactly the named entries.
func requireOnly(dir string, names ...string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	if len(entries) != len(names) {
		return fmt.Errorf("%s holds %d entries and %d are expected", dir, len(entries), len(names))
	}

	for _, entry := range entries {
		if !slices.Contains(names, entry.Name()) {
			return fmt.Errorf("%s holds an unexpected entry %q", dir, entry.Name())
		}
	}

	return nil
}

// errNotRegular means a path is a link, a directory or another special file.
var errNotRegular = errors.New("is not a regular file")

// openRegular opens a file that must be a regular file and not a link. It looks
// at the path first and compares it with what was opened, so a file swapped for
// a link in between is refused too.
func openRegular(path string) (*os.File, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}

	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("%s %w", path, errNotRegular)
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	after, err := file.Stat()
	if err != nil {
		_ = file.Close()

		return nil, err
	}

	if !after.Mode().IsRegular() || !os.SameFile(before, after) {
		_ = file.Close()

		return nil, fmt.Errorf("%s %w", path, errNotRegular)
	}

	if reason := unsafeFile(after); reason != "" {
		_ = file.Close()

		return nil, fmt.Errorf("%s %s", path, reason)
	}

	return file, nil
}

// hashFile returns the hex SHA-256 of a regular file.
func hashFile(path string) (string, error) {
	file, err := openRegular(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()

	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// readRegularFile reads a regular file that is not a link, which is at most
// limit bytes.
func readRegularFile(path string, limit int64) ([]byte, error) {
	file, err := openRegular(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := plugins.ReadCapped(file, limit)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return data, nil
}
