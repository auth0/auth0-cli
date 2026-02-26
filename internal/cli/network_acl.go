package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/prompt"
)

var (
	networkACLID = Argument{
		Name: "Id",
		Help: "Id of the network ACL",
	}

	networkACLDescription = Flag{
		Name:      "Description",
		LongForm:  "description",
		ShortForm: "d",
		Help:      "Description of the network ACL (Eg. \"Block suspicious IPs\", required)",
	}

	networkACLActive = Flag{
		Name:     "Active",
		LongForm: "active",
		Help:     "Whether the network ACL is active (Eg. true, default: false)",
	}

	networkACLPriority = Flag{
		Name:      "Priority",
		LongForm:  "priority",
		ShortForm: "p",
		Help:      "Priority of the network ACL in number(Eg. 5)",
	}

	networkACLRuleAction = Flag{
		Name:     "Action",
		LongForm: "action",
		Help:     "Action for the rule (block, allow, log, redirect)",
	}

	networkACLRedirectURI = Flag{
		Name:     "RedirectURI",
		LongForm: "redirect-uri",
		Help:     "URI to redirect to when action is redirect (Eg. \"https://example.com/blocked\")",
	}

	networkACLASNs = Flag{
		Name:     "ASNs",
		LongForm: "asns",
		Help:     "Comma-separated list of ASNs to match (Eg. 64496,64497,64498)",
	}

	networkACLCountryCodes = Flag{
		Name:     "CountryCodes",
		LongForm: "country-codes",
		Help:     "Comma-separated list of country codes to match (Eg. US,CA,MX)",
	}

	networkACLSubdivisionCodes = Flag{
		Name:     "SubdivisionCodes",
		LongForm: "subdivision-codes",
		Help:     "Comma-separated list of subdivision codes to match (Eg. US-NY,US-CA)",
	}

	networkACLIPv4CIDRs = Flag{
		Name:     "IPv4CIDRs",
		LongForm: "ipv4-cidrs",
		Help:     "Comma-separated list of IPv4 CIDR ranges (Eg. 192.168.1.0/24,10.0.0.0/8)",
	}

	networkACLIPv6CIDRs = Flag{
		Name:     "IPv6CIDRs",
		LongForm: "ipv6-cidrs",
		Help:     "Comma-separated list of IPv6 CIDR ranges (Eg. 2001:db8::/32,2001:db8:1234::/48)",
	}

	networkACLJA3Fingerprints = Flag{
		Name:     "JA3Fingerprints",
		LongForm: "ja3-fingerprints",
		Help:     "Comma-separated list of JA3 fingerprints to match (Eg. deadbeef,cafebabe)",
	}

	networkACLJA4Fingerprints = Flag{
		Name:     "JA4Fingerprints",
		LongForm: "ja4-fingerprints",
		Help:     "Comma-separated list of JA4 fingerprints to match (Eg. t13d1516h2_8daaf6152771)",
	}

	networkACLUserAgents = Flag{
		Name:     "UserAgents",
		LongForm: "user-agents",
		Help:     "Comma-separated list of user agents to match (Eg. badbot/*,malicious/*)",
	}
)

// validateAndSetBasicFields handles the common validation and patch building logic for basic fields.
func validateAndSetBasicFields(inputs *struct {
	ID           string
	Description  string
	Active       bool
	ActiveStr    string
	Priority     int
	RuleJSON     string
	Action       string
	RedirectURI  string
	Scope        string
	ASNs         []int
	CountryCodes []string
	SubdivCodes  []string
	IPv4CIDRs    []string
	IPv6CIDRs    []string
	JA3          []string
	JA4          []string
	UserAgents   []string
	MatchRule    bool
	NoMatchRule  bool
}, patch *management.NetworkACL, cmd *cobra.Command) error {
	if cmd.Flags().Changed("description") {
		if len(inputs.Description) > 255 {
			return fmt.Errorf("description cannot exceed 255 characters")
		}
		if len(inputs.Description) == 0 {
			return fmt.Errorf("description cannot be empty")
		}
		patch.Description = &inputs.Description
	}

	if cmd.Flags().Changed("active") {
		switch inputs.ActiveStr {
		case "true":
			inputs.Active = true
		case "false":
			inputs.Active = false
		default:
			return fmt.Errorf("--active must be either 'true' or 'false', got %q", inputs.ActiveStr)
		}
		patch.Active = &inputs.Active
	}

	if cmd.Flags().Changed("priority") {
		patch.Priority = &inputs.Priority
	}

	if cmd.Flags().Changed("rule") {
		var rule management.NetworkACLRule
		if err := json.Unmarshal([]byte(inputs.RuleJSON), &rule); err != nil {
			return fmt.Errorf("invalid rule JSON: %w", err)
		}
		patch.Rule = &rule
	}

	return nil
}

