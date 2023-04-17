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

// setup will try to initialize the config context, as well as figure out if
// there's a readily available tenant. A management API SDK instance is initialized IFF:
//
// 1. A tenant is found.
// 2. The tenant has an access token.
func (c *cli) setup(ctx context.Context) error {
	cobra.EnableCommandSorting = false

	if err := c.Config.VerifyAuthentication(); err != nil {
		return err
	}

	if c.tenant == "" {
		c.tenant = c.Config.DefaultTenant
	}

	c.configureRenderer()

	tenant, err := c.ensureTenantAccessTokenIsUpdated(ctx)
	if err != nil {
		return err
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

// ensureTenantAccessTokenIsUpdated loads the tenant, refreshing its token if necessary.
// The tenant access token needs a refresh if:
// 1. The tenant scopes are different than the currently required scopes.
// 2. The access token is expired.
func (c *cli) ensureTenantAccessTokenIsUpdated(ctx context.Context) (config.Tenant, error) {
	t, err := c.Config.GetTenant(c.tenant)
	if err != nil {
		return config.Tenant{}, err
	}

	if !t.HasAllRequiredScopes() && t.IsAuthenticatedWithDeviceCodeFlow() {
		c.renderer.Warnf("Required scopes have changed. Please log in to re-authorize the CLI.\n")
		return RunLoginAsUser(ctx, c, t.GetExtraRequestedScopes())
	}

	accessToken := t.GetAccessToken()
	if accessToken != "" && !t.HasExpiredToken() {
		return t, nil
	}

	if err := t.RegenerateAccessToken(ctx); err != nil {
		if t.IsAuthenticatedWithClientCredentials() {
			errorMessage := fmt.Errorf(
				"failed to fetch access token using client credentials: %w\n\nThis may occur if the designated Auth0 application has been deleted, the client secret has been rotated or previous failure to store client secret in the keyring.\n\nPlease re-authenticate by running: %s",
				err,
				ansi.Bold("auth0 login --domain <tenant-domain --client-id <client-id> --client-secret <client-secret>"),
			)

			return t, errorMessage
		}

		c.renderer.Warnf("Failed to renew access token: %s", err)
		c.renderer.Warnf("Please log in to re-authorize the CLI.\n")

		return RunLoginAsUser(ctx, c, t.GetExtraRequestedScopes())
	}

	if err := c.Config.AddTenant(t); err != nil {
		return config.Tenant{}, fmt.Errorf("unexpected error adding tenant to config: %w", err)
	}

	return t, nil
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
