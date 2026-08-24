package cli

import (
	"bytes"
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

// flowCreateSkeleton seeds the editor for interactive flow creation. The name is
// prompted separately, so the seed only carries the empty actions container.
const flowCreateSkeleton = `{
  "actions": []
}
`

const flowCreateExample = `{
  "name": "Enrich Profile",
  "actions": [
    {
      "id": "step_http",
      "type": "HTTP",
      "action": "SEND_REQUEST",
      "allow_failure": false,
      "mask_output": false,
      "params": {
        "method": "GET",
        "url": "https://api.example.com/enrich",
        "content_type": "JSON"
      }
    }
  ]
}
`

// flowServerManagedFields cannot be sent in create or update request bodies.
var flowServerManagedFields = []string{
	"id",
	"created_at",
	"updated_at",
	"executed_at",
}

// vaultConnectionServerManagedFields cannot be sent in update request bodies.
var vaultConnectionServerManagedFields = []string{
	"id",
	"created_at",
	"updated_at",
	"refreshed_at",
	"ready",
	"fingerprint",
}

var (
	flowID = Argument{
		Name: "Id",
		Help: "Id of the Flow.",
	}

	flowExecutionID = Argument{
		Name: "Execution Id",
		Help: "Id of the Flow execution.",
	}

	vaultConnectionID = Argument{
		Name: "Id",
		Help: "Id of the Vault connection.",
	}

	vaultAppID = Argument{
		Name: "App Id",
		Help: "Identifier of the Vault app to open (e.g. AUTH0, JWT, HTTP, SLACK).",
	}

	flowName = Flag{
		Name:     "Name",
		LongForm: "name",
		Help:     "Name of the Flow.",
	}

	flowFile = Flag{
		Name:      "File",
		LongForm:  "file",
		ShortForm: "f",
		Help:      "Path to a JSON file with the flow body. Use '-' to read from stdin.",
	}

	flowEdit = Flag{
		Name:     "Edit",
		LongForm: "edit",
		Help:     "Open an editor to author the flow graph after entering the name.",
	}

	flowExample = Flag{
		Name:     "Example",
		LongForm: "example",
		Help:     "Print an example flow JSON body and exit.",
	}

	flowHydrate = Flag{
		Name:     "Hydrate",
		LongForm: "hydrate",
		Help:     "Hydrate the response with the number of forms referencing each flow.",
	}

	vaultConnectionName = Flag{
		Name:     "Name",
		LongForm: "name",
		Help:     "Name of the Vault connection.",
	}

	vaultConnectionAppID = Flag{
		Name:     "App Id",
		LongForm: "app-id",
		Help:     "Identifier of the app the Vault connection integrates with (e.g. HTTP, SLACK).",
	}

	vaultConnectionFile = Flag{
		Name:      "File",
		LongForm:  "file",
		ShortForm: "f",
		Help:      "Path to a JSON file with the vault connection body (including its setup secrets). Use '-' to read from stdin.",
	}
)

const vaultConnectionExample = `{
  "app_id": "HTTP",
  "name": "My HTTP Connection",
  "setup": {
    "type": "BEARER",
    "token": "REPLACE_WITH_YOUR_TOKEN"
  }
}
`

// vaultConnectionCreateSkeleton seeds the editor for interactive vault connection
// creation. The name and app id are prompted separately, so the seed only carries
// a provider-specific setup template for the user to edit.
const vaultConnectionCreateSkeleton = `{
  "setup": {
    "type": "BEARER",
    "token": "REPLACE_WITH_YOUR_TOKEN"
  }
}
`

func flowsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "flows",
		Short: "Manage Flows",
		Long: "Flows let you orchestrate custom logic during authentication and other journeys, " +
			"chaining actions such as HTTP requests and vault-backed integrations.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listFlowsCmd(cli))
	cmd.AddCommand(showFlowCmd(cli))
	cmd.AddCommand(createFlowCmd(cli))
	cmd.AddCommand(updateFlowCmd(cli))
	cmd.AddCommand(deleteFlowCmd(cli))
	cmd.AddCommand(openFlowCmd(cli))
	cmd.AddCommand(flowExecutionsCmd(cli))
	cmd.AddCommand(flowVaultCmd(cli))

	return cmd
}

func listFlowsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Number      int
		Hydrate     bool
		Synchronous bool
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your flows",
		Long:    "List your existing flows. To create one, run: `auth0 flows create`.",
		Example: `  auth0 flows list
  auth0 flows ls
  auth0 flows ls --number 100
  auth0 flows ls --hydrate
  auth0 flows ls --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			params := &managementv3.ListFlowsRequestParameters{}
			if inputs.Hydrate {
				params.Hydrate = []*managementv3.ListFlowsRequestParametersHydrateEnum{
					managementv3.ListFlowsRequestParametersHydrateEnumFormCount.Ptr(),
				}
			}
			if cmd.Flags().Changed("synchronous") {
				params.Synchronous = &inputs.Synchronous
			}

			var flows []*managementv3.FlowSummary
			if err := ansi.Waiting(func() (err error) {
				flows, err = collectFlows(cmd.Context(), cli, params, inputs.Number)
				return err
			}); err != nil {
				return fmt.Errorf("failed to list flows: %w", err)
			}

			return cli.renderer.FlowsList(flows)
		},
	}

	cmd.Flags().IntVarP(&inputs.Number, "number", "n", 100, "Number of flows to retrieve. Fetched across pages.")
	flowHydrate.RegisterBool(cmd, &inputs.Hydrate, false)
	cmd.Flags().BoolVar(&inputs.Synchronous, "synchronous", false, "Filter to synchronous (true) or asynchronous (false) flows.")
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	return cmd
}

func showFlowCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show a flow",
		Long:  "Display information about a flow.",
		Example: `  auth0 flows show
  auth0 flows show <flow-id>
  auth0 flows show <flow-id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := flowID.Pick(cmd, &inputs.ID, cli.flowPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			flow, err := cli.flowRawGet(cmd.Context(), inputs.ID)
			if err != nil {
				return fmt.Errorf("failed to read flow with ID %q: %w", inputs.ID, err)
			}

			return cli.renderer.FlowShowRaw(flow)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func createFlowCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Name    string
		File    string
		Edit    bool
		Example bool
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new flow",
		Long: "Create a new flow.\n\n" +
			"Interactive behavior: `auth0 flows create` asks only for the name and creates a minimal " +
			"scaffold; it does not open an editor. Pass `--edit` to open an editor and author the flow " +
			"actions before it is created, or supply the whole body via `--file` (or piped stdin) with " +
			"an optional `--name` override. Run `auth0 flows create --example > flow.json` to generate " +
			"an accepted file payload.",
		Example: `  auth0 flows create
  auth0 flows create --name "My Flow"
  auth0 flows create --name "My Flow" --edit
  auth0 flows create --example > flow.json
  auth0 flows create --file ./flow.json
  cat flow.json | auth0 flows create -f -`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Example {
				cli.renderer.FlowExport(flowCreateExample)
				return nil
			}

			body, err := readBodyInput(inputs.File, "flow")
			if err != nil {
				return err
			}

			rawBody := json.RawMessage(body)
			if body == nil {
				if err := flowName.Ask(cmd, &inputs.Name, nil); err != nil {
					return err
				}
				if inputs.Name == "" {
					return errors.New("a flow name is required; supply --name, provide --file, or pipe JSON via stdin")
				}
				if inputs.Edit {
					if !canPrompt(cmd) {
						return errors.New("the --edit flag requires an interactive terminal")
					}
					if err := editJSONBody(cli, "flow", flowCreateSkeleton, &rawBody); err != nil {
						return err
					}
				} else {
					rawBody = json.RawMessage(flowCreateSkeleton)
				}
			}

			rawBody, err = applyRawNameOverride(rawBody, inputs.Name)
			if err != nil {
				return fmt.Errorf("failed to parse flow body: %w", err)
			}

			name, err := rawJSONStringField(rawBody, "name")
			if err != nil {
				return fmt.Errorf("failed to parse flow body: %w", err)
			}
			if name == "" {
				return errors.New("a flow name is required; set it in the body or with --name")
			}

			created, err := cli.flowRawCreate(cmd.Context(), rawBody)
			if err != nil {
				return fmt.Errorf("failed to create flow: %w", err)
			}
			return cli.renderer.FlowCreateRaw(created)
		},
	}

	flowName.RegisterString(cmd, &inputs.Name, "")
	flowFile.RegisterString(cmd, &inputs.File, "")
	flowEdit.RegisterBool(cmd, &inputs.Edit, false)
	flowExample.RegisterBool(cmd, &inputs.Example, false)
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func updateFlowCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID   string
		Name string
		File string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update a flow",
		Long: "Update a flow.\n\n" +
			"Passing `--file` (or piped stdin) replaces every top-level field present in the file. " +
			"Passing only `--name` performs a merge that preserves the flow's actions. Server-managed " +
			"fields such as `id`, `created_at`, and `updated_at` are removed before the request is sent.",
		Example: `  auth0 flows update <flow-id> --name "New Name"
  auth0 flows update <flow-id> --file ./flow.json
  cat flow.json | auth0 flows update <flow-id> -f -`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := flowID.Pick(cmd, &inputs.ID, cli.flowPickerOptions); err != nil {
					return err
				}
			}

			body, err := readBodyInput(inputs.File, "flow")
			if err != nil {
				return err
			}

			var rawBody json.RawMessage

			switch {
			case body != nil:
				rawBody, err = applyRawNameOverride(body, inputs.Name)
				if err != nil {
					return fmt.Errorf("failed to parse flow body: %w", err)
				}
			case inputs.Name != "":
				rawBody, err = applyRawNameOverride(json.RawMessage(`{}`), inputs.Name)
				if err != nil {
					return fmt.Errorf("failed to build flow update: %w", err)
				}
			case canPrompt(cmd):
				current, err := cli.flowRawGet(cmd.Context(), inputs.ID)
				if err != nil {
					return fmt.Errorf("failed to read flow with ID %q: %w", inputs.ID, err)
				}

				var seed bytes.Buffer
				if err := json.Indent(&seed, current, "", "  "); err != nil {
					return fmt.Errorf("failed to parse flow with ID %q: %w", inputs.ID, err)
				}

				if err := editJSONBody(cli, "flow", seed.String(), &rawBody); err != nil {
					return err
				}
			default:
				return errors.New("nothing to update; supply --file, pipe JSON via stdin, or the --name flag")
			}

			updated, err := cli.flowRawUpdate(cmd.Context(), inputs.ID, rawBody)
			if err != nil {
				return fmt.Errorf("failed to update flow with ID %q: %w", inputs.ID, err)
			}
			return cli.renderer.FlowUpdateRaw(updated)
		},
	}

	flowName.RegisterStringU(cmd, &inputs.Name, "")
	flowFile.RegisterStringU(cmd, &inputs.File, "")
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func deleteFlowCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Args:    cobra.ArbitraryArgs,
		Short:   "Delete a flow",
		Long: "Delete a flow.\n\n" +
			"To delete interactively, use `auth0 flows delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the flow id and the `--force` flag to skip confirmation.",
		Example: `  auth0 flows delete
  auth0 flows rm
  auth0 flows delete <flow-id>
  auth0 flows delete <flow-id> --force
  auth0 flows delete <flow-id> <flow-id2> <flow-idn>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var ids []string
			if len(args) == 0 {
				if err := flowID.PickMany(cmd, &ids, cli.flowPickerOptions); err != nil {
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

			return ansi.ProgressBar("Deleting flow(s)", ids, func(_ int, id string) error {
				if id == "" {
					return nil
				}
				if err := cli.apiv3.Flow.Delete(cmd.Context(), id); err != nil {
					return fmt.Errorf("failed to delete flow with ID %q: %w", id, err)
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

// --- Executions ---.

func flowExecutionsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "executions",
		Short: "Manage Flow executions",
		Long:  "Inspect the runtime executions produced when a flow runs.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listFlowExecutionsCmd(cli))
	cmd.AddCommand(showFlowExecutionCmd(cli))
	cmd.AddCommand(deleteFlowExecutionCmd(cli))

	return cmd
}

func listFlowExecutionsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		FlowID string
		Number int
		From   string
		Take   int
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "List a flow's executions",
		Long:    "List the executions produced by a flow.",
		Example: `  auth0 flows executions list <flow-id>
  auth0 flows executions ls <flow-id> --number 100
  auth0 flows executions list <flow-id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.FlowID = args[0]
			} else {
				if err := flowID.Pick(cmd, &inputs.FlowID, cli.flowPickerOptions); err != nil {
					return err
				}
			}

			params := &managementv3.ListFlowExecutionsRequestParameters{}
			if inputs.From != "" {
				params.From = &inputs.From
			}
			if cmd.Flags().Changed("take") {
				params.Take = &inputs.Take
			}

			var executions []*managementv3.FlowExecutionSummary
			if err := ansi.Waiting(func() (err error) {
				executions, err = collectFlowExecutions(cmd.Context(), cli, inputs.FlowID, params, inputs.Number)
				return err
			}); err != nil {
				return fmt.Errorf("failed to list flow executions: %w", err)
			}

			return cli.renderer.FlowExecutionsList(executions)
		},
	}

	cmd.Flags().IntVarP(&inputs.Number, "number", "n", 100, "Number of executions to retrieve. Fetched across pages.")
	cmd.Flags().StringVar(&inputs.From, "from", "", "Cursor id from which to start selection.")
	cmd.Flags().IntVar(&inputs.Take, "take", 0, "Number of executions to retrieve per page.")
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	return cmd
}

