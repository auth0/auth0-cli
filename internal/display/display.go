package display

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/cyx/auth0/management"
	"github.com/logrusorgru/aurora"
)

type Renderer struct {
	Tenant string

	Writer io.Writer

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

func (r *Renderer) ActionList(actions []*management.Action) {
	r.Heading(ansi.Bold(r.Tenant), "actions")

	for _, a := range actions {
		fmt.Fprintf(r.Writer, "%s\n", a.Name)
	}
}

func (r *Renderer) ActionInfo(action *management.Action, versions []*management.ActionVersion) {
	fmt.Fprintln(r.Writer)
	fmt.Fprintf(r.Writer, "%-7s : %s\n", "Name", action.Name)
	fmt.Fprintf(r.Writer, "%-7s : %s\n", "Trigger", action.SupportedTriggers[0].ID)

	var (
		lines   []string
		maxLine int
	)

	for _, v := range versions {
		version := fmt.Sprintf("v%d", v.Number)

		// TODO(cyx): fix dates
		line := fmt.Sprintf("%-3s | %-10s | %-10s", version, v.Status, "a minute ago")
		if n := len(line); n > maxLine {
			maxLine = n
		}
		lines = append(lines, line)
	}

	fmt.Fprintln(r.Writer)
	fmt.Fprintln(r.Writer, strings.Repeat("-", maxLine))

	for _, l := range lines {
		fmt.Fprintln(r.Writer, l)
	}

	fmt.Fprintln(r.Writer, strings.Repeat("-", maxLine))
}

func (r *Renderer) Heading(text ...string) {
	fmt.Fprintf(r.Writer, "%s %s\n", ansi.Faint("==="), strings.Join(text, " "))
}
