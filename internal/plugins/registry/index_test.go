package registry_test

import (
	"bytes"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
	"github.com/auth0/auth0-cli/internal/plugins/registry/registrytest"
)

// schemaEnum returns the enum found at path in the embedded schema.
func schemaEnum(t *testing.T, path ...any) []string {
	t.Helper()

	var schema map[string]any
	require.NoError(t, json.Unmarshal(registry.IndexSchema(), &schema))

	var values []string
	for _, value := range follow(schema, path).([]any) {
		values = append(values, value.(string))
	}

	return values
}

func TestIndexSchema(t *testing.T) {
	t.Run("is valid JSON", func(t *testing.T) {
		assert.True(t, json.Valid(registry.IndexSchema()))
	})

	t.Run("targets draft 2020-12", func(t *testing.T) {
		var schema map[string]any
		require.NoError(t, json.Unmarshal(registry.IndexSchema(), &schema))
		assert.Equal(t, "https://json-schema.org/draft/2020-12/schema", schema["$schema"])
	})

	t.Run("lets the index grow new fields", func(t *testing.T) {
		var schema map[string]any
		require.NoError(t, json.Unmarshal(registry.IndexSchema(), &schema))

		assert.NotContains(t, schema, "additionalProperties")

		for _, name := range []string{"plugin", "versionRecord", "artifact"} {
			assert.NotContains(t, follow(schema, []any{"$defs", name}), "additionalProperties", name)
		}
	})

	t.Run("never lets a permission go unnoticed", func(t *testing.T) {
		var schema map[string]any
		require.NoError(t, json.Unmarshal(registry.IndexSchema(), &schema))

		assert.Equal(t, false, follow(schema, []any{"$defs", "permissions", "additionalProperties"}))
	})

	t.Run("returns a copy that cannot change the embedded schema", func(t *testing.T) {
		first := registry.IndexSchema()
		original := string(first)
		first[0] = 'X'

		assert.Equal(t, original, string(registry.IndexSchema()))

		_, err := registry.ParseIndex(registrytest.NewIndexBuilder().Build())
		assert.NoError(t, err)
	})

	t.Run("lists the same platforms as the manifest", func(t *testing.T) {
		var got []string
		for _, platform := range plugins.AllPlatforms() {
			got = append(got, string(platform))
		}

		assert.Equal(t, schemaEnum(t, "$defs", "platform", "enum"), got)
	})

	t.Run("lists the same artifact formats as the constants", func(t *testing.T) {
		got := []string{
			string(registry.ArtifactFormatRaw), string(registry.ArtifactFormatTarGz), string(registry.ArtifactFormatZip),
		}

		assert.Equal(t, schemaEnum(t, "$defs", "artifact", "properties", "format", "enum"), got)
	})

	t.Run("describes permissions like the manifest schema", func(t *testing.T) {
		var index, manifest map[string]any
		require.NoError(t, json.Unmarshal(registry.IndexSchema(), &index))
		require.NoError(t, json.Unmarshal(plugins.ManifestSchema(), &manifest))

		for _, property := range []string{"scopes", "hosts", "files", "env"} {
			path := []any{"$defs", "permissions", "properties", property}
			assert.Equal(t, follow(manifest, path), follow(index, path), property)
		}
	})
}

func TestParseIndex(t *testing.T) {
	data := registrytest.NewIndexBuilder().
		SetVersion(12).
		SetExpires(time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)).
		AddPlugin(registrytest.NewPlugin(testPluginID, registrytest.NewVersion("1.0.0"))).
		Build()

	index, err := registry.ParseIndex(data)

	require.NoError(t, err)
	assert.Equal(t, 1, index.FormatVersion)
	assert.Equal(t, 12, index.Version)
	assert.Equal(t, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), index.Expires)
	assert.Len(t, index.Plugins, 1)
}

