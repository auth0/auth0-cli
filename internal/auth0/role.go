package auth0

import "github.com/auth0/go-auth0/management"

type RoleAPI interface {
	// Create a new role.
	Create(r *management.Role, opts ...management.RequestOption) (err error)

	// Retrieve a role.
	Read(id string, opts ...management.RequestOption) (r *management.Role, err error)

	// List all roles that can be assigned to users or groups.
	List(opts ...management.RequestOption) (r *management.RoleList, err error)

	// Update a role.
	Update(id string, r *management.Role, opts ...management.RequestOption) (err error)

	// Delete a role.
	Delete(id string, opts ...management.RequestOption) (err error)

	// AssociatePermissions associates permissions to a role.
	//
	// See: https://auth0.com/docs/api/management/v2#!/Roles/post_role_permission_assignment
	AssociatePermissions(id string, permissions []*management.Permission, opts ...management.RequestOption) error

	// Permissions retrieves all permissions granted by a role.
	//
	// See: https://auth0.com/docs/api/management/v2#!/Roles/get_role_permission
	Permissions(id string, opts ...management.RequestOption) (p *management.PermissionList, err error)

	// RemovePermissions removes permissions associated to a role.
	//
	// See: https://auth0.com/docs/api/management/v2#!/Roles/delete_role_permission_assignment
	RemovePermissions(id string, permissions []*management.Permission, opts ...management.RequestOption) error
}
