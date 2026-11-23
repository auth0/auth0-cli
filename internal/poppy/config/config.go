// Package config owns the poppy.json file: the typed model, validation,
// reading, atomic writing, merging tenant-derived values with an existing
// file, and diffing two configs. It performs no network calls and has no
// dependency on Cobra or the Management API.
package config

import (
	"encoding/json"
	"maps"
	"net/url"
	"regexp"
	"strings"
)

const (
	// ProtocolVersion is the Poppy protocol version this CLI writes.
	ProtocolVersion = "1.0"

	// DefaultFileName is the default name of the config file.
	DefaultFileName = "poppy.json"
)

// SupportedVersions are the Poppy protocol versions this CLI knows by name, in
// order. Understands also accepts every later 1.x version. Reading a config file
// accepts any version it understands; writing one uses ProtocolVersion.
var SupportedVersions = []string{"0.3", "0.4", "1.0"}

// versionPattern matches 1.N, where N is a non-negative integer with no leading
// zeros.
var versionPattern = regexp.MustCompile(`^1\.(0|[1-9][0-9]*)$`)

// Understands tells whether this CLI understands the protocol version. Versions
// before 1.0 need an exact major.minor match, so only 0.3 and 0.4 are understood.
// From 1.0 on, a newer minor version is backward compatible, so every 1.N is
// understood.
func Understands(version string) bool {
	return version == "0.3" || version == "0.4" || versionPattern.MatchString(version)
}

// Config is the content of poppy.json: a protocol poppy.json document and
// nothing else. The CLI adds no member of its own. The fields below are the ones
// the CLI reads and writes; the file holds them at these places:
//
//	ProtocolVersion            protocol_version
//	Organization               organization
//	Issuer                     auth.issuer
//	Auth.Disabled              auth.disabled
//	Scopes[n].Description      auth.scopes.<n>
//	Scopes[n].Hints            auth.scope_hints.<n>
//	APIs[i]                    apis[i]
//	Web.BrowserSessionEndpoint web.browser_session_endpoint
//	Web.Unscoped, Web.Disabled web.unscoped, web.disabled
//
// Every other member of the file, such as agent, feedback or extensions, is kept
// as it was read and written back unchanged.
type Config struct {
	ProtocolVersion string
	Organization    Organization
	Issuer          string
	Auth            Auth
	Scopes          map[string]Scope
	APIs            []API
	Web             Web

	// Extra holds the members of the file that the fields above do not cover.
	extra extraFields
	// Problems are the violations that only the raw file shows: a hint without
	// its scope and a hint field that is missing.
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

// Auth describes the settings of auth that only the owner of the website knows.
type Auth struct {
	// Disabled is the auth.disabled setting of poppy.json: the website does not
	// offer agents access to an API. The CLI sets it when the owner turns the API
	// part off, and otherwise keeps what an existing file has. It never clears it.
	Disabled bool
	// DisabledSet tells that the file has the auth.disabled member, so that an
	// explicit false is written back. A true Disabled is always written.
	DisabledSet bool
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
	// poppy.json (protocol 1.0). Only the owner of the website knows them. The
	// CLI sets Unscoped only from the owner's answer (a flag or a question) and
	// otherwise keeps what an existing file has. It never sets Disabled.
	Unscoped bool
	Disabled bool
	// UnscopedSet tells that the file has the web.unscoped member, or that the CLI
	// set it, so that an explicit false differs from an absent member. A true
	// Unscoped is always written, whatever UnscopedSet says.
	UnscopedSet bool
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

// EntriesFor returns the indexes of the apis entries that belong to one API of
// the tenant. It is AssignEntries for a single identifier, so see there how an
// entry is linked to an API.
func (c *Config) EntriesFor(identifier string) []int {
	return c.AssignEntries([]string{identifier})[identifier]
}

// AssignEntries shares the apis entries out to the APIs of the tenant, and returns
// the indexes of the entries of each identifier in file order. The file holds no
// identifier, so the origin is the link between the two: only an entry at the
// origin of an identifier can be its entry. An identifier that is not an https URL
// has none.
//
// APIs on one host tell themselves apart by path, such as
// https://api.shop.example/orders and https://api.shop.example/users. An entry goes
// to the API whose path it is at or under, and when several are, to the one with
// the longest path, so /v1/orders/openapi.json is for /v1/orders and not for /v1.
//
// An entry that no API path matches, such as /openapi.json, is shared by the APIs
// at its origin that have no entry by path: an API with no path, an API with no
// entry under its path, or all of them when none has. Several entries may then
// go to one API, which covers one API that is published in several forms, and
// one entry may go to several, which the caller reports as ambiguous. An API that
// has an entry by path never takes one of these.
func (c *Config) AssignEntries(identifiers []string) map[string][]int {
	assigned := make(map[string][]int, len(identifiers))

	byOrigin := map[string][]string{}

	for _, id := range identifiers {
		if origin := Origin(id); origin != "" {
			if _, seen := assigned[id]; !seen {
				assigned[id] = nil
				byOrigin[origin] = append(byOrigin[origin], id)
			}
		}
	}

	// Pass one: the entries that an API path matches.
	var unowned []int

	for i, api := range c.APIs {
		candidates := byOrigin[Origin(api.URL)]
		if len(candidates) == 0 {
			continue
		}

		longest, owners := 0, []string(nil)

		for _, id := range candidates {
			if !PathUnder(api.URL, id) {
				continue
			}

			switch n := len(strings.TrimRight(pathOf(id), "/")); {
			case n > longest:
				longest, owners = n, []string{id}
			case n == longest:
				owners = append(owners, id)
			}
		}

		if len(owners) == 0 {
			unowned = append(unowned, i)
			continue
		}

		for _, id := range owners {
			assigned[id] = append(assigned[id], i)
		}
	}

	// Pass two: the other entries go to the APIs at their origin with none so far.
	free := map[string][]string{}

	for origin, ids := range byOrigin {
		for _, id := range ids {
			if len(assigned[id]) == 0 {
				free[origin] = append(free[origin], id)
			}
		}
	}

	for _, i := range unowned {
		for _, id := range free[Origin(c.APIs[i].URL)] {
			assigned[id] = append(assigned[id], i)
		}
	}

	return assigned
}

// pathOf returns the path of a URL, empty when it does not parse.
func pathOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	return u.Path
}

// PathUnder tells whether the path of rawURL is the path of the identifier or is
// under it, compared by whole segments: /orders/mcp is under /orders and
// /orders-v2 is not. It is false when the identifier has no path, or when either
// is not a URL.
func PathUnder(rawURL, identifier string) bool {
	base, err := url.Parse(identifier)
	if err != nil {
		return false
	}

	target, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	basePath := strings.TrimRight(base.Path, "/")
	if basePath == "" {
		return false
	}

	targetPath := strings.TrimRight(target.Path, "/")

	return targetPath == basePath || strings.HasPrefix(targetPath, basePath+"/")
}
