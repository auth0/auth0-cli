package approvals_test

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/approvals"
)

const (
	notEnforced = "Each command runs with only the permissions it needs. Hosts and files are declared by the publisher and are not enforced by the CLI.\n\n"
	firstFooter = "\n" + notEnforced
	firstHeader = "\nThis plugin asks to use your Auth0 tenant acme.us.auth0.com:\n\n"
)

func TestFirstApprovalScreen(t *testing.T) {
	tests := []struct {
		name    string
		request func(approvals.Request) approvals.Request // Nil keeps the request as it is.
		want    string
	}{
		{
			name: "everything the plugin asks for",
			want: firstHeader +
				"  Can read     clients, connections, tenant settings\n" +
				"  Can change   clients (update), connections (update)\n" +
				"  Hosts        api.github.com    Check SDK versions\n" +
				"  Files        $CWD (write)    Save the report\n" +
				"  Environment  GITHUB_TOKEN    Call the GitHub API\n" +
				"  Interactive  no\n" + firstFooter,
		},
		{
			name: "an interactive command is flagged",
			request: func(r approvals.Request) approvals.Request {
				r.New = plugins.Permissions{Scopes: permissions.Scopes[:3]}
				r.NewCommands = []plugins.ConsentCommand{{Name: "a"}, {Name: "b", Interactive: true}}

				return r
			},
			want: firstHeader +
				"  Can read     clients, connections, tenant settings\n" +
				"  Interactive  yes\n" + firstFooter,
		},
		{
			name: "several hosts line up their reasons",
			request: func(r approvals.Request) approvals.Request {
				r.New = plugins.Permissions{Hosts: []plugins.HostPermission{{Host: "a.io", Reason: "One"}, {Host: "longer.example.com", Reason: "Two"}}}

				return r
			},
			want: firstHeader +
				"  Auth0 API    none\n" +
				"  Hosts        a.io                  One\n" +
				"               longer.example.com    Two\n" +
				"  Interactive  no\n" + firstFooter,
		},
		{
			name: "control characters from the manifest cannot rewrite the screen",
			request: func(r approvals.Request) approvals.Request {
				r.New = plugins.Permissions{Hosts: []plugins.HostPermission{{Host: "a.io", Reason: "\x1b[2Jok\r\n  Interactive  yes"}}}

				return r
			},
			want: firstHeader +
				"  Auth0 API    none\n" +
				"  Hosts        a.io    �[2Jok�� Interactive yes\n" +
				"  Interactive  no\n" + firstFooter,
		},
		{
			name: "a long run of white space cannot push the rows off the screen or forge one",
			request: func(r approvals.Request) approvals.Request {
				r.New = plugins.Permissions{Hosts: []plugins.HostPermission{{Host: "a.io", Reason: strings.Repeat(" ", 8000) + "Scopes       read:clients"}}}

				return r
			},
			want: firstHeader +
				"  Auth0 API    none\n" +
				"  Hosts        a.io    Scopes read:clients\n" +
				"  Interactive  no\n" + firstFooter,
		},
		{
			name: "white space of any kind collapses to one space",
			request: func(r approvals.Request) approvals.Request {
				r.New = plugins.Permissions{Hosts: []plugins.HostPermission{{Host: "a.io", Reason: "  Check \u00a0 SDK\u3000\u2003versions  "}}}

				return r
			},
			want: firstHeader +
				"  Auth0 API    none\n" +
				"  Hosts        a.io    Check SDK versions\n" +
				"  Interactive  no\n" + firstFooter,
		},
		{
			name: "a reason and a file path are cut after 200 runes, which are not bytes",
			request: func(r approvals.Request) approvals.Request {
				r.New = plugins.Permissions{
					Hosts: []plugins.HostPermission{{Host: "a.io", Reason: strings.Repeat("é", 201)}},
					Files: []plugins.FilePermission{{Path: "$CWD/" + strings.Repeat("y", 300), Access: plugins.FileAccessRead, Reason: strings.Repeat("é", 200)}},
				}

				return r
			},
			want: firstHeader +
				"  Auth0 API    none\n" +
				"  Hosts        a.io    " + strings.Repeat("é", 200) + "…\n" +
				"  Files        $CWD/" + strings.Repeat("y", 195) + "… (read)    " + strings.Repeat("é", 200) + "\n" +
				"  Interactive  no\n" + firstFooter,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, _, out := newConsenter(t, person, "n")

			req := request
			if test.request != nil {
				req = test.request(request)
			}

			require.ErrorIs(t, c.Ask(t.Context(), req), approvals.ErrDeclined)
			assert.Equal(t, test.want, out.String())
		})
	}
}

