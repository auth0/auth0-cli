// Package config owns the auth0-poppy.json file: the typed model, validation,
// reading, atomic writing, merging tenant-derived values with an existing
// file, and diffing two configs. It performs no network calls and has no
// dependency on Cobra or the Management API.
package config

import (
	"encoding/json"
	"maps"
	"net/url"
	"slices"
	"strings"
)

const (
	// ProtocolVersion is the Poppy protocol version this CLI writes.
	ProtocolVersion = "0.4"

	// DefaultFileName is the default name of the config file.
	DefaultFileName = "auth0-poppy.json"
)

// SupportedVersions are the Poppy protocol versions this CLI understands, in
// order. Reading a config file accepts any of them; writing one uses
// ProtocolVersion.
var SupportedVersions = []string{"0.3", "0.4"}

// Config is the content of auth0-poppy.json: a protocol poppy.json document and
// nothing else. The CLI adds no member of its own. The fields below are the ones
// the CLI reads and writes; the file holds them at these places:
//
//	ProtocolVersion            protocol_version
//	Organization               organization
//	Issuer                     auth.issuer
//	Scopes[n].Description      auth.scopes.<n>
//	Scopes[n].Hints            auth.scope_hints.<n>
//	APIs[i]                    apis[i]
//	Web.BrowserSessionEndpoint web.browser_session_endpoint
//	Web.Unscoped, Web.Disabled web.unscoped, web.disabled
//
// Every other member of the file, such as agent, feedback, extensions or
// auth.disabled, is kept as it was read and written back unchanged.
type Config struct {
	ProtocolVersion string
	Organization    Organization
	Issuer          string
	Scopes          map[string]Scope
	APIs            []API
	Web             Web

	// Extra holds the members of the file that the fields above do not cover.
	extra extraFields
	// Problems are the violations that only the raw file shows: a hint without
	// its scope, a hint field that is missing and the old layout.
	problems []Violation
}

// extraFields holds the members of each object of the file that the CLI does
// not use, as raw JSON.
type extraFields struct {
	top, auth, web map[string]json.RawMessage
	// EmptyWeb tells that the file has a web member with nothing in it. It is
	// still a web member for the rule that a file has agent, apis or web, so it
	// is written back.
	emptyWeb bool
}

func (e extraFields) clone() extraFields {
	return extraFields{top: maps.Clone(e.top), auth: maps.Clone(e.auth), web: maps.Clone(e.web), emptyWeb: e.emptyWeb}
}

// Organization describes the Company that owns the website.
type Organization struct {
	Name    string `json:"name"`
	Domain  string `json:"domain"`
	LogoURI string `json:"logo_uri,omitempty"`

	extra map[string]json.RawMessage
}

// Scope is a scope offered to agents, with an optional set of hints.
type Scope struct {
	Description string `json:"description"`
	Hints       *Hints `json:"hints,omitempty"`
}

// ValidChanges are the values of Hints.Changes.
var ValidChanges = []string{"none", "reversible", "irreversible"}

// KnownAPITypes are the API types protocol 0.4 defines (section 7). The list is
// open: a Personal Agent skips the types it does not know, so a file may hold
// any other non-empty type.
var KnownAPITypes = []string{"openapi", "mcp"}

// Hints tells agents what a scope does.
type Hints struct {
	Changes      string `json:"changes"`
	PersonalData bool   `json:"personal_data"`

	// Extra holds the members of the hint object that the fields above do not
	// cover, as raw JSON. They are kept as they were read and written back.
	extra map[string]json.RawMessage
}

// API is an entry of apis: an API that agents get tokens for.
type API struct {
	Type        string
	URL         string
	Description string

	extra map[string]json.RawMessage
}

// Web describes the website: the browser join address and the settings that only
// the owner of the website knows.
type Web struct {
	BrowserSessionEndpoint string
	// Unscoped and Disabled are the web.unscoped and web.disabled settings of
	// poppy.json (protocol 0.4). Only the owner of the website knows them, so
	// the CLI never sets them: it keeps what an existing file has.
	Unscoped bool
	Disabled bool
}

// BrowserSessionEndpoint returns the browser join address for a web app. The
// issuer is expected to already end with a slash.
func BrowserSessionEndpoint(issuer, clientID string) string {
	return issuer + "poppy/" + clientID + "/browser-session"
}

// Origin returns the origin of an https URL, such as https://api.shop.example,
// or "" when s is not an https URL with a host.
//
// The host is lowercased and an explicit default port (:443) is dropped, so that
// the spellings of one origin compare equal.
func Origin(s string) string {
	u, err := url.Parse(s)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return ""
	}

	host := strings.ToLower(u.Hostname())
	if host == "" {
		return ""
	}

	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}

	if port := u.Port(); port != "" && port != "443" {
		host += ":" + port
	}

	return "https://" + host
}

// EntriesFor returns the indexes of the apis entries that belong to an API of the
// tenant: those whose URL is at the origin of the API identifier. The file holds
// no identifier, so the origin is the link between the two. An identifier that
// is not an https URL has no entry.
func (c *Config) EntriesFor(identifier string) []int {
	origin := Origin(identifier)
	if origin == "" {
		return nil
	}

	var out []int

	for i, api := range c.APIs {
		if Origin(api.URL) == origin {
			out = append(out, i)
		}
	}

	return out
}

// OldLayout tells whether the file is in the layout of the first drafts, with
// issuer, scopes and agents at the top level and client_id in web. That layout
// was never released, so it is not read: the config holds nothing else.
func (c *Config) OldLayout() bool {
	return slices.ContainsFunc(c.problems, func(v Violation) bool { return v.Path == oldLayoutPath })
}
