package display

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/poppy/doctor"
	"github.com/auth0/auth0-cli/internal/poppy/setup"
	"github.com/auth0/auth0-cli/internal/poppy/text"
)

const (
	// The most characters of a value from the tenant that the text prints.
	poppyMaxValueRunes = 300

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

	// The most characters of a message that the text report of the doctor prints.
	poppyMaxMessageRunes = 200

	doctorIndent       = "  "
	doctorFixLabel     = "FIX  "
	doctorPlatformNote = " (Auth0 must fix this)"
	doctorOtherGroup   = "Other"

	// The most hint lines the answers block shows. Past it, only the first
	// poppyMaxHintRowsShown are shown, then how many scopes are left out.
	poppyMaxHintRows      = 5
	poppyMaxHintRowsShown = 3
)

// PoppyHuman reports whether a Poppy command prints for a person: no JSON format
// is selected.
func (r *Renderer) PoppyHuman() bool {
	return r.Format != OutputFormatJSON && r.Format != OutputFormatJSONCompact
}

// PoppyJSON writes the result as JSON when a JSON output format is selected, and
// reports whether it did.
func (r *Renderer) PoppyJSON(result any) bool {
	switch r.Format {
	case OutputFormatJSONCompact:
		r.JSONCompactResult(result)
	case OutputFormatJSON:
		r.JSONResult(result)
	default:
		return false
	}

	return true
}

// PoppyDeclined tells that the user said no to a question, under the text that
// came before it and after a blank line.
func (r *Renderer) PoppyDeclined(message string) {
	_, _ = fmt.Fprintln(r.ResultWriter)
	r.Infof("%s", message)
}

// poppyWrite writes the text of a Poppy command, laid out for what the result
// writer is.
func (r *Renderer) poppyWrite(build func(l PoppyLayout) string) {
	_, _ = io.WriteString(r.ResultWriter, build(PoppyLayoutOf(r.ResultWriter)))
}

// poppyClean makes a value from the tenant or the user safe to print on one line.
func poppyClean(s string) string {
	return text.Cap(text.Line(s), poppyMaxValueRunes)
}

func poppyCleanAll(values []string) []string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = poppyClean(value)
	}

	return out
}

func poppySortedKeys[V any](m map[string]V) []string {
	return slices.Sorted(maps.Keys(m))
}

// PoppyChangeCount says how many changes there are, such as "3 changes".
func PoppyChangeCount(n int) string {
	if n == 1 {
		return "1 change"
	}

	return fmt.Sprintf("%d changes", n)
}

// poppyScopesNoun is "scope" for one and "scopes" otherwise.
func poppyScopesNoun(count int) string {
	if count == 1 {
		return "scope"
	}

	return "scopes"
}

// poppyNext lists the steps to take next, each a command and what it is for.
func poppyNext(l PoppyLayout, steps ...[2]string) string {
	var b strings.Builder

	b.WriteString(ansi.Bold("Next") + "\n")

	for _, step := range steps {
		b.WriteString(l.hang("  ", "  ", step[0]+"   "+step[1]))
	}

	return b.String()
}

// poppyHeader is the title of a Poppy run, with the tenant at the right edge. The
// plan starts with it, and so does a terminal run that asks questions before it.
func poppyHeader(l PoppyLayout, title, domain, tenant string) string {
	if domain != "" {
		title += " for " + poppyClean(domain)
	}

	return l.header(title, "tenant "+poppyClean(tenant))
}

// The question screen.

// PoppyCancelled is what a Poppy command says when the user ends a question with
// Ctrl-C. The questions come before anything is changed.
const PoppyCancelled = "Cancelled. Nothing was changed."

// PoppyScreen is what a Poppy command shows while it asks questions in a
// terminal. It works like a wizard: a question draws its own block of lines, and
// the block is taken back once the answer is given, so only the question being
// answered is on the screen. The title of the run is printed once, above the
// first question.
type PoppyScreen struct {
	out    io.Writer
	layout PoppyLayout
	// Block holds the lines of the question on the screen.
	block *poppyBlock
	// Head holds the title line of the run.
	head *poppyBlock

	title, domain, tenant string
	// Shown tells that the title line is on the screen. Sticky tells that
	// something else was printed below it, so it can no longer be taken back.
	shown, sticky bool
}

