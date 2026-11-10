package devpolicy

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins/proxy"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
)

const oneRule = `{"method":"GET","path":"/api/v2/clients","anyOf":["read:clients"]}`

// write writes a dev policy in a new directory and returns its path.
func write(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), FileName)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

func TestPathFor(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "build")
	override := filepath.Join(t.TempDir(), "other.json")

	assert.Equal(t, filepath.Join(dir, FileName), PathFor(dir, ""))
	assert.Equal(t, override, PathFor(dir, override))
}

func TestLoad_RefusesWhatIsNotTrustworthy(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"not JSON", `{"rules":`},
		{"empty", ``},
		{"an array", `[]`},
		{"an unknown field", `{"rules":[` + oneRule + `],"extra":true}`},
		{"an unknown field of a rule", `{"rules":[{"method":"GET","path":"/api/v2/clients","anyOf":["read:clients"],"extra":1}]}`},
		{"a duplicate key", `{"rules":[` + oneRule + `],"rules":[]}`},
		{"a duplicate key in a rule", `{"rules":[{"method":"GET","method":"POST","path":"/api/v2/clients","anyOf":["read:clients"]}]}`},
		{"a field name in the wrong case", `{"Rules":[` + oneRule + `]}`},
		{"a rule field in the wrong case", `{"rules":[{"METHOD":"GET","path":"/api/v2/clients","anyOf":["read:clients"]}]}`},
		{"the same key in two cases", `{"rules":[` + oneRule + `],"RULES":[{"method":"DELETE","path":"/api/v2/{a}","anyOf":["read:clients"]}]}`},
		{"the same rule key in two cases", `{"rules":[{"method":"GET","Method":"DELETE","path":"/api/v2/clients","anyOf":["read:clients"]}]}`},
		{"a field of the wrong type", `{"rules":"all"}`},
		{"two documents", `{"rules":[` + oneRule + `]}{}`},
		{"a file larger than allowed", `{"rules":[` + oneRule + `],"secretFields":["` + strings.Repeat("x", maxFileBytes) + `"]}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := write(t, test.content)

			file, err := Load(path)

			require.Error(t, err)
			assert.Nil(t, file)
			assert.ErrorIs(t, err, ErrInvalid)
			assert.NotErrorIs(t, err, ErrMissing)
			assert.Contains(t, err.Error(), path, "the error names the file")
		})
	}

	t.Run("a directory", func(t *testing.T) {
		dir := t.TempDir()

		_, err := Load(dir)

		assert.ErrorIs(t, err, ErrInvalid)
		assert.Contains(t, err.Error(), dir)
	})
}

func TestLoad_AMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)

	file, err := Load(path)

	require.Error(t, err)
	assert.Nil(t, file)
	assert.ErrorIs(t, err, ErrMissing)
	assert.NotErrorIs(t, err, ErrInvalid)
	assert.Contains(t, err.Error(), path)
	assert.Contains(t, err.Error(), "dev-policy.json next to plugin.json")
	assert.Contains(t, err.Error(), "auth0 plugin dev link --policy <file>")
}

func TestLoad_AMissingFileWithAnotherName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "other.json")

	file, err := Load(path)

	require.Error(t, err)
	assert.Nil(t, file)
	assert.ErrorIs(t, err, ErrMissing)
	assert.Contains(t, err.Error(), path)
	assert.NotContains(t, err.Error(), "next to plugin.json", "creating the default file would not help")
	assert.Contains(t, err.Error(), "auth0 plugin dev link --policy <file>")
}

func TestLoad_AValidFile(t *testing.T) {
	path := write(t, `{"rules":[`+oneRule+`],"secretFields":["token_value"]}`)

	file, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, []registry.ScopeRule{{Method: "GET", Path: "/api/v2/clients", AnyOf: []string{"read:clients"}}}, file.Rules)
	assert.Equal(t, []string{"token_value"}, file.SecretFields)
}

func TestFile_IndexRefusesWhatTheRegistryWould(t *testing.T) {
	rule := func(method, path string, scopes ...string) registry.ScopeRule {
		return registry.ScopeRule{Method: method, Path: path, AnyOf: scopes}
	}

	clients := []registry.ScopeRule{rule("GET", "/api/v2/clients", "read:clients")}

	tests := []struct {
		name string
		file File
		want string
	}{
		{"no rule", File{}, "no rule"},
		{"a rule list that is empty", File{Rules: []registry.ScopeRule{}}, "no rule"},
		{"a path outside the Management API", File{Rules: []registry.ScopeRule{rule("GET", "/clients", "read:clients")}}, "scopeTable.rules[0].path"},
		{"a method that is not served", File{Rules: []registry.ScopeRule{rule("TRACE", "/api/v2/clients", "read:clients")}}, "scopeTable.rules[0].method"},
		{"a rule with no scope", File{Rules: []registry.ScopeRule{rule("GET", "/api/v2/clients")}}, "scopeTable.rules[0].anyOf"},
		{"a scope that is not one", File{Rules: []registry.ScopeRule{rule("GET", "/api/v2/clients", "READ CLIENTS")}}, "scopeTable.rules[0].anyOf"},
		{"a dot segment", File{Rules: []registry.ScopeRule{rule("GET", "/api/v2/clients/..", "read:clients")}}, "must not be"},
		{"two rules for the same call", File{Rules: []registry.ScopeRule{
			rule("GET", "/api/v2/clients/{id}", "read:clients"),
			rule("GET", "/api/v2/clients/{other}", "read:clients"),
		}}, "duplicate rule"},
		{"a secret field with a dot", File{Rules: clients, SecretFields: []string{"options.secret"}}, "secretFields.names"},
		{"an empty secret field", File{Rules: clients, SecretFields: []string{""}}, "secretFields.names"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			index, err := test.file.Index()

			require.Error(t, err)
			assert.Nil(t, index)
			assert.ErrorIs(t, err, ErrInvalid)
			assert.Contains(t, err.Error(), test.want)
		})
	}

	t.Run("names the file it was read from", func(t *testing.T) {
		path := write(t, `{"rules":[]}`)

		file, err := Load(path)
		require.NoError(t, err)

		_, err = file.Index()

		require.ErrorIs(t, err, ErrInvalid)
		assert.Contains(t, err.Error(), path)
	})
}

func TestFile_IndexSecretFields(t *testing.T) {
	rules := []registry.ScopeRule{{Method: "GET", Path: "/api/v2/clients", AnyOf: []string{"read:clients"}}}

	withDefaults := func(extra ...string) []string {
		return slices.Concat(DefaultSecretFields, extra)
	}

	tests := []struct {
		name  string
		extra []string
		want  []string
	}{
		{"the defaults only", nil, withDefaults()},
		{"the defaults first, then the extra names", []string{"token_value", "api_key"}, withDefaults("token_value", "api_key")},
		{"a name that a default equals when case is ignored", []string{"Client_Secret", "SIGNING_KEYS"}, withDefaults()},
		{"an extra name given twice in another case", []string{"api_key", "API_KEY"}, withDefaults("api_key")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			index, err := (&File{Rules: rules, SecretFields: test.extra}).Index()

			require.NoError(t, err)
			assert.Equal(t, test.want, index.SecretFields.Names)
			assert.Equal(t, 1, index.SecretFields.FormatVersion)
		})
	}

	t.Run("the defaults are not changed by an index", func(t *testing.T) {
		before := slices.Clone(DefaultSecretFields)

		_, err := (&File{Rules: rules, SecretFields: []string{"api_key"}}).Index()
		require.NoError(t, err)

		assert.Equal(t, before, DefaultSecretFields)
	})
}

func TestFile_IndexOfAValidFile(t *testing.T) {
	file := &File{Rules: []registry.ScopeRule{
		{Method: "GET", Path: "/api/v2/clients", AnyOf: []string{"read:clients"}},
		{Method: "PATCH", Path: "/api/v2/clients/{id}", AnyOf: []string{"update:clients"}},
	}}

	index, err := file.Index()
	require.NoError(t, err)

	assert.Equal(t, registry.ScopeTable{FormatVersion: 1, Rules: file.Rules}, index.ScopeTable)
	assert.Equal(t, registry.Policy{}, index.Policy, "the policy is the zero one")
	assert.Empty(t, index.Plugins)
	require.NoError(t, index.ScopeTable.Validate())
	require.NoError(t, index.SecretFields.Validate())

	// The defaults of the registry apply to a zero policy.
	assert.Equal(t, 10*time.Minute, mustDuration(t, index.Policy.WithDefaults().DefaultTimeout))

	// The proxy accepts it, which is what a run needs.
	policy, err := proxy.NewPolicy(index)
	require.NoError(t, err)
	assert.NotNil(t, policy)
}

func mustDuration(t *testing.T, value string) time.Duration {
	t.Helper()

	duration, err := time.ParseDuration(value)
	require.NoError(t, err)

	return duration
}
