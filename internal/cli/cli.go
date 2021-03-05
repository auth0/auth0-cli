package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/lestrrat-go/jwx/jwt"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/auth0.v5/management"
)

const (
	userAgent               = "Auth0 CLI"
	accessTokenExpThreshold = 5 * time.Minute
)

// config defines the exact set of tenants, access tokens, which only exists
// for a particular user's machine.
type config struct {
	DefaultTenant string            `json:"default_tenant"`
	Tenants       map[string]tenant `json:"tenants"`
}

// tenant is the cli's concept of an auth0 tenant. The fields are tailor fit
// specifically for interacting with the management API.
type tenant struct {
	Name        string    `json:"name"`
	Domain      string    `json:"domain"`
	AccessToken string    `json:"access_token,omitempty"`
	ExpiresAt   time.Time `json:"expires_at"`
}

var errUnauthenticated = errors.New("Not yet configured. Try `auth0 login`.")

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
	api      *auth0.API
	renderer *display.Renderer

	// set of flags which are user specified.
	debug   bool
	tenant  string
	format  string
	force   bool
	noInput bool

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

	t, err := c.getTenant()
	if err != nil {
		return err
	}

	if t.AccessToken == "" {
		return errUnauthenticated

	}

	// check if the stored access token is expired:
	if isExpired(t.ExpiresAt, accessTokenExpThreshold) {
		// ask and guide the user through the login process:
		err := RunLogin(ctx, c, true)
		if err != nil {
			return err
		}
	}

	// continue with the command setup:
	if t.AccessToken != "" {
		m, err := management.New(t.Domain,
			management.WithStaticToken(t.AccessToken),
			management.WithDebug(c.debug),
			management.WithUserAgent(userAgent))
		if err != nil {
			return err
		}

		c.api = auth0.NewAPI(m)
	}

	return err
}

// isExpired is true if now() + a threshold is after the given date
func isExpired(t time.Time, threshold time.Duration) bool {
	return time.Now().Add(threshold).After(t)
}

// getTenant fetches the default tenant configured (or the tenant specified via
// the --tenant flag).
func (c *cli) getTenant() (tenant, error) {
	if err := c.init(); err != nil {
		return tenant{}, err
	}

	t, ok := c.config.Tenants[c.tenant]
	if !ok {
		return tenant{}, fmt.Errorf("Unable to find tenant: %s", c.tenant)
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
		c.config.DefaultTenant = ten.Name
	}

	// If we're dealing with an empty file, we'll need to initialize this
	// map.
	if c.config.Tenants == nil {
		c.config.Tenants = map[string]tenant{}
	}

	c.config.Tenants[ten.Name] = ten

	if err := c.persistConfig(); err != nil {
		return fmt.Errorf("persisting config: %w", err)
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
		c.path = path.Join(os.Getenv("HOME"), ".config", "auth0", "config.json")
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

func mustRequireFlags(cmd *cobra.Command, flags ...string) {
	for _, f := range flags {
		if err := cmd.MarkFlagRequired(f); err != nil {
			panic(err)
		}
	}
}

func canPrompt(cmd *cobra.Command) bool {
	noInput, err := cmd.Root().Flags().GetBool("no-input")

	if err != nil {
		return false
	}

	return ansi.IsTerminal() && !noInput
}

func shouldPrompt(cmd *cobra.Command, flag string) bool {
	return canPrompt(cmd) && !cmd.Flags().Changed(flag)
}

func shouldPromptWhenFlagless(cmd *cobra.Command, flag string) bool {
	isSet := false

	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if (f.Changed) {
			isSet = true
		}
	})

	return canPrompt(cmd) && !isSet
}

func prepareInteractivity(cmd *cobra.Command) {
	if canPrompt(cmd) {
		cmd.Flags().VisitAll(func(flag *pflag.Flag) {
			_ = cmd.Flags().SetAnnotation(flag.Name, cobra.BashCompOneRequiredFlag, []string{"false"})
		})
	}
}
