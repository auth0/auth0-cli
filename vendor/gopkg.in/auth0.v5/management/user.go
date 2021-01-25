package management

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"
	"time"
)

// User represents an Auth0 user resource
//
// See: https://auth0.com/docs/users
type User struct {

	// The users identifier.
	ID *string `json:"user_id,omitempty"`

	// The connection the user belongs to.
	Connection *string `json:"connection,omitempty"`

	// The user's email
	Email *string `json:"email,omitempty"`

	// The users name
	Name *string `json:"name,omitempty"`

	// The users given name
	GivenName *string `json:"given_name,omitempty"`

	// The users family name
	FamilyName *string `json:"family_name,omitempty"`

	// The user's username. Only valid if the connection requires a username
	Username *string `json:"username,omitempty"`

	// The user's nickname
	Nickname *string `json:"nickname,omitempty"`

	// The user's password (mandatory for non SMS connections)
	Password *string `json:"password,omitempty"`

	// The user's phone number (following the E.164 recommendation), only valid
	// for users to be added to SMS connections.
	PhoneNumber *string `json:"phone_number,omitempty"`

	// The time the user is created.
	CreatedAt *time.Time `json:"created_at,omitempty"`

	// The last time the user is updated.
	UpdatedAt *time.Time `json:"updated_at,omitempty"`

	// The last time the user has logged in.
	LastLogin *time.Time `json:"last_login,omitempty"`

	// UserMetadata holds data that the user has read/write access to (e.g.
	// color_preference, blog_url, etc).
	UserMetadata map[string]interface{} `json:"user_metadata,omitempty"`

	// Identities is a list of user identities for when accounts are linked.
	Identities []*UserIdentity `json:"identities,omitempty"`

	// True if the user's email is verified, false otherwise. If it is true then
	// the user will not receive a verification email, unless verify_email: true
	// was specified.
	EmailVerified *bool `json:"-"`

	// If true, the user will receive a verification email after creation, even
	// if created with email_verified set to true. If false, the user will not
	// receive a verification email, even if created with email_verified set to
	// false. If unspecified, defaults to the behavior determined by the value
	// of email_verified.
	VerifyEmail *bool `json:"verify_email,omitempty"`

	// True if the user's phone number is verified, false otherwise. When the
	// user is added to a SMS connection, they will not receive an verification
	// SMS if this is true.
	PhoneVerified *bool `json:"phone_verified,omitempty"`

	// AppMetadata holds data that the user has read-only access to (e.g. roles,
	// permissions, vip, etc).
	AppMetadata map[string]interface{} `json:"app_metadata,omitempty"`

	// The user's picture url
	Picture *string `json:"picture,omitempty"`

	// True if the user is blocked from the application, false if the user is enabled
	Blocked *bool `json:"blocked,omitempty"`

	// Last IP address from which this user logged in. Read only, cannot be modified.
	LastIP *string `json:"last_ip,omitempty"`

	// Total number of logins this user has performed. Read only, cannot be modified.
	LoginsCount *int64 `json:"logins_count,omitempty"`
}

// UnmarshalJSON is a custom deserializer for the User type.
//
// We have to use a custom one due to possible inconsistencies in value types.
func (u *User) UnmarshalJSON(b []byte) error {

	type user User
	type userAlias struct {
		*user
		RawEmailVerified interface{} `json:"email_verified,omitempty"`
	}

	alias := &userAlias{(*user)(u), nil}

	err := json.Unmarshal(b, alias)
	if err != nil {
		return err
	}

	if alias.RawEmailVerified != nil {
		var emailVerified bool
		switch rawEmailVerified := alias.RawEmailVerified.(type) {
		case bool:
			emailVerified = rawEmailVerified
		case string:
			emailVerified, err = strconv.ParseBool(rawEmailVerified)
			if err != nil {
				return err
			}
		default:
			panic(reflect.TypeOf(rawEmailVerified))
		}
		alias.EmailVerified = &emailVerified
	}

	return nil
}
func (u *User) MarshalJSON() ([]byte, error) {

	type user User
	type userAlias struct {
		*user
		RawEmailVerified interface{} `json:"email_verified,omitempty"`
	}

	alias := &userAlias{user: (*user)(u)}
	if u.EmailVerified != nil {
		alias.RawEmailVerified = u.EmailVerified
	}

	return json.Marshal(alias)
}

// UserIdentityLink contains the data needed for linking an identity to a given user.
type UserIdentityLink struct {
	// Connection id of the secondary user account being linked when more than one auth0 database provider exists.
	ConnectionID *string `json:"connection_id,omitempty"`
	// Secondary account user id.
	UserID *string `json:"user_id,omitempty"`
	// Identity provider of the secondary user account being linked.
	Provider *string `json:"provider,omitempty"`
	// LinkWith requires the authenticated primary account's JWT in the Authorization header.
	// Must be a JWT for the secondary account being linked. If sending this parameter,
	// provider, user_id, and connection_id must not be sent.
	LinkWith *string `json:"link_with,omitempty"`
}

