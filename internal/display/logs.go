package display

import (
	"fmt"
	"strings"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"

	"github.com/auth0/go-auth0/management"
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
	silent bool
	*management.Log
	raw interface{}
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
		}
		return notApplicable
	}
	return notApplicable
}

func (v *logView) AsTableRow() []string {
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

func (v *logView) Object() interface{} {
	return v.raw
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
	switch logType := v.GetType(); {
	case strings.HasPrefix(logType, "s"):
		return logCategorySuccess
	case strings.HasPrefix(logType, "w"):
		return logCategoryWarning
	case strings.HasPrefix(logType, "f"):
		return logCategoryFailure
	default:
		return logCategoryUnknown
	}
}

func (v *logView) typeDesc() (typ, desc string) {
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
		res = append(res, &logView{Log: l, silent: silent, raw: l})
	}

	r.Results(res)
}

func (r *Renderer) LogTail(logs []*management.Log, ch <-chan []*management.Log, silent bool) {
	r.Heading("logs")

	var res []View
	for _, l := range logs {
		res = append(res, &logView{Log: l, silent: silent, raw: l})
	}

	viewChan := make(chan View)

	go func() {
		defer close(viewChan)

		for list := range ch {
			for _, l := range list {
				viewChan <- &logView{Log: l, silent: silent, raw: l}
			}
		}
	}()

	r.Stream(res, viewChan)
}
