package display

import (
	"fmt"
	"io"
	"strings"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/cyx/auth0/management"
)

type Renderer struct {
	Tenant string

	Writer io.Writer
}

func (r *Renderer) ActionList(actions []*management.Action) {
	r.Heading(ansi.Bold(r.Tenant), "actions")

	for _, a := range actions {
		fmt.Fprintf(r.Writer, "%s\n", a.Name)
	}
}

func (r *Renderer) Heading(text ...string) {
	fmt.Fprintf(r.Writer, "%s %s\n", ansi.Faint("==="), strings.Join(text, " "))
}
