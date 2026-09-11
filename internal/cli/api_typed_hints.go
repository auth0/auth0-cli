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
// typed commands. It is intentionally NOT exhaustive: endpoints that have no
// typed command (for example /connections) are omitted so that no hint is
// printed for them and the raw request is used as intended.
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

	// Organizations.
	{http.MethodGet, "organizations", "auth0 orgs list"},
	{http.MethodPost, "organizations", "auth0 orgs create"},
	{http.MethodGet, "organizations/{id}", "auth0 orgs show"},
	{http.MethodPatch, "organizations/{id}", "auth0 orgs update"},
	{http.MethodDelete, "organizations/{id}", "auth0 orgs delete"},
	{http.MethodGet, "organizations/{id}/members", "auth0 orgs members list"},
	{http.MethodGet, "organizations/{id}/invitations", "auth0 orgs invitations list"},
	{http.MethodPost, "organizations/{id}/invitations", "auth0 orgs invitations create"},

	// Client grants.
	{http.MethodGet, "client-grants", "auth0 client-grants list"},
	{http.MethodPost, "client-grants", "auth0 client-grants create"},
	{http.MethodPatch, "client-grants/{id}", "auth0 client-grants update"},
	{http.MethodDelete, "client-grants/{id}", "auth0 client-grants delete"},

	// Custom domains.
	{http.MethodGet, "custom-domains", "auth0 domains list"},
	{http.MethodPost, "custom-domains", "auth0 domains create"},
	{http.MethodGet, "custom-domains/{id}", "auth0 domains show"},
	{http.MethodDelete, "custom-domains/{id}", "auth0 domains delete"},

	// Log streams.
	{http.MethodGet, "log-streams", "auth0 logs streams list"},
	{http.MethodPost, "log-streams", "auth0 logs streams create"},
	{http.MethodGet, "log-streams/{id}", "auth0 logs streams show"},
	{http.MethodPatch, "log-streams/{id}", "auth0 logs streams update"},
	{http.MethodDelete, "log-streams/{id}", "auth0 logs streams delete"},

	// Rules.
	{http.MethodGet, "rules", "auth0 rules list"},
	{http.MethodPost, "rules", "auth0 rules create"},
	{http.MethodGet, "rules/{id}", "auth0 rules show"},
	{http.MethodPatch, "rules/{id}", "auth0 rules update"},
	{http.MethodDelete, "rules/{id}", "auth0 rules delete"},

	// Network ACLs.
	{http.MethodGet, "network-acls", "auth0 network-acl list"},
	{http.MethodPost, "network-acls", "auth0 network-acl create"},
	{http.MethodGet, "network-acls/{id}", "auth0 network-acl show"},
	{http.MethodPatch, "network-acls/{id}", "auth0 network-acl update"},
	{http.MethodDelete, "network-acls/{id}", "auth0 network-acl delete"},

	// Event streams.
	{http.MethodGet, "event-streams", "auth0 event-streams list"},
	{http.MethodPost, "event-streams", "auth0 event-streams create"},
	{http.MethodGet, "event-streams/{id}", "auth0 event-streams show"},
	{http.MethodPatch, "event-streams/{id}", "auth0 event-streams update"},
	{http.MethodDelete, "event-streams/{id}", "auth0 event-streams delete"},

	// Tenant settings.
	{http.MethodGet, "tenants/settings", "auth0 tenant-settings show"},
	{http.MethodPatch, "tenants/settings", "auth0 tenant-settings update"},

	// Attack protection.
	{http.MethodGet, "attack-protection/brute-force-protection", "auth0 protection brute-force-protection show"},
	{http.MethodPatch, "attack-protection/brute-force-protection", "auth0 protection brute-force-protection update"},
	{http.MethodGet, "attack-protection/breached-password-detection", "auth0 protection breached-password-detection show"},
	{http.MethodPatch, "attack-protection/breached-password-detection", "auth0 protection breached-password-detection update"},
	{http.MethodGet, "attack-protection/suspicious-ip-throttling", "auth0 protection suspicious-ip-throttling show"},
	{http.MethodPatch, "attack-protection/suspicious-ip-throttling", "auth0 protection suspicious-ip-throttling update"},

	// Email provider.
	{http.MethodGet, "emails/provider", "auth0 email provider show"},
	{http.MethodPost, "emails/provider", "auth0 email provider create"},
	{http.MethodPatch, "emails/provider", "auth0 email provider update"},
	{http.MethodDelete, "emails/provider", "auth0 email provider delete"},
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
