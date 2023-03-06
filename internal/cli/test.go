package cli

import (
	"errors"
	"fmt"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth/authutil"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
)

const (
	NEW_CLIENT = "NEW CLIENT"
)

var (
	testClientID = Argument{
		Name: "Client ID",
		Help: "Client ID of an Auth0 application.",
	}

	testConnectionName = Flag{
		Name:      "Connection Name",
		LongForm:  "connection-name",
		ShortForm: "c",
		Help:      "The connection name to test during login.",
	}

	testAudience = Flag{
		Name:      "Audience",
		LongForm:  "audience",
		ShortForm: "a",
		Help:      "The unique identifier of the target API you want to access.",
	}

	testScopes = Flag{
		Name:      "Scopes",
		LongForm:  "scopes",
		ShortForm: "s",
		Help:      "The list of scopes you want to use.",
	}

	testDomain = Flag{
		Name:      "Custom Domain",
		LongForm:  "domain",
		ShortForm: "d",
		Help:      "One of your custom domains.",
	}

	errNoCustomDomains = errors.New("there are currently no custom domains")
)

type testCmdInputs struct {
	ClientID       string
	Audience       string
	Scopes         []string
	ConnectionName string
	CustomDomain   string
}

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
	var inputs testCmdInputs

	cmd := &cobra.Command{
		Use:   "login",
		Args:  cobra.MaximumNArgs(1),
		Short: "Try out your tenant's Universal Login experience",
		Long:  "Try out your tenant's Universal Login experience in a browser.",
		Example: `  auth0 test login
  auth0 test login <client-id>
  auth0 test login <client-id> --connection-name <connection-name>
  auth0 test login <client-id> --connection-name <connection-name> --audience <api-identifier|api-audience>
  auth0 test login <client-id> --connection-name <connection-name> --audience <api-identifier|api-audience> --domain <domain>
  auth0 test login <client-id> --connection-name <connection-name> --audience <api-identifier|api-audience> --domain <domain> --scopes <scope1,scope2>
  auth0 test login <client-id> -c <connection-name> -a <api-identifier|api-audience> -d <domain> -s <scope1,scope2> --force
  auth0 test login <client-id> -c <connection-name> -a <api-identifier|api-audience> -d <domain> -s <scope1,scope2> --json
  auth0 test login <client-id> -c <connection-name> -a <api-identifier|api-audience> -d <domain> -s <scope1,scope2> --force --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := selectClientToUseForTestsAndValidateExistence(cli, cmd, args, &inputs)
			if err != nil {
				return err
			}

			if client.GetAppType() == appTypeNonInteractive {
				return fmt.Errorf(
					"cannot test the Universal Login with a %s application.\n\n"+
						"Run 'auth0 test token %s' to fetch an access token instead.",
					ansi.Bold("Machine to Machine"),
					client.GetClientID(),
				)
			}

			err = testDomain.Pick(cmd, &inputs.CustomDomain, cli.customDomainPickerOptions)
			if err != nil && err != errNoCustomDomains {
				return err
			}

			if proceed := runLoginFlowPreflightChecks(cli, client); !proceed {
				return nil
			}

			if inputs.Audience != "" {
				if err := checkClientIsAuthorizedForAPI(cli, client, inputs.Audience); err != nil {
					return err
				}
			}

			tenant, err := cli.getTenant()
			if err != nil {
				return err
			}

			tokenResponse, err := runLoginFlow(
				cli,
				tenant,
				client,
				inputs.ConnectionName,
				inputs.Audience,
				"login", // Force a login page when using the test login command.
				inputs.Scopes,
				inputs.CustomDomain,
			)
			if err != nil {
				return fmt.Errorf("failed to log into the client %s: %w", inputs.ClientID, err)
			}

			var userInfo *authutil.UserInfo
			if err := ansi.Spinner("Fetching user metadata", func() error {
				// Use the access token to fetch user information from the /userinfo endpoint.
				userInfo, err = authutil.FetchUserInfo(tenant.Domain, tokenResponse.AccessToken)
				return err
			}); err != nil {
				return fmt.Errorf("failed to fetch user info: %w", err)
			}

			cli.renderer.Newline()
			cli.renderer.TestLogin(userInfo, tokenResponse)
			cli.renderer.Newline()

			const commandKey = "test_login"
			isFirstRun, err := cli.isFirstCommandRun(inputs.ClientID, commandKey)
			if err != nil {
				return err
			}

			if isFirstRun {
				cli.renderer.Infof("Login flow is working!")
				cli.renderer.Infof(
					"%s Consider downloading and running a quickstart next by running `auth0 quickstarts download %s`",
					ansi.Faint("Hint:"),
					inputs.ClientID,
				)

				if err := cli.setFirstCommandRun(inputs.ClientID, commandKey); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	testAudience.RegisterString(cmd, &inputs.Audience, "")
	testScopes.RegisterStringSlice(cmd, &inputs.Scopes, cliLoginTestingScopes)
	testConnectionName.RegisterString(cmd, &inputs.ConnectionName, "")
	testDomain.RegisterString(cmd, &inputs.CustomDomain, "")

	return cmd
}

func testTokenCmd(cli *cli) *cobra.Command {
	var inputs testCmdInputs

	cmd := &cobra.Command{
		Use:   "token",
		Args:  cobra.MaximumNArgs(1),
		Short: "Request an access token for a given application and API",
		Long: "Request an access token for a given application. " +
			"Specify the API you want this token for with `--audience` (API Identifier). " +
			"Additionally, you can also specify the `--scopes` to grant.",
		Example: `  auth0 test token
  auth0 test token <client-id> --audience <api-audience|api-identifier> --scopes <scope1,scope2>
  auth0 test token <client-id> -a <api-audience|api-identifier> -s <scope1,scope2>
  auth0 test token <client-id> -a <api-audience|api-identifier> -s <scope1,scope2> --force
  auth0 test token <client-id> -a <api-audience|api-identifier> -s <scope1,scope2> --json
  auth0 test token <client-id> -a <api-audience|api-identifier> -s <scope1,scope2> --force --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := selectClientToUseForTestsAndValidateExistence(cli, cmd, args, &inputs)
			if err != nil {
				return err
			}

			if err := testAudience.Ask(cmd, inputs.Audience, nil); err != nil {
				return nil
			}

			tenant, err := cli.getTenant()
			if err != nil {
				return err
			}

			appType := client.GetAppType()

			cli.renderer.Infof("Domain    : " + ansi.Blue(tenant.Domain))
			cli.renderer.Infof("Client ID : " + ansi.Bold(client.GetClientID()))
			cli.renderer.Infof("Type      : " + display.ApplyColorToFriendlyAppType(display.FriendlyAppType(appType)))
			cli.renderer.Newline()

			if appType == appTypeNonInteractive {
				tokenResponse, err := runClientCredentialsFlow(cli, client, inputs.Audience, tenant.Domain)
				if err != nil {
					return fmt.Errorf(
						"failed to log in with client credentials for client with ID %q: %w",
						inputs.ClientID,
						err,
					)
				}

				cli.renderer.TestToken(client, tokenResponse)

				return nil
			}

			if proceed := runLoginFlowPreflightChecks(cli, client); !proceed {
				return nil
			}

			tokenResponse, err := runLoginFlow(
				cli,
				tenant,
				client,
				"", // Specifying a connection is only supported for the test login command.
				inputs.Audience,
				"", // We don't want to force a prompt for the test token command.
				inputs.Scopes,
				"", // Specifying a custom domain is only supported for the test login command.
			)
			if err != nil {
				return fmt.Errorf("failed to log into the client %s: %w", inputs.ClientID, err)
			}

			cli.renderer.TestToken(client, tokenResponse)

			return nil
		},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	testAudience.IsRequired = true
	testAudience.RegisterString(cmd, &inputs.Audience, "")
	testScopes.RegisterStringSlice(cmd, &inputs.Scopes, nil)

	return cmd
}

