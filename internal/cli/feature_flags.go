package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/auth0/go-auth0/v2/management"
	managementcore "github.com/auth0/go-auth0/v2/management/core"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
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
		Name:      "Description",
		LongForm:  "description",
		ShortForm: "d",
		Help:      "Description of the variation.",
	}

	variationOverrides = Flag{
		Name:       "Overrides",
		LongForm:   "overrides",
		ShortForm:  "o",
		Help:       `Parameter overrides as JSON. Example: '{"color":{"value":"red"}}'`,
		IsRequired: true,
	}

	featureFlagStatus = Flag{
		Name:     "Status",
		LongForm: "status",
		Help:     "Transition the feature flag to a new status (active, archived).",
	}
)

func featureFlagsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feature-flags",
		Short: "Manage experimentation feature flags",
		Long:  "Feature flags define named parameters (string, boolean, number) that experiments vary across user groups.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listFeatureFlagsCmd(cli))
	cmd.AddCommand(createFeatureFlagCmd(cli))
	cmd.AddCommand(showFeatureFlagCmd(cli))
	cmd.AddCommand(updateFeatureFlagCmd(cli))
	cmd.AddCommand(deleteFeatureFlagCmd(cli))
	cmd.AddCommand(statusFeatureFlagCmd(cli))
	cmd.AddCommand(variationsCmd(cli))

	return cmd
}

func listFeatureFlagsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Status string
		Type   string
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your feature flags",
		Long:    "List all feature flags. To create one, run: `auth0 experimentation feature-flags create`.",
		Example: `  auth0 experimentation feature-flags list
  auth0 experimentation feature-flags ls
  auth0 experimentation feature-flags list --json
  auth0 experimentation feature-flags list --status active`,
		RunE: func(cmd *cobra.Command, args []string) error {
			req := &management.ListFeatureFlagsRequestParameters{}
			if inputs.Status != "" {
				s := management.FeatureFlagStatusEnum(inputs.Status)
				req.Status = &s
			}
			if inputs.Type != "" {
				t := management.FeatureFlagTypeEnum(inputs.Type)
				req.Type = &t
			}

			var allFlags []*management.FeatureFlag

			if err := ansi.Waiting(func() error {
				page, err := cli.apiv2.FeatureFlags.List(cmd.Context(), req)
				if err != nil {
					return err
				}
				allFlags = append(allFlags, page.Results...)
				for {
					next, err := page.GetNextPage(cmd.Context())
					if errors.Is(err, managementcore.ErrNoPages) {
						break
					}
					if err != nil {
						return err
					}
					allFlags = append(allFlags, next.Results...)
					page = next
				}
				return nil
			}); err != nil {
				return fmt.Errorf("failed to list feature flags: %w", err)
			}

			cli.renderer.FeatureFlagList(allFlags)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.Flags().StringVar(&inputs.Status, "status", "", "Filter by status (draft, active, archived).")
	cmd.Flags().StringVar(&inputs.Type, "type", "", "Filter by type (auth0, self).")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

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
				ff, err = cli.apiv2.FeatureFlags.Get(cmd.Context(), inputs.ID)
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
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new feature flag",
		Long: "Create a new feature flag.\n\n" +
			"To create interactively, use `auth0 experimentation feature-flags create` with no flags.\n\n" +
			"To create non-interactively, supply name and parameters through the flags.",
		Example: `  auth0 experimentation feature-flags create
  auth0 experimentation feature-flags create --name "dark-mode" --parameters '{"enabled":{"type":"boolean","value":false}}'
  auth0 experimentation feature-flags create -n "checkout-flow" -p '{"variant":{"type":"string","value":"control"}}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := featureFlagName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			if err := featureFlagDescription.Ask(cmd, &inputs.Description, nil); err != nil {
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
				return fmt.Errorf("--parameters is required (e.g. --parameters '{\"color\":{\"type\":\"string\",\"value\":\"blue\"}}')")
			}
			var params management.CreateFeatureFlagParameters
			if err := json.Unmarshal([]byte(inputs.Parameters), &params); err != nil {
				return fmt.Errorf("invalid JSON for --parameters (ensure the value is quoted in your shell): %w", err)
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
				result, err = cli.apiv2.FeatureFlags.Create(cmd.Context(), req)
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

	return cmd
}

func updateFeatureFlagCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID          string
		Name        string
		Description string
		Parameters  string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update a feature flag",
		Long: "Update a feature flag.\n\n" +
			"To update interactively, use `auth0 experimentation feature-flags update` with no arguments.\n\n" +
			"To update non-interactively, supply the feature flag ID and fields to change through the flags.",
		Example: `  auth0 experimentation feature-flags update
  auth0 experimentation feature-flags update <feature-flag-id>
  auth0 experimentation feature-flags update <feature-flag-id> --name "new-name"
  auth0 experimentation feature-flags update <feature-flag-id> --parameters '{"enabled":{"type":"boolean","value":true}}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := featureFlagID.Pick(cmd, &inputs.ID, cli.featureFlagPickerOptions); err != nil {
					return err
				}
			}

			// Read the current feature flag so untouched fields keep their existing
			// value and only changed fields are sent.
			var current *management.GetFeatureFlagResponseContent
			if err := ansi.Waiting(func() (err error) {
				current, err = cli.apiv2.FeatureFlags.Get(cmd.Context(), inputs.ID)
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

			// Diff scalar fields against the current flag; parameters are replaced
			// wholesale only when they actually changed.
			req := &management.UpdateFeatureFlagRequestContent{}
			updated := false

			if inputs.Name != currentName {
				req.Name = &inputs.Name
				updated = true
			}
			if inputs.Description != currentDescription {
				req.Description = &inputs.Description
				updated = true
			}
			if inputs.Parameters != "" && inputs.Parameters != currentParameters {
				var params management.UpdateFeatureFlagParameters
				if err := json.Unmarshal([]byte(inputs.Parameters), &params); err != nil {
					return fmt.Errorf("invalid JSON for --parameters (ensure the value is quoted in your shell): %w", err)
				}
				req.Parameters = &params
				updated = true
			}

			if !updated {
				return fmt.Errorf("nothing to update — provide at least one flag")
			}

			var result *management.UpdateFeatureFlagResponseContent
			if err := ansi.Waiting(func() (err error) {
				result, err = cli.apiv2.FeatureFlags.Update(cmd.Context(), inputs.ID, req)
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

	return cmd
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

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			return ansi.ProgressBar("Deleting feature flag(s)", ids, func(_ int, id string) error {
				if id != "" {
					if err := cli.apiv2.FeatureFlags.Delete(cmd.Context(), id); err != nil {
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

// resolveStatusTarget fills id and status for a `status` command: each is taken from a positional arg, else an interactive picker, then status is validated against validStatuses.
func resolveStatusTarget(cmd *cobra.Command, args []string, idArg *Argument, picker pickerOptionsFunc, statusFlag *Flag, validStatuses []string, id, status *string) error {
	if len(args) > 0 {
		*id = args[0]
	} else if err := idArg.Pick(cmd, id, picker); err != nil {
		return err
	}

	switch {
	case len(args) > 1:
		*status = args[1]
	case canPrompt(cmd):
		if err := statusFlag.Select(cmd, status, validStatuses, nil); err != nil {
			return err
		}
	default:
		return fmt.Errorf("a target status is required (one of: %s)", strings.Join(validStatuses, ", "))
	}

	if !slices.Contains(validStatuses, *status) {
		return fmt.Errorf("invalid status %q: must be one of %s", *status, strings.Join(validStatuses, ", "))
	}

	return nil
}

// statusFeatureFlagCmd transitions a feature flag to a target status (active or archived), confirming the irreversible archive.
func statusFeatureFlagCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID     string
		Status string
	}

	// ValidStatuses are the states a feature flag can be transitioned to (draft is the initial state only).
	validStatuses := []string{"active", "archived"}

	cmd := &cobra.Command{
		Use:   "status",
		Args:  cobra.MaximumNArgs(2),
		Short: "Change a feature flag's status",
		Long: "Transition a feature flag to a new status: active or archived.\n\n" +
			"  • active   — activate the feature flag (from draft)\n" +
			"  • archived — archive the feature flag (irreversible)\n\n" +
			"To set the status interactively, run `auth0 experimentation feature-flags status` with no arguments.",
		Example: `  auth0 experimentation feature-flags status
  auth0 experimentation feature-flags status <feature-flag-id>
  auth0 experimentation feature-flags status <feature-flag-id> active
  auth0 experimentation feature-flags status <feature-flag-id> archived`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := resolveStatusTarget(cmd, args, &featureFlagID, cli.featureFlagPickerOptions, &featureFlagStatus, validStatuses, &inputs.ID, &inputs.Status); err != nil {
				return err
			}

			// Archiving is irreversible — confirm unless --force or non-interactive.
			if inputs.Status == "archived" && !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Archiving is irreversible. Are you sure?"); !confirmed {
					return nil
				}
			}

			status := management.FeatureFlagStatusEnum(inputs.Status)
			if err := ansi.Waiting(func() error {
				_, err := cli.apiv2.FeatureFlags.UpdateStatus(cmd.Context(), inputs.ID, &management.UpdateFeatureFlagStatusRequestContent{
					Status: status,
				})
				return err
			}); err != nil {
				return fmt.Errorf("failed to set feature flag %q to %s: %w", inputs.ID, inputs.Status, err)
			}

			cli.renderer.Infof("Feature flag %s is now %s.", ansi.Faint(inputs.ID), inputs.Status)
			return nil
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
				result, err = cli.apiv2.Variations.List(cmd.Context(), inputs.FeatureFlagID)
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
				v, err = cli.apiv2.Variations.Get(cmd.Context(), inputs.FeatureFlagID, inputs.VariationID)
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
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.MaximumNArgs(1),
		Short: "Create a new variation",
		Long: "Create a new variation for a feature flag.\n\n" +
			"To create interactively, use `auth0 experimentation feature-flags variations create` with no flags.\n\n" +
			"To create non-interactively, supply the feature flag ID, name, and overrides through the flags.",
		Example: `  auth0 experimentation feature-flags variations create
  auth0 experimentation feature-flags variations create <feature-flag-id>
  auth0 experimentation feature-flags variations create <feature-flag-id> --name "treatment" --overrides '{"color":{"value":"red"}}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.FeatureFlagID = args[0]
			} else {
				if err := featureFlagID.Pick(cmd, &inputs.FeatureFlagID, cli.featureFlagPickerOptions); err != nil {
					return err
				}
			}

			if err := variationName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			if err := variationDescription.Ask(cmd, &inputs.Description, nil); err != nil {
				return err
			}

			if err := variationOverrides.OpenEditor(
				cmd,
				&inputs.Overrides,
				`{"param_name":{"value":"override_value"}}`,
				"variation-overrides.*.json",
				cli.variationOverridesEditorHint,
			); err != nil {
				return err
			}

			if inputs.Overrides == "" {
				return fmt.Errorf("--overrides is required (e.g. --overrides '{\"color\":{\"value\":\"red\"}}')")
			}
			var overrides management.VariationOverridesMap
			if err := json.Unmarshal([]byte(inputs.Overrides), &overrides); err != nil {
				return fmt.Errorf("invalid JSON for --overrides (ensure the value is quoted in your shell): %w", err)
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
				result, err = cli.apiv2.Variations.Create(cmd.Context(), inputs.FeatureFlagID, req)
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

	return cmd
}

func updateVariationCmd(cli *cli) *cobra.Command {
	var inputs struct {
		FeatureFlagID string
		VariationID   string
		Name          string
		Description   string
		Overrides     string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(2),
		Short: "Update a variation",
		Long: "Update a variation.\n\n" +
			"To update interactively, use `auth0 experimentation feature-flags variations update` with no arguments.\n\n" +
			"To update non-interactively, supply the IDs and fields to change through the flags.",
		Example: `  auth0 experimentation feature-flags variations update
  auth0 experimentation feature-flags variations update <feature-flag-id> <variation-id>
  auth0 experimentation feature-flags variations update <feature-flag-id> <variation-id> --name "new-name"`,
		RunE: func(cmd *cobra.Command, args []string) error {
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

			// Read the current variation so untouched fields keep their existing
			// value and only changed fields are sent.
			var current *management.GetVariationResponseContent
			if err := ansi.Waiting(func() (err error) {
				current, err = cli.apiv2.Variations.Get(cmd.Context(), inputs.FeatureFlagID, inputs.VariationID)
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
			// replaced wholesale only when they actually changed.
			req := &management.UpdateVariationRequestContent{}
			updated := false

			if inputs.Name != currentName {
				req.Name = &inputs.Name
				updated = true
			}
			if inputs.Description != currentDescription {
				req.Description = &inputs.Description
				updated = true
			}
			if inputs.Overrides != "" && inputs.Overrides != currentOverrides {
				var overrides management.UpdateVariationOverridesMap
				if err := json.Unmarshal([]byte(inputs.Overrides), &overrides); err != nil {
					return fmt.Errorf("invalid JSON for --overrides (ensure the value is quoted in your shell): %w", err)
				}
				req.Overrides = &overrides
				updated = true
			}

			if !updated {
				return fmt.Errorf("nothing to update — provide at least one flag")
			}

			var result *management.UpdateVariationResponseContent
			if err := ansi.Waiting(func() (err error) {
				result, err = cli.apiv2.Variations.Update(cmd.Context(), inputs.FeatureFlagID, inputs.VariationID, req)
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

	return cmd
}

func deleteVariationCmd(cli *cli) *cobra.Command {
	var inputs struct {
		FeatureFlagID string
		VariationID   string
	}

	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Short:   "Delete a variation",
		Long: "Delete a variation.\n\n" +
			"To delete interactively, use `auth0 experimentation feature-flags variations delete` with no arguments.",
		Example: `  auth0 experimentation feature-flags variations delete
  auth0 experimentation feature-flags variations delete <feature-flag-id> <variation-id>
  auth0 experimentation feature-flags variations delete <feature-flag-id> <variation-id> --force`,
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

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Are you sure you want to proceed?"); !confirmed {
					return nil
				}
			}

			if err := ansi.Waiting(func() error {
				return cli.apiv2.Variations.Delete(cmd.Context(), inputs.FeatureFlagID, inputs.VariationID)
			}); err != nil {
				return fmt.Errorf("failed to delete variation %q: %w", inputs.VariationID, err)
			}

			cli.renderer.Infof("Variation %s deleted.", ansi.Faint(inputs.VariationID))
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

// Picker helpers.

func (c *cli) featureFlagPickerOptions(ctx context.Context) (pickerOptions, error) {
	page, err := c.apiv2.FeatureFlags.List(ctx, &management.ListFeatureFlagsRequestParameters{})
	if err != nil {
		return nil, err
	}

	var opts pickerOptions
	for _, ff := range page.Results {
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
		result, err := c.apiv2.Variations.List(ctx, featureFlagID)
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

// marshalToJSON renders a value as a compact JSON string for pre-filling an
// editor on update. It returns an empty string if the value is nil or can't be
// marshaled, so the editor simply opens blank.
func marshalToJSON(v interface{}) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

// Editor hints.

func (c *cli) featureFlagParamsEditorHint() {
	c.renderer.Infof("Define parameters as a JSON object. Each key is the parameter name.")
	c.renderer.Infof(`Supported types: "string", "boolean", "number"`)
	c.renderer.Infof(`Example: {"color":{"type":"string","value":"blue"},"enabled":{"type":"boolean","value":false}}`)
}

func (c *cli) variationOverridesEditorHint() {
	c.renderer.Infof("Define overrides as a JSON object. Keys must match the feature flag's parameter names.")
	c.renderer.Infof(`Example: {"color":{"value":"red"},"enabled":{"value":true}}`)
}