type UserIdentity struct {
	Connection   *string `json:"connection,omitempty"`
	UserID       *string `json:"-"`
	Provider     *string `json:"provider,omitempty"`
	IsSocial     *bool   `json:"isSocial,omitempty"`
	AccessToken  *string `json:"access_token,omitempty"`
	RefreshToken *string `json:"refresh_token,omitempty"`
}

// UnmarshalJSON is a custom deserializer for the UserIdentity type.
//
// We have to use a custom one due to a bug in the Auth0 Management API which
// might return a number for `user_id` instead of a string.
//
// See https://community.auth0.com/t/users-user-id-returns-inconsistent-type-for-identities-user-id/39236
func (i *UserIdentity) UnmarshalJSON(b []byte) error {

	type userIdentity UserIdentity
	type userIdentityAlias struct {
		*userIdentity
		RawUserID interface{} `json:"user_id,omitempty"`
	}

	alias := &userIdentityAlias{(*userIdentity)(i), nil}

	err := json.Unmarshal(b, alias)
	if err != nil {
		return err
	}

	if alias.RawUserID != nil {
		var id string
		switch rawID := alias.RawUserID.(type) {
		case string:
			id = rawID
		case float64:
			id = strconv.Itoa(int(rawID))
		default:
			panic(reflect.TypeOf(rawID))
		}
		alias.UserID = &id
	}

	return nil
}

func (i *UserIdentity) MarshalJSON() ([]byte, error) {

	type userIdentity UserIdentity
	type userIdentityAlias struct {
		*userIdentity
		RawUserID interface{} `json:"user_id,omitempty"`
	}

	alias := &userIdentityAlias{userIdentity: (*userIdentity)(i)}
	if i.UserID != nil {
		alias.RawUserID = i.UserID
	}

	return json.Marshal(alias)
}

type userBlock struct {
	BlockedFor []*UserBlock `json:"blocked_for,omitempty"`
}

type UserBlock struct {
	Identifier *string `json:"identifier,omitempty"`
	IP         *string `json:"ip,omitempty"`
}

// UserList is an envelope struct which is used when calling List() or Search()
// methods.
//
// It holds metadata such as the total result count, starting offset and limit.
type UserList struct {
	List
	Users []*User `json:"users"`
}

// UserManager manages Auth0 User resources.
type UserManager struct {
	*Management
}

// newUserManager returns a new instance of a user manager.
func newUserManager(m *Management) *UserManager {
	return &UserManager{m}
}

// Create a new user. It works only for database and passwordless connections.
//
// The samples on the right show you every attribute that could be used. The
// attribute connection is always mandatory but depending on the type of
// connection you are using there could be others too. For instance, database
// connections require `email` and `password`.
//
// See: https://auth0.com/docs/api/management/v2#!/Users/post_users
func (m *UserManager) Create(u *User, opts ...RequestOption) error {
	return m.Request("POST", m.URI("users"), u, opts...)
}

// Read user details for a given user_id.
//
// See: https://auth0.com/docs/api/management/v2#!/Users/get_users_by_id
func (m *UserManager) Read(id string, opts ...RequestOption) (u *User, err error) {
	err = m.Request("GET", m.URI("users", id), &u, opts...)
	return
}

// Update user.
//
// The following attributes can be updated at the root level:
//
// - `app_metadata`
// - `blocked`
// - `email`
// - `email_verified`
// - `family_name`
// - `given_name`
// - `name`
// - `nickname`
// - `password`
// - `phone_number`
// - `phone_verified`
// - `picture`
// - `username`
// - `user_metadata`
// - `verify_email`
//
// See: https://auth0.com/docs/api/management/v2#!/Users/patch_users_by_id
func (m *UserManager) Update(id string, u *User, opts ...RequestOption) (err error) {
	return m.Request("PATCH", m.URI("users", id), u, opts...)
}

// Delete a single user based on its id.
//
// See: https://auth0.com/docs/api/management/v2#!/Users/delete_users_by_id
func (m *UserManager) Delete(id string, opts ...RequestOption) (err error) {
	return m.Request("DELETE", m.URI("users", id), nil, opts...)
}

// List all users. This method forces the `include_totals` option.
//
// See: https://auth0.com/docs/api/management/v2#!/Users/get_users
func (m *UserManager) List(opts ...RequestOption) (ul *UserList, err error) {
	err = m.Request("GET", m.URI("users"), &ul, applyListDefaults(opts))
	return
}

// Search is an alias for List.
func (m *UserManager) Search(opts ...RequestOption) (ul *UserList, err error) {
	return m.List(opts...)
}

