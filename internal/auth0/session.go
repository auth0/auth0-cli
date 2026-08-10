//go:generate mockgen -source=session.go -destination=mock/session_mock.go -package=mock

package auth0

import (
	"context"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/option"
)

// SessionAPIV3 is the V3 SDK interface for the top-level /sessions endpoint.
// Sessions are keyed by session ID; user-scoped listing lives in UserSessionAPIV3.
type SessionAPIV3 interface {
	// Get retrieves session information by session ID.
	//
	// Required scope: `read:sessions`.
	Get(ctx context.Context, id string, opts ...option.RequestOption) (*managementv3.GetSessionResponseContent, error)

	// Update session metadata by session ID.
	//
	// Required scope: `update:sessions`.
	Update(ctx context.Context, id string, request *managementv3.UpdateSessionRequestContent, opts ...option.RequestOption) (*managementv3.UpdateSessionResponseContent, error)

	// Delete a session by ID.
	//
	// Required scope: `delete:sessions`.
	Delete(ctx context.Context, id string, opts ...option.RequestOption) error

	// Revoke a session by ID and all associated refresh tokens.
	//
	// Required scope: `delete:sessions`.
	Revoke(ctx context.Context, id string, opts ...option.RequestOption) error
}
