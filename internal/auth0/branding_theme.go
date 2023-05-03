//go:generate mockgen -source=branding_theme.go -destination=mock/branding_theme_mock.go -package=mock

package auth0

import "github.com/auth0/go-auth0/management"

type BrandingThemeAPI interface {
	Default(opts ...management.RequestOption) (theme *management.BrandingTheme, err error)
	Create(theme *management.BrandingTheme, opts ...management.RequestOption) (err error)
	Read(id string, opts ...management.RequestOption) (theme *management.BrandingTheme, err error)
	Update(id string, theme *management.BrandingTheme, opts ...management.RequestOption) (err error)
	Delete(id string, opts ...management.RequestOption) (err error)
}
