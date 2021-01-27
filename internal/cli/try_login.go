package cli

import (
	"fmt"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth/authutil"
	"github.com/spf13/cobra"
)

func tryLoginCmd(cli *cli) *cobra.Command {
	var clientID string
	var connectionName string

	cmd := &cobra.Command{
		Use:   "try-login",
		Short: "Try out your universal login box",
		Long: `auth0 try-login
Launch a browser to try out your universal login box for the given client.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var userInfo *authutil.UserInfo

			tenant, err := cli.getTenant()
			if err != nil {
				return err
			}

			// use the client ID as passed in by the user, or default to the
			// "CLI Login Testing" client if none passed. This client is only
			// used for testing login from the CLI and will be created if it
			// does not exist.
			if clientID == "" {
				client, err := getOrCreateCLITesterClient(cli.api.Client)
				if err != nil {
					return err
				}
				clientID = client.GetClientID()
			}

			client, err := cli.api.Client.Read(clientID)
			if err != nil {
				return err
			}

			if proceed := runLoginFlowPreflightChecks(cli, client); !proceed {
				return nil
			}

			tokenResponse, err := runLoginFlow(
				cli,
				tenant,
				client,
				connectionName,
				"",      // audience is only supported for get-token
				"login", // force a login page when using try-login
				cliLoginTestingScopes,
			)
			if err != nil {
				return err
			}

			if err := ansi.Spinner("Fetching user metadata", func() error {
				// Use the access token to fetch user information from the /userinfo
				// endpoint.
				userInfo, err = authutil.FetchUserInfo(tenant.Domain, tokenResponse.AccessToken)
				return err
			}); err != nil {
				return err
			}

			fmt.Fprint(cli.renderer.MessageWriter, "\n")
			cli.renderer.TryLogin(userInfo, tokenResponse)
			return nil
		},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.Flags().StringVarP(&clientID, "client-id", "c", "", "Client Id for which to test login.")
	cmd.Flags().StringVarP(&connectionName, "connection", "", "", "Connection to test during login.")
	return cmd
}