// applyNetworkACLPatch handles the common API call and rendering logic.
func applyNetworkACLPatch(ctx context.Context, cli *cli, id string, patch *management.NetworkACL) error {
	if err := ansi.Waiting(func() error {
		return cli.api.NetworkACL.Patch(ctx, id, patch)
	}); err != nil {
		return fmt.Errorf("failed to update network ACL with ID %q: %w", id, err)
	}

	cli.renderer.NetworkACLUpdate(patch)
	return nil
}

func selectNetworkACLParams(cmd *cobra.Command) (map[string]bool, error) {
	options := []string{
		"ASNs",
		"Country Codes",
		"Subdivision Codes",
		"IPv4CIDRs",
		"IPv6CIDRs",
		"JA3Fingerprints",
		"JA4Fingerprints",
		"User Agents",
	}

	var selected []string
	if err := prompt.AskMultiSelect(
		"Please select the desired parameters using the spacebar and press Enter to confirm.\n"+
			ansi.Faint(" Only the selected parameters will be reflected in the final state:"),
		&selected,
		options...,
	); err != nil {
		return nil, err
	}

	if len(selected) == 0 {
		return nil, errors.New("at least one parameter must be selected")
	}

	// Convert selected slice to map for easier lookup.
	selectedParams := make(map[string]bool)
	for _, opt := range selected {
		selectedParams[opt] = true
	}

	return selectedParams, nil
}

// ruleDefaults holds default values extracted from current ACL rule.
type ruleDefaults struct {
	Scope        string
	Action       string
	RedirectURI  string
	ASNs         []int
	CountryCodes []string
	SubdivCodes  []string
	IPv4CIDRs    []string
	IPv6CIDRs    []string
	JA3          []string
	JA4          []string
	UserAgents   []string
	IsMatchRule  bool
	HasMatchRule bool
	HasNotMatch  bool
}

// extractCurrentRuleDefaults extracts default values from current ACL rule for interactive prompts.
func extractCurrentRuleDefaults(currentACL *management.NetworkACL) *ruleDefaults {
	defaults := &ruleDefaults{}

	if currentACL == nil || currentACL.Rule == nil {
		defaults.Scope = "tenant"
		defaults.Action = "block"
		return defaults
	}

	// Extract scope.
	if currentACL.Rule.Scope != nil {
		defaults.Scope = *currentACL.Rule.Scope
	}

	// Extract action.
	if currentACL.Rule.Action != nil {
		switch {
		case currentACL.Rule.Action.Block != nil && *currentACL.Rule.Action.Block:
			defaults.Action = "block"
		case currentACL.Rule.Action.Allow != nil && *currentACL.Rule.Action.Allow:
			defaults.Action = "allow"
		case currentACL.Rule.Action.Log != nil && *currentACL.Rule.Action.Log:
			defaults.Action = "log"
		case currentACL.Rule.Action.Redirect != nil && *currentACL.Rule.Action.Redirect:
			defaults.Action = "redirect"
			if currentACL.Rule.Action.RedirectURI != nil {
				defaults.RedirectURI = *currentACL.Rule.Action.RedirectURI
			}
		}
	}

	// Extract match criteria from either Match or NotMatch.
	var match *management.NetworkACLRuleMatch
	if currentACL.Rule.Match != nil {
		match = currentACL.Rule.Match
		defaults.IsMatchRule = true
		defaults.HasMatchRule = true
	} else if currentACL.Rule.NotMatch != nil {
		match = currentACL.Rule.NotMatch
		defaults.IsMatchRule = false
		defaults.HasNotMatch = true
	}

	if match != nil {
		if len(match.Asns) > 0 {
			defaults.ASNs = match.Asns
		}
		if match.GeoCountryCodes != nil {
			defaults.CountryCodes = *match.GeoCountryCodes
		}
		if match.GeoSubdivisionCodes != nil {
			defaults.SubdivCodes = *match.GeoSubdivisionCodes
		}
		if match.IPv4Cidrs != nil {
			defaults.IPv4CIDRs = *match.IPv4Cidrs
		}
		if match.IPv6Cidrs != nil {
			defaults.IPv6CIDRs = *match.IPv6Cidrs
		}
		if match.Ja3Fingerprints != nil {
			defaults.JA3 = *match.Ja3Fingerprints
		}
		if match.Ja4Fingerprints != nil {
			defaults.JA4 = *match.Ja4Fingerprints
		}
		if match.UserAgents != nil {
			defaults.UserAgents = *match.UserAgents
		}
	}

	return defaults
}

