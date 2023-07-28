//go:generate mockgen -source=tenant.go -destination=mock/tenant_mock.go -package=mock

package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
)

type TenantAPI interface {
	Read(ctx context.Context, opts ...management.RequestOption) (t *management.Tenant, err error)
}