// NewPoppyScreen starts the screen of a command that writes to the result writer.
// The layout gives the width of the terminal, and erase tells whether the blocks
// can be taken back, see PoppyCanErase. The title of the run needs the company
// domain, see ForDomain.
func (r *Renderer) NewPoppyScreen(title, tenant string, l PoppyLayout, erase bool) *PoppyScreen {
	w := r.ResultWriter

	return &PoppyScreen{
		out:    w,
		layout: l,
		block:  newPoppyBlock(w, l, erase),
		head:   newPoppyBlock(w, l, erase),
		title:  title,
		tenant: tenant,
	}
}

// ForDomain says which company domain the run is for. The title is printed above
// the first question once it is known.
func (s *PoppyScreen) ForDomain(domain string) { s.domain = domain }

// HeaderShown tells whether the title line of the run is on the screen, in which
// case the text that follows must not print it again.
func (s *PoppyScreen) HeaderShown() bool { return s.shown }

// DropHeader takes the title line back, for a run that prints no text after its
// questions. It does nothing once something else was printed below it.
func (s *PoppyScreen) DropHeader() {
	if !s.shown || s.sticky {
		return
	}

	_ = s.head.erase()
	s.shown = false
}

// Printed tells the screen that text was printed below the title of the run, such
// as the plan, which comes with the title when no question was asked first. From
// here the title stays, and a question goes below the text.
func (s *PoppyScreen) Printed() { s.shown, s.sticky = true, true }

// Note prints a line that stays on the screen, such as a decision that was made
// for the user.
func (s *PoppyScreen) Note(format string, args ...any) {
	s.sticky = true

	_, _ = io.WriteString(s.out, s.layout.entries([]poppyEntry{{mark: markWarning, value: text.Line(fmt.Sprintf(format, args...))}}))
}

// Begin draws a block with a title and a reason, below the title of the run, which
// it prints first when it is not on the screen yet.
func (s *PoppyScreen) Begin(title, reason string) {
	if !s.shown && s.domain != "" {
		s.shown = true

		_, _ = io.WriteString(s.head, poppyHeader(s.layout, s.title, s.domain, s.tenant))
	}

	s.say("\n")
	s.lines(ansi.Bold, title)
	s.lines(ansi.Faint, reason)
}

// End takes the block back that Begin drew.
func (s *PoppyScreen) End() error { return s.block.erase() }

// Cancelled tells that the user ended the questions with Ctrl-C.
func (s *PoppyScreen) Cancelled() {
	_, _ = fmt.Fprintln(s.out, PoppyCancelled)
}

// say writes to the block.
func (s *PoppyScreen) say(line string) {
	_, _ = io.WriteString(s.block, line)
}

// lines writes a text to the block, wrapped, in a style. An empty text writes
// nothing.
func (s *PoppyScreen) lines(style func(string) string, line string) {
	if line == "" {
		return
	}

	for _, wrapped := range s.layout.wrap(text.Line(line), 0) {
		s.say(style(wrapped) + "\n")
	}
}

// Answered writes a line to the block that tells an answer to a question of the
// group, so that it stays in view while the next question is asked.
func (s *PoppyScreen) Answered(question, answer string) {
	s.say(s.layout.entries([]poppyEntry{{mark: markDone, label: text.Line(question), value: text.Line(answer)}}))
}

// Invalid writes a line to the block that says an answer was not accepted.
func (s *PoppyScreen) Invalid(reason string) {
	s.say(s.layout.entries([]poppyEntry{{mark: markFailed, value: text.Line(reason)}}))
}

// Rows is the number of screen rows that a line takes.
func (s *PoppyScreen) Rows(line string) int { return poppyLineRows(line, s.layout.Width) }

// AddRows counts rows that a prompt left in the place of the block.
func (s *PoppyScreen) AddRows(rows int) { s.block.add(rows) }

