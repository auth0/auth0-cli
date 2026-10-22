package registrytest

import (
	"encoding/json"
	"path"
	"time"

	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
)

// NewVersion returns a valid record for the given version, with one command, no
// permissions, and an artifact for macOS on Apple silicon and for Linux on x64.
func NewVersion(version string) registry.VersionRecord {
	sum := SHA256Hex([]byte(version))

	return Seal(registry.VersionRecord{
		Version:   version,
		Platforms: []plugins.Platform{plugins.PlatformDarwinArm64, plugins.PlatformLinuxX64},
		Permissions: plugins.Permissions{
			Scopes: []string{},
			Hosts:  []plugins.HostPermission{},
			Files:  []plugins.FilePermission{},
			Env:    []plugins.EnvPermission{},
		},
		Commands: []plugins.ConsentCommand{{Name: "run"}},
		Artifacts: []registry.Artifact{
			{
				OS: "darwin", Arch: "arm64",
				URL:    "https://downloads.example.com/" + version + "/plugin-darwin-arm64",
				SHA256: sum, Size: 1024, Format: registry.ArtifactFormatRaw, Bin: "plugin",
			},
			{
				OS: "linux", Arch: "x64", Libc: "glibc",
				URL:    "https://downloads.example.com/" + version + "/plugin-linux-x64.tar.gz",
				SHA256: sum, Size: 2048, Format: registry.ArtifactFormatTarGz, Bin: "plugin",
			},
		},
	})
}

// Seal returns the record with its permissionsHash worked out from its
// permissions and commands, using the same algorithm as the CLI. Call it after
// changing either of them.
func Seal(record registry.VersionRecord) registry.VersionRecord {
	record.PermissionsHash = plugins.ConsentHash(record.Permissions, record.Commands)

	return record
}

// NewPlugin returns a valid plugin with the given versions, whose latest version
// is the last one.
func NewPlugin(id string, versions ...registry.VersionRecord) registry.Plugin {
	plugin := registry.Plugin{
		ID:          id,
		Description: "Test plugin " + id,
		Owner:       plugins.Owner{Team: "Test team", Bugs: "https://example.com/bugs"},
		Versions:    versions,
	}

	if len(versions) > 0 {
		plugin.Latest = versions[len(versions)-1].Version
	}

	return plugin
}

// IndexBuilder builds the index.json of a test registry.
type IndexBuilder struct {
	index registry.Index
	files map[string][]byte
}

// defaultExpires is when an index expires unless a test sets another time. It
// is a fixed time far in the future, so that building the same index twice gives
// the same bytes and no test depends on the clock.
var defaultExpires = time.Date(2099, time.January, 1, 0, 0, 0, 0, time.UTC)

// NewIndexBuilder returns a builder for an empty index of version 1 that
// expires on 1 January 2099.
func NewIndexBuilder() *IndexBuilder {
	return &IndexBuilder{
		index: registry.Index{
			FormatVersion: 1,
			Version:       1,
			Expires:       defaultExpires,
		},
		files: make(map[string][]byte),
	}
}

// SetVersion sets the version of the index.
func (b *IndexBuilder) SetVersion(version int) *IndexBuilder {
	b.index.Version = version

	return b
}

// SetExpires sets the time the index expires at.
func (b *IndexBuilder) SetExpires(expires time.Time) *IndexBuilder {
	b.index.Expires = expires.UTC()

	return b
}

// SetPolicy sets the policy of the index.
func (b *IndexBuilder) SetPolicy(policy registry.Policy) *IndexBuilder {
	b.index.Policy = policy

	return b
}

// SetScopeTable sets the scope table of the index.
func (b *IndexBuilder) SetScopeTable(table registry.ScopeTable) *IndexBuilder {
	b.index.ScopeTable = table

	return b
}

// SetSecretFields sets the secret fields of the index.
func (b *IndexBuilder) SetSecretFields(fields registry.SecretFields) *IndexBuilder {
	b.index.SecretFields = fields

	return b
}

// AddPlugin adds a plugin whose versions are listed in the index.
func (b *IndexBuilder) AddPlugin(plugin registry.Plugin) *IndexBuilder {
	b.index.Plugins = append(b.index.Plugins, plugin)

	return b
}

// AddRefPlugin adds a plugin whose versions are listed in a separate file that
// is pinned by its hash. The file is available from Files.
func (b *IndexBuilder) AddRefPlugin(plugin registry.Plugin) *IndexBuilder {
	file := mustValue(json.Marshal(map[string]any{"versions": plugin.Versions}))

	name := plugins.NameOf(plugin.ID)
	filePath := path.Join("plugins", name+".json")
	b.files[filePath] = file

	plugin.Versions = nil
	plugin.Ref = &registry.Ref{Path: filePath, SHA256: SHA256Hex(file)}

	return b.AddPlugin(plugin)
}

// Revoke revokes a version of a plugin.
func (b *IndexBuilder) Revoke(id, version, reason string) *IndexBuilder {
	b.index.Revocations = append(b.index.Revocations, registry.Revocation{ID: id, Version: version, Reason: reason})

	return b
}

// Deprecate deprecates a plugin, or a version of it.
func (b *IndexBuilder) Deprecate(deprecation registry.Deprecation) *IndexBuilder {
	b.index.Deprecations = append(b.index.Deprecations, deprecation)

	return b
}

// Build returns the index.json document.
func (b *IndexBuilder) Build() []byte {
	return mustValue(json.Marshal(b.index))
}

// Files returns the per-plugin files of the plugins added with AddRefPlugin,
// by their path relative to the index. The caller must not change the map.
func (b *IndexBuilder) Files() map[string][]byte {
	return b.files
}

// mustValue returns value, and panics when err is not nil. It is for building test
// data out of values that this package controls, where an error is a bug.
func mustValue[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}

	return value
}
