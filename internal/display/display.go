package display

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/olekukonko/tablewriter"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/iostream"
)

type OutputFormat string

const (
	OutputFormatJSON        OutputFormat = "json"
	OutputFormatJSONCompact OutputFormat = "json-compact"
	OutputFormatCSV         OutputFormat = "csv"
)

type Renderer struct {
	Tenant string

	// MessageWriter receives the renderer messages (typically os.Stderr).
	MessageWriter io.Writer

	// ResultWriter writes the final result of the commands (typically os.Stdout which can be piped to other commands).
	ResultWriter io.Writer

	// Format indicates how the results are rendered. Default (empty) will write as table.
	Format OutputFormat

	// StructuredMessages, when true, writes diagnostic messages (info, warning,
	// success and non-fatal errors) to MessageWriter as one JSON object per line
	// instead of decorated human prose. It is enabled in agent mode so that an
	// agent which reads or merges stderr gets fully machine-parseable output.
	StructuredMessages bool
}

type View interface {
	AsTableHeader() []string
	AsTableRow() []string
	Object() interface{}
}

func NewRenderer() *Renderer {
	return &Renderer{
		MessageWriter: iostream.Messages,
		ResultWriter:  iostream.Output,
	}
}

func (r *Renderer) Output(message string) {
	fmt.Fprint(r.ResultWriter, message)

	// In JSON modes always terminate stdout with a newline so a piped reader (or
	// an NDJSON parser) never drops the final record. A bare pipe otherwise gets
	// no trailing newline, since the terminal check below is false.
	if r.Format == OutputFormatJSON || r.Format == OutputFormatJSONCompact {
		fmt.Fprintln(r.ResultWriter)
		return
	}

	if iostream.IsOutputTerminal() {
		r.Newline()
	}
}

func (r *Renderer) Newline() {
	fmt.Fprintln(r.MessageWriter)
}

// ErrorEnvelope is the machine-readable error emitted on stderr in JSON/agent
// mode, so agents can parse failures instead of scraping a human sentence.
type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody carries the classified error. Code is the coarse, stable failure
// class; Reason is a finer sub-classification that lives alongside it. Status and
// Details are omitted when unavailable. Status is omitted when the failure did
// not come from the Auth0 Management API. Details holds structured (e.g.
// field-level validation) errors. Reason is omitted when no finer classification
// is available.
type ErrorBody struct {
	Code    string          `json:"code"`
	Reason  string          `json:"reason,omitempty"`
	Message string          `json:"message"`
	Status  int             `json:"status,omitempty"`
	Details json.RawMessage `json:"details,omitempty"`
}

// ErrorJSON writes the error envelope as a single compact JSON line to stderr,
// keeping stdout clean for any partial result and giving agents one parseable line.
func (r *Renderer) ErrorJSON(envelope ErrorEnvelope) {
	b, err := json.Marshal(envelope)
	if err != nil {
		// Fall back to a human line rather than emitting nothing.
		r.Errorf("%s", envelope.Error.Message)
		return
	}

	fmt.Fprintln(r.MessageWriter, string(b))
}

// messageLine is the structured form of a diagnostic message written to stderr
// in agent mode. Every non-error diagnostic becomes one such JSON object per
// line, so an agent can parse stderr the same way it parses the error envelope.
type messageLine struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

// structuredMessage writes a diagnostic message as a single JSON line to
// MessageWriter and reports whether it did. It returns false when structured
// messages are disabled (human mode) so callers fall back to decorated prose.
func (r *Renderer) structuredMessage(level, format string, a ...interface{}) bool {
	if !r.StructuredMessages {
		return false
	}

	// The format string is concatenated (rather than forwarded verbatim) so vet
	// does not classify these Renderer methods as printf wrappers, which would
	// flag every existing non-constant-format caller across the codebase.
	line := messageLine{
		Level: level,
		// TrimSuffix drops only the single newline appended above (the concatenation
		// keeps vet from classifying this as a printf wrapper), preserving any
		// trailing newline that was already part of the message content.
		Message: strings.TrimSuffix(fmt.Sprintf(format+"\n", a...), "\n"),
	}

	b, err := json.Marshal(line)
	if err != nil {
		return false
	}

	fmt.Fprintln(r.MessageWriter, string(b))
	return true
}

func (r *Renderer) Infof(format string, a ...interface{}) {
	if r.structuredMessage("info", format, a...) {
		return
	}
	fmt.Fprint(r.MessageWriter, ansi.Green(" ▸    "))
	fmt.Fprintf(r.MessageWriter, format+"\n", a...)
}

// Successf writes a success line with a green check-mark prefix.
func (r *Renderer) Successf(format string, a ...interface{}) {
	if r.structuredMessage("success", format, a...) {
		return
	}
	fmt.Fprint(r.MessageWriter, ansi.Green("✓ "))
	fmt.Fprintf(r.MessageWriter, format+"\n", a...)
}

