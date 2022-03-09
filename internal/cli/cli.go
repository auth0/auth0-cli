package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/auth0/go-auth0/management"
	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/jwt"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/auth0/auth0-cli/internal/analytics"
	"github.com/auth0/auth0-cli/internal/auth"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/buildinfo"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/iostream"
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
	Tenants       map[string]tenant `json:"tenants"`
}

// tenant is the cli's concept of an auth0 tenant. The fields are tailor fit
// specifically for interacting with the management API.
type tenant struct {
	Name         string         `json:"name"`
	Domain       string         `json:"domain"`
	AccessToken  string         `json:"access_token,omitempty"`
	Scopes       []string       `json:"scopes,omitempty"`
	ExpiresAt    time.Time      `json:"expires_at"`
	Apps         map[string]app `json:"apps,omitempty"`
	DefaultAppID string         `json:"default_app_id,omitempty"`

	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
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
// 1. --format
// 2. --tenant
// 3. --debug
//
type cli struct {
	// core primitives exposed to command builders.
	api           *auth0.API
	authenticator *auth.Authenticator
	renderer      *display.Renderer
	tracker       *analytics.Tracker
	// set of flags which are user specified.
	debug   bool
	tenant  string
	format  string
	force   bool
	noInput bool
	noColor bool

	// config state management.
	initOnce sync.Once
	errOnce  error
	path     string
	config   config
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

	var (
		m  *management.Management
		ua = fmt.Sprintf("%v/%v", userAgent, strings.TrimPrefix(buildinfo.Version, "v"))
	)

	if t.ClientID != "" && t.ClientSecret != "" {
		m, err = management.New(t.Domain,
			management.WithClientCredentials(t.ClientID, t.ClientSecret),
			management.WithUserAgent(ua),
		)
	} else {
		m, err = management.New(t.Domain,
			management.WithStaticToken(t.AccessToken),
			management.WithUserAgent(ua),
		)
	}

	if err != nil {
		return err
	}

	c.api = auth0.NewAPI(m)
	return nil
}

// prepareTenant loads the tenant, refreshing its token if necessary.
// The tenant access token needs a refresh if:
// 1. the tenant scopes are different than the currently required scopes.
// 2. the access token is expired.
func (c *cli) prepareTenant(ctx context.Context) (tenant, error) {
	t, err := c.getTenant()
	if err != nil {
		return tenant{}, err
	}

	if t.ClientID != "" && t.ClientSecret != "" {
		return t, nil
	}

	if t.AccessToken == "" || scopesChanged(t) {
		t, err = RunLogin(ctx, c, true)
		if err != nil {
			return tenant{}, err
		}
	} else if isExpired(t.ExpiresAt, accessTokenExpThreshold) {
		// check if the stored access token is expired:
		// use the refresh token to get a new access token:
		tr := &auth.TokenRetriever{
			Authenticator: c.authenticator,
			Secrets:       &auth.Keyring{},
			Client:        http.DefaultClient,
		}

		// NOTE(cyx): this code will have to be adapted to instead
		// maybe take the clientID/secret as additional params, or
		// something similar.
		res, err := tr.Refresh(ctx, t.Domain)
		if err != nil {
			// ask and guide the user through the login process:
			c.renderer.Errorf("failed to renew access token, %s", err)
			t, err = RunLogin(ctx, c, true)
			if err != nil {
				return tenant{}, err
			}
		} else {
			// persist the updated tenant with renewed access token
			t.AccessToken = res.AccessToken
			t.ExpiresAt = time.Now().Add(
				time.Duration(res.ExpiresIn) * time.Second,
			)

			err = c.addTenant(t)
			if err != nil {
				return tenant{}, err
			}
		}
	}

	return t, nil
}

// isExpired is true if now() + a threshold is after the given date
func isExpired(t time.Time, threshold time.Duration) bool {
	return time.Now().Add(threshold).After(t)
}

// scopesChanged compare the tenant scopes
// with the currently required scopes.
func scopesChanged(t tenant) bool {
	want := auth.RequiredScopes()
	got := t.Scopes

	sort.Strings(want)
	sort.Strings(got)

	if (want == nil) != (got == nil) {
		return true
	}

	if len(want) != len(got) {
		return true
	}

	for i := range t.Scopes {
		if want[i] != got[i] {
			return true
		}
	}

	return false
}

// getTenant fetches the default tenant configured (or the tenant specified via
// the --tenant flag).
func (c *cli) getTenant() (tenant, error) {
	if err := c.init(); err != nil {
		return tenant{}, err
	}

	t, ok := c.config.Tenants[c.tenant]
	if !ok {
		return tenant{}, fmt.Errorf("Unable to find tenant: %s; run 'auth0 tenants use' to see your configured tenants or run 'auth0 login' to configure a new tenant", c.tenant)
	}

	if t.Apps == nil {
		t.Apps = map[string]app{}
	}

	return t, nil
}

// listTenants fetches all of the configured tenants
func (c *cli) listTenants() ([]tenant, error) {
	if err := c.init(); err != nil {
		return []tenant{}, err
	}

	tenants := make([]tenant, 0, len(c.config.Tenants))
	for _, t := range c.config.Tenants {
		tenants = append(tenants, t)
	}

	return tenants, nil
}

// addTenant assigns an existing, or new tenant. This is expected to be called
// after a login has completed.
func (c *cli) addTenant(ten tenant) error {
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
		c.config.Tenants = map[string]tenant{}
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
		c.config.Tenants = map[string]tenant{}
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
		return fmt.Errorf("Unexpected error persisting config: %w", err)
	}

	tr := &auth.TokenRetriever{Secrets: &auth.Keyring{}}
	if err := tr.Delete(ten); err != nil {
		return fmt.Errorf("Unexpected error clearing tenant information: %w", err)
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

	if err := ioutil.WriteFile(c.path, buf, 0600); err != nil {
		return err
	}
	return nil
}

func (c *cli) init() error {
	c.initOnce.Do(func() {
		// Initialize the context -- e.g. the configuration
		// information, tenants, etc.
		if c.errOnce = c.initContext(); c.errOnce != nil {
			return
		}
		c.renderer.Tenant = c.tenant

		cobra.EnableCommandSorting = false
	})

	// Determine what the desired output format is.
	//
	// NOTE(cyx): Since this isn't expensive to do, we don't need to put it
	// inside initOnce.
	format := strings.ToLower(c.format)
	if format != "" && format != string(display.OutputFormatJSON) {
		return fmt.Errorf("Invalid format. Use `--format=json` or omit this option to use the default format.")
	}
	c.renderer.Format = display.OutputFormat(format)

	c.renderer.Tenant = c.tenant

	// Once initialized, we'll keep returning the same err that was
	// originally encountered.
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
	if buf, err = ioutil.ReadFile(c.path); err != nil {
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

func (c *cli) setPath(p string) {
	if p == "" {
		c.path = defaultConfigPath()
		return
	}
	c.path = p
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

func shouldPromptWhenFlagless(cmd *cobra.Command, flag string) bool {
	isSet := false

	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Changed {
			isSet = true
		}
	})

	return canPrompt(cmd) && !isSet
}

func prepareInteractivity(cmd *cobra.Command) {
	if canPrompt(cmd) || !iostream.IsInputTerminal() {
		cmd.Flags().VisitAll(func(flag *pflag.Flag) {
			_ = cmd.Flags().SetAnnotation(flag.Name, cobra.BashCompOneRequiredFlag, []string{"false"})
		})
	}
}