// BlockRows is the number of rows the block takes.
func (s *PoppyScreen) BlockRows() int { return s.block.rows }

// PoppyListMarks is the width of what starts each row of a list to choose from
// several: the two spaces of the indent, the focus mark and the checkbox.
const PoppyListMarks = 8

// PoppyScopeChoice is a scope in a list to choose from.
type PoppyScopeChoice struct {
	Name        string
	Description string
	// Hints are the hints of the scope, nil when they are not known.
	Hints *config.Hints
}

// poppyHintNote says what the hints of a scope are, for a list of scopes: what the
// scope changes and whether it touches personal data.
func poppyHintNote(hints *config.Hints) string {
	if hints == nil {
		return ""
	}

	note := "[changes: " + text.Line(hints.Changes)
	if hints.PersonalData {
		note += ", personal data"
	}

	return note + "]"
}

// ScopeRows lays out the scopes of a list to choose from, one row each: the name,
// the description in a column, and the hints when they are known. A description
// that does not fit the width of the terminal is cut with an ellipsis, so no row
// wraps.
func (s *PoppyScreen) ScopeRows(scopes []PoppyScopeChoice) []string {
	nameWidth := 0
	for _, scope := range scopes {
		nameWidth = max(nameWidth, utf8.RuneCountInString(text.Line(scope.Name)))
	}

	// The room taken by the focus mark, the checkbox and the gaps.
	room := s.layout.lineWidth() - PoppyListMarks - nameWidth - 2

	rows := make([]string, len(scopes))

	for i, scope := range scopes {
		name := text.Line(scope.Name)
		note := poppyHintNote(scope.Hints)

		description := text.Line(scope.Description)
		if note != "" {
			description = text.Cap(description, max(room-utf8.RuneCountInString(note)-1, poppyMinWidth/2)) + " " + note
		} else {
			description = text.Cap(description, max(room, poppyMinWidth/2))
		}

		rows[i] = name + strings.Repeat(" ", nameWidth-utf8.RuneCountInString(name)+2) + description
	}

	return rows
}

// The doctor.

// PoppyDoctor writes the report of the doctor checks: as JSON when a JSON format
// is selected, else as text.
func (r *Renderer) PoppyDoctor(report doctor.Report) {
	if r.PoppyJSON(report) {
		return
	}

	r.poppyWrite(func(l PoppyLayout) string { return doctorText(l, report) })
}

// doctorGroups lists the groups of the text report in the order they are printed,
// with the check IDs of each. A check that is in no group is printed under the
// last group, "Other".
var doctorGroups = []struct {
	name string
	ids  []string
}{
	{"Website files", []string{"D1", "D2", "D17"}},
	{"Auth0 issuer", []string{"D3", "D4"}},
	{"Scopes", []string{"D5"}},
	{"Browser sign-in", []string{"D6", "D15", "D16"}},
	{"API", []string{"D7"}},
	{"Tenant", []string{"D8", "D9", "D10", "D11", "D12", "D13", "D14"}},
	{doctorOtherGroup, nil},
}

// doctorGroupOf returns the name of the group that prints the results of a check.
func doctorGroupOf(id string) string {
	for _, g := range doctorGroups {
		if slices.Contains(g.ids, id) {
			return g.name
		}
	}

	return doctorOtherGroup
}

// doctorMark returns the symbol of a level and what to print for it, colored when
// the output has colors.
func doctorMark(l PoppyLayout, level doctor.Level) (symbol, colored string) {
	switch level {
	case doctor.Pass:
		return l.mark(markDone)
	case doctor.Warn:
		return "WARN", ansi.Yellow("WARN")
	case doctor.Fail:
		return "FAIL", ansi.Red("FAIL")
	default:
		return l.mark(markSkipped)
	}
}

