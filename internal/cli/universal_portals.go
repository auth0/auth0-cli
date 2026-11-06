package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/portals"
)

var (
	upTemplate = Flag{
		Name:     "Template",
		LongForm: "template",
		Help:     "Kind of portal: consumer, for end-users to manage their account, or business, for organization administrators to manage their organization. Asked when omitted.",
	}
	upName = Flag{
		Name:      "Name",
		LongForm:  "name",
		ShortForm: "n",
		Help:      "Name of the portal. Asked when omitted.",
	}
	upSlug = Flag{
		Name:      "Slug",
		LongForm:  "slug",
		ShortForm: "s",
		Help:      "URL slug of the portal, for example acme-account. Defaults to a slug made from the name.",
	}
	upUser = Flag{
		Name:      "User",
		LongForm:  "user",
		ShortForm: "u",
		Help:      "Business only: the email or user ID of the user who will test the portal.",
	}
	upOrganization = Flag{
		Name:      "Organization",
		LongForm:  "organization",
		ShortForm: "o",
		Help:      "Business only: the ID or name of the organization, when the user has several.",
	}
)

// universalPortalsCmd groups the Universal Portals commands.
func universalPortalsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "universal-portals",
		Aliases: []string{"up"},
		Short:   "Set up Universal Portals",
		Long:    "Set up Auth0 Universal Portals.",
	}

	cmd.SetUsageTemplate(namespaceUsageTemplate())
	cmd.AddCommand(universalPortalsSetupCmd(cli))

	return cmd
}

// universalPortalsSetupInputs are the flags of setup.
type universalPortalsSetupInputs struct {
	Template     string
	Name         string
	Slug         string
	User         string
	Organization string
}

// options turns the flags into the options of setup, and checks them.
func (i universalPortalsSetupInputs) options() (portals.SetupOptions, error) {
	template := portals.Template(i.Template)
	if i.Template != "" && !template.IsValid() {
		return portals.SetupOptions{}, usageError{
			err:    fmt.Errorf("unknown template %q: use consumer or business", i.Template),
			reason: "invalid_input",
		}
	}

	return portals.SetupOptions{
		Template:     template,
		Name:         i.Name,
		Slug:         i.Slug,
		UserQuery:    i.User,
		Organization: i.Organization,
	}, nil
}

// missingForNoPrompt returns the flags a run that cannot ask has to be given.
func (i universalPortalsSetupInputs) missingForNoPrompt() []string {
	var missing []string

	if i.Template == "" {
		missing = append(missing, "--template")
	}

	if i.Name == "" {
		missing = append(missing, "--name")
	}

	if i.Template == string(portals.TemplateBusiness) && i.User == "" {
		missing = append(missing, "--user")
	}

	return missing
}

