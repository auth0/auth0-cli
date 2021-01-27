//go:generate mockgen -source=user.go -destination=user_mock.go -package=auth0

package auth0

import "gopkg.in/auth0.v5/management"

type UserAPI interface {
	// Read a user by its id.
	Read(id string, opts ...management.RequestOption) (u *management.User, err error)

	// List users by email.
	ListByEmail(email string, opts ...management.RequestOption) (us []*management.User, err error)
}
