package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/poppy/setup"
)

const poppyTestManagementAPI = "https://shop.us.auth0.com/api/v2/"

func TestPoppyAPIOptions(t *testing.T) {
	tests := []struct {
		name         string
		apis         []setup.API
		wantValues   []string
		wantSelected []int
		wantMarked   []bool
	}{
		{
			name:       "an API with no Poppy is listed and not selected",
			apis:       []setup.API{{Name: "Shop API", Identifier: poppyTestAPI}},
			wantValues: []string{poppyTestAPI},
			wantMarked: []bool{false},
		},
		{
			name: "an API with Poppy is marked and selected",
			apis: []setup.API{
				{Name: "Other API", Identifier: poppyTestOtherAPI},
				{Name: "Shop API", Identifier: poppyTestAPI, Poppy: true},
			},
			wantValues:   []string{poppyTestOtherAPI, poppyTestAPI},
			wantSelected: []int{1},
			wantMarked:   []bool{false, true},
		},
		{
			name: "the Management API is left out, and the indexes follow the options",
			apis: []setup.API{
				{Name: "Auth0 Management API", Identifier: poppyTestManagementAPI, Poppy: true},
				{Name: "Shop API", Identifier: poppyTestAPI, Poppy: true},
			},
			wantValues:   []string{poppyTestAPI},
			wantSelected: []int{0},
			wantMarked:   []bool{true},
		},
		{
			name: "an API that only looks like the Management API is kept",
			apis: []setup.API{
				{Name: "Docs", Identifier: "https://shop.example/api/v2/docs"},
				{Name: "Other", Identifier: "https://shop.example/api/v3"},
			},
			wantValues: []string{"https://shop.example/api/v2/docs", "https://shop.example/api/v3"},
			wantMarked: []bool{false, false},
		},
		{
			name: "a tenant with only the Management API has nothing to pick",
			apis: []setup.API{{Name: "Auth0 Management API", Identifier: poppyTestManagementAPI}},
		},
		{
			name: "no APIs",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			opts, selected := poppyAPIOptions(test.apis)

			var values []string
			for i, opt := range opts {
				values = append(values, opt.value)

				assert.Contains(t, opt.label, test.apis[slicesIndexOfAPI(test.apis, opt.value)].Name)
				assert.Contains(t, opt.label, opt.value, "the identifier is part of the label")
				assert.Equal(t, test.wantMarked[i], strings.HasSuffix(opt.label, poppyAPIMark), opt.label)
			}

			assert.Equal(t, test.wantValues, values)
			assert.Equal(t, test.wantSelected, selected)
		})
	}
}

func slicesIndexOfAPI(apis []setup.API, identifier string) int {
	for i, api := range apis {
		if api.Identifier == identifier {
			return i
		}
	}

	return -1
}

func TestPoppyAPIPickerOptions(t *testing.T) {
	poppyOn := func(identifier, name string) *managementv3.ResourceServer {
		return &managementv3.ResourceServer{
			ID:                          testPtr("rs_" + name),
			Identifier:                  testPtr(identifier),
			Name:                        testPtr(name),
			UseForPersonalAgentProtocol: testPtr(true),
		}
	}
	plain := func(identifier, name string) *managementv3.ResourceServer {
		return &managementv3.ResourceServer{ID: testPtr("rs_" + name), Identifier: testPtr(identifier), Name: testPtr(name)}
	}

	tests := []struct {
		name         string
		servers      []*managementv3.ResourceServer
		wantValues   []string
		wantSelected []int
		wantReason   string
	}{
		{
			name:         "the APIs of the tenant, without the Management API, and the Poppy ones selected",
			servers:      []*managementv3.ResourceServer{plain(poppyTestManagementAPI, "Auth0 Management API"), plain(poppyTestOtherAPI, "Other"), poppyOn(poppyTestAPI, "Shop")},
			wantValues:   []string{poppyTestOtherAPI, poppyTestAPI},
			wantSelected: []int{1},
		},
		{
			name:       "a tenant with only the Management API",
			servers:    []*managementv3.ResourceServer{plain(poppyTestManagementAPI, "Auth0 Management API")},
			wantReason: "api_not_found",
		},
		{
			name:       "a tenant with no API",
			wantReason: "api_not_found",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			servers := mock.NewMockResourceServerAPIV3(ctrl)
			servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(testServerPage(test.servers...), nil)

			c := &cli{api: &auth0.API{}, apiv3: &auth0.APIV3{ResourceServerV3: servers}}

			opts, selected, err := c.poppyAPIPickerOptions(context.Background())

			if test.wantReason != "" {
				require.Error(t, err)
				assert.Equal(t, test.wantReason, errorReason(err))
				assert.Contains(t, err.Error(), "auth0 apis create")

				return
			}

			require.NoError(t, err)

			var values []string
			for _, opt := range opts {
				values = append(values, opt.value)
			}

			assert.Equal(t, test.wantValues, values)
			assert.Equal(t, test.wantSelected, selected)
		})
	}
}

