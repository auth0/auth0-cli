package portals

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// AuditOptions are the choices that shape an audit.
type AuditOptions struct {
	PortalID string
	// Organization is the ID of the organization a business portal opens in.
	Organization string
	// Strict fails the audit for a warning too.
	Strict bool
	// Fix applies the errors the CLI can fix, without asking.
	Fix bool
}

// AuditDeps is what an audit works with.
type AuditDeps struct {
	API          API
	Reporter     Reporter
	Prompter     Prompter
	Interactive  bool
	TenantDomain string
}

// AuditResult is what an audit found. The order of the fields is the order of its JSON.
type AuditResult struct {
	PortalID string    `json:"portalId"`
	Findings []Finding `json:"findings"`
	Errors   int       `json:"errors"`
	Warnings int       `json:"warnings"`
	Failed   bool      `json:"failed"`
	// Unchecked is what the session was not allowed to read. It does not fail the audit.
	Unchecked []UncheckedCheck `json:"unchecked,omitempty"`
	// Fixes is what each fix did, when the audit fixed anything.
	Fixes []FixRecord `json:"fixes,omitempty"`
}

// Fix statuses.
const (
	fixApplied      = "applied"
	fixFailed       = "failed"
	fixAlreadyFixed = "already_fixed"
)

// FixRecord is what the audit did about a finding.
type FixRecord struct {
	ID      string `json:"id"`
	Subject string `json:"subject,omitempty"`
	Check   string `json:"check"`
	// Status is applied, failed or already_fixed, when the finding was gone by the time its fix ran.
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// UncheckedCheck is something the audit could not read, so the rules that need it did not run.
type UncheckedCheck struct {
	Check  string   `json:"check"`
	Rules  []string `json:"rules"`
	Reason string   `json:"reason"`
}

// AuditOutcome is an audit and the fixes made after it.
type AuditOutcome struct {
	// Result is the audit after the fixes, or the first one when nothing was fixed.
	Result AuditResult
	// FixesFailed is the number of fixes that failed. A fix that failed fails the audit, even when the
	// last audit no longer sees the error.
	FixesFailed int
}

// Failed reports whether the audit failed: it found errors, or warnings in a strict audit, or a fix failed.
func (o AuditOutcome) Failed() bool {
	return o.Result.Failed || o.FixesFailed > 0
}

// portalIDPattern is the shape of the ID of a portal.
var portalIDPattern = regexp.MustCompile(`^up_[A-Za-z0-9]+$`)

// resolvePortalID returns the ID of the portal a person named by its ID or by its slug. Anything else is
// told before a request, because the API answers a malformed ID with a 400 that does not say what is wrong.
func resolvePortalID(ctx context.Context, store PortalStore, name string) (string, error) {
	switch {
	case portalIDPattern.MatchString(name):
		return name, nil
	case IsValidSlug(name):
		portal, err := findPortalBySlug(ctx, store, name)
		if err != nil {
			return "", err
		}

		if portal == nil {
			return "", &UsageError{
				Message: fmt.Sprintf("No portal has the slug %q", name),
				Hints:   []string{"Use the ID of the portal (up_...) or the slug of a portal of this tenant."},
			}
		}

		return portal.ID, nil
	default:
		return "", &UsageError{
			Message: fmt.Sprintf("%q is neither a portal ID (up_...) nor a portal slug", name),
			Hints:   []string{"Use the ID of the portal (up_...) or the slug of a portal of this tenant."},
		}
	}
}

// Audit checks a portal and its application against what setup makes. It can fix the errors the CLI knows
// how to fix: all of them without asking, or the ones a person chooses.
func Audit(ctx context.Context, options AuditOptions, deps AuditDeps) (AuditOutcome, error) {
	portalID, err := resolvePortalID(ctx, deps.API, options.PortalID)
	if err != nil {
		return AuditOutcome{}, err
	}

	options.PortalID = portalID
	run := &auditRun{options: options, deps: deps}

	run.deps.Reporter.Banner("Universal Portals audit", deps.TenantDomain)

	report, err := run.audit(ctx)
	if err != nil {
		return AuditOutcome{}, err
	}

	outcome := AuditOutcome{Result: report}
	fixable := fixableFindings(report.Findings)

	fixes := fixDeps{api: deps.API, reporter: deps.Reporter, read: run.read}

	switch {
	case options.Fix:
		// Scripts: apply the errors the CLI can fix, without asking.
		if len(fixable) > 0 {
			outcome, err = run.fix(ctx, outcome, fixable, fixes, func(exposed []Finding) ([]Finding, error) { return exposed, nil })
		}
	case deps.Interactive && len(fixable) > 0:
		var selected []Finding

		if selected, err = chooseFixes(deps.Prompter, deps.Reporter, fixable); err == nil && len(selected) > 0 {
			outcome, err = run.fix(ctx, outcome, selected, fixes, func(exposed []Finding) ([]Finding, error) {
				deps.Reporter.Detail("These errors appeared after the fixes above.")

				return chooseFixes(deps.Prompter, deps.Reporter, exposed)
			})
		}
	case len(fixable) > 0:
		deps.Reporter.Detail("Run with --fix to apply the errors the CLI can fix.")
	}

	if err != nil {
		return AuditOutcome{}, err
	}

	return outcome, nil
}

// auditRun is one run of Audit.
type auditRun struct {
	options AuditOptions
	deps    AuditDeps
}

func (r *auditRun) read(ctx context.Context) (snapshot, error) {
	return readSnapshot(ctx, r.deps.API, r.options.PortalID, r.deps.TenantDomain, r.options.Organization)
}

// fix applies the fixes and audits the portal again.
func (r *auditRun) fix(
	ctx context.Context,
	outcome AuditOutcome,
	selected []Finding,
	fixes fixDeps,
	chooseNext func(exposed []Finding) ([]Finding, error),
) (AuditOutcome, error) {
	r.deps.Reporter.Section("Fix")

	applied, err := applyFixRounds(ctx, selected, fixes, chooseNext)
	if err != nil {
		return AuditOutcome{}, err
	}

	r.deps.Reporter.Section("Checking the portal again")

	report, err := r.audit(ctx)
	if err != nil {
		return AuditOutcome{}, err
	}

	report.Fixes = applied.records

	return AuditOutcome{Result: report, FixesFailed: outcome.FixesFailed + applied.failed}, nil
}

// audit reads the portal, shows what it found, and returns it.
func (r *auditRun) audit(ctx context.Context) (AuditResult, error) {
	reporter := r.deps.Reporter

	current, err := task(reporter, "Reading the portal", func() (snapshot, error) { return r.read(ctx) })
	if err != nil {
		return AuditResult{}, err
	}

	findings := auditPortal(current)
	if findings == nil {
		findings = []Finding{}
	}

	errors := 0

	for _, finding := range findings {
		if finding.Severity == SeverityError {
			errors++
		}
	}

	warnings := len(findings) - errors

	reporter.Section(fmt.Sprintf("Portal %q", current.Portal.Slug))

	if len(findings) == 0 {
		reporter.Item("No problems found", "")
	} else {
		r.showFindings(findings)
	}

	for _, unchecked := range current.Unchecked {
		reporter.Warn(fmt.Sprintf("Not checked: %s (%s). Skipped %s.", unchecked.Check, unchecked.Reason, strings.Join(unchecked.Rules, ", ")))
	}

	reporter.Newline()

	summary := fmt.Sprintf("%d error%s, %d warning%s", errors, plural(errors), warnings, plural(warnings))

	switch {
	case errors > 0:
		reporter.Error(summary)
	case warnings > 0:
		reporter.Warn(summary)
	default:
		reporter.Item(summary, "")
	}

	return AuditResult{
		PortalID: r.options.PortalID,
		Findings: findings,
		Errors:   errors,
		Warnings: warnings,
		Failed:   errors > 0 || (r.options.Strict && warnings > 0),

		Unchecked: current.Unchecked,
	}, nil
}

func plural(count int) string {
	if count == 1 {
		return ""
	}

	return "s"
}

// showFindings shows the findings in a table, and the commands for the ones the CLI cannot fix.
func (r *auditRun) showFindings(findings []Finding) {
	rows := make([][]string, 0, len(findings))

	for _, finding := range findings {
		symbol := "▲"
		if finding.Severity == SeverityError {
			symbol = "✖"
		}

		fix := finding.Message
		if finding.Remedy != nil {
			fix = finding.Remedy.Summary
		}

		rows = append(rows, []string{symbol, finding.ID, finding.Check, fix})
	}

	r.deps.Reporter.Table([]string{"", "ID", "Check", "Fix"}, rows)

	for _, finding := range findings {
		if finding.Remedy != nil && finding.Remedy.Command != "" && finding.Remedy.apply == nil {
			r.deps.Reporter.Detail(fmt.Sprintf("%s manually: %s", finding.ID, finding.Remedy.Command))
		}
	}
}