// askScreens asks for the scopes with the answers, and returns what was written.
func askScreens(t *testing.T, scopes []string, lines ...string) (string, []string) {
	t.Helper()

	c, prompter, out := newConsenter(t, person, lines...)

	req := request
	req.New = plugins.Permissions{Scopes: scopes}
	req.NewCommands = nil

	require.ErrorIs(t, c.Ask(t.Context(), req), approvals.ErrDeclined)

	return out.String(), prompter.prompts
}

func TestScopeGrouping(t *testing.T) {
	// A long list is shown in full, with no cut and no hint to press d.
	var many, manyWant []string

	for n := range 12 {
		many = append(many, fmt.Sprintf("read:resource_%02d", n))
		manyWant = append(manyWant, fmt.Sprintf("resource %02d", n))
	}

	tests := []struct {
		name   string
		scopes []string
		want   string // The rows between the header and the Interactive row.
	}{
		{
			name:   "read only",
			scopes: []string{"read:tenant_settings", "read:clients"},
			want:   "  Can read     clients, tenant settings\n",
		},
		{
			name:   "change only",
			scopes: []string{"update:clients", "create:clients", "delete:clients"},
			want:   "  Can change   clients (create, update, delete)\n",
		},
		{
			name:   "mixed",
			scopes: []string{"read:clients", "read:tenant_settings", "create:clients", "update:clients", "delete:clients"},
			want: "  Can read     clients, tenant settings\n" +
				"  Can change   clients (create, update, delete)\n",
		},
		{
			name:   "verbs other than create, update and delete follow in alphabetical order",
			scopes: []string{"deploy:actions", "create:actions", "block:actions", "delete:users", "update:users", "create:users", "revoke:users"},
			want:   "  Can change   actions (create, block, deploy), users (create, update, delete, revoke)\n",
		},
		{
			name:   "an underscore is shown as a space",
			scopes: []string{"read:client_grants", "update:email_templates"},
			want: "  Can read     client grants\n" +
				"  Can change   email templates (update)\n",
		},
		{
			name:   "the same scope twice is shown once",
			scopes: []string{"read:clients", "read:clients", "update:logs", "update:logs"},
			want: "  Can read     clients\n" +
				"  Can change   logs (update)\n",
		},
		{
			name:   "only the first colon splits a scope",
			scopes: []string{"read:a:b"},
			want:   "  Can read     a:b\n",
		},
		{
			name:   "scopes that do not parse go to Other",
			scopes: []string{"read:clients", "admin", ":clients", "read:", "update:"},
			want: "  Can read     clients\n" +
				"  Other        :clients, admin, read:, update:\n",
		},
		{
			name:   "no scopes",
			scopes: nil,
			want:   "  Auth0 API    none\n",
		},
		{
			name:   "a long list is not cut",
			scopes: many,
			want:   "  Can read     " + strings.Join(manyWant, ", ") + "\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, _ := askScreens(t, test.scopes, "n")

			assert.Equal(t, firstHeader+test.want+"  Interactive  no\n"+firstFooter, got)
			assert.NotContains(t, got, "  Scopes ")
		})
	}
}

