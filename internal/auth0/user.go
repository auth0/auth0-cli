//go:generate mockgen -source=user.go -destination=mock/user_mock.go -package=mock

package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"
)

// UserSessionPage is the paginated response returned by the user sessions list
// endpoint. It is aliased here so the mock generator (which cannot parse
// instantiated generic types) sees a plain named type in the interface.
type UserSessionPage = core.Page[*string, *managementv3.SessionResponseContent, *managementv3.ListUserSessionsPaginatedResponseContent]

// UserRefreshTokenPage is the paginated response returned by the user refresh
// tokens list endpoint. It is aliased here so the mock generator (which cannot
// parse instantiated generic types) sees a plain named type in the interface.
type UserRefreshTokenPage = core.Page[*string, *managementv3.RefreshTokenResponseContent, *managementv3.ListRefreshTokensPaginatedResponseContent]

type UserAPI interface {
	// Blocks retrieves a list of blocked IP addresses of a particular user.
	Blocks(ctx context.Context, id string, opts ...management.RequestOption) ([]*management.UserBlock, error)

	// BlocksByIdentifier retrieves a list of blocked IP addresses of a particular user using any of the user identifiers: username, phone number or email.
	BlocksByIdentifier(ctx context.Context, identifier string, opts ...management.RequestOption) ([]*management.UserBlock, error)

	// Unblock a user that was blocked due to an excessive amount of incorrectly
	// provided credentials.
	Unblock(ctx context.Context, id string, opts ...management.RequestOption) error

	// UnblockByIdentifier a user that was blocked due to an excessive amount of incorrectly provided credentials using any of the user identifiers: username, phone number or email.
	UnblockByIdentifier(ctx context.Context, identifier string, opts ...management.RequestOption) error

	// Create a new user.
	Create(ctx context.Context, u *management.User, opts ...management.RequestOption) (err error)

	// Read user details for a given user.
	Read(ctx context.Context, id string, opts ...management.RequestOption) (u *management.User, err error)

	// Update user.
	Update(ctx context.Context, id string, u *management.User, opts ...management.RequestOption) (err error)

	// Delete a user.
	Delete(ctx context.Context, id string, opts ...management.RequestOption) (err error)

	// List all users.
	List(ctx context.Context, opts ...management.RequestOption) (ul *management.UserList, err error)

	// Search for users.
	Search(ctx context.Context, opts ...management.RequestOption) (us *management.UserList, err error)

	// Roles lists all roles associated with a user.
	Roles(ctx context.Context, id string, opts ...management.RequestOption) (r *management.RoleList, err error)

	// AssignRoles assigns roles to a user.
	AssignRoles(ctx context.Context, id string, roles []*management.Role, opts ...management.RequestOption) error

	// RemoveRoles removes roles from a user.
	RemoveRoles(ctx context.Context, id string, roles []*management.Role, opts ...management.RequestOption) error

	// ListByEmail lists all users by email in all the connections.
	ListByEmail(ctx context.Context, email string, opts ...management.RequestOption) (us []*management.User, err error)
}

// UserSessionAPIV3 is the V3 SDK interface for user-scoped sessions
// (/users/{id}/sessions).
type UserSessionAPIV3 interface {
	// List a user's sessions (cursor-paginated).
	//
	// Required scope: `read:sessions`.
	List(ctx context.Context, userID string, request *managementv3.ListUserSessionsRequestParameters, opts ...option.RequestOption) (*UserSessionPage, error)

	// Delete all sessions for a user.
	//
	// Required scope: `delete:sessions`.
	Delete(ctx context.Context, userID string, opts ...option.RequestOption) error
}

// UserRefreshTokenAPIV3 is the V3 SDK interface for user-scoped refresh tokens
// (/users/{id}/refresh-tokens).
type UserRefreshTokenAPIV3 interface {
	// List a user's refresh tokens (cursor-paginated).
	//
	// Required scope: `read:refresh_tokens`.
	List(ctx context.Context, userID string, request *managementv3.ListRefreshTokensRequestParameters, opts ...option.RequestOption) (*UserRefreshTokenPage, error)

	// Delete all refresh tokens for a user.
	//
	// Required scope: `delete:refresh_tokens`.
	Delete(ctx context.Context, userID string, opts ...option.RequestOption) error
}