func TestPoppyAppPickerOptions(t *testing.T) {
	app := func(id, name string, appType managementv3.ClientAppTypeEnum, firstParty, join bool) *managementv3.Client {
		return &managementv3.Client{
			ClientID:                      testPtr(id),
			Name:                          testPtr(name),
			AppType:                       testPtr(appType),
			IsFirstParty:                  testPtr(firstParty),
			AllowPersonalAgentBrowserJoin: testPtr(join),
		}
	}

	tests := []struct {
		name       string
		clients    []*managementv3.Client
		wantIDs    []string
		wantMarked []bool
		wantReason string
	}{
		{
			name: "the join app comes first and is marked",
			clients: []*managementv3.Client{
				app("c_one", "One", managementv3.ClientAppTypeEnumSpa, true, false),
				app("c_two", "Two", managementv3.ClientAppTypeEnumRegularWeb, true, true),
			},
			wantIDs:    []string{"c_two", "c_one"},
			wantMarked: []bool{true, false},
		},
		{
			name: "apps that cannot do the join are left out",
			clients: []*managementv3.Client{
				app("c_m2m", "Machine", managementv3.ClientAppTypeEnumNonInteractive, true, false),
				app("c_third", "Third party", managementv3.ClientAppTypeEnumRegularWeb, false, false),
				app("c_web", "Web", managementv3.ClientAppTypeEnumRegularWeb, true, false),
			},
			wantIDs:    []string{"c_web"},
			wantMarked: []bool{false},
		},
		{
			name:       "only unfit apps",
			clients:    []*managementv3.Client{app("c_m2m", "Machine", managementv3.ClientAppTypeEnumNonInteractive, true, false)},
			wantReason: "app_not_found",
		},
		{
			name:       "no apps",
			wantReason: "app_not_found",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			clients := mock.NewMockClientAPIV3(ctrl)
			clients.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, request *managementv3.ListClientsRequestParameters, _ ...any) (*auth0.ClientListPage, error) {
					require.NotNil(t, request.IsFirstParty)
					assert.True(t, *request.IsFirstParty, "only first-party apps are asked for")
					require.NotNil(t, request.Fields)
					assert.Contains(t, *request.Fields, "allow_personal_agent_browser_join")
					require.NotNil(t, request.IncludeFields)
					assert.True(t, *request.IncludeFields)

					return testClientPage(test.clients...), nil
				})

			c := &cli{apiv3: &auth0.APIV3{ClientV3: clients}}

			opts, err := c.poppyAppPickerOptions(context.Background())

			if test.wantReason != "" {
				require.Error(t, err)
				assert.Equal(t, test.wantReason, errorReason(err))
				assert.Contains(t, err.Error(), "auth0 apps create")

				return
			}

			require.NoError(t, err)

			var ids []string
			for i, opt := range opts {
				ids = append(ids, opt.value)

				assert.Contains(t, opt.label, "("+opt.value+")", "the client ID is part of the label")
				assert.Equal(t, test.wantMarked[i], strings.HasSuffix(opt.label, poppyAppMark), opt.label)
			}

			assert.Equal(t, test.wantIDs, ids)
		})
	}
}