func TestExactScopesAreShownOnlyAfterD(t *testing.T) {
	scopes := []string{"read:clients", "update:clients", "weird", "read:tenant_settings"}
	block := "\n  Exact scopes read:clients, read:tenant_settings, update:clients, weird\n\n"

	const (
		first = "Approve for acme.us.auth0.com? [y/N/d]  (d shows the exact scopes) "
		plain = "Approve for acme.us.auth0.com? [y/N] "
	)

	t.Run("the default screen has no exact scopes and offers d", func(t *testing.T) {
		got, prompts := askScreens(t, scopes, "n")

		assert.NotContains(t, got, "Exact scopes")
		assert.Equal(t, []string{first}, prompts)
	})

	t.Run("d prints only the block, without the screen again, and offers no d", func(t *testing.T) {
		got, prompts := askScreens(t, scopes, "d", "n")

		assert.Equal(t, []string{first, plain}, prompts)
		assert.Equal(t, 1, strings.Count(got, "This plugin asks to use your Auth0 tenant"), "the screen is not printed again")
		assert.Equal(t, 1, strings.Count(got, notEnforced))

		screen, _ := askScreens(t, scopes, "n")
		assert.Equal(t, screen+block, got, "the block follows the screen and nothing else is written")
	})

	t.Run("a second d declines", func(t *testing.T) {
		got, prompts := askScreens(t, scopes, "d", "d", "y")

		assert.Equal(t, []string{first, plain}, prompts)
		assert.Equal(t, 1, strings.Count(got, "Exact scopes"))
	})

	t.Run("end of input after d declines", func(t *testing.T) {
		got, prompts := askScreens(t, scopes, "d")

		assert.Equal(t, []string{first, plain}, prompts)
		assert.Equal(t, 1, strings.Count(got, "Exact scopes"))
	})

	t.Run("no scopes, no d", func(t *testing.T) {
		got, prompts := askScreens(t, nil, "d", "y")

		assert.Equal(t, []string{plain}, prompts, "d is not offered, and it declines as any answer but yes")
		assert.Equal(t, 1, strings.Count(got, "This plugin asks to use your Auth0 tenant"))
		assert.NotContains(t, got, "Exact scopes")
	})
}

// exactBlock returns the lines of the block that d printed, without the blank
// lines around it.
func exactBlock(t *testing.T, got string) []string {
	t.Helper()

	parts := strings.SplitAfter(got, notEnforced)
	require.Len(t, parts, 2)
	require.True(t, strings.HasPrefix(parts[1], "\n") && strings.HasSuffix(parts[1], "\n\n"), "a blank line before and after: %q", parts[1])

	return strings.Split(strings.TrimSuffix(strings.TrimPrefix(parts[1], "\n"), "\n\n"), "\n")
}

