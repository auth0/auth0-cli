package proxy

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/pem"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCertificate(t *testing.T) {
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)

	cert, certPEM, err := newCertificate(now)
	require.NoError(t, err)

	block, rest := pem.Decode(certPEM)
	require.NotNil(t, block)
	assert.Empty(t, rest)
	assert.Equal(t, "CERTIFICATE", block.Type)

	parsed, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	assert.Equal(t, cert.Certificate[0], block.Bytes)

	key, ok := parsed.PublicKey.(*ecdsa.PublicKey)
	require.True(t, ok, "the key is an ECDSA key")
	assert.Equal(t, elliptic.P256(), key.Curve)

	assert.True(t, parsed.IsCA)
	assert.Equal(t, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, parsed.ExtKeyUsage)
	assert.Empty(t, parsed.DNSNames)
	require.Len(t, parsed.IPAddresses, 1)
	assert.True(t, parsed.IPAddresses[0].Equal(net.IPv4(127, 0, 0, 1)))
	assert.Equal(t, now.Add(-5*time.Minute), parsed.NotBefore)
	assert.Equal(t, now.Add(24*time.Hour), parsed.NotAfter)

	pool := x509.NewCertPool()
	pool.AddCert(parsed)

	verify := func(host string, at time.Time) error {
		_, err := parsed.Verify(x509.VerifyOptions{DNSName: host, Roots: pool, CurrentTime: at})

		return err
	}

	assert.NoError(t, verify("127.0.0.1", now))
	assert.Error(t, verify("localhost", now), "only the loopback address is covered")
	assert.Error(t, verify("example.com", now))
	assert.Error(t, verify("127.0.0.1", now.Add(25*time.Hour)), "it expires")
	assert.Error(t, verify("127.0.0.1", now.Add(-6*time.Minute)), "it is not valid before the allowed clock skew")
}
