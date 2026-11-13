package setup

import (
	"context"
	"fmt"
	"slices"
	"strings"

	managementv3 "github.com/auth0/go-auth0/v3/management"

	"github.com/auth0/auth0-cli/internal/poppy/config"
)

// ListPoppyAPIs returns the APIs of the tenant that have the Personal Agent Protocol
// on, in the order the tenant lists them.
func (r *Reader) ListPoppyAPIs(ctx context.Context) ([]API, error) {
	servers, err := r.servers(ctx, &managementv3.ListResourceServerRequestParameters{})
	if err != nil {
		return nil, err
	}

	var apis []API
	for _, server := range servers {
		if server.GetUseForPersonalAgentProtocol() {
			apis = append(apis, apiFromServer(server))
		}
	}

	return apis, nil
}

// ResolveAPIs returns the identifiers of the APIs of the tenant that the config
// file describes. The file holds the URL of each API and no identifier, so an
// entry of apis is linked to the Poppy API of the tenant whose identifier is at
// the origin of its URL. An entry that no Poppy API is at, such as a third-party
// MCP server, is not an API of the tenant and is skipped.
//
// It returns an *InputError, with a FIX line that says to pass --api, when the
// file is missing (the message says so) or lists no API of the tenant, or when an entry could belong to
// several.
func ResolveAPIs(file *config.Config, poppy []API) ([]string, error) {
	var found []string

	if file != nil {
		for _, entry := range file.APIs {
			origin := config.Origin(entry.URL)
			if origin == "" {
				continue
			}

			var matches []string
			for _, api := range poppy {
				if config.Origin(api.Identifier) == origin {
					matches = append(matches, api.Identifier)
				}
			}

			switch len(matches) {
			case 0:
			case 1:
				if !slices.Contains(found, matches[0]) {
					found = append(found, matches[0])
				}
			default:
				return nil, &InputError{
					Reason:     ReasonAmbiguousAPI,
					Message:    fmt.Sprintf("The config file entry %s could be any of these APIs: %s.", entry.URL, strings.Join(matches, ", ")),
					Fix:        "Pass --api <identifier> to name the API.",
					Candidates: matches,
				}
			}
		}
	}

	if file == nil {
		return nil, &InputError{
			Reason:  ReasonAPINotFound,
			Message: "There is no config file, so no API of the tenant is known.",
			Fix:     "Pass --api <identifier> to name the API, or create the file with auth0 poppy setup.",
		}
	}

	if len(found) == 0 {
		return nil, &InputError{
			Reason:  ReasonAPINotFound,
			Message: "No API of the tenant is known: the config file lists no URL at the identifier of an API that has use_for_personal_agent_protocol on.",
			Fix:     "Pass --api <identifier> to name the API.",
		}
	}

	return found, nil
}

// ResolveApp returns the client ID of the web app when none is given: the only
// app of the tenant that has the browser join on. It returns an *InputError, with
// a FIX line that says to pass --app, when there are none or several.
func ResolveApp(apps []App) (string, error) {
	switch len(apps) {
	case 1:
		return apps[0].ClientID, nil
	case 0:
		return "", &InputError{
			Reason:  ReasonAppNotFound,
			Message: "No app of the tenant has allow_personal_agent_browser_join on.",
			Fix:     "Pass --app <client ID or name> to name the web app.",
		}
	}

	names := make([]string, len(apps))
	ids := make([]string, len(apps))
	for i, app := range apps {
		names[i] = fmt.Sprintf("%s (%s)", app.Name, app.ClientID)
		ids[i] = app.ClientID
	}

	return "", &InputError{
		Reason:     ReasonAmbiguousApp,
		Message:    "Several apps of the tenant have allow_personal_agent_browser_join on: " + strings.Join(names, ", ") + ".",
		Fix:        "Pass --app <client ID or name> to name the web app.",
		Candidates: ids,
	}
}
