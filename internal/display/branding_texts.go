package display

import "github.com/auth0/auth0-cli/internal/ansi"

func (r *Renderer) BrandingTextShow(b string) {
	r.Heading("custom texts")
	r.Output(ansi.ColorizeJSON(b, false))
}

func (r *Renderer) BrandingTextUpdate(b string) {
	r.Heading("custom texts updated")
	r.Output(ansi.ColorizeJSON(b, false))
}
