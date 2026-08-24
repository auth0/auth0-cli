package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/config"
	"github.com/auth0/auth0-cli/internal/iostream"
	"github.com/auth0/auth0-cli/internal/prompt"
)

// formCreateSkeleton seeds the editor for interactive form creation. The name is
// prompted separately, so the seed only carries the empty graph containers, which
// are all valid on their own.
const formCreateSkeleton = `{
  "start": {},
  "nodes": [],
  "ending": {}
}
`

const formCreateExample = `{
  "name": "Customer Profile Form",
  "languages": {
    "primary": "en",
    "default": "en"
  },
  "start": {
    "next_node": "step_profile",
    "coordinates": {
      "x": 0,
      "y": 0
    }
  },
  "nodes": [
    {
      "id": "step_profile",
      "type": "STEP",
      "coordinates": {
        "x": 300,
        "y": 0
      },
      "alias": "Collect profile",
      "config": {
        "components": [
          {
            "id": "full_name",
            "category": "FIELD",
            "type": "TEXT",
            "label": "Full name",
            "required": true,
            "sensitive": false,
            "config": {
              "multiline": false
            }
          },
          {
            "id": "continue_button",
            "category": "BLOCK",
            "type": "NEXT_BUTTON",
            "config": {
              "text": "Continue"
            }
          }
        ],
        "next_node": "$ending"
      }
    }
  ],
  "ending": {
    "resume_flow": true,
    "coordinates": {
      "x": 600,
      "y": 0
    }
  }
}
`

// formServerManagedFields cannot be sent in create or update request bodies.
var formServerManagedFields = []string{
	"id",
	"created_at",
	"updated_at",
	"embedded_at",
	"submitted_at",
	"flow_count",
	"links",
}

var (
	formID = Argument{
		Name: "Id",
		Help: "Id of the Form.",
	}

	formName = Flag{
		Name:     "Name",
		LongForm: "name",
		Help:     "Name of the Form.",
	}

	formFile = Flag{
		Name:      "File",
		LongForm:  "file",
		ShortForm: "f",
		Help:      "Path to a JSON file with the form body. Use '-' to read from stdin.",
	}

	formLanguagePrimary = Flag{
		Name:     "Language Primary",
		LongForm: "language-primary",
		Help:     "Primary language of the Form (e.g. en).",
	}

	formLanguageDefault = Flag{
		Name:     "Language Default",
		LongForm: "language-default",
		Help:     "Default language of the Form (e.g. en).",
	}

	formOutput = Flag{
		Name:      "Output",
		LongForm:  "output",
		ShortForm: "o",
		Help:      "Path to write the exported form. Writes to stdout when omitted.",
	}

	formImportID = Flag{
		Name:     "Id",
		LongForm: "id",
		Help:     "Id of an existing Form to replace. When omitted, a new form is created.",
	}

	formEdit = Flag{
		Name:     "Edit",
		LongForm: "edit",
		Help:     "Open an editor to author the form graph after entering the name.",
	}

	formExample = Flag{
		Name:     "Example",
		LongForm: "example",
		Help:     "Print an example form JSON body and exit.",
	}
)

func formsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forms",
		Short: "Manage Forms",
		Long: "Forms are customizable screens you can insert into a flow to collect input " +
			"from users during authentication and other journeys.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listFormsCmd(cli))
	cmd.AddCommand(showFormCmd(cli))
	cmd.AddCommand(createFormCmd(cli))
	cmd.AddCommand(updateFormCmd(cli))
	cmd.AddCommand(deleteFormCmd(cli))
	cmd.AddCommand(exportFormCmd(cli))
	cmd.AddCommand(importFormCmd(cli))
	cmd.AddCommand(openFormCmd(cli))

	return cmd
}

func listFormsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Number int
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your forms",
		Long:    "List your existing forms. To create one, run: `auth0 forms create`.",
		Example: `  auth0 forms list
  auth0 forms ls
  auth0 forms ls --number 100
  auth0 forms ls --json
  auth0 forms ls --csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			params := &managementv3.ListFormsRequestParameters{}

			var forms []*managementv3.FormSummary
			if err := ansi.Waiting(func() (err error) {
				forms, err = collectForms(cmd.Context(), cli, params, inputs.Number)
				return err
			}); err != nil {
				return fmt.Errorf("failed to list forms: %w", err)
			}

			return cli.renderer.FormsList(forms)
		},
	}

	cmd.Flags().IntVarP(&inputs.Number, "number", "n", 100, "Number of forms to retrieve. Fetched across pages.")
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	return cmd
}

func showFormCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show a form",
		Long:  "Display information about a form.",
		Example: `  auth0 forms show
  auth0 forms show <form-id>
  auth0 forms show <form-id> --json
  auth0 forms show <form-id> --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := formID.Pick(cmd, &inputs.ID, cli.formPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			form, err := cli.formRawGet(cmd.Context(), inputs.ID)
			if err != nil {
				return fmt.Errorf("failed to read form with ID %q: %w", inputs.ID, err)
			}

			return cli.renderer.FormShowRaw(form)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func createFormCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Name            string
		File            string
		LanguagePrimary string
		LanguageDefault string
		Edit            bool
		Example         bool
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new form",
		Long: "Create a new form.\n\n" +
			"Interactive behavior: `auth0 forms create` asks only for the name and creates a minimal " +
			"scaffold; it does not open an editor. You can then refine the form in the dashboard builder.\n\n" +
			"Pass `--edit` to open an editor and author the form graph before it is created, or supply " +
			"the whole body via `--file` (or piped stdin) with optional `--name` and `--language-*` " +
			"overrides. Run `auth0 forms create --example > form.json` to generate an accepted file payload.",
		Example: `  auth0 forms create
  auth0 forms create --name "My Form"
  auth0 forms create --name "My Form" --edit
  auth0 forms create --example > form.json
  auth0 forms create --file ./form.json
  auth0 forms create --file ./form.json --name "My Form" --language-primary en
  cat form.json | auth0 forms create -f -`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Example {
				cli.renderer.FormExport(formCreateExample)
				return nil
			}

			body, err := readFormBody(inputs.File)
			if err != nil {
				return err
			}

			rawBody := json.RawMessage(body)
			if body == nil {
				// No file or piped body: the name is a required scalar, so prompt for
				// it explicitly (only when interactive and --name was not supplied).
				if err := formName.Ask(cmd, &inputs.Name, nil); err != nil {
					return err
				}
				if inputs.Name == "" {
					return errors.New("a form name is required; supply --name, provide --file, or pipe JSON via stdin")
				}
				if inputs.Edit {
					if !canPrompt(cmd) {
						return errors.New("the --edit flag requires an interactive terminal")
					}
					if err := editFormJSON(cli, formCreateSkeleton, &rawBody); err != nil {
						return err
					}
				} else {
					rawBody = json.RawMessage(formCreateSkeleton)
				}
			}

			rawBody, err = applyRawFormOverrides(
				rawBody,
				inputs.Name,
				inputs.LanguagePrimary,
				inputs.LanguageDefault,
			)
			if err != nil {
				return fmt.Errorf("failed to parse form body: %w", err)
			}

			name, err := rawFormStringField(rawBody, "name")
			if err != nil {
				return fmt.Errorf("failed to parse form body: %w", err)
			}
			if name == "" {
				return errors.New("a form name is required; set it in the body or with --name")
			}

			created, err := cli.formRawCreate(cmd.Context(), rawBody)
			if err != nil {
				return fmt.Errorf("failed to create form: %w", err)
			}
			if err := cli.renderer.FormCreateRaw(created); err != nil {
				return err
			}

			id, err := rawFormStringField(created, "id")
			if err != nil {
				return fmt.Errorf("failed to parse created form: %w", err)
			}
			formNextStepsHint(cli, id)
			return nil
		},
	}

	formName.RegisterString(cmd, &inputs.Name, "")
	formFile.RegisterString(cmd, &inputs.File, "")
	formLanguagePrimary.RegisterString(cmd, &inputs.LanguagePrimary, "")
	formLanguageDefault.RegisterString(cmd, &inputs.LanguageDefault, "")
	formEdit.RegisterBool(cmd, &inputs.Edit, false)
	formExample.RegisterBool(cmd, &inputs.Example, false)
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func updateFormCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID              string
		Name            string
		File            string
		LanguagePrimary string
		LanguageDefault string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update a form",
		Long: "Update a form.\n\n" +
			"Passing `--file` (or piped stdin) replaces every top-level field present in the file. " +
			"Passing only scalar flags such as `--name` performs a merge that preserves the form's " +
			"graph fields (nodes, style, translations). Server-managed fields such as `id`, " +
			"`created_at`, and `updated_at` are removed before the update request is sent.",
		Example: `  auth0 forms update <form-id> --name "New Name"
  auth0 forms update <form-id> --file ./form.json
  cat form.json | auth0 forms update <form-id> -f -`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := formID.Pick(cmd, &inputs.ID, cli.formPickerOptions); err != nil {
					return err
				}
			}

			body, err := readFormBody(inputs.File)
			if err != nil {
				return err
			}

			var rawBody json.RawMessage

			switch {
			case body != nil:
				// File / stdin: whole-file overwrite of present top-level fields.
				rawBody, err = applyRawFormOverrides(
					body,
					inputs.Name,
					inputs.LanguagePrimary,
					inputs.LanguageDefault,
				)
				if err != nil {
					return fmt.Errorf("failed to parse form body: %w", err)
				}
			case inputs.Name != "" || inputs.LanguagePrimary != "" || inputs.LanguageDefault != "":
				primary := inputs.LanguagePrimary
				def := inputs.LanguageDefault
				if primary != "" || def != "" {
					// The API replaces the languages object, so retain the value that was
					// not explicitly overridden. This scalar read is safe through v3.
					var current *managementv3.GetFormResponseContent
					if err := ansi.Waiting(func() (err error) {
						current, err = cli.apiv3.Form.Get(
							cmd.Context(),
							inputs.ID,
							&managementv3.GetFormRequestParameters{},
						)
						return err
					}); err != nil {
						return fmt.Errorf("failed to read form with ID %q: %w", inputs.ID, err)
					}
					languages := current.GetLanguages()
					if primary == "" {
						primary = languages.GetPrimary()
					}
					if def == "" {
						def = languages.GetDefault()
					}
				}

				rawBody, err = applyRawFormOverrides(json.RawMessage(`{}`), inputs.Name, primary, def)
				if err != nil {
					return fmt.Errorf("failed to build form update: %w", err)
				}
			case canPrompt(cmd):
				// Editor fallback: pre-load the exact wire body and full-replace.
				current, err := cli.formRawGet(cmd.Context(), inputs.ID)
				if err != nil {
					return fmt.Errorf("failed to read form with ID %q: %w", inputs.ID, err)
				}

				var seed bytes.Buffer
				if err := json.Indent(&seed, current, "", "  "); err != nil {
					return fmt.Errorf("failed to parse form with ID %q: %w", inputs.ID, err)
				}

				if err := editFormJSON(cli, seed.String(), &rawBody); err != nil {
					return err
				}
			default:
				return errors.New("nothing to update; supply --file, pipe JSON via stdin, or a scalar flag such as --name")
			}

			updated, err := cli.formRawUpdate(cmd.Context(), inputs.ID, rawBody)
			if err != nil {
				return fmt.Errorf("failed to update form with ID %q: %w", inputs.ID, err)
			}
			if err := cli.renderer.FormUpdateRaw(updated); err != nil {
				return err
			}
			formNextStepsHint(cli, inputs.ID)
			return nil
		},
	}

	formName.RegisterStringU(cmd, &inputs.Name, "")
	formFile.RegisterStringU(cmd, &inputs.File, "")
	formLanguagePrimary.RegisterStringU(cmd, &inputs.LanguagePrimary, "")
	formLanguageDefault.RegisterStringU(cmd, &inputs.LanguageDefault, "")
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func deleteFormCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Args:    cobra.ArbitraryArgs,
		Short:   "Delete a form",
		Long: "Delete a form.\n\n" +
			"To delete interactively, use `auth0 forms delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the form id and the `--force` flag to skip confirmation.",
		Example: `  auth0 forms delete
  auth0 forms rm
  auth0 forms delete <form-id>
  auth0 forms delete <form-id> --force
  auth0 forms delete <form-id> <form-id2> <form-idn>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var ids []string
			if len(args) == 0 {
				if err := formID.PickMany(cmd, &ids, cli.formPickerOptions); err != nil {
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

			return ansi.ProgressBar("Deleting form(s)", ids, func(_ int, id string) error {
				if id == "" {
					return nil
				}
				if err := cli.apiv3.Form.Delete(cmd.Context(), id); err != nil {
					return fmt.Errorf("failed to delete form with ID %q: %w", id, err)
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

func exportFormCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID      string
		Output  string
		Compact bool
	}

	cmd := &cobra.Command{
		Use:   "export",
		Args:  cobra.MaximumNArgs(1),
		Short: "Export a form",
		Long: "Export a form as JSON. Writes to stdout by default (pipe-friendly) or to a file " +
			"with `--output`. The output uses the same envelope as the Auth0 Dashboard " +
			"(`version`, `form`, `flows`, `connections`), bundling the flows and vault connections " +
			"the form references with portable `#FLOW-N#`/`#CONN-N#` placeholders, so it can be " +
			"imported by the CLI or opened in the Dashboard.",
		Example: `  auth0 forms export <form-id>
  auth0 forms export <form-id> --output ./form.json
  auth0 forms export <form-id> --json-compact
  auth0 forms export <form-id> | auth0 forms import -f -`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := formID.Pick(cmd, &inputs.ID, cli.formPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			form, err := cli.formRawGet(cmd.Context(), inputs.ID)
			if err != nil {
				return fmt.Errorf("failed to read form with ID %q: %w", inputs.ID, err)
			}

			env, err := cli.buildFormEnvelope(cmd.Context(), form)
			if err != nil {
				return err
			}

			var data []byte
			if inputs.Compact {
				data, err = json.Marshal(env)
			} else {
				data, err = json.MarshalIndent(env, "", "  ")
			}
			if err != nil {
				return fmt.Errorf("failed to marshal form: %w", err)
			}

			if inputs.Output != "" {
				if err := os.WriteFile(inputs.Output, data, 0600); err != nil {
					return fmt.Errorf("failed to write form to %q: %w", inputs.Output, err)
				}
				cli.renderer.Infof("Exported form %s to %s", inputs.ID, inputs.Output)
				return nil
			}

			cli.renderer.FormExport(string(data))
			return nil
		},
	}

	formOutput.RegisterString(cmd, &inputs.Output, "")
	cmd.Flags().BoolVar(&inputs.Compact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func importFormCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID          string
		File        string
		Connections map[string]string
	}

	cmd := &cobra.Command{
		Use:   "import",
		Args:  cobra.NoArgs,
		Short: "Import a form",
		Long: "Import a form from a JSON file (or piped stdin). Without `--id` a new form is " +
			"created; with `--id` the existing form is replaced.\n\n" +
			"Both a flat form graph and the Dashboard envelope (`version`, `form`, `flows`, " +
			"`connections`) are accepted. For an envelope, the bundled flows are created and each " +
			"`#CONN-N#` connection placeholder is mapped to an existing vault connection, either " +
			"interactively or with `--connection`.",
		Example: `  auth0 forms import --file ./form.json
  auth0 forms import --file ./form.json --id <form-id>
  auth0 forms import --file ./form.json --connection '#CONN-1#=ac_123'
  cat form.json | auth0 forms import -f -`,
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := readFormBody(inputs.File)
			if err != nil {
				return err
			}
			if body == nil {
				return errors.New("no form body provided; supply --file or pipe JSON via stdin")
			}

			if isFormEnvelope(body) {
				resolved, err := cli.resolveFormEnvelope(cmd, body, inputs.Connections)
				if err != nil {
					return err
				}
				body = resolved
			}

			// Parse just enough to validate the JSON and read the name. The body is
			// created/updated as raw JSON so STEP/ROUTER node config is preserved
			// (the typed request models drop it via the lossy FormNode union).
			var meta struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(body, &meta); err != nil {
				return fmt.Errorf("failed to parse form body: %w", err)
			}

			if inputs.ID == "" {
				if meta.Name == "" {
					return errors.New("a form name is required in the imported body")
				}

				raw, err := cli.formRawCreate(cmd.Context(), body)
				if err != nil {
					return fmt.Errorf("failed to create form: %w", err)
				}

				return cli.renderer.FormCreateRaw(raw)
			}

			raw, err := cli.formRawUpdate(cmd.Context(), inputs.ID, body)
			if err != nil {
				return fmt.Errorf("failed to update form with ID %q: %w", inputs.ID, err)
			}

			return cli.renderer.FormUpdateRaw(raw)
		},
	}

	formFile.RegisterString(cmd, &inputs.File, "")
	formImportID.RegisterString(cmd, &inputs.ID, "")
	cmd.Flags().StringToStringVar(&inputs.Connections, "connection", nil,
		"Map an exported connection placeholder to an existing vault connection ID, "+
			"e.g. --connection '#CONN-1#=ac_123'. Repeatable.")
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func openFormCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "open",
		Args:  cobra.MaximumNArgs(1),
		Short: "Open a form in the Auth0 Dashboard",
		Long:  "Open a form's page in the Auth0 Dashboard form builder.",
		Example: `  auth0 forms open
  auth0 forms open <form-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := formID.Pick(cmd, &inputs.ID, cli.formPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			openFormEditURL(cli, inputs.ID)

			return nil
		},
	}

	return cmd
}

// formsBuilderURL is the host for the Auth0 Forms visual builder. Forms live on a
// dedicated host rather than under the main management dashboard.
const formsBuilderURL = "https://forms.auth0.com"

// openFormEditURL opens the form's builder page in a browser, or prints the URL
// when interactivity is disabled.
func openFormEditURL(cli *cli, id string) {
	url := formatFormEditURL(cli.Config.DefaultTenant, &cli.Config, id)
	if url == "" {
		cli.renderer.Warnf("Failed to format the correct URL, please ensure you have run 'auth0 login' and try again.")
		return
	}

	if cli.noInput {
		cli.renderer.Infof("Open the following URL in a browser: %s", url)
		return
	}

	if err := browser.OpenURL(url); err != nil {
		cli.renderer.Warnf("Couldn't open the URL, please do it manually: %s", url)
	}
}

// formatFormEditURL builds the Forms builder URL, deriving the region and tenant
// name the same way formatManageTenantURL does for the management dashboard.
func formatFormEditURL(tenant string, cfg *config.Config, id string) string {
	if len(tenant) == 0 || len(id) == 0 {
		return ""
	}

	s := strings.Split(tenant, ".")
	if len(s) < 3 {
		return ""
	}

	region := "us" // A PUS1 tenant looks like dev-tti06f6y.auth0.com (3 parts).
	if len(s) > 3 {
		region = s[len(s)-3]
	}

	tenantName := cfg.Tenants[tenant].Name
	if len(tenantName) == 0 {
		return ""
	}

	return fmt.Sprintf("%s/tenants/%s/%s/forms/%s/edit", formsBuilderURL, region, tenantName, id)
}

// editFormJSON opens an editor seeded with `seed` and unmarshals the result into
// `target`. When the buffer is not valid JSON it re-opens the editor with the
// user's edits intact rather than discarding them, so a typo never costs work.
func editFormJSON(cli *cli, seed string, target interface{}) error {
	content := seed
	for {
		var edited string
		if err := openCreateEditor(&edited, content, "form.*.json", nil, nil); err != nil {
			return err
		}

		if err := json.Unmarshal([]byte(edited), target); err != nil {
			cli.renderer.Warnf("The form body is not valid JSON: %s", err)
			if !prompt.Confirm("Re-open the editor to fix it?") {
				return errors.New("aborted; the form was not saved")
			}
			content = edited
			continue
		}

		return nil
	}
}

// formNextStepsHint prints follow-up commands after a form is created or updated.
// It stays quiet in JSON output modes so scripted consumers get a clean stream.
func formNextStepsHint(cli *cli, id string) {
	if id == "" || cli.json || cli.jsonCompact {
		return
	}
	cli.renderer.Infof("Inspect it with: %s", ansi.Faint("auth0 forms show "+id))
	cli.renderer.Infof("Edit it in the dashboard with: %s", ansi.Faint("auth0 forms open "+id))
}

// readFormBody resolves a JSON body from an explicit --file, "-"/piped stdin, and
// returns nil when no such source is available so the caller can decide whether to
// fall back to an editor or error.
func readFormBody(filePath string) ([]byte, error) {
	if filePath == "-" {
		data, err := io.ReadAll(iostream.Input)
		if err != nil {
			return nil, fmt.Errorf("failed to read form body from stdin: %w", err)
		}
		return data, nil
	}
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read form file %q: %w", filePath, err)
		}
		return data, nil
	}
	if piped := iostream.PipedInput(); len(piped) > 0 {
		return piped, nil
	}
	return nil, nil
}

// applyRawFormOverrides applies scalar flag overrides without deserializing the
// form graph into the v3 SDK's lossy union types.
func applyRawFormOverrides(body json.RawMessage, name, primary, def string) (json.RawMessage, error) {
	var form map[string]json.RawMessage
	if err := json.Unmarshal(body, &form); err != nil {
		return nil, err
	}
	if form == nil {
		return nil, errors.New("form body must be a JSON object")
	}

	if name != "" {
		encoded, err := json.Marshal(name)
		if err != nil {
			return nil, err
		}
		form["name"] = encoded
	}

	if primary != "" || def != "" {
		languages := make(map[string]json.RawMessage)
		if existing := form["languages"]; len(existing) > 0 && string(existing) != "null" {
			if err := json.Unmarshal(existing, &languages); err != nil {
				return nil, fmt.Errorf("parse languages: %w", err)
			}
		}
		if primary != "" {
			encoded, err := json.Marshal(primary)
			if err != nil {
				return nil, err
			}
			languages["primary"] = encoded
		}
		if def != "" {
			encoded, err := json.Marshal(def)
			if err != nil {
				return nil, err
			}
			languages["default"] = encoded
		}
		encoded, err := json.Marshal(languages)
		if err != nil {
			return nil, err
		}
		form["languages"] = encoded
	}

	return json.Marshal(form)
}

func rawFormStringField(body json.RawMessage, field string) (string, error) {
	var form map[string]json.RawMessage
	if err := json.Unmarshal(body, &form); err != nil {
		return "", err
	}
	if form == nil {
		return "", errors.New("form body must be a JSON object")
	}

	raw, ok := form[field]
	if !ok || string(raw) == "null" {
		return "", nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string: %w", field, err)
	}
	return value, nil
}

// formRawGet fetches a form through the v1 client's HTTP layer without using
// the v3 SDK's lossy form-node unions.
func (c *cli) formRawGet(ctx context.Context, id string) (json.RawMessage, error) {
	return c.formRawRequest(ctx, http.MethodGet, c.api.HTTPClient.URI("forms", id), nil)
}

// formRawCreate creates a form from raw JSON, preserving node config that the
// typed CreateFormRequestContent would drop. It returns the created form JSON.
func (c *cli) formRawCreate(ctx context.Context, body json.RawMessage) (json.RawMessage, error) {
	return c.formRawRequest(ctx, http.MethodPost, c.api.HTTPClient.URI("forms"), body)
}

// formRawUpdate replaces a form from raw JSON, preserving node config that the
// typed UpdateFormRequestContent would drop. It returns the updated form JSON.
func (c *cli) formRawUpdate(ctx context.Context, id string, body json.RawMessage) (json.RawMessage, error) {
	var form map[string]json.RawMessage
	if err := json.Unmarshal(body, &form); err != nil {
		return nil, err
	}
	for _, field := range formServerManagedFields {
		delete(form, field)
	}
	cleanBody, err := json.Marshal(form)
	if err != nil {
		return nil, err
	}

	return c.formRawRequest(ctx, http.MethodPatch, c.api.HTTPClient.URI("forms", id), cleanBody)
}

// formRawRequest sends a raw JSON request to the Management API and returns the
// response body, surfacing API errors the same way the `api` command does.
func (c *cli) formRawRequest(
	ctx context.Context,
	method string,
	uri string,
	body json.RawMessage,
) (json.RawMessage, error) {
	var payload interface{}
	if len(body) > 0 {
		payload = body
	}

	request, err := c.api.HTTPClient.NewRequest(ctx, method, uri, payload)
	if err != nil {
		return nil, err
	}

	var out json.RawMessage
	if err := ansi.Waiting(func() error {
		response, err := c.api.HTTPClient.Do(request)
		if err != nil {
			return err
		}
		defer func() {
			_ = response.Body.Close()
		}()

		data, err := io.ReadAll(response.Body)
		if err != nil {
			return err
		}
		if response.StatusCode >= http.StatusBadRequest {
			return newAPIResponseError(response.StatusCode, response.Header, data)
		}
		out = data
		return nil
	}); err != nil {
		return nil, err
	}

	return out, nil
}

// collectForms pages through the forms list, collecting up to `limit` results
// (all results when limit <= 0).
func collectForms(ctx context.Context, cli *cli, params *managementv3.ListFormsRequestParameters, limit int) ([]*managementv3.FormSummary, error) {
	page, err := cli.apiv3.Form.List(ctx, params)
	if err != nil {
		return nil, err
	}

	var out []*managementv3.FormSummary
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

func (c *cli) formPickerOptions(ctx context.Context) (pickerOptions, error) {
	forms, err := collectForms(ctx, c, &managementv3.ListFormsRequestParameters{}, 0)
	if err != nil {
		return nil, err
	}

	var opts pickerOptions
	for _, f := range forms {
		label := fmt.Sprintf("%s %s", f.GetName(), ansi.Faint("("+f.GetID()+")"))
		opts = append(opts, pickerOption{value: f.GetID(), label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("there are currently no forms to choose from. Create one by running: `auth0 forms create`")
	}

	return opts, nil
}