func TestExactScopesWrap(t *testing.T) {
	const valueColumn = 2 + 12 + 1

	indent := strings.Repeat(" ", valueColumn)

	// Check asserts that the lines hold the scopes in order, and that the lines wrap
	// as described. A scope that does not fit in a line is alone on its line.
	check := func(t *testing.T, lines []string, label string, want []string) {
		t.Helper()

		var got []string

		for n, line := range lines {
			if n == 0 {
				require.True(t, strings.HasPrefix(line, "  "+label+" "), line)
			} else {
				require.True(t, strings.HasPrefix(line, indent) && line[valueColumn] != ' ', "continuation indent: %q", line)
			}

			fields := strings.Fields(line)
			if n == 0 {
				fields = fields[len(strings.Fields(label)):]
			}

			if utf8.RuneCountInString(line) > 80 {
				assert.Len(t, fields, 1, "only a scope that is wider than a line is over 80 columns: %q", line)
			}

			for _, field := range fields {
				got = append(got, strings.TrimSuffix(field, ","))
			}

			assert.Equal(t, n < len(lines)-1, strings.HasSuffix(line, ","), "a comma ends every line but the last: %q", line)
		}

		assert.Equal(t, want, got)
	}

	t.Run("a long list wraps between scopes", func(t *testing.T) {
		scopes := make([]string, 0, 12)
		for n := range 12 {
			scopes = append(scopes, fmt.Sprintf("read:resource_%02d", n))
		}

		got, _ := askScreens(t, scopes, "d", "n")
		lines := exactBlock(t, got)

		assert.Greater(t, len(lines), 1)
		check(t, lines, "Exact scopes", scopes)
		assert.Equal(t, "  Exact scopes read:resource_00, read:resource_01, read:resource_02,", lines[0])
	})

	t.Run("a line takes 80 columns and no more", func(t *testing.T) {
		// The value has 65 columns. A scope of 31 columns with its comma takes 32, so two
		// of them and the space between fill a line to its 80th column, and a third
		// scope, or a second one that is a column longer, starts the next line.
		fits := []string{"a:" + strings.Repeat("x", 29), "b:" + strings.Repeat("x", 29), "c:y"}

		got, _ := askScreens(t, fits, "d", "n")
		lines := exactBlock(t, got)

		require.Len(t, lines, 2)
		assert.Equal(t, 80, utf8.RuneCountInString(lines[0]))
		check(t, lines, "Exact scopes", fits)

		over := []string{"a:" + strings.Repeat("x", 29), "b:" + strings.Repeat("x", 30), "c:y"}

		got, _ = askScreens(t, over, "d", "n")
		lines = exactBlock(t, got)

		assert.Equal(t, []string{"  Exact scopes " + over[0] + ",", indent + over[1] + ", " + over[2]}, lines)
		check(t, lines, "Exact scopes", over)
	})

	t.Run("a scope that is wider than a line is not split", func(t *testing.T) {
		long := "read:" + strings.Repeat("x", 90)
		scopes := []string{"a:b", long, "z:y"}

		got, _ := askScreens(t, scopes, "d", "n")
		lines := exactBlock(t, got)

		assert.Equal(t, []string{"  Exact scopes a:b,", indent + long + ",", indent + "z:y"}, lines)
	})

	t.Run("the rows of an update wrap in the same way", func(t *testing.T) {
		added := make([]string, 0, 10)
		for n := range 10 {
			added = append(added, fmt.Sprintf("create:resource_%02d", n))
		}

		before := plugins.Permissions{Scopes: []string{"read:clients"}}
		change := approvals.Request{ID: id, Version: "1.1.0", Old: &before, New: plugins.Permissions{Scopes: append([]string{"read:clients"}, added...)}}

		c, _, out := newConsenter(t, person, "d", "n")
		require.NoError(t, c.Approvals.Approve(t.Context(), tenant, id, plugins.ConsentHash(plugins.NormalizePermissions(before), nil)))
		require.ErrorIs(t, c.Ask(t.Context(), change), approvals.ErrDeclined)

		lines := exactBlock(t, out.String())

		assert.Greater(t, len(lines), 1)
		check(t, lines, "Added", added)
	})
}

