package setup

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/poppy/config"
)

func TestResolveAPIs(t *testing.T) {
	const other = "https://other.example"

	poppy := []API{{Identifier: shopAPI}, {Identifier: other}, {Identifier: "urn:shop:api"}}

	file := func(urls ...string) *config.Config {
		cfg := &config.Config{}
		for _, url := range urls {
			cfg.APIs = append(cfg.APIs, config.API{Type: "openapi", URL: url, Description: "d"})
		}

		return cfg
	}

	tests := []struct {
		name       string
		file       *config.Config
		poppy      []API
		want       []string
		wantReason string
		wantText   string
		wantCands  []string
	}{
		{name: "one entry at the origin of an api", file: file(shopAPI + "/openapi.json"), poppy: poppy, want: []string{shopAPI}},
		{name: "two apis in file order", file: file(other+"/o.json", shopAPI+"/openapi.json"), poppy: poppy, want: []string{other, shopAPI}},
		{name: "two entries of one api give it once", file: file(shopAPI+"/openapi.json", shopAPI+"/mcp"), poppy: poppy, want: []string{shopAPI}},
		{name: "the origin is compared in lowercase and without the default port", file: file("https://API.Shop.example:443/openapi.json"), poppy: poppy, want: []string{shopAPI}},
		{name: "an entry no api is at is skipped", file: file("https://mcp.vendor.example/mcp", shopAPI+"/openapi.json"), poppy: poppy, want: []string{shopAPI}},
		{
			name:  "apis on one host with their own entries",
			file:  file(shopAPI+"/users/openapi.json", shopAPI+"/orders/mcp"),
			poppy: []API{{Identifier: shopAPI + "/orders"}, {Identifier: shopAPI + "/users"}},
			want:  []string{shopAPI + "/users", shopAPI + "/orders"},
		},
		{
			name:  "a root api and an api with a path each have their entries",
			file:  file(shopAPI+"/orders/openapi.json", shopAPI+"/openapi.json"),
			poppy: []API{{Identifier: shopAPI}, {Identifier: shopAPI + "/orders"}},
			want:  []string{shopAPI + "/orders", shopAPI},
		},
		{
			name:  "the api with the longest path takes the entry",
			file:  file(shopAPI+"/v1/openapi.json", shopAPI+"/v1/orders/openapi.json"),
			poppy: []API{{Identifier: shopAPI + "/v1"}, {Identifier: shopAPI + "/v1/orders"}},
			want:  []string{shopAPI + "/v1", shopAPI + "/v1/orders"},
		},
		{
			name:  "an api with no entry by path does not take the entry of another",
			file:  file(shopAPI+"/b/openapi.json", shopAPI+"/x/openapi.json"),
			poppy: []API{{Identifier: shopAPI + "/a"}, {Identifier: shopAPI + "/b"}},
			want:  []string{shopAPI + "/b", shopAPI + "/a"},
		},
		{
			name:       "no file",
			poppy:      poppy,
			wantReason: "api_not_found",
			wantText:   "There is no config file, so no API of the tenant is known.\n  FIX  Pass --api <identifier> to name the API, or create the file with auth0 poppy setup.",
		},
		{
			name:       "no entry matches",
			file:       file("https://mcp.vendor.example/mcp"),
			poppy:      poppy,
			wantReason: "api_not_found",
			wantText:   "the config file lists no URL at the identifier of an API that has use_for_personal_agent_protocol on.\n  FIX  Pass --api <identifier> to name the API.",
		},
		{
			name:       "an api with a path in the identifier is not at an origin only",
			file:       file(shopAPI + "/openapi.json"),
			poppy:      []API{{Identifier: shopAPI + "/v1"}, {Identifier: shopAPI + "/v2"}},
			wantReason: "ambiguous_api",
			wantText:   "could be any of these APIs: " + shopAPI + "/v1, " + shopAPI + "/v2.\n  FIX  Pass --api <identifier>",
			wantCands:  []string{shopAPI + "/v1", shopAPI + "/v2"},
		},
		{
			name:       "an identifier that is not a url never matches",
			file:       file("https://shop.example/openapi.json"),
			poppy:      []API{{Identifier: "urn:shop:api"}},
			wantReason: "api_not_found",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ResolveAPIs(test.file, test.poppy)

			if test.wantReason == "" {
				require.NoError(t, err)
				assert.Equal(t, test.want, got)

				return
			}

			var input *InputError
			require.ErrorAs(t, err, &input)
			assert.Equal(t, test.wantReason, input.Reason)
			assert.Contains(t, err.Error(), test.wantText)
			assert.Equal(t, test.wantCands, input.Candidates)
		})
	}
}

func TestResolveApp(t *testing.T) {
	one := App{ClientID: shopApp, Name: "Shop Web"}

	got, err := ResolveApp([]App{one})
	require.NoError(t, err)
	assert.Equal(t, shopApp, got)

	_, err = ResolveApp(nil)
	var input *InputError
	require.ErrorAs(t, err, &input)
	assert.Equal(t, "app_not_found", input.Reason)
	assert.Contains(t, err.Error(), "FIX  Pass --app <client ID or name>")

	_, err = ResolveApp([]App{one, {ClientID: "T3rdPartyApp0001", Name: "Partner Portal"}})
	require.ErrorAs(t, err, &input)
	assert.Equal(t, "ambiguous_app", input.Reason)
	assert.Contains(t, err.Error(), "Shop Web ("+shopApp+"), Partner Portal (T3rdPartyApp0001)")
	assert.Contains(t, err.Error(), "FIX  Pass --app")
	assert.Equal(t, []string{shopApp, "T3rdPartyApp0001"}, input.Candidates)
	assert.NotContains(t, input.Message, "FIX")
}