// doctorText is the report as plain text: a header, the results in groups, and a
// summary line. The width and the symbols follow the layout: a terminal gets its
// width, and any other writer gets whole lines, which a script can search.
// Messages and fixes can hold text from the site or the tenant, so control
// characters are escaped and a message is kept to at most 200 characters. The
// JSON report is not changed: encoding/json already escapes it.
func doctorText(l PoppyLayout, r doctor.Report) string {
	var b strings.Builder

	doctorHeader(&b, l, r)
	doctorWriteGroups(&b, l, r, func(doctor.Result) bool { return true })

	b.WriteByte('\n')
	b.WriteString(r.Summary())
	b.WriteByte('\n')

	return b.String()
}

// doctorProblems returns the warnings and failures of the report, with their
// fixes, in the groups of the text report, each line starting with the prefix. It
// is empty when nothing warned or failed.
func doctorProblems(l PoppyLayout, r doctor.Report, prefix string) string {
	if l.Width > 0 {
		l.Width = max(l.Width-len(prefix), poppyMinWidth)
	}

	var b strings.Builder
	doctorWriteGroups(&b, l, r, func(res doctor.Result) bool { return res.Level == doctor.Warn || res.Level == doctor.Fail })

	if b.Len() == 0 {
		return ""
	}

	// The groups start with a blank line, which a block under a heading does not need.
	lines := strings.Split(strings.TrimPrefix(b.String(), "\n"), "\n")
	for i, line := range lines[:len(lines)-1] {
		if line != "" {
			lines[i] = prefix + line
		}
	}

	return strings.Join(lines, "\n")
}

// doctorWriteGroups writes the results that keep accepts, under the heading of
// their group, each group after a blank line.
func doctorWriteGroups(b *strings.Builder, l PoppyLayout, r doctor.Report, keep func(doctor.Result) bool) {
	for _, g := range doctorGroups {
		var results []doctor.Result
		for _, res := range r.Results {
			if doctorGroupOf(res.ID) == g.name && keep(res) {
				results = append(results, res)
			}
		}
		if len(results) == 0 {
			continue
		}

		b.WriteByte('\n')
		b.WriteString(ansi.Bold(g.name))
		b.WriteByte('\n')
		for _, e := range doctorCollapse(results) {
			doctorWriteResult(b, l, e)
		}
	}
}

// doctorEntry is a result with the number of equal results it stands for.
type doctorEntry struct {
	doctor.Result
	count int
}

// doctorCollapse merges runs of skipped results that give the same reason into
// one entry, so a login that is missing is said once, not for every check it
// stops.
func doctorCollapse(results []doctor.Result) []doctorEntry {
	var entries []doctorEntry

	for _, res := range results {
		if last := len(entries) - 1; last >= 0 && res.Level == doctor.Skipped && entries[last].Level == doctor.Skipped &&
			entries[last].Message == res.Message && entries[last].Fix == res.Fix && entries[last].PlatformDependency == res.PlatformDependency {
			entries[last].count++

			continue
		}

		entries = append(entries, doctorEntry{Result: res, count: 1})
	}

	return entries
}

// doctorHeader writes the domain with the protocol version at the right edge,
// then the issuer when it is known.
func doctorHeader(b *strings.Builder, l PoppyLayout, r doctor.Report) {
	left := "Doctor for " + text.Cap(text.Line(r.Domain), poppyMaxMessageRunes)
	b.WriteString(ansi.Bold(left))

	if r.ProtocolVersion != "" {
		right := "protocol " + text.Cap(text.Line(r.ProtocolVersion), poppyMaxMessageRunes)
		gap := l.lineWidth() - utf8.RuneCountInString(left) - utf8.RuneCountInString(right)
		b.WriteString(strings.Repeat(" ", max(gap, 1)))
		b.WriteString(right)
	}
	b.WriteByte('\n')

	if r.Issuer != "" {
		b.WriteString("Issuer " + text.Cap(text.Line(r.Issuer), poppyMaxMessageRunes) + "\n")
	}
}

