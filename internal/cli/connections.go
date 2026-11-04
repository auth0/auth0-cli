package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
)

var (
	connectionIDArg = Argument{
		Name: "Id",
		Help: "Id of the connection.",
	}

	connectionName = Flag{
		Name:       "Name",
		LongForm:   "name",
		ShortForm:  "n",
		Help:       "Name of the connection.",
		IsRequired: true,
	}

	connectionStrategy = Flag{
		Name:       "Strategy",
		LongForm:   "strategy",
		ShortForm:  "s",
		Help:       "Strategy of the connection. Determines the identity provider (e.g. auth0, google-oauth2, samlp, oidc, waad, ad, oauth2).",
		IsRequired: true,
	}

	// ConnectionCommonStrategies powers the interactive strategy picker. Any
	// strategy string is accepted via --strategy; this is only a convenience list.
	connectionCommonStrategies = []string{
		"auth0",
		"google-oauth2",
		"facebook",
		"apple",
		"github",
		"windowslive",
		"linkedin",
		"samlp",
		"oidc",
		"okta",
		"waad",
		"adfs",
		"ad",
		"google-apps",
		"email",
		"sms",
		"oauth2",
	}

	// ConnectionImmutableFields are dropped before pre-filling the interactive
	// update editor: they are set at create time and cannot be changed via PATCH.
	connectionImmutableFields = []string{"id", "name", "strategy"}
)

func connectionsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connections",
		Short: "Manage resources for connections",
		Long: `Connections are sources of users, linking your applications to the identity providers
that authenticate them: database, social (Google, Facebook, ...), and enterprise
(SAML, OIDC, Azure AD, LDAP, ...) connections.

## Schema Discovery & JSON Input

Use '--schema' on a command to print its request payload schema, and '--data'
to provide that payload programmatically (validated against the schema before the call).

Examples:
  auth0 connections create --schema                       # Show the create payload schema
  auth0 connections create --data @connection.json        # Create from JSON file
  auth0 connections create --data '{"name":"..."}'        # Create from inline JSON

For more details: https://auth0.com/docs/api/management/v2`,
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listConnectionsCmd(cli))
	cmd.AddCommand(createConnectionCmd(cli))
	cmd.AddCommand(showConnectionCmd(cli))
	cmd.AddCommand(updateConnectionCmd(cli))
	cmd.AddCommand(deleteConnectionCmd(cli))
	cmd.AddCommand(connectionsEnabledClientsCmd(cli))

	return cmd
}

func listConnectionsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Schema bool
		Query  string
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your connections",
		Long: `List your existing connections. To create one, run: ` + "`auth0 connections create`" + `.

Use '--schema' to see available query parameters.
Use '--query' to filter results via a JSON object (any API-supported parameter works immediately).`,
		Example: `  auth0 connections list
  auth0 connections ls
  auth0 connections ls --json
  auth0 connections ls --csv
  auth0 connections list --schema
  auth0 connections list --query '{"strategy":["auth0"]}'
  auth0 connections list --query '{"name":"my-connection"}' --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Schema {
				return printOperationSchema(cli, "GET", "/connections")
			}

			if inputs.Query != "" {
				return runJSONQuery(cli, cmd, jsonQuerySpec{
					Path:      "connections",
					SchemaCmd: "auth0 connections list",
				}, inputs.Query)
			}

			var list []*managementv3.ConnectionForList
			if err := ansi.Waiting(func() (err error) {
				list, err = collectConnections(cmd.Context(), cli, &managementv3.ListConnectionsQueryParameters{}, defaultPageSize)
				return err
			}); err != nil {
				return fmt.Errorf("failed to list connections: %w", err)
			}

			return cli.renderer.ConnectionList(list)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")
	schemaFlag.RegisterBool(cmd, &inputs.Schema, false)
	listQueryFlag.RegisterString(cmd, &inputs.Query, "")

	return cmd
}

func showConnectionCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show a connection",
		Long:  "Display the full configuration of a connection, including its strategy-specific options.",
		Example: `  auth0 connections show
  auth0 connections show <connection-id>
  auth0 connections show <connection-id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cli.resolveConnectionID(cmd, args, &inputs.ID); err != nil {
				return err
			}

			raw, err := cli.connectionRawGet(cmd.Context(), inputs.ID)
			if err != nil {
				return fmt.Errorf("failed to read connection with ID %q: %w", inputs.ID, err)
			}

			return cli.renderer.ConnectionShowRaw(raw)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func createConnectionCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Name     string
		Strategy string
		Data     string
		Schema   bool
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new connection",
		Long: `Create a new connection.

To create interactively, use 'auth0 connections create' with no flags.

To create non-interactively, supply the connection name and strategy through the flags,
or the whole payload through '--data'.

## JSON Input (for agents and automation)

Use '--schema' to print the request payload schema, then '--data' to provide
connection data as JSON:
  - Inline JSON: --data '{"name":"my-connection","strategy":"auth0"}'
  - From file: --data @connection.json
  - From stdin: pipe data in (e.g. cat connection.json | auth0 connections create)

The JSON is validated against the OpenAPI schema before sending to the API.`,
		Example: `  # Interactive mode
  auth0 connections create

  # Flag-based mode
  auth0 connections create --name my-db --strategy auth0

  # JSON mode
  auth0 connections create --schema
  auth0 connections create --data @connection.json
  auth0 connections create --data '{"name":"my-db","strategy":"auth0"}'
  cat connection.json | auth0 connections create`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Schema {
				return printOperationSchema(cli, "POST", "/connections")
			}

			// JSON input mode (for agents and automation): explicit --data or piped stdin.
			payload, provided, err := ResolveData(cmd)
			if err != nil {
				return err
			}
			if provided {
				return cli.createConnectionFromJSON(cmd, payload)
			}

			if err := connectionName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			if err := connectionStrategy.Select(cmd, &inputs.Strategy, connectionCommonStrategies, nil); err != nil {
				return err
			}

			body, err := json.Marshal(map[string]string{
				"name":     inputs.Name,
				"strategy": inputs.Strategy,
			})
			if err != nil {
				return err
			}

			// The flag path builds a trivially-valid {name, strategy} body, so it
			// sends raw and skips the schema handler. That keeps interactive and
			// flag-based creation working even when the OpenAPI schema can't be
			// fetched (e.g. a fresh machine that is offline).
			return cli.sendConnectionCreate(cmd, body)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	connectionName.RegisterString(cmd, &inputs.Name, "")
	connectionStrategy.RegisterString(cmd, &inputs.Strategy, "")
	dataFlag.RegisterString(cmd, &inputs.Data, "")
	schemaFlag.RegisterBool(cmd, &inputs.Schema, false)

	// --data supplies the whole payload, so it cannot be combined with the
	// granular input flags. Output flags (--json) and --schema are not affected.
	markDataExclusive(cmd)

	return cmd
}

func updateConnectionCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID     string
		Data   string
		Schema bool
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update a connection",
		Long: `Update a connection.

To update interactively, use 'auth0 connections update' with no '--data' flag: the
current configuration opens in your editor and the saved result is sent as a PATCH.

To update non-interactively, supply the new configuration through '--data'.

Note: the entire 'options' object is overridden on update, so include all option
fields you want to keep.

## JSON Input (for agents and automation)

Use '--schema' to print the request payload schema, then '--data' to provide
connection data as JSON (inline, @file, or piped stdin). The JSON is validated
against the OpenAPI schema before sending to the API.`,
		Example: `  auth0 connections update <connection-id>
  auth0 connections update <connection-id> --data @connection.json
  auth0 connections update <connection-id> --data '{"options":{...}}'
  auth0 connections update --schema`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Schema {
				return printOperationSchema(cli, "PATCH", "/connections/{id}")
			}

			if err := cli.resolveConnectionID(cmd, args, &inputs.ID); err != nil {
				return err
			}

			payload, provided, err := ResolveData(cmd)
			if err != nil {
				return err
			}

			if !provided {
				// Interactive: pre-fill the editor with the current (mutable) config.
				current, err := cli.connectionRawGet(cmd.Context(), inputs.ID)
				if err != nil {
					return fmt.Errorf("failed to read connection with ID %q: %w", inputs.ID, err)
				}

				editable, err := stripConnectionImmutableFields(current)
				if err != nil {
					return err
				}

				var edited string
				if err := dataFlag.OpenEditor(
					cmd,
					&edited,
					string(editable),
					inputs.ID+".connection.*.json",
					cli.connectionEditorHint,
				); err != nil {
					return err
				}
				payload = edited
			}

			return cli.updateConnectionFromJSON(cmd, inputs.ID, payload)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	dataFlag.RegisterString(cmd, &inputs.Data, "")
	schemaFlag.RegisterBool(cmd, &inputs.Schema, false)

	// --data supplies the whole payload, so it cannot be combined with granular
	// input flags. There are none today, so this is a no-op that keeps future
	// per-field update flags automatically exclusive with --data.
	markDataExclusive(cmd)

	return cmd
}

func deleteConnectionCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Args:    cobra.MinimumNArgs(0),
		Short:   "Delete a connection",
		Long: "Delete a connection.\n\n" +
			"To delete interactively, use `auth0 connections delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the connection id and the `--force` flag to skip confirmation.",
		Example: `  auth0 connections delete
  auth0 connections rm
  auth0 connections delete <connection-id>
  auth0 connections delete <connection-id> --force
  auth0 connections delete <connection-id> <connection-id2> <connection-idn>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var ids []string
			if len(args) == 0 {
				if err := connectionIDArg.PickMany(cmd, &ids, cli.connectionPickerOptions); err != nil {
					return err
				}
			} else {
				ids = args
			}

			if !cli.force && cli.agentMode {
				return errDestructiveNoConfirm
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			return ansi.ProgressBar("Deleting connection(s)", ids, func(_ int, id string) error {
				if id != "" {
					if err := cli.apiv3.Connection.Delete(cmd.Context(), id); err != nil {
						return fmt.Errorf("failed to delete connection with ID %q: %w", id, err)
					}
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

func connectionsEnabledClientsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enabled-clients",
		Short: "Manage the clients enabled on a connection",
		Long:  "Manage which applications (clients) are able to use a connection.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(showConnectionEnabledClientsCmd(cli))
	cmd.AddCommand(updateConnectionEnabledClientsCmd(cli))

	return cmd
}

func showConnectionEnabledClientsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show the clients enabled on a connection",
		Long:  "List the applications (clients) that have this connection enabled.",
		Example: `  auth0 connections enabled-clients show
  auth0 connections enabled-clients show <connection-id>
  auth0 connections enabled-clients show <connection-id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cli.resolveConnectionID(cmd, args, &inputs.ID); err != nil {
				return err
			}

			var clients []*managementv3.ConnectionEnabledClient
			if err := ansi.Waiting(func() (err error) {
				clients, err = collectConnectionEnabledClients(cmd.Context(), cli, inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to read enabled clients for connection %q: %w", inputs.ID, err)
			}

			return cli.renderer.ConnectionEnabledClientList(clients)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func updateConnectionEnabledClientsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID     string
		Data   string
		Schema bool
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update the clients enabled on a connection",
		Long: `Update which applications (clients) have this connection enabled.

To update interactively, run without '--data': the tenant's applications are listed
with the currently-enabled ones pre-selected, and only your changes are sent.

To update non-interactively, supply the desired client statuses through '--data' as a
JSON array of objects with 'client_id' and 'status' fields (up to 50 per request).

Use '--schema' to print the request payload schema.`,
		Example: `  auth0 connections enabled-clients update
  auth0 connections enabled-clients update <connection-id>
  auth0 connections enabled-clients update <connection-id> --data @clients.json
  auth0 connections enabled-clients update <connection-id> --data '[{"client_id":"abc","status":true}]'
  auth0 connections enabled-clients update --schema`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Schema {
				return printOperationSchema(cli, "PATCH", "/connections/{id}/clients")
			}

			if err := cli.resolveConnectionID(cmd, args, &inputs.ID); err != nil {
				return err
			}

			payload, provided, err := ResolveData(cmd)
			if err != nil {
				return err
			}

			var clients managementv3.UpdateEnabledClientConnectionsRequestContent
			if provided {
				if err := json.Unmarshal([]byte(payload), &clients); err != nil {
					return fmt.Errorf("invalid --data value: must be a JSON array of {client_id, status}: %w", err)
				}
			} else {
				if !canPrompt(cmd) {
					return errors.New("no client statuses provided: pass them with --data as a JSON array of {client_id, status}")
				}

				clients, err = cli.pickConnectionEnabledClients(cmd, inputs.ID)
				if err != nil {
					return err
				}
				if len(clients) == 0 {
					cli.renderer.Infof("No changes to the enabled clients for connection %s.", inputs.ID)
					return nil
				}
			}

			if err := ansi.Waiting(func() error {
				return cli.apiv3.ConnectionEnabledClient.Update(cmd.Context(), inputs.ID, clients)
			}); err != nil {
				return fmt.Errorf("failed to update enabled clients for connection %q: %w", inputs.ID, err)
			}

			cli.renderer.Infof("Updated enabled clients for connection %s", inputs.ID)
			return nil
		},
	}

	dataFlag.RegisterString(cmd, &inputs.Data, "")
	schemaFlag.RegisterBool(cmd, &inputs.Schema, false)

	return cmd
}

