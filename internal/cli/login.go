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
)

type LoginInputs struct {
	Domain       string
	ClientID     string
	ClientSecret string
}

func (i *LoginInputs) shouldLoginAsMachine() bool {
	return i.ClientID != "" || i.ClientSecret != "" || i.Domain != ""
}

func loginCmd(cli *cli) *cobra.Command {
	var inputs LoginInputs

	cmd := &cobra.Command{
		Use:   "login",
		Args:  cobra.NoArgs,
		Short: "Authenticate the Auth0 CLI",
		Long:  "Authenticates the Auth0 CLI either as a user using personal credentials or as a machine using client credentials.",
		Example: `auth0 login
auth0 login --domain <tenant-domain> --client-id <client-id> --client-secret <client-secret>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if inputs.shouldLoginAsMachine() {
				if err := RunLoginAsMachine(ctx, inputs, cli, cmd); err != nil {
					return err
				}
			} else {
				cli.renderer.Output(fmt.Sprintf(
					"%s\n\n%s\n\n",
					"✪ Welcome to the Auth0 CLI 🎊",
					"If you don't have an account, please create one here: https://auth0.com/signup.",
				))
				if _, err := RunLoginAsUser(ctx, cli); err != nil {
					return err
				}
			}

			cli.renderer.Infof("Successfully authenticated to %s", inputs.Domain)
			cli.tracker.TrackCommandRun(cmd, cli.config.InstallID)

			return nil
		},
	}

	loginTenantDomain.RegisterString(cmd, &inputs.Domain, "")
	loginClientID.RegisterString(cmd, &inputs.ClientID, "")
	loginClientSecret.RegisterString(cmd, &inputs.ClientSecret, "")
	cmd.MarkFlagsRequiredTogether("client-id", "client-secret", "domain")

	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		_ = cmd.Flags().MarkHidden("tenant")
		_ = cmd.Flags().MarkHidden("json")
		_ = cmd.Flags().MarkHidden("no-input")
		cmd.Parent().HelpFunc()(cmd, args)
	})

	return cmd
}

// RunLoginAsUser runs the login flow guiding the user through the process
// by showing the login instructions, opening the browser.
func RunLoginAsUser(ctx context.Context, cli *cli) (Tenant, error) {
	state, err := cli.authenticator.Start(ctx)
	if err != nil {
		return Tenant{}, fmt.Errorf("Failed to start the authentication process: %w.", err)
	}

	message := fmt.Sprintf("Your device confirmation code is: %s\n\n", ansi.Bold(state.UserCode))
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
		Scopes:      auth.RequiredScopes(),
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

// RunLoginAsMachine facilitates the authentication process using client credentials (client ID, client secret)
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

	token, err := auth.GetAccessTokenFromClientCreds(auth.ClientCredentials{
		ClientID:     inputs.ClientID,
		ClientSecret: inputs.ClientSecret,
		Domain:       inputs.Domain,
	})
	if err != nil {
		return err
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