func TestPoppyCompanyDomainGuess(t *testing.T) {
	tests := []struct {
		name  string
		hosts []string
		want  string
	}{
		{name: "no hosts"},
		{name: "only the Auth0 domain", hosts: []string{"shop.us.auth0.com"}},
		{name: "a custom domain under the company domain", hosts: []string{"shop.us.auth0.com", "auth.shop.example"}, want: "shop.example"},
		{name: "the first custom domain is used", hosts: []string{"shop.us.auth0.com", "login.shop.example", "auth.other.example"}, want: "shop.example"},
		{name: "a custom domain that is the company domain", hosts: []string{"shop.us.auth0.com", "shop.example"}, want: "shop.example"},
		{name: "a deeper name", hosts: []string{"shop.us.auth0.com", "id.eu.shop.example"}, want: "eu.shop.example"},
		{name: "a name under a country suffix", hosts: []string{"shop.us.auth0.com", "shop.co.uk"}, want: "shop.co.uk"},
		{name: "a custom domain under a country suffix", hosts: []string{"shop.us.auth0.com", "auth.shop.co.uk"}, want: "shop.co.uk"},
		{name: "upper case", hosts: []string{"shop.us.auth0.com", "Auth.Shop.Example"}, want: "shop.example"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, poppyCompanyDomainGuess(test.hosts))
		})
	}
}

func TestQuestionScreenPickAPIs(t *testing.T) {
	opts, selected := poppyAPIOptions([]setup.API{
		{Name: "Other", Identifier: poppyTestOtherAPI},
		{Name: "Shop", Identifier: poppyTestAPI, Poppy: true},
	})

	t.Run("the chosen options give their identifiers, and the marked ones start selected", func(t *testing.T) {
		var gotDefault []int

		screen, out := newTestPromptScreen(t, promptAsker{
			chooseMany: func(_ string, _ []string, def []int) ([]int, error) {
				gotDefault = def

				return []int{1, 0}, nil
			},
		})

		got, err := screen.pickAPIs(opts, selected)

		require.NoError(t, err)
		assert.Equal(t, []string{poppyTestAPI, poppyTestOtherAPI}, got)
		assert.Equal(t, []int{1}, gotDefault)
		assert.Contains(t, out.String(), "APIs\nThe APIs that agents will use.")
		assert.Contains(t, out.String(), "(Poppy)")
	})

	t.Run("choosing none asks again", func(t *testing.T) {
		answers := [][]int{{}, {0}}
		calls := 0

		screen, out := newTestPromptScreen(t, promptAsker{
			chooseMany: func(string, []string, []int) ([]int, error) {
				calls++
				answer := answers[0]
				answers = answers[1:]

				return answer, nil
			},
		})

		got, err := screen.pickAPIs(opts, nil)

		require.NoError(t, err)
		assert.Equal(t, []string{poppyTestOtherAPI}, got)
		assert.Equal(t, 2, calls)
		assert.Equal(t, 1, strings.Count(out.String(), "Choose at least one API."))
		assert.NotContains(t, out.String(), "have Poppy on already", "nothing is selected, so nothing is said about it")
	})

	t.Run("Ctrl-C says nothing was changed", func(t *testing.T) {
		screen, out := newTestPromptScreen(t, promptAsker{
			chooseMany: func(string, []string, []int) ([]int, error) { return nil, terminal.InterruptErr },
		})

		_, err := screen.pickAPIs(opts, selected)

		require.ErrorIs(t, err, terminal.InterruptErr)
		assert.True(t, strings.HasSuffix(out.String(), "Cancelled. Nothing was changed.\n"), out.String())
	})
}

