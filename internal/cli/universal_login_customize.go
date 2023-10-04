package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func customizeUniversalLoginCmd(_ *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "customize",
		Args:  cobra.NoArgs,
		Short: "Customize the entire Universal Login experience",
		Long: "Customize and preview changes to the Universal Login experience. This command will open a webpage " +
			"within your browser where you can edit and preview your branding changes. For a comprehensive list of " +
			"editable parameters and their values please visit the " +
			"[Management API Documentation](https://auth0.com/docs/api/management/v2).",
		Example: `  auth0 universal-login customize
  auth0 ul customize`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("not yet implemented")
		},
	}

	return cmd
}
