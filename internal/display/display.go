package display

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
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

	Writer io.Writer

	Format OutputFormat

	initOnce sync.Once
}

func (r *Renderer) init() {
	r.initOnce.Do(func() {
		if r.Writer == nil {
			r.Writer = os.Stdout
		}
	})
}

func (r *Renderer) Infof(format string, a ...interface{}) {
	r.init()

	fmt.Fprint(r.Writer, aurora.Green(" ▸    "))
	fmt.Fprintf(r.Writer, format+"\n", a...)
}

func (r *Renderer) Warnf(format string, a ...interface{}) {
	r.init()

	fmt.Fprint(r.Writer, aurora.Yellow(" ▸    "))
	fmt.Fprintf(r.Writer, format+"\n", a...)
}

func (r *Renderer) Errorf(format string, a ...interface{}) {
	r.init()

	fmt.Fprint(r.Writer, aurora.BrightRed(" ▸    "))
	fmt.Fprintf(r.Writer, format+"\n", a...)
}

func (r *Renderer) Heading(text ...string) {
	fmt.Fprintf(r.Writer, "%s %s\n", ansi.Faint("==="), strings.Join(text, " "))
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
			fmt.Fprint(r.Writer, string(b))

		default:
			rows := make([][]string, len(data))
			for i, d := range data {
				rows[i] = d.AsTableRow()
			}
			r.table(data[0].AsTableHeader(), rows)
		}
	}
}

func (r *Renderer) table(header []string, data [][]string) {
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
	fmt.Fprint(r.Writer, tableString.String())
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