func showFlowExecutionCmd(cli *cli) *cobra.Command {
	var inputs struct {
		FlowID      string
		ExecutionID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(2),
		Short: "Show a flow execution",
		Long:  "Display information about a flow execution.",
		Example: `  auth0 flows executions show <flow-id> <execution-id>
  auth0 flows executions show <flow-id> <execution-id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.FlowID = args[0]
			} else {
				if err := flowID.Pick(cmd, &inputs.FlowID, cli.flowPickerOptions); err != nil {
					return err
				}
			}

			if len(args) > 1 {
				inputs.ExecutionID = args[1]
			} else {
				if err := flowExecutionID.Pick(cmd, &inputs.ExecutionID, cli.flowExecutionPickerOptions(inputs.FlowID)); err != nil {
					return err
				}
			}

			execution, err := cli.flowExecutionRawGet(cmd.Context(), inputs.FlowID, inputs.ExecutionID)
			if err != nil {
				return fmt.Errorf("failed to read flow execution with ID %q: %w", inputs.ExecutionID, err)
			}

			return cli.renderer.FlowExecutionShowRaw(execution)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func deleteFlowExecutionCmd(cli *cli) *cobra.Command {
	var inputs struct {
		FlowID string
	}

	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Args:    cobra.ArbitraryArgs,
		Short:   "Delete a flow execution",
		Long: "Delete one or more executions of a flow.\n\n" +
			"Supply the flow id followed by the execution ids. Use `--force` to skip confirmation.",
		Example: `  auth0 flows executions delete <flow-id> <execution-id>
  auth0 flows executions rm <flow-id> <execution-id> --force
  auth0 flows executions delete <flow-id> <execution-id> <execution-id2>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.FlowID = args[0]
			} else {
				if err := flowID.Pick(cmd, &inputs.FlowID, cli.flowPickerOptions); err != nil {
					return err
				}
			}

			var ids []string
			if len(args) > 1 {
				ids = args[1:]
			} else {
				if err := flowExecutionID.PickMany(cmd, &ids, cli.flowExecutionPickerOptions(inputs.FlowID)); err != nil {
					return err
				}
			}

			if !cli.force && cli.agentMode {
				return errDestructiveNoConfirm
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			return ansi.ProgressBar("Deleting flow execution(s)", ids, func(_ int, id string) error {
				if id == "" {
					return nil
				}
				if err := cli.apiv3.FlowExecution.Delete(cmd.Context(), inputs.FlowID, id); err != nil {
					return fmt.Errorf("failed to delete flow execution with ID %q: %w", id, err)
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

// --- Vault connections ---.

func flowVaultCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vault",
		Short: "Manage Flow vault connections",
		Long:  "Manage the vault connections that store credentials for flow integrations.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(flowVaultConnectionsCmd(cli))
	cmd.AddCommand(openVaultAppCmd(cli))

	return cmd
}

func flowVaultConnectionsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connections",
		Short: "Manage Flow vault connections",
		Long:  "List, inspect, create, update, and delete flow vault connections.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listVaultConnectionsCmd(cli))
	cmd.AddCommand(showVaultConnectionCmd(cli))
	cmd.AddCommand(createVaultConnectionCmd(cli))
	cmd.AddCommand(updateVaultConnectionCmd(cli))
	cmd.AddCommand(deleteVaultConnectionCmd(cli))

	return cmd
}

func listVaultConnectionsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Number int
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your vault connections",
		Long:    "List your existing vault connections. To create one, run: `auth0 flows vault connections create`.",
		Example: `  auth0 flows vault connections list
  auth0 flows vault connections ls --number 100
  auth0 flows vault connections ls --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			params := &managementv3.ListFlowsVaultConnectionsRequestParameters{}

			var connections []*managementv3.FlowsVaultConnectionSummary
			if err := ansi.Waiting(func() (err error) {
				connections, err = collectVaultConnections(cmd.Context(), cli, params, inputs.Number)
				return err
			}); err != nil {
				return fmt.Errorf("failed to list vault connections: %w", err)
			}

			return cli.renderer.FlowVaultConnectionsList(connections)
		},
	}

	cmd.Flags().IntVarP(&inputs.Number, "number", "n", 100, "Number of connections to retrieve. Fetched across pages.")
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	return cmd
}

func showVaultConnectionCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show a vault connection",
		Long:  "Display information about a vault connection. Secret values are never returned by the API.",
		Example: `  auth0 flows vault connections show
  auth0 flows vault connections show <connection-id>
  auth0 flows vault connections show <connection-id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := vaultConnectionID.Pick(cmd, &inputs.ID, cli.vaultConnectionPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			connection, err := cli.vaultConnectionRawGet(cmd.Context(), inputs.ID)
			if err != nil {
				return fmt.Errorf("failed to read vault connection with ID %q: %w", inputs.ID, err)
			}

			return cli.renderer.FlowVaultConnectionShowRaw("vault connection", connection)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func createVaultConnectionCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Name    string
		AppID   string
		File    string
		Example bool
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new vault connection",
		Long: "Create a new vault connection.\n\n" +
			"Interactive behavior: `auth0 flows vault connections create` asks for the name and app id, " +
			"then opens an editor seeded with a provider-specific `setup` template so you can enter the " +
			"connection secrets. Alternatively, supply the whole body (including its `setup` secrets) via " +
			"`--file` (or piped stdin); `--name` and `--app-id` override the corresponding fields after the " +
			"file is parsed. Run `auth0 flows vault connections create --example` to print a template.",
		Example: `  auth0 flows vault connections create
  auth0 flows vault connections create --file ./connection.json
  auth0 flows vault connections create --file ./connection.json --name "My Connection"
  auth0 flows vault connections create --example > connection.json
  cat connection.json | auth0 flows vault connections create -f -`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Example {
				cli.renderer.FlowExport(vaultConnectionExample)
				return nil
			}

			body, err := readBodyInput(inputs.File, "vault connection")
			if err != nil {
				return err
			}

			var rawBody json.RawMessage
			if body != nil {
				rawBody, err = applyRawVaultConnectionOverrides(body, inputs.Name, inputs.AppID)
				if err != nil {
					return fmt.Errorf("failed to parse vault connection body: %w", err)
				}
			} else {
				if !canPrompt(cmd) {
					return errors.New("no vault connection body provided; supply --file or pipe JSON via stdin")
				}
				if err := vaultConnectionName.Ask(cmd, &inputs.Name, nil); err != nil {
					return err
				}
				if err := vaultConnectionAppID.Ask(cmd, &inputs.AppID, nil); err != nil {
					return err
				}
				if inputs.Name == "" || inputs.AppID == "" {
					return errors.New("a vault connection name and app id are required")
				}
				if err := editJSONBody(cli, "vault connection", vaultConnectionCreateSkeleton, &rawBody); err != nil {
					return err
				}
				rawBody, err = applyRawVaultConnectionOverrides(rawBody, inputs.Name, inputs.AppID)
				if err != nil {
					return fmt.Errorf("failed to parse vault connection body: %w", err)
				}
			}

			created, err := cli.vaultConnectionRawCreate(cmd.Context(), rawBody)
			if err != nil {
				return fmt.Errorf("failed to create vault connection: %w", err)
			}

			return cli.renderer.FlowVaultConnectionShowRaw("vault connection created", created)
		},
	}

	vaultConnectionName.RegisterString(cmd, &inputs.Name, "")
	vaultConnectionAppID.RegisterString(cmd, &inputs.AppID, "")
	vaultConnectionFile.RegisterString(cmd, &inputs.File, "")
	flowExample.RegisterBool(cmd, &inputs.Example, false)
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func updateVaultConnectionCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID   string
		Name string
		File string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update a vault connection",
		Long: "Update a vault connection.\n\n" +
			"Passing `--file` (or piped stdin) replaces every top-level field present in the file. " +
			"Passing only `--name` performs a merge. Server-managed fields such as `id`, `ready`, and " +
			"`fingerprint` are removed before the request is sent.",
		Example: `  auth0 flows vault connections update <connection-id> --name "New Name"
  auth0 flows vault connections update <connection-id> --file ./connection.json
  cat connection.json | auth0 flows vault connections update <connection-id> -f -`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := vaultConnectionID.Pick(cmd, &inputs.ID, cli.vaultConnectionPickerOptions); err != nil {
					return err
				}
			}

			body, err := readBodyInput(inputs.File, "vault connection")
			if err != nil {
				return err
			}

			var rawBody json.RawMessage
			switch {
			case body != nil:
				rawBody, err = applyRawNameOverride(body, inputs.Name)
				if err != nil {
					return fmt.Errorf("failed to parse vault connection body: %w", err)
				}
			case inputs.Name != "":
				rawBody, err = applyRawNameOverride(json.RawMessage(`{}`), inputs.Name)
				if err != nil {
					return fmt.Errorf("failed to build vault connection update: %w", err)
				}
			default:
				return errors.New("nothing to update; supply --file, pipe JSON via stdin, or the --name flag")
			}

			updated, err := cli.vaultConnectionRawUpdate(cmd.Context(), inputs.ID, rawBody)
			if err != nil {
				return fmt.Errorf("failed to update vault connection with ID %q: %w", inputs.ID, err)
			}

			return cli.renderer.FlowVaultConnectionShowRaw("vault connection updated", updated)
		},
	}

	vaultConnectionName.RegisterStringU(cmd, &inputs.Name, "")
	vaultConnectionFile.RegisterStringU(cmd, &inputs.File, "")
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func deleteVaultConnectionCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Args:    cobra.ArbitraryArgs,
		Short:   "Delete a vault connection",
		Long: "Delete a vault connection.\n\n" +
			"To delete interactively, use `auth0 flows vault connections delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the connection id and the `--force` flag.",
		Example: `  auth0 flows vault connections delete
  auth0 flows vault connections rm
  auth0 flows vault connections delete <connection-id>
  auth0 flows vault connections delete <connection-id> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var ids []string
			if len(args) == 0 {
				if err := vaultConnectionID.PickMany(cmd, &ids, cli.vaultConnectionPickerOptions); err != nil {
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

			return ansi.ProgressBar("Deleting vault connection(s)", ids, func(_ int, id string) error {
				if id == "" {
					return nil
				}
				if err := cli.apiv3.FlowVaultConnection.Delete(cmd.Context(), id); err != nil {
					return fmt.Errorf("failed to delete vault connection with ID %q: %w", id, err)
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

// --- Open in Dashboard ---.

func openFlowCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "open",
		Args:  cobra.MaximumNArgs(1),
		Short: "Open a flow in the Auth0 Dashboard",
		Long:  "Open a flow's page in the Auth0 Dashboard flow builder.",
		Example: `  auth0 flows open
  auth0 flows open <flow-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := flowID.Pick(cmd, &inputs.ID, cli.flowPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			openBuilderURL(cli, fmt.Sprintf("flows/%s/edit", inputs.ID))

			return nil
		},
	}

	return cmd
}

