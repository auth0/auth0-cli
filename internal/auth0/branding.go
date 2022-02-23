package auth0

import "github.com/auth0/go-auth0/management"

type BrandingAPI interface {
	Read(opts ...management.RequestOption) (b *management.Branding, err error)
	Update(t *management.Branding, opts ...management.RequestOption) (err error)
	UniversalLogin(opts ...management.RequestOption) (ul *management.BrandingUniversalLogin, err error)
	SetUniversalLogin(ul *management.BrandingUniversalLogin, opts ...management.RequestOption) (err error)
	DeleteUniversalLogin(opts ...management.RequestOption) (err error)
}
