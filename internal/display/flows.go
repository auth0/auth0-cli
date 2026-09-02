package display

import (
	"encoding/json"
	"fmt"
	"time"

	managementv3 "github.com/auth0/go-auth0/v3/management"

	"github.com/auth0/auth0-cli/internal/ansi"
)

// --- Flows ---.

type flowView struct {
	ID          string
	Name        string
	ActionCount int
	CreatedAt   string
	UpdatedAt   string
	ExecutedAt  string

	raw interface{}
}

func (v *flowView) AsTableHeader() []string {
	return []string{"ID", "Name", "Executed At", "Updated"}
}

func (v *flowView) AsTableRow() []string {
	return []string{ansi.Faint(v.ID), v.Name, v.ExecutedAt, v.UpdatedAt}
}

func (v *flowView) KeyValues() [][]string {
	return [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"NAME", v.Name},
		{"ACTIONS", fmt.Sprintf("%d actions", v.ActionCount)},
		{"CREATED AT", v.CreatedAt},
		{"UPDATED AT", v.UpdatedAt},
	}
}

func (v *flowView) Object() interface{} {
	return v.raw
}

type flowSummaryView struct {
	ID         string
	Name       string
	UpdatedAt  string
	ExecutedAt string

	raw interface{}
}

func (v *flowSummaryView) AsTableHeader() []string {
	return []string{"ID", "Name", "Executed At", "Updated"}
}

func (v *flowSummaryView) AsTableRow() []string {
	return []string{ansi.Faint(v.ID), v.Name, v.ExecutedAt, v.UpdatedAt}
}

func (v *flowSummaryView) Object() interface{} {
	return v.raw
}

// FlowsList renders the list of flows.
func (r *Renderer) FlowsList(flows []*managementv3.FlowSummary) error {
	resource := "flows"

	r.Heading(resource)

	if len(flows) == 0 {
		r.EmptyState(resource, "Use 'auth0 flows create' to add one")
		return nil
	}

	var res []View
	for _, f := range flows {
		res = append(res, &flowSummaryView{
			ID:         f.GetID(),
			Name:       f.GetName(),
			UpdatedAt:  timeAgo(f.GetUpdatedAt()),
			ExecutedAt: f.GetExecutedAt(),
			raw:        mergeExtraProperties(f, f.GetExtraProperties()),
		})
	}

	r.Results(res)

	return nil
}

// FlowShowRaw renders a full-fidelity flow response read through the v1 HTTP
// client, avoiding the v3 SDK's lossy flow-action unions.
func (r *Renderer) FlowShowRaw(flow json.RawMessage) error {
	return r.renderRawFlow("flow", flow)
}

// FlowCreateRaw renders a full-fidelity create response.
func (r *Renderer) FlowCreateRaw(flow json.RawMessage) error {
	return r.renderRawFlow("flow created", flow)
}

// FlowUpdateRaw renders a full-fidelity update response.
func (r *Renderer) FlowUpdateRaw(flow json.RawMessage) error {
	return r.renderRawFlow("flow updated", flow)
}

func (r *Renderer) renderRawFlow(heading string, flow json.RawMessage) error {
	view, err := makeFlowViewFromRaw(flow)
	if err != nil {
		return fmt.Errorf("failed to parse flow response: %w", err)
	}
	r.Heading(heading)
	r.Result(view)
	return nil
}

func makeFlowViewFromRaw(raw json.RawMessage) (*flowView, error) {
	var flow struct {
		ID         string            `json:"id"`
		Name       string            `json:"name"`
		Actions    []json.RawMessage `json:"actions"`
		CreatedAt  time.Time         `json:"created_at"`
		UpdatedAt  time.Time         `json:"updated_at"`
		ExecutedAt string            `json:"executed_at"`
	}
	if err := json.Unmarshal(raw, &flow); err != nil {
		return nil, err
	}

	return &flowView{
		ID:          flow.ID,
		Name:        flow.Name,
		ActionCount: len(flow.Actions),
		CreatedAt:   rawTimeAgo(flow.CreatedAt),
		UpdatedAt:   rawTimeAgo(flow.UpdatedAt),
		ExecutedAt:  flow.ExecutedAt,
		raw:         raw,
	}, nil
}

