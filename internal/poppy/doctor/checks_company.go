package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"

	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/poppy/discovery"
)

var validChanges = []string{"none", "reversible", "irreversible"}

// namesIssuer is the D2 message when the protected resource metadata lists the issuer.
const namesIssuer = "the protected resource metadata names the issuer"

// openAPIDocument is the part of an OpenAPI document that D7 reads.
type openAPIDocument struct {
	OpenAPI string `json:"openapi"`
}

// checkPoppy runs D1 and returns the document and the URL it was served from
// after redirects, or nil when the document cannot be used.
func (r *runner) checkPoppy(ctx context.Context, c *collector) (*discovery.Poppy, string) {
	rawURL := r.origin + wellKnownPoppy
	doc, finalURL := fetchDocumentFrom[discovery.Poppy](ctx, r, c, "D1", specDiscovery, rawURL)
	if doc == nil {
		return nil, ""
	}
	c.pass("D1", specDiscovery, "poppy.json is served over HTTPS at "+wellKnownPoppy)
	checkFinalURL(finalURL, c)

	if _, understood := protocolRules[doc.ProtocolVersion]; understood {
		c.pass("D1", specDiscovery, "protocol version "+doc.ProtocolVersion+" is one this CLI understands")
	} else {
		supported := strings.Join(config.SupportedVersions, " and ")
		c.warn("D1", specDiscovery,
			fmt.Sprintf("protocol_version is %q, and this CLI understands %s", doc.ProtocolVersion, supported),
			"Check the document against protocol "+supported+", or update the CLI if the site is newer.")
	}

	if strings.EqualFold(doc.Organization.Domain, r.domain) {
		c.pass("D1", specOrganization, "organization.domain is the serving host")
	} else {
		c.fail("D1", specOrganization,
			fmt.Sprintf("organization.domain is %q, and the serving host is %s", doc.Organization.Domain, r.domain),
			"Set organization.domain to "+r.domain+".")
	}

	checkHTTPSURLs(doc, c)

	if doc.Auth.Disabled {
		c.warn("D1", specOrganization, "agent sign-in is disabled (auth.disabled is true)",
			"Remove auth.disabled, or set it to false, when agents should be able to sign in.")
	}

	return doc, finalURL
}

// checkFinalURL records a failure when the document was served from a URL other
// than the well-known path with no query. Personal agents use the document only
// then (section 3), so that a file uploaded at some path of the domain cannot
// speak for it.
func checkFinalURL(finalURL string, c *collector) {
	if u, err := url.Parse(finalURL); err == nil && u.Path == wellKnownPoppy && u.RawQuery == "" {
		return
	}

	c.fail("D1", specDiscovery, "poppy.json was served from "+finalURL+", and the last URL must have the path "+wellKnownPoppy+" and no query",
		"Redirect the well-known URL only to a URL whose path is exactly "+wellKnownPoppy+", with no query.")
}

// checkHTTPSURLs records one result for every URL of poppy.json that must be HTTPS and is not.
func checkHTTPSURLs(doc *discovery.Poppy, c *collector) {
	urls := map[string]string{
		"organization.logo_uri":        doc.Organization.LogoURI,
		"web.browser_session_endpoint": doc.Web.BrowserSessionEndpoint,
		"feedback.endpoint":            doc.Feedback.Endpoint,
		"excluded_tasks.url":           doc.ExcludedTasks.URL,
	}
	for i, p := range doc.Agent.Protocols {
		urls[fmt.Sprintf("agent.protocols[%d].endpoint", i)] = p.Endpoint
	}
	for i, a := range doc.APIs {
		urls[fmt.Sprintf("apis[%d].url", i)] = a.URL
	}

	insecure := false
	for _, name := range slices.Sorted(maps.Keys(urls)) {
		if value := urls[name]; value != "" && !isHTTPS(value) {
			insecure = true
			c.fail("D1", specOrganization, fmt.Sprintf("%s is not an HTTPS URL: %s", name, value),
				"Publish "+name+" as an HTTPS URL.")
		}
	}
	if !insecure {
		c.pass("D1", specOrganization, "every URL in poppy.json is HTTPS")
	}
}

// checkProtectedResource runs D2 and returns the document, or nil when it cannot be used.
func (r *runner) checkProtectedResource(ctx context.Context, c *collector) *discovery.ProtectedResource {
	rawURL := r.origin + wellKnownProtectedResource
	doc := fetchDocument[discovery.ProtectedResource](ctx, r, c, "D2", specResource, rawURL)
	if doc == nil {
		return nil
	}

	if doc.Resource == r.origin {
		c.pass("D2", specResource, "the protected resource is "+r.origin)
	} else {
		c.fail("D2", specResource, fmt.Sprintf("resource is %q, and it must be exactly %s", doc.Resource, r.origin),
			"Set resource to "+r.origin+" (no path and no trailing slash).")
	}

	if len(doc.AuthorizationServers) == 0 {
		c.fail("D2", specResource, "authorization_servers is empty",
			"List the issuer in authorization_servers of "+wellKnownProtectedResource+".")
	}

	return doc
}

