//go:generate mockgen -source=resource_server.go -destination=resource_server_mock.go -package=auth0

package auth0

import "gopkg.in/auth0.v5/management"

type ResourceServerAPI interface {
	// Create a resource server.
	Create(rs *management.ResourceServer, opts ...management.RequestOption) (err error)

	// Read retrieves a resource server by its id or audience.
	Read(id string, opts ...management.RequestOption) (rs *management.ResourceServer, err error)

	// Update a resource server.
	Update(id string, rs *management.ResourceServer, opts ...management.RequestOption) (err error)

	// Delete a resource server.
	Delete(id string, opts ...management.RequestOption) (err error)

	// List all resource server.
	List(opts ...management.RequestOption) (rl *management.ResourceServerList, err error)

	// Stream is a helper method which handles pagination
	Stream(fn func(s *management.ResourceServer), opts ...management.RequestOption) error
}
