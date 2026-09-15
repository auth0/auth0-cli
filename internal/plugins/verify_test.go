package plugins

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyChecksum(t *testing.T) {
	data := []byte("the quick brown fox")
	sum := sha256.Sum256(data)

	require.NoError(t, verifyChecksum(data, hex.EncodeToString(sum[:])))

	err := verifyChecksum(data, hex.EncodeToString(make([]byte, sha256.Size)))
	assert.ErrorIs(t, err, ErrChecksumMismatch)

	err = verifyChecksum(data, "not-hex")
	assert.ErrorIs(t, err, ErrChecksumMismatch)
}

func TestParsePublicKeyRejectsBadInput(t *testing.T) {
	_, err := parsePublicKey("not base64!!")
	assert.Error(t, err)

	_, err = parsePublicKey("aGVsbG8=") // Valid base64, wrong length.
	assert.Error(t, err)
}

func TestEmbeddedPublicKeyIsValid(t *testing.T) {
	_, err := parsePublicKey(embeddedRegistryPublicKey)
	assert.NoError(t, err)
}
