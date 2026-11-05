package doctor

import (
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/poppy/config"
)

// want is a result a mutated site must produce.
type want struct {
	id       string
	level    Level
	spec     string
	contains string
	platform bool
}

func obj(m map[string]any, key string) map[string]any {
	return m[key].(map[string]any)
}

func nonPass(r Report) []Result {
	return slices.DeleteFunc(slices.Clone(r.Results), func(res Result) bool { return res.Level == Pass })
}

func TestRunValidSite(t *testing.T) {
	report := validSite().run(t)

	assert.Equal(t, testDomain, report.Domain)
	assert.Zero(t, report.Count(Warn), "%+v", nonPass(report))
	assert.Zero(t, report.Count(Fail), "%+v", nonPass(report))
	assert.Zero(t, report.Count(Skipped), "%+v", nonPass(report))
	assert.Zero(t, report.ExitCode())

	var ids []string
	for _, res := range report.Results {
		assert.Equal(t, Pass, res.Level)
		assert.NotEmpty(t, res.Spec, res.Message)
		if len(ids) == 0 || ids[len(ids)-1] != res.ID {
			ids = append(ids, res.ID)
		}
	}
	assert.Equal(t, []string{"D1", "D2", "D3", "D4", "D5", "D6", "D7", "D15", "D16"}, ids, "results come in check order, once per check")
}

