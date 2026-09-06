package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/localserver"
)

func initCmd(cli *cli) *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Start a local identity server for agent-driven development",
		Long: "Start a local OIDC identity server on localhost so a coding agent can build\n" +
			"and test a real Auth0 login flow before any Auth0 account exists.\n\n" +
			"Point your Auth0 SDK's domain or issuerBaseURL at the printed address.\n" +
			"Every login is auto-approved as a local test user. Tokens are signed with\n" +
			"an ephemeral key that has no relation to any real Auth0 tenant — they will\n" +
			"fail production validation by design.\n\n" +
			"When you are ready to use a real tenant, run `auth0 claim` to replay your\n" +
			"local configuration onto a new Auth0 account in one step.",
		Example: `  auth0 init
  auth0 init --port 6789`,
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := localserver.New(port)
			if err != nil {
				return fmt.Errorf("creating local server: %w", err)
			}

			if err := srv.Start(); err != nil {
				return fmt.Errorf("starting local server: %w", err)
			}

			cli.renderer.Infof("Starting local identity server on %s", ansi.Bold(srv.Addr()))
			cli.renderer.Newline()
			cli.renderer.Infof("No Auth0 account needed. Point your SDK at this domain:")
			cli.renderer.Infof("  %s", ansi.Cyan(srv.Addr()))
			cli.renderer.Newline()
			cli.renderer.Infof("Endpoints:")
			cli.renderer.Infof("  %-55s OIDC discovery", ansi.Faint(srv.Addr()+"/.well-known/openid-configuration"))
			cli.renderer.Infof("  %-55s Authorization (auto-approves every login)", ansi.Faint(srv.Addr()+"/authorize"))
			cli.renderer.Infof("  %-55s Token exchange", ansi.Faint(srv.Addr()+"/oauth/token"))
			cli.renderer.Infof("  %-55s User info", ansi.Faint(srv.Addr()+"/userinfo"))
			cli.renderer.Infof("  %-55s Last captured OTP code (agent-readable)", ansi.Faint(srv.Addr()+"/__test/last-code"))
			cli.renderer.Newline()
			cli.renderer.Warnf("Tokens issued here are intentionally invalid on any real Auth0 tenant.")
			cli.renderer.Newline()
			cli.renderer.Infof("Press Ctrl+C to stop.")

			quit := make(chan os.Signal, 1)
			signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
			<-quit

			cli.renderer.Newline()
			cli.renderer.Infof("Shutting down local identity server...")

			stopCtx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer cancel()
			return srv.Stop(stopCtx)
		},
	}

	cmd.Flags().IntVar(&port, "port", 6789, "Port for the local identity server.")
	return cmd
}
