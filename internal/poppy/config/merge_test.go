package config

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const shopID = "https://api.shop.example"

// derivedConfig holds only what the tenant provides.
func derivedConfig() *Config {
	return &Config{
		Organization: Organization{Name: "Tenant Shop"},
		Issuer:       "https://auth.shop.example/",
		Scopes: map[string]Scope{
			"orders:read": {Description: "See your orders and their status. Read only."},
			"checkout":    {Description: "Place orders and pay with your saved card. Can't be undone."},
			// The default hint that setup gives this scope.
			"poppy:browser-sign-in": {
				Description: "Use the website signed in, with the other access you approve",
				Hints:       &Hints{Changes: "none", PersonalData: false},
			},
		},
	}
}

func fullInputs() Inputs {
	return Inputs{
		OrganizationName: "Shop",
		CompanyDomain:    "shop.example",
		ClientID:         "W1x9aBcD2eFgH3iJ",
		APIs: map[string]API{
			shopID: {URL: "https://api.shop.example/openapi.json", Description: "Orders, cart and checkout"},
		},
		Hints: map[string]Hints{
			"orders:read": {Changes: "none", PersonalData: true},
			"checkout":    {Changes: "irreversible", PersonalData: true},
		},
	}
}

func TestMerge(t *testing.T) {
	t.Run("no file and no inputs lists exactly the missing values", func(t *testing.T) {
		in := Inputs{APIs: map[string]API{shopID: {}, "https://admin.shop.example": {}}}

		got, err := Merge(nil, derivedConfig(), in, MergeOptions{})
		assert.Nil(t, got)

		var merr *MissingFieldsError
		require.True(t, errors.As(err, &merr))
		assert.Equal(t, []string{
			"apis[https://admin.shop.example].url",
			"apis[https://api.shop.example].url",
			"organization.domain",
		}, merr.Fields)
		assert.Equal(t, "missing required values: "+merr.Fields[0]+", "+merr.Fields[1]+", "+merr.Fields[2], err.Error())
	})

	t.Run("a missing organization name is a missing value", func(t *testing.T) {
		derived := derivedConfig()
		derived.Organization.Name = ""

		in := fullInputs()
		in.OrganizationName = ""

		_, err := Merge(nil, derived, in, MergeOptions{})

		var merr *MissingFieldsError
		require.True(t, errors.As(err, &merr))
		assert.Equal(t, []string{"organization.name"}, merr.Fields)
	})

	t.Run("a missing api description is a missing value, the default fills it", func(t *testing.T) {
		in := fullInputs()
		in.APIs = map[string]API{shopID: {URL: "https://api.shop.example/openapi.json"}}

		_, err := Merge(nil, derivedConfig(), in, MergeOptions{})

		var merr *MissingFieldsError
		require.True(t, errors.As(err, &merr))
		assert.Equal(t, []string{"apis[https://api.shop.example].description"}, merr.Fields)

		in.APIDescriptions = map[string]string{shopID: "Orders, cart and checkout"}
		got, err := Merge(nil, derivedConfig(), in, MergeOptions{})
		require.NoError(t, err)
		assert.Equal(t, exampleConfig(), got)

		// The existing description wins over the default, an input over both.
		in.APIDescriptions = map[string]string{shopID: "From the tenant"}
		got, err = Merge(exampleConfig(), derivedConfig(), in, MergeOptions{})
		require.NoError(t, err)
		assert.Equal(t, "Orders, cart and checkout", got.APIs[0].Description)

		in.APIs[shopID] = API{Description: "From the flag"}
		got, err = Merge(exampleConfig(), derivedConfig(), in, MergeOptions{})
		require.NoError(t, err)
		assert.Equal(t, "From the flag", got.APIs[0].Description)
	})

	t.Run("no file with all inputs gives the example config", func(t *testing.T) {
		got, err := Merge(nil, derivedConfig(), fullInputs(), MergeOptions{})
		require.NoError(t, err)
		assert.Equal(t, exampleConfig(), got)
		assert.Empty(t, got.Validate())
	})

	t.Run("existing file keeps domain, hints, apis and endpoint", func(t *testing.T) {
		got, err := Merge(exampleConfig(), derivedConfig(), Inputs{}, MergeOptions{})
		require.NoError(t, err)
		assert.Equal(t, exampleConfig().Organization.Domain, got.Organization.Domain)
		assert.Equal(t, exampleConfig().Scopes, got.Scopes)
		assert.Equal(t, exampleConfig().APIs, got.APIs)
		assert.Equal(t, exampleConfig().Web, got.Web)
	})

	for _, old := range []string{"0.3", "0.4", "1.1"} {
		t.Run("a file written at protocol "+old+" is written at 1.0 and the diff shows it", func(t *testing.T) {
			existing := exampleConfig()
			existing.ProtocolVersion = old

			derived := derivedConfig()
			derived.Organization.Name = "Shop"

			got, err := Merge(existing, derived, Inputs{}, MergeOptions{})

			require.NoError(t, err)
			assert.Equal(t, "1.0", got.ProtocolVersion)
			assert.Equal(t, []Change{{Path: "protocol_version", Kind: ChangeChanged, Old: old, New: "1.0"}}, Diff(existing, got))
		})
	}

	t.Run("web.unscoped and web.disabled are kept and never set", func(t *testing.T) {
		got, err := Merge(exampleConfig(), derivedConfig(), Inputs{}, MergeOptions{})
		require.NoError(t, err)
		assert.False(t, got.Web.Unscoped)
		assert.False(t, got.Web.Disabled)

		existing := exampleConfig()
		existing.Web.Unscoped, existing.Web.Disabled = true, true

		derived := derivedConfig()
		derived.Organization.Name = "Shop"

		got, err = Merge(existing, derived, Inputs{}, MergeOptions{})
		require.NoError(t, err)
		assert.True(t, got.Web.Unscoped)
		assert.True(t, got.Web.Disabled)
		assert.Empty(t, Diff(existing, got))
	})

	t.Run("existing file keeps the logo", func(t *testing.T) {
		existing := exampleConfig()
		existing.Organization.LogoURI = "https://shop.example/logo.png"

		got, err := Merge(existing, derivedConfig(), Inputs{}, MergeOptions{})
		require.NoError(t, err)
		assert.Equal(t, "https://shop.example/logo.png", got.Organization.LogoURI)
	})

	t.Run("inputs override the existing file", func(t *testing.T) {
		in := Inputs{
			CompanyDomain: "new.example",
			APIs: map[string]API{
				shopID: {Type: "mcp", URL: "https://api.shop.example/mcp", Description: "New"},
			},
			Hints: map[string]Hints{"checkout": {Changes: "reversible", PersonalData: false}},
		}

		got, err := Merge(exampleConfig(), derivedConfig(), in, MergeOptions{})
		require.NoError(t, err)

		assert.Equal(t, "new.example", got.Organization.Domain)
		assert.Equal(t, []API{{Type: "mcp", URL: "https://api.shop.example/mcp", Description: "New"}}, got.APIs)
		assert.Equal(t, &Hints{Changes: "reversible", PersonalData: false}, got.Scopes["checkout"].Hints)
		assert.Equal(t, &Hints{Changes: "none", PersonalData: true}, got.Scopes["orders:read"].Hints)
	})

	t.Run("tenant values override the existing file", func(t *testing.T) {
		derived := derivedConfig()
		derived.Scopes = map[string]Scope{"checkout": {Description: "Tenant description"}}

		got, err := Merge(exampleConfig(), derived, Inputs{ClientID: "NewClient"}, MergeOptions{})
		require.NoError(t, err)

		assert.Equal(t, ProtocolVersion, got.ProtocolVersion)
		assert.Equal(t, "Tenant Shop", got.Organization.Name)
		want := exampleConfig().Scopes
		want["checkout"] = Scope{
			Description: "Tenant description",
			Hints:       &Hints{Changes: "irreversible", PersonalData: true},
		}
		assert.Equal(t, want, got.Scopes)
		assert.Equal(t, "https://auth.shop.example/poppy/NewClient/browser-session", got.Web.BrowserSessionEndpoint)
	})

	t.Run("organization name prefers the input, then the tenant, then the file", func(t *testing.T) {
		existing := exampleConfig()

		derived := derivedConfig()
		got, err := Merge(existing, derived, Inputs{OrganizationName: "Flag"}, MergeOptions{})
		require.NoError(t, err)
		assert.Equal(t, "Flag", got.Organization.Name)

		got, err = Merge(existing, derived, Inputs{}, MergeOptions{})
		require.NoError(t, err)
		assert.Equal(t, "Tenant Shop", got.Organization.Name)

		derived.Organization.Name = ""
		got, err = Merge(existing, derived, Inputs{}, MergeOptions{})
		require.NoError(t, err)
		assert.Equal(t, "Shop", got.Organization.Name)
	})

	t.Run("a new api needs its url and gets an entry at the end", func(t *testing.T) {
		const id = "https://new.shop.example"

		_, err := Merge(exampleConfig(), derivedConfig(), Inputs{APIs: map[string]API{id: {}}}, MergeOptions{})
		var merr *MissingFieldsError
		require.True(t, errors.As(err, &merr))
		assert.Equal(t, []string{"apis[https://new.shop.example].url"}, merr.Fields)

		in := Inputs{APIs: map[string]API{id: {URL: "https://new.shop.example/openapi.json", Description: "New"}}}
		got, err := Merge(exampleConfig(), derivedConfig(), in, MergeOptions{})
		require.NoError(t, err)
		assert.Equal(t, []API{
			exampleConfig().APIs[0],
			{Type: "openapi", URL: "https://new.shop.example/openapi.json", Description: "New"},
		}, got.APIs)
	})

	t.Run("an entry is found by its url, else by the origin of the identifier", func(t *testing.T) {
		existing := exampleConfig()
		existing.APIs = []API{
			{Type: "mcp", URL: "https://api.shop.example/mcp", Description: "Search"},
			{Type: "openapi", URL: "https://api.shop.example/openapi.json", Description: "Orders"},
		}

		// By URL.
		in := Inputs{APIs: map[string]API{shopID: {URL: "https://api.shop.example/openapi.json", Description: "Changed"}}}
		got, err := Merge(existing, derivedConfig(), in, MergeOptions{})
		require.NoError(t, err)
		assert.Equal(t, existing.APIs[0], got.APIs[0])
		assert.Equal(t, API{Type: "openapi", URL: "https://api.shop.example/openapi.json", Description: "Changed"}, got.APIs[1])

		// By origin, with one entry there.
		existing.APIs = existing.APIs[1:]
		got, err = Merge(existing, derivedConfig(), Inputs{APIs: map[string]API{shopID: {}}}, MergeOptions{})
		require.NoError(t, err)
		assert.Equal(t, existing.APIs, got.APIs)
	})

	t.Run("two entries at the origin of an api are both kept and filled", func(t *testing.T) {
		existing := exampleConfig()
		existing.APIs = []API{
			{Type: "mcp", URL: "https://api.shop.example/mcp", Description: "Search"},
			{Type: "openapi", URL: "https://api.shop.example/openapi.json"},
		}

		in := Inputs{
			APIs:            map[string]API{shopID: {}},
			APIDescriptions: map[string]string{shopID: "From the tenant"},
		}

		got, err := Merge(existing, derivedConfig(), in, MergeOptions{})
		require.NoError(t, err)
		assert.Equal(t, []API{
			{Type: "mcp", URL: "https://api.shop.example/mcp", Description: "Search"},
			{Type: "openapi", URL: "https://api.shop.example/openapi.json", Description: "From the tenant"},
		}, got.APIs)
	})

	t.Run("two entries at the origin of an api and a url that is neither are ambiguous", func(t *testing.T) {
		existing := exampleConfig()
		existing.APIs = []API{
			{Type: "mcp", URL: "https://api.shop.example/mcp", Description: "Search"},
			{Type: "openapi", URL: "https://api.shop.example/openapi.json", Description: "Orders"},
		}

		in := Inputs{APIs: map[string]API{shopID: {URL: "https://api.shop.example/v2/openapi.json"}}}
		got, err := Merge(existing, derivedConfig(), in, MergeOptions{})
		assert.Nil(t, got)

		var aerr *AmbiguousAPIError
		require.True(t, errors.As(err, &aerr))
		assert.Equal(t, shopID, aerr.Identifier)
		assert.Equal(t, []string{"https://api.shop.example/mcp", "https://api.shop.example/openapi.json"}, aerr.URLs)
		assert.Contains(t, err.Error(), "2 APIs")
	})

	t.Run("scopes and hints of an api outside this run are kept", func(t *testing.T) {
		existing := exampleConfig()
		existing.Scopes["refunds"] = Scope{Description: "Refund an order", Hints: &Hints{Changes: "reversible", PersonalData: true, extra: map[string]json.RawMessage{"x-note": json.RawMessage(`"keep"`)}}}
		existing.Scopes["hand-written"] = Scope{Description: "Added by hand"}

		derived := derivedConfig()
		derived.Scopes = map[string]Scope{"checkout": {Description: "Tenant description"}}

		got, err := Merge(existing, derived, Inputs{}, MergeOptions{})
		require.NoError(t, err)

		assert.Equal(t, existing.Scopes["refunds"], got.Scopes["refunds"])
		assert.Equal(t, existing.Scopes["hand-written"], got.Scopes["hand-written"])
		assert.Equal(t, existing.Scopes["orders:read"], got.Scopes["orders:read"])
		assert.Equal(t, "Tenant description", got.Scopes["checkout"].Description)
		assert.Equal(t, existing.Scopes["checkout"].Hints, got.Scopes["checkout"].Hints)

		// The kept hints are copies.
		got.Scopes["refunds"].Hints.Changes = "changed"
		assert.Equal(t, "reversible", existing.Scopes["refunds"].Hints.Changes)
	})

	t.Run("two runs for two apis keep the scopes of both", func(t *testing.T) {
		const adminID = "https://admin.shop.example"

		first := derivedConfig()
		first.Scopes = map[string]Scope{"orders:read": {Description: "Read orders"}}
		in := Inputs{
			OrganizationName: "Shop",
			CompanyDomain:    "shop.example",
			APIs:             map[string]API{shopID: {URL: "https://api.shop.example/openapi.json", Description: "Orders"}},
			Hints:            map[string]Hints{"orders:read": {Changes: "none", PersonalData: true}},
		}

		afterFirst, err := Merge(nil, first, in, MergeOptions{})
		require.NoError(t, err)

		second := derivedConfig()
		second.Scopes = map[string]Scope{"users:write": {Description: "Change users"}}
		in = Inputs{
			APIs:  map[string]API{adminID: {URL: "https://admin.shop.example/openapi.json", Description: "Admin"}},
			Hints: map[string]Hints{"users:write": {Changes: "reversible", PersonalData: true}},
		}

		got, err := Merge(afterFirst, second, in, MergeOptions{})
		require.NoError(t, err)

		assert.Len(t, got.APIs, 2)
		assert.Equal(t, Scope{Description: "Read orders", Hints: &Hints{Changes: "none", PersonalData: true}}, got.Scopes["orders:read"])
		assert.Equal(t, Scope{Description: "Change users", Hints: &Hints{Changes: "reversible", PersonalData: true}}, got.Scopes["users:write"])
	})

	t.Run("apis of the tenant are handled in identifier order", func(t *testing.T) {
		in := fullInputs()
		in.APIs = map[string]API{
			"https://b.shop.example": {URL: "https://b.shop.example/o.json", Description: "B"},
			"https://a.shop.example": {URL: "https://a.shop.example/o.json", Description: "A"},
		}

		got, err := Merge(nil, derivedConfig(), in, MergeOptions{})
		require.NoError(t, err)
		require.Len(t, got.APIs, 2)
		assert.Equal(t, "https://a.shop.example/o.json", got.APIs[0].URL)
		assert.Equal(t, "https://b.shop.example/o.json", got.APIs[1].URL)
	})

	t.Run("scope hints for scopes the tenant does not have are ignored", func(t *testing.T) {
		in := fullInputs()
		in.Hints["gone"] = Hints{Changes: "none"}

		got, err := Merge(nil, derivedConfig(), in, MergeOptions{})
		require.NoError(t, err)
		assert.NotContains(t, got.Scopes, "gone")
	})

	t.Run("issuer mismatch", func(t *testing.T) {
		existing := exampleConfig()
		existing.Issuer = "https://other.example/"

		got, err := Merge(existing, derivedConfig(), Inputs{}, MergeOptions{})
		assert.Nil(t, got)
		require.ErrorIs(t, err, ErrIssuerMismatch)
		assert.Contains(t, err.Error(), "https://other.example/")
		assert.Contains(t, err.Error(), "https://auth.shop.example/")

		got, err = Merge(existing, derivedConfig(), Inputs{}, MergeOptions{ForceConfig: true})
		require.NoError(t, err)
		assert.Equal(t, "https://auth.shop.example/", got.Issuer)
	})

	t.Run("an existing file without an issuer is not a mismatch", func(t *testing.T) {
		existing := exampleConfig()
		existing.Issuer = ""

		_, err := Merge(existing, derivedConfig(), Inputs{}, MergeOptions{})
		require.NoError(t, err)
	})

	t.Run("derived values are required", func(t *testing.T) {
		_, err := Merge(nil, nil, Inputs{}, MergeOptions{})
		require.Error(t, err)
	})

	t.Run("no endpoint is built without a client id", func(t *testing.T) {
		in := fullInputs()
		in.ClientID = ""

		got, err := Merge(nil, derivedConfig(), in, MergeOptions{})
		require.NoError(t, err)
		assert.Empty(t, got.Web.BrowserSessionEndpoint)
	})

	t.Run("arguments are not mutated", func(t *testing.T) {
		existing, derived, in := exampleConfig(), derivedConfig(), fullInputs()
		in.APIs["https://b.shop.example"] = API{URL: "https://b.shop.example/o.json", Description: "B"}

		wantExisting, wantDerived, wantIn := exampleConfig(), derivedConfig(), fullInputs()
		wantIn.APIs["https://b.shop.example"] = API{URL: "https://b.shop.example/o.json", Description: "B"}

		got, err := Merge(existing, derived, in, MergeOptions{})
		require.NoError(t, err)

		// Changing the result must not reach the arguments either.
		got.Scopes["checkout"].Hints.Changes = "changed"
		got.APIs[0].Description = "changed"

		assert.Equal(t, wantExisting, existing)
		assert.Equal(t, wantDerived, derived)
		assert.Equal(t, wantIn, in)
	})

	t.Run("the default hint of a derived scope fills a gap and loses to the file and to inputs", func(t *testing.T) {
		existing := exampleConfig()
		existing.Scopes["poppy:browser-sign-in"] = Scope{Description: "x", Hints: &Hints{Changes: "reversible", PersonalData: true}}

		got, err := Merge(existing, derivedConfig(), Inputs{}, MergeOptions{})
		require.NoError(t, err)
		assert.Equal(t, &Hints{Changes: "reversible", PersonalData: true}, got.Scopes["poppy:browser-sign-in"].Hints)

		in := Inputs{Hints: map[string]Hints{"poppy:browser-sign-in": {Changes: "none", PersonalData: true}}}
		got, err = Merge(existing, derivedConfig(), in, MergeOptions{})
		require.NoError(t, err)
		assert.Equal(t, &Hints{Changes: "none", PersonalData: true}, got.Scopes["poppy:browser-sign-in"].Hints)

		existing.Scopes = map[string]Scope{}
		got, err = Merge(existing, derivedConfig(), Inputs{}, MergeOptions{})
		require.NoError(t, err)
		assert.Equal(t, &Hints{Changes: "none", PersonalData: false}, got.Scopes["poppy:browser-sign-in"].Hints)
		assert.Nil(t, got.Scopes["checkout"].Hints)
	})

	t.Run("members the cli does not use survive a merge", func(t *testing.T) {
		existing, err := Read(goldenFullFile)
		require.NoError(t, err)

		derived := derivedConfig()
		derived.Organization.Name = "Shop"

		in := Inputs{ClientID: "W1x9aBcD2eFgH3iJ", APIs: map[string]API{shopID: {}}}

		got, err := Merge(existing, derived, in, MergeOptions{})
		require.NoError(t, err)

		want, err := os.ReadFile(goldenFullFile)
		require.NoError(t, err)

		data, err := marshal(got)
		require.NoError(t, err)
		assert.Equal(t, string(want), string(data))
		assert.Empty(t, Diff(existing, got))
	})

	t.Run("the arguments keep their members when the result changes", func(t *testing.T) {
		existing, err := Read(goldenFullFile)
		require.NoError(t, err)

		got, err := Merge(existing, derivedConfig(), Inputs{}, MergeOptions{})
		require.NoError(t, err)

		got.extra.top["feedback"] = json.RawMessage(`{}`)
		assert.JSONEq(t, `{"endpoint":"https://api.shop.example/poppy/feedback"}`, string(existing.extra.top["feedback"]))
	})

	t.Run("an api the tenant does not list stays, and apis keep their place", func(t *testing.T) {
		existing := exampleConfig()
		existing.APIs = []API{
			{Type: "mcp", URL: "https://mcp.shop.example/mcp", Description: "Search"},
			existing.APIs[0],
		}

		derived := derivedConfig()
		derived.Organization.Name = "Shop"

		in := Inputs{APIs: map[string]API{
			shopID:                   {},
			"https://a.shop.example": {URL: "https://a.shop.example/o.json", Description: "A"},
		}}

		got, err := Merge(existing, derived, in, MergeOptions{})
		require.NoError(t, err)
		require.Len(t, got.APIs, 3)
		assert.Equal(t, existing.APIs[0], got.APIs[0])
		assert.Equal(t, existing.APIs[1], got.APIs[1])
		assert.Equal(t, "https://a.shop.example/o.json", got.APIs[2].URL)
		assert.Empty(t, got.Validate())
	})
}
