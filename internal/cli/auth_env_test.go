package cli

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/config"
	"github.com/auth0/auth0-cli/internal/display"
)

func TestEnvAuthEnabled(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{name: "exact match", value: "env", expected: true},
		{name: "case insensitive", value: "ENV", expected: true},
		{name: "trimmed", value: "  env  ", expected: true},
		{name: "empty is disabled", value: "", expected: false},
		{name: "false is disabled", value: "false", expected: false},
		{name: "other value is disabled", value: "keychain", expected: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			getenv := func(key string) string {
				if key == authModeEnvVar {
					return test.value
				}
				return ""
			}
			assert.Equal(t, test.expected, envAuthEnabled(getenv))
		})
	}
}

func TestValidateAuthMode(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		expectErr bool
	}{
		{name: "empty is accepted (env mode off)", value: "", expectErr: false},
		{name: "whitespace only is accepted", value: "   ", expectErr: false},
		{name: "exact env is accepted", value: "env", expectErr: false},
		{name: "case insensitive env is accepted", value: "ENV", expectErr: false},
		{name: "trimmed env is accepted", value: "  env  ", expectErr: false},
		{name: "typo is rejected", value: "evn", expectErr: true},
		{name: "unrelated value is rejected", value: "keychain", expectErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			getenv := func(key string) string {
				if key == authModeEnvVar {
					return test.value
				}
				return ""
			}

			err := validateAuthMode(getenv)
			if !test.expectErr {
				assert.NoError(t, err)
				return
			}

			assert.Error(t, err)
			var usageErr usageError
			assert.True(t, errors.As(err, &usageErr))
			assert.Equal(t, "invalid_auth_mode", usageErr.reason)
		})
	}
}

