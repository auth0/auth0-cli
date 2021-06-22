package management

type Organization struct {
	// Organization identifier
	ID *string `json:"id,omitempty"`

	// Name of this organization.
	Name *string `json:"name,omitempty"`

	// DisplayName of this organization.
	DisplayName *string `json:"display_name,omitempty"`

	// Branding defines how to style the login pages
	Branding *OrganizationBranding `json:"branding,omitempty"`

	// Metadata associated with the organization, in the form of an object with string values (max 255 chars). Maximum of 10 metadata properties allowed.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type OrganizationBranding struct {
	// URL of logo to display on login page
	LogoUrl *string `json:"logo_url,omitempty"`

	// Color scheme used to customize the login pages
	Colors map[string]string `json:"colors,omitempty"`
}

type OrganizationMember struct {
	UserID  *string `json:"user_id,omitempty"`
	Picture *string `json:"picture,omitempty"`
	Name    *string `json:"name,omitempty"`
	Email   *string `json:"email,omitempty"`
}

type OrganizationConnection struct {
	// ID of the connection.
	ConnectionID *string `json:"connection_id,omitempty"`

	// When true, all users that log in with this connection will be automatically granted membership in the organization. When false, users must be granted membership in the organization before logging in with this connection.
	AssignMembershipOnLogin *bool `json:"assign_membership_on_login,omitempty"`

	// Connection details
	Connection *OrganizationConnectionDetails `json:"connection,omitempty"`
}

type OrganizationConnectionDetails struct {
	// The name of the enabled connection.
	Name *string `json:"name,omitempty"`

	// The strategy of the enabled connection.
	Strategy *string `json:"strategy,omitempty"`
}

type OrganizationInvitationInviter struct {
	// The inviter's name.
	Name *string `json:"name,omitempty"`
}

type OrganizationInvitationInvitee struct {
	// The invitee's email.
	Email *string `json:"email,omitempty"`
}

type OrganizationInvitation struct {
	// The id of the user invitation.
	ID *string `json:"id,omitempty"`

	// Organization identifier
	OrganizationID *string `json:"organization_id,omitempty"`

	Inviter *OrganizationInvitationInviter `json:"inviter,omitempty"`

	Invitee *OrganizationInvitationInvitee `json:"invitee,omitempty"`

	// The invitation url to be send to the invitee.
	InvitationUrl *string `json:"invitation_url,omitempty"`

	// The ISO 8601 formatted timestamp representing the creation time of the invitation.
	CreatedAt *string `json:"created_at,omitempty"`

	// Number of seconds for which the invitation is valid before expiration. If unspecified or set to 0, this
	// value defaults to 604800 seconds (7 days). Max value: 2592000 seconds (30 days).
	TTLSec *int `json:"ttl_sec,omitempty"`

	// The ISO 8601 formatted timestamp representing the expiration time of the invitation.
	ExpiresAt *string `json:"expires_at,omitempty"`

	// Auth0 client ID. Used to resolve the application's login initiation endpoint.
	ClientID *string `json:"client_id,omitempty"`

	// The id of the connection to force invitee to authenticate with.
	ConnectionID *string `json:"connection_id,omitempty"`

	// Data related to the user that does affect the application's core functionality.
	AppMetadata map[string]interface{} `json:"app_metadata,omitempty"`

	// Data related to the user that does not affect the application's core functionality.
	UserMetadata map[string]interface{} `json:"user_metadata,omitempty"`

	// List of roles IDs to associated with the user.
	Roles []string `json:"roles,omitempty"`

	// The id of the invitation ticket
	TicketID *string `json:"ticket_id,omitempty"`

	// Whether the user will receive an invitation email (true) or no email (false), true by default
	SendInvitationEmail *bool `json:"send_invitation_email,omitempty"`
}

type OrganizationMemberRole struct {
	// ID for this role.
	ID *string `json:"id,omitempty"`

	// Name of the role.
	Name *string `json:"name,omitempty"`

	// Description of the role.
	Description *string `json:"description,omitempty"`
}

type OrganizationMemberRoleList struct {
	List
	Roles []OrganizationMemberRole `json:"roles"`
}

type OrganizationInvitationList struct {
	List
	OrganizationInvitations []*OrganizationInvitation `json:"invitations"`
}

type OrganizationConnectionList struct {
	List
	OrganizationConnections []*OrganizationConnection `json:"enabled_connections"`
}

