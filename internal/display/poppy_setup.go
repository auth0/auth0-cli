package display

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/poppy/setup"
)

const (
	// The width of the label column of the Checked block.
	poppyCheckedLabelWidth = 10

	// The width of the label column of the answers block.
	poppyAnswerLabelWidth = 14

	// The most the column of the changes is wide before a note moves below.
	poppyMaxChangeColumn = 24

	// The indent of the lines that go under a labeled line.
	poppyBelowIndent = "      "

	poppyToApplyLabel = "To apply:  "
	poppyToRerunLabel = "To rerun:  "

	// The most hint lines the answers block shows. Past it, only the first
	// poppyMaxHintRowsShown are shown, then how many scopes are left out.
	poppyMaxHintRows      = 5
	poppyMaxHintRowsShown = 3
)

// PoppySetupOptions says what the text of a setup is written for.
type PoppySetupOptions struct {
	// Inputs are the flag values and the answers the plan was built from. They
	// give the answers block and the command that applies the plan.
	Inputs setup.Inputs
	// DryRun adds the notice that nothing is changed and the command that applies
	// the plan.
	DryRun bool
	// Command holds the flags of the command that the inputs do not.
	Command setup.CommandOptions
	// NoHeader leaves out the title line, which a run that asked questions has
	// already printed above them.
	NoHeader bool
	// Unhinted is the number of scopes that were left without hints on purpose,
	// which the next steps say how to set.
	Unhinted int
}

// PoppyPlan writes the plan for a person to read: what was checked, the answers
// given, then the changes grouped by target, tenant-wide ones first, each from its
// value now to its value after. The width follows the result writer.
func (r *Renderer) PoppyPlan(plan setup.Plan, opts PoppySetupOptions) {
	r.poppyWrite(func(l Layout) string { return poppyPlanText(l, plan, opts) })
}

// PoppySetupFailure writes the steps of a setup that stopped partway: the ones
// made, the one that failed and the ones not run, and the command that continues.
func (r *Renderer) PoppySetupFailure(plan setup.Plan, outcome setup.Outcome, opts PoppySetupOptions) {
	r.poppyWrite(func(l Layout) string { return "\n" + poppyApplyText(l, plan, outcome, opts) })
}

// PoppySetupResult writes a setup that went through: the changes made, and what
// to do next.
func (r *Renderer) PoppySetupResult(plan setup.Plan, opts PoppySetupOptions) {
	r.poppyWrite(func(l Layout) string {
		var b strings.Builder

		if !plan.Empty() {
			b.WriteString("\n")
			b.WriteString(poppyApplyText(l, plan, setup.Outcome{Done: len(plan.Changes)}, opts))
		}

		b.WriteString("\n")
		b.WriteString(poppySetupNext(l, opts.Inputs.CompanyDomain, opts.Inputs.Out, opts.Unhinted))

		return b.String()
	})
}

// poppyPlanText is the text of a plan for a layout.
func poppyPlanText(l Layout, p setup.Plan, opts PoppySetupOptions) string {
	var b strings.Builder

	if !opts.NoHeader {
		b.WriteString(poppyHeader(l, "Poppy setup", p.CompanyDomain, p.Tenant))
	}

	if p.Empty() {
		b.WriteString("\n")
		b.WriteString(poppyChecked(l, p, opts.Inputs))
		b.WriteString("\nNothing to change. Everything already matches.\n")

		return b.String()
	}

	if opts.DryRun {
		b.WriteString("Dry run: nothing will be changed.\n")
	}

	b.WriteString("\n")
	b.WriteString(poppyChecked(l, p, opts.Inputs))
	b.WriteString("\n")

	if answers := poppyAnswers(l, opts.Inputs); answers != "" {
		b.WriteString(answers)
		b.WriteString("\n")
	}

	b.WriteString(poppySummary(p))
	b.WriteString("\n")

	for _, group := range poppyGroups(l, p) {
		b.WriteString("\n")
		b.WriteString(group)
	}

	if len(p.Unchanged) > 0 {
		noun := "settings"
		if len(p.Unchanged) == 1 {
			noun = "setting"
		}

		line := fmt.Sprintf("Already correct: %d %s (%s)", len(p.Unchanged), noun, strings.Join(poppyCleanAll(p.Unchanged), ", "))
		b.WriteString("\n")
		b.WriteString(poppyHangAfter(l, line, "  "))
	}

	if opts.DryRun {
		b.WriteString("\n")
		b.WriteString(poppyRerun(l, poppyToApplyLabel, opts))
		b.WriteString("\n")
	}

	return b.String()
}