func TestValidateTenantDomainHost(t *testing.T) {
	tests := []struct {
		name      string
		domain    string
		expectErr bool
	}{
		{name: "bare domain is valid", domain: "tenant.us.auth0.com", expectErr: false},
		{name: "custom domain is valid", domain: "login.example.com", expectErr: false},
		{name: "url with scheme is rejected", domain: "https://tenant.us.auth0.com", expectErr: true},
		{name: "domain with path is rejected", domain: "tenant.us.auth0.com/api/v2", expectErr: true},
		{name: "domain with space is rejected", domain: "tenant us.auth0.com", expectErr: true},
		{name: "host without dot is rejected", domain: "localhost", expectErr: true},
		{name: "userinfo is rejected", domain: "tenant.example@other.example", expectErr: true},
		{name: "port is rejected", domain: "tenant.us.auth0.com:8443", expectErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateTenantDomainHost(test.domain)
			if test.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestResolveEnvAccessToken(t *testing.T) {
	c := &cli{}

	t.Run("prefers a directly supplied API token", func(t *testing.T) {
		token, err := c.resolveEnvAccessToken(context.Background(), "tenant.us.auth0.com", "direct-token", "", "")
		assert.NoError(t, err)
		assert.Equal(t, "direct-token", token)
	})

	t.Run("uses the API token even when client credentials are also present", func(t *testing.T) {
		token, err := c.resolveEnvAccessToken(context.Background(), "tenant.us.auth0.com", "direct-token", "client-id", "client-secret")
		assert.NoError(t, err)
		assert.Equal(t, "direct-token", token)
	})

	t.Run("fails clearly when no usable credentials are provided", func(t *testing.T) {
		_, err := c.resolveEnvAccessToken(context.Background(), "tenant.us.auth0.com", "", "", "")
		assert.Error(t, err)

		var authErr authError
		assert.True(t, errors.As(err, &authErr))
		assert.Equal(t, "env_auth_incomplete", authErr.reason)
	})

	t.Run("fails clearly when only the client id is provided", func(t *testing.T) {
		_, err := c.resolveEnvAccessToken(context.Background(), "tenant.us.auth0.com", "", "client-id", "")
		assert.Error(t, err)

		var authErr authError
		assert.True(t, errors.As(err, &authErr))
		assert.Equal(t, "env_auth_incomplete", authErr.reason)
	})
}

func TestSetupWithEnvAuthenticationTenantConflict(t *testing.T) {
	t.Run("rejects an explicit --tenant that differs from AUTH0_DOMAIN", func(t *testing.T) {
		t.Setenv(authModeEnvVar, authModeEnv)
		t.Setenv(envDomain, "tenant.us.auth0.com")
		t.Setenv(envAPIToken, "direct-token")

		c := &cli{tenant: "other.us.auth0.com", tenantExplicit: true}
		err := c.setupWithEnvAuthentication(context.Background())
		assert.Error(t, err)

		var authErr authError
		assert.True(t, errors.As(err, &authErr))
		assert.Equal(t, "env_auth_tenant_conflict", authErr.reason)
	})

	t.Run("allows an explicit --tenant that matches AUTH0_DOMAIN case-insensitively", func(t *testing.T) {
		t.Setenv(authModeEnvVar, authModeEnv)
		t.Setenv(envDomain, "tenant.us.auth0.com")
		t.Setenv(envAPIToken, "direct-token")

		c := &cli{tenant: "TENANT.us.auth0.com", tenantExplicit: true, renderer: &display.Renderer{}}
		err := c.setupWithEnvAuthentication(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, "tenant.us.auth0.com", c.tenant)
		assert.Equal(t, "tenant.us.auth0.com", c.renderer.Tenant)
	})

	t.Run("ignores the tenant default when --tenant was not explicitly set", func(t *testing.T) {
		t.Setenv(authModeEnvVar, authModeEnv)
		t.Setenv(envDomain, "tenant.us.auth0.com")
		t.Setenv(envAPIToken, "direct-token")

		// Here c.tenant carries the configured default, but the flag was not changed.
		c := &cli{tenant: "default.us.auth0.com", tenantExplicit: false, renderer: &display.Renderer{}}
		err := c.setupWithEnvAuthentication(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, "tenant.us.auth0.com", c.tenant)
		assert.Equal(t, "tenant.us.auth0.com", c.renderer.Tenant)
	})
}

func TestSetupWithEnvAuthenticationNoSavedConfig(t *testing.T) {
	// An empty HOME means no config file exists on disk. Env auth must not depend
	// on saved config or the keychain, so setup should still wire up the clients.
	t.Setenv("HOME", t.TempDir())
	t.Setenv(authModeEnvVar, authModeEnv)
	t.Setenv(envDomain, "tenant.us.auth0.com")
	t.Setenv(envAPIToken, "direct-token")

	c := &cli{renderer: &display.Renderer{}}
	err := c.setupWithEnvAuthentication(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "tenant.us.auth0.com", c.tenant)
	assert.NotNil(t, c.api)
	assert.NotNil(t, c.apiv3)
}

func TestResolveInstallIDForTrackingSkipsDiskInEnvMode(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Persist a config carrying an install ID to disk.
	seed := config.Config{}
	assert.NoError(t, seed.AddTenant(config.Tenant{Domain: "seed.us.auth0.com", Name: "seed"}))
	if seed.InstallID == "" {
		t.Fatal("expected AddTenant to assign an install ID")
	}

	// In env auth mode, tracking must not read that config: it reports no install
	// ID rather than touching config.json, keeping the non-persistent contract.
	t.Setenv(authModeEnvVar, authModeEnv)
	assert.Empty(t, resolveInstallIDForTracking(&cli{}))

	// Without env mode, the same lookup reads the persisted install ID from disk,
	// confirming the value really was on disk and only the env-mode path skips it.
	t.Setenv(authModeEnvVar, "")
	assert.Equal(t, seed.InstallID, resolveInstallIDForTracking(&cli{}))
}

func TestConfigPersistError(t *testing.T) {
	t.Run("returns nil for a nil error", func(t *testing.T) {
		assert.NoError(t, configPersistError(nil))
	})

	t.Run("passes through an unrelated error unchanged", func(t *testing.T) {
		original := errors.New("some other failure")
		assert.Equal(t, original, configPersistError(original))
	})

	t.Run("wraps a not-writable error with actionable env-mode guidance", func(t *testing.T) {
		wrapped := fmt.Errorf("failed to save tenant data: %w", fmt.Errorf("%w: /some/path: read-only file system", config.ErrConfigNotWritable))

		err := configPersistError(wrapped)
		assert.Error(t, err)

		// The sentinel chain is preserved so callers can still detect it.
		assert.ErrorIs(t, err, config.ErrConfigNotWritable)

		var authErr authError
		assert.True(t, errors.As(err, &authErr))
		assert.Equal(t, "config_not_writable", authErr.reason)
		assert.Contains(t, err.Error(), "AUTH0_CLI_AUTH_MODE=env")
	})
}
