//go:generate mockgen -source=user.go -destination=user_mock.go -package=auth0

package auth0

import "gopkg.in/auth0.v5/management"

type UserAPI interface {
	// Retrieves a list of blocked IP addresses of a particular user.
	Blocks(id string, opts ...management.RequestOption) ([]*management.UserBlock, error)

	// Unblock a user that was blocked due to an excessive amount of incorrectly
	// provided credentials.
	Unblock(id string, opts ...management.RequestOption) error
}
