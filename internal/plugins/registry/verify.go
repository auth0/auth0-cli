package registry

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"

	"github.com/auth0/auth0-cli/internal/plugins"
)

// signatureAlgorithm is the only signature algorithm the index can use.
const signatureAlgorithm = "ed25519"

// KeySet is a set of trusted Ed25519 public keys by key ID. The key ID lets the
// signer of an index pick which key signed it, which makes it possible to rotate
// keys without a new CLI release once both keys are embedded.
type KeySet map[string]ed25519.PublicKey

// signature is the content of an index.json.sig file.
type signature struct {
	KeyID     string `json:"keyId"`
	Algorithm string `json:"algorithm"`
	Signature string `json:"signature"`
}

// Verify checks that signatureDoc, the content of an index.json.sig file, holds
// a valid signature of data by one of the keys. Data is the index exactly as it
// was downloaded, because the signature covers its raw bytes.
//
// It returns ErrNoTrustedKeys when the set is empty. In every other case that
// the signature does not verify, such as an unknown key ID, an unknown
// algorithm, a signature of the wrong length or a signature that does not
// match, it returns an error that wraps ErrSignatureInvalid.
func (k KeySet) Verify(data, signatureDoc []byte) error {
	if len(k) == 0 {
		return ErrNoTrustedKeys
	}

	var doc signature
	if err := plugins.UnmarshalStrict(signatureDoc, &doc); err != nil {
		return invalid("the signature file is not valid: %v", err)
	}

	if doc.Algorithm != signatureAlgorithm {
		return invalid("unknown signature algorithm %q", doc.Algorithm)
	}

	key, ok := k[doc.KeyID]
	if !ok {
		return invalid("unknown signing key %q", doc.KeyID)
	}

	if len(key) != ed25519.PublicKeySize {
		return invalid("the trusted key %q is malformed", doc.KeyID)
	}

	raw, err := base64.StdEncoding.DecodeString(doc.Signature)
	if err != nil {
		return invalid("the signature is not valid base64: %v", err)
	}

	if len(raw) != ed25519.SignatureSize {
		return invalid("the signature is %d bytes and should be %d", len(raw), ed25519.SignatureSize)
	}

	if !ed25519.Verify(key, data, raw) {
		return invalid("the signature does not match key %q", doc.KeyID)
	}

	return nil
}

// invalid returns an error that wraps ErrSignatureInvalid and says why.
func invalid(format string, args ...any) error {
	return fmt.Errorf("%w (%s)", ErrSignatureInvalid, fmt.Sprintf(format, args...))
}
