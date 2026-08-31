package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
)

var (
	vaultConnectionID = Argument{
		Name: "Id",
		Help: "Id of the Vault connection.",
	}

	vaultAppID = Argument{
		Name: "App Id",
		Help: "Identifier of the Vault app to open (e.g. AUTH0, JWT, HTTP, SLACK).",
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
		Name:      "Setup File",
		LongForm:  "setup-file",
		ShortForm: "f",
		Help:      "Path to a JSON file containing the vault connection setup credentials. Run with --setup-template --app-id <APP_ID> to see the expected setup schema for a given app.",
	}

	vaultConnectionSetupTemplate = Flag{
		Name:     "Setup Template",
		LongForm: "setup-template",
		Help:     "Print the setup credentials template for the given --app-id and exit.",
	}
)

// vaultConnectionCreateSkeleton is the fallback editor seed when the app_id is not recognized.
const vaultConnectionCreateSkeleton = `{
  "setup": {
    "type": "BEARER",
    "token": "REPLACE_WITH_YOUR_TOKEN"
  }
}
`

// vaultConnectionSkeletons maps each known Vault app_id to an editor seed that
// matches the setup type(s) the Management API accepts for that app.
var vaultConnectionSkeletons = map[string]string{
	"ACTIVECAMPAIGN": "{\n  \"setup\": {\n    \"type\": \"API_KEY\",\n    \"api_key\": \"REPLACE_WITH_API_KEY\",\n    \"base_url\": \"https://REPLACE_WITH_YOUR_INSTANCE.api-us1.com\"\n  }\n}\n",
	"AIRTABLE":       "{\n  \"setup\": {\n    \"type\": \"API_KEY\",\n    \"api_key\": \"REPLACE_WITH_API_KEY\"\n  }\n}\n",
	"AUTH0":          "{\n  \"setup\": {\n    \"type\": \"OAUTH_APP\",\n    \"client_id\": \"REPLACE_WITH_CLIENT_ID\",\n    \"client_secret\": \"REPLACE_WITH_CLIENT_SECRET\",\n    \"domain\": \"REPLACE_WITH_YOUR_DOMAIN.auth0.com\"\n  }\n}\n",
	"BIGQUERY":       "{\n  \"setup\": {\n    \"type\": \"OAUTH_JWT\",\n    \"project_id\": \"REPLACE_WITH_PROJECT_ID\",\n    \"private_key\": \"REPLACE_WITH_PRIVATE_KEY\",\n    \"client_email\": \"REPLACE@PROJECT.iam.gserviceaccount.com\"\n  }\n}\n",
	"CLEARBIT":       "{\n  \"setup\": {\n    \"type\": \"API_KEY\",\n    \"secret_key\": \"REPLACE_WITH_SECRET_KEY\"\n  }\n}\n",
	"DOCUSIGN":       "{\n  \"setup\": {\n    \"type\": \"OAUTH_CODE\",\n    \"code\": \"REPLACE_WITH_AUTHORIZATION_CODE\"\n  }\n}\n",
	"GOOGLE_SHEETS":  "{\n  \"setup\": {\n    \"type\": \"OAUTH_CODE\",\n    \"code\": \"REPLACE_WITH_AUTHORIZATION_CODE\"\n  }\n}\n",
	"HTTP":           "{\n  \"setup\": {\n    \"type\": \"BEARER\",\n    \"token\": \"REPLACE_WITH_YOUR_TOKEN\"\n  }\n}\n",
	"HUBSPOT":        "{\n  \"setup\": {\n    \"type\": \"API_KEY\",\n    \"api_key\": \"REPLACE_WITH_API_KEY\"\n  }\n}\n",
	"JWT":            "{\n  \"setup\": {\n    \"type\": \"JWT\",\n    \"algorithm\": \"RS256\"\n  }\n}\n",
	"MAILCHIMP":      "{\n  \"setup\": {\n    \"type\": \"API_KEY\",\n    \"secret_key\": \"REPLACE_WITH_SECRET_KEY\"\n  }\n}\n",
	"MAILJET":        "{\n  \"setup\": {\n    \"type\": \"API_KEY\",\n    \"api_key\": \"REPLACE_WITH_API_KEY\",\n    \"secret_key\": \"REPLACE_WITH_SECRET_KEY\"\n  }\n}\n",
	"PIPEDRIVE":      "{\n  \"setup\": {\n    \"type\": \"TOKEN\",\n    \"token\": \"REPLACE_WITH_YOUR_TOKEN\"\n  }\n}\n",
	"SALESFORCE":     "{\n  \"setup\": {\n    \"type\": \"OAUTH_CODE\",\n    \"code\": \"REPLACE_WITH_AUTHORIZATION_CODE\"\n  }\n}\n",
	"SENDGRID":       "{\n  \"setup\": {\n    \"type\": \"API_KEY\",\n    \"api_key\": \"REPLACE_WITH_API_KEY\"\n  }\n}\n",
	"SLACK":          "{\n  \"setup\": {\n    \"type\": \"WEBHOOK\",\n    \"url\": \"https://hooks.slack.com/services/REPLACE_WITH_YOUR_WEBHOOK_URL\"\n  }\n}\n",
	"STRIPE":         "{\n  \"setup\": {\n    \"type\": \"KEY_PAIR\",\n    \"private_key\": \"sk_REPLACE_WITH_PRIVATE_KEY\",\n    \"public_key\": \"pk_REPLACE_WITH_PUBLIC_KEY\"\n  }\n}\n",
	"TELEGRAM":       "{\n  \"setup\": {\n    \"type\": \"TOKEN\",\n    \"token\": \"REPLACE_WITH_BOT_TOKEN\"\n  }\n}\n",
	"TWILIO":         "{\n  \"setup\": {\n    \"type\": \"API_KEY\",\n    \"account_id\": \"REPLACE_WITH_ACCOUNT_ID\",\n    \"api_key\": \"REPLACE_WITH_API_KEY\"\n  }\n}\n",
	"WHATSAPP":       "{\n  \"setup\": {\n    \"type\": \"TOKEN\",\n    \"token\": \"REPLACE_WITH_YOUR_TOKEN\"\n  }\n}\n",
	"ZAPIER":         "{\n  \"setup\": {\n    \"type\": \"WEBHOOK\",\n    \"url\": \"https://hooks.zapier.com/hooks/catch/REPLACE_WITH_YOUR_WEBHOOK_PATH\"\n  }\n}\n",
}

