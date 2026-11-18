package display

import (
	management "github.com/auth0/go-auth0/v3/management"

	"github.com/auth0/auth0-cli/internal/ansi"
)

type agentView struct {
	ID         string
	Name       string
	ExternalID string
	Metadata   string
	CreatedAt  string
	UpdatedAt  string
	raw        interface{}
}

func (v *agentView) AsTableHeader() []string {
	return []string{"ID", "Name", "External ID", "Created At"}
}

func (v *agentView) AsTableRow() []string {
	return []string{ansi.Faint(v.ID), v.Name, v.ExternalID, v.CreatedAt}
}

func (v *agentView) KeyValues() [][]string {
	kvs := [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"NAME", v.Name},
	}
	if v.ExternalID != "" {
		kvs = append(kvs, []string{"EXTERNAL ID", v.ExternalID})
	}
	if v.Metadata != "" {
		kvs = append(kvs, []string{"METADATA", v.Metadata})
	}
	kvs = append(kvs,
		[]string{"CREATED AT", v.CreatedAt},
		[]string{"UPDATED AT", v.UpdatedAt},
	)
	return kvs
}

func (v *agentView) Object() interface{} {
	return v.raw
}

func makeAgentView(a *management.AgentResponseContent) *agentView {
	var metadata string
	if len(a.GetMetadata()) > 0 {
		metadata, _ = toJSONString(a.GetMetadata())
	}
	return &agentView{
		ID:         a.GetAgentID(),
		Name:       a.GetName(),
		ExternalID: a.GetExternalAgentID(),
		Metadata:   metadata,
		CreatedAt:  timeAgo(a.GetCreatedAt()),
		UpdatedAt:  timeAgo(a.GetUpdatedAt()),
		raw:        a,
	}
}

func (r *Renderer) AgentList(agents []*management.AgentResponseContent) {
	r.Heading("agents")
	if len(agents) == 0 {
		r.EmptyState("agents", "Use 'auth0 agents create' to add one")
		return
	}
	var res []View
	for _, a := range agents {
		res = append(res, makeAgentView(a))
	}
	r.Results(res)
}

func (r *Renderer) AgentShow(a *management.AgentResponseContent) {
	r.Heading("agent")
	r.Result(makeAgentView(a))
}

func (r *Renderer) AgentCreate(a *management.AgentResponseContent) error {
	r.Heading("agent created")
	r.Result(makeAgentView(a))
	r.Newline()
	r.Infof("To view this agent, run: auth0 agents show %s", a.GetAgentID())
	r.Infof("To update this agent, run: auth0 agents update %s", a.GetAgentID())
	r.Infof("To link an application to this agent, run: auth0 apps update <app-id> --agent-id %s", a.GetAgentID())
	r.Infof("To unlink an application from its agent, run: auth0 apps update <app-id> --unlink-agent")
	return nil
}

func (r *Renderer) AgentUpdate(a *management.AgentResponseContent) error {
	r.Heading("agent updated")
	r.Result(makeAgentView(a))
	return nil
}
