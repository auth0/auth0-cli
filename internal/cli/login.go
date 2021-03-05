package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth"
	"github.com/auth0/auth0-cli/internal/open"
	"github.com/spf13/cobra"
)

func loginCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate the Auth0 CLI",
		Long:  "sign in to your Auth0 account and authorize the CLI to access the API",
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
		cli.renderer.Heading("✪ Welcome to the Auth0 CLI 🎊.")
		cli.renderer.Infof("To set it up, you will need to sign in to your Auth0 account and authorize the CLI to access the API.")
		cli.renderer.Infof("If you don't have an account, please go to https://auth0.com/signup, otherwise continue in the browser.\n\n")
	}

	a := &auth.Authenticator{Secrets: &auth.Keyring{}}
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
	cli.renderer.Infof("Tenant: %s\n", res.Tenant)

	return cli.addTenant(tenant{
		Name:        res.Tenant,
		Domain:      res.Domain,
		AccessToken: res.AccessToken,
		ExpiresAt: time.Now().Add(
			time.Duration(res.ExpiresIn) * time.Second,
		),
	})

}
