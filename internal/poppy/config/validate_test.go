package config

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func paths(violations []Violation) []string {
	out := make([]string, 0, len(violations))
	for _, v := range violations {
		out = append(out, v.Path)
	}
	return out
}

func TestValidate(t *testing.T) {
	const domainMsg = "must be a lowercase host name such as shop.example"

	tests := []struct {
		name      string
		modify    func(c *Config)
		wantPaths []string
		wantMsg   string
	}{
		{name: "valid example", modify: func(c *Config) {}},
		{
			name:      "missing protocol version",
			modify:    func(c *Config) { c.ProtocolVersion = "" },
			wantPaths: []string{"protocol_version"},
			wantMsg:   "is required",
		},
		{name: "protocol version 0.3 is still understood", modify: func(c *Config) { c.ProtocolVersion = "0.3" }},
		{name: "web.unscoped and web.disabled are kept as they are", modify: func(c *Config) { c.Web.Unscoped, c.Web.Disabled = true, true }},
		{
			name:      "wrong protocol version",
			modify:    func(c *Config) { c.ProtocolVersion = "0.2" },
			wantPaths: []string{"protocol_version"},
			wantMsg:   `must be one of "0.3", "0.4", got "0.2"`,
		},
		{
			name:      "unparsable protocol version",
			modify:    func(c *Config) { c.ProtocolVersion = "next" },
			wantPaths: []string{"protocol_version"},
			wantMsg:   `must be one of "0.3", "0.4", got "next"`,
		},
		{name: "logo with a query", modify: func(c *Config) { c.Organization.LogoURI = "https://shop.example/logo.png?v=2" }},
		{
			name:      "logo over http",
			modify:    func(c *Config) { c.Organization.LogoURI = "http://shop.example/logo.png" },
			wantPaths: []string{"organization.logo_uri"},
			wantMsg:   "must be an https URL",
		},
		{
			name:      "missing domain",
			modify:    func(c *Config) { c.Organization.Domain = "" },
			wantPaths: []string{"organization.domain"},
			wantMsg:   "is required",
		},
		{
			name:      "domain with a scheme",
			modify:    func(c *Config) { c.Organization.Domain = "https://shop.example" },
			wantPaths: []string{"organization.domain"},
			wantMsg:   domainMsg,
		},
		{
			name:      "domain with a path",
			modify:    func(c *Config) { c.Organization.Domain = "shop.example/store" },
			wantPaths: []string{"organization.domain"},
			wantMsg:   domainMsg,
		},
		{
			name:      "domain with a port",
			modify:    func(c *Config) { c.Organization.Domain = "shop.example:8443" },
			wantPaths: []string{"organization.domain"},
			wantMsg:   domainMsg,
		},
		{
			name:      "domain with spaces",
			modify:    func(c *Config) { c.Organization.Domain = "shop .example" },
			wantPaths: []string{"organization.domain"},
			wantMsg:   domainMsg,
		},
		{
			name:      "domain with uppercase",
			modify:    func(c *Config) { c.Organization.Domain = "Shop.Example" },
			wantPaths: []string{"organization.domain"},
			wantMsg:   domainMsg,
		},
		{
			name:      "domain with a trailing dot",
			modify:    func(c *Config) { c.Organization.Domain = "shop.example." },
			wantPaths: []string{"organization.domain"},
			wantMsg:   domainMsg,
		},
		{
			name:      "domain that is too long",
			modify:    func(c *Config) { c.Organization.Domain = strings.Repeat("abcdefghi.", 26) + "example" },
			wantPaths: []string{"organization.domain"},
			wantMsg:   domainMsg,
		},
		{name: "no issuer is fine, the protocol makes it optional", modify: func(c *Config) { c.Issuer = "" }},
		{
			name:      "issuer without a trailing slash",
			modify:    func(c *Config) { c.Issuer = "https://auth.shop.example" },
			wantPaths: []string{"auth.issuer"},
			wantMsg:   "trailing slash",
		},
		{
			name:      "issuer over http",
			modify:    func(c *Config) { c.Issuer = "http://auth.shop.example/" },
			wantPaths: []string{"auth.issuer"},
			wantMsg:   "https",
		},
		{
			name:      "issuer with a path",
			modify:    func(c *Config) { c.Issuer = "https://auth.shop.example/tenant/" },
			wantPaths: []string{"auth.issuer"},
			wantMsg:   "path",
		},
		{
			name:      "issuer without a host",
			modify:    func(c *Config) { c.Issuer = "https:///" },
			wantPaths: []string{"auth.issuer"},
			wantMsg:   "host",
		},
		{
			name:      "issuer with a query",
			modify:    func(c *Config) { c.Issuer = "https://auth.shop.example/?a=b" },
			wantPaths: []string{"auth.issuer"},
			wantMsg:   "query",
		},
		{
			name:      "issuer that is not a URL",
			modify:    func(c *Config) { c.Issuer = "https://auth shop/" },
			wantPaths: []string{"auth.issuer"},
			wantMsg:   "valid URL",
		},
		{
			name:      "missing organization name",
			modify:    func(c *Config) { c.Organization.Name = " " },
			wantPaths: []string{"organization.name"},
			wantMsg:   "is required",
		},
		{name: "no browser session endpoint is fine", modify: func(c *Config) { c.Web.BrowserSessionEndpoint = "" }},
		{
			name:      "browser session endpoint over http",
			modify:    func(c *Config) { c.Web.BrowserSessionEndpoint = "http://shop.example/poppy/browser-session" },
			wantPaths: []string{"web.browser_session_endpoint"},
			wantMsg:   "https",
		},
		{
			name:      "browser session endpoint that is not a URL",
			modify:    func(c *Config) { c.Web.BrowserSessionEndpoint = "browser-session" },
			wantPaths: []string{"web.browser_session_endpoint"},
			wantMsg:   "https",
		},
		{name: "browser session endpoint on the company domain", modify: func(c *Config) { c.Web.BrowserSessionEndpoint = "https://shop.example/poppy/browser-session" }},
		{name: "api with a type the protocol does not define", modify: func(c *Config) { c.APIs[0].Type = "grpc" }},
		{
			name:      "api without a type",
			modify:    func(c *Config) { c.APIs[0].Type = "" },
			wantPaths: []string{"apis[0].type"},
			wantMsg:   "is required",
		},
		{
			name:      "api with a blank type",
			modify:    func(c *Config) { c.APIs[0].Type = "  " },
			wantPaths: []string{"apis[0].type"},
			wantMsg:   "is required",
		},
		{name: "mcp api", modify: func(c *Config) { c.APIs[0].Type = "mcp" }},
		{
			name:      "api without a url",
			modify:    func(c *Config) { c.APIs[0].URL = "" },
			wantPaths: []string{"apis[0].url"},
			wantMsg:   "is required",
		},
		{
			name:      "api url over http",
			modify:    func(c *Config) { c.APIs[0].URL = "http://api.shop.example/openapi.json" },
			wantPaths: []string{"apis[0].url"},
			wantMsg:   "https",
		},
		{
			name:      "api url with credentials",
			modify:    func(c *Config) { c.APIs[0].URL = "https://u@api.shop.example/openapi.json" },
			wantPaths: []string{"apis[0].url"},
			wantMsg:   "credentials",
		},
		{name: "api url with a query", modify: func(c *Config) { c.APIs[0].URL = "https://api.shop.example/openapi.json?v=2" }},
		{
			name:      "api without a description",
			modify:    func(c *Config) { c.APIs[0].Description = "" },
			wantPaths: []string{"apis[0].description"},
			wantMsg:   "is required",
		},
		{
			name:      "api with a blank description",
			modify:    func(c *Config) { c.APIs[0].Description = " " },
			wantPaths: []string{"apis[0].description"},
			wantMsg:   "is required",
		},
		{
			name: "a second api",
			modify: func(c *Config) {
				c.APIs = append(c.APIs, API{Type: "mcp", URL: "https://mcp.shop.example/mcp", Description: "Search"})
			},
		},
		{
			name: "the second api is checked at its own index",
			modify: func(c *Config) {
				c.APIs = append(c.APIs, API{Type: "", URL: "http://mcp.shop.example/mcp"})
			},
			wantPaths: []string{"apis[1].type", "apis[1].url", "apis[1].description"},
		},
		{
			name: "problems of the raw file are reported",
			modify: func(c *Config) {
				c.problems = []Violation{{Path: "auth.scope_hints.nope", Message: "has no matching entry in auth.scopes"}}
			},
			wantPaths: []string{"auth.scope_hints.nope"},
		},
		{
			name: "only apis is enough",
			modify: func(c *Config) {
				c.Web = Web{}
			},
		},
		{
			name: "only web is enough",
			modify: func(c *Config) {
				c.APIs = nil
			},
		},
		{
			name: "only agent is enough",
			modify: func(c *Config) {
				c.APIs, c.Web = nil, Web{}
				c.extra.top = map[string]json.RawMessage{"agent": json.RawMessage(`{"protocols":[]}`)}
			},
		},
		{
			name: "an empty web member counts",
			modify: func(c *Config) {
				c.APIs, c.Web = nil, Web{}
				c.extra.emptyWeb = true
			},
		},
		{
			name: "none of agent, apis and web",
			modify: func(c *Config) {
				c.APIs, c.Web = nil, Web{}
			},
			wantPaths: []string{rootPath},
			wantMsg:   "at least one of agent, apis or web\n  FIX  ",
		},
		{
			name: "web.unscoped alone counts as a web member",
			modify: func(c *Config) {
				c.APIs, c.Web = nil, Web{Unscoped: true}
			},
		},
		{
			name:      "scope without a description",
			modify:    func(c *Config) { c.Scopes["checkout"] = Scope{Description: "  "} },
			wantPaths: []string{"auth.scopes.checkout"},
			wantMsg:   "must not be empty",
		},
		{
			name: "hint with an unknown changes value",
			modify: func(c *Config) {
				c.Scopes["checkout"] = Scope{Description: "Place orders.", Hints: &Hints{Changes: "dangerous"}}
			},
			wantPaths: []string{"auth.scope_hints.checkout.changes"},
			wantMsg:   `must be one of none, reversible, irreversible, got "dangerous"`,
		},
		{
			name:      "hint without a changes value",
			modify:    func(c *Config) { c.Scopes["checkout"] = Scope{Description: "Place orders.", Hints: &Hints{}} },
			wantPaths: []string{"auth.scope_hints.checkout.changes"},
			wantMsg:   "must be one of",
		},
		{
			name: "hints with each valid changes value",
			modify: func(c *Config) {
				c.Scopes["checkout"] = Scope{Description: "Place orders.", Hints: &Hints{Changes: "reversible"}}
			},
		},
		{
			name: "violations come in a stable field order",
			modify: func(c *Config) {
				c.Organization.Domain = ""
				c.APIs[0].Type = ""
				c.Scopes = map[string]Scope{"b": {}, "a": {}}
				c.Web.BrowserSessionEndpoint = "nope"
			},
			wantPaths: []string{"organization.domain", "auth.scopes.a", "auth.scopes.b", "apis[0].type", "web.browser_session_endpoint"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := exampleConfig()
			tt.modify(cfg)

			got := cfg.Validate()
			if len(tt.wantPaths) == 0 {
				assert.Empty(t, got)
				return
			}

			assert.Equal(t, tt.wantPaths, paths(got))
			if tt.wantMsg != "" {
				assert.Contains(t, got[0].Message, tt.wantMsg)
			}
		})
	}

	t.Run("an empty config returns violations without panicking", func(t *testing.T) {
		got := (&Config{}).Validate()
		assert.Equal(t, []string{"protocol_version", "organization.name", "organization.domain", rootPath}, paths(got))
	})

	t.Run("a file in the old layout has the one violation", func(t *testing.T) {
		cfg := &Config{problems: []Violation{{Path: oldLayoutPath, Message: "uses the old layout"}}}
		assert.Equal(t, []string{oldLayoutPath}, paths(cfg.Validate()))
	})
}

func TestValidationError(t *testing.T) {
	t.Run("lists every violation", func(t *testing.T) {
		err := &ValidationError{File: "x.json", Violations: []Violation{{Path: "a", Message: "is bad"}, {Path: "b", Message: "is worse"}}}
		assert.Equal(t, "x.json is invalid: a is bad; b is worse", err.Error())
	})

	t.Run("names the config when no file is known", func(t *testing.T) {
		err := &ValidationError{Violations: []Violation{{Path: "a", Message: "is bad"}}}
		assert.Equal(t, "config is invalid: a is bad", err.Error())
	})
}
