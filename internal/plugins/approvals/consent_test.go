package approvals_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/approvals"
	"github.com/auth0/auth0-cli/internal/plugins/store"
)

// answers is a Prompter that gives the lines it was given, and then io.EOF.
type answers struct {
	lines   []string
	err     error
	prompts []string
}

func (a *answers) ReadLine(prompt string) (string, error) {
	a.prompts = append(a.prompts, prompt)

	if a.err != nil {
		return "", a.err
	}

	if len(a.lines) == 0 {
		return "", io.EOF
	}

	line := a.lines[0]
	a.lines = a.lines[1:]

	return line, nil
}

var (
	person = approvals.Session{Interactive: true}

	permissions = plugins.Permissions{
		Scopes: []string{"read:clients", "read:connections", "read:tenant_settings", "update:clients", "update:connections"},
		Hosts:  []plugins.HostPermission{{Host: "api.github.com", Reason: "Check SDK versions"}},
		Files:  []plugins.FilePermission{{Path: "$CWD", Access: plugins.FileAccessWrite, Reason: "Save the report"}},
		Env:    []plugins.EnvPermission{{Name: "GITHUB_TOKEN", Reason: "Call the GitHub API"}},
	}
	commands = []plugins.ConsentCommand{{Name: "checkmate", Interactive: false}}
	request  = approvals.Request{ID: id, Version: "1.0.0", New: permissions, NewCommands: commands}

	// The consent hash of the permissions and commands above.
	consentHash = plugins.ConsentHash(permissions, commands)
)

// newConsenter returns a consenter whose person gives the lines.
func newConsenter(t *testing.T, session approvals.Session, lines ...string) (*approvals.Consenter, *answers, *bytes.Buffer) {
	t.Helper()

	prompter, out := &answers{lines: lines}, &bytes.Buffer{}

	return &approvals.Consenter{
		Tenant:    tenant,
		Approvals: newApprovals(t),
		Session:   session,
		Prompter:  prompter,
		Out:       out,
	}, prompter, out
}

// installed returns the results of installing version 1.0.0 of the plugins.
func installed(ids ...string) []store.Result {
	results := make([]store.Result, 0, len(ids))
	for _, pluginID := range ids {
		results = append(results, store.Result{ID: pluginID, Version: "1.0.0", Action: store.ActionInstalled})
	}

	return results
}

func approved(t *testing.T, c *approvals.Consenter, hash string) bool {
	t.Helper()

	ok, err := c.Approvals.Approved(tenant, id, hash)
	require.NoError(t, err)

	return ok
}

func TestAskRefusesWithoutAPerson(t *testing.T) {
	tests := []struct {
		name    string
		session approvals.Session
		want    error
	}{
		{name: "zero session", session: approvals.Session{}, want: approvals.ErrNotInteractive},
		{name: "agent without a terminal", session: approvals.Session{Agent: true}, want: approvals.ErrAgentMode},
		{name: "agent at a terminal", session: approvals.Session{Interactive: true, Agent: true}, want: approvals.ErrAgentMode},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for name, ask := range map[string]func(*approvals.Consenter) error{
				"Ask": func(c *approvals.Consenter) error { return c.Ask(t.Context(), request) },
				"ConsentFunc": func(c *approvals.Consenter) error {
					return c.ConsentFunc()(t.Context(), store.Change{ID: id, NewVersion: "1.0.0", New: permissions, NewCommands: commands})
				},
			} {
				c, prompter, out := newConsenter(t, test.session, "y")

				require.ErrorIs(t, ask(c), test.want, name)
				assert.Empty(t, prompter.prompts, name)
				assert.Zero(t, out.Len(), name)
				require.NoError(t, c.Record(t.Context()))

				_, err := os.Stat(c.Approvals.Dir)
				assert.ErrorIs(t, err, os.ErrNotExist, name)
			}
		})
	}

	t.Run("an invalid tenant is refused before anything is shown", func(t *testing.T) {
		c, prompter, out := newConsenter(t, person, "y")
		c.Tenant = "acme/evil"

		require.Error(t, c.Ask(t.Context(), request))
		assert.Empty(t, prompter.prompts)
		assert.Zero(t, out.Len())
	})
}

