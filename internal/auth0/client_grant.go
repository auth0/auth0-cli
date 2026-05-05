package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
)

type ClientGrantAPI interface {
	// Create a new client grant, authorizing the given client for the specified API (audience).
	// Returns an error if the grant already exists or the request fails.
	Create(ctx context.Context, g *management.ClientGrant, opts ...management.RequestOption) error

	// List returns all client grants for the tenant, with optional filtering via opts.
	List(ctx context.Context, opts ...management.RequestOption) (*management.ClientGrantList, error)
}
