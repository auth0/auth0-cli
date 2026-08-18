package cli

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
)

// invokerMetadataHeader carries structured information about who invoked the CLI so
// that human, AI agent and CI traffic can be told apart. It is a dedicated header
// rather than extra User-Agent tokens so new fields can be added without reparsing.
//
// Spelled in Go's canonical form: net/http canonicalizes header names, so "CLI" would
// go out as "Cli" regardless. Header names are case-insensitive per RFC 9110 section 5.1,
// and HTTP/2 lower-cases them, so the server must match this case-insensitively.
const invokerMetadataHeader = "Auth0-Cli-Metadata"

const (
	invokerKindAgent = "agent"
	invokerKindCI    = "ci"
	invokerKindHuman = "human"
	// InvokerUnknown is the invoker_agent value when no agent can be named. It is also
	// what detectAgent returns for a non-interactive invocation carrying no agent signal.
	invokerUnknown = "unknown"
)

// invokerMetadata is the JSON payload of the Auth0-Cli-Metadata header.
//
// InvokerKind is the single best label for the invoker, always one of "agent", "ci" or
// "human", and CI is the raw environment signal. They are deliberately not merged: an
// agent running inside CI reports kind "agent" so the agent is not lost, and CI true so
// the environment is not either. Never use omitempty here, since an explicit false
// ("not CI") carries different meaning from an absent field ("this CLI version did not
// report it").
type invokerMetadata struct {
	InvokerKind  string `json:"invoker_kind"`
	InvokerAgent string `json:"invoker_agent"`
	CI           bool   `json:"ci"`
}

// resolveInvokerMetadata maps a detected agent client (see detectAgent) onto the header
// payload. A named agent takes precedence over CI for InvokerKind, since an agent
// running inside CI is still an agent; the CI field preserves that combination.
func resolveInvokerMetadata(agentClient string, isCI bool) invokerMetadata {
	kind, agent := invokerKindAgent, agentClient

	switch agentClient {
	// No agent signal at all, so this is a person. Both fallbacks land here: detectAgent
	// says "human" on a TTY and "unknown" without one, but a human piping output or
	// driving the CLI from a script has no TTY and is still human. CI overrides, because
	// a CI run is a more specific signal than the absence of a terminal.
	case invokerKindHuman, invokerUnknown:
		kind, agent = invokerKindHuman, invokerUnknown
		if isCI {
			kind = invokerKindCI
		}
	// An agent was detected but could not be named.
	case agentClientUnknownAgent:
		agent = invokerUnknown
	}

	return invokerMetadata{InvokerKind: kind, InvokerAgent: agent, CI: isCI}
}

// headerValue renders the metadata as base64-encoded JSON. The encoding mirrors the
// sibling "Auth0-Client" header (base64.StdEncoding over json.Marshal) so the server
// side can reuse the same decode path, and it keeps the JSON commas out of a header
// value, which intermediaries are otherwise allowed to split on per RFC 9110.
// Returns an empty string if the payload cannot be encoded, in which case the header
// is omitted rather than sent blank.
func (m invokerMetadata) headerValue() string {
	value, err := json.Marshal(m)
	if err != nil {
		return ""
	}

	return base64.StdEncoding.EncodeToString(value)
}

// invokerMetadataHeaderValue returns the Auth0-CLI-Metadata value for this invocation,
// reusing the cached agent detection that mode resolution and analytics also rely on.
func (c *cli) invokerMetadataHeaderValue() string {
	return resolveInvokerMetadata(c.agentClientName(), isCIEnvironment(os.Getenv)).headerValue()
}

// invokerMetadataTransport sets the Auth0-CLI-Metadata header on every outbound request.
type invokerMetadataTransport struct {
	base     http.RoundTripper
	metadata string
}

func (t invokerMetadataTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if t.metadata == "" {
		return t.base.RoundTrip(request)
	}

	// Clone before mutating: a RoundTripper must not modify the request it is given.
	request = request.Clone(request.Context())
	request.Header.Set(invokerMetadataHeader, t.metadata)

	return t.base.RoundTrip(request)
}
