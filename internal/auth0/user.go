//go:generate mockgen -source=user.go -destination=mock/user_mock.go -package=mock

package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
)

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
