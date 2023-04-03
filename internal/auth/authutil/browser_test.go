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
	t.Run("Test success callback", func(t *testing.T) {
		// Set a timer to wait for the server to have started and then call the URL and assert.
		time.AfterFunc(1*time.Second, func() {
			client := &http.Client{}
			url := "http://localhost:1234?code=1234&state=1234"
			resp, err := client.Get(url)
			assert.NoError(t, err)

			bytes, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)
			body := string(bytes)

			defer resp.Body.Close()
			assert.Contains(t, body, "You can close the window and go back to the CLI to see the user info and tokens.")
		})

		code, state, callbackErr := WaitForBrowserCallback("localhost:1234")
		assert.NoError(t, callbackErr)
		assert.Equal(t, "1234", code)
		assert.Equal(t, "1234", state)
	})

	t.Run("Test error callback", func(t *testing.T) {
		// Set a timer to wait for the server to have started and then call the URL and assert.
		time.AfterFunc(1*time.Second, func() {
			client := &http.Client{}
			url := "http://localhost:1234?error=foo&error_description=bar"
			resp, err := client.Get(url)
			assert.NoError(t, err)
			bytes, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)
			body := string(bytes)
			defer resp.Body.Close()

			assert.Contains(t, body, "Unable to extract code from request, please try authenticating again")
		})

		code, state, callbackErr := WaitForBrowserCallback("localhost:1234")
		assert.Error(t, callbackErr)
		assert.Equal(t, "", code)
		assert.Equal(t, "", state)
	})
}
