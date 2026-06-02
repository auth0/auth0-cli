package skills

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	agentSkillsRepo   = "https://github.com/auth0/agent-skills"
	agentSkillsAPI    = "https://api.github.com/repos/auth0/agent-skills/commits/"
	pluginSubtreePath = "plugins/auth0"
	skillsHTTPTimeout = 60 * time.Second
	gitCmdTimeout     = 120 * time.Second
	minGitMajor       = 2
	minGitMinor       = 25
)

// maxSkillsDownload is the per-archive byte limit for HTTP downloads. Declared as a var so
// tests can override it without allocating a 100 MB body.
var maxSkillsDownload int64 = 100 * 1024 * 1024 // 100 MB.

var skillsHTTPClient = &http.Client{Timeout: skillsHTTPTimeout}

// DownloadPlugin downloads the auth0 agent-skills plugin into targetDir using the best
// available strategy: git sparse-checkout > tar.gz > ZIP. Returns the commit SHA.
func DownloadPlugin(targetDir, ref string) (string, error) {
	if ref == "" {
		ref = "main"
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("create target dir: %w", err)
	}

	if _, err := exec.LookPath("git"); err == nil && checkGitVersion() == nil {
		if sha, err := downloadViaGit(targetDir, ref); err == nil {
			return sha, checkNonEmpty(targetDir)
		}
	}

	if sha, err := downloadViaTarGz(targetDir, ref); err == nil {
		return sha, checkNonEmpty(targetDir)
	}

	sha, err := downloadViaZip(targetDir, ref)
	if err != nil {
		return "", err
	}
	return sha, checkNonEmpty(targetDir)
}

// checkNonEmpty returns an error if dir contains no entries.
func checkNonEmpty(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("check extraction result: %w", err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("extraction produced no files in %s (archive prefix may not match)", dir)
	}
	return nil
}

// checkGitVersion returns an error if git is not found or is older than 2.25.
func checkGitVersion() error {
	out, err := exec.Command("git", "--version").Output()
	if err != nil {
		return fmt.Errorf("git --version: %w", err)
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) < 3 {
		return fmt.Errorf("unexpected git --version output: %s", strings.TrimSpace(string(out)))
	}
	vParts := strings.SplitN(fields[2], ".", 3)
	if len(vParts) < 2 {
		return fmt.Errorf("cannot parse git version: %s", fields[2])
	}
	major, err1 := strconv.Atoi(vParts[0])
	minor, err2 := strconv.Atoi(vParts[1])
	if err1 != nil || err2 != nil {
		return fmt.Errorf("cannot parse git version: %s", fields[2])
	}
	if major < minGitMajor || (major == minGitMajor && minor < minGitMinor) {
		return fmt.Errorf("git >= %d.%d required, found %s", minGitMajor, minGitMinor, fields[2])
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

// downloadViaGit uses git sparse-checkout to download only plugins/auth0 directly into targetDir.
func downloadViaGit(targetDir, ref string) (string, error) {
	run := func(args ...string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), gitCmdTimeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = targetDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			if ctx.Err() != nil {
				return "", fmt.Errorf("git %s: timed out after %s", strings.Join(args, " "), gitCmdTimeout)
			}
			return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out)), nil
	}

	if _, err := run("clone", "--no-checkout", "--depth", "1", "--filter=blob:none", "--branch", ref,
		agentSkillsRepo+".git", "."); err != nil {
		return "", err
	}

	if _, err := run("sparse-checkout", "set", pluginSubtreePath); err != nil {
		return "", err
	}

	if _, err := run("checkout"); err != nil {
		return "", err
	}

	return run("rev-parse", "HEAD")
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

// ExtractEntry writes a single archive entry to destDir. IsDir and mode describe the entry;
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
	// Zip.NewReader needs an io.ReaderAt, so we re-open the file.
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

