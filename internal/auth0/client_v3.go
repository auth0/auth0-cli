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

	// Delete a client.
	//
	// Required scope: `delete:clients`.
	Delete(
		ctx context.Context,
		id string,
		opts ...option.RequestOption,
	) error

	// PreviewCimdMetadata fetches and validates a Client ID Metadata Document
	// without creating a client.
	//
	// Required scope: `read:clients`.
	PreviewCimdMetadata(
		ctx context.Context,
		request *managementv3.PreviewCimdMetadataRequestContent,
		opts ...option.RequestOption,
	) (*managementv3.PreviewCimdMetadataResponseContent, error)

	// RegisterCimdClient creates a client from a Client ID Metadata Document, or
	// updates the client if one is already registered for it.
	//
	// Required scope: `create:clients` when a client is created, `update:clients`
	// when an existing client is updated.
	RegisterCimdClient(
		ctx context.Context,
		request *managementv3.RegisterCimdClientRequestContent,
		opts ...option.RequestOption,
	) (*managementv3.RegisterCimdClientResponseContent, error)
}

// ClientCredentialAPIV3 is the V3 SDK interface for the
// /clients/{client_id}/credentials endpoint.
type ClientCredentialAPIV3 interface {
	// List the credentials of a client.
	//
	// Required scope: `read:client_credentials`.
	List(
		ctx context.Context,
		clientID string,
		opts ...option.RequestOption,
	) ([]*managementv3.ClientCredential, error)
}
