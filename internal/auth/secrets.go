package auth

import (
	"github.com/pkg/errors"
	"github.com/zalando/go-keyring"
)

const (
	secretRefreshToken = "Auth0 CLI Refresh Token"
)

// StoreRefreshToken stores a tenant's refresh token in the system keyring
func StoreRefreshToken(tenant, value string) error {
	return keyring.Set(secretRefreshToken, tenant, value)
}

// GetRefreshToken retrieves a tenant's refresh token from the system keyring
func GetRefreshToken(tenant string) (string, error) {
	return keyring.Get(secretRefreshToken, tenant)
}

// Delete deletes a value for the given namespace and key.
func DeleteSecretsForTenant(tenant string) error {
	var errs error

	e := keyring.Delete(secretRefreshToken, tenant)
	if e != nil {
		errs = errors.Wrap(errs, e.Error())
	}

	return errs
}
