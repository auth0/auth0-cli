package display

import (
	"github.com/auth0/go-auth0/management"

	"github.com/auth0/auth0-cli/internal/ansi"
)

type logStreamView struct {
	ID        string
	Name      string
	Type      string
	Status    string
	PIIConfig string
	raw       interface{}
}

func (v *logStreamView) AsTableHeader() []string {
	return []string{"ID", "Name", "Type", "Status", "PII Config"}
}

func (v *logStreamView) AsTableRow() []string {
	return []string{
		ansi.Faint(v.ID),
		v.Name,
		v.Type,
		v.Status,
		v.PIIConfig,
	}
}

func (v *logStreamView) KeyValues() [][]string {
	return [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"NAME", v.Name},
		{"TYPE", v.Type},
		{"STATUS", v.Status},
		{"PII CONFIG", v.PIIConfig},
	}
}

func (v *logStreamView) Object() interface{} {
	return v.raw
}

func (r *Renderer) LogStreamList(logs []*management.LogStream) error {
	resource := "log streams"

	r.Heading(resource)

	if len(logs) == 0 {
		r.EmptyState(resource, "Use 'auth0 logs streams create' to add one")
		return nil
	}

	var res []View
	for _, ls := range logs {
		view, err := makeLogStreamView(ls)
		if err != nil {
			return err
		}

		res = append(res, view)
	}

	r.Results(res)

	return nil
}

func (r *Renderer) LogStreamShow(logs *management.LogStream) error {
	r.Heading("log streams")
	view, err := makeLogStreamView(logs)
	if err != nil {
		return err
	}
	r.Result(view)

	return nil
}

func (r *Renderer) LogStreamCreate(logs *management.LogStream) error {
	r.Heading("log streams created")
	view, err := makeLogStreamView(logs)
	if err != nil {
		return err
	}
	r.Result(view)

	return nil
}

func (r *Renderer) LogStreamUpdate(logs *management.LogStream) error {
	r.Heading("log streams updated")
	view, err := makeLogStreamView(logs)
	if err != nil {
		return err
	}
	r.Result(view)

	return nil
}

func makeLogStreamView(logs *management.LogStream) (*logStreamView, error) {
	config, err := toJSONString(logs.GetPIIConfig())
	if err != nil {
		return nil, err
	}
	return &logStreamView{
		ID:        ansi.Faint(logs.GetID()),
		Name:      logs.GetName(),
		Type:      logs.GetType(),
		Status:    logs.GetStatus(),
		PIIConfig: config,
		raw:       logs,
	}, nil
}
