package doctor

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/poppy/discovery"
)

const (
	wellKnownPoppy             = "/.well-known/poppy.json"
	wellKnownProtectedResource = "/.well-known/oauth-protected-resource"
	wellKnownAuthServer        = "/.well-known/oauth-authorization-server"

	poppyUnavailable = "poppy.json is not available"

	// The messages of a check that is skipped when the owner turned a part of Poppy off.
	messageWebOff = "web is turned off"
	messageAPIOff = "API access is turned off"

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
	// ConfigFileExplicit tells that the user named ConfigFile, rather than the
	// default name being used. Without a file at the default name, D14 compares
	// the poppy.json the website serves with the tenant; a file the user named
	// that does not exist skips it.
	ConfigFileExplicit bool
	// TenantFlag is the tenant the user chose with --tenant, which the fix
	// commands repeat. Empty when they did not choose one.
	TenantFlag string
	// TenantOnly runs only the checks that read the tenant (D8 to D14), such as
	// right after setup, when the website has not been deployed yet.
	TenantOnly bool
}

// runner holds what every check needs.
type runner struct {
	client *discovery.Client
	domain string
	origin string

	// ConfigFile and tenantFlag are what the fix commands repeat.
	configFile string
	tenantFlag string

	// FileWebOff and fileAPIOff tell that the config file has web.disabled or
	// auth.disabled. LiveWebOff and liveAPIOff tell the same of the poppy.json the
	// website serves. A part that is off has its checks skipped.
	fileWebOff, fileAPIOff bool
	liveWebOff, liveAPIOff bool

	// LiveConfig is the poppy.json the website serves, read as a config file, for
	// D14 when there is no config file to compare. LiveConfigErr is why it could
	// not be read as one. Both are empty when D1 did not get a document.
	liveConfig    *config.Config
	liveConfigErr error
}

// fix is how to repair a result: a command that can be run as it is, a sentence
// on what to change, or both.
type fix struct {
	command string
	hint    string
}

// hint is a fix that is a sentence.
func hint(sentence string) fix { return fix{hint: sentence} }

// command is a fix that is a command.
func command(line string) fix { return fix{command: line} }

// collector gathers the results of one check, in the order they are recorded.
type collector struct {
	results []Result
}

// add records a result. The code names the finding and starts with the ID of the
// check, such as "D8.iss_parameter_off".
func (c *collector) add(level Level, code, spec, message string, f fix) {
	id, _, _ := strings.Cut(code, ".")
	c.results = append(c.results, Result{ID: id, Code: code, Spec: spec, Level: level, Message: message, FixCommand: f.command, FixHint: f.hint})
}

func (c *collector) pass(code, spec, message string) { c.add(Pass, code, spec, message, fix{}) }

func (c *collector) warn(code, spec, message string, f fix) { c.add(Warn, code, spec, message, f) }

func (c *collector) fail(code, spec, message string, f fix) { c.add(Fail, code, spec, message, f) }

// platform records a result that only Auth0 can fix, with a sentence that says
// what to ask for.
func (c *collector) platform(level Level, code, spec, message, sentence string) {
	c.add(level, code, spec, message, hint(sentence))
	c.results[len(c.results)-1].PlatformDependency = true
}

func (c *collector) skip(code, spec string, reason SkipReason, message string) {
	c.skipWith(code, spec, reason, message, fix{})
}

// skipWith records a skipped check with a fix that makes it run.
func (c *collector) skipWith(code, spec string, reason SkipReason, message string, f fix) {
	c.add(Skipped, code, spec, message, f)
	c.results[len(c.results)-1].SkipReason = reason
}

// has tells whether a result of the check was recorded.
func (c *collector) has(id string) bool {
	return slices.ContainsFunc(c.results, func(res Result) bool { return res.ID == id })
}

// Run checks the discovery chain of a company domain without any credentials
// (checks D1 to D7 and D15 to D17), then the tenant when the options give access
// to it (checks D8 to D14). The domain must be a bare host name; see
// config.NormalizeDomain. A failing request is a failed check, never an
// error, and the results always come in check order. Every check of the run
// appears at least once, as a skipped result when it did not run.
func Run(ctx context.Context, domain string, opts Options) Report {
	r := &runner{
		client:     discovery.New(opts.HTTPClient),
		domain:     domain,
		origin:     "https://" + domain,
		configFile: opts.ConfigFile,
		tenantFlag: opts.TenantFlag,
	}

	file, fileErr := readConfigFile(opts.ConfigFile)
	if fileErr == nil && file != nil {
		r.fileWebOff, r.fileAPIOff = file.Web.Disabled, file.Auth.Disabled
	}

	report := Report{Domain: domain, TenantOnly: opts.TenantOnly}
	checked := []string{"D8", "D9", "D10", "D11", "D12", "D13", "D14"}

	if !opts.TenantOnly {
		report.Results, report.ProtocolVersion, report.Issuer = r.checkDiscovery(ctx)
		checked = append([]string{"D1", "D2", "D3", "D4", "D5", "D6", "D7", "D15", "D16", "D17"}, checked...)
	}

	tenant, ran := r.checkTenant(ctx, opts, report.Issuer, file, fileErr)
	report.Results = append(report.Results, tenant...)
	report.TenantChecked = ran
	report.Results = withEveryCheck(report.Results, checked)
	report.Totals = report.counts()

	return report
}

