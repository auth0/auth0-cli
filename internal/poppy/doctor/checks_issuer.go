package doctor

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/auth0/auth0-cli/internal/poppy/discovery"
)

const joinCookieName = "__Host-poppy_join"

var (
	requiredGrantTypes = []string{
		"authorization_code",
		"refresh_token",
		"urn:ietf:params:oauth:grant-type:device_code",
	}

	formTag    = regexp.MustCompile(`(?i)<form[\s>]`)
	inputTag   = regexp.MustCompile(`(?is)<input\b[^>]*>`)
	hiddenType = regexp.MustCompile(`(?i)\btype\s*=\s*["']?hidden\b`)
	codeName   = regexp.MustCompile(`(?i)\bname\s*=\s*["']?code["'\s/>]`)
)

// tenantSettingsFix turns on the Poppy settings of the tenant.
func (r *runner) tenantSettingsFix() fix {
	return fix{command: r.setupCommand(), hint: "Setup turns on the Poppy settings of the tenant. Contact support if the value stays wrong."}
}

// checkAuthServer fetches the authorization server metadata of the issuer and
// runs D3 and D4 on it. It returns the metadata, or nil when it is not available.
func (r *runner) checkAuthServer(ctx context.Context, issuer string, poppy *discovery.Poppy, d3, d4 *collector) *discovery.AuthorizationServer {
	rawURL := strings.TrimSuffix(issuer, "/") + wellKnownAuthServer
	doc := fetchDocument[discovery.AuthorizationServer](ctx, r, d3, "D3", specIssuerMetadata, rawURL)
	if doc == nil {
		d4.skip("D4.metadata_unavailable", specSigningAlgs, SkipNotAvailable, "the authorization server metadata is not available")
		return nil
	}

	r.checkIssuerMetadata(issuer, doc, poppy, d3)
	checkSigningAlgs(doc, d4)

	return doc
}

