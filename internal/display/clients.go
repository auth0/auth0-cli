package display

import (
	"github.com/auth0/auth0-cli/internal/ansi"
	"gopkg.in/auth0.v5"
	"gopkg.in/auth0.v5/management"
)

type clientView struct {
	Name     string
	Type     string
	ClientID string
}

func (v *clientView) AsTableHeader() []string {
	return []string{"Name", "Type", "ClientID"}
}

func (v *clientView) AsTableRow() []string {
	return []string{v.Name, v.Type, ansi.Faint(v.ClientID)}
}

func (r *Renderer) ClientList(clients []*management.Client) {
	r.Heading(ansi.Bold(r.Tenant), "clients\n")

	var res []View
	for _, c := range clients {
		if auth0.StringValue(c.Name) == deprecatedAppName {
			continue
		}
		res = append(res, &clientView{
			Name:     auth0.StringValue(c.Name),
			Type:     appTypeFor(c.AppType),
			ClientID: auth0.StringValue(c.ClientID),
		})

	}

	r.Results(res)
}

func (r *Renderer) ClientCreate(client *management.Client) {
	r.Heading(ansi.Bold(r.Tenant), "client created\n")

	v := &clientView{
		Name:     auth0.StringValue(client.Name),
		Type:     appTypeFor(client.AppType),
		ClientID: auth0.StringValue(client.ClientID),
	}

	r.Results([]View{v})
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
