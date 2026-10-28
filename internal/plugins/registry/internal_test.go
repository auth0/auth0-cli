package registry

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// StageTempFile is the function that stores the files of the cache, so that a
// test can wrap it.
var StageTempFile = stageTempFile

// SetStageFile replaces the function that writes a file of the cache to a
// temporary file, and returns a function that puts the original back.
func SetStageFile(stage func(dir, name string, data []byte) (string, error)) (restore func()) {
	original := stageFile
	stageFile = stage

	return func() { stageFile = original }
}

// SetRenameFile replaces the function that moves a temporary file into place,
// and returns a function that puts the original back.
func SetRenameFile(rename func(oldPath, newPath string) error) (restore func()) {
	original := renameFile
	renameFile = rename

	return func() { renameFile = original }
}

// HoldLock takes the lock of the cache in dir, which it creates when it does not
// exist, and returns the function that frees it.
func HoldLock(dir string) (release func(), err error) {
	return (&Client{Dir: dir}).lock(context.Background())
}

// LoadWithoutRetry is Load as it reads the cache the first time, without taking
// the lock to read it again after a failure. It returns only the error.
func (c *Client) LoadWithoutRetry() error {
	_, _, err := c.loadCached()

	return err
}

func TestWriteFileAtomic(t *testing.T) {
	t.Run("creates and replaces a file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "state.json")

		require.NoError(t, writeFileAtomic(path, []byte("one")))
		require.NoError(t, writeFileAtomic(path, []byte("two")))

		content, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "two", string(content))

		if runtime.GOOS != "windows" {
			info, err := os.Stat(path)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		}
	})

	t.Run("leaves neither a partial file nor a temporary file when it fails", func(t *testing.T) {
		dir := t.TempDir()

		// A rename onto a directory that has content fails after the data was written.
		path := filepath.Join(dir, "state.json")
		require.NoError(t, os.MkdirAll(filepath.Join(path, "child"), 0o700))

		require.Error(t, writeFileAtomic(path, []byte("new content")))

		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, "state.json", entries[0].Name())
		assert.True(t, entries[0].IsDir())
	})

	t.Run("fails when the directory does not exist", func(t *testing.T) {
		assert.Error(t, writeFileAtomic(filepath.Join(t.TempDir(), "missing", "state.json"), []byte("x")))
	})
}

func TestParseKeys(t *testing.T) {
	public, _, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	encoded := base64.StdEncoding.EncodeToString(public)

	t.Run("reads every .pub file and ignores the rest", func(t *testing.T) {
		keys, err := parseKeys(fstest.MapFS{
			"keys/first.pub":  {Data: []byte(encoded + "\n")},
			"keys/second.pub": {Data: []byte("  " + encoded + "  ")},
			"keys/README":     {Data: []byte("not a key")},
			"keys/notes.txt":  {Data: []byte("not a key")},
			"keys/dir.pub/x":  {Data: []byte("not a key")},
		})

		require.NoError(t, err)
		assert.Equal(t, KeySet{"first": public, "second": public}, keys)
	})

	t.Run("returns an empty set when there is no key", func(t *testing.T) {
		keys, err := parseKeys(fstest.MapFS{"keys/README": {Data: []byte("x")}})

		require.NoError(t, err)
		assert.Empty(t, keys)
	})

	failures := map[string]string{
		"not base64":        "not base64!",
		"too short":         base64.StdEncoding.EncodeToString(public[:16]),
		"too long":          base64.StdEncoding.EncodeToString(append(public[:len(public):len(public)], 0)),
		"empty":             "",
		"a private key too": base64.StdEncoding.EncodeToString(make([]byte, ed25519.PrivateKeySize)),
	}

	for name, content := range failures {
		t.Run("rejects a key that is "+name, func(t *testing.T) {
			_, err := parseKeys(fstest.MapFS{"keys/bad.pub": {Data: []byte(content)}})

			require.Error(t, err)
			assert.Contains(t, err.Error(), "bad.pub")
		})
	}

	t.Run("fails when the directory is missing", func(t *testing.T) {
		_, err := parseKeys(fstest.MapFS{})
		assert.Error(t, err)
	})
}