func openVaultAppCmd(cli *cli) *cobra.Command {
	var inputs struct {
		AppID string
	}

	cmd := &cobra.Command{
		Use:   "open",
		Args:  cobra.MaximumNArgs(1),
		Short: "Open the Vault in the Auth0 Dashboard",
		Long: "Open a Vault app's page in the Auth0 Dashboard. This opens the app's Vault page " +
			"(for example AUTH0, JWT, HTTP, or SLACK), not a specific connection.",
		Example: `  auth0 flows vault open
  auth0 flows vault open HTTP
  auth0 flows vault open SLACK`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := vaultAppID.Pick(cmd, &inputs.AppID, cli.vaultAppPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.AppID = args[0]
			}

			openBuilderURL(cli, fmt.Sprintf("vault/apps/%s/edit", inputs.AppID))

			return nil
		},
	}

	return cmd
}

// vaultAppPickerOptions offers the distinct app ids among existing vault
// connections, so `auth0 flows vault open` can be run without arguments.
func (c *cli) vaultAppPickerOptions(ctx context.Context) (pickerOptions, error) {
	connections, err := collectVaultConnections(ctx, c, &managementv3.ListFlowsVaultConnectionsRequestParameters{}, 0)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var opts pickerOptions
	for _, conn := range connections {
		appID := conn.GetAppID()
		if appID == "" || seen[appID] {
			continue
		}
		seen[appID] = true
		opts = append(opts, pickerOption{value: appID, label: appID})
	}

	if len(opts) == 0 {
		return nil, errors.New("there are no vault apps to choose from; supply an app id, e.g. `auth0 flows vault open HTTP`")
	}

	return opts, nil
}

