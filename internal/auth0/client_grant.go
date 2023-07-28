package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
)

type ClientGrantAPI interface {
	// List all client grants.
	List(ctx context.Context, opts ...management.RequestOption) (*management.ClientGrantList, error)
}