// FlowExport writes a flow body verbatim (uncolored) to the result writer so it
// stays pipe- and import-friendly.
func (r *Renderer) FlowExport(body string) {
	fmt.Fprintln(r.ResultWriter, body)
}

// --- Flow executions ---.

type flowExecutionView struct {
	ID        string
	Status    string
	TraceID   string
	StartedAt string
	EndedAt   string
	CreatedAt string
	UpdatedAt string

	raw interface{}
}

func (v *flowExecutionView) AsTableHeader() []string {
	return []string{"ID", "Status", "Started", "Ended"}
}

func (v *flowExecutionView) AsTableRow() []string {
	return []string{ansi.Faint(v.ID), v.Status, v.StartedAt, v.EndedAt}
}

func (v *flowExecutionView) KeyValues() [][]string {
	kvs := [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"STATUS", v.Status},
		{"TRACE ID", v.TraceID},
	}
	if v.StartedAt != "" {
		kvs = append(kvs, []string{"STARTED AT", v.StartedAt})
	}
	if v.EndedAt != "" {
		kvs = append(kvs, []string{"ENDED AT", v.EndedAt})
	}
	kvs = append(kvs,
		[]string{"CREATED AT", v.CreatedAt},
		[]string{"UPDATED AT", v.UpdatedAt},
	)
	return kvs
}

func (v *flowExecutionView) Object() interface{} {
	return v.raw
}

// FlowExecutionsList renders the list of flow executions.
func (r *Renderer) FlowExecutionsList(executions []*managementv3.FlowExecutionSummary) error {
	resource := "flow executions"

	r.Heading(resource)

	if len(executions) == 0 {
		r.EmptyState(resource, "This flow has not been executed yet")
		return nil
	}

	var res []View
	for _, e := range executions {
		res = append(res, &flowExecutionView{
			ID:        e.GetID(),
			Status:    e.GetStatus(),
			TraceID:   e.GetTraceID(),
			StartedAt: rawTimeAgo(e.GetStartedAt()),
			EndedAt:   rawTimeAgo(e.GetEndedAt()),
			CreatedAt: rawTimeAgo(e.GetCreatedAt()),
			UpdatedAt: rawTimeAgo(e.GetUpdatedAt()),
			raw:       mergeExtraProperties(e, e.GetExtraProperties()),
		})
	}

	r.Results(res)

	return nil
}

// FlowExecutionShowRaw renders a full-fidelity execution response read through
// the v1 HTTP client.
func (r *Renderer) FlowExecutionShowRaw(execution json.RawMessage) error {
	view, err := makeFlowExecutionViewFromRaw(execution)
	if err != nil {
		return fmt.Errorf("failed to parse flow execution response: %w", err)
	}
	r.Heading("flow execution")
	r.Result(view)
	return nil
}

