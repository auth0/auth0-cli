package doctor

import (
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"

	"github.com/auth0/auth0-cli/internal/poppy/config"
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

// rulesOf returns the rules for a protocol version. Version 0.3 has no extra
// checks, and 0.4 and every 1.x have the web and supported_versions checks. A
// version that config.Understands rejects has no extra checks: D1 warns about it
// and only the checks of every version run.
func rulesOf(version string) versionRules {
	switch {
	case !config.Understands(version), version == "0.3":
		return versionRules{}
	default:
		return versionRules{web: true, versions: true}
	}
}

// checkVersionRules runs the checks that the protocol version of the site adds:
// D15 for the web object, D16 for the poppy:browser-sign-in scope and D17 for
// supported_versions. A version that does not add a check has it skipped. D16 is
// skipped when webOff tells that the owner turned the web part off.
func (r *runner) checkVersionRules(doc *discovery.Poppy, finalURL string, issuer *discovery.AuthorizationServer, webOff bool, c *collector) {
	rules := rulesOf(doc.ProtocolVersion)

	if rules.web {
		valid := checkWebFields(doc, c)

		if webOff {
			c.skip("D16.web_off", specBrowserJoin, SkipWebOff, messageWebOff)
		} else {
			r.checkBrowserSignIn(doc, issuer, valid, c)
		}
	} else {
		message := "protocol version " + doc.ProtocolVersion + " has no web settings to check"
		c.skip("D15.protocol_version", specWebFields, SkipProtocolVersion, message)
		c.skip("D16.protocol_version", specBrowserJoin, SkipProtocolVersion, message)
	}

	if rules.versions {
		checkSupportedVersions(doc, finalURL, c)
	} else {
		c.skip("D17.protocol_version", specVersions, SkipProtocolVersion,
			"protocol version "+doc.ProtocolVersion+" has no supported_versions to check")
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
			c.fail("D15.web_field_type", specWebFields, fmt.Sprintf("%s is %s, and it must be true or false", field.name, describeJSON(field.value)),
				hint("Set "+field.name+" to true or false, or remove it."))
		}
	}

	if !valid {
		return false
	}

	problems := 0
	fail := func(code, message, sentence string) {
		problems++
		c.fail("D15."+code, specBrowserJoin, message, hint(sentence))
	}

	if isTrue(web.Disabled) {
		if web.BrowserSessionEndpoint != "" {
			fail("web_disabled_with_endpoint", "web.disabled is true, and web.browser_session_endpoint is listed",
				"Remove web.browser_session_endpoint, or set web.disabled to false.")
		}

		if len(doc.Agent.Protocols) == 0 && len(doc.APIs) == 0 {
			fail("web_disabled_unusable", "web.disabled is true, and poppy.json lists neither agent nor apis",
				"List the Company Agent in agent or the APIs in apis, so that agents have something to use instead of the website.")
		}
	}

	if isTrue(web.Unscoped) && web.BrowserSessionEndpoint == "" {
		fail("unscoped_without_endpoint", "web.unscoped is true, and web.browser_session_endpoint is not listed",
			"List web.browser_session_endpoint, or set web.unscoped to false.")
	}

	if problems == 0 {
		c.pass("D15.web", specWebFields, "the web settings are valid")
	}

	return problems == 0
}