func TestUpdateScreen(t *testing.T) {
	old := plugins.Permissions{
		Scopes: []string{"read:clients", "read:logs"},
		Hosts:  []plugins.HostPermission{{Host: "api.github.com", Reason: "Check SDK versions"}, {Host: "old.example.com", Reason: "Old"}},
		Files:  []plugins.FilePermission{{Path: "$CWD", Access: plugins.FileAccessWrite, Reason: "Save it"}},
		Env:    []plugins.EnvPermission{{Name: "GITHUB_TOKEN", Reason: "Call the GitHub API"}},
	}
	oldCommands := []plugins.ConsentCommand{{Name: "checkmate"}, {Name: "gone"}, {Name: "toggle"}}
	oldHash := plugins.ConsentHash(plugins.NormalizePermissions(old), oldCommands)

	updated := approvals.Request{
		ID:      id,
		Version: "1.1.0",
		Old:     &old,
		New: plugins.Permissions{
			Scopes: []string{"read:clients", "read:connections"},
			Hosts:  []plugins.HostPermission{{Host: "api.github.com", Reason: "Check SDK versions"}, {Host: "new.example.com", Reason: "New"}},
			Files:  []plugins.FilePermission{{Path: "$CWD", Access: plugins.FileAccessWrite, Reason: "Save the report"}},
			Env:    []plugins.EnvPermission{{Name: "GITHUB_TOKEN", Reason: "A new reason"}},
		},
		OldCommands: oldCommands,
		NewCommands: []plugins.ConsentCommand{{Name: "checkmate"}, {Name: "fresh", Interactive: true}, {Name: "toggle", Interactive: true}},
	}

	t.Run("shows only what was added, removed or changed", func(t *testing.T) {
		c, _, out := newConsenter(t, person, "n")
		require.NoError(t, c.Approvals.Approve(t.Context(), tenant, id, oldHash))

		require.ErrorIs(t, c.Ask(t.Context(), updated), approvals.ErrDeclined)
		assert.Equal(t, "\nauth0/checkmate 1.1.0 asks for different permissions on tenant acme.us.auth0.com\n"+
			"  Added        can read connections\n"+
			"  Removed      can read logs\n"+
			"  Added        host new.example.com    New\n"+
			"  Removed      host old.example.com    Old\n"+
			"  Changed      file $CWD (write)    Save the report\n"+
			"  Changed      variable GITHUB_TOKEN    A new reason\n"+
			"  Added        command fresh (interactive)\n"+
			"  Removed      command gone\n"+
			"  Changed      command toggle is now interactive\n"+
			notEnforced, out.String())
	})

	t.Run("scopes use the same groups as the first screen", func(t *testing.T) {
		before := plugins.Permissions{Scopes: []string{"read:clients", "read:logs", "update:clients", "delete:clients"}}
		after := plugins.Permissions{Scopes: []string{"read:clients", "read:tenant_settings", "update:clients", "create:clients", "deploy:actions", "odd"}}
		change := approvals.Request{ID: id, Version: "1.1.0", Old: &before, New: after}

		const header = "\nauth0/checkmate 1.1.0 asks for different permissions on tenant acme.us.auth0.com\n"

		t.Run("groups", func(t *testing.T) {
			c, _, out := newConsenter(t, person, "n")
			require.NoError(t, c.Approvals.Approve(t.Context(), tenant, id, plugins.ConsentHash(plugins.NormalizePermissions(before), nil)))

			require.ErrorIs(t, c.Ask(t.Context(), change), approvals.ErrDeclined)
			assert.Equal(t, header+
				"  Added        can read tenant settings\n"+
				"  Added        can change actions (deploy), clients (create)\n"+
				"  Added        other scopes odd\n"+
				"  Removed      can read logs\n"+
				"  Removed      can change clients (delete)\n"+
				notEnforced, out.String())
		})

		t.Run("d prints the scopes that were added and removed as they are", func(t *testing.T) {
			c, prompter, out := newConsenter(t, person, "d", "n")
			require.NoError(t, c.Approvals.Approve(t.Context(), tenant, id, plugins.ConsentHash(plugins.NormalizePermissions(before), nil)))

			require.ErrorIs(t, c.Ask(t.Context(), change), approvals.ErrDeclined)

			assert.Equal(t, []string{
				"Approve for acme.us.auth0.com? [y/N/d]  (d shows the exact scopes) ",
				"Approve for acme.us.auth0.com? [y/N] ",
			}, prompter.prompts)

			parts := strings.SplitAfter(out.String(), notEnforced)
			require.Len(t, parts, 2, "the screen is printed once")
			assert.Equal(t, "\n"+
				"  Added        create:clients, deploy:actions, odd, read:tenant_settings\n"+
				"  Removed      delete:clients, read:logs\n"+
				"\n", parts[1])
			assert.NotContains(t, out.String(), "Exact scopes")
		})

		t.Run("d leaves out the row that is empty", func(t *testing.T) {
			only := approvals.Request{ID: id, Version: "1.1.0", Old: &before, New: plugins.Permissions{Scopes: append([]string{"read:billing"}, before.Scopes...)}}

			c, _, out := newConsenter(t, person, "d", "n")
			require.NoError(t, c.Approvals.Approve(t.Context(), tenant, id, plugins.ConsentHash(plugins.NormalizePermissions(before), nil)))

			require.ErrorIs(t, c.Ask(t.Context(), only), approvals.ErrDeclined)
			assert.Equal(t, "\n  Added        read:billing\n\n", strings.SplitAfter(out.String(), notEnforced)[1])
		})

		t.Run("an update with no difference in scopes offers no d", func(t *testing.T) {
			hosts := approvals.Request{ID: id, Version: "1.1.0", Old: &before, New: plugins.Permissions{Scopes: before.Scopes, Hosts: []plugins.HostPermission{{Host: "a.io", Reason: "One"}}}}

			c, prompter, out := newConsenter(t, person, "d", "y")
			require.NoError(t, c.Approvals.Approve(t.Context(), tenant, id, plugins.ConsentHash(plugins.NormalizePermissions(before), nil)))

			require.ErrorIs(t, c.Ask(t.Context(), hosts), approvals.ErrDeclined)
			assert.Equal(t, []string{"Approve for acme.us.auth0.com? [y/N] "}, prompter.prompts)
			assert.Contains(t, out.String(), "asks for different permissions")
			assert.NotContains(t, out.String(), "Added        read")
		})
	})

	t.Run("an update with the same permissions shows the full screen", func(t *testing.T) {
		same := updated
		same.New, same.NewCommands = old, oldCommands

		c, _, out := newConsenter(t, person, "n")
		require.NoError(t, c.Approvals.Approve(t.Context(), tenant, id, oldHash))

		require.ErrorIs(t, c.Ask(t.Context(), same), approvals.ErrDeclined)
		assert.Contains(t, out.String(), "This plugin asks to use your Auth0 tenant acme.us.auth0.com")
		assert.NotContains(t, out.String(), "asks for different permissions")
		assert.NotContains(t, out.String(), "no difference")
	})

	t.Run("a removed access to a file is shown", func(t *testing.T) {
		both := plugins.Permissions{Files: []plugins.FilePermission{
			{Path: "$CWD", Access: plugins.FileAccessRead, Reason: "Read it"},
			{Path: "$CWD", Access: plugins.FileAccessWrite, Reason: "Save it"},
		}}
		fewer := approvals.Request{ID: id, Version: "1.1.0", Old: &both, New: plugins.Permissions{Files: both.Files[1:]}}

		c, _, out := newConsenter(t, person, "n")
		require.NoError(t, c.Approvals.Approve(t.Context(), tenant, id, plugins.ConsentHash(plugins.NormalizePermissions(both), nil)))

		require.ErrorIs(t, c.Ask(t.Context(), fewer), approvals.ErrDeclined)
		assert.Contains(t, out.String(), "  Removed      file $CWD (read)    Read it\n")
		assert.NotContains(t, out.String(), "no difference")
	})

	t.Run("a difference that shows no item is never reported as no difference", func(t *testing.T) {
		// Two entries for one host with other reasons count as one when they are
		// compared by host, so dropping one of them changes the hash and not the rows.
		twice := plugins.Permissions{Hosts: []plugins.HostPermission{{Host: "a.io", Reason: "One"}, {Host: "a.io", Reason: "Two"}}}
		once := plugins.Permissions{Hosts: twice.Hosts[1:]}
		fewer := approvals.Request{ID: id, Version: "1.1.0", Old: &twice, New: once}

		c, _, out := newConsenter(t, person, "n")
		require.NoError(t, c.Approvals.Approve(t.Context(), tenant, id, plugins.ConsentHash(plugins.NormalizePermissions(twice), nil)))

		require.ErrorIs(t, c.Ask(t.Context(), fewer), approvals.ErrDeclined)
		assert.Contains(t, out.String(), "This plugin asks to use your Auth0 tenant acme.us.auth0.com")
		assert.NotContains(t, out.String(), "no difference")
	})

	// The installed version is whatever is on this machine, which says nothing about
	// what the tenant approved. The difference is only shown to a tenant that
	// approved the installed version, and everything is shown to any other.
	tests := []struct {
		name      string
		setup     func(*testing.T, *approvals.Consenter)
		wantDiff  bool
		wantError error
	}{
		{name: "a tenant with no record sees everything", setup: func(*testing.T, *approvals.Consenter) {}},
		{name: "a tenant that approved another version sees everything", setup: func(t *testing.T, c *approvals.Consenter) {
			require.NoError(t, c.Approvals.Approve(t.Context(), tenant, id, hashA))
		}},
		{name: "a tenant that approved the installed version sees the difference", wantDiff: true, setup: func(t *testing.T, c *approvals.Consenter) {
			require.NoError(t, c.Approvals.Approve(t.Context(), tenant, id, oldHash))
		}},
		{name: "a pin of the installed version shows the difference", wantDiff: true, setup: func(_ *testing.T, c *approvals.Consenter) {
			c.Approvals.Pins = approvals.Pins{id: {oldHash}}
		}},
		{name: "a pin of another version sees everything", setup: func(_ *testing.T, c *approvals.Consenter) {
			c.Approvals.Pins = approvals.Pins{id: {hashA}}
		}},
		{name: "a damaged file stops the update", wantError: approvals.ErrDamaged, setup: func(t *testing.T, c *approvals.Consenter) {
			writeFile(t, c.Approvals, "not json")
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, _, out := newConsenter(t, person, "n")
			test.setup(t, c)

			err := c.Ask(t.Context(), updated)

			if test.wantError != nil {
				require.ErrorIs(t, err, test.wantError)
				assert.Zero(t, out.Len())

				return
			}

			require.ErrorIs(t, err, approvals.ErrDeclined)

			// The host and the scope exist in the old and the new version, so only the
			// full screen lists them.
			if test.wantDiff {
				assert.Contains(t, out.String(), "asks for different permissions")
				assert.NotContains(t, out.String(), "api.github.com")
				assert.NotContains(t, out.String(), "This plugin asks to use")

				return
			}

			assert.Contains(t, out.String(), "This plugin asks to use your Auth0 tenant acme.us.auth0.com")
			assert.Contains(t, out.String(), "api.github.com")
			assert.Contains(t, out.String(), "  Can read     clients, connections\n")
			assert.NotContains(t, out.String(), "asks for different permissions")
		})
	}
}

