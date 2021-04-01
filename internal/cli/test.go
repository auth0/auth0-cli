package cli

import (
	"fmt"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth/authutil"
	"github.com/spf13/cobra"
)

var (
	testClientID = Flag{
		Name:      "Client ID",
		LongForm:  "client-id",
		ShortForm: "c",
		Help:      "Client Id of an Auth0 application.",
	}

	testConnection = Flag{
		Name:     "Connection",
		LongForm: "connection",
		Help:     "Connection to test during login.",
	}

	testAudience = Flag{
		Name:      "Audience",
		LongForm:  "audience",
		ShortForm: "a",
		Help:      "The unique identifier of the target Audience you want to access.",
	}

	testScopes = Flag{
		Name:      "Scopes",
		LongForm:  "scopes",
		ShortForm: "s",
		Help:      "The list of scope you want to use to generate the token.",
	}
)

func testCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Try your universal login box or get a token",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(testTokenCmd(cli))
	cmd.AddCommand(testLoginCmd(cli))

	return cmd
}

func testLoginCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ClientID       string
		Audience       string
		ConnectionName string
	}

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Try out your universal login box",
		Long: `Launch a browser to try out your universal login box.
If --client-id is not provided, the default client "CLI Login Testing" will be used (and created if not exists.)`,
		Example: `auth0 test login
auth0 test login --client-id <id>
auth0 test login -c <id> --connection <connection>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			const commandKey = "test_login"
			var userInfo *authutil.UserInfo

			tenant, err := cli.getTenant()
			if err != nil {
				return err
			}

			// use the client ID as passed in by the user, or default to the
			// "CLI Login Testing" client if none passed. This client is only
			// used for testing login from the CLI and will be created if it
			// does not exist.
			if inputs.ClientID == "" {
				client, err := getOrCreateCLITesterClient(cli.api.Client)
				if err != nil {
					return fmt.Errorf("Unable to create an app for testing the login box: %w", err)
				}
				inputs.ClientID = client.GetClientID()
			}

			client, err := cli.api.Client.Read(inputs.ClientID)
			if err != nil {
				return fmt.Errorf("Unable to find client %s; if you specified a client, please verify it exists, otherwise re-run the command", inputs.ClientID)
			}

			if proceed := runLoginFlowPreflightChecks(cli, client); !proceed {
				return nil
			}

			tokenResponse, err := runLoginFlow(
				cli,
				tenant,
				client,
				inputs.ConnectionName,
				inputs.Audience, // audience is only supported for the test token command
				"login",         // force a login page when using the test login command
				cliLoginTestingScopes,
			)
			if err != nil {
				return fmt.Errorf("An unexpected error occurred while logging in to client %s: %w", inputs.ClientID, err)
			}

			if err := ansi.Spinner("Fetching user metadata", func() error {
				// Use the access token to fetch user information from the /userinfo
				// endpoint.
				userInfo, err = authutil.FetchUserInfo(tenant.Domain, tokenResponse.AccessToken)
				return err
			}); err != nil {
				return fmt.Errorf("An unexpected error occurred: %w", err)
			}

			fmt.Fprint(cli.renderer.MessageWriter, "\n")
			cli.renderer.TryLogin(userInfo, tokenResponse)

			isFirstRun, err := cli.isFirstCommandRun(inputs.ClientID, commandKey)
			if err != nil {
				return err
			}

			if isFirstRun {
				cli.renderer.Infof("%s Login flow is working! Next, try downloading and running a Quickstart: 'auth0 quickstarts download %s'",
					ansi.Faint("Hint:"), inputs.ClientID)

				if err := cli.setFirstCommandRun(inputs.ClientID, commandKey); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	testClientID.RegisterString(cmd, &inputs.ClientID, "")
	testAudience.RegisterString(cmd, &inputs.Audience, "")
	testConnection.RegisterString(cmd, &inputs.ConnectionName, "")
	return cmd
}

func testTokenCmd(cli *cli) *cobra.Command {
	var inputs struct {
		ClientID string
		Audience string
		Scopes   []string
	}

	cmd := &cobra.Command{
		Use:   "token",
		Short: "Fetch a token for the given client and API",
		Long: `Fetch an access token for the given client.
If --client-id is not provided, the default client "CLI Login Testing" will be used (and created if not exists).
Additionally, you can also specify the --audience and --scope to use.`,
		Example: `auth0 test token
auth0 test token --client-id <id> --audience <audience> --scopes <scope1,scope2>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			tenant, err := cli.getTenant()
			if err != nil {
				return err
			}

			// use the client ID as passed in by the user, or default to the
			// "CLI Login Testing" client if none passed. This client is only
			// used for testing login from the CLI and will be created if it
			// does not exist.
			if inputs.ClientID == "" {
				client, err := getOrCreateCLITesterClient(cli.api.Client)
				if err != nil {
					return fmt.Errorf("Unable to create an app to test getting a token: %w", err)
				}
				inputs.ClientID = client.GetClientID()
			}

			client, err := cli.api.Client.Read(inputs.ClientID)
			if err != nil {
				return fmt.Errorf("Unable to find client %s; if you specified a client, please verify it exists, otherwise re-run the command", inputs.ClientID)
			}

			appType := client.GetAppType()

			cli.renderer.Infof("Domain:   " + tenant.Domain)
			cli.renderer.Infof("ClientID: " + inputs.ClientID)
			cli.renderer.Infof("Type:     " + appType + "\n")

			// We can check here if the client is an m2m client, and if so
			// initiate the client credentials flow instead to fetch a token,
			// avoiding the browser and HTTP server shenanigans altogether.
			if appType == "non_interactive" {
				tokenResponse, err := runClientCredentialsFlow(cli, client, inputs.ClientID, inputs.Audience, tenant)
				if err != nil {
					return fmt.Errorf("An unexpected error occurred while logging in to machine-to-machine client %s: %w", inputs.ClientID, err)
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
				"", // specifying a connection is only supported for the test login command
				inputs.Audience,
				"", // We don't want to force a prompt for the test token command
				inputs.Scopes,
			)
			if err != nil {
				return fmt.Errorf("An unexpected error occurred when logging in to client %s: %w", inputs.ClientID, err)
			}

			fmt.Fprint(cli.renderer.MessageWriter, "\n")
			cli.renderer.GetToken(client, tokenResponse)
			return nil
		},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	testClientID.RegisterString(cmd, &inputs.ClientID, "")
	testAudience.RegisterString(cmd, &inputs.Audience, "")
	testScopes.RegisterStringSlice(cmd, &inputs.Scopes, nil)
	return cmd
}