const detailIndent = "  "

// Detailf writes an indented detail line with no prefix symbol, used for
// supplementary information displayed beneath a success or info message.
func (r *Renderer) Detailf(format string, a ...interface{}) {
	if r.structuredMessage("detail", format, a...) {
		return
	}
	fmt.Fprintf(r.MessageWriter, detailIndent+format+"\n", a...)
}

func (r *Renderer) Warnf(format string, a ...interface{}) {
	if r.structuredMessage("warning", format, a...) {
		return
	}
	fmt.Fprint(r.MessageWriter, ansi.Yellow(" ▸    "))
	fmt.Fprintf(r.MessageWriter, format+"\n", a...)
}

func (r *Renderer) Errorf(format string, a ...interface{}) {
	if r.structuredMessage("error", format, a...) {
		return
	}
	fmt.Fprint(r.MessageWriter, ansi.BrightRed(" ▸    "))
	fmt.Fprintf(r.MessageWriter, format+"\n", a...)
}

func (r *Renderer) Heading(text ...string) {
	// The heading is purely decorative, so it is suppressed in agent mode and
	// in any JSON output mode to keep stderr free of non-JSON output.
	if r.StructuredMessages || r.Format == OutputFormatJSON || r.Format == OutputFormatJSONCompact {
		return
	}

	heading := fmt.Sprintf("%s %s\n", ansi.Bold(r.Tenant), strings.Join(text, " "))
	fmt.Fprintf(r.MessageWriter, "\n%s %s\n", ansi.Faint("==="), heading)
}

func (r *Renderer) EmptyState(resource string, hint string) {
	if r.Format == OutputFormatJSON || r.Format == OutputFormatJSONCompact {
		r.JSONResult([]interface{}{})
		return
	}
	r.Warnf("No %s available. %s\n", resource, hint)
}

func (r *Renderer) JSONResult(data interface{}) {
	b, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		r.Errorf("couldn't marshal results as JSON: %v", err)
		return
	}
	r.Output(ansi.ColorizeJSON(string(b)))
}

func (r *Renderer) JSONCompactResult(data interface{}) {
	b, err := json.Marshal(data)
	if err != nil {
		r.Errorf("couldn't marshal results as JSON: %v", err)
		return
	}

	r.Output(ansi.ColorizeJSON(string(b)))
}

func (r *Renderer) Results(data []View) {
	if len(data) == 0 {
		if r.Format == OutputFormatJSON || r.Format == OutputFormatJSONCompact {
			r.JSONResult([]interface{}{})
		}
		return
	}

	switch r.Format {
	case OutputFormatJSON:
		var list []interface{}
		for _, item := range data {
			list = append(list, item.Object())
		}
		r.JSONResult(list)
	case OutputFormatJSONCompact:
		var list []interface{}
		for _, item := range data {
			list = append(list, item.Object())
		}
		r.JSONCompactResult(list)
	case OutputFormatCSV:
		rows := make([][]string, 0, len(data))
		for _, d := range data {
			rows = append(rows, d.AsTableRow())
		}
		if err := writeCSV(r.ResultWriter, data[0].AsTableHeader(), rows); err != nil {
			r.Errorf("couldn't render results as csv: %v", err)
			return
		}
	default:
		rows := make([][]string, 0, len(data))
		for _, d := range data {
			rows = append(rows, d.AsTableRow())
		}
		writeTable(r.ResultWriter, data[0].AsTableHeader(), rows)
	}
}

func (r *Renderer) Result(data View) {
	switch r.Format {
	case OutputFormatJSONCompact:
		r.JSONCompactResult(data.Object())
	case OutputFormatJSON:
		r.JSONResult(data.Object())
	default:
		// TODO(cyx): we're type asserting on the fly to prevent too
		// many changes in other places. In the future we should
		// enforce `KeyValues` on all `View` types.
		if v, ok := data.(interface{ KeyValues() [][]string }); ok {
			var kvs [][]string
			for _, pair := range v.KeyValues() {
				k := pair[0]
				v := pair[1]
				kvs = append(kvs, []string{k, v})
			}
			writeTable(r.ResultWriter, nil, kvs)
		}
	}
}

