package keyring

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

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

	t.Run("it successfully stores an access token", func(t *testing.T) {
		keyring.MockInit()

		expectedAccessToken := randomStringOfLength((2048 * 5) + 1) // Some arbitrarily long random string
		err := StoreAccessToken(testTenantName, expectedAccessToken)
		assert.NoError(t, err)

		actualAccessToken, err := GetAccessToken(testTenantName)
		assert.NoError(t, err)
		assert.Equal(t, expectedAccessToken, actualAccessToken)
	})

	t.Run("it successfully retrieves an access token split up into multiple chunks", func(t *testing.T) {
		keyring.MockInit()

		err := keyring.Set(fmt.Sprintf("%s %d", secretAccessToken, 0), testTenantName, "chunk0")
		assert.NoError(t, err)
		err = keyring.Set(fmt.Sprintf("%s %d", secretAccessToken, 1), testTenantName, "chunk1")
		assert.NoError(t, err)
		err = keyring.Set(fmt.Sprintf("%s %d", secretAccessToken, 2), testTenantName, "chunk2")
		assert.NoError(t, err)

		actualAccessToken, err := GetAccessToken(testTenantName)
		assert.NoError(t, err)
		assert.Equal(t, "chunk0chunk1chunk2", actualAccessToken)
	})
}

func randomStringOfLength(length int) string {
	var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))
	charset := "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}
