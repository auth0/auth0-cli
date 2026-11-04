package openapi

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompilePattern(t *testing.T) {
	tests := []struct {
		name      string
		pattern   string
		matches   []string
		rejects   []string
		wantRE2   bool
		wantError bool
	}{
		{
			name:    "it compiles an RE2-compatible pattern with RE2",
			pattern: `^[a-z]+$`,
			matches: []string{"abc"},
			rejects: []string{"ABC", ""},
			wantRE2: true,
		},
		{
			name:    "it rewrites ECMA-262 unicode escapes for RE2",
			pattern: `^\u0041+$`,
			matches: []string{"AAA"},
			rejects: []string{"BBB"},
			wantRE2: true,
		},
		{
			name:    "it falls back for a negative lookahead (trimmed, no NUL)",
			pattern: `^(?!.*\x00)\S(.*\S)?$`,
			matches: []string{"login-ui", "a b", "x"},
			rejects: []string{" leading", "trailing ", "", "nul\x00byte"},
		},
		{
			name:    "it falls back for a prefix-excluding lookahead",
			pattern: `^(?:(?!org_))[a-z0-9]([a-z0-9-_]*[a-z0-9])?$`,
			matches: []string{"acme", "acme-co"},
			rejects: []string{"org_acme", "Acme"},
		},
		{
			name:    "it falls back for chained lookaheads",
			pattern: `^(?!-)(?!.*--)(?:[a-zA-Z0-9-]|{org_id}|{org_name})*$`,
			matches: []string{"con-{org_name}", "abc"},
			rejects: []string{"-abc", "a--b"},
		},
		{
			name:      "it returns an error for a pattern neither engine compiles",
			pattern:   `([a-z`,
			wantError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			matcher, err := compilePattern(test.pattern)
			if test.wantError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			_, isRE2 := matcher.(*regexp.Regexp)
			assert.Equal(t, test.wantRE2, isRE2)

			for _, s := range test.matches {
				assert.True(t, matcher.MatchString(s), "expected %q to match", s)
			}
			for _, s := range test.rejects {
				assert.False(t, matcher.MatchString(s), "expected %q not to match", s)
			}
		})
	}
}

// TestValidateRequestSupportsLookaheadPatterns checks that a pattern RE2 can't
// compile is still enforced, instead of failing with "cannot compile pattern".
func TestValidateRequestSupportsLookaheadPatterns(t *testing.T) {
	const doc = `{
		"openapi": "3.0.0",
		"info": {"title": "fixture", "version": "1.0.0"},
		"paths": {"/segments": {"post": {
			"operationId": "post_segments",
			"requestBody": {"content": {"application/json": {"schema": {
				"type": "object",
				"properties": {"name": {"type": "string", "pattern": "^(?!.*\\x00)\\S(.*\\S)?$"}}
			}}}}
		}}}
	}`
	parsed, err := LoadDocFromData([]byte(doc))
	require.NoError(t, err)
	manager := NewSchemaManagerFromDoc(parsed)

	result, err := manager.ValidateRequest("POST", "/segments", []byte(`{"name":"us-users"}`))
	require.NoError(t, err)
	assert.Equal(t, StatusValid, result.Status, "a matching value passes: %v", result.Errors)

	result, err = manager.ValidateRequest("POST", "/segments", []byte(`{"name":" us-users"}`))
	require.NoError(t, err)
	assert.Equal(t, StatusInvalid, result.Status)
	require.NotEmpty(t, result.Errors)
	assert.Contains(t, result.Errors[0], "doesn't match the regular expression")
	assert.NotContains(t, result.Errors[0], "cannot compile pattern")
}