type OrganizationMemberList struct {
	List
	Members []OrganizationMember `json:"members"`
}

type OrganizationList struct {
	List
	Organizations []*Organization `json:"organizations"`
}

type OrganizationManager struct {
	*Management
}

func newOrganizationManager(m *Management) *OrganizationManager {
	return &OrganizationManager{m}
}

// List available organizations
//
// See: https://auth0.com/docs/api/management/v2/#!/Organizations/get_organizations
func (m *OrganizationManager) List(opts ...RequestOption) (o *OrganizationList, err error) {
	err = m.Request("GET", m.URI("organizations"), &o, applyListDefaults(opts))
	return
}

// Create an Organization
//
// See: https://auth0.com/docs/api/management/v2/#!/Organizations/post_organizations
func (m *OrganizationManager) Create(o *Organization, opts ...RequestOption) (err error) {
	o.ID = nil
	err = m.Request("POST", m.URI("organizations"), &o, opts...)
	return
}

// Get a specific organization
//
// See: https://auth0.com/docs/api/management/v2/#!/Organizations/get_organizations_by_id
func (m *OrganizationManager) Read(id string, opts ...RequestOption) (o *Organization, err error) {
	err = m.Request("GET", m.URI("organizations", id), &o, opts...)
	return
}

// Delete a specific organization
//
// See: https://auth0.com/docs/api/management/v2/#!/Organizations/delete_organizations_by_id
func (m *OrganizationManager) Delete(id string, opts ...RequestOption) (err error) {
	err = m.Request("DELETE", m.URI("organizations", id), nil, opts...)
	return
}

// Modify an organization
//
// See: https://auth0.com/docs/api/management/v2/#!/Organizations/patch_organizations_by_id
func (m *OrganizationManager) Update(o *Organization, opts ...RequestOption) (err error) {
	id := o.GetID()
	if o != nil {
		o.ID = nil
		o.Name = nil
	}
	err = m.Request("PATCH", m.URI("organizations", id), &o, opts...)
	return
}

// Get a specific organization by name
//
// See: https://auth0.com/docs/api/management/v2/#!/Organizations/get_name_by_name
func (m *OrganizationManager) ReadByName(name string, opts ...RequestOption) (o *Organization, err error) {
	err = m.Request("GET", m.URI("organizations", "name", name), &o, opts...)
	return
}

// Get connections enabled for an organization
//
// See: https://auth0.com/docs/api/management/v2/#!/Organizations/get_enabled_connections
func (m *OrganizationManager) Connections(id string, opts ...RequestOption) (c *OrganizationConnectionList, err error) {
	err = m.Request("GET", m.URI("organizations", id, "enabled_connections"), &c, applyListDefaults(opts))
	return
}

// Add connections to an organization
//
// See: https://auth0.com/docs/api/management/v2/#!/Organizations/post_enabled_connections
func (m *OrganizationManager) AddConnection(id string, c *OrganizationConnection, opts ...RequestOption) (err error) {
	err = m.Request("POST", m.URI("organizations", id, "enabled_connections"), &c, opts...)
	return
}

// Get an enabled connection for an organization
//
// See: https://auth0.com/docs/api/management/v2/#!/Organizations/get_enabled_connections_by_connectionId
func (m *OrganizationManager) Connection(id string, connectionID string, opts ...RequestOption) (c *OrganizationConnection, err error) {
	err = m.Request("GET", m.URI("organizations", id, "enabled_connections", connectionID), &c, opts...)
	return
}

// Delete connections from an organization
//
// See: https://auth0.com/docs/api/management/v2/#!/Organizations/delete_enabled_connections_by_connectionId
func (m *OrganizationManager) DeleteConnection(id string, connectionID string, opts ...RequestOption) (err error) {
	err = m.Request("DELETE", m.URI("organizations", id, "enabled_connections", connectionID), nil, opts...)
	return
}

// Modify an enabled_connection belonging to an Organization
//
// See: https://auth0.com/docs/api/management/v2/#!/Organizations/patch_enabled_connections_by_connectionId
func (m *OrganizationManager) UpdateConnection(id string, c *OrganizationConnection, opts ...RequestOption) (err error) {
	connectionID := c.GetConnectionID()
	if c != nil {
		c.ConnectionID = nil
		c.Connection = nil
	}
	err = m.Request("PATCH", m.URI("organizations", id, "enabled_connections", connectionID), &c, opts...)
	return
}

