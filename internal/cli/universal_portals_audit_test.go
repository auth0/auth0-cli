package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/portals"
)

// auditRun is a run of the audit command on a fake tenant that holds a portal made by setup.
type auditRun struct {
	*setupRun
	portalID string
	clientID string
}

func newAuditRun(t *testing.T, json bool) *auditRun {
	t.Helper()

	setup := newSetupRun(json)
	setup.tenant.AddConnection(portals.Connection{ID: "con_db", Name: "Username-Password-Authentication"}, "auth0")

	require.NoError(t, setup.run(consumerInputs))

	setup.messages.Reset()
	setup.results.Reset()

	portal := setup.tenant.Portals()[0]
	setup.tenant.EnableClientConnection(portal.Client.ClientID, "con_db")

	return &auditRun{setupRun: setup, portalID: portal.ID, clientID: portal.Client.ClientID}
}

func (r *auditRun) audit(inputs universalPortalsAuditInputs) error {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	return runUniversalPortalsAuditOn(cmd, r.cli, r.portalID, inputs, r.tenant)
}

func TestAuditCommand_APortalThatPasses(t *testing.T) {
	t.Parallel()

	t.Run("JSON is the result, with the fields and the order of the tool this replaces", func(t *testing.T) {
		t.Parallel()

		run := newAuditRun(t, true)

		require.NoError(t, run.audit(universalPortalsAuditInputs{}))

		var reindented bytes.Buffer
		require.NoError(t, json.Indent(&reindented, run.results.Bytes(), "", "  "))
		assert.Equal(t, "{\n  \"portalId\": \""+run.portalID+"\",\n  \"findings\": [],\n  \"errors\": 0,\n  \"warnings\": 0,\n  \"failed\": false\n}", strings.TrimSpace(reindented.String()))
		assert.Empty(t, run.messages.String(), "the result is alone on its stream")
	})

	t.Run("a person sees the box, the portal and the summary", func(t *testing.T) {
		t.Parallel()

		run := newAuditRun(t, false)

		require.NoError(t, run.audit(universalPortalsAuditInputs{}))

		lines := strings.Split(run.messages.String(), "\n")
		assert.Equal(t, "│ ╭─Universal Portals audit──╮", lines[0])
		assert.Contains(t, run.messages.String(), `◇  Portal "acme-account"`)
		assert.Contains(t, run.messages.String(), "No problems found")
		assert.Contains(t, run.messages.String(), "0 errors, 0 warnings")
		assert.Empty(t, run.results.String())
	})
}

func TestAuditCommand_APortalThatFails(t *testing.T) {
	t.Parallel()

	breakIt := func(run *auditRun) {
		run.tenant.ModifyClient(run.clientID, func(client *portals.Client) { client.RefreshToken.RotationType = "rotating" })
	}

	t.Run("it exits with a validation error, and still prints the result", func(t *testing.T) {
		t.Parallel()

		run := newAuditRun(t, true)
		breakIt(run)

		err := run.audit(universalPortalsAuditInputs{})

		require.Error(t, err)
		assert.Equal(t, "the audit failed: 1 error(s)", err.Error())
		assert.Equal(t, "validation", errorClass(err))
		assert.Equal(t, "audit_failed", errorReason(err))

		var printed portals.AuditResult
		require.NoError(t, json.Unmarshal(run.results.Bytes(), &printed))
		assert.True(t, printed.Failed)
		assert.Equal(t, "P013", printed.Findings[0].ID)
	})

	t.Run("--fix fixes it and the audit passes", func(t *testing.T) {
		t.Parallel()

		run := newAuditRun(t, true)
		breakIt(run)

		require.NoError(t, run.audit(universalPortalsAuditInputs{Fix: true}))

		var printed portals.AuditResult
		require.NoError(t, json.Unmarshal(run.results.Bytes(), &printed))
		assert.Empty(t, printed.Findings)

		client, err := run.tenant.ReadClient(context.Background(), run.clientID)
		require.NoError(t, err)
		assert.Equal(t, "non-rotating", client.RefreshToken.RotationType)
	})

	t.Run("a fix that failed is why it failed", func(t *testing.T) {
		t.Parallel()

		run := newAuditRun(t, true)
		breakIt(run)
		run.tenant.Deny("PATCH clients/"+run.clientID, "Insufficient scope, expected any of: update:clients")

		err := run.audit(universalPortalsAuditInputs{Fix: true})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "1 error(s), 1 fix(es) failed")
	})
}

