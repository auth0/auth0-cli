package display

import (
	"fmt"
	"slices"
	"strings"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/poppy/doctor"
	"github.com/auth0/auth0-cli/internal/text"
)

const (
	// The most characters of a message that the text report of the doctor prints.
	poppyMaxMessageRunes = 200

	doctorIndent       = "  "
	doctorFixLabel     = "FIX  "
	doctorPlatformNote = " (Auth0 must fix this)"
	doctorOtherGroup   = "Other"
)

// PoppyDoctor writes the report of the doctor checks: as JSON when a JSON format
// is selected, else as text.
func (r *Renderer) PoppyDoctor(report doctor.Report) {
	if r.PoppyJSON(report) {
		return
	}

	r.poppyWrite(func(l Layout) string { return doctorText(l, report) })
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
func doctorMark(l Layout, level doctor.Level) (symbol, colored string) {
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
func doctorText(l Layout, r doctor.Report) string {
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
func doctorProblems(l Layout, r doctor.Report, prefix string) string {
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
func doctorWriteGroups(b *strings.Builder, l Layout, r doctor.Report, keep func(doctor.Result) bool) {
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
			entries[last].Message == res.Message && entries[last].FixCommand == res.FixCommand && entries[last].FixHint == res.FixHint &&
			entries[last].SkipReason == res.SkipReason && entries[last].PlatformDependency == res.PlatformDependency {
			entries[last].count++

			continue
		}

		entries = append(entries, doctorEntry{Result: res, count: 1})
	}

	return entries
}

// doctorHeader writes the domain with the protocol version at the right edge,
// then the issuer when it is known.
func doctorHeader(b *strings.Builder, l Layout, r doctor.Report) {
	left := "Doctor for " + text.Cap(text.Line(r.Domain), poppyMaxMessageRunes)
	b.WriteString(ansi.Bold(left))

	if r.ProtocolVersion != "" {
		right := "protocol " + text.Cap(text.Line(r.ProtocolVersion), poppyMaxMessageRunes)
		gap := l.lineWidth() - poppyCells(left) - poppyCells(right)
		b.WriteString(strings.Repeat(" ", max(gap, 1)))
		b.WriteString(right)
	}
	b.WriteByte('\n')

	if r.Issuer != "" {
		b.WriteString("Issuer ")
		b.WriteString(text.Cap(text.Line(r.Issuer), poppyMaxMessageRunes))
		b.WriteString("\n")
	}
}

// doctorLead is what comes before the message of a warning or a failure: the
// check, and the section of the protocol that defines it. Either can be unknown.
func doctorLead(e doctorEntry) string {
	if e.Level != doctor.Warn && e.Level != doctor.Fail {
		return ""
	}

	var parts []string
	if e.ID != "" {
		parts = append(parts, text.Line(e.ID))
	}
	if e.Spec != "" {
		parts = append(parts, "(spec "+text.Line(e.Spec)+")")
	}
	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, " ") + " "
}

// doctorFixLines is the text of the FIX line of a result: the sentence, then the
// command, each wrapped to the layout after the given indent. It is empty when
// the result has no fix.
func doctorFixLines(l Layout, e doctorEntry, indent int) []string {
	var raw []string
	for _, fix := range []string{e.FixHint, e.FixCommand} {
		if fix != "" {
			raw = append(raw, strings.Split(text.Escape(fix), "\n")...)
		}
	}
	if len(raw) == 0 {
		return nil
	}

	if e.PlatformDependency {
		raw[len(raw)-1] += doctorPlatformNote
	}

	var lines []string
	for _, line := range raw {
		lines = append(lines, l.wrap(line, indent)...)
	}

	return lines
}

// doctorWriteResult writes one result, its message wrapped under the symbol, and
// its fix under the message.
func doctorWriteResult(b *strings.Builder, l Layout, e doctorEntry) {
	symbol, colored := doctorMark(l, e.Level)
	message := doctorLead(e) + text.Cap(text.Line(e.Message), poppyMaxMessageRunes)
	if e.Level == doctor.Skipped {
		label := "skipped"
		if e.count > 1 {
			label = fmt.Sprintf("skipped (%d checks)", e.count)
		}
		message = label + ": " + message
	}

	pad := strings.Repeat(" ", len(doctorIndent)+poppyCells(symbol)+1)
	lines := l.wrap(message, len(pad))

	b.WriteString(doctorIndent)
	b.WriteString(colored)
	b.WriteString(" ")
	b.WriteString(lines[0])
	b.WriteString("\n")
	for _, line := range lines[1:] {
		b.WriteString(pad)
		b.WriteString(line)
		b.WriteString("\n")
	}

	fix := doctorFixLines(l, e, len(pad)+len(doctorFixLabel))
	for i, line := range fix {
		b.WriteString(pad)
		if i == 0 {
			b.WriteString(doctorFixLabel)
		} else {
			b.WriteString(strings.Repeat(" ", len(doctorFixLabel)))
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
}

// poppyDoctorAfterSetup is what setup says about the doctor checks: the counts,
// then only the warnings and failures. A failed check does not fail setup: the
// website may not serve the config file until the SDK is deployed.
func poppyDoctorAfterSetup(l Layout, report doctor.Report) string {
	var b strings.Builder

	b.WriteString(ansi.Bold("Checking your setup (auth0 poppy doctor)"))
	b.WriteString("\n")
	b.WriteString("  ")
	b.WriteString(report.Summary())
	b.WriteString("\n")

	if problems := doctorProblems(l, report, "  "); problems != "" {
		b.WriteString("\n")
		b.WriteString(problems)
	}

	b.WriteString("\n")

	domain := text.Line(report.Domain)

	if failed := report.Count(doctor.Fail); failed > 0 {
		noun := "checks"
		if failed == 1 {
			noun = "check"
		}

		line := fmt.Sprintf("%d %s failed. Deploy the config file with your app and run `auth0 poppy doctor %s` again.", failed, noun, domain)
		if report.TenantOnly {
			line = fmt.Sprintf("%d %s failed. Follow the FIX lines above and run `auth0 poppy doctor %s` again.", failed, noun, domain)
		}
		b.WriteString(l.hang("  ", "  ", line))
	}

	if report.TenantOnly {
		b.WriteString(l.hang("  ", "  ", "Only the checks of your tenant ran, because your website is usually not deployed yet. "+
			"Once it is, run `auth0 poppy doctor "+domain+"` to check everything."))

		return b.String()
	}

	b.WriteString(l.hang("  ", "  ", "Run auth0 poppy doctor "+domain+" to see every check."))

	return b.String()
}
