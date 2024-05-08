package authutil

import (
	_ "embed"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWaitForBrowserCallback(t *testing.T) {
	t.Run("Handle success on callback", func(t *testing.T) {
		// Set a timer to wait for the server to have started and then call the URL and assert.
		time.AfterFunc(1*time.Second, func() {
			client := &http.Client{}
			url := "http://localhost:1234?code=1234&state=1234"
			resp, err := client.Get(url)
			assert.NoError(t, err)

			bytes, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)
			body := string(bytes)

			defer func() {
				_ = resp.Body.Close()
			}()
			assert.Contains(t, body, "You can close the window and go back to the CLI to see the user info and tokens.")
		})

		code, state, callbackErr := WaitForBrowserCallback("localhost:1234")
		assert.NoError(t, callbackErr)
		assert.Equal(t, "1234", code)
		assert.Equal(t, "1234", state)
	})

	t.Run("Handle error on callback", func(t *testing.T) {
		// Set a timer to wait for the server to have started and then call the URL and assert.
		time.AfterFunc(1*time.Second, func() {
			client := &http.Client{}
			url := "http://localhost:1234?error=foo&error_description=bar"
			resp, err := client.Get(url)
			assert.NoError(t, err)
			bytes, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)
			body := string(bytes)
			defer func() {
				_ = resp.Body.Close()
			}()

			assert.Contains(t, body, "Failed to extract code from request, please try authenticating again")
		})

		code, state, callbackErr := WaitForBrowserCallback("localhost:1234")
		assert.Error(t, callbackErr)
		assert.Equal(t, "", code)
		assert.Equal(t, "", state)
	})

	t.Run("Failure to start server", func(t *testing.T) {
		s := &http.Server{Addr: "localhost:1234"}

		go func() {
			if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				assert.NoError(t, err)
			}
		}()

		time.Sleep(1 * time.Second)

		defer func() {
			_ = s.Close()
		}()

		code, state, callbackErr := WaitForBrowserCallback("localhost:1234")

		assert.EqualError(t, callbackErr, ErrBindFailure)
		assert.Equal(t, "", code)
		assert.Equal(t, "", state)
	})
}
