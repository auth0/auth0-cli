package auth0

import "gopkg.in/auth0.v5/management"

type BrandingAPI interface {
	Read(opts ...management.RequestOption) (b *management.Branding, err error)
	Update(t *management.Branding, opts ...management.RequestOption) (err error)
	UniversalLogin(opts ...management.RequestOption) (ul *management.BrandingUniversalLogin, err error)
	SetUniversalLogin(ul *management.BrandingUniversalLogin, opts ...management.RequestOption) (err error)
	DeleteUniversalLogin(opts ...management.RequestOption) (err error)
}