// checkBrowserSignIn runs D16: the scope that the signed-in browser join needs
// must be offered, with hints, and an unscoped website needs a person's review.
func (r *runner) checkBrowserSignIn(doc *discovery.Poppy, issuer *discovery.AuthorizationServer, webValid bool, c *collector) {
	const scope = setup.BrowserSignInScope

	if !webValid {
		c.skip("D16.web_invalid", specBrowserJoin, SkipNotAvailable, "the web settings are invalid")
		return
	}

	if isTrue(doc.Web.Unscoped) {
		c.warn("D16.unscoped_review", specBrowserJoin, "web.unscoped is true: check by hand that the description of "+scope+" says it gives full account access",
			hint("Describe "+scope+" as full access to the account on the website, for example \"Browse the website, and see and do anything your account can do there\"."))
	}

	switch {
	case isTrue(doc.Web.Disabled):
		skipBrowserSignIn(c, "D16.web_off", SkipWebOff, messageWebOff)
		return
	case doc.Auth.Disabled:
		skipBrowserSignIn(c, "D16.api_off", SkipAPIOff, messageAPIOff)
		return
	case doc.Web.BrowserSessionEndpoint == "":
		skipBrowserSignIn(c, "D16.no_browser_session_endpoint", SkipNotAvailable, "poppy.json lists no web.browser_session_endpoint")
		return
	}

	_, described := doc.Auth.Scopes[scope]

	switch {
	case described:
		c.pass("D16.scope_described", specScopeHints, scope+" is listed in auth.scopes")
	case issuer != nil && slices.Contains(issuer.ScopesSupported, scope):
		c.pass("D16.scope_supported", specScopeHints, scope+" is listed in scopes_supported of the issuer")
	default:
		c.warn("D16.scope_missing", specScopeHints, scope+" is in neither auth.scopes nor the scopes_supported of the issuer, and the signed-in browser join needs it",
			r.setupFix())
	}

	if _, hinted := doc.Auth.ScopeHints[scope]; len(doc.Auth.ScopeHints) > 0 && !hinted && !described {
		c.warn("D16.hint_missing", specScopeHints, "scope_hints has no entry for "+scope,
			hint(fmt.Sprintf("Add a hint with auth0 poppy setup --hint %s=changes:<%s>,personal_data:<true|false>.", scope, strings.Join(validChanges, "|"))))
	}
}

// skipBrowserSignIn records that D16 did not run, unless it already said
// something about the scope.
func skipBrowserSignIn(c *collector, code string, reason SkipReason, message string) {
	if !c.has("D16") {
		c.skip(code, specBrowserJoin, reason, message)
	}
}

// checkSupportedVersions runs D17 on supported_versions. The member is optional,
// so a site without it passes. The URLs are not fetched.
func checkSupportedVersions(doc *discovery.Poppy, finalURL string, c *collector) {
	if doc.SupportedVersions == nil {
		c.pass("D17.not_declared", specVersions, "supported_versions is not declared, which is allowed")
		return
	}

	versions, isObject := doc.SupportedVersions.(map[string]any)
	if !isObject {
		c.fail("D17.not_object", specVersions, fmt.Sprintf("supported_versions is %s, and it must be an object that maps versions to URLs", describeJSON(doc.SupportedVersions)),
			hint(`Write supported_versions as {"<version>": "https://<host>/<path>"}, or remove it.`))
		return
	}

	host := ""
	if u, err := url.Parse(finalURL); err == nil {
		host = u.Host
	}

	problems := 0
	fail := func(code, message, sentence string) {
		problems++
		c.fail("D17."+code, specVersions, message, hint(sentence))
	}

	for _, version := range slices.Sorted(maps.Keys(versions)) {
		value, isString := versions[version].(string)
		name := fmt.Sprintf("supported_versions[%q]", version)

		switch u, err := url.Parse(value); {
		case !isString:
			fail("entry_not_string", fmt.Sprintf("%s is %s, and it must be an HTTPS URL", name, describeJSON(versions[version])),
				"Set "+name+" to the HTTPS URL of the document for that version.")
		case err != nil || !isHTTPS(value):
			fail("entry_not_https", fmt.Sprintf("%s is not an HTTPS URL: %s", name, value),
				"Publish "+name+" as an HTTPS URL.")
		case !strings.EqualFold(u.Host, host):
			fail("entry_other_host", fmt.Sprintf("%s is on %s, and it must be on %s, the host that serves poppy.json", name, u.Host, host),
				"Serve the document of each version on "+host+".")
		}
	}

	if problems == 0 {
		c.pass("D17.versions", specVersions, fmt.Sprintf("supported_versions lists %d other version(s), each an HTTPS URL on %s", len(versions), host))
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
