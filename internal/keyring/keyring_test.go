package keyring

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zalando/go-keyring"
)

const testTenantName = "auth0-cli-test.us.auth0.com"

func TestSecrets(t *testing.T) {
	t.Run("it fails to retrieve an nonexistent refresh token", func(t *testing.T) {
		keyring.MockInit()

		_, actualError := GetRefreshToken(testTenantName)
		assert.EqualError(t, actualError, keyring.ErrNotFound.Error())
	})

	t.Run("it successfully retrieves an existent refresh token", func(t *testing.T) {
		keyring.MockInit()

		expectedRefreshToken := "fake-refresh-token"
		err := keyring.Set(secretRefreshToken, testTenantName, expectedRefreshToken)
		assert.NoError(t, err)

		actualRefreshToken, err := GetRefreshToken(testTenantName)
		assert.NoError(t, err)
		assert.Equal(t, expectedRefreshToken, actualRefreshToken)
	})

	t.Run("it successfully stores a refresh token", func(t *testing.T) {
		keyring.MockInit()

		expectedRefreshToken := "fake-refresh-token"
		err := StoreRefreshToken(testTenantName, expectedRefreshToken)
		assert.NoError(t, err)

		actualRefreshToken, err := GetRefreshToken(testTenantName)
		assert.NoError(t, err)
		assert.Equal(t, expectedRefreshToken, actualRefreshToken)
	})

	t.Run("it fails to retrieve an nonexistent client secret", func(t *testing.T) {
		keyring.MockInit()

		_, actualError := GetClientSecret(testTenantName)
		assert.EqualError(t, actualError, keyring.ErrNotFound.Error())
	})

	t.Run("it successfully retrieves an existent client secret", func(t *testing.T) {
		keyring.MockInit()

		expectedRefreshToken := "fake-refresh-token"
		err := keyring.Set(secretClientSecret, testTenantName, expectedRefreshToken)
		assert.NoError(t, err)

		actualRefreshToken, err := GetClientSecret(testTenantName)
		assert.NoError(t, err)
		assert.Equal(t, expectedRefreshToken, actualRefreshToken)
	})

	t.Run("it successfully stores a client secret", func(t *testing.T) {
		keyring.MockInit()

		expectedRefreshToken := "fake-refresh-token"
		err := StoreClientSecret(testTenantName, expectedRefreshToken)
		assert.NoError(t, err)

		actualRefreshToken, err := GetClientSecret(testTenantName)
		assert.NoError(t, err)
		assert.Equal(t, expectedRefreshToken, actualRefreshToken)
	})
}
