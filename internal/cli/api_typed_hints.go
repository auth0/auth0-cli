package cli

import (
	"net/http"
	"strings"
)

// typedCommandHint links a Management API operation to the dedicated auth0-cli
// command that covers it. Typed commands add schema discovery (--schema),
// local payload validation (--data), and structured output (--json) on top of
// the raw request, so we nudge users toward them for discoverability. The raw
// request still runs — the hint never blocks the escape hatch.
type typedCommandHint struct {
	// The HTTP method of the operation.
	method string
	// The API path relative to /api/v2/, where a "{...}" segment matches any
	// single path segment (e.g. "clients/{id}").
	path string
	// The equivalent typed command to recommend.
	command string
}

// typedCommandHints maps common Management API operations to their dedicated
// typed commands. It is intentionally limited to resources whose typed command
// already offers schema discovery and input validation (--schema, --data,
// --query); the remaining resources are added here as that support lands for
// each one. Any endpoint not listed (for example /connections) prints no hint
// and runs the raw request as intended.
var typedCommandHints = []typedCommandHint{
	// Applications (clients).
	{http.MethodGet, "clients", "auth0 apps list"},
	{http.MethodPost, "clients", "auth0 apps create"},
	{http.MethodGet, "clients/{id}", "auth0 apps show"},
	{http.MethodPatch, "clients/{id}", "auth0 apps update"},
	{http.MethodDelete, "clients/{id}", "auth0 apps delete"},

	// APIs (resource servers).
	{http.MethodGet, "resource-servers", "auth0 apis list"},
	{http.MethodPost, "resource-servers", "auth0 apis create"},
	{http.MethodGet, "resource-servers/{id}", "auth0 apis show"},
	{http.MethodPatch, "resource-servers/{id}", "auth0 apis update"},
	{http.MethodDelete, "resource-servers/{id}", "auth0 apis delete"},

	// Actions.
	{http.MethodGet, "actions/actions", "auth0 actions list"},
	{http.MethodPost, "actions/actions", "auth0 actions create"},
	{http.MethodGet, "actions/actions/{id}", "auth0 actions show"},
	{http.MethodPatch, "actions/actions/{id}", "auth0 actions update"},
	{http.MethodDelete, "actions/actions/{id}", "auth0 actions delete"},

	// Users.
	{http.MethodGet, "users", "auth0 users search"},
	{http.MethodPost, "users", "auth0 users create"},
	{http.MethodGet, "users/{id}", "auth0 users show"},
	{http.MethodPatch, "users/{id}", "auth0 users update"},
	{http.MethodDelete, "users/{id}", "auth0 users delete"},

	// Roles.
	{http.MethodGet, "roles", "auth0 roles list"},
	{http.MethodPost, "roles", "auth0 roles create"},
	{http.MethodGet, "roles/{id}", "auth0 roles show"},
	{http.MethodPatch, "roles/{id}", "auth0 roles update"},
	{http.MethodDelete, "roles/{id}", "auth0 roles delete"},
	{http.MethodGet, "roles/{id}/permissions", "auth0 roles permissions list"},
	{http.MethodPost, "roles/{id}/permissions", "auth0 roles permissions add"},
	{http.MethodDelete, "roles/{id}/permissions", "auth0 roles permissions remove"},
}

// suggestTypedCommand returns the recommended typed command for the given raw
// API method and URI, or "" when the endpoint has no dedicated typed command.
func suggestTypedCommand(method, rawURI string) string {
	reqSegments := splitAPIPath(rawURI)
	if len(reqSegments) == 0 {
		return ""
	}

	for _, hint := range typedCommandHints {
		if !strings.EqualFold(hint.method, method) {
			continue
		}

		if pathMatchesPattern(reqSegments, strings.Split(hint.path, "/")) {
			return hint.command
		}
	}

	return ""
}

// splitAPIPath normalizes a raw API URI into its path segments: it drops any
// query or fragment, an optional "api/v2" prefix, and surrounding slashes.
func splitAPIPath(rawURI string) []string {
	path := rawURI
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}

	path = strings.Trim(path, "/")
	path = strings.TrimPrefix(path, "api/v2")
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}

	return strings.Split(path, "/")
}

// pathMatchesPattern reports whether the request path segments match the
// pattern segments, where a "{...}" pattern segment matches any single
// segment and literal segments are compared case-insensitively.
func pathMatchesPattern(reqSegments, patternSegments []string) bool {
	if len(reqSegments) != len(patternSegments) {
		return false
	}

	for i, pattern := range patternSegments {
		if strings.HasPrefix(pattern, "{") && strings.HasSuffix(pattern, "}") {
			continue
		}

		if !strings.EqualFold(pattern, reqSegments[i]) {
			return false
		}
	}

	return true
}
