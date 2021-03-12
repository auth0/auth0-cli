package auth

import (
	"testing"

	"github.com/zalando/go-keyring"
)

func TestSecrets(t *testing.T) {
	t.Run("fail: not found", func(t *testing.T) {
		// init underlying keychain manager
		keyring.MockInit()

		kr := &Keyring{}
		_, err := kr.Get("mynamespace", "foo")

		if got, want := err, keyring.ErrNotFound; got != want {
			t.Fatalf("wanted error: %v, got: %v", want, got)
		}
	})

	t.Run("succeed: get secret", func(t *testing.T) {
		// init underlying keychain manager
		keyring.MockInit()

		// set with the underlying manager:
		err := keyring.Set("mynamespace", "foo", "bar")
		if err != nil {
			t.Fatal(err)
		}

		kr := &Keyring{}
		v, err := kr.Get("mynamespace", "foo")
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

		kr := &Keyring{}
		err := kr.Set("mynamespace", "foo", "bar")
		if err != nil {
			t.Fatal(err)
		}

		// get with the underlying manager:
		v, err := keyring.Get("mynamespace", "foo")
		if err != nil {
			t.Fatal(err)
		}

		if got, want := v, "bar"; got != want {
			t.Fatalf("wanted secret: %v, got: %v", want, got)
		}
	})
}
