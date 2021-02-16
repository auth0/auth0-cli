//go:generate mockgen -source=role.go -destination=role_mock.go -package=auth0

package auth0

import (
	"fmt"

	"gopkg.in/auth0.v5/management"
)

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

	RolePermissionsAPI

	RoleUsersAPI
}

type RolePermissionsAPI interface {
	// Permissions retrieves all permissions granted by a role.
	Permissions(id string, opts ...management.RequestOption) (pl *management.PermissionList, err error)

	// RemovePermissions removes permissions associated to a role.
	RemovePermissions(id string, permissions []*management.Permission, opts ...management.RequestOption) (err error)

	// AssociatePermissions associates permissions to a role.
	AssociatePermissions(id string, permissions []*management.Permission, opts ...management.RequestOption) (err error)
}

type RoleUsersAPI interface {
	// Users retrieves all users associated with a role.
	Users(id string, opts ...management.RequestOption) (ul *management.UserList, err error)

	// AssignUsers assigns users to a role.
	AssignUsers(id string, users []*management.User, opts ...management.RequestOption) (err error)
}

// GetRolesForMultiSelect returns a slice of role id and name strings which can be passed into survey.MultiSelect.
func GetRolesForMultiSelect(r RoleAPI) ([]string, error) {
	roles := []string{}

	list, err := r.List()
	if err != nil {
		return nil, err
	}

	for _, i := range list.Roles {
		roles = append(roles, fmt.Sprintf("%s\t%s", *i.ID, *i.Name))
	}

	return roles, nil
}