func TestIndex_Lookups(t *testing.T) {
	data := registrytest.NewIndexBuilder().
		AddPlugin(registrytest.NewPlugin("auth0/one", registrytest.NewVersion("1.0.0"), registrytest.NewVersion("1.1.0"))).
		AddRefPlugin(registrytest.NewPlugin("auth0/two", registrytest.NewVersion("3.0.0"))).
		Revoke("auth0/one", "1.0.0", "Leaks tokens").
		Deprecate(registry.Deprecation{ID: "auth0/one", Message: "Use the new plugin", Replacement: "auth0/two"}).
		Deprecate(registry.Deprecation{ID: "auth0/one", Version: "1.1.0", Message: "Has a known issue"}).
		Deprecate(registry.Deprecation{ID: "auth0/two", Version: "3.0.0", Message: "Too old"}).
		Build()

	index, err := registry.ParseIndex(data)
	require.NoError(t, err)

	t.Run("FindPlugin", func(t *testing.T) {
		plugin, found := index.FindPlugin("auth0/one")
		require.True(t, found)
		assert.Equal(t, "auth0/one", plugin.ID)

		_, found = index.FindPlugin("auth0/missing")
		assert.False(t, found)
	})

	t.Run("Plugin.Version", func(t *testing.T) {
		plugin, _ := index.FindPlugin("auth0/one")

		record, found := plugin.Version("1.1.0")
		require.True(t, found)
		assert.Equal(t, "1.1.0", record.Version)

		_, found = plugin.Version("2.0.0")
		assert.False(t, found)

		ref, _ := index.FindPlugin("auth0/two")
		_, found = ref.Version("3.0.0")
		assert.False(t, found, "the versions of a ref plugin come from Client.Versions")
	})

	t.Run("Revoked", func(t *testing.T) {
		tests := []struct {
			id, version string
			want        bool
		}{
			{id: "auth0/one", version: "1.0.0", want: true},
			{id: "auth0/one", version: "1.1.0"},
			{id: "auth0/two", version: "1.0.0"},
		}

		for _, test := range tests {
			revocation, found := index.Revoked(test.id, test.version)
			assert.Equal(t, test.want, found, "%s %s", test.id, test.version)

			if test.want {
				assert.Equal(t, "Leaks tokens", revocation.Reason)
			}
		}
	})

	t.Run("Deprecation", func(t *testing.T) {
		tests := []struct {
			name        string
			id, version string
			wantMessage string
		}{
			{name: "the whole plugin", id: "auth0/one", version: "1.0.0", wantMessage: "Use the new plugin"},
			{name: "a version wins over the whole plugin", id: "auth0/one", version: "1.1.0", wantMessage: "Has a known issue"},
			{name: "only one version", id: "auth0/two", version: "3.0.0", wantMessage: "Too old"},
			{name: "another version of that plugin", id: "auth0/two", version: "3.1.0"},
			{name: "an unknown plugin", id: "auth0/missing", version: "1.0.0"},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				deprecation, found := index.Deprecation(test.id, test.version)

				assert.Equal(t, test.wantMessage != "", found)
				assert.Equal(t, test.wantMessage, deprecation.Message)
			})
		}

		deprecation, _ := index.Deprecation("auth0/one", "1.0.0")
		assert.Equal(t, "auth0/two", deprecation.Replacement)
	})
}

func TestPolicy_WithDefaults(t *testing.T) {
	tests := []struct {
		name   string
		policy registry.Policy
		want   registry.Policy
	}{
		{
			name:   "fills in everything for an empty policy",
			policy: registry.Policy{},
			want: registry.Policy{
				OfflineGraceDays: 30, CheckIntervalHours: 24, MaxDownloadBytes: 256 << 20, DefaultTimeout: "10m", VersionRetention: 0,
			},
		},
		{
			name: "keeps the settings that are set",
			policy: registry.Policy{
				OfflineGraceDays: 7, CheckIntervalHours: 6, MaxDownloadBytes: 1024, DefaultTimeout: "30s", VersionRetention: 3,
			},
			want: registry.Policy{
				OfflineGraceDays: 7, CheckIntervalHours: 6, MaxDownloadBytes: 1024, DefaultTimeout: "30s", VersionRetention: 3,
			},
		},
		{
			name:   "fills in only what is missing",
			policy: registry.Policy{CheckIntervalHours: 1, VersionRetention: 2},
			want: registry.Policy{
				OfflineGraceDays: 30, CheckIntervalHours: 1, MaxDownloadBytes: 256 << 20, DefaultTimeout: "10m", VersionRetention: 2,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, test.policy.WithDefaults())
		})
	}
}

