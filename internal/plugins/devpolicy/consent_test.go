package devpolicy

import (
	"crypto/sha256"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins/registry"
)

func TestLoad_RecordsTheSumOfTheBytes(t *testing.T) {
	content := `{"rules":[` + oneRule + `]}`
	file, err := Load(write(t, content))
	require.NoError(t, err)

	assert.Equal(t, sha256.Sum256([]byte(content)), file.sum)

	other, err := Load(write(t, content+"\n"))
	require.NoError(t, err)

	assert.NotEqual(t, file.sum, other.sum, "a byte that changes nothing else still changes the sum")
}

func TestConsentHash(t *testing.T) {
	const manifestHash = "sha256:0000000000000000000000000000000000000000000000000000000000000001"

	content := `{"rules":[` + oneRule + `]}`

	load := func(t *testing.T, content string) *File {
		t.Helper()

		file, err := Load(write(t, content))
		require.NoError(t, err)

		return file
	}

	hashPattern := regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	base := ConsentHash(manifestHash, load(t, content))

	t.Run("is the manifest hash when there is no dev policy", func(t *testing.T) {
		assert.Equal(t, manifestHash, ConsentHash(manifestHash, nil))
	})

	t.Run("is a consent hash that covers more than the manifest hash", func(t *testing.T) {
		assert.Regexp(t, hashPattern, base)
		assert.NotEqual(t, manifestHash, base)
	})

	t.Run("is the same for the same bytes", func(t *testing.T) {
		assert.Equal(t, base, ConsentHash(manifestHash, load(t, content)))
		assert.Equal(t, base, ConsentHash(manifestHash, load(t, content)))
	})

	tests := []struct {
		name         string
		manifestHash string
		content      string
	}{
		{"a byte of the file changes", manifestHash, strings.Replace(content, "read:clients", "read:client2", 1)},
		{"a space is added to the file", manifestHash, content + " "},
		{"a rule is added", manifestHash, `{"rules":[` + oneRule + `,{"method":"GET","path":"/api/v2/users","anyOf":["read:users"]}]}`},
		{"the manifest hash changes", "sha256:0000000000000000000000000000000000000000000000000000000000000002", content},
	}

	for _, test := range tests {
		t.Run("differs when "+test.name, func(t *testing.T) {
			got := ConsentHash(test.manifestHash, load(t, test.content))

			assert.Regexp(t, hashPattern, got)
			assert.NotEqual(t, base, got)
		})
	}
}

func TestFile_CopyRules(t *testing.T) {
	file, err := Load(write(t, `{"rules":[`+oneRule+`]}`))
	require.NoError(t, err)

	rules := file.CopyRules()
	assert.Equal(t, file.Rules, rules)

	rules[0].Path = "/api/v2/changed"
	rules[0].AnyOf[0] = "changed"

	assert.Equal(t, "/api/v2/clients", file.Rules[0].Path)
	assert.Equal(t, []string{"read:clients"}, file.Rules[0].AnyOf, "the copy shares nothing with the file")
}

func TestFile_BroadRules(t *testing.T) {
	tests := []struct {
		path  string
		broad bool
	}{
		{"/api/v2/{a}", true},
		{"/api/v2/{a}/{b}", true},
		{"/api/v2/{id}/{other}/{third}", true},
		{"/api/v2/clients", false},
		{"/api/v2/clients/{id}", false},
		{"/api/v2/{a}/keys", false},
		{"/api/v2/{a}/{b}/keys", false},
		{"/api/v2/", false},
		{"/other/{a}", false},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			file := &File{Rules: []registry.ScopeRule{{Method: "DELETE", Path: test.path, AnyOf: []string{"delete:clients"}}}}

			if test.broad {
				assert.Equal(t, file.Rules, file.BroadRules())
			} else {
				assert.Empty(t, file.BroadRules())
			}
		})
	}

	t.Run("lists only the broad rules, in order", func(t *testing.T) {
		file := &File{Rules: []registry.ScopeRule{
			{Method: "GET", Path: "/api/v2/clients", AnyOf: []string{"read:clients"}},
			{Method: "PATCH", Path: "/api/v2/{a}/{b}", AnyOf: []string{"update:clients"}},
			{Method: "GET", Path: "/api/v2/{a}", AnyOf: []string{"read:clients"}},
		}}

		assert.Equal(t, file.Rules[1:], file.BroadRules())
	})
}
