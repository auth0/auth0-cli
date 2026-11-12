package cli

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"slices"
	"strings"

	managementv3 "github.com/auth0/go-auth0/v3/management"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/poppy/setup"
	"github.com/auth0/auth0-cli/internal/text"
)

// The marks that a picker puts after an option.
const (
	poppyAPIMark = "  (Poppy)"
	poppyAppMark = "  (browser sign-in on)"
)

// poppyAppFields limits the clients listed for the picker of the web app to what
// it shows and filters on.
const poppyAppFields = "client_id,name,app_type,is_first_party,allow_personal_agent_browser_join"

// The types of app that can do the browser join, as the Management API names them.
var poppyWebAppTypes = []string{"regular_web", "spa"}

// poppyWebApp is a web app of the tenant, as the picker lists it.
type poppyWebApp struct {
	ClientID string
	Name     string
	AppType  string
	// BrowserJoin tells that the app has the browser join on.
	BrowserJoin bool
}

// isPoppyManagementAPI tells whether an API identifier is the one of the Management
// API of a tenant, which is not an API that agents use.
func isPoppyManagementAPI(identifier string) bool {
	u, err := url.Parse(identifier)

	return err == nil && u.Host != "" && path.Clean(u.Path) == "/api/v2"
}

// poppyAPIOptions builds the options of the picker of the APIs: the Management API
// is left out, an API that has Poppy on is marked, and the indexes of the marked
// ones are returned, to start the picker with them selected. The value of an
// option is the identifier of the API, which is what --api takes.
func poppyAPIOptions(apis []setup.API) (opts pickerOptions, selected []int) {
	for _, api := range apis {
		if isPoppyManagementAPI(api.Identifier) {
			continue
		}

		label := fmt.Sprintf("%s %s", text.Line(api.Name), ansi.Faint("("+text.Line(api.Identifier)+")"))

		if api.Poppy {
			label += poppyAPIMark
			selected = append(selected, len(opts))
		}

		opts = append(opts, pickerOption{value: api.Identifier, label: label})
	}

	return opts, selected
}

// poppyAPIPickerOptions lists the APIs of the tenant as the options of a picker.
func (c *cli) poppyAPIPickerOptions(ctx context.Context) (pickerOptions, []int, error) {
	apis, err := c.poppyReader().ListAPIs(ctx)
	if err != nil {
		return nil, nil, err
	}

	opts, selected := poppyAPIOptions(apis)
	if len(opts) == 0 {
		return nil, nil, validationError{
			err:    errors.New("there are currently no APIs to choose from. Create one by running: `auth0 apis create`"),
			reason: setup.ReasonAPINotFound,
		}
	}

	return opts, selected, nil
}

// poppyAppOptions builds the options of the picker of the web app, with the label
// of the other pickers of apps: the name, the type and the client ID. An app that
// has the browser join on is marked and put first, so that the picker starts on
// it. The value of an option is the client ID, which is what --app takes.
func poppyAppOptions(apps []poppyWebApp) pickerOptions {
	var joined, others pickerOptions

	for _, app := range apps {
		label := text.Line(app.Name)

		if app.AppType != "" {
			label += fmt.Sprintf(" [%s]", display.ApplyColorToFriendlyAppType(display.FriendlyAppType(app.AppType)))
		}

		label += " " + ansi.Faint("("+app.ClientID+")")

		if app.BrowserJoin {
			joined = append(joined, pickerOption{value: app.ClientID, label: label + poppyAppMark})

			continue
		}

		others = append(others, pickerOption{value: app.ClientID, label: label})
	}

	return append(joined, others...)
}

