package cli

import (
	"strings"

	"github.com/auth0/auth0-cli/internal/openapi"
)

// enhanceAPIError enhances an API error with schema information if available.
// This is a best-effort enhancement - if schema loading fails, it returns the original error.
func enhanceAPIError(err error, method, path string) error {
	if err == nil {
		return nil
	}

	manager, managerErr := openapi.NewSchemaManager()
	if managerErr != nil {
		// If we can't load the schema, just return the original error.
		return err
	}

	// Normalize the path to the API path format.
	apiPath := normalizeAPIPath(path)
	if apiPath == "" {
		return err
	}

	return manager.EnhanceError(err, method, apiPath)
}

// normalizeAPIPath normalizes a path to the OpenAPI format.
// It handles both full URLs and relative paths.
func normalizeAPIPath(path string) string {
	// If it's a full URL, extract the path.
	if strings.Contains(path, "/api/v2") {
		return openapi.ExtractPathFromURL(path)
	}

	// If it's already in the right format, return it.
	if strings.HasPrefix(path, "/") {
		return path
	}

	// Otherwise, add the leading slash.
	return "/" + path
}