func TestQuestionScreenPickApp(t *testing.T) {
	opts := poppyAppOptions([]poppyWebApp{
		{ClientID: "c_one", Name: "One", AppType: "spa"},
		{ClientID: "c_two", Name: "Two", AppType: "regular_web", BrowserJoin: true},
	})

	t.Run("the choice gives the client ID, starting on the first option", func(t *testing.T) {
		var gotDefault int

		screen, out := newTestPromptScreen(t, promptAsker{
			choose: func(_ string, _ []string, def int) (int, error) {
				gotDefault = def

				return 1, nil
			},
		})

		got, err := screen.pickApp(opts)

		require.NoError(t, err)
		assert.Equal(t, "c_one", got)
		assert.Equal(t, 0, gotDefault)
		assert.Contains(t, out.String(), "have it on already")
	})

	t.Run("no mark, nothing said about it", func(t *testing.T) {
		screen, out := newTestPromptScreen(t, promptAsker{
			choose: func(string, []string, int) (int, error) { return 0, nil },
		})

		_, err := screen.pickApp(poppyAppOptions([]poppyWebApp{{ClientID: "c_one", Name: "One"}}))

		require.NoError(t, err)
		assert.NotContains(t, out.String(), "have it on already")
	})
}

// poppyPickerRun records what the pickers of a run were shown.
type poppyPickerRun struct {
	apiLabels    []string
	apiDefault   []int
	appLabels    []string
	textDefaults map[string]string
	asked        []string
}

// installPoppyPickers answers the pickers of a run in a terminal: the APIs by
// the identifiers it is given and the app by the position. Nothing is asked
// unless the run gets to it.
func installPoppyPickers(t *testing.T, apis []string, appIndex int) *poppyPickerRun {
	t.Helper()

	run := &poppyPickerRun{textDefaults: map[string]string{}}

	installPoppyAsker(t, modeTerminal)

	previousText, previousConfirm := promptAsk.text, promptAsk.confirm

	promptAsk.text = func(label, def string) (string, error) {
		run.textDefaults[label] = def

		if label == "Domain" {
			return "shop.example", nil
		}

		return previousText(label, def)
	}
	promptAsk.confirm = func(message string, def bool) (bool, error) {
		switch message {
		case poppyAPIPartText:
			run.asked = append(run.asked, "api part")
		case poppyWebPartText:
			run.asked = append(run.asked, "web part")
		}

		return previousConfirm(message, def)
	}
	promptAsk.chooseMany = func(_ string, labels []string, def []int) ([]int, error) {
		run.asked = append(run.asked, "apis")
		run.apiLabels, run.apiDefault = labels, def

		var picked []int

		for i, label := range labels {
			for _, identifier := range apis {
				if strings.Contains(label, "("+identifier+")") {
					picked = append(picked, i)
				}
			}
		}

		return picked, nil
	}
	promptAsk.choose = func(_ string, labels []string, _ int) (int, error) {
		run.asked = append(run.asked, "app")
		run.appLabels = labels

		return appIndex, nil
	}

	return run
}

