package proxy

import (
	"encoding/json"
	"maps"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

// proxyError is an error that is a constant, so that the sentinel errors below
// cannot be changed. Their messages are sentences that are shown to users as
// they are.
type proxyError string

// Error returns the message of the error.
func (e proxyError) Error() string {
	return string(e)
}

// The errors that building a policy and starting the proxy can return. Use
// errors.Is to tell them apart.
const (
	// ErrReloginRequired is what a TokenSource returns from Refresh when the
	// session cannot be renewed without the user, which is the case for a user
	// session. The plugin then gets a session_expired error that tells the user to
	// log in again.
	ErrReloginRequired = proxyError("Your Auth0 session cannot be renewed silently. Run `auth0 login` and try again.")

	// ErrNoScopeTable means the registry index has no scope table, so there is no
	// way to tell which Management API calls a scope allows. No plugin runs then.
	ErrNoScopeTable = proxyError("The registry index has no scope table, so plugins cannot call the Management API.")

	// ErrNoSecretFields means the registry index lists no secret fields. The
	// proxy would forward responses without removing secrets, so no plugin runs.
	ErrNoSecretFields = proxyError("The registry index lists no secret fields, so plugins cannot call the Management API.")

	// ErrInvalidDomain means the tenant domain is not a bare host name.
	ErrInvalidDomain = proxyError("The tenant domain must be a host name such as example.us.auth0.com, without a scheme, port or path.")
)

// errRefreshLimit is why the proxy stops asking the token source for a new
// token. The plugin never sees it, because it gets session_expired instead.
const errRefreshLimit = proxyError("The proxy refreshed the token too often in this run.")

// errorCode names why the proxy answered with an error. SDKs of the Management
// API read it from the errorCode field of the error body.
type errorCode string

// The error codes that the proxy answers with.
const (
	codeInvalidProxyToken   errorCode = "invalid_proxy_token"
	codeBadRequest          errorCode = "bad_request"
	codeMethodNotAllowed    errorCode = "method_not_allowed"
	codeURITooLong          errorCode = "uri_too_long"
	codeForbiddenPath       errorCode = "forbidden_path"
	codeNoScopeRule         errorCode = "no_scope_rule"
	codeInsufficientScope   errorCode = "insufficient_scope"
	codePayloadTooLarge     errorCode = "payload_too_large"
	codeSessionExpired      errorCode = "session_expired"
	codeUnsupportedResponse errorCode = "unsupported_response"
	codeUpstreamRedirect    errorCode = "upstream_redirect"
	codeUpstreamFailed      errorCode = "upstream_failed"
)

// rejection is a request that the proxy answers itself instead of forwarding.
type rejection struct {
	status  int
	code    errorCode
	message string

	// The scope is the first one that would have allowed the call, when a missing
	// scope is why it was denied. The summary names it.
	scope string

	// The header holds headers that go with the answer, such as Allow.
	header http.Header
}

// reject returns a rejection with the given status and message.
func reject(status int, code errorCode, message string) *rejection {
	return &rejection{status: status, code: code, message: message}
}

// write sends the rejection as the error body of the Management API.
func (r *rejection) write(w http.ResponseWriter) {
	// Strings and integers always encode, so there is no error to handle.
	body, _ := json.Marshal(struct {
		StatusCode int       `json:"statusCode"`
		Error      string    `json:"error"`
		Message    string    `json:"message"`
		ErrorCode  errorCode `json:"errorCode"`
	}{r.status, http.StatusText(r.status), r.message, r.code})

	maps.Copy(w.Header(), r.header)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(r.status)
	_, _ = w.Write(body)
}

// maxEchoBytes is the longest piece of a request that an error message repeats.
const maxEchoBytes = 80

// safeText returns s in a form that is safe to print to a terminal and to put in
// a message: printable ASCII only, with anything else replaced by a question
// mark, and cut to maxEchoBytes.
func safeText(s string) string {
	var out strings.Builder

	i := 0
	for i < len(s) && out.Len() < maxEchoBytes {
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size

		if r >= ' ' && r <= '~' {
			out.WriteRune(r)
		} else {
			out.WriteByte('?')
		}
	}

	if i < len(s) {
		out.WriteString("...")
	}

	return out.String()
}

// quoted returns s as a quoted, escaped and cut piece of text, for the errors
// that repeat a request that was not valid.
func quoted(s string) string {
	if len(s) > maxEchoBytes {
		s = strings.ToValidUTF8(s[:maxEchoBytes], "") + "..."
	}

	return strconv.QuoteToASCII(s)
}
