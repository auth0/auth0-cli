package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/portals"
	"github.com/auth0/auth0-cli/internal/portals/portalstest"
)

const portalsTestTenant = "acme.us.auth0.com"

// writeTrackingTenant is a fake tenant that says whether setup created anything in it.
type writeTrackingTenant struct {
	*portalstest.Tenant
}

func (t writeTrackingTenant) hasWritten() bool {
	return len(t.Clients())+len(t.Portals())+len(t.ResourceServers())+len(t.Forms())+len(t.VaultConnections()) > 0
}

// setupRun is a run of the setup command on a fake tenant.
type setupRun struct {
	tenant   *portalstest.Tenant
	messages bytes.Buffer
	results  bytes.Buffer
	cli      *cli
}

func newSetupRun(json bool) *setupRun {
	run := &setupRun{tenant: portalstest.New()}
	run.cli = &cli{
		renderer: &display.Renderer{MessageWriter: &run.messages, ResultWriter: &run.results},
		tenant:   portalsTestTenant,
		json:     json,
	}

	return run
}

func (r *setupRun) run(inputs universalPortalsSetupInputs) error {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	tracked := writeTrackingTenant{Tenant: r.tenant}

	return runUniversalPortalsSetupOn(cmd, r.cli, inputs, portalsSetupTarget{api: tracked, hasWritten: tracked.hasWritten})
}

var consumerInputs = universalPortalsSetupInputs{Template: "consumer", Name: "Acme Account", Slug: "acme-account"}

func TestSetupCommand_JSON(t *testing.T) {
	t.Parallel()

	run := newSetupRun(true)

	require.NoError(t, run.run(consumerInputs))

	assert.Empty(t, run.messages.String(), "the result is alone on its stream")

	// The result has the content and the order of keys of the tool this replaces, and the
	// indentation of every other command of the CLI.
	var reindented bytes.Buffer
	require.NoError(t, json.Indent(&reindented, run.results.Bytes(), "", "  "))

	want, err := os.ReadFile("../portals/testdata/consumer-setup.json")
	require.NoError(t, err)
	assert.Equal(t, strings.TrimSpace(string(want)), strings.TrimSpace(reindented.String()))
}

func TestSetupCommand_HumanOutput(t *testing.T) {
	t.Parallel()

	run := newSetupRun(false)

	require.NoError(t, run.run(consumerInputs))

	assert.Empty(t, run.results.String(), "a person gets no JSON")

	lines := strings.Split(run.messages.String(), "\n")
	assert.Equal(t, "│ ╭─Universal Portals setup──╮", lines[0])
	assert.Equal(t, "│ │    acme.us.auth0.com     │", lines[1])
	assert.Contains(t, run.messages.String(), "◇  Permissions")
	assert.Contains(t, run.messages.String(), "◆  The CLI session has every permission setup needs")
	assert.Contains(t, run.messages.String(), `"Acme Account (Universal Portals)" created`)
	assert.Contains(t, run.messages.String(), "Open portal: https://acme.us.auth0.com/portals/acme-account")
	assert.Len(t, run.tenant.Portals(), 1)
}

func TestSetupCommand_TheSlugDefaultsToTheOneOfTheName(t *testing.T) {
	t.Parallel()

	run := newSetupRun(true)

	require.NoError(t, run.run(universalPortalsSetupInputs{Template: "consumer", Name: "Acme Account"}))

	assert.Equal(t, "acme-account", run.tenant.Portals()[0].Slug)
}

func TestSetupCommand_FlagsAreChecked(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inputs     universalPortalsSetupInputs
		wantReason string
		wantText   string
	}{
		{
			name:       "a run that cannot ask needs the template and the name",
			inputs:     universalPortalsSetupInputs{},
			wantReason: "missing_required_flags",
			wantText:   "missing required flags for a non-interactive run: --template, --name",
		},
		{
			name:       "a business portal also needs the user",
			inputs:     universalPortalsSetupInputs{Template: "business", Name: "Acme Admin"},
			wantReason: "missing_required_flags",
			wantText:   "missing required flags for a non-interactive run: --user",
		},
		{
			name:       "a template nobody knows",
			inputs:     universalPortalsSetupInputs{Template: "enterprise", Name: "Acme"},
			wantReason: "invalid_input",
			wantText:   `unknown template "enterprise": use consumer or business`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			run := newSetupRun(true)

			err := run.run(tt.inputs)

			require.Error(t, err)
			assert.Equal(t, tt.wantText, err.Error())
			assert.Equal(t, "usage", errorClass(err))
			assert.Equal(t, tt.wantReason, errorReason(err))
			assert.Empty(t, run.tenant.Calls(), "nothing is sent before the flags are checked")
		})
	}
}

