package plugins

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// defaultRegistryBaseURL is where the signed registry index is fetched from.
//
// NOTE: this points at the public auth0/auth0-cli-plugins repo, which only
// serves anonymous raw-main fetches once the repo is public. While the registry
// lives in the private atko-cic repo, tests and local runs override the base URL
// (and the trusted key) via NewRegistryWith.
const defaultRegistryBaseURL = "https://raw.githubusercontent.com/auth0/auth0-cli-plugins/main"

const (
	indexFileName     = "index.json"
	signatureFileName = "index.json.sig"
)

// InstallType identifies how a plugin's artifact is fetched and run.
type InstallType string

const (
	// InstallNPM runs the plugin through npx from a pinned npm package version.
	InstallNPM InstallType = "npm"
	// InstallGitHubRelease downloads a pinned, checksum-verified binary asset
	// from a GitHub release, selected per OS/arch.
	InstallGitHubRelease InstallType = "github-release"
)

// Index is the top-level registry document listing every available plugin.
type Index struct {
	Version int      `json:"version"`
	Plugins []Plugin `json:"plugins"`
}

// Plugin describes a single installable plugin.
type Plugin struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Install        Install  `json:"install"`
	RequiredScopes []string `json:"required_scopes,omitempty"`
}

// Install captures the artifact source for a plugin. The relevant fields depend
// on Type: NPM uses Package/Version, GitHubRelease uses Repo/Version/Assets.
type Install struct {
	Type    InstallType    `json:"type"`
	Package string         `json:"package,omitempty"`
	Repo    string         `json:"repo,omitempty"`
	Version string         `json:"version"`
	Assets  []ReleaseAsset `json:"assets,omitempty"`
}

// ReleaseAsset is a single per-platform binary published in a GitHub release.
type ReleaseAsset struct {
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// Registry fetches and validates the signed plugin index. The base URL and
// trusted public key are injectable so the client can be pointed at a local
// fixture (with a test key) while the registry is private.
type Registry struct {
	baseURL string
	pubKey  ed25519.PublicKey
	client  *http.Client
}

// registryURLEnvVar overrides the registry base URL. It exists so the CLI can
// be pointed at the registry while it lives in a private repo (served locally
// or via a fixture). The embedded public key still verifies the signature, so
// an overridden URL cannot serve an index this binary would trust unless it was
// signed with the production key.
const registryURLEnvVar = "AUTH0_CLI_PLUGINS_REGISTRY_URL"

// NewRegistry returns a Registry configured against the production index URL and
// the public key embedded in the binary. The base URL can be overridden with
// AUTH0_CLI_PLUGINS_REGISTRY_URL for the private-repo phase.
func NewRegistry() (*Registry, error) {
	pubKey, err := parsePublicKey(embeddedRegistryPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load embedded registry public key: %w", err)
	}

	baseURL := defaultRegistryBaseURL
	if override := os.Getenv(registryURLEnvVar); override != "" {
		baseURL = override
	}

	return NewRegistryWith(baseURL, pubKey), nil
}

// NewRegistryWith returns a Registry pointed at an arbitrary base URL and trust
// anchor. It is used by tests and while the registry index is served from a
// private location that requires a fixture/injectable source.
func NewRegistryWith(baseURL string, pubKey ed25519.PublicKey) *Registry {
	return &Registry{
		baseURL: strings.TrimRight(baseURL, "/"),
		pubKey:  pubKey,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Fetch downloads the index and its detached signature, verifies the signature
// against the trusted key, and returns the parsed index. A tampered or unsigned
// index is rejected before it is ever parsed for install decisions.
func (r *Registry) Fetch(ctx context.Context) (*Index, error) {
	indexBytes, err := r.get(ctx, indexFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch registry index: %w", err)
	}

	signature, err := r.get(ctx, signatureFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch registry signature: %w", err)
	}

	if err := verifyIndexSignature(indexBytes, signature, r.pubKey); err != nil {
		return nil, err
	}

	var index Index
	if err := json.Unmarshal(indexBytes, &index); err != nil {
		return nil, fmt.Errorf("failed to parse registry index: %w", err)
	}

	return &index, nil
}

// get performs a single GET against the registry base URL and returns the body,
// erroring on any non-200 response.
func (r *Registry) get(ctx context.Context, name string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+"/"+name, nil)
	if err != nil {
		return nil, err
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d fetching %s", resp.StatusCode, name)
	}

	return io.ReadAll(resp.Body)
}

// Plugin returns the named plugin from the index, if present.
func (i *Index) Plugin(name string) (Plugin, bool) {
	for _, p := range i.Plugins {
		if p.Name == name {
			return p, true
		}
	}
	return Plugin{}, false
}
