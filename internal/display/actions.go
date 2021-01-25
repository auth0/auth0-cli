package display

import (
	"strings"

	"github.com/auth0/auth0-cli/internal/ansi"
	"gopkg.in/auth0.v5"
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

type resultView struct {
	// {"logs":"Test danny from post login\n","stats":{"action_duration_ms":6,"boot_duration_ms":35,"network_duration_ms":4}}
	Logs            string
	ActionDuration  string
	BootDuration    string
	NetworkDuration string
}

func (v *resultView) AsTableHeader() []string {
	return []string{"Logs", "Action Duration", "Boot Duration", "Network Duration"}
}

func (v *resultView) AsTableRow() []string {
	return []string{v.Logs, v.ActionDuration, v.BootDuration, v.NetworkDuration}
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
	r.JSONResult(payload)
}
