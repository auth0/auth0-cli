package skills

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/auth0/auth0-cli/internal/utils"
)

const (
	agentSkillsRepo   = "https://github.com/auth0/agent-skills"
	agentSkillsAPI    = "https://api.github.com/repos/auth0/agent-skills/commits/"
	pluginSubtreePath = "plugins/auth0"
	skillsHTTPTimeout = 60 * time.Second
)

// maxSkillsDownload is the per-archive byte limit for HTTP downloads. Declared as a var so
// tests can override it without allocating a 100 MB body.
var maxSkillsDownload int64 = 100 * 1024 * 1024 // 100 MB.

var skillsHTTPClient = &http.Client{Timeout: skillsHTTPTimeout}

// DownloadPlugin downloads the auth0 agent-skills plugin into targetDir via ZIP.
// Returns the commit SHA. TargetDir is only written once everything succeeds.
func DownloadPlugin(targetDir, ref string) (string, error) {
	if ref == "" {
		ref = "main"
	}
	return downloadViaZip(targetDir, ref)
}

// downloadViaZip fetches the commit SHA first, downloads and extracts the ZIP archive,
// then promotes the plugins/auth0 subtree into targetDir.
func downloadViaZip(targetDir, ref string) (string, error) {
	sha, err := fetchCommitSHA(ref)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/archive/%s.zip", agentSkillsRepo, ref)
	f, _, err := fetchToTempFile(url, "auth0-agent-skills-*.zip", "ZIP")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	defer f.Close()

	tmpUnzipDir, err := os.MkdirTemp("", "auth0-skills-unzip-*")
	if err != nil {
		return "", fmt.Errorf("create unzip dir: %w", err)
	}
	defer os.RemoveAll(tmpUnzipDir)

	if err := utils.Unzip(f.Name(), tmpUnzipDir); err != nil {
		return "", fmt.Errorf("unzip plugin: %w", err)
	}

	// GitHub flattens "/" in ref names to "-" in archive root directory names.
	archiveRef := strings.ReplaceAll(ref, "/", "-")
	subtreeSrc := filepath.Join(tmpUnzipDir, "auth0-agent-skills-"+archiveRef, filepath.FromSlash(pluginSubtreePath))

	if err := checkHasSkills(subtreeSrc); err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
		return "", fmt.Errorf("create parent dir: %w", err)
	}

	os.RemoveAll(targetDir)

	// Attempt atomic rename (succeeds when tmpUnzipDir and targetDir share a filesystem).
	if err := os.Rename(subtreeSrc, targetDir); err != nil {
		// Cross-filesystem fallback: copy content into a freshly created targetDir.
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return "", fmt.Errorf("create target dir: %w", err)
		}
		if err := mergeDir(subtreeSrc, targetDir); err != nil {
			return "", fmt.Errorf("install to target dir: %w", err)
		}
	}

	return sha, nil
}

// checkHasSkills returns an error if dir/skills/ does not exist or contains no entries.
func checkHasSkills(dir string) error {
	entries, err := os.ReadDir(filepath.Join(dir, "skills"))
	if err != nil || len(entries) == 0 {
		return fmt.Errorf("no skills found under %s/skills/ (archive prefix may not match)", dir)
	}
	return nil
}

// fetchCommitSHA fetches the latest commit SHA for ref from the GitHub API.
func fetchCommitSHA(ref string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, agentSkillsAPI+ref, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := skillsHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("github API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github API returned status %d", resp.StatusCode)
	}

	var payload struct {
		SHA string `json:"sha"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1024*1024)).Decode(&payload); err != nil {
		return "", fmt.Errorf("failed to decode github API response: %w", err)
	}
	if payload.SHA == "" {
		return "", fmt.Errorf("github API returned empty SHA")
	}
	return payload.SHA, nil
}

// fetchToTempFile downloads url into a new temp file and returns it open and seeked to the
// start, along with the number of bytes written. The caller is responsible for closing and
// removing the file.
func fetchToTempFile(url, pattern, label string) (*os.File, int64, error) {
	resp, err := skillsHTTPClient.Get(url)
	if err != nil {
		return nil, 0, fmt.Errorf("%s download failed: %w", label, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("%s download returned status %d", label, resp.StatusCode)
	}

	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return nil, 0, err
	}

	size, err := io.Copy(f, io.LimitReader(resp.Body, maxSkillsDownload))
	if err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return nil, 0, fmt.Errorf("failed to save %s: %w", label, err)
	}

	if size == maxSkillsDownload {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return nil, 0, fmt.Errorf("%s: archive exceeds size limit of %d bytes", label, maxSkillsDownload)
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return nil, 0, err
	}

	return f, size, nil
}
