package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/jwt"
)

// ErrConfigFileMissing is thrown when the config.json file is missing.
var ErrConfigFileMissing = errors.New("config.json file is missing")

// ErrNoAuthenticatedTenants is thrown when the config file has no authenticated tenants.
var ErrNoAuthenticatedTenants = errors.New("not logged in. Try `auth0 login`")

// Config holds cli configuration settings.
type Config struct {
	onlyOnce  sync.Once
	initError error

	path string

	InstallID     string  `json:"install_id,omitempty"`
	DefaultTenant string  `json:"default_tenant"`
	Tenants       Tenants `json:"tenants"`
}

// Initialize will load the config settings into memory.
func (c *Config) Initialize() error {
	c.onlyOnce.Do(func() {
		c.initError = c.loadFromDisk()
	})

	return c.initError
}

// Validate checks to see if the config is not corrupted,
// and we have an authenticated tenant saved.
// If we have at least one tenant saved but the DefaultTenant
// is empty, it will attempt to set the first available
// tenant as the DefaultTenant and save to disk.
func (c *Config) Validate() error {
	if err := c.Initialize(); err != nil {
		return err
	}

	if len(c.Tenants) == 0 {
		return ErrNoAuthenticatedTenants
	}

	if c.DefaultTenant != "" {
		return nil
	}

	for tenant := range c.Tenants {
		c.DefaultTenant = tenant
		break // Pick first tenant and exit.
	}

	return c.saveToDisk()
}

// IsLoggedInWithTenant checks if we're logged in with the given tenant.
func (c *Config) IsLoggedInWithTenant(tenantName string) bool {
	// Ignore error as we could
	// be not logged in yet.
	_ = c.Initialize()

	if tenantName == "" {
		tenantName = c.DefaultTenant
	}

	tenant, ok := c.Tenants[tenantName]
	if !ok {
		return false
	}

	token, err := jwt.ParseString(tenant.GetAccessToken())
	if err != nil {
		return false
	}

	if err = jwt.Validate(token, jwt.WithIssuer("https://auth0.auth0.com/")); err != nil {
		return false
	}

	return true
}

// GetTenant retrieves all the tenant information from the config.
func (c *Config) GetTenant(tenantName string) (Tenant, error) {
	if err := c.Initialize(); err != nil {
		return Tenant{}, err
	}

	tenant, ok := c.Tenants[tenantName]
	if !ok {
		return Tenant{}, fmt.Errorf(
			"failed to find tenant: %s. Run 'auth0 tenants list' to see your configured tenants "+
				"or run 'auth0 login' to configure a new tenant",
			tenantName,
		)
	}

	return tenant, nil
}

// AddTenant adds a tenant to the config.
// This is called after a login has completed.
func (c *Config) AddTenant(tenant Tenant) error {
	// Ignore error as we could be
	// logging in the first time.
	_ = c.Initialize()

	c.ensureInstallIDAssigned()

	if c.DefaultTenant == "" {
		c.DefaultTenant = tenant.Domain
	}

	if c.Tenants == nil {
		c.Tenants = make(map[string]Tenant)
	}

	c.Tenants[tenant.Domain] = tenant

	return c.saveToDisk()
}

// RemoveTenant removes a tenant from the config.
// This is called after a logout has completed.
func (c *Config) RemoveTenant(tenant string) error {
	if err := c.Initialize(); err != nil {
		if errors.Is(err, ErrConfigFileMissing) {
			return nil // Config file is missing, so nothing to remove.
		}
		return err
	}

	if c.DefaultTenant == "" && len(c.Tenants) == 0 {
		return nil // Nothing to remove.
	}

	if c.DefaultTenant != "" && len(c.Tenants) == 0 {
		c.DefaultTenant = "" // Reset possible corruption of config file.
		return c.saveToDisk()
	}

	delete(c.Tenants, tenant)

	if c.DefaultTenant == tenant {
		c.DefaultTenant = ""

		for otherTenant := range c.Tenants {
			c.DefaultTenant = otherTenant
			break // Pick first tenant and exit as we called delete above.
		}
	}

	return c.saveToDisk()
}

// ListAllTenants retrieves a list with all configured tenants.
func (c *Config) ListAllTenants() ([]Tenant, error) {
	if err := c.Initialize(); err != nil {
		return nil, err
	}

	tenants := make([]Tenant, 0, len(c.Tenants))
	for _, tenant := range c.Tenants {
		tenants = append(tenants, tenant)
	}

	return tenants, nil
}

// SetDefaultTenant saves the new default tenant to the disk.
func (c *Config) SetDefaultTenant(tenantName string) error {
	tenant, err := c.GetTenant(tenantName)
	if err != nil {
		return err
	}

	c.DefaultTenant = tenant.Domain

	return c.saveToDisk()
}

// SetDefaultAppIDForTenant saves the new default app id for the tenant to the disk.
func (c *Config) SetDefaultAppIDForTenant(tenantName, appID string) error {
	tenant, err := c.GetTenant(tenantName)
	if err != nil {
		return err
	}

	tenant.DefaultAppID = appID
	c.Tenants[tenant.Domain] = tenant

	return c.saveToDisk()
}

func (c *Config) ensureInstallIDAssigned() {
	if c.InstallID != "" {
		return
	}

	c.InstallID = uuid.NewString()
}

func (c *Config) loadFromDisk() error {
	if c.path == "" {
		c.path = defaultPath()
	}

	if _, err := os.Stat(c.path); os.IsNotExist(err) {
		return ErrConfigFileMissing
	}

	buffer, err := os.ReadFile(c.path)
	if err != nil {
		return err
	}

	return json.Unmarshal(buffer, c)
}

func (c *Config) saveToDisk() error {
	dir := filepath.Dir(c.path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		const dirPerm os.FileMode = 0700 // Directory permissions (read, write, and execute for the owner only).
		if err := os.MkdirAll(dir, dirPerm); err != nil {
			return err
		}
	}

	buffer, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		return err
	}

	const filePerm os.FileMode = 0600 // File permissions (read and write for the owner only).
	return os.WriteFile(c.path, buffer, filePerm)
}

func defaultPath() string {
	return path.Join(os.Getenv("HOME"), ".config", "auth0", "config.json")
}