// ruleInputs holds user inputs for rule configuration.
type ruleInputs struct {
	Scope        string
	Action       string
	RedirectURI  string
	ASNs         []int
	CountryCodes []string
	SubdivCodes  []string
	IPv4CIDRs    []string
	IPv6CIDRs    []string
	JA3          []string
	JA4          []string
	UserAgents   []string
	IsMatchRule  bool
	MatchRule    bool
	NoMatchRule  bool
}

// promptForRuleDetails handles interactive prompting for rule configuration.
func promptForRuleDetails(cmd *cobra.Command, cli *cli, defaults *ruleDefaults, isUpdate bool) (*ruleInputs, error) {
	inputs := &ruleInputs{}

	cli.renderer.Infof("Define the rule for the network ACL.\n")

	// Ask for scope.
	scopes := []string{"management", "authentication", "tenant"}
	if err := (&Flag{
		Name: "Scope",
		Help: "Scope of the rule (management, authentication, tenant)",
	}).Select(cmd, &inputs.Scope, scopes, &defaults.Scope); err != nil {
		return nil, err
	}

	// Ask for action.
	actions := []string{"block", "allow", "log", "redirect"}
	if err := networkACLRuleAction.Select(cmd, &inputs.Action, actions, &defaults.Action); err != nil {
		return nil, err
	}

	// If action is redirect, ask for redirect URI.
	if inputs.Action == "redirect" {
		if err := networkACLRedirectURI.Ask(cmd, &inputs.RedirectURI, &defaults.RedirectURI); err != nil {
			return nil, err
		}
		if inputs.RedirectURI == "" {
			return nil, fmt.Errorf("redirect URI is required when action is redirect")
		}
	}

	// Handle Match/NotMatch rule changes for updates.
	if isUpdate {
		if defaults.HasMatchRule {
			if err := prompt.AskBool("The current rule uses 'Match' criteria. Do you want to change it to 'NotMatch'?", &inputs.NoMatchRule, false); err != nil {
				return nil, err
			}
		}
		if defaults.HasNotMatch {
			if err := prompt.AskBool("The current rule uses 'NotMatch' criteria. Do you want to change it to 'Match'?", &inputs.MatchRule, false); err != nil {
				return nil, err
			}
		}
		// If no change requested, preserve current rule type.
		if !inputs.NoMatchRule && !inputs.MatchRule {
			inputs.IsMatchRule = defaults.IsMatchRule
		} else {
			inputs.IsMatchRule = inputs.MatchRule
		}
	} else {
		// For create, ask for Match or NotMatch rule.
		matchOptions := []string{"match", "not_match"}
		var selectedMatchOption string
		if err := (&Flag{
			Name: "What kind of rule do you want to create?",
			Help: "Match or Not Match rule (ASNs, Country Codes, Subdivision Codes, IPv4 CIDRs, IPv6 CIDRs, JA3/JA4 Fingerprints, User Agents)",
		}).Select(cmd, &selectedMatchOption, matchOptions, nil); err != nil {
			return nil, err
		}
		inputs.IsMatchRule = selectedMatchOption == "match"
	}

	// Select which parameters to provide.
	selectedParams, err := selectNetworkACLParams(cmd)
	if err != nil {
		return nil, err
	}

	// Ask for values only for selected parameters.
	if err := promptForMatchCriteria(cmd, selectedParams, inputs, defaults); err != nil {
		return nil, err
	}

	return inputs, nil
}

