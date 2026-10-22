package proxy

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins/registry"
	"github.com/auth0/auth0-cli/internal/plugins/registry/registrytest"
)

// loadFixture reads the scope table and the secret fields of testdata.
func loadFixture(t *testing.T) (registry.ScopeTable, registry.SecretFields) {
	t.Helper()

	content, err := os.ReadFile("testdata/scope-table.json")
	require.NoError(t, err)

	var fixture struct {
		ScopeTable   registry.ScopeTable   `json:"scopeTable"`
		SecretFields registry.SecretFields `json:"secretFields"`
	}
	require.NoError(t, json.Unmarshal(content, &fixture))

	return fixture.ScopeTable, fixture.SecretFields
}

// testPolicy returns the policy of the fixture, after the index that holds it
// went through the registry parser.
func testPolicy(t *testing.T) *Policy {
	t.Helper()

	table, fields := loadFixture(t)
	index, err := registry.ParseIndex(registrytest.NewIndexBuilder().SetScopeTable(table).SetSecretFields(fields).Build())
	require.NoError(t, err)

	policy, err := NewPolicy(index)
	require.NoError(t, err)

	return policy
}

// indexWith returns an index with the given rules and secret field names.
func indexWith(rules []registry.ScopeRule, names ...string) *registry.Index {
	return &registry.Index{
		ScopeTable:   registry.ScopeTable{FormatVersion: 1, Rules: rules},
		SecretFields: registry.SecretFields{FormatVersion: 1, Names: names},
	}
}

func TestNewPolicy(t *testing.T) {
	oneRule := []registry.ScopeRule{{Method: "GET", Path: "/api/v2/clients", AnyOf: []string{"read:clients"}}}

	tests := []struct {
		name    string
		index   *registry.Index
		wantErr error
	}{
		{name: "a valid index", index: indexWith(oneRule, "client_secret")},
		{name: "no index", index: nil, wantErr: ErrNoScopeTable},
		{name: "an index without a scope table", index: &registry.Index{}, wantErr: ErrNoScopeTable},
		{name: "a scope table without rules", index: indexWith(nil, "client_secret"), wantErr: ErrNoScopeTable},
		{name: "a scope table of an unknown format", index: &registry.Index{
			ScopeTable:   registry.ScopeTable{FormatVersion: 2, Rules: oneRule},
			SecretFields: registry.SecretFields{FormatVersion: 1, Names: []string{"client_secret"}},
		}, wantErr: ErrNoScopeTable},
		{name: "no secret fields", index: &registry.Index{ScopeTable: registry.ScopeTable{FormatVersion: 1, Rules: oneRule}}, wantErr: ErrNoSecretFields},
		{name: "secret fields without names", index: indexWith(oneRule), wantErr: ErrNoSecretFields},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy, err := NewPolicy(test.index)

			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
				assert.Nil(t, policy)

				return
			}

			require.NoError(t, err)
			assert.NoError(t, policy.usable())
		})
	}
}

