package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/auth0/go-auth0/v3/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/prompt"
)

var (
	featureFlagID = Argument{
		Name: "Feature Flag ID",
		Help: "ID of the feature flag.",
	}

	featureFlagName = Flag{
		Name:       "Name",
		LongForm:   "name",
		ShortForm:  "n",
		Help:       "Name of the feature flag.",
		IsRequired: true,
	}

	featureFlagDescription = Flag{
		Name:         "Description",
		LongForm:     "description",
		ShortForm:    "d",
		AlwaysPrompt: true,
		Help:         "Description of the feature flag.",
	}

	featureFlagParameters = Flag{
		Name:       "Parameters",
		LongForm:   "parameters",
		ShortForm:  "p",
		Help:       `Parameters schema as JSON. Example: '{"color":{"type":"string","value":"blue"}}'`,
		IsRequired: true,
	}

	variationID = Argument{
		Name: "Variation ID",
		Help: "ID of the variation.",
	}

	variationName = Flag{
		Name:       "Name",
		LongForm:   "name",
		ShortForm:  "n",
		Help:       "Name of the variation.",
		IsRequired: true,
	}

	variationDescription = Flag{
		Name:         "Description",
		LongForm:     "description",
		ShortForm:    "d",
		AlwaysPrompt: true,
		Help:         "Description of the variation.",
	}

	variationOverrides = Flag{
		Name:       "Overrides",
		LongForm:   "overrides",
		ShortForm:  "o",
		Help:       `Parameter overrides as JSON mapping each parameter name directly to its override value. Example: '{"color":"red"}'`,
		IsRequired: true,
	}

	featureFlagStatus = Flag{
		Name:     "Status",
		LongForm: "status",
		Help:     "Transition the feature flag to a new status (active, archived). Archiving is irreversible.",
	}

	featureFlagFilterStatus = Flag{
		Name:     "Status",
		LongForm: "status",
		Help:     "Filter by status (draft, active, archived).",
	}

	featureFlagFilterType = Flag{
		Name:     "Type",
		LongForm: "type",
		Help:     "Filter by type (auth0, self).",
	}
)

// featureFlagStatuses are the states a feature flag can be transitioned to
// (draft is the initial state only).
var featureFlagStatuses = []string{"active", "archived"}

func featureFlagsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feature-flags",
		Short: "Manage experimentation feature flags",
		Long:  "Feature flags define named parameters (string, boolean, number, array, object) that experiments vary across user groups.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listFeatureFlagsCmd(cli))
	cmd.AddCommand(createFeatureFlagCmd(cli))
	cmd.AddCommand(showFeatureFlagCmd(cli))
	cmd.AddCommand(updateFeatureFlagCmd(cli))
	cmd.AddCommand(deleteFeatureFlagCmd(cli))
	cmd.AddCommand(variationsCmd(cli))

	return cmd
}

func listFeatureFlagsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Status string
		Type   string
		Schema bool
		Query  string
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your feature flags",
		Long:    "List all feature flags. To create one, run: `auth0 experimentation feature-flags create`." + experimentationQueryHelp,
		Example: `  auth0 experimentation feature-flags list
  auth0 experimentation feature-flags ls
  auth0 experimentation feature-flags list --json
  auth0 experimentation feature-flags list --status active
  auth0 experimentation feature-flags list --schema
  auth0 experimentation feature-flags list --schema --json
  auth0 experimentation feature-flags list --query '{"status":"active","take":10}'
  auth0 experimentation feature-flags list --query '{"type":"self"}' --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Schema {
				return printOperationSchema(cli, "GET", "/experimentation/feature-flags")
			}

			if inputs.Query != "" {
				return runJSONQuery(cli, cmd, jsonQuerySpec{
					Path:      "experimentation/feature-flags",
					SchemaCmd: "auth0 experimentation feature-flags list",
				}, inputs.Query)
			}

			req := &management.ListFeatureFlagsRequestParameters{}
			if inputs.Status != "" {
				s := management.FeatureFlagStatusEnum(inputs.Status)
				req.Status = &s
			}
			if inputs.Type != "" {
				t := management.FeatureFlagTypeEnum(inputs.Type)
				req.Type = &t
			}

			allFlags, err := collectV3Pages(cmd.Context(), 0,
				func(ctx context.Context) (*auth0.FeatureFlagListPage, error) {
					return cli.apiv3.FeatureFlags.List(ctx, req)
				})
			if err != nil {
				return fmt.Errorf("failed to list feature flags: %w", err)
			}

			cli.renderer.FeatureFlagList(allFlags)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	featureFlagFilterStatus.RegisterString(cmd, &inputs.Status, "")
	featureFlagFilterType.RegisterString(cmd, &inputs.Type, "")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")
	schemaFlag.RegisterBool(cmd, &inputs.Schema, false)
	listQueryFlag.RegisterString(cmd, &inputs.Query, "")
	markQueryFilterExclusive(cmd, featureFlagFilterStatus.LongForm, featureFlagFilterType.LongForm)

	return cmd
}

func showFeatureFlagCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show a feature flag",
		Long:  "Display details about a feature flag including its parameters.",
		Example: `  auth0 experimentation feature-flags show
  auth0 experimentation feature-flags show <feature-flag-id>
  auth0 experimentation feature-flags show <feature-flag-id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := featureFlagID.Pick(cmd, &inputs.ID, cli.featureFlagPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var ff *management.GetFeatureFlagResponseContent
			if err := ansi.Waiting(func() (err error) {
				ff, err = cli.apiv3.FeatureFlags.Get(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to get feature flag %q: %w", inputs.ID, err)
			}

			cli.renderer.FeatureFlagShow(ff)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func createFeatureFlagCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Name        string
		Description string
		Parameters  string
		Data        string
		Schema      bool
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new feature flag",
		Long: "Create a new feature flag.\n\n" +
			"To create interactively, use `auth0 experimentation feature-flags create` with no flags.\n\n" +
			"To create non-interactively, supply name and parameters through the flags." + experimentationDataHelp,
		Example: `  auth0 experimentation feature-flags create
  auth0 experimentation feature-flags create --name "dark-mode" --parameters '{"enabled":{"type":"boolean","value":false}}'
  auth0 experimentation feature-flags create -n "checkout-flow" -p '{"variant":{"type":"string","value":"control"}}'

  # Discover the payload schema
  auth0 experimentation feature-flags create --schema
  auth0 experimentation feature-flags create --schema --json

  # JSON input mode (for agents and automation)
  auth0 experimentation feature-flags create --data '{"name":"dark-mode","parameters":{"enabled":{"type":"boolean","value":false}}}'
  auth0 experimentation feature-flags create --data @feature-flag.json
  cat feature-flag.json | auth0 experimentation feature-flags create`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Schema {
				return printOperationSchema(cli, "POST", "/experimentation/feature-flags")
			}

			payload, provided, err := ResolveData(cmd)
			if err != nil {
				return err
			}
			if provided {
				return createFeatureFlagFromJSON(cli, cmd, payload)
			}

			if err := featureFlagName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}
			if inputs.Name == "" {
				return usageError{err: errors.New("--name is required"), reason: "missing_required_flags"}
			}

			if err := featureFlagDescription.Ask(cmd, &inputs.Description, nil); err != nil {
				return err
			}
			if err := validateMinLength(featureFlagDescription.LongForm, inputs.Description, experimentationMinDescriptionLength); err != nil {
				return err
			}

			if err := featureFlagParameters.OpenEditor(
				cmd,
				&inputs.Parameters,
				`{"param_name":{"type":"string","value":"default_value"}}`,
				"feature-flag-params.*.json",
				cli.featureFlagParamsEditorHint,
			); err != nil {
				return err
			}

			if inputs.Parameters == "" {
				return usageError{
					err:    errors.New("--parameters is required (e.g. --parameters '{\"color\":{\"type\":\"string\",\"value\":\"blue\"}}')"),
					reason: "missing_required_flags",
				}
			}
			var params management.CreateFeatureFlagParameters
			if err := json.Unmarshal([]byte(inputs.Parameters), &params); err != nil {
				return validationError{err: fmt.Errorf("invalid JSON for --parameters (ensure the value is quoted in your shell): %w", err), reason: "malformed_json"}
			}

			req := &management.CreateFeatureFlagRequestContent{
				Name:       inputs.Name,
				Parameters: params,
			}
			if inputs.Description != "" {
				req.Description = &inputs.Description
			}

			var result *management.CreateFeatureFlagResponseContent
			if err := ansi.Waiting(func() (err error) {
				result, err = cli.apiv3.FeatureFlags.Create(cmd.Context(), req)
				return err
			}); err != nil {
				return fmt.Errorf("failed to create feature flag: %w", err)
			}

			return cli.renderer.FeatureFlagCreate(result)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	featureFlagName.RegisterString(cmd, &inputs.Name, "")
	featureFlagDescription.RegisterString(cmd, &inputs.Description, "")
	featureFlagParameters.RegisterString(cmd, &inputs.Parameters, "")
	dataFlag.RegisterString(cmd, &inputs.Data, "")
	schemaFlag.RegisterBool(cmd, &inputs.Schema, false)
	markDataExclusive(cmd)

	return cmd
}

func updateFeatureFlagCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID          string
		Name        string
		Description string
		Parameters  string
		Status      string
		Data        string
		Schema      bool
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update a feature flag",
		Long: "Update a feature flag.\n\n" +
			"To update interactively, use `auth0 experimentation feature-flags update` with no arguments.\n\n" +
			"To update non-interactively, supply the feature flag ID and fields to change through the flags." + experimentationDataHelp,
		Example: `  auth0 experimentation feature-flags update
  auth0 experimentation feature-flags update <feature-flag-id>
  auth0 experimentation feature-flags update <feature-flag-id> --name "new-name"
  auth0 experimentation feature-flags update <feature-flag-id> --parameters '{"enabled":{"type":"boolean","value":true}}'
  auth0 experimentation feature-flags update <feature-flag-id> --status active

  # Discover the payload schema
  auth0 experimentation feature-flags update --schema
  auth0 experimentation feature-flags update --schema --json

  # JSON input mode (for agents and automation)
  auth0 experimentation feature-flags update <feature-flag-id> --data '{"status":"active"}'
  auth0 experimentation feature-flags update <feature-flag-id> --data @feature-flag.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Schema {
				return printOperationSchema(cli, "PATCH", "/experimentation/feature-flags/{id}")
			}

			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := featureFlagID.Pick(cmd, &inputs.ID, cli.featureFlagPickerOptions); err != nil {
					return err
				}
			}

			payload, provided, err := ResolveData(cmd)
			if err != nil {
				return err
			}
			if provided {
				return updateFeatureFlagFromJSON(cli, cmd, inputs.ID, payload)
			}

			// Read the current feature flag so untouched fields keep their existing
			// value and only changed fields are sent.
			var current *management.GetFeatureFlagResponseContent
			if err := ansi.Waiting(func() (err error) {
				current, err = cli.apiv3.FeatureFlags.Get(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to get feature flag %q: %w", inputs.ID, err)
			}

			currentName := current.GetName()
			currentDescription := current.GetDescription()

			if !featureFlagName.IsSet(cmd) {
				inputs.Name = currentName
			}
			if !featureFlagDescription.IsSet(cmd) {
				inputs.Description = currentDescription
			}

			if err := featureFlagName.AskU(cmd, &inputs.Name, &currentName); err != nil {
				return err
			}
			if err := featureFlagDescription.AskU(cmd, &inputs.Description, &currentDescription); err != nil {
				return err
			}

			// Open the editor pre-filled with the current parameters (same as
			// create), unless the value was supplied via the flag. Keeping the
			// content unchanged leaves the parameters as-is.
			currentParameters := marshalToJSON(current.GetParameters())
			if !featureFlagParameters.IsSet(cmd) {
				if err := featureFlagParameters.OpenEditorU(
					cmd,
					&inputs.Parameters,
					currentParameters,
					"feature-flag-params.*.json",
				); err != nil {
					return err
				}
			}

			// Status isn't a free-text field, so (when not supplied via a flag)
			// offer it behind a confirmation rather than always prompting.
			statusChanged := featureFlagStatus.IsSet(cmd)
			if !statusChanged && canPrompt(cmd) && prompt.Confirm("Change the status?") {
				if err := featureFlagStatus.Select(cmd, &inputs.Status, featureFlagStatuses, nil); err != nil {
					return err
				}
				statusChanged = true
			}

			// Diff scalar fields against the current flag; parameters are replaced
			// wholesale only when they actually changed.
			req := &management.UpdateFeatureFlagRequestContent{}
			updated := false

			if inputs.Name != currentName {
				req.Name = &inputs.Name
				updated = true
			}
			if inputs.Description != currentDescription {
				// The API rejects an empty description; clearing it means sending null.
				if inputs.Description == "" {
					req.SetDescription(nil)
				} else {
					if err := validateMinLength(featureFlagDescription.LongForm, inputs.Description, experimentationMinDescriptionLength); err != nil {
						return err
					}
					req.Description = &inputs.Description
				}
				updated = true
			}
			if inputs.Parameters != "" && inputs.Parameters != currentParameters {
				var params management.UpdateFeatureFlagParameters
				if err := json.Unmarshal([]byte(inputs.Parameters), &params); err != nil {
					return validationError{err: fmt.Errorf("invalid JSON for --parameters (ensure the value is quoted in your shell): %w", err), reason: "malformed_json"}
				}
				req.Parameters = &params
				updated = true
			}
			if statusChanged {
				if !slices.Contains(featureFlagStatuses, inputs.Status) {
					return usageError{err: fmt.Errorf("invalid status %q: must be one of %s", inputs.Status, strings.Join(featureFlagStatuses, ", ")), reason: "invalid_flag_value"}
				}
				status := management.FeatureFlagStatusEnum(inputs.Status)
				req.Status = &status
				updated = true
			}

			if !updated {
				return usageError{err: errors.New("nothing to update: provide at least one flag or --data"), reason: "missing_required_flags"}
			}

			var result *management.UpdateFeatureFlagResponseContent
			if err := ansi.Waiting(func() (err error) {
				result, err = cli.apiv3.FeatureFlags.Update(cmd.Context(), inputs.ID, req)
				return err
			}); err != nil {
				return fmt.Errorf("failed to update feature flag %q: %w", inputs.ID, err)
			}

			return cli.renderer.FeatureFlagUpdate(result)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	featureFlagName.RegisterStringU(cmd, &inputs.Name, "")
	featureFlagDescription.RegisterStringU(cmd, &inputs.Description, "")
	featureFlagParameters.RegisterStringU(cmd, &inputs.Parameters, "")
	featureFlagStatus.RegisterStringU(cmd, &inputs.Status, "")
	dataFlag.RegisterString(cmd, &inputs.Data, "")
	schemaFlag.RegisterBool(cmd, &inputs.Schema, false)
	markDataExclusive(cmd)

	return cmd
}

func createFeatureFlagFromJSON(cli *cli, cmd *cobra.Command, dataStr string) error {
	result, err := runJSONWrite[management.CreateFeatureFlagResponseContent](cli, cmd, jsonWriteSpec{
		Method:     http.MethodPost,
		SchemaPath: "/experimentation/feature-flags",
		URI:        cli.api.HTTPClient.URI("experimentation", "feature-flags"),
		Data:       dataStr,
		SchemaCmd:  "auth0 experimentation feature-flags create",
	})
	if err != nil {
		return fmt.Errorf("failed to create feature flag: %w", err)
	}

	return cli.renderer.FeatureFlagCreate(result)
}

func updateFeatureFlagFromJSON(cli *cli, cmd *cobra.Command, id, dataStr string) error {
	result, err := runJSONWrite[management.UpdateFeatureFlagResponseContent](cli, cmd, jsonWriteSpec{
		Method:     http.MethodPatch,
		SchemaPath: "/experimentation/feature-flags/{id}",
		URI:        cli.api.HTTPClient.URI("experimentation", "feature-flags", id),
		Data:       dataStr,
		SchemaCmd:  "auth0 experimentation feature-flags update",
	})
	if err != nil {
		return fmt.Errorf("failed to update feature flag %q: %w", id, err)
	}

	return cli.renderer.FeatureFlagUpdate(result)
}

func deleteFeatureFlagCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Short:   "Delete a feature flag",
		Long: "Delete a feature flag.\n\n" +
			"To delete interactively, use `auth0 experimentation feature-flags delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the feature flag ID and use `--force` to skip confirmation.",
		Example: `  auth0 experimentation feature-flags delete
  auth0 experimentation feature-flags delete <feature-flag-id>
  auth0 experimentation feature-flags delete <feature-flag-id> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var ids []string
			if len(args) == 0 {
				if err := featureFlagID.PickMany(cmd, &ids, cli.featureFlagPickerOptions); err != nil {
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

			return ansi.ProgressBar("Deleting feature flag(s)", ids, func(_ int, id string) error {
				if id != "" {
					// Delete returns 204 for an unknown ID, so check that it exists first.
					if _, err := cli.apiv3.FeatureFlags.Get(cmd.Context(), id); err != nil {
						return fmt.Errorf("failed to delete feature flag %q: %w", id, err)
					}

					if err := cli.apiv3.FeatureFlags.Delete(cmd.Context(), id); err != nil {
						return fmt.Errorf("failed to delete feature flag %q: %w", id, err)
					}
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

// variationsCmd groups variation sub-commands under feature-flags.
func variationsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "variations",
		Short: "Manage variations of a feature flag",
		Long:  "Variations define the different parameter overrides for a feature flag (e.g. control vs treatment arms).",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listVariationsCmd(cli))
	cmd.AddCommand(createVariationCmd(cli))
	cmd.AddCommand(showVariationCmd(cli))
	cmd.AddCommand(updateVariationCmd(cli))
	cmd.AddCommand(deleteVariationCmd(cli))

	return cmd
}

func listVariationsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		FeatureFlagID string
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "List variations of a feature flag",
		Long:    "List all variations for a given feature flag.",
		Example: `  auth0 experimentation feature-flags variations list
  auth0 experimentation feature-flags variations list <feature-flag-id>
  auth0 experimentation feature-flags variations list <feature-flag-id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.FeatureFlagID = args[0]
			} else {
				if err := featureFlagID.Pick(cmd, &inputs.FeatureFlagID, cli.featureFlagPickerOptions); err != nil {
					return err
				}
			}

			var result *management.ListVariationsResponseContent
			if err := ansi.Waiting(func() (err error) {
				result, err = cli.apiv3.Variations.List(cmd.Context(), inputs.FeatureFlagID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to list variations for feature flag %q: %w", inputs.FeatureFlagID, err)
			}

			cli.renderer.VariationList(result.GetVariations())
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	return cmd
}

func showVariationCmd(cli *cli) *cobra.Command {
	var inputs struct {
		FeatureFlagID string
		VariationID   string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(2),
		Short: "Show a variation",
		Long:  "Display details about a specific variation.",
		Example: `  auth0 experimentation feature-flags variations show
  auth0 experimentation feature-flags variations show <feature-flag-id> <variation-id>
  auth0 experimentation feature-flags variations show <feature-flag-id> <variation-id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) >= 1 {
				inputs.FeatureFlagID = args[0]
			}
			if len(args) == 2 {
				inputs.VariationID = args[1]
			}

			if inputs.FeatureFlagID == "" {
				if err := featureFlagID.Pick(cmd, &inputs.FeatureFlagID, cli.featureFlagPickerOptions); err != nil {
					return err
				}
			}

			if inputs.VariationID == "" {
				if err := variationID.Pick(cmd, &inputs.VariationID, cli.variationPickerOptions(inputs.FeatureFlagID)); err != nil {
					return err
				}
			}

			var v *management.GetVariationResponseContent
			if err := ansi.Waiting(func() (err error) {
				v, err = cli.apiv3.Variations.Get(cmd.Context(), inputs.FeatureFlagID, inputs.VariationID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to get variation %q: %w", inputs.VariationID, err)
			}

			cli.renderer.VariationShow(v)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func createVariationCmd(cli *cli) *cobra.Command {
	var inputs struct {
		FeatureFlagID string
		Name          string
		Description   string
		Overrides     string
		Data          string
		Schema        bool
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.MaximumNArgs(1),
		Short: "Create a new variation",
		Long: "Create a new variation for a feature flag.\n\n" +
			"To create interactively, use `auth0 experimentation feature-flags variations create` with no flags.\n\n" +
			"To create non-interactively, supply the feature flag ID, name, and overrides through the flags." + experimentationDataHelp,
		Example: `  auth0 experimentation feature-flags variations create
  auth0 experimentation feature-flags variations create <feature-flag-id>
  auth0 experimentation feature-flags variations create <feature-flag-id> --name "treatment" --overrides '{"color":"red"}'

  # Discover the payload schema
  auth0 experimentation feature-flags variations create --schema
  auth0 experimentation feature-flags variations create --schema --json

  # JSON input mode (for agents and automation)
  auth0 experimentation feature-flags variations create <feature-flag-id> --data '{"name":"treatment","overrides":{"color":"red"}}'
  auth0 experimentation feature-flags variations create <feature-flag-id> --data @variation.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Schema {
				return printOperationSchema(cli, "POST", "/experimentation/feature-flags/{id}/variations")
			}

			if len(args) > 0 {
				inputs.FeatureFlagID = args[0]
			} else {
				if err := featureFlagID.Pick(cmd, &inputs.FeatureFlagID, cli.featureFlagPickerOptions); err != nil {
					return err
				}
			}

			payload, provided, err := ResolveData(cmd)
			if err != nil {
				return err
			}
			if provided {
				return createVariationFromJSON(cli, cmd, inputs.FeatureFlagID, payload)
			}

			if err := variationName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}
			if inputs.Name == "" {
				return usageError{err: errors.New("--name is required"), reason: "missing_required_flags"}
			}

			if err := variationDescription.Ask(cmd, &inputs.Description, nil); err != nil {
				return err
			}
			if err := validateMinLength(variationDescription.LongForm, inputs.Description, experimentationMinDescriptionLength); err != nil {
				return err
			}

			if err := variationOverrides.OpenEditor(
				cmd,
				&inputs.Overrides,
				`{"param_name":"override_value"}`,
				"variation-overrides.*.json",
				cli.variationOverridesEditorHint,
			); err != nil {
				return err
			}

			if inputs.Overrides == "" {
				return usageError{err: errors.New("--overrides is required (e.g. --overrides '{\"color\":\"red\"}')"), reason: "missing_required_flags"}
			}
			var overrides management.VariationOverridesMap
			if err := json.Unmarshal([]byte(inputs.Overrides), &overrides); err != nil {
				return validationError{err: fmt.Errorf("invalid JSON for --overrides (ensure the value is quoted in your shell): %w", err), reason: "malformed_json"}
			}

			req := &management.CreateVariationRequestContent{
				Name:      inputs.Name,
				Overrides: overrides,
			}
			if inputs.Description != "" {
				req.Description = &inputs.Description
			}

			var result *management.CreateVariationResponseContent
			if err := ansi.Waiting(func() (err error) {
				result, err = cli.apiv3.Variations.Create(cmd.Context(), inputs.FeatureFlagID, req)
				return err
			}); err != nil {
				return fmt.Errorf("failed to create variation: %w", err)
			}

			return cli.renderer.VariationCreate(result)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	variationName.RegisterString(cmd, &inputs.Name, "")
	variationDescription.RegisterString(cmd, &inputs.Description, "")
	variationOverrides.RegisterString(cmd, &inputs.Overrides, "")
	dataFlag.RegisterString(cmd, &inputs.Data, "")
	schemaFlag.RegisterBool(cmd, &inputs.Schema, false)
	markDataExclusive(cmd)

	return cmd
}

func updateVariationCmd(cli *cli) *cobra.Command {
	var inputs struct {
		FeatureFlagID string
		VariationID   string
		Name          string
		Description   string
		Overrides     string
		Data          string
		Schema        bool
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(2),
		Short: "Update a variation",
		Long: "Update a variation.\n\n" +
			"To update interactively, use `auth0 experimentation feature-flags variations update` with no arguments.\n\n" +
			"To update non-interactively, supply the IDs and fields to change through the flags." + experimentationDataHelp,
		Example: `  auth0 experimentation feature-flags variations update
  auth0 experimentation feature-flags variations update <feature-flag-id> <variation-id>
  auth0 experimentation feature-flags variations update <feature-flag-id> <variation-id> --name "new-name"

  # Discover the payload schema
  auth0 experimentation feature-flags variations update --schema
  auth0 experimentation feature-flags variations update --schema --json

  # JSON input mode (for agents and automation)
  auth0 experimentation feature-flags variations update <feature-flag-id> <variation-id> --data '{"name":"new-name"}'
  auth0 experimentation feature-flags variations update <feature-flag-id> <variation-id> --data @variation.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Schema {
				return printOperationSchema(cli, "PATCH", "/experimentation/feature-flags/{id}/variations/{vid}")
			}

			if len(args) > 0 {
				inputs.FeatureFlagID = args[0]
			}
			if len(args) > 1 {
				inputs.VariationID = args[1]
			}

			if inputs.FeatureFlagID == "" {
				if err := featureFlagID.Pick(cmd, &inputs.FeatureFlagID, cli.featureFlagPickerOptions); err != nil {
					return err
				}
			}

			if inputs.VariationID == "" {
				if err := variationID.Pick(cmd, &inputs.VariationID, cli.variationPickerOptions(inputs.FeatureFlagID)); err != nil {
					return err
				}
			}

			payload, provided, err := ResolveData(cmd)
			if err != nil {
				return err
			}
			if provided {
				return updateVariationFromJSON(cli, cmd, inputs.FeatureFlagID, inputs.VariationID, payload)
			}

			// Read the current variation so untouched fields keep their existing
			// value and only changed fields are sent.
			var current *management.GetVariationResponseContent
			if err := ansi.Waiting(func() (err error) {
				current, err = cli.apiv3.Variations.Get(cmd.Context(), inputs.FeatureFlagID, inputs.VariationID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to get variation %q: %w", inputs.VariationID, err)
			}

			currentName := current.GetName()
			currentDescription := current.GetDescription()

			if !variationName.IsSet(cmd) {
				inputs.Name = currentName
			}
			if !variationDescription.IsSet(cmd) {
				inputs.Description = currentDescription
			}

			if err := variationName.AskU(cmd, &inputs.Name, &currentName); err != nil {
				return err
			}
			if err := variationDescription.AskU(cmd, &inputs.Description, &currentDescription); err != nil {
				return err
			}

			// Open the editor pre-filled with the current overrides (same as
			// create), unless supplied via the flag. Leaving it unchanged keeps
			// the overrides as-is.
			currentOverrides := marshalToJSON(current.GetOverrides())
			if !variationOverrides.IsSet(cmd) {
				if err := variationOverrides.OpenEditorU(
					cmd,
					&inputs.Overrides,
					currentOverrides,
					"variation-overrides.*.json",
				); err != nil {
					return err
				}
			}

			// Diff scalar fields against the current variation; overrides are
			// sent only when they actually changed. The API merges them into the
			// existing overrides, so a key can be added or changed but not removed.
			req := &management.UpdateVariationRequestContent{}
			updated := false

			if inputs.Name != currentName {
				req.Name = &inputs.Name
				updated = true
			}
			if inputs.Description != currentDescription {
				// The API rejects an empty description; clearing it means sending null.
				if inputs.Description == "" {
					req.SetDescription(nil)
				} else {
					req.Description = &inputs.Description
				}
				updated = true
			}
			if inputs.Overrides != "" && inputs.Overrides != currentOverrides {
				var overrides management.UpdateVariationOverridesMap
				if err := json.Unmarshal([]byte(inputs.Overrides), &overrides); err != nil {
					return validationError{err: fmt.Errorf("invalid JSON for --overrides (ensure the value is quoted in your shell): %w", err), reason: "malformed_json"}
				}
				if removed := removedOverrideKeys(current.GetOverrides(), overrides); len(removed) > 0 {
					return usageError{
						err: fmt.Errorf(
							"override keys can't be removed from an existing variation (%s); "+
								"delete the variation and create it again without them",
							strings.Join(removed, ", "),
						),
						reason: "invalid_flag_value",
					}
				}
				req.Overrides = &overrides
				updated = true
			}

			if !updated {
				return usageError{err: errors.New("nothing to update: provide at least one flag or --data"), reason: "missing_required_flags"}
			}

			var result *management.UpdateVariationResponseContent
			if err := ansi.Waiting(func() (err error) {
				result, err = cli.apiv3.Variations.Update(cmd.Context(), inputs.FeatureFlagID, inputs.VariationID, req)
				return err
			}); err != nil {
				return fmt.Errorf("failed to update variation %q: %w", inputs.VariationID, err)
			}

			return cli.renderer.VariationUpdate(result)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	variationName.RegisterStringU(cmd, &inputs.Name, "")
	variationDescription.RegisterStringU(cmd, &inputs.Description, "")
	variationOverrides.RegisterStringU(cmd, &inputs.Overrides, "")
	dataFlag.RegisterString(cmd, &inputs.Data, "")
	schemaFlag.RegisterBool(cmd, &inputs.Schema, false)
	markDataExclusive(cmd)

	return cmd
}

func createVariationFromJSON(cli *cli, cmd *cobra.Command, featureFlagID, dataStr string) error {
	result, err := runJSONWrite[management.CreateVariationResponseContent](cli, cmd, jsonWriteSpec{
		Method:     http.MethodPost,
		SchemaPath: "/experimentation/feature-flags/{id}/variations",
		URI:        cli.api.HTTPClient.URI("experimentation", "feature-flags", featureFlagID, "variations"),
		Data:       dataStr,
		SchemaCmd:  "auth0 experimentation feature-flags variations create",
	})
	if err != nil {
		return fmt.Errorf("failed to create variation: %w", err)
	}

	return cli.renderer.VariationCreate(result)
}

func updateVariationFromJSON(cli *cli, cmd *cobra.Command, featureFlagID, id, dataStr string) error {
	result, err := runJSONWrite[management.UpdateVariationResponseContent](cli, cmd, jsonWriteSpec{
		Method:     http.MethodPatch,
		SchemaPath: "/experimentation/feature-flags/{id}/variations/{vid}",
		URI:        cli.api.HTTPClient.URI("experimentation", "feature-flags", featureFlagID, "variations", id),
		Data:       dataStr,
		SchemaCmd:  "auth0 experimentation feature-flags variations update",
	})
	if err != nil {
		return fmt.Errorf("failed to update variation %q: %w", id, err)
	}

	return cli.renderer.VariationUpdate(result)
}

func deleteVariationCmd(cli *cli) *cobra.Command {
	var inputs struct {
		FeatureFlagID string
		VariationIDs  []string
	}

	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Short:   "Delete one or more variations",
		Long: "Delete one or more variations of a feature flag.\n\n" +
			"To delete interactively, use `auth0 experimentation feature-flags variations delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the feature flag ID, one or more variation IDs, and `--force` to skip confirmation.",
		Example: `  auth0 experimentation feature-flags variations delete
  auth0 experimentation feature-flags variations delete <feature-flag-id>
  auth0 experimentation feature-flags variations delete <feature-flag-id> <variation-id>
  auth0 experimentation feature-flags variations delete <feature-flag-id> <variation-id> --force
  auth0 experimentation feature-flags variations delete <feature-flag-id> <variation-id> <variation-id2> <variation-idn> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) >= 1 {
				inputs.FeatureFlagID = args[0]
			}
			if len(args) >= 2 {
				inputs.VariationIDs = args[1:]
			}

			if inputs.FeatureFlagID == "" {
				if err := featureFlagID.Pick(cmd, &inputs.FeatureFlagID, cli.featureFlagPickerOptions); err != nil {
					return err
				}
			}

			if len(inputs.VariationIDs) == 0 {
				if err := variationID.PickMany(cmd, &inputs.VariationIDs, cli.variationPickerOptions(inputs.FeatureFlagID)); err != nil {
					return err
				}
				if len(inputs.VariationIDs) == 0 {
					return nil
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

			// Deletes continue past individual failures so one rejected variation
			// (e.g. the last one on a non-archived flag) doesn't block the rest.
			if err := ansi.ProgressBar("Deleting variation(s)", inputs.VariationIDs, func(_ int, id string) error {
				if id != "" {
					// Delete returns 204 for an unknown ID, so check that it exists first.
					if _, err := cli.apiv3.Variations.Get(cmd.Context(), inputs.FeatureFlagID, id); err != nil {
						return fmt.Errorf("failed to delete variation %q: %w", id, err)
					}

					if err := cli.apiv3.Variations.Delete(cmd.Context(), inputs.FeatureFlagID, id); err != nil {
						return fmt.Errorf("failed to delete variation %q: %w", id, err)
					}
				}
				return nil
			}); err != nil {
				return err
			}

			if len(inputs.VariationIDs) == 1 {
				cli.renderer.Infof("Variation %s deleted.", ansi.Faint(inputs.VariationIDs[0]))
			} else {
				cli.renderer.Infof("%d variations deleted.", len(inputs.VariationIDs))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

// Picker helpers.

func (c *cli) featureFlagPickerOptions(ctx context.Context) (pickerOptions, error) {
	all, err := collectV3Pages(ctx, 0,
		func(ctx context.Context) (*auth0.FeatureFlagListPage, error) {
			return c.apiv3.FeatureFlags.List(ctx, &management.ListFeatureFlagsRequestParameters{})
		})
	if err != nil {
		return nil, err
	}

	var opts pickerOptions
	for _, ff := range all {
		label := fmt.Sprintf("%s %s", ff.GetName(), ansi.Faint("("+ff.GetID()+")"))
		opts = append(opts, pickerOption{value: ff.GetID(), label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("no feature flags available. Create one by running: `auth0 experimentation feature-flags create`")
	}

	return opts, nil
}

func (c *cli) variationPickerOptions(featureFlagID string) func(ctx context.Context) (pickerOptions, error) {
	return func(ctx context.Context) (pickerOptions, error) {
		result, err := c.apiv3.Variations.List(ctx, featureFlagID)
		if err != nil {
			return nil, err
		}

		var opts pickerOptions
		for _, v := range result.GetVariations() {
			label := fmt.Sprintf("%s %s", v.GetName(), ansi.Faint("("+v.GetID()+")"))
			opts = append(opts, pickerOption{value: v.GetID(), label: label})
		}

		if len(opts) == 0 {
			return nil, fmt.Errorf("no variations for feature flag %q. Create one by running: `auth0 experimentation feature-flags variations create %s`", featureFlagID, featureFlagID)
		}

		return opts, nil
	}
}

// Editor hints.

func (c *cli) featureFlagParamsEditorHint() {
	c.renderer.Infof("Define parameters as a JSON object. Each key is the parameter name.")
	c.renderer.Infof(`Supported types: "string", "boolean", "number", "array", "object"`)
	c.renderer.Infof(`Example: {"color":{"type":"string","value":"blue"},"enabled":{"type":"boolean","value":false}}`)
}

// removedOverrideKeys returns, sorted, the keys of current that are missing from
// updated. The API merges overrides on update, so such keys would silently stay.
func removedOverrideKeys(current, updated map[string]any) []string {
	var removed []string
	for key := range current {
		if _, ok := updated[key]; !ok {
			removed = append(removed, key)
		}
	}
	slices.Sort(removed)
	return removed
}

func (c *cli) variationOverridesEditorHint() {
	c.renderer.Infof("Define overrides as a JSON object. Keys must match the feature flag's parameter names, mapped directly to the override value.")
	c.renderer.Infof(`Example: {"color":"red","enabled":true}`)
}
