package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	management "github.com/auth0/go-auth0/v2/management"
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
	cmd.AddCommand(activateFeatureFlagCmd(cli))
	cmd.AddCommand(archiveFeatureFlagCmd(cli))
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
		Long:    "List all feature flags. To create one, run: `auth0 feature-flags create`.",
		Example: `  auth0 feature-flags list
  auth0 feature-flags ls
  auth0 feature-flags list --json
  auth0 feature-flags list --status active`,
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
		Example: `  auth0 feature-flags show
  auth0 feature-flags show <feature-flag-id>
  auth0 feature-flags show <feature-flag-id> --json`,
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
			"To create interactively, use `auth0 feature-flags create` with no flags.\n\n" +
			"To create non-interactively, supply name and parameters through the flags.",
		Example: `  auth0 feature-flags create
  auth0 feature-flags create --name "dark-mode" --parameters '{"enabled":{"type":"boolean","value":false}}'
  auth0 feature-flags create -n "checkout-flow" -p '{"variant":{"type":"string","value":"control"}}'`,
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
			"To update interactively, use `auth0 feature-flags update` with no arguments.\n\n" +
			"To update non-interactively, supply the feature flag ID and fields to change through the flags.",
		Example: `  auth0 feature-flags update
  auth0 feature-flags update <feature-flag-id>
  auth0 feature-flags update <feature-flag-id> --name "new-name"
  auth0 feature-flags update <feature-flag-id> --parameters '{"enabled":{"type":"boolean","value":true}}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := featureFlagID.Pick(cmd, &inputs.ID, cli.featureFlagPickerOptions); err != nil {
					return err
				}
			}

			if err := featureFlagName.AskU(cmd, &inputs.Name, nil); err != nil {
				return err
			}
			if err := featureFlagDescription.AskU(cmd, &inputs.Description, nil); err != nil {
				return err
			}
			if err := featureFlagParameters.AskU(cmd, &inputs.Parameters, nil); err != nil {
				return err
			}

			req := &management.UpdateFeatureFlagRequestContent{}
			updated := false

			if inputs.Name != "" {
				req.Name = &inputs.Name
				updated = true
			}
			if inputs.Description != "" {
				req.Description = &inputs.Description
				updated = true
			}
			if inputs.Parameters != "" {
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
			"To delete interactively, use `auth0 feature-flags delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the feature flag ID and use `--force` to skip confirmation.",
		Example: `  auth0 feature-flags delete
  auth0 feature-flags delete <feature-flag-id>
  auth0 feature-flags delete <feature-flag-id> --force`,
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

func activateFeatureFlagCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "activate",
		Args:  cobra.MaximumNArgs(1),
		Short: "Activate a feature flag",
		Long:  "Transition a feature flag from draft to active status.",
		Example: `  auth0 feature-flags activate
  auth0 feature-flags activate <feature-flag-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := featureFlagID.Pick(cmd, &inputs.ID, cli.featureFlagPickerOptions); err != nil {
					return err
				}
			}

			status := management.FeatureFlagStatusEnumActive
			if err := ansi.Waiting(func() error {
				_, err := cli.apiv2.FeatureFlags.UpdateStatus(cmd.Context(), inputs.ID, &management.UpdateFeatureFlagStatusRequestContent{
					Status: status,
				})
				return err
			}); err != nil {
				return fmt.Errorf("failed to activate feature flag %q: %w", inputs.ID, err)
			}

			cli.renderer.Infof("Feature flag %s is now active.", ansi.Faint(inputs.ID))
			return nil
		},
	}

	return cmd
}

func archiveFeatureFlagCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "archive",
		Args:  cobra.MaximumNArgs(1),
		Short: "Archive a feature flag",
		Long:  "Transition a feature flag to archived status.",
		Example: `  auth0 feature-flags archive
  auth0 feature-flags archive <feature-flag-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := featureFlagID.Pick(cmd, &inputs.ID, cli.featureFlagPickerOptions); err != nil {
					return err
				}
			}

			if !cli.force && canPrompt(cmd) {
				if confirmed := prompt.Confirm("Archiving is irreversible. Are you sure?"); !confirmed {
					return nil
				}
			}

			status := management.FeatureFlagStatusEnumArchived
			if err := ansi.Waiting(func() error {
				_, err := cli.apiv2.FeatureFlags.UpdateStatus(cmd.Context(), inputs.ID, &management.UpdateFeatureFlagStatusRequestContent{
					Status: status,
				})
				return err
			}); err != nil {
				return fmt.Errorf("failed to archive feature flag %q: %w", inputs.ID, err)
			}

			cli.renderer.Infof("Feature flag %s has been archived.", ansi.Faint(inputs.ID))
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
		Example: `  auth0 feature-flags variations list
  auth0 feature-flags variations list <feature-flag-id>
  auth0 feature-flags variations list <feature-flag-id> --json`,
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
		Example: `  auth0 feature-flags variations show
  auth0 feature-flags variations show <feature-flag-id> <variation-id>
  auth0 feature-flags variations show <feature-flag-id> <variation-id> --json`,
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
			"To create interactively, use `auth0 feature-flags variations create` with no flags.\n\n" +
			"To create non-interactively, supply the feature flag ID, name, and overrides through the flags.",
		Example: `  auth0 feature-flags variations create
  auth0 feature-flags variations create <feature-flag-id>
  auth0 feature-flags variations create <feature-flag-id> --name "treatment" --overrides '{"color":{"value":"red"}}'`,
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
			"To update interactively, use `auth0 feature-flags variations update` with no arguments.\n\n" +
			"To update non-interactively, supply the IDs and fields to change through the flags.",
		Example: `  auth0 feature-flags variations update
  auth0 feature-flags variations update <feature-flag-id> <variation-id>
  auth0 feature-flags variations update <feature-flag-id> <variation-id> --name "new-name"`,
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

			if err := variationName.AskU(cmd, &inputs.Name, nil); err != nil {
				return err
			}
			if err := variationDescription.AskU(cmd, &inputs.Description, nil); err != nil {
				return err
			}
			if err := variationOverrides.AskU(cmd, &inputs.Overrides, nil); err != nil {
				return err
			}

			req := &management.UpdateVariationRequestContent{}
			updated := false

			if inputs.Name != "" {
				req.Name = &inputs.Name
				updated = true
			}
			if inputs.Description != "" {
				req.Description = &inputs.Description
				updated = true
			}
			if inputs.Overrides != "" {
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
			"To delete interactively, use `auth0 feature-flags variations delete` with no arguments.",
		Example: `  auth0 feature-flags variations delete
  auth0 feature-flags variations delete <feature-flag-id> <variation-id>
  auth0 feature-flags variations delete <feature-flag-id> <variation-id> --force`,
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
		return nil, errors.New("no feature flags available. Create one by running: `auth0 feature-flags create`")
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
			return nil, fmt.Errorf("no variations for feature flag %q. Create one by running: `auth0 feature-flags variations create %s`", featureFlagID, featureFlagID)
		}

		return opts, nil
	}
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
