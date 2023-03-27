package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/auth0/go-auth0/management"
	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/jwt"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/auth0/auth0-cli/internal/analytics"
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/buildinfo"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/iostream"
	"github.com/auth0/auth0-cli/internal/keyring"
)

const (
	userAgent               = "Auth0 CLI"
	accessTokenExpThreshold = 5 * time.Minute
)

// config defines the exact set of tenants, access tokens, which only exists
// for a particular user's machine.
type config struct {
	InstallID     string            `json:"install_id,omitempty"`
	DefaultTenant string            `json:"default_tenant"`
	Tenants       map[string]Tenant `json:"tenants"`
}

// Tenant is the cli's concept of an auth0 tenant.
// The fields are tailor fit specifically for
// interacting with the management API.
type Tenant struct {
	Name         string         `json:"name"`
	Domain       string         `json:"domain"`
	AccessToken  string         `json:"access_token,omitempty"`
	Scopes       []string       `json:"scopes,omitempty"`
	ExpiresAt    time.Time      `json:"expires_at"`
	Apps         map[string]app `json:"apps,omitempty"`
	DefaultAppID string         `json:"default_app_id,omitempty"`
	ClientID     string         `json:"client_id"`
}

type app struct {
	FirstRuns map[string]bool `json:"first_runs"`
}

var errUnauthenticated = errors.New("Not logged in. Try 'auth0 login'.")

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

	// Config state management.
	initOnce sync.Once
	errOnce  error
	path     string
	config   config
}

func (t *Tenant) authenticatedWithClientCredentials() bool {
	return t.ClientID != ""
}

func (t *Tenant) authenticatedWithDeviceCodeFlow() bool {
	return t.ClientID == ""
}

func (t *Tenant) hasExpiredToken() bool {
	return time.Now().Add(accessTokenExpThreshold).After(t.ExpiresAt)
}

func (t *Tenant) additionalRequestedScopes() []string {
	additionallyRequestedScopes := make([]string, 0)

	for _, scope := range t.Scopes {
		found := false

		for _, defaultScope := range auth.RequiredScopes {
			if scope == defaultScope {
				found = true
				break
			}
		}

		if !found {
			additionallyRequestedScopes = append(additionallyRequestedScopes, scope)
		}
	}

	return additionallyRequestedScopes
}

func (t *Tenant) regenerateAccessToken(ctx context.Context) error {
	if t.authenticatedWithClientCredentials() {
		clientSecret, err := keyring.GetClientSecret(t.Domain)
		if err != nil {
			return fmt.Errorf("failed to retrieve client secret from keyring: %w", err)
		}

		token, err := auth.GetAccessTokenFromClientCreds(
			ctx,
			auth.ClientCredentials{
				ClientID:     t.ClientID,
				ClientSecret: clientSecret,
				Domain:       t.Domain,
			},
		)
		if err != nil {
			return err
		}

		t.AccessToken = token.AccessToken
		t.ExpiresAt = token.ExpiresAt
	}

	if t.authenticatedWithDeviceCodeFlow() {
		tokenResponse, err := auth.RefreshAccessToken(http.DefaultClient, t.Domain)
		if err != nil {
			return err
		}

		t.AccessToken = tokenResponse.AccessToken
		t.ExpiresAt = time.Now().Add(
			time.Duration(tokenResponse.ExpiresIn) * time.Second,
		)
	}

	err := keyring.StoreAccessToken(t.Domain, t.AccessToken)
	if err != nil {
		t.AccessToken = ""
	}

	return nil
}

// isLoggedIn encodes the domain logic for determining whether or not we're
// logged in. This might check our config storage, or just in memory.
func (c *cli) isLoggedIn() bool {
	// No need to check errors for initializing context.
	_ = c.init()

	if c.tenant == "" {
		return false
	}

	// Parse the access token for the tenant.
	t, err := jwt.ParseString(c.config.Tenants[c.tenant].AccessToken)
	if err != nil {
		return false
	}

	// Check if token is valid.
	if err = jwt.Validate(t, jwt.WithIssuer("https://auth0.auth0.com/")); err != nil {
		return false
	}

	return true
}

