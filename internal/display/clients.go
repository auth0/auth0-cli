package display

import (
	"fmt"

	"github.com/auth0/auth0-cli/internal/ansi"
	"gopkg.in/auth0.v5"
	"gopkg.in/auth0.v5/management"
)

func (r *Renderer) ClientList(clients []*management.Client) {
	r.Heading(ansi.Bold(r.Tenant), "clients")

	for _, c := range clients {
		fmt.Fprintf(r.Writer, "- %s (%s)\n", auth0.StringValue(c.Name), appTypeFor(c.AppType))
		fmt.Fprintf(r.Writer, "  %s: %s\n", ansi.Italic("ClientID"), ansi.Faint(auth0.StringValue(c.ClientID)))
	}
}

func appTypeFor(v *string) string {
	switch {
	case v == nil:
		return "generic"

	case *v == "non_interactive":
		return "machine to machine"

	case *v == "native":
		return "native"

	case *v == "spa":
		return "single page application"

	case *v == "regular_web":
		return "regular web application"

	default:
		return *v
	}
}
