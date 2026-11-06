//go:generate go tool mockgen -source=client_connection_v3.go -destination=mock/client_connection_v3_mock.go -package=mock

package auth0

import (
	"context"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"
)

// ClientConnectionPage aliases the checkpoint-paginated list of the connections
// enabled for a client. The alias keeps the interface return type a single
// identifier so mockgen's source parser can handle it.
type ClientConnectionPage = core.Page[*string, *managementv3.ConnectionForList, *managementv3.ListClientConnectionsResponseContent]

// ClientConnectionAPIV3 is the V3 SDK interface for the /clients/{id}/connections
// endpoint, used to read which connections are enabled for a client.
type ClientConnectionAPIV3 interface {
	// Get the connections enabled for a client. The request must not be nil.
	//
	// Required scope: `read:connections`.
	Get(
		ctx context.Context,
		id string,
		request *managementv3.ConnectionsGetRequest,
		opts ...option.RequestOption,
	) (*ClientConnectionPage, error)
}
