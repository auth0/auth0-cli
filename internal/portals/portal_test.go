package portals_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/portals"
)

func TestPortal_APageSentBackIsNotChanged(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		json string
	}{
		{
			name: "a field the package does not know is kept",
			json: `{"title":"Profile","slug":"profile","description":"Your profile","components":[]}`,
		},
		{
			name: "an empty list of components stays an empty list",
			json: `{"title":"Billing","slug":"billing","components":[]}`,
		},
		{
			name: "a page with no list of components has none",
			json: `{"title":"Billing","slug":"billing"}`,
		},
		{
			name: "the fields of a component the package does not know are kept",
			json: `{"title":"P","slug":"p","components":[{"type":"page:component:auth0:form","config":{"form_id":"ap_1"},"visible":false}]}`,
		},
		{
			name: "a component with an empty config keeps it",
			json: `{"title":"P","slug":"p","components":[{"type":"page:component:auth0:my_account:mfa_management","config":{}}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var page portals.Page
			require.NoError(t, json.Unmarshal([]byte(tt.json), &page))

			sent, err := json.Marshal(page)

			require.NoError(t, err)
			assert.JSONEq(t, tt.json, string(sent))
		})
	}
}

func TestPortal_APageIsReadForItsFields(t *testing.T) {
	t.Parallel()

	var page portals.Page
	require.NoError(t, json.Unmarshal([]byte(`{"title":"Profile","slug":"profile","extra":1,"components":[{"type":"a","config":{"children":[]}}]}`), &page))

	assert.Equal(t, "Profile", page.Title)
	assert.Equal(t, "profile", page.Slug)
	require.Len(t, page.Components, 1)
	assert.Equal(t, "a", page.Components[0].Type)
	assert.Equal(t, map[string]any{"children": []any{}}, page.Components[0].Config)
}

func TestPortal_APortalKeepsItsPagesWhenReadAndSent(t *testing.T) {
	t.Parallel()

	const source = `{"id":"up_1","slug":"acme","pages":{"default":"profile","standard":[{"title":"Profile","slug":"profile","hidden":true}]}}`

	var portal portals.Portal
	require.NoError(t, json.Unmarshal([]byte(source), &portal))

	sent, err := json.Marshal(portals.PortalPatch{Pages: portal.Pages})

	require.NoError(t, err)
	assert.JSONEq(t, `{"pages":{"default":"profile","standard":[{"title":"Profile","slug":"profile","hidden":true}]}}`, string(sent))
}

func TestPortal_APageThatCannotBeReadSaysWhichField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		json  string
		field string
	}{
		{name: "a title that is not text", json: `{"title":5,"slug":"profile"}`, field: "title"},
		{name: "components that are not a list", json: `{"title":"Profile","slug":"profile","components":"none"}`, field: "components"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var page portals.Page

			err := json.Unmarshal([]byte(tt.json), &page)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.field)
		})
	}
}
