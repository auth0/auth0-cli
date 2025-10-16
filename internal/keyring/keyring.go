package keyring

import (
	"errors"
	"fmt"
	"strings"

	"github.com/zalando/go-keyring"
)

const (
	secretRefreshToken                = "Auth0 CLI Refresh Token"
	secretClientSecret                = "Auth0 CLI Client Secret"
	secretAccessToken                 = "Auth0 CLI Access Token"
	secretAccessTokenChunkSizeInBytes = 2048

	// Access tokens have no size limit, but should be smaller than (50*2048) bytes.
	// The max number of loops safeguards against infinite loops, however unlikely.
	secretAccessTokenMaxChunks = 50
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

	for i := 0; i < secretAccessTokenMaxChunks; i++ {
		if err := keyring.Delete(fmt.Sprintf("%s %d", secretAccessToken, i), tenant); err != nil {
			if !errors.Is(err, keyring.ErrNotFound) {
				multiErrors = append(multiErrors, fmt.Sprintf("failed to delete access token from keyring: %s", err))
			}
		}
	}

	if len(multiErrors) == 0 {
		return nil
	}

	return errors.New(strings.Join(multiErrors, ", "))
}

func StoreAccessToken(tenant, value string) error {
	// First, clear any existing chunks to prevent concatenation issues.
	for i := 0; i < secretAccessTokenMaxChunks; i++ {
		if err := keyring.Delete(secretClientSecret, tenant); err != nil {
			if !errors.Is(err, keyring.ErrNotFound) {
				return fmt.Errorf("failed to delete client secret from keyring: %s", err)
			}
		}
	}

	// Now store the new token in chunks.
	chunks := chunk(value, secretAccessTokenChunkSizeInBytes)

	for i := 0; i < len(chunks); i++ {
		err := keyring.Set(fmt.Sprintf("%s %d", secretAccessToken, i), tenant, chunks[i])
		if err != nil {
			return err
		}
	}

	return nil
}

func GetAccessToken(tenant string) (string, error) {
	var accessToken string

	for i := 0; i < secretAccessTokenMaxChunks; i++ {
		a, err := keyring.Get(fmt.Sprintf("%s %d", secretAccessToken, i), tenant)
		// Only return if we have pulled more than 1 item from the keyring, otherwise this will be
		// a valid "secret not found in keyring".
		if err == keyring.ErrNotFound && i > 0 {
			return accessToken, nil
		}
		if err != nil {
			return "", err
		}
		accessToken += a
	}

	return accessToken, nil
}

func chunk(slice string, chunkSize int) []string {
	var chunks []string
	for i := 0; i < len(slice); i += chunkSize {
		end := i + chunkSize

		// Necessary check to avoid slicing beyond
		// slice capacity.
		if end > len(slice) {
			end = len(slice)
		}

		chunks = append(chunks, slice[i:end])
	}

	return chunks
}
