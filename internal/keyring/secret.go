package keyring

import (
	"fmt"

	"github.com/zalando/go-keyring"
)

const (
	secretRefreshToken = "Auth0 CLI Refresh Token"
)

// StoreRefreshToken stores a tenant's refresh token in the system keyring
func StoreRefreshToken(tenant, value string) error {
	if err := keyring.Set(secretRefreshToken, tenant, value); err != nil {
		return fmt.Errorf("unable to retrieve refresh token from keyring: %w", err)
	}
	return nil
}

// GetRefreshToken retrieves a tenant's refresh token from the system keyring
func GetRefreshToken(tenant string) (string, error) {
	cs, err := keyring.Get(secretRefreshToken, tenant)
	if err != nil {
		return "", fmt.Errorf("unable to retrieve refresh token from keyring: %w", err)
	}
	return cs, nil
}

// DeleteSecretsForTenant deletes all secrets for a given tenant
func DeleteSecretsForTenant(tenant string) error {
	err := keyring.Delete(secretRefreshToken, tenant)
	if err != nil {
		return fmt.Errorf("unable to delete refresh token from keyring: %s", err)
	}

	return nil
}