// listPoppyWebApps lists the first-party apps of the tenant that can do the browser
// join: regular web apps and single-page apps.
func (c *cli) listPoppyWebApps(ctx context.Context) ([]poppyWebApp, error) {
	include, firstParty, fields := true, true, poppyAppFields

	page, err := c.apiv3.ClientV3.List(ctx, &managementv3.ListClientsRequestParameters{
		Fields:        &fields,
		IncludeFields: &include,
		IsFirstParty:  &firstParty,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list applications: %w", err)
	}

	var apps []poppyWebApp

	iter := page.Iterator()
	for iter.Next(ctx) {
		client := iter.Current()

		// The types are filtered here, because the request takes one type at most.
		appType := string(client.GetAppType())
		if !client.GetIsFirstParty() || !slices.Contains(poppyWebAppTypes, appType) {
			continue
		}

		apps = append(apps, poppyWebApp{
			ClientID:    client.GetClientID(),
			Name:        client.GetName(),
			AppType:     appType,
			BrowserJoin: client.GetAllowPersonalAgentBrowserJoin(),
		})
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to list applications: %w", err)
	}

	return apps, nil
}

// poppyAppPickerOptions lists the web apps of the tenant as the options of a picker.
func (c *cli) poppyAppPickerOptions(ctx context.Context) (pickerOptions, error) {
	apps, err := c.listPoppyWebApps(ctx)
	if err != nil {
		return nil, err
	}

	opts := poppyAppOptions(apps)
	if len(opts) == 0 {
		return nil, validationError{
			err:    errors.New("there are currently no regular web or single-page apps to choose from. Create one by running: `auth0 apps create`"),
			reason: setup.ReasonAppNotFound,
		}
	}

	return opts, nil
}

// pickAPIs asks which of the APIs to use, with the ones in selected chosen to begin
// with, and asks again for an answer that chooses none. It returns the values of
// the options chosen.
func (s *questionScreen) pickAPIs(opts pickerOptions, selected []int) ([]string, error) {
	const title = "APIs"

	reason := "The APIs that agents will use."
	if len(selected) > 0 {
		reason += " The ones marked (Poppy) have Poppy on already, and are selected."
	}

	var picked []int

	err := s.ask(title, reason, func() error {
		for {
			var err error
			if picked, err = s.asker.chooseMany(title, opts.labels(), selected); err != nil {
				return err
			}

			if len(picked) > 0 {
				return nil
			}

			s.Invalid("Choose at least one API.")
		}
	})
	if err != nil {
		return nil, err
	}

	values := make([]string, len(picked))
	for i, index := range picked {
		values[i] = opts[index].value
	}

	return values, nil
}

// pickApp asks which of the apps to use for the browser join, starting on the first
// option. It returns the value of the option chosen.
func (s *questionScreen) pickApp(opts pickerOptions) (string, error) {
	const title = "Web app"

	reason := "The first-party web app that does the browser join."
	if strings.HasSuffix(opts[0].label, poppyAppMark) {
		reason += " The ones marked (browser sign-in on) have it on already."
	}

	var picked int

	err := s.ask(title, reason, func() (err error) {
		picked, err = s.asker.choose("App", opts.labels(), 0)

		return err
	})
	if err != nil {
		return "", err
	}

	return opts[picked].value, nil
}

// pickAPIsOf asks which APIs to use, listing the ones of the tenant.
func (c *cli) pickAPIsOf(ctx context.Context, screen *questionScreen) ([]string, error) {
	var (
		opts     pickerOptions
		selected []int
	)

	err := ansi.Waiting(func() (err error) {
		opts, selected, err = c.poppyAPIPickerOptions(ctx)

		return err
	})
	if err != nil {
		return nil, err
	}

	return screen.pickAPIs(opts, selected)
}

// pickAppOf asks which web app to use, listing the ones of the tenant.
func (c *cli) pickAppOf(ctx context.Context, screen *questionScreen) (string, error) {
	var opts pickerOptions

	err := ansi.Waiting(func() (err error) {
		opts, err = c.poppyAppPickerOptions(ctx)

		return err
	})
	if err != nil {
		return "", err
	}

	return screen.pickApp(opts)
}

// poppyCompanyDomainGuess returns the company domain that the custom domains of the
// tenant suggest: the first one with its first label dropped, so that
// auth.shop.example gives shop.example. It returns "" when there is none. The
// first of hosts is the Auth0 domain of the tenant, which says nothing about the
// company.
func poppyCompanyDomainGuess(hosts []string) string {
	if len(hosts) < 2 {
		return ""
	}

	labels := strings.Split(strings.ToLower(hosts[1]), ".")

	switch {
	case len(labels) < 3:
		return strings.Join(labels, ".")
	case len(labels) == 3 && poppySecondLevelLabels[labels[1]] && len(labels[2]) == 2:
		// A name under a suffix such as co.uk is the company domain itself.
		return strings.Join(labels, ".")
	}

	return strings.Join(labels[1:], ".")
}

// poppySecondLevelLabels are the labels that country domains put under their top
// level, such as co in shop.co.uk.
var poppySecondLevelLabels = map[string]bool{"co": true, "com": true, "org": true, "net": true, "gov": true, "edu": true, "ac": true}

// guessPoppyCompanyDomain reads the custom domains of the tenant to suggest a
// company domain. It suggests none when they cannot be read: the suggestion is a
// convenience.
func (c *cli) guessPoppyCompanyDomain(ctx context.Context) string {
	var hosts []string

	_ = ansi.Waiting(func() error {
		hosts, _ = c.poppyReader().TenantHosts(ctx)

		return nil
	})

	return poppyCompanyDomainGuess(hosts)
}
