//go:generate mockgen -source=network_acl_key.go -destination=mock/network_acl_key_mock.go -package=mock

package auth0

import (
	"context"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/option"
)

// NetworkACLKeyAPIV3 is the V3 SDK interface for the /keys/network-acls endpoint.
//
// Only List is exposed today: the network-acl command uses it to let a user pick
// existing signing keys by name when adding the http_message_signature signal to a
// rule. Key create/delete is intentionally not wired yet (DXCDT-2269).
type NetworkACLKeyAPIV3 interface {
	// List retrieves all Network ACL keys for the tenant.
	//
	// Required scope: `read:network_acl_keys`. The response is not paginated.
	List(ctx context.Context, opts ...option.RequestOption) (*managementv3.GetAllKeysNetworkACLsResponseContent, error)
}
