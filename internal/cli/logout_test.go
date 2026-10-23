package cli

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gokeyring "github.com/zalando/go-keyring"

	"github.com/auth0/auth0-cli/internal/config"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/plugins/approvals"
)

func TestLogoutRemovesPluginApprovals(t *testing.T) {
	const (
		tenant = "acme.us.auth0.com"
		other  = "other.us.auth0.com"
		id     = "auth0/checkmate"
		hash   = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	)

	// Setup logs in to two tenants in a new home directory, and returns the CLI
	// and the approvals in the plugins directory next to its config file. The path
	// is written out here so that the test notices if logout looks somewhere else.
	setup := func(t *testing.T) (*cli, *approvals.Approvals) {
		t.Helper()

		gokeyring.MockInit()

		home := t.TempDir()
		t.Setenv("HOME", home)

		// The config remembers that its file was missing, so it is saved through
		// one config and read by the CLI through another.
		var saved config.Config
		for _, domain := range []string{tenant, other} {
			require.NoError(t, saved.AddTenant(config.Tenant{Domain: domain}))
		}

		return &cli{renderer: display.NewRenderer()}, &approvals.Approvals{Dir: filepath.Join(home, ".config", "auth0", "plugins")}
	}

	logout := func(c *cli) error {
		cmd := logoutCmd(c)
		cmd.SetArgs([]string{tenant})
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)

		return cmd.Execute()
	}

	t.Run("approvals of the tenant go and the others stay", func(t *testing.T) {
		c, plugins := setup(t)
		require.NoError(t, plugins.Approve(t.Context(), tenant, id, hash))
		require.NoError(t, plugins.Approve(t.Context(), other, id, hash))

		require.NoError(t, logout(c))

		assert.Error(t, plugins.Check(tenant, id, hash))
		assert.NoError(t, plugins.Check(other, id, hash))

		_, err := c.Config.GetTenant(tenant)
		assert.Error(t, err, "the tenant is logged out")
	})

	t.Run("a failing removal stops the logout and the tenant stays configured", func(t *testing.T) {
		c, plugins := setup(t)

		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			t.Skip("a directory that cannot be written needs a user who is not root, on a system with permission bits")
		}

		// A damaged file is deleted, which fails when its directory cannot be
		// written. The lock file exists already, so that taking the lock still works.
		require.NoError(t, os.MkdirAll(plugins.Dir, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(plugins.Dir, "approvals.json"), []byte("not json"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(plugins.Dir, ".approvals.lock"), nil, 0o600))
		require.NoError(t, os.Chmod(plugins.Dir, 0o500))

		t.Cleanup(func() { _ = os.Chmod(plugins.Dir, 0o700) })

		err := logout(c)

		require.ErrorContains(t, err, "failed to remove the plugin approvals")
		assert.ErrorContains(t, err, "failed to remove the damaged plugin approvals file")

		_, err = c.Config.GetTenant(tenant)
		assert.NoError(t, err, "the tenant is still logged in, so logout can be tried again")
	})

	t.Run("a user who never used plugins sees no change", func(t *testing.T) {
		c, plugins := setup(t)

		require.NoError(t, logout(c))

		_, err := os.Stat(plugins.Dir)
		assert.ErrorIs(t, err, os.ErrNotExist)
	})
}
