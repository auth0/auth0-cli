package plugins

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// embeddedRegistryPublicKey is the base64-encoded Ed25519 public key that the
// registry index is signed with. It is compiled into the binary so the trust
// anchor cannot be swapped at runtime by a hostile environment.
//
// NOTE: while the registry lives in the private atko-cic repo this is a
// placeholder/test key. It is replaced with the production key before the
// registry migrates to the public auth0/auth0-cli-plugins repo.
//
//go:embed keys/registry_ed25519.pub
var embeddedRegistryPublicKey string

// ErrInvalidSignature is returned when the registry index signature does not
// verify against the trusted public key.
var ErrInvalidSignature = errors.New("registry index signature verification failed")

// ErrChecksumMismatch is returned when a downloaded artifact does not match the
// SHA-256 checksum published in the (signed) registry index.
var ErrChecksumMismatch = errors.New("artifact checksum mismatch")

// parsePublicKey decodes a base64-encoded Ed25519 public key.
func parsePublicKey(encoded string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %w", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("public key must be %d bytes, got %d", ed25519.PublicKeySize, len(raw))
	}
	return ed25519.PublicKey(raw), nil
}

// verifyIndexSignature checks the detached Ed25519 signature over the raw index
// bytes against the given public key. The signature is base64-encoded, as it is
// stored alongside the index (index.json.sig).
func verifyIndexSignature(index, encodedSignature []byte, pubKey ed25519.PublicKey) error {
	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encodedSignature)))
	if err != nil {
		return fmt.Errorf("%w: malformed signature encoding: %v", ErrInvalidSignature, err)
	}
	if len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("%w: signature must be %d bytes, got %d", ErrInvalidSignature, ed25519.SignatureSize, len(signature))
	}
	if !ed25519.Verify(pubKey, index, signature) {
		return ErrInvalidSignature
	}
	return nil
}

// verifyChecksum confirms that data hashes to the expected SHA-256, given as a
// hex string. The comparison is constant-time.
func verifyChecksum(data []byte, expectedHex string) error {
	expected, err := hex.DecodeString(strings.TrimSpace(expectedHex))
	if err != nil {
		return fmt.Errorf("%w: malformed expected checksum: %v", ErrChecksumMismatch, err)
	}
	sum := sha256.Sum256(data)
	if subtle.ConstantTimeCompare(sum[:], expected) != 1 {
		return fmt.Errorf("%w: got %s, want %s", ErrChecksumMismatch, hex.EncodeToString(sum[:]), expectedHex)
	}
	return nil
}
