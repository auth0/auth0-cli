package store_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
	"github.com/auth0/auth0-cli/internal/plugins/registry/registrytest"
	"github.com/auth0/auth0-cli/internal/plugins/store"
)

func TestInstall_RefusesBeforeDownloading(t *testing.T) {
	t.Run("a plugin or version the registry does not list", func(t *testing.T) {
		f := newFixture(t)
		index := f.index(f.release("1.0.0"), f.release("1.9.0"), f.release("2.0.0"))

		_, err := f.store.Install(t.Context(), index, testID, "1.8.5", store.Options{})

		var notAvailable *store.NotAvailableError
		require.ErrorAs(t, err, &notAvailable)
		assert.Equal(t, []string{"2.0.0", "1.9.0", "1.0.0"}, notAvailable.Available, "latest first")
		assert.Equal(t, "checkmate 1.8.5 is not available in the auth0 registry.\nAvailable versions: 2.0.0 (latest), 1.9.0, 1.0.0", err.Error())

		_, err = f.store.Install(t.Context(), index, "auth0/other", "", store.Options{})
		assert.ErrorAs(t, err, &notAvailable)
		assert.Equal(t, "other is not listed in the auth0 registry.", err.Error())

		assert.Zero(t, f.server.Requests(artifactPath("2.0.0")))
	})

	t.Run("an ID that is not safe to use as a path", func(t *testing.T) {
		f := newFixture(t)
		index := f.index(f.release("1.0.0"))

		for _, id := range []string{"../escape", "auth0/../../escape", "auth0/Check", "checkmate", ""} {
			_, err := f.store.Install(t.Context(), index, id, "", store.Options{})
			assert.ErrorContains(t, err, "is not a plugin ID in the form registry/name", id)
		}

		assert.NoDirExists(t, f.dir)
	})

	t.Run("a revoked version", func(t *testing.T) {
		f := newFixture(t)
		index := f.indexFrom(registrytest.NewIndexBuilder().
			AddPlugin(registrytest.NewPlugin(testID, f.release("1.0.0"), f.release("2.0.0"))).
			Revoke(testID, "2.0.0", "Leaks the token."))

		_, err := f.store.Install(t.Context(), index, testID, "", store.Options{})

		var revoked *store.RevokedError
		require.ErrorAs(t, err, &revoked)
		assert.Equal(t, "Leaks the token.", revoked.Reason)
		assert.Equal(t, "checkmate 2.0.0 was revoked: Leaks the token. Install a newer version with: auth0 plugin update checkmate", err.Error())
		assert.Zero(t, f.requests("2.0.0"))
	})

	t.Run("versions that were revoked are not offered", func(t *testing.T) {
		f := newFixture(t)

		elsewhere := plugins.PlatformDarwinArm64
		if f.platform == elsewhere {
			elsewhere = plugins.PlatformWindowsX64
		}

		index := f.indexFrom(registrytest.NewIndexBuilder().
			AddPlugin(registrytest.NewPlugin(testID, f.release("1.0.0"), f.release("2.0.0"), f.release("3.0.0", func(r *release) {
				r.platform = elsewhere
				r.record.Platforms = []plugins.Platform{elsewhere}
			}))).
			Revoke(testID, "2.0.0", "Leaks the token."))

		_, err := f.store.Install(t.Context(), index, testID, "1.8.5", store.Options{})

		var notAvailable *store.NotAvailableError
		require.ErrorAs(t, err, &notAvailable)
		assert.Equal(t, []string{"3.0.0", "1.0.0"}, notAvailable.Available)

		_, err = f.store.Install(t.Context(), index, testID, "3.0.0", store.Options{})

		var noBuild *store.NoBuildError
		require.ErrorAs(t, err, &noBuild)
		assert.Equal(t, []string{"1.0.0"}, noBuild.Versions)
	})

	t.Run("a version with no build for this machine", func(t *testing.T) {
		f := newFixture(t)

		other := plugins.PlatformDarwinArm64
		if f.platform == other {
			other = plugins.PlatformWindowsX64
		}

		elsewhere := func(r *release) {
			r.platform = other
			r.record.Platforms = []plugins.Platform{other}
		}

		index := f.index(f.release("1.0.0"), f.release("2.0.0", elsewhere), f.release("3.0.0", elsewhere))

		_, err := f.store.Install(t.Context(), index, testID, "2.0.0", store.Options{})

		var noBuild *store.NoBuildError
		require.ErrorAs(t, err, &noBuild)
		assert.Equal(t, []plugins.Platform{other}, noBuild.Platforms)
		assert.Equal(t, []string{"1.0.0"}, noBuild.Versions)
		assert.Contains(t, err.Error(), "checkmate 2.0.0 has no build for ")
		assert.Contains(t, err.Error(), "Versions with a ")
		assert.Zero(t, f.requests("2.0.0"))
	})

	t.Run("an artifact that is not an archive", func(t *testing.T) {
		f := newFixture(t)
		raw := func(r *release) { r.format = registry.ArtifactFormatRaw }
		index := f.index(f.release("1.0.0", raw))

		_, err := f.store.Install(t.Context(), index, testID, "", store.Options{})

		var unsupported *store.UnsupportedFormatError
		require.ErrorAs(t, err, &unsupported)
		assert.Equal(t, "raw", unsupported.Format)
		assert.Contains(t, err.Error(), "unsupported artifact format")
		assert.Zero(t, f.requests("1.0.0"))
	})

	t.Run("a download bigger than the policy allows", func(t *testing.T) {
		f := newFixture(t)
		f.policy = registry.Policy{MaxDownloadBytes: 10}
		index := f.index(f.release("1.0.0"))

		_, err := f.store.Install(t.Context(), index, testID, "", store.Options{})

		assert.ErrorIs(t, err, store.ErrDownloadTooLarge)
		assert.Zero(t, f.requests("1.0.0"))
	})
}

func TestInstall_Engines(t *testing.T) {
	tests := []struct {
		name       string
		constraint string
		cli        string
		wantErr    bool
	}{
		{"a stable CLI that is new enough", ">=1.39.0", "1.39.0", false},
		{"a beta of the required version", ">=1.39.0", "1.39.0-beta.2", false},
		{"a dev build skips the check", ">=9.0.0", "dev", false},
		{"an empty version skips the check", ">=9.0.0", "", false},
		{"a CLI that is too old", ">=1.40.0", "1.39.0", true},
		{"a CLI that is too new", "<1.0.0", "1.39.0", true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			f.store.CLIVersion = test.cli
			index := f.index(f.release("1.0.0", func(r *release) { r.record.Engines = &plugins.Engines{Auth0CLI: test.constraint} }))

			_, err := f.store.Install(t.Context(), index, testID, "", store.Options{})

			if !test.wantErr {
				require.NoError(t, err)

				return
			}

			var engines *store.EnginesError
			require.ErrorAs(t, err, &engines)
			assert.Equal(t, test.constraint, engines.Requires)
			assert.Equal(t, test.cli, engines.CLIVersion)
			assert.Equal(t, "checkmate 1.0.0 requires auth0-cli "+test.constraint+". You have "+test.cli+".", err.Error())
			assert.Zero(t, f.requests("1.0.0"))
		})
	}
}
