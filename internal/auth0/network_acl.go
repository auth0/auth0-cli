//go:generate mockgen -source=network_acl.go -destination=mock/network_acl_mock.go -package=mock

package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
)

type NetworkACLAPI interface {
	// Create a new Network ACL.
	Create(ctx context.Context, n *management.NetworkACL, opts ...management.RequestOption) error

	// Read Network ACL details.
	Read(ctx context.Context, id string, opts ...management.RequestOption) (n *management.NetworkACL, err error)

	// Update an existing Network ACL.
	Update(ctx context.Context, id string, n *management.NetworkACL, opts ...management.RequestOption) error

	// Patch an existing Network ACL.
	Patch(ctx context.Context, id string, n *management.NetworkACL, opts ...management.RequestOption) error

	// Delete a Network ACL.
	Delete(ctx context.Context, id string, opts ...management.RequestOption) error

	// List Network ACLs.
	List(ctx context.Context, opts ...management.RequestOption) (n []*management.NetworkACL, err error)
}
