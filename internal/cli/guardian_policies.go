package cli

import (
	"fmt"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
)

// guardianPolicyNone is the interactive/CLI value that clears all MFA policies.
// The two real policies are mutually exclusive, so the CLI treats the policy as
// a single choice (all-applications, confidence-score or none) even though the
// Management API models it as a list.
const guardianPolicyNone = "none"

var guardianPolicyOptions = []string{
	string(managementv3.MfaPolicyEnumAllApplications),
	string(managementv3.MfaPolicyEnumConfidenceScore),
	guardianPolicyNone,
}

var guardianPolicies = Flag{
	Name:      "Policy",
	LongForm:  "policy",
	ShortForm: "p",
	Help: "MFA policy to enable. Supported values: all-applications, confidence-score. " +
		"The policies are mutually exclusive; pass none (or --none) to clear all policies.",
}

var guardianPoliciesNone = Flag{
	Name:     "None",
	LongForm: "none",
	Help:     "Clear all MFA policies.",
}

func guardianPoliciesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policies",
		Short: "Manage multi-factor authentication policies",
		Long:  "Manage the tenant-wide multi-factor authentication (MFA) policies.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(showGuardianPoliciesCmd(cli))
	cmd.AddCommand(setGuardianPoliciesCmd(cli))

	return cmd
}

func showGuardianPoliciesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.NoArgs,
		Short: "Show the multi-factor authentication policies",
		Long:  "Display the tenant-wide multi-factor authentication (MFA) policies.",
		Example: `  auth0 guardian policies show
  auth0 guardian policies show --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var policies managementv3.ListGuardianPoliciesResponseContent
			if err := ansi.Waiting(func() (err error) {
				policies, err = cli.apiv3.GuardianPolicy.List(cmd.Context())
				return err
			}); err != nil {
				return fmt.Errorf("failed to read guardian policies: %w", err)
			}

			cli.renderer.GuardianPolicyList(policies)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	return cmd
}

func setGuardianPoliciesCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Policy string
		None   bool
	}

	cmd := &cobra.Command{
		Use:   "set",
		Args:  cobra.NoArgs,
		Short: "Set the multi-factor authentication policy",
		Long: "Set the tenant-wide multi-factor authentication (MFA) policy.\n\n" +
			"The policies are mutually exclusive, so this sets a single policy and replaces the " +
			"existing one. Pass `--policy none` or `--none` (or select none interactively) to clear " +
			"the policy.",
		Example: `  auth0 guardian policies set
  auth0 guardian policies set --policy all-applications
  auth0 guardian policies set --policy confidence-score
  auth0 guardian policies set --none
  auth0 guardian policies set --policy all-applications --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Interactively pick a policy unless the user passed --policy or --none.
			if !guardianPolicies.IsSet(cmd) && !inputs.None {
				if !canPrompt(cmd) {
					return fmt.Errorf("--policy or --none is required when running non-interactively; supported values: all-applications, confidence-score, none")
				}
				if err := guardianPolicies.Select(cmd, &inputs.Policy, guardianPolicyOptions, nil); err != nil {
					return err
				}
			}

			body := make(managementv3.SetGuardianPoliciesRequestContent, 0, 1)
			if !inputs.None && inputs.Policy != "" && inputs.Policy != guardianPolicyNone {
				policy, err := managementv3.NewMfaPolicyEnumFromString(inputs.Policy)
				if err != nil {
					return fmt.Errorf("invalid policy %q: valid values are all-applications, confidence-score, none", inputs.Policy)
				}
				body = append(body, policy)
			}

			var policies managementv3.SetGuardianPoliciesResponseContent
			if err := ansi.Waiting(func() (err error) {
				policies, err = cli.apiv3.GuardianPolicy.Set(cmd.Context(), body)
				return err
			}); err != nil {
				return fmt.Errorf("failed to set guardian policies: %w", err)
			}

			cli.renderer.GuardianPolicyList(policies)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	guardianPolicies.RegisterString(cmd, &inputs.Policy, "")
	guardianPoliciesNone.RegisterBool(cmd, &inputs.None, false)
	cmd.MarkFlagsMutuallyExclusive("policy", "none")

	return cmd
}
