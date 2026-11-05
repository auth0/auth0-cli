package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	goldenFile     = "testdata/auth0-poppy.golden.json"
	goldenFullFile = "testdata/auth0-poppy-full.golden.json"
)

// exampleConfig is the example from the config file format description.
func exampleConfig() *Config {
	return &Config{
		ProtocolVersion: "0.4",
		Organization:    Organization{Name: "Shop", Domain: "shop.example"},
		Issuer:          "https://auth.shop.example/",
		Scopes: map[string]Scope{
			"orders:read": {
				Description: "See your orders and their status. Read only.",
				Hints:       &Hints{Changes: "none", PersonalData: true},
			},
			"checkout": {
				Description: "Place orders and pay with your saved card. Can't be undone.",
				Hints:       &Hints{Changes: "irreversible", PersonalData: true},
			},
			"poppy:browser-sign-in": {
				Description: "Use the website signed in, with the other access you approve",
				Hints:       &Hints{Changes: "none", PersonalData: false},
			},
		},
		APIs: []API{{
			Type:        "openapi",
			URL:         "https://api.shop.example/openapi.json",
			Description: "Orders, cart and checkout",
		}},
		Web: Web{BrowserSessionEndpoint: "https://auth.shop.example/poppy/W1x9aBcD2eFgH3iJ/browser-session"},
	}
}

func TestBrowserSessionEndpoint(t *testing.T) {
	assert.Equal(t, "https://auth.shop.example/poppy/abc/browser-session", BrowserSessionEndpoint("https://auth.shop.example/", "abc"))
}

func TestGolden(t *testing.T) {
	want, err := os.ReadFile(goldenFile)
	require.NoError(t, err)

	t.Run("marshal matches the golden file byte for byte", func(t *testing.T) {
		got, err := marshal(exampleConfig())
		require.NoError(t, err)
		assert.Equal(t, string(want), string(got))
	})

	t.Run("read round-trips the golden file", func(t *testing.T) {
		cfg, err := Read(goldenFile)
		require.NoError(t, err)
		assert.Equal(t, exampleConfig(), cfg)

		got, err := marshal(cfg)
		require.NoError(t, err)
		assert.Equal(t, string(want), string(got))
	})

	t.Run("write then read gives an equal config", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), DefaultFileName)
		require.NoError(t, Write(path, exampleConfig()))

		written, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, string(want), string(written))

		cfg, err := Read(path)
		require.NoError(t, err)
		assert.Equal(t, exampleConfig(), cfg)
	})
}

func TestGoldenFull(t *testing.T) {
	want, err := os.ReadFile(goldenFullFile)
	require.NoError(t, err)

	cfg, err := Read(goldenFullFile)
	require.NoError(t, err)

	t.Run("the typed fields are read", func(t *testing.T) {
		assert.Equal(t, "https://shop.example/logo.png", cfg.Organization.LogoURI)
		assert.True(t, cfg.Web.Unscoped)
		assert.False(t, cfg.Web.Disabled)
		require.Len(t, cfg.APIs, 2)
		assert.Equal(t, "mcp", cfg.APIs[1].Type)
	})

	t.Run("marshal gives the file back byte for byte", func(t *testing.T) {
		got, err := marshal(cfg)
		require.NoError(t, err)
		assert.Equal(t, string(want), string(got))
	})

	t.Run("write then read keeps every member", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), DefaultFileName)
		require.NoError(t, Write(path, cfg))

		written, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, string(want), string(written))
	})

	t.Run("json.Marshal and json.Unmarshal use the file shape", func(t *testing.T) {
		data, err := json.Marshal(cfg)
		require.NoError(t, err)
		assert.JSONEq(t, string(want), string(data))

		var back Config
		require.NoError(t, json.Unmarshal(want, &back))
		assert.Equal(t, cfg, &back)
	})
}

func TestMemberOrder(t *testing.T) {
	cfg := exampleConfig()
	cfg.extra.top = map[string]json.RawMessage{
		"zzz":      json.RawMessage(`1`),
		"agent":    json.RawMessage(`{"protocols":[]}`),
		"feedback": json.RawMessage(`{"endpoint":"https://shop.example/f"}`),
		"aaa":      json.RawMessage(`2`),
	}

	got, err := cfg.encode()
	require.NoError(t, err)

	var order []string
	dec := json.NewDecoder(bytes.NewReader(got))
	_, err = dec.Token()
	require.NoError(t, err)

	for dec.More() {
		key, err := dec.Token()
		require.NoError(t, err)
		order = append(order, key.(string))

		var skip json.RawMessage
		require.NoError(t, dec.Decode(&skip))
	}

	assert.Equal(t, []string{"protocol_version", "organization", "auth", "agent", "apis", "web", "feedback", "aaa", "zzz"}, order)
}

