//go:generate mockgen -source=client.go -destination=client_mock.go -package=auth0

package auth0

import "gopkg.in/auth0.v5/management"

type ClientAPI interface {
	// Create a new client application.
	Create(c *management.Client, opts ...management.RequestOption) (err error)

	// Read a client by its id.
	Read(id string, opts ...management.RequestOption) (c *management.Client, err error)

	// List all client applications.
	List(opts ...management.RequestOption) (c *management.ClientList, err error)

	// Update a client.
	Update(id string, c *management.Client, opts ...management.RequestOption) (err error)

	// RotateSecret rotates a client secret.
	RotateSecret(id string, opts ...management.RequestOption) (c *management.Client, err error)

	// Delete a client and all its related assets (like rules, connections, etc)
	// given its id.
	Delete(id string, opts ...management.RequestOption) error
}
