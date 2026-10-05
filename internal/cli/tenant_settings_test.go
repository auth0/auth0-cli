package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTenantSettingsNoInputGuard(t *testing.T) {
	expectedError := "missing required arguments in non-interactive mode: pass the setting flags to change as arguments"

	t.Run("set errors when no flags are passed in non-interactive mode", func(t *testing.T) {
		cli := &cli{}
		cli.noInput = true // Non-interactive mode.

		cmd := set(cli)
		cmd.SetArgs([]string{})

		assert.EqualError(t, cmd.Execute(), expectedError)
	})

	t.Run("unset errors when no flags are passed in non-interactive mode", func(t *testing.T) {
		cli := &cli{}
		cli.noInput = true // Non-interactive mode.

		cmd := unset(cli)
		cmd.SetArgs([]string{})

		assert.EqualError(t, cmd.Execute(), expectedError)
	})
}
