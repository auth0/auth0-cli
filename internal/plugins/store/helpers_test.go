package store_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
	"github.com/auth0/auth0-cli/internal/plugins/registry/registrytest"
	"github.com/auth0/auth0-cli/internal/plugins/store"
)

const (
	testID  = "auth0/checkmate"
	testBin = "checkmate"
)

// release describes one version of the test plugin that a test publishes.
type release struct {
	t       *testing.T
	id      string
	version string
	bin     string
	format  registry.ArtifactFormat
	binary  []byte

	// Record is the registry record, which a mutator may change before it is
	// sealed.
	record registry.VersionRecord

	// ManifestRecord, when it is set, changes the record that the manifest in the
	// archive is built from, so that the manifest differs from the registry.
	manifestRecord func(registry.VersionRecord) registry.VersionRecord

	// Archive, when it is set, replaces the archive that is built.
	archive func(r *release) []byte

	// Platform is the platform of the artifact, which is this machine unless a
	// test says otherwise.
	platform plugins.Platform

	// SkipServing is true when a test serves the archive itself.
	skipServing bool
}

// fixture is a store with a registry that serves releases of the test plugin.
type fixture struct {
	t        *testing.T
	server   *registrytest.Server
	store    *store.Store
	dir      string
	platform plugins.Platform
	policy   registry.Policy
}

// newFixture returns a store in an empty directory whose downloads go to a test
// server, for CLI version 1.39.0. It skips the test on a machine that is not one
// of the plugin platforms.
func newFixture(t *testing.T) *fixture {
	t.Helper()

	platform := plugins.CurrentPlatform()
	if platform == "" {
		t.Skip("this machine is not one of the plugin platforms")
	}

	server := registrytest.NewServer(t)
	dir := filepath.Join(t.TempDir(), "plugins")

	return &fixture{
		t:        t,
		server:   server,
		dir:      dir,
		platform: platform,
		store:    &store.Store{Dir: dir, CLIVersion: "1.39.0", HTTP: server.AnyHostClient()},
	}
}

// otherProcess returns a second store on the same directory, as another auth0
// process would have.
func (f *fixture) otherProcess() *store.Store {
	return &store.Store{Dir: f.dir, CLIVersion: "1.39.0", HTTP: f.server.AnyHostClient()}
}

// artifactPath returns the path of the archive of a version on the test server.
func artifactPath(version string) string {
	return "/auth0/auth0-cli-plugins/releases/download/v" + version + "/checkmate.tar.gz"
}

// release builds and serves a version of the test plugin for this machine, and
// returns its record. The mutators run before the record is sealed.
func (f *fixture) release(version string, mutate ...func(*release)) registry.VersionRecord {
	f.t.Helper()

	r := &release{
		t:        f.t,
		id:       testID,
		version:  version,
		bin:      testBin,
		format:   registry.ArtifactFormatTarGz,
		binary:   []byte("#!/bin/sh\necho checkmate " + version + "\n"),
		record:   registrytest.NewVersion(version),
		platform: f.platform,
	}
	r.record.Platforms = []plugins.Platform{f.platform}

	for _, change := range mutate {
		change(r)
	}

	r.record = registrytest.Seal(r.record)

	var archive []byte

	switch {
	case r.archive != nil:
		archive = r.archive(r)
	default:
		manifest := r.record
		if r.manifestRecord != nil {
			manifest = r.manifestRecord(r.record)
		}

		archive = registrytest.Package(r.id, manifest, r.format, r.bin, r.binary)
	}

	path := artifactPath(version)
	if r.format == registry.ArtifactFormatZip {
		path = "/auth0/auth0-cli-plugins/releases/download/v" + version + "/checkmate.zip"
	}

	r.record.Artifacts = []registry.Artifact{
		registrytest.NewArtifact(r.platform, "https://github.com"+path, r.format, r.bin, archive),
	}

	if !r.skipServing {
		f.server.Set(path, archive)
	}

	return r.record
}

// index parses an index that lists the records as versions of the test plugin,
// whose latest version is the last record. The index goes through the same
// validation as one from a registry.
func (f *fixture) index(records ...registry.VersionRecord) *registry.Index {
	f.t.Helper()

	return f.indexFrom(registrytest.NewIndexBuilder().AddPlugin(registrytest.NewPlugin(testID, records...)).SetPolicy(f.policy))
}

// indexFrom parses the index of the builder.
func (f *fixture) indexFrom(builder *registrytest.IndexBuilder) *registry.Index {
	f.t.Helper()

	index, err := registry.ParseIndex(builder.Build())
	require.NoError(f.t, err)

	return index
}

// install installs a version, and fails the test when that fails.
func (f *fixture) install(index *registry.Index, version string) store.Result {
	f.t.Helper()

	result, err := f.store.Install(context.Background(), index, testID, version, store.Options{})
	require.NoError(f.t, err)

	return result
}

// requests returns how many times the archive of a version was requested.
func (f *fixture) requests(version string) int {
	return f.server.Requests(artifactPath(version))
}

// versionDir returns where the files of a version are installed.
func (f *fixture) versionDir(version string) string {
	return filepath.Join(f.dir, "store", "auth0", "checkmate", version)
}

// lockfilePath returns the path of the lockfile.
func (f *fixture) lockfilePath() string {
	return filepath.Join(f.dir, "plugins.lock.json")
}