// checkIssuerMetadata runs D3 on the authorization server metadata.
func (r *runner) checkIssuerMetadata(issuer string, doc *discovery.AuthorizationServer, poppy *discovery.Poppy, c *collector) {
	if doc.Issuer == issuer {
		c.pass("D3.issuer", specIssuerMetadata, "issuer matches "+issuer)
	} else {
		c.fail("D3.issuer_mismatch", specIssuerMetadata, fmt.Sprintf("issuer is %q, and it must match %q character for character", doc.Issuer, issuer),
			hint("Make the issuer in Auth0's metadata and the one named by poppy.json or the protected resource metadata identical, trailing slash included."))
	}

	switch {
	case slices.Contains(doc.ProtectedResources, r.origin):
		c.pass("D3.protected_resources", specIssuerMetadata, "protected_resources lists "+r.origin)
	case len(doc.ProtectedResources) == 0:
		// Auth0 lists the company domains here. A tenant that lists none does not
		// publish the field yet, and no setting of the tenant changes that.
		c.platform(Fail, "D3.protected_resources_unpublished", specIssuerMetadata, "the issuer does not publish protected_resources, so agents cannot confirm "+r.origin+" from Auth0",
			"Auth0 publishes protected_resources for tenants with the Poppy beta. Contact support.")
	default:
		c.fail("D3.protected_resources_missing", specIssuerMetadata, "protected_resources does not list "+r.origin,
			r.setupFix())
	}

	endpoints := []struct{ name, value string }{
		{"token_endpoint", doc.TokenEndpoint},
		{"revocation_endpoint", doc.RevocationEndpoint},
		{"authorization_endpoint", doc.AuthorizationEndpoint},
		{"device_authorization_endpoint", doc.DeviceAuthorizationEndpoint},
	}
	problems := 0
	for _, e := range endpoints {
		switch {
		case e.value == "":
			problems++
			c.fail("D3.endpoint_missing", specAuthEndpoints, e.name+" is missing", r.tenantSettingsFix())
		case !isHTTPS(e.value):
			problems++
			c.fail("D3.endpoint_not_https", specAuthEndpoints, fmt.Sprintf("%s is not an HTTPS URL: %s", e.name, e.value), r.tenantSettingsFix())
		}
	}
	if problems == 0 {
		c.pass("D3.endpoints", specAuthEndpoints, "token, revocation, authorization and device endpoints are HTTPS")
	}

	problems = 0
	for _, grant := range requiredGrantTypes {
		if !slices.Contains(doc.GrantTypesSupported, grant) {
			problems++
			c.fail("D3.grant_type_missing", specAuthEndpoints, "grant_types_supported does not include "+grant, r.tenantSettingsFix())
		}
	}
	if problems == 0 {
		c.pass("D3.grant_types", specAuthEndpoints, "supports the grants agents use (authorization code, refresh token, device code)")
	}

	if doc.AuthorizationResponseIssParameterSupported {
		c.pass("D3.iss_parameter", specAuthEndpoints, "returns iss in sign-in responses")
	} else {
		c.fail("D3.iss_parameter_off", specAuthEndpoints, "authorization_response_iss_parameter_supported is not true",
			fix{command: r.setupCommand(), hint: "Setup turns on the iss parameter in the tenant settings."})
	}

	scopes := slices.Sorted(maps.Keys(poppy.Auth.Scopes))
	missing := slices.DeleteFunc(slices.Clone(scopes), func(scope string) bool {
		return slices.Contains(doc.ScopesSupported, scope)
	})
	switch {
	case len(missing) == 0:
		c.pass("D3.scopes", specAuthEndpoints, "lists every scope of poppy.json")
	case len(missing) == len(scopes):
		// None is listed: the issuer lists no scope of a Poppy API, which a tenant
		// setting cannot change.
		c.platform(Warn, "D3.scopes_unpublished", specAuthEndpoints, "scopes_supported lists no scope of poppy.json, so agents cannot pick scopes from Auth0's metadata",
			"Auth0 lists the scopes of Poppy APIs in scopes_supported for tenants with the Poppy beta. Contact support.")
	default:
		c.warn("D3.scopes_missing", specAuthEndpoints, "scopes_supported does not list: "+strings.Join(missing, ", "),
			hint("Add these scopes to a Poppy API of the tenant, or remove them from auth.scopes in poppy.json."))
	}
}

// checkSigningAlgs runs D4 on the authorization server metadata.
func checkSigningAlgs(doc *discovery.AuthorizationServer, c *collector) {
	algs := doc.TokenEndpointAuthSigningAlgValuesSupported

	if slices.Contains(algs, "ES256") {
		c.pass("D4.es256", specSigningAlgs, "accepts ES256 client assertions")
	} else {
		c.platform(Fail, "D4.es256_missing", specSigningAlgs, "client assertions: ES256 not accepted by the issuer",
			"Agents must sign with ES256. Contact support if the tenant does not list it.")
	}

	if slices.Contains(algs, "PS384") {
		c.pass("D4.ps384", specSigningAlgs, "accepts PS384 client assertions")
	} else {
		c.platform(Warn, "D4.ps384_missing", specSigningAlgs, "client assertions: PS384 not accepted by the issuer",
			"PS384 needs the HRI SKU. Contact support.")
	}
}