// promptForMatchCriteria handles prompting for all match criteria based on selected parameters.
func promptForMatchCriteria(cmd *cobra.Command, selectedParams map[string]bool, inputs *ruleInputs, defaults *ruleDefaults) error {
	if selectedParams["ASNs"] {
		if err := networkACLASNs.AskIntSlice(cmd, &inputs.ASNs, &defaults.ASNs); err != nil {
			return err
		}
	}

	if selectedParams["Country Codes"] {
		currentCountryCodesStr := strings.Join(defaults.CountryCodes, ",")
		if err := networkACLCountryCodes.AskMany(cmd, &inputs.CountryCodes, &currentCountryCodesStr); err != nil {
			return err
		}
	}

	if selectedParams["Subdivision Codes"] {
		currentSubDivCodesStr := strings.Join(defaults.SubdivCodes, ",")
		if err := networkACLSubdivisionCodes.AskMany(cmd, &inputs.SubdivCodes, &currentSubDivCodesStr); err != nil {
			return err
		}
	}

	if selectedParams["IPv4CIDRs"] {
		currentIPv4CIDRsStr := strings.Join(defaults.IPv4CIDRs, ",")
		if err := networkACLIPv4CIDRs.AskMany(cmd, &inputs.IPv4CIDRs, &currentIPv4CIDRsStr); err != nil {
			return err
		}
	}

	if selectedParams["IPv6CIDRs"] {
		currentIPv6CIDRsStr := strings.Join(defaults.IPv6CIDRs, ",")
		if err := networkACLIPv6CIDRs.AskMany(cmd, &inputs.IPv6CIDRs, &currentIPv6CIDRsStr); err != nil {
			return err
		}
	}

	if selectedParams["JA3Fingerprints"] {
		currentJA3Str := strings.Join(defaults.JA3, ",")
		if err := networkACLJA3Fingerprints.AskMany(cmd, &inputs.JA3, &currentJA3Str); err != nil {
			return err
		}
	}

	if selectedParams["JA4Fingerprints"] {
		currentJA4Str := strings.Join(defaults.JA4, ",")
		if err := networkACLJA4Fingerprints.AskMany(cmd, &inputs.JA4, &currentJA4Str); err != nil {
			return err
		}
	}

	if selectedParams["User Agents"] {
		currentUserAgentsStr := strings.Join(defaults.UserAgents, ",")
		if err := networkACLUserAgents.AskMany(cmd, &inputs.UserAgents, &currentUserAgentsStr); err != nil {
			return err
		}
	}

	return nil
}

// buildNetworkACLRule creates a NetworkACLRule from the provided inputs.
func buildNetworkACLRule(inputs *ruleInputs) (*management.NetworkACLRule, error) {
	rule := &management.NetworkACLRule{
		Scope: &inputs.Scope,
	}

	// Set the action.
	rule.Action = &management.NetworkACLRuleAction{}
	switch inputs.Action {
	case "block":
		rule.Action.Block = auth0.Bool(true)
	case "allow":
		rule.Action.Allow = auth0.Bool(true)
	case "log":
		rule.Action.Log = auth0.Bool(true)
	case "redirect":
		rule.Action.Redirect = auth0.Bool(true)
		rule.Action.RedirectURI = &inputs.RedirectURI
	}

	// Build match criteria.
	match := &management.NetworkACLRuleMatch{}
	matchProvided := false

	if len(inputs.ASNs) > 0 {
		match.Asns = inputs.ASNs
		matchProvided = true
	}
	if len(inputs.CountryCodes) > 0 {
		match.GeoCountryCodes = &inputs.CountryCodes
		matchProvided = true
	}
	if len(inputs.SubdivCodes) > 0 {
		match.GeoSubdivisionCodes = &inputs.SubdivCodes
		matchProvided = true
	}
	if len(inputs.IPv4CIDRs) > 0 {
		match.IPv4Cidrs = &inputs.IPv4CIDRs
		matchProvided = true
	}
	if len(inputs.IPv6CIDRs) > 0 {
		match.IPv6Cidrs = &inputs.IPv6CIDRs
		matchProvided = true
	}
	if len(inputs.JA3) > 0 {
		match.Ja3Fingerprints = &inputs.JA3
		matchProvided = true
	}
	if len(inputs.JA4) > 0 {
		match.Ja4Fingerprints = &inputs.JA4
		matchProvided = true
	}
	if len(inputs.UserAgents) > 0 {
		match.UserAgents = &inputs.UserAgents
		matchProvided = true
	}

	if !matchProvided {
		return nil, fmt.Errorf("at least one match criteria must be provided")
	}

	// Set match or notmatch based on user choice.
	if inputs.IsMatchRule {
		rule.Match = match
	} else {
		rule.NotMatch = match
	}

	return rule, nil
}

func networkACLCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network-acl",
		Short: "Manage network ACL settings",
		Long: `Manage network access control list (ACL) settings for your tenant.
Network ACLs allow you to control access to your applications based on IP addresses,
country codes, anonymous proxies, and other criteria.`,
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listNetworkACLsCmd(cli))
	cmd.AddCommand(createNetworkACLCmd(cli))
	cmd.AddCommand(showNetworkACLCmd(cli))
	cmd.AddCommand(updateNetworkACLCmd(cli))
	cmd.AddCommand(deleteNetworkACLCmd(cli))

	return cmd
}

func listNetworkACLsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List network ACLs",
		Long:    "List your network ACLs. To create one, run: auth0 network-acl create",
		Example: `  auth0 network-acl list
  auth0 network-acl ls
  auth0 network-acl ls --json
  auth0 network-acl ls --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var list []*management.NetworkACL
			if err := ansi.Waiting(func() error {
				var err error
				list, err = cli.api.NetworkACL.List(cmd.Context())
				return err
			}); err != nil {
				return fmt.Errorf("failed to list network ACLs: %w", err)
			}

			cli.renderer.NetworkACLList(list)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in JSON format")
	return cmd
}

func showNetworkACLCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.MaximumNArgs(1),
		Short: "Show a network ACL",
		Long:  "Show the details of a network ACL.",
		Example: `  auth0 network-acl show <id>
  auth0 network-acl show <id> --json
  auth0 network-acl show <id> --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := networkACLID.Pick(cmd, &inputs.ID, cli.networkACLPickerOptions); err != nil {
					return err
				}
			}

			acl, err := cli.api.NetworkACL.Read(cmd.Context(), inputs.ID)
			if err != nil {
				return fmt.Errorf("failed to get network ACL with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.NetworkACLShow(acl)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in JSON format")

	return cmd
}

func createNetworkACLCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Description  string
		Active       bool
		ActiveStr    string // Added for handling --active true/false.
		Priority     int
		RuleJSON     string
		Action       string
		RedirectURI  string
		ASNs         []int
		CountryCodes []string
		SubdivCodes  []string
		IPv4CIDRs    []string
		IPv6CIDRs    []string
		JA3          []string
		JA4          []string
		UserAgents   []string
		Scope        string
		isMatchRule  bool
	}

	cmd := &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new network ACL",
		Long: `Create a new network ACL.
To create interactively, use "auth0 network-acl create" with no arguments.
To create non-interactively, supply the required parameters (description, active, priority, and rule) through flags.
The --rule parameter is required and must contain a valid JSON object with action, scope, and match properties.`,
		Example: `  auth0 network-acl create
  auth0 network-acl create --description "Block IPs" --priority 1 --active true --rule '{"action":{"block":true},"scope":"tenant","match":{"ipv4_cidrs":["192.168.1.0/24","10.0.0.0/8"]}}'
  auth0 network-acl create --description "Geo Block" --priority 2 --active true --rule '{"action":{"block":true},"scope":"authentication","match":{"geo_country_codes":["US","CA"]}}'
  auth0 network-acl create --description "Redirect Traffic" --priority 3 --active true --rule '{"action":{"redirect":true,"redirect_uri":"https://example.com"},"scope":"management","match":{"ipv4_cidrs":["192.168.1.0/24"]}}'
  auth0 network-acl create -d "Block Bots" -p 4 --active true --rule '{"action":{"block":true},"scope":"tenant","match":{"user_agents":["badbot/*","malicious/*"],"ja3_fingerprints":["deadbeef","cafebabe"]}}'
  auth0 network-acl create --description "Complex Rule" --priority 5 --active true --rule '{"action":{"block":true},"scope":"tenant","match":{"ipv4_cidrs":["192.168.1.0/24"],"geo_country_codes":["US"]}}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if we're in non-interactive mode (flags provided) but rule JSON is missing.
			if !canPrompt(cmd) && !cmd.Flags().Changed("rule") {
				return fmt.Errorf("the --rule parameter is required for non-interactive mode. Please provide a valid JSON rule")
			}

			// Parse the active flag if provided.
			if cmd.Flags().Changed("active") {
				switch inputs.ActiveStr {
				case "true":
					inputs.Active = true
				case "false":
					inputs.Active = false
				default:
					return fmt.Errorf("--active must be either 'true' or 'false', got %q", inputs.ActiveStr)
				}
			}

			// Check if rule JSON was provided.
			if cmd.Flags().Changed("rule") {
				// Parse the rule JSON.
				var rule map[string]interface{}
				if err := json.Unmarshal([]byte(inputs.RuleJSON), &rule); err != nil {
					return fmt.Errorf("invalid rule JSON: %w", err)
				}

				// Create the network ACL with the provided rule.
				acl := &management.NetworkACL{
					Description: &inputs.Description,
					Active:      &inputs.Active,
					Priority:    &inputs.Priority,
				}

				// Convert the rule map to the appropriate structure.
				if err := json.Unmarshal([]byte(inputs.RuleJSON), &acl.Rule); err != nil {
					return fmt.Errorf("failed to parse rule JSON: %w", err)
				}

				if err := ansi.Waiting(func() error {
					return cli.api.NetworkACL.Create(cmd.Context(), acl)
				}); err != nil {
					return fmt.Errorf("failed to create network ACL: %w", err)
				}

				cli.renderer.NetworkACLCreate(acl)
				return nil
			}

			// Interactive or flag-based creation.
			if err := networkACLDescription.Ask(cmd, &inputs.Description, nil); err != nil {
				return err
			}

			if len(inputs.Description) > 255 {
				return fmt.Errorf("description cannot exceed 255 characters")
			}

			if len(inputs.Description) == 0 {
				return fmt.Errorf("description is required")
			}

			defaultStatus := false
			if err := networkACLActive.AskBool(cmd, &inputs.Active, &defaultStatus); err != nil {
				return err
			}

			if err := networkACLPriority.AskInt(cmd, &inputs.Priority, nil); err != nil {
				return err
			}

			// Use helper functions for rule configuration.
			defaults := &ruleDefaults{
				Scope:  "tenant",
				Action: "log",
			}

			ruleInputs, err := promptForRuleDetails(cmd, cli, defaults, false)
			if err != nil {
				return err
			}

			// Build the network ACL.
			acl := &management.NetworkACL{
				Description: &inputs.Description,
				Active:      &inputs.Active,
				Priority:    &inputs.Priority,
			}

			// Build the rule.
			acl.Rule, err = buildNetworkACLRule(ruleInputs)
			if err != nil {
				return err
			}

			if err := ansi.Waiting(func() error {
				return cli.api.NetworkACL.Create(cmd.Context(), acl)
			}); err != nil {
				return fmt.Errorf("failed to create network ACL: %w", err)
			}

			cli.renderer.NetworkACLCreate(acl)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().StringVarP(&inputs.Description, "description", "d", "", "Description of the network ACL (required)")
	cmd.Flags().StringVar(&inputs.ActiveStr, "active", "", "Whether the network ACL is active (required, 'true' or 'false')")
	cmd.Flags().IntVarP(&inputs.Priority, "priority", "p", 0, "Priority of the network ACL (required)")
	cmd.Flags().StringVar(&inputs.RuleJSON, "rule", "", "Network ACL rule configuration in JSON format (required for non-interactive mode)")
	cmd.Flags().StringVar(&inputs.Action, "action", "", "Action for the rule (block, allow, log, redirect)")
	cmd.Flags().StringVar(&inputs.RedirectURI, "redirect-uri", "", "URI to redirect to when action is redirect")
	cmd.Flags().StringVar(&inputs.Scope, "scope", "", "Scope of the rule (management, authentication, tenant)")

	// Register the string slice flags.
	networkACLASNs.RegisterIntSlice(cmd, &inputs.ASNs, nil)
	networkACLCountryCodes.RegisterStringSlice(cmd, &inputs.CountryCodes, nil)
	networkACLSubdivisionCodes.RegisterStringSlice(cmd, &inputs.SubdivCodes, nil)
	networkACLIPv4CIDRs.RegisterStringSlice(cmd, &inputs.IPv4CIDRs, nil)
	networkACLIPv6CIDRs.RegisterStringSlice(cmd, &inputs.IPv6CIDRs, nil)
	networkACLJA3Fingerprints.RegisterStringSlice(cmd, &inputs.JA3, nil)
	networkACLJA4Fingerprints.RegisterStringSlice(cmd, &inputs.JA4, nil)
	networkACLUserAgents.RegisterStringSlice(cmd, &inputs.UserAgents, nil)

	cmd.MarkFlagRequired("description")
	cmd.MarkFlagRequired("active")
	cmd.MarkFlagRequired("priority")
	cmd.MarkFlagRequired("rule")
	return cmd
}

func updateNetworkACLCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ID           string
		Description  string
		Active       bool
		ActiveStr    string
		Priority     int
		RuleJSON     string
		Action       string
		RedirectURI  string
		Scope        string
		ASNs         []int
		CountryCodes []string
		SubdivCodes  []string
		IPv4CIDRs    []string
		IPv6CIDRs    []string
		JA3          []string
		JA4          []string
		UserAgents   []string
		MatchRule    bool
		NoMatchRule  bool
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update a network ACL",
		Long: `Update a network ACL.
