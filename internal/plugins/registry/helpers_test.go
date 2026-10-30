package registry_test

import (
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins/registry"
	"github.com/auth0/auth0-cli/internal/plugins/registry/registrytest"
)

const testPluginID = "auth0/checkmate"

// fixture is a registry served over TLS, signed with a fresh key, and an empty
// cache directory for a client.
type fixture struct {
	t       *testing.T
	server  *registrytest.Server
	keyID   string
	keys    registry.KeySet
	private ed25519.PrivateKey
	dir     string
}

// newFixture serves a valid index of version 1 with one plugin that has two
// versions.
func newFixture(t *testing.T) *fixture {
	t.Helper()

	keyID, keys, private := registrytest.NewKeyPair(t)

	f := &fixture{
		t:       t,
		server:  registrytest.NewServer(t),
		keyID:   keyID,
		keys:    keys,
		private: private,
		dir:     filepath.Join(t.TempDir(), "registries", "auth0"),
	}

	f.publish(f.builder())

	return f
}

// testBuilder returns a builder for a valid index of version 1 with one plugin
// that has two versions.
func testBuilder() *registrytest.IndexBuilder {
	return registrytest.NewIndexBuilder().
		AddPlugin(registrytest.NewPlugin(testPluginID, registrytest.NewVersion("1.0.0"), registrytest.NewVersion("2.0.0")))
}

// builder returns a builder for the index that the fixture serves at first.
func (f *fixture) builder() *registrytest.IndexBuilder {
	return testBuilder()
}

// publish serves the index of the builder, signed with the key of the fixture.
func (f *fixture) publish(builder *registrytest.IndexBuilder) {
	f.t.Helper()

	f.server.Publish(builder, f.keyID, f.private)
}

// publishRaw serves index.json as it is, signed with the key of the fixture.
func (f *fixture) publishRaw(index []byte) {
	f.t.Helper()

	f.server.Set("/index.json", index)
	f.server.Set("/index.json.sig", registrytest.Sign(index, f.keyID, f.private))
}

// client returns a client for the registry of the fixture.
func (f *fixture) client() *registry.Client {
	return &registry.Client{
		Registry: f.server.Registry(),
		Keys:     f.keys,
		HTTP:     f.server.Client(),
		Dir:      f.dir,
	}
}

// cache returns the content of every file in the cache directory by file name,
// without the lock file, or an empty map when the directory does not exist.
func (f *fixture) cache() map[string]string {
	f.t.Helper()

	entries, err := os.ReadDir(f.dir)
	if os.IsNotExist(err) {
		return map[string]string{}
	}
	require.NoError(f.t, err)

	files := make(map[string]string, len(entries))
	for _, entry := range entries {
		if entry.Name() == ".lock" {
			continue
		}

		content, err := os.ReadFile(filepath.Join(f.dir, entry.Name()))
		require.NoError(f.t, err)
		files[entry.Name()] = string(content)
	}

	return files
}

// readCache returns the content of one file in the cache directory.
func (f *fixture) readCache(name string) []byte {
	f.t.Helper()

	content, err := os.ReadFile(filepath.Join(f.dir, name))
	require.NoError(f.t, err)

	return content
}

// writeCache replaces one file in the cache directory.
func (f *fixture) writeCache(name string, content []byte) {
	f.t.Helper()

	require.NoError(f.t, os.WriteFile(filepath.Join(f.dir, name), content, 0o600))
}

// mustHighest returns the highest version in the state file of the cache.
func mustHighest(t *testing.T, f *fixture) int {
	t.Helper()

	var state struct {
		HighestVersion int `json:"highestVersion"`
	}
	require.NoError(t, json.Unmarshal(f.readCache("state.json"), &state))

	return state.HighestVersion
}

// decodeIndex decodes an index.json document into generic values.
func decodeIndex(t *testing.T, data []byte) map[string]any {
	t.Helper()

	var index map[string]any
	require.NoError(t, json.Unmarshal(data, &index))

	return index
}

// mutateIndex applies mutate to the generic form of an index.json document and
// returns the result.
func mutateIndex(t *testing.T, data []byte, mutate func(index map[string]any)) []byte {
	t.Helper()

	index := decodeIndex(t, data)
	mutate(index)

	mutated, err := json.Marshal(index)
	require.NoError(t, err)

	return mutated
}

// follow returns the value found by walking path from root, where a string
// selects a key of an object and an int selects an item of a list.
func follow(root any, path []any) any {
	for _, step := range path {
		switch step := step.(type) {
		case string:
			root = root.(map[string]any)[step]
		case int:
			root = root.([]any)[step]
		}
	}

	return root
}

// setValue returns a mutation that sets the value at path.
func setValue(value any, path ...any) func(index map[string]any) {
	return func(index map[string]any) {
		parent := follow(index, path[:len(path)-1])

		switch key := path[len(path)-1].(type) {
		case string:
			parent.(map[string]any)[key] = value
		case int:
			parent.([]any)[key] = value
		}
	}
}

// removeValue returns a mutation that removes the key at path.
func removeValue(path ...any) func(index map[string]any) {
	return func(index map[string]any) {
		delete(follow(index, path[:len(path)-1]).(map[string]any), path[len(path)-1].(string))
	}
}

// versionPath returns the path of a field of the first version of the first
// plugin.
func versionPath(path ...any) []any {
	return append([]any{"plugins", 0, "versions", 0}, path...)
}

// fixedClock returns a clock function that always returns now.
func fixedClock(now time.Time) func() time.Time {
	return func() time.Time { return now }
}
