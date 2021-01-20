package management

//go:generate go run gen-methods.go

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/oauth2"
	"gopkg.in/auth0.v5/internal/client"
)

type ManagementOption func(*Management)

// WithDebug configures the management client to dump http requests and
// responses to stdout.
func WithDebug(d bool) ManagementOption {
	return func(m *Management) {
		m.debug = d
	}
}

// WitContext configures the management client to use the provided context
// instead of the provided one.
func WithContext(ctx context.Context) ManagementOption {
	return func(m *Management) {
		m.ctx = ctx
	}
}

// WithUserAgent configures the management client to use the provided user agent
// string instead of the default one.
func WithUserAgent(userAgent string) ManagementOption {
	return func(m *Management) {
		m.userAgent = userAgent
	}
}

// WithClientCredentials configures management to authenticate using the client
// credentials authentication flow.
func WithClientCredentials(clientID, clientSecret string) ManagementOption {
	return func(m *Management) {
		m.tokenSource = client.ClientCredentials(m.ctx, m.url.String(), clientID, clientSecret)
	}
}

// WithStaticToken configures management to authenticate using a static
// authentication token.
func WithStaticToken(token string) ManagementOption {
	return func(m *Management) {
		m.tokenSource = client.StaticToken(token)
	}
}

// WithInsecure configures management to not use an authentication token and
// use HTTP instead of HTTPS.
//
// This options is available for testing purposes and should not be used in
// production.
func WithInsecure() ManagementOption {
	return func(m *Management) {
		m.tokenSource = client.StaticToken("insecure")
		m.url.Scheme = "http"
	}
}

// WithClient configures management to use the provided client.
func WithClient(client *http.Client) ManagementOption {
	return func(m *Management) {
		m.http = client
	}
}

// Management is an Auth0 management client used to interact with the Auth0
// Management API v2.
//
type Management struct {
	// Client manages Auth0 Client (also known as Application) resources.
	Client *ClientManager

	// ClientGrant manages Auth0 ClientGrant resources.
	ClientGrant *ClientGrantManager

	// ResourceServer manages Auth0 Resource Server (also known as API)
	// resources.
	ResourceServer *ResourceServerManager

	// Connection manages Auth0 Connection resources.
	Connection *ConnectionManager

	// CustomDomain manages Auth0 Custom Domains.
	CustomDomain *CustomDomainManager

	// Grant manages Auth0 Grants.
	Grant *GrantManager

	// Log reads Auth0 Logs.
	Log *LogManager

	// LogStream reads Auth0 Logs.
	LogStream *LogStreamManager

	// RoleManager manages Auth0 Roles.
	Role *RoleManager

	// RuleManager manages Auth0 Rules.
	Rule *RuleManager

	// HookManager manages Auth0 Hooks
	Hook *HookManager

	// RuleManager manages Auth0 Rule Configurations.
	RuleConfig *RuleConfigManager

	// Email manages Auth0 Email Providers.
	Email *EmailManager

	// EmailTemplate manages Auth0 Email Templates.
	EmailTemplate *EmailTemplateManager

	// User manages Auth0 User resources.
	User *UserManager

	// Job manages Auth0 jobs.
	Job *JobManager

	// Tenant manages your Auth0 Tenant.
	Tenant *TenantManager

	// Ticket creates verify email or change password tickets.
	Ticket *TicketManager

	// Stat is used to retrieve usage statistics.
	Stat *StatManager

	// Branding settings such as company logo or primary color.
	Branding *BrandingManager

	// Guardian manages your Auth0 Guardian settings
	Guardian *GuardianManager

	// Prompt manages your prompt settings.
	Prompt *PromptManager

	// Blacklist manages the auth0 blacklists
	Blacklist *BlacklistManager

	url         *url.URL
	basePath    string
	userAgent   string
	debug       bool
	ctx         context.Context
	tokenSource oauth2.TokenSource
	http        *http.Client
}

