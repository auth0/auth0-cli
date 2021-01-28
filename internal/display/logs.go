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
)

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

func (v *logView) getActionExecutionID() string {
	if v.Details["actions"] == nil {
		return ""
	}

	actions, ok := v.Details["actions"].(map[string]interface{})
	if ok && actions["executions"] != nil {
		execs, ok := actions["executions"].([]interface{})
		if ok {
			v, ok := execs[0].(string)
			if ok {
				return v
			}
			return ""
		} else {
			return ""
		}
	} else {
		return ""
	}
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
		conn = ansi.Faint(truncate(conn, 30))
	} else {
		conn = truncate(conn, 30)
	}

	return []string{
		typ,
		truncate(desc, 50),
		ansi.Faint(truncate(timeAgo(v.GetDate()), 20)),
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
	fallback := []string{ansi.Faint(indent(string(raw), "\t"))}

	id := v.getActionExecutionID()
	if id == "" {
		return fallback
	}

	exec, err := v.ActionExecutionAPI.Read(id)
	if err != nil {
		return fallback
	}

	res := []string{ansi.Bold("\t=== ACTION EXECUTIONS:")}
	for _, r := range exec.Results {
		if r.Response.Error == nil {
			res = append(res, ansi.Faint(fmt.Sprintf("\t✓ Action %s logs", auth0.StringValue(r.ActionName))))
		} else {
			stack := strings.ReplaceAll(r.Response.Error["stack"], "\n", "\n\t\t")
			logs := strings.ReplaceAll(auth0.StringValue(r.Response.Logs), "\n", "\n\t\t")
			message := fmt.Sprintf("\t✘ Action %s\n\t\t%s\n\t\t%s\n", auth0.StringValue(r.ActionName), logs, stack)
			res = append(res, aurora.BrightRed(message).String())
		}
	}

	return []string{strings.Join(res, "\n")}
}

type logCategory int

const (
	logCategorySuccess logCategory = iota
	logCategoryWarning
	logCategoryFailure
	logCategoryUnknown
)

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

	typ = truncate(chunks[0], 30)

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
	r.Heading(ansi.Bold(r.Tenant), "logs\n")

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
