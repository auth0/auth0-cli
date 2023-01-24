package auth0

import "github.com/auth0/go-auth0/management"

type ClientGrantAPI interface {
	// List all client grants.
	//
	// This method forces the `include_totals=true` and defaults to `per_page=50` if
	// not provided.
	//
	// See: https://auth0.com/docs/api/management/v2#!/Client_Grants/get_client_grants
	List(opts ...management.RequestOption) (gs *management.ClientGrantList, err error)
}
