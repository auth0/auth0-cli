package keyring

import (
	"errors"
	"fmt"
	"strings"

	"github.com/zalando/go-keyring"
)

const (
	secretRefreshToken = "Auth0 CLI Refresh Token"
	secretClientSecret = "Auth0 CLI Client Secret"
)

// StoreRefreshToken stores a tenant's refresh token in the system keyring.
func StoreRefreshToken(tenant, value string) error {
	return keyring.Set(secretRefreshToken, tenant, value)
}

// GetRefreshToken retrieves a tenant's refresh token from the system keyring.
func GetRefreshToken(tenant string) (string, error) {
	return keyring.Get(secretRefreshToken, tenant)
}

// StoreClientSecret stores a tenant's client secret in the system keyring.
func StoreClientSecret(tenant, value string) error {
	return keyring.Set(secretClientSecret, tenant, value)
}

// GetClientSecret retrieves a tenant's client secret from the system keyring.
func GetClientSecret(tenant string) (string, error) {
	return keyring.Get(secretClientSecret, tenant)
}

// DeleteSecretsForTenant deletes all secrets for a given tenant.
func DeleteSecretsForTenant(tenant string) error {
	var multiErrors []string

	if err := keyring.Delete(secretRefreshToken, tenant); err != nil {
		if !errors.Is(err, keyring.ErrNotFound) {
			multiErrors = append(multiErrors, fmt.Sprintf("failed to delete refresh token from keyring: %s", err))
		}
	}

	if err := keyring.Delete(secretClientSecret, tenant); err != nil {
		if !errors.Is(err, keyring.ErrNotFound) {
			multiErrors = append(multiErrors, fmt.Sprintf("failed to delete client secret from keyring: %s", err))
		}
	}

	if len(multiErrors) == 0 {
		return nil
	}

	return errors.New(strings.Join(multiErrors, ", "))
}