func TestPoppySetupPicksTheAPIsAndTheApp(t *testing.T) {
	tenantWithMore := func() poppyTenantState {
		state := freshPoppyTenant()
		state.moreServers = []*managementv3.ResourceServer{
			{ID: testPtr("rs_mgmt"), Identifier: testPtr(poppyTestManagementAPI), Name: testPtr("Auth0 Management API")},
			{ID: testPtr("rs_other"), Identifier: testPtr(poppyTestOtherAPI), Name: testPtr("Other API"), UseForPersonalAgentProtocol: testPtr(true)},
		}
		state.moreApps = []*managementv3.GetClientResponseContent{
			{ClientID: testPtr("c_spa"), Name: testPtr("Shop SPA"), AppType: testPtr(managementv3.ClientAppTypeEnumSpa), IsFirstParty: testPtr(true), InitiateLoginURI: testPtr("https://shop.example/spa")},
			{ClientID: testPtr("c_m2m"), Name: testPtr("Machine"), AppType: testPtr(managementv3.ClientAppTypeEnumNonInteractive), IsFirstParty: testPtr(true)},
			{ClientID: testPtr("c_ext"), Name: testPtr("Partner"), AppType: testPtr(managementv3.ClientAppTypeEnumRegularWeb), IsFirstParty: testPtr(false)},
		}

		return state
	}

	pickerArgs := func(t *testing.T, extra ...string) []string {
		t.Helper()

		return append([]string{
			"setup", "--company-domain", "shop.example",
			"--api-url", poppyTestAPI + "=" + poppyTestAPI + "/openapi.json",
			"--out", filepath.Join(t.TempDir(), "poppy.json"),
			"--skip-doctor",
		}, extra...)
	}

	t.Run("the picked API and app go into the plan like --api and --app", func(t *testing.T) {
		h := newPoppySetupHarness(t, tenantWithMore())
		run := installPoppyPickers(t, []string{poppyTestAPI}, 0)

		args := pickerArgs(t, "--dry-run")
		require.NoError(t, h.run(args...))

		assert.Equal(t, []string{"apis", "web part", "app"}, run.asked)
		assert.Contains(t, h.out.String(), "--api "+poppyTestAPI)
		assert.Contains(t, h.out.String(), "--app "+poppySetupAppID)

		flagged := newPoppySetupHarness(t, tenantWithMore())
		installPoppyAsker(t, modeTerminal)
		require.NoError(t, flagged.run(pickerArgs(t, "--dry-run", "--api", poppyTestAPI, "--app", poppySetupAppID)...))

		clean := func(out string) string {
			return strings.ReplaceAll(stripEscapes(out), outOf(args), "FILE")
		}

		assert.Equal(t, strings.Count(clean(flagged.out.String()), "\n"), strings.Count(clean(h.out.String()), "\n"), "the same plan")
	})

	t.Run("the pickers list what the tenant has and nothing else", func(t *testing.T) {
		h := newPoppySetupHarness(t, tenantWithMore())
		run := installPoppyPickers(t, []string{poppyTestAPI}, 0)

		require.NoError(t, h.run(pickerArgs(t, "--dry-run")...))

		require.Len(t, run.apiLabels, 2, "the Management API is not offered")
		assert.Contains(t, stripEscapes(run.apiLabels[0]), "Shop API ("+poppyTestAPI+")")
		assert.NotContains(t, stripEscapes(run.apiLabels[0]), "(Poppy)")
		assert.Contains(t, stripEscapes(run.apiLabels[1]), "Other API ("+poppyTestOtherAPI+")  (Poppy)")
		assert.Equal(t, []int{1}, run.apiDefault, "the API that has Poppy on starts selected")

		require.Len(t, run.appLabels, 2, "only first-party web and single-page apps are offered")
		assert.Contains(t, stripEscapes(run.appLabels[0]), "Shop Web")
		assert.Contains(t, stripEscapes(run.appLabels[0]), "("+poppySetupAppID+")")
		assert.Contains(t, stripEscapes(run.appLabels[1]), "Shop SPA")
	})

	t.Run("several APIs and the second app", func(t *testing.T) {
		h := newPoppySetupHarness(t, tenantWithMore())
		installPoppyPickers(t, []string{poppyTestAPI, poppyTestOtherAPI}, 1)

		require.NoError(t, h.run(pickerArgs(t, "--dry-run", "--api-url", poppyTestOtherAPI+"="+poppyTestOtherAPI+"/openapi.json")...))

		assert.Contains(t, h.out.String(), "--api "+poppyTestAPI)
		assert.Contains(t, h.out.String(), "--api "+poppyTestOtherAPI)
		assert.Contains(t, h.out.String(), "--app c_spa")
	})

	t.Run("the pickers are not shown for what the flags gave", func(t *testing.T) {
		h := newPoppySetupHarness(t, tenantWithMore())
		run := installPoppyPickers(t, nil, 0)

		require.NoError(t, h.run(pickerArgs(t, "--dry-run", "--api", poppyTestAPI, "--app", poppySetupAppID)...))

		assert.Empty(t, run.asked)
	})

	t.Run("the company domain starts on the one the custom domain suggests", func(t *testing.T) {
		state := tenantWithMore()
		state.domains = configuredPoppyTenant().domains
		h := newPoppySetupHarness(t, state)
		run := installPoppyPickers(t, []string{poppyTestAPI}, 0)

		args := pickerArgs(t, "--dry-run")
		args = append(args[:1], args[3:]...)

		require.NoError(t, h.run(args...))

		assert.Equal(t, "shop.example", run.textDefaults["Domain"], run.textDefaults)
	})

	t.Run("without a custom domain the company domain has no default", func(t *testing.T) {
		h := newPoppySetupHarness(t, tenantWithMore())
		run := installPoppyPickers(t, []string{poppyTestAPI}, 0)

		args := pickerArgs(t, "--dry-run")
		args = append(args[:1], args[3:]...)

		require.NoError(t, h.run(args...))

		assert.Empty(t, run.textDefaults["Domain"], run.textDefaults)
	})

	t.Run("a tenant with only the Management API points to apis create", func(t *testing.T) {
		state := freshPoppyTenant()
		state.server.Identifier = testPtr(poppyTestManagementAPI)
		h := newPoppySetupHarness(t, state)
		run := installPoppyPickers(t, nil, 0)

		err := h.run(pickerArgs(t, "--dry-run")...)

		require.Error(t, err)
		assert.Equal(t, "api_not_found", errorReason(err))
		assert.Contains(t, err.Error(), "auth0 apis create")
		assert.NotContains(t, run.asked, "apis")
	})
}

