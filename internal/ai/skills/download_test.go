package skills

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
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

// --- fetchToTempFile ---.

func TestFetchToTempFile(t *testing.T) {
	t.Run("returns open seeked file and byte count on 200", func(t *testing.T) {
		body := "file content"
		setHTTPClient(t, func(_ *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
		})
		f, size, err := fetchToTempFile("http://example.com/f", "test-*", "test")
		require.NoError(t, err)
		t.Cleanup(func() { f.Close(); os.Remove(f.Name()) })
		assert.Equal(t, int64(len(body)), size)
		data, _ := io.ReadAll(f)
		assert.Equal(t, body, string(data))
	})

	t.Run("returns error on non-200 status", func(t *testing.T) {
		setHTTPClient(t, func(_ *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(""))}, nil
		})
		_, _, err := fetchToTempFile("http://example.com/f", "test-*", "mylabel")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "404")
	})

	t.Run("returns error on request failure", func(t *testing.T) {
		setHTTPClient(t, func(_ *http.Request) (*http.Response, error) {
			return nil, errors.New("connection refused")
		})
		_, _, err := fetchToTempFile("http://example.com/f", "test-*", "mylabel")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "download failed")
	})
}

// --- fetchToTempFile truncation ---.

func TestFetchToTempFile_Truncation(t *testing.T) {
	t.Run("returns error when response body hits size limit", func(t *testing.T) {
		orig := maxSkillsDownload
		maxSkillsDownload = 10
		t.Cleanup(func() { maxSkillsDownload = orig })

		body := strings.Repeat("x", 20)
		setHTTPClient(t, func(_ *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
		})
		_, _, err := fetchToTempFile("http://example.com/f", "test-*", "test")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds size limit")
	})

	t.Run("succeeds when response body is exactly one byte under limit", func(t *testing.T) {
		orig := maxSkillsDownload
		maxSkillsDownload = 10
		t.Cleanup(func() { maxSkillsDownload = orig })

		body := strings.Repeat("x", 9)
		setHTTPClient(t, func(_ *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
		})
		f, size, err := fetchToTempFile("http://example.com/f", "test-*", "test")
		require.NoError(t, err)
		t.Cleanup(func() { f.Close(); os.Remove(f.Name()) })
		assert.Equal(t, int64(9), size)
	})
}

// --- fetchCommitSHA ---.

func TestFetchCommitSHA(t *testing.T) {
	shaResponse := func(sha string) roundTripFunc {
		return func(_ *http.Request) (*http.Response, error) {
			body, _ := json.Marshal(map[string]string{"sha": sha})
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}, nil
		}
	}

	t.Run("returns SHA from valid response", func(t *testing.T) {
		setHTTPClient(t, shaResponse("abc123def456"))
		sha, err := fetchCommitSHA("main")
		require.NoError(t, err)
		assert.Equal(t, "abc123def456", sha)
	})

	t.Run("returns error on non-200 status", func(t *testing.T) {
		setHTTPClient(t, func(_ *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader(""))}, nil
		})
		_, err := fetchCommitSHA("main")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "403")
	})

	t.Run("returns error when SHA field is empty", func(t *testing.T) {
		setHTTPClient(t, shaResponse(""))
		_, err := fetchCommitSHA("main")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty SHA")
	})

	t.Run("returns error on invalid JSON body", func(t *testing.T) {
		setHTTPClient(t, func(_ *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("not json"))}, nil
		})
		_, err := fetchCommitSHA("main")
		require.Error(t, err)
	})

	t.Run("returns error on request failure", func(t *testing.T) {
		setHTTPClient(t, func(_ *http.Request) (*http.Response, error) {
			return nil, errors.New("network error")
		})
		_, err := fetchCommitSHA("main")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "github API request failed")
	})

	t.Run("sends Authorization header when GITHUB_TOKEN is set", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "test-token-xyz")
		var capturedAuth string
		setHTTPClient(t, func(r *http.Request) (*http.Response, error) {
			capturedAuth = r.Header.Get("Authorization")
			body, _ := json.Marshal(map[string]string{"sha": "abc123"})
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}, nil
		})
		_, err := fetchCommitSHA("main")
		require.NoError(t, err)
		assert.Equal(t, "Bearer test-token-xyz", capturedAuth)
	})

	t.Run("omits Authorization header when GITHUB_TOKEN is not set", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "")
		var capturedAuth string
		setHTTPClient(t, func(r *http.Request) (*http.Response, error) {
			capturedAuth = r.Header.Get("Authorization")
			body, _ := json.Marshal(map[string]string{"sha": "abc123"})
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}, nil
		})
		_, err := fetchCommitSHA("main")
		require.NoError(t, err)
		assert.Empty(t, capturedAuth)
	})
}

// --- checkHasSkills ---.

func TestCheckHasSkills(t *testing.T) {
	t.Run("returns error when skills subdirectory is absent", func(t *testing.T) {
		dir := t.TempDir()
		err := checkHasSkills(dir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no skills found")
	})

	t.Run("returns error when skills subdirectory is empty", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "skills"), 0o755))
		err := checkHasSkills(dir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no skills found")
	})

	t.Run("returns nil when skills subdirectory has at least one entry", func(t *testing.T) {
		dir := t.TempDir()
		skillDir := filepath.Join(dir, "skills", "my-skill")
		require.NoError(t, os.MkdirAll(skillDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("x"), 0o644))
		assert.NoError(t, checkHasSkills(dir))
	})

	t.Run("returns error for non-existent directory", func(t *testing.T) {
		err := checkHasSkills(filepath.Join(t.TempDir(), "does-not-exist"))
		require.Error(t, err)
	})
}

