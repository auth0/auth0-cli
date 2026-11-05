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

const (
	joinCookieName = "__Host-poppy_join"

	tenantSettingsFix = "Run auth0 poppy setup to turn on the Poppy settings of the tenant, and contact support if the value stays wrong."
)

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

// checkAuthServer fetches the authorization server metadata of the issuer and
// runs D3 and D4 on it. It returns the metadata, or nil when it is not available.
func (r *runner) checkAuthServer(ctx context.Context, issuer string, poppy *discovery.Poppy, d3, d4 *collector) *discovery.AuthorizationServer {
	rawURL := strings.TrimSuffix(issuer, "/") + wellKnownAuthServer
	doc := fetchDocument[discovery.AuthorizationServer](ctx, r, d3, "D3", specIssuerMetadata, rawURL)
	if doc == nil {
		d4.skip("D4", specSigningAlgs, "the authorization server metadata is not available")
		return nil
	}

	r.checkIssuerMetadata(issuer, doc, poppy, d3)
	checkSigningAlgs(doc, d4)

	return doc
}

// checkIssuerMetadata runs D3 on the authorization server metadata.
func (r *runner) checkIssuerMetadata(issuer string, doc *discovery.AuthorizationServer, poppy *discovery.Poppy, c *collector) {
	if doc.Issuer == issuer {
		c.pass("D3", specIssuerMetadata, "issuer matches "+issuer)
	} else {
		c.fail("D3", specIssuerMetadata, fmt.Sprintf("issuer is %q, and it must match %q character for character", doc.Issuer, issuer),
			"Make the issuer in Auth0's metadata and the one named by poppy.json or the protected resource metadata identical, trailing slash included.")
	}

	if slices.Contains(doc.ProtectedResources, r.origin) {
		c.pass("D3", specIssuerMetadata, "protected_resources lists "+r.origin)
	} else {
		c.fail("D3", specIssuerMetadata, "protected_resources does not list "+r.origin,
			"Run auth0 poppy setup --company-domain "+r.domain)
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
			c.fail("D3", specAuthEndpoints, e.name+" is missing", tenantSettingsFix)
		case !isHTTPS(e.value):
			problems++
			c.fail("D3", specAuthEndpoints, fmt.Sprintf("%s is not an HTTPS URL: %s", e.name, e.value), tenantSettingsFix)
		}
	}
	if problems == 0 {
		c.pass("D3", specAuthEndpoints, "token, revocation, authorization and device endpoints are HTTPS")
	}

	problems = 0
	for _, grant := range requiredGrantTypes {
		if !slices.Contains(doc.GrantTypesSupported, grant) {
			problems++
			c.fail("D3", specAuthEndpoints, "grant_types_supported does not include "+grant, tenantSettingsFix)
		}
	}
	if problems == 0 {
		c.pass("D3", specAuthEndpoints, "supports the grants agents use (authorization code, refresh token, device code)")
	}

	if doc.AuthorizationResponseIssParameterSupported {
		c.pass("D3", specAuthEndpoints, "returns iss in sign-in responses")
	} else {
		c.fail("D3", specAuthEndpoints, "authorization_response_iss_parameter_supported is not true",
			"Turn on the iss parameter in the tenant settings: run auth0 poppy setup.")
	}

	missing := slices.DeleteFunc(slices.Sorted(maps.Keys(poppy.Auth.Scopes)), func(scope string) bool {
		return slices.Contains(doc.ScopesSupported, scope)
	})
	if len(missing) == 0 {
		c.pass("D3", specAuthEndpoints, "lists every scope of poppy.json")
	} else {
		c.warn("D3", specAuthEndpoints, "scopes_supported does not list: "+strings.Join(missing, ", "),
			"Add these scopes to a Poppy API of the tenant, or remove them from auth.scopes in poppy.json.")
	}
}

