//go:generate go tool mockgen -source=client_v3.go -destination=mock/client_v3_mock.go -package=mock

package auth0

import (
	"context"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"
)

// ClientListPage is the offset-paginated response returned by the clients list
// endpoint. It is aliased here so the mock generator (which cannot parse
// instantiated generic types) sees a plain named type in the interface.
type ClientListPage = core.Page[*int, *managementv3.Client, *managementv3.ListClientsOffsetPaginatedResponseContent]

// ClientAPIV3 is the V3 SDK interface for the /clients endpoint.
type ClientAPIV3 interface {
	// List clients, optionally limiting the returned fields with fields and include_fields.
	//
	// Required scope: `read:clients`.
	List(
		ctx context.Context,
		request *managementv3.ListClientsRequestParameters,
		opts ...option.RequestOption,
	) (*ClientListPage, error)

	// Get a single client, optionally limiting the returned fields with fields and include_fields.
	//
	// Required scope: `read:clients`.
	Get(
		ctx context.Context,
		id string,
		request *managementv3.GetClientRequestParameters,
		opts ...option.RequestOption,
	) (*managementv3.GetClientResponseContent, error)

	// Update a client.
	//
	// Required scope: `update:clients`.
	Update(
		ctx context.Context,
		id string,
		request *managementv3.UpdateClientRequestContent,
		opts ...option.RequestOption,
	) (*managementv3.UpdateClientResponseContent, error)
}
