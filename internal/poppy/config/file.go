package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
)

// The members of each object of the file, in the order the protocol lists them.
// The encoder writes the members it knows in this order, then any other member
// in alphabetical order.
var (
	topOrder  = []string{"protocol_version", "supported_versions", "organization", "auth", "agent", "apis", "web", "feedback", "excluded_tasks", "extensions", "skill_index"}
	orgOrder  = []string{"name", "domain", "logo_uri"}
	authOrder = []string{"issuer", "disabled", "scopes", "scope_hints"}
	apiOrder  = []string{"type", "url", "description"}
	webOrder  = []string{"browser_session_endpoint", "disabled", "unscoped"}
	hintOrder = []string{"changes", "personal_data"}
)

// rootPath is the path of a violation about the document as a whole.
const rootPath = "file"

// oldLayoutPath is the path of the violation for a file in the old layout.
const oldLayoutPath = rootPath

// fileDoc is the typed part of the file. It checks the types of the members the
// CLI uses; everything else is read again as raw JSON.
type fileDoc struct {
	ProtocolVersion string `json:"protocol_version"`
	Organization    struct {
		Name    string `json:"name"`
		Domain  string `json:"domain"`
		LogoURI string `json:"logo_uri"`
	} `json:"organization"`
	Auth *struct {
		Issuer     string              `json:"issuer"`
		Scopes     map[string]string   `json:"scopes"`
		ScopeHints map[string]fileHint `json:"scope_hints"`
	} `json:"auth"`
	APIs []struct {
		Type        string `json:"type"`
		URL         string `json:"url"`
		Description string `json:"description"`
	} `json:"apis"`
	Web *struct {
		BrowserSessionEndpoint string `json:"browser_session_endpoint"`
		Disabled               bool   `json:"disabled"`
		Unscoped               bool   `json:"unscoped"`
	} `json:"web"`
}

// fileHint is a hint as the file has it. PersonalData is a pointer because the
// protocol requires it, and only a pointer shows that it is missing.
type fileHint struct {
	Changes      string `json:"changes"`
	PersonalData *bool  `json:"personal_data"`
}

// decode parses the content of a config file. Syntax and type errors keep their
// offsets, so that the caller can name the position. A file in the old layout
// decodes to an empty config that carries the violation.
func decode(data []byte) (*Config, error) {
	dec := json.NewDecoder(bytes.NewReader(data))

	var doc fileDoc
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}

	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errTrailingContent
	}

	top, err := rawObject(data)
	if err != nil {
		return nil, err
	}

	if oldLayout(top) {
		return &Config{problems: []Violation{{
			Path: oldLayoutPath,
			Message: "uses the old layout (issuer, scopes or agents at the top level, or client_id in web), which was never released\n" +
				"  FIX  Run auth0 poppy config (or auth0 poppy setup --force-config) to write the new layout.",
		}}}, nil
	}

	cfg := &Config{ProtocolVersion: doc.ProtocolVersion}

	cfg.Organization = Organization{
		Name:    doc.Organization.Name,
		Domain:  doc.Organization.Domain,
		LogoURI: doc.Organization.LogoURI,
		extra:   without(rawObjectOf(top["organization"]), orgOrder...),
	}

	cfg.extra.top = without(top, "protocol_version", "organization", "auth", "apis", "web")

	if doc.Auth != nil {
		rawAuth := rawObjectOf(top["auth"])

		cfg.readAuth(doc.Auth.Issuer, doc.Auth.Scopes, doc.Auth.ScopeHints, rawObjectOf(rawAuth["scope_hints"]))
		cfg.extra.auth = without(rawAuth, "issuer", "scopes", "scope_hints")
	}

	if doc.Web != nil {
		cfg.Web = Web{BrowserSessionEndpoint: doc.Web.BrowserSessionEndpoint, Disabled: doc.Web.Disabled, Unscoped: doc.Web.Unscoped}
		cfg.extra.web = without(rawObjectOf(top["web"]), webOrder...)
		cfg.extra.emptyWeb = cfg.Web == Web{} && cfg.extra.web == nil
	}

	cfg.readAPIs(doc, top)

	return cfg, nil
}

