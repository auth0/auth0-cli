package display

import (
	"fmt"
	"github.com/auth0/auth0-cli/internal/ansi"
)

func (r *Renderer) promptsPartialsAction(partialJSON, segment, action string) {
	r.Heading(
		fmt.Sprintf(
			"partials for prompt (%s) %s",
			ansi.Bold(segment),
			action,
		),
	)
	r.Output(ansi.ColorizeJSON(partialJSON))
	r.Newline()
}

func (r *Renderer) PromptsPartialsShow(partialJSON, segment string) {
	r.promptsPartialsAction(partialJSON, segment, "viewed")
}

func (r *Renderer) PromptsPartialsCreate(partialJSON, segment string) {
	r.promptsPartialsAction(partialJSON, segment, "created")
}

func (r *Renderer) PromptsPartialsUpdate(partialJSON, segment string) {
	r.promptsPartialsAction(partialJSON, segment, "updated")
}

func (r *Renderer) PromptsPartialsDelete(partialJSON, segment string) {
	r.promptsPartialsAction(partialJSON, segment, "deleted")
}