func TestSetupCommand_ErrorsAreClassified(t *testing.T) {
	t.Parallel()

	t.Run("a missing scope is an auth error that lists how to get it", func(t *testing.T) {
		t.Parallel()

		run := newSetupRun(true)
		run.tenant.Deny("POST portals", "Insufficient scope, expected any of: create:portals")

		err := run.run(consumerInputs)

		require.Error(t, err)
		assert.Equal(t, "The CLI session is missing 1 permission(s) setup needs: create:portals. Nothing was created", err.Error())
		assert.Equal(t, "auth", errorClass(err))
		assert.Equal(t, "missing_scopes", errorReason(err))

		var details struct {
			Fixes []string `json:"fixes"`
		}
		require.NoError(t, json.Unmarshal(buildErrorEnvelope(err).Error.Details, &details))
		assert.Equal(t, portals.ScopeFixes([]string{"create:portals"}), details.Fixes)
	})

	t.Run("the steps that resolve an error are printed after its message", func(t *testing.T) {
		t.Parallel()

		run := newSetupRun(true)
		run.tenant.Deny("POST portals", "Insufficient scope, expected any of: create:portals")

		err := run.run(consumerInputs)

		var shown bytes.Buffer
		renderErrorMessage(&display.Renderer{MessageWriter: &shown, ResultWriter: &shown}, err)

		assert.Contains(t, shown.String(), "Signed in as a user: auth0 login --scopes")
		assert.Contains(t, shown.String(), "Signed in with client credentials: add create:portals to the application's Management API grant")
	})

	t.Run("a name in use is a validation error", func(t *testing.T) {
		t.Parallel()

		run := newSetupRun(true)
		run.tenant.AddClient(portals.Client{ID: "cli_manual", Secret: "x", ClientSettings: portals.ClientSettings{Name: "Acme Account (Universal Portals)"}})

		err := run.run(consumerInputs)

		require.Error(t, err)
		assert.Equal(t, "validation", errorClass(err))
		assert.Equal(t, "name_in_use", errorReason(err))
	})

	t.Run("a slug another portal uses is a validation error that says how to resolve it", func(t *testing.T) {
		t.Parallel()

		run := newSetupRun(true)
		run.tenant.AddPortal(portals.Portal{ID: "up_other", Slug: "acme-account", Name: "Other"})

		err := run.run(consumerInputs)

		require.Error(t, err)
		assert.Equal(t, "validation", errorClass(err))
		assert.Equal(t, "slug_taken", errorReason(err))

		var details struct {
			Fixes []string `json:"fixes"`
		}
		require.NoError(t, json.Unmarshal(buildErrorEnvelope(err).Error.Details, &details))
		assert.Contains(t, details.Fixes, "Rerun with a different --slug")
		assert.Empty(t, run.tenant.Clients(), "nothing is created before the slug is checked")
	})

	t.Run("a feature that is not enabled is reported as such", func(t *testing.T) {
		t.Parallel()

		err := portalsCommandError(&portals.FeatureNotEnabledError{Tenant: portalsTestTenant}, portalsTestTenant, false)

		assert.Equal(t, "auth", errorClass(err))
		assert.Equal(t, "feature_not_enabled", errorReason(err))
	})
}

// The class and the reason of an error are what a script reads from the JSON envelope, so each error
// of the portals package keeps its own.
func TestPortalsCommandError_EachErrorHasItsOwnReason(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantClass  string
		wantReason string
		wantFixes  []string
	}{
		{
			name:       "an input the command cannot run with",
			err:        &portals.UsageError{Message: "bad slug"},
			wantClass:  "usage",
			wantReason: "invalid_input",
		},
		{
			name:       "a user who cannot test a business portal",
			err:        &portals.BusinessAccessError{Message: "no access", Hints: []string{"Add the user to an organization"}},
			wantClass:  "validation",
			wantReason: "business_access",
			wantFixes:  []string{"Add the user to an organization"},
		},
		{
			name:       "an application that setup did not create",
			err:        &portals.NameInUseError{Name: "Acme"},
			wantClass:  "validation",
			wantReason: "name_in_use",
		},
		{
			name:       "a slug another portal uses",
			err:        &portals.SlugTakenError{Slug: "acme"},
			wantClass:  "validation",
			wantReason: "slug_taken",
			wantFixes:  []string{"Rerun with a different --slug"},
		},
		{
			name:       "a slug taken while setup runs",
			err:        &portals.SlugRaceError{Slug: "acme"},
			wantClass:  "validation",
			wantReason: "slug_race",
		},
		{
			name:       "a secret that needs a scope",
			err:        &portals.ClientSecretUnreadableError{ClientID: "cli_1"},
			wantClass:  "auth",
			wantReason: "missing_scopes",
			wantFixes:  portals.ScopeFixes([]string{"read:client_keys"}),
		},
		{
			name:       "a check refused with no reason",
			err:        &portals.PermissionCheckError{Probes: []string{"create portal"}, Tenant: portalsTestTenant},
			wantClass:  "auth",
			wantReason: "permission_check_failed",
			wantFixes:  []string{"Check that Universal Portals is enabled for this tenant."},
		},
		{
			name:       "a feature that is not enabled",
			err:        &portals.FeatureNotEnabledError{Tenant: portalsTestTenant},
			wantClass:  "auth",
			wantReason: "feature_not_enabled",
		},
		{
			name:       "a feature that is not enabled, told by any request and not by the checks",
			err:        fmt.Errorf("read portal: %w", &portals.APIError{Status: 403, Code: "feature_not_enabled", Message: "Feature not enabled"}),
			wantClass:  "auth",
			wantReason: "feature_not_enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := portalsCommandError(tt.err, portalsTestTenant, false)

			assert.Equal(t, tt.wantClass, errorClass(err))
			assert.Equal(t, tt.wantReason, errorReason(err))

			if tt.wantFixes == nil {
				return
			}

			var details struct {
				Fixes []string `json:"fixes"`
			}
			require.NoError(t, json.Unmarshal(buildErrorEnvelope(err).Error.Details, &details))
			assert.Subset(t, details.Fixes, tt.wantFixes)
		})
	}
}

