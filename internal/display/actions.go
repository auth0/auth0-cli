package display

import (
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
}

func (v *actionView) AsTableHeader() []string {
	return []string{"ID", "Name", "Type", "Created At"}
}

func (v *actionView) AsTableRow() []string {
	return []string{v.ID, v.Name, v.Type, v.CreatedAt}
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
	return []string{v.ID, v.ActionID, v.DisplayName}
}

type actionVersionView struct {
	ID         string
	ActionID   string
	ActionName string
	Runtime    string
	Status     string
	CreatedAt  string
}

func (v *actionVersionView) AsTableHeader() []string {
	return []string{"ID", "Action ID", "Action Name", "Runtime", "Status", "Created At"}
}

func (v *actionVersionView) AsTableRow() []string {
	return []string{v.getID(), v.ActionID, v.ActionName, v.Runtime, v.Status, v.CreatedAt}
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
			CreatedAt: timeAgo(auth0.TimeValue(a.CreatedAt)),
			Type:      strings.Join(triggers, ", "),
			// Runtime: auth0.StringValue(a.Runtime),
		})

	}

	r.Results(res)
}

func (r *Renderer) ActionTest(payload management.Object) {
	r.Heading(ansi.Bold(r.Tenant), "Actions test result\n")
	r.JSONResult(payload, nil)
}

func (r *Renderer) ActionCreate(action *management.Action, version *management.ActionVersion) {
	r.Heading(ansi.Bold(r.Tenant), "action created\n")
	
	v := &actionVersionView{
		ID:         version.ID,
		ActionID:   auth0.StringValue(action.ID),
		ActionName: auth0.StringValue(action.Name),
		Runtime:    version.Runtime,
		Status:     string(version.Status),
		CreatedAt:  timeAgo(auth0.TimeValue(version.CreatedAt)),
	}

	r.Results([]View{v})
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

func (r *Renderer) ActionVersion(version *management.ActionVersion) {
	r.Heading(ansi.Bold(r.Tenant), "action version\n")

	v := &actionVersionView{
		ID:        auth0.StringValue(&version.ID),
		Status:    string(version.Status),
		Runtime:   auth0.StringValue(&version.Runtime),
		CreatedAt: timeAgo(auth0.TimeValue(version.CreatedAt)),
	}

	r.Results([]View{v})
}

func (r *Renderer) ActionVersionList(list []*management.ActionVersion) {
	r.Heading(ansi.Bold(r.Tenant), "action versions\n")

	var res []View
	for _, version := range list {
		res = append(res, &actionVersionView{
			ID:        auth0.StringValue(&version.ID),
			Status:    string(version.Status),
			Runtime:   auth0.StringValue(&version.Runtime),
			CreatedAt: timeAgo(auth0.TimeValue(version.CreatedAt)),
		})
	}

	r.Results(res)
}
