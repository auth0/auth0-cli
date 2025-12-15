//go:generate mockgen -source=token_exchange.go -destination=mock/token_exchange_mock.go -package=mock

package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
)

// TokenExchangeAPI is an interface that describes all the Token Exchange Profile related operations.
type TokenExchangeAPI interface {
	// List retrieves all token exchange profiles.
	List(ctx context.Context, opts ...management.RequestOption) (*management.TokenExchangeProfileList, error)

	// Read retrieves a token exchange profile by its ID.
	Read(ctx context.Context, id string, opts ...management.RequestOption) (*management.TokenExchangeProfile, error)

	// Create creates a new token exchange profile.
	Create(ctx context.Context, profile *management.TokenExchangeProfile, opts ...management.RequestOption) error

	// Update updates an existing token exchange profile.
	Update(ctx context.Context, id string, profile *management.TokenExchangeProfile, opts ...management.RequestOption) error

	// Delete deletes a token exchange profile.
	Delete(ctx context.Context, id string, opts ...management.RequestOption) error
}