// checkSigningAlgs runs D4 on the authorization server metadata.
func checkSigningAlgs(doc *discovery.AuthorizationServer, c *collector) {
	algs := doc.TokenEndpointAuthSigningAlgValuesSupported

	if slices.Contains(algs, "ES256") {
		c.pass("D4", specSigningAlgs, "accepts ES256 client assertions")
	} else {
		c.fail("D4", specSigningAlgs, "client assertions: ES256 not accepted by the issuer",
			"Agents must sign with ES256. Contact support if the tenant does not list it.")
	}

	if slices.Contains(algs, "PS384") {
		c.pass("D4", specSigningAlgs, "accepts PS384 client assertions")
	} else {
		c.results = append(c.results, Result{
			ID:                 "D4",
			Spec:               specSigningAlgs,
			Level:              Warn,
			Message:            "client assertions: PS384 not accepted by the issuer",
			Fix:                "PS384 needs the HRI SKU. Contact support.",
			PlatformDependency: true,
		})
	}
}

// checkBrowserJoin runs D6 with a single GET of the browser session endpoint.
func (r *runner) checkBrowserJoin(ctx context.Context, poppy *discovery.Poppy, c *collector) {
	rawURL := poppy.Web.BrowserSessionEndpoint
	if rawURL == "" && rulesOf(poppy).web && isTrue(poppy.Web.Disabled) {
		c.pass("D6", specBrowserJoin, "web.disabled is true, so no browser join is expected")
		return
	}

	if rawURL == "" {
		c.warn("D6", specBrowserJoin, "web.browser_session_endpoint is not published, so agents cannot join a browser session",
			"Publish web.browser_session_endpoint in poppy.json if agents should open signed-in browser sessions.")
		return
	}

	resp := r.get(ctx, c, "D6", specBrowserJoin, rawURL)
	switch {
	case resp == nil:
		return
	case resp.StatusCode == http.StatusNotFound:
		c.fail("D6", specBrowserJoin, rawURL+" returned HTTP 404: the browser join may be off for the web app, or the tenant may lack the beta",
			"Turn on the browser join for the web app with auth0 poppy setup, and ask Auth0 about the beta if the tenant should have it.")
		return
	case resp.StatusCode != http.StatusOK:
		c.fail("D6", specBrowserJoin, fmt.Sprintf("%s returned HTTP %d", rawURL, resp.StatusCode),
			"Serve the browser join at "+rawURL+" with status 200.")
		return
	}

	if hasHiddenCodeInput(string(resp.Body)) {
		c.pass("D6", specBrowserJoin, "the join page has a hidden code input")
	} else {
		c.fail("D6", specBrowserJoin, "browser join does not serve a form with a hidden code input",
			"Serve an HTML form that holds a hidden input named code.")
	}

	checkJoinCookie(resp.Header, c)

	if strings.Contains(strings.ToLower(strings.Join(resp.Header.Values("Cache-Control"), ",")), "no-store") {
		c.pass("D6", specBrowserJoin, "the join response is not cached")
	} else {
		c.warn("D6", specBrowserJoin, "browser join response does not send Cache-Control: no-store",
			"Send Cache-Control: no-store so the one-time code is never cached.")
	}
}

func checkJoinCookie(header http.Header, c *collector) {
	cookies := (&http.Response{Header: header}).Cookies()
	i := slices.IndexFunc(cookies, func(ck *http.Cookie) bool { return ck.Name == joinCookieName })
	if i < 0 {
		c.fail("D6", specBrowserJoin, "browser join does not set the "+joinCookieName+" cookie",
			"Set the "+joinCookieName+" cookie with Secure, HttpOnly, Path=/, SameSite=Strict and a Max-Age.")
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
		c.pass("D6", specBrowserJoin, "the join cookie has the required attributes")
		return
	}
	c.fail("D6", specBrowserJoin, fmt.Sprintf("the %s cookie lacks: %s", joinCookieName, strings.Join(lacking, ", ")),
		"Set the "+joinCookieName+" cookie with Secure, HttpOnly, Path=/, SameSite=Strict and a Max-Age.")
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
