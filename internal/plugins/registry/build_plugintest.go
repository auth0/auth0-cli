//go:build plugintest

package registry

import (
	"crypto/ed25519"
	"encoding/base64"
)

// allowLoopbackHTTP lets a build made with the plugintest tag fetch a registry
// or a plugin file over plain http from the loopback address, so that a
// registry can be served from the same machine without a certificate.
const allowLoopbackHTTP = true

// testKeyID is the ID of the throwaway key that a build made with the plugintest
// tag trusts. The private half is public on purpose, see
// registrytest.TestPrivateKey, so a regular build never trusts it.
const testKeyID = "test-1"

// testPublicKey is the base64 of the public half of the test key.
const testPublicKey = "ME2ftmtFqfkzeM4eRtCuvCyp7rjm/FBvTrAKXyF2s90="

// testKeys returns the test key.
func testKeys() KeySet {
	key, err := base64.StdEncoding.DecodeString(testPublicKey)
	if err != nil || len(key) != ed25519.PublicKeySize {
		panic("the embedded registry test key is malformed")
	}

	return KeySet{testKeyID: key}
}
