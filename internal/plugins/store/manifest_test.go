package store_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
	"github.com/auth0/auth0-cli/internal/plugins/registry/registrytest"
	"github.com/auth0/auth0-cli/internal/plugins/store"
)

func TestInstall_ChecksTheManifestAgainstTheRecord(t *testing.T) {
	changeManifest := func(change func(manifest map[string]any)) func(*release) {
		return func(r *release) {
			r.archive = func(r *release) []byte {
				manifest := decodeObject(t, registrytest.Manifest(testID, r.record))
				change(manifest)

				return registrytest.TarGz(
					registrytest.ArchiveEntry{Name: r.bin, Body: r.binary},
					registrytest.ArchiveEntry{Name: "plugin.json", Body: encodeObject(t, manifest)},
				)
			}
		}
	}

	command := func(manifest map[string]any) map[string]any {
		return manifest["commands"].([]any)[0].(map[string]any)
	}

	tests := []struct {
		name    string
		release func(*release)
		want    string
	}{
		{"another id", changeManifest(func(m map[string]any) { m["id"] = "auth0/other" }), `has the id "auth0/other"`},
		{"another version", changeManifest(func(m map[string]any) { m["version"] = "1.0.1" }), `has the version "1.0.1"`},
		{"another platform list", changeManifest(func(m map[string]any) { m["platforms"] = []string{"windows-x64", "darwin-arm64"} }), "lists the platforms"},
		{"engines the record does not have", changeManifest(func(m map[string]any) { m["engines"] = map[string]any{"auth0-cli": ">=1.0.0"} }), "other engines"},
		{"another command", changeManifest(func(m map[string]any) { command(m)["name"] = "other" }), "other commands"},
		{"an interactive command", changeManifest(func(m map[string]any) { command(m)["interactive"] = true }), "interactive values"},
		{"another scope", changeManifest(func(m map[string]any) {
			command(m)["permissions"] = map[string]any{"scopes": []string{"read:clients"}, "hosts": []any{}, "files": []any{}, "env": []any{}}
		}), "other permissions"},
		{"another host reason", changeManifest(func(m map[string]any) {
			command(m)["permissions"] = map[string]any{
				"scopes": []any{}, "env": []any{}, "files": []any{},
				"hosts": []map[string]string{{"host": "api.example.com", "reason": "Because"}},
			}
		}), "other permissions"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			index := f.index(f.release("1.0.0", test.release))

			_, err := f.store.Install(t.Context(), index, testID, "", store.Options{})

			require.ErrorIs(t, err, store.ErrArtifactMismatch)
			assert.Contains(t, err.Error(), test.want)
			assert.NoFileExists(t, f.lockfilePath())
			assert.Empty(t, f.installedVersions())
			assert.Empty(t, f.leftovers())
		})
	}

	t.Run("an engines constraint that differs", func(t *testing.T) {
		f := newFixture(t)
		index := f.index(f.release("1.0.0", func(r *release) {
			r.record.Engines = &plugins.Engines{Auth0CLI: ">=1.0.0"}
			r.manifestRecord = func(record registry.VersionRecord) registry.VersionRecord {
				record.Engines = &plugins.Engines{Auth0CLI: ">=1.1.0"}

				return record
			}
		}))

		_, err := f.store.Install(t.Context(), index, testID, "", store.Options{})

		assert.ErrorIs(t, err, store.ErrArtifactMismatch)
	})

	t.Run("a plugin API version this CLI does not support", func(t *testing.T) {
		f := newFixture(t)
		index := f.index(f.release("1.0.0", changeManifest(func(m map[string]any) { m["apiVersion"] = 2 })))

		_, err := f.store.Install(t.Context(), index, testID, "", store.Options{})

		var unsupported *store.UnsupportedAPIVersionError
		require.ErrorAs(t, err, &unsupported)
		assert.Equal(t, 2, unsupported.APIVersion)
		assert.Empty(t, f.installedVersions())
	})

	t.Run("a manifest that is not valid", func(t *testing.T) {
		f := newFixture(t)
		index := f.index(f.release("1.0.0", changeManifest(func(m map[string]any) { delete(m, "description") })))

		_, err := f.store.Install(t.Context(), index, testID, "", store.Options{})

		assert.ErrorIs(t, err, store.ErrInvalidArchive)
		assert.Empty(t, f.installedVersions())
	})

	t.Run("a manifest that lists the platforms in another order is the same", func(t *testing.T) {
		f := newFixture(t)

		others := slices.DeleteFunc(plugins.AllPlatforms(), func(p plugins.Platform) bool { return p == f.platform })
		forwards := append([]plugins.Platform{f.platform}, others[:2]...)
		backwards := []plugins.Platform{others[1], others[0], f.platform}

		index := f.index(f.release("1.0.0", func(r *release) {
			r.record.Platforms = forwards
			r.manifestRecord = func(record registry.VersionRecord) registry.VersionRecord {
				record.Platforms = backwards

				return record
			}
		}))

		f.install(index, "")
	})
}
