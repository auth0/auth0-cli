package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/auth0/auth0-cli/internal/auth"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/config"
)

const (
	// AUTH0_CLI_AUTH_MODE opts the CLI into non-persistent, environment-driven
	// authentication. It must be set explicitly: the AUTH0_* credential vars this
	// mode reads are also consumed by the Terraform provider and by the sample
	// apps the CLI scaffolds, so their mere presence must never silently replace a
	// saved login or select a different tenant.
	authModeEnvVar = "AUTH0_CLI_AUTH_MODE"
	// Value of authModeEnvVar that selects the env credential source.
	authModeEnv = "env"

	envDomain       = "AUTH0_DOMAIN"
	envAPIToken     = "AUTH0_API_TOKEN"
	envClientID     = "AUTH0_CLIENT_ID"
	envClientSecret = "AUTH0_CLIENT_SECRET"
)

// envAuthEnabled reports whether AUTH0_CLI_AUTH_MODE selects the env credential
// source. The comparison is trimmed and case-insensitive.
func envAuthEnabled(getenv func(string) string) bool {
	return strings.EqualFold(strings.TrimSpace(getenv(authModeEnvVar)), authModeEnv)
}

// validateAuthMode rejects a nonempty AUTH0_CLI_AUTH_MODE that is not a
// recognized value. Without this a typo such as "evn" would fail the
// envAuthEnabled check and silently fall through to the saved-login path, so a
// command could target the saved default tenant even though the caller set
// AUTH0_DOMAIN and expected env credentials. An empty value (env mode off) and
// the exact env value are both accepted.
func validateAuthMode(getenv func(string) string) error {
	mode := strings.TrimSpace(getenv(authModeEnvVar))
	if mode == "" || strings.EqualFold(mode, authModeEnv) {
		return nil
	}

	return usageError{
		err: fmt.Errorf(
			"unrecognized %s=%q; the only supported value is %q. Unset %s to use your saved login",
			authModeEnvVar, mode, authModeEnv, authModeEnvVar,
		),
		reason: "invalid_auth_mode",
	}
}

// setupWithEnvAuthentication authenticates purely from environment variables,
// without reading or writing the on-disk config or the OS keychain. It is the
// opt-in path for agents, CI, and sandboxes that cannot reach the keychain.
//
// It accepts either a pre-minted Management API token via AUTH0_API_TOKEN (used
// directly, preserving whatever identity minted it) or client credentials via
// AUTH0_CLIENT_ID and AUTH0_CLIENT_SECRET (exchanged for a token). AUTH0_DOMAIN
// is always required. The resolved token is held in memory for this command only
// and is never persisted. This mirrors the env convention the Terraform provider
// already uses (see terraformProviderCredentialsAreAvailable).
func (c *cli) setupWithEnvAuthentication(ctx context.Context) error {
	domain := strings.TrimSpace(os.Getenv(envDomain))
	apiToken := strings.TrimSpace(os.Getenv(envAPIToken))
	clientID := strings.TrimSpace(os.Getenv(envClientID))
	clientSecret := strings.TrimSpace(os.Getenv(envClientSecret))

	if domain == "" {
		return authError{
			err: fmt.Errorf(
				"%s=%s requires %s to be set to your tenant domain (for example tenant.us.auth0.com)",
				authModeEnvVar, authModeEnv, envDomain,
			),
			reason: "env_auth_incomplete",
		}
	}

	if err := validateTenantDomainHost(domain); err != nil {
		return authError{err: err, reason: "env_auth_invalid_domain"}
	}

	// In env mode the tenant is fixed by AUTH0_DOMAIN. If the caller also passed an
	// explicit --tenant that points somewhere else, fail loudly rather than
	// silently ignoring the flag and targeting AUTH0_DOMAIN, which could send a
	// write to the wrong tenant.
	if c.tenantExplicit {
		if requested := strings.TrimSpace(c.tenant); !strings.EqualFold(requested, domain) {
			return authError{
				err: fmt.Errorf(
					"--tenant %q conflicts with %s=%q; in %s=%s mode the tenant is fixed by %s. "+
						"Drop --tenant, or set %s to the tenant you want to target",
					c.tenant, envDomain, domain, authModeEnvVar, authModeEnv, envDomain, envDomain,
				),
				reason: "env_auth_tenant_conflict",
			}
		}
	}

	accessToken, err := c.resolveEnvAccessToken(ctx, domain, apiToken, clientID, clientSecret)
	if err != nil {
		return err
	}

	// Record the tenant for display and analytics only; no config is loaded or
	// written in this mode. The renderer's tenant was set from the flag default
	// during earlier setup, so refresh it here too, otherwise human-readable
	// headings would show the old or an empty tenant instead of AUTH0_DOMAIN.
	c.tenant = domain
	c.renderer.Tenant = domain

	invokerMetadata := c.invokerMetadataHeaderValue()

	api, err := initializeManagementClient(domain, accessToken, invokerMetadata)
	if err != nil {
		return authError{err: err, reason: "client_init_failed"}
	}

	apiv3, err := initializeManagementClientV3(domain, accessToken, invokerMetadata)
	if err != nil {
		return authError{err: err, reason: "client_init_failed"}
	}

	c.api = auth0.NewAPI(api)
	c.apiv3 = auth0.NewAPIV3(apiv3)

	return nil
}

