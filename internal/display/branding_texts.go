package display

import (
	"fmt"

	"github.com/auth0/auth0-cli/internal/ansi"
)

func (r *Renderer) BrandingTextShow(brandingTextJSON, prompt, language string) {
	r.Heading(
		fmt.Sprintf(
			"custom text for prompt (%s) and language (%s)",
			ansi.Bold(prompt),
			ansi.Bold(language),
		),
	)
	r.Output(ansi.ColorizeJSON(brandingTextJSON))
	r.Newline()
}

func (r *Renderer) BrandingTextUpdate(brandingTextJSON string, prompt, language string) {
	r.Heading(
		fmt.Sprintf(
			"custom text for prompt (%s) and language (%s) updated",
			ansi.Bold(prompt),
			ansi.Bold(language),
		),
	)
	r.Output(ansi.ColorizeJSON(brandingTextJSON))
	r.Newline()
}