func selectClientToUseForTestsAndValidateExistence(cli *cli, cmd *cobra.Command, args []string, inputs *testCmdInputs) (*management.Client, error) {
	if len(args) == 0 {
		if err := testClientID.Pick(cmd, &inputs.ClientID, cli.appPickerWithCreateOption); err != nil {
			return nil, err
		}

		if inputs.ClientID == NEW_CLIENT {
			client := &management.Client{
				Name:             auth0.String(cliLoginTestingClientName),
				Description:      auth0.String(cliLoginTestingClientDescription),
				Callbacks:        &[]string{cliLoginTestingCallbackURL},
				InitiateLoginURI: auth0.String(cliLoginTestingInitiateLoginURI),
			}

			if err := cli.api.Client.Create(client); err != nil {
				return nil, fmt.Errorf("failed to create a new client to use for testing the login: %w", err)
			}

			inputs.ClientID = client.GetClientID()

			cli.renderer.Infof("New client created successfully.")
			cli.renderer.Infof(
				"If you wish to remove the created client after testing the login, run: 'auth0 apps delete %s'",
				client.GetClientID(),
			)
			cli.renderer.Newline()

			return client, nil
		}
	} else {
		inputs.ClientID = args[0]
	}

	client, err := cli.api.Client.Read(inputs.ClientID)
	if err != nil {
		return nil, fmt.Errorf("failed to find client with ID :%q: %w", inputs.ClientID, err)
	}

	return client, nil
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

func (c *cli) appPickerWithCreateOption() (pickerOptions, error) {
	options, err := c.appPickerOptions()()
	if err != nil {
		return nil, err
	}

	enhancedOptions := []pickerOption{
		{
			value: NEW_CLIENT,
			label: "Create a new client to use for testing the login",
		},
	}
	enhancedOptions = append(enhancedOptions, options...)

	return enhancedOptions, nil
}

func checkClientIsAuthorizedForAPI(cli *cli, client *management.Client, audience string) error {
	var list *management.ClientGrantList
	if err := ansi.Waiting(func() (err error) {
		list, err = cli.api.ClientGrant.List(
			management.Parameter("audience", audience),
			management.Parameter("client_id", client.GetClientID()),
		)
		return err
	}); err != nil {
		return fmt.Errorf(
			"failed to find client grants for API identifier %q and client ID %q: %w",
			audience,
			client.GetClientID(),
			err,
		)
	}

	if len(list.ClientGrants) < 1 {
		return fmt.Errorf(
			"the %s application is not authorized to request access tokens for this API %s.\n\n"+
				"Run: 'auth0 apps open %s' to open the dashboard and authorize the application.",
			ansi.Bold(client.GetName()),
			ansi.Bold(audience),
			client.GetClientID(),
		)
	}

	return nil
}
