package cli

import (
	"github.com/spf13/cobra"
)

func emailCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "email",
		Short: "Manage email settings",
		Long:  "Manage email settings.",
	}

	cmd.AddCommand(emailTemplateCmd(cli))

	return cmd
}
