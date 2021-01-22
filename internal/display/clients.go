package display

import (
	"github.com/auth0/auth0-cli/internal/ansi"
	"gopkg.in/auth0.v5"
	"gopkg.in/auth0.v5/management"
)

func (r *Renderer) ClientList(clients []*management.Client) {
	r.Heading(ansi.Bold(r.Tenant), "clients\n")

	var rows [][]string
	for _, c := range clients {
		if auth0.StringValue(c.Name) == deprecatedAppName {
			continue
		}
		rows = append(rows, []string{
			auth0.StringValue(c.Name),
			appTypeFor(c.AppType),
			ansi.Faint(auth0.StringValue(c.ClientID)),
		})
	}
	r.Table([]string{"Name", "Type", "ClientID"}, rows)
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
