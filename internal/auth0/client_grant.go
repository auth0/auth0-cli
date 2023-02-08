package auth0

import "github.com/auth0/go-auth0/management"

type ClientGrantAPI interface {
	// List all client grants.
	List(opts ...management.RequestOption) (*management.ClientGrantList, error)
}
