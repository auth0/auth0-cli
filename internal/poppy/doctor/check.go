package doctor

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sync"

	"github.com/auth0/auth0-cli/internal/poppy/discovery"
)

const (
	wellKnownPoppy             = "/.well-known/poppy.json"
	wellKnownProtectedResource = "/.well-known/oauth-protected-resource"
	wellKnownAuthServer        = "/.well-known/oauth-authorization-server"

	poppyUnavailable = "poppy.json is not available"

	// The protocol sections the results point at.
	specDiscovery      = "3"
	specOrganization   = "3.1"
	specVersions       = "3.4"
	specResource       = "3.2"
	specIssuerMetadata = "3.2"
	specAuthEndpoints  = "4.2"
	specSigningAlgs    = "4.1"
	specScopeHints     = "4.7"
	specBrowserJoin    = "6"
	specAPIs           = "7"
)

// Options configures Run.
type Options struct {
	// HTTPClient sends the requests. Nil means a plain client. The checks never
	// send credentials, whatever the client is.
	HTTPClient *http.Client
	// Tenant gives checks D8 to D14 read access to the tenant. Nil means there
	// is no login, and those checks are skipped.
	Tenant *Tenant
	// ConfigFile is the config file that D9 and D14 use. A file that does not
	// exist is not an error.
	ConfigFile string
}

// runner holds what every check needs.
type runner struct {
	client *discovery.Client
	domain string
	origin string
}

// collector gathers the results of one check, in the order they are recorded.
type collector struct {
	results []Result
}

func (c *collector) add(level Level, id, spec, message, fix string) {
	c.results = append(c.results, Result{ID: id, Spec: spec, Level: level, Message: message, Fix: fix})
}

func (c *collector) pass(id, spec, message string) { c.add(Pass, id, spec, message, "") }

func (c *collector) warn(id, spec, message, fix string) { c.add(Warn, id, spec, message, fix) }

func (c *collector) fail(id, spec, message, fix string) { c.add(Fail, id, spec, message, fix) }

func (c *collector) skip(id, spec, message string) { c.add(Skipped, id, spec, message, "") }

// Run checks the discovery chain of a company domain without any credentials
// (checks D1 to D7), then the tenant when the options give access to it (checks
// D8 to D14). The domain must be a bare host name; see discovery.NormalizeDomain.
// A failing request is a failed check, never an error, and the results always
// come in check order.
func Run(ctx context.Context, domain string, opts Options) Report {
	r := &runner{
		client: discovery.New(opts.HTTPClient),
		domain: domain,
		origin: "https://" + domain,
	}

	results, version, issuer := r.checkDiscovery(ctx)
	report := Report{Domain: domain, ProtocolVersion: version, Issuer: issuer, Results: results}
	report.Results = append(report.Results, r.checkTenant(ctx, opts, issuer)...)

	return report
}

// checkDiscovery runs D1 to D7. It also returns the protocol version poppy.json
// declares and the issuer it found, which are empty when poppy.json could not be
// read or the issuer could not be discovered.
func (r *runner) checkDiscovery(ctx context.Context) (results []Result, version, issuer string) {
	var (
		poppyCol, resourceCol collector
		poppy                 *discovery.Poppy
		poppyURL              string
		resource              *discovery.ProtectedResource
		wg                    sync.WaitGroup
	)
	wg.Go(func() { poppy, poppyURL = r.checkPoppy(ctx, &poppyCol) })
	wg.Go(func() { resource = r.checkProtectedResource(ctx, &resourceCol) })
	wg.Wait()

	results = slices.Concat(poppyCol.results, resourceCol.results)

	if poppy == nil {
		var skipped collector
		for _, c := range []struct{ id, spec string }{
			{"D3", specIssuerMetadata}, {"D4", specSigningAlgs}, {"D5", specScopeHints},
			{"D6", specBrowserJoin}, {"D7", specAPIs},
		} {
			skipped.skip(c.id, c.spec, poppyUnavailable)
		}

		return append(results, skipped.results...), "", ""
	}

	var issuerCol collector
	issuer = resolveIssuer(poppy, resource, &issuerCol)
	results = append(results, issuerCol.results...)

	var (
		d3, d4     collector
		issuerMeta *discovery.AuthorizationServer
	)
	if issuer == "" {
		d3.skip("D3", specIssuerMetadata, "the issuer is not known")
		d4.skip("D4", specSigningAlgs, "the issuer is not known")
	} else {
		issuerMeta = r.checkAuthServer(ctx, issuer, poppy, &d3, &d4)
	}
	results = append(results, d3.results...)
	results = append(results, d4.results...)

	var d5, d6, d7 collector
	checkScopes(poppy, &d5)
	r.checkBrowserJoin(ctx, poppy, &d6)
	r.checkAPIs(ctx, poppy, &d7)
	results = append(results, d5.results...)
	results = append(results, d6.results...)
	results = append(results, d7.results...)

	var versioned collector
	checkVersionRules(poppy, poppyURL, issuerMeta, &versioned)
	results = append(results, versioned.results...)

	return results, poppy.ProtocolVersion, issuer
}

// get requests a URL. A transport failure is recorded as a failed check and
// returns nil.
func (r *runner) get(ctx context.Context, c *collector, id, spec, rawURL string) *discovery.Response {
	resp, err := r.client.Get(ctx, rawURL)
	if err == nil {
		return resp
	}

	fix := "Check that " + rawURL + " is reachable over HTTPS."
	var insecure *discovery.InsecureURLError
	switch {
	case errors.As(err, &insecure):
		fix = "Serve the document over HTTPS, and redirect only to HTTPS URLs."
	case errors.Is(err, discovery.ErrTooManyRedirects):
		fix = "Serve the document at " + rawURL + " without a chain of redirects."
	case errors.Is(err, discovery.ErrBodyTooLarge):
		fix = "Serve a smaller document."
	}
	c.fail(id, spec, fmt.Sprintf("%s could not be fetched: %v", rawURL, err), fix)

	return nil
}

// fetch is get for a document that must answer 200.
func (r *runner) fetch(ctx context.Context, c *collector, id, spec, rawURL string) *discovery.Response {
	resp := r.get(ctx, c, id, spec, rawURL)
	if resp == nil {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		c.fail(id, spec, fmt.Sprintf("%s returned HTTP %d", rawURL, resp.StatusCode),
			"Serve the document at "+rawURL+" with status 200.")
		return nil
	}

	return resp
}

// fetchDocument fetches a URL that must answer 200 with a JSON document.
func fetchDocument[T any](ctx context.Context, r *runner, c *collector, id, spec, rawURL string) *T {
	doc, _ := fetchDocumentFrom[T](ctx, r, c, id, spec, rawURL)

	return doc
}

// fetchDocumentFrom is fetchDocument that also returns the URL the document came
// from after any redirects.
func fetchDocumentFrom[T any](ctx context.Context, r *runner, c *collector, id, spec, rawURL string) (*T, string) {
	resp := r.fetch(ctx, c, id, spec, rawURL)
	if resp == nil {
		return nil, ""
	}

	doc, err := discovery.Decode[T](resp.Body)
	if err != nil {
		c.fail(id, spec, fmt.Sprintf("%s is not a valid document: %v", rawURL, err),
			"Serve a JSON document at "+rawURL+".")
		return nil, ""
	}

	return doc, resp.URL
}