// --- Raw HTTP helpers ---.

// flowRawGet fetches a flow through the v1 client's HTTP layer without using the
// v3 SDK's lossy flow-action unions.
func (c *cli) flowRawGet(ctx context.Context, id string) (json.RawMessage, error) {
	return c.rawJSONRequest(ctx, http.MethodGet, c.api.HTTPClient.URI("flows", id), nil)
}

func (c *cli) flowRawCreate(ctx context.Context, body json.RawMessage) (json.RawMessage, error) {
	return c.rawJSONRequest(ctx, http.MethodPost, c.api.HTTPClient.URI("flows"), body)
}

func (c *cli) flowRawUpdate(ctx context.Context, id string, body json.RawMessage) (json.RawMessage, error) {
	cleanBody, err := stripRawFields(body, flowServerManagedFields)
	if err != nil {
		return nil, err
	}
	return c.rawJSONRequest(ctx, http.MethodPatch, c.api.HTTPClient.URI("flows", id), cleanBody)
}

func (c *cli) flowExecutionRawGet(ctx context.Context, flowID, executionID string) (json.RawMessage, error) {
	return c.rawJSONRequest(ctx, http.MethodGet, c.api.HTTPClient.URI("flows", flowID, "executions", executionID), nil)
}

func (c *cli) vaultConnectionRawGet(ctx context.Context, id string) (json.RawMessage, error) {
	return c.rawJSONRequest(ctx, http.MethodGet, c.api.HTTPClient.URI("flows", "vault", "connections", id), nil)
}

