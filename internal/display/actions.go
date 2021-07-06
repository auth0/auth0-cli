package display

import (
	"strconv"
	"strings"

	"github.com/auth0/auth0-cli/internal/ansi"
	"gopkg.in/auth0.v5/management"
)

type actionView struct {
	ID              string
	Name            string
	Type            string
	Deployed        string
	Status          string
	DeployedVersion string
	BuiltAt         string
	UpdatedAt       string
	CreatedAt       string
	Code            string
	raw             interface{}
}

func (v *actionView) AsTableHeader() []string {
	return []string{"ID", "Name", "Type", "Status", "Deployed"}
}

func (v *actionView) AsTableRow() []string {
	return []string{ansi.Faint(v.ID), v.Name, v.Type, v.Status, v.Deployed}
}

func (v *actionView) KeyValues() [][]string {
	return [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"NAME", v.Name},
		{"TYPE", v.Type},
		{"STATUS", v.Status},
		{"DEPLOYED", v.Deployed},
		{"LAST DEPLOYED", v.BuiltAt},
		{"LAST UPDATED", v.UpdatedAt},
		{"CREATED", v.CreatedAt},
		{"CODE", v.Code},
	}
}

func (v *actionView) Object() interface{} {
	return v.raw
}

func (r *Renderer) ActionList(actions []*management.Action) {
	resource := "actions"

	r.Heading(resource)

	if len(actions) == 0 {
		r.EmptyState(resource)
		r.Infof("Use 'auth0 actions create' to add one")
		return
	}

	var res []View
	for _, a := range actions {
		res = append(res, makeActionView(a))
	}

	r.Results(res)
}

func (r *Renderer) ActionTest(payload management.ActionTestPayload) {
	r.Heading("action test")
	r.JSONResult(payload)
}

func (r *Renderer) ActionShow(action *management.Action) {
	r.Heading("action")
	r.Result(makeActionView(action))
}

func (r *Renderer) ActionCreate(action *management.Action) {
	r.Heading("action created")
	r.Result(makeActionView(action))
}

func (r *Renderer) ActionUpdate(action *management.Action) {
	r.Heading("action updated")
	r.Result(makeActionView(action))
}

func makeActionView(action *management.Action) *actionView {
	var triggers = make([]string, 0, len(action.SupportedTriggers))
	for _, t := range action.SupportedTriggers {
		triggers = append(triggers, string(*t.ID))
	}

	isDeployed := false
	deployedVersionNumber := ""
	lastDeployed := ""

	if action.GetDeployedVersion() != nil {
		isDeployed = action.GetDeployedVersion().Deployed
		deployedVersionNumber = strconv.Itoa(action.GetDeployedVersion().Number)
		lastDeployed = timeAgo(action.GetBuiltAt())
	}

	return &actionView{
		ID:              action.GetID(),
		Name:            action.GetName(),
		Type:            strings.Join(triggers, ", "),
		Status:          actionStatus(action.GetStatus()),
		Deployed:        boolean(isDeployed),
		DeployedVersion: deployedVersionNumber,
		BuiltAt:         lastDeployed,
		CreatedAt:       timeAgo(action.GetCreatedAt()),
		UpdatedAt:       timeAgo(action.GetUpdatedAt()),
		Code:            action.GetCode(),
		raw:             action,
	}
}

func actionStatus(v string) string {
	switch strings.ToLower(v) {
	case "failed":
		return ansi.Red(v)
	case "pending", "retrying":
		return ansi.Yellow(v)
	case "building", "packaged":
		return ansi.Blue(v)
	case "built":
		return ansi.Green(v)
	default: // including "unspecified"
		return v
	}
}
