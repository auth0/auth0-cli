package registry_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins/registry"
	"github.com/auth0/auth0-cli/internal/plugins/registry/registrytest"
)

const testPluginID = "auth0/checkmate"

// fixture is a registry served over TLS, published by a repository with fresh
// keys, and an empty cache directory for a client.
type fixture struct {
	t      *testing.T
	server *registrytest.Server
	repo   *registrytest.Repo
	dir    string
}

// newFixture serves a valid index of version 1 with one plugin that has two
// versions.
func newFixture(t *testing.T) *fixture {
	t.Helper()

	f := &fixture{
		t:      t,
		server: registrytest.NewServer(t),
		repo:   registrytest.NewRepo(t),
		dir:    filepath.Join(t.TempDir(), "registries", "auth0"),
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

// publish signs and serves the index of the builder with the repository of the
// fixture.
func (f *fixture) publish(builder *registrytest.IndexBuilder) {
	f.t.Helper()

	f.server.Publish(builder, f.repo)
}

// publishRaw signs and serves index.json as it is.
func (f *fixture) publishRaw(index []byte) {
	f.t.Helper()

	f.server.PublishRaw(index, f.repo)
}

// client returns a client for the registry of the fixture, which trusts the
// first root of its repository.
func (f *fixture) client() *registry.Client {
	return &registry.Client{
		Registry: f.server.Registry(),
		Root:     f.repo.Root(),
		HTTP:     f.server.Client(),
		Dir:      f.dir,
	}
}

// indexRequests returns how many times the server was asked for the index of the
// builder.
func (f *fixture) indexRequests(builder *registrytest.IndexBuilder) int {
	return f.server.Requests(registrytest.IndexPath(builder.Build()))
}

// cache returns the content of every file in the cache directory by its path
// relative to it, written with forward slashes, without the lock file, or an
// empty map when the directory does not exist.
func (f *fixture) cache() map[string]string {
	f.t.Helper()

	files := map[string]string{}

	err := filepath.WalkDir(f.dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || entry.Name() == ".lock" {
			return err
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		relative, err := filepath.Rel(f.dir, path)
		if err != nil {
			return err
		}

		files[filepath.ToSlash(relative)] = string(content)

		return nil
	})
	if !os.IsNotExist(err) {
		require.NoError(f.t, err)
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
