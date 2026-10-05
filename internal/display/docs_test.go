package display

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDocsSearchResultObject verifies the JSON emitted for a search hit carries the
// expected keys in a stable, logical order (title first, score last) rather than the
// alphabetical order a map would produce.
func TestDocsSearchResultObject(t *testing.T) {
	v := &DocsSearchResult{
		Title:   "Actions",
		Type:    "Doc",
		URL:     "https://auth0.com/docs/customize/actions.md",
		Snippet: "Actions are secure.",
		Score:   9.5,
	}

	b, err := json.Marshal(v.Object())
	require.NoError(t, err)

	// Field order is stable: title first, score last (a struct, not a map).
	assert.Equal(t,
		`{"title":"Actions","type":"Doc","url":"https://auth0.com/docs/customize/actions.md","snippet":"Actions are secure.","score":9.5}`,
		string(b),
	)
}
