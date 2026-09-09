package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/iostream"
	"github.com/auth0/auth0-cli/internal/prompt"
)

// formCreateSkeleton seeds the editor for interactive form creation. The name is
// prompted separately, so the seed carries empty containers for all writable fields.
const formCreateSkeleton = `{
  "messages": {},
  "languages": {},
  "translations": {},
  "nodes": [],
  "start": {},
  "ending": {},
  "style": {}
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

// formEditorSeed controls the field order in the interactive editor seed.
type formEditorSeed struct {
	Name         json.RawMessage `json:"name,omitempty"`
	Messages     json.RawMessage `json:"messages,omitempty"`
	Languages    json.RawMessage `json:"languages,omitempty"`
	Translations json.RawMessage `json:"translations,omitempty"`
	Nodes        json.RawMessage `json:"nodes,omitempty"`
	Start        json.RawMessage `json:"start,omitempty"`
	Ending       json.RawMessage `json:"ending,omitempty"`
	Style        json.RawMessage `json:"style,omitempty"`
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
		Data            string
		LanguagePrimary string
		LanguageDefault string
		Example         bool
		Schema          bool
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new form",
		Long: "Create a new form.\n\n" +
			"Interactive behavior: `auth0 forms create` asks for the name, then offers to author the " +
			"form body in an editor. Decline the prompt to create a minimal scaffold and refine it " +
			"in the dashboard builder instead.\n\n" +
			"Alternatively, supply " +
			"the whole body via `--data` as inline JSON, a file (`@form.json`), or piped stdin. Run " +
			"`auth0 forms create --schema` to print the accepted payload schema and " +
			"`auth0 forms create --example > form.json` to generate a starter body.\n\n" +
			"`--data` provides the whole payload and cannot be combined with `--name` or the " +
			"`--language-*` flags; it is checked for valid JSON and a form name before it is sent, " +
			"and the form graph itself is validated by the API.",
		Example: `  auth0 forms create
  auth0 forms create --name "My Form"
  auth0 forms create --example > form.json
  auth0 forms create --schema
  auth0 forms create --data '{"name":"My Form"}'
  auth0 forms create --data @form.json
  cat form.json | auth0 forms create`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Example {
				cli.renderer.FormExport(formCreateExample)
				return nil
			}

			// Schema discovery mode: print the request payload and exit.
			if inputs.Schema {
				return printOperationSchema(cli, http.MethodPost, "/forms")
			}

			// JSON input mode (for agents and automation): explicit --data or piped
			// stdin. The body is sent verbatim so STEP/ROUTER node config is preserved.
			dataStr, provided, err := ResolveData(cmd)
			if err != nil {
				return err
			}
			if provided {
				return cli.createFormFromJSON(cmd, dataStr)
			}

			// Interactive: the name is a required scalar, so prompt for it (only when
			// interactive and --name was not supplied).
			if err := formName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}
			if inputs.Name == "" {
				return errors.New("a form name is required; supply --name, --data, or pipe JSON via stdin")
			}

			rawBody := json.RawMessage(formCreateSkeleton)
			// When the name was gathered interactively, offer to author the body
			// now. Declining creates a minimal scaffold to refine in the dashboard
			// builder later.
			if canPrompt(cmd) {
				cli.renderer.Infof("A form body is the JSON graph behind the screen: the fields, " +
					"buttons and blocks users see, the steps and routing between them, plus languages " +
					"and styling. You can author it now, or skip and design it visually in the dashboard.")
				if prompt.ConfirmWithDefault("Do you want to author the form body now?", false) {
					if err := editFormJSON(cli, formCreateSkeleton, &rawBody); err != nil {
						return err
					}
				}
			}

			rawBody, err = applyRawFormOverrides(
				rawBody,
				inputs.Name,
				inputs.LanguagePrimary,
				inputs.LanguageDefault,
			)
			if err != nil {
				return fmt.Errorf("failed to build form body: %w", err)
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
	dataFlag.RegisterString(cmd, &inputs.Data, "")
	formLanguagePrimary.RegisterString(cmd, &inputs.LanguagePrimary, "")
	formLanguageDefault.RegisterString(cmd, &inputs.LanguageDefault, "")
	formExample.RegisterBool(cmd, &inputs.Example, false)
	schemaFlag.RegisterBool(cmd, &inputs.Schema, false)
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	// --data supplies the whole payload, so it cannot be combined with the
	// granular input flags. Output flags (--json) and --schema are not affected.
	markDataExclusive(cmd)

	return cmd
}

// createFormFromJSON creates a form from a --data JSON payload, sent verbatim
// to preserve node config the v3 SDK's union types would drop.
func (c *cli) createFormFromJSON(cmd *cobra.Command, dataStr string) error {
	payload, err := validateFormData(c, dataStr, "auth0 forms create", true, nil)
	if err != nil {
		return err
	}

	created, err := c.formRawCreate(cmd.Context(), payload)
	if err != nil {
		return fmt.Errorf("failed to create form: %w", err)
	}
	if err := c.renderer.FormCreateRaw(created); err != nil {
		return err
	}

	id, err := rawFormStringField(created, "id")
	if err != nil {
		return fmt.Errorf("failed to parse created form: %w", err)
	}
	formNextStepsHint(c, id)
	return nil
}

func updateFormCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID              string
		Name            string
		Data            string
		LanguagePrimary string
		LanguageDefault string
		Schema          bool
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update a form",
		Long: "Update a form.\n\n" +
			"Passing `--data` as inline JSON, a file (`@form.json`), or piped stdin replaces every " +
			"top-level field present in the payload. The payload is checked for valid JSON before it " +
			"is sent, and the form graph itself is validated by the API. Passing only scalar flags " +
			"such as `--name` performs a merge that " +
			"preserves the form's graph fields (nodes, style, translations). Server-managed fields " +
			"such as `id`, `created_at`, and `updated_at` are removed before the update request is " +
			"sent.\n\n" +
			"`--data` provides the whole payload and cannot be combined with `--name` or the " +
			"`--language-*` flags. Run `auth0 forms update --schema` to print the accepted payload schema.",
		Example: `  auth0 forms update <form-id> --name "New Name"
  auth0 forms update <form-id> --schema
  auth0 forms update <form-id> --data '{"name":"New Name"}'
  auth0 forms update <form-id> --data @form.json
  cat form.json | auth0 forms update <form-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Schema discovery mode: print the request payload and exit.
			// This does not require a form ID.
			if inputs.Schema {
				return printOperationSchema(cli, http.MethodPatch, "/forms/{id}")
			}

			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := formID.Pick(cmd, &inputs.ID, cli.formPickerOptions); err != nil {
					return err
				}
			}

			// JSON input mode (for agents and automation): explicit --data or piped stdin.
			dataStr, provided, err := ResolveData(cmd)
			if err != nil {
				return err
			}

			var rawBody json.RawMessage

			switch {
			case provided:
				// --data / stdin: whole-payload overwrite. Validated for JSON and sent
				// verbatim; server-managed fields are stripped inside formRawUpdate.
				rawBody, err = validateFormData(
					cli,
					dataStr,
					"auth0 forms update",
					false,
					nil,
				)
				if err != nil {
					return err
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
				// Editor fallback: strip server-managed fields, reorder for DX, and full-replace.
				current, err := cli.formRawGet(cmd.Context(), inputs.ID)
				if err != nil {
					return fmt.Errorf("failed to read form with ID %q: %w", inputs.ID, err)
				}

				var form map[string]json.RawMessage
				if err := json.Unmarshal(current, &form); err != nil {
					return fmt.Errorf("failed to parse form with ID %q: %w", inputs.ID, err)
				}
				for _, f := range formServerManagedFields {
					delete(form, f)
				}

				seedBytes, err := json.MarshalIndent(formEditorSeed{
					Name:         form["name"],
					Messages:     form["messages"],
					Languages:    form["languages"],
					Translations: form["translations"],
					Nodes:        form["nodes"],
					Start:        form["start"],
					Ending:       form["ending"],
					Style:        form["style"],
				}, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to build form editor seed for %q: %w", inputs.ID, err)
				}

				if err := editFormJSON(cli, string(seedBytes), &rawBody); err != nil {
					return err
				}
			default:
				return errors.New("nothing to update; supply --data, pipe JSON via stdin, or a scalar flag such as --name")
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
	dataFlag.RegisterString(cmd, &inputs.Data, "")
	formLanguagePrimary.RegisterStringU(cmd, &inputs.LanguagePrimary, "")
	formLanguageDefault.RegisterStringU(cmd, &inputs.LanguageDefault, "")
	schemaFlag.RegisterBool(cmd, &inputs.Schema, false)
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	// --data supplies the whole payload, so it cannot be combined with the
	// granular input flags. Output flags (--json) and --schema are not affected.
	markDataExclusive(cmd)

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
  auth0 forms export <form-id> | auth0 forms import`,
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
		Data        string
		Connections map[string]string
	}

	cmd := &cobra.Command{
		Use:   "import",
		Args:  cobra.NoArgs,
		Short: "Import a form",
		Long: "Import a form from `--data`, given as inline JSON, a file (`@form.json`), or piped " +
			"stdin. Without `--id` a new form is created; with `--id` the existing form is replaced.\n\n" +
			"Both a flat form graph and the Dashboard envelope (`version`, `form`, `flows`, " +
			"`connections`) are accepted. For an envelope, the bundled flows are created and each " +
			"`#CONN-N#` connection placeholder is mapped to an existing vault connection, either " +
			"interactively or with `--connection`.",
		Example: `  auth0 forms import --data @form.json
  auth0 forms import --data @form.json --id <form-id>
  auth0 forms import --data @form.json --connection '#CONN-1#=ac_123'
  auth0 forms export <form-id> | auth0 forms import`,
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := readFormData(cmd)
			if err != nil {
				return err
			}
			if body == nil {
				return errors.New("no form body provided; supply --data or pipe JSON via stdin")
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

	dataFlag.RegisterString(cmd, &inputs.Data, "")
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

			openBuilderURL(cli, fmt.Sprintf("forms/%s/edit", inputs.ID))

			return nil
		},
	}

	return cmd
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

// readFormData resolves a JSON body from --data or piped stdin, returning nil
// when neither is available. Does not reject other set flags, so import can
// combine --data with --id and --connection.
func readFormData(cmd *cobra.Command) ([]byte, error) {
	if HasData(cmd) {
		value, _ := GetData(cmd)
		if value != "" && value[0] == '@' {
			data, err := os.ReadFile(value[1:])
			if err != nil {
				return nil, fmt.Errorf("failed to read form file %q: %w", value[1:], err)
			}
			return data, nil
		}
		return []byte(value), nil
	}
	if piped := iostream.PipedInput(); len(piped) > 0 {
		return piped, nil
	}
	return nil, nil
}

// validateFormData validates a --data JSON payload and returns the raw bytes to
// send verbatim. Only the envelope is checked locally (valid JSON object, non-empty
// "name" when requireName is set) because the v3 SDK's union types are lossy and
// the OpenAPI schema cannot faithfully represent the form graph (e.g. the "$ending"
// node pointer trips the schema's non-exclusive oneOf). The API validates the graph
// server-side. PreClean runs before validation when non-nil.
func validateFormData(
	cli *cli,
	dataStr, schemaCmd string,
	requireName bool,
	preClean func(json.RawMessage) (json.RawMessage, error),
) (json.RawMessage, error) {
	handler := &DataJSONHandler{cli: cli}

	raw, err := handler.readJSONInput(dataStr)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON input: %w", err)
	}

	if preClean != nil {
		raw, err = preClean(raw)
		if err != nil {
			return nil, fmt.Errorf("failed to parse form body: %w", err)
		}
	}

	var form map[string]json.RawMessage
	if err := json.Unmarshal(raw, &form); err != nil {
		cli.renderer.Infof("Run '%s --schema' to see the accepted schema.", schemaCmd)
		return nil, fmt.Errorf("invalid JSON: the form payload must be a JSON object: %w", err)
	}

	if requireName {
		name, err := rawFormStringField(raw, "name")
		if err != nil {
			return nil, err
		}
		if name == "" {
			cli.renderer.Infof("Run '%s --schema' to see the accepted schema.", schemaCmd)
			return nil, errors.New(`the form payload must include a non-empty "name"`)
		}
	}

	return raw, nil
}

