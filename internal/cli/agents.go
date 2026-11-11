package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	management1 "github.com/auth0/go-auth0/management"
	"github.com/auth0/go-auth0/v3/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/prompt"
)

var (
	agentID = Argument{
		Name: "Agent ID",
		Help: "ID of the agent.",
	}

	agentName = Flag{
		Name:       "Name",
		LongForm:   "name",
		ShortForm:  "n",
		Help:       "Name of the agent (1-255 characters).",
		IsRequired: true,
	}

	agentClientID = Flag{
		Name:         "Client ID",
		LongForm:     "client-id",
		ShortForm:    "c",
		Help:         "ID of an existing machine to machine application to link to the agent on creation.",
		AlwaysPrompt: true,
	}

	agentExternalID = Flag{
		Name:         "External ID",
		LongForm:     "external-id",
		ShortForm:    "e",
		Help:         "Your own identifier for the agent. It cannot be changed after creation.",
		AlwaysPrompt: true,
	}

	agentMetadata = Flag{
		Name:      "Metadata",
		LongForm:  "metadata",
		ShortForm: "m",
		Help:      "Metadata for the agent as a JSON object (max 10KB), e.g. '{\"team\":\"ai\"}', or a file (@metadata.json). On update, null or {} clears it.",
	}

	agentNumber = Flag{
		Name:      "Number",
		LongForm:  "number",
		ShortForm: "n",
		Help:      "Number of agents to retrieve. Minimum 1, maximum 1000.",
	}
)

func agentsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agents",
		Short: "Manage agents (Early Access)",
		Long: "Manage agents (Early Access): first-class agent principals in your tenant. " +
			"Machine to machine applications can be linked to an agent with `auth0 apps update <app-id> --agent-id <agent-id>`.\n\n" +
			"These commands need the read:agents, create:agents, update:agents and delete:agents scopes, " +
			"which are available through a machine to machine (M2M) login only. " +
			"Authorize an M2M application for the Auth0 Management API with these scopes, then log in with it.\n\n" +
			"To manage skills for your AI coding assistants, use `auth0 agent` instead.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listAgentsCmd(cli))
	cmd.AddCommand(createAgentCmd(cli))
	cmd.AddCommand(showAgentCmd(cli))
	cmd.AddCommand(updateAgentCmd(cli))
	cmd.AddCommand(deleteAgentCmd(cli))

	return cmd
}

func listAgentsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Number int
		Schema bool
		Query  string
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your agents",
		Long:    "List your agents. To create one, run: `auth0 agents create`." + experimentationQueryHelp,
		Example: `  auth0 agents list
  auth0 agents ls
  auth0 agents list --number 10
  auth0 agents ls -n 10 --json
  auth0 agents list --csv
  auth0 agents list --schema
  auth0 agents list --query '{"take":10}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Schema {
				return printOperationSchema(cli, "GET", "/agents")
			}

			if inputs.Query != "" {
				return runJSONQuery(cli, cmd, jsonQuerySpec{
					Path:      "agents",
					SchemaCmd: "auth0 agents list",
				}, inputs.Query)
			}

			if inputs.Number < 1 || inputs.Number > 1000 {
				return validationError{err: fmt.Errorf("number flag invalid, please pass a number between 1 and 1000")}
			}

			// The API caps take at 100 per page.
			req := &management.ListAgentsRequestParameters{Take: auth0.Int(min(inputs.Number, 100))}

			agents, err := collectV3Pages(cmd.Context(), inputs.Number,
				func(ctx context.Context) (*auth0.AgentListPage, error) {
					return cli.apiv3.Agents.List(ctx, req)
				})
			if err != nil {
				return fmt.Errorf("failed to list agents: %w", err)
			}

			cli.renderer.AgentList(agents)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")
	agentNumber.RegisterInt(cmd, &inputs.Number, defaultPageSize)
	schemaFlag.RegisterBool(cmd, &inputs.Schema, false)
	listQueryFlag.RegisterString(cmd, &inputs.Query, "")

	return cmd
}

func showAgentCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show an agent",
		Long:  "Display details about an agent.",
		Example: `  auth0 agents show
  auth0 agents show <agent-id>
  auth0 agents show <agent-id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := agentID.Pick(cmd, &inputs.ID, cli.agentPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var agent *management.AgentResponseContent
			if err := ansi.Waiting(func() (err error) {
				agent, err = cli.apiv3.Agents.Read(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to get agent %q: %w", inputs.ID, err)
			}

			cli.renderer.AgentShow(agent)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func createAgentCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Name       string
		ClientID   string
		ExternalID string
		Metadata   string
		Data       string
		Schema     bool
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new agent",
		Long: "Create a new agent.\n\n" +
			"To create interactively, use `auth0 agents create` with no flags.\n\n" +
			"To create non-interactively, supply at least the name through the flags." + experimentationDataHelp,
		Example: `  auth0 agents create
  auth0 agents create --name "Support Bot"
  auth0 agents create -n "Support Bot" --client-id <client-id> --external-id bot-42 --metadata '{"team":"support"}' --json
  auth0 agents create -n "Support Bot" --metadata @metadata.json

  # Discover the payload schema
  auth0 agents create --schema

  # JSON input mode (for agents and automation)
  auth0 agents create --data '{"name":"Support Bot"}'
  auth0 agents create --data @agent.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Schema {
				return printOperationSchema(cli, "POST", "/agents")
			}

			payload, provided, err := ResolveData(cmd)
			if err != nil {
				return err
			}
			if provided {
				return createAgentFromJSON(cli, cmd, payload)
			}

			if err := agentName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}
			if inputs.Name == "" {
				return usageError{err: errors.New("--name is required"), reason: "missing_required_flags"}
			}

			if err := agentClientID.Pick(cmd, &inputs.ClientID, cli.agentClientPickerOptions); err != nil {
				return err
			}
			if err := agentExternalID.Ask(cmd, &inputs.ExternalID, nil); err != nil {
				return err
			}
			if !agentMetadata.IsSet(cmd) && canPrompt(cmd) && prompt.Confirm("Add metadata?") {
				if err := agentMetadata.OpenEditor(cmd, &inputs.Metadata, "{}", inputs.Name+".*.json", cli.agentMetadataEditorHint); err != nil {
					return fmt.Errorf("failed to capture input from the editor: %w", err)
				}
			}

			var metadata map[string]any
			if inputs.Metadata != "" {
				if metadata, err = parseAgentMetadata(inputs.Metadata); err != nil {
					return err
				}
			}

			req := &management.CreateAgentRequestContent{Name: inputs.Name}
			if inputs.ClientID != "" {
				req.ClientID = &inputs.ClientID
			}
			if inputs.ExternalID != "" {
				req.ExternalAgentID = &inputs.ExternalID
			}
			if metadata != nil {
				req.Metadata = &metadata
			}

			var result *management.AgentResponseContent
			if err := ansi.Waiting(func() (err error) {
				result, err = cli.apiv3.Agents.Create(cmd.Context(), req)
				return err
			}); err != nil {
				return fmt.Errorf("failed to create agent %q: %w", inputs.Name, err)
			}

			return cli.renderer.AgentCreate(result)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	agentName.RegisterString(cmd, &inputs.Name, "")
	agentClientID.RegisterString(cmd, &inputs.ClientID, "")
	agentExternalID.RegisterString(cmd, &inputs.ExternalID, "")
	agentMetadata.RegisterString(cmd, &inputs.Metadata, "")
	dataFlag.RegisterString(cmd, &inputs.Data, "")
	schemaFlag.RegisterBool(cmd, &inputs.Schema, false)
	markDataExclusive(cmd)

	return cmd
}

func updateAgentCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID       string
		Name     string
		Metadata string
		Data     string
		Schema   bool
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update an agent",
		Long: "Update an agent.\n\n" +
			"To update interactively, use `auth0 agents update` with no arguments.\n\n" +
			"To update non-interactively, supply the agent ID and the fields to change through the flags. " +
			"Pass `--metadata null` to clear all metadata. The external ID cannot be changed." + experimentationDataHelp,
		Example: `  auth0 agents update
  auth0 agents update <agent-id> --name "New Name"
  auth0 agents update <agent-id> --metadata '{"team":"platform"}' --json
  auth0 agents update <agent-id> --metadata @metadata.json
  auth0 agents update <agent-id> --metadata null

  # Discover the payload schema
  auth0 agents update --schema

  # JSON input mode (for agents and automation)
  auth0 agents update <agent-id> --data '{"name":"New Name"}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Schema {
				return printOperationSchema(cli, "PATCH", "/agents/{id}")
			}

			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := agentID.Pick(cmd, &inputs.ID, cli.agentPickerOptions); err != nil {
					return err
				}
			}

			payload, provided, err := ResolveData(cmd)
			if err != nil {
				return err
			}
			if provided {
				return updateAgentFromJSON(cli, cmd, inputs.ID, payload)
			}

			var existing *management.AgentResponseContent
			if err := ansi.Waiting(func() (err error) {
				existing, err = cli.apiv3.Agents.Read(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to get agent %q: %w", inputs.ID, err)
			}

			currentName := existing.GetName()
			if !agentName.IsSet(cmd) {
				inputs.Name = currentName
			}
			if err := agentName.AskU(cmd, &inputs.Name, &currentName); err != nil {
				return err
			}

			currentMetadata := ""
			if len(existing.GetMetadata()) > 0 {
				b, err := json.MarshalIndent(existing.GetMetadata(), "", "  ")
				if err != nil {
					return fmt.Errorf("failed to encode metadata of agent %q: %w", inputs.ID, err)
				}
				currentMetadata = string(b)
			}
			interactive := canPrompt(cmd) && noLocalFlagSet(cmd)
			if !agentMetadata.IsSet(cmd) {
				inputs.Metadata = currentMetadata
				if interactive && prompt.Confirm("Change metadata?") {
					if err := agentMetadata.OpenEditor(cmd, &inputs.Metadata, currentMetadata, inputs.Name+".*.json", cli.agentMetadataUpdateEditorHint); err != nil {
						return fmt.Errorf("failed to capture input from the editor: %w", err)
					}
					// An empty file keeps the current metadata; null or {} clears it.
					if strings.TrimSpace(inputs.Metadata) == "" {
						inputs.Metadata = currentMetadata
					}
				}
			}

			req := &management.PatchAgentRequestParameters{}
			updated := false

			if inputs.Name != currentName {
				req.Name = &inputs.Name
				updated = true
			}
			if agentMetadata.IsSet(cmd) || strings.TrimSpace(inputs.Metadata) != strings.TrimSpace(currentMetadata) {
				metadata, err := parseAgentMetadata(inputs.Metadata)
				if err != nil {
					return err
				}
				// The setter marks the field explicit so {} and null are sent as-is.
				req.SetMetadata(metadata)
				updated = true
			}

			if !updated {
				if interactive {
					cli.renderer.Infof("No changes to apply.")
					return nil
				}
				return usageError{err: errors.New("nothing to update: provide at least one flag or --data"), reason: "missing_required_flags"}
			}

			var result *management.AgentResponseContent
			if err := ansi.Waiting(func() (err error) {
				result, err = cli.apiv3.Agents.Update(cmd.Context(), inputs.ID, req)
				return err
			}); err != nil {
				return fmt.Errorf("failed to update agent %q: %w", inputs.ID, err)
			}

			return cli.renderer.AgentUpdate(result)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	agentName.RegisterStringU(cmd, &inputs.Name, "")
	agentMetadata.RegisterStringU(cmd, &inputs.Metadata, "")
	dataFlag.RegisterString(cmd, &inputs.Data, "")
	schemaFlag.RegisterBool(cmd, &inputs.Schema, false)
	markDataExclusive(cmd)

	return cmd
}

func createAgentFromJSON(cli *cli, cmd *cobra.Command, dataStr string) error {
	result, err := runJSONWrite[management.AgentResponseContent](cli, cmd, jsonWriteSpec{
		Method:     http.MethodPost,
		SchemaPath: "/agents",
		URI:        cli.api.HTTPClient.URI("agents"),
		Data:       dataStr,
		SchemaCmd:  "auth0 agents create",
	})
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	return cli.renderer.AgentCreate(result)
}

func updateAgentFromJSON(cli *cli, cmd *cobra.Command, id, dataStr string) error {
	result, err := runJSONWrite[management.AgentResponseContent](cli, cmd, jsonWriteSpec{
		Method:     http.MethodPatch,
		SchemaPath: "/agents/{id}",
		URI:        cli.api.HTTPClient.URI("agents", id),
		Data:       dataStr,
		SchemaCmd:  "auth0 agents update",
	})
	if err != nil {
		return fmt.Errorf("failed to update agent %q: %w", id, err)
	}

	return cli.renderer.AgentUpdate(result)
}

func deleteAgentCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Short:   "Delete an agent",
		Long: "Delete an agent.\n\n" +
			"Linked applications are not deleted and keep the deleted agent's ID; run `auth0 apps update <app-id> --unlink-agent` to clear it.\n\n" +
			"To delete interactively, use `auth0 agents delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the agent ID and use `--force` to skip confirmation.",
		Example: `  auth0 agents delete
  auth0 agents rm
  auth0 agents delete <agent-id>
  auth0 agents delete <agent-id> --force
  auth0 agents delete <agent-id> <agent-id2> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var ids []string
			if len(args) == 0 {
				if err := agentID.PickMany(cmd, &ids, cli.agentPickerOptions); err != nil {
					return err
				}
			} else {
				ids = args
			}

			if !cli.force && cli.agentMode {
				return errDestructiveNoConfirm
			}

			if !cli.force && canPrompt(cmd) {
				cli.renderer.Warnf("Linked applications are not deleted and keep the deleted agent's ID; run `auth0 apps update <app-id> --unlink-agent` to clear it.")
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			return ansi.ProgressBar("Deleting agent(s)", ids, func(_ int, id string) error {
				if id != "" {
					// Delete returns 204 for an unknown ID, so check that it exists first.
					if _, err := cli.apiv3.Agents.Read(cmd.Context(), id); err != nil {
						return fmt.Errorf("failed to delete agent %q: %w", id, err)
					}

					if err := cli.apiv3.Agents.Delete(cmd.Context(), id); err != nil {
						return fmt.Errorf("failed to delete agent %q: %w", id, err)
					}
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

func (c *cli) agentPickerOptions(ctx context.Context) (pickerOptions, error) {
	all, err := collectV3Pages(ctx, 0,
		func(ctx context.Context) (*auth0.AgentListPage, error) {
			return c.apiv3.Agents.List(ctx, &management.ListAgentsRequestParameters{})
		})
	if err != nil {
		return nil, err
	}

	var opts pickerOptions
	for _, a := range all {
		label := fmt.Sprintf("%s %s", a.GetName(), ansi.Faint("("+a.GetAgentID()+")"))
		opts = append(opts, pickerOption{value: a.GetAgentID(), label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("no agents available. Create one by running: `auth0 agents create`")
	}

	return opts, nil
}

// agentClientPickerOptions lists M2M applications to link, led by a "None" option that links nothing.
func (c *cli) agentClientPickerOptions(ctx context.Context) (pickerOptions, error) {
	opts := pickerOptions{{value: "", label: "None (don't link an application)"}}

	apps, err := c.appPickerOptions(management1.Parameter("app_type", "non_interactive"))(ctx)
	if err != nil && !errors.Is(err, errNoApps) {
		return nil, err
	}

	return append(opts, apps...), nil
}

func (c *cli) agentMetadataEditorHint() {
	c.renderer.Infof("%s Enter the metadata as a JSON object, e.g. {\"team\":\"ai\"}. Once you close the editor, it will be saved. To cancel, press CTRL+C.", ansi.Faint("Hint:"))
}

func (c *cli) agentMetadataUpdateEditorHint() {
	c.renderer.Infof("%s Edit the metadata as a JSON object. Use null or {} to clear it; an empty file keeps the current value. Once you close the editor, it will be saved. To cancel, press CTRL+C.", ansi.Faint("Hint:"))
}

// parseAgentMetadata decodes --metadata (inline JSON or @file) into a JSON object; null yields a nil map.
func parseAgentMetadata(raw string) (map[string]any, error) {
	data, err := readJSONInput(raw)
	if err != nil {
		return nil, validationError{err: fmt.Errorf("failed to read --metadata: %w", err)}
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, validationError{err: fmt.Errorf("invalid JSON for --metadata (must be a JSON object, e.g. '{\"team\":\"ai\"}'): %w", err), reason: "malformed_json"}
	}
	return m, nil
}
