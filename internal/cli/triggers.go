package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func triggersCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "triggers",
		Short: "manage resources for triggers.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(listTriggersCmd())

	return cmd
}

func listTriggersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "A brief description of your command",
		Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("list triggers")
			return nil
		},
	}

	return cmd
}
