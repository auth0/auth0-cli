package cli

import (
	"github.com/spf13/cobra"
)

func attackProtectionCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "protection",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"attack-protection", "ap"},
		Short:   "Manage resources for attack protection",
		Long:    "Manage resources for attack protection.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())

	cmd.AddCommand(breachedPasswordDetectionCmd(cli))
	cmd.AddCommand(bruteForceProtectionCmd(cli))
	cmd.AddCommand(suspiciousIPThrottlingCmd(cli))

	return cmd
}
