package skills

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// roundTripFunc lets a plain function satisfy http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// setHTTPClient replaces skillsHTTPClient for the duration of the test.
func setHTTPClient(t *testing.T, fn roundTripFunc) {
	t.Helper()
	orig := skillsHTTPClient
	skillsHTTPClient = &http.Client{Transport: fn}
	t.Cleanup(func() { skillsHTTPClient = orig })
}

// makeZipBytes builds an in-memory ZIP archive from name→content pairs and returns the bytes.
func makeZipBytes(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range entries {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, want, string(data))
}

// zipResponder serves zipData with the given ETag for any request.
func zipResponder(zipData []byte, etag string) roundTripFunc {
	return func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Etag": {etag}},
			Body:       io.NopCloser(bytes.NewReader(zipData)),
		}, nil
	}
}

// --- findExtractedRepoDir ---.

func TestFindExtractedRepoDir(t *testing.T) {
	t.Run("returns the agent-skills-* directory", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "agent-skills-main"), 0o755))
		got, err := findExtractedRepoDir(dir)
		require.NoError(t, err)
		assert.Equal(t, "agent-skills-main", got)
	})

	t.Run("returns error when no matching directory exists", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "some-other-repo"), 0o755))
		_, err := findExtractedRepoDir(dir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "could not find extracted")
	})
}

// --- checkHasSkills ---.

func TestCheckHasSkills(t *testing.T) {
	t.Run("returns error when skills directory is empty", func(t *testing.T) {
		dir := t.TempDir()
		err := checkHasSkills(dir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no skills found")
	})

	t.Run("returns nil when skills directory has at least one entry", func(t *testing.T) {
		skillsDir := t.TempDir()
		skillDir := filepath.Join(skillsDir, "my-skill")
		require.NoError(t, os.MkdirAll(skillDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("x"), 0o644))
		assert.NoError(t, checkHasSkills(skillsDir))
	})

	t.Run("returns error for non-existent directory", func(t *testing.T) {
		err := checkHasSkills(filepath.Join(t.TempDir(), "does-not-exist"))
		require.Error(t, err)
	})
}

// --- DownloadSkills ---.

func TestDownloadSkills(t *testing.T) {
	// The archive root GitHub produces for the main branch, plus the skills subtree path.
	prefix := fmt.Sprintf("agent-skills-main/%s/", pluginSubtreePath)

	t.Run("extracts the skills folder and returns the ETag", func(t *testing.T) {
		zipData := makeZipBytes(t, map[string]string{
			prefix + "auth0/SKILL.md": "# auth0",
		})
		setHTTPClient(t, zipResponder(zipData, `"v1"`))

		skillsDir := filepath.Join(t.TempDir(), "deep", "nested", "skills")
		etag, notModified, err := DownloadSkills(skillsDir, "")
		require.NoError(t, err)
		assert.False(t, notModified)
		assert.Equal(t, `"v1"`, etag)
		assertFileContent(t, filepath.Join(skillsDir, "auth0", "SKILL.md"), "# auth0")
	})

	t.Run("sends If-None-Match and skips on 304", func(t *testing.T) {
		var sentETag string
		setHTTPClient(t, func(r *http.Request) (*http.Response, error) {
			sentETag = r.Header.Get("If-None-Match")
			return &http.Response{StatusCode: http.StatusNotModified, Body: io.NopCloser(strings.NewReader(""))}, nil
		})

		skillsDir := filepath.Join(t.TempDir(), "skills")
		etag, notModified, err := DownloadSkills(skillsDir, `"v1"`)
		require.NoError(t, err)
		assert.True(t, notModified)
		assert.Equal(t, `"v1"`, etag, "prior ETag should be preserved on 304")
		assert.Equal(t, `"v1"`, sentETag, "prior ETag should be sent as If-None-Match")

		// Nothing should have been written on a 304.
		_, statErr := os.Stat(skillsDir)
		assert.True(t, os.IsNotExist(statErr), "skillsDir must not be created on 304")
	})

	t.Run("returns error when download fails", func(t *testing.T) {
		setHTTPClient(t, func(_ *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(""))}, nil
		})
		_, _, err := DownloadSkills(filepath.Join(t.TempDir(), "skills"), "")
		require.Error(t, err)
	})

	t.Run("returns error when archive is missing the skills folder", func(t *testing.T) {
		zipData := makeZipBytes(t, map[string]string{
			"agent-skills-main/README.md": "content",
		})
		setHTTPClient(t, zipResponder(zipData, `"v1"`))

		_, _, err := DownloadSkills(filepath.Join(t.TempDir(), "skills"), "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no skills found")
	})

	t.Run("returns error when archive root is not an agent-skills dir", func(t *testing.T) {
		zipData := makeZipBytes(t, map[string]string{
			"completely-wrong-prefix/file.txt": "content",
		})
		setHTTPClient(t, zipResponder(zipData, `"v1"`))

		_, _, err := DownloadSkills(filepath.Join(t.TempDir(), "skills"), "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "could not find extracted")
	})
}
