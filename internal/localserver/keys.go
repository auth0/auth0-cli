package localserver

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
)

type keySet struct {
	privateKey *rsa.PrivateKey
	jwksJSON   []byte
}

func newKeySet() (*keySet, error) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generating RSA key: %w", err)
	}

	jwkPriv, err := jwk.FromRaw(rsaKey)
	if err != nil {
		return nil, fmt.Errorf("building JWK from raw key: %w", err)
	}
	if err := jwkPriv.Set(jwk.KeyIDKey, "local-key-1"); err != nil {
		return nil, fmt.Errorf("setting key ID: %w", err)
	}
	if err := jwkPriv.Set(jwk.AlgorithmKey, jwa.RS256); err != nil {
		return nil, fmt.Errorf("setting algorithm: %w", err)
	}

	pubKey, err := jwk.PublicKeyOf(jwkPriv)
	if err != nil {
		return nil, fmt.Errorf("deriving public JWK: %w", err)
	}

	set := jwk.NewSet()
	if err := set.AddKey(pubKey); err != nil {
		return nil, fmt.Errorf("adding key to JWKS: %w", err)
	}

	jwksBytes, err := json.Marshal(set)
	if err != nil {
		return nil, fmt.Errorf("marshaling JWKS: %w", err)
	}

	return &keySet{
		privateKey: rsaKey,
		jwksJSON:   jwksBytes,
	}, nil
}
