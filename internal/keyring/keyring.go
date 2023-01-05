package keyring

import (
	"errors"

	"github.com/zalando/go-keyring"
)

const secretRefreshToken = "Auth0 CLI Refresh Token"

// StoreRefreshToken stores a tenant's refresh token in the system keyring.
func StoreRefreshToken(tenant, value string) error {
	return keyring.Set(secretRefreshToken, tenant, value)
}

// GetRefreshToken retrieves a tenant's refresh token from the system keyring.
func GetRefreshToken(tenant string) (string, error) {
	return keyring.Get(secretRefreshToken, tenant)
}

// DeleteSecretsForTenant deletes all secrets for a given tenant.
func DeleteSecretsForTenant(tenant string) error {
	if err := keyring.Delete(secretRefreshToken, tenant); err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil
		}

		return err
	}

	return nil
}