func TestDeriveWithSeveralFileEntriesAtTheOrigin(t *testing.T) {
	state, in := configuredState(t), configuredInputs()
	state.File.APIs = append(state.File.APIs, config.API{Type: "mcp", URL: shopAPI + "/mcp", Description: "Search"})

	t.Run("no url given updates every entry", func(t *testing.T) {
		cfg, err := Derive(state, in)

		require.NoError(t, err)
		assert.Equal(t, state.File.APIs, cfg.APIs)
	})

	t.Run("the url of one entry names it", func(t *testing.T) {
		in := in
		in.APIURLs = map[string]string{shopAPI: shopAPI + "/openapi.json"}

		cfg, err := Derive(state, in)

		require.NoError(t, err)
		assert.Equal(t, state.File.APIs, cfg.APIs)
	})

	t.Run("a url that is none of them is ambiguous", func(t *testing.T) {
		in := in
		in.APIURLs = map[string]string{shopAPI: shopAPI + "/v2/openapi.json"}

		_, err := Derive(state, in)

		var input *InputError
		require.ErrorAs(t, err, &input)
		assert.Equal(t, "ambiguous_api_entry", input.Reason)
		assert.Contains(t, err.Error(), "FIX  Pass --api-url "+shopAPI+"=<url>")
		assert.Equal(t, []string{shopAPI + "/openapi.json", shopAPI + "/mcp"}, input.Candidates)
	})
}

func TestDeriveWithTwoAPIsOnOneHost(t *testing.T) {
	orders, users := shopAPI+"/orders", shopAPI+"/users"

	state, in := configuredState(t), configuredInputs()

	second := state.APIs[0]
	state.APIs[0].Identifier = orders
	second.ID, second.Identifier = "rs_users", users
	state.APIs = append(state.APIs, second)
	state.File.APIs = []config.API{
		{Type: "openapi", URL: orders + "/openapi.json", Description: "Orders"},
		{Type: "openapi", URL: users + "/openapi.json", Description: "Users"},
	}
	in.APIs = []string{orders, users}

	cfg, err := Derive(state, in)

	require.NoError(t, err)
	assert.Equal(t, state.File.APIs, cfg.APIs)
	assert.Empty(t, APIURLQuestions(state, in))

	t.Run("an api that no entry is for is asked for its URL", func(t *testing.T) {
		state.File.APIs = state.File.APIs[:1]

		questions := APIURLQuestions(state, in)

		require.Len(t, questions, 1)
		assert.Contains(t, questions[0].Title, users)
	})
}

func TestAPIsOnOneHostShareTheFileEntries(t *testing.T) {
	tests := []struct {
		name        string
		identifiers []string
		urls        map[string]string
		entries     []string
		wantAsked   []string
	}{
		{
			name:        "a root API and an API with a path",
			identifiers: []string{shopAPI, shopAPI + "/orders"},
			entries:     []string{"/orders/openapi.json", "/openapi.json"},
		},
		{
			name:        "nested paths",
			identifiers: []string{shopAPI + "/v1", shopAPI + "/v1/orders"},
			entries:     []string{"/v1/openapi.json", "/v1/orders/openapi.json"},
		},
		{
			name:        "an API with no entry of its own does not take the entry of another",
			identifiers: []string{shopAPI + "/a", shopAPI + "/b"},
			entries:     []string{"/b/openapi.json", "/x/openapi.json"},
		},
		{
			name:        "a URL that names an entry claims only that entry",
			identifiers: []string{shopAPI, shopAPI + "/b"},
			urls:        map[string]string{shopAPI: shopAPI + "/openapi.json"},
			entries:     []string{"/openapi.json", "/b/openapi.json"},
		},
		{
			name:        "an API that no entry is for is asked for its URL",
			identifiers: []string{shopAPI + "/a", shopAPI + "/b"},
			entries:     []string{"/b/openapi.json"},
			wantAsked:   []string{shopAPI + "/a"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state, in := configuredState(t), configuredInputs()
			in.APIs, in.APIURLs = test.identifiers, test.urls

			state.APIs = nil
			state.File.APIs = nil

			for i, identifier := range test.identifiers {
				api := configuredState(t).APIs[0]
				api.ID, api.Identifier = fmt.Sprintf("rs_%d", i), identifier
				state.APIs = append(state.APIs, api)
			}

			for _, path := range test.entries {
				state.File.APIs = append(state.File.APIs, config.API{Type: "openapi", URL: shopAPI + path, Description: "Entry " + path})
			}

			var asked []string
			for _, question := range APIURLQuestions(state, in) {
				asked = append(asked, question.Title)
			}

			require.Len(t, asked, len(test.wantAsked))

			for i, identifier := range test.wantAsked {
				assert.Contains(t, asked[i], identifier)
			}

			if len(test.wantAsked) > 0 {
				return
			}

			cfg, err := Derive(state, in)

			require.NoError(t, err)
			assert.Equal(t, state.File.APIs, cfg.APIs)

			var poppy []API
			for _, api := range state.APIs {
				poppy = append(poppy, API{Identifier: api.Identifier})
			}

			found, err := ResolveAPIs(state.File, poppy)

			require.NoError(t, err)
			assert.ElementsMatch(t, test.identifiers, found)
		})
	}
}
