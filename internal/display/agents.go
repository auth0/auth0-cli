package display

import (
	"cmp"
	"encoding/json"
	"fmt"
	"strings"

	management "github.com/auth0/go-auth0/v3/management"

	coreagents "github.com/auth0/auth0-cli/internal/agents"
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/text"
)

type agentView struct {
	ID         string
	Name       string
	ExternalID string
	Metadata   string
	CreatedAt  string
	UpdatedAt  string
	raw        interface{}

	// Truncate shortens long cells in the table. It is off for CSV, which
	// keeps every value in full. Truncated records whether a cell was cut.
	truncate  bool
	truncated bool

	// Extras and detail hold the data that `agents list` and `agents show` add
	// to an agent. Both are nil for the other agent commands.
	extras *coreagents.Extras
	detail *coreagents.DetailExtras
}

func (v *agentView) AsTableHeader() []string {
	header := []string{"ID", "Name", "External ID", "Created At"}
	if v.extras != nil {
		header = append(header, "Client ID", "Metadata", "APIs")
	}

	return header
}

func (v *agentView) AsTableRow() []string {
	row := []string{ansi.Faint(v.ID), v.Name, v.cell(v.ExternalID, agentExternalIDWidth), v.CreatedAt}
	if v.extras != nil {
		row = append(row, describeAgentClientID(*v.extras), describeAgentStatus(*v.extras), v.cell(text.Line(describeAgentAPIs(v.extras.APIs, ", ")), agentAPIsWidth))
	}

	return row
}

// The widest the External ID and APIs cells get in the table.
const (
	agentExternalIDWidth = 48
	agentAPIsWidth       = 40
)

// Cell returns value cut to maxLen runes, ending in "...", when the view
// truncates and value is longer.
func (v *agentView) cell(value string, maxLen int) string {
	runes := []rune(value)
	if !v.truncate || len(runes) <= maxLen {
		return value
	}

	v.truncated = true

	return string(runes[:maxLen-3]) + "..."
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

	if v.detail != nil {
		kvs = append(kvs, agentDetailRows(v.detail)...)
	}

	return kvs
}

func (v *agentView) Object() interface{} {
	switch {
	case v.detail != nil:
		return jsonWithFields{base: v.raw, fields: v.detail}
	case v.extras != nil:
		return jsonWithFields{base: v.raw, fields: v.extras}
	default:
		return v.raw
	}
}

// jsonWithFields encodes an object with more fields after its own, so the
// fields it already had keep their place.
type jsonWithFields struct {
	base   interface{}
	fields interface{}
}

func (j jsonWithFields) MarshalJSON() ([]byte, error) {
	base, err := json.Marshal(j.base)
	if err != nil {
		return nil, err
	}

	fields, err := json.Marshal(j.fields)
	if err != nil {
		return nil, err
	}

	if len(base) < 2 || len(fields) < 2 || base[0] != '{' || fields[0] != '{' {
		return nil, fmt.Errorf("cannot add fields to %s", base)
	}

	// An empty object has nothing to separate the new fields from.
	if string(base) == "{}" {
		return fields, nil
	}

	merged := make([]byte, 0, len(base)+len(fields))
	merged = append(merged, base[:len(base)-1]...)
	merged = append(merged, ',')

	return append(merged, fields[1:]...), nil
}

// describeAgentClientID reads the agent app of an agent for a table.
func describeAgentClientID(x coreagents.Extras) string {
	if !x.ClientID.Known {
		return "unknown"
	}

	return cmp.Or(text.Line(x.ClientID.Value), "-")
}

// describeAgentStatus reads the metadata status of an agent app for a table.
func describeAgentStatus(x coreagents.Extras) string {
	if !x.MetadataStatus.Known {
		return "unknown"
	}

	return cmp.Or(text.Line(x.MetadataStatus.Value), "-")
}

// describeAgentAPIs lists the audiences of the APIs an agent has grants on.
func describeAgentAPIs(apis coreagents.Maybe[[]coreagents.Grant], separator string) string {
	if !apis.Known {
		return "unknown"
	}

	if len(apis.Value) == 0 {
		return "-"
	}

	audiences := make([]string, len(apis.Value))
	for i, grant := range apis.Value {
		audiences[i] = grant.Audience
	}

	return strings.Join(audiences, separator)
}

