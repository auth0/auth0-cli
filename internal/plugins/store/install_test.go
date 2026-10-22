package store_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
	"github.com/auth0/auth0-cli/internal/plugins/registry/registrytest"
	"github.com/auth0/auth0-cli/internal/plugins/store"
)

func TestInstall_Succeeds(t *testing.T) {
	f := newFixture(t)
	record := f.release("1.0.0")
	index := f.index(record)

	result, err := f.store.Install(t.Context(), index, testID, "", store.Options{})

	require.NoError(t, err)
	assert.Equal(t, store.Result{ID: testID, Version: "1.0.0", Action: store.ActionInstalled}, result)

	verified, err := f.store.Verify(testID)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", verified.Version)
	assert.Equal(t, filepath.Join(f.versionDir("1.0.0"), testBin), verified.Bin)
	assert.Equal(t, record.PermissionsHash, verified.PermissionsHash)

	t.Run("records the hashes that Verify checks", func(t *testing.T) {
		binary, err := os.ReadFile(verified.Bin)
		require.NoError(t, err)

		manifest, err := os.ReadFile(verified.ManifestPath)
		require.NoError(t, err)

		installed := f.lockfileVersion("1.0.0")
		assert.Equal(t, registrytest.SHA256Hex(binary), installed["binSha256"])
		assert.Equal(t, registrytest.SHA256Hex(manifest), installed["manifestSha256"])
		assert.Equal(t, record.Artifacts[0].SHA256, installed["artifactSha256"])
		assert.Equal(t, record.PermissionsHash, installed["permissionsHash"])
		assert.Equal(t, testBin, installed["bin"])
		assert.EqualValues(t, len(binary), installed["size"])
	})

	t.Run("leaves nothing in tmp and uses private modes", func(t *testing.T) {
		assert.Empty(t, f.leftovers())

		if runtime.GOOS == "windows" {
			return
		}

		modes := map[string]os.FileMode{
			f.dir:                                  0o700,
			f.versionDir("1.0.0"):                  0o700,
			f.lockfilePath():                       0o600,
			filepath.Join(f.dir, ".lock"):          0o600,
			verified.Bin:                           0o755,
			verified.ManifestPath:                  0o600,
			filepath.Join(f.dir, "store"):          0o700,
			filepath.Join(f.dir, "store", "auth0"): 0o700,
		}

		for path, want := range modes {
			info, err := os.Stat(path)
			require.NoError(t, err, path)
			assert.Equal(t, want, info.Mode().Perm(), path)
		}
	})
}