func TestArtifact_Platform(t *testing.T) {
	tests := []struct {
		artifact registry.Artifact
		want     plugins.Platform
		wantOK   bool
	}{
		{artifact: registry.Artifact{OS: "darwin", Arch: "arm64"}, want: plugins.PlatformDarwinArm64, wantOK: true},
		{artifact: registry.Artifact{OS: "linux", Arch: "x64", Libc: "glibc"}, want: plugins.PlatformLinuxX64, wantOK: true},
		{artifact: registry.Artifact{OS: "linux", Arch: "arm64", Libc: "musl"}, want: plugins.PlatformLinuxArm64Musl, wantOK: true},
		{artifact: registry.Artifact{OS: "windows", Arch: "x64"}, want: plugins.PlatformWindowsX64, wantOK: true},
		{artifact: registry.Artifact{OS: "linux", Arch: "x64"}},
		{artifact: registry.Artifact{OS: "darwin", Arch: "arm64", Libc: "glibc"}},
		{artifact: registry.Artifact{OS: "windows", Arch: "arm64"}},
	}

	for _, test := range tests {
		got, ok := test.artifact.Platform()

		assert.Equal(t, test.wantOK, ok, "%+v", test.artifact)
		assert.Equal(t, test.want, got, "%+v", test.artifact)
	}
}

func TestRegistrytest_HashesMatchTheManifestAlgorithm(t *testing.T) {
	record := registrytest.NewVersion("1.0.0")
	record.Permissions.Scopes = []string{"read:clients"}
	record.Permissions.Hosts = []plugins.HostPermission{{Host: "api.example.com", Reason: "Fetch data"}}
	record = registrytest.Seal(record)

	manifest := &plugins.Manifest{
		ID:       testPluginID,
		Commands: []plugins.Command{{Name: "run", Permissions: &plugins.Permissions{Scopes: []string{"read:clients"}, Hosts: record.Permissions.Hosts}}},
	}

	assert.Equal(t, manifest.PermissionsHash(), record.PermissionsHash)
}

// combine returns a mutation that applies all of the given mutations.
func combine(mutations ...func(index map[string]any)) func(index map[string]any) {
	return func(index map[string]any) {
		for _, mutate := range mutations {
			mutate(index)
		}
	}
}

// parseMutated parses the index of testBuilder after the mutation was applied to
// its generic form.
func parseMutated(t *testing.T, mutate func(index map[string]any)) (*registry.Index, error) {
	t.Helper()

	return registry.ParseIndex(mutateIndex(t, testBuilder().Build(), mutate))
}

// requireInvalid checks that err says the index is invalid and lists every
// wanted problem.
func requireInvalid(t *testing.T, err error, wantErrs ...string) {
	t.Helper()

	require.ErrorIs(t, err, registry.ErrInvalidIndex)

	var validation *plugins.ValidationError
	require.ErrorAs(t, err, &validation)

	for _, want := range wantErrs {
		assert.Contains(t, err.Error(), want)
	}
}

