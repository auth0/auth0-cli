package portalstest

import (
	"slices"

	"github.com/auth0/auth0-cli/internal/portals"
)

// AddUser adds a user to the tenant.
func (t *Tenant) AddUser(user portals.User) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.users = append(t.users, user)
}

// AddOrganization adds an organization with the given members.
func (t *Tenant) AddOrganization(organization portals.Organization, memberIDs ...string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.organizations = append(t.organizations, organization)
	t.members[organization.ID] = slices.Clone(memberIDs)
}

// EnableOrganizationConnectionFor enables connections for an organization, without a request.
func (t *Tenant) EnableOrganizationConnectionFor(organizationID string, connectionIDs ...string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.orgConnections[organizationID] = append(t.orgConnections[organizationID], connectionIDs...)
}

// AddConnection adds a connection with the given strategy, for example "auth0" for a database.
func (t *Tenant) AddConnection(connection portals.Connection, strategy string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.connections = append(t.connections, connectionRecord{Connection: connection, strategy: strategy})
}

// EnableClientConnection enables a connection for an application, without a request.
func (t *Tenant) EnableClientConnection(clientID, connectionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if index := slices.IndexFunc(t.clients, func(record clientRecord) bool { return record.ID == clientID }); index >= 0 {
		t.clients[index].enabledConnections = append(t.clients[index].enabledConnections, connectionID)
	}
}

// DisableClientConnection turns a connection off for an application, without a request.
func (t *Tenant) DisableClientConnection(clientID, connectionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if index := slices.IndexFunc(t.clients, func(record clientRecord) bool { return record.ID == clientID }); index >= 0 {
		t.clients[index].enabledConnections = slices.DeleteFunc(t.clients[index].enabledConnections, func(id string) bool { return id == connectionID })
	}
}

// AddRole adds a role with the given permissions.
func (t *Tenant) AddRole(role portals.Role, permissions ...portals.Permission) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.roles = append(t.roles, roleRecord{Role: role, permissions: slices.Clone(permissions)})
}

// AssignRole gives a member of an organization a role, without a request.
func (t *Tenant) AssignRole(organizationID, userID, roleID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := organizationID + ":" + userID
	t.orgRoles[key] = append(t.orgRoles[key], roleID)
}

// AddResourceServer adds a resource server, without a request.
func (t *Tenant) AddResourceServer(server portals.ResourceServer) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.resourceServers = append(t.resourceServers, server)
}

// AddClient adds an application, without a request.
func (t *Tenant) AddClient(client portals.Client) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.clients = append(t.clients, clientRecord{Client: clone(client)})
}

// AddGrant adds a client grant, without a request.
func (t *Tenant) AddGrant(grant portals.ClientGrant) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.grants = append(t.grants, clone(grant))
}

// AddPortal adds a portal, without a request.
func (t *Tenant) AddPortal(portal portals.Portal) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.portals = append(t.portals, clone(portal))
}

// AddForm adds a form that runs the flow, without a request.
func (t *Tenant) AddForm(form portals.Form, flowID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.forms = append(t.forms, formRecord{Form: form, flowID: flowID})
}

// DeleteForm removes a form, without a request.
func (t *Tenant) DeleteForm(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.forms = slices.DeleteFunc(t.forms, func(form formRecord) bool { return form.ID == id })
}

// DeleteClient removes an application, without a request.
func (t *Tenant) DeleteClient(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.clients = slices.DeleteFunc(t.clients, func(record clientRecord) bool { return record.ID == id })
}

// Clients returns every application with its secret.
func (t *Tenant) Clients() []portals.Client {
	t.mu.Lock()
	defer t.mu.Unlock()

	clients := make([]portals.Client, 0, len(t.clients))
	for _, record := range t.clients {
		clients = append(clients, clone(record.Client))
	}

	return clients
}

// Grants returns every client grant.
func (t *Tenant) Grants() []portals.ClientGrant {
	t.mu.Lock()
	defer t.mu.Unlock()

	return clone(t.grants)
}

// Portals returns every portal.
func (t *Tenant) Portals() []portals.Portal {
	t.mu.Lock()
	defer t.mu.Unlock()

	return clone(t.portals)
}

// ResourceServers returns every resource server.
func (t *Tenant) ResourceServers() []portals.ResourceServer {
	t.mu.Lock()
	defer t.mu.Unlock()

	return clone(t.resourceServers)
}

// Forms returns every form.
func (t *Tenant) Forms() []portals.Form {
	t.mu.Lock()
	defer t.mu.Unlock()

	forms := make([]portals.Form, 0, len(t.forms))
	for _, record := range t.forms {
		forms = append(forms, record.Form)
	}

	return forms
}

// VaultConnections returns every vault connection.
func (t *Tenant) VaultConnections() []portals.VaultConnection {
	t.mu.Lock()
	defer t.mu.Unlock()

	vaults := make([]portals.VaultConnection, 0, len(t.vaults))
	for _, record := range t.vaults {
		vaults = append(vaults, record.VaultConnection)
	}

	return vaults
}

// Members returns the IDs of the members of an organization.
func (t *Tenant) Members(organizationID string) []string {
	t.mu.Lock()
	defer t.mu.Unlock()

	return slices.Clone(t.members[organizationID])
}

// Organizations returns every organization.
func (t *Tenant) Organizations() []portals.Organization {
	t.mu.Lock()
	defer t.mu.Unlock()

	return slices.Clone(t.organizations)
}

// Roles returns every role.
func (t *Tenant) Roles() []portals.Role {
	t.mu.Lock()
	defer t.mu.Unlock()

	roles := make([]portals.Role, 0, len(t.roles))
	for _, record := range t.roles {
		roles = append(roles, record.Role)
	}

	return roles
}

// RolePermissions returns the permissions of a role.
func (t *Tenant) RolePermissions(roleID string) []portals.Permission {
	t.mu.Lock()
	defer t.mu.Unlock()

	if index := slices.IndexFunc(t.roles, func(role roleRecord) bool { return role.ID == roleID }); index >= 0 {
		return slices.Clone(t.roles[index].permissions)
	}

	return nil
}

// ModifyPortal changes a portal without a request, for a test that needs a portal that broke.
func (t *Tenant) ModifyPortal(id string, change func(portal *portals.Portal)) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if index := slices.IndexFunc(t.portals, func(portal portals.Portal) bool { return portal.ID == id }); index >= 0 {
		change(&t.portals[index])
	}
}

// ModifyClient changes an application without a request, for a test that needs an application that
// broke.
func (t *Tenant) ModifyClient(id string, change func(client *portals.Client)) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if index := slices.IndexFunc(t.clients, func(record clientRecord) bool { return record.ID == id }); index >= 0 {
		change(&t.clients[index].Client)
	}
}

// DeleteGrant removes a client grant without a request.
func (t *Tenant) DeleteGrant(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.grants = slices.DeleteFunc(t.grants, func(grant portals.ClientGrant) bool { return grant.ID == id })
}

// DeleteResourceServer removes a resource server without a request.
func (t *Tenant) DeleteResourceServer(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.resourceServers = slices.DeleteFunc(t.resourceServers, func(server portals.ResourceServer) bool { return server.ID == id })
}
