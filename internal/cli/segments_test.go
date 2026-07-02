package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSegmentAttributesAndConditions(t *testing.T) {
	// These lists are derived from the SDK types via reflection. Assert the
	// expected members so an unexpected SDK change is caught here rather than
	// silently altering the CLI's help and validation.
	assert.ElementsMatch(t, []string{
		"client_id", "connection", "connection_type", "organization_id",
		"domain", "device_type", "browser", "platform", "user_agent",
		"country", "region",
	}, segmentAttributes)

	assert.ElementsMatch(t, []string{
		"contains", "starts_with", "ends_with", "exists",
	}, segmentConditions)
}

func TestParseSegmentRules_Valid(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "single match with operator",
			raw:  `[{"match":{"domain":{"ends_with":["example.com"]}}}]`,
		},
		{
			name: "plain list is an exact match",
			raw:  `[{"match":{"country":["US"]}}]`,
		},
		{
			name: "multiple attributes in one match",
			raw:  `[{"match":{"country":["US"],"browser":{"contains":["Chrome"]}}}]`,
		},
		{
			name: "match and not_match together",
			raw:  `[{"match":{"domain":{"ends_with":["example.com"]}},"not_match":{"country":["US"]}}]`,
		},
		{
			name: "not_match only",
			raw:  `[{"not_match":{"connection":["google-oauth2"]}}]`,
		},
		{
			name: "exists condition",
			raw:  `[{"match":{"organization_id":{"exists":true}}}]`,
		},
		{
			name: "multiple rules",
			raw:  `[{"match":{"domain":{"contains":["a.com"]}}},{"not_match":{"country":["US"]}}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules, err := parseSegmentRules(tt.raw)
			assert.NoError(t, err)
			assert.NotEmpty(t, rules)
		})
	}
}

func TestParseSegmentRules_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{
			name:    "operator used as attribute",
			raw:     `[{"match":{"contains":["@example.com"]}}]`,
			wantErr: `rule[0].match.contains: unknown attribute`,
		},
		{
			name:    "unknown condition operator",
			raw:     `[{"match":{"domain":{"has":["x"]}}}]`,
			wantErr: `rule[0].match.domain.has: unknown condition`,
		},
		{
			name:    "unknown top-level key",
			raw:     `[{"foo":{"domain":["x"]}}]`,
			wantErr: `rule[0].foo: unknown key`,
		},
		{
			name:    "unknown attribute under not_match",
			raw:     `[{"not_match":{"bogus":["x"]}}]`,
			wantErr: `rule[0].not_match.bogus: unknown attribute`,
		},
		{
			name:    "error reports correct rule index",
			raw:     `[{"match":{"domain":{"contains":["a.com"]}}},{"match":{"nope":["x"]}}]`,
			wantErr: `rule[1].match.nope: unknown attribute`,
		},
		{
			name:    "invalid json",
			raw:     `not json`,
			wantErr: `invalid JSON for --rules`,
		},
		{
			name:    "object instead of array",
			raw:     `{"match":{"domain":["x"]}}`,
			wantErr: `invalid JSON for --rules`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseSegmentRules(tt.raw)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