// Get invitations to organization
//
// See: https://auth0.com/docs/api/management/v2/#!/Organizations/get_invitations
func (m *OrganizationManager) Invitations(id string, opts ...RequestOption) (i *OrganizationInvitationList, err error) {
	err = m.Request("GET", m.URI("organizations", id, "invitations"), &i, applyListDefaults(opts))
	return
}

// Create invitations to organization
//
// See: https://auth0.com/docs/api/management/v2/#!/Organizations/post_invitations
func (m *OrganizationManager) CreateInvitation(i *OrganizationInvitation, opts ...RequestOption) (err error) {
	organizationID := i.GetOrganizationID()
	i.OrganizationID = nil
	err = m.Request("POST", m.URI("organizations", organizationID, "invitations"), &i, opts...)
	return
}

// Get an invitation to organization
//
// See: https://auth0.com/docs/api/management/v2/#!/Organizations/get_invitations_by_invitation_id
func (m *OrganizationManager) Invitation(id string, invitationID string, opts ...RequestOption) (i *OrganizationInvitation, err error) {
	err = m.Request("GET", m.URI("organizations", id, "invitations", invitationID), &i, opts...)
	return
}

// Delete an invitation to organization
//
// See: https://auth0.com/docs/api/management/v2/#!/Organizations/delete_invitations_by_invitation_id
func (m *OrganizationManager) DeleteInvitation(id string, invitationID string, opts ...RequestOption) (err error) {
	err = m.Request("DELETE", m.URI("organizations", id, "invitations", invitationID), nil, opts...)
	return
}

// List organization members
//
// See: https://auth0.com/docs/api/management/v2/#!/Organizations/get_members
func (m *OrganizationManager) Members(id string, opts ...RequestOption) (o *OrganizationMemberList, err error) {
	err = m.Request("GET", m.URI("organizations", id, "members"), &o, applyListDefaults(opts))
	return
}

// Add members to an organization
//
// See: https://auth0.com/docs/api/management/v2/#!/Organizations/post_members
func (m *OrganizationManager) AddMembers(id string, memberIDs []string, opts ...RequestOption) (err error) {
	body := struct {
		Members []string `json:"members"`
	}{
		Members: memberIDs,
	}
	err = m.Request("POST", m.URI("organizations", id, "members"), &body, opts...)
	return
}

// Delete members from an organization
//
// See: https://auth0.com/docs/api/management/v2/#!/Organizations/delete_members
func (m *OrganizationManager) DeleteMember(id string, memberIDs []string, opts ...RequestOption) (err error) {
	body := struct {
		Members []string `json:"members"`
	}{
		Members: memberIDs,
	}
	err = m.Request("DELETE", m.URI("organizations", id, "members"), &body, opts...)
	return
}

// Get the roles assigned to an organization member
//
// See: https://auth0.com/docs/api/management/v2/#!/Organizations/get_organization_member_roles
func (m *OrganizationManager) MemberRoles(id string, userID string, opts ...RequestOption) (r *OrganizationMemberRoleList, err error) {
	err = m.Request("GET", m.URI("organizations", id, "members", userID, "roles"), &r, applyListDefaults(opts))
	return
}

// Assign one or more roles to a given user that will be applied in the context of the provided organization
//
// See: https://auth0.com/docs/api/management/v2/#!/Organizations/post_organization_member_roles
func (m *OrganizationManager) AssignMemberRoles(id string, userID string, roles []string, opts ...RequestOption) (err error) {
	body := struct {
		Roles []string `json:"roles"`
	}{
		Roles: roles,
	}
	err = m.Request("POST", m.URI("organizations", id, "members", userID, "roles"), &body, opts...)
	return
}

// Remove one or more roles from a given user in the context of the provided organization
//
// See: https://auth0.com/docs/api/management/v2/#!/Organizations/delete_organization_member_roles
func (m *OrganizationManager) DeleteMemberRoles(id string, userID string, roles []string, opts ...RequestOption) (err error) {
	body := struct {
		Roles []string `json:"roles"`
	}{
		Roles: roles,
	}
	err = m.Request("DELETE", m.URI("organizations", id, "members", userID, "roles"), &body, opts...)
	return
}
