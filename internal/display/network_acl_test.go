package display

import (
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/stretchr/testify/assert"
)

// keyValue returns the value for a given key in a KeyValues() slice, and whether it was present.
func keyValue(kvs [][]string, key string) (string, bool) {
	for _, kv := range kvs {
		if kv[0] == key {
			return kv[1], true
		}
	}
	return "", false
}

func TestNetworkACLView_KeyValues_Auth0Managed(t *testing.T) {
	tests := []struct {
		name      string
		acl       *management.NetworkACL
		wantKey   string
		wantValue string
	}{
		{
			name: "auth0_managed on match",
			acl: &management.NetworkACL{
				ID:          strPtr("acl-1"),
				Description: strPtr("Curated Blocklist"),
				Priority:    intPtr(1),
				Active:      boolPtr(true),
				Rule: &management.NetworkACLRule{
					Scope:  strPtr("tenant"),
					Action: &management.NetworkACLRuleAction{Block: boolPtr(true)},
					Match: &management.NetworkACLRuleMatch{
						Auth0Managed: &[]string{"auth0.low_reputation", "auth0.icloud_relay_proxy"},
					},
				},
			},
			wantKey:   "AUTH0 MANAGED",
			wantValue: "auth0.low_reputation, auth0.icloud_relay_proxy",
		},
		{
			name: "auth0_managed on not_match",
			acl: &management.NetworkACL{
				ID:          strPtr("acl-2"),
				Description: strPtr("Curated Blocklist Not Match"),
				Priority:    intPtr(2),
				Active:      boolPtr(true),
				Rule: &management.NetworkACLRule{
					Scope:  strPtr("tenant"),
					Action: &management.NetworkACLRuleAction{Block: boolPtr(true)},
					NotMatch: &management.NetworkACLRuleMatch{
						Auth0Managed: &[]string{"auth0.low_reputation"},
					},
				},
			},
			wantKey:   "NOT AUTH0 MANAGED",
			wantValue: "auth0.low_reputation",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			view := makeNetworkACLView(test.acl)
			kvs := view.KeyValues()

			value, ok := keyValue(kvs, test.wantKey)
			assert.True(t, ok, "expected key %q to be present in KeyValues()", test.wantKey)
			assert.Equal(t, test.wantValue, value)

			// The "NOT " label prefix already conveys the rule type, so no
			// redundant marker row is emitted.
			_, hasMarker := keyValue(kvs, "NOT MATCH")
			assert.False(t, hasMarker, "unexpected redundant \"NOT MATCH\" row")
		})
	}
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }
func boolPtr(b bool) *bool    { return &b }
