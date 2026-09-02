package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
)

// flowCreateSkeleton seeds the editor for interactive flow creation.
const flowCreateSkeleton = `{
  "actions": []
}
`

const flowCreateExample = `{
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

var (
	flowID = Argument{
		Name: "Id",
		Help: "Id of the Flow.",
	}

	flowExecutionID = Argument{
		Name: "Execution Id",
		Help: "Id of the Flow execution.",
	}

	flowName = Flag{
		Name:     "Name",
		LongForm: "name",
		Help:     "Name of the Flow.",
	}

	flowFile = Flag{
		Name:      "Actions File",
		LongForm:  "actions-file",
		ShortForm: "f",
		Help:      "Path to a JSON file containing the flow actions body. Run with --actions-template to see the expected format.",
	}

	flowActionsTemplate = Flag{
		Name:     "Actions Template",
		LongForm: "actions-template",
		Help:     "Print the actions template for --actions-file and exit.",
	}

	flowHydrate = Flag{
		Name:     "Hydrate",
		LongForm: "hydrate",
		Help:     "Hydrate the response with the number of forms referencing each flow.",
	}
)

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
		Name            string
		File            string
		ActionsTemplate bool
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new flow",
		Long: "Create a new flow.\n\n" +
			"A name is required, supplied with `--name` or the interactive prompt. The actions graph " +
			"is authored interactively or supplied with `--actions-file`; the file must contain only the " +
			"actions body, not a name. " +
			"Run `auth0 flows create --actions-template > flow.json` to generate an actions template.",
		Example: `  auth0 flows create
  auth0 flows create --name "My Flow"
  auth0 flows create --actions-template > flow.json
  auth0 flows create --name "My Flow" --actions-file ./flow.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.ActionsTemplate {
				cli.renderer.Warnf("Flow actions body. Save to a file and pass with --actions-file.")
				cli.renderer.FlowExport(flowCreateExample)
				return nil
			}

			if err := flowName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}
			if inputs.Name == "" {
				return errors.New("a flow name is required; supply --name")
			}

			var rawBody json.RawMessage
			if inputs.File != "" {
				fileBody, err := os.ReadFile(inputs.File)
				if err != nil {
					return fmt.Errorf("failed to read flow file %q: %w", inputs.File, err)
				}
				if err := json.Unmarshal(fileBody, &rawBody); err != nil {
					return fmt.Errorf("failed to parse flow body: %w", err)
				}
				if err := rejectRawNameField(rawBody, "actions file"); err != nil {
					return err
				}
			} else {
				var editActions bool
				if err := prompt.AskBool("Do you want to edit the actions now?", &editActions, false); err != nil {
					return err
				}
				if editActions {
					if err := editJSONBody(cli, "flow", flowCreateSkeleton, &rawBody); err != nil {
						return err
					}
					if err := rejectRawNameField(rawBody, "flow actions"); err != nil {
						return err
					}
				} else {
					rawBody = json.RawMessage(flowCreateSkeleton)
				}
			}

			rawBody, err := applyRawNameOverride(rawBody, inputs.Name)
			if err != nil {
				return fmt.Errorf("failed to parse flow body: %w", err)
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
	flowActionsTemplate.RegisterBool(cmd, &inputs.ActionsTemplate, false)
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
			"Passing `--actions-file` replaces the flow's actions graph; the file must contain only the " +
			"actions body, not a name. Passing `--name` renames the flow; omit it to keep the current name.",
		Example: `  auth0 flows update <flow-id> --name "New Name"
  auth0 flows update <flow-id> --actions-file ./flow.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := flowID.Pick(cmd, &inputs.ID, cli.flowPickerOptions); err != nil {
					return err
				}
			}

			current, err := cli.flowRawGet(cmd.Context(), inputs.ID)
			if err != nil {
				return fmt.Errorf("failed to read flow with ID %q: %w", inputs.ID, err)
			}

			currentName, err := rawJSONStringField(current, "name")
			if err != nil {
				return fmt.Errorf("failed to read flow name: %w", err)
			}

			if err := flowName.AskU(cmd, &inputs.Name, &currentName); err != nil {
				return err
			}
			if inputs.Name == "" {
				inputs.Name = currentName
			}

			var rawBody json.RawMessage
			if inputs.File != "" {
				fileBody, err := os.ReadFile(inputs.File)
				if err != nil {
					return fmt.Errorf("failed to read flow file %q: %w", inputs.File, err)
				}
				if err := json.Unmarshal(fileBody, &rawBody); err != nil {
					return fmt.Errorf("failed to parse flow body: %w", err)
				}
				if err := rejectRawNameField(rawBody, "actions file"); err != nil {
					return err
				}
			} else {
				var updateActions bool
				if err := prompt.AskBool("Do you want to update the actions?", &updateActions, false); err != nil {
					return err
				}
				if updateActions {
					var currentBody map[string]json.RawMessage
					if err := json.Unmarshal(current, &currentBody); err != nil {
						return fmt.Errorf("failed to parse flow with ID %q: %w", inputs.ID, err)
					}
					actions := currentBody["actions"]
					if actions == nil {
						actions = json.RawMessage(`[]`)
					}
					seedBytes, err := json.MarshalIndent(map[string]json.RawMessage{"actions": actions}, "", "  ")
					if err != nil {
						return fmt.Errorf("failed to build flow actions seed: %w", err)
					}
					if err := editJSONBody(cli, "flow", string(seedBytes), &rawBody); err != nil {
						return err
					}
					if err := rejectRawNameField(rawBody, "flow actions"); err != nil {
						return err
					}
				} else {
					rawBody = json.RawMessage(`{}`)
				}
			}

			rawBody, err = applyRawNameOverride(rawBody, inputs.Name)
			if err != nil {
				return fmt.Errorf("failed to parse flow body: %w", err)
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
	return c.rawJSONRequest(ctx, http.MethodPatch, c.api.HTTPClient.URI("flows", id), body)
}

func (c *cli) flowExecutionRawGet(ctx context.Context, flowID, executionID string) (json.RawMessage, error) {
	return c.rawJSONRequest(ctx, http.MethodGet, c.api.HTTPClient.URI("flows", flowID, "executions", executionID), nil)
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
