//go:generate mockgen -source=branding_theme.go -destination=mock/branding_theme_mock.go -package=mock

package auth0

import (
	"context"

	"github.com/auth0/go-auth0/management"
)

type BrandingThemeAPI interface {
	Default(ctx context.Context, opts ...management.RequestOption) (theme *management.BrandingTheme, err error)
	Create(ctx context.Context, theme *management.BrandingTheme, opts ...management.RequestOption) (err error)
	Read(ctx context.Context, id string, opts ...management.RequestOption) (theme *management.BrandingTheme, err error)
	Update(ctx context.Context, id string, theme *management.BrandingTheme, opts ...management.RequestOption) (err error)
	Delete(ctx context.Context, id string, opts ...management.RequestOption) (err error)
}
