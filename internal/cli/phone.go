package cli

import (
	"github.com/spf13/cobra"
)

func phoneCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "phone",
		Short: "Manage phone providers",
		Long:  "Configure the phone providers like twilio and custom",
	}

	cmd.AddCommand(phoneProviderCmd(cli))

	return cmd
}
