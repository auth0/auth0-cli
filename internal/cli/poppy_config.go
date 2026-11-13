package cli

import (
	"bytes"
	"errors"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/poppy/setup"
)

// poppyConfigCommandOptions are the options that rebuild the config command being
// run, for the example in an error.
func (c *cli) poppyConfigCommandOptions() setup.CommandOptions {
	return setup.CommandOptions{Tenant: c.tenant, Config: true}
}

func poppyConfigCmd(cli *cli) *cobra.Command {
	var (
		flags       setup.Flags
		webUnscoped bool
	)

	cmd := &cobra.Command{
		Use:   "config",
		Args:  cobra.NoArgs,
		Short: "Rebuild the Poppy config file from the tenant",
		Long: poppyLong("Rebuilds the config file from the tenant without changing the tenant, so CI and deploys can refresh it safely.\n\n" +
			"It reads the tenant, adds the values Auth0 does not store, and writes a protocol 1.0 `poppy.json` document, which your web SDK can serve as `/.well-known/poppy.json`. A file written for 0.3 or 0.4 is upgraded, and members of the file that the CLI does not use are kept. If the tenant is not set up for Poppy, it exits with an error naming the missing setting.\n\n" +
			"The file is for one website, so with several company domains pass `--company-domain` to pick the one this app serves. Without `--api`, the APIs are the Poppy APIs that the existing file points to, or the only Poppy API of the tenant. Without `--app`, the web app is the only app that has the browser join on. When there are several, a terminal lets you pick, and otherwise the command stops and asks for `--api` or `--app`.\n\n" +
			"In a terminal, it also asks whether signing in to the browser gives an agent full access to the user's account on your website (`web.unscoped`), unless you pass `--web-unscoped` or the file already has a value."),
		Example: `  auth0 poppy config
  auth0 poppy config --company-domain shop.example --out ./config/poppy.json
  auth0 poppy config --company-domain shop.example --api https://api.shop.example --app "Shop Web" --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if poppyWebUnscoped.IsSet(cmd) {
				flags.WebUnscoped = &webUnscoped
			}

			return runPoppyConfig(cmd, cli, flags)
		},
	}

	poppyCompanyDomain.RegisterString(cmd, &flags.CompanyDomain, "")
	poppyAPI.RegisterStringArray(cmd, &flags.APIs, nil)
	poppyAPIURL.RegisterStringArray(cmd, &flags.APIURLs, nil)
	poppyApp.RegisterString(cmd, &flags.App, "")
	poppyConfigNoAPI.RegisterBool(cmd, &flags.NoAPI, false)
	poppyConfigNoWeb.RegisterBool(cmd, &flags.NoWeb, false)
	poppyHint.RegisterStringArray(cmd, &flags.Hints, nil)
	poppyCompanyName.RegisterString(cmd, &flags.Name, "")
	poppyOut.RegisterString(cmd, &flags.Out, config.DefaultFileName)
	poppyWebUnscoped.RegisterBool(cmd, &webUnscoped, false)
	poppyOverwriteConfig.RegisterBool(cmd, &flags.ForceConfig, false)
	registerPoppyOutputFlags(cmd, cli)

	return cmd
}

func runPoppyConfig(cmd *cobra.Command, cli *cli, flags setup.Flags) error {
	in, err := flags.Parse()
	if err != nil {
		return poppySetupError(err)
	}

	screen := cli.newPromptScreen("Poppy config")
	screen.ForDomain(in.CompanyDomain)

	if err := cli.checkPoppyScopes(setup.ReadScopes); err != nil {
		return err
	}

	file, err := readPoppyConfigLenient(in.Out)
	if err != nil {
		return err
	}

	if err := in.UseConfigFile(file); err != nil {
		return poppySetupError(err)
	}

	apis, app, err := cli.resolvePoppyAPIsAndApp(cmd, screen, &in, file)
	if err != nil {
		return err
	}

	state, err := cli.readPoppyState(cmd.Context(), apis, app, file)
	if err != nil {
		return err
	}

	domain, choices := state.CompanyDomain(in)
	if len(choices) > 1 {
		if domain, err = askPoppyCompanyDomain(cmd, screen, choices, in, cli.poppyConfigCommandOptions()); err != nil {
			return err
		}
	}
	in.CompanyDomain = domain
	screen.ForDomain(domain)

	if err := setup.Ready(state, in); err != nil {
		return poppySetupError(err)
	}

	// A file of another issuer is refused before any hint is asked.
	if err := setup.CheckIssuer(state, in); err != nil {
		return poppySetupError(err)
	}

	unhinted := 0
	if promptAsk.canPrompt(cmd) {
		if err := askPoppyMissingValues(screen, state, &in); err != nil {
			return err
		}

		if unhinted, err = askPoppyHints(screen, setup.HintScopes(state, in, false), &in); err != nil {
			return err
		}
	}

	if err := askPoppyWebUnscoped(screen, promptAsk.canPrompt(cmd), state, &in); err != nil {
		return err
	}

	cfg, err := setup.Derive(state, in)
	if err != nil {
		return poppyBuildError(err, in, cli.poppyConfigCommandOptions())
	}

	before, readErr := os.ReadFile(in.Out)

	if err := config.Write(in.Out, cfg); err != nil {
		return err
	}

	if !cli.renderer.PoppyHuman() {
		cli.renderer.PoppyJSON(cfg)

		return nil
	}

	after, err := os.ReadFile(in.Out)
	unchanged := readErr == nil && err == nil && bytes.Equal(before, after)

	cli.renderer.PoppyConfigResult(in, unchanged, screen.HeaderShown(), unhinted)

	return nil
}

// askPoppyMissingValues asks, in a terminal, for the values the config file
// needs that neither Auth0, the flags nor the existing file has, such as the
// company name or the OpenAPI URL of an API. A value no question gives is left
// to the error of setup.Derive.
func askPoppyMissingValues(screen *questionScreen, state *setup.State, in *setup.Inputs) error {
	asked := map[string]bool{}

	for {
		var missing *config.MissingFieldsError
		if _, err := setup.Derive(state, *in); !errors.As(err, &missing) {
			return nil
		}

		var fields []string
		for _, field := range missing.Fields {
			if !asked[field] {
				fields = append(fields, field)
				asked[field] = true
			}
		}

		questions, _ := setup.QuestionsForMissing(&config.MissingFieldsError{Fields: fields})
		if len(questions) == 0 {
			return nil
		}

		if err := askPoppyQuestions(screen, in, questions); err != nil {
			return err
		}
	}
}

// askPoppyCompanyDomain picks one of the company domains the tenant lists.
func askPoppyCompanyDomain(cmd *cobra.Command, screen *questionScreen, choices []string, in setup.Inputs, commandOpts setup.CommandOptions) (domain string, err error) {
	if !promptAsk.canPrompt(cmd) {
		return "", poppyMissingFlagsError(in, commandOpts, []setup.Question{{
			Flag:   "--company-domain",
			Reason: "The tenant lists several company domains (" + strings.Join(choices, ", ") + "), and the file is for the one website your app serves.",
			Prompt: "domain",
		}})
	}

	err = screen.ask("Company domain", "The tenant lists several company domains, and the config file is for the one website your app serves.", func() error {
		picked, err := screen.asker.choose("Company domain this app serves", choices, 0)
		if err == nil {
			domain = choices[picked]
		}

		return err
	})

	return domain, err
}

// resolvePoppyAPIsAndApp returns the APIs and the web app to put in the config
// file: the ones given by flags, else the ones the tenant and the existing file
// settle on. The file names neither, so the APIs are the Poppy APIs of the
// tenant that its apis entries are at, else the only Poppy API of the tenant, and
// the app is the only app that has the browser join on. In a terminal, a choice of
// several is picked. The question whether a part is wanted is asked only when the
// tenant has something to offer for it. A part that is off has neither. It stops
// with a FIX line when that is not one, or names the flags to pass when both the
// APIs and the app are missing.
func (c *cli) resolvePoppyAPIsAndApp(cmd *cobra.Command, screen *questionScreen, in *setup.Inputs, file *config.Config) (apis []string, app string, err error) {
	apis, app = in.APIs, in.App
	needAPIs, needApp := len(apis) == 0 && !in.NoAPI, app == "" && !in.NoWeb

	if !needAPIs && !needApp {
		return apis, app, nil
	}

	var (
		poppy  []setup.API
		joined []setup.App
	)

	reader := c.poppyReader()

	err = ansi.Waiting(func() (err error) {
		if needAPIs {
			if poppy, err = reader.ListPoppyAPIs(cmd.Context()); err != nil {
				return err
			}
		}

		if needApp {
			joined, err = reader.BrowserJoinApps(cmd.Context())
		}

		return err
	})
	if err != nil {
		return nil, "", c.poppyError(poppySetupError(err))
	}

	// Asking whether a part is wanted, only to find there is nothing to use, is a trap.
	interactive := promptAsk.canPrompt(cmd)

	if needAPIs && len(poppy) > 0 {
		if err = askPoppyPart(screen, interactive, setup.PartAPI, in); err != nil {
			return nil, "", err
		}

		needAPIs = !in.NoAPI
	}

	if needApp && len(joined) > 0 {
		if err = askPoppyPart(screen, interactive, setup.PartWeb, in); err != nil {
			return nil, "", err
		}

		needApp = !in.NoWeb
	}

	var apiErr, appErr error

	if needAPIs {
		if apis, apiErr = c.resolvePoppyAPIs(cmd, screen, file, poppy); apiErr != nil && !isPoppyInputError(apiErr) {
			return nil, "", apiErr
		}
	}

	if needApp {
		if app, appErr = c.resolvePoppyApp(cmd, screen, joined); appErr != nil && !isPoppyInputError(appErr) {
			return nil, "", appErr
		}
	}

	switch {
	case apiErr != nil && appErr != nil:
		return nil, "", poppyMissingFlagsError(*in, c.poppyConfigCommandOptions(), []setup.Question{
			{Flag: "--api", Reason: poppyInputMessage(apiErr), Prompt: "api"},
			{Flag: "--app", Reason: poppyInputMessage(appErr), Prompt: "app"},
		})
	case apiErr != nil:
		return nil, "", c.poppyError(poppySetupError(apiErr))
	case appErr != nil:
		return nil, "", c.poppyError(poppySetupError(appErr))
	}

	return apis, app, nil
}

// isPoppyInputError tells whether err is a problem with the inputs, which the
// command can report together with another.
func isPoppyInputError(err error) bool {
	var input *setup.InputError

	return errors.As(err, &input)
}

// poppyInputMessage returns the message of an input error without its FIX line.
func poppyInputMessage(err error) string {
	var input *setup.InputError
	if errors.As(err, &input) {
		return input.Message
	}

	return err.Error()
}

// resolvePoppyAPIs settles on the APIs of the config file. The existing file
// names them. Without that, the only Poppy API of the tenant is the one, and of
// several a terminal picks.
func (c *cli) resolvePoppyAPIs(cmd *cobra.Command, screen *questionScreen, file *config.Config, poppy []setup.API) ([]string, error) {
	apis, err := setup.ResolveAPIs(file, poppy)
	if err == nil {
		return apis, nil
	}

	var input *setup.InputError
	if !errors.As(err, &input) || input.Reason != setup.ReasonAPINotFound || len(poppy) == 0 {
		return nil, err
	}

	if len(poppy) == 1 {
		return []string{poppy[0].Identifier}, nil
	}

	if !promptAsk.canPrompt(cmd) {
		return nil, err
	}

	opts, selected := poppyAPIOptions(poppy)

	return screen.pickAPIs(opts, selected)
}

// resolvePoppyApp settles on the web app: the only app that has the browser join
// on, and of several a terminal picks.
func (c *cli) resolvePoppyApp(cmd *cobra.Command, screen *questionScreen, joined []setup.App) (string, error) {
	app, err := setup.ResolveApp(joined)
	if err == nil {
		return app, nil
	}

	var input *setup.InputError
	if !errors.As(err, &input) || input.Reason != setup.ReasonAmbiguousApp || !promptAsk.canPrompt(cmd) {
		return "", err
	}

	// The apps are listed again for their type, which the join list does not give.
	var listed []poppyWebApp

	_ = ansi.Waiting(func() (err error) {
		listed, err = c.listPoppyWebApps(cmd.Context())

		return err
	})

	apps := make([]poppyWebApp, len(joined))
	for i, app := range joined {
		apps[i] = poppyWebApp{ClientID: app.ClientID, Name: app.Name, BrowserJoin: true}

		if j := slices.IndexFunc(listed, func(l poppyWebApp) bool { return l.ClientID == app.ClientID }); j >= 0 {
			apps[i].AppType = listed[j].AppType
		}
	}

	return screen.pickApp(poppyAppOptions(apps))
}
