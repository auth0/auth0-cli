package display

import (
	"fmt"
	"io"
	"strings"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/text"
)

const (
	// The most characters of a value from the tenant that the text prints.
	poppyMaxValueRunes = 300
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
	_, _ = fmt.Fprintln(r.poppyOut())
	r.Infof("%s", message)
}

// poppyOut is where the text of a Poppy command goes: the result writer for a
// person. With a JSON format only the JSON is a result, so the text that a person
// at a terminal reads before answering, such as the plan, goes to the message
// writer.
func (r *Renderer) poppyOut() io.Writer {
	if r.PoppyHuman() {
		return r.ResultWriter
	}

	return r.MessageWriter
}

// poppyWrite writes the text of a Poppy command, laid out for what the writer is.
func (r *Renderer) poppyWrite(build func(l Layout) string) {
	w := r.poppyOut()

	_, _ = io.WriteString(w, build(LayoutOf(w)))
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
func poppyNext(l Layout, steps ...[2]string) string {
	var b strings.Builder

	b.WriteString(ansi.Bold("Next"))
	b.WriteString("\n")

	for _, step := range steps {
		b.WriteString(l.hang("  ", "  ", step[0]+"   "+step[1]))
	}

	return b.String()
}

// poppyHeader is the title of a Poppy run, with the tenant at the right edge. The
// plan starts with it, and so does a terminal run that asks questions before it.
func poppyHeader(l Layout, title, domain, tenant string) string {
	if domain != "" {
		title += " for " + poppyClean(domain)
	}

	return l.header(title, "tenant "+poppyClean(tenant))
}
