package display

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/logrusorgru/aurora"
	"github.com/olekukonko/tablewriter"
)

type OutputFormat string

const (
	OutputFormatJSON OutputFormat = "json"
)

type Renderer struct {
	Tenant string

	// MessageWriter receives the renderer messages (typically os.Stderr)
	MessageWriter io.Writer

	// ResultWriter writes the final result of the commands (typically os.Stdout which can be piped to other commands)
	ResultWriter io.Writer

	// Format indicates how the results are rendered. Default (empty) will write as table
	Format OutputFormat
}

func NewRenderer() *Renderer {
	return &Renderer{
		MessageWriter: os.Stderr,
		ResultWriter:  os.Stdout,
	}
}

func (r *Renderer) Infof(format string, a ...interface{}) {
	fmt.Fprint(r.MessageWriter, aurora.Green(" ▸    "))
	fmt.Fprintf(r.MessageWriter, format+"\n", a...)
}

func (r *Renderer) Warnf(format string, a ...interface{}) {
	fmt.Fprint(r.MessageWriter, aurora.Yellow(" ▸    "))
	fmt.Fprintf(r.MessageWriter, format+"\n", a...)
}

func (r *Renderer) Errorf(format string, a ...interface{}) {
	fmt.Fprint(r.MessageWriter, aurora.BrightRed(" ▸    "))
	fmt.Fprintf(r.MessageWriter, format+"\n", a...)
}

func (r *Renderer) Heading(text ...string) {
	fmt.Fprintf(r.MessageWriter, "%s %s\n", ansi.Faint("==="), strings.Join(text, " "))
}

type View interface {
	AsTableHeader() []string
	AsTableRow() []string
}

func (r *Renderer) Results(data []View) {
	if len(data) > 0 {
		switch r.Format {
		case OutputFormatJSON:
			b, err := json.MarshalIndent(data, "", "    ")
			if err != nil {
				r.Errorf("couldn't marshal results as JSON: %v", err)
				return
			}
			fmt.Fprint(r.ResultWriter, string(b))

		default:
			rows := make([][]string, len(data))
			for i, d := range data {
				rows[i] = d.AsTableRow()
			}
			writeTable(r.ResultWriter, data[0].AsTableHeader(), rows)
		}
	}
}

func writeTable(w io.Writer, header []string, data [][]string) {
	tableString := &strings.Builder{}
	table := tablewriter.NewWriter(tableString)
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
	fmt.Fprint(w, tableString.String())
}

func timeAgo(ts time.Time) string {
	const (
		day   = time.Hour * 24
		month = day * 30
	)

	v := time.Since(ts)
	switch {
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
