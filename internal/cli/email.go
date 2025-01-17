package cli

import (
	"github.com/spf13/cobra"
)

func emailCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "email",
		Short: "Manage email settings",
		Long: "You can configure a test SMTP email server in your development or test environments to check for " +
			"successful email delivery and view how emails you send appear to recipients prior to going to production.",
	}

	cmd.AddCommand(emailTemplateCmd(cli))
	cmd.AddCommand(emailProviderCmd(cli))

	return cmd
}
