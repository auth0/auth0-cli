package store

import (
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
)

// SetRenameFile replaces the function that moves a directory or the lockfile
// into place, and returns a function that puts the original back.
func SetRenameFile(rename func(oldPath, newPath string) error) (restore func()) {
	original := renameFile
	renameFile = rename

	return func() { renameFile = original }
}

// SetCurrentPlatform replaces the platform of this machine, and returns a
// function that puts the original back.
func SetCurrentPlatform(platform plugins.Platform) (restore func()) {
	original := currentPlatform
	currentPlatform = func() plugins.Platform { return platform }

	return func() { currentPlatform = original }
}

// SetReadDir replaces the function that lists a version directory when sizes are
// added up, and returns a function that puts the original back.
func SetReadDir(read func(name string) ([]os.DirEntry, error)) (restore func()) {
	original := readDir
	readDir = read

	return func() { readDir = original }
}

func TestCheckDownloadURL(t *testing.T) {
	tests := []struct {
		url     string
		allowed bool
	}{
		{"https://github.com/auth0/plugin/releases/download/v1.0.0/plugin.tar.gz", true},
		{"https://objects.githubusercontent.com/github-production-release-asset/1", true},
		{"https://release-assets.githubusercontent.com/github-production-release-asset/1", true},
		{"https://GitHub.com/auth0/plugin", true},
		{"https://github.com:443/auth0/plugin", true},
		{"http://github.com/auth0/plugin", false},
		{"https://example.com/plugin.tar.gz", false},
		{"https://githubusercontent.com/plugin", false},
		{"https://evilgithub.com/plugin", false},
		{"https://github.com.evil.example/plugin", false},
		{"https://evil.example/github.com", false},
		{"https://github.com@evil.example/plugin", false},
		{"https://user:password@github.com/plugin", false},
		{"https://github.com:8443/plugin", false},
		{"https://github.com./plugin", false},
		{"https://api.github.com/plugin", false},
		{"//github.com/plugin", false},
		{"ftp://github.com/plugin", false},
		{"file:///etc/passwd", false},
		{"github.com/plugin", false},
		{"", false},
		{"https://", false},
		{"https://[::1/plugin", false},
	}

	for _, test := range tests {
		t.Run(test.url, func(t *testing.T) {
			err := checkDownloadURL(test.url)

			if test.allowed {
				assert.NoError(t, err)

				return
			}

			assert.ErrorIs(t, err, ErrDownloadHost)
		})
	}
}

func TestBadFileName(t *testing.T) {
	good := []string{"plugin", "plugin.exe", "plugin.json", "my-plugin_1.0", "console", "connect.exe", "com10", "com0", "lpt", "lpt10.exe", "au.x", "nulled"}

	bad := []string{
		"", ".", "..", "../plugin", "/plugin", "dir/plugin", `..\plugin`, `dir\plugin`, "C:plugin", "plugin:stream", "plug\x00in", "plug\nin", "plug\x7fin",
		"plugin.", "plugin ", "plugin. ", "plugin..",
		"con", "CON", "Prn", "aux", "NUL", "com1", "COM9", "lpt1", "Lpt9",
		"con.exe", "NUL.txt", "aux.tar.gz", "com1.", "con .txt",
	}

	for _, name := range good {
		assert.Empty(t, badFileName(name), name)
	}

	for _, name := range bad {
		assert.NotEmpty(t, badFileName(name), "%q", name)
	}
}

func TestArtifactFor(t *testing.T) {
	record := registry.VersionRecord{Artifacts: []registry.Artifact{{OS: "linux", Arch: "x64", Libc: "glibc", Bin: "plugin"}}}

	artifact, found := artifactFor(record, plugins.PlatformLinuxX64)
	assert.True(t, found)
	assert.Equal(t, "plugin", artifact.Bin)

	_, found = artifactFor(record, plugins.PlatformDarwinArm64)
	assert.False(t, found, "an artifact for another platform")

	// A machine that is not a plugin platform has the empty platform, which no
	// artifact is built for.
	_, found = artifactFor(registry.VersionRecord{Artifacts: []registry.Artifact{{Bin: "plugin"}}}, "")
	assert.False(t, found, "an unknown platform has no build")
}

func BenchmarkHashFile90MB(b *testing.B) {
	const size = 90 << 20

	path := filepath.Join(b.TempDir(), "plugin")

	content := make([]byte, size)
	_, err := rand.Read(content)
	require.NoError(b, err)
	require.NoError(b, os.WriteFile(path, content, 0o755))

	b.SetBytes(size)
	b.ResetTimer()

	for range b.N {
		if _, err := hashFile(path); err != nil {
			b.Fatalf("hashFile: %v", err)
		}
	}
}

// countingReader returns zeros until it has returned total bytes, and counts
// what was read from it.
type countingReader struct{ read, total int64 }

func (r *countingReader) Read(p []byte) (int, error) {
	if r.read >= r.total {
		return 0, io.EOF
	}

	n := min(int64(len(p)), r.total-r.read)
	clear(p[:n])
	r.read += n

	return int(n), nil
}

func TestWriteEntry_StopsReadingPastTheLimit(t *testing.T) {
	const limit = 1 << 10

	// The source is much longer than the limit, but finite, so that a missing
	// cap fails the test and does not fill the disk.
	source := &countingReader{total: 64 * limit}
	entry := archiveEntry{name: "checkmate", open: func() (io.ReadCloser, error) { return io.NopCloser(source), nil }}

	_, _, _, err := writeEntry(entry, filepath.Join(t.TempDir(), "checkmate"), limit)

	require.ErrorIs(t, err, ErrInvalidArchive)
	assert.Contains(t, err.Error(), "is larger than the allowed")
	assert.LessOrEqual(t, source.read, int64(limit+1), "the entry is read no further than one byte past the limit")
}
