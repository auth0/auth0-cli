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
		Section: "Customize Actions",
		Type:    "Doc",
		URL:     "https://auth0.com/docs/customize/actions.md",
		Snippet: "Actions are secure.",
		Score:   9.5,
	}

	b, err := json.Marshal(v.Object())
	require.NoError(t, err)

	// Field order is stable: title first, score last (a struct, not a map).
	assert.Equal(t,
		`{"title":"Actions","section":"Customize Actions","type":"Doc","url":"https://auth0.com/docs/customize/actions.md","snippet":"Actions are secure.","score":9.5}`,
		string(b),
	)
}

// TestShortSection verifies the breadcrumb collapse used for the table view: short
// paths pass through unchanged, while long ones keep only the top category and leaf
// with an ellipsis in between.
func TestShortSection(t *testing.T) {
	tests := []struct {
		name     string
		section  string
		expected string
	}{
		{
			name:     "empty",
			section:  "",
			expected: "",
		},
		{
			name:     "single segment",
			section:  "Customize",
			expected: "Customize",
		},
		{
			name:     "three segments unchanged",
			section:  "Customize > Brand Customization > Portals",
			expected: "Customize > Brand Customization > Portals",
		},
		{
			name:     "four segments collapsed",
			section:  "Customize > Brand Customization > Customize Portals > Universal Portals",
			expected: "Customize > … > Universal Portals",
		},
		{
			name:     "seven segments collapsed",
			section:  "Get Started > Learn the Basics > Architecture Scenarios > Business Use Case Planning Guides > Business to Business > Launch Preparation (B2B) > Support Readiness (B2B)",
			expected: "Get Started > … > Support Readiness (B2B)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, shortSection(tt.section))
		})
	}
}
