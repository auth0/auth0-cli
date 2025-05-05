package display

import (
	"fmt"
	"strings"

	"github.com/auth0/go-auth0/management"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"

	"github.com/chzyer/readline"
	"github.com/manifoldco/promptui"
	"github.com/mattn/go-tty"
	"gopkg.in/yaml.v2"
)

const (
	notApplicable = "N/A"

	logCategorySuccess logCategory = iota
	logCategoryWarning
	logCategoryFailure
	logCategoryUnknown
	colWidthType       = 20
	colWidthDesc       = 40
	colWidthDate       = 25
	colWidthConnection = 20
	colWidthClient     = 30
)

type logCategory int

var _ View = &LogView{}

type LogView struct {
	silent bool
	*management.Log
	raw interface{}
}

func (v *LogView) AsTableHeader() []string {
	return []string{"Type", "Description", "Date", "Connection", "Client"}
}

func (v *LogView) getConnection() string {
	if v.Details["prompts"] == nil {
		return notApplicable
	}

	prompt, ok := v.Details["prompts"].([]interface{})
	if ok && len(prompt) > 0 {
		dict, ok := prompt[0].(map[string]interface{})
		if ok {
			v, ok := dict["connection"].(string)
			if ok {
				return v
			}
			return notApplicable
		}
		return notApplicable
	}
	return notApplicable
}

func (v *LogView) AsTableRowString() string {
	row := v.AsTableRow()
	return fmt.Sprintf(
		"%-*s  %-*s  %-*s  %-*s  %-*s",
		colWidthType, row[0],
		colWidthDesc, row[1],
		colWidthDate, row[2],
		colWidthConnection, row[3],
		colWidthClient, row[4],
	)
}

func (v *LogView) AsTableHeaderString() string {
	row := v.AsTableHeader()
	return fmt.Sprintf(
		"    "+"\033[4m%-*s  %-*s  %-*s  %-*s  %-*s\033[0m",
		colWidthType+3, row[0],
		colWidthDesc+14, row[1],
		colWidthDate, row[2],
		colWidthConnection, row[3],
		colWidthClient, row[4],
	)
}

func (v *LogView) AsTableRow() []string {
	typ, desc := v.typeDesc()

	clientName := v.GetClientName()
	if clientName == "" {
		clientName = ansi.Faint(notApplicable)
	}

	conn := v.getConnection()
	if conn == notApplicable {
		conn = ansi.Faint(truncate(conn, 20))
	} else {
		conn = truncate(conn, 20)
	}

	return []string{
		typ,
		truncate(desc, 54),
		truncate(v.GetDate().Format("Jan 02 15:04:05.000"), 20),
		conn,
		clientName,
	}
}

func (v *LogView) Object() interface{} {
	return v.raw
}

func (v *LogView) Extras() []string {
	if v.silent {
		return nil
	}

	// NOTE(cyx): For now we only want to return full log information when
	// it's an error.
	if v.category() != logCategoryFailure {
		return nil
	}

	raw, _ := yaml.Marshal(v.Log)
	return []string{ansi.Faint(indent(string(raw), "\t"))}
}

func (v *LogView) category() logCategory {
	switch logType := v.GetType(); {
	case strings.HasPrefix(logType, "s") || strings.HasPrefix(logType, "m"):
		return logCategorySuccess
	case strings.HasPrefix(logType, "w"):
		return logCategoryWarning
	case strings.HasPrefix(logType, "f"):
		return logCategoryFailure
	default:
		return logCategoryUnknown
	}
}