func TestInstall_RestrictsAnExistingPluginsDirectory(t *testing.T) {
	skipOnWindows(t)

	f := newFixture(t)
	require.NoError(t, os.MkdirAll(f.dir, 0o755))
	require.NoError(t, os.Chmod(f.dir, 0o755))

	f.install(f.index(f.release("1.0.0")), "1.0.0")

	info, err := os.Stat(f.dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

func TestInstall_ZipArtifact(t *testing.T) {
	f := newFixture(t)
	record := f.release("1.0.0", func(r *release) { r.format = registry.ArtifactFormatZip })

	f.install(f.index(record), "1.0.0")

	_, err := f.store.Verify(testID)
	require.NoError(t, err)
}

func TestInstall_LatestAndPinning(t *testing.T) {
	f := newFixture(t)
	index := f.index(f.release("1.0.0"), f.release("2.0.0"))

	t.Run("an empty version installs the latest and does not pin", func(t *testing.T) {
		result := f.install(index, "")

		assert.Equal(t, "2.0.0", result.Version)
		assert.False(t, result.Pinned)
	})

	t.Run("an exact version pins, also when it is the active one", func(t *testing.T) {
		result := f.install(index, "2.0.0")

		assert.Equal(t, store.ActionUnchanged, result.Action, "nothing is downloaded or switched")
		assert.True(t, result.Pinned)

		installed, err := f.store.List()
		require.NoError(t, err)
		require.Len(t, installed, 1)
		assert.True(t, installed[0].Pinned)
		assert.Equal(t, 1, f.requests("2.0.0"))
	})
}

func TestInstall_IsIdempotent(t *testing.T) {
	f := newFixture(t)
	index := f.index(f.release("1.0.0"))

	f.install(index, "1.0.0")

	before, err := os.ReadFile(f.lockfilePath())
	require.NoError(t, err)

	result, err := f.store.Install(t.Context(), index, testID, "1.0.0", store.Options{})

	require.NoError(t, err)
	assert.Equal(t, store.ActionUnchanged, result.Action)
	assert.Equal(t, 1, f.requests("1.0.0"), "nothing was downloaded again")

	after, err := os.ReadFile(f.lockfilePath())
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "the lockfile was not rewritten")
}

func TestInstall_ReinstallsAVersionThatNoLongerVerifies(t *testing.T) {
	f := newFixture(t)
	index := f.index(f.release("1.0.0"))
	f.install(index, "1.0.0")

	require.NoError(t, os.WriteFile(filepath.Join(f.versionDir("1.0.0"), testBin), []byte("tampered"), 0o755))
	_, err := f.store.Verify(testID)
	require.Error(t, err)

	f.install(index, "1.0.0")

	assert.Equal(t, 2, f.requests("1.0.0"), "the version was downloaded again")

	_, err = f.store.Verify(testID)
	assert.NoError(t, err)
}

func TestInstall_ReplacesAnUnverifiedDirectory(t *testing.T) {
	f := newFixture(t)
	index := f.index(f.release("1.0.0"))

	require.NoError(t, os.MkdirAll(f.versionDir("1.0.0"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(f.versionDir("1.0.0"), "leftover"), []byte("x"), 0o600))

	f.install(index, "1.0.0")

	_, err := f.store.Verify(testID)
	require.NoError(t, err, "the leftover file is gone, so the directory has only the two expected files")
}

// otherBinary is the content of a different build of a version.
var otherBinary = []byte("#!/bin/sh\necho another build\n")

// otherBuild makes the release a different build of the same version.
func otherBuild(r *release) { r.binary = otherBinary }

func TestInstall_ReplacesADifferentBuildOfTheSameVersion(t *testing.T) {
	f := newFixture(t)
	f.install(f.index(f.release("1.0.0")), "1.0.0")

	result := f.install(f.index(f.release("1.0.0", otherBuild)), "1.0.0")

	assert.Equal(t, store.ActionInstalled, result.Action)
	assert.Equal(t, 2, f.requests("1.0.0"), "a different artifact is downloaded, not reused")

	verified, err := f.store.Verify(testID)
	require.NoError(t, err)

	binary, err := os.ReadFile(verified.Bin)
	require.NoError(t, err)
	assert.Equal(t, string(otherBinary), string(binary))
	assert.Equal(t, []string{"1.0.0"}, f.installedVersions(), "the copy that was moved aside is gone")
}

func TestInstall_KeepsTheOldCopyWhenReplacingAVersionFails(t *testing.T) {
	tests := []struct {
		name string

		// Later installs 2.0.0 after 1.0.0, so that 1.0.0 is the previous version
		// and not the active one.
		later bool

		// Fail picks the rename that fails.
		fail func(oldPath, newPath string) bool
	}{
		{"the active version, moving the new directory in", false, func(oldPath, _ string) bool { return filepath.Base(oldPath) == "unpacked" }},
		{"the active version, writing the lockfile", false, func(_, newPath string) bool { return filepath.Base(newPath) == "plugins.lock.json" }},
		{"the previous version, moving the new directory in", true, func(oldPath, _ string) bool { return filepath.Base(oldPath) == "unpacked" }},
		{"the previous version, writing the lockfile", true, func(_, newPath string) bool { return filepath.Base(newPath) == "plugins.lock.json" }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			index := f.index(f.release("1.0.0"), f.release("2.0.0"))
			f.install(index, "1.0.0")

			active, want := "1.0.0", []string{"1.0.0"}

			if test.later {
				f.install(index, "2.0.0")

				active, want = "2.0.0", []string{"1.0.0", "2.0.0"}
			}

			oldBinary, err := os.ReadFile(filepath.Join(f.versionDir("1.0.0"), testBin))
			require.NoError(t, err)

			lockfile, err := os.ReadFile(f.lockfilePath())
			require.NoError(t, err)

			republished := f.index(f.release("1.0.0", otherBuild), f.release("2.0.0"))

			restore := store.SetRenameFile(func(oldPath, newPath string) error {
				if test.fail(oldPath, newPath) {
					return errors.New("injected failure")
				}

				return os.Rename(oldPath, newPath)
			})

			_, err = f.store.Install(t.Context(), republished, testID, "1.0.0", store.Options{})

			restore()

			require.ErrorContains(t, err, "injected failure")

			after, err := os.ReadFile(f.lockfilePath())
			require.NoError(t, err)
			assert.Equal(t, string(lockfile), string(after), "the lockfile is as it was")

			verified, err := f.store.Verify(testID)
			require.NoError(t, err)
			assert.Equal(t, active, verified.Version)

			binary, err := os.ReadFile(filepath.Join(f.versionDir("1.0.0"), testBin))
			require.NoError(t, err)
			assert.Equal(t, string(oldBinary), string(binary), "the verified copy is back in place")

			assert.ElementsMatch(t, want, f.installedVersions(), "no new or moved-aside directory is left")
			assert.Empty(t, f.leftovers())
		})
	}
}

func TestInstall_RemovesDirectoriesAnInterruptedInstallMovedAside(t *testing.T) {
	f := newFixture(t)
	index := f.index(f.release("1.0.0"))

	stale := filepath.Join(filepath.Dir(f.versionDir("1.0.0")), ".aside-9.9.9")
	require.NoError(t, os.MkdirAll(stale, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(stale, testBin), []byte("old"), 0o700))

	f.install(index, "1.0.0")

	assert.Equal(t, []string{"1.0.0"}, f.installedVersions())
}

func TestInstall_PutsBackAVersionAnInterruptedInstallLeftOnlyAside(t *testing.T) {
	aside := func(f *fixture, version string) string {
		return filepath.Join(filepath.Dir(f.versionDir(version)), ".aside-"+version)
	}

	t.Run("a version the lockfile lists, with no version directory", func(t *testing.T) {
		f := newFixture(t)
		index := f.index(f.release("1.0.0"))
		f.install(index, "1.0.0")

		require.NoError(t, os.Rename(f.versionDir("1.0.0"), aside(f, "1.0.0")))

		result := f.install(index, "1.0.0")

		assert.Equal(t, store.ActionUnchanged, result.Action)
		assert.Equal(t, 1, f.requests("1.0.0"), "the restored copy is reused, not downloaded again")

		_, err := f.store.Verify(testID)
		require.NoError(t, err)
		assert.Equal(t, []string{"1.0.0"}, f.installedVersions(), "the aside directory is gone")
	})

	t.Run("a version the lockfile does not list is deleted", func(t *testing.T) {
		f := newFixture(t)
		index := f.index(f.release("1.0.0"), f.release("2.0.0"))
		f.install(index, "1.0.0")

		require.NoError(t, os.Mkdir(aside(f, "2.0.0"), 0o700))

		f.install(index, "1.0.0")

		assert.Equal(t, []string{"1.0.0"}, f.installedVersions())
	})

	t.Run("a version whose directory exists is deleted and the directory is left alone", func(t *testing.T) {
		f := newFixture(t)
		index := f.index(f.release("1.0.0"))
		f.install(index, "1.0.0")

		stale := aside(f, "1.0.0")
		require.NoError(t, os.Mkdir(stale, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(stale, testBin), []byte("old"), 0o700))

		before, err := os.Stat(f.versionDir("1.0.0"))
		require.NoError(t, err)

		f.install(index, "1.0.0")

		after, err := os.Stat(f.versionDir("1.0.0"))
		require.NoError(t, err)
		assert.True(t, os.SameFile(before, after), "the version directory was not replaced")
		assert.Equal(t, 1, f.requests("1.0.0"), "the copy that was in place is reused")
		assert.Equal(t, []string{"1.0.0"}, f.installedVersions())

		_, err = f.store.Verify(testID)
		require.NoError(t, err)
	})
}

func TestInstall_DoesNotReuseAVersionWhoseRecordHasOtherPermissions(t *testing.T) {
	f := newFixture(t)
	scopes := func(names ...string) func(*release) {
		return func(r *release) { r.record.Permissions.Scopes = names }
	}

	first := f.release("1.0.0", scopes("read:clients"))
	second := f.release("2.0.0", scopes("delete:clients", "read:clients"))
	index := f.index(first, second)
	f.install(index, "1.0.0")
	f.install(index, "2.0.0")

	// The registry lists 1.0.0 with the artifact that is installed, but with
	// other permissions than the manifest in that artifact has.
	changed := f.release("1.0.0", scopes("read:users"), func(r *release) { r.skipServing = true })
	changed.Artifacts = first.Artifacts

	asked := 0

	_, err := f.store.Install(t.Context(), f.index(changed, second), testID, "1.0.0", store.Options{Consent: func(context.Context, store.Change) error {
		asked++

		return nil
	}})

	require.ErrorIs(t, err, store.ErrArtifactMismatch)
	assert.Equal(t, 1, asked, "the change of permissions is shown to the user")
	assert.Equal(t, 2, f.requests("1.0.0"), "the version was downloaded and checked, not reused")
	assert.Equal(t, "2.0.0", f.activeVersion(t))
}

func TestInstall_RefusesAnApprovalThatAnotherProcessMadeStale(t *testing.T) {
	f := newFixture(t)
	scopes := func(names ...string) func(*release) {
		return func(r *release) { r.record.Permissions.Scopes = names }
	}

	index := f.index(f.release("1.0.0", scopes("read:clients")), f.release("2.0.0", scopes("read:users")), f.release("3.0.0", scopes("read:users")))
	f.install(index, "1.0.0")

	other := f.otherProcess()

	_, err := f.store.Install(t.Context(), index, testID, "2.0.0", store.Options{Consent: func(ctx context.Context, _ store.Change) error {
		// Another process moves the plugin to 3.0.0 while the user is asked.
		_, err := other.Install(ctx, index, testID, "3.0.0", store.Options{})

		return err
	}})

	require.ErrorIs(t, err, store.ErrChanged)
	assert.Zero(t, f.requests("2.0.0"))
	assert.Equal(t, "3.0.0", f.activeVersion(t))
}

func TestInstall_RollbackNeedsNoDownload(t *testing.T) {
	f := newFixture(t)
	index := f.index(f.release("1.0.0"), f.release("2.0.0"))

	f.install(index, "1.0.0")
	f.install(index, "2.0.0")

	result := f.install(index, "1.0.0")

	assert.Equal(t, store.ActionSwitched, result.Action)
	assert.Equal(t, "2.0.0", result.Previous)
	assert.True(t, result.Pinned)
	assert.Equal(t, 1, f.requests("1.0.0"), "the old version was still on disk")

	verified, err := f.store.Verify(testID)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", verified.Version)

	assert.Equal(t, "2.0.0", f.lockfileEntry()["previous"])
}

func TestInstall_KeepsThePreviousVersionAndPrunesOlderOnes(t *testing.T) {
	f := newFixture(t)
	index := f.index(f.release("1.0.0"), f.release("2.0.0"), f.release("3.0.0"))

	f.install(index, "1.0.0")
	f.install(index, "2.0.0")
	assert.ElementsMatch(t, []string{"1.0.0", "2.0.0"}, f.installedVersions())

	f.install(index, "3.0.0")

	assert.ElementsMatch(t, []string{"2.0.0", "3.0.0"}, f.installedVersions(), "1.0.0 is older than the previous version")

	entry := f.lockfileEntry()
	assert.Equal(t, "3.0.0", entry["active"])
	assert.Equal(t, "2.0.0", entry["previous"])
	assert.Len(t, entry["versions"], 2)
}

func TestInstall_Consent(t *testing.T) {
	scopes := func(names ...string) func(*release) {
		return func(r *release) { r.record.Permissions.Scopes = names }
	}

	t.Run("is asked before anything is downloaded when the plugin is new", func(t *testing.T) {
		f := newFixture(t)
		index := f.index(f.release("1.0.0", scopes("read:clients")))

		var asked []store.Change

		_, err := f.store.Install(t.Context(), index, testID, "", store.Options{Consent: func(_ context.Context, change store.Change) error {
			asked = append(asked, change)
			assert.Zero(t, f.requests("1.0.0"), "nothing is downloaded before the answer")

			return nil
		}})

		require.NoError(t, err)
		require.Len(t, asked, 1)
		assert.Equal(t, "1.0.0", asked[0].NewVersion)
		assert.Empty(t, asked[0].OldVersion)
		assert.Equal(t, []string{"read:clients"}, asked[0].New.Scopes)
		assert.Equal(t, []plugins.ConsentCommand{{Name: "run"}}, asked[0].NewCommands)
	})

	t.Run("a refusal stops the install and nothing is downloaded or written", func(t *testing.T) {
		f := newFixture(t)
		index := f.index(f.release("1.0.0"))
		refusal := errors.New("not approved")

		_, err := f.store.Install(t.Context(), index, testID, "", store.Options{Consent: func(context.Context, store.Change) error { return refusal }})

		var declined *store.DeclinedError
		require.ErrorAs(t, err, &declined)
		assert.ErrorIs(t, err, refusal)
		assert.Zero(t, f.requests("1.0.0"))
		assert.NoDirExists(t, f.dir)
	})

	t.Run("is skipped when the permissions of the active version do not change", func(t *testing.T) {
		f := newFixture(t)
		index := f.index(f.release("1.0.0", scopes("read:clients")), f.release("2.0.0", scopes("read:clients")))
		f.install(index, "1.0.0")

		_, err := f.store.Install(t.Context(), index, testID, "2.0.0", store.Options{Consent: func(context.Context, store.Change) error {
			t.Error("consent must not be asked when the permissions are the same")

			return nil
		}})

		require.NoError(t, err)
	})

	t.Run("receives the old and the new permissions when they change", func(t *testing.T) {
		f := newFixture(t)
		index := f.index(f.release("1.0.0", scopes("read:clients")), f.release("2.0.0", scopes("read:clients", "read:users"), func(r *release) {
			r.record.Commands = []plugins.ConsentCommand{{Name: "run", Interactive: true}}
		}))
		f.install(index, "1.0.0")

		var asked store.Change

		_, err := f.store.Install(t.Context(), index, testID, "2.0.0", store.Options{Consent: func(_ context.Context, change store.Change) error {
			asked = change

			return nil
		}})

		require.NoError(t, err)
		assert.Equal(t, "1.0.0", asked.OldVersion)
		assert.Equal(t, "2.0.0", asked.NewVersion)
		assert.Equal(t, []string{"read:clients"}, asked.Old.Scopes)
		assert.Equal(t, []string{"read:clients", "read:users"}, asked.New.Scopes)
		assert.Equal(t, []plugins.ConsentCommand{{Name: "run"}}, asked.OldCommands)
		assert.Equal(t, []plugins.ConsentCommand{{Name: "run", Interactive: true}}, asked.NewCommands)
	})

	t.Run("shows everything when the installed manifest was edited", func(t *testing.T) {
		f := newFixture(t)
		index := f.index(f.release("1.0.0", scopes("read:clients")), f.release("2.0.0", scopes("read:clients", "read:users")))
		f.install(index, "1.0.0")

		// The edited manifest already lists the scope that 2.0.0 adds, so that a
		// comparison with it would not show the scope as new. The lockfile still
		// has the hash of the manifest that was installed.
		f.editManifest("1.0.0", func(manifest map[string]any) {
			manifest["commands"].([]any)[0].(map[string]any)["permissions"] = map[string]any{
				"scopes": []string{"read:clients", "read:users"}, "hosts": []any{}, "files": []any{}, "env": []any{},
			}
		})

		var asked store.Change

		_, err := f.store.Install(t.Context(), index, testID, "2.0.0", store.Options{Consent: func(_ context.Context, change store.Change) error {
			asked = change

			return nil
		}})

		require.NoError(t, err)
		assert.Equal(t, "1.0.0", asked.OldVersion)
		assert.Empty(t, asked.Old.Scopes, "the old permissions are not taken from a manifest that was changed")
		assert.Empty(t, asked.OldCommands)
		assert.Equal(t, []string{"read:clients", "read:users"}, asked.New.Scopes)
	})

	t.Run("a refusal keeps the active version", func(t *testing.T) {
		f := newFixture(t)
		index := f.index(f.release("1.0.0"), f.release("2.0.0", scopes("read:clients")))
		f.install(index, "1.0.0")

		_, err := f.store.Install(t.Context(), index, testID, "2.0.0", store.Options{Consent: func(context.Context, store.Change) error { return errors.New("no") }})

		require.Error(t, err)
		assert.Zero(t, f.requests("2.0.0"))

		verified, err := f.store.Verify(testID)
		require.NoError(t, err)
		assert.Equal(t, "1.0.0", verified.Version)
	})
}

func TestInstall_LeavesThePreviousStateWhenAStepFails(t *testing.T) {
	failRename := func(match func(newPath string) bool) func(*fixture) func() {
		return func(*fixture) func() {
			return store.SetRenameFile(func(oldPath, newPath string) error {
				if match(newPath) {
					return errors.New("injected failure")
				}

				return os.Rename(oldPath, newPath)
			})
		}
	}

	tests := []struct {
		name string

		// Release changes how version 2.0.0 is built.
		release func(*release)

		// Inject makes a step fail and returns the function that undoes it.
		inject func(*fixture) func()
	}{
		{"the server answers with an error", nil, func(f *fixture) func() {
			f.server.Fail(artifactPath("2.0.0"), 500)

			return func() {}
		}},
		{"the download has the wrong hash", func(r *release) {
			r.archive = func(r *release) []byte {
				return registrytest.TarGz(registrytest.ArchiveEntry{Name: r.bin, Body: r.binary})
			}
		}, func(f *fixture) func() {
			// The record keeps the hash of the good archive, and the server serves another.
			f.server.Set(artifactPath("2.0.0"), repeated('x', 64))

			return func() {}
		}},
		{"the archive holds a link", func(r *release) {
			r.archive = func(r *release) []byte {
				return registrytest.TarGz(registrytest.ArchiveEntry{Name: r.bin, Type: 's', Link: "/bin/sh"})
			}
		}, nil},
		{"the manifest differs from the record", func(r *release) {
			r.manifestRecord = func(record registry.VersionRecord) registry.VersionRecord {
				record.Version = "9.9.9"

				return record
			}
		}, nil},
		{"the directory cannot be moved into the store", nil, failRename(func(newPath string) bool {
			return filepath.Base(newPath) == "2.0.0"
		})},
		{"the lockfile cannot be written", nil, failRename(func(newPath string) bool {
			return filepath.Base(newPath) == "plugins.lock.json"
		})},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)

			second := []func(*release){}
			if test.release != nil {
				second = append(second, test.release)
			}

			index := f.index(f.release("1.0.0"), f.release("2.0.0", second...))
			f.install(index, "1.0.0")

			before, err := os.ReadFile(f.lockfilePath())
			require.NoError(t, err)

			if test.inject != nil {
				defer test.inject(f)()
			}

			_, err = f.store.Install(t.Context(), index, testID, "2.0.0", store.Options{})
			require.Error(t, err)

			after, err := os.ReadFile(f.lockfilePath())
			require.NoError(t, err)
			assert.Equal(t, string(before), string(after), "the lockfile is as it was")

			verified, err := f.store.Verify(testID)
			require.NoError(t, err)
			assert.Equal(t, "1.0.0", verified.Version, "the old version is still the one that runs")

			assert.Equal(t, []string{"1.0.0"}, f.installedVersions(), "nothing of the new version is left in the store")
			assert.Empty(t, f.leftovers())

			entries, err := os.ReadDir(f.dir)
			require.NoError(t, err)

			for _, entry := range entries {
				assert.NotContains(t, entry.Name(), ".tmp-", "no half written lockfile is left")
			}
		})
	}
}