func (r *Renderer) Stream(data []View, ch <-chan View) {
	// In JSON modes a stream must be NDJSON (one object per line), never a JSON
	// array, because an array never closes while tailing. Agent mode forces JSON,
	// so this is also the path a tailing agent takes.
	if r.Format == OutputFormatJSON || r.Format == OutputFormatJSONCompact {
		r.streamJSON(data, ch)
		return
	}

	w := r.ResultWriter

	displayRow := func(row []string) {
		fmtStr := strings.Repeat("%s    ", len(row))
		fprintfStr(w, fmtStr, row...)
		fmt.Fprintln(w)
	}

	displayView := func(v View) {
		row := v.AsTableRow()
		displayRow(row)

		if extras := extractExtras(v); extras != nil {
			fmt.Fprintln(w)
			displayRow(extras)
			fmt.Fprintln(w)
		}
	}

	if len(data) > 0 {
		header := []string{
			truncate("TYPE", 23),
			truncate("DESCRIPTION", 54),
			truncate("DATE", 20),
			truncate("CONNECTION", 20),
			truncate("CLIENT", 20),
		}
		displayRow(header)
	}

	for _, v := range data {
		displayView(v)
	}

	if ch == nil {
		return
	}

	for v := range ch {
		displayView(v)
	}
}

// streamJSON writes each streamed record as its own JSON object to stdout, one
// after another separated by newlines, never a JSON array (an array never closes
// while tailing). With --json-compact each object is a single line (NDJSON); with
// --json each object is indented for a human watching the live stream, which
// streaming parsers such as jq still accept. Agent mode always uses the compact
// form, since its machine-readable contract promises one object per line.
func (r *Renderer) streamJSON(data []View, ch <-chan View) {
	compact := r.Format == OutputFormatJSONCompact || r.StructuredMessages

	emit := func(v View) {
		var (
			b   []byte
			err error
		)
		if compact {
			b, err = json.Marshal(v.Object())
		} else {
			b, err = json.MarshalIndent(v.Object(), "", "  ")
		}
		if err != nil {
			r.Errorf("couldn't marshal stream record as JSON: %v", err)
			return
		}
		fmt.Fprintln(r.ResultWriter, string(b))
	}

	for _, v := range data {
		emit(v)
	}

	if ch == nil {
		return
	}

	for v := range ch {
		emit(v)
	}
}

func (r *Renderer) Markdown(document string) {
	g, _ := glamour.NewTermRenderer(glamour.WithAutoStyle())
	output, err := g.Render(document)

	if err != nil {
		r.Errorf("couldn't render Markdown: %v", err)
		return
	}

	fmt.Fprint(r.MessageWriter, output)
}

func fprintfStr(w io.Writer, fmtStr string, argsStr ...string) {
	var args []interface{}
	for _, a := range argsStr {
		args = append(args, a)
	}

	fmt.Fprintf(w, fmtStr, args...)
}

func writeTable(w io.Writer, header []string, data [][]string) {
	table := tablewriter.NewWriter(w)
	table.SetHeader(header)

	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)

	for _, v := range data {
		table.Append(v)
	}
	table.Render()
}

func writeCSV(w io.Writer, header []string, data [][]string) error {
	sheet := csv.NewWriter(w)
	if err := sheet.Write(header); err != nil {
		return fmt.Errorf("error writing csv header: %w", err)
	}

	if err := sheet.WriteAll(data); err != nil {
		return fmt.Errorf("error writing csv data: %w", err)
	}

	return nil
}

func timeAgo(ts time.Time) string {
	const (
		day   = time.Hour * 24
		month = day * 30
	)

	v := time.Since(ts)
	switch {
	case v < time.Minute:
		return fmt.Sprintf("%d seconds ago", v/time.Second)

	case v < 2*time.Minute:
		return "a minute ago"

	case v < time.Hour:
		return fmt.Sprintf("%d minutes ago", v/time.Minute)

	case v < 2*time.Hour:
		return "an hour ago"

	case v < day:
		return fmt.Sprintf("%d hours ago", v/time.Hour)

	case v < 2*day:
		return "a day ago"

	case v < month:
		return fmt.Sprintf("%d days ago", v/day)

	default:
		return ts.Format("Jan 02 2006")
	}
}

func extractExtras(v View) []string {
	if e, ok := v.(interface{ Extras() []string }); ok {
		return e.Extras()
	}

	return nil
}

func truncate(str string, maxLen int) string {
	str = strings.Trim(str, " ")

	if len(str) < maxLen {
		missing := maxLen - len([]rune(str))

		return str + strings.Repeat(" ", missing)
	}

	return str[:maxLen-3] + "..."
}

func indent(text, indent string) string {
	if text[len(text)-1:] == "\n" {
		result := ""
		for _, j := range strings.Split(text[:len(text)-1], "\n") {
			result += indent + j + "\n"
		}
		return result
	}
	result := ""
	for _, j := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		result += indent + j + "\n"
	}
	return result[:len(result)-1]
}

func boolean(v bool) string {
	if v {
		return ansi.Green("✓")
	}
	return ansi.Red("✗")
}

func toJSONString(data interface{}) (string, error) {
	if data == nil {
		return "", nil
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	return string(raw), nil
}
