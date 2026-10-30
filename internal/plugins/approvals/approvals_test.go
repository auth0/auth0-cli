package approvals_test

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins/approvals"
	"github.com/auth0/auth0-cli/internal/plugins/filelock"
)

const (
	tenant = "acme.us.auth0.com"
	id     = "auth0/checkmate"
	hashA  = "sha256:" + hexA
	hashB  = "sha256:" + hexB
	hexA   = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	hexB   = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	upper  = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
)

var approvedAt = time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

// removers are the functions that remove approvals, which drop the plugin and
// the tenant above.
var removers = map[string]func(*approvals.Approvals) error{
	"RemoveAll":    func(a *approvals.Approvals) error { return a.RemoveAll(id) },
	"RemoveTenant": func(a *approvals.Approvals) error { return a.RemoveTenant(tenant) },
}

// newApprovals returns approvals in a plugins directory that does not exist yet.
func newApprovals(t *testing.T) *approvals.Approvals {
	t.Helper()

	return &approvals.Approvals{
		Dir: filepath.Join(t.TempDir(), "plugins"),
		Now: func() time.Time { return approvedAt },
	}
}

// writeFile puts content in the approvals file with the permissions that the
// CLI gives it.
func writeFile(t *testing.T, a *approvals.Approvals, content string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(a.Dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(a.Dir, "approvals.json"), []byte(content), 0o600))
}

func TestApprove(t *testing.T) {
	tests := []struct {
		name   string
		tenant string
		id     string
		hash   string
		want   bool
	}{
		{name: "same tenant, plugin and hash", tenant: tenant, id: id, hash: hashA, want: true},
		{name: "upper case tenant", tenant: "ACME.us.Auth0.com", id: id, hash: hashA, want: true},
		{name: "another tenant", tenant: "other.us.auth0.com", id: id, hash: hashA},
		{name: "another hash", tenant: tenant, id: id, hash: hashB},
		{name: "another plugin", tenant: tenant, id: "auth0/other", hash: hashA},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a := newApprovals(t)
			require.NoError(t, a.Approve(t.Context(), tenant, id, hashA))

			approved, err := a.Approved(test.tenant, test.id, test.hash)

			require.NoError(t, err)
			assert.Equal(t, test.want, approved)
		})
	}

	t.Run("the tenant is folded when it is approved, too", func(t *testing.T) {
		a := newApprovals(t)
		require.NoError(t, a.Approve(t.Context(), "ACME.us.auth0.com", id, hashA))

		approved, err := a.Approved(tenant, id, hashA)

		require.NoError(t, err)
		assert.True(t, approved)
	})

	t.Run("a new hash replaces the old one", func(t *testing.T) {
		a := newApprovals(t)
		require.NoError(t, a.Approve(t.Context(), tenant, id, hashA))
		require.NoError(t, a.Approve(t.Context(), tenant, id, hashB))

		oldApproved, err := a.Approved(tenant, id, hashA)
		require.NoError(t, err)

		newApproved, err := a.Approved(tenant, id, hashB)
		require.NoError(t, err)

		assert.False(t, oldApproved)
		assert.True(t, newApproved)

		records, err := a.List(id)
		require.NoError(t, err)
		assert.Len(t, records, 1)
	})

	t.Run("the file is private and nothing else is left behind", func(t *testing.T) {
		a := newApprovals(t)
		require.NoError(t, a.Approve(t.Context(), tenant, id, hashA))

		info, err := os.Stat(filepath.Join(a.Dir, "approvals.json"))
		require.NoError(t, err)

		if info.Mode().Perm() != 0o600 && runtime.GOOS != "windows" {
			t.Errorf("mode is %v, want 0600", info.Mode().Perm())
		}

		assert.ElementsMatch(t, []string{"approvals.json", ".approvals.lock"}, dirNames(t, a.Dir))
	})
}

func TestApproveRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name   string
		tenant string
		id     string
		hash   string
	}{
		{name: "empty tenant", tenant: "", id: id, hash: hashA},
		{name: "tenant with a slash", tenant: "acme/evil", id: id, hash: hashA},
		{name: "tenant with a space", tenant: "acme .auth0.com", id: id, hash: hashA},
		{name: "tenant with a line break", tenant: "acme.auth0.com\n", id: id, hash: hashA},
		{name: "tenant with a character that folds to a letter", tenant: "ac\u212ae.auth0.com", id: id, hash: hashA},
		{name: "tenant that is too long", tenant: strings.Repeat("a", 254), id: id, hash: hashA},
		{name: "ID without a registry", tenant: tenant, id: "checkmate", hash: hashA},
		{name: "ID in upper case", tenant: tenant, id: "Auth0/checkmate", hash: hashA},
		{name: "ID with a path", tenant: tenant, id: "auth0/../checkmate", hash: hashA},
		{name: "hash without a prefix", tenant: tenant, id: id, hash: hexA},
		{name: "hash in upper case", tenant: tenant, id: id, hash: "sha256:" + upper},
		{name: "short hash", tenant: tenant, id: id, hash: "sha256:abc"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a := newApprovals(t)

			require.Error(t, a.Approve(t.Context(), test.tenant, test.id, test.hash))
			require.Error(t, a.Check(test.tenant, test.id, test.hash))

			_, err := os.Stat(a.Dir)
			assert.ErrorIs(t, err, os.ErrNotExist)
		})
	}
}