// poppyRerun returns the command that applies the plan after a label, wrapped to
// the layout.
func poppyRerun(l Layout, label string, opts PoppySetupOptions) string {
	return setup.FormatCommand(label, setup.Command(opts.Inputs, opts.Command), l.Width)
}

// poppyHangAfter wraps s, indenting the lines after the first.
func poppyHangAfter(l Layout, s, indent string) string {
	lines := l.wrap(s, len(indent))

	var b strings.Builder
	b.WriteString(lines[0])
	b.WriteString("\n")
	for _, line := range lines[1:] {
		b.WriteString(indent)
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

// poppySummary says how many changes there are, and how many of them reach every
// app.
func poppySummary(p setup.Plan) string {
	total, wide := p.Count(), 0
	for _, change := range p.Changes {
		if change.Impact == setup.ImpactTenantWide {
			wide += change.Count()
		}
	}

	line := "Plan: " + PoppyChangeCount(total)
	switch {
	case wide == 1:
		line += ", 1 of them affects every app on the tenant"
	case wide > 1:
		line += fmt.Sprintf(", %d of them affect every app on the tenant", wide)
	}

	return line
}

// poppyLabeled is a line of a block: a mark and a label, then a value that wraps
// in its own column.
type poppyLabeled struct {
	mark  poppyMark
	label string
	value string
	// Rest are lines under the value, that sit in its column.
	rest []string
	// Below are lines under the whole line, indented a little, for a text that is
	// too long for the column of the value.
	below []string
}

// poppyWriteLabeled writes the lines of a block, labels padded to one width.
func poppyWriteLabeled(b *strings.Builder, l Layout, labelWidth int, lines []poppyLabeled) {
	for _, line := range lines {
		symbol, colored := l.mark(line.mark)
		pad := strings.Repeat(" ", max(l.markWidth()-poppyCells(symbol), 0))
		label := line.label + strings.Repeat(" ", max(labelWidth-poppyCells(line.label), 0))

		plain := "  " + symbol + pad + " " + label
		b.WriteString(l.hang(plain, "  "+colored+pad+" "+label, line.value))

		indent := strings.Repeat(" ", poppyCells(plain))
		for _, rest := range line.rest {
			for _, wrapped := range l.wrap(rest, len(indent)) {
				b.WriteString(indent)
				b.WriteString(wrapped)
				b.WriteString("\n")
			}
		}

		for _, below := range line.below {
			for _, wrapped := range l.wrap(below, len(poppyBelowIndent)) {
				b.WriteString(poppyBelowIndent)
				b.WriteString(wrapped)
				b.WriteString("\n")
			}
		}
	}
}

// poppyChecked is the block of what the planner looked at: the API, the web app,
// the scopes, and each warning.
func poppyChecked(l Layout, p setup.Plan, in setup.Inputs) string {
	var lines []poppyLabeled

	for _, api := range p.APIs {
		lines = append(lines, poppyLabeled{mark: markDone, label: "API", value: poppyClean(api)})
	}

	if p.App != "" {
		party := "first party"
		if !p.AppFirstParty {
			party = "third party"
		}
		lines = append(lines, poppyLabeled{mark: markDone, label: "Web app", value: poppyClean(p.App) + " (" + party + ")"})
	}

	if !in.NoAPI {
		scopes := "all described"
		if names := slices.Sorted(maps.Keys(in.Scopes)); len(names) > 0 {
			scopes += "  (" + strings.Join(poppyCleanAll(names), ", ") + " from your --scope)"
		}
		lines = append(lines, poppyLabeled{mark: markDone, label: "Scopes", value: scopes})
	}

	for i, warning := range p.Warnings {
		switch p.WarningKind(i) {
		case setup.WarningIssuer:
			lines = append(lines, poppyLabeled{mark: markWarning, label: "Issuer", value: poppyClean(p.Issuer), rest: []string{poppyClean(warning)}})
		case setup.WarningAPI:
			lines = append(lines, poppyLabeled{mark: markWarning, label: "API", value: poppyClean(warning)})
		default:
			lines = append(lines, poppyLabeled{mark: markWarning, label: "Note", value: poppyClean(warning)})
		}
	}

	var b strings.Builder
	b.WriteString(ansi.Bold("Checked"))
	b.WriteString("\n")
	poppyWriteLabeled(&b, l, poppyCheckedLabelWidth, lines)

	return b.String()
}

// poppyAnswerRow is a line of the answers block: a label, a value, and lines
// under the value that sit in its column.
type poppyAnswerRow struct {
	label string
	value string
	more  []string
}

// poppyAnswers is the block of what this run was asked or given.
func poppyAnswers(l Layout, in setup.Inputs) string {
	var rows []poppyAnswerRow

	for i, identifier := range slices.Sorted(maps.Keys(in.APIURLs)) {
		label, value := "", poppyClean(in.APIURLs[identifier])
		if i == 0 {
			label = "OpenAPI URL"
		}
		if len(in.APIURLs) > 1 {
			value += "  (" + poppyClean(identifier) + ")"
		}

		rows = append(rows, poppyAnswerRow{label: label, value: value})
	}

	if in.LoginURI != "" {
		rows = append(rows, poppyAnswerRow{label: "Login URI", value: poppyClean(in.LoginURI)})
	}

	rows = append(rows, poppyPartAnswers(in)...)
	rows = append(rows, poppyHintAnswers(in)...)
	rows = append(rows, poppyWebAnswers(in)...)

	if len(rows) == 0 {
		return ""
	}

	return poppyAnswersText(l, rows)
}

// poppyHintsText is the answers block of a run that has no plan, with only the
// hints and the answer about the website. It is empty when there are neither.
func poppyHintsText(l Layout, in setup.Inputs) string {
	rows := append(poppyPartAnswers(in), poppyHintAnswers(in)...)
	rows = append(rows, poppyWebAnswers(in)...)
	if len(rows) == 0 {
		return ""
	}

	return poppyAnswersText(l, rows)
}

func poppyAnswersText(l Layout, rows []poppyAnswerRow) string {
	var b strings.Builder
	b.WriteString(ansi.Bold("Your answers"))
	b.WriteString("\n")

	for _, row := range rows {
		label := "  " + row.label + strings.Repeat(" ", max(poppyAnswerLabelWidth-poppyCells(row.label), 0))
		b.WriteString(l.hang(label, label, row.value))

		indent := strings.Repeat(" ", poppyCells(label))
		for _, more := range row.more {
			b.WriteString(l.hang(indent, indent, more))
		}
	}

	return b.String()
}

// poppyHintAnswers lists the hints, one line for each scope, the ones that change
// nothing first and the ones that cannot be undone last. With many, it shows the
// first few and counts the rest.
func poppyHintAnswers(in setup.Inputs) []poppyAnswerRow {
	var lines []string

	for _, changes := range config.ValidChanges {
		for _, name := range slices.Sorted(maps.Keys(in.Hints)) {
			hint := in.Hints[name]
			if hint.Changes != changes {
				continue
			}

			line := poppyClean(name) + ": " + changes
			if hint.PersonalData {
				line += ", personal data"
			}

			lines = append(lines, line)
		}
	}

	if len(lines) == 0 {
		return nil
	}

	if len(lines) > poppyMaxHintRows {
		more := fmt.Sprintf("(%d more scopes not shown here)", len(lines)-poppyMaxHintRowsShown)
		lines = append(lines[:poppyMaxHintRowsShown], more)
	}

	return []poppyAnswerRow{{label: "Hints", value: lines[0], more: lines[1:]}}
}

// poppyWebAnswers says what the owner decided about web.unscoped, when this run was
// given a value for it, by a flag or by an answer.
func poppyWebAnswers(in setup.Inputs) []poppyAnswerRow {
	if in.WebUnscoped == nil {
		return nil
	}

	return []poppyAnswerRow{{label: "Web access", value: fmt.Sprintf("web.unscoped %t", *in.WebUnscoped)}}
}

// poppyPartAnswers says which part of Poppy the run left out, by a flag, an answer
// or the config file.
func poppyPartAnswers(in setup.Inputs) []poppyAnswerRow {
	var rows []poppyAnswerRow

	if in.NoAPI {
		rows = append(rows, poppyAnswerRow{label: "API", value: "off (auth.disabled)"})
	}

	if in.NoWeb {
		rows = append(rows, poppyAnswerRow{label: "Browser join", value: "off (web.disabled)"})
	}

	return rows
}

// poppyRow is a line of a group of changes.
type poppyRow struct {
	mark   poppyMark
	label  string
	change string
	note   string
}

// poppyGroups renders the changes, grouped by target, in the order of risk: the
// tenant, each API, the app and the file.
func poppyGroups(l Layout, p setup.Plan) []string {
	var tenant, apis, apps, files []string

	for _, change := range p.Changes {
		switch change.Kind {
		case setup.KindTenant:
			tenant = append(tenant, poppyGroup(l, "Tenant settings", poppyTenantNote(l, change), poppyChangeRows(l, change)))
		case setup.KindAPI:
			apis = append(apis, poppyGroup(l, "API "+poppyClean(change.Target()), "", poppyChangeRows(l, change)))
		case setup.KindApp:
			apps = append(apps, poppyGroup(l, "App "+poppyClean(p.App), "", poppyChangeRows(l, change)))
		case setup.KindFile:
			rows := poppyChangeRows(l, change)
			if !p.NewFile {
				rows[0].mark, rows[0].change = markChanged, "updated"
			}
			files = append(files, poppyGroup(l, "File", "", rows))
		}
	}

	var groups []string
	for _, kind := range [][]string{tenant, apis, apps, files} {
		groups = append(groups, kind...)
	}

	return groups
}

// poppyTenantNote is what the heading of the tenant settings says about who is
// affected.
func poppyTenantNote(l Layout, c setup.Change) string {
	if c.Impact != setup.ImpactTenantWide {
		return ""
	}

	return l.alert() + " affects all apps"
}

// poppyGroup renders a heading and its rows. A note sits at the right edge of the
// heading.
func poppyGroup(l Layout, heading, note string, rows []poppyRow) string {
	var b strings.Builder

	b.WriteString(ansi.Bold(heading))
	if note != "" {
		gap := l.lineWidth() - poppyCells(heading) - poppyCells(note)
		b.WriteString(strings.Repeat(" ", max(gap, 2)))
		b.WriteString(ansi.Yellow(note))
	}
	b.WriteString("\n")

	poppyWriteRows(&b, l, rows)

	return b.String()
}

// poppyWriteRows writes the rows of a group with the labels and the changes in
// columns. A note goes after the change when it fits, else under it.
func poppyWriteRows(b *strings.Builder, l Layout, rows []poppyRow) {
	labelWidth, changeWidth := 0, 0
	for _, r := range rows {
		labelWidth = max(labelWidth, poppyCells(r.label))
		changeWidth = max(changeWidth, min(poppyCells(r.change), poppyMaxChangeColumn))
	}
	changeWidth += 2

	for _, r := range rows {
		symbol, colored := l.mark(r.mark)
		pad := strings.Repeat(" ", max(l.markWidth()-poppyCells(symbol), 0))
		label := r.label + strings.Repeat(" ", labelWidth-poppyCells(r.label)+2)

		plain := "  " + symbol + pad + " " + label
		indent := strings.Repeat(" ", poppyCells(plain))
		lines := l.wrap(r.change, len(indent))

		last := lines[len(lines)-1]
		sameLine := r.note != "" && len(lines) == 1 && poppyCells(last) <= changeWidth-2 &&
			(l.Width == 0 || len(indent)+changeWidth+poppyCells(r.note) <= l.Width)

		for i, line := range lines {
			prefix := indent
			if i == 0 {
				prefix = "  " + colored + pad + " " + label
			}

			if i == len(lines)-1 && sameLine {
				line += strings.Repeat(" ", changeWidth-poppyCells(line)) + ansi.Faint(r.note)
			}

			b.WriteString(prefix)
			b.WriteString(line)
			b.WriteString("\n")
		}

		if r.note != "" && !sameLine {
			column := indent
			if l.Width == 0 || len(indent)+changeWidth+poppyCells(r.note) <= l.Width {
				column += strings.Repeat(" ", changeWidth)
			}

			b.WriteString(column)
			b.WriteString(ansi.Faint(r.note))
			b.WriteString("\n")
		}
	}
}

// poppyShown is a value of a field as the text prints it.
func poppyShown(f setup.Field, value string) string {
	switch {
	case f.IsToggle() && value == "true":
		return "on"
	case f.IsToggle() && value == "false":
		return "off"
	case value == "":
		return "?"
	}

	return poppyClean(value)
}

// poppyBeforeAfter is the change of a field: the value before, an arrow, the value
// after.
func poppyBeforeAfter(l Layout, f setup.Field) string {
	return poppyShown(f, f.Before) + " " + l.arrow() + " " + poppyShown(f, f.Value)
}

// poppyChangeRows turns a change into the rows of its group.
func poppyChangeRows(l Layout, c setup.Change) []poppyRow {
	if c.Kind == setup.KindFile {
		return []poppyRow{{mark: markAdded, label: poppyClean(strings.TrimPrefix(c.Description, "write ")), change: "new"}}
	}

	var rows []poppyRow

	for _, scope := range c.ScopeChanges {
		description := `"` + poppyClean(scope.Description) + `"`
		if scope.New {
			rows = append(rows, poppyRow{mark: markAdded, label: "scope " + poppyClean(scope.Name), change: description})

			continue
		}

		rows = append(rows, poppyRow{mark: markChanged, label: "scope " + poppyClean(scope.Name), change: setup.ValueNone + " " + l.arrow() + " " + description})
	}

	for _, field := range c.Settings() {
		label, ok := field.Label()
		if !ok {
			label = poppyClean(field.Name)
		}

		if field.Name == setup.FieldCompanyDomains {
			domains := strings.Split(field.Value, ", ")
			rows = append(rows, poppyRow{mark: markAdded, label: label, change: poppyClean(domains[len(domains)-1]), note: field.Effect})

			continue
		}

		rows = append(rows, poppyRow{mark: markChanged, label: label, change: poppyBeforeAfter(l, field), note: poppyClean(field.Effect)})
	}

	return rows
}

// poppyStepLabel names a change in the list of steps. The name of the API is added
// when the plan has several.
func poppyStepLabel(p setup.Plan, c setup.Change) string {
	switch c.Kind {
	case setup.KindTenant:
		return "Tenant settings"
	case setup.KindAPI:
		if len(p.APIs) > 1 {
			return "API " + poppyClean(c.Target())
		}

		return "API"
	case setup.KindApp:
		return "App " + poppyClean(p.App)
	default:
		return poppyClean(strings.TrimPrefix(c.Description, "write "))
	}
}

// poppyApplyText renders the result of applying the plan: each step with a mark,
// the ones done, the one that failed with its error, and the ones not run. A
// failed apply ends with the command that continues it.
func poppyApplyText(l Layout, p setup.Plan, o setup.Outcome, opts PoppySetupOptions) string {
	var b strings.Builder

	b.WriteString(ansi.Bold("Applying " + PoppyChangeCount(p.Count())))
	b.WriteString("\n")

	labelWidth := 0
	for _, change := range p.Changes {
		labelWidth = max(labelWidth, poppyCells(poppyStepLabel(p, change)))
	}

	lines := make([]poppyLabeled, len(p.Changes))
	for i, change := range p.Changes {
		lines[i] = poppyLabeled{label: poppyStepLabel(p, change)}

		switch {
		case i < o.Done:
			lines[i].mark = markDone
			lines[i].value = PoppyChangeCount(change.Count())
			if change.Kind == setup.KindFile {
				lines[i].value = "written"
			}
		case i == o.Done && o.Failure != nil:
			lines[i].mark = markFailed
			lines[i].value = "failed"
			lines[i].below = []string{poppyClean(o.Failure.Error())}
		default:
			lines[i].mark = markSkipped
			lines[i].value = "not run"
		}
	}

	poppyWriteLabeled(&b, l, labelWidth+3, lines)

	if o.Failure != nil {
		b.WriteString("\nRunning the command again continues from here.\n")
		b.WriteString(poppyRerun(l, poppyToRerunLabel, opts))
		b.WriteString("\n")
	}

	return b.String()
}

// poppySetupNext lists what to do after a setup that went through. The first step
// is to deploy the config file, so it names the file. When scopes were left
// without hints, a step says how to set them.
func poppySetupNext(l Layout, domain, out string, unhinted int) string {
	if out == "" {
		out = config.DefaultFileName
	}

	steps := [][2]string{{"Deploy " + poppyClean(out) + " with your web and API SDKs.", ""}}

	if unhinted > 0 {
		steps = append(steps, [2]string{
			fmt.Sprintf("Set the hints of %d %s with --hint <scope>=changes:<none|reversible|irreversible>,personal_data:<true|false>.", unhinted, poppyScopesNoun(unhinted)),
			"Agents show them before asking for approval.",
		})
	}

	steps = append(steps,
		[2]string{"auth0 agents register <agent-url>", "to allow an agent."},
	)

	var b strings.Builder

	b.WriteString(ansi.Bold("Next"))
	b.WriteString("\n")

	for i, step := range steps {
		line := step[0]
		if step[1] != "" {
			line += "   " + step[1]
		}

		prefix := "  " + string(rune('1'+i)) + ". "
		b.WriteString(l.hang(prefix, prefix, line))
	}

	return b.String()
}
