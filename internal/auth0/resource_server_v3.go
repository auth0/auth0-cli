//go:generate go tool mockgen -source=resource_server_v3.go -destination=mock/resource_server_v3_mock.go -package=mock

package auth0

import (
	"context"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"
)

// ResourceServerSearchPage is one cursor-paginated page of resource-servers search results.
type ResourceServerSearchPage = core.Page[*string, *managementv3.ResourceServerSearchResponse, *managementv3.SearchResourceServersResponseContent]

// ResourceServerPage is the offset-paginated response returned by the
// resource-servers list endpoint. It is aliased here so the mock generator
// (which cannot parse instantiated generic types) sees a plain named type in the
// interface.
type ResourceServerPage = core.Page[*int, *managementv3.ResourceServer, *managementv3.ListResourceServerOffsetPaginatedResponseContent]

// ResourceServerAPIV3 is the V3 SDK interface for the /resource-servers
// endpoint, including the eventually consistent search endpoint
// (GET /resource-servers/search).
type ResourceServerAPIV3 interface {
	// List resource servers, optionally filtered by identifiers.
	//
	// Required scope: `read:resource_servers`.
	List(
		ctx context.Context,
		request *managementv3.ListResourceServerRequestParameters,
		opts ...option.RequestOption,
	) (*ResourceServerPage, error)

	// Update a resource server by ID or audience.
	//
	// Required scope: `update:resource_servers`.
	Update(
		ctx context.Context,
		id string,
		request *managementv3.UpdateResourceServerRequestContent,
		opts ...option.RequestOption,
	) (*managementv3.UpdateResourceServerResponseContent, error)

	// Search resource servers using SCIM or Lucene filter syntax.
	//
	// Required scope: `read:resource_servers`.
	Search(
		ctx context.Context,
		request *managementv3.SearchResourceServersRequestParameters,
		opts ...option.RequestOption,
	) (*core.Page[*string, *managementv3.ResourceServerSearchResponse, *managementv3.SearchResourceServersResponseContent], error)
}