func TestNewPolicy_RejectsRulesThatDidNotComeFromTheParser(t *testing.T) {
	tests := []struct {
		name string
		rule registry.ScopeRule
	}{
		{name: "an unknown method", rule: registry.ScopeRule{Method: "HEAD", Path: "/api/v2/a", AnyOf: []string{"read:a"}}},
		{name: "no scope", rule: registry.ScopeRule{Method: "GET", Path: "/api/v2/a"}},
		{name: "an empty list of scopes", rule: registry.ScopeRule{Method: "GET", Path: "/api/v2/a", AnyOf: []string{}}},
		{name: "another prefix", rule: registry.ScopeRule{Method: "GET", Path: "/oauth/token", AnyOf: []string{"read:a"}}},
		{name: "a prefix in another case", rule: registry.ScopeRule{Method: "GET", Path: "/API/v2/a", AnyOf: []string{"read:a"}}},
		{name: "a dot dot segment", rule: registry.ScopeRule{Method: "GET", Path: "/api/v2/a/../b", AnyOf: []string{"read:a"}}},
		{name: "a trailing slash", rule: registry.ScopeRule{Method: "GET", Path: "/api/v2/a/", AnyOf: []string{"read:a"}}},
		{name: "an empty parameter", rule: registry.ScopeRule{Method: "GET", Path: "/api/v2/a/{}", AnyOf: []string{"read:a"}}},
		{name: "a misplaced brace", rule: registry.ScopeRule{Method: "GET", Path: "/api/v2/a/x{id}", AnyOf: []string{"read:a"}}},
		{name: "an empty scope", rule: registry.ScopeRule{Method: "GET", Path: "/api/v2/a", AnyOf: []string{""}}},
		{name: "a scope that is not a scope", rule: registry.ScopeRule{Method: "GET", Path: "/api/v2/a", AnyOf: []string{"read-a"}}},
		{name: "a scope in capitals", rule: registry.ScopeRule{Method: "GET", Path: "/api/v2/a", AnyOf: []string{"READ:A"}}},
		{name: "too many scopes", rule: registry.ScopeRule{Method: "GET", Path: "/api/v2/a", AnyOf: []string{"a:a", "a:b", "a:c", "a:d", "a:e", "a:f", "a:g", "a:h", "a:i"}}},
		{name: "a parameter that starts with a digit", rule: registry.ScopeRule{Method: "GET", Path: "/api/v2/a/{1id}", AnyOf: []string{"read:a"}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy, err := NewPolicy(indexWith([]registry.ScopeRule{test.rule}, "client_secret"))

			assert.Nil(t, policy)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "scopeTable.rules[0]")
		})
	}
}

func TestNewPolicy_RejectsWhatTheParserRejects(t *testing.T) {
	read := []string{"read:a"}

	tests := []struct {
		name  string
		rules []registry.ScopeRule
		names []string
		want  string
	}{
		{
			name:  "the same rule twice",
			rules: []registry.ScopeRule{{Method: "GET", Path: "/api/v2/a", AnyOf: read}, {Method: "GET", Path: "/api/v2/a", AnyOf: []string{"read:b"}}},
			names: []string{"client_secret"},
			want:  "scopeTable.rules[1]: duplicate rule",
		},
		{
			name:  "rules that only differ in the name of a parameter",
			rules: []registry.ScopeRule{{Method: "GET", Path: "/api/v2/a/{id}", AnyOf: read}, {Method: "GET", Path: "/api/v2/a/{aId}", AnyOf: []string{"read:b"}}},
			names: []string{"client_secret"},
			want:  "scopeTable.rules[1]: duplicate rule",
		},
		{
			name:  "a secret field name with a dot",
			rules: []registry.ScopeRule{{Method: "GET", Path: "/api/v2/a", AnyOf: read}},
			names: []string{"options.client_secret"},
			want:  "secretFields.names[0]:",
		},
		{
			name:  "secret field names that only differ in case",
			rules: []registry.ScopeRule{{Method: "GET", Path: "/api/v2/a", AnyOf: read}},
			names: []string{"client_secret", "Client_Secret"},
			want:  "secretFields.names[1]: duplicate name",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy, err := NewPolicy(indexWith(test.rules, test.names...))

			assert.Nil(t, policy)
			require.ErrorIs(t, err, registry.ErrInvalidIndex)
			assert.Contains(t, err.Error(), test.want)
		})
	}
}

