package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
)

type ClientGrantAPI interface {
	// Create a client grant.
	Create(ctx context.Context, g *management.ClientGrant, opts ...management.RequestOption) error

	// List all client grants.
	List(ctx context.Context, opts ...management.RequestOption) (*management.ClientGrantList, error)
}
