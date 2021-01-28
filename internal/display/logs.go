package display

import (
	"fmt"
	"strings"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/logrusorgru/aurora"

	"gopkg.in/auth0.v5/management"
)

var (
	notApplicable = ansi.Faint("N/A")
)

var _ View = &logView{}

type logView struct {
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
	typ, desc := typeDescFor(v.Log, false)

	clientName := v.GetClientName()
	if clientName == "" {
		clientName = notApplicable
	}

	return []string{
		typ,
		desc,
		ansi.Faint(timeAgo(v.GetDate())),
		v.getConnection(),
		clientName,
	}
}

func (v *logView) Extras() []string {
	if strings.HasPrefix(v.GetType(), "f") == false {
		return nil
	}

	id := v.getActionExecutionID()
	if id == "" {
		return nil
	}

	exec, err := v.ActionExecutionAPI.Read(id)
	if err != nil {
		return nil
	}

	res := []string{ansi.Bold("\tAction Executions:")}
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

func typeDescFor(l *management.Log, noColor bool) (typ, desc string) {
	chunks := strings.Split(l.TypeName(), "(")
	typ = chunks[0]

	if len(chunks) == 2 {
		desc = strings.TrimSuffix(chunks[1], ")")
	}

	desc = fmt.Sprintf("%s %s", desc, auth0.StringValue(l.Description))

	if !noColor {
		// colorize the event type field based on whether it's a success or failure
		if strings.HasPrefix(l.GetType(), "s") {
			typ = aurora.Green(typ).String()
		} else if strings.HasPrefix(l.GetType(), "f") {
			typ = aurora.BrightRed(typ).String()
		} else if strings.HasPrefix(l.GetType(), "w") {
			typ = aurora.BrightYellow(typ).String()
		}
	}

	return typ, desc
}

func (r *Renderer) LogList(logs []*management.Log, ch <-chan []*management.Log, api auth0.ActionExecutionAPI, noColor bool) {
	r.Heading(ansi.Bold(r.Tenant), "logs\n")

	var res []View
	for _, l := range logs {
		res = append(res, &logView{Log: l, ActionExecutionAPI: api})
	}

	var viewChan chan View

	if ch != nil {
		viewChan = make(chan View)

		go func() {
			defer close(viewChan)

			for list := range ch {
				for _, l := range list {
					viewChan <- &logView{Log: l, ActionExecutionAPI: api}
				}
			}
		}()
	}

	r.Stream(res, viewChan)
}
