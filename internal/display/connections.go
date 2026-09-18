package display

import (
	"encoding/json"
	"fmt"
	"strings"

	managementv3 "github.com/auth0/go-auth0/v3/management"

	"github.com/auth0/auth0-cli/internal/ansi"
)

type connectionView struct {
	ID          string
	Name        string
	DisplayName string
	Strategy    string
	Realms      string

	raw interface{}
}

func (v *connectionView) AsTableHeader() []string {
	return []string{"ID", "Name", "Strategy", "Realms"}
}

func (v *connectionView) AsTableRow() []string {
	return []string{ansi.Faint(v.ID), v.Name, v.Strategy, v.Realms}
}

func (v *connectionView) KeyValues() [][]string {
	kvs := [][]string{
		{"ID", ansi.Faint(v.ID)},
		{"NAME", v.Name},
	}
	if v.DisplayName != "" {
		kvs = append(kvs, []string{"DISPLAY NAME", v.DisplayName})
	}
	kvs = append(kvs,
		[]string{"STRATEGY", v.Strategy},
		[]string{"REALMS", v.Realms},
	)
	return kvs
}

func (v *connectionView) Object() interface{} {
	return v.raw
}

type connectionEnabledClientView struct {
	ClientID string

	raw interface{}
}

func (v *connectionEnabledClientView) AsTableHeader() []string {
	return []string{"Client ID"}
}

func (v *connectionEnabledClientView) AsTableRow() []string {
	return []string{ansi.Faint(v.ClientID)}
}

func (v *connectionEnabledClientView) KeyValues() [][]string {
	return [][]string{{"CLIENT ID", ansi.Faint(v.ClientID)}}
}

func (v *connectionEnabledClientView) Object() interface{} {
	return v.raw
}

// ConnectionList renders the connections list read through the v3 SDK.
func (r *Renderer) ConnectionList(connections []*managementv3.ConnectionForList) error {
	resource := "connections"

	r.Heading(resource)

	if len(connections) == 0 {
		r.EmptyState(resource, "Use 'auth0 connections create' to add one")
		return nil
	}

	var res []View
	for _, c := range connections {
		res = append(res, makeConnectionSummaryView(c))
	}

	r.Results(res)

	return nil
}

// ConnectionShowRaw renders a full-fidelity connection response read through the
// v1 HTTP client, avoiding the v3 SDK's lossy per-strategy option unions.
func (r *Renderer) ConnectionShowRaw(connection json.RawMessage) error {
	return r.renderRawConnection("connection", connection)
}

// ConnectionCreateRaw renders a full-fidelity create response.
func (r *Renderer) ConnectionCreateRaw(connection json.RawMessage) error {
	return r.renderRawConnection("connection created", connection)
}

// ConnectionUpdateRaw renders a full-fidelity update response.
func (r *Renderer) ConnectionUpdateRaw(connection json.RawMessage) error {
	return r.renderRawConnection("connection updated", connection)
}

func (r *Renderer) renderRawConnection(heading string, connection json.RawMessage) error {
	view, err := makeConnectionViewFromRaw(connection)
	if err != nil {
		return fmt.Errorf("failed to parse connection response: %w", err)
	}
	r.Heading(heading)
	r.Result(view)
	return nil
}

// ConnectionEnabledClientList renders the clients that have a connection enabled.
func (r *Renderer) ConnectionEnabledClientList(clients []*managementv3.ConnectionEnabledClient) error {
	resource := "enabled clients"

	r.Heading(resource)

	if len(clients) == 0 {
		r.EmptyState(resource, "Use 'auth0 connections enabled-clients update' to enable one")
		return nil
	}

	var res []View
	for _, c := range clients {
		res = append(res, &connectionEnabledClientView{ClientID: c.ClientID, raw: c})
	}

	r.Results(res)

	return nil
}

func makeConnectionSummaryView(c *managementv3.ConnectionForList) *connectionView {
	return &connectionView{
		ID:          c.GetID(),
		Name:        c.GetName(),
		DisplayName: c.GetDisplayName(),
		Strategy:    c.GetStrategy(),
		Realms:      formatConnectionRealms(c.GetRealms()),
		raw:         mergeExtraProperties(c, c.GetExtraProperties()),
	}
}

func makeConnectionViewFromRaw(raw json.RawMessage) (*connectionView, error) {
	var connection struct {
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		DisplayName string   `json:"display_name"`
		Strategy    string   `json:"strategy"`
		Realms      []string `json:"realms"`
	}
	if err := json.Unmarshal(raw, &connection); err != nil {
		return nil, err
	}

	return &connectionView{
		ID:          connection.ID,
		Name:        connection.Name,
		DisplayName: connection.DisplayName,
		Strategy:    connection.Strategy,
		Realms:      formatConnectionRealms(connection.Realms),
		raw:         raw,
	}, nil
}

func formatConnectionRealms(realms []string) string {
	if len(realms) == 0 {
		return "-"
	}
	return strings.Join(realms, ", ")
}