func TestAskAnswers(t *testing.T) {
	tests := []struct {
		name    string
		lines   []string
		err     error
		approve bool
	}{
		{name: "y", lines: []string{"y"}, approve: true},
		{name: "upper case Y", lines: []string{"Y"}, approve: true},
		{name: "yes", lines: []string{"yes"}, approve: true},
		{name: "YES with spaces", lines: []string{"  YES \n"}, approve: true},
		{name: "n", lines: []string{"n"}},
		{name: "empty line", lines: []string{""}},
		{name: "maybe", lines: []string{"maybe"}},
		{name: "yy", lines: []string{"yy"}},
		{name: "end of input", lines: nil},
		{name: "read error", err: errors.New("terminal closed")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, prompter, _ := newConsenter(t, person, test.lines...)
			prompter.err = test.err

			err := c.Ask(t.Context(), request)

			if test.approve {
				require.NoError(t, err)
				assert.True(t, approved(t, c, consentHash))
				assert.Equal(t, []string{"Approve for acme.us.auth0.com? [y/N/d]  (d shows the exact scopes) "}, prompter.prompts)

				return
			}

			require.ErrorIs(t, err, approvals.ErrDeclined)
			assert.False(t, approved(t, c, consentHash))

			_, statErr := os.Stat(c.Approvals.Dir)
			assert.ErrorIs(t, statErr, os.ErrNotExist, "a declined request writes nothing")

			if test.err != nil {
				assert.ErrorIs(t, err, test.err)
			}
		})
	}

	t.Run("d shows the exact scopes without the screen and asks again", func(t *testing.T) {
		c, prompter, out := newConsenter(t, person, "d", "y")

		require.NoError(t, c.Ask(t.Context(), request))
		assert.True(t, approved(t, c, consentHash))

		assert.Equal(t, []string{
			"Approve for acme.us.auth0.com? [y/N/d]  (d shows the exact scopes) ",
			"Approve for acme.us.auth0.com? [y/N] ",
		}, prompter.prompts)
		assert.Equal(t, 1, strings.Count(out.String(), "This plugin asks to use your Auth0 tenant"), "the screen is printed once")
		assert.True(t, strings.HasSuffix(out.String(), "\n  Exact scopes read:clients, read:connections, read:tenant_settings,\n"+
			"               update:clients, update:connections\n\n"))
	})

	t.Run("without scopes d is not offered and declines", func(t *testing.T) {
		none := request
		none.New = plugins.Permissions{Hosts: permissions.Hosts}

		c, prompter, out := newConsenter(t, person, "d", "y")

		require.ErrorIs(t, c.Ask(t.Context(), none), approvals.ErrDeclined)
		assert.Equal(t, []string{"Approve for acme.us.auth0.com? [y/N] "}, prompter.prompts)
		assert.NotContains(t, out.String(), "Exact scopes")
	})

	t.Run("end of input after d declines", func(t *testing.T) {
		c, prompter, _ := newConsenter(t, person, "d")

		require.ErrorIs(t, c.Ask(t.Context(), request), approvals.ErrDeclined)
		assert.Len(t, prompter.prompts, 2)
		assert.False(t, approved(t, c, consentHash))
	})

	t.Run("d works once, so a reader that always answers d cannot loop", func(t *testing.T) {
		c, prompter, _ := newConsenter(t, person, "d", "d", "d", "y")

		require.ErrorIs(t, c.Ask(t.Context(), request), approvals.ErrDeclined)
		assert.Len(t, prompter.prompts, 2)
	})
}

func TestAskApprovesWhatWasShown(t *testing.T) {
	c, _, _ := newConsenter(t, person, "y")

	unsorted := request
	unsorted.New.Scopes = []string{"update:connections", "read:clients", "read:tenant_settings", "update:clients", "read:connections"}

	require.NoError(t, c.Ask(t.Context(), unsorted))

	assert.True(t, approved(t, c, consentHash))
}