func TestInvisibleCharactersAreReplaced(t *testing.T) {
	const (
		bidi      = "\u202e"
		isolate   = "\u2066"
		zeroWidth = "\u200b"
		kept      = "für 日本語"
	)

	files := []plugins.FilePermission{{Path: "$CWD/evil" + bidi + "txt.exe", Access: plugins.FileAccessWrite, Reason: "Save" + isolate + " the" + zeroWidth + " report " + kept}}
	first := request
	first.New = plugins.Permissions{Files: files}

	old := plugins.Permissions{Scopes: []string{"read:clients"}}
	update := approvals.Request{ID: id, Version: "1.1.0", Old: &old, New: first.New}

	for header, req := range map[string]approvals.Request{"This plugin asks to use": first, "asks for different permissions": update} {
		t.Run(header, func(t *testing.T) {
			c, _, out := newConsenter(t, person, "n")
			require.NoError(t, c.Approvals.Approve(t.Context(), tenant, id, plugins.ConsentHash(plugins.NormalizePermissions(old), nil)))

			require.ErrorIs(t, c.Ask(t.Context(), req), approvals.ErrDeclined)
			require.Contains(t, out.String(), header)

			for _, character := range []string{bidi, isolate, zeroWidth} {
				assert.NotContains(t, out.String(), character)
			}

			assert.Contains(t, out.String(), "$CWD/evil\ufffdtxt.exe (write)")
			assert.Contains(t, out.String(), "Save\ufffd the\ufffd report "+kept)
		})
	}
}