func TestParseIndex_Invalid(t *testing.T) {
	hash := string(bytes.Repeat([]byte("a"), 64))

	tests := []struct {
		name     string
		mutate   func(index map[string]any)
		wantErrs []string
	}{
		{name: "a plugin without an id", mutate: removeValue("plugins", 0, "id"), wantErrs: []string{"plugins[0].id: required field is missing"}},
		{name: "a plugin id without a registry", mutate: setValue("checkmate", "plugins", 0, "id"), wantErrs: []string{"plugins[0].id:"}},
		{name: "a version that is not semver", mutate: setValue("1.0", versionPath("version")...), wantErrs: []string{"plugins[0].versions[0].version:"}},
		{name: "a prerelease version", mutate: setValue("1.0.0-beta.1", versionPath("version")...), wantErrs: []string{"plugins[0].versions[0].version:"}},
		{name: "a version with build metadata", mutate: setValue("1.0.0+abc", versionPath("version")...), wantErrs: []string{"plugins[0].versions[0].version:"}},
		{name: "a linux artifact without libc", mutate: removeValue(versionPath("artifacts", 1, "libc")...), wantErrs: []string{"plugins[0].versions[0].artifacts[1].libc: required field is missing"}},
		{name: "a macOS artifact with libc", mutate: setValue("glibc", versionPath("artifacts", 0, "libc")...), wantErrs: []string{"plugins[0].versions[0].artifacts[0].libc:"}},
		{name: "an artifact URL that is not https", mutate: setValue("http://downloads.example.com/plugin", versionPath("artifacts", 0, "url")...), wantErrs: []string{"plugins[0].versions[0].artifacts[0].url:"}},
		{name: "a sha256 with a prefix", mutate: setValue("sha256:"+hash, versionPath("artifacts", 0, "sha256")...), wantErrs: []string{"plugins[0].versions[0].artifacts[0].sha256:"}},
		{name: "a sha256 in uppercase", mutate: setValue(strings.ToUpper(hash), versionPath("artifacts", 0, "sha256")...), wantErrs: []string{"plugins[0].versions[0].artifacts[0].sha256:"}},
		{name: "an artifact without size", mutate: setValue(0, versionPath("artifacts", 0, "size")...), wantErrs: []string{"plugins[0].versions[0].artifacts[0].size:"}},
		{name: "an unknown artifact format", mutate: setValue("7z", versionPath("artifacts", 0, "format")...), wantErrs: []string{"plugins[0].versions[0].artifacts[0].format:"}},
		{name: "a bin with a path", mutate: setValue("bin/plugin", versionPath("artifacts", 0, "bin")...), wantErrs: []string{"plugins[0].versions[0].artifacts[0].bin:"}},
		{name: "a description that is too long", mutate: setValue(strings.Repeat("a", 121), "plugins", 0, "description"), wantErrs: []string{"plugins[0].description:"}},
		{name: "a description with a control character", mutate: setValue("Line one\x1b[31m", "plugins", 0, "description"), wantErrs: []string{"plugins[0].description:"}},
		{name: "a description with an 8-bit CSI escape", mutate: setValue("Harmless\u009b31m", "plugins", 0, "description"), wantErrs: []string{"plugins[0].description:"}},
		{
			name:     "a revocation reason with a C1 control",
			mutate:   setValue([]any{map[string]any{"id": testPluginID, "version": "1.0.0", "reason": "Bad\u0085news"}}, "revoked"),
			wantErrs: []string{"revoked[0].reason:"},
		},
		{name: "a homepage that is not https", mutate: setValue("http://example.com", "plugins", 0, "homepage"), wantErrs: []string{"plugins[0].homepage:"}},
		{
			name:     "a homepage written in another case",
			mutate:   setValue("http://example.com", "plugins", 0, "Homepage"),
			wantErrs: []string{`plugins[0].Homepage: field name must be written as "homepage"`},
		},
		{name: "a plugin with neither versions nor ref", mutate: removeValue("plugins", 0, "versions"), wantErrs: []string{"plugins[0]"}},
		{
			name:     "a plugin with both versions and ref",
			mutate:   setValue(map[string]any{"path": "plugins/checkmate.json", "sha256": hash}, "plugins", 0, "ref"),
			wantErrs: []string{"plugins[0]"},
		},
		{
			name:     "a ref path that leaves the registry",
			mutate:   combine(removeValue("plugins", 0, "versions"), setValue(map[string]any{"path": "../x.json", "sha256": hash}, "plugins", 0, "ref")),
			wantErrs: []string{"plugins[0].ref.path:"},
		},
		{
			name:     "a ref path that is a URL",
			mutate:   combine(removeValue("plugins", 0, "versions"), setValue(map[string]any{"path": "https://evil.example.com/x.json", "sha256": hash}, "plugins", 0, "ref")),
			wantErrs: []string{"plugins[0].ref.path:"},
		},
		{name: "an index without a version", mutate: removeValue("version"), wantErrs: []string{"version: required field is missing"}},
		{name: "an index version of zero", mutate: setValue(0, "version"), wantErrs: []string{"version:"}},
		{name: "an expiry that is not UTC", mutate: setValue("2026-12-01T00:00:00+02:00", "expires"), wantErrs: []string{"expires:"}},
		{name: "an expiry that is not a date", mutate: setValue("2026-13-45T00:00:00Z", "expires"), wantErrs: []string{"could not be decoded"}},
		{name: "a permissions hash without a prefix", mutate: setValue(hash, versionPath("permissionsHash")...), wantErrs: []string{"plugins[0].versions[0].permissionsHash:"}},
		{
			name:     "a revocation without a reason",
			mutate:   setValue([]any{map[string]any{"id": testPluginID, "version": "1.0.0"}}, "revoked"),
			wantErrs: []string{"revoked[0].reason: required field is missing"},
		},
		{name: "a default timeout that is not a duration", mutate: setValue(map[string]any{"defaultTimeout": "soon"}, "policy"), wantErrs: []string{"policy.defaultTimeout:"}},
		{name: "a default timeout of zero", mutate: setValue(map[string]any{"defaultTimeout": "0s"}, "policy"), wantErrs: []string{"policy.defaultTimeout:", "greater than zero"}},
		{name: "a check interval of zero", mutate: setValue(map[string]any{"checkIntervalHours": 0}, "policy"), wantErrs: []string{"policy.checkIntervalHours:"}},
		{
			name:     "an unknown permission in a host entry",
			mutate:   setValue([]any{map[string]any{"host": "example.com", "reason": "Why", "port": 443}}, versionPath("permissions", "hosts")...),
			wantErrs: []string{"plugins[0].versions[0].permissions.hosts[0].port: unknown field"},
		},
		{
			name:     "an unknown permission",
			mutate:   setValue([]any{}, append(versionPath("permissions"), "network")...),
			wantErrs: []string{"plugins[0].versions[0].permissions.network: unknown field"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			index, err := parseMutated(t, test.mutate)

			assert.Nil(t, index)
			requireInvalid(t, err, test.wantErrs...)
		})
	}

	t.Run("an index that is not JSON", func(t *testing.T) {
		_, err := registry.ParseIndex([]byte("not json"))

		requireInvalid(t, err, "not valid JSON")
	})
}