// ListByEmail retrieves all users matching a given email.
//
// If Auth0 is the identify provider (idP), the email address associated with a
// user is saved in lower case, regardless of how you initially provided it.
// For example, if you register a user as JohnSmith@example.com, Auth0 saves the
// user's email as johnsmith@example.com.
//
// In cases where Auth0 is not the idP, the `email` is stored based on the rules
// of idP, so make sure the search is made using the correct capitalization.
//
// When using this endpoint, make sure that you are searching for users via
// email addresses using the correct case.
//
// See: https://auth0.com/docs/api/management/v2#!/Users_By_Email/get_users_by_email
func (m *UserManager) ListByEmail(email string, opts ...RequestOption) (us []*User, err error) {
	opts = append(opts, Parameter("email", email))
	err = m.Request("GET", m.URI("users-by-email"), &us, opts...)
	return
}

// Roles lists all roles associated with a user.
//
// See: https://auth0.com/docs/api/management/v2#!/Users/get_user_roles
func (m *UserManager) Roles(id string, opts ...RequestOption) (r *RoleList, err error) {
	err = m.Request("GET", m.URI("users", id, "roles"), &r, applyListDefaults(opts))
	return
}

// AssignRoles assignes roles to a user.
//
// See: https://auth0.com/docs/api/management/v2#!/Users/post_user_roles
func (m *UserManager) AssignRoles(id string, roles []*Role, opts ...RequestOption) error {
	r := make(map[string][]*string)
	r["roles"] = make([]*string, len(roles))
	for i, role := range roles {
		r["roles"][i] = role.ID
	}
	return m.Request("POST", m.URI("users", id, "roles"), &r, opts...)
}

// RemoveRoles removes any roles associated to a user.
//
// See: https://auth0.com/docs/api/management/v2#!/Users/delete_user_roles
func (m *UserManager) RemoveRoles(id string, roles []*Role, opts ...RequestOption) error {
	r := make(map[string][]*string)
	r["roles"] = make([]*string, len(roles))
	for i, role := range roles {
		r["roles"][i] = role.ID
	}
	return m.Request("DELETE", m.URI("users", id, "roles"), &r, opts...)
}

// Permissions lists the permissions associated to the user.
//
// See: https://auth0.com/docs/api/management/v2#!/Users/get_permissions
func (m *UserManager) Permissions(id string, opts ...RequestOption) (p *PermissionList, err error) {
	err = m.Request("GET", m.URI("users", id, "permissions"), &p, applyListDefaults(opts))
	return
}

// AssignPermissions assigns permissions to the user.
//
// See: https://auth0.com/docs/api/management/v2#!/Users/post_permissions
func (m *UserManager) AssignPermissions(id string, permissions []*Permission, opts ...RequestOption) error {
	p := make(map[string][]*Permission)
	p["permissions"] = permissions
	return m.Request("POST", m.URI("users", id, "permissions"), &p, opts...)
}

// RemovePermissions removes any permissions associated to a user.
//
// See: https://auth0.com/docs/api/management/v2#!/Users/delete_permissions
func (m *UserManager) RemovePermissions(id string, permissions []*Permission, opts ...RequestOption) error {
	p := make(map[string][]*Permission)
	p["permissions"] = permissions
	return m.Request("DELETE", m.URI("users", id, "permissions"), &p, opts...)
}

// Blocks retrieves a list of blocked IP addresses of a particular user.
//
// See: https://auth0.com/docs/api/management/v2#!/User_Blocks/get_user_blocks_by_id
func (m *UserManager) Blocks(id string, opts ...RequestOption) ([]*UserBlock, error) {
	b := new(userBlock)
	err := m.Request("GET", m.URI("user-blocks", id), &b, opts...)
	return b.BlockedFor, err
}

// Unblock a user that was blocked due to an excessive amount of incorrectly
// provided credentials.
//
// Note: This endpoint does not unblock users that were blocked by admins.
//
// See: https://auth0.com/docs/api/management/v2#!/User_Blocks/delete_user_blocks_by_id
func (m *UserManager) Unblock(id string, opts ...RequestOption) error {
	return m.Request("DELETE", m.URI("user-blocks", id), nil, opts...)
}

// Link links two user accounts together forming a primary and secondary relationship.
//
// See: https://auth0.com/docs/api/management/v2#!/Users/post_identities
func (m *UserManager) Link(id string, il *UserIdentityLink, opts ...RequestOption) (uIDs []UserIdentity, err error) {
	req, err := m.NewRequest("POST", m.URI("users", id, "identities"), il, opts...)
	if err != nil {
		return uIDs, err
	}

	res, err := m.Do(req)
	if err != nil {
		return uIDs, err
	}

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusBadRequest {
		return uIDs, newError(res.Body)
	}

	if res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusAccepted {
		err := json.NewDecoder(res.Body).Decode(&uIDs)
		if err != nil {
			return uIDs, err
		}
		return uIDs, res.Body.Close()
	}

	return uIDs, nil
}
