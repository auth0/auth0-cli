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
		Help:      "Description of the network ACL",
	}

	networkACLActive = Flag{
		Name:     "Active",
		LongForm: "active",
		Help:     "Whether the network ACL is active",
	}

	networkACLPriority = Flag{
		Name:      "Priority",
		LongForm:  "priority",
		ShortForm: "p",
		Help:      "Priority of the network ACL",
	}

	networkACLRuleAction = Flag{
		Name:     "Action",
		LongForm: "action",
		Help:     "Action for the rule (block, allow, log, redirect)",
	}

	networkACLRedirectURI = Flag{
		Name:     "RedirectURI",
		LongForm: "redirect-uri",
		Help:     "URI to redirect to when action is redirect",
	}

	networkACLAnonymousProxy = Flag{
		Name:     "AnonymousProxy",
		LongForm: "anonymous-proxy",
		Help:     "Match anonymous proxy traffic",
	}

	networkACLASNs = Flag{
		Name:     "ASNs",
		LongForm: "asns",
		Help:     "Comma-separated list of ASNs to match",
	}

	networkACLCountryCodes = Flag{
		Name:     "CountryCodes",
		LongForm: "country-codes",
		Help:     "Comma-separated list of country codes to match",
	}

	networkACLSubdivisionCodes = Flag{
		Name:     "SubdivisionCodes",
		LongForm: "subdivision-codes",
		Help:     "Comma-separated list of subdivision codes to match",
	}

	networkACLIPv4CIDRs = Flag{
		Name:     "IPv4CIDRs",
		LongForm: "ipv4-cidrs",
		Help:     "Comma-separated list of IPv4 CIDR ranges",
	}

	networkACLIPv6CIDRs = Flag{
		Name:     "IPv6CIDRs",
		LongForm: "ipv6-cidrs",
		Help:     "Comma-separated list of IPv6 CIDR ranges",
	}

	networkACLJA3Fingerprints = Flag{
		Name:     "JA3Fingerprints",
		LongForm: "ja3-fingerprints",
		Help:     "Comma-separated list of JA3 fingerprints to match",
	}

	networkACLJA4Fingerprints = Flag{
		Name:     "JA4Fingerprints",
		LongForm: "ja4-fingerprints",
		Help:     "Comma-separated list of JA4 fingerprints to match",
	}

	networkACLUserAgents = Flag{
		Name:     "UserAgents",
		LongForm: "user-agents",
		Help:     "Comma-separated list of user agents to match",
	}
)