func (c *cli) vaultConnectionRawCreate(ctx context.Context, body json.RawMessage) (json.RawMessage, error) {
	return c.rawJSONRequest(ctx, http.MethodPost, c.api.HTTPClient.URI("flows", "vault", "connections"), body)
}

func (c *cli) vaultConnectionRawUpdate(ctx context.Context, id string, body json.RawMessage) (json.RawMessage, error) {
	cleanBody, err := stripRawFields(body, vaultConnectionServerManagedFields)
	if err != nil {
		return nil, err
	}
	return c.rawJSONRequest(ctx, http.MethodPatch, c.api.HTTPClient.URI("flows", "vault", "connections", id), cleanBody)
}

// applyRawVaultConnectionOverrides overlays the --name and --app-id scalar flags
// on a vault connection body.
func applyRawVaultConnectionOverrides(body json.RawMessage, name, appID string) (json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, errors.New("vault connection body must be a JSON object")
	}

	if name != "" {
		encoded, err := json.Marshal(name)
		if err != nil {
			return nil, err
		}
		obj["name"] = encoded
	}
	if appID != "" {
		encoded, err := json.Marshal(appID)
		if err != nil {
			return nil, err
		}
		obj["app_id"] = encoded
	}

	return json.Marshal(obj)
}

// --- Paging + pickers ---.

