package localserver

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

const (
	localUserSub   = "local|test-user"
	localUserEmail = "test@local.dev"
	localUserName  = "Local Test User"
	tokenTTL       = 24 * time.Hour
)

func (s *Server) issueIDToken(req authRequest, now, exp time.Time) (string, error) {
	builder := jwt.NewBuilder().
		Issuer(s.issuer+"/").
		Subject(localUserSub).
		Audience([]string{req.clientID}).
		IssuedAt(now).
		Expiration(exp).
		Claim("email", localUserEmail).
		Claim("name", localUserName).
		Claim("email_verified", true).
		// Local_only marks tokens as intentionally non-production.
		// They will fail validation against any real Auth0 tenant's JWKS.
		Claim("local_only", true)

	if req.nonce != "" {
		builder = builder.Claim("nonce", req.nonce)
	}

	tok, err := builder.Build()
	if err != nil {
		return "", fmt.Errorf("building id_token: %w", err)
	}

	signed, err := jwt.Sign(tok, jwt.WithKey(jwa.RS256, s.keys.privateKey))
	if err != nil {
		return "", fmt.Errorf("signing id_token: %w", err)
	}

	return string(signed), nil
}

func (s *Server) issueAccessToken(req authRequest, now, exp time.Time) (string, error) {
	tok, err := jwt.NewBuilder().
		Issuer(s.issuer+"/").
		Subject(localUserSub).
		Audience([]string{req.clientID}).
		IssuedAt(now).
		Expiration(exp).
		Claim("scope", req.scope).
		Claim("local_only", true).
		Build()
	if err != nil {
		return "", fmt.Errorf("building access_token: %w", err)
	}

	signed, err := jwt.Sign(tok, jwt.WithKey(jwa.RS256, s.keys.privateKey))
	if err != nil {
		return "", fmt.Errorf("signing access_token: %w", err)
	}

	return string(signed), nil
}

// verifyPKCE checks that the given verifier satisfies the stored challenge.
func verifyPKCE(method, challenge, verifier string) bool {
	switch method {
	case "S256":
		h := sha256.Sum256([]byte(verifier))
		return base64.RawURLEncoding.EncodeToString(h[:]) == challenge
	case "plain", "":
		return verifier == challenge
	default:
		return false
	}
}
