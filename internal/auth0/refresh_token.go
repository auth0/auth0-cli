//go:generate mockgen -source=refresh_token.go -destination=mock/refresh_token_mock.go -package=mock

package auth0

import (
	"context"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/option"
)

// RefreshTokenAPIV3 is the V3 SDK interface for the top-level /refresh-tokens
// endpoint. Refresh tokens are keyed by token ID; user-scoped listing lives in
// UserRefreshTokenAPIV3.
type RefreshTokenAPIV3 interface {
	// Get retrieves refresh token information by token ID.
	//
	// Required scope: `read:refresh_tokens`.
	Get(ctx context.Context, id string, opts ...option.RequestOption) (*managementv3.GetRefreshTokenResponseContent, error)

	// Update refresh token metadata by token ID.
	//
	// Required scope: `update:refresh_tokens`.
	Update(ctx context.Context, id string, request *managementv3.UpdateRefreshTokenRequestContent, opts ...option.RequestOption) (*managementv3.UpdateRefreshTokenResponseContent, error)

	// Delete a refresh token by its ID.
	//
	// Required scope: `delete:refresh_tokens`.
	Delete(ctx context.Context, id string, opts ...option.RequestOption) error

	// Revoke refresh tokens in bulk by ID list, user, user+client, or
	// user+client+audience.
	//
	// Required scope: `delete:refresh_tokens`.
	Revoke(ctx context.Context, request *managementv3.RevokeRefreshTokensRequestContent, opts ...option.RequestOption) error
}
