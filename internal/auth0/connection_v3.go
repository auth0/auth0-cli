//go:generate go tool mockgen -source=connection_v3.go -destination=mock/connection_v3_mock.go -package=mock

package auth0

import (
	"context"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"
)

// ConnectionForListPage aliases the checkpoint-paginated connections list
// response. The alias keeps the interface return type a single identifier so
// mockgen's source parser can handle it (it cannot parse the multi-type-parameter
// generic inline).
type ConnectionForListPage = core.Page[*string, *managementv3.ConnectionForList, *managementv3.ListConnectionsCheckpointPaginatedResponseContent]

// ConnectionEnabledClientPage aliases the checkpoint-paginated enabled-clients
// list response.
type ConnectionEnabledClientPage = core.Page[*string, *managementv3.ConnectionEnabledClient, *managementv3.GetConnectionEnabledClientsResponseContent]

// ConnectionAPIV3 is the V3 SDK interface for the /connections endpoint. Create,
// read, and update go through the raw HTTP client because a connection's options
// are a large per-strategy union that the typed request models would drop, so
// only paging and delete live here.
type ConnectionAPIV3 interface {
	// List connections.
	//
	// Required scope: `read:connections`.
	List(
		ctx context.Context,
		request *managementv3.ListConnectionsQueryParameters,
		opts ...option.RequestOption,
	) (*ConnectionForListPage, error)

	// Delete a connection.
	//
	// Required scope: `delete:connections`.
	Delete(
		ctx context.Context,
		id string,
		opts ...option.RequestOption,
	) error
}

// ConnectionEnabledClientAPIV3 is the V3 SDK interface for the
// /connections/{id}/clients endpoint, used to read and modify which clients have
// a connection enabled.
type ConnectionEnabledClientAPIV3 interface {
	// Get the clients that have this connection enabled.
	//
	// Required scope: `read:connections`.
	Get(
		ctx context.Context,
		id string,
		request *managementv3.GetConnectionEnabledClientsRequestParameters,
		opts ...option.RequestOption,
	) (*ConnectionEnabledClientPage, error)

	// Update the enabled clients for this connection.
	//
	// Required scope: `update:connections`.
	Update(
		ctx context.Context,
		id string,
		request managementv3.UpdateEnabledClientConnectionsRequestContent,
		opts ...option.RequestOption,
	) error
}
