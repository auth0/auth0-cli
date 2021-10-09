package management

type Role struct {
	// A unique ID for the role.
	ID *string `json:"id,omitempty"`

	// The name of the role created.
	Name *string `json:"name,omitempty"`

	// A description of the role created.
	Description *string `json:"description,omitempty"`
}

type RoleList struct {
	List
	Roles []*Role `json:"roles"`
}

type Permission struct {
	// The resource server that the permission is attached to.
	ResourceServerIdentifier *string `json:"resource_server_identifier,omitempty"`

	// The name of the resource server.
	ResourceServerName *string `json:"resource_server_name,omitempty"`

	// The name of the permission.
	Name *string `json:"permission_name,omitempty"`

	// The description of the permission.
	Description *string `json:"description,omitempty"`
}

type PermissionList struct {
	List
	Permissions []*Permission `json:"permissions"`
}

type RoleManager struct {
	*Management
}

func newRoleManager(m *Management) *RoleManager {
	return &RoleManager{m}
}

// Create a new role.
//
// See: https://auth0.com/docs/api/management/v2#!/Roles/post_roles
func (m *RoleManager) Create(r *Role, opts ...RequestOption) error {
	return m.Request("POST", m.URI("roles"), r, opts...)
}

// Retrieve a role.
//
// See: https://auth0.com/docs/api/management/v2#!/Roles/get_roles_by_id
func (m *RoleManager) Read(id string, opts ...RequestOption) (r *Role, err error) {
	err = m.Request("GET", m.URI("roles", id), &r, opts...)
	return
}

// Update a role.
//
// See: https://auth0.com/docs/api/management/v2#!/Roles/patch_roles_by_id
func (m *RoleManager) Update(id string, r *Role, opts ...RequestOption) (err error) {
	return m.Request("PATCH", m.URI("roles", id), r, opts...)
}

// Delete a role.
//
// See: https://auth0.com/docs/api/management/v2#!/Roles/delete_roles_by_id
func (m *RoleManager) Delete(id string, opts ...RequestOption) (err error) {
	// Deleting a role results in a 200 status code instead of 204 which
	// triggers decoding of the response payload.
	//
	// In order to avoid Unmarshal(nil) errors, we pass an empty &Role{}.
	return m.Request("DELETE", m.URI("roles", id), &Role{}, opts...)
}

// List all roles that can be assigned to users or groups.
//
// See: https://auth0.com/docs/api/management/v2#!/Roles/get_roles
func (m *RoleManager) List(opts ...RequestOption) (r *RoleList, err error) {
	err = m.Request("GET", m.URI("roles"), &r, applyListDefaults(opts))
	return
}

// AssignUsers assigns users to a role.
//
// See: https://auth0.com/docs/api/management/v2#!/Roles/post_role_users
func (m *RoleManager) AssignUsers(id string, users []*User, opts ...RequestOption) error {
	u := make(map[string][]*string)
	u["users"] = make([]*string, len(users))
	for i, user := range users {
		u["users"][i] = user.ID
	}
	return m.Request("POST", m.URI("roles", id, "users"), &u, opts...)
}

// Users retrieves all users associated with a role.
//
// See: https://auth0.com/docs/api/management/v2#!/Roles/get_role_user
func (m *RoleManager) Users(id string, opts ...RequestOption) (u *UserList, err error) {
	err = m.Request("GET", m.URI("roles", id, "users"), &u, applyListDefaults(opts))
	return
}

// AssociatePermissions associates permissions to a role.
//
// See: https://auth0.com/docs/api/management/v2#!/Roles/post_role_permission_assignment
func (m *RoleManager) AssociatePermissions(id string, permissions []*Permission, opts ...RequestOption) error {
	p := make(map[string][]*Permission)
	p["permissions"] = permissions
	return m.Request("POST", m.URI("roles", id, "permissions"), &p, opts...)
}

// Permissions retrieves all permissions granted by a role.
//
// See: https://auth0.com/docs/api/management/v2#!/Roles/get_role_permission
func (m *RoleManager) Permissions(id string, opts ...RequestOption) (p *PermissionList, err error) {
	err = m.Request("GET", m.URI("roles", id, "permissions"), &p, applyListDefaults(opts))
	return
}

// RemovePermissions removes permissions associated to a role.
//
// See: https://auth0.com/docs/api/management/v2#!/Roles/delete_role_permission_assignment
func (m *RoleManager) RemovePermissions(id string, permissions []*Permission, opts ...RequestOption) error {
	p := make(map[string][]*Permission)
	p["permissions"] = permissions
	return m.Request("DELETE", m.URI("roles", id, "permissions"), &p, opts...)
}