func TestTrustedKeys_CannotBeChangedByTheCaller(t *testing.T) {
	first := TrustedKeys()
	first["injected"] = make(ed25519.PublicKey, ed25519.PublicKeySize)

	assert.NotContains(t, TrustedKeys(), "injected")
}

func TestCheckURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "https", url: "https://example.com/index.json"},
		{name: "https with a port", url: "https://example.com:8443/index.json"},
		{name: "https on a loopback address", url: "https://127.0.0.1:8443/index.json"},
		{name: "http", url: "http://example.com/index.json", wantErr: true},
		{name: "http on another loopback address", url: "http://127.0.0.2/index.json", wantErr: true},
		{name: "http on a name that only starts with localhost", url: "http://localhost.example.com/index.json", wantErr: true},
		{name: "http with a user that looks like localhost", url: "http://localhost@example.com/index.json", wantErr: true},
		{name: "a file URL", url: "file:///etc/passwd", wantErr: true},
		{name: "no scheme", url: "example.com/index.json", wantErr: true},
		{name: "no host", url: "https:///index.json", wantErr: true},
		{name: "a password", url: "https://user:pass@example.com/index.json", wantErr: true},
		{name: "empty", url: "", wantErr: true},
		{name: "not a URL", url: "https://exa mple.com", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := checkURL(test.url)

			if test.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSignatureURL(t *testing.T) {
	tests := map[string]string{
		"https://example.com/index.json":         "https://example.com/index.json.sig",
		"https://example.com/a/b/index.json":     "https://example.com/a/b/index.json.sig",
		"https://example.com/index.json?token=1": "https://example.com/index.json.sig?token=1",
		"https://example.com/index.json?a=1&b=2": "https://example.com/index.json.sig?a=1&b=2",
		"https://example.com/index.json#top":     "https://example.com/index.json.sig",
	}

	for in, want := range tests {
		got, err := signatureURL(in)

		require.NoError(t, err)
		assert.Equal(t, want, got)
	}
}

func TestDescribeAge(t *testing.T) {
	tests := []struct {
		age  time.Duration
		want string
	}{
		{age: 10 * time.Minute, want: "less than an hour"},
		{age: time.Hour, want: "1 hour"},
		{age: 90 * time.Minute, want: "1 hour"},
		{age: 2 * time.Hour, want: "2 hours"},
		{age: 47 * time.Hour, want: "47 hours"},
		{age: 48 * time.Hour, want: "2 days"},
		{age: 31*24*time.Hour + time.Hour, want: "31 days"},
	}

	for _, test := range tests {
		assert.Equal(t, test.want, describeAge(test.age), test.age.String())
	}
}

func TestRejectCaseVariants(t *testing.T) {
	type inner struct {
		Name string `json:"name"`
	}

	type outer struct {
		Title string  `json:"title"`
		Items []inner `json:"items"`
		Link  *inner  `json:"link,omitempty"`
		Skip  string  `json:"-"`
	}

	tests := []struct {
		name     string
		document map[string]any
		want     []string
	}{
		{name: "exact names", document: map[string]any{"title": "x", "items": []any{map[string]any{"name": "y"}}}},
		{name: "unknown names are left alone", document: map[string]any{"other": 1, "items": []any{map[string]any{"extra": 1}}}},
		{name: "a name in another case", document: map[string]any{"Title": "x"}, want: []string{`Title: field name must be written as "title"`}},
		{name: "a nested name in another case", document: map[string]any{"items": []any{map[string]any{"NAME": "y"}}}, want: []string{`items[0].NAME: field name must be written as "name"`}},
		{name: "behind a pointer", document: map[string]any{"link": map[string]any{"Name": "y"}}, want: []string{`link.Name: field name must be written as "name"`}},
		{name: "a field that is not decoded", document: map[string]any{"Skip": "x"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, rejectCaseVariants(test.document, reflect.TypeFor[outer]()))
		})
	}
}