func TestParseIndex_Format(t *testing.T) {
	t.Run("rejects a newer format version and says to update the CLI", func(t *testing.T) {
		_, err := parseMutated(t, setValue(2, "formatVersion"))

		require.ErrorIs(t, err, registry.ErrUnsupportedFormat)
		assert.Contains(t, err.Error(), "Update the auth0 CLI")
	})

	t.Run("ignores fields it does not know", func(t *testing.T) {
		index, err := parseMutated(t, func(index map[string]any) {
			index["futureField"] = map[string]any{"a": 1}
			setValue("x", "plugins", 0, "futureField")(index)
			setValue("x", "plugins", 0, "owner", "slack")(index)
			setValue("x", append(versionPath(), "futureField")...)(index)
			setValue("x", append(versionPath("artifacts", 0), "futureField")...)(index)
			setValue("x", append(versionPath("commands", 0), "futureField")...)(index)
			index["policy"] = map[string]any{"futureSetting": true, "checkIntervalHours": 12}
			index["revoked"] = []any{map[string]any{"id": testPluginID, "version": "1.0.0", "reason": "Bad", "futureField": 1}}
		})

		require.NoError(t, err)
		assert.Equal(t, 12, index.Policy.CheckIntervalHours)
	})

	t.Run("keeps the scope table and the secret fields as they are", func(t *testing.T) {
		index, err := parseMutated(t, func(index map[string]any) {
			index["scopeTable"] = map[string]any{"checkmate": []any{"read:clients"}}
			index["secretFields"] = []any{"client_secret"}
		})

		require.NoError(t, err)
		assert.JSONEq(t, `{"checkmate":["read:clients"]}`, string(index.ScopeTable))
		assert.JSONEq(t, `["client_secret"]`, string(index.SecretFields))
	})
}

