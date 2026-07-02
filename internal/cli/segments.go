package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/auth0/go-auth0/v2/management"
	managementcore "github.com/auth0/go-auth0/v2/management/core"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
)

// segmentAttributes and segmentConditions are derived at startup from the SDK
// types via reflection, so they stay in sync with go-auth0 automatically
// instead of needing manual updates whenever the API adds a field. They are the
// single source of truth for both the help text and the client-side validation.
var (
	// Attributes are the JSON field names of SegmentMatchConditions
	// (client_id, domain, country, ...).
	segmentAttributes = jsonFieldNames(reflect.TypeOf(management.SegmentMatchConditions{}))

	// Conditions are the JSON field names of the expression structs the SDK
	// unions over (contains, starts_with, ends_with, exists).
	segmentConditions = jsonFieldNames(
		reflect.TypeOf(management.SegmentContainsExpression{}),
		reflect.TypeOf(management.SegmentStartsWithExpression{}),
		reflect.TypeOf(management.SegmentEndsWithExpression{}),
		reflect.TypeOf(management.SegmentExistsExpression{}),
	)

	segmentRulesHelp = "Rules for matching users, as a JSON array. Each rule has a `match` and/or `not_match` object that maps an attribute to a condition.\n" +
		"Attributes: " + strings.Join(segmentAttributes, ", ") + ".\n" +
		"Conditions: " + strings.Join(segmentConditions, ", ") + `, or a plain list ["a","b"] for an exact match.` + "\n" +
		`Example: '[{"match":{"domain":{"ends_with":["example.com"]}}}]'`
)

// jsonFieldNames returns the JSON tag names of the exported fields of the given
// struct types, in declaration order, skipping fields tagged "-" or without a
// tag. It is used to derive the set of valid segment attributes and conditions
// directly from the SDK types.
func jsonFieldNames(types ...reflect.Type) []string {
	var names []string
	for _, t := range types {
		for i := 0; i < t.NumField(); i++ {
			tag := t.Field(i).Tag.Get("json")
			if tag == "" || tag == "-" {
				continue
			}
			name := strings.Split(tag, ",")[0]
			if name == "" {
				continue
			}
			names = append(names, name)
		}
	}
	return names
}

var (
	segmentID = Argument{
		Name: "Segment ID",
		Help: "ID of the segment.",
	}

	segmentName = Flag{
		Name:       "Name",
		LongForm:   "name",
		ShortForm:  "n",
		Help:       "Name of the segment.",
		IsRequired: true,
	}

	segmentDescription = Flag{
		Name:      "Description",
		LongForm:  "description",
		ShortForm: "d",
		Help:      "Description of the segment.",
	}

	segmentRules = Flag{
		Name:       "Rules",
		LongForm:   "rules",
		ShortForm:  "r",
		Help:       segmentRulesHelp,
		IsRequired: true,
	}
)

func segmentsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "segments",
		Short: "Manage experimentation segments",
		Long:  "Segments define groups of users matched by rules (email domain, attribute presence, etc.) for use in experiments.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listSegmentsCmd(cli))
	cmd.AddCommand(createSegmentCmd(cli))
	cmd.AddCommand(showSegmentCmd(cli))
	cmd.AddCommand(updateSegmentCmd(cli))
	cmd.AddCommand(deleteSegmentCmd(cli))

	return cmd
}

func listSegmentsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your segments",
		Long:    "List all segments. To create one, run: `auth0 segments create`.",
		Example: `  auth0 segments list
  auth0 segments ls
  auth0 segments list --json
  auth0 segments list --csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var allSegments []*management.Segment

			if err := ansi.Waiting(func() error {
				page, err := cli.apiv2.Segments.List(cmd.Context(), &management.ListSegmentsRequestParameters{})
				if err != nil {
					return err
				}
				allSegments = append(allSegments, page.Results...)
				for {
					next, err := page.GetNextPage(cmd.Context())
					if errors.Is(err, managementcore.ErrNoPages) {
						break
					}
					if err != nil {
						return err
					}
					allSegments = append(allSegments, next.Results...)
					page = next
				}
				return nil
			}); err != nil {
				return fmt.Errorf("failed to list segments: %w", err)
			}

			cli.renderer.SegmentList(allSegments)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	return cmd
}

func showSegmentCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show a segment",
		Long:  "Display details about a segment including its rules.",
		Example: `  auth0 segments show
  auth0 segments show <segment-id>
  auth0 segments show <segment-id> --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if err := segmentID.Pick(cmd, &inputs.ID, cli.segmentPickerOptions); err != nil {
					return err
				}
			} else {
				inputs.ID = args[0]
			}

			var segment *management.GetSegmentResponseContent
			if err := ansi.Waiting(func() (err error) {
				segment, err = cli.apiv2.Segments.Get(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to get segment %q: %w", inputs.ID, err)
			}

			cli.renderer.SegmentShow(segment)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

	return cmd
}

func createSegmentCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Name        string
		Description string
		Rules       string
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new segment",
		Long: "Create a new segment.\n\n" +
			"To create interactively, use `auth0 segments create` with no flags.\n\n" +
			"To create non-interactively, supply name and rules through the flags.",
		Example: `  auth0 segments create
  auth0 segments create --name "Beta Users" --rules '[{"match":{"domain":{"contains":["beta.example.com"]}}}]'
  auth0 segments create -n "Internal" -r '[{"match":{"domain":{"ends_with":["mycompany.com"]}}}]'
  auth0 segments create -n "US Chrome" -r '[{"match":{"country":["US"],"browser":{"contains":["Chrome"]}}}]'
  auth0 segments create -n "External non-US" -r '[{"match":{"domain":{"ends_with":["example.com"]}},"not_match":{"country":["US"]}}]'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := segmentName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			if err := segmentDescription.Ask(cmd, &inputs.Description, nil); err != nil {
				return err
			}

			if err := segmentRules.OpenEditor(
				cmd,
				&inputs.Rules,
				`[{"match":{"domain":{"ends_with":["example.com"]}}}]`,
				"segment.*.json",
				cli.segmentRulesEditorHint,
			); err != nil {
				return err
			}

			if inputs.Rules == "" {
				return fmt.Errorf("--rules is required (e.g. --rules '[{\"match\":{\"domain\":{\"ends_with\":[\"example.com\"]}}}]')")
			}
			rules, err := parseSegmentRules(inputs.Rules)
			if err != nil {
				return err
			}

			req := &management.CreateSegmentRequestContent{
				Name:  inputs.Name,
				Rules: rules,
			}
			if inputs.Description != "" {
				req.Description = &inputs.Description
			}

			var result *management.CreateSegmentResponseContent
			if err := ansi.Waiting(func() (err error) {
				result, err = cli.apiv2.Segments.Create(cmd.Context(), req)
				return err
			}); err != nil {
				return fmt.Errorf("failed to create segment: %w", err)
			}

			return cli.renderer.SegmentCreate(result)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	segmentName.RegisterString(cmd, &inputs.Name, "")
	segmentDescription.RegisterString(cmd, &inputs.Description, "")
	segmentRules.RegisterString(cmd, &inputs.Rules, "")

	return cmd
}

func updateSegmentCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID          string
		Name        string
		Description string
		Rules       string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update a segment",
		Long: "Update a segment.\n\n" +
			"To update interactively, use `auth0 segments update` with no arguments.\n\n" +
			"To update non-interactively, supply the segment ID and fields to change through the flags.",
		Example: `  auth0 segments update
  auth0 segments update <segment-id>
  auth0 segments update <segment-id> --name "New Name"
  auth0 segments update <segment-id> --rules '[{"match":{"domain":{"contains":["newdomain.com"]}}}]'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := segmentID.Pick(cmd, &inputs.ID, cli.segmentPickerOptions); err != nil {
					return err
				}
			}

			var existing *management.GetSegmentResponseContent
			if err := ansi.Waiting(func() (err error) {
				existing, err = cli.apiv2.Segments.Get(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to get segment %q: %w", inputs.ID, err)
			}

			if err := segmentName.AskU(cmd, &inputs.Name, nil); err != nil {
				return err
			}

			if err := segmentDescription.AskU(cmd, &inputs.Description, nil); err != nil {
				return err
			}

			if err := segmentRules.AskU(cmd, &inputs.Rules, nil); err != nil {
				return err
			}

			req := &management.UpdateSegmentRequestContent{}
			updated := false

			if inputs.Name != "" {
				req.Name = &inputs.Name
				updated = true
			}
			if inputs.Description != "" {
				req.Description = &inputs.Description
				updated = true
			}
			if inputs.Rules != "" {
				rules, err := parseSegmentRules(inputs.Rules)
				if err != nil {
					return err
				}
				req.Rules = rules
				updated = true
			}

			if !updated {
				return fmt.Errorf("nothing to update — provide at least one flag")
			}

			_ = existing

			var result *management.UpdateSegmentResponseContent
			if err := ansi.Waiting(func() (err error) {
				result, err = cli.apiv2.Segments.Update(cmd.Context(), inputs.ID, req)
				return err
			}); err != nil {
				return fmt.Errorf("failed to update segment %q: %w", inputs.ID, err)
			}

			return cli.renderer.SegmentUpdate(result)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	segmentName.RegisterStringU(cmd, &inputs.Name, "")
	segmentDescription.RegisterStringU(cmd, &inputs.Description, "")
	segmentRules.RegisterStringU(cmd, &inputs.Rules, "")

	return cmd
}

func deleteSegmentCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Short:   "Delete a segment",
		Long: "Delete a segment.\n\n" +
			"To delete interactively, use `auth0 segments delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the segment ID and use `--force` to skip confirmation.",
		Example: `  auth0 segments delete
  auth0 segments rm
  auth0 segments delete <segment-id>
  auth0 segments delete <segment-id> --force
  auth0 segments delete <segment-id> <segment-id2> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var ids []string
			if len(args) == 0 {
				if err := segmentID.PickMany(cmd, &ids, cli.segmentPickerOptions); err != nil {
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

			return ansi.ProgressBar("Deleting segment(s)", ids, func(_ int, id string) error {
				if id != "" {
					if err := cli.apiv2.Segments.Delete(cmd.Context(), id); err != nil {
						return fmt.Errorf("failed to delete segment %q: %w", id, err)
					}
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

func (c *cli) segmentPickerOptions(ctx context.Context) (pickerOptions, error) {
	page, err := c.apiv2.Segments.List(ctx, &management.ListSegmentsRequestParameters{})
	if err != nil {
		return nil, err
	}

	var opts pickerOptions
	for _, s := range page.Results {
		label := fmt.Sprintf("%s %s", s.GetName(), ansi.Faint("("+s.GetID()+")"))
		opts = append(opts, pickerOption{value: s.GetID(), label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("no segments available. Create one by running: `auth0 segments create`")
	}

	return opts, nil
}

func (c *cli) segmentRulesEditorHint() {
	c.renderer.Infof("Enter the segment rules as a JSON array. Each rule has a `match` and/or `not_match` object mapping an attribute to a condition.")
	c.renderer.Infof("Attributes: %s.", strings.Join(segmentAttributes, ", "))
	c.renderer.Infof(`Conditions: %s, or a plain list ["a","b"] for an exact match.`, strings.Join(segmentConditions, ", "))
	c.renderer.Infof(`Example: [{"match":{"domain":{"ends_with":["example.com"]}}}]`)
}

// parseSegmentRules unmarshals the raw --rules JSON into SDK rules and validates
// that every attribute and condition is one the API recognizes. The SDK silently
// drops unknown keys on unmarshal, so without this a typo like {"match":{"contains":[...]}}
// would be accepted locally and only fail server-side with an opaque error.
func parseSegmentRules(raw string) ([]*management.SegmentRule, error) {
	var rules []*management.SegmentRule
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return nil, fmt.Errorf("invalid JSON for --rules (ensure the value is quoted in your shell): %w", err)
	}

	// Re-decode as a generic structure so we can inspect the keys the user
	// actually wrote, which the typed unmarshal above discards.
	var generic []map[string]map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &generic); err != nil {
		return nil, fmt.Errorf("--rules must be a JSON array of rule objects: %w", err)
	}

	attrSet := sliceToSet(segmentAttributes)
	condSet := sliceToSet(segmentConditions)

	for i, rule := range generic {
		for block, conditions := range rule {
			if block != "match" && block != "not_match" {
				return nil, fmt.Errorf("rule[%d].%s: unknown key — each rule may only have \"match\" and/or \"not_match\"", i, block)
			}
			for attr, expr := range conditions {
				if !attrSet[attr] {
					return nil, fmt.Errorf("rule[%d].%s.%s: unknown attribute — valid attributes are: %s", i, block, attr, strings.Join(segmentAttributes, ", "))
				}
				// A bare list ["a","b"] is a valid exact-match condition; only
				// object conditions carry an operator to check.
				var opObj map[string]json.RawMessage
				if err := json.Unmarshal(expr, &opObj); err != nil {
					continue
				}
				for op := range opObj {
					if !condSet[op] {
						return nil, fmt.Errorf("rule[%d].%s.%s.%s: unknown condition — valid conditions are: %s, or a plain list for an exact match", i, block, attr, op, strings.Join(segmentConditions, ", "))
					}
				}
			}
		}
	}

	return rules, nil
}

func sliceToSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}
