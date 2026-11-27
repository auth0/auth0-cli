package store_test

import (
	"runtime"
	"strings"
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

	t.Run("a version in a separate file that is not written as MAJOR.MINOR.PATCH", func(t *testing.T) {
		f := newFixture(t)

		// The index itself only lists versions in this form, so the version comes
		// from the separate file.
		prerelease := f.release("1.0.0-beta.1")
		f.store.Versions = refSource{records: []registry.VersionRecord{prerelease}}
		index := f.indexFrom(registrytest.NewIndexBuilder().AddRefPlugin(registrytest.NewPlugin(testID, f.release("1.0.0"))))

		_, err := f.store.Install(t.Context(), index, testID, "1.0.0-beta.1", store.Options{})

		assert.ErrorContains(t, err, `with the version "1.0.0-beta.1", which is not in the form MAJOR.MINOR.PATCH`)
		assert.Zero(t, f.requests("1.0.0-beta.1"))
		assert.NoDirExists(t, f.dir)
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

func TestNoBuildError_NamesThePlatformOfThisMachine(t *testing.T) {
	tests := []struct {
		name     string
		platform plugins.Platform
	}{
		{"a musl system", plugins.PlatformLinuxX64Musl},
		{"a machine that is not a plugin platform", ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)

			// The only build is for a platform that is not this machine.
			index := f.index(f.release("1.0.0", func(r *release) {
				r.platform = plugins.PlatformWindowsX64
				r.record.Platforms = []plugins.Platform{r.platform}
			}))

			defer store.SetCurrentPlatform(test.platform)()

			_, err := f.store.Install(t.Context(), index, testID, "", store.Options{})

			var noBuild *store.NoBuildError
			require.ErrorAs(t, err, &noBuild)
			assert.Contains(t, err.Error(), "has no build for "+noBuild.Machine+".")

			if test.platform == "" {
				assert.True(t, strings.HasPrefix(noBuild.Machine, runtime.GOOS+"-"), noBuild.Machine)

				return
			}

			assert.Equal(t, string(test.platform), noBuild.Machine)
		})
	}
}

func TestPluginID_MustBeSafeToUseAsAPath(t *testing.T) {
	operations := map[string]func(f *fixture, id string) error{
		"Install": func(f *fixture, id string) error {
			_, err := f.store.Install(f.t.Context(), f.index(f.release("1.0.0")), id, "", store.Options{})

			return err
		},
		"Update": func(f *fixture, id string) error {
			_, err := f.store.Update(f.t.Context(), f.index(f.release("1.0.0")), id, store.Options{})

			return err
		},
		"Verify": func(f *fixture, id string) error {
			_, err := f.store.Verify(id)

			return err
		},
		"Remove": func(f *fixture, id string) error {
			_, err := f.store.Remove(f.t.Context(), id)

			return err
		},
	}

	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			for _, id := range []string{"../escape", "auth0/../../escape", "auth0/Check", "checkmate", "", "auth0/con", "nul/checkmate"} {
				f := newFixture(t)

				assert.ErrorContains(t, operation(f, id), "is not a plugin ID in the form registry/name", id)
				assert.NoDirExists(t, f.dir)
			}
		})
	}
}

func TestInstall_Engines(t *testing.T) {
	tests := []struct {
		name       string
		constraint string
		cli        string
		wantErr    bool
	}{
		{"a stable CLI that is new enough", ">=1.39.0", "1.39.0", false},
		{"a dev build skips the check", ">=9.0.0", "dev", false},
		{"a CLI that is too old", ">=1.40.0", "1.39.0", true},
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
