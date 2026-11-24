//go:generate go tool mockgen -source=portal.go -destination=mock/portal_mock.go -package=mock

package auth0

import (
	"context"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/option"
)

// PortalAPIV3 is the V3 SDK interface for the /portals endpoint.
type PortalAPIV3 interface {
	// List portals. The endpoint does not page.
	//
	// Required scope: `read:portals`.
	List(
		ctx context.Context,
		opts ...option.RequestOption,
	) (*managementv3.ListPortalsResponseContent, error)

	// Create a portal.
	//
	// Required scope: `create:portals`.
	Create(
		ctx context.Context,
		request *managementv3.CreatePortalRequestContent,
		opts ...option.RequestOption,
	) (*managementv3.Portal, error)

	// Get a portal by its ID.
	//
	// Required scope: `read:portals`.
	Get(
		ctx context.Context,
		id managementv3.PortalID,
		opts ...option.RequestOption,
	) (*managementv3.Portal, error)

	// Delete a portal.
	//
	// Required scope: `delete:portals`.
	Delete(
		ctx context.Context,
		id managementv3.PortalID,
		opts ...option.RequestOption,
	) error

	// Update a portal. Only the parts of the request that are set change.
	//
	// Required scope: `update:portals`.
	Update(
		ctx context.Context,
		id managementv3.PortalID,
		request *managementv3.UpdatePortalRequestContent,
		opts ...option.RequestOption,
	) (*managementv3.Portal, error)
}