To update interactively, use "auth0 network-acl update" with no arguments.
To update non-interactively, supply the description, active, priority, and rule through flags.
`,
		Example: `  auth0 network-acl update <id>
  auth0 network-acl update <id> --priority 5 
  auth0 network-acl update <id> --active true
  auth0 network-acl update <id> --description "Updated description"
  auth0 network-acl update <id> --rule '{"action":{"block":true},"scope":"tenant","match":{"ipv4_cidrs":["192.168.1.0/24"]}}'
  auth0 network-acl update <id> --description "Complex Rule updated" --priority 1 --active true --rule '{"action":{"block":true},"scope":"tenant","match":{"ipv4_cidrs":["192.168.1.0/24"],"geo_country_codes":["US"]}}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get the network ACL ID.
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := networkACLID.Pick(cmd, &inputs.ID, cli.networkACLPickerOptions); err != nil {
					return err
				}
			}

			// Check if we're in non-interactive mode (any flags provided).
			flagsProvided := cmd.Flags().Changed("description") || cmd.Flags().Changed("active") ||
				cmd.Flags().Changed("priority") || cmd.Flags().Changed("rule")

			if !canPrompt(cmd) && !flagsProvided {
				return fmt.Errorf("in non-interactive mode, at least one field must be specified to update")
			}

			// Build patch object with only the fields that should be updated.
			patch := &management.NetworkACL{}

			// Non-interactive mode with flags only - no need to read current ACL.
			if !canPrompt(cmd) && flagsProvided {
				// Validate and set basic fields from flags.
				if err := validateAndSetBasicFields(&inputs, patch, cmd); err != nil {
					return err
				}

				// Apply the patch.
				return applyNetworkACLPatch(cmd.Context(), cli, inputs.ID, patch)
			}

			// Interactive mode - read current ACL first for defaults.
			var currentACL *management.NetworkACL
			err := ansi.Waiting(func() (err error) {
				currentACL, err = cli.api.NetworkACL.Read(cmd.Context(), inputs.ID)
				return err
			})
			if err != nil {
				return fmt.Errorf("failed to get network ACL with ID %q: %w", inputs.ID, err)
			}

			// If some flags were provided in interactive mode, only update those fields.
			if canPrompt(cmd) && flagsProvided {
				// Only update the fields that were specified via flags.
				if err := validateAndSetBasicFields(&inputs, patch, cmd); err != nil {
					return err
				}

				// Apply the patch.
				return applyNetworkACLPatch(cmd.Context(), cli, inputs.ID, patch)
			}

			// Full Interactive mode, use current values as defaults for interactive prompts.
			currentDescriptionStr := *currentACL.Description
			if err := networkACLDescription.Ask(cmd, &inputs.Description, &currentDescriptionStr); err != nil {
				return err
			}
			patch.Description = &inputs.Description

			if err := networkACLActive.AskBool(cmd, &inputs.Active, currentACL.Active); err != nil {
				return err
			}
			patch.Active = &inputs.Active

			currentPriorityStr := fmt.Sprintf("%d", *currentACL.Priority)
			if err := networkACLPriority.AskInt(cmd, &inputs.Priority, &currentPriorityStr); err != nil {
				return err
			}

			patch.Priority = &inputs.Priority

			// Use helper functions for rule configuration.
			defaults := extractCurrentRuleDefaults(currentACL)

			ruleInputs, err := promptForRuleDetails(cmd, cli, defaults, true)
			if err != nil {
				return err
			}

			// Build the rule for the patch.
			patch.Rule, err = buildNetworkACLRule(ruleInputs)
			if err != nil {
				return err
			}

			// Apply the patch.
			return applyNetworkACLPatch(cmd.Context(), cli, inputs.ID, patch)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in JSON format")
	cmd.Flags().StringVarP(&inputs.Description, "description", "d", "", "Description of the network ACL")
	cmd.Flags().StringVar(&inputs.ActiveStr, "active", "", "Whether the network ACL is active ('true' or 'false')")
	cmd.Flags().IntVarP(&inputs.Priority, "priority", "p", 1, "Priority of the network ACL")
	cmd.Flags().StringVar(&inputs.RuleJSON, "rule", "", "Network ACL rule configuration in JSON format")
	cmd.Flags().StringVar(&inputs.Action, "action", "", "Action for the rule (block, allow, log, redirect)")
	cmd.Flags().StringVar(&inputs.RedirectURI, "redirect-uri", "", "URI to redirect to when action is redirect")
	cmd.Flags().StringVar(&inputs.Scope, "scope", "", "Scope of the rule (management, authentication, tenant)")

	// Register the string slice flags.
	networkACLASNs.RegisterIntSlice(cmd, &inputs.ASNs, nil)
	networkACLCountryCodes.RegisterStringSlice(cmd, &inputs.CountryCodes, nil)
	networkACLSubdivisionCodes.RegisterStringSlice(cmd, &inputs.SubdivCodes, nil)
	networkACLIPv4CIDRs.RegisterStringSlice(cmd, &inputs.IPv4CIDRs, nil)
	networkACLIPv6CIDRs.RegisterStringSlice(cmd, &inputs.IPv6CIDRs, nil)
	networkACLJA3Fingerprints.RegisterStringSlice(cmd, &inputs.JA3, nil)
	networkACLJA4Fingerprints.RegisterStringSlice(cmd, &inputs.JA4, nil)
	networkACLUserAgents.RegisterStringSlice(cmd, &inputs.UserAgents, nil)

	return cmd
}

