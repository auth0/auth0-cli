//go:generate mockgen -source=branding.go -destination=mock/branding_mock.go -package=mock

package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
)

type BrandingAPI interface {
	Read(ctx context.Context, opts ...management.RequestOption) (b *management.Branding, err error)
	Update(ctx context.Context, t *management.Branding, opts ...management.RequestOption) (err error)
	UniversalLogin(ctx context.Context, opts ...management.RequestOption) (ul *management.BrandingUniversalLogin, err error)
	SetUniversalLogin(ctx context.Context, ul *management.BrandingUniversalLogin, opts ...management.RequestOption) (err error)
	DeleteUniversalLogin(ctx context.Context, opts ...management.RequestOption) (err error)
}