func TestParseIndex_SemanticFailures(t *testing.T) {
	record := func(version string, change func(record *registry.VersionRecord)) registry.VersionRecord {
		r := registrytest.NewVersion(version)
		change(&r)

		return r
	}
	nothing := func(*registry.VersionRecord) {}

	tests := []struct {
		name    string
		plugins []registry.Plugin
		wantErr string
	}{
		{
			name: "latest is not among the versions",
			plugins: []registry.Plugin{func() registry.Plugin {
				p := registrytest.NewPlugin(testPluginID, registrytest.NewVersion("1.0.0"))
				p.Latest = "9.9.9"

				return p
			}()},
			wantErr: `plugins[0].latest: version "9.9.9" is not listed in versions`,
		},
		{
			name:    "a version is listed twice",
			plugins: []registry.Plugin{registrytest.NewPlugin(testPluginID, record("1.0.0", nothing), record("1.0.0", nothing))},
			wantErr: `plugins[0].versions[1].version: duplicate version "1.0.0", already used by plugins[0].versions[0]`,
		},
		{
			name: "an artifact is for a platform that is not listed",
			plugins: []registry.Plugin{registrytest.NewPlugin(testPluginID, record("1.0.0", func(r *registry.VersionRecord) {
				r.Platforms = []plugins.Platform{plugins.PlatformDarwinArm64}
			}))},
			wantErr: `plugins[0].versions[0].artifacts[1]: platform "linux-x64" is not listed in platforms`,
		},
		{
			name: "an artifact is for a platform that does not exist",
			plugins: []registry.Plugin{registrytest.NewPlugin(testPluginID, record("1.0.0", func(r *registry.VersionRecord) {
				r.Artifacts[0].OS = "windows"
				r.Artifacts[0].Arch = "arm64"
			}))},
			wantErr: "plugins[0].versions[0].artifacts[0]: os \"windows\", arch \"arm64\" and libc \"\" are not a supported platform",
		},
		{
			name: "two artifacts are for the same platform",
			plugins: []registry.Plugin{registrytest.NewPlugin(testPluginID, record("1.0.0", func(r *registry.VersionRecord) {
				r.Artifacts = append(r.Artifacts, r.Artifacts[0])
			}))},
			wantErr: `plugins[0].versions[0].artifacts[2]: duplicate artifact for platform "darwin-arm64"`,
		},
		{
			name: "the permissions hash does not match the permissions",
			plugins: []registry.Plugin{registrytest.NewPlugin(testPluginID, record("1.0.0", func(r *registry.VersionRecord) {
				r.Permissions.Scopes = []string{"read:clients"}
			}))},
			wantErr: "plugins[0].versions[0].permissionsHash:",
		},
		{
			name: "the permissions hash does not match the commands",
			plugins: []registry.Plugin{registrytest.NewPlugin(testPluginID, record("1.0.0", func(r *registry.VersionRecord) {
				r.Commands[0].Interactive = true
			}))},
			wantErr: "plugins[0].versions[0].permissionsHash:",
		},
		{
			name: "two commands have the same name",
			plugins: []registry.Plugin{registrytest.NewPlugin(testPluginID, record("1.0.0", func(r *registry.VersionRecord) {
				r.Commands = append(r.Commands, r.Commands[0])
				*r = registrytest.Seal(*r)
			}))},
			wantErr: `plugins[0].versions[0].commands[1].name: duplicate command name "run"`,
		},
		{
			name: "a plugin id is used twice",
			plugins: []registry.Plugin{
				registrytest.NewPlugin(testPluginID, registrytest.NewVersion("1.0.0")),
				registrytest.NewPlugin(testPluginID, registrytest.NewVersion("1.0.0")),
			},
			wantErr: `plugins[1].id: duplicate plugin id "auth0/checkmate", already used by plugins[0]`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			builder := registrytest.NewIndexBuilder()
			for _, plugin := range test.plugins {
				builder.AddPlugin(plugin)
			}

			_, err := registry.ParseIndex(builder.Build())

			requireInvalid(t, err, test.wantErr)
		})
	}
}

