package proxy

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStripSecrets(t *testing.T) {
	policy := testPolicy(t)

	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "a top level secret", body: `{"client_id":"a","client_secret":"s"}`, want: `{"client_id":"a"}`},
		{name: "the first member", body: `{"client_secret":"s","client_id":"a"}`, want: `{"client_id":"a"}`},
		{name: "the only member", body: `{"client_secret":"s"}`, want: `{}`},
		{name: "removed and not nulled", body: `{"a":1,"private_key":null}`, want: `{"a":1}`},
		{name: "a nested object", body: `{"a":{"b":{"client_secret":"s","c":1}}}`, want: `{"a":{"b":{"c":1}}}`},
		{name: "options.client_secret", body: `{"options":{"client_secret":"s","scope":"openid"}}`, want: `{"options":{"scope":"openid"}}`},
		{name: "an array of objects", body: `[{"client_secret":"a","id":1},{"id":2,"signing_keys":[{"cert":"c"}]}]`, want: `[{"id":1},{"id":2}]`},
		{name: "mixed case", body: `{"Client_Secret":"a","SIGNING_KEYS":[],"Encryption_Key":{}}`, want: `{}`},
		{name: "a name written with an escape", body: `{"client\u005fsecret":"a","ok":true}`, want: `{"ok":true}`},
		{name: "a secret with an object value", body: `{"a":1,"signing_keys":{"x":[1,{"y":[]}]},"b":2}`, want: `{"a":1,"b":2}`},
		{name: "a secret with an array value", body: `{"signing_keys":[[1],[{"a":"b"}]],"b":2}`, want: `{"b":2}`},
		{name: "the same secret twice", body: `{"client_secret":"a","x":1,"client_secret":"b"}`, want: `{"x":1}`},
		{name: "a secret as a string value is not a name", body: `{"a":"client_secret"}`, want: `{"a":"client_secret"}`},
		{name: "a secret as an item of an array is not a name", body: `["client_secret",{"client_secret":1}]`, want: `["client_secret",{}]`},
		{name: "a name that only contains a secret name", body: `{"client_secret_expires_at":0,"x_client_secret":1}`, want: `{"client_secret_expires_at":0,"x_client_secret":1}`},
		{name: "scalars and nulls", body: `{"a":true,"b":false,"c":null,"d":"x","e":[]}`, want: `{"a":true,"b":false,"c":null,"d":"x","e":[]}`},
		{name: "numbers as they were", body: `{"a":12345678901234567890,"b":1.10,"c":1e400,"d":-0.0}`, want: `{"a":12345678901234567890,"b":1.10,"c":1e400,"d":-0.0}`},
		{name: "whitespace", body: " {\n\t\"a\" : [ 1 , 2 ] ,\n \"client_secret\" : \"s\" }\n", want: `{"a":[1,2]}`},
		{name: "text with escapes", body: `{"a":"line\nbreak \"quoted\" é <b>"}`, want: "{\"a\":\"line\\nbreak \\\"quoted\\\" é \\u003cb\\u003e\"}"},
		{name: "a top level string", body: `"client_secret"`, want: `"client_secret"`},
		{name: "a top level number", body: `42`, want: `42`},
		{name: "a top level array", body: `[]`, want: `[]`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := stripSecrets([]byte(test.body), policy.isSecret)

			require.NoError(t, err)
			assert.Equal(t, test.want, string(got))
		})
	}
}

func TestStripSecrets_RejectsWhatItCannotRead(t *testing.T) {
	policy := testPolicy(t)

	tests := []struct {
		name string
		body string
	}{
		{name: "text", body: `not json`},
		{name: "a truncated object", body: `{"a":1`},
		{name: "a truncated secret value", body: `{"client_secret":{"a":[1,`},
		{name: "a missing colon", body: `{"a" 1}`},
		{name: "a trailing comma", body: `{"a":1,}`},
		{name: "a second value", body: `{"a":1}{"client_secret":"s"}`},
		{name: "a second value after whitespace", body: "{\"a\":1}\n[\"client_secret\"]"},
		{name: "data after a value", body: `{"a":1} x`},
		{name: "a closing bracket after a value", body: `{"a":1}}`},
		{name: "only whitespace", body: " \n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := stripSecrets([]byte(test.body), policy.isSecret)

			require.Error(t, err)
			assert.Nil(t, got)
		})
	}
}

func TestStripSecrets_DoesNotRecurse(t *testing.T) {
	policy := testPolicy(t)

	// A body can be as deep as encoding/json allows, and it is not read deeper.
	const depth = maxJSONDepth

	deep := func(levels int) string { return strings.Repeat("[", levels) + strings.Repeat("]", levels) }
	nested := func(levels int) string {
		return strings.Repeat(`{"a":`, levels-1) + `{"client_secret":"s"}` + strings.Repeat("}", levels-1)
	}
	skipped := func(levels int) string { return `{"client_secret":` + deep(levels-1) + `,"a":1}` }

	t.Run("the deepest body that is allowed", func(t *testing.T) {
		got, err := stripSecrets([]byte(deep(depth)), policy.isSecret)
		require.NoError(t, err)
		assert.Equal(t, deep(depth), string(got))

		got, err = stripSecrets([]byte(nested(depth)), policy.isSecret)
		require.NoError(t, err)
		assert.Equal(t, strings.Repeat(`{"a":`, depth-1)+`{}`+strings.Repeat("}", depth-1), string(got))

		got, err = stripSecrets([]byte(skipped(depth)), policy.isSecret)
		require.NoError(t, err)
		assert.Equal(t, `{"a":1}`, string(got))
	})

	t.Run("a body that is deeper", func(t *testing.T) {
		for _, levels := range []int{depth + 1, 100000} {
			for name, body := range map[string]string{"arrays": deep(levels), "objects": nested(levels), "a skipped secret": skipped(levels)} {
				got, err := stripSecrets([]byte(body), policy.isSecret)

				require.ErrorIs(t, err, errTooDeep, "%d levels of %s", levels, name)
				assert.Nil(t, got)
			}
		}
	})
}
