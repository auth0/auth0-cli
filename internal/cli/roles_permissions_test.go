package cli

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestPickRolePermissionsNoInputGuard(t *testing.T) {
	c := &cli{}
	c.noInput = true // Non-interactive mode.

	var permissions []string
	err := c.pickRolePermissions(&cobra.Command{}, nil, &permissions)

	assert.EqualError(t, err, "missing a required flag in non-interactive mode: --permissions")
}