func TestRunMutations(t *testing.T) {
	redirectTo := func(location string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, location, http.StatusMovedPermanently)
		}
	}
	notFound := func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }
	skippedD3toD7 := []want{
		{id: "D3", level: Skipped, spec: "3.2", contains: "poppy.json is not available"},
		{id: "D4", level: Skipped, spec: "4.1", contains: "poppy.json is not available"},
		{id: "D5", level: Skipped, spec: "4.7", contains: "poppy.json is not available"},
		{id: "D6", level: Skipped, spec: "6", contains: "poppy.json is not available"},
		{id: "D7", level: Skipped, spec: "7", contains: "poppy.json is not available"},
	}

	tests := []struct {
		name   string
		mutate func(s *site)
		want   []want
	}{
		{
			name:   "protected_resources is missing",
			mutate: func(s *site) { delete(s.authz, "protected_resources") },
			want:   []want{{id: "D3", level: Fail, spec: "3.2", contains: "protected_resources does not list https://shop.example"}},
		},
		{
			name:   "protected_resources has a trailing slash",
			mutate: func(s *site) { s.authz["protected_resources"] = []any{"https://shop.example/"} },
			want:   []want{{id: "D3", level: Fail, spec: "3.2", contains: "protected_resources"}},
		},
		{
			name:   "issuer is off by a trailing slash",
			mutate: func(s *site) { s.authz["issuer"] = "https://auth.shop.example" },
			want:   []want{{id: "D3", level: Fail, spec: "3.2", contains: "character for character"}},
		},
		{
			name:   "PS384 is missing",
			mutate: func(s *site) { s.authz["token_endpoint_auth_signing_alg_values_supported"] = []any{"ES256"} },
			want:   []want{{id: "D4", level: Warn, spec: "4.1", contains: "PS384", platform: true}},
		},
		{
			name:   "ES256 is missing",
			mutate: func(s *site) { s.authz["token_endpoint_auth_signing_alg_values_supported"] = []any{"PS384"} },
			want:   []want{{id: "D4", level: Fail, spec: "4.1", contains: "ES256"}},
		},
		{
			name:   "iss parameter is false",
			mutate: func(s *site) { s.authz["authorization_response_iss_parameter_supported"] = false },
			want:   []want{{id: "D3", level: Fail, spec: "4.2", contains: "authorization_response_iss_parameter_supported"}},
		},
		{
			name:   "token endpoint is missing",
			mutate: func(s *site) { delete(s.authz, "token_endpoint") },
			want:   []want{{id: "D3", level: Fail, spec: "4.2", contains: "token_endpoint is missing"}},
		},
		{
			name:   "revocation endpoint is HTTP",
			mutate: func(s *site) { s.authz["revocation_endpoint"] = "http://auth.shop.example/oauth/revoke" },
			want:   []want{{id: "D3", level: Fail, spec: "4.2", contains: "revocation_endpoint is not an HTTPS URL"}},
		},
		{
			name: "refresh_token grant is missing",
			mutate: func(s *site) {
				s.authz["grant_types_supported"] = []any{"authorization_code", "urn:ietf:params:oauth:grant-type:device_code"}
			},
			want: []want{{id: "D3", level: Fail, spec: "4.2", contains: "refresh_token"}},
		},
		{
			name:   "scopes_supported lacks an offered scope",
			mutate: func(s *site) { s.authz["scopes_supported"] = []any{"orders:read", "orders:write"} },
			want:   []want{{id: "D3", level: Warn, spec: "4.2", contains: "addresses"}},
		},
		{
			name:   "authorization server metadata is not served",
			mutate: func(s *site) { s.routes["auth.shop.example/.well-known/oauth-authorization-server"] = notFound },
			want: []want{
				{id: "D3", level: Fail, spec: "3.2", contains: "HTTP 404"},
				{id: "D4", level: Skipped, spec: "4.1", contains: "authorization server metadata is not available"},
			},
		},
		{
			name: "issuer is not HTTPS",
			mutate: func(s *site) {
				obj(s.poppy, "auth")["issuer"] = "http://auth.shop.example/"
				s.resource["authorization_servers"] = []any{"http://auth.shop.example/"}
			},
			want: []want{
				{id: "D3", level: Fail, spec: "3.2", contains: "not an HTTPS URL"},
				{id: "D4", level: Skipped, spec: "4.1"},
			},
		},
		{
			name: "protected resource redirects to HTTP",
			mutate: func(s *site) {
				s.routes[testDomain+"/.well-known/oauth-protected-resource"] = redirectTo("http://shop.example/prm")
			},
			want: []want{{id: "D2", level: Fail, spec: "3.2", contains: "http://shop.example/prm is not an HTTPS URL"}},
		},
		{
			name: "poppy.json redirects to HTTP",
			mutate: func(s *site) {
				s.routes[testDomain+"/.well-known/poppy.json"] = redirectTo("http://shop.example/poppy.json")
			},
			want: append([]want{
				{id: "D1", level: Fail, spec: "3", contains: "http://shop.example/poppy.json is not an HTTPS URL"},
			}, skippedD3toD7...),
		},
		{
			name:   "poppy.json is not served",
			mutate: func(s *site) { s.routes[testDomain+"/.well-known/poppy.json"] = notFound },
			want: append([]want{
				{id: "D1", level: Fail, spec: "3", contains: "HTTP 404"},
				{id: "D2", level: Pass, spec: "3.2", contains: "resource is https://shop.example"},
			}, skippedD3toD7...),
		},
		{
			name: "poppy.json is not JSON",
			mutate: func(s *site) {
				s.routes[testDomain+"/.well-known/poppy.json"] = func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "<html>") }
			},
			want: append([]want{{id: "D1", level: Fail, spec: "3", contains: "not a valid document"}}, skippedD3toD7...),
		},
		{
			name:   "browser join answers 404",
			mutate: func(s *site) { s.browserStatus = http.StatusNotFound },
			want:   []want{{id: "D6", level: Fail, spec: "6", contains: "may lack the beta"}},
		},
		{
			name:   "browser join answers 500",
			mutate: func(s *site) { s.browserStatus = http.StatusInternalServerError },
			want:   []want{{id: "D6", level: Fail, spec: "6", contains: "HTTP 500"}},
		},
		{
			name: "join cookie lacks flags",
			mutate: func(s *site) {
				s.browserHeaders.Set("Set-Cookie", "__Host-poppy_join=abc; Path=/; Secure")
			},
			want: []want{{id: "D6", level: Fail, spec: "6", contains: "lacks: HttpOnly, SameSite=Strict, Max-Age"}},
		},
		{
			name:   "join cookie is missing",
			mutate: func(s *site) { s.browserHeaders.Del("Set-Cookie") },
			want:   []want{{id: "D6", level: Fail, spec: "6", contains: "does not set"}},
		},
		{
			name:   "join response is cacheable",
			mutate: func(s *site) { s.browserHeaders.Del("Cache-Control") },
			want:   []want{{id: "D6", level: Warn, spec: "6", contains: "no-store"}},
		},
		{
			name:   "join form has no hidden code input",
			mutate: func(s *site) { s.browserBody = `<form><input type="text" name="code"></form>` },
			want:   []want{{id: "D6", level: Fail, spec: "6", contains: "hidden code input"}},
		},
		{
			name:   "join page has no form",
			mutate: func(s *site) { s.browserBody = `<input type="hidden" name="code">` },
			want:   []want{{id: "D6", level: Fail, spec: "6", contains: "hidden code input"}},
		},
		{
			name:   "no browser session endpoint",
			mutate: func(s *site) { delete(obj(s.poppy, "web"), "browser_session_endpoint") },
			want:   []want{{id: "D6", level: Warn, spec: "6", contains: "not published"}},
		},
		{
			name:   "OpenAPI document is not version 3",
			mutate: func(s *site) { s.openapi = `{"swagger":"2.0"}` },
			want:   []want{{id: "D7", level: Fail, spec: "7", contains: `has openapi ""`}},
		},
		{
			name: "OpenAPI document is YAML",
			mutate: func(s *site) {
				s.openapi = "openapi: 3.1.0\n"
				s.openapiType = "application/yaml"
			},
			want: []want{{id: "D7", level: Warn, spec: "7", contains: "only JSON documents are validated"}},
		},
		{
			name:   "OpenAPI document is not served",
			mutate: func(s *site) { s.routes["api.shop.example/openapi.json"] = notFound },
			want:   []want{{id: "D7", level: Fail, spec: "7", contains: "HTTP 404"}},
		},
		{
			name:   "scope_hints are missing for a scope",
			mutate: func(s *site) { delete(obj(obj(s.poppy, "auth"), "scope_hints"), "addresses") },
			want:   []want{{id: "D5", level: Warn, spec: "4.7", contains: "scope_hints missing for addresses"}},
		},
		{
			name: "a hint has an invalid value",
			mutate: func(s *site) {
				obj(obj(s.poppy, "auth"), "scope_hints")["orders:read"] = map[string]any{"changes": "sometimes", "personal_data": "yes"}
			},
			want: []want{{id: "D5", level: Fail, spec: "4.7", contains: "changes is sometimes"}},
		},
		{
			name: "a hint has no personal_data",
			mutate: func(s *site) {
				obj(obj(s.poppy, "auth"), "scope_hints")["orders:read"] = map[string]any{"changes": "none"}
			},
			want: []want{{id: "D5", level: Fail, spec: "4.7", contains: "personal_data is missing"}},
		},
		{
			name: "a hint names an unknown scope",
			mutate: func(s *site) {
				obj(obj(s.poppy, "auth"), "scope_hints")["ghost"] = map[string]any{"changes": "none", "personal_data": false}
			},
			want: []want{{id: "D5", level: Warn, spec: "4.7", contains: "ghost"}},
		},
		{
			name:   "a scope has no description",
			mutate: func(s *site) { obj(obj(s.poppy, "auth"), "scopes")["addresses"] = "  " },
			want:   []want{{id: "D5", level: Fail, spec: "4.7", contains: "scope addresses has no description"}},
		},
		{
			name:   "agent sign-in is disabled",
			mutate: func(s *site) { obj(s.poppy, "auth")["disabled"] = true },
			want:   []want{{id: "D1", level: Warn, spec: "3.1", contains: "agent sign-in is disabled"}},
		},
		{
			name:   "protocol version is newer",
			mutate: func(s *site) { s.poppy["protocol_version"] = "0.5" },
			want:   []want{{id: "D1", level: Warn, spec: "3", contains: `"0.5", and this CLI understands 0.3 and 0.4`}},
		},
		{
			name:   "protocol version is 0.4",
			mutate: func(*site) {},
			want:   []want{{id: "D1", level: Pass, spec: "3", contains: "protocol version 0.4 is one this CLI understands"}},
		},
		{
			name:   "a site on 0.3 passes and gets no 0.4 checks",
			mutate: func(s *site) { *s = *valid03Site() },
			want:   []want{{id: "D1", level: Pass, spec: "3", contains: "protocol version 0.3 is one this CLI understands"}},
		},
		{
			name: "a 0.3 site with 0.4 members is not held to the 0.4 rules",
			mutate: func(s *site) {
				*s = *valid03Site()
				obj(s.poppy, "web")["disabled"] = "yes"
				s.poppy["supported_versions"] = "0.4"
			},
			want: []want{{id: "D1", level: Pass, spec: "3", contains: "protocol version 0.3 is one this CLI understands"}},
		},
		{
			name:   "web.disabled is a string",
			mutate: func(s *site) { obj(s.poppy, "web")["disabled"] = "yes" },
			want:   []want{{id: "D15", level: Fail, spec: "3.1", contains: `web.disabled is "yes" (a string), and it must be true or false`}},
		},
		{
			name:   "web.unscoped is a number",
			mutate: func(s *site) { obj(s.poppy, "web")["unscoped"] = 1 },
			want:   []want{{id: "D15", level: Fail, spec: "3.1", contains: "web.unscoped is 1 (a number), and it must be true or false"}},
		},
		{
			name:   "web.disabled and web.unscoped are booleans",
			mutate: func(s *site) { obj(s.poppy, "web")["disabled"], obj(s.poppy, "web")["unscoped"] = false, false },
			want:   []want{{id: "D15", level: Pass, spec: "3.1", contains: "the web settings are valid"}},
		},
		{
			name:   "web.disabled is true and the endpoint is still listed",
			mutate: func(s *site) { obj(s.poppy, "web")["disabled"] = true },
			want:   []want{{id: "D15", level: Fail, spec: "6", contains: "web.disabled is true, and web.browser_session_endpoint is listed"}},
		},
		{
			name: "web.disabled is true with no endpoint",
			mutate: func(s *site) {
				obj(s.poppy, "web")["disabled"] = true
				delete(obj(s.poppy, "web"), "browser_session_endpoint")
			},
			want: []want{
				{id: "D15", level: Pass, spec: "3.1", contains: "the web settings are valid"},
				{id: "D6", level: Pass, spec: "6", contains: "web.disabled is true"},
			},
		},
		{
			name: "web.disabled is true and nothing replaces the website",
			mutate: func(s *site) {
				obj(s.poppy, "web")["disabled"] = true
				delete(obj(s.poppy, "web"), "browser_session_endpoint")
				delete(s.poppy, "agent")
				delete(s.poppy, "apis")
			},
			want: []want{{id: "D15", level: Fail, spec: "6", contains: "lists neither agent nor apis"}},
		},
		{
			name: "web.unscoped is true and the endpoint is not listed",
			mutate: func(s *site) {
				obj(s.poppy, "web")["unscoped"] = true
				delete(obj(s.poppy, "web"), "browser_session_endpoint")
			},
			want: []want{
				{id: "D15", level: Fail, spec: "6", contains: "web.unscoped is true, and web.browser_session_endpoint is not listed"},
				{id: "D6", level: Warn, spec: "6", contains: "web.browser_session_endpoint is not published"},
			},
		},
		{
			name:   "web.unscoped is true",
			mutate: func(s *site) { obj(s.poppy, "web")["unscoped"] = true },
			want: []want{
				{id: "D15", level: Pass, spec: "3.1", contains: "valid"},
				{id: "D16", level: Warn, spec: "6", contains: "web.unscoped is true: check by hand that the description of poppy:browser-sign-in says it gives full account access"},
			},
		},
		{
			name: "the browser scope is in neither auth.scopes nor scopes_supported",
			mutate: func(s *site) {
				delete(obj(obj(s.poppy, "auth"), "scopes"), "poppy:browser-sign-in")
				delete(obj(obj(s.poppy, "auth"), "scope_hints"), "poppy:browser-sign-in")
				s.authz["scopes_supported"] = []any{"orders:read", "orders:write", "addresses", "openid"}
			},
			want: []want{
				{id: "D16", level: Warn, spec: "4.7", contains: "is in neither auth.scopes nor the scopes_supported of the issuer"},
				{id: "D16", level: Warn, spec: "4.7", contains: "scope_hints has no entry for poppy:browser-sign-in"},
			},
		},
		{
			name: "the browser scope is only in scopes_supported",
			mutate: func(s *site) {
				delete(obj(obj(s.poppy, "auth"), "scopes"), "poppy:browser-sign-in")
				delete(obj(obj(s.poppy, "auth"), "scope_hints"), "poppy:browser-sign-in")
			},
			want: []want{
				{id: "D16", level: Pass, spec: "4.7", contains: "listed in scopes_supported of the issuer"},
				{id: "D16", level: Warn, spec: "4.7", contains: "scope_hints has no entry for poppy:browser-sign-in"},
			},
		},
		{
			name: "the browser scope is not needed when the website is disabled",
			mutate: func(s *site) {
				obj(s.poppy, "web")["disabled"] = true
				delete(obj(s.poppy, "web"), "browser_session_endpoint")
				delete(obj(obj(s.poppy, "auth"), "scopes"), "poppy:browser-sign-in")
				delete(obj(obj(s.poppy, "auth"), "scope_hints"), "poppy:browser-sign-in")
				s.authz["scopes_supported"] = []any{"orders:read", "orders:write", "addresses", "openid"}
			},
			want: []want{{id: "D15", level: Pass, spec: "3.1", contains: "valid"}},
		},
		{
			name: "supported_versions lists another version",
			mutate: func(s *site) {
				s.poppy["supported_versions"] = map[string]any{"0.3": "https://shop.example/.well-known/poppy-0.3.json"}
			},
			want: []want{{id: "D17", level: Pass, spec: "3.4", contains: "supported_versions lists 1 other version(s), each an HTTPS URL on shop.example"}},
		},
		{
			name:   "supported_versions is a string",
			mutate: func(s *site) { s.poppy["supported_versions"] = "0.3" },
			want:   []want{{id: "D17", level: Fail, spec: "3.4", contains: `supported_versions is "0.3" (a string), and it must be an object`}},
		},
		{
			name:   "a supported_versions value is not a string",
			mutate: func(s *site) { s.poppy["supported_versions"] = map[string]any{"0.3": 3} },
			want:   []want{{id: "D17", level: Fail, spec: "3.4", contains: `supported_versions["0.3"] is 3 (a number), and it must be an HTTPS URL`}},
		},
		{
			name: "a supported_versions value is HTTP",
			mutate: func(s *site) {
				s.poppy["supported_versions"] = map[string]any{"0.3": "http://shop.example/poppy-0.3.json"}
			},
			want: []want{{id: "D17", level: Fail, spec: "3.4", contains: `supported_versions["0.3"] is not an HTTPS URL: http://shop.example/poppy-0.3.json`}},
		},
		{
			name: "a supported_versions value is on another host",
			mutate: func(s *site) {
				s.poppy["supported_versions"] = map[string]any{"0.3": "https://other.example/poppy-0.3.json"}
			},
			want: []want{{id: "D17", level: Fail, spec: "3.4", contains: "is on other.example, and it must be on shop.example"}},
		},
		{
			name: "poppy.json is served from another path after a redirect",
			mutate: func(s *site) {
				doc := s.poppy
				s.routes[testDomain+"/.well-known/poppy.json"] = redirectTo("https://shop.example/files/poppy.json")
				s.routes[testDomain+"/files/poppy.json"] = func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, doc) }
			},
			want: []want{{id: "D1", level: Fail, spec: "3", contains: "poppy.json was served from https://shop.example/files/poppy.json, and the last URL must have the path /.well-known/poppy.json and no query"}},
		},
		{
			name: "a 0.3 poppy.json is served from another path after a redirect",
			mutate: func(s *site) {
				*s = *valid03Site()
				doc := s.poppy
				s.routes[testDomain+"/.well-known/poppy.json"] = redirectTo("https://shop.example/files/poppy.json")
				s.routes[testDomain+"/files/poppy.json"] = func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, doc) }
			},
			want: []want{{id: "D1", level: Fail, spec: "3", contains: "poppy.json was served from https://shop.example/files/poppy.json"}},
		},
		{
			name: "poppy.json is served with a query after a redirect",
			mutate: func(s *site) {
				doc := s.poppy
				s.routes[testDomain+"/.well-known/poppy.json"] = func(w http.ResponseWriter, r *http.Request) {
					if r.URL.RawQuery == "" {
						http.Redirect(w, r, "https://shop.example/.well-known/poppy.json?v=1", http.StatusFound)
						return
					}
					writeJSON(w, doc)
				}
			},
			want: []want{{id: "D1", level: Fail, spec: "3", contains: "poppy.json?v=1, and the last URL"}},
		},
		{
			name: "poppy.json is served from the well-known path on another host after a redirect",
			mutate: func(s *site) {
				doc := s.poppy
				s.routes[testDomain+"/.well-known/poppy.json"] = redirectTo("https://auth.shop.example/.well-known/poppy.json")
				s.routes["auth.shop.example/.well-known/poppy.json"] = func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, doc) }
			},
			want: []want{{id: "D1", level: Pass, spec: "3", contains: "protocol version 0.4 is one this CLI understands"}},
		},
		{
			name:   "organization.domain is another host",
			mutate: func(s *site) { obj(s.poppy, "organization")["domain"] = "other.example" },
			want:   []want{{id: "D1", level: Fail, spec: "3.1", contains: "organization.domain"}},
		},
		{
			name:   "organization.domain differs only in case",
			mutate: func(s *site) { obj(s.poppy, "organization")["domain"] = "Shop.Example" },
		},
		{
			name:   "a listed URL is HTTP",
			mutate: func(s *site) { obj(s.poppy, "feedback")["endpoint"] = "http://api.shop.example/feedback" },
			want:   []want{{id: "D1", level: Fail, spec: "3.1", contains: "feedback.endpoint is not an HTTPS URL"}},
		},
		{
			name: "a protocol endpoint is HTTP",
			mutate: func(s *site) {
				obj(s.poppy, "agent")["protocols"] = []any{map[string]any{"type": "poppy", "endpoint": "http://api.shop.example/c"}}
			},
			want: []want{{id: "D1", level: Fail, spec: "3.1", contains: "agent.protocols[0].endpoint"}},
		},
		{
			name:   "resource has a trailing slash",
			mutate: func(s *site) { s.resource["resource"] = "https://shop.example/" },
			want:   []want{{id: "D2", level: Fail, spec: "3.2", contains: "must be exactly https://shop.example"}},
		},
		{
			name: "authorization_servers is empty but poppy.json names the issuer",
			mutate: func(s *site) {
				s.resource["authorization_servers"] = []any{}
			},
			want: []want{{id: "D2", level: Fail, spec: "3.2", contains: "authorization_servers is empty"}},
		},
		{
			name: "authorization_servers is empty and poppy.json has no issuer",
			mutate: func(s *site) {
				delete(obj(s.poppy, "auth"), "issuer")
				s.resource["authorization_servers"] = []any{}
			},
			want: []want{
				{id: "D2", level: Fail, spec: "3.2", contains: "authorization_servers is empty"},
				{id: "D3", level: Skipped, spec: "3.2", contains: "issuer is not known"},
				{id: "D4", level: Skipped, spec: "4.1", contains: "issuer is not known"},
			},
		},
		{
			name: "protected resource metadata is not served and poppy.json has no issuer",
			mutate: func(s *site) {
				delete(obj(s.poppy, "auth"), "issuer")
				s.routes[testDomain+"/.well-known/oauth-protected-resource"] = notFound
			},
			want: []want{
				{id: "D2", level: Fail, spec: "3.2", contains: "HTTP 404"},
				{id: "D3", level: Skipped, spec: "3.2", contains: "issuer is not known"},
				{id: "D4", level: Skipped, spec: "4.1", contains: "issuer is not known"},
			},
		},
		{
			name:   "protected resource metadata is not served but poppy.json names the issuer",
			mutate: func(s *site) { s.routes[testDomain+"/.well-known/oauth-protected-resource"] = notFound },
			want:   []want{{id: "D2", level: Fail, spec: "3.2", contains: "HTTP 404"}},
		},
		{
			name: "several authorization servers and no issuer",
			mutate: func(s *site) {
				delete(obj(s.poppy, "auth"), "issuer")
				s.resource["authorization_servers"] = []any{testIssuer, "https://other.example/"}
			},
			want: []want{
				{id: "D2", level: Fail, spec: "3.2", contains: "issuer is required with several authorization servers"},
				{id: "D3", level: Skipped, spec: "3.2", contains: "issuer is not known"},
				{id: "D4", level: Skipped, spec: "4.1", contains: "issuer is not known"},
			},
		},
		{
			name: "several authorization servers and an issuer among them",
			mutate: func(s *site) {
				s.resource["authorization_servers"] = []any{testIssuer, "https://other.example/"}
			},
		},
		{
			name: "issuer is not among the authorization servers",
			mutate: func(s *site) {
				s.resource["authorization_servers"] = []any{"https://other.example/"}
			},
			want: []want{{id: "D2", level: Fail, spec: "3.2", contains: "is not in authorization_servers"}},
		},
		{
			name: "no issuer in poppy.json and a single authorization server",
			mutate: func(s *site) {
				delete(obj(s.poppy, "auth"), "issuer")
			},
			want: []want{{id: "D2", level: Pass, spec: "3.2", contains: namesIssuer}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := validSite()
			test.mutate(s)

			report := s.run(t)

			for _, w := range test.want {
				i := slices.IndexFunc(report.Results, func(res Result) bool {
					return res.ID == w.id && res.Level == w.level && res.Spec == w.spec && strings.Contains(res.Message, w.contains)
				})
				if !assert.GreaterOrEqual(t, i, 0, "missing %+v in %+v", w, report.Results) {
					continue
				}

				res := report.Results[i]
				assert.Equal(t, w.platform, res.PlatformDependency, res.Message)
				if res.Level == Warn || res.Level == Fail {
					assert.NotEmpty(t, res.Fix, res.Message)
				}
			}

			wantedNonPass := 0
			for _, w := range test.want {
				if w.level != Pass {
					wantedNonPass++
				}
			}
			assert.Len(t, nonPass(report), wantedNonPass, "unexpected extra results: %+v", nonPass(report))

			wantExit := 0
			if slices.ContainsFunc(test.want, func(w want) bool { return w.level == Fail }) {
				wantExit = 1
			}
			assert.Equal(t, wantExit, report.ExitCode())
		})
	}
}