func TestAuditCommand_Warnings(t *testing.T) {
	t.Parallel()

	warn := func(run *auditRun) {
		run.tenant.ModifyClient(run.clientID, func(client *portals.Client) { client.IPHeaderTrusted = boolPointer(false) })
	}

	t.Run("a warning does not fail the audit", func(t *testing.T) {
		t.Parallel()

		run := newAuditRun(t, true)
		warn(run)

		require.NoError(t, run.audit(universalPortalsAuditInputs{}))
	})

	t.Run("a warning fails a strict audit", func(t *testing.T) {
		t.Parallel()

		run := newAuditRun(t, true)
		warn(run)

		err := run.audit(universalPortalsAuditInputs{Strict: true})

		require.Error(t, err)
		assert.Equal(t, "the audit failed: 1 warning(s) in a strict audit", err.Error())
	})

	t.Run("--fix does not change a warning", func(t *testing.T) {
		t.Parallel()

		run := newAuditRun(t, true)
		warn(run)

		require.NoError(t, run.audit(universalPortalsAuditInputs{Fix: true}))

		client, err := run.tenant.ReadClient(context.Background(), run.clientID)
		require.NoError(t, err)
		assert.False(t, *client.IPHeaderTrusted)
	})
}

func TestAuditCommand_Errors(t *testing.T) {
	t.Parallel()

	t.Run("a portal that does not exist is a not found error", func(t *testing.T) {
		t.Parallel()

		run := newAuditRun(t, true)
		run.portalID = "up_missing"

		err := run.audit(universalPortalsAuditInputs{})

		require.Error(t, err)
		assert.Equal(t, "not_found", errorClass(err))
		assert.Equal(t, 404, errorHTTPStatus(err))
		assert.Empty(t, run.results.String())
	})

	t.Run("a missing scope is an auth error that lists how to get it", func(t *testing.T) {
		t.Parallel()

		run := newAuditRun(t, true)
		run.tenant.Deny("GET portals/"+run.portalID, "Insufficient scope, expected any of: read:portals")

		err := run.audit(universalPortalsAuditInputs{})

		require.Error(t, err)
		assert.Equal(t, "auth", errorClass(err))
		assert.Equal(t, "insufficient_scope", errorReason(err))

		var details struct {
			Fixes []string `json:"fixes"`
		}
		require.NoError(t, json.Unmarshal(buildErrorEnvelope(err).Error.Details, &details))
		assert.Equal(t, portals.ScopeFixes([]string{"read:portals"}), details.Fixes)
	})

	t.Run("the command needs one portal ID", func(t *testing.T) {
		t.Parallel()

		cmd := universalPortalsAuditCmd(&cli{})

		for _, args := range [][]string{nil, {"up_1", "up_2"}} {
			err := cmd.Args(cmd, args)

			require.Error(t, err)
			assert.Equal(t, "usage", errorClass(err))
			assert.Equal(t, "missing_required_args", errorReason(err))
			assert.Contains(t, err.Error(), "auth0 universal-portals audit <portal-id>")
		}

		assert.NoError(t, cmd.Args(cmd, []string{"up_1"}))
	})
}

func TestAuditFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		outcome portals.AuditOutcome
		strict  bool
		want    string
	}{
		{
			name:    "errors",
			outcome: portals.AuditOutcome{Result: portals.AuditResult{Errors: 2}},
			want:    "the audit failed: 2 error(s)",
		},
		{
			name:    "errors and a fix that failed",
			outcome: portals.AuditOutcome{Result: portals.AuditResult{Errors: 1}, FixesFailed: 1},
			want:    "the audit failed: 1 error(s), 1 fix(es) failed",
		},
		{
			name:    "warnings count in a strict audit",
			outcome: portals.AuditOutcome{Result: portals.AuditResult{Warnings: 3}},
			strict:  true,
			want:    "the audit failed: 3 warning(s) in a strict audit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.EqualError(t, auditFailure(tt.outcome, tt.strict), tt.want)
		})
	}
}
