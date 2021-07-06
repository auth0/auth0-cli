package auth0

import "gopkg.in/auth0.v5/management"

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
}
