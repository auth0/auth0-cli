package cli

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	cliconfig "github.com/auth0/auth0-cli/internal/config"
	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/poppy/doctor"
)

// errPoppyDoctorFailed is returned after the report is printed when a doctor check failed.
var errPoppyDoctorFailed = errors.New("some doctor checks failed")

// errPoppyDoctorIncomplete is returned after the report is printed when no check
// failed, but some did not run and --strict is set.
var errPoppyDoctorIncomplete = errors.New("some doctor checks did not run and --strict is set")

// errNoPoppySession means there is no tenant login the doctor checks can use
// without asking the user for anything.
var errNoPoppySession = errors.New("no usable tenant login")

// poppyDoctorTestOptions is what tests set to point the doctor at a test server.
// The doctor commands start from it, so it is the zero value outside of tests.
var poppyDoctorTestOptions doctor.Options

var poppyStrict = Flag{
	Name:     "Strict",
	LongForm: "strict",
	Help:     "Exit with 1 when a check could not run, as well as when a check fails.",
}

func poppyDoctorCmd(cli *cli) *cobra.Command {
	var (
		configFile string
		strict     bool
	)

	cmd := &cobra.Command{
		Use:   "doctor [domain]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Check the Poppy discovery chain end to end",
		Long: poppyLong("Checks the whole discovery chain without changing anything, from the files on your website to the settings of " +
			"your tenant. A failure names the check and the section of the protocol, and comes with a FIX line telling you what to change.\n\n" +
			"The checks cover five things: the Poppy files on your website, Auth0's issuer metadata, the settings of your tenant, " +
			"each registered agent, and your config file. They understand sites on protocol 0.3, 0.4 and 1.x, and run the extra checks of " +
			"0.4 and later only on sites that declare one of them.\n\n" +
			"The checks of the website and the issuer need no login, so they also work against other companies. The checks of the tenant, " +
			"the agents and the config file run when you are logged in to a tenant, or use the environment credentials of " +
			"`AUTH0_CLI_AUTH_MODE=env`, and show as skipped otherwise. This command never starts a login by itself: an expired " +
			"login that needs a person only skips them, and an expired client-credentials login is refreshed. A check whose scope " +
			"is missing is skipped with the login command that adds it. When the tenant does not list the domain as a company " +
			"domain, its checks are skipped too, because they would read the wrong tenant.\n\n" +
			"The command exits with 1 when a check fails, and with `--strict` also when a check could not run. " +
			"The JSON report holds every check, with a stable code, and says why a check was skipped."),
		Example: `  auth0 poppy doctor shop.example
  auth0 poppy doctor
  auth0 poppy doctor shop.example --config ./config/poppy.json
  auth0 poppy doctor shop.example --strict
  auth0 poppy doctor shop.example --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			domain, err := poppyDoctorDomain(args, configFile)
			if err != nil {
				return err
			}

			cli.tenantExplicit = cmd.Flags().Changed("tenant")

			opts := poppyDoctorTestOptions
			opts.ConfigFile, opts.ConfigFileExplicit = configFile, cmd.Flags().Changed("config")
			if cli.tenantExplicit {
				opts.TenantFlag = cli.tenant
			}

			if opts.Tenant, err = cli.optionalPoppyTenant(cmd.Context()); err != nil {
				return err
			}

			return runPoppyDoctor(cmd.Context(), cli, domain, opts, strict)
		},
	}

	poppyConfig.RegisterString(cmd, &configFile, config.DefaultFileName)
	poppyStrict.RegisterBool(cmd, &strict, false)
	registerPoppyOutputFlags(cmd, cli)

	return cmd
}

// poppyDoctorTenant gives the doctor checks access to the tenant through the
// Management API clients of a login.
func (c *cli) poppyDoctorTenant() *doctor.Tenant {
	return &doctor.Tenant{Domain: c.tenant, State: c.poppyReader(), Agents: c.poppyAgents(), Policies: c.apiv3.GuardianPolicy}
}

// optionalPoppyTenant returns access to the tenant when a login can be used
// without a person, and nil when it cannot. It never starts a login, asks a
// question or repairs the config: see setupPoppyAuthentication. An error means
// the environment credentials the caller asked for do not work.
func (c *cli) optionalPoppyTenant(ctx context.Context) (*doctor.Tenant, error) {
	switch err := c.setupPoppyAuthentication(ctx); {
	case errors.Is(err, errNoPoppySession):
		return nil, nil
	case err != nil:
		return nil, err
	}

	return c.poppyDoctorTenant(), nil
}

// setupPoppyAuthentication is setupWithAuthentication limited to what needs no
// person. With environment credentials selected it uses them. Otherwise it reads
// the saved login and uses it if its token is valid. An expired token of a
// client-credentials login is refreshed, as setupWithAuthentication does. Any
// other login that is not usable returns errNoPoppySession, because it would need
// a device-code login or a repair of the config.
func (c *cli) setupPoppyAuthentication(ctx context.Context) error {
	if err := validateAuthMode(os.Getenv); err != nil {
		return err
	}

	if envAuthEnabled(os.Getenv) {
		return c.setupWithEnvAuthentication(ctx)
	}

	if err := c.Config.Initialize(); err != nil {
		return fmt.Errorf("%w: %w", errNoPoppySession, err)
	}

	tenant, err := c.Config.GetTenant(cmp.Or(c.tenant, c.Config.DefaultTenant))
	if err != nil {
		return fmt.Errorf("%w: %w", errNoPoppySession, err)
	}

	accessToken, err := tenant.CheckAuthenticationStatus()
	switch {
	case err == nil:
	case errors.Is(err, cliconfig.ErrInvalidToken) && tenant.IsAuthenticatedWithClientCredentials():
		if err := refreshClientCredentials(ctx, &tenant); err != nil {
			return fmt.Errorf("%w: %w", errNoPoppySession, err)
		}

		// The new token is already stored. Saving the config keeps its expiry, so
		// the next run does not refresh again; the token is usable without it.
		_ = c.Config.AddTenant(tenant)
		accessToken = tenant.GetAccessToken()
	default:
		return fmt.Errorf("%w: %w", errNoPoppySession, err)
	}

	if err := c.initManagementClients(tenant.Domain, accessToken); err != nil {
		return err
	}

	c.tenant = tenant.Domain

	return nil
}

// poppyDoctorDomain returns the normalized company domain to check: the
// argument, else organization.domain of the config file when it exists.
func poppyDoctorDomain(args []string, configFile string) (string, error) {
	if len(args) == 1 {
		domain, err := config.NormalizeDomain(args[0])
		if err != nil {
			return "", validationError{err: err}
		}

		return domain, nil
	}

	cfg, err := config.ReadLenient(configFile)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", validationError{err: fmt.Errorf("pass a domain, such as auth0 poppy doctor shop.example, or run the command where %s exists", configFile)}
	case err != nil:
		return "", err
	}

	domain, err := config.NormalizeDomain(cfg.Organization.Domain)
	if err != nil {
		return "", validationError{err: fmt.Errorf("organization.domain of %s: %w", configFile, err)}
	}

	return domain, nil
}

// runPoppyDoctor prints the doctor report and fails when a check failed, or when
// strict is set and a check could not run.
func runPoppyDoctor(ctx context.Context, cli *cli, domain string, opts doctor.Options, strict bool) error {
	var report doctor.Report

	run := func() error {
		report = doctor.Run(ctx, domain, opts)

		return nil
	}

	// The spinner goes to the messages stream, and a script that asked for JSON
	// should see only the report.
	if cli.wantsJSONError() {
		_ = run()
	} else {
		_ = ansi.Waiting(run)
	}

	cli.renderer.PoppyDoctor(report)

	return poppyDoctorError(report, strict)
}

// poppyDoctorError is the error of a report that has a failed check, or a skipped
// one when strict is set. The counts are in its details.
func poppyDoctorError(report doctor.Report, strict bool) error {
	var err error

	switch {
	case report.ExitCode() != 0:
		err = errPoppyDoctorFailed
	case strict && report.StrictExitCode() != 0:
		err = errPoppyDoctorIncomplete
	default:
		return nil
	}

	details, _ := json.Marshal(map[string]int{
		"failed":   report.Count(doctor.Fail),
		"warnings": report.Count(doctor.Warn),
		"skipped":  report.Count(doctor.Skipped),
	})

	return validationError{err: err, reason: "doctor_checks_failed", details: details}
}
