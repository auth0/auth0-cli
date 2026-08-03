package openapi

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
)

const (
	// SchemaURL is the URL to the Auth0 Management API OpenAPI schema.
	SchemaURL = "https://auth0.com/docs/oas/management/v2/management-api-oas.json"

	// CacheTTL is how long to cache the schema before re-fetching.
	CacheTTL = 24 * time.Hour
)

var (
	globalDoc *openapi3.T
	cachedAt  time.Time
)

// GetDoc returns the cached or freshly fetched OpenAPI document.
func GetDoc() (*openapi3.T, error) {
	if globalDoc != nil && time.Since(cachedAt) < CacheTTL {
		return globalDoc, nil
	}

	// Try to load from disk cache first.
	if doc, err := loadCachedDoc(); err == nil {
		globalDoc = doc
		return globalDoc, nil
	}

	// Fetch from network.
	doc, err := fetchDoc()
	if err != nil {
		// If we have a stale cache, return it rather than failing.
		if globalDoc != nil {
			return globalDoc, nil
		}
		return nil, fmt.Errorf("failed to fetch OpenAPI schema: %w", err)
	}

	globalDoc = doc
	cachedAt = time.Now()
	_ = saveCachedDoc(doc) // Best effort save.
	return globalDoc, nil
}

// fetchDoc downloads and parses the OpenAPI schema.
func fetchDoc() (*openapi3.T, error) {
	resp, err := http.Get(SchemaURL)
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

// loadCachedDoc loads the schema from disk cache.
func loadCachedDoc() (*openapi3.T, error) {
	cacheDir, err := getCacheDir()
	if err != nil {
		return nil, err
	}

	cachePath := filepath.Join(cacheDir, "openapi-schema.json")

	// Check if cache file exists and is recent.
	info, err := os.Stat(cachePath)
	if err != nil {
		return nil, err
	}

	if time.Since(info.ModTime()) > CacheTTL {
		return nil, fmt.Errorf("cache expired")
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(data)
	if err != nil {
		return nil, err
	}

	cachedAt = info.ModTime()
	return doc, nil
}

// saveCachedDoc saves the schema to disk cache.
func saveCachedDoc(doc *openapi3.T) error {
	cacheDir, err := getCacheDir()
	if err != nil {
		return err
	}

	cachePath := filepath.Join(cacheDir, "openapi-schema.json")

	// Marshal the document.
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

// GetResponseSchema returns the response schema for a specific status code.
func GetResponseSchema(operation *openapi3.Operation, statusCode string) *openapi3.SchemaRef {
	response := operation.Responses.Status(mustParseInt(statusCode))
	if response == nil {
		return nil
	}

	if response.Value.Content == nil {
		return nil
	}

	// Try application/json.
	if mediaType := response.Value.Content.Get("application/json"); mediaType != nil {
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

// mustParseInt is a helper to parse status codes.
func mustParseInt(s string) int {
	var result int
	fmt.Sscanf(s, "%d", &result)
	return result
}