// errTrailingContent is returned by decode for text after the JSON document.
var errTrailingContent = errors.New("unexpected content after the JSON document")

func (c *Config) readAuth(issuer string, scopes map[string]string, hints map[string]fileHint, rawHints map[string]json.RawMessage) {
	c.Issuer = issuer

	if scopes != nil {
		c.Scopes = make(map[string]Scope, len(scopes))
	}

	for name, description := range scopes {
		c.Scopes[name] = Scope{Description: description}
	}

	for _, name := range slices.Sorted(maps.Keys(hints)) {
		hint := hints[name]

		scope, found := c.Scopes[name]
		if !found {
			c.problems = append(c.problems, Violation{Path: "auth.scope_hints." + name, Message: "has no matching entry in auth.scopes"})
			continue
		}

		if hint.PersonalData == nil {
			c.problems = append(c.problems, Violation{Path: "auth.scope_hints." + name + ".personal_data", Message: "is required"})
		}

		scope.Hints = &Hints{
			Changes:      hint.Changes,
			PersonalData: hint.PersonalData != nil && *hint.PersonalData,
			extra:        without(rawObjectOf(rawHints[name]), hintOrder...),
		}
		c.Scopes[name] = scope
	}
}

// readAPIs fills APIs from apis.
func (c *Config) readAPIs(doc fileDoc, top map[string]json.RawMessage) {
	extras := rawObjects(top["apis"])

	for i, api := range doc.APIs {
		entry := API{Type: api.Type, URL: api.URL, Description: api.Description}
		if i < len(extras) {
			entry.extra = without(extras[i], apiOrder...)
		}

		c.APIs = append(c.APIs, entry)
	}
}

// oldLayout tells whether the top-level members are those of the old layout.
func oldLayout(top map[string]json.RawMessage) bool {
	for _, member := range []string{"issuer", "scopes", "agents"} {
		if _, found := top[member]; found {
			return true
		}
	}

	_, found := rawObjectOf(top["web"])["client_id"]

	return found
}

// rawObject decodes a JSON object into its raw members.
func rawObject(data []byte) (map[string]json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	return m, nil
}

// rawObjectOf is rawObject for a member that already passed the typed decode.
// It returns nil for a missing member.
func rawObjectOf(data json.RawMessage) map[string]json.RawMessage {
	m, _ := rawObject(data)

	return m
}

// rawObjects decodes a JSON array of objects.
func rawObjects(data json.RawMessage) []map[string]json.RawMessage {
	var list []map[string]json.RawMessage
	_ = json.Unmarshal(data, &list)

	return list
}

// without returns the members of m except the named ones, or nil when none is
// left.
func without(m map[string]json.RawMessage, names ...string) map[string]json.RawMessage {
	out := map[string]json.RawMessage{}

	for name, value := range m {
		if !slices.Contains(names, name) {
			out[name] = value
		}
	}

	if len(out) == 0 {
		return nil
	}

	return out
}

// encode returns the compact JSON of the file: members in the order of the
// protocol, then any other member alphabetically. Lists and maps are never null.
func (c *Config) encode() ([]byte, error) {
	top := maps.Clone(c.extra.top)
	if top == nil {
		top = map[string]json.RawMessage{}
	}

	var err error
	put := func(m map[string]json.RawMessage, name string, value any) {
		if err != nil {
			return
		}

		var raw json.RawMessage
		if raw, err = rawJSON(value); err == nil {
			m[name] = raw
		}
	}

	put(top, "protocol_version", c.ProtocolVersion)
	put(top, "organization", c.organization(put))
	put(top, "auth", c.auth(put))
	put(top, "apis", c.apisOut(put))
	if web := c.webOut(put); len(web.members) > 0 || c.extra.emptyWeb {
		put(top, "web", web)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to encode config: %w", err)
	}

	return object(top, topOrder), nil
}

type putFunc func(m map[string]json.RawMessage, name string, value any)

