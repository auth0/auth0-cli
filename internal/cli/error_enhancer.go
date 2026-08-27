package cli

import (
	"strings"

	"github.com/auth0/auth0-cli/internal/openapi"
)

// enhanceAPIError enriches an API error with the expected schema, best-effort:
// on any lookup failure it returns the original error unchanged.
func enhanceAPIError(err error, method, path string) error {
	if err == nil {
		return nil
	}

	manager, managerErr := openapi.NewSchemaManager()
	if managerErr != nil {
		return err
	}

	apiPath := normalizeAPIPath(path)
	if apiPath == "" {
		return err
	}

	return manager.EnhanceError(err, method, apiPath)
}

// normalizeAPIPath converts a full URL or relative path to the OpenAPI path format.
func normalizeAPIPath(path string) string {
	if strings.Contains(path, "/api/v2") {
		return openapi.ExtractPathFromURL(path)
	}
	if strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}