func vaultConnectionSeedForApp(appID string) string {
	if s, ok := vaultConnectionSkeletons[strings.ToUpper(appID)]; ok {
		return s
	}
	return vaultConnectionCreateSkeleton
}

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
		Short: "Manage Flow vault connections.",
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
				if err := vaultConnectionID.Pick(cmd, &inputs.ID, cli.flowsVaultConnectionPickerOptions); err != nil {
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
		Name          string
		AppID         string
		File          string
		SetupTemplate bool
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new vault connection",
		Long: "Create a new vault connection.\n\n" +
			"Prompts for name and app id, then asks whether to add setup credentials. " +
			"Use `--setup-file` to supply credentials non-interactively. " +
			"Run `--setup-template --app-id <APP_ID>` to print the setup credentials template for a given app.",
		Example: `  auth0 flows vault connections create
  auth0 flows vault connections create --name "My Connection" --app-id SLACK
  auth0 flows vault connections create --name "My Connection" --app-id SLACK --setup-file ./setup.json
  auth0 flows vault connections create --setup-template --app-id SLACK > setup.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			inputs.AppID = strings.ToUpper(inputs.AppID)

			if inputs.SetupTemplate {
				if err := vaultConnectionAppID.Pick(cmd, &inputs.AppID, cli.vaultAppIDPickerOptions); err != nil {
					return err
				}
				if inputs.AppID == "" {
					return errors.New("an app id is required")
				}
				cli.renderer.Warnf("Setup schema for %s. Save to a file and pass with --setup-file.", inputs.AppID)
				cli.renderer.FlowExport(vaultConnectionSeedForApp(inputs.AppID))
				return nil
			}

			if err := vaultConnectionName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}
			if inputs.Name == "" {
				return errors.New("a name is required")
			}
			if err := vaultConnectionAppID.Pick(cmd, &inputs.AppID, cli.vaultAppIDPickerOptions); err != nil {
				return err
			}
			if inputs.AppID == "" {
				return errors.New("an app id is required")
			}

			var setupBody json.RawMessage
			if inputs.File != "" {
				fileBody, err := os.ReadFile(inputs.File)
				if err != nil {
					return fmt.Errorf("failed to read setup file %q: %w", inputs.File, err)
				}
				if err = json.Unmarshal(fileBody, &setupBody); err != nil {
					return fmt.Errorf("setup file is not valid JSON: %w\n\nRun 'auth0 flows vault connections create --setup-template --app-id <APP_ID>' to see the expected setup schema", err)
				}
			} else {
				var addSetup bool
				if err := prompt.AskBool("Do you want to add setup credentials now?", &addSetup, false); err != nil {
					return err
				}

				if addSetup {
					if err := editJSONBody(cli, "vault connection", vaultConnectionSeedForApp(inputs.AppID), &setupBody); err != nil {
						return err
					}
				} else {
					setupBody = json.RawMessage(`{}`)
				}
			}

			m := make(map[string]json.RawMessage)
			if err := json.Unmarshal(setupBody, &m); err != nil {
				return fmt.Errorf("failed to parse vault connection body: %w", err)
			}
			nameJSON, _ := json.Marshal(inputs.Name)
			appIDJSON, _ := json.Marshal(inputs.AppID)
			m["name"] = nameJSON
			m["app_id"] = appIDJSON
			rawBody, err := json.Marshal(m)
			if err != nil {
				return fmt.Errorf("failed to build vault connection body: %w", err)
			}

			created, err := cli.vaultConnectionRawCreate(cmd.Context(), rawBody)
			if err != nil {
				if strings.Contains(err.Error(), "oneOf") {
					cli.renderer.Warnf("The setup payload does not match the expected schema for %s.\nRun 'auth0 flows vault connections create --setup-template --app-id %s' to see the correct setup format.", inputs.AppID, inputs.AppID)
				}
				return fmt.Errorf("failed to create vault connection: %w", err)
			}

			return cli.renderer.FlowVaultConnectionShowRaw("vault connection created", created)
		},
	}

	vaultConnectionName.RegisterString(cmd, &inputs.Name, "")
	vaultConnectionAppID.RegisterString(cmd, &inputs.AppID, "")
	vaultConnectionFile.RegisterString(cmd, &inputs.File, "")
	vaultConnectionSetupTemplate.RegisterBool(cmd, &inputs.SetupTemplate, false)
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
			"Use `--setup-file` to replace setup credentials, or `--name` to rename. " +
			"Run `auth0 flows vault connections create --setup-template --app-id <APP_ID>` to see the setup schema.",
		Example: `  auth0 flows vault connections update <connection-id> --name "New Name"
  auth0 flows vault connections update <connection-id> --setup-file ./setup.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := vaultConnectionID.Pick(cmd, &inputs.ID, cli.flowsVaultConnectionPickerOptions); err != nil {
					return err
				}
			}

			current, err := cli.vaultConnectionRawGet(cmd.Context(), inputs.ID)
			if err != nil {
				return fmt.Errorf("failed to read vault connection with ID %q: %w", inputs.ID, err)
			}

			currentName, err := rawJSONStringField(current, "name")
			if err != nil {
				return fmt.Errorf("failed to read vault connection name: %w", err)
			}

			if err := vaultConnectionName.AskU(cmd, &inputs.Name, &currentName); err != nil {
				return err
			}
			if inputs.Name == "" {
				inputs.Name = currentName
			}

			var appID string
			var rawBody json.RawMessage
			if inputs.File != "" {
				fileBody, err := os.ReadFile(inputs.File)
				if err != nil {
					return fmt.Errorf("failed to read setup file %q: %w", inputs.File, err)
				}
				var setupBody json.RawMessage
				if err := json.Unmarshal(fileBody, &setupBody); err != nil {
					return fmt.Errorf("setup file is not valid JSON: %w", err)
				}
				m := make(map[string]json.RawMessage)
				if err := json.Unmarshal(setupBody, &m); err != nil {
					return fmt.Errorf("failed to parse setup body: %w", err)
				}
				nameJSON, _ := json.Marshal(inputs.Name)
				m["name"] = nameJSON
				rawBody, err = json.Marshal(m)
				if err != nil {
					return fmt.Errorf("failed to build update body: %w", err)
				}
			} else {
				var updateSetup bool
				if err := prompt.AskBool("Do you want to update the setup details?", &updateSetup, false); err != nil {
					return err
				}

				if updateSetup {
					appID, err = rawJSONStringField(current, "app_id")
					if err != nil {
						return fmt.Errorf("failed to read app_id from vault connection %q: %w", inputs.ID, err)
					}
					if err := editJSONBody(cli, "vault connection", vaultConnectionSeedForApp(appID), &rawBody); err != nil {
						return err
					}
				} else {
					rawBody = json.RawMessage(`{}`)
				}

				rawBody, err = applyRawNameOverride(rawBody, inputs.Name)
				if err != nil {
					return fmt.Errorf("failed to apply name override: %w", err)
				}
			}

			updated, err := cli.vaultConnectionRawUpdate(cmd.Context(), inputs.ID, rawBody)
			if err != nil {
				if strings.Contains(err.Error(), "oneOf") {
					hint := "<APP_ID>"
					if appID != "" {
						hint = appID
					}
					cli.renderer.Warnf("The setup payload does not match the expected schema for %s.\nRun 'auth0 flows vault connections create --setup-template --app-id %s' to see the correct setup format.", hint, hint)
				}
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
				if err := vaultConnectionID.PickMany(cmd, &ids, cli.flowsVaultConnectionPickerOptions); err != nil {
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

// knownVaultAppIDs is the static list of app IDs recognised by the Flows Vault,
// derived from the FlowsVaultConnectionAppID* enums in the v3 SDK.
var knownVaultAppIDs = []string{
	"ACTIVECAMPAIGN",
	"AIRTABLE",
	"AUTH0",
	"BIGQUERY",
	"CLEARBIT",
	"DOCUSIGN",
	"GOOGLE_SHEETS",
	"HTTP",
	"HUBSPOT",
	"JWT",
	"MAILCHIMP",
	"MAILJET",
	"PIPEDRIVE",
	"SALESFORCE",
	"SENDGRID",
	"SLACK",
	"STRIPE",
	"TELEGRAM",
	"TWILIO",
	"WHATSAPP",
	"ZAPIER",
}

// vaultAppIDPickerOptions returns the static list of Vault app IDs for vault connection creation.
func (c *cli) vaultAppIDPickerOptions(_ context.Context) (pickerOptions, error) {
	var opts pickerOptions
	for _, id := range knownVaultAppIDs {
		opts = append(opts, pickerOption{value: id, label: id})
	}
	return opts, nil
}

func (c *cli) flowsVaultConnectionPickerOptions(ctx context.Context) (pickerOptions, error) {
	connections, err := collectVaultConnections(ctx, c, &managementv3.ListFlowsVaultConnectionsRequestParameters{}, 0)
	if err != nil {
		return nil, err
	}

	var opts pickerOptions
	for _, conn := range connections {
		label := fmt.Sprintf("%s %s", conn.GetName(), ansi.Faint("("+conn.GetID()+")"))
		opts = append(opts, pickerOption{value: conn.GetID(), label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("there are no vault connections to choose from; create one with `auth0 flows vault connections create`")
	}

	return opts, nil
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

func (c *cli) vaultConnectionRawGet(ctx context.Context, id string) (json.RawMessage, error) {
	return c.rawJSONRequest(ctx, http.MethodGet, c.api.HTTPClient.URI("flows", "vault", "connections", id), nil)
}

func (c *cli) vaultConnectionRawCreate(ctx context.Context, body json.RawMessage) (json.RawMessage, error) {
	return c.rawJSONRequest(ctx, http.MethodPost, c.api.HTTPClient.URI("flows", "vault", "connections"), body)
}

func (c *cli) vaultConnectionRawUpdate(ctx context.Context, id string, body json.RawMessage) (json.RawMessage, error) {
	return c.rawJSONRequest(ctx, http.MethodPatch, c.api.HTTPClient.URI("flows", "vault", "connections", id), body)
}

// --- Paging + pickers ---.

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
