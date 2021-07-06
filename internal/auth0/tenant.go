package auth0

import "gopkg.in/auth0.v5/management"

type TenantAPI interface {
	Read(opts ...management.RequestOption) (t *management.Tenant, err error)
}