// setup will try to initialize the config context, as well as figure out if
// there's a readily available tenant. A management API SDK instance is initialized IFF:
//
// 1. A tenant is found.
// 2. The tenant has an access token.
func (c *cli) setup(ctx context.Context) error {
	if err := c.init(); err != nil {
		return err
	}

	t, err := c.prepareTenant(ctx)
	if err != nil {
		return err
	}

	userAgent := fmt.Sprintf("%v/%v", userAgent, strings.TrimPrefix(buildinfo.Version, "v"))

	api, err := management.New(
		t.Domain,
		management.WithStaticToken(getAccessToken(t)),
		management.WithUserAgent(userAgent),
	)
	if err != nil {
		return err
	}

	c.api = auth0.NewAPI(api)
	return nil
}

func getAccessToken(t Tenant) string {
	accessToken, err := keyring.GetAccessToken(t.Domain)
	if err == nil && accessToken != "" {
		return accessToken
	}

	return t.AccessToken
}

// prepareTenant loads the tenant, refreshing its token if necessary.
// The tenant access token needs a refresh if:
// 1. The tenant scopes are different than the currently required scopes.
// 2. The access token is expired.
func (c *cli) prepareTenant(ctx context.Context) (Tenant, error) {
	t, err := c.getTenant()
	if err != nil {
		return Tenant{}, err
	}

	if !hasAllRequiredScopes(t) && t.authenticatedWithDeviceCodeFlow() {
		c.renderer.Warnf("Required scopes have changed. Please log in to re-authorize the CLI.\n")
		return RunLoginAsUser(ctx, c, t.additionalRequestedScopes())
	}

	accessToken := getAccessToken(t)
	if accessToken != "" && !t.hasExpiredToken() {
		return t, nil
	}

	if err := t.regenerateAccessToken(ctx); err != nil {
		if t.authenticatedWithClientCredentials() {
			errorMessage := fmt.Errorf(
				"failed to fetch access token using client credentials: %w\n\n"+
					"This may occur if the designated Auth0 application has been deleted, "+
					"the client secret has been rotated or previous failure to store client secret in the keyring.\n\n"+
					"Please re-authenticate by running: %s",
				err,
				ansi.Bold("auth0 login --domain <tenant-domain --client-id <client-id> --client-secret <client-secret>"),
			)

			return t, errorMessage
		}

		c.renderer.Warnf("Failed to renew access token: %s", err)
		c.renderer.Warnf("Please log in to re-authorize the CLI.\n")

		return RunLoginAsUser(ctx, c, t.additionalRequestedScopes())
	}

	if err := c.addTenant(t); err != nil {
		return Tenant{}, fmt.Errorf("unexpected error adding tenant to config: %w", err)
	}

	return t, nil
}

// hasAllRequiredScopes compare the tenant scopes
// with the currently required scopes.
func hasAllRequiredScopes(t Tenant) bool {
	for _, requiredScope := range auth.RequiredScopes {
		if !containsStr(t.Scopes, requiredScope) {
			return false
		}
	}

	return true
}

// getTenant fetches the default tenant configured (or the tenant specified via
// the --tenant flag).
func (c *cli) getTenant() (Tenant, error) {
	if err := c.init(); err != nil {
		return Tenant{}, err
	}

	t, ok := c.config.Tenants[c.tenant]
	if !ok {
		return Tenant{}, fmt.Errorf("Unable to find tenant: %s; run 'auth0 tenants use' to see your configured tenants or run 'auth0 login' to configure a new tenant", c.tenant)
	}

	if t.Apps == nil {
		t.Apps = map[string]app{}
	}

	return t, nil
}

// listTenants fetches all the configured tenants.
func (c *cli) listTenants() ([]Tenant, error) {
	if err := c.init(); err != nil {
		return []Tenant{}, err
	}

	tenants := make([]Tenant, 0, len(c.config.Tenants))
	for _, t := range c.config.Tenants {
		tenants = append(tenants, t)
	}

	return tenants, nil
}

// addTenant assigns an existing, or new tenant. This is expected to be called
// after a login has completed.
func (c *cli) addTenant(ten Tenant) error {
	// init will fail here with a `no tenant found` error if we're logging
	// in for the first time and that's expected.
	_ = c.init()

	// If there's no existing DefaultTenant yet, might as well set the
	// first successfully logged in tenant during onboarding.
	if c.config.DefaultTenant == "" {
		c.config.DefaultTenant = ten.Domain
	}

	// If we're dealing with an empty file, we'll need to initialize this
	// map.
	if c.config.Tenants == nil {
		c.config.Tenants = map[string]Tenant{}
	}

	c.config.Tenants[ten.Domain] = ten

	if err := c.persistConfig(); err != nil {
		return fmt.Errorf("unexpected error persisting config: %w", err)
	}

	return nil
}

