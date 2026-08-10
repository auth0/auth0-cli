package skills

import (
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
	agentSkillsRepo = "https://github.com/auth0/agent-skills"

	// PluginSubtreePath is the path, within the repo, to the skills folder we install:
	// https://github.com/auth0/agent-skills/tree/main/plugins/auth0/skills
	pluginSubtreePath = "plugins/auth0/skills"

	skillsHTTPTimeout = 60 * time.Second
)

var skillsHTTPClient = &http.Client{Timeout: skillsHTTPTimeout}

// DownloadSkills installs the auth0 skills folder into skillsDir, skipping the download
// when prevETag still matches the server (notModified=true) and returning the new ETag otherwise.
func DownloadSkills(skillsDir, prevETag string) (etag string, notModified bool, err error) {
	zipFile, etag, notModified, err := downloadArchive(prevETag)
	if err != nil {
		return "", false, err
	}
	if notModified {
		return prevETag, true, nil
	}
	defer os.Remove(zipFile)

	tempUnzipDir, err := os.MkdirTemp("", "auth0-agent-skills-*")
	if err != nil {
		return "", false, fmt.Errorf("create unzip dir: %w", err)
	}
	defer os.RemoveAll(tempUnzipDir)

	if err := utils.Unzip(zipFile, tempUnzipDir); err != nil {
		return "", false, fmt.Errorf("unzip archive: %w", err)
	}

	extractedDir, err := findExtractedRepoDir(tempUnzipDir)
	if err != nil {
		return "", false, err
	}

	skillsSrc := filepath.Join(tempUnzipDir, extractedDir, filepath.FromSlash(pluginSubtreePath))
	if err := checkHasSkills(skillsSrc); err != nil {
		return "", false, err
	}

	if err := replaceDir(skillsSrc, skillsDir); err != nil {
		return "", false, err
	}

	return etag, false, nil
}

// downloadArchive does a conditional GET for the archive: 304 returns notModified=true;
// otherwise it saves the archive to a temp file (caller must remove) and returns its path and ETag.
func downloadArchive(prevETag string) (zipFile, etag string, notModified bool, err error) {
	url := fmt.Sprintf("%s/archive/refs/heads/main.zip", agentSkillsRepo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", "", false, err
	}
	if prevETag != "" {
		req.Header.Set("If-None-Match", prevETag)
	}

	resp, err := skillsHTTPClient.Do(req)
	if err != nil {
		return "", "", false, fmt.Errorf("download archive failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return "", "", true, nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", false, fmt.Errorf("download archive returned status %d", resp.StatusCode)
	}

	f, err := os.CreateTemp("", "auth0-agent-skills-*.zip")
	if err != nil {
		return "", "", false, err
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = os.Remove(f.Name())
		return "", "", false, fmt.Errorf("failed to save archive: %w", err)
	}

	return f.Name(), resp.Header.Get("ETag"), false, nil
}

// findExtractedRepoDir returns the "agent-skills-<ref>" archive root inside tempUnzipDir.
func findExtractedRepoDir(tempUnzipDir string) (string, error) {
	entries, err := os.ReadDir(tempUnzipDir)
	if err != nil {
		return "", fmt.Errorf("failed to read temp directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "agent-skills-") {
			return entry.Name(), nil
		}
	}

	return "", fmt.Errorf("could not find extracted agent-skills directory")
}

// checkHasSkills returns an error if skillsDir does not exist or contains no entries.
func checkHasSkills(skillsDir string) error {
	entries, err := os.ReadDir(skillsDir)
	if err != nil || len(entries) == 0 {
		return fmt.Errorf("no skills found under %s (archive layout may have changed)", skillsDir)
	}
	return nil
}

// replaceDir replaces skillsDir with src via an atomic rename, falling back to a
// recursive copy when they are on different filesystems.
func replaceDir(src, skillsDir string) error {
	if err := os.MkdirAll(filepath.Dir(skillsDir), 0o755); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}

	os.RemoveAll(skillsDir)

	if err := os.Rename(src, skillsDir); err != nil {
		// Cross-filesystem fallback: copy content into a freshly created skillsDir.
		if err := os.MkdirAll(skillsDir, 0o755); err != nil {
			return fmt.Errorf("create target dir: %w", err)
		}
		if err := copyTree(src, skillsDir); err != nil {
			return fmt.Errorf("install to target dir: %w", err)
		}
	}

	return nil
}