// --- Raw HTTP writes (full-fidelity, schema-validated) ---.

func (c *cli) createConnectionFromJSON(cmd *cobra.Command, payload string) error {
	body, validated, err := readAndValidateJSON(c, payload, http.MethodPost, "/connections")
	if err != nil {
		c.renderer.Infof("Run 'auth0 connections create --schema' to see the expected schema.")
		return err
	}

	if !validated {
		c.renderer.Warnf(
			"No local schema found for %s %s; sending --data to the API without local validation.",
			http.MethodPost, "/connections",
		)
	}

	return c.sendConnectionCreate(cmd, body)
}

// sendConnectionCreate POSTs a connection body to the API verbatim and renders
// the result. It is shared by the --data path (which validates against the schema
// first) and the flag path (name/strategy only), so the trivially-valid flag
// payload never has to load the OpenAPI schema.
func (c *cli) sendConnectionCreate(cmd *cobra.Command, body json.RawMessage) error {
	raw, err := c.rawJSONRequest(cmd.Context(), http.MethodPost, c.api.HTTPClient.URI("connections"), body)
	if err != nil {
		return fmt.Errorf("failed to create connection: %w", c.enhanceConnectionAPIError(err, http.MethodPost, "/connections"))
	}

	return c.renderer.ConnectionCreateRaw(raw)
}

// enhanceConnectionAPIError appends the expected request schema to an API error
// for human output, matching runJSONWrite. In JSON/agent error mode the schema
// dump is noise inside the error envelope, so the raw API error is returned
// unchanged and the schema stays discoverable via --schema.
func (c *cli) enhanceConnectionAPIError(err error, method, path string) error {
	if c.wantsJSONError() {
		return err
	}
	return enhanceAPIError(err, method, path)
}

func (c *cli) updateConnectionFromJSON(cmd *cobra.Command, id, payload string) error {
	body, validated, err := readAndValidateJSON(c, payload, http.MethodPatch, "/connections/{id}")
	if err != nil {
		c.renderer.Infof("Run 'auth0 connections update --schema' to see the expected schema.")
		return err
	}

	if !validated {
		c.renderer.Warnf(
			"No local schema found for %s %s; sending --data to the API without local validation.",
			http.MethodPatch, "/connections/{id}",
		)
	}

	raw, err := c.rawJSONRequest(cmd.Context(), http.MethodPatch, c.api.HTTPClient.URI("connections", id), body)
	if err != nil {
		return fmt.Errorf("failed to update connection with ID %q: %w", id, c.enhanceConnectionAPIError(err, http.MethodPatch, "/connections/{id}"))
	}

	return c.renderer.ConnectionUpdateRaw(raw)
}

func (c *cli) connectionRawGet(ctx context.Context, id string) (json.RawMessage, error) {
	return c.rawJSONRequest(ctx, http.MethodGet, c.api.HTTPClient.URI("connections", id), nil)
}

// stripConnectionImmutableFields removes fields that cannot change on PATCH so the
// interactive editor only shows the mutable configuration.
func stripConnectionImmutableFields(body json.RawMessage) (json.RawMessage, error) {
	var connection map[string]json.RawMessage
	if err := json.Unmarshal(body, &connection); err != nil {
		return nil, err
	}
	for _, field := range connectionImmutableFields {
		delete(connection, field)
	}
	return json.MarshalIndent(connection, "", "  ")
}

// --- Paging + pickers ---.

