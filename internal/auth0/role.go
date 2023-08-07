//go:generate mockgen -source=role.go -destination=mock/roles_mock.go -package=mock

package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
)

type RoleAPI interface {
	// Create a new role.
	Create(ctx context.Context, r *management.Role, opts ...management.RequestOption) (err error)

	// Retrieve a role.
	Read(ctx context.Context, id string, opts ...management.RequestOption) (r *management.Role, err error)

	// List all roles that can be assigned to users or groups.
	List(ctx context.Context, opts ...management.RequestOption) (r *management.RoleList, err error)

	// Update a role.
	Update(ctx context.Context, id string, r *management.Role, opts ...management.RequestOption) (err error)

	// Delete a role.
	Delete(ctx context.Context, id string, opts ...management.RequestOption) (err error)

	// AssociatePermissions associates permissions to a role.
	//
	// See: https://auth0.com/docs/api/management/v2#!/Roles/post_role_permission_assignment
	AssociatePermissions(ctx context.Context, id string, permissions []*management.Permission, opts ...management.RequestOption) error

	// Permissions retrieves all permissions granted by a role.
	//
	// See: https://auth0.com/docs/api/management/v2#!/Roles/get_role_permission
	Permissions(ctx context.Context, id string, opts ...management.RequestOption) (p *management.PermissionList, err error)

	// RemovePermissions removes permissions associated to a role.
	//
	// See: https://auth0.com/docs/api/management/v2#!/Roles/delete_role_permission_assignment
	RemovePermissions(ctx context.Context, id string, permissions []*management.Permission, opts ...management.RequestOption) error
}
