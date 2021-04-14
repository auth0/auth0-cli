package display

import (
	"fmt"
	"strings"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/logrusorgru/aurora"

	"gopkg.in/auth0.v5/management"
	"gopkg.in/yaml.v2"
)

const (
	notApplicable = "N/A"

	logCategorySuccess logCategory = iota
	logCategoryWarning
	logCategoryFailure
	logCategoryUnknown
)

type logCategory int

var _ View = &logView{}

type logView struct {
	silent  bool
	noColor bool
	*management.Log

	ActionExecutionAPI auth0.ActionExecutionAPI
}

func (v *logView) AsTableHeader() []string {
	return []string{"Type", "Description", "Date", "Connection", "Client"}
}

func (v *logView) getConnection() string {
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
		} else {
			return notApplicable
		}
	} else {
		return notApplicable
	}
}

func (v *logView) AsTableRow() []string {
	typ, desc := v.typeDesc()

	clientName := v.GetClientName()
	if clientName == "" {
		clientName = ansi.Faint(notApplicable)
	}

	conn := v.getConnection()
	if conn == notApplicable {
		conn = ansi.Faint(truncate(conn, 25))
	} else {
		conn = truncate(conn, 25)
	}

	return []string{
		typ,
		truncate(desc, 50),
		ansi.Faint(truncate(timeAgo(v.GetDate()), 14)),
		conn,
		clientName,
	}
}

func (v *logView) Extras() []string {
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

func (v *logView) category() logCategory {
	if strings.HasPrefix(v.GetType(), "s") {
		return logCategorySuccess

	} else if strings.HasPrefix(v.GetType(), "w") {
		return logCategoryWarning

	} else if strings.HasPrefix(v.GetType(), "f") {
		return logCategoryFailure
	}

	return logCategoryUnknown
}

func (v *logView) typeDesc() (typ, desc string) {
	chunks := strings.Split(v.TypeName(), "(")

	// NOTE(cyx): Some logs don't have a typ at all -- for those we'll
	// provide some indicator that it's empty so it's not as surprising.
	typ = chunks[0]
	if typ == "" {
		typ = "..."
	}

	typ = truncate(chunks[0], 25)

	if len(chunks) == 2 {
		desc = strings.TrimSuffix(chunks[1], ")")
	}

	desc = fmt.Sprintf("%s %s", desc, auth0.StringValue(v.Description))

	if !v.noColor {
		switch v.category() {
		case logCategorySuccess:
			typ = aurora.Green(typ).String()
		case logCategoryFailure:
			typ = aurora.BrightRed(typ).String()
		case logCategoryWarning:
			typ = aurora.BrightYellow(typ).String()
		default:
			typ = ansi.Faint(typ)
		}
	}

	return typ, desc
}

func (r *Renderer) LogList(logs []*management.Log, ch <-chan []*management.Log, api auth0.ActionExecutionAPI, noColor, silent bool) {
	resource := "logs"

	r.Heading(resource)

	if len(logs) == 0 {
		r.EmptyState(resource)
		r.Infof("To generate logs, run a test command like 'auth0 test login' or 'auth0 test token'")
		return
	}

	var res []View
	for _, l := range logs {
		res = append(res, &logView{Log: l, ActionExecutionAPI: api, silent: silent, noColor: noColor})
	}

	var viewChan chan View

	if ch != nil {
		viewChan = make(chan View)

		go func() {
			defer close(viewChan)

			for list := range ch {
				for _, l := range list {
					viewChan <- &logView{Log: l, ActionExecutionAPI: api, silent: silent, noColor: noColor}
				}
			}
		}()
	}

	r.Stream(res, viewChan)
}
