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

// agentSkillsGitURL is the URL used by downloadViaGit. Declared as a var so tests can
// point it at a local bare repository instead of the real GitHub remote.
var agentSkillsGitURL = agentSkillsRepo + ".git"

// gitLookPath is exec.LookPath by default; tests override it to force HTTP fallback strategies.
var gitLookPath = exec.LookPath

var skillsHTTPClient = &http.Client{Timeout: skillsHTTPTimeout}

// DownloadPlugin downloads the auth0 agent-skills plugin into targetDir using the best
// available strategy: git sparse-checkout > tar.gz > ZIP. Returns the commit SHA.
// All intermediate work happens in a system temp directory; targetDir is only written
// once everything succeeds.
func DownloadPlugin(targetDir, ref string) (string, error) {
	if ref == "" {
		ref = "main"
	}

	tmpDir, err := os.MkdirTemp("", "auth0-skills-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir) // No-op if renamed to targetDir below.

	sha, dlErr := func() (string, error) {
		var errs []string
		if _, err := gitLookPath("git"); err == nil && checkGitVersion() == nil {
			sha, err := downloadViaGit(tmpDir, ref)
			if err == nil {
				return sha, nil
			}
			errs = append(errs, "git: "+err.Error())
		}
		sha, err := downloadViaTarGz(tmpDir, ref)
		if err == nil {
			return sha, nil
		}
		errs = append(errs, "tar.gz: "+err.Error())
		sha, err = downloadViaZip(tmpDir, ref)
		if err == nil {
			return sha, nil
		}
		errs = append(errs, "zip: "+err.Error())
		return "", fmt.Errorf("all download strategies failed: %s", strings.Join(errs, "; "))
	}()
	if dlErr != nil {
		return "", dlErr
	}

	if err := checkHasSkills(tmpDir); err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
		return "", fmt.Errorf("create parent dir: %w", err)
	}

	os.RemoveAll(targetDir)

	// Attempt atomic rename (succeeds when /tmp and targetDir share a filesystem).
	if err := os.Rename(tmpDir, targetDir); err == nil {
		return sha, nil
	}

	// Cross-filesystem fallback: copy content into a freshly created targetDir.
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("create target dir: %w", err)
	}
	if err := mergeDir(tmpDir, targetDir); err != nil {
		return "", fmt.Errorf("install to target dir: %w", err)
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

// checkGitVersion returns an error if git is not found or is older than 2.25.
func checkGitVersion() error {
	ctx, cancel := context.WithTimeout(context.Background(), gitCmdTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "--version").Output()
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

// downloadViaGit clones into a temp directory, then promotes the contents of
// plugins/auth0/ into targetDir so the layout matches the tar.gz/ZIP strategies.
func downloadViaGit(targetDir, ref string) (string, error) {
	cloneDir, err := os.MkdirTemp("", "auth0-agent-skills-git-*")
	if err != nil {
		return "", fmt.Errorf("create git clone dir: %w", err)
	}
	defer os.RemoveAll(cloneDir)

	run := func(args ...string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), gitCmdTimeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = cloneDir
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
		agentSkillsGitURL, "."); err != nil {
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

	// Promote plugins/auth0/ into targetDir (the DownloadPlugin temp directory) by rename.
	// Remove targetDir first so the rename can take its place.
	subtreeSrc := filepath.Join(cloneDir, filepath.FromSlash(pluginSubtreePath))
	if err := os.RemoveAll(targetDir); err != nil {
		return "", fmt.Errorf("clear temp dir for promotion: %w", err)
	}
	if err := os.Rename(subtreeSrc, targetDir); err != nil {
		// Restore the empty dir so fallback strategies in the caller can still use this path.
		_ = os.MkdirAll(targetDir, 0o755)
		return "", fmt.Errorf("promote git subtree: %w", err)
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

// downloadViaTarGz fetches the commit SHA first, then downloads and extracts the tar.gz archive.
func downloadViaTarGz(targetDir, ref string) (string, error) {
	sha, err := fetchCommitSHA(ref)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://codeload.github.com/auth0/agent-skills/tar.gz/%s", ref)
	f, _, err := fetchToTempFile(url, "auth0-agent-skills-*.tar.gz", "tar.gz")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	defer f.Close()

	// GitHub flattens "/" in ref names to "-" in archive root directory names.
	archiveRef := strings.ReplaceAll(ref, "/", "-")
	prefix := fmt.Sprintf("auth0-agent-skills-%s/%s/", archiveRef, pluginSubtreePath)
	if err := extractTarGzSubtree(f, prefix, targetDir); err != nil {
		return "", err
	}

	return sha, nil
}

// downloadViaZip fetches the commit SHA first, then downloads and extracts the ZIP archive.
func downloadViaZip(targetDir, ref string) (string, error) {
	sha, err := fetchCommitSHA(ref)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/archive/%s.zip", agentSkillsRepo, ref)
	f, size, err := fetchToTempFile(url, "auth0-agent-skills-*.zip", "ZIP")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	defer f.Close()

	// GitHub flattens "/" in ref names to "-" in archive root directory names.
	archiveRef := strings.ReplaceAll(ref, "/", "-")
	prefix := fmt.Sprintf("auth0-agent-skills-%s/%s/", archiveRef, pluginSubtreePath)
	if err := extractZipSubtree(f.Name(), size, prefix, targetDir); err != nil {
		return "", err
	}

	return sha, nil
}

// mergeDir recursively copies the contents of src into dst. Symlinks are preserved
// (not dereferenced) so the layout matches what git sparse-checkout produces.
func mergeDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		switch {
		case entry.Type()&os.ModeSymlink != 0:
			target, err := os.Readlink(srcPath)
			if err != nil {
				return err
			}
			if err := os.Symlink(target, dstPath); err != nil {
				return err
			}
		case entry.IsDir():
			if err := os.MkdirAll(dstPath, 0o755); err != nil {
				return err
			}
			if err := mergeDir(srcPath, dstPath); err != nil {
				return err
			}
		default:
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if err := copyFile(srcPath, dstPath, info.Mode()); err != nil {
				return err
			}
		}
	}
	return nil
}

// copyFile copies src to dst with the given permission mode.
func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
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
