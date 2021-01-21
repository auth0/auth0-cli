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
		if auth0.StringValue(c.Name) == deprecatedAppName {
			continue
		}

		fmt.Fprintf(r.Writer, "- %s (%s)\n", auth0.StringValue(c.Name), appTypeFor(c.AppType))
		fmt.Fprintf(r.Writer, "  client id: %s\n", ansi.Faint(auth0.StringValue(c.ClientID)))
		fmt.Fprintln(r.Writer)
	}
}

// TODO(cyx): determine if there's a better way to filter this out.
const deprecatedAppName = "All Applications"

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
