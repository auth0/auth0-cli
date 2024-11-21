package cli

import (
	"context"
	"errors"
	"fmt"
	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"
)

func tokenCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Args:  cobra.MaximumNArgs(1),
		Short: "Fetches the auth0 token value",
		Long:  "Fetches the auth0 token value.",
		Example: `  auth0 token
  auth0 token <tenant>
  auth0 token "example.us.auth0.com"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get the tenant from the config.
			selectedTenant, err := selectValidTenantFromConfig(cli, cmd, args)
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			err = cli.storeToken(ctx, selectedTenant)
			if err != nil {
				return err
			}

			return nil
		},
	}

	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		_ = cmd.Flags().MarkHidden("tenant")
		cmd.Parent().HelpFunc()(cmd, args)
	})

	return cmd
}

func (c *cli) storeToken(ctx context.Context, tenantName string) error {
	// Get the tenant from the config.
	tenant, err := c.Config.GetTenant(tenantName)
	if err != nil {
		return err
	}

	accessToken := tenant.GetAccessToken()
	if accessToken != "" && !tenant.HasExpiredToken() {

	} else {
		if err = tenant.RegenerateAccessToken(ctx); err != nil {
			return err
		} else {
			accessToken = tenant.GetAccessToken()
		}
	}

	err = clipboard.WriteAll(accessToken)
	if err != nil {
		return errors.New(fmt.Sprint("Failed to copy to clipboard:", err))
	} else {
		c.renderer.Output("Management API Token copied to clipboard!\n")
	}

	//if accessToken != "" && !tenant.HasExpiredToken() {
	//	c.renderer.Output(fmt.Sprintf("Access Token: %v", ansi.Bold(accessToken)))
	//	return nil
	//} else {
	//	if err := tenant.RegenerateAccessToken(ctx); err != nil {
	//
	//	}
	//}

	return nil
}