func TestConsentFunc(t *testing.T) {
	change := store.Change{ID: id, NewVersion: "1.0.0", New: permissions, NewCommands: commands}

	t.Run("an approved hash needs no person and no prompt", func(t *testing.T) {
		c, prompter, out := newConsenter(t, approvals.Session{})
		require.NoError(t, c.Approvals.Approve(t.Context(), tenant, id, consentHash))

		require.NoError(t, c.ConsentFunc()(t.Context(), change))
		assert.Empty(t, prompter.prompts)
		assert.Zero(t, out.Len())
	})

	t.Run("a pinned hash needs no person and no prompt", func(t *testing.T) {
		c, prompter, out := newConsenter(t, approvals.Session{Agent: true})
		c.Approvals.Pins = approvals.Pins{id: {consentHash}}

		require.NoError(t, c.ConsentFunc()(t.Context(), change))
		assert.Empty(t, prompter.prompts)
		assert.Zero(t, out.Len())
	})

	t.Run("a damaged file stops the install", func(t *testing.T) {
		c, prompter, _ := newConsenter(t, person, "y")
		writeFile(t, c.Approvals, "not json")

		require.ErrorIs(t, c.ConsentFunc()(t.Context(), change), approvals.ErrDamaged)
		assert.Empty(t, prompter.prompts)
	})

	t.Run("the approval is written by Record and not before", func(t *testing.T) {
		c, _, _ := newConsenter(t, person, "y")

		require.NoError(t, c.ConsentFunc()(t.Context(), change))
		assert.False(t, approved(t, c, consentHash), "a failed install must leave no approval")

		require.NoError(t, c.Record(t.Context(), installed(id)...))
		assert.True(t, approved(t, c, consentHash))

		records, err := c.Approvals.List(id)
		require.NoError(t, err)
		require.Len(t, records, 1)
		assert.Equal(t, consentHash, records[0].PermissionsHash)
	})

	t.Run("a decline stops the install and records nothing", func(t *testing.T) {
		c, _, _ := newConsenter(t, person, "n")

		require.ErrorIs(t, c.ConsentFunc()(t.Context(), change), approvals.ErrDeclined)
		require.NoError(t, c.Record(t.Context(), installed(id)...))
		assert.False(t, approved(t, c, consentHash))
	})

	t.Run("an earlier yes does not outlive a later no for the same plugin", func(t *testing.T) {
		c, _, _ := newConsenter(t, person, "y", "n")
		consent := c.ConsentFunc()

		require.NoError(t, consent(t.Context(), change))
		require.ErrorIs(t, consent(t.Context(), change), approvals.ErrDeclined)
		require.NoError(t, c.Record(t.Context(), installed(id)...))
		assert.False(t, approved(t, c, consentHash))
	})

	t.Run("Record writes each approval once", func(t *testing.T) {
		c, _, _ := newConsenter(t, person, "y")
		require.NoError(t, c.ConsentFunc()(t.Context(), change))
		require.NoError(t, c.Record(t.Context(), installed(id)...))
		require.NoError(t, c.Approvals.RemoveAll(id))
		require.NoError(t, c.Record(t.Context(), installed(id)...))

		assert.False(t, approved(t, c, consentHash))
	})

	t.Run("Record approves only the plugins it names and discards the others", func(t *testing.T) {
		other := change
		other.ID = "auth0/other"

		granted := func(t *testing.T, c *approvals.Consenter) {
			t.Helper()

			consent := c.ConsentFunc()
			require.NoError(t, consent(t.Context(), change))
			require.NoError(t, consent(t.Context(), other))
		}

		tests := []struct {
			name       string
			results    []store.Result
			wantFirst  bool
			wantSecond bool
		}{
			{name: "no results"},
			{name: "a plugin that was not granted", results: installed("auth0/unknown")},
			{name: "one of two", results: installed(id), wantFirst: true},
			{name: "the other one, with an unknown ID and a repeat", results: installed("auth0/unknown", other.ID, other.ID), wantSecond: true},
			{name: "both", results: installed(other.ID, id), wantFirst: true, wantSecond: true},
			{name: "the same ID at another version", results: []store.Result{{ID: id, Version: "1.0.1"}, {ID: other.ID, Version: "1.0.0"}}, wantSecond: true},
			{name: "an empty result, as a skipped or failed update has", results: []store.Result{{}}},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				c, _, _ := newConsenter(t, person, "y", "y")
				granted(t, c)

				require.NoError(t, c.Record(t.Context(), test.results...))
				require.NoError(t, c.Record(t.Context(), installed(id, other.ID)...), "what was not named is gone for good")

				first, err := c.Approvals.Approved(tenant, id, consentHash)
				require.NoError(t, err)
				assert.Equal(t, test.wantFirst, first)

				second, err := c.Approvals.Approved(tenant, other.ID, consentHash)
				require.NoError(t, err)
				assert.Equal(t, test.wantSecond, second)
			})
		}
	})

	t.Run("Record writes for the tenant the person was asked about", func(t *testing.T) {
		c, _, _ := newConsenter(t, person, "y")
		require.NoError(t, c.ConsentFunc()(t.Context(), change))

		c.Tenant = "other.us.auth0.com"
		require.NoError(t, c.Record(t.Context(), installed(id)...))

		assert.True(t, approved(t, c, consentHash), "the tenant of the grant")

		ok, err := c.Approvals.Approved("other.us.auth0.com", id, consentHash)
		require.NoError(t, err)
		assert.False(t, ok, "the tenant of Record")
	})

	t.Run("Record with nothing to write creates nothing", func(t *testing.T) {
		c, _, _ := newConsenter(t, person)

		require.NoError(t, c.Record(t.Context()))

		_, err := os.Stat(c.Approvals.Dir)
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	update := change
	update.OldVersion = "0.9.0"
	update.Old = plugins.Permissions{Scopes: permissions.Scopes[:4], Hosts: permissions.Hosts, Files: permissions.Files, Env: permissions.Env}
	update.OldCommands = commands

	t.Run("an update of a version the tenant approved shows the difference", func(t *testing.T) {
		c, _, out := newConsenter(t, person, "y")
		require.NoError(t, c.Approvals.Approve(t.Context(), tenant, id, plugins.ConsentHash(plugins.NormalizePermissions(update.Old), commands)))

		require.NoError(t, c.ConsentFunc()(t.Context(), update))

		assert.Contains(t, out.String(), "auth0/checkmate 1.0.0 asks for different permissions on tenant acme.us.auth0.com")
		assert.Contains(t, out.String(), "Added        can change connections (update)")
		assert.NotContains(t, out.String(), "api.github.com")
	})

	t.Run("an update of a version the tenant never approved shows everything", func(t *testing.T) {
		c, _, out := newConsenter(t, person, "y")

		require.NoError(t, c.ConsentFunc()(t.Context(), update))

		assert.Contains(t, out.String(), "This plugin asks to use your Auth0 tenant acme.us.auth0.com")
		assert.Contains(t, out.String(), "api.github.com")
	})
}
