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
	friendlyM2M           = "Machine to Machine"
	friendlyNative        = "Native"
	friendlySpa           = "Single Page Web Application"
	friendlyReg           = "Regular Web Application"
)

type applicationView struct {
	Name              string
	Description       string
	Type              string
	ClientID          string
	ClientSecret      string
	Callbacks         []string
	AllowedOrigins    []string
	AllowedWebOrigins []string
	AllowedLogoutURLs []string
	AuthMethod        string
	Grants            []string
	revealSecret      bool

	raw interface{}
}

func (v *applicationView) AsTableHeader() []string {
	if v.revealSecret {
		return []string{"Client ID", "Name", "Type", "Client Secret"}
	}
	return []string{"Client ID", "Name", "Type"}
}

func (v *applicationView) AsTableRow() []string {
	if v.revealSecret {
		return []string{
			ansi.Faint(v.ClientID),
			v.Name,
			applyColor(v.Type),
			ansi.Italic(v.ClientSecret),
		}
	}
	return []string{
		ansi.Faint(v.ClientID),
		v.Name,
		applyColor(v.Type),
	}
}

func (v *applicationView) KeyValues() [][]string {
	callbacks := strings.Join(v.Callbacks, ", ")
	allowedOrigins := strings.Join(v.AllowedOrigins, ", ")
	allowedWebOrigins := strings.Join(v.AllowedWebOrigins, ", ")
	allowedLogoutURLs := strings.Join(v.AllowedLogoutURLs, ", ")
	grants := strings.Join(v.Grants, ", ")

	if v.revealSecret {
		return [][]string{
			{"CLIENT ID", ansi.Faint(v.ClientID)},
			{"NAME", v.Name},
			{"DESCRIPTION", v.Description},
			{"TYPE", applyColor(v.Type)},
			{"CLIENT SECRET", ansi.Italic(v.ClientSecret)},
			{"CALLBACKS", callbacks},
			{"ALLOWED LOGOUT URLS", allowedLogoutURLs},
			{"ALLOWED ORIGINS", allowedOrigins},
			{"ALLOWED WEB ORIGINS", allowedWebOrigins},
			{"TOKEN ENDPOINT AUTH", v.AuthMethod},
			{"GRANTS", grants},
		}
	}

	return [][]string{
		{"CLIENT ID", ansi.Faint(v.ClientID)},
		{"NAME", v.Name},
		{"DESCRIPTION", v.Description},
		{"TYPE", applyColor(v.Type)},
		{"CALLBACKS", callbacks},
		{"ALLOWED LOGOUT URLS", allowedLogoutURLs},
		{"ALLOWED ORIGINS", allowedOrigins},
		{"ALLOWED WEB ORIGINS", allowedWebOrigins},
		{"TOKEN ENDPOINT AUTH", v.AuthMethod},
		{"GRANTS", grants},
	}
}

func (v *applicationView) Object() interface{} {
	return safeRaw(v.raw.(*management.Client), v.revealSecret)
}

func (r *Renderer) ApplicationList(clients []*management.Client, revealSecrets bool) {
	resource := "applications"

	r.Heading(resource)

	if len(clients) == 0 {
		r.EmptyState(resource)
		r.Infof("Use 'auth0 apps create' to add one")
		return
	}

	var res []View
	for _, c := range clients {
		if auth0.StringValue(c.Name) == deprecatedAppName {
			continue
		}

		if !revealSecrets {
			c.ClientSecret = auth0.String("")
		}

		res = append(res, makeApplicationView(c, revealSecrets))
	}

	r.Results(res)
}

func (r *Renderer) ApplicationShow(client *management.Client, revealSecrets bool) {
	r.Heading("application")
	r.Result(makeApplicationView(client, revealSecrets))
}

func (r *Renderer) ApplicationCreate(client *management.Client, revealSecrets bool) {
	r.Heading("application created")

	if !revealSecrets {
		client.ClientSecret = auth0.String("")
	}

	r.Result(makeApplicationView(client, revealSecrets))
	r.Newline()
	r.Infof("Quickstarts: %s", quickstartsURIFor(client.AppType))

	// TODO(cyx): possibly guard this with a --no-hint flag.
	r.Infof("%s Test this app's login box with 'auth0 test login %s'",
		ansi.Faint("Hint:"),
		client.GetClientID(),
	)
	r.Infof("%s You might wanna try 'auth0 quickstarts download %s'",
		ansi.Faint("Hint:"),
		client.GetClientID(),
	)
}

func (r *Renderer) ApplicationUpdate(client *management.Client, revealSecrets bool) {
	r.Heading("application updated")

	if !revealSecrets {
		client.ClientSecret = auth0.String("")
	}

	r.Result(makeApplicationView(client, revealSecrets))
}

func makeApplicationView(client *management.Client, revealSecrets bool) *applicationView {
	return &applicationView{
		revealSecret:      revealSecrets,
		Name:              auth0.StringValue(client.Name),
		Description:       auth0.StringValue(client.Description),
		Type:              appTypeFor(client.AppType),
		ClientID:          auth0.StringValue(client.ClientID),
		ClientSecret:      auth0.StringValue(client.ClientSecret),
		Callbacks:         interfaceSliceToString(client.Callbacks),
		AllowedOrigins:    interfaceSliceToString(client.AllowedOrigins),
		AllowedWebOrigins: interfaceSliceToString(client.WebOrigins),
		AllowedLogoutURLs: interfaceSliceToString(client.AllowedLogoutURLs),
		AuthMethod:        auth0.StringValue(client.TokenEndpointAuthMethod),
		Grants:            interfaceSliceToString(client.GrantTypes),
		raw:               client,
	}
}

// TODO(cyx): determine if there's a better way to filter this out.
const deprecatedAppName = "All Applications"

func appTypeFor(v *string) string {
	switch {
	case v == nil:
		return "Generic"

	case *v == "non_interactive":
		return friendlyM2M

	case *v == "native":
		return friendlyNative

	case *v == "spa":
		return friendlySpa

	case *v == "regular_web":
		return friendlyReg

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

func interfaceSliceToString(s []interface{}) []string {
	res := make([]string, len(s))
	for i, v := range s {
		res[i] = fmt.Sprintf("%s", v)
	}
	return res
}

func applyColor(a string) string {
	switch {
	case a == friendlyM2M:
		return ansi.Green(a)
	case a == friendlyNative:
		return ansi.Cyan(a)
	case a == friendlySpa:
		return ansi.Blue(a)
	case a == friendlyReg:
		return ansi.Magenta(a)
	default:
		return a
	}
}

func safeRaw(c *management.Client, revealSecrets bool) *management.Client {
	if revealSecrets {
		return c
	}

	c.ClientSecret = nil
	c.SigningKeys = nil
	return c
}
