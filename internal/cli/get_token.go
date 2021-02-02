package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func getTokenCmd(cli *cli) *cobra.Command {
	var clientID string
	var audience string
	var scopes []string

	cmd := &cobra.Command{
		Use:   "get-token",
		Short: "fetch a token for the given client and API.",
		Long: `$ auth0 get-token
Fetch an access token for the given client and API.
`,
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

			if proceed := runLoginFlowPreflightChecks(cli, client); !proceed {
				return nil
			}

			tokenResponse, err := runLoginFlow(
				cli,
				tenant,
				client,
				"", // specifying a connection is only supported for try-login
				audience,
				"", // We don't want to force a prompt for get-token
				scopes,
			)
			if err != nil {
				return err
			}

			fmt.Fprint(cli.renderer.MessageWriter, "\n")
			cli.renderer.GetToken(client, tokenResponse)
			return nil
		},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.Flags().StringVarP(&clientID, "client-id", "c", "", "Client ID for which to fetch a token.")
	cmd.Flags().StringVarP(&audience, "audience", "a", "", "The unique identifier of the target API you want to access.")
	cmd.Flags().StringSliceVarP(&scopes, "scope", "s", []string{}, "Client ID for which to test login.")
	return cmd
}
