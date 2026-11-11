package portals

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// A fix can expose errors the first audit could not see: a new application has no client grant, so
// linking it makes P019 visible. After each pass, the next errors are chosen, and none ends the run. A
// finding is attempted once, and the passes stop after maxFixRounds so a fix that keeps breaking another
// cannot loop.
const maxFixRounds = 3

// fixDeps is what the fixes work with.
type fixDeps struct {
	api      API
	reporter Reporter
	// The portal is read again before each fix, so a fix never works on a stale copy and never
	// overwrites a change made by an earlier fix.
	read func(ctx context.Context) (snapshot, error)
}

// fixResult is what a pass of fixes did.
type fixResult struct {
	applied int
	failed  int
	// The fixes that were no longer needed by the time they ran.
	alreadyFixed int
	// The scopes the failed fixes needed, so the fix for them is shown once at the end.
	missingScopes []string
	// records say what each fix did, in the order they ran.
	records []FixRecord
}

// fixableFindings returns the findings the CLI fixes. Only errors are fixed: warnings may be
// intentional, so they stay in the report and are never changed.
func fixableFindings(findings []Finding) []Finding {
	return slices.DeleteFunc(slices.Clone(findings), func(finding Finding) bool {
		return finding.Severity != SeverityError || !finding.canFix()
	})
}

// applyFixes applies the fixes of the findings one by one.
func applyFixes(ctx context.Context, selected []Finding, deps fixDeps) (fixResult, error) {
	var result fixResult

	for _, finding := range selected {
		current, err := deps.read(ctx)
		if err != nil {
			return result, err
		}

		fresh, found := findFinding(auditPortal(current), finding)
		if !found || !fresh.canFix() {
			result.alreadyFixed++
			result.records = append(result.records, FixRecord{ID: finding.ID, Subject: finding.Subject, Check: finding.Check, Status: fixAlreadyFixed})

			deps.reporter.Item(fmt.Sprintf("%s %s", finding.ID, finding.Check), "already fixed")

			continue
		}

		// Each fix shows the finding it resolves, with a spinner while it runs.
		label := fmt.Sprintf("%s %s", fresh.ID, fresh.Check)

		err = deps.reporter.Task("Fixing "+fresh.Check, func() error { return fresh.Remedy.apply(ctx, deps.api) })
		if err != nil {
			// A cancel or an interrupt ends the audit, and is not a fix that failed.
			if ctx.Err() != nil {
				return result, ctx.Err()
			}

			result.failed++
			result.records = append(result.records, FixRecord{ID: fresh.ID, Subject: fresh.Subject, Check: fresh.Check, Status: fixFailed, Error: err.Error()})

			deps.reporter.Error(fmt.Sprintf("%s: %s", label, err))

			if gap, isGap := ParseScopeGap(label, err); isGap {
				result.missingScopes = append(result.missingScopes, gap.Required()...)
			}

			continue
		}

		result.applied++
		result.records = append(result.records, FixRecord{ID: fresh.ID, Subject: fresh.Subject, Check: fresh.Check, Status: fixApplied})

		deps.reporter.Fixed(label, fresh.Remedy.Summary)
	}

	return result, nil
}

func findFinding(findings []Finding, wanted Finding) (Finding, bool) {
	index := slices.IndexFunc(findings, func(candidate Finding) bool { return candidate.key() == wanted.key() })
	if index < 0 {
		return Finding{}, false
	}

	return findings[index], true
}

// applyFixRounds applies the fixes, and then those of the errors they exposed that chooseNext picks.
func applyFixRounds(
	ctx context.Context,
	selected []Finding,
	deps fixDeps,
	chooseNext func(exposed []Finding) ([]Finding, error),
) (fixResult, error) {
	var total fixResult

	attempted := map[string]bool{}
	pending := selected

	for round := 0; round < maxFixRounds && len(pending) > 0; round++ {
		for _, finding := range pending {
			attempted[finding.key()] = true
		}

		result, err := applyFixes(ctx, pending, deps)

		total.applied += result.applied
		total.failed += result.failed
		total.alreadyFixed += result.alreadyFixed
		total.records = append(total.records, result.records...)
		total.missingScopes = append(total.missingScopes, result.missingScopes...)

		if err != nil {
			return total, err
		}

		current, err := deps.read(ctx)
		if err != nil {
			return total, err
		}

		exposed := fixableFindings(auditPortal(current))
		exposed = slices.DeleteFunc(exposed, func(finding Finding) bool { return attempted[finding.key()] })

		pending = nil

		if len(exposed) > 0 {
			if pending, err = chooseNext(exposed); err != nil {
				return total, err
			}
		}
	}

	slices.Sort(total.missingScopes)
	total.missingScopes = slices.Compact(total.missingScopes)

	if total.failed > 0 {
		reportFailedFixes(deps.reporter, total)
	}

	return total, nil
}

// reportFailedFixes sums up the fixes that failed, with how to get the scopes they needed, once.
func reportFailedFixes(reporter Reporter, total fixResult) {
	missing := ""
	if len(total.missingScopes) > 0 {
		missing = ", missing: " + strings.Join(total.missingScopes, ", ")
	}

	reporter.Error(fmt.Sprintf("%d fix(es) failed%s", total.failed, missing))

	if len(total.missingScopes) > 0 {
		for _, fix := range ScopeFixes(total.missingScopes) {
			reporter.Detail(fix)
		}
	}
}

// chooseFixes asks what to do with the errors: fix every one, choose the fixes, or skip. The errors
// start checked in the list.
func chooseFixes(prompter Prompter, reporter Reporter, errors []Finding) ([]Finding, error) {
	reporter.Detail("Warnings are not changed. They may be intentional.")

	picked, err := prompter.Choose("What do you want to fix?", []Choice{
		{Label: "Fix all errors", Hint: "recommended"},
		{Label: "Choose errors to fix", Hint: "every error starts checked"},
		{Label: "Skip", Hint: "leave the portal as it is"},
	})
	if err != nil {
		return nil, err
	}

	switch picked {
	case 0:
		return errors, nil
	case 1:
		items := make([]ChecklistItem, 0, len(errors))
		for _, finding := range errors {
			items = append(items, ChecklistItem{Label: fmt.Sprintf("%s %s", finding.ID, finding.Remedy.Summary), Checked: true})
		}

		indexes, err := prompter.Checklist("Errors to fix", items)
		if err != nil {
			return nil, err
		}

		chosen := make([]Finding, 0, len(indexes))
		for _, index := range indexes {
			chosen = append(chosen, errors[index])
		}

		return chosen, nil
	default:
		return nil, nil
	}
}