func TestMarshal(t *testing.T) {
	t.Run("empty collections encode as empty values, never null", func(t *testing.T) {
		got, err := marshal(&Config{})
		require.NoError(t, err)
		assert.Contains(t, string(got), `"scopes": {}`)
		assert.Contains(t, string(got), `"apis": []`)
		assert.NotContains(t, string(got), "null")
	})

	t.Run("html characters are not escaped", func(t *testing.T) {
		cfg := exampleConfig()
		cfg.Scopes["checkout"] = Scope{Description: "Pay <now> & save"}

		got, err := marshal(cfg)
		require.NoError(t, err)
		assert.Contains(t, string(got), "Pay <now> & save")
	})

	t.Run("scope_hints is omitted when no scope has hints", func(t *testing.T) {
		cfg := exampleConfig()
		cfg.Scopes = map[string]Scope{"a": {Description: "A"}}

		got, err := marshal(cfg)
		require.NoError(t, err)
		assert.NotContains(t, string(got), "scope_hints")
	})

	t.Run("does not modify its argument", func(t *testing.T) {
		cfg := &Config{}
		_, err := marshal(cfg)
		require.NoError(t, err)
		assert.Equal(t, &Config{}, cfg)
	})
}

func TestPassthrough(t *testing.T) {
	want, err := os.ReadFile(goldenFullFile)
	require.NoError(t, err)

	t.Run("a hand-edited file is written back unchanged", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), DefaultFileName)
		require.NoError(t, os.WriteFile(path, want, 0o644))

		cfg, err := Read(path)
		require.NoError(t, err)
		require.NoError(t, Write(path, cfg))

		written, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, string(want), string(written))
	})

	t.Run("members the typed fields do not cover are kept at every level", func(t *testing.T) {
		raw := `{
  "protocol_version": "0.4",
  "organization": {"name": "Shop", "domain": "shop.example", "x_org": 1},
  "auth": {"issuer": "https://auth.shop.example/", "scopes": {}, "x_auth": 2},
  "apis": [{"type": "openapi", "url": "https://api.shop.example/o.json", "description": "d", "x_api": 3}],
  "web": {"x_web": 4},
  "x_top": 5,
  "auth0": {"x_auth0": 6}
}`
		path := filepath.Join(t.TempDir(), DefaultFileName)
		require.NoError(t, os.WriteFile(path, []byte(raw), 0o644))

		cfg, err := Read(path)
		require.NoError(t, err)
		require.NoError(t, Write(path, cfg))

		written, err := os.ReadFile(path)
		require.NoError(t, err)

		var got map[string]any
		require.NoError(t, json.Unmarshal(written, &got))

		assert.EqualValues(t, 5, got["x_top"])
		assert.EqualValues(t, 1, got["organization"].(map[string]any)["x_org"])
		assert.EqualValues(t, 2, got["auth"].(map[string]any)["x_auth"])
		assert.EqualValues(t, 3, got["apis"].([]any)[0].(map[string]any)["x_api"])
		assert.EqualValues(t, 4, got["web"].(map[string]any)["x_web"])
		assert.EqualValues(t, map[string]any{"x_auth0": float64(6)}, got["auth0"])
	})

	t.Run("an empty web member is kept", func(t *testing.T) {
		raw := `{"protocol_version":"0.4","organization":{"name":"Shop","domain":"shop.example"},"web":{}}`

		cfg, err := Read(writeRaw(t, raw))
		require.NoError(t, err)

		got, err := cfg.encode()
		require.NoError(t, err)
		assert.JSONEq(t, `{"protocol_version":"0.4","organization":{"name":"Shop","domain":"shop.example"},"auth":{"scopes":{}},"apis":[],"web":{}}`, string(got))
	})

	t.Run("the cli writes only protocol members", func(t *testing.T) {
		got, err := marshal(exampleConfig())
		require.NoError(t, err)

		var doc map[string]any
		require.NoError(t, json.Unmarshal(got, &doc))

		for name := range doc {
			assert.Contains(t, topOrder, name)
		}

		assert.ElementsMatch(t, []string{"issuer", "scopes", "scope_hints"}, keysOf(doc["auth"].(map[string]any)))
		assert.ElementsMatch(t, []string{"browser_session_endpoint"}, keysOf(doc["web"].(map[string]any)))
		assert.ElementsMatch(t, []string{"type", "url", "description"}, keysOf(doc["apis"].([]any)[0].(map[string]any)))
	})
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	return out
}

func TestOrigin(t *testing.T) {
	tests := map[string]string{
		"https://api.shop.example/openapi.json": "https://api.shop.example",
		"https://API.Shop.example":              "https://api.shop.example",
		"https://api.shop.example:443":          "https://api.shop.example",
		"https://api.shop.example:443/x":        "https://api.shop.example",
		"https://api.shop.example:8443/x":       "https://api.shop.example:8443",
		"https://u@api.shop.example/x":          "https://api.shop.example",
		"https://[::1]:443/x":                   "https://[::1]",
		"http://api.shop.example":               "",
		"not a url":                             "",
	}

	for in, want := range tests {
		assert.Equal(t, want, Origin(in), in)
	}
}

func TestEntriesForNormalizesTheOrigin(t *testing.T) {
	cfg := &Config{APIs: []API{
		{URL: "https://API.shop.example/mcp"},
		{URL: "https://api.shop.example:443/openapi.json"},
		{URL: "https://api.shop.example:8443/other"},
	}}

	assert.Equal(t, []int{0, 1}, cfg.EntriesFor("https://api.shop.example"))
	assert.Equal(t, []int{0, 1}, cfg.EntriesFor("https://Api.shop.example:443/v1"))
}
