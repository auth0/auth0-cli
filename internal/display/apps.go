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
	friendlyM2M           = "machine to machine"
	friendlyNative        = "native"
	friendlySpa           = "single page application"
	friendlyReg           = "regular web application"
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
		return []string{
			"ClientID",
			"Description",
			"Name",
			"Type",
			"Client Secret",
			"Callbacks",
			"Allowed Origins",
			"Allowed Web Origins",
			"Allowed Logout URLs",
			"Token Endpoint Auth",
			"Grants",
		}
	}
	return []string{
		"Client ID",
		"Description",
		"Name",
		"Type",
		"Callbacks",
		"Allowed Origins",
		"Allowed Web Origins",
		"Allowed Logout URLs",
		"Token Endpoint Auth",
		"Grants",
	}
}

func (v *applicationView) AsTableRow() []string {
	if v.revealSecret {
		return []string{
			v.Name,
			applyColor(v.Type),
			ansi.Faint(v.ClientID),
			ansi.Italic(v.ClientSecret),
			strings.Join(v.Callbacks, ", "),
		}
	}
	return []string{
		v.Name,
		applyColor(v.Type),
		ansi.Faint(v.ClientID),
		strings.Join(v.Callbacks, ", "),
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
			[]string{"CLIENT ID", ansi.Faint(v.ClientID)},
			[]string{"NAME", v.Name},
			[]string{"DESCRIPTION", v.Description},
			[]string{"TYPE", applyColor(v.Type)},
			[]string{"CLIENT SECRET", ansi.Italic(v.ClientSecret)},
			[]string{"CALLBACKS", callbacks},
			[]string{"ALLOWED LOGOUT URLS", allowedLogoutURLs},
			[]string{"ALLOWED ORIGINS", allowedOrigins},
			[]string{"ALLOWED WEB ORIGINS", allowedWebOrigins},
			[]string{"TOKEN ENDPOINT AUTH", v.AuthMethod},
			[]string{"GRANTS", grants},
		}
	}

	return [][]string{
		[]string{"CLIENT ID", ansi.Faint(v.ClientID)},
		[]string{"NAME", v.Name},
		[]string{"DESCRIPTION", v.Description},
		[]string{"TYPE", applyColor(v.Type)},
		[]string{"CALLBACKS", callbacks},
		[]string{"ALLOWED LOGOUT URLS", allowedLogoutURLs},
		[]string{"ALLOWED ORIGINS", allowedOrigins},
		[]string{"ALLOWED WEB ORIGINS", allowedWebOrigins},
		[]string{"TOKEN ENDPOINT AUTH", v.AuthMethod},
		[]string{"GRANTS", grants},
	}
}

func (v *applicationView) Object() interface{} {
	return v.raw
}

// applicationListView is a slimmed down view of a client for displaying
// larger numbers of applications
type applicationListView struct {
	Name         string
	Type         string
	ClientID     string
	ClientSecret string
	revealSecret bool
}

func (v *applicationListView) AsTableHeader() []string {
	if v.revealSecret {
		return []string{"ClientID", "Name", "Type", "Client Secret"}
	}
	return []string{"Client ID", "Name", "Type"}
}

func (v *applicationListView) AsTableRow() []string {
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

func (r *Renderer) ApplicationList(clients []*management.Client) {
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
		res = append(res, &applicationListView{
			Name:         auth0.StringValue(c.Name),
			Type:         appTypeFor(c.AppType),
			ClientID:     auth0.StringValue(c.ClientID),
			ClientSecret: auth0.StringValue(c.ClientSecret),
		})
	}

	r.Results(res)
}

func (r *Renderer) ApplicationShow(client *management.Client, revealSecrets bool) {
	r.Heading("application")

	v := &applicationView{
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

	r.Result(v)
}

func (r *Renderer) ApplicationCreate(client *management.Client, revealSecrets bool) {
	r.Heading("application created")

	v := &applicationView{
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

	r.Result(v)

	r.Newline()
	r.Infof("Quickstarts: %s", quickstartsURIFor(client.AppType))

	// TODO(cyx): possibly guard this with a --no-hint flag.
	r.Infof("%s You might wanna try 'auth0 test login --client-id %s'",
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

	v := &applicationView{
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

	r.Result(v)
}

// TODO(cyx): determine if there's a better way to filter this out.
const deprecatedAppName = "All Applications"

func appTypeFor(v *string) string {
	switch {
	case v == nil:
		return "generic"

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
