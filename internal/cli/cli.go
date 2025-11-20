package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/auth0/auth0-cli/internal/analytics"
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/config"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/iostream"
	"github.com/auth0/auth0-cli/internal/prompt"
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
// 2. --csv
// 3. --tenant
// 4. --debug.
type cli struct {
	// Core primitives exposed to command builders.
	api      *auth0.API
	renderer *display.Renderer
	tracker  *analytics.Tracker

	// Set of flags which are user specified.
	debug       bool
	tenant      string
	json        bool
	jsonCompact bool
	csv         bool
	force       bool
	noInput     bool
	noColor     bool

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
	var scopesErr config.ErrTokenMissingRequiredScopes
	if errors.As(err, &scopesErr) {
		c.renderer.Warnf("Required scopes have changed (missing: %s). Please log in to re-authorize the CLI.\n", strings.Join(scopesErr.MissingScopes, ", "))
		tenant, err = RunLoginAsUser(ctx, c, scopesErr.MissingScopes, "")
		if err != nil {
			return err
		}
	}

	if errors.Is(err, config.ErrInvalidToken) {
		if err := tenant.RegenerateAccessToken(ctx); err != nil {
			if tenant.IsAuthenticatedWithClientCredentials() {
				errorMessage := fmt.Errorf(
					"failed to fetch access token using client credentials: %w\n\n"+
						"This may occur if the designated Auth0 application has been deleted, "+
						"the client secret has been rotated or previous failure to store client "+
						"secret in the keyring.\n\n"+
						"Please re-authenticate by running: %s",
					err,
					ansi.Bold("auth0 login --domain <tenant-domain> --client-id <client-id> --client-secret <client-secret>"),
				)
				return errorMessage
			}

			c.renderer.Warnf("Failed to renew access token: %s", err)
			c.renderer.Warnf("Please log in to re-authorize the CLI.\n")

			// Determine tenant domain for login.
			tenantDomain := ""
			if c.Config.DefaultTenant != "" {
				if prompt.Confirm(fmt.Sprintf("Continue login with default tenant '%s'?", c.Config.DefaultTenant)) {
					tenantDomain = c.Config.DefaultTenant
				}
			}

			tenant, err = RunLoginAsUser(ctx, c, tenant.GetExtraRequestedScopes(), tenantDomain)
			if err != nil {
				return err
			}
		}

		if err := c.Config.AddTenant(tenant); err != nil {
			return err
		}
	}

	if errors.Is(err, config.ErrMalformedToken) {
		return fmt.Errorf("authentication token is corrupted, please run: %s\n\n%s",
			ansi.Cyan("auth0 logout && auth0 login"),
			ansi.Yellow("Note: Token handling was enhanced in v1.18.0+ to prevent malformed tokens."),
		)
	}

	api, err := initializeManagementClient(tenant.Domain, tenant.GetAccessToken())
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

	if c.jsonCompact {
		c.renderer.Format = display.OutputFormatJSONCompact
	}

	if c.csv {
		c.renderer.Format = display.OutputFormatCSV
	}
}

func canPrompt(cmd *cobra.Command) bool {
	noInput, err := cmd.Root().Flags().GetBool("no-input")
	if err != nil {
		return false
	}

	return iostream.IsInputTerminal() && iostream.IsOutputTerminal() && !noInput
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
