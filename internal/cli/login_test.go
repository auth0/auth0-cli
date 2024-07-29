package cli

import (
	"bytes"
	"testing"

	"github.com/auth0/auth0-cli/internal/display"

	"github.com/stretchr/testify/assert"
)

func TestLoginCommand(t *testing.T) {
	t.Run("Negative Test: it returns an error since client-id, client-secret and domain must be passed together", func(t *testing.T) {
		cli := &cli{}
		cli.noInput = true
		cmd := loginCmd(cli)
		cmd.SetArgs([]string{"--client-id", "t3dbMFeTokYBguVu1Ty88gqntUXELSn9"})
		err := cmd.Execute()

		assert.EqualError(t, err, "flags client-id, client-secret and domain are required together")
	})

	t.Run("Negative Test: it returns an error since client-id, client-secret and domain must be passed together", func(t *testing.T) {
		cli := &cli{}
		cli.noInput = true
		cmd := loginCmd(cli)
		cmd.SetArgs([]string{"--client-secret", "3OAzE7j2HTnGOPeCRFX3Hg-0sipaEnodzQK8xpwsRiTkqdjjwEFT04rgCjfslianfs"})
		err := cmd.Execute()
		assert.EqualError(t, err, "flags client-id, client-secret and domain are required together")
	})

	t.Run("Negative Test: it returns an error since client-id, client-secret and domain must be passed together", func(t *testing.T) {
		cli := &cli{}
		cli.noInput = true
		cmd := loginCmd(cli)
		cmd.SetArgs([]string{"--client-id", "t3dbMFeTokYBguVu1Ty88gqntUXELSn9", "--client-secret", "3OAzE7j2HTnGOPeCRFX3Hg-0sipaEnodzQK8xpkqdjjwEFT0EFT04rgCp4PZL4Z"})
		err := cmd.Execute()
		assert.EqualError(t, err, "flags client-id, client-secret and domain are required together")
	})

	t.Run("Negative Test: it returns an error since client-id, client-secret and domain must be passed together", func(t *testing.T) {
		cli := &cli{}
		cli.noInput = true
		cmd := loginCmd(cli)
		cmd.SetArgs([]string{"--client-id", "t3dbMFeTokYBguVu1Ty88gqntUXELSn9", "--domain", "duedares.us.auth0.com"})
		err := cmd.Execute()
		assert.EqualError(t, err, "flags client-id, client-secret and domain are required together")
	})

	t.Run("Negative Test: it returns an error since client-id, client-secret and domain must be passed together", func(t *testing.T) {
		cli := &cli{}
		cli.noInput = true
		cmd := loginCmd(cli)
		cmd.SetArgs([]string{"--client-secret", "3OAzE7j2HTnGOPeCRFX3Hg-0sipaEnodzQK8xpkqdjjwEFT0EFT04rgCp4PZL4Z", "--domain", "duedares.us.auth0.com"})
		err := cmd.Execute()
		assert.EqualError(t, err, "flags client-id, client-secret and domain are required together")
	})

	t.Run("Positive Test: All three params are passed and Machine Flow is executed", func(t *testing.T) {
		message := &bytes.Buffer{}
		result := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: message,
				ResultWriter:  result,
			},
			noInput: true,
		}

		cmd := loginCmd(cli)
		cmd.SetArgs([]string{"--client-id", "t3dbMFeTokYBguVu1Ty88gqntUXELSn9", "--client-secret", "3OAzE7j2HTnGOPeCRFX3Hg-0sipaEnodzQK8xpwsRiTkqdjjwEFT04rgCp4PZL4Z", "--domain", "duedares.us.auth0.com"})
		err := cmd.Execute()
		assert.NoError(t, err)
		assert.Contains(t, message.String(), "Successfully logged in.")
	})
}
