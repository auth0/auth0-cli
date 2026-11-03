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

// ResourceServerAPIV3 is the V3 SDK interface for the eventually consistent
// resource-servers search endpoint (GET /resource-servers/search).
type ResourceServerAPIV3 interface {
	// Search resource servers using SCIM or Lucene filter syntax.
	Search(
		ctx context.Context,
		request *managementv3.SearchResourceServersRequestParameters,
		opts ...option.RequestOption,
	) (*core.Page[*string, *managementv3.ResourceServerSearchResponse, *managementv3.SearchResourceServersResponseContent], error)
}