func TestCheck(t *testing.T) {
	a := newApprovals(t)
	require.NoError(t, a.Approve(t.Context(), tenant, id, hashA))

	tests := []struct {
		name        string
		tenant      string
		hash        string
		wantErr     bool
		wantChanged bool
	}{
		{name: "approved", tenant: tenant, hash: hashA},
		{name: "another hash was approved", tenant: tenant, hash: hashB, wantErr: true, wantChanged: true},
		{name: "another tenant", tenant: "other.us.auth0.com", hash: hashA, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := a.Check(test.tenant, id, test.hash)

			if !test.wantErr {
				require.NoError(t, err)
				return
			}

			var notApproved *approvals.NotApprovedError

			require.True(t, errors.As(err, &notApproved))
			assert.Equal(t, approvals.NotApprovedError{Tenant: test.tenant, ID: id, Changed: test.wantChanged}, *notApproved)
		})
	}
}

func TestList(t *testing.T) {
	a := newApprovals(t)

	for _, approval := range []struct{ tenant, id, hash string }{
		{"zeta.auth0.com", id, hashA},
		{"alpha.auth0.com", id, hashB},
		{"alpha.auth0.com", "auth0/other", hashA},
	} {
		require.NoError(t, a.Approve(t.Context(), approval.tenant, approval.id, approval.hash))
	}

	records, err := a.List(id)

	require.NoError(t, err)
	assert.Equal(t, []approvals.Record{
		{Tenant: "alpha.auth0.com", PermissionsHash: hashB, ApprovedAt: approvedAt},
		{Tenant: "zeta.auth0.com", PermissionsHash: hashA, ApprovedAt: approvedAt},
	}, records)

	records, err = newApprovals(t).List(id)

	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestUsable(t *testing.T) {
	tests := []struct {
		name    string
		tenant  string // Empty means the tenant of the other tests.
		content string // Empty leaves the plugins directory out.
		wantErr error
	}{
		{name: "a new store"},
		{name: "a valid table", content: document(nil)},
		{name: "an upper case tenant", tenant: "ACME.us.auth0.com", content: document(nil)},
		{name: "a damaged table", content: "not json", wantErr: approvals.ErrDamaged},
		{name: "a table with an unknown field", content: document(map[string]string{"extra": `, "note": "x"`}), wantErr: approvals.ErrDamaged},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a := newApprovals(t)

			if test.content != "" {
				writeFile(t, a, test.content)
			}

			err := a.Usable(cmp.Or(test.tenant, tenant))

			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
			} else {
				require.NoError(t, err)
			}

			if test.content == "" {
				_, err := os.Stat(a.Dir)
				assert.ErrorIs(t, err, os.ErrNotExist, "nothing is created")
			}
		})
	}

	t.Run("a tenant that is not a domain", func(t *testing.T) {
		require.ErrorContains(t, newApprovals(t).Usable("acme/evil"), "is not a tenant domain")
	})
}

// approveAll approves two plugins on two tenants.
func approveAll(t *testing.T, a *approvals.Approvals) {
	t.Helper()

	for _, name := range []string{tenant, "other.us.auth0.com"} {
		for _, pluginID := range []string{id, "auth0/other"} {
			require.NoError(t, a.Approve(t.Context(), name, pluginID, hashA))
		}
	}
}

func TestRemoveAll(t *testing.T) {
	a := newApprovals(t)
	approveAll(t, a)

	require.NoError(t, a.RemoveAll(id))

	for _, name := range []string{tenant, "other.us.auth0.com"} {
		removed, err := a.Approved(name, id, hashA)
		require.NoError(t, err)
		assert.False(t, removed)

		kept, err := a.Approved(name, "auth0/other", hashA)
		require.NoError(t, err)
		assert.True(t, kept)
	}
}

func TestRemoveTenant(t *testing.T) {
	a := newApprovals(t)
	approveAll(t, a)

	require.NoError(t, a.RemoveTenant("ACME.us.auth0.com"))

	for _, pluginID := range []string{id, "auth0/other"} {
		removed, err := a.Approved(tenant, pluginID, hashA)
		require.NoError(t, err)
		assert.False(t, removed)

		kept, err := a.Approved("other.us.auth0.com", pluginID, hashA)
		require.NoError(t, err)
		assert.True(t, kept)
	}
}

func TestRemoveWithoutFile(t *testing.T) {
	for name, remove := range removers {
		t.Run(name+" creates nothing when there is no plugins directory", func(t *testing.T) {
			a := newApprovals(t)

			require.NoError(t, remove(a))

			_, err := os.Stat(a.Dir)
			assert.ErrorIs(t, err, os.ErrNotExist)
		})

		t.Run(name+" creates nothing when there is no file", func(t *testing.T) {
			a := newApprovals(t)
			require.NoError(t, os.MkdirAll(a.Dir, 0o700))

			require.NoError(t, remove(a))

			assert.Empty(t, dirNames(t, a.Dir))
		})
	}
}

