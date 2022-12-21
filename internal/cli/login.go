package cli

import (
	"context"
	"fmt"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth"
	"github.com/auth0/auth0-cli/internal/prompt"
)

var (
	loginTenantDomain = Flag{
		Name:         "Tenant Domain",
		LongForm:     "domain",
		Help:         "Tenant domain of the application when authenticating via client credentials.",
		IsRequired:   false,
		AlwaysPrompt: false,
	}

	loginClientID = Flag{
		Name:         "Client ID",
		LongForm:     "client-id",
		Help:         "Client ID of the application when authenticating via client credentials.",
		IsRequired:   false,
		AlwaysPrompt: false,
	}

	loginClientSecret = Flag{
		Name:         "Client Secret",
		LongForm:     "client-secret",
		Help:         "Client secret of the application when authenticating via client credentials.",
		IsRequired:   false,
		AlwaysPrompt: false,
	}

	loginAdditionalScopes = Flag{
		Name:         "Additional Scopes",
		LongForm:     "scopes",
		Help:         "Additional scopes to request when authenticating via device code flow. By default, only scopes for first-class functions are requested. Primarily useful when using the api command to execute arbitrary Management API requests.",
		IsRequired:   false,
		AlwaysPrompt: false,
	}
)

type LoginInputs struct {
	Domain           string
	ClientID         string
	ClientSecret     string
	AdditionalScopes []string
}

func (i *LoginInputs) isLoggingInAsAMachine() bool {
	return i.ClientID != "" || i.ClientSecret != "" || i.Domain != ""
}

func (i *LoginInputs) isLoggingInWithAdditionalScopes() bool {
	return len(i.AdditionalScopes) > 0
}