// resolveIssuer picks the issuer that D3 and D4 check: auth.issuer from poppy.json
// when present, else the only entry of authorization_servers. It returns "" when
// the issuer cannot be known, after recording why when D2 has not already.
func resolveIssuer(poppy *discovery.Poppy, resource *discovery.ProtectedResource, c *collector) string {
	var servers []string
	if resource != nil {
		servers = resource.AuthorizationServers
	}

	switch issuer := poppy.Auth.Issuer; {
	case issuer != "" && len(servers) > 0 && !slices.Contains(servers, issuer):
		c.fail("D2", specResource, "auth.issuer "+issuer+" is not in authorization_servers",
			"List "+issuer+" in authorization_servers of "+wellKnownProtectedResource+", or correct auth.issuer in poppy.json.")
		return issuer
	case issuer != "":
		if len(servers) > 0 {
			c.pass("D2", specResource, namesIssuer)
		}
		return issuer
	case len(servers) == 1:
		c.pass("D2", specResource, namesIssuer)
		return servers[0]
	case len(servers) > 1:
		c.fail("D2", specResource, "issuer is required with several authorization servers",
			"Set auth.issuer in poppy.json to the authorization server that issues tokens for this site.")
	}

	return ""
}

// checkScopes runs D5 on the poppy.json document.
func checkScopes(doc *discovery.Poppy, c *collector) {
	scopes := slices.Sorted(maps.Keys(doc.Auth.Scopes))
	problems := 0

	for _, scope := range scopes {
		if strings.TrimSpace(doc.Auth.Scopes[scope]) == "" {
			problems++
			c.fail("D5", specScopeHints, "scope "+scope+" has no description",
				"Give "+scope+" a description in auth.scopes, because agents' models choose scopes from it.")
		}
	}
	if problems == 0 {
		c.pass("D5", specScopeHints, "every scope has a description")
	}

	problems = 0
	for _, scope := range scopes {
		hint, ok := doc.Auth.ScopeHints[scope]
		if !ok {
			problems++
			c.warn("D5", specScopeHints, "scope_hints missing for "+scope,
				fmt.Sprintf("auth0 poppy setup --hint %s=changes:<%s>,personal_data:<true|false>", scope, strings.Join(validChanges, "|")))
			continue
		}
		if msg := invalidHint(hint); msg != "" {
			problems++
			c.fail("D5", specScopeHints, fmt.Sprintf("scope_hints for %s: %s", scope, msg),
				fmt.Sprintf("Set changes to one of %s and personal_data to true or false for %s.", strings.Join(validChanges, ", "), scope))
		}
	}
	for _, scope := range slices.Sorted(maps.Keys(doc.Auth.ScopeHints)) {
		if _, ok := doc.Auth.Scopes[scope]; !ok {
			problems++
			c.warn("D5", specScopeHints, "scope_hints lists "+scope+", which is not in auth.scopes",
				"Remove the hint, or add "+scope+" to auth.scopes.")
		}
	}
	if problems == 0 {
		c.pass("D5", specScopeHints, "every scope has valid hints")
	}
}

// invalidHint returns what is wrong with a scope_hints entry, or "" when it is valid.
func invalidHint(hint map[string]any) string {
	changes, _ := hint["changes"].(string)
	if !slices.Contains(validChanges, changes) {
		return fmt.Sprintf("changes is %s, and it must be one of %s", describe(hint["changes"]), strings.Join(validChanges, ", "))
	}
	if _, ok := hint["personal_data"].(bool); !ok {
		return fmt.Sprintf("personal_data is %s, and it must be true or false", describe(hint["personal_data"]))
	}

	return ""
}

// checkAPIs runs D7 on every API of poppy.json.
func (r *runner) checkAPIs(ctx context.Context, doc *discovery.Poppy, c *collector) {
	for _, api := range doc.APIs {
		switch api.Type {
		case "openapi":
			r.checkOpenAPI(ctx, api.URL, c)
		case "mcp":
			c.pass("D7", specAPIs, "mcp endpoint listed: "+api.URL)
		}
	}
}

func (r *runner) checkOpenAPI(ctx context.Context, rawURL string, c *collector) {
	resp := r.fetch(ctx, c, "D7", specAPIs, rawURL)
	if resp == nil {
		return
	}

	var doc openAPIDocument
	if err := json.Unmarshal(resp.Body, &doc); err != nil {
		c.warn("D7", specAPIs, rawURL+" not checked (only JSON documents are validated)",
			"Check by hand that "+rawURL+" is a valid OpenAPI 3.x document.")
		return
	}

	if strings.HasPrefix(doc.OpenAPI, "3.") {
		c.pass("D7", specAPIs, fmt.Sprintf("%s is OpenAPI %s", rawURL, doc.OpenAPI))
	} else {
		c.fail("D7", specAPIs, fmt.Sprintf("%s has openapi %q, and it must be 3.x", rawURL, doc.OpenAPI),
			"Serve an OpenAPI 3.x document at "+rawURL+".")
	}
}

func describe(v any) string {
	if v == nil {
		return "missing"
	}

	return fmt.Sprintf("%v", v)
}

func isHTTPS(rawURL string) bool {
	u, err := url.Parse(rawURL)
	return err == nil && u.Scheme == "https" && u.Host != ""
}
