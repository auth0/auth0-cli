package display

import (
	"encoding/json"
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

func TestNetworkACLView_KeyValues_MatchAll(t *testing.T) {
	acl := &management.NetworkACL{
		ID:          strPtr("acl-match-all"),
		Description: strPtr("Deny All"),
		Priority:    intPtr(99),
		Active:      boolPtr(true),
		Rule: &management.NetworkACLRule{
			Scope:    strPtr("tenant"),
			Action:   &management.NetworkACLRuleAction{Block: boolPtr(true)},
			MatchAll: boolPtr(true),
		},
	}

	kvs := makeNetworkACLView(acl).KeyValues()

	value, ok := keyValue(kvs, "MATCH ALL")
	assert.True(t, ok, "expected key \"MATCH ALL\" to be present in KeyValues()")
	assert.Equal(t, "true", value)
}

// TestNetworkACLView_Object_IncludesID guards against a regression where storing
// a *management.NetworkACL in the view's raw field engaged that type's pointer
// receiver MarshalJSON, which emits only the writable subset of fields and drops
// "id". The integration tests read the id out of `network-acl create --json`, so
// losing it breaks every command that consumes a created ACL's identifier.
func TestNetworkACLView_Object_IncludesID(t *testing.T) {
	tests := []struct {
		name string
		acl  *management.NetworkACL
		want map[string]interface{}
	}{
		{
			name: "fully populated ACL",
			acl: &management.NetworkACL{
				ID:          strPtr("acl_6wZqimFkPpMFvqXRwjsq9J"),
				Description: strPtr("integration-test-acl"),
				Priority:    intPtr(9),
				Active:      boolPtr(false),
				Rule: &management.NetworkACLRule{
					Scope:  strPtr("tenant"),
					Action: &management.NetworkACLRuleAction{Log: boolPtr(true)},
					Match: &management.NetworkACLRuleMatch{
						IPv4Cidrs: &[]string{"192.168.1.5/24"},
					},
				},
			},
			want: map[string]interface{}{
				"id":          "acl_6wZqimFkPpMFvqXRwjsq9J",
				"description": "integration-test-acl",
				"priority":    float64(9),
				"active":      false,
			},
		},
		{
			name: "ACL with an empty description still reports its id",
			acl: &management.NetworkACL{
				ID:       strPtr("acl_2"),
				Priority: intPtr(1),
				Active:   boolPtr(true),
				Rule: &management.NetworkACLRule{
					Scope:  strPtr("tenant"),
					Action: &management.NetworkACLRuleAction{Block: boolPtr(true)},
				},
			},
			want: map[string]interface{}{
				"id":       "acl_2",
				"priority": float64(1),
				"active":   true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b, err := json.Marshal(makeNetworkACLView(test.acl).Object())
			assert.NoError(t, err)

			var got map[string]interface{}
			assert.NoError(t, json.Unmarshal(b, &got))

			for key, want := range test.want {
				value, ok := got[key]
				assert.True(t, ok, "expected %q to be present in --json output, got: %s", key, b)
				assert.Equal(t, want, value)
			}

			assert.Contains(t, got, "rule", "expected the rule to survive marshalling")
		})
	}
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }
func boolPtr(b bool) *bool    { return &b }