func universalPortalsSetupCmd(cli *cli) *cobra.Command {
	var inputs universalPortalsSetupInputs

	cmd := &cobra.Command{
		Use:   "setup",
		Args:  cobra.NoArgs,
		Short: "Set up a Universal Portal",
		Long: `Creates a Universal Portal with its application, forms and pages, and checks that the logged-in session has the permissions it needs before it creates anything.

Running the command again after a failure resumes: what exists is reused.`,
		Example: `  auth0 universal-portals setup
  auth0 universal-portals setup --template business --name "Acme Admin" --user jane@acme.com
  auth0 up setup --no-input --template consumer --name "Acme Account" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUniversalPortalsSetup(cmd, cli, inputs)
		},
	}

	upTemplate.RegisterString(cmd, &inputs.Template, "")
	upName.RegisterString(cmd, &inputs.Name, "")
	upSlug.RegisterString(cmd, &inputs.Slug, "")
	upUser.RegisterString(cmd, &inputs.User, "")
	upOrganization.RegisterString(cmd, &inputs.Organization, "")
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

	return cmd
}

// portalsBackend is the tenant a command works on, and whether it changed it.
type portalsBackend interface {
	portals.API
	hasWritten() bool
}

// runUniversalPortalsSetup runs setup on the tenant of the CLI.
func runUniversalPortalsSetup(cmd *cobra.Command, cli *cli, inputs universalPortalsSetupInputs) error {
	return runUniversalPortalsSetupOn(cmd, cli, inputs, newPortalsTenant(cli.api, cli.apiv3))
}

// runUniversalPortalsSetupOn runs setup. It takes over the interrupt from the root command for the
// time it runs, so it can say what was and was not done when the person cancels.
func runUniversalPortalsSetupOn(
	cmd *cobra.Command,
	cli *cli,
	inputs universalPortalsSetupInputs,
	tenant portalsBackend,
) error {
	options, err := inputs.options()
	if err != nil {
		return err
	}

	interactive := canPrompt(cmd)
	if missing := inputs.missingForNoPrompt(); !interactive && len(missing) > 0 {
		return usageError{
			err:    fmt.Errorf("missing required flags for a non-interactive run: %s", strings.Join(missing, ", ")),
			reason: "missing_required_flags",
		}
	}

	ctx, stop := portalsInterruptContext(cmd.Context())
	reporter := newPortalsTerminal(
		cli.renderer.MessageWriter,
		ansiPortalsStyle(),
		cli.json || cli.renderer.AgentMode,
		true,
	)

	result, err := executeUniversalPortalsSetup(ctx, universalPortalsSetup{
		options: options,
		deps: portals.SetupDeps{
			API:          tenant,
			Reporter:     reporter,
			Prompter:     portalsPrompter{},
			Interactive:  interactive,
			TenantDomain: cli.tenant,
		},
	})

	interrupted := ctx.Err() != nil || errors.Is(err, portals.ErrCancelled)
	stop()

	if interrupted {
		reportPortalsCancelled(reporter, tenant.hasWritten())
		os.Exit(exitInterrupted)
	}

	if err != nil {
		return portalsCommandError(err, tenant.hasWritten())
	}

	if cli.json {
		cli.renderer.JSONResult(result)

		return nil
	}

	reporter.Ready(result.Portal.URL)

	if interactive {
		openPortal(cli, result.Portal.URL)
	}

	return nil
}

// universalPortalsSetup is one run of setup.
type universalPortalsSetup struct {
	options portals.SetupOptions
	deps    portals.SetupDeps
}

// executeUniversalPortalsSetup shows the title, asks for what is missing, and sets the portal up.
func executeUniversalPortalsSetup(ctx context.Context, run universalPortalsSetup) (portals.SetupResult, error) {
	run.deps.Reporter.Banner("Universal Portals setup", run.deps.TenantDomain)

	options := run.options

	if run.deps.Interactive {
		var err error

		if options, err = portals.AskForMissing(options, run.deps.Prompter); err != nil {
			return portals.SetupResult{}, err
		}
	}

	if options.Slug == "" {
		options.Slug = portals.SlugFromName(options.Name)
	}

	return portals.Setup(ctx, options, run.deps)
}

// openPortal offers to open the portal in the browser. A cancel means no: the portal is already set
// up, so cancelling this question is not a cancelled setup.
func openPortal(cli *cli, url string) {
	open, err := portalsPrompter{}.Confirm("Open the portal in your browser?", true)
	if err != nil || !open {
		return
	}

	if err := browser.OpenURL(url); err != nil {
		cli.renderer.Warnf("Could not open the browser: %s\nOpen the portal yourself: %s", err, ansi.URL(url))
	}
}

// reportPortalsCancelled says what a cancel left in the tenant.
func reportPortalsCancelled(reporter portals.Reporter, wrote bool) {
	if wrote {
		reporter.Warn("Cancelled. Nothing was rolled back: what was already created is still in your tenant.")
		reporter.Warn(portalsResumeHint)
	} else {
		reporter.Warn("Cancelled. Nothing was created.")
	}

	reporter.Cancelled("Setup cancelled.")
}

// portalsInterruptContext returns a context that ends when the process is interrupted. The root
// command exits at once on an interrupt, which would cut the command off before it could say what it
// left behind, so the command takes the signal over for the time it runs.
func portalsInterruptContext(parent context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancel(parent)

	signal.Reset(os.Interrupt, syscall.SIGTERM)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	go func() {
		select {
		case <-signals:
			cancel()
		case <-ctx.Done():
		}
	}()

	return ctx, func() {
		signal.Stop(signals)
		cancel()
	}
}