func makeFlowExecutionViewFromRaw(raw json.RawMessage) (*flowExecutionView, error) {
	var execution struct {
		ID        string    `json:"id"`
		Status    string    `json:"status"`
		TraceID   string    `json:"trace_id"`
		StartedAt time.Time `json:"started_at"`
		EndedAt   time.Time `json:"ended_at"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	if err := json.Unmarshal(raw, &execution); err != nil {
		return nil, err
	}

	return &flowExecutionView{
		ID:        execution.ID,
		Status:    execution.Status,
		TraceID:   execution.TraceID,
		StartedAt: rawTimeAgo(execution.StartedAt),
		EndedAt:   rawTimeAgo(execution.EndedAt),
		CreatedAt: rawTimeAgo(execution.CreatedAt),
		UpdatedAt: rawTimeAgo(execution.UpdatedAt),
		raw:       raw,
	}, nil
}

// --- Flow vault connections ---.

type flowVaultConnectionView struct {
	ID          string
	Name        string
	AppID       string
	Ready       bool
	AccountName string
	CreatedAt   string
	UpdatedAt   string

	raw interface{}
}

func (v *flowVaultConnectionView) AsTableHeader() []string {
	return []string{"ID", "Name", "App", "Ready"}
}

func (v *flowVaultConnectionView) AsTableRow() []string {
	return []string{ansi.Faint(v.ID), v.Name, v.AppID, boolToPresence(v.Ready)}
}

func (v *flowVaultConnectionView) KeyValues() [][]string {
	kvs := [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"NAME", v.Name},
		{"APP ID", v.AppID},
		{"READY", boolToReady(v.Ready)},
	}
	if v.AccountName != "" {
		kvs = append(kvs, []string{"ACCOUNT NAME", v.AccountName})
	}
	kvs = append(kvs,
		[]string{"CREATED AT", v.CreatedAt},
		[]string{"UPDATED AT", v.UpdatedAt},
	)
	return kvs
}

func (v *flowVaultConnectionView) Object() interface{} {
	return v.raw
}

// FlowVaultConnectionsList renders the list of vault connections.
func (r *Renderer) FlowVaultConnectionsList(connections []*managementv3.FlowsVaultConnectionSummary) error {
	resource := "flow vault connections"

	r.Heading(resource)

	if len(connections) == 0 {
		r.EmptyState(resource, "Use 'auth0 flows vault connections create' to add one")
		return nil
	}

	var res []View
	for _, c := range connections {
		res = append(res, &flowVaultConnectionView{
			ID:          c.GetID(),
			Name:        c.GetName(),
			AppID:       c.GetAppID(),
			Ready:       c.GetReady(),
			AccountName: c.GetAccountName(),
			CreatedAt:   rawTimeAgo(c.GetCreatedAt()),
			UpdatedAt:   rawTimeAgo(c.GetUpdatedAt()),
			raw:         mergeExtraProperties(c, c.GetExtraProperties()),
		})
	}

	r.Results(res)

	return nil
}

// FlowVaultConnectionShowRaw renders a full-fidelity vault connection response.
// The Management API never returns the write-only `setup` secrets, so nothing is
// masked here; the CLI simply never echoes the create/update body.
func (r *Renderer) FlowVaultConnectionShowRaw(heading string, connection json.RawMessage) error {
	view, err := makeFlowVaultConnectionViewFromRaw(connection)
	if err != nil {
		return fmt.Errorf("failed to parse vault connection response: %w", err)
	}
	r.Heading(heading)
	r.Result(view)
	return nil
}

func makeFlowVaultConnectionViewFromRaw(raw json.RawMessage) (*flowVaultConnectionView, error) {
	var connection struct {
		ID          string    `json:"id"`
		Name        string    `json:"name"`
		AppID       string    `json:"app_id"`
		Ready       bool      `json:"ready"`
		AccountName string    `json:"account_name"`
		CreatedAt   time.Time `json:"created_at"`
		UpdatedAt   time.Time `json:"updated_at"`
	}
	if err := json.Unmarshal(raw, &connection); err != nil {
		return nil, err
	}

	return &flowVaultConnectionView{
		ID:          connection.ID,
		Name:        connection.Name,
		AppID:       connection.AppID,
		Ready:       connection.Ready,
		AccountName: connection.AccountName,
		CreatedAt:   rawTimeAgo(connection.CreatedAt),
		UpdatedAt:   rawTimeAgo(connection.UpdatedAt),
		raw:         raw,
	}, nil
}

func boolToPresence(present bool) string {
	if present {
		return "set"
	}
	return "none"
}

func boolToReady(ready bool) string {
	if ready {
		return "yes"
	}
	return "no"
}

func rawTimeAgo(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return timeAgo(value)
}

func mergeExtraProperties(obj interface{}, extra map[string]interface{}) interface{} {
	if len(extra) == 0 {
		return obj
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return obj
	}
	var merged map[string]interface{}
	if err := json.Unmarshal(data, &merged); err != nil {
		return obj
	}
	for key, value := range extra {
		if _, ok := merged[key]; !ok {
			merged[key] = value
		}
	}
	return merged
}