// checkBrowserJoin runs D6 with a single GET of the browser session endpoint.
func (r *runner) checkBrowserJoin(ctx context.Context, poppy *discovery.Poppy, c *collector) {
	rawURL := poppy.Web.BrowserSessionEndpoint
	if rawURL == "" && rulesOf(poppy.ProtocolVersion).web && isTrue(poppy.Web.Disabled) {
		c.pass("D6.web_disabled", specBrowserJoin, "web.disabled is true, so no browser join is expected")
		return
	}

	if rawURL == "" && r.fileWebOff {
		c.skip("D6.web_off", specBrowserJoin, SkipWebOff, messageWebOff)
		return
	}

	if rawURL == "" {
		c.warn("D6.endpoint_unpublished", specBrowserJoin, "web.browser_session_endpoint is not published, so agents cannot join a browser session",
			hint("Publish web.browser_session_endpoint in poppy.json if agents should open signed-in browser sessions."))
		return
	}

	resp := r.get(ctx, c, "D6", specBrowserJoin, rawURL)
	switch {
	case resp == nil:
		return
	case resp.StatusCode == http.StatusNotFound:
		c.fail("D6.not_found", specBrowserJoin, rawURL+" returned HTTP 404: the browser join may be off for the web app, or the tenant may lack the beta",
			fix{command: r.setupCommand(), hint: "Setup turns on the browser join for the web app. Ask Auth0 about the beta if the tenant should have it."})
		return
	case resp.StatusCode != http.StatusOK:
		// Auth0 serves this page, so the user cannot serve it: setup turns the
		// browser join on, and anything else is the tenant's beta.
		c.fail("D6.http_status", specBrowserJoin, fmt.Sprintf("%s returned HTTP %d, and a join page is expected", rawURL, resp.StatusCode),
			fix{command: r.setupCommand(), hint: "Auth0 serves this page. Setup turns on the browser join for the web app; if it is on, ask Auth0 whether the tenant has the browser join beta."})
		return
	}

	if hasHiddenCodeInput(string(resp.Body)) {
		c.pass("D6.code_input", specBrowserJoin, "the join page has a hidden code input")
	} else {
		c.fail("D6.code_input_missing", specBrowserJoin, "browser join does not serve a form with a hidden code input",
			hint("Serve an HTML form that holds a hidden input named code."))
	}

	checkJoinCookie(resp.Header, c)

	if strings.Contains(strings.ToLower(strings.Join(resp.Header.Values("Cache-Control"), ",")), "no-store") {
		c.pass("D6.not_cached", specBrowserJoin, "the join response is not cached")
	} else {
		c.warn("D6.cache_control", specBrowserJoin, "browser join response does not send Cache-Control: no-store",
			hint("Send Cache-Control: no-store so the one-time code is never cached."))
	}
}

func checkJoinCookie(header http.Header, c *collector) {
	cookies := (&http.Response{Header: header}).Cookies()
	i := slices.IndexFunc(cookies, func(ck *http.Cookie) bool { return ck.Name == joinCookieName })
	if i < 0 {
		c.fail("D6.cookie_missing", specBrowserJoin, "browser join does not set the "+joinCookieName+" cookie",
			hint("Set the "+joinCookieName+" cookie with Secure, HttpOnly, Path=/, SameSite=Strict and a Max-Age."))
		return
	}

	cookie := cookies[i]
	var lacking []string
	for _, flag := range []struct {
		name string
		ok   bool
	}{
		{"Secure", cookie.Secure},
		{"HttpOnly", cookie.HttpOnly},
		{"Path=/", cookie.Path == "/"},
		{"SameSite=Strict", cookie.SameSite == http.SameSiteStrictMode},
		{"Max-Age", cookie.MaxAge > 0},
	} {
		if !flag.ok {
			lacking = append(lacking, flag.name)
		}
	}

	if len(lacking) == 0 {
		c.pass("D6.cookie", specBrowserJoin, "the join cookie has the required attributes")
		return
	}
	c.fail("D6.cookie_attributes", specBrowserJoin, fmt.Sprintf("the %s cookie lacks: %s", joinCookieName, strings.Join(lacking, ", ")),
		hint("Set the "+joinCookieName+" cookie with Secure, HttpOnly, Path=/, SameSite=Strict and a Max-Age."))
}

// hasHiddenCodeInput reports whether the HTML holds a form with a hidden input named code.
func hasHiddenCodeInput(body string) bool {
	if !formTag.MatchString(body) {
		return false
	}

	return slices.ContainsFunc(inputTag.FindAllString(body, -1), func(tag string) bool {
		return hiddenType.MatchString(tag) && codeName.MatchString(tag)
	})
}
