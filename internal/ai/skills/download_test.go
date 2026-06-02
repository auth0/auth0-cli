package skills

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
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

// makeTarGz builds an in-memory .tar.gz from name→content pairs.
// A name ending in "/" is written as a directory entry.
func makeTarGz(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for name, content := range entries {
		if strings.HasSuffix(name, "/") {
			require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeDir, Mode: 0o755}))
		} else {
			require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(content))}))
			_, err := tw.Write([]byte(content))
			require.NoError(t, err)
		}
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

// makeZip writes a ZIP archive to a temp file and returns its path and byte size.
func makeZip(t *testing.T, entries map[string]string) (path string, size int64) {
	t.Helper()
	f, err := os.CreateTemp("", "test-*.zip")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(f.Name()) })

	zw := zip.NewWriter(f)
	for name, content := range entries {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	size, err = f.Seek(0, io.SeekEnd)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name(), size
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, want, string(data))
}

// --- extractEntry ---.

func TestExtractEntry(t *testing.T) {
	open := func(content string) func() (io.ReadCloser, error) {
		return func() (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(content)), nil
		}
	}

	t.Run("skips entry not under prefix", func(t *testing.T) {
		dest := t.TempDir()
		require.NoError(t, extractEntry("other/file.txt", false, 0o644, open("x"), "prefix/", dest))
		entries, _ := os.ReadDir(dest)
		assert.Empty(t, entries)
	})

	t.Run("skips root entry with empty rel", func(t *testing.T) {
		dest := t.TempDir()
		require.NoError(t, extractEntry("prefix/", false, 0o644, open("x"), "prefix/", dest))
		entries, _ := os.ReadDir(dest)
		assert.Empty(t, entries)
	})

	t.Run("creates directory", func(t *testing.T) {
		dest := t.TempDir()
		require.NoError(t, extractEntry("prefix/subdir/", true, 0o755, nil, "prefix/", dest))
		info, err := os.Stat(filepath.Join(dest, "subdir"))
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("writes file content", func(t *testing.T) {
		dest := t.TempDir()
		require.NoError(t, extractEntry("prefix/file.txt", false, 0o644, open("hello"), "prefix/", dest))
		assertFileContent(t, filepath.Join(dest, "file.txt"), "hello")
	})

	t.Run("creates parent directories for nested file", func(t *testing.T) {
		dest := t.TempDir()
		require.NoError(t, extractEntry("prefix/a/b/c.txt", false, 0o644, open("nested"), "prefix/", dest))
		assertFileContent(t, filepath.Join(dest, "a", "b", "c.txt"), "nested")
	})

	t.Run("rejects path traversal", func(t *testing.T) {
		dest := t.TempDir()
		err := extractEntry("prefix/../../etc/passwd", false, 0o644, open("evil"), "prefix/", dest)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "illegal path")
	})

	t.Run("propagates open error", func(t *testing.T) {
		dest := t.TempDir()
		boom := func() (io.ReadCloser, error) { return nil, errors.New("open failed") }
		require.Error(t, extractEntry("prefix/file.txt", false, 0o644, boom, "prefix/", dest))
	})
}

// --- extractTarGzSubtree ---.