func (c *cli) removeTenant(ten string) error {
	// init will fail here with a `no tenant found` error if we're logging
	// in for the first time and that's expected.
	_ = c.init()

	// If we're dealing with an empty file, we'll need to initialize this
	// map.
	if c.config.Tenants == nil {
		c.config.Tenants = map[string]Tenant{}
	}

	delete(c.config.Tenants, ten)

	// If the default tenant is being removed, we'll pick the first tenant
	// that's not the one being removed, and make that the new default.
	if c.config.DefaultTenant == ten {
		if len(c.config.Tenants) == 0 {
			c.config.DefaultTenant = ""
		} else {
		Loop:
			for t := range c.config.Tenants {
				if t != ten {
					c.config.DefaultTenant = t
					break Loop
				}
			}
		}
	}

	if err := c.persistConfig(); err != nil {
		return fmt.Errorf("failed to persist config: %w", err)
	}

	if err := keyring.DeleteSecretsForTenant(ten); err != nil {
		return fmt.Errorf("failed to delete tenant secrets: %w", err)
	}

	return nil
}

func (c *cli) isFirstCommandRun(clientID string, command string) (bool, error) {
	tenant, err := c.getTenant()

	if err != nil {
		return false, err
	}

	if a, found := tenant.Apps[clientID]; found {
		if a.FirstRuns[command] {
			return false, nil
		}
	}

	return true, nil
}

func (c *cli) setDefaultAppID(id string) error {
	tenant, err := c.getTenant()
	if err != nil {
		return err
	}

	tenant.DefaultAppID = id

	c.config.Tenants[tenant.Domain] = tenant
	if err := c.persistConfig(); err != nil {
		return fmt.Errorf("Unexpected error persisting config: %w", err)
	}

	return nil
}

func (c *cli) setFirstCommandRun(clientID string, command string) error {
	tenant, err := c.getTenant()
	if err != nil {
		return err
	}

	if a, found := tenant.Apps[clientID]; found {
		if a.FirstRuns == nil {
			a.FirstRuns = map[string]bool{}
		}
		a.FirstRuns[command] = true
		tenant.Apps[clientID] = a
	} else {
		tenant.Apps[clientID] = app{
			FirstRuns: map[string]bool{
				command: true,
			},
		}
	}

	c.config.Tenants[tenant.Domain] = tenant

	return nil
}

func checkInstallID(c *cli) error {
	if c.config.InstallID == "" {
		c.config.InstallID = uuid.NewString()

		if err := c.persistConfig(); err != nil {
			return fmt.Errorf("unexpected error persisting config: %w", err)
		}

		c.tracker.TrackFirstLogin(c.config.InstallID)
	}

	return nil
}

func (c *cli) persistConfig() error {
	dir := filepath.Dir(c.path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}

	buf, err := json.MarshalIndent(c.config, "", "    ")
	if err != nil {
		return err
	}

	err = os.WriteFile(c.path, buf, 0600)

	return err
}

func (c *cli) init() error {
	c.initOnce.Do(func() {
		if c.errOnce = c.initContext(); c.errOnce != nil {
			return
		}

		c.renderer.Tenant = c.tenant

		cobra.EnableCommandSorting = false
	})

	if c.json {
		c.renderer.Format = display.OutputFormatJSON
	}

	c.renderer.Tenant = c.tenant

	// Once initialized, we'll keep returning the
	// same err that was originally encountered.
	return c.errOnce
}

func (c *cli) initContext() (err error) {
	if c.path == "" {
		c.path = defaultConfigPath()
	}

	if _, err := os.Stat(c.path); os.IsNotExist(err) {
		return errUnauthenticated
	}

	var buf []byte
	if buf, err = os.ReadFile(c.path); err != nil {
		return err
	}

	if err := json.Unmarshal(buf, &c.config); err != nil {
		return err
	}

	if c.tenant == "" && c.config.DefaultTenant == "" {
		return errUnauthenticated
	}

	if c.tenant == "" {
		c.tenant = c.config.DefaultTenant
	}

	return nil
}

func defaultConfigPath() string {
	return path.Join(os.Getenv("HOME"), ".config", "auth0", "config.json")
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