// readLockfile returns the lockfile as generic JSON.
func (f *fixture) readLockfile() map[string]any {
	f.t.Helper()

	data, err := os.ReadFile(f.lockfilePath())
	require.NoError(f.t, err)

	var document map[string]any
	require.NoError(f.t, json.Unmarshal(data, &document))

	return document
}

// writeLockfile replaces the lockfile with the generic JSON.
func (f *fixture) writeLockfile(document map[string]any) {
	f.t.Helper()

	data, err := json.MarshalIndent(document, "", "  ")
	require.NoError(f.t, err)
	require.NoError(f.t, os.WriteFile(f.lockfilePath(), data, 0o600))
}

// lockfileEntry returns the lockfile entry of the test plugin.
func (f *fixture) lockfileEntry() map[string]any {
	f.t.Helper()

	return f.readLockfile()["plugins"].(map[string]any)[testID].(map[string]any)
}

// lockfileVersion returns the lockfile entry of an installed version.
func (f *fixture) lockfileVersion(version string) map[string]any {
	f.t.Helper()

	return f.lockfileEntry()["versions"].(map[string]any)[version].(map[string]any)
}

// editManifest changes the installed plugin.json of a version and returns the new
// content. The lockfile keeps the hash of the original.
func (f *fixture) editManifest(version string, mutate func(manifest map[string]any)) []byte {
	f.t.Helper()

	path := filepath.Join(f.versionDir(version), "plugin.json")

	data, err := os.ReadFile(path)
	require.NoError(f.t, err)

	var manifest map[string]any
	require.NoError(f.t, json.Unmarshal(data, &manifest))

	mutate(manifest)

	changed, err := json.Marshal(manifest)
	require.NoError(f.t, err)
	require.NoError(f.t, os.WriteFile(path, changed, 0o600))

	return changed
}

// rewriteManifest changes the installed plugin.json of a version and records the
// new hash in the lockfile, so that only the rules that look at the content of
// the manifest can refuse it.
func (f *fixture) rewriteManifest(version string, mutate func(manifest map[string]any)) {
	f.t.Helper()

	changed := f.editManifest(version, mutate)

	document := f.readLockfile()
	entry := document["plugins"].(map[string]any)[testID].(map[string]any)
	entry["versions"].(map[string]any)[version].(map[string]any)["manifestSha256"] = registrytest.SHA256Hex(changed)
	f.writeLockfile(document)
}

// dirNames lists the names in a directory, or nothing when it does not exist.
func dirNames(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}

	require.NoError(t, err)

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	return names
}

// leftovers lists what is in the tmp directory.
func (f *fixture) leftovers() []string {
	f.t.Helper()

	return dirNames(f.t, filepath.Join(f.dir, "tmp"))
}

// installedVersions lists the version directories on disk.
func (f *fixture) installedVersions() []string {
	f.t.Helper()

	return dirNames(f.t, filepath.Dir(f.versionDir("0.0.0")))
}

// assertNothingInstalled checks that a failed install left no lockfile, no
// version directory and nothing in tmp.
func (f *fixture) assertNothingInstalled(t *testing.T) {
	t.Helper()

	assert.NoFileExists(t, f.lockfilePath())
	assert.Empty(t, f.installedVersions())
	assert.Empty(t, f.leftovers())
}

// activeVersion returns the version that Verify finds active.
func (f *fixture) activeVersion(t *testing.T) string {
	t.Helper()

	verified, err := f.store.Verify(testID)
	require.NoError(t, err)

	return verified.Version
}

// filesSize adds up the size of the executable and plugin.json of the versions.
func (f *fixture) filesSize(versions ...string) int64 {
	f.t.Helper()

	var total int64

	for _, version := range versions {
		for _, name := range []string{testBin, "plugin.json"} {
			info, err := os.Stat(filepath.Join(f.versionDir(version), name))
			require.NoError(f.t, err)

			total += info.Size()
		}
	}

	return total
}

// skipOnWindows skips a test that relies on Unix file modes or symlinks.
func skipOnWindows(t *testing.T) {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("relies on Unix file modes and symlinks")
	}
}

// goodEntries returns the two good entries of the test plugin, for a test to
// change.
func goodEntries(r *release) []registrytest.ArchiveEntry {
	return []registrytest.ArchiveEntry{
		{Name: r.bin, Body: r.binary},
		{Name: "plugin.json", Body: registrytest.Manifest(r.id, r.record)},
	}
}

// repeated returns n copies of the byte.
func repeated(b byte, n int) []byte {
	return bytes.Repeat([]byte{b}, n)
}

// decodeObject decodes a JSON object.
func decodeObject(t *testing.T, data []byte) map[string]any {
	t.Helper()

	var object map[string]any
	require.NoError(t, json.Unmarshal(data, &object))

	return object
}

// encodeObject encodes a JSON object.
func encodeObject(t *testing.T, object map[string]any) []byte {
	t.Helper()

	data, err := json.Marshal(object)
	require.NoError(t, err)

	return data
}

// archiveOf returns the archive that the record of a release points to, as the
// test server serves it.
func (f *fixture) archiveOf(t *testing.T, record registry.VersionRecord) []byte {
	t.Helper()

	response, err := f.server.AnyHostClient().Get(record.Artifacts[0].URL)
	require.NoError(t, err)

	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)

	return body
}
