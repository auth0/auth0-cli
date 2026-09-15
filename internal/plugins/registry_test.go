package plugins

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testKeyPair loads the fixture Ed25519 private key from testdata and returns
// it alongside its public key.
func testKeyPair(t *testing.T) (ed25519.PrivateKey, ed25519.PublicKey) {
	t.Helper()

	encoded, err := os.ReadFile("testdata/registry_ed25519.key")
	require.NoError(t, err)

	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	require.NoError(t, err)
	require.Len(t, raw, ed25519.PrivateKeySize)

	priv := ed25519.PrivateKey(raw)
	return priv, priv.Public().(ed25519.PublicKey)
}

// signedRegistryServer serves a fixture index signed by priv, plus the detached
// base64 signature. When tamper is true the served index no longer matches the
// signature.
func signedRegistryServer(t *testing.T, priv ed25519.PrivateKey, index []byte, tamper bool) *httptest.Server {
	t.Helper()

	signature := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, index))
	served := index
	if tamper {
		served = append([]byte(nil), index...)
		served[len(served)-1] ^= 0xFF
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/"+indexFileName, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(served)
	})
	mux.HandleFunc("/"+signatureFileName, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(signature))
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

const fixtureIndex = `{
  "version": 1,
  "plugins": [
    {
      "name": "checkmate",
      "description": "Audit an Auth0 tenant for security best practices",
      "install": {
        "type": "npm",
        "package": "checkmate",
        "version": "1.2.3"
      },
      "required_scopes": ["read:clients", "read:connections"]
    },
    {
      "name": "widget",
      "description": "Example binary plugin",
      "install": {
        "type": "github-release",
        "repo": "auth0/widget",
        "version": "v0.1.0",
        "assets": [
          {"os": "darwin", "arch": "arm64", "url": "https://example.com/widget-darwin-arm64", "sha256": "abc123"}
        ]
      }
    }
  ]
}`

func TestRegistryFetch(t *testing.T) {
	priv, pub := testKeyPair(t)
	server := signedRegistryServer(t, priv, []byte(fixtureIndex), false)

	registry := NewRegistryWith(server.URL, pub)
	index, err := registry.Fetch(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, index.Version)
	require.Len(t, index.Plugins, 2)

	checkmate, ok := index.Plugin("checkmate")
	require.True(t, ok)
	assert.Equal(t, InstallNPM, checkmate.Install.Type)
	assert.Equal(t, "checkmate", checkmate.Install.Package)
	assert.Equal(t, "1.2.3", checkmate.Install.Version)
	assert.Equal(t, []string{"read:clients", "read:connections"}, checkmate.RequiredScopes)

	widget, ok := index.Plugin("widget")
	require.True(t, ok)
	assert.Equal(t, InstallGitHubRelease, widget.Install.Type)
	require.Len(t, widget.Install.Assets, 1)
	assert.Equal(t, "darwin", widget.Install.Assets[0].OS)

	_, ok = index.Plugin("does-not-exist")
	assert.False(t, ok)
}

func TestRegistryFetchRejectsTamperedIndex(t *testing.T) {
	priv, pub := testKeyPair(t)
	server := signedRegistryServer(t, priv, []byte(fixtureIndex), true)

	registry := NewRegistryWith(server.URL, pub)
	_, err := registry.Fetch(context.Background())
	assert.ErrorIs(t, err, ErrInvalidSignature)
}

func TestRegistryFetchRejectsWrongKey(t *testing.T) {
	priv, _ := testKeyPair(t)
	server := signedRegistryServer(t, priv, []byte(fixtureIndex), false)

	// Verify against a different key than the one that signed the index.
	otherPub, _, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	registry := NewRegistryWith(server.URL, otherPub)
	_, err = registry.Fetch(context.Background())
	assert.ErrorIs(t, err, ErrInvalidSignature)
}

func TestRegistryFetchServerError(t *testing.T) {
	priv, pub := testKeyPair(t)
	_ = priv
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	registry := NewRegistryWith(server.URL, pub)
	_, err := registry.Fetch(context.Background())
	assert.Error(t, err)
}
