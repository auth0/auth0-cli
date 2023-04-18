package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/auth0/auth0-cli/internal/analytics"
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/buildinfo"
	"github.com/auth0/auth0-cli/internal/config"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/iostream"
)

const userAgent = "Auth0 CLI"

// cli provides all the foundational things for all the commands in the CLI,
// specifically:
//
// 1. A management API instance (e.g. go-auth0/auth0)
// 2. A renderer (which provides ansi, coloring, etc).
//
// In addition, it stores a reference to all the flags passed, e.g.:
//
// 1. --json
// 2. --tenant
// 3. --debug.
type cli struct {
	// Core primitives exposed to command builders.
	api      *auth0.API
	renderer *display.Renderer
	tracker  *analytics.Tracker

	// Set of flags which are user specified.
	debug   bool
	tenant  string
	json    bool
	force   bool
	noInput bool
	noColor bool

	Config config.Config
}

// setupWithAuthentication will fetch the tenant from the config.json
// and regenerate its access token if needed. The access token will
// then be used to configure an instance of the Auth0 Management SDK.
func (c *cli) setupWithAuthentication(ctx context.Context) error {
	// Validate that we have at least one tenant that we can use.
	if err := c.Config.Validate(); err != nil {
		return err
	}

	// If we didn't pass any tenant through the
	// flags we're going to use the default one.
	if c.tenant == "" {
		c.tenant = c.Config.DefaultTenant
	}

	// Get the tenant from the config.
	tenant, err := c.Config.GetTenant(c.tenant)
	if err != nil {
		return err
	}

	// Check authentication status.
	err = tenant.CheckAuthenticationStatus()
	switch err {
	case config.ErrTokenMissingRequiredScopes:
		c.renderer.Warnf("Required scopes have changed. Please log in to re-authorize the CLI.\n")
		tenant, err = RunLoginAsUser(ctx, c, tenant.GetExtraRequestedScopes())
		if err != nil {
			return err
		}
	case config.ErrInvalidToken:
		if err := tenant.RegenerateAccessToken(ctx); err != nil {
			if tenant.IsAuthenticatedWithClientCredentials() {
				errorMessage := fmt.Errorf(
					"failed to fetch access token using client credentials: %w\n\n"+
						"This may occur if the designated Auth0 application has been deleted, "+
						"the client secret has been rotated or previous failure to store client "+
						"secret in the keyring.\n\n"+
						"Please re-authenticate by running: %s",
					err,
					ansi.Bold("auth0 login --domain <tenant-domain --client-id <client-id> --client-secret <client-secret>"),
				)
				return errorMessage
			}

			c.renderer.Warnf("Failed to renew access token: %s", err)
			c.renderer.Warnf("Please log in to re-authorize the CLI.\n")

			tenant, err = RunLoginAsUser(ctx, c, tenant.GetExtraRequestedScopes())
			if err != nil {
				return err
			}
		}

		if err := c.Config.AddTenant(tenant); err != nil {
			return err
		}
	}

	userAgent := fmt.Sprintf("%v/%v", userAgent, strings.TrimPrefix(buildinfo.Version, "v"))

	api, err := management.New(
		tenant.Domain,
		management.WithStaticToken(tenant.GetAccessToken()),
		management.WithUserAgent(userAgent),
	)
	if err != nil {
		return err
	}

	c.api = auth0.NewAPI(api)
	return nil
}

func (c *cli) configureRenderer() {
	c.renderer.Tenant = c.tenant

	if c.json {
		c.renderer.Format = display.OutputFormatJSON
	}
}

func canPrompt(cmd *cobra.Command) bool {
	noInput, err := cmd.Root().Flags().GetBool("no-input")
	if err != nil {
		return false
	}

	return iostream.IsInputTerminal() && iostream.IsOutputTerminal() && !noInput
}

func shouldPrompt(cmd *cobra.Command, flag *Flag) bool {
	return canPrompt(cmd) && !flag.IsSet(cmd)
}

func shouldPromptWhenNoLocalFlagsSet(cmd *cobra.Command) bool {
	localFlagIsSet := false
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name != "json" && f.Name != "force" && f.Changed {
			localFlagIsSet = true
		}
	})

	return canPrompt(cmd) && !localFlagIsSet
}

func prepareInteractivity(cmd *cobra.Command) {
	if canPrompt(cmd) || !iostream.IsInputTerminal() {
		cmd.Flags().VisitAll(func(flag *pflag.Flag) {
			_ = cmd.Flags().SetAnnotation(flag.Name, cobra.BashCompOneRequiredFlag, []string{"false"})
		})
	}
}
