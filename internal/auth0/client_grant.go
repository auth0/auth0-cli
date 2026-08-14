//go:generate mockgen -source=client_grant.go -destination=mock/client_grant_mock.go -package=mock

package auth0

import (
	"context"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"
)

// ClientGrantPage is the paginated response returned by the client-grants list
// endpoint. It is aliased here so the mock generator (which cannot parse
// instantiated generic types) sees a plain named type in the interface.
type ClientGrantPage = core.Page[*string, *managementv3.ClientGrantResponseContent, *managementv3.ListClientGrantPaginatedResponseContent]

// ClientGrantAPIV3 is the interface for the /client-grants endpoint.
type ClientGrantAPIV3 interface {
	// List client grants, including the scopes associated with the application/API pair.
	//
	// Required scope: `read:client_grants`
	//
	// See: https://auth0.com/docs/api/management/v2/client-grants/get-client-grants
	List(
		ctx context.Context,
		request *managementv3.ListClientGrantsRequestParameters,
		opts ...option.RequestOption,
	) (*ClientGrantPage, error)

	// Create a client grant, authorizing a client for the specified API (audience).
	//
	// Required scope: `create:client_grants`
	//
	// See: https://auth0.com/docs/api/management/v2/client-grants/post-client-grants
	Create(
		ctx context.Context,
		request *managementv3.CreateClientGrantRequestContent,
		opts ...option.RequestOption,
	) (*managementv3.CreateClientGrantResponseContent, error)

	// Get a single client grant, including the scopes associated with the application/API pair.
	//
	// Required scope: `read:client_grants`
	//
	// See: https://auth0.com/docs/api/management/v2/client-grants/get-client-grants-by-id
	Get(
		ctx context.Context,
		id string,
		opts ...option.RequestOption,
	) (*managementv3.GetClientGrantResponseContent, error)

	// Update a client grant. The client_id and audience of a grant cannot be changed.
	//
	// Required scope: `update:client_grants`
	//
	// See: https://auth0.com/docs/api/management/v2/client-grants/patch-client-grants-by-id
	Update(
		ctx context.Context,
		id string,
		request *managementv3.UpdateClientGrantRequestContent,
		opts ...option.RequestOption,
	) (*managementv3.UpdateClientGrantResponseContent, error)

	// Delete a client grant.
	//
	// Required scope: `delete:client_grants`
	//
	// See: https://auth0.com/docs/api/management/v2/client-grants/delete-client-grants-by-id
	Delete(
		ctx context.Context,
		id string,
		opts ...option.RequestOption,
	) error
}

// ClientGrantOrganizationPage is the paginated response returned by the
// client-grant organizations endpoint. It is aliased here so the mock generator
// (which cannot parse instantiated generic types) sees a plain named type in the
// interface.
type ClientGrantOrganizationPage = core.Page[*string, *managementv3.Organization, *managementv3.ListClientGrantOrganizationsPaginatedResponseContent]

// ClientGrantOrganizationAPIV3 is the interface for the
// /client-grants/{id}/organizations endpoint.
type ClientGrantOrganizationAPIV3 interface {
	// List the organizations associated with a client grant.
	//
	// Required scope: `read:organization_client_grants`
	//
	// See: https://auth0.com/docs/api/management/v2/client-grants/get-client-grant-organizations
	List(
		ctx context.Context,
		id string,
		request *managementv3.ListClientGrantOrganizationsRequestParameters,
		opts ...option.RequestOption,
	) (*ClientGrantOrganizationPage, error)
}