// doctorWriteResult writes one result, its message wrapped under the symbol, and
// its fix under the message.
func doctorWriteResult(b *strings.Builder, l PoppyLayout, e doctorEntry) {
	symbol, colored := doctorMark(l, e.Level)
	message := text.Cap(text.Line(e.Message), poppyMaxMessageRunes)
	if e.Level == doctor.Skipped {
		label := "skipped"
		if e.count > 1 {
			label = fmt.Sprintf("skipped (%d checks)", e.count)
		}
		message = label + ": " + message
	}

	pad := strings.Repeat(" ", len(doctorIndent)+utf8.RuneCountInString(symbol)+1)
	lines := l.wrap(message, len(pad))

	b.WriteString(doctorIndent + colored + " " + lines[0] + "\n")
	for _, line := range lines[1:] {
		b.WriteString(pad + line + "\n")
	}

	if e.Fix == "" {
		return
	}
	fix := text.Escape(e.Fix)
	if e.PlatformDependency {
		fix += doctorPlatformNote
	}
	b.WriteString(pad + doctorFixLabel)
	b.WriteString(strings.ReplaceAll(fix, "\n", "\n"+pad+strings.Repeat(" ", len(doctorFixLabel))))
	b.WriteByte('\n')
}

// The setup.

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
	r.poppyWrite(func(l PoppyLayout) string { return poppyPlanText(l, plan, opts) })
}

// PoppySetupFailure writes the steps of a setup that stopped partway: the ones
// made, the one that failed and the ones not run, and the command that continues.
func (r *Renderer) PoppySetupFailure(plan setup.Plan, outcome setup.Outcome, opts PoppySetupOptions) {
	r.poppyWrite(func(l PoppyLayout) string { return "\n" + poppyApplyText(l, plan, outcome, opts) })
}

// PoppySetupResult writes a setup that went through: the changes made, what the
// doctor found, and what to do next. The report is nil when the doctor did not
// run.
func (r *Renderer) PoppySetupResult(plan setup.Plan, report *doctor.Report, opts PoppySetupOptions) {
	r.poppyWrite(func(l PoppyLayout) string {
		var b strings.Builder

		if !plan.Empty() {
			b.WriteString("\n" + poppyApplyText(l, plan, setup.Outcome{Done: len(plan.Changes)}, opts))
		}

		if report != nil {
			b.WriteString("\n" + poppyDoctorAfterSetup(l, *report))
		}

		b.WriteString("\n" + poppySetupNext(l, opts.Inputs.CompanyDomain, opts.Inputs.Out, opts.Unhinted))

		return b.String()
	})
}

// poppyDoctorAfterSetup is what setup says about the doctor checks: the counts,
// then only the warnings and failures. A failed check does not fail setup: the
// website may not serve the config file until the SDK is deployed.
func poppyDoctorAfterSetup(l PoppyLayout, report doctor.Report) string {
	var b strings.Builder

	b.WriteString(ansi.Bold("Checking your setup (auth0 poppy doctor)") + "\n")
	b.WriteString("  " + report.Summary() + "\n")

	if problems := doctorProblems(l, report, "  "); problems != "" {
		b.WriteString("\n" + problems)
	}

	b.WriteString("\n")

	if failed := report.Count(doctor.Fail); failed > 0 {
		noun := "checks"
		if failed == 1 {
			noun = "check"
		}

		line := fmt.Sprintf("%d %s failed. Deploy the config file with your app and run `auth0 poppy doctor %s` again.", failed, noun, text.Line(report.Domain))
		b.WriteString(l.hang("  ", "  ", line))
	}

	b.WriteString(l.hang("  ", "  ", "Run auth0 poppy doctor "+text.Line(report.Domain)+" to see every check."))

	return b.String()
}

// poppyPlanText is the text of a plan for a layout.
func poppyPlanText(l PoppyLayout, p setup.Plan, opts PoppySetupOptions) string {
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
	b.WriteString(poppyAnswers(l, p, opts.Inputs))

	b.WriteString("\n" + poppySummary(p) + "\n")

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
		b.WriteString("\n" + poppyRerun(l, poppyToApplyLabel, opts) + "\n")
	}

	return b.String()
}

// poppyRerun returns the command that applies the plan after a label, wrapped to
// the layout.
func poppyRerun(l PoppyLayout, label string, opts PoppySetupOptions) string {
	return setup.FormatCommand(label, setup.Command(opts.Inputs, opts.Command), l.Width)
}

