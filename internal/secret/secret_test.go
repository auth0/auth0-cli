package secret

import (
	"testing"

	"github.com/zalando/go-keyring"
)

const testTenantName = "auth0-cli-test.us.auth0.com"

func TestSecrets(t *testing.T) {
	t.Run("fail: not found", func(t *testing.T) {
		keyring.MockInit()

		_, err := GetRefreshToken(testTenantName)

		if got, want := err, keyring.ErrNotFound; got != want {
			t.Fatalf("wanted error: %v, got: %v", want, got)
		}
	})

	t.Run("succeed: get refresh token", func(t *testing.T) {
		keyring.MockInit()

		err := keyring.Set(secretRefreshToken, testTenantName, "fake-refresh-token")
		if err != nil {
			t.Fatal(err)
		}

		v, err := GetRefreshToken(testTenantName)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := v, "fake-refresh-token"; got != want {
			t.Fatalf("wanted error: %v, got: %v", want, got)
		}
	})

	t.Run("succeed: set refresh token", func(t *testing.T) {
		keyring.MockInit()

		err := StoreRefreshToken(testTenantName, "fake-refresh-token")
		if err != nil {
			t.Fatal(err)
		}

		v, err := keyring.Get(secretRefreshToken, testTenantName)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := v, "fake-refresh-token"; got != want {
			t.Fatalf("wanted secret: %v, got: %v", want, got)
		}
	})
}