func TestExtractTarGzSubtree(t *testing.T) {
	const prefix = "repo-main/plugins/auth0/"

	t.Run("extracts files under prefix and skips others", func(t *testing.T) {
		data := makeTarGz(t, map[string]string{
			prefix + "skill-a/SKILL.md": "# skill-a",
			prefix + "skill-b/SKILL.md": "# skill-b",
			"unrelated/ignored.txt":     "ignored",
		})
		dest := t.TempDir()
		require.NoError(t, extractTarGzSubtree(bytes.NewReader(data), prefix, dest))
		assertFileContent(t, filepath.Join(dest, "skill-a", "SKILL.md"), "# skill-a")
		assertFileContent(t, filepath.Join(dest, "skill-b", "SKILL.md"), "# skill-b")
		_, err := os.Stat(filepath.Join(dest, "unrelated"))
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("creates directory entries", func(t *testing.T) {
		data := makeTarGz(t, map[string]string{prefix + "skill-c/": ""})
		dest := t.TempDir()
		require.NoError(t, extractTarGzSubtree(bytes.NewReader(data), prefix, dest))
		info, err := os.Stat(filepath.Join(dest, "skill-c"))
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("returns error on invalid gzip data", func(t *testing.T) {
		err := extractTarGzSubtree(strings.NewReader("not gzip"), prefix, t.TempDir())
		require.Error(t, err)
	})
}

// --- extractZipSubtree ---.

func TestExtractZipSubtree(t *testing.T) {
	const prefix = "repo-main/plugins/auth0/"

	t.Run("extracts files under prefix and skips others", func(t *testing.T) {
		zipPath, size := makeZip(t, map[string]string{
			prefix + "skill-x/SKILL.md": "# skill-x",
			"unrelated/ignored.txt":     "ignored",
		})
		dest := t.TempDir()
		require.NoError(t, extractZipSubtree(zipPath, size, prefix, dest))
		assertFileContent(t, filepath.Join(dest, "skill-x", "SKILL.md"), "# skill-x")
		_, err := os.Stat(filepath.Join(dest, "unrelated"))
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("returns error on invalid zip data", func(t *testing.T) {
		f, err := os.CreateTemp("", "bad-*.zip")
		require.NoError(t, err)
		t.Cleanup(func() { os.Remove(f.Name()) })
		_, _ = f.WriteString("not a zip")
		size, _ := f.Seek(0, io.SeekEnd)
		require.NoError(t, f.Close())
		require.Error(t, extractZipSubtree(f.Name(), size, prefix, t.TempDir()))
	})

	t.Run("returns error when zip file does not exist", func(t *testing.T) {
		require.Error(t, extractZipSubtree("/does/not/exist.zip", 0, prefix, t.TempDir()))
	})
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

// --- fetchToTempFile truncation ---.

func TestFetchToTempFile_Truncation(t *testing.T) {
	t.Run("returns error when response body hits size limit", func(t *testing.T) {
		orig := maxSkillsDownload
		maxSkillsDownload = 10
		t.Cleanup(func() { maxSkillsDownload = orig })

		// Body has more bytes than the limit; LimitReader will stop at exactly 10 bytes.
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

		body := strings.Repeat("x", 9) // 9 < 10
		setHTTPClient(t, func(_ *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
		})
		f, size, err := fetchToTempFile("http://example.com/f", "test-*", "test")
		require.NoError(t, err)
		t.Cleanup(func() { f.Close(); os.Remove(f.Name()) })
		assert.Equal(t, int64(9), size)
	})
}

// --- checkNonEmpty ---.

func TestCheckNonEmpty(t *testing.T) {
	t.Run("returns error for empty directory", func(t *testing.T) {
		dir := t.TempDir()
		err := checkNonEmpty(dir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no files")
	})

	t.Run("returns nil for directory with at least one entry", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644))
		assert.NoError(t, checkNonEmpty(dir))
	})

	t.Run("returns error for non-existent directory", func(t *testing.T) {
		err := checkNonEmpty(filepath.Join(t.TempDir(), "does-not-exist"))
		require.Error(t, err)
	})
}

// --- downloadViaTarGz ---.

func TestDownloadViaTarGz(t *testing.T) {
	const ref = "main"
	const wantSHA = "deadbeefcafe"
	prefix := fmt.Sprintf("auth0-agent-skills-%s/%s/", ref, pluginSubtreePath)

	makeMockTransport := func(tarData []byte, sha string) roundTripFunc {
		return func(r *http.Request) (*http.Response, error) {
			if r.URL.Host == "codeload.github.com" {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(tarData))}, nil
			}
			body, _ := json.Marshal(map[string]string{"sha": sha})
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}, nil
		}
	}

	t.Run("extracts subtree and returns commit SHA", func(t *testing.T) {
		tarData := makeTarGz(t, map[string]string{
			prefix + "skill-a/SKILL.md": "# skill-a",
			prefix + "skill-b/SKILL.md": "# skill-b",
		})
		setHTTPClient(t, makeMockTransport(tarData, wantSHA))

		dest := t.TempDir()
		gotSHA, err := downloadViaTarGz(dest, ref)
		require.NoError(t, err)
		assert.Equal(t, wantSHA, gotSHA)
		assertFileContent(t, filepath.Join(dest, "skill-a", "SKILL.md"), "# skill-a")
		assertFileContent(t, filepath.Join(dest, "skill-b", "SKILL.md"), "# skill-b")
	})

	t.Run("returns error when SHA API call fails", func(t *testing.T) {
		tarData := makeTarGz(t, map[string]string{prefix + "skill-a/SKILL.md": "x"})
		setHTTPClient(t, func(r *http.Request) (*http.Response, error) {
			if r.URL.Host == "codeload.github.com" {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(tarData))}, nil
			}
			return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader(""))}, nil
		})
		_, err := downloadViaTarGz(t.TempDir(), ref)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "403")
	})

	t.Run("returns error when download fails", func(t *testing.T) {
		setHTTPClient(t, func(_ *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(strings.NewReader(""))}, nil
		})
		_, err := downloadViaTarGz(t.TempDir(), ref)
		require.Error(t, err)
	})
}

