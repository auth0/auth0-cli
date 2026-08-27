package openapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/auth0/auth0-cli/internal/buildinfo"
)

const (
	// SchemaURL is the URL to the Auth0 Management API OpenAPI schema.
	SchemaURL = "https://auth0.com/docs/oas/management/v2/management-api-oas.json"

	// CacheTTL is how long to cache the schema before re-fetching.
	CacheTTL = 24 * time.Hour

	// Bound the schema fetch so a slow or unreachable host cannot hang the CLI.
	schemaHTTPTimeout = 30 * time.Second
)

// schemaHTTPClient fetches the OpenAPI schema with an explicit timeout, matching
// the convention used elsewhere for ad-hoc external fetches (see auth0.quickstartHTTPClient).
var schemaHTTPClient = &http.Client{Timeout: schemaHTTPTimeout}

var (
	globalDoc *openapi3.T
	cachedAt  time.Time
)

// GetDoc returns the OpenAPI document, serving a fresh copy (in-memory or on-disk,
// <CacheTTL) without a network call and falling back to a stale copy if a fetch fails.
func GetDoc() (*openapi3.T, error) {
	if globalDoc != nil && time.Since(cachedAt) < CacheTTL {
		return globalDoc, nil
	}

	// A fresh on-disk copy avoids the network; a stale one is kept as a fallback.
	cachedDoc, fresh, cacheErr := loadCachedDoc()
	if cacheErr == nil && fresh {
		return cacheInMemory(cachedDoc), nil
	}

	doc, err := fetchDoc()
	if err != nil {
		// Offline fallback: serve a stale copy rather than failing.
		if globalDoc != nil {
			return globalDoc, nil
		}
		if cacheErr == nil {
			return cacheInMemory(cachedDoc), nil
		}
		return nil, fmt.Errorf("failed to fetch OpenAPI schema: %w", err)
	}

	_ = saveCachedDoc(doc) // Best effort save.
	return cacheInMemory(doc), nil
}

// cacheInMemory stores the document as the process-wide copy and returns it.
func cacheInMemory(doc *openapi3.T) *openapi3.T {
	globalDoc = doc
	cachedAt = time.Now()
	return doc
}

// fetchDoc downloads and parses the OpenAPI schema.
func fetchDoc() (*openapi3.T, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, SchemaURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", fmt.Sprintf("Auth0 CLI/%s", strings.TrimPrefix(buildinfo.Version, "v")))

	resp, err := schemaHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	doc, err := loader.LoadFromData(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI schema: %w", err)
	}

	return doc, nil
}

// getCacheDir returns the cache directory for OpenAPI schemas.
func getCacheDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	cacheDir := filepath.Join(homeDir, ".auth0", "cache")
	return cacheDir, os.MkdirAll(cacheDir, 0755)
}

// loadCachedDoc loads the schema from the disk cache and reports whether it is
// fresh (<CacheTTL); a stale copy is still returned for use as an offline fallback.
func loadCachedDoc() (doc *openapi3.T, fresh bool, err error) {
	cacheDir, err := getCacheDir()
	if err != nil {
		return nil, false, err
	}

	cachePath := filepath.Join(cacheDir, "openapi-schema.json")

	info, err := os.Stat(cachePath)
	if err != nil {
		return nil, false, err
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, false, err
	}

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true // Match fetchDoc so a cached copy always parses.
	doc, err = loader.LoadFromData(data)
	if err != nil {
		return nil, false, err
	}

	fresh = time.Since(info.ModTime()) < CacheTTL
	return doc, fresh, nil
}

// saveCachedDoc saves the schema to disk cache.
func saveCachedDoc(doc *openapi3.T) error {
	cacheDir, err := getCacheDir()
	if err != nil {
		return err
	}

	cachePath := filepath.Join(cacheDir, "openapi-schema.json")

	data, err := doc.MarshalJSON()
	if err != nil {
		return err
	}

	return os.WriteFile(cachePath, data, 0644)
}

// FindOperation finds an operation by HTTP method and path.
// Path should be in the format "/actions/actions" or "actions/actions".
func FindOperation(doc *openapi3.T, method, path string) (*openapi3.Operation, error) {
	method = strings.ToUpper(method)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	pathItem := doc.Paths.Find(path)
	if pathItem == nil {
		return nil, fmt.Errorf("path %q not found in schema", path)
	}

	operation := pathItem.GetOperation(method)
	if operation == nil {
		return nil, fmt.Errorf("method %q not found for path %q", method, path)
	}

	return operation, nil
}

// GetRequestSchema returns the request body schema for an operation.
func GetRequestSchema(operation *openapi3.Operation) *openapi3.SchemaRef {
	if operation.RequestBody == nil {
		return nil
	}

	// Try application/json first.
	content := operation.RequestBody.Value.Content
	if mediaType := content.Get("application/json"); mediaType != nil {
		return mediaType.Schema
	}

	// Fallback to application/x-www-form-urlencoded.
	if mediaType := content.Get("application/x-www-form-urlencoded"); mediaType != nil {
		return mediaType.Schema
	}

	return nil
}

// ExtractPathFromURL extracts the API path from a full URL.
// Example: "https://tenant.auth0.com/api/v2/actions/actions" -> "/actions/actions".
func ExtractPathFromURL(fullURL string) string {
	// Remove the base URL part - use the last occurrence of /api/v2.
	parts := strings.Split(fullURL, "/api/v2")
	if len(parts) < 2 {
		return ""
	}
	// Take the last part (in case /api/v2 appears multiple times).
	return parts[len(parts)-1]
}
