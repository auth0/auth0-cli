package cli

import (
	"errors"
	"fmt"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth/authutil"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

var (
	testClientIDArg = Argument{
		Name: "Client ID",
		Help: "Client Id of an Auth0 application.",
	}

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
		Help:      "The unique identifier of the target API you want to access.",
	}

	testAudienceRequired = Flag{
		Name:       testAudience.Name,
		LongForm:   testAudience.LongForm,
		ShortForm:  testAudience.ShortForm,
		Help:       testAudience.Help,
		IsRequired: true,
	}

	testScopes = Flag{
		Name:      "Scopes",
		LongForm:  "scopes",
		ShortForm: "s",
		Help:      "The list of scopes you want to use.",
	}

	testDomainArg = Argument{
		Name: "Custom Domain",
		Help: "One of your custom domains.",
	}

	testDomain = Flag{
		Name:      "Custom Domain",
		LongForm:  "domain",
		ShortForm: "d",
		Help:      "One of your custom domains.",
	}

	errNoCustomDomains = errors.New("there are currently no custom domains")
)

func testCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Try your Universal Login box or get a token",
		Long:  "Try your Universal Login box or get a token.",
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
		Scopes         []string
		ConnectionName string
		CustomDomain   string
	}

	cmd := &cobra.Command{
		Use:   "login",
		Args:  cobra.MaximumNArgs(1),
		Short: "Try out your Universal Login box",
		Long:  "Launch a browser to try out your Universal Login box.",
		Example: `auth0 test login
auth0 test login <client-id>
auth0 test login <client-id> --connection <connection>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			const commandKey = "test_login"
			var userInfo *authutil.UserInfo
			isTempClient := false

			tenant, err := cli.getTenant()
			if err != nil {
				return err
			}

			if len(args) == 0 {
				err := testClientIDArg.Pick(cmd, &inputs.ClientID, cli.appPickerOptions)
				if err != nil {
					if err != errNoApps {
						return err
					}
					cli.renderer.Infof("No applications to select from, we will create a default test application " +
						"for you and remove it once the test is complete.")
					client := &management.Client{
						Name:             auth0.String(cliLoginTestingClientName),
						Description:      auth0.String(cliLoginTestingClientDescription),
						Callbacks:        []interface{}{cliLoginTestingCallbackURL},
						InitiateLoginURI: auth0.String(cliLoginTestingInitiateLoginURI),
					}
					if err := cli.api.Client.Create(client); err != nil {
						return fmt.Errorf("Unable to create an app for testing the login box: %w", err)
					}
					inputs.ClientID = client.GetClientID()
					isTempClient = true
					cli.renderer.Infof("Default test application successfully created\n")
				}
			} else {
				inputs.ClientID = args[0]
			}

			defer cleanupTempApplication(isTempClient, cli, inputs.ClientID)

			client, err := cli.api.Client.Read(inputs.ClientID)
			if err != nil {
				return fmt.Errorf("Unable to find client %s; if you specified a client, please verify it exists, otherwise re-run the command", inputs.ClientID)
			}

			if inputs.CustomDomain == "" {
				err = testDomainArg.Pick(cmd, &inputs.CustomDomain, cli.customDomainPickerOptions)
				if err != nil && err != errNoCustomDomains {
					return err
				}
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
				inputs.Scopes,
				inputs.CustomDomain,
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
	testAudience.RegisterString(cmd, &inputs.Audience, "")
	testScopes.RegisterStringSlice(cmd, &inputs.Scopes, cliLoginTestingScopes)
	testConnection.RegisterString(cmd, &inputs.ConnectionName, "")
	testDomain.RegisterString(cmd, &inputs.CustomDomain, "")
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
		Args:  cobra.NoArgs,
		Short: "Fetch a token for the given application and API",
		Long: `Fetch an access token for the given application.
If --client-id is not provided, the default client "CLI Login Testing" will be used (and created if not exists).
Specify the API you want this token for with --audience (API Identifer). Additionally, you can also specify the --scope to use.`,
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
				"",
			)
			if err != nil {
				return fmt.Errorf("An unexpected error occurred when logging in to client %s: %w", inputs.ClientID, err)
			}
			cli.renderer.GetToken(client, tokenResponse)
			return nil
		},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	testClientID.RegisterString(cmd, &inputs.ClientID, "")
	testAudienceRequired.RegisterString(cmd, &inputs.Audience, "")
	testScopes.RegisterStringSlice(cmd, &inputs.Scopes, nil)
	return cmd
}

// cleanupTempApplication will delete the specified application if it is marked
// as a temporary application. It will log success or failure to the user.
func cleanupTempApplication(isTemp bool, cli *cli, id string) {
	if isTemp {
		if err := cli.api.Client.Delete(id); err != nil {
			cli.renderer.Errorf("unable to remove the default test application", err.Error())
		}
		cli.renderer.Infof("Default test application removed")
	}
}

func (c *cli) customDomainPickerOptions() (pickerOptions, error) {
	var opts pickerOptions

	domains, err := c.api.CustomDomain.List()
	if err != nil {
		errStatus := err.(management.Error)
		// 403 is a valid response for free tenants that don't have
		// custom domains enabled
		if errStatus != nil && errStatus.Status() == 403 {
			return nil, errNoCustomDomains
		}

		return nil, err
	}

	tenant, err := c.getTenant()
	if err != nil {
		return nil, err
	}

	for _, d := range domains {
		if d.GetStatus() != "ready" {
			continue
		}

		opts = append(opts, pickerOption{value: d.GetDomain(), label: d.GetDomain()})
	}

	if len(opts) == 0 {
		return nil, errNoCustomDomains
	}

	opts = append(opts, pickerOption{value: "", label: fmt.Sprintf("none (use %s)", tenant.Domain)})

	return opts, nil
}