// withEveryCheck adds a skipped result for each check that has none, so that a
// script finds every check in the report, and puts the results in the order of
// the checks. The checks themselves record why they did not run; this only
// covers the ones that said nothing.
func withEveryCheck(results []Result, ids []string) []Result {
	var missing collector
	for _, id := range ids {
		if !slices.ContainsFunc(results, func(res Result) bool { return res.ID == id }) {
			missing.skip(id+".not_run", "", SkipNotAvailable, "nothing to check")
		}
	}

	results = append(results, missing.results...)
	slices.SortStableFunc(results, func(a, b Result) int { return slices.Index(ids, a.ID) - slices.Index(ids, b.ID) })

	return results
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
			{"D15", specWebFields}, {"D16", specBrowserJoin}, {"D17", specVersions},
		} {
			skipped.skip(c.id+".poppy_unavailable", c.spec, SkipNotAvailable, poppyUnavailable)
		}

		return append(results, skipped.results...), "", ""
	}

	r.liveWebOff, r.liveAPIOff = rulesOf(poppy.ProtocolVersion).web && isTrue(poppy.Web.Disabled), poppy.Auth.Disabled

	var issuerCol collector
	issuer = resolveIssuer(poppy, resource, &issuerCol)
	results = append(results, issuerCol.results...)

	var (
		d3, d4     collector
		issuerMeta *discovery.AuthorizationServer
	)
	if issuer == "" {
		d3.skip("D3.issuer_unknown", specIssuerMetadata, SkipNotAvailable, "the issuer is not known")
		d4.skip("D4.issuer_unknown", specSigningAlgs, SkipNotAvailable, "the issuer is not known")
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
	r.checkVersionRules(poppy, poppyURL, issuerMeta, r.fileWebOff, &versioned)
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

	code, sentence := "unreachable", "Check that "+rawURL+" is reachable over HTTPS."
	var insecure *discovery.InsecureURLError
	switch {
	case errors.As(err, &insecure):
		code, sentence = "insecure_url", "Serve the document over HTTPS, and redirect only to HTTPS URLs."
	case errors.Is(err, discovery.ErrTooManyRedirects):
		code, sentence = "too_many_redirects", "Serve the document at "+rawURL+" without a chain of redirects."
	case errors.Is(err, discovery.ErrBodyTooLarge):
		code, sentence = "body_too_large", "Serve a smaller document."
	}
	c.fail(id+"."+code, spec, fmt.Sprintf("%s could not be fetched: %v", rawURL, err), hint(sentence))

	return nil
}

// fetch is get for a document that must answer 200.
func (r *runner) fetch(ctx context.Context, c *collector, id, spec, rawURL string) *discovery.Response {
	resp := r.get(ctx, c, id, spec, rawURL)
	if resp == nil {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		c.fail(id+".http_status", spec, fmt.Sprintf("%s returned HTTP %d", rawURL, resp.StatusCode),
			hint("Serve the document at "+rawURL+" with status 200."))
		return nil
	}

	return resp
}

// fetchDocument fetches a URL that must answer 200 with a JSON document.
func fetchDocument[T any](ctx context.Context, r *runner, c *collector, id, spec, rawURL string) *T {
	doc, _ := fetchDocumentFrom[T](ctx, r, c, id, spec, rawURL)

	return doc
}

// fetchDocumentFrom is fetchDocument that also returns the response the document
// came in, which holds the URL it came from after any redirects.
func fetchDocumentFrom[T any](ctx context.Context, r *runner, c *collector, id, spec, rawURL string) (*T, *discovery.Response) {
	resp := r.fetch(ctx, c, id, spec, rawURL)
	if resp == nil {
		return nil, nil
	}

	doc, err := discovery.Decode[T](resp.Body)
	if err != nil {
		c.fail(id+".invalid_document", spec, fmt.Sprintf("%s is not a valid document: %v", rawURL, err),
			hint("Serve a JSON document at "+rawURL+"."))
		return nil, nil
	}

	return doc, resp
}

// auth0 writes a command line of the CLI. It repeats the tenant the user chose
// with --tenant, so that the command acts on the same tenant.
func (r *runner) auth0(args ...string) string {
	if r.tenantFlag != "" {
		args = append(args, "--tenant", quoteArg(r.tenantFlag))
	}

	return "auth0 " + strings.Join(args, " ")
}

// outFlag is the --out of a command that writes the config file, when the doctor
// reads a file other than the default.
func (r *runner) outFlag() []string {
	if r.configFile == "" || r.configFile == config.DefaultFileName {
		return nil
	}

	return []string{"--out", quoteArg(r.configFile)}
}

// setupCommand is auth0 poppy setup for the domain. The extra words come before
// the flags that every setup command has.
func (r *runner) setupCommand(extra ...string) string {
	args := append([]string{"poppy", "setup", "--company-domain", quoteArg(r.domain)}, extra...)
	args = append(args, r.outFlag()...)

	return r.auth0(append(args, "--force")...)
}

// configCommand is auth0 poppy config for the domain, which writes the config
// file again from the tenant.
func (r *runner) configCommand() string {
	return r.auth0(append([]string{"poppy", "config", "--company-domain", quoteArg(r.domain)}, r.outFlag()...)...)
}

// setupFix is a fix that is the setup command.
func (r *runner) setupFix(extra ...string) fix { return command(r.setupCommand(extra...)) }

// quoteArg quotes a word of a command line when a shell would split or change it.
func quoteArg(word string) string {
	if word != "" && strings.IndexFunc(word, func(c rune) bool {
		return (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && !strings.ContainsRune("-_.,/:=@+%", c)
	}) < 0 {
		return word
	}

	return "'" + strings.ReplaceAll(word, "'", `'\''`) + "'"
}
