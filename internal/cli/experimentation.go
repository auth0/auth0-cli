package cli

import (
	"github.com/spf13/cobra"
)

// experimentationCmd groups the Experiment Center resources (feature flags,
// segments, and experiments) under a single `auth0 experimentation` namespace,
// mirroring the Management API's experimentation grouping.
func experimentationCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "experimentation",
		Aliases: []string{"exp"},
		Short:   "Manage Experiment Center resources",
		Long: "Manage Auth0 Experiment Center resources: feature flags, segments, and experiments.\n\n" +
			"Feature flags define named parameters that experiments vary across user groups, segments group " +
			"users by matching rules, and experiments run A/B tests by tying a feature flag, its variations, " +
			"and traffic allocations together.",
	}

	cmd.SetUsageTemplate(namespaceUsageTemplate())
	cmd.AddCommand(featureFlagsCmd(cli))
	cmd.AddCommand(segmentsCmd(cli))
	cmd.AddCommand(experimentsCmd(cli))

	return cmd
}
