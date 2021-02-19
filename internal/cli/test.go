package cli

import (
	"fmt"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth/authutil"
	"github.com/spf13/cobra"
)

func testCmd(cli *cli) *cobra.Command {
	var clientID string
	var connectionName string
	var audience string
	var scopes []string
	var prompt string

	cmd := &cobra.Command{
		Use:       "test [login|token]",
		Short:     "",
		Long:      ``,
		ValidArgs: []string{"login", "token"},
		Args:      cobra.ExactValidArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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

			switch args[0] {
			case "login":
				audience = ""    // audience is only supported for get-token
				prompt = "login" // force a login page when using try-login
				scopes = cliLoginTestingScopes

			case "token":
				appType := client.GetAppType()

				cli.renderer.Infof("Domain:   " + tenant.Domain)
				cli.renderer.Infof("ClientID: " + clientID)
				cli.renderer.Infof("Type:     " + appType + "\n")

				// We can check here if the client is an m2m client, and if so
				// initiate the client credentials flow instead to fetch a token,
				// avoiding the browser and HTTP server shenanigans altogether.
				if appType == "non_interactive" {
					tokenResponse, err := runClientCredentialsFlow(cli, client, clientID, audience, tenant)
					if err != nil {
						return err
					}

					fmt.Fprint(cli.renderer.MessageWriter, "\n")
					cli.renderer.GetToken(client, tokenResponse)
					return nil
				}

				connectionName = ""
			}

			if proceed := runLoginFlowPreflightChecks(cli, client); !proceed {
				return nil
			}

			tokenResponse, err := runLoginFlow(
				cli,
				tenant,
				client,
				connectionName,
				audience,
				prompt,
				scopes,
			)
			if err != nil {
				return err
			}

			switch args[0] {
			case "login":
				var userInfo *authutil.UserInfo

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
			case "token":
				fmt.Fprint(cli.renderer.MessageWriter, "\n")
				cli.renderer.GetToken(client, tokenResponse)
				return nil
			}

			return nil
		},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.Flags().StringVarP(&clientID, "client-id", "c", "", "Client ID for which to test login or fetch a token.")
	cmd.Flags().StringVarP(&audience, "audience", "a", "", "The unique identifier of the target API you want to access.")
	cmd.Flags().StringSliceVarP(&scopes, "scope", "s", []string{}, "Client ID for which to test login.")
	cmd.Flags().StringVarP(&connectionName, "connection", "", "", "Connection to test during login.")
	return cmd
}
