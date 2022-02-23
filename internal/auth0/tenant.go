package auth0

import "github.com/auth0/go-auth0/management"

type TenantAPI interface {
	Read(opts ...management.RequestOption) (t *management.Tenant, err error)
}
