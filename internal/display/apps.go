package display

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/auth0/go-auth0/management"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
)

const (
	quickstartsNative      = "https://auth0.com/docs/quickstart/native"
	quickstartsSPA         = "https://auth0.com/docs/quickstart/spa"
	quickstartsRegularWeb  = "https://auth0.com/docs/quickstart/webapp"
	quickstartsM2M         = "https://auth0.com/docs/quickstart/backend"
	quickstartsGeneric     = "https://auth0.com/docs/quickstarts"
	friendlyM2M            = "Machine to Machine"
	friendlyNative         = "Native"
	friendlySpa            = "Single Page Web Application"
	friendlyReg            = "Regular Web Application"
	friendlyResourceServer = "Resource Server"
)

type applicationView struct {
	Name                     string
	Description              string
	Type                     string
	ClientID                 string
	ClientSecret             string
	Callbacks                []string
	AllowedOrigins           []string
	AllowedWebOrigins        []string
	AllowedLogoutURLs        []string
	AuthMethod               string
	Grants                   []string
	Metadata                 []string
	RefreshToken             string
	ResourceServerIdentifier string
	revealSecret             bool

	raw interface{}
}

func (v *applicationView) AsTableHeader() []string {
	if v.revealSecret {
		return []string{"Client ID", "Name", "Type", "Client Secret", "Resource Server"}
	}
	return []string{"Client ID", "Name", "Type", "Resource Server"}
}

func (v *applicationView) AsTableRow() []string {
	// Show resource server identifier if present, otherwise empty.
	resourceServerDisplay := ""
	if v.ResourceServerIdentifier != "" {
		resourceServerDisplay = v.ResourceServerIdentifier
	}

	if v.revealSecret {
		return []string{
			ansi.Faint(v.ClientID),
			v.Name,
			ApplyColorToFriendlyAppType(v.Type),
			ansi.Italic(v.ClientSecret),
			resourceServerDisplay,
		}
	}
	return []string{
		ansi.Faint(v.ClientID),
		v.Name,
		ApplyColorToFriendlyAppType(v.Type),
		resourceServerDisplay,
	}
}

func (v *applicationView) KeyValues() [][]string {
	callbacks := strings.Join(v.Callbacks, ", ")
	allowedOrigins := strings.Join(v.AllowedOrigins, ", ")
	allowedWebOrigins := strings.Join(v.AllowedWebOrigins, ", ")
	allowedLogoutURLs := strings.Join(v.AllowedLogoutURLs, ", ")
	grants := strings.Join(v.Grants, ", ")
	metadata := strings.Join(v.Metadata, ", ")

	var keyValues [][]string

	if v.revealSecret {
		keyValues = [][]string{
			{"CLIENT ID", ansi.Faint(v.ClientID)},
			{"NAME", v.Name},
			{"DESCRIPTION", v.Description},
			{"TYPE", ApplyColorToFriendlyAppType(v.Type)},
			{"CLIENT SECRET", ansi.Italic(v.ClientSecret)},
			{"CALLBACKS", callbacks},
			{"ALLOWED LOGOUT URLS", allowedLogoutURLs},
			{"ALLOWED ORIGINS", allowedOrigins},
			{"ALLOWED WEB ORIGINS", allowedWebOrigins},
			{"TOKEN ENDPOINT AUTH", v.AuthMethod},
			{"GRANTS", grants},
			{"METADATA", metadata},
			{"REFRESH TOKEN", v.RefreshToken},
		}
	} else {
		keyValues = [][]string{
			{"CLIENT ID", ansi.Faint(v.ClientID)},
			{"NAME", v.Name},
			{"DESCRIPTION", v.Description},
			{"TYPE", ApplyColorToFriendlyAppType(v.Type)},
			{"CALLBACKS", callbacks},
			{"ALLOWED LOGOUT URLS", allowedLogoutURLs},
			{"ALLOWED ORIGINS", allowedOrigins},
			{"ALLOWED WEB ORIGINS", allowedWebOrigins},
			{"TOKEN ENDPOINT AUTH", v.AuthMethod},
			{"GRANTS", grants},
			{"METADATA", metadata},
			{"REFRESH TOKEN", v.RefreshToken},
		}
	}

	// Add resource server identifier if present.
	if v.ResourceServerIdentifier != "" {
		keyValues = append(keyValues, []string{"RESOURCE SERVER IDENTIFIER", v.ResourceServerIdentifier})
	}

	return keyValues
}