func TestRunValid03Site(t *testing.T) {
	report := valid03Site().run(t)

	assert.Zero(t, report.Count(Warn), "%+v", nonPass(report))
	assert.Zero(t, report.Count(Fail), "%+v", nonPass(report))
	assert.Zero(t, report.ExitCode())

	var ids []string
	for _, res := range report.Results {
		if len(ids) == 0 || ids[len(ids)-1] != res.ID {
			ids = append(ids, res.ID)
		}
	}
	assert.Equal(t, []string{"D1", "D2", "D3", "D4", "D5", "D6", "D7"}, ids, "the 0.4 checks do not run for a 0.3 site")
	assert.Contains(t, report.Results[1].Message, "protocol version 0.3 is one this CLI understands")
}

func TestProtocolRulesCoverTheSupportedVersions(t *testing.T) {
	assert.ElementsMatch(t, config.SupportedVersions, slices.Collect(maps.Keys(protocolRules)),
		"a version the config accepts needs its doctor rules, and the other way round")
}

func TestRunReportsNetworkErrorsAsFailures(t *testing.T) {
	s := validSite()
	s.routes[testDomain+"/.well-known/poppy.json"] = func(w http.ResponseWriter, _ *http.Request) {
		// Closing the connection mid-response is a transport error for the client.
		hj, ok := w.(http.Hijacker)
		require.True(t, ok)
		conn, _, err := hj.Hijack()
		require.NoError(t, err)
		_ = conn.Close()
	}

	report := s.run(t)

	require.NotEmpty(t, report.Results)
	assert.Equal(t, "D1", report.Results[0].ID)
	assert.Equal(t, Fail, report.Results[0].Level)
	assert.Contains(t, report.Results[0].Message, "could not be fetched")
	assert.Equal(t, 1, report.ExitCode())
}

func TestHasHiddenCodeInput(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{name: "type before name", body: `<form><input type="hidden" name="code"></form>`, want: true},
		{name: "name before type", body: `<FORM><INPUT NAME='code' TYPE=hidden></FORM>`, want: true},
		{name: "unquoted values", body: `<form><input type=hidden name=code></form>`, want: true},
		{name: "self closing", body: `<form><input name="code" type="hidden"/></form>`, want: true},
		{name: "visible input", body: `<form><input type="text" name="code"></form>`},
		{name: "other hidden input", body: `<form><input type="hidden" name="state"></form>`},
		{name: "name that only starts with code", body: `<form><input type="hidden" name="codex"></form>`},
		{name: "no form", body: `<input type="hidden" name="code">`},
		{name: "empty", body: ``},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, hasHiddenCodeInput(test.body))
		})
	}
}
