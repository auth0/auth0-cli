package display

import (
	"fmt"
	"strings"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"gopkg.in/auth0.v5/management"
)

const (
	quickstartsNative     = "https://auth0.com/docs/quickstart/native"
	quickstartsSPA        = "https://auth0.com/docs/quickstart/spa"
	quickstartsRegularWeb = "https://auth0.com/docs/quickstart/webapp"
	quickstartsM2M        = "https://auth0.com/docs/quickstart/backend"
	quickstartsGeneric    = "https://auth0.com/docs/quickstarts"
)

type clientView struct {
	Name         string
	Type         string
	ClientID     string
	ClientSecret string
	Callbacks    []string
	revealSecret bool
}

func (v *clientView) AsTableHeader() []string {
	if v.revealSecret {
		return []string{"Name", "Type", "ClientID", "Client Secret", "Callbacks"}
	}
	return []string{"Name", "Type", "Client ID", "Callbacks"}

}

func (v *clientView) AsTableRow() []string {
	if v.revealSecret {
		return []string{
			v.Name,
			v.Type,
			ansi.Faint(v.ClientID),
			ansi.Italic(v.ClientSecret),
			strings.Join(v.Callbacks, ", "),
		}
	}
	return []string{
		v.Name,
		v.Type,
		ansi.Faint(v.ClientID),
		strings.Join(v.Callbacks, ", "),
	}

}

// listClientView is a slimmed down view of a client for displaying
// larger numbers of clients
type listClientView struct {
	Name         string
	Type         string
	ClientID     string
	ClientSecret string
	revealSecret bool
}

func (v *listClientView) AsTableHeader() []string {
	if v.revealSecret {
		return []string{"Name", "Type", "ClientID", "Client Secret"}
	}
	return []string{"Name", "Type", "Client ID"}

}

func (v *listClientView) AsTableRow() []string {
	if v.revealSecret {
		return []string{
			v.Name,
			v.Type,
			ansi.Faint(v.ClientID),
			ansi.Italic(v.ClientSecret),
		}
	}
	return []string{
		v.Name,
		v.Type,
		ansi.Faint(v.ClientID),
	}

}

func (r *Renderer) ClientList(clients []*management.Client) {
	r.Heading(ansi.Bold(r.Tenant), "clients\n")
	var res []View
	for _, c := range clients {
		if auth0.StringValue(c.Name) == deprecatedAppName {
			continue
		}
		res = append(res, &listClientView{
			Name:         auth0.StringValue(c.Name),
			Type:         appTypeFor(c.AppType),
			ClientID:     auth0.StringValue(c.ClientID),
			ClientSecret: auth0.StringValue(c.ClientSecret),
		})
	}

	r.Results(res)
}

func (r *Renderer) ClientCreate(client *management.Client, revealSecrets bool) {
	r.Heading(ansi.Bold(r.Tenant), "client created\n")

	// note(jfatta): list and create uses the same view for now,
	// eventually we might want to show different columns for each command:
	v := &clientView{
		revealSecret: revealSecrets,
		Name:         auth0.StringValue(client.Name),
		Type:         appTypeFor(client.AppType),
		ClientID:     auth0.StringValue(client.ClientID),
		ClientSecret: auth0.StringValue(client.ClientSecret),
		Callbacks:    callbacksFor(client.Callbacks),
	}

	r.Results([]View{v})

	r.Infof("\nQuickstarts: %s", quickstartsURIFor(client.AppType))
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

func quickstartsURIFor(v *string) string {
	switch {
	case *v == "native":
		return quickstartsNative
	case *v == "spa":
		return quickstartsSPA
	case *v == "regular_web":
		return quickstartsRegularWeb
	case *v == "non_interactive":
		return quickstartsM2M
	default:
		return quickstartsGeneric
	}
}

func callbacksFor(s []interface{}) []string {
	res := make([]string, len(s))
	for i, v := range s {
		res[i] = fmt.Sprintf("%s", v)
	}
	return res
}
