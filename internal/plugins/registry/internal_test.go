package registry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/theupdateframework/go-tuf/v2/metadata"
	"github.com/theupdateframework/go-tuf/v2/metadata/trustedmetadata"
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

func TestCheckURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "https", url: "https://example.com/registry/"},
		{name: "https with a port", url: "https://example.com:8443/registry/"},
		{name: "https on a loopback address", url: "https://127.0.0.1:8443/registry/"},
		{name: "http", url: "http://example.com/registry/", wantErr: true},
		{name: "http on another loopback address", url: "http://127.0.0.2/registry/", wantErr: true},
		{name: "http on a name that only starts with localhost", url: "http://localhost.example.com/registry/", wantErr: true},
		{name: "http with a user that looks like localhost", url: "http://localhost@example.com/registry/", wantErr: true},
		{name: "a file URL", url: "file:///etc/passwd", wantErr: true},
		{name: "no scheme", url: "example.com/registry/", wantErr: true},
		{name: "no host", url: "https:///registry/", wantErr: true},
		{name: "a password", url: "https://user:pass@example.com/registry/", wantErr: true},
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

func TestBaseURL(t *testing.T) {
	tests := map[string]string{
		"https://example.com/registry/":         "https://example.com/registry/",
		"https://example.com/registry":          "https://example.com/registry/",
		"https://example.com":                   "https://example.com/",
		"https://example.com/registry/?token=1": "https://example.com/registry/?token=1",
		"https://example.com/registry?a=1&b=2":  "https://example.com/registry/?a=1&b=2",
		"https://example.com/registry/#top":     "https://example.com/registry/",
		"https://example.com/a%20b/registry":    "https://example.com/a%20b/registry/",
	}

	for in, want := range tests {
		got, err := (&Client{Registry: Registry{URL: in}}).baseURL()

		require.NoError(t, err, in)
		assert.Equal(t, want, got.String(), in)
	}

	_, err := (&Client{Registry: Registry{URL: "https://exa mple.com"}}).baseURL()
	assert.Error(t, err)
}

func TestTUFError(t *testing.T) {
	now := time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		err         error
		want        error
		wantTimeout bool
	}{
		{name: "offline", err: fmt.Errorf("download: %w", ErrOffline), want: ErrOffline},
		{name: "cancelled", err: fmt.Errorf("download: %w", context.Canceled), want: context.Canceled},
		{name: "insecure url", err: fmt.Errorf("download: %w", ErrInsecureURL), want: ErrInsecureURL},
		{name: "a wrong version", err: &metadata.ErrBadVersionNumber{Msg: "bad version"}, want: ErrRollback},
		{name: "an equal version", err: &metadata.ErrEqualVersionNumber{Msg: "equal version"}, want: ErrSignatureInvalid},
		{name: "not enough signatures", err: &metadata.ErrUnsignedMetadata{Msg: "unsigned"}, want: ErrSignatureInvalid},
		{name: "a hash mismatch", err: &metadata.ErrLengthOrHashMismatch{Msg: "mismatch"}, want: ErrSignatureInvalid},
		{name: "a length mismatch", err: &metadata.ErrDownloadLengthMismatch{Msg: "too long"}, want: ErrSignatureInvalid},
		{name: "something unknown", err: errors.New("unexpected"), want: ErrSignatureInvalid},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := tufError(nil, now, test.err)

			assert.ErrorIs(t, got, test.want)

			if test.want == ErrSignatureInvalid {
				assert.NotErrorIs(t, got, ErrOffline)
			}
		})
	}
}

func TestExpiryError(t *testing.T) {
	now := time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	root := func(expires time.Time) *metadata.Metadata[metadata.RootType] { return metadata.Root(expires) }
	timestamp := func(expires time.Time) *metadata.Metadata[metadata.TimestampType] {
		return metadata.Timestamp(expires)
	}
	snapshot := func(expires time.Time) *metadata.Metadata[metadata.SnapshotType] {
		return metadata.Snapshot(expires)
	}
	cause := errors.New("expired")

	tests := []struct {
		name          string
		trusted       trustedmetadata.TrustedMetadata
		wantFile      string
		wantTimestamp bool
	}{
		{
			name:     "the root",
			trusted:  trustedmetadata.TrustedMetadata{Root: root(past), Timestamp: timestamp(past)},
			wantFile: rootFileName,
		},
		{
			name:          "the timestamp",
			trusted:       trustedmetadata.TrustedMetadata{Root: root(future), Timestamp: timestamp(past), Snapshot: snapshot(past)},
			wantFile:      timestampFileName,
			wantTimestamp: true,
		},
		{
			name:     "the snapshot",
			trusted:  trustedmetadata.TrustedMetadata{Root: root(future), Timestamp: timestamp(future), Snapshot: snapshot(past)},
			wantFile: snapshotFileName,
		},
		{
			name:     "the targets, which go-tuf does not keep when it is expired",
			trusted:  trustedmetadata.TrustedMetadata{Root: root(future), Timestamp: timestamp(future), Snapshot: snapshot(future)},
			wantFile: targetsFileName,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := expiryError(test.trusted, now, cause)

			require.ErrorIs(t, err, ErrExpired)
			assert.Contains(t, err.Error(), test.wantFile)

			var timestampErr *timestampExpiredError
			assert.Equal(t, test.wantTimestamp, errors.As(err, &timestampErr))
			assert.True(t, canUseCacheAfter(err) == test.wantTimestamp, "only the timestamp may fall back to the cache")
		})
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