func TestRemoveWithAFileInPlaceOfTheDirectory(t *testing.T) {
	for name, remove := range removers {
		t.Run(name+" has nothing to remove and creates nothing", func(t *testing.T) {
			a := newApprovals(t)
			require.NoError(t, os.WriteFile(a.Dir, []byte("not a directory"), 0o600))

			require.NoError(t, remove(a))

			content, err := os.ReadFile(a.Dir)
			require.NoError(t, err)
			assert.Equal(t, "not a directory", string(content))
			assert.Equal(t, []string{"plugins"}, dirNames(t, filepath.Dir(a.Dir)))
		})
	}

	t.Run("Approve reports it", func(t *testing.T) {
		a := newApprovals(t)
		require.NoError(t, os.WriteFile(a.Dir, []byte("not a directory"), 0o600))

		require.ErrorContains(t, a.Approve(t.Context(), tenant, id, hashA), "failed to create the plugins directory")
	})
}

func TestDirectoryMustBeAbsolute(t *testing.T) {
	methods := map[string]func(*approvals.Approvals) error{
		"Approve":      func(a *approvals.Approvals) error { return a.Approve(t.Context(), tenant, id, hashA) },
		"Approved":     func(a *approvals.Approvals) error { return errOf(a.Approved(tenant, id, hashA)) },
		"Check":        func(a *approvals.Approvals) error { return a.Check(tenant, id, hashA) },
		"List":         func(a *approvals.Approvals) error { return errOf(a.List(id)) },
		"RemoveAll":    removers["RemoveAll"],
		"RemoveTenant": removers["RemoveTenant"],
	}

	for _, dir := range []string{"", "plugins", filepath.Join("..", "plugins")} {
		for name, call := range methods {
			t.Run(fmt.Sprintf("%s with %q", name, dir), func(t *testing.T) {
				// The working directory holds an approvals file that would approve, in
				// the place that a relative path points to.
				work := t.TempDir()
				t.Chdir(work)

				planted := filepath.Join(work, "approvals.json")
				if dir != "" {
					planted = filepath.Join(work, dir, "approvals.json")
					require.NoError(t, os.MkdirAll(filepath.Dir(planted), 0o700))
				}

				require.NoError(t, os.WriteFile(planted, []byte(document(nil)), 0o600))

				before := dirNames(t, filepath.Dir(planted))

				err := call(&approvals.Approvals{Dir: dir})

				require.ErrorContains(t, err, "is not an absolute path")
				assert.Equal(t, before, dirNames(t, filepath.Dir(planted)), "nothing is created")

				content, readErr := os.ReadFile(planted)
				require.NoError(t, readErr)
				assert.Equal(t, document(nil), string(content), "the file is neither read nor changed")
			})
		}
	}
}

func TestConcurrentApprovals(t *testing.T) {
	t.Run("no approval is lost", func(t *testing.T) {
		a := newApprovals(t)

		var wg sync.WaitGroup

		for n := range 8 {
			wg.Add(1)

			go func() {
				defer wg.Done()

				assert.NoError(t, a.Approve(t.Context(), tenant, fmt.Sprintf("auth0/plugin-%d", n), hashA))
			}()
		}

		wg.Wait()

		for n := range 8 {
			approved, err := a.Approved(tenant, fmt.Sprintf("auth0/plugin-%d", n), hashA)

			require.NoError(t, err)
			assert.True(t, approved, "plugin-%d", n)
		}
	})

	t.Run("removing while the store holds its lock and another process approves does not deadlock", func(t *testing.T) {
		a := newApprovals(t)
		approveAll(t, a)

		// The store holds its own lock while it calls RemoveAll.
		release, err := filelock.Acquire(t.Context(), filepath.Join(a.Dir, ".lock"), errors.New("busy"))
		require.NoError(t, err)

		defer release()

		done := make(chan error, 2)

		go func() { done <- a.RemoveAll(id) }()
		go func() { done <- a.Approve(t.Context(), tenant, "auth0/new", hashA) }()

		for range 2 {
			select {
			case err := <-done:
				require.NoError(t, err)
			case <-time.After(10 * time.Second):
				t.Fatal("RemoveAll or Approve is waiting for the lock of the store")
			}
		}
	})
}

func TestApproveWaitsForTheLock(t *testing.T) {
	a := newApprovals(t)
	require.NoError(t, os.MkdirAll(a.Dir, 0o700))

	release, err := filelock.Acquire(t.Context(), filepath.Join(a.Dir, ".approvals.lock"), errors.New("busy"))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	require.ErrorIs(t, a.Approve(ctx, tenant, id, hashA), approvals.ErrBusy)

	release()

	require.NoError(t, a.Approve(t.Context(), tenant, id, hashA))
}

// dirNames lists the names in a directory.
func dirNames(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	return names
}
