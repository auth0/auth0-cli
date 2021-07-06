package auth0

import "gopkg.in/auth0.v5/management"

type UserAPI interface {
	// Retrieves a list of blocked IP addresses of a particular user.
	Blocks(id string, opts ...management.RequestOption) ([]*management.UserBlock, error)

	// Unblock a user that was blocked due to an excessive amount of incorrectly
	// provided credentials.
	Unblock(id string, opts ...management.RequestOption) error

	// Create a new user.
	Create(u *management.User, opts ...management.RequestOption) (err error)

	// Read user details for a given user.
	Read(id string, opts ...management.RequestOption) (u *management.User, err error)

	// Update user.
	Update(id string, u *management.User, opts ...management.RequestOption) (err error)

	// Delete a user.
	Delete(id string, opts ...management.RequestOption) (err error)

	// List all users.
	List(opts ...management.RequestOption) (ul *management.UserList, err error)

	// Search for users
	Search(opts ...management.RequestOption) (us *management.UserList, err error)
}
