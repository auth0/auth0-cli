package cli

import (
	"github.com/auth0/auth0-cli/internal/auth"
	"github.com/spf13/cobra"
)

func loginCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "authenticate the Auth0 CLI.",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := &auth.Authenticator{}
			res, err := a.Authenticate(cmd.Context())
			if err != nil {
				return err
			}
			// TODO(jfatta): update the configuration with the token, tenant, audience, etc
			cli.renderer.Infof("Successfully logged in.")
			cli.renderer.Infof("Tenant: %s", res.Tenant)
			return nil
		},
	}

	return cmd
}