func (v *applicationView) Object() interface{} {
	return safeRaw(v.raw.(*management.Client), v.revealSecret)
}

func (r *Renderer) ApplicationList(clients []*management.Client, revealSecrets bool) {
	resource := "applications"

	r.Heading(fmt.Sprintf("%s (%v)", resource, len(clients)))

	if len(clients) == 0 {
		r.EmptyState(resource, "Use 'auth0 apps create' to add one")
		return
	}

	var res []View
	for _, c := range clients {
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
	r.Infof("Quickstarts: %s", quickstartsURIFor(client.GetAppType()))

	// TODO(cyx): possibly guard this with a --no-hint flag.
	r.Infof("%s Emulate this app's login flow by running `auth0 test login %s`",
		ansi.Faint("Hint:"),
		client.GetClientID(),
	)
	r.Infof("%s Consider running `auth0 quickstarts download %s`",
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
	jsonRefreshToken, _ := json.Marshal(client.GetRefreshToken())

	return &applicationView{
		revealSecret:             revealSecrets,
		Name:                     client.GetName(),
		Description:              client.GetDescription(),
		Type:                     FriendlyAppType(client.GetAppType()),
		ClientID:                 client.GetClientID(),
		ClientSecret:             client.GetClientSecret(),
		Callbacks:                client.GetCallbacks(),
		AllowedOrigins:           client.GetAllowedOrigins(),
		AllowedWebOrigins:        client.GetWebOrigins(),
		AllowedLogoutURLs:        client.GetAllowedLogoutURLs(),
		AuthMethod:               client.GetTokenEndpointAuthMethod(),
		Grants:                   client.GetGrantTypes(),
		Metadata:                 mapPointerToArray(client.ClientMetadata),
		ResourceServerIdentifier: client.GetResourceServerIdentifier(),
		raw:                      client,
		RefreshToken:             string(jsonRefreshToken),
	}
}

func FriendlyAppType(appType string) string {
	switch {
	case appType == "":
		return "Generic"
	case appType == "non_interactive":
		return friendlyM2M
	case appType == "native":
		return friendlyNative
	case appType == "spa":
		return friendlySpa
	case appType == "regular_web":
		return friendlyReg
	case appType == "resource_server":
		return friendlyResourceServer
	default:
		return appType
	}
}

func mapPointerToArray(m *map[string]interface{}) []string {
	var result []string
	if m != nil {
		for k, v := range *m {
			result = append(result, fmt.Sprintf("%s=%v", k, v))
		}
	}
	return result
}

func quickstartsURIFor(appType string) string {
	switch {
	case appType == "native":
		return quickstartsNative
	case appType == "spa":
		return quickstartsSPA
	case appType == "regular_web":
		return quickstartsRegularWeb
	case appType == "non_interactive":
		return quickstartsM2M
	case appType == "resource_server":
		return quickstartsGeneric
	default:
		return quickstartsGeneric
	}
}

func ApplyColorToFriendlyAppType(a string) string {
	switch {
	case a == friendlyM2M:
		return ansi.Green(a)
	case a == friendlyNative:
		return ansi.Cyan(a)
	case a == friendlySpa:
		return ansi.Blue(a)
	case a == friendlyReg:
		return ansi.Magenta(a)
	case a == friendlyResourceServer:
		return ansi.Yellow(a)
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
