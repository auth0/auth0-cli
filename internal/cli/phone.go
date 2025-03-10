package cli

import (
	"github.com/spf13/cobra"
)

func phoneCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "phone",
		Short: "Manage phone providers",
		Long:  "Manage all the resources related to phone.",
	}

	cmd.AddCommand(phoneProviderCmd(cli))

	return cmd
}
