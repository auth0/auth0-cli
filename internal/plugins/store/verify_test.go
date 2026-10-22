package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins/store"
)

// chmod returns a change that sets the mode of a file in the version directory,
// or of the directory itself when name is empty. A name made of ".." climbs to
// the directories above it, up to the plugins directory.
func chmod(name string, mode os.FileMode) func(t *testing.T, f *fixture) {
	return func(t *testing.T, f *fixture) {
		t.Helper()

		require.NoError(t, os.Chmod(filepath.Join(f.versionDir("1.0.0"), name), mode))
	}
}

func TestVerify(t *testing.T) {
	// Install returns a fixture with 1.0.0 installed and active.
	install := func(t *testing.T) *fixture {
		t.Helper()

		f := newFixture(t)
		f.install(f.index(f.release("1.0.0")), "1.0.0")

		return f
	}

	t.Run("passes for a clean install and returns the parsed manifest", func(t *testing.T) {
		f := install(t)

		verified, err := f.store.Verify(testID)

		require.NoError(t, err)
		assert.Equal(t, testID, verified.ID)
		assert.Equal(t, "1.0.0", verified.Version)
		assert.Equal(t, f.versionDir("1.0.0"), verified.Dir)
		assert.Equal(t, filepath.Join(f.versionDir("1.0.0"), "plugin.json"), verified.ManifestPath)
		require.NotNil(t, verified.Manifest)
		assert.Equal(t, testID, verified.Manifest.ID)
		assert.Equal(t, verified.Manifest.PermissionsHash(), verified.PermissionsHash)
	})

	t.Run("fails for a plugin that is not installed", func(t *testing.T) {
		f := install(t)

		_, err := f.store.Verify("auth0/other")

		var notInstalled *store.NotInstalledError
		require.ErrorAs(t, err, &notInstalled)
		assert.Equal(t, "other is not installed. Install it with: auth0 plugin install other", err.Error())
	})

	t.Run("fails for an ID that is not a path-safe plugin ID", func(t *testing.T) {
		f := install(t)

		_, err := f.store.Verify("../auth0/checkmate")

		assert.Error(t, err)
	})

	refused := []struct {
		name   string
		change func(t *testing.T, f *fixture)
		want   string
	}{
		{"a binary with one byte flipped", func(t *testing.T, f *fixture) {
			path := filepath.Join(f.versionDir("1.0.0"), testBin)
			content, err := os.ReadFile(path)
			require.NoError(t, err)

			content[len(content)/2] ^= 0xff
			require.NoError(t, os.WriteFile(path, content, 0o755))
		}, "the hash of checkmate changed"},
		{"a binary with a byte appended", func(t *testing.T, f *fixture) {
			file, err := os.OpenFile(filepath.Join(f.versionDir("1.0.0"), testBin), os.O_APPEND|os.O_WRONLY, 0)
			require.NoError(t, err)
			_, err = file.WriteString("x")
			require.NoError(t, err)
			require.NoError(t, file.Close())
		}, "the hash of checkmate changed"},
		{"a manifest that was modified", func(t *testing.T, f *fixture) {
			path := filepath.Join(f.versionDir("1.0.0"), "plugin.json")
			content, err := os.ReadFile(path)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(path, append(content, ' '), 0o600))
		}, "the hash of plugin.json changed"},
		{"a missing binary", func(t *testing.T, f *fixture) {
			require.NoError(t, os.Remove(filepath.Join(f.versionDir("1.0.0"), testBin)))
		}, "entries"},
		{"a missing manifest", func(t *testing.T, f *fixture) {
			require.NoError(t, os.Remove(filepath.Join(f.versionDir("1.0.0"), "plugin.json")))
		}, "entries"},
		{"a missing version directory", func(t *testing.T, f *fixture) {
			require.NoError(t, os.RemoveAll(f.versionDir("1.0.0")))
		}, "is not a directory"},
		{"an extra file next to the binary", func(t *testing.T, f *fixture) {
			require.NoError(t, os.WriteFile(filepath.Join(f.versionDir("1.0.0"), "libevil.so"), []byte("x"), 0o644))
		}, "entries"},
		{"an extra directory", func(t *testing.T, f *fixture) {
			require.NoError(t, os.Mkdir(filepath.Join(f.versionDir("1.0.0"), "sub"), 0o755))
		}, "entries"},
		{"a manifest that is for another version", func(t *testing.T, f *fixture) {
			f.rewriteManifest("1.0.0", func(m map[string]any) { m["version"] = "9.9.9" })
		}, "is for"},
		{"a manifest that is for another plugin", func(t *testing.T, f *fixture) {
			f.rewriteManifest("1.0.0", func(m map[string]any) { m["id"] = "auth0/other" })
		}, "is for"},
		{"a manifest with other permissions, even when its hash was recorded", func(t *testing.T, f *fixture) {
			f.rewriteManifest("1.0.0", func(m map[string]any) {
				m["commands"].([]any)[0].(map[string]any)["permissions"] = map[string]any{
					"scopes": []string{"delete:clients"}, "hosts": []any{}, "files": []any{}, "env": []any{},
				}
			})
		}, "the permissions of plugin.json changed"},
		{"a manifest that is not valid, even when its hash was recorded", func(t *testing.T, f *fixture) {
			f.rewriteManifest("1.0.0", func(m map[string]any) { delete(m, "description") })
		}, "is not valid"},
		{"a manifest with a permission field this CLI does not know", func(t *testing.T, f *fixture) {
			f.rewriteManifest("1.0.0", func(m map[string]any) {
				m["commands"].([]any)[0].(map[string]any)["permissions"] = map[string]any{
					"scopes": []any{}, "hosts": []any{}, "files": []any{}, "env": []any{}, "network": "all",
				}
			})
		}, "is not valid"},
	}

	for _, test := range refused {
		t.Run("refuses "+test.name, func(t *testing.T) {
			f := install(t)

			test.change(t, f)

			_, err := f.store.Verify(testID)

			var verification *store.VerificationError
			require.ErrorAs(t, err, &verification)
			assert.Contains(t, err.Error(), test.want)
			assert.Contains(t, err.Error(), "Reinstall it with: auth0 plugin install checkmate")
		})
	}

	// These rely on Unix modes and link counts, which Verify does not check on
	// Windows.
	unsafe := []struct {
		name   string
		change func(t *testing.T, f *fixture)
		want   string
	}{
		{"a binary with another hard link", func(t *testing.T, f *fixture) {
			other := filepath.Join(filepath.Dir(f.versionDir("1.0.0")), "linked")
			require.NoError(t, os.Link(filepath.Join(f.versionDir("1.0.0"), testBin), other))
		}, "has more than one hard link"},
		{"a manifest with another hard link", func(t *testing.T, f *fixture) {
			other := filepath.Join(filepath.Dir(f.versionDir("1.0.0")), "linked")
			require.NoError(t, os.Link(filepath.Join(f.versionDir("1.0.0"), "plugin.json"), other))
		}, "has more than one hard link"},
		{"a binary writable by its group", chmod(testBin, 0o775), "is writable by other users"},
		{"a binary writable by everyone", chmod(testBin, 0o757), "is writable by other users"},
		{"a manifest writable by everyone", chmod("plugin.json", 0o666), "is writable by other users"},
		{"a manifest writable by its group", chmod("plugin.json", 0o620), "is writable by other users"},
		{"a version directory writable by its group", chmod("", 0o770), "is writable by other users"},
		{"a version directory writable by everyone", chmod("", 0o707), "is writable by other users"},
		{"a plugin directory writable by its group", chmod("..", 0o770), "is writable by other users"},
		{"a plugin directory writable by everyone", chmod("..", 0o707), "is writable by other users"},
		{"a registry directory writable by its group", chmod("../..", 0o770), "is writable by other users"},
		{"a registry directory writable by everyone", chmod("../..", 0o707), "is writable by other users"},
		{"the store directory writable by its group", chmod("../../..", 0o770), "is writable by other users"},
		{"the store directory writable by everyone", chmod("../../..", 0o707), "is writable by other users"},
		{"the plugins directory writable by its group", chmod("../../../..", 0o770), "is writable by other users"},
		{"the plugins directory writable by everyone", chmod("../../../..", 0o707), "is writable by other users"},
	}

	for _, test := range unsafe {
		t.Run("refuses "+test.name, func(t *testing.T) {
			skipOnWindows(t)

			f := install(t)

			test.change(t, f)

			_, err := f.store.Verify(testID)

			var verification *store.VerificationError
			require.ErrorAs(t, err, &verification)
			assert.Contains(t, err.Error(), test.want)
			assert.Contains(t, err.Error(), "Reinstall it with: auth0 plugin install checkmate")
		})
	}

	t.Run("refuses a binary or a manifest that was swapped for a link to identical content", func(t *testing.T) {
		skipOnWindows(t)

		for _, name := range []string{testBin, "plugin.json"} {
			t.Run(name, func(t *testing.T) {
				f := install(t)

				path := filepath.Join(f.versionDir("1.0.0"), name)
				content, err := os.ReadFile(path)
				require.NoError(t, err)

				elsewhere := filepath.Join(t.TempDir(), name)
				require.NoError(t, os.WriteFile(elsewhere, content, 0o755))
				require.NoError(t, os.Remove(path))
				require.NoError(t, os.Symlink(elsewhere, path))

				_, err = f.store.Verify(testID)

				var verification *store.VerificationError
				require.ErrorAs(t, err, &verification)
				assert.Contains(t, err.Error(), "is not a regular file")
			})
		}
	})

	t.Run("refuses a version directory that was swapped for a link", func(t *testing.T) {
		skipOnWindows(t)

		f := install(t)

		moved := f.versionDir("1.0.0") + ".real"
		require.NoError(t, os.Rename(f.versionDir("1.0.0"), moved))
		require.NoError(t, os.Symlink(moved, f.versionDir("1.0.0")))

		_, err := f.store.Verify(testID)

		var verification *store.VerificationError
		require.ErrorAs(t, err, &verification)
		assert.Contains(t, err.Error(), "is not a directory")
	})

	t.Run("refuses a plugin directory that was swapped for a link", func(t *testing.T) {
		skipOnWindows(t)

		f := install(t)

		pluginDir := filepath.Dir(f.versionDir("1.0.0"))
		moved := pluginDir + ".real"
		require.NoError(t, os.Rename(pluginDir, moved))
		require.NoError(t, os.Symlink(moved, pluginDir))

		_, err := f.store.Verify(testID)

		var verification *store.VerificationError
		require.ErrorAs(t, err, &verification)
		assert.Contains(t, err.Error(), pluginDir+" is not a directory")
	})

	t.Run("checks the active version only", func(t *testing.T) {
		f := newFixture(t)
		index := f.index(f.release("1.0.0"), f.release("2.0.0"))
		f.install(index, "1.0.0")
		f.install(index, "2.0.0")

		require.NoError(t, os.WriteFile(filepath.Join(f.versionDir("1.0.0"), testBin), []byte("tampered"), 0o755))

		verified, err := f.store.Verify(testID)

		require.NoError(t, err)
		assert.Equal(t, "2.0.0", verified.Version)
	})
}
