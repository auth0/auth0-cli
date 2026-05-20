package skills

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	agentSkillsRepo   = "https://github.com/auth0/agent-skills"
	agentSkillsAPI    = "https://api.github.com/repos/auth0/agent-skills/commits/"
	pluginSubtreePath = "plugins/auth0"
	skillsHTTPTimeout = 60 * time.Second
	maxSkillsDownload = 100 * 1024 * 1024 // 100 MB
)

var skillsHTTPClient = &http.Client{Timeout: skillsHTTPTimeout}

// DownloadPlugin downloads the auth0 agent-skills plugin into targetDir using the best
// available strategy: git sparse-checkout > tar.gz > ZIP. Returns the commit SHA.
func DownloadPlugin(targetDir, ref string) (string, error) {
	if ref == "" {
		ref = "main"
	}

	if _, err := exec.LookPath("git"); err == nil {
		sha, err := downloadViaGit(targetDir, ref)
		if err == nil {
			return sha, nil
		}
	}

	sha, err := downloadViaTarGz(targetDir, ref)
	if err == nil {
		return sha, nil
	}

	return downloadViaZip(targetDir, ref)
}

// fetchCommitSHA fetches the latest commit SHA for ref from the GitHub API.
func fetchCommitSHA(ref string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, agentSkillsAPI+ref, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

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

// downloadViaGit uses git sparse-checkout to download only plugins/auth0.
func downloadViaGit(targetDir, ref string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "auth0-agent-skills-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	run := func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out)), nil
	}

	if _, err := run("clone", "--no-checkout", "--depth", "1", "--filter=blob:none",
		agentSkillsRepo+".git", "."); err != nil {
		return "", err
	}

	if _, err := run("sparse-checkout", "set", pluginSubtreePath); err != nil {
		return "", err
	}

	if _, err := run("checkout"); err != nil {
		return "", err
	}

	sha, err := run("rev-parse", "HEAD")
	if err != nil {
		return "", err
	}

	srcDir := filepath.Join(tmpDir, pluginSubtreePath)
	if err := mergeDir(srcDir, targetDir); err != nil {
		return "", err
	}

	return sha, nil
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

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return nil, 0, err
	}

	return f, size, nil
}

// downloadViaTarGz downloads the archive from codeload.github.com and extracts the subtree.
func downloadViaTarGz(targetDir, ref string) (string, error) {
	url := fmt.Sprintf("https://codeload.github.com/auth0/agent-skills/tar.gz/refs/heads/%s", ref)
	f, _, err := fetchToTempFile(url, "auth0-agent-skills-*.tar.gz", "tar.gz")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	defer f.Close()

	prefix := fmt.Sprintf("auth0-agent-skills-%s/%s/", ref, pluginSubtreePath)
	if err := extractTarGzSubtree(f, prefix, targetDir); err != nil {
		return "", err
	}

	return fetchCommitSHA(ref)
}

// downloadViaZip downloads the ZIP archive from github.com and extracts the subtree.
func downloadViaZip(targetDir, ref string) (string, error) {
	url := fmt.Sprintf("%s/archive/refs/heads/%s.zip", agentSkillsRepo, ref)
	f, size, err := fetchToTempFile(url, "auth0-agent-skills-*.zip", "ZIP")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	defer f.Close()

	prefix := fmt.Sprintf("auth0-agent-skills-%s/%s/", ref, pluginSubtreePath)
	if err := extractZipSubtree(f.Name(), size, prefix, targetDir); err != nil {
		return "", err
	}

	return fetchCommitSHA(ref)
}

// extractTarGzSubtree reads a .tar.gz from r and copies entries whose name starts with
// prefix into destDir (stripping the prefix from the output path).
// extractEntry writes a single archive entry to destDir. isDir and mode describe the entry;
// open returns a reader for its content (ignored when isDir is true). The name is checked
// against prefix and any path-traversal attempt is rejected.
func extractEntry(name string, isDir bool, mode os.FileMode, open func() (io.ReadCloser, error), prefix, destDir string) error {
	if !strings.HasPrefix(name, prefix) {
		return nil
	}
	rel := strings.TrimPrefix(name, prefix)
	if rel == "" {
		return nil
	}
	dest := filepath.Join(destDir, filepath.FromSlash(rel))
	if !strings.HasPrefix(dest, filepath.Clean(destDir)+string(os.PathSeparator)) {
		return fmt.Errorf("illegal path in archive: %s", name)
	}
	if isDir {
		return os.MkdirAll(dest, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	rc, err := open()
	if err != nil {
		return err
	}
	outFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		_ = rc.Close()
		return err
	}
	_, copyErr := io.Copy(outFile, rc)
	_ = rc.Close()
	_ = outFile.Close()
	return copyErr
}

// extractTarGzSubtree reads a .tar.gz from r and copies entries whose name starts with
// prefix into destDir (stripping the prefix from the output path).
func extractTarGzSubtree(r io.Reader, prefix, destDir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip open: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}
		if err := extractEntry(hdr.Name, hdr.Typeflag == tar.TypeDir, hdr.FileInfo().Mode(),
			func() (io.ReadCloser, error) { return io.NopCloser(tr), nil },
			prefix, destDir); err != nil {
			return err
		}
	}
	return nil
}

// extractZipSubtree opens the ZIP at zipPath (with known size) and copies entries whose
// name starts with prefix into destDir (stripping the prefix).
func extractZipSubtree(zipPath string, size int64, prefix, destDir string) error {
	// zip.NewReader needs an io.ReaderAt, so we re-open the file.
	f, err := os.Open(zipPath)
	if err != nil {
		return err
	}
	defer f.Close()

	zr, err := zip.NewReader(f, size)
	if err != nil {
		return fmt.Errorf("zip open: %w", err)
	}

	for _, entry := range zr.File {
		if err := extractEntry(entry.Name, entry.FileInfo().IsDir(), entry.Mode(),
			entry.Open, prefix, destDir); err != nil {
			return err
		}
	}
	return nil
}

// mergeDir copies all files from src into dst, creating directories as needed.
func mergeDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}

		in, err := os.Open(path)
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
		if err != nil {
			_ = in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		_ = in.Close()
		_ = out.Close()
		return copyErr
	})
}
