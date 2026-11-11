package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/portals"
)

var (
	upAuditOrganization = Flag{
		Name:      "Organization",
		LongForm:  "organization",
		ShortForm: "o",
		Help:      "Business only: the ID of the organization the portal opens in, to check that it shares a connection with the application.",
	}
	upFix = Flag{
		Name:     "Fix",
		LongForm: "fix",
		Help:     "Apply the errors the CLI can fix, without asking. Warnings are never changed.",
	}
	upStrict = Flag{
		Name:     "Strict",
		LongForm: "strict",
		Help:     "Also fail when there are warnings.",
	}
)

// universalPortalsAuditInputs are the flags of audit.
type universalPortalsAuditInputs struct {
	Organization string
	Fix          bool
	Strict       bool
}

func universalPortalsAuditCmd(cli *cli) *cobra.Command {
	var inputs universalPortalsAuditInputs

	cmd := &cobra.Command{
		Use:   "audit <portal-id>",
		Short: "Check a Universal Portal",
		Long: `Checks a Universal Portal and its application against what setup makes, and tells what is wrong and how to fix it.

An error breaks the portal or one of its components, and a warning is something degraded or different from the setup guide. The command fails when there is an error, and with --strict when there is a warning too.

With a terminal it offers to fix the errors the CLI can fix, and with --fix it fixes them without asking. Warnings are never changed, because they may be intentional.`,
		Example: `  auth0 universal-portals audit up_123
  auth0 universal-portals audit up_123 --fix
  auth0 up audit up_123 --organization org_123 --strict --json`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageError{
					err:    errors.New("audit needs the ID of one portal: auth0 universal-portals audit <portal-id> [--organization <id>] [--strict]"),
					reason: "missing_required_args",
				}
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUniversalPortalsAudit(cmd, cli, args[0], inputs)
		},
	}

	upAuditOrganization.RegisterString(cmd, &inputs.Organization, "")
	upFix.RegisterBool(cmd, &inputs.Fix, false)
	upStrict.RegisterBool(cmd, &inputs.Strict, false)
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

	return cmd
}

// runUniversalPortalsAudit audits a portal of the tenant of the CLI.
func runUniversalPortalsAudit(cmd *cobra.Command, cli *cli, portalID string, inputs universalPortalsAuditInputs) error {
	return runUniversalPortalsAuditOn(cmd, cli, portalID, inputs, newPortalsTenant(cli.api, cli.apiv3))
}

// runUniversalPortalsAuditOn runs an audit. It takes over the interrupt from the root command for the
// time it runs, so it can say what was and was not done when the person cancels.
func runUniversalPortalsAuditOn(
	cmd *cobra.Command,
	cli *cli,
	portalID string,
	inputs universalPortalsAuditInputs,
	api portals.API,
) error {
	ctx, stop := portalsInterruptContext(cmd.Context())
	reporter := newPortalsTerminal(
		cli.renderer.MessageWriter,
		ansiPortalsStyle(),
		cli.json || cli.renderer.AgentMode,
		true,
	)

	outcome, err := portals.Audit(ctx, portals.AuditOptions{
		PortalID:     portalID,
		Organization: inputs.Organization,
		Strict:       inputs.Strict,
		Fix:          inputs.Fix,
	}, portals.AuditDeps{
		API:          api,
		Reporter:     reporter,
		Prompter:     portalsPrompter{},
		Interactive:  canPrompt(cmd),
		TenantDomain: cli.tenant,
	})

	interrupted := ctx.Err() != nil || errors.Is(err, portals.ErrCancelled)
	stop()

	if interrupted {
		reporter.Warn("Cancelled. Fixes already applied stay in your tenant.")
		reporter.Cancelled("Audit cancelled.")
		os.Exit(exitInterrupted)
	}

	if err != nil {
		return portalsCommandError(err, cli.tenant, false)
	}

	if cli.json {
		cli.renderer.JSONResult(outcome.Result)
	}

	if outcome.Failed() {
		return validationError{err: auditFailure(outcome, inputs.Strict), reason: "audit_failed"}
	}

	return nil
}

// auditFailure says why an audit failed. The findings were shown already, so this is the short reason
// for the exit code.
func auditFailure(outcome portals.AuditOutcome, strict bool) error {
	var reasons []string

	if outcome.Result.Errors > 0 {
		reasons = append(reasons, fmt.Sprintf("%d error(s)", outcome.Result.Errors))
	}

	if strict && outcome.Result.Warnings > 0 {
		reasons = append(reasons, fmt.Sprintf("%d warning(s) in a strict audit", outcome.Result.Warnings))
	}

	if outcome.FixesFailed > 0 {
		var failed []string

		for _, fix := range outcome.Result.Fixes {
			if fix.Status == "failed" {
				failed = append(failed, fix.ID)
			}
		}

		reason := fmt.Sprintf("%d fix(es) failed", outcome.FixesFailed)
		if len(failed) > 0 {
			reason += " (" + strings.Join(failed, ", ") + ")"
		}

		reasons = append(reasons, reason)
	}

	return fmt.Errorf("the audit failed: %s", strings.Join(reasons, ", "))
}
