//go:generate go tool mockgen -source=client_v3.go -destination=mock/client_v3_mock.go -package=mock

package auth0

import (
	"context"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"
)

// ClientSearchPage is one cursor-paginated page of clients search results. It is
// aliased here so the mock generator (which cannot parse instantiated generic types)
// sees a plain named type in the interface.
type ClientSearchPage = core.Page[*string, *managementv3.ClientSearchResponse, *managementv3.SearchClientsResponseContent]

// ClientAPIV3 is the V3 SDK interface for the eventually consistent
// clients search endpoint (GET /clients/search).
type ClientAPIV3 interface {
	// Search clients using SCIM or Lucene filter syntax.
	//
	// Required scope: `read:clients` or `read:client_summary`
	//
	// See: https://auth0.com/docs/api/management/v2/clients/get-clients-search
	Search(
		ctx context.Context,
		request *managementv3.SearchClientsRequestParameters,
		opts ...option.RequestOption,
	) (*ClientSearchPage, error)
}