func deleteNetworkACLCmd(cli *cli) *cobra.Command {
	var inputs struct {
		All bool
	}

	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Args:    cobra.MaximumNArgs(1),
		Short:   "Delete a network ACL",
		Long: `Delete a network ACL.
To delete interactively, use "auth0 network-acl delete" with no arguments.
To delete non-interactively, supply the network ACL ID and --force flag to skip confirmation.
Use --all flag to delete all network ACLs at once.`,
		Example: `  auth0 network-acl delete
  auth0 network-acl delete <id>
  auth0 network-acl delete <id> --force
  auth0 network-acl delete --all
  auth0 network-acl delete --all --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if --all flag is set.
			if inputs.All {
				// Get all network ACLs.
				var list []*management.NetworkACL
				if err := ansi.Waiting(func() error {
					var err error
					list, err = cli.api.NetworkACL.List(cmd.Context())
					return err
				}); err != nil {
					return fmt.Errorf("failed to list network ACLs: %w", err)
				}

				if len(list) == 0 {
					fmt.Println("No network ACLs found to delete.")
					return nil
				}

				// Confirm deletion.
				if !cli.force && canPrompt(cmd) {
					if confirmed := prompt.Confirm(fmt.Sprintf("Are you sure you want to delete ALL %d network ACLs?", len(list))); !confirmed {
						return nil
					}
				}

				// Delete all ACLs with progress bar.
				return ansi.ProgressBar("Deleting all network ACLs", list, func(i int, acl *management.NetworkACL) error {
					if acl != nil && acl.ID != nil {
						return cli.api.NetworkACL.Delete(cmd.Context(), *acl.ID)
					}
					return nil
				})
			}

			// Regular single or multiple ACL delete flow.
			var ids []string
			if len(args) == 0 {
				if err := networkACLID.PickMany(cmd, &ids, cli.networkACLPickerOptions); err != nil {
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

			return ansi.ProgressBar("Deleting network ACL(s)", ids, func(i int, id string) error {
				if id != "" {
					if err := cli.api.NetworkACL.Delete(cmd.Context(), id); err != nil {
						return fmt.Errorf("failed to delete network ACL with ID %q: %w", id, err)
					}
				}
				return nil
			})
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation")
	cmd.Flags().BoolVar(&inputs.All, "all", false, "Delete all network ACLs")

	return cmd
}

func (c *cli) networkACLPickerOptions(ctx context.Context) (pickerOptions, error) {
	list, err := c.api.NetworkACL.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list network ACLs: %w", err)
	}

	var opts pickerOptions
	for _, acl := range list {
		label := fmt.Sprintf("%s %s", *acl.Description, ansi.Faint("("+*acl.ID+")"))
		opts = append(opts, pickerOption{
			value: *acl.ID,
			label: label,
		})
	}

	if len(opts) == 0 {
		return nil, errors.New("there are currently no network ACLs to choose from")
	}

	return opts, nil
}
