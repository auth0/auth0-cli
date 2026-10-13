package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/auth0/go-auth0/v3/management"
	managementcore "github.com/auth0/go-auth0/v3/management/core"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
)

const (
	// Caps how many variations (allocations) a single experiment can be created with.
	maxExperimentAllocations = 20

	// Number of variations shown at once in the interactive multi-select;
	// longer lists are paged/scrolled.
	variationPickerPageSize = 10
)

// authenticationFlowOptions are the flows an experiment can target. `all`
// targets every flow at once but requires the experiment to be the only active
// one (enforced server-side).
var authenticationFlowOptions = []string{
	"authentication",
	"mfa_enrollment",
	"mfa_challenge",
	"password_reset",
	"passkey_enrollment",
	"all",
}

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
		Name:         "Description",
		LongForm:     "description",
		ShortForm:    "d",
		AlwaysPrompt: true,
		Help:         "Description of the experiment.",
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
		Help:       "Authentication flow this experiment applies to: authentication, mfa_enrollment, mfa_challenge, password_reset, passkey_enrollment, or all.",
		IsRequired: true,
	}

	experimentAllocationStrategy = Flag{
		Name:      "Allocation Strategy",
		LongForm:  "allocation-strategy",
		ShortForm: "s",
		Help:      "Allocation strategy: percentage or segment. Required when --allocations or --levels is set.",
	}

	experimentAllocations = Flag{
		Name:      "Allocations",
		LongForm:  "allocations",
		ShortForm: "A",
		Help:      "JSON array of allocation items ({variation_id, weight, is_control} for percentage, where weight is an integer percentage from 1 to 100; {variation_id, segment_id, priority, is_control, is_fallback} for segment).",
	}

	experimentDefaultConfig = Flag{
		Name:     "Default Config",
		LongForm: "default-config",
		Help:     "For Auth0-managed flags only: where non-overridden config keys resolve from — 'tenant' (inherit the tenant's live config) or 'flag' (use the flag's frozen defaults). Rejected for customer-defined flags.",
	}

	experimentLevels = Flag{
		Name:     "Levels",
		LongForm: "levels",
		Help:     "Ramp exposure levels as a comma-separated, strictly-increasing list of integer percentages in [0, 100] (e.g. 10,25,50,100). Requires --allocation-strategy percentage, and the allocations must omit weights — traffic is controlled by the level schedule, not per-variation weights.",
	}

	experimentStatus = Flag{
		Name:     "Status",
		LongForm: "status",
		Help:     "Transition the experiment to a new status (active, paused, completed, archived).",
	}
)

// experimentStatuses are the lifecycle states an experiment can be transitioned to.
var experimentStatuses = []string{"active", "paused", "completed", "archived"}

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
	cmd.AddCommand(advanceRampExperimentCmd(cli))

	return cmd
}

func listExperimentsCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Status             string
		FeatureFlagID      string
		AuthenticationFlow string
		Schema             bool
		Query              string
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your experiments",
		Long:    "List all experiments. To create one, run: `auth0 experimentation experiments create`." + experimentationQueryHelp,
		Example: `  auth0 experimentation experiments list
  auth0 experimentation experiments ls
  auth0 experimentation experiments list --json
  auth0 experimentation experiments list --status active
  auth0 experimentation experiments list --feature-flag-id <id>
  auth0 experimentation experiments list --schema
  auth0 experimentation experiments list --schema --json
  auth0 experimentation experiments list --query '{"status":"active","take":10}'
  auth0 experimentation experiments list --query '{"status":"active"}' --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Schema {
				return printOperationSchema(cli, "GET", "/experimentation/experiments")
			}

			if inputs.Query != "" {
				return runJSONQuery(cli, cmd, jsonQuerySpec{
					Path:      "experimentation/experiments",
					SchemaCmd: "auth0 experimentation experiments list",
				}, inputs.Query)
			}

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
				page, err := cli.apiv3.Experiments.List(cmd.Context(), req)
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
	schemaFlag.RegisterBool(cmd, &inputs.Schema, false)
	listQueryFlag.RegisterString(cmd, &inputs.Query, "")
	markQueryExclusive(cmd, "status", "feature-flag-id", "authentication-flow")

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
		Long:  "Display details about an experiment including its allocations, ramp levels, and validation status.",
		Example: `  auth0 experimentation experiments show
  auth0 experimentation experiments show <experiment-id>
  auth0 experimentation experiments show <experiment-id> --json`,
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
				exp, err = cli.apiv3.Experiments.Get(cmd.Context(), inputs.ID)
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
		DefaultConfig      string
		Levels             string
		Data               string
		Schema             bool
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new experiment",
		Long: "Create a new experiment.\n\n" +
			"To create interactively, use `auth0 experimentation experiments create` with no flags.\n\n" +
			"To create non-interactively, supply all required flags.\n\n" +
			"The allocation strategy and allocations are optional: omit them to create a draft and add the " +
			"allocations later with `auth0 experimentation experiments update`." + experimentationDataHelp,
		Example: `  auth0 experimentation experiments create
  auth0 experimentation experiments create --name "draft-test" --feature-flag-id ff_abc --authentication-flow authentication
  auth0 experimentation experiments create --name "button-color" --feature-flag-id ff_abc --authentication-flow authentication --allocation-strategy percentage --allocations '[{"variation_id":"vid_1","weight":50,"is_control":true},{"variation_id":"vid_2","weight":50,"is_control":false}]'
  auth0 experimentation experiments create -n "gradual-rollout" -f ff_abc -a authentication -s percentage --levels 10,25,50,100 --allocations '[{"variation_id":"vid_1","is_control":true},{"variation_id":"vid_2","is_control":false}]'
  auth0 experimentation experiments create -n "by-region" -f ff_abc -a authentication -s segment --allocations '[{"variation_id":"vid_1","is_control":true,"is_fallback":true},{"variation_id":"vid_2","segment_id":"seg_abc","priority":1,"is_control":false}]'

  # Discover the payload schema
  auth0 experimentation experiments create --schema
  auth0 experimentation experiments create --schema --json

  # JSON input mode (for agents and automation)
  auth0 experimentation experiments create --data '{"name":"draft-test","feature_flag_id":"ff_abc","authentication_flow":"authentication"}'
  auth0 experimentation experiments create --data @experiment.json
  cat experiment.json | auth0 experimentation experiments create`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Schema {
				return printOperationSchema(cli, "POST", "/experimentation/experiments")
			}

			payload, provided, err := ResolveData(cmd)
			if err != nil {
				return err
			}
			if provided {
				return createExperimentFromJSON(cli, cmd, payload)
			}

			if err := experimentName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}
			if inputs.Name == "" {
				return usageError{err: errors.New("--name is required"), reason: "missing_required_flags"}
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
				return usageError{err: errors.New("--feature-flag-id is required"), reason: "missing_required_flags"}
			}

			// Authentication flow — dropdown of the supported flows.
			if inputs.AuthenticationFlow == "" && canPrompt(cmd) {
				if err := experimentAuthFlow.Select(cmd, &inputs.AuthenticationFlow, authenticationFlowOptions, nil); err != nil {
					return err
				}
			} else if inputs.AuthenticationFlow == "" {
				return usageError{err: errors.New("--authentication-flow is required"), reason: "missing_required_flags"}
			}

			// Allocation strategy — optional, as the API accepts a draft without
			// one, so (when not supplied via a flag) offer it behind a confirmation.
			if inputs.AllocationStrategy == "" && canPrompt(cmd) && prompt.ConfirmWithDefault("Set the allocation strategy now?", true) {
				strategyOptions := []string{"percentage", "segment"}
				if err := experimentAllocationStrategy.Select(cmd, &inputs.AllocationStrategy, strategyOptions, nil); err != nil {
					return err
				}
			}

			// Ramp levels decide whether allocations carry weights (a ramp's traffic
			// follows its level schedule), so settle them before the allocations.
			// Ramps require the percentage strategy, so only offer them there.
			if inputs.Levels == "" && inputs.AllocationStrategy == "percentage" && canPrompt(cmd) && prompt.Confirm("Configure ramp levels?") {
				if err := experimentLevels.Ask(cmd, &inputs.Levels, nil); err != nil {
					return err
				}
			}

			// Allocations — optional; build interactively from the variation picker
			// behind a confirmation, or accept raw JSON. They need a strategy, so
			// only offer them once one is set.
			if inputs.Allocations == "" && inputs.AllocationStrategy != "" && canPrompt(cmd) && prompt.ConfirmWithDefault("Configure the traffic allocations now?", true) {
				allocs, err := cli.buildAllocationsInteractively(cmd, inputs.FeatureFlagID, inputs.AllocationStrategy, inputs.Levels != "")
				if err != nil {
					return err
				}
				b, err := json.Marshal(allocs)
				if err != nil {
					return err
				}
				inputs.Allocations = string(b)
			}

			var allocations []*management.AllocationRequestItem
			if inputs.Allocations != "" {
				if inputs.AllocationStrategy == "" {
					return usageError{err: errors.New("--allocations requires --allocation-strategy"), reason: "missing_required_flags"}
				}
				if err := json.Unmarshal([]byte(inputs.Allocations), &allocations); err != nil {
					return validationError{err: fmt.Errorf("invalid JSON for --allocations (ensure the value is quoted in your shell): %w", err), reason: "malformed_json"}
				}
				if err := validateAllocationCount(allocations); err != nil {
					return err
				}
				if err := validateAllocationWeights(allocations); err != nil {
					return err
				}
			}
			if inputs.Levels != "" {
				// Report a malformed schedule before the rules that depend on it.
				if _, err := parseLevels(inputs.Levels); err != nil {
					return err
				}
				if err := validateRampAllocations(inputs.AllocationStrategy, allocations); err != nil {
					return err
				}
			}

			// Default config only applies to Auth0-managed flags, so offer it behind
			// a confirmation when not supplied via a flag.
			if inputs.DefaultConfig == "" && canPrompt(cmd) && prompt.Confirm("Set a default config (Auth0-managed flags only)?") {
				if err := experimentDefaultConfig.Select(cmd, &inputs.DefaultConfig, []string{"tenant", "flag"}, nil); err != nil {
					return err
				}
			}

			req := &management.CreateExperimentRequestContent{
				Name:               inputs.Name,
				FeatureFlagID:      inputs.FeatureFlagID,
				AuthenticationFlow: management.AuthenticationFlowEnum(inputs.AuthenticationFlow),
				Allocations:        allocations,
			}
			if inputs.AllocationStrategy != "" {
				strategy := management.AllocationStrategyEnum(inputs.AllocationStrategy)
				req.AllocationStrategy = &strategy
			}
			if inputs.Description != "" {
				req.Description = &inputs.Description
			}
			if inputs.DefaultConfig != "" {
				dc, err := parseDefaultConfig(inputs.DefaultConfig)
				if err != nil {
					return err
				}
				req.DefaultConfig = &dc
			}
			if inputs.Levels != "" {
				levels, err := parseLevels(inputs.Levels)
				if err != nil {
					return err
				}
				req.Levels = levels
			}

			var result *management.CreateExperimentResponseContent
			if err := ansi.Waiting(func() (err error) {
				result, err = cli.apiv3.Experiments.Create(cmd.Context(), req)
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
	experimentDefaultConfig.RegisterString(cmd, &inputs.DefaultConfig, "")
	experimentLevels.RegisterString(cmd, &inputs.Levels, "")
	dataFlag.RegisterString(cmd, &inputs.Data, "")
	schemaFlag.RegisterBool(cmd, &inputs.Schema, false)
	markDataExclusive(cmd)

	return cmd
}

func updateExperimentCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID                 string
		Name               string
		Description        string
		AuthenticationFlow string
		Allocations        string
		DefaultConfig      string
		Levels             string
		Status             string
		Data               string
		Schema             bool
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update an experiment",
		Long: "Update an experiment, including transitioning its status with --status " +
			"(active, paused, completed, archived).\n\n" +
			"Note: the feature flag and allocation strategy cannot be changed after creation, and " +
			"authentication flow, allocations, default config, and ramp levels can only be changed while the " +
			"experiment is a draft.\n\n" +
			"To update interactively, use `auth0 experimentation experiments update` with no arguments." +
			experimentationDataHelp,
		Example: `  auth0 experimentation experiments update
  auth0 experimentation experiments update <experiment-id>
  auth0 experimentation experiments update <experiment-id> --name "new-name"
  auth0 experimentation experiments update <experiment-id> --levels 10,50,100
  auth0 experimentation experiments update <experiment-id> --allocations '[{"variation_id":"vid","weight":100,"is_control":true}]'
  auth0 experimentation experiments update <experiment-id> --status active

  # Discover the payload schema
  auth0 experimentation experiments update --schema
  auth0 experimentation experiments update --schema --json

  # JSON input mode (for agents and automation)
  auth0 experimentation experiments update <experiment-id> --data '{"status":"active"}'
  auth0 experimentation experiments update <experiment-id> --data @experiment.json
  cat experiment.json | auth0 experimentation experiments update <experiment-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Schema {
				return printOperationSchema(cli, "PATCH", "/experimentation/experiments/{id}")
			}

			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := experimentID.Pick(cmd, &inputs.ID, cli.experimentPickerOptions); err != nil {
					return err
				}
			}

			payload, provided, err := ResolveData(cmd)
			if err != nil {
				return err
			}
			if provided {
				return updateExperimentFromJSON(cli, cmd, inputs.ID, payload)
			}

			// Read the current experiment so interactive prompts can be pre-filled
			// with the existing values. Pressing Enter then keeps the current value
			// rather than clearing it, so only the fields the user actually changes
			// are updated.
			var current *management.GetExperimentResponseContent
			if err := ansi.Waiting(func() (err error) {
				current, err = cli.apiv3.Experiments.Get(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to get experiment %q: %w", inputs.ID, err)
			}

			currentName := current.GetName()
			currentDescription := current.GetDescription()

			// Seed inputs with the current values so untouched fields keep their
			// existing value. Only seed when the flag wasn't explicitly passed,
			// otherwise we'd clobber the value the user set on the command line.
			if !experimentName.IsSet(cmd) {
				inputs.Name = currentName
			}
			if !experimentDescription.IsSet(cmd) {
				inputs.Description = currentDescription
			}

			if err := experimentName.AskU(cmd, &inputs.Name, &currentName); err != nil {
				return err
			}
			if err := experimentDescription.AskU(cmd, &inputs.Description, &currentDescription); err != nil {
				return err
			}

			// These fields can only be changed while the experiment is a draft, so
			// (when not supplied via a flag) offer each behind a confirmation rather
			// than always prompting. A flag always wins; the confirm gate only runs
			// in interactive mode.
			authFlowChanged := experimentAuthFlow.IsSet(cmd)
			defaultConfigChanged := experimentDefaultConfig.IsSet(cmd)
			levelsChanged := experimentLevels.IsSet(cmd)
			statusChanged := experimentStatus.IsSet(cmd)

			// Ramp levels decide whether allocations carry weights, so settle them
			// before the allocations. Ramps require the percentage strategy.
			strategy := string(current.GetAllocationStrategy())
			if !levelsChanged && strategy == "percentage" && canPrompt(cmd) && prompt.Confirm("Change the ramp levels?") {
				if err := experimentLevels.Ask(cmd, &inputs.Levels, nil); err != nil {
					return err
				}
				levelsChanged = true
			}
			if levelsChanged && strategy != "percentage" {
				// An experiment created without a strategy keeps none, as PATCH can't set one.
				if strategy == "" {
					return usageError{err: errors.New("ramp levels require the percentage allocation strategy, but this experiment has none"), reason: "invalid_flag_value"}
				}
				return usageError{err: fmt.Errorf("ramp levels require the percentage allocation strategy, but this experiment uses %s", strategy), reason: "invalid_flag_value"}
			}
			ramp := levelsChanged || len(current.GetLevels()) > 0
			// Turning a draft into a ramp leaves any existing weights in place,
			// which the API then rejects at validation, so the allocations must be
			// re-entered without them. Levels can only change on a draft, so leave
			// any other status to the API to reject.
			mustReplaceAllocations := levelsChanged &&
				current.GetStatus() == management.ExperimentStatusEnumDraft &&
				len(current.GetLevels()) == 0 &&
				allocationsHaveWeights(current.GetAllocations())

			// Allocations aren't a simple text field, so (when not supplied via a
			// flag) offer to edit them interactively behind a confirmation rather
			// than always prompting.
			if inputs.Allocations == "" && !experimentAllocations.IsSet(cmd) && canPrompt(cmd) {
				if mustReplaceAllocations {
					cli.renderer.Infof("Ramp experiments don't use allocation weights, so re-enter the allocations without them.")
				}
				if mustReplaceAllocations || prompt.Confirm("Change the traffic allocations?") {
					// Mirror create: build allocations for the experiment's current
					// (immutable) strategy.
					allocs, err := cli.buildAllocationsInteractively(cmd, current.GetFeatureFlagID(), strategy, ramp)
					if err != nil {
						return err
					}
					b, err := json.Marshal(allocs)
					if err != nil {
						return err
					}
					inputs.Allocations = string(b)
				}
			} else if err := experimentAllocations.AskU(cmd, &inputs.Allocations, nil); err != nil {
				return err
			}
			if mustReplaceAllocations && inputs.Allocations == "" {
				return usageError{
					err:    errors.New("the current allocations have weights, which ramp experiments don't use: also pass --allocations without \"weight\" when setting --levels"),
					reason: "missing_required_flags",
				}
			}

			if !authFlowChanged && canPrompt(cmd) && prompt.Confirm("Change the authentication flow?") {
				if err := experimentAuthFlow.Select(cmd, &inputs.AuthenticationFlow, authenticationFlowOptions, nil); err != nil {
					return err
				}
				authFlowChanged = true
			}
			if !defaultConfigChanged && canPrompt(cmd) && prompt.Confirm("Change the default config (Auth0-managed flags only)?") {
				if err := experimentDefaultConfig.Select(cmd, &inputs.DefaultConfig, []string{"tenant", "flag"}, nil); err != nil {
					return err
				}
				defaultConfigChanged = true
			}
			if !statusChanged && canPrompt(cmd) && prompt.Confirm("Change the status?") {
				if err := experimentStatus.Select(cmd, &inputs.Status, experimentStatuses, nil); err != nil {
					return err
				}
				statusChanged = true
			}

			// Build a request containing only the fields the user actually
			// changed. Scalar fields are diffed against the current experiment;
			// allocations are replaced wholesale (PUT-like) whenever provided.
			req := &management.UpdateExperimentRequestParameters{}
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
			if authFlowChanged {
				flow := management.AuthenticationFlowEnum(inputs.AuthenticationFlow)
				req.AuthenticationFlow = &flow
				updated = true
			}
			if defaultConfigChanged {
				dc, err := parseDefaultConfig(inputs.DefaultConfig)
				if err != nil {
					return err
				}
				req.DefaultConfig = &dc
				updated = true
			}
			if levelsChanged {
				levels, err := parseLevels(inputs.Levels)
				if err != nil {
					return err
				}
				req.Levels = levels
				updated = true
			}
			if inputs.Allocations != "" {
				var allocations []*management.AllocationRequestItem
				if err := json.Unmarshal([]byte(inputs.Allocations), &allocations); err != nil {
					return validationError{err: fmt.Errorf("invalid JSON for --allocations: %w", err), reason: "malformed_json"}
				}
				if err := validateAllocationCount(allocations); err != nil {
					return err
				}
				if err := validateAllocationWeights(allocations); err != nil {
					return err
				}
				if ramp {
					if err := validateRampAllocations(strategy, allocations); err != nil {
						return err
					}
				}
				// Allocations are PUT-like: whenever the user opts in (via the flag
				// or by confirming the interactive prompt) we send the whole array.
				req.Allocations = allocations
				updated = true
			}
			if statusChanged {
				if !slices.Contains(experimentStatuses, inputs.Status) {
					return usageError{err: fmt.Errorf("invalid status %q: must be one of %s", inputs.Status, strings.Join(experimentStatuses, ", ")), reason: "invalid_flag_value"}
				}
				status := management.ExperimentTransitionStatusEnum(inputs.Status)
				req.Status = &status
				updated = true
			}

			if !updated {
				return usageError{
					err:    errors.New("nothing to update — provide at least one flag (--name, --description, --authentication-flow, --allocations, --default-config, --levels, --status) or --data"),
					reason: "missing_required_flags",
				}
			}

			var result *management.UpdateExperimentResponseContent
			if err := ansi.Waiting(func() (err error) {
				result, err = cli.apiv3.Experiments.Update(cmd.Context(), inputs.ID, req)
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
	experimentAuthFlow.RegisterStringU(cmd, &inputs.AuthenticationFlow, "")
	experimentAllocations.RegisterStringU(cmd, &inputs.Allocations, "")
	experimentDefaultConfig.RegisterStringU(cmd, &inputs.DefaultConfig, "")
	experimentLevels.RegisterStringU(cmd, &inputs.Levels, "")
	experimentStatus.RegisterStringU(cmd, &inputs.Status, "")
	dataFlag.RegisterString(cmd, &inputs.Data, "")
	schemaFlag.RegisterBool(cmd, &inputs.Schema, false)
	markDataExclusive(cmd)

	return cmd
}

func createExperimentFromJSON(cli *cli, cmd *cobra.Command, dataStr string) error {
	result, err := runJSONWrite[management.CreateExperimentResponseContent](cli, cmd, jsonWriteSpec{
		Method:     http.MethodPost,
		SchemaPath: "/experimentation/experiments",
		URI:        cli.api.HTTPClient.URI("experimentation", "experiments"),
		Data:       dataStr,
		SchemaCmd:  "auth0 experimentation experiments create",
	})
	if err != nil {
		return fmt.Errorf("failed to create experiment: %w", err)
	}

	return cli.renderer.ExperimentCreate(result)
}

func updateExperimentFromJSON(cli *cli, cmd *cobra.Command, id, dataStr string) error {
	result, err := runJSONWrite[management.UpdateExperimentResponseContent](cli, cmd, jsonWriteSpec{
		Method:     http.MethodPatch,
		SchemaPath: "/experimentation/experiments/{id}",
		URI:        cli.api.HTTPClient.URI("experimentation", "experiments", id),
		Data:       dataStr,
		SchemaCmd:  "auth0 experimentation experiments update",
	})
	if err != nil {
		return fmt.Errorf("failed to update experiment %q: %w", id, err)
	}

	return cli.renderer.ExperimentUpdate(result)
}

func deleteExperimentCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Short:   "Delete an experiment",
		Long: "Delete an experiment.\n\n" +
			"Active experiments must be paused or completed before deleting.\n\n" +
			"To delete non-interactively, supply the experiment ID and use `--force` to skip confirmation.",
		Example: `  auth0 experimentation experiments delete
  auth0 experimentation experiments delete <experiment-id>
  auth0 experimentation experiments delete <experiment-id> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var ids []string
			if len(args) == 0 {
				if err := experimentID.PickMany(cmd, &ids, cli.experimentPickerOptions); err != nil {
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

			return ansi.ProgressBar("Deleting experiment(s)", ids, func(_ int, id string) error {
				if id != "" {
					if err := cli.apiv3.Experiments.Delete(cmd.Context(), id); err != nil {
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
		Example: `  auth0 experimentation experiments validate
  auth0 experimentation experiments validate <experiment-id>
  auth0 experimentation experiments validate <experiment-id> --json`,
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
				result, err = cli.apiv3.Experiments.Validate(cmd.Context(), inputs.ID)
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

// advanceRampExperimentCmd advances a ramp experiment to the next exposure
// level in its configured schedule.
func advanceRampExperimentCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID          string
		TargetLevel int
	}

	cmd := &cobra.Command{
		Use:   "advance-ramp",
		Args:  cobra.MaximumNArgs(1),
		Short: "Advance a ramp experiment to its next level",
		Long: "Advance a ramp experiment to the next exposure level in its schedule.\n\n" +
			"The target level must be the immediate next level configured on the experiment. " +
			"Configure ramp levels with `auth0 experimentation experiments create --levels` (or `update --levels`).",
		Example: `  auth0 experimentation experiments advance-ramp
  auth0 experimentation experiments advance-ramp <experiment-id> --target-level 25`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else if err := experimentID.Pick(cmd, &inputs.ID, cli.experimentPickerOptions); err != nil {
				return err
			}

			if !cmd.Flags().Changed("target-level") {
				if !canPrompt(cmd) {
					return usageError{err: errors.New("--target-level is required"), reason: "missing_required_flags"}
				}
				var levelStr string
				q := prompt.TextInput(
					"target-level",
					"Target level (immediate next percentage)",
					"The target percentage level to advance to. Must be the immediate next level in the schedule.",
					"",
					true,
				)
				if err := prompt.AskOne(q, &levelStr); err != nil {
					return err
				}
				level, err := strconv.Atoi(strings.TrimSpace(levelStr))
				if err != nil {
					return usageError{err: fmt.Errorf("invalid target level %q: must be a whole number", levelStr), reason: "invalid_flag_value"}
				}
				inputs.TargetLevel = level
			}

			if inputs.TargetLevel < 0 || inputs.TargetLevel > 100 {
				return usageError{err: fmt.Errorf("invalid target level %d: must be between 0 and 100", inputs.TargetLevel), reason: "invalid_flag_value"}
			}

			var result *management.AdvanceRampResponseContent
			if err := ansi.Waiting(func() (err error) {
				result, err = cli.apiv3.Experiments.AdvanceRamp(cmd.Context(), inputs.ID, &management.AdvanceRampRequestContent{
					TargetLevel: inputs.TargetLevel,
				})
				return err
			}); err != nil {
				return fmt.Errorf("failed to advance ramp for experiment %q: %w", inputs.ID, err)
			}

			cli.renderer.ExperimentAdvanceRamp(result)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().IntVar(&inputs.TargetLevel, "target-level", 0, "The target percentage level to advance to. Must be the immediate next level in the schedule.")

	return cmd
}

// Picker helpers.

func (c *cli) experimentPickerOptions(ctx context.Context) (pickerOptions, error) {
	page, err := c.apiv3.Experiments.List(ctx, &management.ListExperimentsRequestParameters{})
	if err != nil {
		return nil, err
	}

	var opts pickerOptions
	for _, e := range page.Results {
		label := fmt.Sprintf("%s %s %s", e.GetName(), ansi.Faint("("+e.GetID()+")"), statusBadge(string(e.GetStatus())))
		opts = append(opts, pickerOption{value: e.GetID(), label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("no experiments available. Create one by running: `auth0 experimentation experiments create`")
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

// buildAllocationsInteractively prompts for the variations to include and how
// each is allocated, returning allocations that satisfy the API's rules for the
// strategy: exactly one control; for percentage, weights summing to 100 (or no
// weights for ramp experiments, whose traffic follows the level schedule); for
// segment, exactly one fallback and a segment for every other variation.
func (c *cli) buildAllocationsInteractively(cmd *cobra.Command, featureFlagID, strategy string, ramp bool) ([]*management.AllocationRequestItem, error) {
	ctx := cmd.Context()
	variations, err := c.apiv3.Variations.List(ctx, featureFlagID)
	if err != nil {
		return nil, fmt.Errorf("failed to list variations: %w", err)
	}

	allVariations := variations.GetVariations()
	if len(allVariations) == 0 {
		return nil, fmt.Errorf("no variations found for feature flag %q. Create some first with `auth0 experimentation feature-flags variations create %s`", featureFlagID, featureFlagID)
	}

	// Let the user pick which variations to include in the experiment. A feature
	// flag can have more variations than an experiment accepts (or than fit on
	// screen), so the picker is paged and the selection is capped: choose at
	// least one and at most maxExperimentAllocations.
	labels := make([]string, 0, len(allVariations))
	for _, v := range allVariations {
		labels = append(labels, variationOptionLabel(v))
	}

	upperBound := min(len(allVariations), maxExperimentAllocations)

	var selectedLabels []string
	if err := prompt.AskMultiSelectWithPageSize(
		fmt.Sprintf("Select the variations to include (choose 1–%d; use arrows/space, type to filter)", upperBound),
		&selectedLabels,
		variationPickerPageSize,
		labels...,
	); err != nil {
		return nil, err
	}
	if len(selectedLabels) == 0 {
		return nil, fmt.Errorf("select at least one variation to include in the experiment")
	}
	if len(selectedLabels) > maxExperimentAllocations {
		return nil, fmt.Errorf("selected %d variations but an experiment accepts at most %d", len(selectedLabels), maxExperimentAllocations)
	}

	// Preserve the original variation order for the variations that were selected.
	selectedSet := make(map[string]bool, len(selectedLabels))
	for _, l := range selectedLabels {
		selectedSet[l] = true
	}
	selectedVariations := make([]*management.Variation, 0, len(selectedLabels))
	for _, v := range allVariations {
		if selectedSet[variationOptionLabel(v)] {
			selectedVariations = append(selectedVariations, v)
		}
	}

	c.renderer.Infof("Selected %d variation(s).", len(selectedVariations))

	// The API requires exactly one control (and, for segment strategy, exactly
	// one fallback), so pick each once rather than asking per variation.
	control, err := c.pickAllocationVariation(cmd, "Control", "Which variation is the control (the baseline the others are compared against)?", selectedVariations)
	if err != nil {
		return nil, err
	}

	fallback := -1
	var segOpts pickerOptions
	if strategy == "segment" {
		if fallback, err = c.pickAllocationVariation(cmd, "Fallback", "Which variation is the fallback (served to users who match no segment)?", selectedVariations); err != nil {
			return nil, err
		}
		// Every variation other than the fallback must target a segment.
		if len(selectedVariations) > 1 {
			if segOpts, err = c.segmentPickerOptions(ctx); err != nil {
				return nil, fmt.Errorf("each variation other than the fallback must target a segment: %w", err)
			}
		}
	}

	c.renderer.Newline()

	var allocations []*management.AllocationRequestItem
	remainingWeight := 100
	priority := 0

	for i, v := range selectedVariations {
		alloc := &management.AllocationRequestItem{
			VariationID: v.GetID(),
			IsControl:   i == control,
		}

		switch {
		case strategy == "percentage" && !ramp:
			c.renderer.Infof("Variation %d/%d: %s %s", i+1, len(selectedVariations), v.GetName(), ansi.Faint("("+v.GetID()+")"))
			// Default to an even split, with the last variation taking the
			// remainder so the defaults always sum to 100.
			defaultWeight := 100 / len(selectedVariations)
			if i == len(selectedVariations)-1 {
				defaultWeight = max(remainingWeight, 0)
			}
			weight, err := askAllocationInt(
				fmt.Sprintf("Weight for %q (0–100)", v.GetName()),
				"Integer percentage of traffic assigned to this variation. Weights must sum to 100.",
				defaultWeight,
				0,
			)
			if err != nil {
				return nil, err
			}
			remainingWeight -= weight
			alloc.Weight = &weight
		case strategy == "segment" && i == fallback:
			isFallback := true
			alloc.IsFallback = &isFallback
		case strategy == "segment":
			c.renderer.Infof("Variation %d/%d: %s %s", i+1, len(selectedVariations), v.GetName(), ansi.Faint("("+v.GetID()+")"))
			selectFlag := Flag{Name: "Segment", LongForm: "segment"}
			var chosen string
			if err := selectFlag.Select(cmd, &chosen, segOpts.labels(), nil); err != nil {
				return nil, err
			}
			sid := segOpts.getValue(chosen)
			alloc.SegmentID = &sid

			priority++
			p, err := askAllocationInt(
				fmt.Sprintf("Priority for %q (1 is evaluated first)", v.GetName()),
				"When a user matches several segments, the allocation with the lowest priority number wins.",
				priority,
				1,
			)
			if err != nil {
				return nil, err
			}
			alloc.Priority = &p
		}

		allocations = append(allocations, alloc)
	}

	if strategy == "percentage" && !ramp && remainingWeight != 0 {
		return nil, fmt.Errorf("allocation weights must sum to 100, got %d", 100-remainingWeight)
	}

	return allocations, nil
}

// pickAllocationVariation asks which of the selected variations plays a role
// (control or fallback) and returns its index. A single variation is chosen
// without prompting.
func (c *cli) pickAllocationVariation(cmd *cobra.Command, name, message string, variations []*management.Variation) (int, error) {
	if len(variations) == 1 {
		c.renderer.Infof("%s: %s", name, variations[0].GetName())
		return 0, nil
	}

	labels := make([]string, 0, len(variations))
	for _, v := range variations {
		labels = append(labels, variationOptionLabel(v))
	}

	var chosen string
	selectFlag := Flag{Name: message, LongForm: strings.ToLower(name)}
	if err := selectFlag.Select(cmd, &chosen, labels, nil); err != nil {
		return 0, err
	}

	return slices.Index(labels, chosen), nil
}

// askAllocationInt prompts for a whole number between minValue and 100.
func askAllocationInt(message, help string, defaultValue, minValue int) (int, error) {
	var raw string
	q := prompt.TextInput("value", message, help, strconv.Itoa(defaultValue), true)
	if err := prompt.AskOne(q, &raw); err != nil {
		return 0, err
	}

	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < minValue || n > 100 {
		return 0, fmt.Errorf("invalid value %q: must be a whole number between %d and 100", raw, minValue)
	}

	return n, nil
}

// allocationsHaveWeights reports whether any existing allocation carries a weight.
func allocationsHaveWeights(allocations []*management.AllocationItem) bool {
	for _, a := range allocations {
		if a.Weight != nil {
			return true
		}
	}
	return false
}

// variationOptionLabel renders a variation as a picker option label ("name (id)").
func variationOptionLabel(v *management.Variation) string {
	return fmt.Sprintf("%s %s", v.GetName(), ansi.Faint("("+v.GetID()+")"))
}

// parseDefaultConfig validates the --default-config value against the API enum (tenant | flag).
func parseDefaultConfig(raw string) (management.DefaultConfigEnum, error) {
	dc, err := management.NewDefaultConfigEnumFromString(raw)
	if err != nil {
		return "", usageError{err: fmt.Errorf("invalid --default-config %q: must be tenant or flag", raw), reason: "invalid_flag_value"}
	}
	return dc, nil
}

// parseLevels decodes a comma-separated --levels value into a strictly-increasing
// slice of integer percentages in [0, 100], matching the ramp schedule the API expects.
func parseLevels(raw string) ([]int, error) {
	parts := strings.Split(raw, ",")
	levels := make([]int, 0, len(parts))
	prev := -1
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, usageError{err: fmt.Errorf("invalid level %q: must be a whole number", p), reason: "invalid_flag_value"}
		}
		if n < 0 || n > 100 {
			return nil, usageError{err: fmt.Errorf("invalid level %d: must be between 0 and 100", n), reason: "invalid_flag_value"}
		}
		if n <= prev {
			return nil, usageError{err: fmt.Errorf("--levels must be strictly increasing: %d does not come after %d", n, prev), reason: "invalid_flag_value"}
		}
		prev = n
		levels = append(levels, n)
	}
	if len(levels) == 0 {
		return nil, usageError{err: errors.New("--levels must contain at least one integer percentage (e.g. 10,25,50,100)"), reason: "invalid_flag_value"}
	}
	return levels, nil
}

// validateAllocationCount enforces that the number of allocations is within the
// bounds an experiment accepts: at least one, and no more than
// maxExperimentAllocations. It applies to the non-interactive --allocations
// flag; the interactive picker enforces the same bounds at selection time.
func validateAllocationCount(allocations []*management.AllocationRequestItem) error {
	switch {
	case len(allocations) == 0:
		return usageError{err: errors.New("--allocations must contain at least one variation"), reason: "invalid_flag_value"}
	case len(allocations) > maxExperimentAllocations:
		return usageError{
			err:    fmt.Errorf("--allocations contains %d variations but an experiment accepts at most %d", len(allocations), maxExperimentAllocations),
			reason: "invalid_flag_value",
		}
	}
	return nil
}

// validateRampAllocations enforces the API's ramp rules up front: ramps need the
// percentage strategy, and their allocations must omit weights. The API accepts
// weights on create but then fails validation, so catch them here.
func validateRampAllocations(strategy string, allocations []*management.AllocationRequestItem) error {
	if strategy != "percentage" {
		return usageError{err: errors.New("ramp levels require --allocation-strategy percentage"), reason: "invalid_flag_value"}
	}
	for _, a := range allocations {
		if a.Weight != nil {
			return usageError{
				err:    errors.New("ramp experiments don't use allocation weights (traffic follows the --levels schedule): remove \"weight\" from --allocations"),
				reason: "invalid_flag_value",
			}
		}
	}
	return nil
}

// validateAllocationWeights enforces that each --allocations JSON weight is a whole-number percentage between 0 and 100.
func validateAllocationWeights(allocations []*management.AllocationRequestItem) error {
	for _, a := range allocations {
		if a.Weight == nil {
			continue
		}
		w := *a.Weight
		if w < 0 || w > 100 {
			return usageError{err: fmt.Errorf("invalid weight %d: must be a whole number between 0 and 100", w), reason: "invalid_flag_value"}
		}
	}
	return nil
}