func collectConnections(ctx context.Context, cli *cli, params *managementv3.ListConnectionsQueryParameters, limit int) ([]*managementv3.ConnectionForList, error) {
	page, err := cli.apiv3.Connection.List(ctx, params)
	if err != nil {
		return nil, err
	}

	var out []*managementv3.ConnectionForList
	for page != nil {
		for _, c := range page.Results {
			out = append(out, c)
			if limit > 0 && len(out) >= limit {
				return out, nil
			}
		}

		page, err = page.GetNextPage(ctx)
		if errors.Is(err, core.ErrNoPages) {
			break
		}
		if err != nil {
			return out, err
		}
	}

	return out, nil
}

func collectConnectionEnabledClients(ctx context.Context, cli *cli, id string) ([]*managementv3.ConnectionEnabledClient, error) {
	page, err := cli.apiv3.ConnectionEnabledClient.Get(ctx, id, &managementv3.GetConnectionEnabledClientsRequestParameters{})
	if err != nil {
		return nil, err
	}

	var out []*managementv3.ConnectionEnabledClient
	for page != nil {
		out = append(out, page.Results...)

		page, err = page.GetNextPage(ctx)
		if errors.Is(err, core.ErrNoPages) {
			break
		}
		if err != nil {
			return out, err
		}
	}

	return out, nil
}

func (c *cli) resolveConnectionID(cmd *cobra.Command, args []string, id *string) error {
	if len(args) > 0 {
		*id = args[0]
		return nil
	}
	return connectionIDArg.Pick(cmd, id, c.connectionPickerOptions)
}

func (c *cli) connectionPickerOptions(ctx context.Context) (pickerOptions, error) {
	list, err := collectConnections(ctx, c, &managementv3.ListConnectionsQueryParameters{}, 0)
	if err != nil {
		return nil, err
	}

	var opts pickerOptions
	for _, r := range list {
		label := fmt.Sprintf("%s %s", r.GetName(), ansi.Faint("("+r.GetID()+")"))
		opts = append(opts, pickerOption{value: r.GetID(), label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("there are currently no connections to choose from. Create one by running: `auth0 connections create`")
	}

	return opts, nil
}

// pickConnectionEnabledClients drives the interactive enabled-clients selection.
// It lists the tenant's applications with the currently-enabled ones pre-selected,
// then returns only the status changes (newly selected -> true, deselected -> false),
// so unrelated clients are never touched.
func (c *cli) pickConnectionEnabledClients(cmd *cobra.Command, id string) (managementv3.UpdateEnabledClientConnectionsRequestContent, error) {
	var opts pickerOptions
	var enabled []*managementv3.ConnectionEnabledClient
	if err := ansi.Waiting(func() (err error) {
		if opts, err = c.appPickerOptions()(cmd.Context()); err != nil {
			return err
		}
		enabled, err = collectConnectionEnabledClients(cmd.Context(), c, id)
		return err
	}); err != nil {
		return nil, err
	}

	enabledSet := make(map[string]bool, len(enabled))
	for _, e := range enabled {
		enabledSet[e.GetClientID()] = true
	}

	defaults := make([]string, 0, len(enabled))
	for _, o := range opts {
		if enabledSet[o.value] {
			defaults = append(defaults, o.label)
		}
	}

	var selectedLabels []string
	if err := prompt.AskMultiSelectWithDefault(
		"Select the applications that should have this connection enabled:",
		&selectedLabels,
		defaults,
		opts.labels()...,
	); err != nil {
		return nil, err
	}

	selectedSet := make(map[string]bool, len(selectedLabels))
	for _, v := range opts.getValues(selectedLabels...) {
		selectedSet[v] = true
	}

	// Only send the diff: enable what was newly selected, disable what was cleared.
	var changes managementv3.UpdateEnabledClientConnectionsRequestContent
	for _, o := range opts {
		switch {
		case selectedSet[o.value] && !enabledSet[o.value]:
			changes = append(changes, &managementv3.UpdateEnabledClientConnectionsRequestContentItem{ClientID: o.value, Status: true})
		case !selectedSet[o.value] && enabledSet[o.value]:
			changes = append(changes, &managementv3.UpdateEnabledClientConnectionsRequestContentItem{ClientID: o.value, Status: false})
		}
	}

	if len(changes) > 50 {
		return nil, fmt.Errorf("too many changes at once (%d): the API accepts up to 50 client status changes per update", len(changes))
	}

	return changes, nil
}

func (c *cli) connectionEditorHint() {
	c.renderer.Infof("%s Once you close the editor, the connection will be updated. To cancel, press CTRL+C.", ansi.Faint("Hint:"))
}
