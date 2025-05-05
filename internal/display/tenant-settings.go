package display

import "github.com/auth0/go-auth0/management"

func (r *Renderer) SettingShow(tenant *management.Tenant) {
	r.Heading("tenant")
}
