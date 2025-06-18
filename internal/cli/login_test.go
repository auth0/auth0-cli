package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoginCommand(t *testing.T) {
	t.Run("Negative Test: it returns an error when only client-id passed", func(t *testing.T) {
		cli := &cli{}
		cli.noInput = true
		cmd := loginCmd(cli)
		cmd.SetArgs([]string{"--client-id", "t3dbMFeTokYBguVu1Ty88gqntUXELSn9"})
		err := cmd.Execute()

		assert.EqualError(t, err, "for machine login, provide domain with either (client-id, client-secret) or (client-id, client-assertion-signing-alg, client-assertion-private-key)")
	})

	t.Run("Negative Test: it returns an error when only client-secret passed", func(t *testing.T) {
		cli := &cli{}
		cli.noInput = true
		cmd := loginCmd(cli)
		cmd.SetArgs([]string{"--client-secret", "3OAzE7j2HTnGOPeCRFX3Hg-0sipaEnodzQK8xpwsRiTkqdjjwEFT04rgCjfslianfs"})
		err := cmd.Execute()
		assert.EqualError(t, err, "for machine login, provide domain with either (client-id, client-secret) or (client-id, client-assertion-signing-alg, client-assertion-private-key)")
	})

	t.Run("Negative Test: it returns an error when only client-id, client-secret passed together", func(t *testing.T) {
		cli := &cli{}
		cli.noInput = true
		cmd := loginCmd(cli)
		cmd.SetArgs([]string{"--client-id", "t3dbMFeTokYBguVu1Ty88gqntUXELSn9", "--client-secret", "3OAzE7j2HTnGOPeCRFX3Hg-0sipaEnodzQK8xpkqdjjwEFT0EFT04rgCp4PZL4Z"})
		err := cmd.Execute()
		assert.EqualError(t, err, "for machine login, provide domain with either (client-id, client-secret) or (client-id, client-assertion-signing-alg, client-assertion-private-key)")
	})

	t.Run("Negative Test: it returns an error when only client-id, domain passed together", func(t *testing.T) {
		cli := &cli{}
		cli.noInput = true
		cmd := loginCmd(cli)
		cmd.SetArgs([]string{"--client-id", "t3dbMFeTokYBguVu1Ty88gqntUXELSn9", "--domain", "test.auth0.com"})
		err := cmd.Execute()
		assert.EqualError(t, err, "for machine login, provide domain with either (client-id, client-secret) or (client-id, client-assertion-signing-alg, client-assertion-private-key)")
	})

	t.Run("Negative Test: it returns an error when only client-secret, domain passed together", func(t *testing.T) {
		cli := &cli{}
		cli.noInput = true
		cmd := loginCmd(cli)
		cmd.SetArgs([]string{"--client-secret", "3OAzE7j2HTnGOPeCRFX3Hg-0sipaEnodzQK8xpkqdjjwEFT0EFT04rgCp4PZL4Z", "--domain", "test.auth0.com"})
		err := cmd.Execute()
		assert.EqualError(t, err, "for machine login, provide domain with either (client-id, client-secret) or (client-id, client-assertion-signing-alg, client-assertion-private-key)")
	})
	t.Run("Negative Test: it returns an error when only client-assertion-signing-alg passed", func(t *testing.T) {
		cli := &cli{}
		cli.noInput = true
		cmd := loginCmd(cli)
		cmd.SetArgs([]string{"--client-assertion-signing-alg", "RS256"})
		err := cmd.Execute()
		assert.EqualError(t, err, "for machine login, provide domain with either (client-id, client-secret) or (client-id, client-assertion-signing-alg, client-assertion-private-key)")
	})
	t.Run("Negative Test: it returns an error when only client-assertion-private-key-path, domain passed together", func(t *testing.T) {
		cli := &cli{}
		cli.noInput = true
		cmd := loginCmd(cli)
		cmd.SetArgs([]string{"--client-assertion-private-key-path", "./secrets/private_key.pem", "--domain", "test.auth0.com"})
		err := cmd.Execute()
		assert.EqualError(t, err, "for machine login, provide domain with either (client-id, client-secret) or (client-id, client-assertion-signing-alg, client-assertion-private-key)")
	})
}