func TestSetupCommand_AFailureAfterAWriteSaysHowToResume(t *testing.T) {
	t.Parallel()

	t.Run("a rerun resumes", func(t *testing.T) {
		t.Parallel()

		run := newSetupRun(true)
		run.tenant.FailNext("POST forms/import", http.StatusInternalServerError)

		err := run.run(consumerInputs)

		require.Error(t, err)

		var details struct {
			Fixes []string `json:"fixes"`
		}
		require.NoError(t, json.Unmarshal(buildErrorEnvelope(err).Error.Details, &details))
		assert.Equal(t, []string{"Run the same command again to resume. Existing resources are reused."}, details.Fixes)
	})

	t.Run("a failure before any write has nothing to resume", func(t *testing.T) {
		t.Parallel()

		run := newSetupRun(true)
		run.tenant.FailNext("GET custom-domains/default", http.StatusInternalServerError)

		err := run.run(consumerInputs)

		require.Error(t, err)
		assert.Nil(t, buildErrorEnvelope(err).Error.Details)
	})

	t.Run("a slug taken while setup runs is not resumed with the same name", func(t *testing.T) {
		t.Parallel()

		run := newSetupRun(true)
		run.tenant.FailNext("POST portals", http.StatusConflict)

		err := run.run(consumerInputs)

		require.Error(t, err)
		assert.Equal(t, "slug_race", errorReason(err))
		assert.Nil(t, buildErrorEnvelope(err).Error.Details)
	})
}

func TestReportPortalsCancelled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		wrote bool
		want  string
	}{
		{
			name: "before any write",
			want: "▲  Cancelled. Nothing was created.\n└  Setup cancelled.\n\n",
		},
		{
			name:  "after a write, nothing is rolled back and a rerun resumes",
			wrote: true,
			want: "▲  Cancelled. Nothing was rolled back: what was already created is still in your tenant.\n" +
				"▲  Run the same command again to resume. Existing resources are reused.\n└  Setup cancelled.\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			terminal, out := newTestTerminal(plainPortalsStyle(), false)

			reportPortalsCancelled(terminal, tt.wrote)

			assert.Equal(t, tt.want, out.String())
		})
	}
}

func TestSetupCommand_DryRun(t *testing.T) {
	t.Parallel()

	planned := func() []plannedRequest {
		return []plannedRequest{{Method: "POST", Path: "portals", Body: map[string]any{"slug": "acme-account"}}}
	}

	runDry := func(run *setupRun, inputs universalPortalsSetupInputs) error {
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())

		tracked := writeTrackingTenant{Tenant: run.tenant}

		return runUniversalPortalsSetupOn(cmd, run.cli, inputs, portalsSetupTarget{api: tracked, hasWritten: tracked.hasWritten, planned: planned})
	}

	t.Run("a person is told it is a dry run, and gets a summary and no portal to open", func(t *testing.T) {
		t.Parallel()

		run := newSetupRun(false)

		require.NoError(t, runDry(run, consumerInputs))

		assert.Contains(t, run.messages.String(), "●  Dry run: nothing is created or changed. Statuses show what a real run would do.")
		assert.Contains(t, run.messages.String(), "Dry run complete. 1 request(s) would change the tenant.")
		assert.Contains(t, run.messages.String(), "Run the same command without --dry-run to set up the portal.")
		assert.NotContains(t, run.messages.String(), "Open portal:")
		assert.Empty(t, run.results.String())
	})

	t.Run("JSON is the result with dryRun and the plan", func(t *testing.T) {
		t.Parallel()

		run := newSetupRun(true)

		require.NoError(t, runDry(run, consumerInputs))

		var printed map[string]any
		require.NoError(t, json.Unmarshal(run.results.Bytes(), &printed))
		assert.Equal(t, true, printed["dryRun"])
		assert.Contains(t, printed, "portal")
		assert.Len(t, printed["requests"], 1)
		assert.Empty(t, run.messages.String())
	})

	t.Run("the flags are checked as in a real run", func(t *testing.T) {
		t.Parallel()

		run := newSetupRun(true)

		err := runDry(run, universalPortalsSetupInputs{DryRun: true})

		require.Error(t, err)
		assert.Equal(t, "missing_required_flags", errorReason(err))
	})
}
