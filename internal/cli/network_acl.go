package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
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
		Name:       "Description",
		LongForm:   "description",
		ShortForm:  "d",
		Help:       "Description of the network ACL (Eg. \"Block suspicious IPs\").",
		IsRequired: true,
	}

	networkACLActive = Flag{
		Name:       "Active",
		LongForm:   "active",
		Help:       "Whether the network ACL is active ('true' or 'false').",
		IsRequired: true,
	}

	networkACLPriority = Flag{
		Name:       "Priority",
		LongForm:   "priority",
		ShortForm:  "p",
		Help:       "Priority of the network ACL (Eg. 5).",
		IsRequired: true,
	}

	networkACLRule = Flag{
		Name:       "Rule",
		LongForm:   "rule",
		Help:       "Network ACL rule configuration in JSON format (required for non-interactive mode).",
		IsRequired: true,
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

	networkACLScope = Flag{
		Name:     "Scope",
		LongForm: "scope",
		Help:     "Scope of the rule (management, authentication, tenant)",
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

	networkACLAuth0Managed = Flag{
		Name:     "Auth0Managed",
		LongForm: "auth0-managed",
		Help:     "Comma-separated list of Auth0-curated blocklists to match (Eg. auth0.icloud_relay_proxy,auth0.low_reputation). (EA only).",
	}
)

// networkACLBasicInputs holds the flag-driven fields shared by create and update.
type networkACLBasicInputs struct {
	ID          string
	Description string
	Active      bool
	ActiveStr   string
	Priority    int
	RuleJSON    string
}

// validateNetworkACLDescription ensures the description is non-empty and within the API length limit.
func validateNetworkACLDescription(description string) error {
	if len(description) == 0 {
		return fmt.Errorf("description cannot be empty")
	}
	if len(description) > 255 {
		return fmt.Errorf("description cannot exceed 255 characters")
	}
	return nil
}

// validateAndSetBasicFields handles the common validation and patch building logic for basic fields.
func validateAndSetBasicFields(inputs *networkACLBasicInputs, patch *management.NetworkACL, cmd *cobra.Command) error {
	if networkACLDescription.IsSet(cmd) {
		if err := validateNetworkACLDescription(inputs.Description); err != nil {
			return err
		}
		patch.Description = &inputs.Description
	}

	if networkACLActive.IsSet(cmd) {
		active, err := strconv.ParseBool(inputs.ActiveStr)
		if err != nil {
			return fmt.Errorf("--active must be either 'true' or 'false', got %q", inputs.ActiveStr)
		}
		inputs.Active = active
		patch.Active = &inputs.Active
	}

	if networkACLPriority.IsSet(cmd) {
		patch.Priority = &inputs.Priority
	}

	if networkACLRule.IsSet(cmd) {
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

	return cli.renderer.NetworkACLUpdate(patch)
}

func selectNetworkACLParams() (map[string]bool, error) {
	options := []string{
		"ASNs",
		"Country Codes",
		"Subdivision Codes",
		"IPv4CIDRs",
		"IPv6CIDRs",
		"JA3Fingerprints",
		"JA4Fingerprints",
		"User Agents",
		"Auth0 Managed",
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
	Auth0Managed []string
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
		if match.Auth0Managed != nil {
			defaults.Auth0Managed = *match.Auth0Managed
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
	Auth0Managed []string
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
	if err := (&Flag{
		Name: "Action",
		Help: "Action for the rule (block, allow, log, redirect)",
	}).Select(cmd, &inputs.Action, actions, &defaults.Action); err != nil {
		return nil, err
	}

	// If action is redirect, ask for redirect URI.
	if inputs.Action == "redirect" {
		if err := (&Flag{
			Name: "RedirectURI",
			Help: "URI to redirect to when action is redirect (Eg. \"https://example.com/blocked\")",
		}).Ask(cmd, &inputs.RedirectURI, &defaults.RedirectURI); err != nil {
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
			Help: "Match or Not Match rule (ASNs, Country Codes, Subdivision Codes, IPv4 CIDRs, IPv6 CIDRs, JA3/JA4 Fingerprints, User Agents, Auth0 Managed)",
		}).Select(cmd, &selectedMatchOption, matchOptions, nil); err != nil {
			return nil, err
		}
		inputs.IsMatchRule = selectedMatchOption == "match"
	}

	// Select which parameters to provide.
	selectedParams, err := selectNetworkACLParams()
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
		if err := (&Flag{
			Name: "ASNs",
			Help: "Comma-separated list of ASNs to match (Eg. 64496,64497,64498)",
		}).AskIntSlice(cmd, &inputs.ASNs, &defaults.ASNs); err != nil {
			return err
		}
	}

	if selectedParams["Country Codes"] {
		currentCountryCodesStr := strings.Join(defaults.CountryCodes, ",")
		if err := (&Flag{
			Name: "CountryCodes",
			Help: "Comma-separated list of country codes to match (Eg. US,CA,MX)",
		}).AskMany(cmd, &inputs.CountryCodes, &currentCountryCodesStr); err != nil {
			return err
		}
	}

	if selectedParams["Subdivision Codes"] {
		currentSubDivCodesStr := strings.Join(defaults.SubdivCodes, ",")
		if err := (&Flag{
			Name: "SubdivisionCodes",
			Help: "Comma-separated list of subdivision codes to match (Eg. US-NY,US-CA)",
		}).AskMany(cmd, &inputs.SubdivCodes, &currentSubDivCodesStr); err != nil {
			return err
		}
	}

	if selectedParams["IPv4CIDRs"] {
		currentIPv4CIDRsStr := strings.Join(defaults.IPv4CIDRs, ",")
		if err := (&Flag{
			Name: "IPv4CIDRs",
			Help: "Comma-separated list of IPv4 CIDR ranges (Eg. 192.168.1.0/24,10.0.0.0/8)",
		}).AskMany(cmd, &inputs.IPv4CIDRs, &currentIPv4CIDRsStr); err != nil {
			return err
		}
	}

	if selectedParams["IPv6CIDRs"] {
		currentIPv6CIDRsStr := strings.Join(defaults.IPv6CIDRs, ",")
		if err := (&Flag{
			Name: "IPv6CIDRs",
			Help: "Comma-separated list of IPv6 CIDR ranges (Eg. 2001:db8::/32,2001:db8:1234::/48)",
		}).AskMany(cmd, &inputs.IPv6CIDRs, &currentIPv6CIDRsStr); err != nil {
			return err
		}
	}

	if selectedParams["JA3Fingerprints"] {
		currentJA3Str := strings.Join(defaults.JA3, ",")
		if err := (&Flag{
			Name: "JA3Fingerprints",
			Help: "Comma-separated list of JA3 fingerprints to match (Eg. deadbeef,cafebabe)",
		}).AskMany(cmd, &inputs.JA3, &currentJA3Str); err != nil {
			return err
		}
	}

	if selectedParams["JA4Fingerprints"] {
		currentJA4Str := strings.Join(defaults.JA4, ",")
		if err := (&Flag{
			Name: "JA4Fingerprints",
			Help: "Comma-separated list of JA4 fingerprints to match (Eg. t13d1516h2_8daaf6152771)",
		}).AskMany(cmd, &inputs.JA4, &currentJA4Str); err != nil {
			return err
		}
	}

	if selectedParams["User Agents"] {
		currentUserAgentsStr := strings.Join(defaults.UserAgents, ",")
		if err := (&Flag{
			Name: "UserAgents",
			Help: "Comma-separated list of user agents to match (Eg. badbot/*,malicious/*)",
		}).AskMany(cmd, &inputs.UserAgents, &currentUserAgentsStr); err != nil {
			return err
		}
	}

	if selectedParams["Auth0 Managed"] {
		currentAuth0ManagedStr := strings.Join(defaults.Auth0Managed, ",")
		if err := (&Flag{
			Name: "Auth0Managed",
			Help: "Comma-separated list of Auth0-curated blocklists to match (Eg. auth0.icloud_relay_proxy,auth0.low_reputation). (EA only).",
		}).AskMany(cmd, &inputs.Auth0Managed, &currentAuth0ManagedStr); err != nil {
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
	if len(inputs.Auth0Managed) > 0 {
		match.Auth0Managed = &inputs.Auth0Managed
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
	var inputs networkACLBasicInputs

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
  auth0 network-acl create --description "Complex Rule" --priority 5 --active true --rule '{"action":{"block":true},"scope":"tenant","match":{"ipv4_cidrs":["192.168.1.0/24"],"geo_country_codes":["US"]}}'
  
  # Early Access (auth0_managed match/not_match value):
  auth0 network-acl create -d "Curated Blocklist" -p 6 --active true --rule '{"action":{"log":true},"scope":"tenant","not_match":{"auth0_managed":["auth0.vpn","auth0.proxy"]}}'
  `,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate --rule JSON up front, before prompting for other fields, so
			// an invalid rule fails immediately instead of after the prompts.
			var rule *management.NetworkACLRule
			if networkACLRule.IsSet(cmd) {
				rule = &management.NetworkACLRule{}
				if err := json.Unmarshal([]byte(inputs.RuleJSON), rule); err != nil {
					return fmt.Errorf("invalid rule JSON: %w", err)
				}
			}

			if err := networkACLDescription.Ask(cmd, &inputs.Description, nil); err != nil {
				return err
			}
			if err := validateNetworkACLDescription(inputs.Description); err != nil {
				return err
			}

			if networkACLActive.IsSet(cmd) {
				active, err := strconv.ParseBool(inputs.ActiveStr)
				if err != nil {
					return fmt.Errorf("--active must be either 'true' or 'false', got %q", inputs.ActiveStr)
				}
				inputs.Active = active
			}
			defaultStatus := false
			if err := networkACLActive.AskBool(cmd, &inputs.Active, &defaultStatus); err != nil {
				return err
			}

			if err := networkACLPriority.AskInt(cmd, &inputs.Priority, nil); err != nil {
				return err
			}

			acl := &management.NetworkACL{
				Description: &inputs.Description,
				Active:      &inputs.Active,
				Priority:    &inputs.Priority,
			}

			// Use the rule parsed from --rule when provided, otherwise prompt for it.
			if rule != nil {
				acl.Rule = rule
			} else {
				defaults := &ruleDefaults{
					Scope:  "tenant",
					Action: "log",
				}

				ruleInputs, err := promptForRuleDetails(cmd, cli, defaults, false)
				if err != nil {
					return err
				}

				acl.Rule, err = buildNetworkACLRule(ruleInputs)
				if err != nil {
					return err
				}
			}

			if err := ansi.Waiting(func() error {
				return cli.api.NetworkACL.Create(cmd.Context(), acl)
			}); err != nil {
				return fmt.Errorf("failed to create network ACL: %w", err)
			}

			return cli.renderer.NetworkACLCreate(acl)
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	networkACLDescription.RegisterString(cmd, &inputs.Description, "")
	networkACLActive.RegisterString(cmd, &inputs.ActiveStr, "")
	networkACLPriority.RegisterInt(cmd, &inputs.Priority, 0)
	networkACLRule.RegisterString(cmd, &inputs.RuleJSON, "")
	registerDeprecatedRuleFlags(cmd)

	return cmd
}

func updateNetworkACLCmd(cli *cli) *cobra.Command {
	var inputs networkACLBasicInputs

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
  auth0 network-acl update <id> --description "Complex Rule updated" --priority 1 --active true --rule '{"action":{"block":true},"scope":"tenant","match":{"ipv4_cidrs":["192.168.1.0/24"],"geo_country_codes":["US"]}}'
  
  # Early Access (auth0_managed match/not_match value):
  auth0 network-acl update <id> --rule '{"action":{"allow":true},"scope":"tenant","match":{"auth0_managed":["auth0.low_reputation"]}}'
  `,
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
			flagsProvided := networkACLDescription.IsSet(cmd) || networkACLActive.IsSet(cmd) ||
				networkACLPriority.IsSet(cmd) || networkACLRule.IsSet(cmd)

			if !canPrompt(cmd) && !flagsProvided {
				return fmt.Errorf("in non-interactive mode, at least one field must be specified to update")
			}

			// Build patch object with only the fields that should be updated.
			patch := &management.NetworkACL{}

			// When flags are provided, update only those fields. This applies in both
			// interactive and non-interactive mode, so there is no need to read the
			// current ACL first.
			if flagsProvided {
				if err := validateAndSetBasicFields(&inputs, patch, cmd); err != nil {
					return err
				}

				return applyNetworkACLPatch(cmd.Context(), cli, inputs.ID, patch)
			}

			// Full interactive mode - read the current ACL to use its values as defaults.
			var currentACL *management.NetworkACL
			err := ansi.Waiting(func() (err error) {
				currentACL, err = cli.api.NetworkACL.Read(cmd.Context(), inputs.ID)
				return err
			})
			if err != nil {
				return fmt.Errorf("failed to get network ACL with ID %q: %w", inputs.ID, err)
			}

			// Use current values as defaults for interactive prompts.
			if err := networkACLDescription.Ask(cmd, &inputs.Description, currentACL.Description); err != nil {
				return err
			}
			if err := validateNetworkACLDescription(inputs.Description); err != nil {
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
	networkACLDescription.RegisterStringU(cmd, &inputs.Description, "")
	networkACLActive.RegisterStringU(cmd, &inputs.ActiveStr, "")
	networkACLPriority.RegisterIntU(cmd, &inputs.Priority, 1)
	networkACLRule.RegisterStringU(cmd, &inputs.RuleJSON, "")
	registerDeprecatedRuleFlags(cmd)

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
				if !cli.force && cli.agentMode {
					return errDestructiveNoConfirm
				}

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

			if !cli.force && cli.agentMode {
				return errDestructiveNoConfirm
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

const deprecatedRuleFlagMessage = "use `--rule` flag to set the configuration as JSON."

func registerDeprecatedRuleFlags(cmd *cobra.Command) {
	var (
		action       string
		redirectURI  string
		scope        string
		asns         []int
		countryCodes []string
		subdivCodes  []string
		ipv4CIDRs    []string
		ipv6CIDRs    []string
		ja3          []string
		ja4          []string
		userAgents   []string
		auth0Managed []string
	)

	networkACLRuleAction.RegisterString(cmd, &action, "")
	networkACLRedirectURI.RegisterString(cmd, &redirectURI, "")
	networkACLScope.RegisterString(cmd, &scope, "")
	networkACLASNs.RegisterIntSlice(cmd, &asns, nil)
	networkACLCountryCodes.RegisterStringSlice(cmd, &countryCodes, nil)
	networkACLSubdivisionCodes.RegisterStringSlice(cmd, &subdivCodes, nil)
	networkACLIPv4CIDRs.RegisterStringSlice(cmd, &ipv4CIDRs, nil)
	networkACLIPv6CIDRs.RegisterStringSlice(cmd, &ipv6CIDRs, nil)
	networkACLJA3Fingerprints.RegisterStringSlice(cmd, &ja3, nil)
	networkACLJA4Fingerprints.RegisterStringSlice(cmd, &ja4, nil)
	networkACLUserAgents.RegisterStringSlice(cmd, &userAgents, nil)
	networkACLAuth0Managed.RegisterStringSlice(cmd, &auth0Managed, nil)

	for _, f := range []*Flag{
		&networkACLRuleAction,
		&networkACLRedirectURI,
		&networkACLScope,
		&networkACLASNs,
		&networkACLCountryCodes,
		&networkACLSubdivisionCodes,
		&networkACLIPv4CIDRs,
		&networkACLIPv6CIDRs,
		&networkACLJA3Fingerprints,
		&networkACLJA4Fingerprints,
		&networkACLUserAgents,
		&networkACLAuth0Managed,
	} {
		f.Deprecate(cmd, deprecatedRuleFlagMessage)
	}
}