// --- downloadViaZip ---.

func makeZipTransport(t *testing.T, zipData []byte, sha string) roundTripFunc {
	t.Helper()
	return func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "github.com" {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(zipData))}, nil
		}
		body, _ := json.Marshal(map[string]string{"sha": sha})
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}, nil
	}
}

func TestDownloadViaZip(t *testing.T) {
	const ref = "main"
	const wantSHA = "cafebabe1234"
	prefix := fmt.Sprintf("auth0-agent-skills-%s/%s/", ref, pluginSubtreePath)

	t.Run("extracts subtree and returns commit SHA", func(t *testing.T) {
		zipData := makeZipBytes(t, map[string]string{
			prefix + "skills/skill-x/SKILL.md": "# skill-x",
		})
		setHTTPClient(t, makeZipTransport(t, zipData, wantSHA))

		dest := t.TempDir()
		gotSHA, err := downloadViaZip(dest, ref)
		require.NoError(t, err)
		assert.Equal(t, wantSHA, gotSHA)
		assertFileContent(t, filepath.Join(dest, "skills", "skill-x", "SKILL.md"), "# skill-x")
	})

	t.Run("returns error when SHA API call fails", func(t *testing.T) {
		setHTTPClient(t, func(r *http.Request) (*http.Response, error) {
			if r.URL.Host == "github.com" {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
			}
			return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader(""))}, nil
		})
		_, err := downloadViaZip(t.TempDir(), ref)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "403")
	})

	t.Run("returns error when download fails", func(t *testing.T) {
		setHTTPClient(t, func(_ *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(""))}, nil
		})
		_, err := downloadViaZip(t.TempDir(), ref)
		require.Error(t, err)
	})

	t.Run("returns error when archive has wrong prefix", func(t *testing.T) {
		zipData := makeZipBytes(t, map[string]string{
			"completely-wrong-prefix/file.txt": "content",
		})
		setHTTPClient(t, makeZipTransport(t, zipData, wantSHA))

		_, err := downloadViaZip(t.TempDir(), ref)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no skills found")
	})

	t.Run("handles slash-containing ref by flattening to dash", func(t *testing.T) {
		const slashRef = "release/1.0"
		const flatRef = "release-1.0"
		prefix := fmt.Sprintf("auth0-agent-skills-%s/%s/", flatRef, pluginSubtreePath)
		zipData := makeZipBytes(t, map[string]string{
			prefix + "skills/skill-y/SKILL.md": "# skill-y",
		})
		setHTTPClient(t, makeZipTransport(t, zipData, wantSHA))

		dest := t.TempDir()
		gotSHA, err := downloadViaZip(dest, slashRef)
		require.NoError(t, err)
		assert.Equal(t, wantSHA, gotSHA)
		assertFileContent(t, filepath.Join(dest, "skills", "skill-y", "SKILL.md"), "# skill-y")
	})
}

// --- DownloadPlugin ---.

func TestDownloadPlugin_EmptyExtraction(t *testing.T) {
	const ref = "main"
	const wantSHA = "abc"

	zipData := makeZipBytes(t, map[string]string{
		"completely-wrong-prefix/file.txt": "content",
	})
	setHTTPClient(t, makeZipTransport(t, zipData, wantSHA))

	base := t.TempDir()
	targetDir := filepath.Join(base, "auth0")
	_, err := DownloadPlugin(targetDir, ref)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no skills found")
}

func TestDownloadPlugin_CreatesMissingTargetDir(t *testing.T) {
	const ref = "main"
	const wantSHA = "abc123"
	prefix := fmt.Sprintf("auth0-agent-skills-%s/%s/", ref, pluginSubtreePath)

	zipData := makeZipBytes(t, map[string]string{
		prefix + "skills/skill-a/SKILL.md": "# skill-a",
	})
	setHTTPClient(t, makeZipTransport(t, zipData, wantSHA))

	targetDir := filepath.Join(t.TempDir(), "deep", "nested", "auth0")
	gotSHA, err := DownloadPlugin(targetDir, ref)
	require.NoError(t, err)
	assert.Equal(t, wantSHA, gotSHA)
	entries, readErr := os.ReadDir(targetDir)
	require.NoError(t, readErr)
	assert.NotEmpty(t, entries, "targetDir must contain extracted files")
}

func TestDownloadPlugin_DefaultsRefToMain(t *testing.T) {
	const wantSHA = "mainsha"
	prefix := fmt.Sprintf("auth0-agent-skills-main/%s/", pluginSubtreePath)

	zipData := makeZipBytes(t, map[string]string{
		prefix + "skills/skill-a/SKILL.md": "# skill-a",
	})

	var capturedURL string
	setHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "github.com" {
			capturedURL = r.URL.String()
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(zipData))}, nil
		}
		body, _ := json.Marshal(map[string]string{"sha": wantSHA})
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}, nil
	})

	targetDir := filepath.Join(t.TempDir(), "auth0")
	gotSHA, err := DownloadPlugin(targetDir, "")
	require.NoError(t, err)
	assert.Equal(t, wantSHA, gotSHA)
	assert.Contains(t, capturedURL, "main", "empty ref should default to main")
}
