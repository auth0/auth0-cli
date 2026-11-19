package cli

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/poppy/setup"
)

// poppyQuestionTitles returns the titles of the questions on the screen, in the
// order they were drawn. A title is the line of a block that is one of the known
// titles, and it is kept once.
func poppyQuestionTitles(out string, known []string) []string {
	var titles []string

	for _, line := range strings.Split(stripEscapes(out), "\n") {
		line = strings.TrimSpace(line)

		if slices.Contains(known, line) && !slices.Contains(titles, line) {
			titles = append(titles, line)
		}
	}

	return titles
}

// installPoppyOrderAsker makes a terminal that picks the first API and the first
// app, and answers every other question with a value that is accepted.
func installPoppyOrderAsker(t *testing.T) {
	t.Helper()

	installPoppyAsker(t, modeTerminal)

	promptAsk.text = func(label, _ string) (string, error) {
		switch label {
		case "Domain":
			return "shop.example", nil
		case "Name":
			return "Shop Inc", nil
		case "URL":
			return "https://shop.example/docs", nil
		}

		return "Reads the orders of the signed-in user.", nil
	}
	promptAsk.chooseMany = func(_ string, _ []string, _ []int) ([]int, error) { return []int{0}, nil }
	promptAsk.choose = func(string, []string, int) (int, error) { return 0, nil }
}

func TestPoppySetupAsksInTheOrderOfTheParts(t *testing.T) {
	// A tenant that gives nothing: no company name, a scope without description,
	// and a web app without a login route.
	bare := func() poppyTenantState {
		state := freshPoppyTenant()
		state.settings.FriendlyName = nil
		state.server.Scopes[1].Description = nil
		state.app.InitiateLoginURI = nil

		return state
	}

	titles := struct{ domain, name, apiPart, apis, openAPI, description, hints, webPart, app, loginURI, unscoped, refresh string }{
		domain:      "Company domain",
		name:        "Company name",
		apiPart:     poppyAPIPartTitle,
		apis:        "APIs",
		openAPI:     "OpenAPI document of " + poppyTestAPI,
		description: "Description of checkout",
		hints:       "Hints for your scopes",
		webPart:     poppyWebPartTitle,
		app:         "Web app",
		loginURI:    "Application Login URI of Shop Web",
		unscoped:    poppyWebUnscopedTitle,
		refresh:     poppyRefreshTokensTitle,
	}

	known := []string{
		titles.domain, titles.name, titles.apiPart, titles.apis, titles.openAPI, titles.description, titles.hints,
		titles.webPart, titles.app, titles.loginURI, titles.unscoped, titles.refresh,
	}

	tests := []struct {
		name  string
		extra []string
		want  []string
	}{
		{
			name: "everything is asked, part by part",
			want: []string{
				titles.domain, titles.name,
				titles.apiPart, titles.apis, titles.openAPI, titles.description, titles.hints,
				titles.webPart, titles.app, titles.loginURI,
				titles.unscoped, titles.refresh,
			},
		},
		{
			name:  "without the web part, the API part is followed by the last two questions only",
			extra: []string{"--no-web"},
			want: []string{
				titles.domain, titles.name,
				titles.apiPart, titles.apis, titles.openAPI, titles.description, titles.hints,
				titles.refresh,
			},
		},
		{
			name:  "without the API part, the web part follows the name",
			extra: []string{"--no-api"},
			want: []string{
				titles.domain, titles.name,
				titles.webPart, titles.app, titles.loginURI,
				titles.unscoped, titles.refresh,
			},
		},
		{
			name:  "the flags answer their questions and the order of the rest stays",
			extra: []string{"--company-name", "Shop Inc", "--api", poppyTestAPI, "--app", poppySetupAppID, "--scope", "checkout=Place orders."},
			want: []string{
				titles.domain,
				titles.openAPI, titles.hints,
				titles.loginURI,
				titles.unscoped, titles.refresh,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := newPoppySetupHarness(t, bare())
			installPoppyOrderAsker(t)

			args := append([]string{"setup", "--out", filepath.Join(t.TempDir(), "poppy.json"), "--dry-run", "--skip-doctor"}, test.extra...)

			require.NoError(t, h.run(args...))
			assert.Equal(t, test.want, poppyQuestionTitles(h.out.String()+h.messages.String(), known))
		})
	}
}

func TestPoppySetupNoTerminalAsksNothingInTheNewOrder(t *testing.T) {
	// Without a terminal every missing flag is named at once, in the order of the
	// flags, whatever the order of the questions in a terminal.
	state := freshPoppyTenant()
	state.settings.FriendlyName = nil
	state.server.Scopes[1].Description = nil

	h := newPoppySetupHarness(t, state)
	installPoppyAsker(t, modePiped)

	err := h.run("setup", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID,
		"--out", filepath.Join(t.TempDir(), "poppy.json"))

	assert.EqualError(t, err, "missing required flags: --company-name, --api-url "+poppyTestAPI+"=<openapi url>, --scope 'checkout=<description>'")
}

func TestPoppyConfigAsksTheDomainBeforeTheName(t *testing.T) {
	state := &setup.State{
		Domain: poppySetupTenant,
		APIs: []setup.API{{
			ID: "rs_shop", Identifier: poppyTestAPI, Name: "Shop API", Poppy: true,
			Scopes: []setup.Scope{{Name: "orders:read", Description: "See your orders."}},
		}},
		App: setup.App{ClientID: poppySetupAppID, Name: "Shop Web", FirstParty: true, LoginURI: "https://shop.example/login", BrowserJoin: true},
	}

	var asked []string

	screen, _ := newTestPromptScreen(t, promptAsker{
		text: func(label, _ string) (string, error) {
			asked = append(asked, label)

			if label == "Domain" {
				return "shop.example", nil
			}

			return "https://shop.example/docs", nil
		},
	})

	in := setup.Inputs{APIs: []string{poppyTestAPI}, App: poppySetupAppID, Scopes: map[string]string{}}

	// The fields are sorted, which lists the OpenAPI URL first, and the questions
	// still come in the order of the parts.
	_, err := setup.Derive(state, in)
	require.ErrorContains(t, err, "organization.domain")

	err = askPoppyMissingValues(screen, state, &in)

	require.NoError(t, err)
	assert.Equal(t, []string{"Domain", "Name", "URL"}, asked)
	assert.Equal(t, "shop.example", in.CompanyDomain)
}