func TestInstall_ClearsAStaleTmpDirectory(t *testing.T) {
	f := newFixture(t)
	index := f.index(f.release("1.0.0"))

	stale := filepath.Join(f.dir, "tmp", "install-123456", "unpacked")
	require.NoError(t, os.MkdirAll(stale, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(stale, testBin), []byte("half"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(f.dir, "tmp", "artifact"), []byte("half"), 0o600))

	f.install(index, "")

	assert.Empty(t, f.leftovers())
}

func TestInstall_ConcurrentInstallsTakeTurns(t *testing.T) {
	f := newFixture(t)
	index := f.index(f.release("1.0.0"))

	const installers = 4

	results := make([]store.Result, installers)
	errs := make([]error, installers)

	var wg sync.WaitGroup

	for n := range installers {
		wg.Add(1)

		go func() {
			defer wg.Done()

			other := f.otherProcess()
			results[n], errs[n] = other.Install(t.Context(), index, testID, "1.0.0", store.Options{})
		}()
	}

	wg.Wait()

	installed := 0

	for n := range installers {
		require.NoError(t, errs[n])

		if results[n].Action == store.ActionInstalled {
			installed++
		}
	}

	assert.Equal(t, 1, installed, "one process installed it and the others found it installed")
	assert.Equal(t, 1, f.requests("1.0.0"))

	_, err := f.store.Verify(testID)
	assert.NoError(t, err)
}

func TestInstall_GivesUpWhileAnotherProcessInstalls(t *testing.T) {
	f := newFixture(t)
	index := f.index(f.release("1.0.0"))

	entered := make(chan struct{})
	release := make(chan struct{})
	releaseOnce := sync.OnceFunc(func() { close(release) })

	defer releaseOnce()

	var once sync.Once

	f.server.OnRequest(func(path string, _ int) {
		if path == artifactPath("1.0.0") {
			once.Do(func() {
				close(entered)
				<-release
			})
		}
	})

	holder := make(chan error, 1)

	go func() {
		_, err := f.store.Install(context.Background(), index, testID, "1.0.0", store.Options{})
		holder <- err
	}()

	<-entered

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	other := f.otherProcess()
	_, err := other.Install(ctx, index, testID, "1.0.0", store.Options{})

	require.ErrorIs(t, err, store.ErrBusy)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	releaseOnce()
	require.NoError(t, <-holder)
}

// refSource is a version source for a plugin whose versions are in a separate
// file.
type refSource struct {
	records []registry.VersionRecord
}

func (r refSource) Versions(context.Context, registry.Plugin) ([]registry.VersionRecord, error) {
	return r.records, nil
}

func TestInstall_VersionsInASeparateFile(t *testing.T) {
	f := newFixture(t)
	record := f.release("1.0.0")
	index := f.indexFrom(registrytest.NewIndexBuilder().AddRefPlugin(registrytest.NewPlugin(testID, record)))

	t.Run("fails when the store cannot read the file", func(t *testing.T) {
		_, err := f.store.Install(t.Context(), index, testID, "", store.Options{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "separate file")
	})

	t.Run("installs through a version source", func(t *testing.T) {
		f.store.Versions = refSource{records: []registry.VersionRecord{record}}

		f.install(index, "")
	})
}