func loginCmd(cli *cli) *cobra.Command {
	var inputs LoginInputs

	cmd := &cobra.Command{
		Use:   "login",
		Args:  cobra.NoArgs,
		Short: "Authenticate the Auth0 CLI",
		Long:  "Authenticates the Auth0 CLI either as a user using personal credentials or as a machine using client credentials.\n\nAuthenticating as a user is recommended when working on a personal machine or other interactive environment; it is not available for Private Cloud users. Authenticating as a machine is recommended when running on a server or non-interactive environments (ex: CI). ",
		Example: `auth0 login
auth0 login --domain <tenant-domain> --client-id <client-id> --client-secret <client-secret>
auth0 login --scopes "read:client_grants,create:client_grants"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var selectedLoginType string
			const loginAsUser, loginAsMachine = "As a user", "As a machine"

			// We want to prompt if we don't pass the following flags:
			// --no-input, --scopes, --client-id, --client-secret, --domain.
			// Because then the prompt is unnecessary as we know the login type.
			shouldPrompt := !inputs.isLoggingInAsAMachine() && !cli.noInput && !inputs.isLoggingInWithAdditionalScopes()
			if shouldPrompt {
				cli.renderer.Output(
					fmt.Sprintf(
						"%s\n\n%s\n%s\n\n%s\n%s\n%s\n%s\n\n",
						ansi.Bold("✪ Welcome to the Auth0 CLI 🎊"),
						"An Auth0 tenant is required to operate this CLI.",
						"To create one, visit: https://auth0.com/signup.",
						"You may authenticate to your tenant either as a user with personal",
						"credentials or as a machine via client credentials. For more",
						"information about authenticating the CLI to your tenant, visit",
						"the docs: https://auth0.github.io/auth0-cli/auth0_login.html",
					),
				)

				label := "How would you like to authenticate?"
				help := fmt.Sprintf(
					"%s\n%s\n",
					"Authenticating as a user is recommended if performing ad-hoc operations or working locally.",
					"Alternatively, authenticating as a machine is recommended for automated workflows (ex:CI).",
				)
				input := prompt.SelectInput("", label, help, []string{loginAsUser, loginAsMachine}, loginAsUser, shouldPrompt)
				if err := prompt.AskOne(input, &selectedLoginType); err != nil {
					return handleInputError(err)
				}
			}

			ctx := cmd.Context()

			// Allows to skip to user login if either the --no-input or --scopes flag is passed.
			shouldLoginAsUser := (cli.noInput && !inputs.isLoggingInAsAMachine()) || inputs.isLoggingInWithAdditionalScopes() || selectedLoginType == loginAsUser
			if shouldLoginAsUser {
				if _, err := RunLoginAsUser(ctx, cli, inputs.AdditionalScopes); err != nil {
					return fmt.Errorf("failed to start the authentication process: %w", err)
				}
			} else {
				if err := RunLoginAsMachine(ctx, inputs, cli, cmd); err != nil {
					return err
				}
			}

			cli.tracker.TrackCommandRun(cmd, cli.config.InstallID)

			if len(cli.config.Tenants) > 1 {
				cli.renderer.Infof("%s switch between authenticated tenants with `auth0 tenants use <tenant>`",
					ansi.Faint("Hint:"),
				)
			}

			return nil
		},
	}

	loginTenantDomain.RegisterString(cmd, &inputs.Domain, "")
	loginClientID.RegisterString(cmd, &inputs.ClientID, "")
	loginClientSecret.RegisterString(cmd, &inputs.ClientSecret, "")
	loginAdditionalScopes.RegisterStringSlice(cmd, &inputs.AdditionalScopes, []string{})
	cmd.MarkFlagsRequiredTogether("client-id", "client-secret", "domain")
	cmd.MarkFlagsMutuallyExclusive("client-id", "scopes")

	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		_ = cmd.Flags().MarkHidden("tenant")
		cmd.Parent().HelpFunc()(cmd, args)
	})

	return cmd
}

// RunLoginAsUser runs the login flow guiding the user through the process
// by showing the login instructions, opening the browser.
func RunLoginAsUser(ctx context.Context, cli *cli, additionalScopes []string) (Tenant, error) {
	state, err := cli.authenticator.GetDeviceCode(ctx, additionalScopes)
	if err != nil {
		return Tenant{}, fmt.Errorf("failed to get the device code: %w", err)
	}

	message := fmt.Sprintf("\n%s\n%s%s\n\n",
		"A browser window needs to be opened to complete authentication.",
		"Note your device confirmation code: ",
		ansi.Bold(state.UserCode))
	cli.renderer.Output(message)

	if cli.noInput {
		message = "Open the following URL in a browser: %s\n"
		cli.renderer.Infof(message, ansi.Green(state.VerificationURI))
	} else {
		message = "%s to open the browser to log in or %s to quit..."
		cli.renderer.Infof(message, ansi.Green("Press Enter"), ansi.Red("^C"))

		if _, err = fmt.Scanln(); err != nil {
			return Tenant{}, err
		}

		if err = browser.OpenURL(state.VerificationURI); err != nil {
			message = "Couldn't open the URL, please do it manually: %s."
			cli.renderer.Warnf(message, state.VerificationURI)
		}
	}

	var result auth.Result
	err = ansi.Spinner("Waiting for the login to complete in the browser", func() error {
		result, err = cli.authenticator.Wait(ctx, state)
		return err
	})
	if err != nil {
		return Tenant{}, fmt.Errorf("login error: %w", err)
	}

	cli.renderer.Newline()
	cli.renderer.Infof("Successfully logged in.")
	cli.renderer.Infof("Tenant: %s", result.Domain)
	cli.renderer.Newline()

	// Store the refresh token.
	secretsStore := &auth.Keyring{}
	err = secretsStore.Set(auth.SecretsNamespace, result.Domain, result.RefreshToken)
	if err != nil {
		message = "Could not store the refresh token locally, " +
			"please expect to login again once your access token expired. See %s."
		cli.renderer.Warnf(message, "https://github.com/auth0/auth0-cli/blob/main/KNOWN-ISSUES.md")
	}

	tenant := Tenant{
		Name:        result.Tenant,
		Domain:      result.Domain,
		AccessToken: result.AccessToken,
		ExpiresAt:   result.ExpiresAt,
		Scopes:      append(auth.RequiredScopes(), additionalScopes...),
	}

	err = cli.addTenant(tenant)
	if err != nil {
		return Tenant{}, fmt.Errorf("Failed to add the tenant to the config: %w", err)
	}

	if err := checkInstallID(cli); err != nil {
		return Tenant{}, fmt.Errorf("Failed to update the config: %w", err)
	}

	if cli.config.DefaultTenant != result.Domain {
		message = fmt.Sprintf(
			"Your default tenant is %s. Do you want to change it to %s?",
			cli.config.DefaultTenant,
			result.Domain,
		)
		if confirmed := prompt.Confirm(message); !confirmed {
			return Tenant{}, nil
		}

		cli.config.DefaultTenant = result.Domain
		if err := cli.persistConfig(); err != nil {
			message = "Failed to set the default tenant, please try 'auth0 tenants use %s' instead: %w"
			cli.renderer.Warnf(message, result.Domain, err)
		}
	}

	return tenant, nil
}

// RunLoginAsMachine facilitates the authentication process using client credentials (client ID, client secret).
func RunLoginAsMachine(ctx context.Context, inputs LoginInputs, cli *cli, cmd *cobra.Command) error {
	if err := loginTenantDomain.Ask(cmd, &inputs.Domain, nil); err != nil {
		return err
	}

	if err := loginClientID.Ask(cmd, &inputs.ClientID, nil); err != nil {
		return err
	}

	if err := loginClientSecret.AskPassword(cmd, &inputs.ClientSecret, nil); err != nil {
		return err
	}

	token, err := auth.GetAccessTokenFromClientCreds(
		ctx,
		auth.ClientCredentials{
			ClientID:     inputs.ClientID,
			ClientSecret: inputs.ClientSecret,
			Domain:       inputs.Domain,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"failed to fetch access token using client credentials. \n\n"+
				"Ensure that the provided client-id, client-secret and domain are correct. \n\nerror: %w\n", err)
	}

	t := Tenant{
		Domain:       inputs.Domain,
		AccessToken:  token.AccessToken,
		ExpiresAt:    token.ExpiresAt,
		ClientID:     inputs.ClientID,
		ClientSecret: inputs.ClientSecret,
	}

	if err := cli.addTenant(t); err != nil {
		return fmt.Errorf("unexpected error when attempting to save tenant data: %w", err)
	}

	cli.renderer.Newline()
	cli.renderer.Infof("Successfully logged in.")
	cli.renderer.Infof("Tenant: %s", inputs.Domain)

	if err := checkInstallID(cli); err != nil {
		return fmt.Errorf("failed to update the config: %w", err)
	}

	return nil
}
