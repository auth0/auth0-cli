package store_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins/store"
)

func TestLockfile_RoundTrip(t *testing.T) {
	f := newFixture(t)
	index := f.index(f.release("1.0.0"), f.release("2.0.0"))

	f.install(index, "1.0.0")
	f.install(index, "2.0.0")

	document := f.readLockfile()
	assert.EqualValues(t, 1, document["formatVersion"])

	entry := document["plugins"].(map[string]any)[testID].(map[string]any)
	assert.Equal(t, "auth0", entry["registry"])
	assert.Equal(t, "checkmate", entry["name"])
	assert.Equal(t, "2.0.0", entry["active"])
	assert.Equal(t, "1.0.0", entry["previous"])
	assert.Equal(t, true, entry["pinned"])

	version := entry["versions"].(map[string]any)["2.0.0"].(map[string]any)
	for _, key := range []string{"bin", "binSha256", "manifestSha256", "artifactSha256", "permissionsHash", "size", "installedAt"} {
		assert.Contains(t, version, key)
	}

	installed, err := f.store.List()
	require.NoError(t, err)
	require.Len(t, installed, 1)
	assert.Equal(t, store.Installed{ID: testID, Active: "2.0.0", Previous: "1.0.0", Pinned: true, Size: installed[0].Size}, installed[0])
	assert.Positive(t, installed[0].Size)
}

func TestLockfile_ListIsEmptyWithoutALockfile(t *testing.T) {
	f := newFixture(t)

	installed, err := f.store.List()

	require.NoError(t, err)
	assert.Empty(t, installed)
	assert.NoDirExists(t, f.dir, "reading creates nothing")
}

func TestLockfile_IsRefusedWhenItIsNotTrustworthy(t *testing.T) {
	f := newFixture(t)
	f.install(f.index(f.release("1.0.0")), "1.0.0")

	good, err := os.ReadFile(f.lockfilePath())
	require.NoError(t, err)

	text := string(good)

	replace := func(old, replacement string) string {
		require.Contains(t, text, old)

		return strings.Replace(text, old, replacement, 1)
	}

	tests := []struct {
		name    string
		content string
	}{
		{"a duplicate key", replace(`"active": "1.0.0"`, `"active": "1.0.0", "active": "1.0.0"`)},
		{"a duplicate plugin", replace(`"plugins": {`, `"plugins": {"auth0/checkmate": null,`)},
		{"an unknown format version", replace(`"formatVersion": 1`, `"formatVersion": 2`)},
		{"no format version", replace(`"formatVersion": 1,`, ``)},
		{"JSON that is cut off", text[:len(text)/2]},
		{"something that is not JSON", "not json"},
		{"a document that is not an object", `["plugins"]`},
		{"an empty file", ""},
		{"an active version that is not listed", replace(`"active": "1.0.0"`, `"active": "9.9.9"`)},
		{"a previous version that is not listed", replace(`"pinned"`, `"previous": "0.0.1", "pinned"`)},
		{"a plugin under another key", replace(`"auth0/checkmate": {`, `"auth0/other": {`)},
		{"a path as the plugin key", strings.Replace(replace(`"auth0/checkmate": {`, `"../../escape/x": {`), `"registry": "auth0"`, `"registry": ".."`, 1)},
		{"a plugin key that is a path and agrees with its fields", strings.Replace(replace(`"auth0/checkmate": {`, `"../checkmate": {`), `"registry": "auth0"`, `"registry": ".."`, 1)},
		{"a version that is a path", strings.ReplaceAll(text, `"1.0.0"`, `"../../1.0.0"`)},
		{"a version with a suffix", strings.ReplaceAll(text, `"1.0.0"`, `"1.0.0-beta.1"`)},
		{"an executable name that is a path", replace(`"bin": "checkmate"`, `"bin": "../checkmate"`)},
		{"an executable named like the manifest", replace(`"bin": "checkmate"`, `"bin": "plugin.json"`)},
		{"a hash that is not hex", strings.Replace(text, `"binSha256": "`, `"binSha256": "zz`, 1)},
		{"an upper case hash", replace(`"permissionsHash": "sha256:`, `"permissionsHash": "SHA256:`)},
		{"a null entry", replace(`"auth0/checkmate": {`, `"auth0/checkmate": null, "auth0/other": {`)},
		{"a file that is too large", text[:len(text)-2] + strings.Repeat(" ", 1<<20) + "\n}"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.NoError(t, os.WriteFile(f.lockfilePath(), []byte(test.content), 0o600))

			_, listErr := f.store.List()
			_, verifyErr := f.store.Verify(testID)

			assert.ErrorIs(t, listErr, store.ErrLockfileDamaged)
			assert.ErrorIs(t, verifyErr, store.ErrLockfileDamaged)

			t.Run("and it is not replaced by an install, an update, or a removal", func(t *testing.T) {
				index := f.index(f.release("2.0.0"))
				before := f.requests("2.0.0")

				_, err := f.store.Install(t.Context(), index, testID, "", store.Options{})
				assert.ErrorIs(t, err, store.ErrLockfileDamaged)

				_, err = f.store.Update(t.Context(), index, testID, store.Options{})
				assert.ErrorIs(t, err, store.ErrLockfileDamaged)

				_, err = f.store.Remove(t.Context(), testID)
				assert.ErrorIs(t, err, store.ErrLockfileDamaged)

				_, err = f.store.UpdateAll(t.Context(), index, store.Options{})
				assert.ErrorIs(t, err, store.ErrLockfileDamaged)

				after, err := os.ReadFile(f.lockfilePath())
				require.NoError(t, err)
				assert.Equal(t, test.content, string(after))
				assert.Equal(t, before, f.requests("2.0.0"), "nothing was downloaded")
				assert.DirExists(t, f.versionDir("1.0.0"), "the installed files were not touched")
			})
		})
	}
}
