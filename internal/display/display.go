package display

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/manifoldco/promptui"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/olekukonko/tablewriter"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/iostream"
)

type OutputFormat string

const (
	OutputFormatJSON OutputFormat = "json"
	OutputFormatCSV  OutputFormat = "csv"
)

var infoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))               // Green
var warningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))           // Yellow
var successStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42")) // Green for success

type Renderer struct {
	Tenant string

	// MessageWriter receives the renderer messages (typically os.Stderr).
	MessageWriter io.Writer

	// ResultWriter writes the final result of the commands (typically os.Stdout which can be piped to other commands).
	ResultWriter io.Writer

	// Format indicates how the results are rendered. Default (empty) will write as table.
	Format OutputFormat

	ID string
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
}

func (r *Renderer) Newline() {
	fmt.Fprintln(r.MessageWriter)
}

func (r *Renderer) Infof(format string, a ...interface{}) {
	fmt.Fprint(r.MessageWriter, infoStyle.Render(" ▸▸   "))
	fmt.Fprintf(r.MessageWriter, format+"\n", a...)
}

func (r *Renderer) Warnf(format string, a ...interface{}) {
	fmt.Fprint(r.MessageWriter, warningStyle.Render(" ⚠️   "))
	fmt.Fprintf(r.MessageWriter, format+"\n", a...)
}

func (r *Renderer) Success(format string, a ...interface{}) {
	fmt.Fprint(r.MessageWriter, successStyle.Render("✔ "))
	fmt.Fprintf(r.MessageWriter, format+"\n", a...)
}

func (r *Renderer) ProgressBar() {
	gradientStyle := func(progress int) lipgloss.Style {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color(fmt.Sprintf("%d", 42+progress/2))).
			Width(60)
	}

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")) // Light gray for percentage

	// Simulate progress
	total := 100
	for i := 0; i <= total; i++ {
		progress := strings.Repeat("✪", i*60/total)
		remaining := strings.Repeat(" ", 60-(i*60/total))

		bar := gradientStyle(i).Render(progress + remaining)
		percentage := labelStyle.Render(fmt.Sprintf("%3d%%", i))

		// Clear the previous output to simulate updating the progress
		if i > 0 {
			fmt.Print("\033[1A\033[K") // Clear the previous line
			//fmt.Print("\033[1A\033[K") // Clear the previous bar
			//fmt.Print("\033[1A\033[K") // Clear the previous label
		}

		fmt.Printf("%s %s\n", bar, percentage)

		time.Sleep(20 * time.Millisecond) // Simulate work
	}

}

func (r *Renderer) Errorf(format string, a ...interface{}) {
	fmt.Fprint(r.MessageWriter, ansi.BrightRed(" ▸    "))
	fmt.Fprintf(r.MessageWriter, format+"\n", a...)
}

func (r *Renderer) Heading(text ...string) {
	heading := fmt.Sprintf("%s %s\n", ansi.Bold(r.Tenant), strings.Join(text, " "))
	fmt.Fprintf(r.MessageWriter, "\n%s %s\n", ansi.Faint("==="), heading)
}

func (r *Renderer) EmptyState(resource string, hint string) {
	if r.Format == OutputFormatJSON {
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

func (r *Renderer) Results(data []View) {
	if len(data) == 0 {
		if r.Format == OutputFormatJSON {
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

		buffer := &bytes.Buffer{}
		writeTable(buffer, data[0].AsTableHeader(), rows)

		if len(data) < 25 {
			// Split the rendered table into rows
			rows := bytes.Split(buffer.Bytes(), []byte("\n"))

			// Convert rows to a list of strings and remove empty rows
			var formattedRows []string
			for _, row := range rows {
				if len(row) > 0 {
					formattedRows = append(formattedRows, string(row))
				}
			}

			// Use the rows as prompt items
			prompt := promptui.Select{
				Label: "Select a User",
				Items: formattedRows[1:], // Skip the header row for selection
			}

			// Run the prompt
			_, result, err := prompt.Run()
			if err != nil {
				fmt.Printf("Prompt failed: %v\n", err)
				return
			}

			r.ID, err = fetchId(result)
			if err != nil {
				return
			}
		} else {
			r.ResultWriter = buffer
		}

	}
}

func fetchId(inputString string) (string, error) {
	regex := regexp.MustCompile(`(sms|auth0|email)\|[a-zA-Z0-9]+`)

	// Find the first match
	match := regex.FindString(inputString)

	// Check if a match was found
	if match == "" {
		return "", fmt.Errorf("no valid ID found in the input string")
	}

	return match, nil
}

func (r *Renderer) Result(data View) {
	switch r.Format {
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
			buffer := &bytes.Buffer{}
			writeTable(buffer, nil, kvs)
		}
	}
}

func (r *Renderer) Stream(data []View, ch <-chan View) {
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

func writeTable(buffer *bytes.Buffer, header []string, data [][]string) {
	table := tablewriter.NewWriter(buffer)

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
