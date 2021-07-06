package display

import (
	"github.com/auth0/auth0-cli/internal/ansi"
	"gopkg.in/auth0.v5/management"
)

type logStreamView struct {
	ID     string
	Name   string
	Type   string
	Status string
	raw    interface{}
}

func (v *logStreamView) AsTableHeader() []string {
	return []string{"ID", "Name", "Type", "Status"}
}

func (v *logStreamView) AsTableRow() []string {
	return []string{
		ansi.Faint(v.ID),
		v.Name,
		v.Type,
		v.Status,
	}
}

func (v *logStreamView) KeyValues() [][]string {
	return [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"NAME", v.Name},
		{"TYPE", v.Type},
		{"STATUS", v.Status},
	}
}

func (v *logStreamView) Object() interface{} {
	return v.raw
}

func (r *Renderer) LogStreamList(logs []*management.LogStream) {
	resource := "log streams"

	r.Heading(resource)

	if len(logs) == 0 {
		r.EmptyState(resource)
		r.Infof("use 'auth0 logs streams create' to create one")
		return
	}

	var res []View
	for _, ls := range logs {
		res = append(res, makeLogStreamView(ls))
	}

	r.Results(res)
}

func (r *Renderer) LogStreamShow(logs *management.LogStream) {
	r.Heading("log streams")
	r.Result(makeLogStreamView(logs))
}

func (r *Renderer) LogStreamCreate(logs *management.LogStream) {
	r.Heading("log streams created")
	r.Result(makeLogStreamView(logs))
}

func (r *Renderer) LogStreamUpdate(logs *management.LogStream) {
	r.Heading("log streams updated")
	r.Result(makeLogStreamView(logs))
}

func makeLogStreamView(logs *management.LogStream) *logStreamView {
	return &logStreamView{
		ID:     ansi.Faint(logs.GetID()),
		Name:   logs.GetName(),
		Type:   logs.GetType(),
		Status: logs.GetStatus(),
		raw:    logs,
	}
}
