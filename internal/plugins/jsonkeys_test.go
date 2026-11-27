package plugins

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRejectDuplicateKeys(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr string
	}{
		{name: "an object", json: `{"a":1,"b":{"a":2},"c":[{"a":3},{"a":4}]}`},
		{name: "nothing nested", json: `{}`},
		{name: "a scalar", json: `"a"`},
		{name: "an array of scalars", json: `[1,"a",null,true,[]]`},
		{name: "keys that only differ in case", json: `{"a":1,"A":2}`},
		{name: "the same key in different objects", json: `[{"a":1},{"a":2}]`},
		{name: "the same key after a nested object", json: `{"a":{"a":{"a":1}},"b":1}`},
		{name: "a duplicate at the root", json: `{"a":1,"a":2}`, wantErr: `$: duplicate key "a"`},
		{name: "a duplicate that follows other keys", json: `{"a":1,"b":2,"c":3,"b":4}`, wantErr: `$: duplicate key "b"`},
		{name: "a duplicate in a nested object", json: `{"x":{"y":{"a":1,"a":2}}}`, wantErr: `$.x.y: duplicate key "a"`},
		{name: "a duplicate in an array item", json: `{"plugins":[{"a":1},{"b":1,"b":2}]}`, wantErr: `$.plugins[1]: duplicate key "b"`},
		{name: "a duplicate in a deep path", json: `{"plugins":[{"versions":[{}, {"permissions":{},"permissions":{}}]}]}`, wantErr: `$.plugins[0].versions[1]: duplicate key "permissions"`},
		{name: "a duplicate inside an array of arrays", json: `[[1],[{"a":1,"a":1}]]`, wantErr: `$[1][0]: duplicate key "a"`},
		{name: "a duplicate written with an escape", json: `{"a":1,"\u0061":2}`, wantErr: `$: duplicate key "a"`},
		{name: "a key that cannot be written after a dot", json: `{"a b":{"c":1,"c":2}}`, wantErr: `$["a b"]: duplicate key "c"`},
		{name: "an empty key twice", json: `{"":1,"":2}`, wantErr: `$: duplicate key ""`},
		{name: "a duplicate after a closed array", json: `{"a":[1,2],"b":1,"a":3}`, wantErr: `$: duplicate key "a"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := RejectDuplicateKeys([]byte(test.json))

			if test.wantErr == "" {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.EqualError(t, err, test.wantErr)
		})
	}
}

func TestRejectDuplicateKeys_MalformedInput(t *testing.T) {
	tests := []string{
		``, `{`, `[`, `{"a"`, `{"a":`, `{"a":1,`, `{"a" 1}`, `}`, `]`, `{"a":1}}`, `nul`, `{"a":tru}`, `{1:2}`,
		`["a",]`, `{"a":1,,}`, "\x00", `"unterminated`,
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			assert.NotPanics(t, func() { _ = RejectDuplicateKeys([]byte(input)) })
		})
	}

	t.Run("a very deeply nested document does not overflow the stack", func(t *testing.T) {
		depth := 2_000_000
		data := strings.Repeat("[", depth) + strings.Repeat("]", depth)

		assert.NotPanics(t, func() { assert.NoError(t, RejectDuplicateKeys([]byte(data))) })
	})
}

// duplicateKeyCases are manifests with a key twice, mostly made by adding a key
// to the start of an object of the valid manifest.
func duplicateKeyCases(t *testing.T) []struct{ name, manifest, want string } {
	t.Helper()

	valid := string(readValidManifest(t))
	inject := func(after, key string) string {
		require.Contains(t, valid, after)

		return strings.Replace(valid, after, after+key, 1)
	}

	return []struct{ name, manifest, want string }{
		{name: "at the root", manifest: inject("{\n", `"id": "auth0/other",`), want: `$: duplicate key "id"`},
		{name: "in a nested object", manifest: inject(`"owner": {`, `"team": "Other",`), want: `$.owner: duplicate key "team"`},
		{name: "in an array item", manifest: inject(`"flags": [`, `{"name": "x", "name": "y", "type": "string", "description": "d"},`), want: `$.commands[0].flags[0]: duplicate key "name"`},
		{name: "in permissions", manifest: inject(`"permissions": {`, `"scopes": [],`), want: `$.commands[0].permissions: duplicate key "scopes"`},
		{name: "in a permission entry", manifest: inject(`"hosts": [`, `{"host": "a.example.com", "host": "b.example.com", "reason": "r"},`), want: `$.commands[0].permissions.hosts[0]: duplicate key "host"`},
		{name: "in a manifest that is not an object", manifest: `[{"a":1,"a":2}]`, want: `$[0]: duplicate key "a"`},
	}
}

func TestParseManifest_DuplicateKeys(t *testing.T) {
	parsers := map[string]func([]byte) (*Manifest, error){
		"ParseManifest":          ParseManifest,
		"ParseInstalledManifest": ParseInstalledManifest,
	}

	for parserName, parse := range parsers {
		for _, test := range duplicateKeyCases(t) {
			t.Run(parserName+" "+test.name, func(t *testing.T) {
				manifest, err := parse([]byte(test.manifest))

				assert.Nil(t, manifest)
				requireValidationError(t, err, test.want)
			})
		}
	}
}

func TestUnmarshalStrict(t *testing.T) {
	type document struct {
		Name string `json:"name"`
	}

	tests := []struct {
		name    string
		data    string
		want    document
		wantErr string
	}{
		{name: "a valid document", data: `{"name":"a"}`, want: document{Name: "a"}},
		{name: "a repeated key", data: `{"name":"a","name":"b"}`, wantErr: `$: duplicate key "name"`},
		{name: "a document that is cut short", data: `{"name":`, wantErr: "not valid JSON"},
		{name: "a value of the wrong type", data: `{"name":1}`, wantErr: "cannot unmarshal number"},
		{name: "text after the document", data: `{"name":"a"} x`, wantErr: "invalid character"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got document

			err := UnmarshalStrict([]byte(test.data), &got)

			if test.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}