// New creates a new Auth0 Management client by authenticating using the
// supplied client id and secret.
func New(domain string, options ...ManagementOption) (*Management, error) {

	// Ignore the scheme if it was defined in the domain variable. Then prefix
	// with https as its the only scheme supported by the Auth0 API.
	if i := strings.Index(domain, "//"); i != -1 {
		domain = domain[i+2:]
	}
	domain = "https://" + domain

	u, err := url.Parse(domain)
	if err != nil {
		return nil, err
	}

	m := &Management{
		url:       u,
		basePath:  "api/v2",
		userAgent: client.UserAgent,
		debug:     false,
		ctx:       context.Background(),
		http:      http.DefaultClient,
	}

	for _, option := range options {
		option(m)
	}

	m.http = client.Wrap(m.http, m.tokenSource,
		client.WithDebug(m.debug),
		client.WithUserAgent(m.userAgent),
		client.WithRateLimit())

	m.Client = newClientManager(m)
	m.ClientGrant = newClientGrantManager(m)
	m.Connection = newConnectionManager(m)
	m.CustomDomain = newCustomDomainManager(m)
	m.Grant = newGrantManager(m)
	m.LogStream = newLogStreamManager(m)
	m.Log = newLogManager(m)
	m.ResourceServer = newResourceServerManager(m)
	m.Role = newRoleManager(m)
	m.Rule = newRuleManager(m)
	m.Hook = newHookManager(m)
	m.RuleConfig = newRuleConfigManager(m)
	m.EmailTemplate = newEmailTemplateManager(m)
	m.Email = newEmailManager(m)
	m.User = newUserManager(m)
	m.Job = newJobManager(m)
	m.Tenant = newTenantManager(m)
	m.Ticket = newTicketManager(m)
	m.Stat = newStatManager(m)
	m.Branding = newBrandingManager(m)
	m.Guardian = newGuardianManager(m)
	m.Prompt = newPromptManager(m)
	m.Blacklist = newBlacklistManager(m)

	return m, nil
}

// URI returns the absolute URL of the Management API with any path segments
// appended to the end.
func (m *Management) URI(path ...string) string {
	return (&url.URL{
		Scheme: m.url.Scheme,
		Host:   m.url.Host,
		Path:   m.basePath + "/" + strings.Join(path, "/"),
	}).String()
}

// NewRequest returns a new HTTP request. If the payload is not nil it will be
// encoded as JSON.
func (m *Management) NewRequest(method, uri string, payload interface{}, options ...RequestOption) (r *http.Request, err error) {

	var buf bytes.Buffer
	if payload != nil {
		err := json.NewEncoder(&buf).Encode(payload)
		if err != nil {
			return nil, err
		}
	}

	r, err = http.NewRequest(method, uri, &buf)
	if err != nil {
		return nil, err
	}
	r.Header.Add("Content-Type", "application/json")

	for _, option := range options {
		option.apply(r)
	}

	return
}

// Do sends an HTTP request and returns an HTTP response, handling any context
// cancellations or timeouts.
func (m *Management) Do(req *http.Request) (*http.Response, error) {

	ctx := req.Context()

	res, err := m.http.Do(req)
	if err != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			return nil, err
		}
	}

	return res, nil
}

// Request combines NewRequest and Do, while also handling decoding of response
// payload.
func (m *Management) Request(method, uri string, v interface{}, options ...RequestOption) error {

	req, err := m.NewRequest(method, uri, v, options...)
	if err != nil {
		return err
	}

	res, err := m.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusBadRequest {
		return newError(res.Body)
	}

	if res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusAccepted {
		err := json.NewDecoder(res.Body).Decode(v)
		if err != nil {
			return err
		}
		return res.Body.Close()
	}

	return nil
}

// Error is an interface describing any error which could be returned by the
// Auth0 Management API.
type Error interface {
	// Status returns the status code returned by the server together with the
	// present error.
	Status() int
	error
}

type managementError struct {
	StatusCode int    `json:"statusCode"`
	Err        string `json:"error"`
	Message    string `json:"message"`
}

func newError(r io.Reader) error {
	m := &managementError{}
	err := json.NewDecoder(r).Decode(m)
	if err != nil {
		return err
	}
	return m
}

func (m *managementError) Error() string {
	return fmt.Sprintf("%d %s: %s", m.StatusCode, m.Err, m.Message)
}

func (m *managementError) Status() int {
	return m.StatusCode
}

// List is an envelope which is typically used when calling List() or Search()
// methods.
//
// It holds metadata such as the total result count, starting offset and limit.
//
// Specific implementations embed this struct, therefore its direct use is not
// useful. Rather it has been made public in order to aid documentation.
type List struct {
	Start  int `json:"start"`
	Limit  int `json:"limit"`
	Length int `json:"length"`
	Total  int `json:"total"`
}

