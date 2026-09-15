package plugins

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// InstallDir is where github-release plugin binaries are stored, alongside the
// CLI config directory (~/.config/auth0/plugins).
func InstallDir() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "auth0", "plugins")
}

// NpmSpec returns the pinned npx spec ("package@version") for an npm plugin.
func NpmSpec(p Plugin) string {
	return p.Install.Package + "@" + p.Install.Version
}

// SelectAsset returns the release asset matching the given OS/arch, if the
// plugin publishes one. Callers pass runtime.GOOS / runtime.GOARCH.
func SelectAsset(p Plugin, goos, goarch string) (ReleaseAsset, bool) {
	for _, asset := range p.Install.Assets {
		if asset.OS == goos && asset.Arch == goarch {
			return asset, true
		}
	}
	return ReleaseAsset{}, false
}

// downloadAndVerify fetches url, verifies its SHA-256 against expectedHex, and
// writes it to destPath with executable permissions only after verification
// succeeds. The download is staged in a temp file so a failed or tampered
// download never leaves a partial binary at destPath.
func downloadAndVerify(ctx context.Context, client *http.Client, url, expectedHex, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d downloading %s", resp.StatusCode, url)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read download body: %w", err)
	}

	if err := verifyChecksum(data, expectedHex); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o700); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(destPath), ".download-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // No-op once renamed.

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return err
	}

	return os.Rename(tmpName, destPath)
}

// newDownloadClient returns the HTTP client used for artifact downloads.
func newDownloadClient() *http.Client {
	return &http.Client{Timeout: 5 * time.Minute}
}

// InstallPlugin materializes a plugin for the current host and returns the
// record to persist. For npm plugins it resolves the pinned npx spec (the package is
// fetched lazily by npx at run time). For github-release plugins it downloads
// and checksum-verifies the binary for the current OS/arch. It does not touch
// the store; callers persist the returned record.
func InstallPlugin(ctx context.Context, p Plugin) (InstalledPlugin, error) {
	switch p.Install.Type {
	case InstallNPM:
		if p.Install.Package == "" || p.Install.Version == "" {
			return InstalledPlugin{}, fmt.Errorf("plugin %q is missing an npm package or version", p.Name)
		}
		return InstalledPlugin{
			Name:           p.Name,
			Version:        p.Install.Version,
			InstallType:    InstallNPM,
			Description:    p.Description,
			Package:        NpmSpec(p),
			RequiredScopes: p.RequiredScopes,
		}, nil
	case InstallGitHubRelease:
		asset, ok := SelectAsset(p, runtime.GOOS, runtime.GOARCH)
		if !ok {
			return InstalledPlugin{}, fmt.Errorf(
				"plugin %q has no release asset for %s/%s", p.Name, runtime.GOOS, runtime.GOARCH)
		}
		dest := filepath.Join(InstallDir(), p.Name, p.Name)
		if err := downloadAndVerify(ctx, newDownloadClient(), asset.URL, asset.SHA256, dest); err != nil {
			return InstalledPlugin{}, err
		}
		return InstalledPlugin{
			Name:           p.Name,
			Version:        p.Install.Version,
			InstallType:    InstallGitHubRelease,
			Description:    p.Description,
			BinaryPath:     dest,
			RequiredScopes: p.RequiredScopes,
		}, nil
	default:
		return InstalledPlugin{}, fmt.Errorf("unsupported install type %q for plugin %q", p.Install.Type, p.Name)
	}
}
