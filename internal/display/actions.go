package display

import (
	"fmt"
	"strings"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"gopkg.in/auth0.v5/management"
)

type actionView struct {
	ID        string
	Name      string
	CreatedAt string
	Type      string
	raw       interface{}
}

func (v *actionView) AsTableHeader() []string {
	return []string{"ID", "Name", "Type", "Created At"}
}

func (v *actionView) AsTableRow() []string {
	return []string{ansi.Faint(v.ID), v.Name, v.Type, v.CreatedAt}
}

func (v *actionView) KeyValues() [][]string {
	return [][]string{
		[]string{"ID", ansi.Faint(v.ID)},
		[]string{"NAME", v.Name},
		[]string{"TYPE", v.Type},
		[]string{"CREATED AT", v.CreatedAt},
	}
}

func (v *actionView) Object() interface{} {
	return v.raw
}

type triggerView struct {
	ID          string
	ActionID    string
	DisplayName string
}

func (v *triggerView) AsTableHeader() []string {
	return []string{"ID", "Action ID", "Action Name"}
}

func (v *triggerView) AsTableRow() []string {
	return []string{ansi.Faint(v.ID), v.ActionID, v.DisplayName}
}

type actionVersionView struct {
	Number     string
	ID         string
	ActionID   string
	ActionName string
	Runtime    string
	Status     string
	Deployed   string
	CreatedAt  string

	raw interface{}
}

func (v *actionVersionView) AsTableHeader() []string {
	return []string{"Number", "ID", "Action ID", "Action Name", "Runtime", "Status", "Created At", "Deployed"}
}

func (v *actionVersionView) AsTableRow() []string {
	return []string{v.Number, ansi.Faint(v.getID()), v.ActionID, v.ActionName, v.Runtime, v.Status, v.CreatedAt, v.Deployed}
}

func (v *actionVersionView) KeyValues() [][]string {
	return [][]string{
		[]string{"Number", v.Number},
		[]string{"ID", ansi.Faint(v.getID())},
		[]string{"ActionID", v.ActionID},
		[]string{"ActionName", v.ActionName},
		[]string{"RUNTIME", v.ActionName},
		[]string{"STATUS", v.Status},
		[]string{"CREATED AT", v.CreatedAt},
		[]string{"DEPLOYED", v.Deployed},
	}
}

func (v *actionVersionView) Object() interface{} {
	return v.raw
}

func (v *actionVersionView) getID() string {
	// draft versions don't have a unique id
	if v.ID == "" {
		return "draft"
	}
	return v.ID
}

func (r *Renderer) ActionList(actions []*management.Action) {
	r.Heading(ansi.Bold(r.Tenant), "actions\n")

	var res []View
	for _, a := range actions {
		var triggers = make([]string, 0, len(*a.SupportedTriggers))
		for _, t := range *a.SupportedTriggers {
			triggers = append(triggers, string(*t.ID))
		}

		res = append(res, &actionView{
			ID:        auth0.StringValue(a.ID),
			Name:      auth0.StringValue(a.Name),
			Type:      strings.Join(triggers, ", "),
			CreatedAt: timeAgo(auth0.TimeValue(a.CreatedAt)),
			raw:       a,
		})

	}

	r.Results(res)
}

func (r *Renderer) ActionTest(payload management.Object) {
	r.Heading(ansi.Bold(r.Tenant), "Actions test result\n")
	r.JSONResult(payload)
}

func (r *Renderer) Action(action *management.Action) {
	r.Heading(ansi.Bold(r.Tenant), "action\n")

	var triggers = make([]string, 0, len(*action.SupportedTriggers))
	for _, t := range *action.SupportedTriggers {
		triggers = append(triggers, string(*t.ID))
	}

	v := &actionView{
		ID:        auth0.StringValue(action.ID),
		Name:      auth0.StringValue(action.Name),
		CreatedAt: timeAgo(auth0.TimeValue(action.CreatedAt)),
		Type:      strings.Join(triggers, ", "),
		raw:       action,
	}

	r.Result(v)
}

func (r *Renderer) ActionTriggersList(bindings []*management.ActionBinding) {
	r.Heading(ansi.Bold(r.Tenant), "triggers\n")

	var res []View
	for _, b := range bindings {
		res = append(res, &triggerView{
			ID:          auth0.StringValue(b.ID),
			ActionID:    auth0.StringValue(b.Action.ID),
			DisplayName: auth0.StringValue(b.DisplayName),
		})

	}

	r.Results(res)
}

func ActionVersionView(version *management.ActionVersion) *actionVersionView {
	deployed := ""
	if version.Deployed {
		deployed = "✓"
	}

	return &actionVersionView{
		Number:     fmt.Sprint(version.Number),
		ID:         version.ID,
		ActionID:   auth0.StringValue(version.Action.ID),
		ActionName: auth0.StringValue(version.Action.Name),
		Runtime:    auth0.StringValue(&version.Runtime),
		Status:     string(version.Status),
		Deployed:   deployed,
		CreatedAt:  timeAgo(auth0.TimeValue(version.CreatedAt)),
		raw:        version,
	}
}

func (r *Renderer) ActionVersion(version *management.ActionVersion) {
	r.Heading(ansi.Bold(r.Tenant), "action version\n")

	v := ActionVersionView(version)

	r.Result(v)
}

func (r *Renderer) ActionVersionList(list []*management.ActionVersion) {
	r.Heading(ansi.Bold(r.Tenant), "action versions\n")

	var res []View
	for _, version := range list {
		res = append(res, ActionVersionView(version))
	}

	r.Results(res)
}