func TestParseIndex_DuplicateKeys(t *testing.T) {
	// The helper "after" adds a key right after the first time the marker
	// appears, which makes the object that the marker opens have that key twice.
	after := func(marker, key string) func(index string) string {
		return func(index string) string {
			require.Contains(t, index, marker)

			return strings.Replace(index, marker, marker+key, 1)
		}
	}
	// The helper "atEnd" adds members to the end of the root object.
	atEnd := func(members string) func(index string) string {
		return func(index string) string {
			return strings.TrimSuffix(index, "}") + "," + members + "}"
		}
	}

	tests := []struct {
		name    string
		corrupt func(index string) string
		wantErr string
	}{
		{name: "at the root", corrupt: after(`{`, `"version":1,`), wantErr: `$: duplicate key "version"`},
		{name: "a second plugins list", corrupt: atEnd(`"plugins":[]`), wantErr: `$: duplicate key "plugins"`},
		{name: "a second policy", corrupt: atEnd(`"policy":{"checkIntervalHours":1},"policy":{}`), wantErr: `$: duplicate key "policy"`},
		{name: "in a plugin", corrupt: after(`"plugins":[{`, `"id":"auth0/other",`), wantErr: `$.plugins[0]: duplicate key "id"`},
		{name: "in an owner", corrupt: after(`"owner":{`, `"team":"Other",`), wantErr: `$.plugins[0].owner: duplicate key "team"`},
		{name: "in a version record", corrupt: after(`"versions":[{`, `"version":"1.0.0",`), wantErr: `$.plugins[0].versions[0]: duplicate key "version"`},
		{name: "in an artifact", corrupt: after(`"artifacts":[{`, `"os":"darwin",`), wantErr: `$.plugins[0].versions[0].artifacts[0]: duplicate key "os"`},
		{name: "in permissions", corrupt: after(`"permissions":{`, `"scopes":["read:users"],`), wantErr: `$.plugins[0].versions[0].permissions: duplicate key "scopes"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := registry.ParseIndex([]byte(test.corrupt(string(testBuilder().Build()))))

			requireInvalid(t, err, test.wantErr)
		})
	}
}

func TestParseIndex_NormalizedPermissions(t *testing.T) {
	host := func(host string) plugins.HostPermission { return plugins.HostPermission{Host: host, Reason: "Why"} }
	file := func(path string) plugins.FilePermission {
		return plugins.FilePermission{Path: path, Access: plugins.FileAccessRead, Reason: "Why"}
	}
	env := func(name string) plugins.EnvPermission { return plugins.EnvPermission{Name: name, Reason: "Why"} }

	tests := []struct {
		name    string
		change  func(p *plugins.Permissions)
		wantErr string
	}{
		{name: "unsorted scopes", change: func(p *plugins.Permissions) { p.Scopes = []string{"read:users", "read:clients"} }, wantErr: ".permissions.scopes:"},
		{name: "duplicate scopes, which the schema already rejects", change: func(p *plugins.Permissions) { p.Scopes = []string{"read:clients", "read:clients"} }, wantErr: ".permissions.scopes: items at 0 and 1 are equal"},
		{name: "unsorted hosts", change: func(p *plugins.Permissions) {
			p.Hosts = []plugins.HostPermission{host("b.example.com"), host("a.example.com")}
		}, wantErr: ".permissions.hosts:"},
		{name: "duplicate hosts", change: func(p *plugins.Permissions) {
			p.Hosts = []plugins.HostPermission{host("a.example.com"), host("a.example.com")}
		}, wantErr: ".permissions.hosts:"},
		{name: "unsorted files", change: func(p *plugins.Permissions) { p.Files = []plugins.FilePermission{file("b"), file("a")} }, wantErr: ".permissions.files:"},
		{name: "duplicate files", change: func(p *plugins.Permissions) { p.Files = []plugins.FilePermission{file("a"), file("a")} }, wantErr: ".permissions.files:"},
		{name: "unsorted env", change: func(p *plugins.Permissions) { p.Env = []plugins.EnvPermission{env("B"), env("A")} }, wantErr: ".permissions.env:"},
		{name: "duplicate env", change: func(p *plugins.Permissions) { p.Env = []plugins.EnvPermission{env("A"), env("A")} }, wantErr: ".permissions.env:"},
		{
			name: "sorted and distinct permissions",
			change: func(p *plugins.Permissions) {
				p.Scopes = []string{"read:clients", "read:users"}
				p.Hosts = []plugins.HostPermission{host("a.example.com"), host("b.example.com")}
				p.Files = []plugins.FilePermission{file("a"), file("b")}
				p.Env = []plugins.EnvPermission{env("A"), env("B")}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// The hash is calculated from the permissions exactly as they are
			// written, so only the normalization check can reject them.
			record := registrytest.NewVersion("1.0.0")
			test.change(&record.Permissions)
			record = registrytest.Seal(record)

			_, err := registry.ParseIndex(registrytest.NewIndexBuilder().AddPlugin(registrytest.NewPlugin(testPluginID, record)).Build())

			if test.wantErr == "" {
				require.NoError(t, err)
				return
			}

			requireInvalid(t, err, "plugins[0].versions[0]"+test.wantErr)

			if !strings.Contains(test.wantErr, "equal") {
				assert.Contains(t, err.Error(), "must be sorted and free of duplicates")
			}
		})
	}
}

func TestParseIndex_PolicyLimits(t *testing.T) {
	tests := []struct {
		setting string
		max     int64
	}{
		{setting: "offlineGraceDays", max: 3650},
		{setting: "checkIntervalHours", max: 8760},
		{setting: "maxDownloadBytes", max: 4294967296},
		{setting: "versionRetention", max: 1000},
	}

	for _, test := range tests {
		t.Run(test.setting+" accepts its maximum", func(t *testing.T) {
			_, err := parseMutated(t, setValue(map[string]any{test.setting: json.Number(strconv.FormatInt(test.max, 10))}, "policy"))
			assert.NoError(t, err)
		})

		t.Run(test.setting+" rejects more than its maximum", func(t *testing.T) {
			_, err := parseMutated(t, setValue(map[string]any{test.setting: json.Number(strconv.FormatInt(test.max+1, 10))}, "policy"))

			requireInvalid(t, err, "policy."+test.setting+":")
		})
	}
}

// contractVector is one document of the index contract vectors, which hold either
// a document or its raw text.
type contractVector struct {
	Name     string          `json:"name"`
	Reason   string          `json:"reason"`
	Names    string          `json:"names"`
	Document json.RawMessage `json:"document"`
	Raw      string          `json:"raw"`
}

// data returns the bytes of the document.
func (v contractVector) data() []byte {
	if v.Raw != "" {
		return []byte(v.Raw)
	}

	return v.Document
}

func TestParseIndex_ContractVectors(t *testing.T) {
	content, err := os.ReadFile("testdata/index-contract-vectors.json")
	require.NoError(t, err)

	var vectors struct {
		Valid   []contractVector `json:"valid"`
		Invalid []contractVector `json:"invalid"`
	}
	require.NoError(t, json.Unmarshal(content, &vectors))
	require.NotEmpty(t, vectors.Valid)
	require.NotEmpty(t, vectors.Invalid)

	for _, vector := range vectors.Valid {
		t.Run("accepts "+vector.Name, func(t *testing.T) {
			_, err := registry.ParseIndex(vector.data())
			assert.NoError(t, err)
		})
	}

	// What the error of each reason says.
	messages := map[string][]string{
		"duplicate-key":             {"duplicate key"},
		"case-variant-key":          {"field name must be written as"},
		"unsorted-permissions":      {"must be sorted and free of duplicates"},
		"duplicate-permissions":     {"must be sorted and free of duplicates"},
		"permissions-hash-mismatch": {"does not match"},
		"unsafe-text":               {"does not match pattern", "maxLength"},
	}

	for _, vector := range vectors.Invalid {
		t.Run("rejects "+vector.Name, func(t *testing.T) {
			wanted, known := messages[vector.Reason]
			require.True(t, known, "unknown reason %q", vector.Reason)

			index, err := registry.ParseIndex(vector.data())

			assert.Nil(t, index)
			require.ErrorIs(t, err, registry.ErrInvalidIndex)
			assert.Contains(t, err.Error(), vector.Names)

			for _, want := range wanted {
				assert.Contains(t, err.Error(), want)
			}
		})
	}
}