// resolveEnvAccessToken returns the access token for env-mode auth, preferring a
// directly supplied AUTH0_API_TOKEN and otherwise exchanging client credentials.
// It never falls back to a saved login: an incomplete env configuration fails
// clearly. The client secret is never included in the returned error.
func (c *cli) resolveEnvAccessToken(ctx context.Context, domain, apiToken, clientID, clientSecret string) (string, error) {
	switch {
	case apiToken != "":
		return apiToken, nil
	case clientID != "" && clientSecret != "":
		token, err := auth.GetAccessTokenFromClientCreds(ctx, auth.ClientCredentials{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Domain:       domain,
		})
		if err != nil {
			return "", authError{
				err: fmt.Errorf(
					"failed to obtain an access token from %s and %s: %w. "+
						"In this mode the CLI exchanges these client credentials for a new token on every "+
						"invocation, so for repeated calls it is often simpler to mint one token yourself and "+
						"pass it through %s instead (remembering to refresh it before it expires)",
					envClientID, envClientSecret, err, envAPIToken,
				),
				reason: "env_auth_exchange_failed",
			}
		}
		return token.AccessToken, nil
	default:
		return "", authError{
			err: fmt.Errorf(
				"%s=%s requires either %s, or both %s and %s",
				authModeEnvVar, authModeEnv, envAPIToken, envClientID, envClientSecret,
			),
			reason: "env_auth_incomplete",
		}
	}
}

// configPersistError converts a config-write failure into an actionable auth
// error. It is used wherever the CLI must persist a tenant or refreshed token to
// disk: when the config file is not writable (for example a read-only sandbox),
// it points the caller at env-mode auth, which needs no disk access, or at
// running outside the sandbox. Any other error is returned unchanged.
func configPersistError(err error) error {
	if err == nil {
		return nil
	}

	if !errors.Is(err, config.ErrConfigNotWritable) {
		return err
	}

	return authError{
		err: fmt.Errorf(
			"%w. The login could not be saved because the config file is not writable, which "+
				"happens in read-only sandboxes. To authenticate without writing to disk, set "+
				"AUTH0_CLI_AUTH_MODE=env together with AUTH0_API_TOKEN (or AUTH0_DOMAIN, AUTH0_CLIENT_ID "+
				"and AUTH0_CLIENT_SECRET). Otherwise run the command outside the sandbox with write access",
			err,
		),
		reason: "config_not_writable",
	}
}

// validateTenantDomainHost ensures AUTH0_DOMAIN is a bare host name rather than a
// URL. The Management SDK expects a host such as "tenant.us.auth0.com"; a scheme
// or path would otherwise produce confusing downstream failures.
//
// It rejects any character that is not valid in a DNS host name. That is a
// security check, not just cosmetics: a value such as "tenant.example@evil.example"
// parses as a URL whose host is "evil.example" with "tenant.example" as userinfo,
// so a client-credentials token request would send the client secret to
// evil.example. Restricting the input to host characters closes that path before
// any token is requested.
func validateTenantDomainHost(domain string) error {
	for _, r := range domain {
		isLetter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		isDigit := r >= '0' && r <= '9'
		if !isLetter && !isDigit && r != '.' && r != '-' {
			return fmt.Errorf(
				"%s must be a bare tenant domain such as tenant.us.auth0.com, not a URL or credentials string",
				envDomain,
			)
		}
	}

	if !strings.Contains(domain, ".") {
		return fmt.Errorf(
			"%s %q does not look like a tenant domain (expected something like tenant.us.auth0.com)",
			envDomain, domain,
		)
	}

	return nil
}