// poppyHangAfter wraps s, indenting the lines after the first.
func poppyHangAfter(l PoppyLayout, s, indent string) string {
	lines := l.wrap(s, len(indent))

	var b strings.Builder
	b.WriteString(lines[0] + "\n")
	for _, line := range lines[1:] {
		b.WriteString(indent + line + "\n")
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
func poppyWriteLabeled(b *strings.Builder, l PoppyLayout, labelWidth int, lines []poppyLabeled) {
	for _, line := range lines {
		symbol, colored := l.mark(line.mark)
		pad := strings.Repeat(" ", max(l.markWidth()-utf8.RuneCountInString(symbol), 0))
		label := line.label + strings.Repeat(" ", max(labelWidth-utf8.RuneCountInString(line.label), 0))

		plain := "  " + symbol + pad + " " + label
		b.WriteString(l.hang(plain, "  "+colored+pad+" "+label, line.value))

		indent := strings.Repeat(" ", utf8.RuneCountInString(plain))
		for _, rest := range line.rest {
			for _, wrapped := range l.wrap(rest, len(indent)) {
				b.WriteString(indent + wrapped + "\n")
			}
		}

		for _, below := range line.below {
			for _, wrapped := range l.wrap(below, len(poppyBelowIndent)) {
				b.WriteString(poppyBelowIndent + wrapped + "\n")
			}
		}
	}
}

// poppyChecked is the block of what the planner looked at: the API, the web app,
// the scopes, and each warning.
func poppyChecked(l PoppyLayout, p setup.Plan, in setup.Inputs) string {
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

	scopes := "all described"
	if names := poppySortedKeys(in.Scopes); len(names) > 0 {
		scopes += "  (" + strings.Join(poppyCleanAll(names), ", ") + " from your --scope)"
	}
	lines = append(lines, poppyLabeled{mark: markDone, label: "Scopes", value: scopes})

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
	b.WriteString(ansi.Bold("Checked") + "\n")
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
func poppyAnswers(l PoppyLayout, p setup.Plan, in setup.Inputs) string {
	var rows []poppyAnswerRow

	for i, identifier := range poppySortedKeys(in.APIURLs) {
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
		rows = append(rows, poppyAnswerRow{label: "Login route", value: poppyClean(in.LoginURI)})
	}

	rows = append(rows, poppyGuestAnswers(p, in)...)
	rows = append(rows, poppyHintAnswers(in)...)

	return poppyAnswersText(l, rows)
}

// poppyHintsText is the answers block of a run that has no plan, with only the
// hints. It is empty when there are no hints.
func poppyHintsText(l PoppyLayout, in setup.Inputs) string {
	rows := poppyHintAnswers(in)
	if len(rows) == 0 {
		return ""
	}

	return poppyAnswersText(l, rows)
}

func poppyAnswersText(l PoppyLayout, rows []poppyAnswerRow) string {
	var b strings.Builder
	b.WriteString(ansi.Bold("Your answers") + "\n")

	for _, row := range rows {
		label := "  " + row.label + strings.Repeat(" ", max(poppyAnswerLabelWidth-utf8.RuneCountInString(row.label), 0))
		b.WriteString(l.hang(label, label, row.value))

		indent := strings.Repeat(" ", utf8.RuneCountInString(label))
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
		for _, name := range poppySortedKeys(in.Hints) {
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

// poppyGuestAnswers says what the run decided about the guest grant: the scopes of
// the grant to make on each API, or that there is none.
func poppyGuestAnswers(p setup.Plan, in setup.Inputs) []poppyAnswerRow {
	if !in.GuestAPIAccess {
		return []poppyAnswerRow{{label: "Guest access", value: "none"}}
	}

	var rows []poppyAnswerRow

	for _, change := range p.Changes {
		if change.Kind != setup.KindGrant {
			continue
		}

		scopes := poppyCleanAll(change.GrantScopes())

		rows = append(rows, poppyAnswerRow{
			value: fmt.Sprintf("%d %s on %s", len(scopes), poppyScopesNoun(len(scopes)), poppyClean(change.Target())),
			more:  []string{strings.Join(scopes, ", ")},
		})
	}

	if len(rows) == 0 {
		return []poppyAnswerRow{{label: "Guest access", value: "already set up"}}
	}

	rows[0].label = "Guest access"

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
// tenant, each API, the app, the guest grants and the file.
func poppyGroups(l PoppyLayout, p setup.Plan) []string {
	var tenant, apis, apps, grants, files []string

	var grantRows []poppyRow

	for _, change := range p.Changes {
		switch change.Kind {
		case setup.KindTenant:
			tenant = append(tenant, poppyGroup(l, "Tenant settings", poppyTenantNote(l, change), poppyChangeRows(l, change)))
		case setup.KindAPI:
			apis = append(apis, poppyGroup(l, "API "+poppyClean(change.Target()), "", poppyChangeRows(l, change)))
		case setup.KindApp:
			apps = append(apps, poppyGroup(l, "App "+poppyClean(p.App), "", poppyChangeRows(l, change)))
		case setup.KindGrant:
			grantRows = append(grantRows, poppyChangeRows(l, change)...)
		case setup.KindFile:
			rows := poppyChangeRows(l, change)
			if !p.NewFile {
				rows[0].mark, rows[0].change = markChanged, "updated"
			}
			files = append(files, poppyGroup(l, "File", "", rows))
		}
	}

	if len(grantRows) > 0 {
		grants = append(grants, poppyGroup(l, "Guest grant", "", grantRows))
	}

	var groups []string
	for _, kind := range [][]string{tenant, apis, apps, grants, files} {
		groups = append(groups, kind...)
	}

	return groups
}

// poppyTenantNote is what the heading of the tenant settings says about who is
// affected.
func poppyTenantNote(l PoppyLayout, c setup.Change) string {
	if c.Impact != setup.ImpactTenantWide {
		return ""
	}

	return l.alert() + " affects all apps"
}

// poppyGroup renders a heading and its rows. A note sits at the right edge of the
// heading.
func poppyGroup(l PoppyLayout, heading, note string, rows []poppyRow) string {
	var b strings.Builder

	b.WriteString(ansi.Bold(heading))
	if note != "" {
		gap := l.lineWidth() - utf8.RuneCountInString(heading) - utf8.RuneCountInString(note)
		b.WriteString(strings.Repeat(" ", max(gap, 2)) + ansi.Yellow(note))
	}
	b.WriteString("\n")

	poppyWriteRows(&b, l, rows)

	return b.String()
}

// poppyWriteRows writes the rows of a group with the labels and the changes in
// columns. A note goes after the change when it fits, else under it.
func poppyWriteRows(b *strings.Builder, l PoppyLayout, rows []poppyRow) {
	labelWidth, changeWidth := 0, 0
	for _, r := range rows {
		labelWidth = max(labelWidth, utf8.RuneCountInString(r.label))
		changeWidth = max(changeWidth, min(utf8.RuneCountInString(r.change), poppyMaxChangeColumn))
	}
	changeWidth += 2

	for _, r := range rows {
		symbol, colored := l.mark(r.mark)
		pad := strings.Repeat(" ", max(l.markWidth()-utf8.RuneCountInString(symbol), 0))
		label := r.label + strings.Repeat(" ", labelWidth-utf8.RuneCountInString(r.label)+2)

		plain := "  " + symbol + pad + " " + label
		indent := strings.Repeat(" ", utf8.RuneCountInString(plain))
		lines := l.wrap(r.change, len(indent))

		last := lines[len(lines)-1]
		sameLine := r.note != "" && len(lines) == 1 && utf8.RuneCountInString(last) <= changeWidth-2 &&
			(l.Width == 0 || len(indent)+changeWidth+utf8.RuneCountInString(r.note) <= l.Width)

		for i, line := range lines {
			prefix := indent
			if i == 0 {
				prefix = "  " + colored + pad + " " + label
			}

			if i == len(lines)-1 && sameLine {
				line += strings.Repeat(" ", changeWidth-utf8.RuneCountInString(line)) + ansi.Faint(r.note)
			}

			b.WriteString(prefix + line + "\n")
		}

		if r.note != "" && !sameLine {
			column := indent
			if l.Width == 0 || len(indent)+changeWidth+utf8.RuneCountInString(r.note) <= l.Width {
				column += strings.Repeat(" ", changeWidth)
			}

			b.WriteString(column + ansi.Faint(r.note) + "\n")
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
func poppyBeforeAfter(l PoppyLayout, f setup.Field) string {
	return poppyShown(f, f.Before) + " " + l.arrow() + " " + poppyShown(f, f.Value)
}

// poppyChangeRows turns a change into the rows of its group.
func poppyChangeRows(l PoppyLayout, c setup.Change) []poppyRow {
	switch c.Kind {
	case setup.KindGrant:
		return []poppyRow{{mark: markAdded, label: poppyClean(c.Target()), change: "scopes: " + strings.Join(poppyCleanAll(c.GrantScopes()), ", ")}}
	case setup.KindFile:
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
// when the plan has several, and the guest grant, one for each API, always names
// its API.
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
	case setup.KindGrant:
		return "Guest grant " + poppyClean(c.Target())
	default:
		return poppyClean(strings.TrimPrefix(c.Description, "write "))
	}
}

// poppyApplyText renders the result of applying the plan: each step with a mark,
// the ones done, the one that failed with its error, and the ones not run. A
// failed apply ends with the command that continues it.
func poppyApplyText(l PoppyLayout, p setup.Plan, o setup.Outcome, opts PoppySetupOptions) string {
	var b strings.Builder

	b.WriteString(ansi.Bold("Applying "+PoppyChangeCount(p.Count())) + "\n")

	labelWidth := 0
	for _, change := range p.Changes {
		labelWidth = max(labelWidth, utf8.RuneCountInString(poppyStepLabel(p, change)))
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
		b.WriteString(poppyRerun(l, poppyToRerunLabel, opts) + "\n")
	}

	return b.String()
}

// poppySetupNext lists what to do after a setup that went through. The first step
// is to deploy the config file, so it names the file. When scopes were left
// without hints, a step says how to set them.
func poppySetupNext(l PoppyLayout, domain, out string, unhinted int) string {
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
		[2]string{"auth0 poppy doctor " + poppyClean(domain), "to check the live site."},
		[2]string{"auth0 poppy agents register <agent-url>", "to allow an agent."},
	)

	var b strings.Builder

	b.WriteString(ansi.Bold("Next") + "\n")

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

// The config.

// PoppyConfigResult writes what the config command did to the file of the inputs:
// that it was written or left as it was, and what to do next. A run that asked
// questions has printed its title, and its answers about hints stay above the
// result; headerShown tells that. Unhinted is the number of scopes that were left
// without hints on purpose.
func (r *Renderer) PoppyConfigResult(in setup.Inputs, unchanged, headerShown bool, unhinted int) {
	r.poppyWrite(func(l PoppyLayout) string { return poppyConfigText(l, in, unchanged, headerShown, unhinted) })
}

func poppyConfigText(l PoppyLayout, in setup.Inputs, unchanged, headerShown bool, unhinted int) string {
	result := "written"
	if unchanged {
		result = "unchanged"
	}

	next := [][2]string{{"auth0 poppy doctor " + text.Quote(text.Line(in.CompanyDomain)), "to check the live site."}}
	if unhinted > 0 {
		next = append(next, [2]string{
			"auth0 poppy config --hint <scope>=changes:<none|reversible|irreversible>,personal_data:<true|false>",
			fmt.Sprintf("to set the hints of %d %s.", unhinted, poppyScopesNoun(unhinted)),
		})
	}

	var b strings.Builder

	// The title of the run is above, once questions were asked.
	if headerShown {
		b.WriteString("\n")

		if answers := poppyHintsText(l, in); answers != "" {
			b.WriteString(answers + "\n")
		}
	}

	b.WriteString(l.entries([]poppyEntry{{mark: markDone, label: text.Line(in.Out), value: result}}) + "\n")
	b.WriteString(poppyNext(l, next...))

	return b.String()
}