// Helper function to check if a string is in a slice
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
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
  auth0 network-acl ls --json`,
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
  auth0 network-acl show <id> --json`,
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
		Description    string
		Active         bool
		ActiveStr      string // Added for handling --active true/false
		Priority       int
		RuleJSON       string
		Action         string
		RedirectURI    string
		AnonymousProxy bool
		ASNs           []int
		CountryCodes   []string
		SubdivCodes    []string
		IPv4CIDRs      []string
		IPv6CIDRs      []string
		JA3            []string
		JA4            []string
		UserAgents     []string
		Scope          string
		isMatchRule    bool
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
  auth0 network-acl create --description "Block IPs" --priority 1 --active true --rule '{"action":{"block":true},"scope":"tenant","match":{"ip_v4_cidrs":["192.168.1.0/24","10.0.0.0/8"]}}'
  auth0 network-acl create --description "Geo Block" --priority 2 --active true --rule '{"action":{"block":true},"scope":"authentication","match":{"country_codes":["US","CA"],"anonymous_proxy":true}}'
  auth0 network-acl create --description "Redirect Traffic" --priority 3 --active true --rule '{"action":{"redirect":true,"redirect_uri":"https://example.com"},"scope":"management","match":{"ip_v4_cidrs":["192.168.1.0/24"]}}'
  auth0 network-acl create -d "Block Bots" -p 4 --active true --rule '{"action":{"block":true},"scope":"tenant","match":{"user_agents":["badbot/*","malicious/*"],"ja3_fingerprints":["deadbeef","cafebabe"]}}'
  auth0 network-acl create --description "Complex Rule" --priority 5 --active true --rule '{"action":{"block":true},"scope":"tenant","match":{"ip_v4_cidrs":["192.168.1.0/24"],"country_codes":["US"]}}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if we're in non-interactive mode (flags provided) but rule JSON is missing
			if !canPrompt(cmd) && !cmd.Flags().Changed("rule") {
				return fmt.Errorf("the --rule parameter is required for non-interactive mode. Please provide a valid JSON rule")
			}

			// Parse the active flag if provided
			if cmd.Flags().Changed("active") {
				if inputs.ActiveStr == "true" {
					inputs.Active = true
				} else if inputs.ActiveStr == "false" {
					inputs.Active = false
				} else {
					return fmt.Errorf("--active must be either 'true' or 'false', got %q", inputs.ActiveStr)
				}
			}

			// Check if rule JSON was provided
			if cmd.Flags().Changed("rule") {
				// Parse the rule JSON
				var rule map[string]interface{}
				if err := json.Unmarshal([]byte(inputs.RuleJSON), &rule); err != nil {
					return fmt.Errorf("invalid rule JSON: %w", err)
				}

				// Create the network ACL with the provided rule
				acl := &management.NetworkACL{
					Description: &inputs.Description,
					Active:      &inputs.Active,
					Priority:    &inputs.Priority,
				}

				// Convert the rule map to the appropriate structure
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

			// Interactive or flag-based creation
			if err := networkACLDescription.Ask(cmd, &inputs.Description, nil); err != nil {
				return err
			}

			if len(inputs.Description) > 255 {
				return fmt.Errorf("description cannot exceed 255 characters")
			}

			if err := networkACLActive.AskBool(cmd, &inputs.Active, nil); err != nil {
				return err
			}

			if err := networkACLPriority.AskInt(cmd, &inputs.Priority, nil); err != nil {
				return err
			}

			if inputs.Priority < 1 || inputs.Priority > 10 {
				return fmt.Errorf("priority must be between 1 and 10")
			}

			cli.renderer.Infof("Define the rule for the network ACL.\n")

			// Ask for scope
			scopes := []string{"management", "authentication", "tenant"}
			if cmd.Flags().Changed("scope") {
				if !contains(scopes, inputs.Scope) {
					return fmt.Errorf("scope must be one of: management, authentication, tenant")
				}
			} else {
				if err := (&Flag{
					Name: "Scope",
					Help: "Scope of the rule (management, authentication, tenant)",
				}).Select(cmd, &inputs.Scope, scopes, nil); err != nil {
					return err
				}
			}

			// Ask for action
			actions := []string{"block", "allow", "log", "redirect"}
			if err := networkACLRuleAction.Select(cmd, &inputs.Action, actions, nil); err != nil {
				return err
			}

			// If action is redirect, ask for redirect URI
			if inputs.Action == "redirect" {
				if err := networkACLRedirectURI.Ask(cmd, &inputs.RedirectURI, nil); err != nil {
					return err
				}
				if inputs.RedirectURI == "" {
					return fmt.Errorf("redirect URI is required when action is redirect")
				}
			}

			// Ask for Match or Not Match rule
			matchOptions := []string{"match", "not_match"}
			var selectedMatchOption string
			if err := (&Flag{
				Name: "What kind of rule do you want to create?",
				Help: "Match or Not Match rule",
			}).Select(cmd, &selectedMatchOption, matchOptions, nil); err != nil {
				return err
			}
			inputs.isMatchRule = selectedMatchOption == "match"

			// Handle match criteria
			if err := networkACLAnonymousProxy.AskBool(cmd, &inputs.AnonymousProxy, nil); err != nil {
				return err
			}

			if err := networkACLASNs.AskIntSlice(cmd, &inputs.ASNs, nil); err != nil {
				return err
			}

			if err := networkACLCountryCodes.AskMany(cmd, &inputs.CountryCodes, nil); err != nil {
				return err
			}

			if err := networkACLSubdivisionCodes.AskMany(cmd, &inputs.SubdivCodes, nil); err != nil {
				return err
			}

			if err := networkACLIPv4CIDRs.AskMany(cmd, &inputs.IPv4CIDRs, nil); err != nil {
				return err
			}

			if err := networkACLIPv6CIDRs.AskMany(cmd, &inputs.IPv6CIDRs, nil); err != nil {
				return err
			}

			if err := networkACLJA3Fingerprints.AskMany(cmd, &inputs.JA3, nil); err != nil {
				return err
			}

			if err := networkACLJA4Fingerprints.AskMany(cmd, &inputs.JA4, nil); err != nil {
				return err
			}

			if err := networkACLUserAgents.AskMany(cmd, &inputs.UserAgents, nil); err != nil {
				return err
			}

			// Build the network ACL
			acl := &management.NetworkACL{
				Description: &inputs.Description,
				Active:      &inputs.Active,
				Priority:    &inputs.Priority,
				Rule: &management.NetworkACLRule{
					Scope: &inputs.Scope,
				},
			}

			// Set the action based on the selected action type
			acl.Rule.Action = &management.NetworkACLRuleAction{}
			switch inputs.Action {
			case "block":
				acl.Rule.Action.Block = auth0.Bool(true)
			case "allow":
				acl.Rule.Action.Allow = auth0.Bool(true)
			case "log":
				acl.Rule.Action.Log = auth0.Bool(true)
			case "redirect":
				acl.Rule.Action.Redirect = auth0.Bool(true)
				acl.Rule.Action.RedirectURI = &inputs.RedirectURI
			}

			// Set match criteria if any were provided
			match := &management.NetworkACLRuleMatch{}
			matchProvided := false

			if inputs.AnonymousProxy {
				match.AnonymousProxy = auth0.Bool(true)
				matchProvided = true
			}

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

			if matchProvided {
				if inputs.isMatchRule {
					acl.Rule.Match = match
				} else {
					acl.Rule.NotMatch = match
				}
			} else {
				return fmt.Errorf("at least one match criteria must be provided")
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

	cmd.Flags().StringVarP(&inputs.Description, "description", "d", "", "Description of the network ACL (required)")
	cmd.Flags().StringVar(&inputs.ActiveStr, "active", "", "Whether the network ACL is active (required, 'true' or 'false')")
	cmd.Flags().IntVarP(&inputs.Priority, "priority", "p", 0, "Priority of the network ACL (required, 1-10)")
	cmd.Flags().StringVar(&inputs.RuleJSON, "rule", "", "Network ACL rule configuration in JSON format (required for non-interactive mode)")
	cmd.Flags().StringVar(&inputs.Action, "action", "", "Action for the rule (block, allow, log, redirect)")
	cmd.Flags().StringVar(&inputs.RedirectURI, "redirect-uri", "", "URI to redirect to when action is redirect")
	cmd.Flags().BoolVar(&inputs.AnonymousProxy, "anonymous-proxy", false, "Match anonymous proxy traffic")
	cmd.Flags().StringVar(&inputs.Scope, "scope", "", "Scope of the rule (management, authentication, tenant)")

	// Register the string slice flags
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
		ID             string
		Description    string
		Active         bool
		ActiveStr      string
		Priority       int
		RuleJSON       string
		Action         string
		RedirectURI    string
		Scope          string
		AnonymousProxy bool
		ASNs           []int
		CountryCodes   []string
		SubdivCodes    []string
		IPv4CIDRs      []string
		IPv6CIDRs      []string
		JA3            []string
		JA4            []string
		UserAgents     []string
		MatchRule      bool
		NoMatchRule    bool
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update a network ACL",
		Long: `Update a network ACL.
To update interactively, use "auth0 network-acl update" with no arguments.
To update non-interactively, supply the required parameters (description, active, priority, and rule) through flags.
When updating the rule, provide a complete JSON object with action, scope, and match/not_match properties.`,
		Example: `  auth0 network-acl update <id>
  auth0 network-acl update <id> --priority 5 
  auth0 network-acl update <id> --active true
  auth0 network-acl update <id> --description "Complex Rule updated" --priority 9 --active true --rule '{"action":{"block":true},"scope":"tenant","match":{"ip_v4_cidrs":["192.168.1.0/24"],"country_codes":["US"]}}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get the network ACL ID
			if len(args) > 0 {
				inputs.ID = args[0]
			} else {
				if err := networkACLID.Pick(cmd, &inputs.ID, cli.networkACLPickerOptions); err != nil {
					return err
				}
			}

			// Read the current ACL
			var currentACL *management.NetworkACL
			err := ansi.Waiting(func() (err error) {
				currentACL, err = cli.api.NetworkACL.Read(cmd.Context(), inputs.ID)
				return err
			})
			if err != nil {
				return fmt.Errorf("failed to get network ACL with ID %q: %w", inputs.ID, err)
			}

			var updatedACL *management.NetworkACL

			// Initialize updatedACL with the ID from the current ACL
			updatedACL = &management.NetworkACL{
				ID: currentACL.ID,
			}

			// Interactive update flow
			if canPrompt(cmd) {
				// Check if specific flags were provided (partial update)
				flagsProvided := cmd.Flags().Changed("description") || cmd.Flags().Changed("active") ||
					cmd.Flags().Changed("priority") || cmd.Flags().Changed("rule")

				// If some flags were provided, ask if user wants to update other fields
				if flagsProvided {
					var updateOtherFields bool
					if err := prompt.AskBool("Do you want to update other fields as well?", &updateOtherFields, false); err != nil {
						return err
					}

					if !updateOtherFields {
						// User doesn't want to update other fields, use current values
						// Initialize with current values
						updatedACL = currentACL

						// Override only the fields that were specified via flags
						if cmd.Flags().Changed("description") {
							updatedACL.Description = &inputs.Description
						}

						if cmd.Flags().Changed("active") {
							if inputs.ActiveStr == "true" {
								inputs.Active = true
							} else if inputs.ActiveStr == "false" {
								inputs.Active = false
							} else {
								return fmt.Errorf("--active must be either 'true' or 'false', got %q", inputs.ActiveStr)
							}
							updatedACL.Active = &inputs.Active
						}

						if cmd.Flags().Changed("priority") {
							updatedACL.Priority = &inputs.Priority
						}

						if cmd.Flags().Changed("rule") {
							var rule management.NetworkACLRule
							if err := json.Unmarshal([]byte(inputs.RuleJSON), &rule); err != nil {
								return fmt.Errorf("invalid rule JSON: %w", err)
							}
							updatedACL.Rule = &rule
						}

						// Skip the rest of the interactive flow
						goto updateACL
					}
				}

				// Use current values as defaults for interactive prompts
				currentDescriptionStr := *currentACL.Description
				if err := networkACLDescription.Ask(cmd, &inputs.Description, &currentDescriptionStr); err != nil {
					return err
				}

				if err := networkACLActive.AskBool(cmd, &inputs.Active, currentACL.Active); err != nil {
					return err
				}

				currentPriorityStr := fmt.Sprintf("%d", *currentACL.Priority)
				if err := networkACLPriority.AskInt(cmd, &inputs.Priority, &currentPriorityStr); err != nil {
					return err
				}

				cli.renderer.Infof("Define the rule for the network ACL.\n")

				// Default scope from current ACL
				currentScope := ""
				if currentACL.Rule != nil && currentACL.Rule.Scope != nil {
					currentScope = *currentACL.Rule.Scope
				}

				scopes := []string{"management", "authentication", "tenant"}
				if err := (&Flag{
					Name: "Scope",
					Help: "Scope of the rule (management, authentication, tenant)",
				}).Select(cmd, &inputs.Scope, scopes, &currentScope); err != nil {
					return err
				}

				// Determine current action
				currentAction := "block"
				if currentACL.Rule != nil && currentACL.Rule.Action != nil {
					if currentACL.Rule.Action.Block != nil && *currentACL.Rule.Action.Block {
						currentAction = "block"
					} else if currentACL.Rule.Action.Allow != nil && *currentACL.Rule.Action.Allow {
						currentAction = "allow"
					} else if currentACL.Rule.Action.Log != nil && *currentACL.Rule.Action.Log {
						currentAction = "log"
					} else if currentACL.Rule.Action.Redirect != nil && *currentACL.Rule.Action.Redirect {
						currentAction = "redirect"
					}
				}

				actions := []string{"block", "allow", "log", "redirect"}
				if err := networkACLRuleAction.Select(cmd, &inputs.Action, actions, &currentAction); err != nil {
					return err
				}

				if inputs.Action == "redirect" {
					currentRedirectURI := ""
					if currentACL.Rule != nil && currentACL.Rule.Action != nil &&
						currentACL.Rule.Action.RedirectURI != nil {
						currentRedirectURI = *currentACL.Rule.Action.RedirectURI
					}

					if err := networkACLRedirectURI.Ask(cmd, &inputs.RedirectURI, &currentRedirectURI); err != nil {
						return err
					}
				}

				// Depending on Match/NotMatch ask the user if they want to update the current or change it
				if currentACL.Rule != nil && currentACL.Rule.Match != nil {
					if err := prompt.AskBool("Do you want to update the current match criteria to NotMatch criteria?", &inputs.NoMatchRule, false); err != nil {
						return err
					}
				}
				if currentACL.Rule != nil && currentACL.Rule.NotMatch != nil {
					if err := prompt.AskBool("Do you want to update the current not match criteria to match criteria?", &inputs.MatchRule, false); err != nil {
						return err
					}
				}

				// Get current match values for defaults
				currentAnonymousProxy := false
				var currentASNs []int
				var currentCountryCodes, currentSubDivCodes []string
				var currentIPv4CIDRs, currentIPv6CIDRs []string
				var currentJA3, currentJA4, currentUserAgents []string

				if currentACL.Rule != nil && currentACL.Rule.Match != nil {
					match := currentACL.Rule.Match

					if match.AnonymousProxy != nil {
						currentAnonymousProxy = *match.AnonymousProxy
					}

					if len(match.Asns) > 0 {
						currentASNs = match.Asns
					}

					if match.GeoCountryCodes != nil {
						currentCountryCodes = *match.GeoCountryCodes
					}

					if match.GeoSubdivisionCodes != nil {
						currentSubDivCodes = *match.GeoSubdivisionCodes
					}

					if match.IPv4Cidrs != nil {
						currentIPv4CIDRs = *match.IPv4Cidrs
					}

					if match.IPv6Cidrs != nil {
						currentIPv6CIDRs = *match.IPv6Cidrs
					}

					if match.Ja3Fingerprints != nil {
						currentJA3 = *match.Ja3Fingerprints
					}

					if match.Ja4Fingerprints != nil {
						currentJA4 = *match.Ja4Fingerprints
					}

					if match.UserAgents != nil {
						currentUserAgents = *match.UserAgents
					}
				}

				if err := networkACLAnonymousProxy.AskBool(cmd, &inputs.AnonymousProxy, &currentAnonymousProxy); err != nil {
					return err
				}

				if err := networkACLASNs.AskIntSlice(cmd, &inputs.ASNs, &currentASNs); err != nil {
					return err
				}

				// Convert string slices to comma-separated strings for AskMany
				currentCountryCodesStr := strings.Join(currentCountryCodes, ",")
				if err := networkACLCountryCodes.AskMany(cmd, &inputs.CountryCodes, &currentCountryCodesStr); err != nil {
					return err
				}

				currentSubDivCodesStr := strings.Join(currentSubDivCodes, ",")
				if err := networkACLSubdivisionCodes.AskMany(cmd, &inputs.SubdivCodes, &currentSubDivCodesStr); err != nil {
					return err
				}

				currentIPv4CIDRsStr := strings.Join(currentIPv4CIDRs, ",")
				if err := networkACLIPv4CIDRs.AskMany(cmd, &inputs.IPv4CIDRs, &currentIPv4CIDRsStr); err != nil {
					return err
				}

				currentIPv6CIDRsStr := strings.Join(currentIPv6CIDRs, ",")
				if err := networkACLIPv6CIDRs.AskMany(cmd, &inputs.IPv6CIDRs, &currentIPv6CIDRsStr); err != nil {
					return err
				}

				currentJA3Str := strings.Join(currentJA3, ",")
				if err := networkACLJA3Fingerprints.AskMany(cmd, &inputs.JA3, &currentJA3Str); err != nil {
					return err
				}

				currentJA4Str := strings.Join(currentJA4, ",")
				if err := networkACLJA4Fingerprints.AskMany(cmd, &inputs.JA4, &currentJA4Str); err != nil {
					return err
				}

				currentUserAgentsStr := strings.Join(currentUserAgents, ",")
				if err := networkACLUserAgents.AskMany(cmd, &inputs.UserAgents, &currentUserAgentsStr); err != nil {
					return err
				}

				// Build the updated ACL from interactive inputs
				updatedACL.Description = &inputs.Description
				updatedACL.Active = &inputs.Active
				updatedACL.Priority = &inputs.Priority
				updatedACL.Rule = &management.NetworkACLRule{
					Scope: &inputs.Scope,
				}

				// Set the action based on the selected action type
				updatedACL.Rule.Action = &management.NetworkACLRuleAction{}
				switch inputs.Action {
				case "block":
					updatedACL.Rule.Action.Block = auth0.Bool(true)
				case "allow":
					updatedACL.Rule.Action.Allow = auth0.Bool(true)
				case "log":
					updatedACL.Rule.Action.Log = auth0.Bool(true)
				case "redirect":
					updatedACL.Rule.Action.Redirect = auth0.Bool(true)
					updatedACL.Rule.Action.RedirectURI = &inputs.RedirectURI
				}

				// Set match criteria if any were provided
				match := &management.NetworkACLRuleMatch{}
				matchProvided := false

				if inputs.AnonymousProxy {
					match.AnonymousProxy = auth0.Bool(true)
					matchProvided = true
				}

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

				if matchProvided {
					if inputs.NoMatchRule {
						updatedACL.Rule.NotMatch = match
						updatedACL.Rule.Match = nil // Clear Match if converting to NotMatch
					} else if inputs.MatchRule {
						updatedACL.Rule.Match = match
						updatedACL.Rule.NotMatch = nil // Clear NotMatch if converting to Match
					} else {
						// Preserve the existing rule type (Match or NotMatch)
						if currentACL.Rule != nil {
							if currentACL.Rule.Match != nil {
								updatedACL.Rule.Match = match
							} else if currentACL.Rule.NotMatch != nil {
								updatedACL.Rule.NotMatch = match
							} else {
								// Default to Match if neither exists
								updatedACL.Rule.Match = match
							}
						} else {
							// Default to Match if no rule exists
							updatedACL.Rule.Match = match
						}
					}
				}

			} else {
				// Non-interactive update flow
				if !(cmd.Flags().Changed("description") && cmd.Flags().Changed("active") &&
					cmd.Flags().Changed("priority") && cmd.Flags().Changed("rule")) {
					return fmt.Errorf("all parameters (description, active, priority, and rule) must be provided for non-interactive mode")
				}

				// Parse the active flag if provided
				if cmd.Flags().Changed("active") {
					if inputs.ActiveStr == "true" {
						inputs.Active = true
						updatedACL.Active = &inputs.Active
					} else if inputs.ActiveStr == "false" {
						inputs.Active = false
						updatedACL.Active = &inputs.Active
					} else {
						return fmt.Errorf("--active must be either 'true' or 'false', got %q", inputs.ActiveStr)
					}
				} else {
					return fmt.Errorf("--active flag not provided")
				}

				// Parse description if provided
				if cmd.Flags().Changed("description") {
					if len(inputs.Description) > 255 {
						return fmt.Errorf("description cannot exceed 255 characters")
					}
					updatedACL.Description = &inputs.Description
				} else {
					return fmt.Errorf("--description flag not provided")
				}

				// Parse priority if provided
				if cmd.Flags().Changed("priority") {
					if inputs.Priority < 1 || inputs.Priority > 10 {
						return fmt.Errorf("priority must be between 1 and 10")
					}
					updatedACL.Priority = &inputs.Priority
				} else {
					return fmt.Errorf("--priority flag not provided")
				}

				// Parse rule JSON if provided
				if cmd.Flags().Changed("rule") {
					var rule management.NetworkACLRule
					if err := json.Unmarshal([]byte(inputs.RuleJSON), &rule); err != nil {
						return fmt.Errorf("invalid rule JSON: %w", err)
					}
					updatedACL.Rule = &rule
				} else {
					return fmt.Errorf("--rule flag not provided")
				}
			}

			// If no changes were made, use the current ACL data as fallback
			if updatedACL.Description == nil && updatedACL.Active == nil &&
				updatedACL.Priority == nil && updatedACL.Rule == nil {
				// Copy current ACL data
				updatedACL = currentACL
			}

		updateACL:
			// Update the network ACL
			if err := ansi.Waiting(func() error {
				return cli.api.NetworkACL.Update(cmd.Context(), inputs.ID, updatedACL)
			}); err != nil {
				return fmt.Errorf("failed to update network ACL with ID %q: %w", inputs.ID, err)
			}

			cli.renderer.NetworkACLUpdate(updatedACL)
			return nil
		},
	}

	cmd.Flags().StringVarP(&inputs.Description, "description", "d", "", "Description of the network ACL")
	cmd.Flags().StringVar(&inputs.ActiveStr, "active", "", "Whether the network ACL is active ('true' or 'false')")
	cmd.Flags().IntVarP(&inputs.Priority, "priority", "p", 1, "Priority of the network ACL (1-10)")
	cmd.Flags().StringVar(&inputs.RuleJSON, "rule", "", "Network ACL rule configuration in JSON format")
	cmd.Flags().StringVar(&inputs.Action, "action", "", "Action for the rule (block, allow, log, redirect)")
	cmd.Flags().StringVar(&inputs.RedirectURI, "redirect-uri", "", "URI to redirect to when action is redirect")
	cmd.Flags().BoolVar(&inputs.AnonymousProxy, "anonymous-proxy", false, "Match anonymous proxy traffic")
	cmd.Flags().StringVar(&inputs.Scope, "scope", "", "Scope of the rule (management, authentication, tenant)")

	// Register the string slice flags
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
			// Check if --all flag is set
			if inputs.All {
				// Get all network ACLs
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

				// Confirm deletion
				if !cli.force && canPrompt(cmd) {
					if confirmed := prompt.Confirm(fmt.Sprintf("Are you sure you want to delete ALL %d network ACLs?", len(list))); !confirmed {
						return nil
					}
				}

				// Delete all ACLs with progress bar
				return ansi.ProgressBar("Deleting all network ACLs", list, func(i int, acl *management.NetworkACL) error {
					if acl != nil && acl.ID != nil {
						return cli.api.NetworkACL.Delete(cmd.Context(), *acl.ID)
					}
					return nil
				})
			}

			// Regular single or multiple ACL delete flow
			ids := make([]string, len(args))
			if len(args) == 0 {
				if err := networkACLID.PickMany(cmd, &ids, cli.networkACLPickerOptions); err != nil {
					return err
				}
			} else {
				ids = append(ids, args...)
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