func TestPoppySetupMissingFlags(t *testing.T) {
	t.Run("every missing flag is named at once", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())

		out := filepath.Join(t.TempDir(), "poppy.json")
		err := h.run("setup", "--out", out)

		reason, details := decodeCLIError(t, err)
		assert.Equal(t, "missing_required_flags", reason)
		assert.Equal(t, "missing required flags: --company-domain, --api, --app", err.Error())
		assert.NotContains(t, err.Error(), "\n")

		flags, ok := details["flags"].([]any)
		require.True(t, ok, details)
		require.Len(t, flags, 3)

		var names []string
		for _, entry := range flags {
			flag := entry.(map[string]any)
			names = append(names, flag["flag"].(string))
			assert.NotEmpty(t, flag["reason"], "each flag says why it is needed")
		}

		assert.Equal(t, []string{"--company-domain", "--api", "--app"}, names)
		assert.Equal(t, "auth0 poppy setup --tenant "+poppySetupTenant+" --out "+out+" --company-domain <domain> --api <api> --app <app>", details["example"],
			"the example keeps the flags that were passed and has a placeholder for each missing one")
	})

	t.Run("the OpenAPI URL is named together with the company domain", func(t *testing.T) {
		h := newPoppySetupHarness(t, freshPoppyTenant())

		err := h.run("setup", "--api", poppyTestAPI, "--app", poppySetupAppID, "--out", filepath.Join(t.TempDir(), "poppy.json"))

		reason, details := decodeCLIError(t, err)
		assert.Equal(t, "missing_required_flags", reason)
		assert.Equal(t, "missing required flags: --company-domain, --api-url "+poppyTestAPI+"=<openapi url>", err.Error())
		assert.Len(t, details["flags"], 2)
		assert.True(t, strings.HasSuffix(details["example"].(string), " --company-domain <domain> --api-url "+poppyTestAPI+"=<openapi url>"), details["example"])
		assert.Contains(t, details["example"], "--api "+poppyTestAPI+" --app "+poppySetupAppID)
	})
}