func (v *LogView) typeDesc() (typ, desc string) {
	chunks := strings.Split(v.TypeName(), "(")

	// NOTE(cyx): Some logs don't have a typ at all -- for those we'll
	// provide some indicator that it's empty so it's not as surprising.
	typ = chunks[0]
	if typ == "" {
		typ = "..."
	}

	typ = truncate(chunks[0], 23)

	if len(chunks) == 2 {
		desc = strings.TrimSuffix(chunks[1], ")")
	}

	desc = fmt.Sprintf("%s %s", desc, auth0.StringValue(v.Description))

	switch v.category() {
	case logCategorySuccess:
		typ = ansi.Green(typ)
	case logCategoryFailure:
		typ = ansi.BrightRed(typ)
	case logCategoryWarning:
		typ = ansi.BrightYellow(typ)
	default:
		typ = ansi.Faint(typ)
	}

	return typ, desc
}

func (r *Renderer) LogPrompt(logs []*management.Log, hasFilter bool, currentIndex *int) string {
	resource := "logs"

	r.Heading(resource)
	if len(logs) == 0 {
		if hasFilter {
			if r.Format == OutputFormatJSON {
				r.JSONResult([]interface{}{})
				return ""
			}
			r.Warnf("No logs available matching filter criteria.\n")
		} else {
			r.EmptyState(resource, "To generate logs, run a test command like 'auth0 test login' or 'auth0 test token'")
		}

		return ""
	}

	view := LogView{Log: logs[0]}
	label := view.AsTableHeaderString()
	var rows []string

	// Recursively append each log from logs list.
	for _, l := range logs {
		view := LogView{Log: l}
		rows = append(rows, view.AsTableRowString())
	}

	promptui.IconInitial = promptui.Styler()("")
	prompt := promptui.Select{
		Label:    label,
		Items:    rows,
		Size:     10,
		HideHelp: true,
		Stdout:   &noBellStdout{},
		Templates: &promptui.SelectTemplates{
			Label: "{{ . }}",
		},
	}
	var err error
	*currentIndex, _, err = prompt.RunCursorAt(*currentIndex, *currentIndex)
	if err != nil {
		r.Errorf("failed to select a log: %w", err)
	}

	// Return the ID of the select log.
	return logs[*currentIndex].GetLogID()
}

func (r *Renderer) LogList(logs []*management.Log, silent, hasFilter bool) {
	resource := "logs"

	r.Heading(resource)

	if len(logs) == 0 {
		if hasFilter {
			if r.Format == OutputFormatJSON {
				r.JSONResult([]interface{}{})
				return
			}
			r.Warnf("No logs available matching filter criteria.\n")
		} else {
			r.EmptyState(resource, "To generate logs, run a test command like 'auth0 test login' or 'auth0 test token'")
		}

		return
	}

	var res []View
	for _, l := range logs {
		res = append(res, &LogView{Log: l, silent: silent, raw: l})
	}

	r.Results(res)
}

func (r *Renderer) LogTail(logs []*management.Log, ch <-chan []*management.Log, silent bool) {
	r.Heading("logs")

	var res []View
	for _, l := range logs {
		res = append(res, &LogView{Log: l, silent: silent, raw: l})
	}

	viewChan := make(chan View)

	go func() {
		defer close(viewChan)

		for list := range ch {
			for _, l := range list {
				viewChan <- &LogView{Log: l, silent: silent, raw: l}
			}
		}
	}()

	r.Stream(res, viewChan)
}

// Using below code to avoid the Bell sound
// when toggling up/down on prompt.
type noBellStdout struct{}

func (n *noBellStdout) Write(p []byte) (int, error) {
	if len(p) == 1 && p[0] == readline.CharBell {
		return 0, nil
	}
	return readline.Stdout.Write(p)
}

func (n *noBellStdout) Close() error {
	return readline.Stdout.Close()
}

func (r *Renderer) QuitPrompt() bool {
	fmt.Print("\nPress 'q' to quit or any other key to continue...\n")

	ContTty, _ := tty.Open()
	defer func(ContTty *tty.TTY) {
		_ = ContTty.Close()
	}(ContTty)

	rn, err := ContTty.ReadRune()
	if err != nil {
		panic(err)
	}

	return rn == 'q' || rn == 'Q'
}