func TestExactScopesAreCleaned(t *testing.T) {
	const (
		bidi      = "\u202e"
		zeroWidth = "\u200b"
	)

	scopes := []string{"read:cli" + bidi + "ents", "update:" + zeroWidth + "logs", "odd\x1b[2J\r\nInteractive  yes"}

	before := plugins.Permissions{Scopes: []string{"read:clients"}}
	update := approvals.Request{ID: id, Version: "1.1.0", Old: &before, New: plugins.Permissions{Scopes: scopes}}

	first := request
	first.New = plugins.Permissions{Scopes: scopes}

	for name, req := range map[string]approvals.Request{"first screen": first, "update": update} {
		t.Run(name, func(t *testing.T) {
			c, _, out := newConsenter(t, person, "d", "n")
			require.NoError(t, c.Approvals.Approve(t.Context(), tenant, id, plugins.ConsentHash(plugins.NormalizePermissions(before), nil)))

			require.ErrorIs(t, c.Ask(t.Context(), req), approvals.ErrDeclined)

			block := strings.SplitAfter(out.String(), notEnforced)[1]

			for _, character := range []string{bidi, zeroWidth, "\x1b", "\r"} {
				assert.NotContains(t, block, character)
			}

			assert.Equal(t, 1, strings.Count(block, "\n\n"), "no scope can forge a line or a blank line")
			assert.Contains(t, block, "read:cli\ufffdents,")
			assert.Contains(t, block, "update:\ufffdlogs\n")
		})
	}
}