func (c *Config) organization(put putFunc) rawMembers {
	m := cloneRaw(c.Organization.extra)
	put(m, "name", c.Organization.Name)
	put(m, "domain", c.Organization.Domain)

	if c.Organization.LogoURI != "" {
		put(m, "logo_uri", c.Organization.LogoURI)
	}

	return rawMembers{m, orgOrder}
}

func (c *Config) auth(put putFunc) rawMembers {
	m := cloneRaw(c.extra.auth)

	if c.Issuer != "" {
		put(m, "issuer", c.Issuer)
	}

	scopes := make(map[string]string, len(c.Scopes))
	hints := map[string]Hints{}

	for name, scope := range c.Scopes {
		scopes[name] = scope.Description
		if scope.Hints != nil {
			hints[name] = *scope.Hints
		}
	}

	put(m, "scopes", scopes)

	if len(hints) > 0 {
		put(m, "scope_hints", hints)
	}

	return rawMembers{m, authOrder}
}

func (c *Config) apisOut(put putFunc) []rawMembers {
	out := make([]rawMembers, 0, len(c.APIs))

	for _, api := range c.APIs {
		m := cloneRaw(api.extra)
		put(m, "type", api.Type)
		put(m, "url", api.URL)
		put(m, "description", api.Description)
		out = append(out, rawMembers{m, apiOrder})
	}

	return out
}

func (c *Config) webOut(put putFunc) rawMembers {
	m := cloneRaw(c.extra.web)

	if c.Web.BrowserSessionEndpoint != "" {
		put(m, "browser_session_endpoint", c.Web.BrowserSessionEndpoint)
	}

	if c.Web.Disabled {
		put(m, "disabled", true)
	}

	if c.Web.Unscoped {
		put(m, "unscoped", true)
	}

	return rawMembers{m, webOrder}
}

// rawMembers is an object whose members are written in a given order.
type rawMembers struct {
	members map[string]json.RawMessage
	order   []string
}

// MarshalJSON writes the members in order.
func (r rawMembers) MarshalJSON() ([]byte, error) {
	return object(r.members, r.order), nil
}

// MarshalJSON writes the hints as the file holds them: changes and personal_data,
// then any other member the file had, alphabetically.
func (h Hints) MarshalJSON() ([]byte, error) {
	m := cloneRaw(h.extra)

	var err error
	for name, value := range map[string]any{"changes": h.Changes, "personal_data": h.PersonalData} {
		var raw json.RawMessage
		if raw, err = rawJSON(value); err != nil {
			return nil, err
		}

		m[name] = raw
	}

	return object(m, hintOrder), nil
}

// MarshalJSON writes the config as the file holds it, so that a command that
// prints a config prints the same document.
func (c *Config) MarshalJSON() ([]byte, error) {
	return c.encode()
}

// UnmarshalJSON reads the config from the content of the file.
func (c *Config) UnmarshalJSON(data []byte) error {
	cfg, err := decode(data)
	if err != nil {
		return err
	}

	*c = *cfg

	return nil
}

func cloneRaw(m map[string]json.RawMessage) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(m)+4)
	maps.Copy(out, m)

	return out
}

// rawJSON encodes a value without escaping HTML, so that "&" and "<" stay as
// they are in the file.
func rawJSON(v any) (json.RawMessage, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)

	if err := enc.Encode(v); err != nil {
		return nil, err
	}

	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// object writes the members as a JSON object: the ones in order first, then the
// others alphabetically.
func object(members map[string]json.RawMessage, order []string) []byte {
	names := make([]string, 0, len(members))

	for _, name := range order {
		if _, found := members[name]; found {
			names = append(names, name)
		}
	}

	var others []string
	for name := range members {
		if !slices.Contains(order, name) {
			others = append(others, name)
		}
	}

	slices.Sort(others)
	names = append(names, others...)

	var buf bytes.Buffer
	buf.WriteByte('{')

	for i, name := range names {
		if i > 0 {
			buf.WriteByte(',')
		}

		key, _ := rawJSON(name)
		buf.Write(key)
		buf.WriteByte(':')
		buf.Write(members[name])
	}

	buf.WriteByte('}')

	return buf.Bytes()
}