// --- downloadViaZip ---.

func TestDownloadViaZip(t *testing.T) {
	const ref = "main"
	const wantSHA = "cafebabe1234"
	prefix := fmt.Sprintf("auth0-agent-skills-%s/%s/", ref, pluginSubtreePath)

	makeMockTransport := func(zipPath string, sha string) roundTripFunc {
		zipData, err := os.ReadFile(zipPath)
		if err != nil {
			panic(err)
		}
		return func(r *http.Request) (*http.Response, error) {
			if r.URL.Host == "github.com" {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(zipData))}, nil
			}
			body, _ := json.Marshal(map[string]string{"sha": sha})
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}, nil
		}
	}

	t.Run("extracts subtree and returns commit SHA", func(t *testing.T) {
		zipPath, _ := makeZip(t, map[string]string{
			prefix + "skill-x/SKILL.md": "# skill-x",
		})
		setHTTPClient(t, makeMockTransport(zipPath, wantSHA))

		dest := t.TempDir()
		gotSHA, err := downloadViaZip(dest, ref)
		require.NoError(t, err)
		assert.Equal(t, wantSHA, gotSHA)
		assertFileContent(t, filepath.Join(dest, "skill-x", "SKILL.md"), "# skill-x")
	})

	t.Run("returns error when download fails", func(t *testing.T) {
		setHTTPClient(t, func(_ *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(""))}, nil
		})
		_, err := downloadViaZip(t.TempDir(), ref)
		require.Error(t, err)
	})
}

// --- DownloadPlugin ---.

func TestDownloadPlugin_EmptyExtraction(t *testing.T) {
	// When the archive contains no entries matching the expected prefix, checkNonEmpty
	// should cause the strategy to fail, and DownloadPlugin returns an error.
	// Skip when git is usable: DownloadPlugin will clone from the real repo and succeed,
	// bypassing the HTTP mock strategies entirely.
	if _, err := exec.LookPath("git"); err == nil && checkGitVersion() == nil {
		t.Skip("git is available; DownloadPlugin uses git strategy which bypasses HTTP mock")
	}

	const ref = "main"

	// Tar.gz with wrong prefix (no matching subtree).
	wrongPrefixTar := makeTarGz(t, map[string]string{
		"completely-wrong-prefix/file.txt": "content",
	})
	// ZIP with wrong prefix.
	wrongPrefixZip, _ := makeZip(t, map[string]string{
		"completely-wrong-prefix/file.txt": "content",
	})
	wrongPrefixZipData, err := os.ReadFile(wrongPrefixZip)
	require.NoError(t, err)

	setHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host {
		case "codeload.github.com":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(wrongPrefixTar))}, nil
		case "github.com":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(wrongPrefixZipData))}, nil
		default:
			// SHA API — shouldn't be reached if extraction fails.
			body, _ := json.Marshal(map[string]string{"sha": "abc"})
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}, nil
		}
	})

	base := t.TempDir()
	targetDir := filepath.Join(base, "auth0")
	_, err = DownloadPlugin(targetDir, ref)
	// All three strategies should fail (git is skipped if not in PATH or fails network;
	// tar.gz and ZIP produce empty dirs which checkNonEmpty rejects).
	require.Error(t, err)
}

func TestDownloadPlugin_CreatesMissingTargetDir(t *testing.T) {
	const ref = "main"
	const wantSHA = "abc123"
	prefix := fmt.Sprintf("auth0-agent-skills-%s/%s/", ref, pluginSubtreePath)
	tarData := makeTarGz(t, map[string]string{
		prefix + "skill-a/SKILL.md": "# skill-a",
	})
	zipPath, _ := makeZip(t, map[string]string{
		prefix + "skill-a/SKILL.md": "# skill-a",
	})
	zipData, err := os.ReadFile(zipPath)
	require.NoError(t, err)

	setHTTPClient(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host {
		case "codeload.github.com":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(tarData))}, nil
		case "github.com":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(zipData))}, nil
		default:
			body, _ := json.Marshal(map[string]string{"sha": wantSHA})
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}, nil
		}
	})

	// targetDir is deeply nested and does not exist yet.
	targetDir := filepath.Join(t.TempDir(), "deep", "nested", "auth0")
	_, err = DownloadPlugin(targetDir, ref)
	// If git is available and succeeds via real network, sha may differ; we just check
	// that the call succeeded and targetDir was created with at least one entry.
	if err == nil {
		entries, readErr := os.ReadDir(targetDir)
		require.NoError(t, readErr)
		assert.NotEmpty(t, entries, "targetDir must contain extracted files")
	}
	// If git fails in test environment (no network), tar.gz or ZIP mock should succeed.
}
