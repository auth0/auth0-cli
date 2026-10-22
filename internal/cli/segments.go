package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"

	"github.com/auth0/go-auth0/v3/management"
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
		"When omitted, an editor opens pre-filled with a commented template (// and /* */ comments are stripped on save).\n" +
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
		Name:         "Description",
		LongForm:     "description",
		ShortForm:    "d",
		AlwaysPrompt: true,
		Help:         "Description of the segment.",
	}

	segmentRules = Flag{
		Name:       "Rules",
		LongForm:   "rules",
		ShortForm:  "r",
		Help:       segmentRulesHelp,
		IsRequired: true,
	}

	segmentStatus = Flag{
		Name:     "Status",
		LongForm: "status",
		Help:     "Transition the segment to a new status (active, archived). Archiving is irreversible.",
	}
)

// segmentStatuses are the states a segment can be transitioned to.
var segmentStatuses = []string{"active", "archived"}

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
	var inputs struct {
		Type   string
		Schema bool
		Query  string
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List your segments",
		Long:    "List all segments. To create one, run: `auth0 experimentation segments create`." + experimentationQueryHelp,
		Example: `  auth0 experimentation segments list
  auth0 experimentation segments ls
  auth0 experimentation segments list --type self
  auth0 experimentation segments list --json
  auth0 experimentation segments list --csv
  auth0 experimentation segments list --schema
  auth0 experimentation segments list --schema --json
  auth0 experimentation segments list --query '{"type":"self","take":10}'
  auth0 experimentation segments list --query '{"type":"self"}' --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Schema {
				return printOperationSchema(cli, "GET", "/experimentation/segments")
			}

			if inputs.Query != "" {
				return runJSONQuery(cli, cmd, jsonQuerySpec{
					Path:      "experimentation/segments",
					SchemaCmd: "auth0 experimentation segments list",
				}, inputs.Query)
			}

			req := &management.ListSegmentsRequestParameters{}
			if inputs.Type != "" {
				t := management.SegmentTypeFilterEnum(inputs.Type)
				req.Type = &t
			}

			var allSegments []*management.Segment

			if err := ansi.Waiting(func() error {
				page, err := cli.apiv3.Segments.List(cmd.Context(), req)
				if err != nil {
					return err
				}
				allSegments, err = collectPages(cmd.Context(), page)
				return err
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
	cmd.Flags().StringVar(&inputs.Type, "type", "", "Filter by type (auth0, self).")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")
	schemaFlag.RegisterBool(cmd, &inputs.Schema, false)
	listQueryFlag.RegisterString(cmd, &inputs.Query, "")
	markQueryExclusive(cmd, "type")

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
		Example: `  auth0 experimentation segments show
  auth0 experimentation segments show <segment-id>
  auth0 experimentation segments show <segment-id> --json`,
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
				segment, err = cli.apiv3.Segments.Get(cmd.Context(), inputs.ID)
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
		Data        string
		Schema      bool
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new segment",
		Long: "Create a new segment.\n\n" +
			"To create interactively, use `auth0 experimentation segments create` with no flags.\n\n" +
			"To create non-interactively, supply name and rules through the flags." + experimentationDataHelp,
		Example: `  auth0 experimentation segments create
  auth0 experimentation segments create --name "Beta Users" --rules '[{"match":{"domain":{"contains":["beta.example.com"]}}}]'
  auth0 experimentation segments create -n "Internal" -r '[{"match":{"domain":{"ends_with":["mycompany.com"]}}}]'
  auth0 experimentation segments create -n "US Chrome" -r '[{"match":{"country":["US"],"browser":{"contains":["Chrome"]}}}]'
  auth0 experimentation segments create -n "External non-US" -r '[{"match":{"domain":{"ends_with":["example.com"]}},"not_match":{"country":["US"]}}]'

  # Discover the payload schema
  auth0 experimentation segments create --schema
  auth0 experimentation segments create --schema --json

  # JSON input mode (for agents and automation)
  auth0 experimentation segments create --data '{"name":"Internal","rules":[{"match":{"domain":{"ends_with":["mycompany.com"]}}}]}'
  auth0 experimentation segments create --data @segment.json
  cat segment.json | auth0 experimentation segments create`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Schema {
				return printOperationSchema(cli, "POST", "/experimentation/segments")
			}

			payload, provided, err := ResolveData(cmd)
			if err != nil {
				return err
			}
			if provided {
				return createSegmentFromJSON(cli, cmd, payload)
			}

			if err := segmentName.Ask(cmd, &inputs.Name, nil); err != nil {
				return err
			}
			if inputs.Name == "" {
				return usageError{err: errors.New("--name is required"), reason: "missing_required_flags"}
			}

			if err := segmentDescription.Ask(cmd, &inputs.Description, nil); err != nil {
				return err
			}

			if err := segmentRules.OpenEditor(
				cmd,
				&inputs.Rules,
				segmentRulesTemplate(),
				"segment.*.json",
				cli.segmentRulesEditorHint,
			); err != nil {
				return err
			}

			if inputs.Rules == "" {
				return usageError{
					err:    errors.New("--rules is required (e.g. --rules '[{\"match\":{\"domain\":{\"ends_with\":[\"example.com\"]}}}]')"),
					reason: "missing_required_flags",
				}
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
				result, err = cli.apiv3.Segments.Create(cmd.Context(), req)
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
	dataFlag.RegisterString(cmd, &inputs.Data, "")
	schemaFlag.RegisterBool(cmd, &inputs.Schema, false)
	markDataExclusive(cmd)

	return cmd
}

func updateSegmentCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID          string
		Name        string
		Description string
		Rules       string
		Status      string
		Data        string
		Schema      bool
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update a segment",
		Long: "Update a segment.\n\n" +
			"To update interactively, use `auth0 experimentation segments update` with no arguments.\n\n" +
			"To update non-interactively, supply the segment ID and fields to change through the flags." + experimentationDataHelp,
		Example: `  auth0 experimentation segments update
  auth0 experimentation segments update <segment-id>
  auth0 experimentation segments update <segment-id> --name "New Name"
  auth0 experimentation segments update <segment-id> --rules '[{"match":{"domain":{"contains":["newdomain.com"]}}}]'
  auth0 experimentation segments update <segment-id> --status archived

  # Discover the payload schema
  auth0 experimentation segments update --schema
  auth0 experimentation segments update --schema --json

  # JSON input mode (for agents and automation)
  auth0 experimentation segments update <segment-id> --data '{"status":"active"}'
  auth0 experimentation segments update <segment-id> --data @segment.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.Schema {
				return printOperationSchema(cli, "PATCH", "/experimentation/segments/{id}")
			}

			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := segmentID.Pick(cmd, &inputs.ID, cli.segmentPickerOptions); err != nil {
					return err
				}
			}

			payload, provided, err := ResolveData(cmd)
			if err != nil {
				return err
			}
			if provided {
				return updateSegmentFromJSON(cli, cmd, inputs.ID, payload)
			}

			// Read the current segment so untouched fields keep their existing
			// value and only changed fields are sent.
			var existing *management.GetSegmentResponseContent
			if err := ansi.Waiting(func() (err error) {
				existing, err = cli.apiv3.Segments.Get(cmd.Context(), inputs.ID)
				return err
			}); err != nil {
				return fmt.Errorf("failed to get segment %q: %w", inputs.ID, err)
			}

			currentName := existing.GetName()
			currentDescription := existing.GetDescription()

			if !segmentName.IsSet(cmd) {
				inputs.Name = currentName
			}
			if !segmentDescription.IsSet(cmd) {
				inputs.Description = currentDescription
			}

			if err := segmentName.AskU(cmd, &inputs.Name, &currentName); err != nil {
				return err
			}

			if err := segmentDescription.AskU(cmd, &inputs.Description, &currentDescription); err != nil {
				return err
			}

			// Open the editor pre-filled with the current rules, prefixed with the
			// commented grammar guide, unless supplied via the flag. Leaving the
			// buffer unchanged keeps the rules as-is; the header comments are
			// stripped on parse. Compare against the exact pre-filled buffer so an
			// untouched edit is correctly treated as "no change".
			currentRules := marshalToJSON(existing.GetRules())
			prefilledRules := currentRules
			if !segmentRules.IsSet(cmd) {
				prefilledRules = segmentRulesHelpComment() + currentRules
				if err := segmentRules.OpenEditorU(
					cmd,
					&inputs.Rules,
					prefilledRules,
					"segment.*.json",
				); err != nil {
					return err
				}
			}

			// Status isn't a free-text field, so (when not supplied via a flag)
			// offer it behind a confirmation rather than always prompting.
			statusChanged := segmentStatus.IsSet(cmd)
			if !statusChanged && canPrompt(cmd) && prompt.Confirm("Change the status?") {
				if err := segmentStatus.Select(cmd, &inputs.Status, segmentStatuses, nil); err != nil {
					return err
				}
				statusChanged = true
			}

			// Diff scalar fields against the current segment; rules are replaced
			// wholesale only when they actually changed.
			req := &management.UpdateSegmentRequestContent{}
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
			if inputs.Rules != "" && inputs.Rules != prefilledRules {
				rules, err := parseSegmentRules(inputs.Rules)
				if err != nil {
					return err
				}
				req.Rules = rules
				updated = true
			}
			if statusChanged {
				if !slices.Contains(segmentStatuses, inputs.Status) {
					return usageError{err: fmt.Errorf("invalid status %q: must be one of %s", inputs.Status, strings.Join(segmentStatuses, ", ")), reason: "invalid_flag_value"}
				}
				status := management.SegmentStatusEnum(inputs.Status)
				req.Status = &status
				updated = true
			}

			if !updated {
				return usageError{err: errors.New("nothing to update — provide at least one flag or --data"), reason: "missing_required_flags"}
			}

			var result *management.UpdateSegmentResponseContent
			if err := ansi.Waiting(func() (err error) {
				result, err = cli.apiv3.Segments.Update(cmd.Context(), inputs.ID, req)
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
	segmentStatus.RegisterStringU(cmd, &inputs.Status, "")
	dataFlag.RegisterString(cmd, &inputs.Data, "")
	schemaFlag.RegisterBool(cmd, &inputs.Schema, false)
	markDataExclusive(cmd)

	return cmd
}

func createSegmentFromJSON(cli *cli, cmd *cobra.Command, dataStr string) error {
	result, err := runJSONWrite[management.CreateSegmentResponseContent](cli, cmd, jsonWriteSpec{
		Method:     http.MethodPost,
		SchemaPath: "/experimentation/segments",
		URI:        cli.api.HTTPClient.URI("experimentation", "segments"),
		Data:       dataStr,
		SchemaCmd:  "auth0 experimentation segments create",
	})
	if err != nil {
		return fmt.Errorf("failed to create segment: %w", err)
	}

	return cli.renderer.SegmentCreate(result)
}

func updateSegmentFromJSON(cli *cli, cmd *cobra.Command, id, dataStr string) error {
	result, err := runJSONWrite[management.UpdateSegmentResponseContent](cli, cmd, jsonWriteSpec{
		Method:     http.MethodPatch,
		SchemaPath: "/experimentation/segments/{id}",
		URI:        cli.api.HTTPClient.URI("experimentation", "segments", id),
		Data:       dataStr,
		SchemaCmd:  "auth0 experimentation segments update",
	})
	if err != nil {
		return fmt.Errorf("failed to update segment %q: %w", id, err)
	}

	return cli.renderer.SegmentUpdate(result)
}

func deleteSegmentCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Short:   "Delete a segment",
		Long: "Delete a segment.\n\n" +
			"To delete interactively, use `auth0 experimentation segments delete` with no arguments.\n\n" +
			"To delete non-interactively, supply the segment ID and use `--force` to skip confirmation.",
		Example: `  auth0 experimentation segments delete
  auth0 experimentation segments rm
  auth0 experimentation segments delete <segment-id>
  auth0 experimentation segments delete <segment-id> --force
  auth0 experimentation segments delete <segment-id> <segment-id2> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var ids []string
			if len(args) == 0 {
				if err := segmentID.PickMany(cmd, &ids, cli.segmentPickerOptions); err != nil {
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

			return ansi.ProgressBar("Deleting segment(s)", ids, func(_ int, id string) error {
				if id != "" {
					// Delete returns 204 for an unknown ID, so check that it exists first.
					if _, err := cli.apiv3.Segments.Get(cmd.Context(), id); err != nil {
						return fmt.Errorf("failed to delete segment %q: %w", id, err)
					}

					if err := cli.apiv3.Segments.Delete(cmd.Context(), id); err != nil {
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
	page, err := c.apiv3.Segments.List(ctx, &management.ListSegmentsRequestParameters{})
	if err != nil {
		return nil, err
	}
	all, err := collectPages(ctx, page)
	if err != nil {
		return nil, err
	}

	var opts pickerOptions
	for _, s := range all {
		label := fmt.Sprintf("%s %s", s.GetName(), ansi.Faint("("+s.GetID()+")"))
		opts = append(opts, pickerOption{value: s.GetID(), label: label})
	}

	if len(opts) == 0 {
		return nil, errors.New("no segments available. Create one by running: `auth0 experimentation segments create`")
	}

	return opts, nil
}

func (c *cli) segmentRulesEditorHint() {
	c.renderer.Infof("Enter the segment rules as a JSON array. Each rule has a `match` and/or `not_match` object mapping an attribute to a condition.")
	c.renderer.Infof("Attributes: %s.", strings.Join(segmentAttributes, ", "))
	c.renderer.Infof(`Conditions: %s, or a plain list ["a","b"] for an exact match.`, strings.Join(segmentConditions, ", "))
	c.renderer.Infof(`Example: [{"match":{"domain":{"ends_with":["example.com"]}}}]`)
}

// parseSegmentRules decodes --rules into typed SDK rules. It builds each
// condition into the correct SegmentMatchExpression union variant by hand rather
// than unmarshaling straight into []*management.SegmentRule. The SDK's generated
// SegmentMatchExpression.UnmarshalJSON tries each object variant in declaration
// order and accepts the first that decodes without error — but Go's JSON decoder
// ignores unknown fields, so {"ends_with":[...]} decodes cleanly into the first
// object variant (SegmentContainsExpression) and is silently rewritten to
// {"contains":null}, which the API then rejects. Decoding generically and
// constructing the variant ourselves avoids that and lets us validate
// attributes/conditions with precise error messages.
func parseSegmentRules(raw string) ([]*management.SegmentRule, error) {
	// The interactive editor pre-fills a commented template; strip any // and
	// /* */ comments so that annotated input still parses as JSON.
	raw = stripJSONComments(raw)

	var generic []map[string]map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &generic); err != nil {
		return nil, validationError{err: fmt.Errorf("invalid JSON for --rules (ensure the value is quoted in your shell): %w", err), reason: "malformed_json"}
	}

	// The JSON is well-formed past this point, so the remaining errors describe
	// a rule the API wouldn't accept.
	invalid := func(err error) error { return usageError{err: err, reason: "invalid_flag_value"} }

	attrSet := sliceToSet(segmentAttributes)

	rules := make([]*management.SegmentRule, 0, len(generic))
	for i, rule := range generic {
		out := &management.SegmentRule{}
		for block, conditions := range rule {
			if block != "match" && block != "not_match" {
				return nil, invalid(fmt.Errorf("rule[%d].%s: unknown key — each rule may only have \"match\" and/or \"not_match\"", i, block))
			}
			for attr, expr := range conditions {
				if !attrSet[attr] {
					return nil, invalid(fmt.Errorf("rule[%d].%s.%s: unknown attribute — valid attributes are: %s", i, block, attr, strings.Join(segmentAttributes, ", ")))
				}
				me, err := parseMatchExpression(expr)
				if err != nil {
					return nil, invalid(fmt.Errorf("rule[%d].%s.%s%s", i, block, attr, err.Error()))
				}
				if err := setConditionField(out, block, attr, me); err != nil {
					return nil, invalid(fmt.Errorf("rule[%d].%s.%s: %w", i, block, attr, err))
				}
			}
		}
		if out.Match == nil && out.NotMatch == nil {
			return nil, invalid(fmt.Errorf(`rule[%d]: must have at least one "match" or "not_match" condition`, i))
		}
		rules = append(rules, out)
	}

	return rules, nil
}

// parseMatchExpression decodes a single condition value into the correct SDK
// union variant. A bare JSON list is an exact-match string list; an object must
// carry exactly one operator (contains, starts_with, ends_with, exists). Returned
// errors begin with a "." or ": " so callers can prefix the attribute path.
func parseMatchExpression(raw json.RawMessage) (*management.SegmentMatchExpression, error) {
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		return &management.SegmentMatchExpression{StringList: list}, nil
	}

	var opObj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &opObj); err != nil {
		return nil, errors.New(`: condition must be a JSON list ["a","b"] or an operator object like {"contains":["a"]}`)
	}
	if len(opObj) != 1 {
		return nil, fmt.Errorf(": a condition object must have exactly one operator — valid operators are: %s", strings.Join(segmentConditions, ", "))
	}

	for op, val := range opObj {
		switch op {
		case "contains", "starts_with", "ends_with":
			var v []string
			if err := json.Unmarshal(val, &v); err != nil {
				return nil, fmt.Errorf(".%s: expected a JSON list of strings", op)
			}
			switch op {
			case "contains":
				return &management.SegmentMatchExpression{SegmentContainsExpression: &management.SegmentContainsExpression{Contains: v}}, nil
			case "starts_with":
				return &management.SegmentMatchExpression{SegmentStartsWithExpression: &management.SegmentStartsWithExpression{StartsWith: v}}, nil
			default:
				return &management.SegmentMatchExpression{SegmentEndsWithExpression: &management.SegmentEndsWithExpression{EndsWith: v}}, nil
			}
		case "exists":
			var v bool
			if err := json.Unmarshal(val, &v); err != nil {
				return nil, errors.New(".exists: expected true or false")
			}
			return &management.SegmentMatchExpression{SegmentExistsExpression: &management.SegmentExistsExpression{Exists: v}}, nil
		default:
			return nil, fmt.Errorf(".%s: unknown condition — valid conditions are: %s, or a plain list for an exact match", op, strings.Join(segmentConditions, ", "))
		}
	}

	return nil, errors.New(": empty condition")
}

// setConditionField assigns a parsed expression onto the match/not_match
// conditions struct of a rule. The two conditions structs are distinct SDK types
// with identical fields, so it locates the field by its JSON tag via reflection
// (attr is already validated against segmentAttributes by the caller).
func setConditionField(rule *management.SegmentRule, block, attr string, expr *management.SegmentMatchExpression) error {
	var conditions reflect.Value
	switch block {
	case "match":
		if rule.Match == nil {
			rule.Match = &management.SegmentMatchConditions{}
		}
		conditions = reflect.ValueOf(rule.Match).Elem()
	case "not_match":
		if rule.NotMatch == nil {
			rule.NotMatch = &management.SegmentNotMatchConditions{}
		}
		conditions = reflect.ValueOf(rule.NotMatch).Elem()
	}

	t := conditions.Type()
	for i := 0; i < t.NumField(); i++ {
		if strings.Split(t.Field(i).Tag.Get("json"), ",")[0] == attr {
			conditions.Field(i).Set(reflect.ValueOf(expr))
			return nil
		}
	}

	return fmt.Errorf("no field for attribute %q", attr)
}

func sliceToSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}

// stripJSONComments removes // line comments and /* */ block comments from a
// JSON document, ignoring markers that appear inside string literals (so a
// value like "https://example.com" is left intact). It lets the interactive
// editor carry a self-documenting, commented template that is discarded before
// the JSON is parsed.
func stripJSONComments(raw string) string {
	var b strings.Builder
	b.Grow(len(raw))

	inString := false
	escaped := false
	for i := 0; i < len(raw); i++ {
		c := raw[i]

		if inString {
			b.WriteByte(c)
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}

		switch {
		case c == '"':
			inString = true
			b.WriteByte(c)
		case c == '/' && i+1 < len(raw) && raw[i+1] == '/':
			// Line comment: skip to (but keep) the end of the line.
			for i+1 < len(raw) && raw[i+1] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(raw) && raw[i+1] == '*':
			// Block comment: skip through the closing */.
			i += 2
			for i+1 < len(raw) && (raw[i] != '*' || raw[i+1] != '/') {
				i++
			}
			i++ // Consume the '/' of '*/'.
		default:
			b.WriteByte(c)
		}
	}

	return b.String()
}

// segmentRulesHelpComment renders the // comment header that documents the rule
// grammar. It is prepended to the interactive editor buffer so the valid
// attributes, conditions, and structure are visible while editing; the comments
// are stripped on save. The attribute/condition lists are the same
// reflection-derived sets used for validation, so they never drift.
func segmentRulesHelpComment() string {
	conditions := append(append([]string{}, segmentConditions...), `or a plain list ["a","b"] for an exact match`)
	return "// A user matches this segment if ANY rule below matches.\n" +
		"// Each rule has a \"match\" and/or \"not_match\" object mapping an\n" +
		"// attribute to a condition. Multiple attributes in one object are ANDed.\n" +
		commentList("Attributes:", segmentAttributes) +
		commentList("Conditions:", conditions) +
		"// (// and /* */ comments are removed before saving.)\n"
}

// segmentRulesTemplate is the full editor buffer for `create`: the commented
// grammar guide followed by an editable starter rule and two commented-out
// examples showing not_match and the exists condition.
func segmentRulesTemplate() string {
	return segmentRulesHelpComment() +
		"[\n" +
		"  {\n" +
		"    \"match\": {\n" +
		"      \"country\": [\"US\", \"CA\"],\n" +
		"      \"domain\": { \"ends_with\": [\"example.com\"] }\n" +
		"    }\n" +
		"  }\n" +
		"  // ,{ \"not_match\": { \"browser\": [\"safari\"] } }\n" +
		"  // ,{ \"match\": { \"organization_id\": { \"exists\": true } } }\n" +
		"]\n"
}

// commentList renders "intro item, item, ..." as one or more "// " lines,
// greedily wrapping near a readable width so long lists stay legible in a
// terminal editor.
func commentList(intro string, items []string) string {
	const maxLine = 60

	var lines []string
	cur := intro
	for i, it := range items {
		tok := it
		if i < len(items)-1 {
			tok += ","
		}
		if candidate := cur + " " + tok; len(candidate) > maxLine && cur != intro {
			lines = append(lines, cur)
			cur = "  " + tok
		} else {
			cur = candidate
		}
	}
	lines = append(lines, cur)

	var b strings.Builder
	for _, l := range lines {
		b.WriteString("// " + l + "\n")
	}
	return b.String()
}
