package doctor

import (
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"

	"github.com/auth0/auth0-cli/internal/poppy/discovery"
	"github.com/auth0/auth0-cli/internal/poppy/setup"
)

const (
	// SpecWebFields is the section that defines the members of the web object.
	specWebFields = "3.1"
)

// versionRules are the checks that depend on the protocol version a site
// declares. Every check that is not here runs for every version.
type versionRules struct {
	// Web covers web.disabled, web.unscoped and the poppy:browser-sign-in scope.
	web bool
	// Versions covers supported_versions.
	versions bool
}

// protocolRules holds the rules of each protocol version the CLI understands,
// keyed by the protocol_version of the site. A version that is not here is not
// understood: D1 warns about it and only the checks of every version run.
var protocolRules = map[string]versionRules{
	"0.3": {},
	"0.4": {web: true, versions: true},
}

// rulesOf returns the rules for the version the document declares.
func rulesOf(doc *discovery.Poppy) versionRules {
	return protocolRules[doc.ProtocolVersion]
}

// checkVersionRules runs the checks that the protocol version of the site adds:
// D15 for the web object, D16 for the poppy:browser-sign-in scope and D17 for
// supported_versions. Nothing runs for a version that adds none.
func checkVersionRules(doc *discovery.Poppy, finalURL string, issuer *discovery.AuthorizationServer, c *collector) {
	rules := rulesOf(doc)

	if rules.web {
		valid := checkWebFields(doc, c)
		checkBrowserSignIn(doc, issuer, valid, c)
	}

	if rules.versions {
		checkSupportedVersions(doc, finalURL, c)
	}
}

// checkWebFields runs D15 on web.disabled and web.unscoped. It reports whether
// both have the right type, so that later checks can rely on them.
func checkWebFields(doc *discovery.Poppy, c *collector) bool {
	web := doc.Web
	valid := true

	for _, field := range []struct {
		name  string
		value any
	}{{"web.disabled", web.Disabled}, {"web.unscoped", web.Unscoped}} {
		if _, isBool := field.value.(bool); field.value != nil && !isBool {
			valid = false
			c.fail("D15", specWebFields, fmt.Sprintf("%s is %s, and it must be true or false", field.name, describeJSON(field.value)),
				"Set "+field.name+" to true or false, or remove it.")
		}
	}

	if !valid {
		return false
	}

	problems := 0
	fail := func(message, fix string) {
		problems++
		c.fail("D15", specBrowserJoin, message, fix)
	}

	if isTrue(web.Disabled) {
		if web.BrowserSessionEndpoint != "" {
			fail("web.disabled is true, and web.browser_session_endpoint is listed",
				"Remove web.browser_session_endpoint, or set web.disabled to false.")
		}

		if len(doc.Agent.Protocols) == 0 && len(doc.APIs) == 0 {
			fail("web.disabled is true, and poppy.json lists neither agent nor apis",
				"List the Company Agent in agent or the APIs in apis, so that agents have something to use instead of the website.")
		}
	}

	if isTrue(web.Unscoped) && web.BrowserSessionEndpoint == "" {
		fail("web.unscoped is true, and web.browser_session_endpoint is not listed",
			"List web.browser_session_endpoint, or set web.unscoped to false.")
	}

	if problems == 0 {
		c.pass("D15", specWebFields, "the web settings are valid")
	}

	return problems == 0
}

// checkBrowserSignIn runs D16: the scope that the signed-in browser join needs
// must be offered, with hints, and an unscoped website needs a person's review.
func checkBrowserSignIn(doc *discovery.Poppy, issuer *discovery.AuthorizationServer, webValid bool, c *collector) {
	const scope = setup.BrowserSignInScope

	if !webValid {
		return
	}

	if isTrue(doc.Web.Unscoped) {
		c.warn("D16", specBrowserJoin, "web.unscoped is true: check by hand that the description of "+scope+" says it gives full account access",
			"Describe "+scope+" as access to everything the account can do on the website.")
	}

	if doc.Web.BrowserSessionEndpoint == "" || isTrue(doc.Web.Disabled) || doc.Auth.Disabled {
		return
	}

	_, described := doc.Auth.Scopes[scope]

	switch {
	case described:
		c.pass("D16", specScopeHints, scope+" is listed in auth.scopes")
	case issuer != nil && slices.Contains(issuer.ScopesSupported, scope):
		c.pass("D16", specScopeHints, scope+" is listed in scopes_supported of the issuer")
	default:
		c.warn("D16", specScopeHints, scope+" is in neither auth.scopes nor the scopes_supported of the issuer, and the signed-in browser join needs it",
			"Run: auth0 poppy setup")
	}

	if _, hinted := doc.Auth.ScopeHints[scope]; len(doc.Auth.ScopeHints) > 0 && !hinted && !described {
		c.warn("D16", specScopeHints, "scope_hints has no entry for "+scope,
			fmt.Sprintf("auth0 poppy setup --hint %s=changes:<%s>,personal_data:<true|false>", scope, strings.Join(validChanges, "|")))
	}
}

// checkSupportedVersions runs D17 on supported_versions, when the site has it.
// The URLs are not fetched.
func checkSupportedVersions(doc *discovery.Poppy, finalURL string, c *collector) {
	if doc.SupportedVersions == nil {
		return
	}

	versions, isObject := doc.SupportedVersions.(map[string]any)
	if !isObject {
		c.fail("D17", specVersions, fmt.Sprintf("supported_versions is %s, and it must be an object that maps versions to URLs", describeJSON(doc.SupportedVersions)),
			`Write supported_versions as {"<version>": "https://<host>/<path>"}, or remove it.`)
		return
	}

	host := ""
	if u, err := url.Parse(finalURL); err == nil {
		host = u.Host
	}

	problems := 0
	fail := func(message, fix string) {
		problems++
		c.fail("D17", specVersions, message, fix)
	}

	for _, version := range slices.Sorted(maps.Keys(versions)) {
		value, isString := versions[version].(string)
		name := fmt.Sprintf("supported_versions[%q]", version)

		switch u, err := url.Parse(value); {
		case !isString:
			fail(fmt.Sprintf("%s is %s, and it must be an HTTPS URL", name, describeJSON(versions[version])),
				"Set "+name+" to the HTTPS URL of the document for that version.")
		case err != nil || !isHTTPS(value):
			fail(fmt.Sprintf("%s is not an HTTPS URL: %s", name, value),
				"Publish "+name+" as an HTTPS URL.")
		case !strings.EqualFold(u.Host, host):
			fail(fmt.Sprintf("%s is on %s, and it must be on %s, the host that serves poppy.json", name, u.Host, host),
				"Serve the document of each version on "+host+".")
		}
	}

	if problems == 0 {
		c.pass("D17", specVersions, fmt.Sprintf("supported_versions lists %d other version(s), each an HTTPS URL on %s", len(versions), host))
	}
}

// isTrue tells whether a decoded JSON value is the boolean true.
func isTrue(v any) bool {
	b, ok := v.(bool)

	return ok && b
}

// describeJSON writes a decoded JSON value with its type, so that a string
// "true" cannot be mistaken for the boolean.
func describeJSON(v any) string {
	switch v.(type) {
	case string:
		return fmt.Sprintf("%q (a string)", v)
	case float64:
		return fmt.Sprintf("%v (a number)", v)
	case bool:
		return fmt.Sprintf("%v (a boolean)", v)
	case []any:
		return "a list"
	case map[string]any:
		return "an object"
	}

	return fmt.Sprintf("%v", v)
}