func TestPoppyConfigPicks(t *testing.T) {
	// ConfigArgs gives what config needs apart from the API and the app.
	configArgs := func(t *testing.T, extra ...string) []string {
		t.Helper()

		return append([]string{
			"config", "--company-domain", "shop.example",
			"--api-url", poppyTestAPI + "=" + poppyTestAPI + "/openapi.json",
			"--out", filepath.Join(t.TempDir(), "poppy.json"),
		}, extra...)
	}

	otherPoppyAPI := func() *managementv3.ResourceServer {
		return &managementv3.ResourceServer{
			ID:                          testPtr("rs_other"),
			Identifier:                  testPtr(poppyTestOtherAPI),
			Name:                        testPtr("Other API"),
			UseForPersonalAgentProtocol: testPtr(true),
			Scopes:                      []*managementv3.ResourceServerScope{{Value: "refund", Description: testPtr("Refund an order.")}},
		}
	}
	plainAPI := func() *managementv3.ResourceServer {
		return &managementv3.ResourceServer{ID: testPtr("rs_plain"), Identifier: testPtr("https://plain.example"), Name: testPtr("Plain API")}
	}
	otherJoinApp := func() *managementv3.GetClientResponseContent {
		return &managementv3.GetClientResponseContent{
			ClientID: testPtr("c_spa"), Name: testPtr("Shop SPA"), AppType: testPtr(managementv3.ClientAppTypeEnumSpa),
			IsFirstParty: testPtr(true), AllowPersonalAgentBrowserJoin: testPtr(true), InitiateLoginURI: testPtr("https://shop.example/spa"),
		}
	}

	t.Run("the only Poppy API and the only join app are used without a picker", func(t *testing.T) {
		state := configuredPoppyTenant()
		state.moreServers = []*managementv3.ResourceServer{plainAPI()}
		h := newPoppySetupHarness(t, state)
		run := installPoppyPickers(t, nil, 0)

		args := configArgs(t)
		require.NoError(t, h.run(args...))

		assert.Equal(t, []string{"web part"}, run.asked, "the part of the web app is asked, no picker is shown")

		cfg, err := config.Read(outOf(args))
		require.NoError(t, err)
		require.Len(t, cfg.APIs, 1)
		assert.Equal(t, "Shop API", cfg.APIs[0].Description)
	})

	t.Run("several Poppy APIs in a terminal are picked from, with the others left out", func(t *testing.T) {
		state := configuredPoppyTenant()
		state.moreServers = []*managementv3.ResourceServer{otherPoppyAPI(), plainAPI()}
		h := newPoppySetupHarness(t, state)
		run := installPoppyPickers(t, []string{poppyTestAPI}, 0)

		args := configArgs(t)
		require.NoError(t, h.run(args...))

		assert.Equal(t, []string{"web part", "apis"}, run.asked)
		require.Len(t, run.apiLabels, 2, "an API that has no Poppy is not offered")
		assert.NotContains(t, strings.Join(run.apiLabels, "\n"), "Plain API")
		assert.Equal(t, []int{0, 1}, run.apiDefault, "both start selected")

		cfg, err := config.Read(outOf(args))
		require.NoError(t, err)
		require.Len(t, cfg.APIs, 1)
		assert.Equal(t, "Shop API", cfg.APIs[0].Description)
	})

	t.Run("several Poppy APIs without a terminal ask for --api", func(t *testing.T) {
		state := configuredPoppyTenant()
		state.moreServers = []*managementv3.ResourceServer{otherPoppyAPI()}
		h := newPoppySetupHarness(t, state)
		installPoppyAsker(t, modePiped)

		err := h.run(configArgs(t, "--app", poppySetupAppID)...)

		require.Error(t, err)
		assert.Equal(t, "api_not_found", errorReason(err))
	})

	t.Run("several join apps in a terminal are picked from", func(t *testing.T) {
		state := configuredPoppyTenant()
		state.moreApps = []*managementv3.GetClientResponseContent{otherJoinApp()}
		h := newPoppySetupHarness(t, state)
		run := installPoppyPickers(t, nil, 1)

		require.NoError(t, h.run(configArgs(t)...))

		assert.Equal(t, []string{"web part", "app"}, run.asked)
		require.Len(t, run.appLabels, 2)
		assert.True(t, strings.HasSuffix(run.appLabels[0], poppyAppMark), "the apps with the join on come first and are marked")
		assert.True(t, strings.HasSuffix(run.appLabels[1], poppyAppMark))
	})

	t.Run("several join apps without a terminal ask for --app", func(t *testing.T) {
		state := configuredPoppyTenant()
		state.moreApps = []*managementv3.GetClientResponseContent{otherJoinApp()}
		h := newPoppySetupHarness(t, state)
		installPoppyAsker(t, modePiped)

		err := h.run(configArgs(t, "--api", poppyTestAPI)...)

		require.Error(t, err)
		assert.Equal(t, "ambiguous_app", errorReason(err))
	})

	t.Run("a tenant with no Poppy API is not asked whether it wants one", func(t *testing.T) {
		state := configuredPoppyTenant()
		state.server.UseForPersonalAgentProtocol = nil
		h := newPoppySetupHarness(t, state)
		run := installPoppyPickers(t, nil, 0)

		err := h.run(configArgs(t, "--app", poppySetupAppID)...)

		require.Error(t, err)
		assert.Equal(t, "api_not_found", errorReason(err))
		assert.NotContains(t, run.asked, "api part")
		assert.NotContains(t, run.asked, "apis")
	})

	t.Run("a tenant with no join app is not asked whether it wants one", func(t *testing.T) {
		state := configuredPoppyTenant()
		state.app.AllowPersonalAgentBrowserJoin = nil
		h := newPoppySetupHarness(t, state)
		run := installPoppyPickers(t, nil, 0)

		err := h.run(configArgs(t, "--api", poppyTestAPI)...)

		require.Error(t, err)
		assert.Equal(t, "app_not_found", errorReason(err))
		assert.NotContains(t, run.asked, "web part")
		assert.NotContains(t, run.asked, "app")
	})

	t.Run("an API and an app both missing are named at once", func(t *testing.T) {
		state := configuredPoppyTenant()
		state.server.UseForPersonalAgentProtocol = nil
		state.app.AllowPersonalAgentBrowserJoin = nil
		h := newPoppySetupHarness(t, state)
		installPoppyAsker(t, modePiped)

		err := h.run(configArgs(t)...)

		reason, details := decodeCLIError(t, err)
		assert.Equal(t, "missing_required_flags", reason)
		assert.Equal(t, "missing required flags: --api, --app", err.Error())

		flags, ok := details["flags"].([]any)
		require.True(t, ok, details)
		require.Len(t, flags, 2)

		for _, entry := range flags {
			assert.NotEmpty(t, entry.(map[string]any)["reason"])
		}

		assert.Contains(t, details["example"], "auth0 poppy config --tenant "+poppySetupTenant+" --company-domain shop.example --api-url "+poppyTestAPI+"=")
		assert.True(t, strings.HasSuffix(details["example"].(string), " --api <api> --app <app>"), details["example"])
	})

	t.Run("Ctrl-C in the picker writes no file", func(t *testing.T) {
		state := configuredPoppyTenant()
		state.moreServers = []*managementv3.ResourceServer{otherPoppyAPI()}
		h := newPoppySetupHarness(t, state)
		installPoppyAsker(t, modeTerminal)

		promptAsk.chooseMany = func(string, []string, []int) ([]int, error) { return nil, terminal.InterruptErr }
		promptAsk.interrupt = func(err error) error { return err }

		args := configArgs(t)
		err := h.run(args...)

		require.ErrorIs(t, err, terminal.InterruptErr)
		assert.NoFileExists(t, outOf(args))
		assert.Contains(t, h.messages.String(), "Cancelled. Nothing was changed.")
	})
}
