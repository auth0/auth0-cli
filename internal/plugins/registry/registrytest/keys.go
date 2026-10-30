// Package registrytest builds, signs and serves plugin registry indexes for
// tests of the registry client and of the code that uses it.
package registrytest

import (
	"crypto/ed25519"
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/auth0/auth0-cli/internal/plugins/registry"
)

// TestKeyID is the ID of the test key that a build made with the plugintest tag
// trusts.
const TestKeyID = "test-1"

// testPrivateKey holds the private half of the test key.
//
// This is a public, throwaway key. It is committed on purpose and is used only
// by tests and by local end to end runs against a build made with the plugintest
// tag. A regular build does not trust the public half, so signing with this key
// proves nothing to a released CLI.
//
//go:embed testdata/test-1.key
var testPrivateKey string

// TestPrivateKey returns the private half of the test key, whose public half a
// build made with the plugintest tag trusts under the ID TestKeyID.
func TestPrivateKey() ed25519.PrivateKey {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(testPrivateKey))
	if err != nil || len(key) != ed25519.PrivateKeySize {
		panic("the registry test key is malformed")
	}

	return key
}

// NewKeyPair generates a new key for a test. It returns the key ID, a key set
// that trusts the public half, and the private half.
func NewKeyPair(t testing.TB) (string, registry.KeySet, ed25519.PrivateKey) {
	t.Helper()

	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate a key: %v", err)
	}

	id := fmt.Sprintf("key-%x", public[:4])

	return id, registry.KeySet{id: public}, private
}

// Sign returns the content of the index.json.sig file for indexJSON, signed
// with the key of the given ID.
func Sign(indexJSON []byte, keyID string, private ed25519.PrivateKey) []byte {
	return mustValue(json.Marshal(map[string]string{
		"keyId":     keyID,
		"algorithm": "ed25519",
		"signature": base64.StdEncoding.EncodeToString(ed25519.Sign(private, indexJSON)),
	}))
}
