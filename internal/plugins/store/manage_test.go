package store_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins/registry"
	"github.com/auth0/auth0-cli/internal/plugins/registry/registrytest"
	"github.com/auth0/auth0-cli/internal/plugins/store"
)

func TestUpdate(t *testing.T) {
	t.Run("fails for a plugin that is not installed", func(t *testing.T) {
		f := newFixture(t)

		_, err := f.store.Update(t.Context(), f.index(f.release("1.0.0")), testID, store.Options{})

		var notInstalled *store.NotInstalledError
		require.ErrorAs(t, err, &notInstalled)
		assert.Zero(t, f.requests("1.0.0"))
	})

	t.Run("moves to the latest version and clears the pin", func(t *testing.T) {
		f := newFixture(t)
		index := f.index(f.release("1.0.0"), f.release("2.0.0"))
		f.install(index, "1.0.0")

		result, err := f.store.Update(t.Context(), index, testID, store.Options{})

		require.NoError(t, err)
		assert.Equal(t, store.Result{ID: testID, Version: "2.0.0", Previous: "1.0.0", Action: store.ActionInstalled}, result)

		installed, err := f.store.List()
		require.NoError(t, err)
		assert.False(t, installed[0].Pinned)
	})

	t.Run("clears the pin of a plugin that is already on the latest version", func(t *testing.T) {
		f := newFixture(t)
		index := f.index(f.release("1.0.0"))
		f.install(index, "1.0.0")

		result, err := f.store.Update(t.Context(), index, testID, store.Options{})

		require.NoError(t, err)
		assert.Equal(t, store.ActionUnchanged, result.Action)
		assert.Equal(t, 1, f.requests("1.0.0"))

		installed, err := f.store.List()
		require.NoError(t, err)
		assert.False(t, installed[0].Pinned)
	})

	t.Run("never goes back to an older version than the active one", func(t *testing.T) {
		f := newFixture(t)
		f.install(f.index(f.release("1.0.0"), f.release("2.0.0")), "")

		older := f.index(f.release("1.0.0", func(r *release) { r.record.Permissions.Scopes = []string{"read:clients"} }))

		result, err := f.store.Update(t.Context(), older, testID, store.Options{Consent: func(context.Context, store.Change) error {
			t.Error("consent must not be asked when there is nothing to update")

			return nil
		}})

		require.NoError(t, err)
		assert.Equal(t, store.Result{ID: testID, Version: "2.0.0", Previous: "2.0.0", Action: store.ActionUnchanged}, result)
		assert.Zero(t, f.requests("1.0.0"), "the older version was not downloaded")
		assert.Equal(t, "2.0.0", f.activeVersion(t))
	})
}

