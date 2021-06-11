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
		[]string{"ID", ansi.Faint(v.ID)},
		[]string{"NAME", v.Name},
		[]string{"TYPE", v.Type},
		[]string{"STATUS", v.Status},
	}
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
		res = append(res, &logStreamView{
			ID:     ansi.Faint(ls.GetID()),
			Name:   ls.GetName(),
			Type:   ls.GetType(),
			Status: ls.GetStatus(),
		})
	}

	r.Results(res)
}

func (r *Renderer) LogStreamShow(logs *management.LogStream) {
	r.Heading("log streams")
	r.logStreamResult(logs)
}

func (r *Renderer) LogStreamCreate(logs *management.LogStream) {
	r.Heading("log streams created")
	r.logStreamResult(logs)
}

func (r *Renderer) LogStreamUpdate(logs *management.LogStream) {
	r.Heading("log streams updated")
	r.logStreamResult(logs)
}

func (r *Renderer) logStreamResult(logs *management.LogStream) {
	r.Result(&logStreamView{
		ID:     ansi.Faint(logs.GetID()),
		Name:   logs.GetName(),
		Type:   logs.GetType(),
		Status: logs.GetStatus(),
	})
}
