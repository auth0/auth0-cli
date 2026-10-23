package approvals_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins/approvals"
)

// document returns an approvals file with one approval, with the given pieces
// replaced by name: "version", "tenant", "id", "hash" and "extra".
func document(replace map[string]string) string {
	pieces := map[string]string{
		"version": `"formatVersion": 1`,
		"tenant":  `"` + tenant + `"`,
		"id":      `"` + id + `"`,
		"hash":    `"permissionsHash": "` + hashA + `"`,
		"time":    `"approvedAt": "2026-05-01T12:00:00Z"`,
		"extra":   ``,
	}

	for name, piece := range replace {
		pieces[name] = piece
	}

	return `{` + pieces["version"] + `, "tenants": {` + pieces["tenant"] + `: {` + pieces["id"] +
		`: {` + pieces["hash"] + `, ` + pieces["time"] + pieces["extra"] + `}}}}`
}

func TestApprovalsFile(t *testing.T) {
	t.Run("a valid file is read", func(t *testing.T) {
		a := newApprovals(t)
		writeFile(t, a, document(nil))

		require.NoError(t, a.Check(tenant, id, hashA))
	})

	damaged := []struct {
		name  string
		write func(t *testing.T, path string)
	}{
		{name: "garbage", write: content("this is not json")},
		{name: "empty file", write: content("")},
		{name: "trailing data", write: content(document(nil) + " {}")},
		{name: "duplicate key", write: content(strings.Replace(document(nil), `"formatVersion": 1`, `"formatVersion": 1, "formatVersion": 1`, 1))},
		{name: "duplicate tenant", write: content(`{"formatVersion": 1, "tenants": {"` + tenant + `": {}, "` + tenant + `": {}}}`)},
		{name: "case variant of a top level key", write: content(strings.Replace(document(nil), `"tenants"`, `"Tenants"`, 1))},
		{name: "case variant of a field", write: content(document(map[string]string{"hash": `"PermissionsHash": "` + hashA + `"`}))},
		{name: "unknown field", write: content(document(map[string]string{"extra": `, "note": "x"`}))},
		{name: "wrong format version", write: content(document(map[string]string{"version": `"formatVersion": 2`}))},
		{name: "format version as text", write: content(document(map[string]string{"version": `"formatVersion": "1"`}))},
		{name: "null tenants", write: content(`{"formatVersion": 1, "tenants": null}`)},
		{name: "tenants as text", write: content(`{"formatVersion": 1, "tenants": "x"}`)},
		{name: "tenants as a list", write: content(`{"formatVersion": 1, "tenants": []}`)},
		{name: "null tenant entry", write: content(`{"formatVersion": 1, "tenants": {"` + tenant + `": null}}`)},
		{name: "null approval", write: content(`{"formatVersion": 1, "tenants": {"` + tenant + `": {"` + id + `": null}}}`)},
		{name: "bad hash", write: content(document(map[string]string{"hash": `"permissionsHash": "sha256:abc"`}))},
		{name: "upper case hash", write: content(document(map[string]string{"hash": `"permissionsHash": "sha256:` + upper + `"`}))},
		{name: "bad time", write: content(document(map[string]string{"time": `"approvedAt": "yesterday"`}))},
		{name: "upper case tenant", write: content(document(map[string]string{"tenant": `"ACME.us.auth0.com"`}))},
		{name: "invalid plugin ID", write: content(document(map[string]string{"id": `"checkmate"`}))},
		{name: "larger than 1 MiB", write: content(document(nil) + strings.Repeat(" ", 1<<20))},
		{name: "a symlink to a valid file", write: func(t *testing.T, path string) {
			target := filepath.Join(t.TempDir(), "target.json")
			require.NoError(t, os.WriteFile(target, []byte(document(nil)), 0o600))

			if err := os.Symlink(target, path); err != nil {
				t.Skipf("symlinks are not available: %v", err)
			}
		}},
		{name: "a directory", write: func(t *testing.T, path string) {
			require.NoError(t, os.Mkdir(path, 0o700))
		}},
		{name: "writable by other users", write: func(t *testing.T, path string) {
			if runtime.GOOS == "windows" {
				t.Skip("Windows has no permission bits to check")
			}

			require.NoError(t, os.WriteFile(path, []byte(document(nil)), 0o600))
			require.NoError(t, os.Chmod(path, 0o666))
		}},
	}

	for _, test := range damaged {
		t.Run(test.name+" is damaged for every reader and kept", func(t *testing.T) {
			a := newApprovals(t)
			require.NoError(t, os.MkdirAll(a.Dir, 0o700))

			path := filepath.Join(a.Dir, "approvals.json")
			test.write(t, path)

			require.ErrorIs(t, a.Approve(t.Context(), tenant, id, hashB), approvals.ErrDamaged)

			err := a.Check(tenant, id, hashA)
			require.ErrorIs(t, err, approvals.ErrDamaged)
			assert.ErrorContains(t, err, path)
			require.ErrorIs(t, errOf(a.Approved(tenant, id, hashA)), approvals.ErrDamaged)
			require.ErrorIs(t, errOf(a.List(id)), approvals.ErrDamaged)

			_, err = os.Lstat(path)
			assert.NoError(t, err, "the damaged file must stay")
		})

		t.Run(test.name+" is deleted by the remove functions", func(t *testing.T) {
			for _, remove := range removers {
				a := newApprovals(t)
				require.NoError(t, os.MkdirAll(a.Dir, 0o700))

				path := filepath.Join(a.Dir, "approvals.json")
				test.write(t, path)

				require.NoError(t, remove(a))

				_, err := os.Lstat(path)
				assert.ErrorIs(t, err, os.ErrNotExist)
			}
		})
	}
}

func TestRemoveKeepsADirectoryWithContent(t *testing.T) {
	for name, remove := range removers {
		t.Run(name, func(t *testing.T) {
			a := newApprovals(t)
			inside := filepath.Join(a.Dir, "approvals.json", "inside")
			require.NoError(t, os.MkdirAll(inside, 0o700))

			require.Error(t, remove(a), "a directory is never emptied for the person")

			_, err := os.Lstat(inside)
			assert.NoError(t, err)
		})
	}
}

// content returns a writer that puts text in the file with the given path.
func content(text string) func(t *testing.T, path string) {
	return func(t *testing.T, path string) {
		require.NoError(t, os.WriteFile(path, []byte(text), 0o600))
	}
}

// errOf returns the error of a function that also returns a value.
func errOf[T any](_ T, err error) error {
	return err
}