// stripFormServerManagedFields removes the fields the API sets and rejects on
// write (id, timestamps, links) so an exported or previously-read form body can be
// sent back on create or update without a schema-additionalProperties violation.
func stripFormServerManagedFields(body json.RawMessage) (json.RawMessage, error) {
	var form map[string]json.RawMessage
	if err := json.Unmarshal(body, &form); err != nil {
		return nil, err
	}
	for _, field := range formServerManagedFields {
		delete(form, field)
	}
	return json.Marshal(form)
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

// formRawCreate creates a form from raw JSON, stripping server-managed fields
// and preserving node config the typed CreateFormRequestContent would drop.
func (c *cli) formRawCreate(ctx context.Context, body json.RawMessage) (json.RawMessage, error) {
	cleanBody, err := stripFormServerManagedFields(body)
	if err != nil {
		return nil, err
	}

	return c.formRawRequest(ctx, http.MethodPost, c.api.HTTPClient.URI("forms"), cleanBody)
}

// formRawUpdate replaces a form from raw JSON, preserving node config that the
// typed UpdateFormRequestContent would drop. It returns the updated form JSON.
func (c *cli) formRawUpdate(ctx context.Context, id string, body json.RawMessage) (json.RawMessage, error) {
	cleanBody, err := stripFormServerManagedFields(body)
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