func (l List) HasNext() bool {
	return l.Total > l.Start+l.Limit
}

// RequestOption configures a call (typically to retrieve a resource) to Auth0 with
// query parameters.
type RequestOption interface {
	apply(*http.Request)
}

func newRequestOption(fn func(r *http.Request)) *requestOption {
	return &requestOption{applyFn: fn}
}

type requestOption struct {
	applyFn func(r *http.Request)
}

func (o *requestOption) apply(r *http.Request) {
	o.applyFn(r)
}

func applyListDefaults(options []RequestOption) RequestOption {
	return newRequestOption(func(r *http.Request) {
		PerPage(50).apply(r)
		for _, option := range options {
			option.apply(r)
		}
		IncludeTotals(true).apply(r)
	})
}

// Context configures a request to use the specified context.
func Context(ctx context.Context) RequestOption {
	return newRequestOption(func(r *http.Request) {
		*r = *r.WithContext(ctx)
	})
}

// WithFields configures a request to include the desired fields.
//
// Deprecated: use IncludeFields instead.
func WithFields(fields ...string) RequestOption {
	return IncludeFields(fields...)
}

// WithoutFields configures a request to exclude the desired fields.
//
// Deprecated: use ExcludeFields instead.
func WithoutFields(fields ...string) RequestOption {
	return ExcludeFields(fields...)
}

// IncludeFields configures a request to include the desired fields.
func IncludeFields(fields ...string) RequestOption {
	return newRequestOption(func(r *http.Request) {
		q := r.URL.Query()
		q.Set("fields", strings.Join(fields, ","))
		q.Set("include_fields", "true")
		r.URL.RawQuery = q.Encode()
	})
}

// ExcludeFields configures a request to exclude the desired fields.
func ExcludeFields(fields ...string) RequestOption {
	return newRequestOption(func(r *http.Request) {
		q := r.URL.Query()
		q.Set("fields", strings.Join(fields, ","))
		q.Set("include_fields", "false")
		r.URL.RawQuery = q.Encode()
	})
}

// Page configures a request to receive a specific page, if the results where
// concatenated.
func Page(page int) RequestOption {
	return newRequestOption(func(r *http.Request) {
		q := r.URL.Query()
		q.Set("page", strconv.FormatInt(int64(page), 10))
		r.URL.RawQuery = q.Encode()
	})
}

// PerPage configures a request to limit the amount of items in the result.
func PerPage(items int) RequestOption {
	return newRequestOption(func(r *http.Request) {
		q := r.URL.Query()
		q.Set("per_page", strconv.FormatInt(int64(items), 10))
		r.URL.RawQuery = q.Encode()
	})
}

// IncludeTotals configures a request to include totals.
func IncludeTotals(include bool) RequestOption {
	return newRequestOption(func(r *http.Request) {
		q := r.URL.Query()
		q.Set("include_totals", strconv.FormatBool(include))
		r.URL.RawQuery = q.Encode()
	})
}

// Query configures a request to search on specific query parameters.
//
// For example:
//   List(Query(`email:"alice@example.com"`))
//   List(Query(`name:"jane smith"`))
//   List(Query(`logins_count:[100 TO 200}`))
//   List(Query(`logins_count:{100 TO *]`))
//
// See: https://auth0.com/docs/users/search/v3/query-syntax
func Query(s string) RequestOption {
	return newRequestOption(func(r *http.Request) {
		q := r.URL.Query()
		q.Set("search_engine", "v3")
		q.Set("q", s)
		r.URL.RawQuery = q.Encode()
	})
}

// Parameter configures a request to add arbitrary query parameters to requests
// made to Auth0.
func Parameter(key, value string) RequestOption {
	return newRequestOption(func(r *http.Request) {
		q := r.URL.Query()
		q.Set(key, value)
		r.URL.RawQuery = q.Encode()
	})
}

// Header configures a request to add HTTP headers to requests made to Auth0.
func Header(key, value string) RequestOption {
	return newRequestOption(func(r *http.Request) {
		r.Header.Set(key, value)
	})
}

// Body configures a requests body.
func Body(b []byte) RequestOption {
	return newRequestOption(func(r *http.Request) {
		r.Body = ioutil.NopCloser(bytes.NewReader(b))
	})
}

// Stringify returns a string representation of the value passed as an argument.
func Stringify(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(b)
}
