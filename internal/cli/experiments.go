package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/auth0/go-auth0/v2/management"
	managementcore "github.com/auth0/go-auth0/v2/management/core"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
)

var (
	experimentID = Argument{
		Name: "Experiment ID",
		Help: "ID of the experiment.",
	}

	experimentName = Flag{
		Name:       "Name",
		LongForm:   "name",
		ShortForm:  "n",
		Help:       "Name of the experiment.",
		IsRequired: true,
	}

	experimentDescription = Flag{
		Name:      "Description",
		LongForm:  "description",
		ShortForm: "d",
		Help:      "Description of the experiment.",
	}

	experimentFeatureFlagID = Flag{
		Name:       "Feature Flag ID",
		LongForm:   "feature-flag-id",
		ShortForm:  "f",
		Help:       "ID of the feature flag to experiment on.",
		IsRequired: true,
	}

	experimentAuthFlow = Flag{
		Name:       "Authentication Flow",
		LongForm:   "authentication-flow",
		ShortForm:  "a",
		Help:       "Authentication flow this experiment applies to (e.g. login, signup).",
		IsRequired: true,
	}

	experimentAllocationStrategy = Flag{
		Name:       "Allocation Strategy",
		LongForm:   "allocation-strategy",
		ShortForm:  "s",
		Help:       "Allocation strategy: percentage or segment.",
		IsRequired: true,
	}

	experimentAllocations = Flag{
		Name:     "Allocations",
		LongForm: "allocations",
		Help:     "JSON array of allocation items ({variation_id, weight, is_control} for percentage, where weight is an integer percentage from 1 to 100; {variation_id, segment_id, is_control} for segment).",
	}

	experimentAssignmentConfig = Flag{
		Name:     "Assignment Config",
		LongForm: "assignment-config",
		Help:     `JSON object configuring how users are assigned to variations (e.g. '{"subject":"device"}').`,
	}

	experimentStatus = Flag{
		Name:     "Status",
		LongForm: "status",
		Help:     "Transition the experiment to a new status (active, paused, completed, archived).",
	}
)

func experimentsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "experiments",
		Short: "Manage experimentation experiments",
		Long: "Experiments run A/B tests by tying a feature flag, its variations, and traffic allocations together.\n\n" +
			"Typical workflow:\n" +
			"  1. Create a feature flag and its variations\n" +
			"  2. Optionally create segments for targeted allocation\n" +
			"  3. Create an experiment\n" +
			"  4. Validate it\n" +
			"  5. Start it",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listExperimentsCmd(cli))
	cmd.AddCommand(createExperimentCmd(cli))
	cmd.AddCommand(showExperimentCmd(cli))
	cmd.AddCommand(updateExperimentCmd(cli))
	cmd.AddCommand(deleteExperimentCmd(cli))
	cmd.AddCommand(validateExperimentCmd(cli))
	cmd.AddCommand(statusExperimentCmd(cli))

	return cmd
}

func listExperimentsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Status             string
		FeatureFlagID      string
		AuthenticationFlow string
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your experiments",
		Long:    "List all experiments. To create one, run: `auth0 experiments create`.",
		Example: `  auth0 experiments list
  auth0 experiments ls
  auth0 experiments list --json
  auth0 experiments list --status active
  auth0 experiments list --feature-flag-id <id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			req := &management.ListExperimentsRequestParameters{}
			if inputs.Status != "" {
				s := management.ExperimentStatusEnum(inputs.Status)
				req.Status = &s
			}
			if inputs.FeatureFlagID != "" {
				req.FeatureFlagID = &inputs.FeatureFlagID
			}
			if inputs.AuthenticationFlow != "" {
				req.AuthenticationFlow = &inputs.AuthenticationFlow
			}

			var allExperiments []*management.ExperimentListItem

			if err := ansi.Waiting(func() error {
				page, err := cli.apiv2.Experiments.List(cmd.Context(), req)
				if err != nil {
					return err
				}
				allExperiments = append(allExperiments, page.Results...)
				for {
					next, err := page.GetNextPage(cmd.Context())
					if errors.Is(err, managementcore.ErrNoPages) {
						break
					}
					if err != nil {
						return err
					}
					allExperiments = append(allExperiments, next.Results...)
					page = next
				}
				return nil
			}); err != nil {
				return fmt.Errorf("failed to list experiments: %w", err)
			}

			cli.renderer.ExperimentList(allExperiments)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.Flags().StringVar(&inputs.Status, "status", "", "Filter by status (draft, active, paused, completed, archived).")
	cmd.Flags().StringVar(&inputs.FeatureFlagID, "feature-flag-id", "", "Filter by feature flag ID.")
	cmd.Flags().StringVar(&inputs.AuthenticationFlow, "authentication-flow", "", "Filter by authentication flow.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	return cmd
}

func showExperimentCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show an experiment",
		Long:  "Display details about an experiment including its allocations and validation status.",
		Example: `  auth0 experiments show
  auth0 experiments show <experiment-id>
  auth0 experiments show <experiment-id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := experimentID.Pick(cmd, &inputs.ID, cli.experimentPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var exp *management.GetExperimentResponseContent
			if err := ansi.Waiting(func() (err error) {
				exp, err = cli.apiv2.Experiments.Get(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to get experiment %q: %w", inputs.ID, err)
			}

			cli.renderer.ExperimentShow(exp)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func createExperimentCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Name               string
		Description        string
		FeatureFlagID      string
		AuthenticationFlow string
		AllocationStrategy string
		Allocations        string
		AssignmentConfig   string
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new experiment",
		Long: "Create a new experiment.\n\n" +
			"To create interactively, use `auth0 experiments create` with no flags.\n\n" +
			"To create non-interactively, supply all required flags.",
		Example: `  auth0 experiments create
  auth0 experiments create --name "button-color" --feature-flag-id ff_abc --authentication-flow login --allocation-strategy percentage --assignment-config '{"subject":"device"}' --allocations '[{"variation_id":"vid_1","weight":50,"is_control":true},{"variation_id":"vid_2","weight":50,"is_control":false}]'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := experimentName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			if err := experimentDescription.Ask(cmd, &inputs.Description, nil); err != nil {
				return err
			}

			// Feature flag — interactive picker if not provided via flag.
			if inputs.FeatureFlagID == "" && canPrompt(cmd) {
				if err := experimentFeatureFlagID.Pick(cmd, &inputs.FeatureFlagID, cli.featureFlagPickerOptions); err != nil {
					return err
				}
			} else if inputs.FeatureFlagID == "" {
				return fmt.Errorf("--feature-flag-id is required")
			}

			if err := experimentAuthFlow.Ask(cmd, &inputs.AuthenticationFlow, nil); err != nil {
				return err
			}

			// Allocation strategy — dropdown.
			if inputs.AllocationStrategy == "" && canPrompt(cmd) {
				strategyOptions := []string{"percentage", "segment"}
				if err := experimentAllocationStrategy.Select(cmd, &inputs.AllocationStrategy, strategyOptions, nil); err != nil {
					return err
				}
			} else if inputs.AllocationStrategy == "" {
				return fmt.Errorf("--allocation-strategy is required")
			}

			// Allocations — build interactively from variation picker, or accept raw JSON.
			if inputs.Allocations == "" && canPrompt(cmd) {
				allocs, err := cli.buildAllocationsInteractively(cmd, inputs.FeatureFlagID, inputs.AllocationStrategy)
				if err != nil {
					return err
				}
				b, err := json.Marshal(allocs)
				if err != nil {
					return err
				}
				inputs.Allocations = string(b)
			} else if inputs.Allocations == "" {
				return fmt.Errorf("--allocations is required")
			}

			// Assignment config — required. Prompt interactively if not provided via flag.
			if inputs.AssignmentConfig == "" && canPrompt(cmd) {
				subjectOptions := []string{"device"}
				var chosen string
				if err := experimentAssignmentConfig.Select(cmd, &chosen, subjectOptions, nil); err != nil {
					return err
				}
				inputs.AssignmentConfig = fmt.Sprintf(`{"subject":%q}`, chosen)
			} else if inputs.AssignmentConfig == "" {
				return fmt.Errorf("--assignment-config is required")
			}

			var ac management.AssignmentConfig
			if err := json.Unmarshal([]byte(inputs.AssignmentConfig), &ac); err != nil {
				return fmt.Errorf("invalid JSON for --assignment-config: %w", err)
			}

			var allocations []*management.AllocationRequestItem
			if err := json.Unmarshal([]byte(inputs.Allocations), &allocations); err != nil {
				return fmt.Errorf("invalid JSON for --allocations (ensure the value is quoted in your shell): %w", err)
			}
			if err := validateAllocationWeights(allocations); err != nil {
				return err
			}

			strategy := management.AllocationStrategyEnum(inputs.AllocationStrategy)
			req := &management.CreateExperimentRequestContent{
				Name:               inputs.Name,
				FeatureFlagID:      inputs.FeatureFlagID,
				AuthenticationFlow: inputs.AuthenticationFlow,
				AllocationStrategy: strategy,
				AssignmentConfig:   &ac,
				Allocations:        allocations,
			}
			if inputs.Description != "" {
				req.Description = &inputs.Description
			}

			var result *management.CreateExperimentResponseContent
			if err := ansi.Waiting(func() (err error) {
				result, err = cli.apiv2.Experiments.Create(cmd.Context(), req)
				return err
			}); err != nil {
				return fmt.Errorf("failed to create experiment: %w", err)
			}

			return cli.renderer.ExperimentCreate(result)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	experimentName.RegisterString(cmd, &inputs.Name, "")
	experimentDescription.RegisterString(cmd, &inputs.Description, "")
	experimentFeatureFlagID.RegisterString(cmd, &inputs.FeatureFlagID, "")
	experimentAuthFlow.RegisterString(cmd, &inputs.AuthenticationFlow, "")
	experimentAllocationStrategy.RegisterString(cmd, &inputs.AllocationStrategy, "")
	experimentAllocations.RegisterString(cmd, &inputs.Allocations, "")
	experimentAssignmentConfig.RegisterString(cmd, &inputs.AssignmentConfig, "")

	return cmd
}

func updateExperimentCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID               string
		Name             string
		Description      string
		Allocations      string
		AssignmentConfig string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update an experiment",
		Long: "Update an experiment.\n\n" +
			"Note: feature flag, authentication flow, and allocation strategy cannot be changed after creation. " +
			"To change an experiment's status, use `auth0 experiments status`.\n\n" +
			"To update interactively, use `auth0 experiments update` with no arguments.",
		Example: `  auth0 experiments update
  auth0 experiments update <experiment-id>
  auth0 experiments update <experiment-id> --name "new-name"
  auth0 experiments update <experiment-id> --assignment-config '{"subject":"device"}'
  auth0 experiments update <experiment-id> --allocations '[{"variation_id":"vid","weight":100,"is_control":true}]'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := experimentID.Pick(cmd, &inputs.ID, cli.experimentPickerOptions); err != nil {
					return err
				}
			}

			if err := experimentName.AskU(cmd, &inputs.Name, nil); err != nil {
				return err
			}
			if err := experimentDescription.AskU(cmd, &inputs.Description, nil); err != nil {
				return err
			}
			if err := experimentAllocations.AskU(cmd, &inputs.Allocations, nil); err != nil {
				return err
			}

			req := &management.UpdateExperimentRequestParameters{}
			updated := false

			if inputs.Name != "" {
				req.Name = &inputs.Name
				updated = true
			}
			if inputs.Description != "" {
				req.Description = &inputs.Description
				updated = true
			}
			if inputs.AssignmentConfig != "" {
				var ac management.AssignmentConfig
				if err := json.Unmarshal([]byte(inputs.AssignmentConfig), &ac); err != nil {
					return fmt.Errorf("invalid JSON for --assignment-config: %w", err)
				}
				req.AssignmentConfig = &ac
				updated = true
			}
			if inputs.Allocations != "" {
				var allocations []*management.AllocationRequestItem
				if err := json.Unmarshal([]byte(inputs.Allocations), &allocations); err != nil {
					return fmt.Errorf("invalid JSON for --allocations: %w", err)
				}
				if err := validateAllocationWeights(allocations); err != nil {
					return err
				}
				req.Allocations = allocations
				updated = true
			}

			if !updated {
				return fmt.Errorf("nothing to update — provide at least one flag (--name, --description, --assignment-config, --allocations)")
			}

			var result *management.UpdateExperimentResponseContent
			if err := ansi.Waiting(func() (err error) {
				result, err = cli.apiv2.Experiments.Update(cmd.Context(), inputs.ID, req)
				return err
			}); err != nil {
				return fmt.Errorf("failed to update experiment %q: %w", inputs.ID, err)
			}

			return cli.renderer.ExperimentUpdate(result)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	experimentName.RegisterStringU(cmd, &inputs.Name, "")
	experimentDescription.RegisterStringU(cmd, &inputs.Description, "")
	experimentAllocations.RegisterStringU(cmd, &inputs.Allocations, "")
	experimentAssignmentConfig.RegisterStringU(cmd, &inputs.AssignmentConfig, "")

	return cmd
}

func deleteExperimentCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Short:   "Delete an experiment",
		Long: "Delete an experiment.\n\n" +
			"Active experiments must be paused or completed before deleting.\n\n" +
			"To delete non-interactively, supply the experiment ID and use `--force` to skip confirmation.",
		Example: `  auth0 experiments delete
  auth0 experiments delete <experiment-id>
  auth0 experiments delete <experiment-id> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var ids []string
			if len(args) == 0 {
				if err := experimentID.PickMany(cmd, &ids, cli.experimentPickerOptions); err != nil {
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

			return ansi.ProgressBar("Deleting experiment(s)", ids, func(_ int, id string) error {
				if id != "" {
					if err := cli.apiv2.Experiments.Delete(cmd.Context(), id); err != nil {
						return fmt.Errorf("failed to delete experiment %q: %w", id, err)
					}
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

func validateExperimentCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "validate",
		Args:  cobra.MaximumNArgs(1),
		Short: "Validate an experiment",
		Long:  "Check whether an experiment is ready to be activated. Returns validation status and any blocking errors.",
		Example: `  auth0 experiments validate
  auth0 experiments validate <experiment-id>
  auth0 experiments validate <experiment-id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := experimentID.Pick(cmd, &inputs.ID, cli.experimentPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var result *management.ValidateExperimentResponseContent
			if err := ansi.Waiting(func() (err error) {
				result, err = cli.apiv2.Experiments.Validate(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to validate experiment %q: %w", inputs.ID, err)
			}

			cli.renderer.ExperimentValidate(inputs.ID, result)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

// statusExperimentCmd transitions an experiment to a target lifecycle status (active, paused, completed, or archived).
func statusExperimentCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID     string
		Status string
	}

	// ValidStatuses are the lifecycle states an experiment can be transitioned to.
	validStatuses := []string{"active", "paused", "completed", "archived"}

	cmd := &cobra.Command{
		Use:   "status",
		Args:  cobra.MaximumNArgs(2),
		Short: "Change an experiment's status",
		Long: "Transition an experiment to a new lifecycle status: active, paused, completed, or archived.\n\n" +
			"  • active    — start (or resume) the experiment; runs full validation before activating\n" +
			"  • paused    — pause a running experiment; it can be resumed by setting it active again\n" +
			"  • completed — mark the experiment as finished; it can then be archived\n" +
			"  • archived  — archive a completed experiment\n\n" +
			"To set the status interactively, run `auth0 experiments status` with no arguments.",
		Example: `  auth0 experiments status
  auth0 experiments status <experiment-id>
  auth0 experiments status <experiment-id> active
  auth0 experiments status <experiment-id> paused`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := resolveStatusTarget(cmd, args, &experimentID, cli.experimentPickerOptions, &experimentStatus, validStatuses, &inputs.ID, &inputs.Status); err != nil {
				return err
			}

			status := management.ExperimentTransitionStatusEnum(inputs.Status)
			var result *management.UpdateExperimentStatusResponseContent
			if err := ansi.Waiting(func() (err error) {
				result, err = cli.apiv2.Experiments.UpdateStatus(cmd.Context(), inputs.ID, &management.UpdateExperimentStatusRequestContent{
					Status: status,
				})
				return err
			}); err != nil {
				return fmt.Errorf("failed to set experiment %q to %s: %w", inputs.ID, inputs.Status, err)
			}

			return cli.renderer.ExperimentStatusUpdate(result)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

// Picker helpers.

func (c *cli) experimentPickerOptions(ctx context.Context) (pickerOptions, error) {
	page, err := c.apiv2.Experiments.List(ctx, &management.ListExperimentsRequestParameters{})
	if err != nil {
		return nil, err
	}

	var opts pickerOptions
	for _, e := range page.Results {
		label := fmt.Sprintf("%s %s %s", e.GetName(), ansi.Faint("("+e.GetID()+")"), statusBadge(string(e.GetStatus())))
		opts = append(opts, pickerOption{value: e.GetID(), label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("no experiments available. Create one by running: `auth0 experiments create`")
	}

	return opts, nil
}

func statusBadge(status string) string {
	switch status {
	case "active":
		return ansi.Green("[active]")
	case "paused":
		return ansi.Yellow("[paused]")
	case "draft":
		return ansi.Yellow("[draft]")
	case "completed", "archived":
		return ansi.Faint("[" + status + "]")
	default:
		return "[" + status + "]"
	}
}

// buildAllocationsInteractively prompts for each variation's weight or segment and returns the assembled allocations.
func (c *cli) buildAllocationsInteractively(cmd *cobra.Command, featureFlagID string, strategy string) ([]*management.AllocationRequestItem, error) {
	ctx := cmd.Context()
	variations, err := c.apiv2.Variations.List(ctx, featureFlagID)
	if err != nil {
		return nil, fmt.Errorf("failed to list variations: %w", err)
	}

	if len(variations.GetVariations()) == 0 {
		return nil, fmt.Errorf("no variations found for feature flag %q. Create some first with `auth0 feature-flags variations create %s`", featureFlagID, featureFlagID)
	}

	c.renderer.Infof("Found %d variation(s). You will be prompted to configure each one.", len(variations.GetVariations()))
	c.renderer.Newline()

	var allocations []*management.AllocationRequestItem

	for i, v := range variations.GetVariations() {
		c.renderer.Infof("Variation %d/%d: %s %s", i+1, len(variations.GetVariations()), v.GetName(), ansi.Faint("("+v.GetID()+")"))

		isControl := i == 0
		isControlStr := "false"
		if isControl {
			isControlStr = "true (first variation is control by default)"
		}
		c.renderer.Detailf("Is control: %s", isControlStr)

		alloc := &management.AllocationRequestItem{
			VariationID: v.GetID(),
			IsControl:   isControl,
		}

		switch strategy {
		case "percentage":
			defaultWeight := strconv.Itoa(100 / len(variations.GetVariations()))
			var weightStr string
			q := prompt.TextInput(
				"weight",
				fmt.Sprintf("Weight for %q (1–100)", v.GetName()),
				"Integer percentage of traffic assigned to this variation (1–100).",
				defaultWeight,
				true,
			)
			if err := prompt.AskOne(q, &weightStr); err != nil {
				return nil, err
			}
			weightInt, err := strconv.Atoi(weightStr)
			if err != nil {
				return nil, fmt.Errorf("invalid weight %q: must be a whole number", weightStr)
			}
			if weightInt < 1 || weightInt > 100 {
				return nil, fmt.Errorf("invalid weight %d: must be between 1 and 100", weightInt)
			}
			weight := float64(weightInt)
			alloc.Weight = &weight
		case "segment":
			// Segment_id is optional — fetch available segments and offer a picker
			// with a "No segment" escape hatch. If no segments exist at all, skip silently.
			segOpts, err := c.segmentPickerOptions(ctx)
			if err != nil {
				// No segments exist — warn and continue without assigning one.
				c.renderer.Warnf("No segments available for variation %q (segment_id left unset). Create segments with `auth0 segments create`.", v.GetName())
			} else {
				// Prepend a skip option so the user can leave segment_id blank.
				skipLabel := "No segment (unassigned)"
				labels := append([]string{skipLabel}, segOpts.labels()...)
				selectFlag := Flag{Name: "Segment", LongForm: "segment"}
				var chosen string
				if err := selectFlag.Select(cmd, &chosen, labels, &skipLabel); err != nil {
					return nil, err
				}
				if chosen != skipLabel {
					sid := segOpts.getValue(chosen)
					alloc.SegmentID = &sid
				}
			}
		}

		allocations = append(allocations, alloc)
	}

	return allocations, nil
}

// validateAllocationWeights enforces that each --allocations JSON weight is a percentage between 1 and 100.
func validateAllocationWeights(allocations []*management.AllocationRequestItem) error {
	for _, a := range allocations {
		if a.Weight != nil && (*a.Weight < 1 || *a.Weight > 100) {
			return fmt.Errorf("invalid weight %g: must be between 1 and 100", *a.Weight)
		}
	}
	return nil
}