func TestUpdateAll(t *testing.T) {
	t.Run("updates plugins that are not pinned", func(t *testing.T) {
		f := newFixture(t)
		first, second := f.release("1.0.0"), f.release("2.0.0")

		_, err := f.store.Install(t.Context(), f.index(first), testID, "", store.Options{})
		require.NoError(t, err)

		results, err := f.store.UpdateAll(t.Context(), f.index(first, second), store.Options{})

		require.NoError(t, err)
		assert.Equal(t, []store.UpdateResult{{
			ID: testID, Active: "1.0.0",
			Result: store.Result{ID: testID, Version: "2.0.0", Previous: "1.0.0", Action: store.ActionInstalled},
		}}, results)
	})

	t.Run("skips pinned plugins and reports the newer version", func(t *testing.T) {
		f := newFixture(t)
		index := f.index(f.release("1.0.0"), f.release("2.0.0"))
		f.install(index, "1.0.0")

		results, err := f.store.UpdateAll(t.Context(), index, store.Options{})

		require.NoError(t, err)
		assert.Equal(t, []store.UpdateResult{{ID: testID, Skipped: true, Active: "1.0.0", Available: "2.0.0"}}, results)
		assert.Zero(t, f.requests("2.0.0"))
		assert.Equal(t, "1.0.0", f.activeVersion(t))
	})

	t.Run("reports no newer version for a pinned plugin that is the newest", func(t *testing.T) {
		f := newFixture(t)
		index := f.index(f.release("1.0.0"))
		f.install(index, "1.0.0")

		results, err := f.store.UpdateAll(t.Context(), index, store.Options{})

		require.NoError(t, err)
		assert.Equal(t, []store.UpdateResult{{ID: testID, Skipped: true, Active: "1.0.0"}}, results)
	})

	t.Run("keeps a pin that another process sets while it runs", func(t *testing.T) {
		f := newFixture(t)
		index := f.index(f.release("1.0.0"), f.release("2.0.0"))
		_, err := f.store.Install(t.Context(), index, testID, "", store.Options{})
		require.NoError(t, err)

		// Reading the versions of the plugin is the first thing an update does
		// after the plugins were listed, and here another process pins it then.
		other := f.otherProcess()
		f.store.Versions = pinningSource{pin: func() {
			_, err := other.Install(t.Context(), f.index(f.release("2.0.0")), testID, "2.0.0", store.Options{})
			require.NoError(t, err)
		}}

		results, err := f.store.UpdateAll(t.Context(), index, store.Options{})

		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.True(t, results[0].Skipped)
		assert.NoError(t, results[0].Err)

		installed, err := f.store.List()
		require.NoError(t, err)
		assert.True(t, installed[0].Pinned)
	})

	t.Run("reports a plugin that the registry no longer lists", func(t *testing.T) {
		f := newFixture(t)
		_, err := f.store.Install(t.Context(), f.index(f.release("1.0.0")), testID, "", store.Options{})
		require.NoError(t, err)

		results, err := f.store.UpdateAll(t.Context(), f.indexFrom(registrytest.NewIndexBuilder()), store.Options{})

		require.NoError(t, err)
		require.Len(t, results, 1)

		var notAvailable *store.NotAvailableError
		assert.ErrorAs(t, results[0].Err, &notAvailable)
	})

	t.Run("stops when the context ends", func(t *testing.T) {
		f := newFixture(t)
		index := f.index(f.release("1.0.0"))
		_, err := f.store.Install(t.Context(), index, testID, "", store.Options{})
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err = f.store.UpdateAll(ctx, index, store.Options{})

		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("asks for consent when the permissions change, and a refusal keeps the old version", func(t *testing.T) {
		f := newFixture(t)
		first := f.release("1.0.0")
		second := f.release("2.0.0", func(r *release) { r.record.Permissions.Scopes = []string{"read:clients"} })

		_, err := f.store.Install(t.Context(), f.index(first), testID, "", store.Options{})
		require.NoError(t, err)

		var asked []store.Change

		refuse := func(_ context.Context, change store.Change) error {
			asked = append(asked, change)

			return errors.New("not today")
		}

		results, err := f.store.UpdateAll(t.Context(), f.index(first, second), store.Options{Consent: refuse})

		require.NoError(t, err)
		require.Len(t, results, 1)

		var declined *store.DeclinedError
		require.ErrorAs(t, results[0].Err, &declined)
		assert.Equal(t, "2.0.0", declined.Version)

		require.Len(t, asked, 1)
		assert.Equal(t, "1.0.0", asked[0].OldVersion)
		assert.Empty(t, asked[0].Old.Scopes)
		assert.Equal(t, []string{"read:clients"}, asked[0].New.Scopes)

		assert.Equal(t, "1.0.0", f.activeVersion(t), "the old version stays active")
		assert.Zero(t, f.requests("2.0.0"), "and nothing was downloaded")

		results, err = f.store.UpdateAll(t.Context(), f.index(first, second), store.Options{Consent: func(context.Context, store.Change) error { return nil }})

		require.NoError(t, err)
		require.NoError(t, results[0].Err)
		assert.Equal(t, "2.0.0", f.activeVersion(t))
	})
}

// pinningSource is a version source that runs a function when it is asked, and
// then answers from the index.
type pinningSource struct {
	pin func()
}

func (p pinningSource) Versions(_ context.Context, plugin registry.Plugin) ([]registry.VersionRecord, error) {
	p.pin()

	return plugin.Versions, nil
}

// approvalsSpy records how it is called and what the store looked like then.
type approvalsSpy struct {
	f        *fixture
	called   []string
	err      error
	verified error
	existed  bool
}

func (a *approvalsSpy) RemoveAll(pluginID string) error {
	a.called = append(a.called, pluginID)
	_, a.verified = a.f.store.Verify(pluginID)

	_, statErr := os.Stat(filepath.Join(a.f.dir, "store", "auth0", "checkmate"))
	a.existed = statErr == nil

	return a.err
}

func TestRemove(t *testing.T) {
	t.Run("removes the lockfile entry first, then the files, then the approvals, and reports the space freed", func(t *testing.T) {
		f := newFixture(t)
		index := f.index(f.release("1.0.0"), f.release("2.0.0"))
		f.install(index, "1.0.0")
		f.install(index, "2.0.0")

		spy := &approvalsSpy{f: f}
		f.store.Approvals = spy

		want := f.filesSize("1.0.0", "2.0.0")

		freed, err := f.store.Remove(t.Context(), testID)

		require.NoError(t, err)
		assert.Equal(t, want, freed)
		assert.Equal(t, []string{testID}, spy.called)

		var notInstalled *store.NotInstalledError
		assert.ErrorAs(t, spy.verified, &notInstalled, "the plugin could no longer run when the approvals were removed")
		assert.False(t, spy.existed, "and its files were gone")

		assert.NoDirExists(t, filepath.Join(f.dir, "store", "auth0"), "the registry directory goes with the last plugin")

		installed, err := f.store.List()
		require.NoError(t, err)
		assert.Empty(t, installed)
	})

	t.Run("works without approvals", func(t *testing.T) {
		f := newFixture(t)
		f.install(f.index(f.release("1.0.0")), "1.0.0")

		_, err := f.store.Remove(t.Context(), testID)

		require.NoError(t, err)
	})

	t.Run("reports approvals that could not be removed, after the plugin is gone", func(t *testing.T) {
		f := newFixture(t)
		f.install(f.index(f.release("1.0.0")), "1.0.0")
		f.store.Approvals = &approvalsSpy{f: f, err: errors.New("disk full")}

		freed, err := f.store.Remove(t.Context(), testID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "disk full")
		assert.Positive(t, freed)

		_, verifyErr := f.store.Verify(testID)
		assert.Error(t, verifyErr)
	})

	t.Run("fails for a plugin that is not installed", func(t *testing.T) {
		f := newFixture(t)
		spy := &approvalsSpy{f: f}
		f.store.Approvals = spy

		_, err := f.store.Remove(t.Context(), testID)

		var notInstalled *store.NotInstalledError
		require.ErrorAs(t, err, &notInstalled)
		assert.Empty(t, spy.called)
	})

	t.Run("deletes nothing when the lockfile cannot be written", func(t *testing.T) {
		f := newFixture(t)
		f.install(f.index(f.release("1.0.0")), "1.0.0")

		spy := &approvalsSpy{f: f}
		f.store.Approvals = spy

		restore := store.SetRenameFile(func(oldPath, newPath string) error {
			if filepath.Base(newPath) == "plugins.lock.json" {
				return errors.New("injected failure")
			}

			return os.Rename(oldPath, newPath)
		})
		defer restore()

		_, err := f.store.Remove(t.Context(), testID)

		require.Error(t, err)
		assert.Empty(t, spy.called, "the approvals are kept")
		assert.Equal(t, "1.0.0", f.activeVersion(t), "the plugin still runs")
	})
}

func TestList_IsSortedByID(t *testing.T) {
	f := newFixture(t)

	const second = "auth0/second"

	index := f.indexFrom(registrytest.NewIndexBuilder().
		AddPlugin(registrytest.NewPlugin(testID, f.release("1.0.0"))).
		AddPlugin(registrytest.NewPlugin(second, f.release("3.0.0", func(r *release) { r.id = second }))))

	_, err := f.store.Install(t.Context(), index, second, "", store.Options{})
	require.NoError(t, err)

	f.install(index, "1.0.0")

	installed, err := f.store.List()

	require.NoError(t, err)
	require.Len(t, installed, 2)
	assert.Equal(t, testID, installed[0].ID)
	assert.True(t, installed[0].Pinned)
	assert.Equal(t, second, installed[1].ID)
	assert.False(t, installed[1].Pinned)
}

func TestList_DoesNotLookAtTheFilesOfThePlugins(t *testing.T) {
	f := newFixture(t)
	f.install(f.index(f.release("1.0.0")), "1.0.0")

	var listed int

	defer store.SetReadDir(func(name string) ([]os.DirEntry, error) {
		listed++

		return os.ReadDir(name)
	})()

	installed, err := f.store.List()

	require.NoError(t, err)
	require.Len(t, installed, 1)
	assert.Zero(t, listed, "List reads the lockfile only")

	_, err = f.store.Sizes()

	require.NoError(t, err)
	assert.Positive(t, listed, "Sizes is what reads the version directories")
}

func TestSizes(t *testing.T) {
	t.Run("adds up every version of a plugin", func(t *testing.T) {
		f := newFixture(t)
		index := f.index(f.release("1.0.0"), f.release("2.0.0"))
		f.install(index, "1.0.0")
		f.install(index, "2.0.0")

		sizes, err := f.store.Sizes()

		require.NoError(t, err)
		assert.Equal(t, map[string]int64{testID: f.filesSize("1.0.0", "2.0.0")}, sizes)
	})

	t.Run("is empty without a lockfile, and creates nothing", func(t *testing.T) {
		f := newFixture(t)

		sizes, err := f.store.Sizes()

		require.NoError(t, err)
		assert.Empty(t, sizes)
		assert.NoDirExists(t, f.dir)
	})
}

func TestManifests(t *testing.T) {
	t.Run("returns the manifest of the active version of every plugin", func(t *testing.T) {
		f := newFixture(t)

		const second = "auth0/second"

		index := f.indexFrom(registrytest.NewIndexBuilder().
			AddPlugin(registrytest.NewPlugin(testID, f.release("1.0.0"), f.release("2.0.0"))).
			AddPlugin(registrytest.NewPlugin(second, f.release("3.0.0", func(r *release) { r.id = second }))))

		f.install(index, "1.0.0")
		f.install(index, "2.0.0")

		_, err := f.store.Install(t.Context(), index, second, "", store.Options{})
		require.NoError(t, err)

		manifests, err := f.store.Manifests()

		require.NoError(t, err)
		require.Len(t, manifests, 2)
		assert.Equal(t, "2.0.0", manifests[testID].Version)
		assert.Equal(t, second, manifests[second].ID)
		assert.Equal(t, "3.0.0", manifests[second].Version)
	})

	t.Run("leaves out a plugin whose manifest cannot be trusted, without an error", func(t *testing.T) {
		f := newFixture(t)

		const second, third = "auth0/second", "auth0/third"

		index := f.indexFrom(registrytest.NewIndexBuilder().
			AddPlugin(registrytest.NewPlugin(testID, f.release("1.0.0"))).
			AddPlugin(registrytest.NewPlugin(second, f.release("2.0.0", func(r *release) { r.id = second }))).
			AddPlugin(registrytest.NewPlugin(third, f.release("3.0.0", func(r *release) { r.id = third }))))

		for _, id := range []string{testID, second, third} {
			_, err := f.store.Install(t.Context(), index, id, "", store.Options{})
			require.NoError(t, err)
		}

		// One manifest is edited without the lockfile knowing, and one is gone.
		f.editManifest("1.0.0", func(manifest map[string]any) { manifest["description"] = "Another description." })
		require.NoError(t, os.Remove(filepath.Join(f.dir, "store", "auth0", "second", "2.0.0", "plugin.json")))

		manifests, err := f.store.Manifests()

		require.NoError(t, err)
		assert.Len(t, manifests, 1)
		assert.Contains(t, manifests, third)
	})

	t.Run("is empty without a lockfile, and fails for a damaged one", func(t *testing.T) {
		f := newFixture(t)

		manifests, err := f.store.Manifests()

		require.NoError(t, err)
		assert.Empty(t, manifests)
		assert.NoDirExists(t, f.dir)

		require.NoError(t, os.MkdirAll(f.dir, 0o700))
		require.NoError(t, os.WriteFile(f.lockfilePath(), []byte("not json"), 0o600))

		_, err = f.store.Manifests()

		assert.ErrorIs(t, err, store.ErrLockfileDamaged)
	})
}
