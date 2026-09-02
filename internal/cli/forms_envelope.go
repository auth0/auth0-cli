package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
)

// formEnvelopeVersion is the schema version the Auth0 Dashboard form builder
// stamps on exported forms. We emit the same value so exports interop.
const formEnvelopeVersion = "4.0.0"

// formEnvelope mirrors the export shape produced by the Auth0 Dashboard form
// builder: the form graph plus the flows and vault connections it references,
// with real resource IDs replaced by portable #FLOW-N#/#CONN-N# placeholders.
type formEnvelope struct {
	Version     string                     `json:"version"`
	Form        json.RawMessage            `json:"form"`
	Flows       map[string]json.RawMessage `json:"flows,omitempty"`
	Connections map[string]envelopeConn    `json:"connections,omitempty"`
}

// envelopeConn is the connection descriptor emitted alongside a form. Vault
// connection secrets are never exported, so on import the placeholder is mapped
// to an existing connection rather than recreated.
type envelopeConn struct {
	ID    string `json:"id"`
	AppID string `json:"app_id,omitempty"`
	Name  string `json:"name,omitempty"`
}

// isFormEnvelope reports whether the given body is a Dashboard-style envelope
// rather than a flat form graph. An envelope always carries a top-level "form"
// object, whereas a flat body carries the form fields such as name and nodes
// at the top level.
func isFormEnvelope(body []byte) bool {
	var probe struct {
		Form json.RawMessage `json:"form"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return false
	}
	return len(probe.Form) > 0
}

// substituteIDs replaces every JSON string value that exactly matches a key in
// `replacements` with its mapped value, walking the whole tree. IDs are opaque
// unique tokens, so exact full-string matching is safe and order-independent.
func substituteIDs(raw json.RawMessage, replacements map[string]string) (json.RawMessage, error) {
	if len(replacements) == 0 {
		return raw, nil
	}

	var tree interface{}
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, err
	}

	return json.Marshal(walkReplace(tree, replacements))
}

func walkReplace(node interface{}, replacements map[string]string) interface{} {
	switch v := node.(type) {
	case map[string]interface{}:
		for key, val := range v {
			v[key] = walkReplace(val, replacements)
		}
		return v
	case []interface{}:
		for i, val := range v {
			v[i] = walkReplace(val, replacements)
		}
		return v
	case string:
		if replaced, ok := replacements[v]; ok {
			return replaced
		}
		return v
	default:
		return node
	}
}

// collectConnectionIDs returns every value stored under a "connection_id" key
// anywhere in the given flow JSON, de-duplicated and sorted for stable ordering.
func collectConnectionIDs(raw json.RawMessage) ([]string, error) {
	var tree interface{}
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var walk func(node interface{})
	walk = func(node interface{}) {
		switch v := node.(type) {
		case map[string]interface{}:
			for key, val := range v {
				if key == "connection_id" {
					if s, ok := val.(string); ok && s != "" {
						seen[s] = true
					}
				}
				walk(val)
			}
		case []interface{}:
			for _, val := range v {
				walk(val)
			}
		}
	}
	walk(tree)

	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)

	return out, nil
}

// collectFlowIDs returns the flow IDs referenced by the form's FLOW nodes, in
// node order and de-duplicated. Working from the raw form map avoids the v3
// SDK's lossy FormNode union.
func collectFlowIDs(formMap map[string]interface{}) []string {
	nodes, ok := formMap["nodes"].([]interface{})
	if !ok {
		return nil
	}

	var ids []string
	seen := map[string]bool{}
	for _, n := range nodes {
		node, ok := n.(map[string]interface{})
		if !ok || node["type"] != "FLOW" {
			continue
		}
		config, ok := node["config"].(map[string]interface{})
		if !ok {
			continue
		}
		id, ok := config["flow_id"].(string)
		if !ok || id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}

	return ids
}

// vaultConnectionPickerOptions lists the tenant's flow vault connections as
// selectable options for mapping envelope connection placeholders on import.
func (c *cli) vaultConnectionPickerOptions(ctx context.Context) (pickerOptions, error) {
	var list *management.FlowVaultConnectionList
	if err := ansi.Waiting(func() (err error) {
		list, err = c.api.FlowVaultConnection.GetConnectionList(ctx)
		return err
	}); err != nil {
		return nil, err
	}

	var opts pickerOptions
	for _, conn := range list.Connections {
		label := fmt.Sprintf("%s %s", conn.GetName(), ansi.Faint("("+conn.GetID()+")"))
		opts = append(opts, pickerOption{value: conn.GetID(), label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("there are currently no vault connections to map to. Create one in the Auth0 Dashboard first")
	}

	return opts, nil
}

// resolveConnectionPlaceholders maps each #CONN-N# placeholder in the envelope
// to a real vault connection ID. It uses the provided mapping first and falls
// back to an interactive picker; without a terminal an unmapped placeholder is
// an error that tells the user to pass --connection.
func (c *cli) resolveConnectionPlaceholders(
	cmd *cobra.Command,
	env *formEnvelope,
	mapping map[string]string,
) (map[string]string, error) {
	placeholders := make([]string, 0, len(env.Connections))
	for ph := range env.Connections {
		placeholders = append(placeholders, ph)
	}
	sort.Strings(placeholders)

	var options pickerOptions
	resolved := make(map[string]string, len(placeholders))
	for _, ph := range placeholders {
		if id := mapping[ph]; id != "" {
			resolved[ph] = id
			continue
		}

		if !canPrompt(cmd) {
			return nil, fmt.Errorf(
				"cannot resolve connection %s: pass --connection '%s=<connection-id>' or run without --no-input",
				ph, ph,
			)
		}

		if options == nil {
			opts, err := c.vaultConnectionPickerOptions(cmd.Context())
			if err != nil {
				return nil, err
			}
			options = opts
		}

		var label string
		message := fmt.Sprintf("Select the vault connection for %s (%s):", ph, env.Connections[ph].Name)
		if err := prompt.AskOne(
			prompt.SelectInput("connection", message, "", options.labels(), options.defaultLabel(), true),
			&label,
		); err != nil {
			return nil, err
		}
		resolved[ph] = options.getValue(label)
	}

	return resolved, nil
}

// resolveFormEnvelope turns a Dashboard-style envelope into a flat form body
// ready for create/update: it maps connection placeholders to existing vault
// connections, creates the bundled flows (substituting the resolved connection
// IDs into them), and swaps the form's #FLOW-N# references for the new flow IDs.
func (c *cli) resolveFormEnvelope(
	cmd *cobra.Command,
	body []byte,
	mapping map[string]string,
) (json.RawMessage, error) {
	var env formEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("failed to parse form body: %w", err)
	}
	if len(env.Form) == 0 {
		return nil, errors.New("the imported envelope has no \"form\" object")
	}

	connReplacements, err := c.resolveConnectionPlaceholders(cmd, &env, mapping)
	if err != nil {
		return nil, err
	}

	// Create flows in placeholder order for a deterministic sequence.
	placeholders := make([]string, 0, len(env.Flows))
	for ph := range env.Flows {
		placeholders = append(placeholders, ph)
	}
	sort.Strings(placeholders)

	flowReplacements := make(map[string]string, len(placeholders))
	for _, ph := range placeholders {
		flowRaw, err := substituteIDs(env.Flows[ph], connReplacements)
		if err != nil {
			return nil, err
		}

		flow := &management.Flow{}
		if err := json.Unmarshal(flowRaw, flow); err != nil {
			return nil, fmt.Errorf("failed to parse flow %s: %w", ph, err)
		}
		if err := ansi.Waiting(func() error {
			return c.api.Flow.Create(cmd.Context(), flow)
		}); err != nil {
			return nil, fmt.Errorf("failed to create flow %s: %w", ph, err)
		}
		flowReplacements[ph] = flow.GetID()
	}

	return substituteIDs(env.Form, flowReplacements)
}

// buildFormEnvelope turns a fetched form (raw wire JSON) into a Dashboard-style
// envelope: it reads every flow the form's FLOW nodes reference and every vault
// connection those flows reference, then swaps the real IDs for #FLOW-N#/#CONN-N#
// placeholders so the export is portable across tenants. The form is handled as
// raw JSON so STEP/ROUTER node config survives the round-trip.
func (c *cli) buildFormEnvelope(
	ctx context.Context,
	formRaw json.RawMessage,
) (*formEnvelope, error) {
	var formMap map[string]interface{}
	if err := json.Unmarshal(formRaw, &formMap); err != nil {
		return nil, fmt.Errorf("failed to parse form: %w", err)
	}

	// Referenced flow IDs, in node order, de-duplicated.
	flowIDs := collectFlowIDs(formMap)

	// Read each flow and gather the connections its actions reference.
	flowsByID := make(map[string]json.RawMessage, len(flowIDs))
	connSet := map[string]bool{}
	for _, id := range flowIDs {
		var flow *management.Flow
		if err := ansi.Waiting(func() (err error) {
			flow, err = c.api.Flow.Read(ctx, id)
			return err
		}); err != nil {
			return nil, fmt.Errorf("failed to read flow with ID %q: %w", id, err)
		}

		raw, err := json.Marshal(flow)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal flow with ID %q: %w", id, err)
		}
		flowsByID[id] = raw

		connIDs, err := collectConnectionIDs(raw)
		if err != nil {
			return nil, err
		}
		for _, cid := range connIDs {
			connSet[cid] = true
		}
	}

	connIDs := make([]string, 0, len(connSet))
	for id := range connSet {
		connIDs = append(connIDs, id)
	}
	sort.Strings(connIDs)

	// Assign placeholders and build the real-ID -> placeholder replacement map.
	replacements := make(map[string]string, len(flowIDs)+len(connIDs))
	flowPlaceholder := make(map[string]string, len(flowIDs))
	for i, id := range flowIDs {
		ph := fmt.Sprintf("#FLOW-%d#", i+1)
		replacements[id] = ph
		flowPlaceholder[id] = ph
	}
	connPlaceholder := make(map[string]string, len(connIDs))
	for i, id := range connIDs {
		ph := fmt.Sprintf("#CONN-%d#", i+1)
		replacements[id] = ph
		connPlaceholder[id] = ph
	}

	// Drop volatile fields and swap in placeholders.
	for _, field := range formServerManagedFields {
		delete(formMap, field)
	}
	formBody, err := json.Marshal(formMap)
	if err != nil {
		return nil, err
	}
	formBody, err = substituteIDs(formBody, replacements)
	if err != nil {
		return nil, err
	}

	env := &formEnvelope{Version: formEnvelopeVersion, Form: formBody}

	if len(flowsByID) > 0 {
		env.Flows = make(map[string]json.RawMessage, len(flowsByID))
		for id, raw := range flowsByID {
			substituted, err := substituteIDs(raw, replacements)
			if err != nil {
				return nil, err
			}
			env.Flows[flowPlaceholder[id]] = substituted
		}
	}

	if len(connIDs) > 0 {
		env.Connections = make(map[string]envelopeConn, len(connIDs))
		for _, id := range connIDs {
			var conn *management.FlowVaultConnection
			if err := ansi.Waiting(func() (err error) {
				conn, err = c.api.FlowVaultConnection.GetConnection(ctx, id)
				return err
			}); err != nil {
				return nil, fmt.Errorf("failed to read vault connection with ID %q: %w", id, err)
			}
			env.Connections[connPlaceholder[id]] = envelopeConn{
				ID:    conn.GetID(),
				AppID: conn.GetAppID(),
				Name:  conn.GetName(),
			}
		}
	}

	return env, nil
}
