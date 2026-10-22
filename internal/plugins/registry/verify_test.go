package registry_test

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins/registry"
	"github.com/auth0/auth0-cli/internal/plugins/registry/registrytest"
)

func TestKeySet_Verify(t *testing.T) {
	keyID, keys, private := registrytest.NewKeyPair(t)
	otherID, _, otherPrivate := registrytest.NewKeyPair(t)
	data := []byte(`{"formatVersion":1}`)

	valid := registrytest.Sign(data, keyID, private)

	tests := []struct {
		name      string
		keys      registry.KeySet
		data      []byte
		signature []byte
		wantErr   error
		wantText  string
	}{
		{name: "a valid signature", keys: keys, data: data, signature: valid},
		{name: "extra fields in the signature file", keys: keys, data: data, signature: append([]byte(`{"extra":1,`), valid[1:]...)},
		{name: "changed data", keys: keys, data: []byte(`{"formatVersion":2}`), signature: valid, wantErr: registry.ErrSignatureInvalid, wantText: "does not match"},
		{name: "a different key", keys: keys, data: data, signature: registrytest.Sign(data, keyID, otherPrivate), wantErr: registry.ErrSignatureInvalid, wantText: "does not match"},
		{name: "an unknown key ID", keys: keys, data: data, signature: registrytest.Sign(data, otherID, otherPrivate), wantErr: registry.ErrSignatureInvalid, wantText: "unknown signing key"},
		{name: "an unknown algorithm", keys: keys, data: data, signature: signatureDocument(t, keyID, "ed448", base64.StdEncoding.EncodeToString(make([]byte, 64))), wantErr: registry.ErrSignatureInvalid, wantText: "unknown signature algorithm"},
		{name: "no algorithm", keys: keys, data: data, signature: []byte(`{"keyId":"` + keyID + `"}`), wantErr: registry.ErrSignatureInvalid, wantText: "unknown signature algorithm"},
		{name: "a signature that is too short", keys: keys, data: data, signature: signatureDocument(t, keyID, "ed25519", base64.StdEncoding.EncodeToString(make([]byte, 10))), wantErr: registry.ErrSignatureInvalid, wantText: "10 bytes"},
		{name: "an empty signature", keys: keys, data: data, signature: signatureDocument(t, keyID, "ed25519", ""), wantErr: registry.ErrSignatureInvalid, wantText: "0 bytes"},
		{name: "a key of the wrong size", keys: registry.KeySet{keyID: ed25519.PublicKey("short")}, data: data, signature: valid, wantErr: registry.ErrSignatureInvalid, wantText: "malformed"},
		{name: "no keys", keys: registry.KeySet{}, data: data, signature: valid, wantErr: registry.ErrNoTrustedKeys},
		{name: "a nil key set", keys: nil, data: data, signature: valid, wantErr: registry.ErrNoTrustedKeys},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.keys.Verify(test.data, test.signature)

			if test.wantErr == nil {
				require.NoError(t, err)
				return
			}

			require.ErrorIs(t, err, test.wantErr)
			assert.Contains(t, err.Error(), test.wantText)
		})
	}
}
