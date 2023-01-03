package auth

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

	t.Run("succeed: get secret", func(t *testing.T) {
		// init underlying keychain manager
		keyring.MockInit()

		// set with the underlying manager:
		err := keyring.Set("auth0-cli-refresh-token", testTenantName, "bar")
		if err != nil {
			t.Fatal(err)
		}

		v, err := GetRefreshToken(testTenantName)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := v, "bar"; got != want {
			t.Fatalf("wanted error: %v, got: %v", want, got)
		}
	})

	t.Run("succeed: set secret", func(t *testing.T) {
		// init underlying keychain manager
		keyring.MockInit()

		testTenantName := "auth0-cli-test.us.auth0.com"

		err := StoreRefreshToken(testTenantName, "bar")
		if err != nil {
			t.Fatal(err)
		}

		// get with the underlying manager:
		v, err := keyring.Get(testTenantName, "foo")
		if err != nil {
			t.Fatal(err)
		}

		if got, want := v, "bar"; got != want {
			t.Fatalf("wanted secret: %v, got: %v", want, got)
		}
	})
}