func TestPolicy_match(t *testing.T) {
	rules := []registry.ScopeRule{
		// Written in the order that makes a naive first match wrong.
		{Method: "GET", Path: "/api/v2/clients/{id}", AnyOf: []string{"read:client"}},
		{Method: "GET", Path: "/api/v2/clients/count", AnyOf: []string{"read:count"}},
		{Method: "GET", Path: "/api/v2/a/{x}/c", AnyOf: []string{"read:param_first"}},
		{Method: "GET", Path: "/api/v2/a/b/{y}", AnyOf: []string{"read:literal_first"}},
		{Method: "GET", Path: "/api/v2/a/b/c", AnyOf: []string{"read:exact"}},
		{Method: "DELETE", Path: "/api/v2/clients/{id}", AnyOf: []string{"delete:client"}},
		{Method: "GET", Path: "/api/v2/Users", AnyOf: []string{"read:users"}},
	}

	policy, err := NewPolicy(indexWith(rules, "client_secret"))
	require.NoError(t, err)

	tests := []struct {
		name     string
		method   string
		segments []string
		want     string
	}{
		{name: "a parameter matches one segment", method: "GET", segments: []string{"clients", "abc"}, want: "read:client"},
		{name: "a literal beats a parameter", method: "GET", segments: []string{"clients", "count"}, want: "read:count"},
		{name: "the first difference decides, a literal beats a parameter", method: "GET", segments: []string{"a", "b", "x"}, want: "read:literal_first"},
		{name: "an exact rule beats both", method: "GET", segments: []string{"a", "b", "c"}, want: "read:exact"},
		{name: "a parameter in the middle", method: "GET", segments: []string{"a", "z", "c"}, want: "read:param_first"},
		{name: "the method picks the rule", method: "DELETE", segments: []string{"clients", "abc"}, want: "delete:client"},
		{name: "a method the rule does not list", method: "PATCH", segments: []string{"clients", "abc"}},
		{name: "a method with a literal rule only", method: "DELETE", segments: []string{"clients", "count"}, want: "delete:client"},
		{name: "one segment too many", method: "GET", segments: []string{"clients", "abc", "def"}},
		{name: "one segment too few", method: "GET", segments: []string{"clients"}},
		{name: "a literal is case sensitive", method: "GET", segments: []string{"users"}},
		{name: "a literal matches in its own case", method: "GET", segments: []string{"Users"}, want: "read:users"},
		{name: "a parameter does not match an empty segment", method: "GET", segments: []string{"clients", ""}},
		{name: "an unknown resource", method: "GET", segments: []string{"other"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			matched, ok := policy.match(test.method, test.segments)

			assert.Equal(t, test.want != "", ok)

			if test.want != "" {
				assert.Equal(t, []string{test.want}, matched.anyOf)
			}
		})
	}
}

func TestRule_allowedBy(t *testing.T) {
	r := rule{anyOf: []string{"read:clients", "read:client_keys"}}

	tests := []struct {
		name     string
		declared []string
		want     bool
	}{
		{name: "the first alternative", declared: []string{"read:clients"}, want: true},
		{name: "the other alternative", declared: []string{"read:users", "read:client_keys"}, want: true},
		{name: "none of them", declared: []string{"read:users"}},
		{name: "nothing declared"},
		{name: "a scope in another case", declared: []string{"READ:CLIENTS"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, r.allowedBy(scopeSet(test.declared...)))
		})
	}
}

func TestPolicy_isSecret(t *testing.T) {
	policy := testPolicy(t)

	assert.True(t, policy.isSecret("client_secret"))
	assert.True(t, policy.isSecret("Client_Secret"))
	assert.True(t, policy.isSecret("SIGNING_KEYS"))
	assert.False(t, policy.isSecret("client_id"))
	assert.False(t, policy.isSecret("client_secret_expires_at"))
}

func TestPolicy_isSecretFollowsEqualFold(t *testing.T) {
	policy := testPolicy(t)

	tests := []struct {
		name string
		key  string
	}{
		{name: "a long s", key: "client_\u017fecret"},
		{name: "a capital long s", key: "CLIENT_\u017fECRET"},
		{name: "a Kelvin sign", key: "signing_\u212aeys"},
		{name: "a Kelvin sign in the middle of a name", key: "private_\u212aey"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// The test is only about names that EqualFold treats as the same.
			require.True(t, strings.EqualFold(test.key, "client_secret") || strings.EqualFold(test.key, "signing_keys") || strings.EqualFold(test.key, "private_key"))
			assert.True(t, policy.isSecret(test.key))

			got, err := stripSecrets([]byte(`{"`+test.key+`":"s","ok":1}`), policy.isSecret)
			require.NoError(t, err)
			assert.JSONEq(t, `{"ok":1}`, string(got))
		})
	}

	assert.False(t, policy.isSecret("client_\u0131secret"), "a dotless i is not a case variant of anything in the list")
}