// describeAgentGrants lists each API of an agent with the scopes it is granted,
// one API per line.
func describeAgentGrants(apis coreagents.Maybe[[]coreagents.Grant]) string {
	if !apis.Known {
		return "unknown"
	}

	if len(apis.Value) == 0 {
		return "-"
	}

	lines := make([]string, len(apis.Value))
	for i, grant := range apis.Value {
		lines[i] = grant.Audience + ": " + cmp.Or(strings.Join(grant.Scopes, " "), "no scopes")
	}

	return strings.Join(lines, "\n")
}

// agentDetailRows are the rows `agents show` adds after the ones of the agent.
func agentDetailRows(d *coreagents.DetailExtras) [][]string {
	dpop := "unknown"
	switch {
	case !d.HasApp() && d.ClientID.Known:
		dpop = "-"
	case d.DPoPRequired.Known:
		dpop = map[bool]string{true: "required", false: "not required"}[d.DPoPRequired.Value]
	}

	document, keys := "-", "-"
	if (d.HasApp() && !d.NotAgentApp) || !d.ClientID.Known {
		document, keys = "unknown", "unknown"

		if d.MetadataCheck.Valid.Known {
			document = "valid"
			if !d.MetadataCheck.Valid.Value {
				document = "invalid: " + text.Line(strings.Join(d.MetadataCheck.Violations, "; "))
			}
		}

		if d.KeysCurrent.Known {
			keys = map[bool]string{true: "current", false: "changed since registration"}[d.KeysCurrent.Value]
		}
	}

	return [][]string{
		{"CLIENT ID", describeAgentClientID(d.Extras)},
		{"METADATA STATUS", describeAgentStatus(d.Extras)},
		{"DPOP REQUIRED", dpop},
		{"APIS", text.Escape(describeAgentGrants(d.APIs))},
		{"METADATA CHECK", document},
		{"KEYS", keys},
	}
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

// AgentList writes the agents as a table. When extras is not nil, it holds the
// extra data of each agent, in the same order, and adds its columns and fields.
func (r *Renderer) AgentList(agents []*management.AgentResponseContent, extras []coreagents.Extras) {
	r.Heading("agents")
	if len(agents) == 0 {
		r.EmptyState("agents", "Use 'auth0 agents create' to add one")
		return
	}
	var res []View
	views := make([]*agentView, len(agents))
	for i, a := range agents {
		v := makeAgentView(a)
		v.truncate = r.Format != OutputFormatCSV
		if extras != nil {
			v.extras = &extras[i]
		}
		views[i] = v
		res = append(res, v)
	}
	r.Results(res)

	r.hintAgentValuesTruncated(views)
}

// hintAgentValuesTruncated says how to see the full values when the table cut
// some. It names the agent when only one was cut.
func (r *Renderer) hintAgentValuesTruncated(views []*agentView) {
	var cut []string
	for _, v := range views {
		if v.truncated {
			cut = append(cut, v.ID)
		}
	}

	if len(cut) == 0 {
		return
	}

	id := "<agent-id>"
	if len(cut) == 1 {
		id = cut[0]
	}

	r.Newline()
	r.Infof("Some values were truncated for display. To see them in full, run %s or %s", ansi.Faint("auth0 agents show "+id), ansi.Faint("auth0 agents list --json"))
}

// AgentShow writes an agent. When detail is not nil, it adds its rows and
// fields, then what could not be checked and the warning that the keys changed.
func (r *Renderer) AgentShow(a *management.AgentResponseContent, detail *coreagents.DetailExtras) {
	r.Heading("agent")
	v := makeAgentView(a)
	v.detail = detail
	r.Result(v)

	// The notes and the key status are fields of the JSON, so they are not
	// repeated on stderr.
	if detail == nil || r.suppressDecorations() {
		return
	}

	for _, note := range detail.Notes {
		r.Warnf("%s", text.Line(note))
	}

	if detail.KeysCurrent.Known && !detail.KeysCurrent.Value {
		r.Warnf("The agent's keys changed since it was registered. Run `auth0 agents register %s` again.", text.Line(cmp.Or(a.GetExternalAgentID(), "<agent-url>")))
	}
}

// AgentUnreadable warns that some values of `agents list` or `agents show` are
// unknown because they could not be read. The JSON output names them in
// unknown_fields instead.
func (r *Renderer) AgentUnreadable(message string) {
	if r.suppressDecorations() {
		return
	}

	r.Warnf("%s", text.Line(message))
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
