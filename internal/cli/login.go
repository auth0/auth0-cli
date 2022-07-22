package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth"
	"github.com/auth0/auth0-cli/internal/prompt"
)

func loginCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Args:  cobra.NoArgs,
		Short: "Authenticate the Auth0 CLI",
		Long:  "Sign in to your Auth0 account and authorize the CLI to access the Management API.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if _, err := RunLogin(ctx, cli, false); err != nil {
				return err
			}

			cli.tracker.TrackCommandRun(cmd, cli.config.InstallID)

			return nil
		},
	}

	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		_ = cmd.Flags().MarkHidden("tenant")
		cmd.Parent().HelpFunc()(cmd, args)
	})

	return cmd
}

// RunLogin runs the login flow guiding the user through the process
// by showing the login instructions, opening the browser.
// Use `expired` to run the login from other commands setup:
// this will only affect the messages.
func RunLogin(ctx context.Context, cli *cli, expired bool) (Tenant, error) {
	message := fmt.Sprintf(
		"%s\n\n%s\n\n",
		"✪ Welcome to the Auth0 CLI 🎊",
		"If you don't have an account, please go to https://auth0.com/signup.",
	)

	if expired {
		message = "Please sign in to re-authorize the CLI."
		cli.renderer.Warnf(message)
	} else {
		cli.renderer.Output(message)
	}

	state, err := cli.authenticator.Start(ctx)
	if err != nil {
		return Tenant{}, fmt.Errorf("Could not start the authentication process: %w.", err)
	}

	message = fmt.Sprintf("Your device confirmation code is: %s\n\n", ansi.Bold(state.UserCode))
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
		ExpiresAt: time.Now().Add(
			time.Duration(result.ExpiresIn) * time.Second,
		),
		Scopes: auth.RequiredScopes(),
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
