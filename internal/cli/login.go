package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth"
	"github.com/auth0/auth0-cli/internal/open"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/spf13/cobra"
)

func loginCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Args:  cobra.NoArgs,
		Short: "Authenticate the Auth0 CLI",
		Long:  "Sign in to your Auth0 account and authorize the CLI to access the Management API.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return RunLogin(ctx, cli, false)
		},
	}

	return cmd
}

// RunLogin runs the login flow guiding the user through the process
// by showing the login instructions, opening the browser.
// Use `expired` to run the login from other commands setup:
// this will only affect the messages.
func RunLogin(ctx context.Context, cli *cli, expired bool) error {
	if expired {
		cli.renderer.Warnf("Please sign in to re-authorize the CLI.")
	} else {
		cli.renderer.Infof("✪ Welcome to the Auth0 CLI 🎊.")
		cli.renderer.Infof("To set it up, you will need to sign in to your Auth0 account and authorize the CLI to access the API.")
		cli.renderer.Infof("If you don't have an account, please go to https://auth0.com/signup, otherwise continue in the browser.\n\n")
	}

	a := &auth.Authenticator{}
	state, err := a.Start(ctx)
	if err != nil {
		return fmt.Errorf("could not start the authentication process: %w.", err)
	}

	cli.renderer.Infof("Your pairing code is: %s\n", ansi.Bold(state.UserCode))
	cli.renderer.Infof("This pairing code verifies your authentication with Auth0.")
	cli.renderer.Infof("Press Enter to open the browser (^C to quit)")
	fmt.Scanln()

	err = open.URL(state.VerificationURI)
	if err != nil {
		cli.renderer.Warnf("Couldn't open the URL, please do it manually: %s.", state.VerificationURI)
	}

	res, err := a.Wait(ctx, state)
	if err != nil {
		return fmt.Errorf("login error: %w", err)
	}

	cli.renderer.Infof("Successfully logged in.")
	cli.renderer.Infof("Tenant: %s\n", res.Domain)

	// store the refresh token
	secretsStore := &auth.Keyring{}
	err = secretsStore.Set(auth.SecretsNamespace, res.Domain, res.RefreshToken)
	if err != nil {
		// log the error but move on
		cli.renderer.Warnf("Could not store the refresh token locally, please expect to login again once your access token expired. See https://github.com/auth0/auth0-cli/blob/main/KNOWN-ISSUES.md.")
	}

	err = cli.addTenant(tenant{
		Name:        res.Tenant,
		Domain:      res.Domain,
		AccessToken: res.AccessToken,
		ExpiresAt: time.Now().Add(
			time.Duration(res.ExpiresIn) * time.Second,
		),
		Scopes: auth.RequiredScopes(),
	})
	if err != nil {
		return fmt.Errorf("Unexpected error adding tenant to config: %w", err)
	}

	if cli.config.DefaultTenant != res.Domain {
		promptText := fmt.Sprintf("Your default tenant is %s. Do you want to change it to %s?", cli.config.DefaultTenant, res.Domain)
		if confirmed := prompt.Confirm(promptText); !confirmed {
			return nil
		}
		cli.config.DefaultTenant = res.Domain
		if err := cli.persistConfig(); err != nil {
			return fmt.Errorf("An error occurred while setting the default tenant: %w", err)
		}
	}

	return nil
}