func collectFlows(ctx context.Context, cli *cli, params *managementv3.ListFlowsRequestParameters, limit int) ([]*managementv3.FlowSummary, error) {
	page, err := cli.apiv3.Flow.List(ctx, params)
	if err != nil {
		return nil, err
	}

	var out []*managementv3.FlowSummary
	for page != nil {
		for _, f := range page.Results {
			out = append(out, f)
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

func collectFlowExecutions(ctx context.Context, cli *cli, flowID string, params *managementv3.ListFlowExecutionsRequestParameters, limit int) ([]*managementv3.FlowExecutionSummary, error) {
	page, err := cli.apiv3.FlowExecution.List(ctx, flowID, params)
	if err != nil {
		return nil, err
	}

	var out []*managementv3.FlowExecutionSummary
	for page != nil {
		for _, e := range page.Results {
			out = append(out, e)
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

func collectVaultConnections(ctx context.Context, cli *cli, params *managementv3.ListFlowsVaultConnectionsRequestParameters, limit int) ([]*managementv3.FlowsVaultConnectionSummary, error) {
	page, err := cli.apiv3.FlowVaultConnection.List(ctx, params)
	if err != nil {
		return nil, err
	}

	var out []*managementv3.FlowsVaultConnectionSummary
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

func (c *cli) flowPickerOptions(ctx context.Context) (pickerOptions, error) {
	flows, err := collectFlows(ctx, c, &managementv3.ListFlowsRequestParameters{}, 0)
	if err != nil {
		return nil, err
	}

	var opts pickerOptions
	for _, f := range flows {
		label := fmt.Sprintf("%s %s", f.GetName(), ansi.Faint("("+f.GetID()+")"))
		opts = append(opts, pickerOption{value: f.GetID(), label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("there are currently no flows to choose from. Create one by running: `auth0 flows create`")
	}

	return opts, nil
}

// flowExecutionPickerOptions returns a picker over the executions of a specific
// flow, so it must be bound to the flow id before use.
func (c *cli) flowExecutionPickerOptions(flowID string) pickerOptionsFunc {
	return func(ctx context.Context) (pickerOptions, error) {
		executions, err := collectFlowExecutions(ctx, c, flowID, &managementv3.ListFlowExecutionsRequestParameters{}, 0)
		if err != nil {
			return nil, err
		}

		var opts pickerOptions
		for _, e := range executions {
			label := fmt.Sprintf("%s %s", e.GetStatus(), ansi.Faint("("+e.GetID()+")"))
			opts = append(opts, pickerOption{value: e.GetID(), label: label})
		}

		if len(opts) == 0 {
			return nil, errors.New("this flow has no executions to choose from")
		}

		return opts, nil
	}
}
