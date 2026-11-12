package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiff(t *testing.T) {
	t.Run("nil against a full config lists everything as added", func(t *testing.T) {
		got := Diff(nil, exampleConfig())

		assert.Equal(t, []Change{
			{Path: "apis[0].description", Kind: ChangeAdded, New: "Orders, cart and checkout"},
			{Path: "apis[0].type", Kind: ChangeAdded, New: "openapi"},
			{Path: "apis[0].url", Kind: ChangeAdded, New: "https://api.shop.example/openapi.json"},
			{Path: "auth.issuer", Kind: ChangeAdded, New: "https://auth.shop.example/"},
			{Path: "auth.scope_hints.checkout.changes", Kind: ChangeAdded, New: "irreversible"},
			{Path: "auth.scope_hints.checkout.personal_data", Kind: ChangeAdded, New: "true"},
			{Path: "auth.scope_hints.orders:read.changes", Kind: ChangeAdded, New: "none"},
			{Path: "auth.scope_hints.orders:read.personal_data", Kind: ChangeAdded, New: "true"},
			{Path: "auth.scope_hints.poppy:browser-sign-in.changes", Kind: ChangeAdded, New: "none"},
			{Path: "auth.scope_hints.poppy:browser-sign-in.personal_data", Kind: ChangeAdded, New: "false"},
			{Path: "auth.scopes.checkout", Kind: ChangeAdded, New: "Place orders and pay with your saved card. Can't be undone."},
			{Path: "auth.scopes.orders:read", Kind: ChangeAdded, New: "See your orders and their status. Read only."},
			{Path: "auth.scopes.poppy:browser-sign-in", Kind: ChangeAdded, New: "Use the website signed in, with the other access you approve"},
			{Path: "organization.domain", Kind: ChangeAdded, New: "shop.example"},
			{Path: "organization.name", Kind: ChangeAdded, New: "Shop"},
			{Path: "protocol_version", Kind: ChangeAdded, New: "1.0"},
			{Path: "web.browser_session_endpoint", Kind: ChangeAdded, New: "https://auth.shop.example/poppy/W1x9aBcD2eFgH3iJ/browser-session"},
		}, got)
	})

	t.Run("a full config against nil lists everything as removed", func(t *testing.T) {
		got := Diff(exampleConfig(), nil)

		assert.Len(t, got, 17)
		for _, c := range got {
			assert.Equal(t, ChangeRemoved, c.Kind, c.Path)
			assert.NotEmpty(t, c.Old, c.Path)
			assert.Empty(t, c.New, c.Path)
		}
	})

	t.Run("web.unscoped and web.disabled show only when set", func(t *testing.T) {
		a, b := exampleConfig(), exampleConfig()
		b.Web.Unscoped, b.Web.Disabled = true, true

		assert.Equal(t, []Change{
			{Path: "web.disabled", Kind: ChangeAdded, New: "true"},
			{Path: "web.unscoped", Kind: ChangeAdded, New: "true"},
		}, Diff(a, b))
		assert.Equal(t, []Change{
			{Path: "web.disabled", Kind: ChangeRemoved, Old: "true"},
			{Path: "web.unscoped", Kind: ChangeRemoved, Old: "true"},
		}, Diff(b, a))
	})

	t.Run("equal configs give an empty slice", func(t *testing.T) {
		got := Diff(exampleConfig(), exampleConfig())
		assert.NotNil(t, got)
		assert.Empty(t, got)

		assert.Empty(t, Diff(nil, nil))
	})

	t.Run("one of each kind", func(t *testing.T) {
		a, b := exampleConfig(), exampleConfig()
		b.Organization.Name = "Shop Inc"                   // Changed.
		b.Scopes["refunds"] = Scope{Description: "Refund"} // Added.
		delete(b.Scopes, "orders:read")                    // Removed.
		b.APIs[0].Description = "Changed"

		assert.Equal(t, []Change{
			{Path: "apis[0].description", Kind: ChangeChanged, Old: "Orders, cart and checkout", New: "Changed"},
			{Path: "auth.scope_hints.orders:read.changes", Kind: ChangeRemoved, Old: "none"},
			{Path: "auth.scope_hints.orders:read.personal_data", Kind: ChangeRemoved, Old: "true"},
			{Path: "auth.scopes.orders:read", Kind: ChangeRemoved, Old: "See your orders and their status. Read only."},
			{Path: "auth.scopes.refunds", Kind: ChangeAdded, New: "Refund"},
			{Path: "organization.name", Kind: ChangeChanged, Old: "Shop", New: "Shop Inc"},
		}, Diff(a, b))
	})

	t.Run("hints that change, appear and disappear", func(t *testing.T) {
		a, b := exampleConfig(), exampleConfig()
		b.Scopes["checkout"] = Scope{
			Description: a.Scopes["checkout"].Description,
			Hints:       &Hints{Changes: "reversible", PersonalData: false},
		}
		b.Scopes["orders:read"] = Scope{Description: a.Scopes["orders:read"].Description}

		assert.Equal(t, []Change{
			{Path: "auth.scope_hints.checkout.changes", Kind: ChangeChanged, Old: "irreversible", New: "reversible"},
			{Path: "auth.scope_hints.checkout.personal_data", Kind: ChangeChanged, Old: "true", New: "false"},
			{Path: "auth.scope_hints.orders:read.changes", Kind: ChangeRemoved, Old: "none"},
			{Path: "auth.scope_hints.orders:read.personal_data", Kind: ChangeRemoved, Old: "true"},
		}, Diff(a, b))
	})

	t.Run("changes are sorted by path", func(t *testing.T) {
		a := &Config{APIs: []API{{Type: "openapi", URL: "https://b.example/old"}}}
		b := &Config{APIs: []API{
			{Type: "openapi", URL: "https://b.example/new"},
			{Type: "mcp", URL: "https://a.example/mcp"},
		}}

		assert.Equal(t, []Change{
			{Path: "apis[0].url", Kind: ChangeChanged, Old: "https://b.example/old", New: "https://b.example/new"},
			{Path: "apis[1].type", Kind: ChangeAdded, New: "mcp"},
			{Path: "apis[1].url", Kind: ChangeAdded, New: "https://a.example/mcp"},
		}, Diff(a, b))
	})

	t.Run("members the cli does not use are not compared", func(t *testing.T) {
		a, b := exampleConfig(), exampleConfig()
		b.extra.top = map[string]json.RawMessage{"feedback": json.RawMessage(`{}`)}

		assert.Empty(t, Diff(a, b))
	})
}
