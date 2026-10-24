package cli

import (
	"context"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/analytics"
	"github.com/auth0/auth0-cli/internal/buildinfo"
	"github.com/auth0/auth0-cli/internal/config"
	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/approvals"
	"github.com/auth0/auth0-cli/internal/plugins/devlink"
	"github.com/auth0/auth0-cli/internal/plugins/store"
)

// telemetryVersion is the version the tests give the CLI. Tracking is off for a
// build without a version, and a prerelease keeps the plugin feature on.
const telemetryVersion = "1.39.0-beta.1"

// sentEvent is an event as it went over the wire.
type sentEvent struct {
	Event      string            `json:"event"`
	Identity   string            `json:"identity"`
	Properties map[string]string `json:"properties"`
}

// eventRecorder is the transport of the tracker in these tests. It keeps what
// the tracker sends and answers for the analytics endpoint, so no request ever
// leaves the test.
type eventRecorder struct {
	mu     sync.Mutex
	bodies []string
}

func (r *eventRecorder) RoundTrip(request *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.bodies = append(r.bodies, string(body))
	r.mu.Unlock()

	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(http.NoBody), Request: request}, nil
}

// newEventRecorder gives the CLI a tracker that sends to the recorder, with a
// build version and an install ID, as a release build has. A command that reads
// a config file with an install ID in it replaces the ID, so the tests compare
// with the install ID of the CLI after the run.
func newEventRecorder(t *testing.T, c *cli) *eventRecorder {
	t.Helper()

	previous := buildinfo.Version
	buildinfo.Version = telemetryVersion

	t.Cleanup(func() { buildinfo.Version = previous })
	t.Setenv("AUTH0_CLI_ANALYTICS", "")

	recorder := &eventRecorder{}

	c.Config.InstallID = "test-install-id"
	c.tracker = analytics.NewTrackerWithClient(&http.Client{Transport: recorder})

	return recorder
}

// events waits for the events in flight and returns the ones that were sent
// since the last call, in the order they were sent.
func (r *eventRecorder) events(t *testing.T, c *cli) []sentEvent {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c.tracker.Wait(ctx)
	require.NoError(t, ctx.Err())

	r.mu.Lock()
	defer r.mu.Unlock()

	var sent []sentEvent

	for _, body := range r.bodies {
		var event sentEvent
		require.NoError(t, json.Unmarshal([]byte(body), &event), body)

		sent = append(sent, event)
	}

	return sent
}

// raw returns the JSON of every event sent so far.
func (r *eventRecorder) raw() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return strings.Join(r.bodies, "\n")
}

// reset forgets the events sent so far.
func (r *eventRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.bodies = nil
}

// wireJSON returns the JSON of one event as it went over the wire.
func wireJSON(t *testing.T, event sentEvent) string {
	t.Helper()

	body, err := json.Marshal(event)
	require.NoError(t, err)

	return string(body)
}

// eventNames returns the names of the events, sorted.
func eventNames(events []sentEvent) []string {
	names := make([]string, 0, len(events))
	for _, event := range events {
		names = append(names, event.Event)
	}

	slices.Sort(names)

	return names
}

// only returns the event with the name, and fails when there is not exactly one.
func only(t *testing.T, events []sentEvent, name string) sentEvent {
	t.Helper()

	var found []sentEvent

	for _, event := range events {
		if event.Event == name {
			found = append(found, event)
		}
	}

	require.Len(t, found, 1, "events named %q in %v", name, eventNames(events))

	return found[0]
}

// pluginProperties is the full set of properties of a plugin event: the ones every
// event has, and the given ones.
func pluginProperties(extra map[string]string) map[string]string {
	properties := map[string]string{
		"version": telemetryVersion,
		"os":      runtime.GOOS,
		"arch":    runtime.GOARCH,
	}

	for name, value := range extra {
		properties[name] = value
	}

	return properties
}

// commandEventKeys are the properties of the event of a core command.
var commandEventKeys = []string{
	"version", "os", "arch", "interactive", "ci", "agent_client", "no_input",
	"output_format", "forced", "is_api", "tenant", "success", "error_class", "error_reason",
}

// newTestRoot builds the command tree the way Execute does, for the lookup of the
// command a command line is for.
func newTestRoot(c *cli) *cobra.Command {
	root := buildRootCmd(c)
	addPersistentFlags(root, c)
	addSubCommands(root, c)
	enforceUnknownSubcommand(root)

	return root
}

// tracked runs a command line of a plugin, then reports the outcome the way
// Execute does, and returns the events that were sent.
func (r *runEnv) tracked(t *testing.T, recorder *eventRecorder, args ...string) ([]sentEvent, error) {
	t.Helper()

	recorder.reset()

	r.cli.pluginRun, r.cli.executedCommandPath = nil, ""

	// The tree binds the flags to the CLI, so it is built before the run, which
	// parses them.
	newTestRoot(r.cli)

	err := r.exec(args...)

	trackCommandOutcome(r.cli, err)

	return recorder.events(t, r.cli), err
}

// tracked is the same for a command of the plugin group.
func (e *pluginEnv) tracked(t *testing.T, recorder *eventRecorder, args ...string) ([]sentEvent, error) {
	t.Helper()

	recorder.reset()

	e.cli.pluginRun, e.cli.executedCommandPath = nil, ""

	// The tree binds the flags to the CLI, so it is built before the run, which
	// parses them.
	newTestRoot(e.cli)

	err := e.run(args...)

	trackCommandOutcome(e.cli, err)

	return recorder.events(t, e.cli), err
}

func TestPluginRunTelemetry(t *testing.T) {
	tests := []struct {
		name       string
		commands   func() []plugins.Command
		unapproved bool
		setup      func(t *testing.T, r *runEnv)
		args       []string
		want       map[string]string
	}{
		{
			name: "a run that succeeds",
			args: []string{"checkmate"},
			want: map[string]string{"success": "true"},
		},
		{
			name: "a plugin that exits with a code",
			args: []string{"checkmate", "--mode=exit", "--code=7"},
			want: map[string]string{"success": "false", "error_reason": "plugin_exit"},
		},
		{
			name: "a plugin that runs out of time",
			commands: func() []plugins.Command {
				commands := fixtureCommands()
				commands[1].Timeout = "1s"

				return commands
			},
			args: []string{"checkmate", "scan", "--mode=sleep", "--marker={marker}"},
			want: map[string]string{"success": "false", "error_reason": "plugin_timeout"},
		},
		{
			name:       "not approved",
			unapproved: true,
			args:       []string{"checkmate"},
			want:       map[string]string{"success": "false", "error_reason": "plugin_not_approved"},
		},
		{
			name: "revoked",
			setup: func(_ *testing.T, r *runEnv) {
				r.republish(r.fixtureIndex(r.record).Revoke(pluginTestID, "1.0.0", "it leaks tokens."))
			},
			args: []string{"checkmate"},
			want: map[string]string{"success": "false", "error_reason": "plugin_revoked"},
		},
		{
			name:  "the session is for another tenant",
			setup: func(_ *testing.T, r *runEnv) { r.Domain = "other.us.auth0.com" },
			args:  []string{"checkmate"},
			want:  map[string]string{"success": "false", "error_reason": "plugin_tenant_mismatch"},
		},
		{
			name: "an output the command does not support",
			args: []string{"checkmate", "scan", "--json"},
			want: map[string]string{"success": "false", "error_reason": "plugin_output_unsupported"},
		},
		{
			name: "a failure that is not one of the reasons of the plugin",
			setup: func(_ *testing.T, r *runEnv) {
				r.loginErr = authError{err: assert.AnError, reason: "session_expired"}
			},
			args: []string{"checkmate"},
			want: map[string]string{"success": "false", "error_reason": "other"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			commands := fixtureCommands
			if test.commands != nil {
				commands = test.commands
			}

			r := newRunEnv(t, commands())
			recorder := newEventRecorder(t, r.cli)

			if !test.unapproved {
				r.approve()
			}

			if test.setup != nil {
				test.setup(t, r)
			}

			marker := filepath.Join(t.TempDir(), "pid")

			args := slices.Clone(test.args)
			for n := range args {
				args[n] = strings.ReplaceAll(args[n], "{marker}", marker)
			}

			events, _ := r.tracked(t, recorder, args...)

			require.Len(t, events, 1, "one event for one run, and no event of a core command: %v", eventNames(events))
			assert.Equal(t, "CLI - Plugin - Run", events[0].Event)
			assert.NotEmpty(t, r.cli.Config.InstallID)
			assert.Equal(t, r.cli.Config.InstallID, events[0].Identity)
			assert.Equal(t, pluginProperties(mergeProperties(map[string]string{"plugin": "checkmate", "plugin_version": "1.0.0"}, test.want)), events[0].Properties)
		})
	}
}

func TestPluginRunTelemetry_NothingBeforeTheFilesAreVerified(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, r *runEnv)
		args  []string
	}{
		{
			name:  "plugins are turned off",
			setup: func(t *testing.T, _ *runEnv) { t.Setenv(pluginsDisabledEnv, "true") },
			args:  []string{"checkmate"},
		},
		{
			name: "a tenant flag that conflicts with the environment",
			setup: func(t *testing.T, _ *runEnv) {
				t.Setenv(authModeEnvVar, authModeEnv)
				t.Setenv(envDomain, "fixed.us.auth0.com")
				t.Setenv(envAPIToken, "env-token")
			},
			args: []string{"checkmate", "--tenant", pluginTestTenant},
		},
		{
			name: "files that changed",
			setup: func(t *testing.T, r *runEnv) {
				verified, err := r.svc.store.Verify(pluginTestID)
				require.NoError(t, err)

				require.NoError(t, appendToFile(verified.Bin, "tampered"))
			},
			args: []string{"checkmate"},
		},
		{
			name: "a flag the CLI cannot read",
			args: []string{"checkmate", "--tenant"},
		},
		{
			name: "the help of the plugin",
			args: []string{"checkmate", "--help"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := newRunEnv(t, fixtureCommands())
			recorder := newEventRecorder(t, r.cli)
			r.approve()

			if test.setup != nil {
				test.setup(t, r)
			}

			events, _ := r.tracked(t, recorder, test.args...)

			assert.Empty(t, events, "neither the event of the plugin nor the event of a core command")
		})
	}
}

func TestPluginRunTelemetry_NothingTheUserTyped(t *testing.T) {
	secrets := []string{pluginTestTenant, "customer-xyz.eu.auth0.com", "sk_live_SECRETVALUE", "hunter2-password", "validators", "--mode", "secret.txt"}

	for _, test := range []struct {
		name string
		mode string
		want map[string]string
	}{
		{"a run that succeeds", "", map[string]string{"success": "true"}},
		{"a run that fails", "--mode=exit", map[string]string{"success": "false", "error_reason": "plugin_exit"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := newRunEnv(t, fixtureCommands())
			recorder := newEventRecorder(t, r.cli)
			r.approve()

			args := []string{"checkmate", "--tenant", pluginTestTenant, "--validators=sk_live_SECRETVALUE", "customer-xyz.eu.auth0.com", "hunter2-password", "--json", "--csv", "secret.txt"}
			if test.mode != "" {
				args = append(args, test.mode, "--code=7")
			}

			events, _ := r.tracked(t, recorder, args...)

			require.Len(t, events, 1, eventNames(events))
			assert.Equal(t, "CLI - Plugin - Run", events[0].Event)

			wire := recorder.raw()
			for _, secret := range secrets {
				assert.NotContains(t, wire, secret)
			}

			assert.Equal(t, pluginProperties(mergeProperties(map[string]string{"plugin": "checkmate", "plugin_version": "1.0.0"}, test.want)), events[0].Properties)
		})
	}
}

func TestPluginRunTelemetry_EnvAuth(t *testing.T) {
	t.Run("env auth mode has no install ID", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		recorder := newEventRecorder(t, r.cli)
		r.approve()

		t.Setenv(authModeEnvVar, authModeEnv)
		t.Setenv(envDomain, pluginTestTenant)
		t.Setenv(envAPIToken, "env-token")

		r.cli.Config.InstallID = ""

		events, err := r.tracked(t, recorder, "checkmate")

		require.NoError(t, err)
		assert.True(t, r.started())
		assert.Empty(t, events)
	})
}

func TestTrackPluginGate(t *testing.T) {
	tests := []struct {
		name     string
		pluginID string
		version  string
		want     bool
	}{
		{"a plugin of the built-in registry", "auth0/checkmate", "1.0.0", true},
		{"a plugin of a registry the user added", "acme/checkmate", "1.0.0", false},
		{"a plugin that only starts like the registry", "auth0x/checkmate", "1.0.0", false},
		{"a plugin that was linked for development", "dev/checkmate", "1.0.0", false},
		{"a plugin without a version", "auth0/checkmate", "", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := &cli{Config: config.Config{InstallID: "test-install-id"}}
			recorder := newEventRecorder(t, c)

			// A run, whatever the plugin is, never sends the event of a core command.
			c.pluginRun = &pluginRunIdentity{id: test.pluginID, version: test.version}
			c.executedCommandPath = "auth0 " + plugins.NameOf(test.pluginID)

			trackCommandOutcome(c, nil)
			c.trackPluginChange(analytics.PluginActionInstall, store.Result{ID: test.pluginID, Version: test.version, Action: store.ActionInstalled})
			c.trackPlugin(analytics.PluginActionUninstall, test.pluginID, analytics.PluginEvent{Version: test.version})

			events := recorder.events(t, c)

			if !test.want {
				assert.Empty(t, events)
				return
			}

			assert.Equal(t, []string{"CLI - Plugin - Install", "CLI - Plugin - Run", "CLI - Plugin - Uninstall"}, eventNames(events))
			assert.Equal(t, "test-install-id", events[0].Identity)
		})
	}
}

func TestPluginChangeTelemetry_Install(t *testing.T) {
	t.Run("an install sends its event and the command event with the tenant flag", func(t *testing.T) {
		e, _ := newInstallEnv(t)
		recorder := newEventRecorder(t, e.cli)
		e.answer("y\n")

		events, err := e.tracked(t, recorder, "plugin", "install", "checkmate", "--tenant", pluginTestTenant)

		require.NoError(t, err)
		require.Equal(t, []string{"CLI - Plugin - Install", "CLI - Plugin - Install - Run"}, eventNames(events))

		install := only(t, events, "CLI - Plugin - Install")
		assert.NotEmpty(t, e.cli.Config.InstallID)
		assert.Equal(t, e.cli.Config.InstallID, install.Identity)
		assert.Equal(t, pluginProperties(map[string]string{"plugin": "checkmate", "plugin_version": "1.0.0"}), install.Properties)
		assert.NotContains(t, wireJSON(t, install), pluginTestTenant)

		command := only(t, events, "CLI - Plugin - Install - Run")
		assert.ElementsMatch(t, commandEventKeys, slices.Collect(maps.Keys(command.Properties)))
		assert.Equal(t, pluginTestTenant, command.Properties["tenant"])
	})

	t.Run("an install without the tenant flag sends an empty tenant, as any command does", func(t *testing.T) {
		e, _ := newInstallEnv(t)
		recorder := newEventRecorder(t, e.cli)
		e.answer("y\n")

		events, err := e.tracked(t, recorder, "plugin", "install", "checkmate")

		require.NoError(t, err)

		command := only(t, events, "CLI - Plugin - Install - Run")
		assert.ElementsMatch(t, commandEventKeys, slices.Collect(maps.Keys(command.Properties)))
		assert.Empty(t, command.Properties["tenant"])
		assert.NotContains(t, wireJSON(t, only(t, events, "CLI - Plugin - Install")), "tenant")
	})

	t.Run("an install that changed nothing sends no plugin event", func(t *testing.T) {
		e, _ := newInstallEnv(t)
		recorder := newEventRecorder(t, e.cli)
		e.install("1.0.0")

		events, err := e.tracked(t, recorder, "plugin", "install", "checkmate")

		require.NoError(t, err)
		assert.Contains(t, e.stderr.String(), "already installed")
		assert.Equal(t, []string{"CLI - Plugin - Install - Run"}, eventNames(events))
	})

	t.Run("a person who declines sends no plugin event", func(t *testing.T) {
		e, _ := newInstallEnv(t)
		recorder := newEventRecorder(t, e.cli)
		e.answer("n\n")

		events, err := e.tracked(t, recorder, "plugin", "install", "checkmate", "--tenant", pluginTestTenant)

		require.NoError(t, err)
		assert.Empty(t, e.installedIDs())
		assert.Equal(t, []string{"CLI - Plugin - Install - Run"}, eventNames(events))
	})

	t.Run("an install that fails sends no plugin event", func(t *testing.T) {
		e, _ := newInstallEnv(t)
		recorder := newEventRecorder(t, e.cli)

		events, err := e.tracked(t, recorder, "plugin", "install", "checkmate@3.0.0")

		require.Error(t, err)
		assert.Equal(t, []string{"CLI - Plugin - Install - Run"}, eventNames(events))
		assert.Equal(t, "false", only(t, events, "CLI - Plugin - Install - Run").Properties["success"])
	})

	t.Run("a version that was installed before and is switched to", func(t *testing.T) {
		e := newPluginEnv(t)
		e.publish(e.index(e.release("1.0.0"), e.release("2.0.0")))
		recorder := newEventRecorder(t, e.cli)
		e.install("2.0.0")

		events, err := e.tracked(t, recorder, "plugin", "install", "checkmate@1.0.0")

		require.NoError(t, err)

		install := only(t, events, "CLI - Plugin - Install")
		assert.Equal(t, pluginProperties(map[string]string{"plugin": "checkmate", "plugin_version": "1.0.0"}), install.Properties)
	})
}

func TestPluginChangeTelemetry_Update(t *testing.T) {
	t.Run("an update sends the version it moved to", func(t *testing.T) {
		e, _, _ := newUpdateEnv(t, withAsks, withAsks)
		recorder := newEventRecorder(t, e.cli)

		events, err := e.tracked(t, recorder, "plugin", "update", "checkmate", "--tenant", pluginTestTenant)

		require.NoError(t, err)
		assert.Equal(t, []string{"CLI - Plugin - Update", "CLI - Plugin - Update - Run"}, eventNames(events))
		update := only(t, events, "CLI - Plugin - Update")
		assert.Equal(t, pluginProperties(map[string]string{"plugin": "checkmate", "plugin_version": "2.0.0"}), update.Properties)
		assert.NotContains(t, wireJSON(t, update), pluginTestTenant)

		command := only(t, events, "CLI - Plugin - Update - Run")
		assert.ElementsMatch(t, commandEventKeys, slices.Collect(maps.Keys(command.Properties)))
		assert.Equal(t, pluginTestTenant, command.Properties["tenant"])
	})

	t.Run("an update to the version that is installed sends no plugin event", func(t *testing.T) {
		e := newPluginEnv(t)
		e.publish(e.index(e.release("1.0.0")))
		e.installPlugin(pluginTestID, "")
		recorder := newEventRecorder(t, e.cli)

		events, err := e.tracked(t, recorder, "plugin", "update", "checkmate")

		require.NoError(t, err)
		assert.Contains(t, e.stderr.String(), "is up to date")
		assert.Equal(t, []string{"CLI - Plugin - Update - Run"}, eventNames(events))
		assert.Empty(t, only(t, events, "CLI - Plugin - Update - Run").Properties["tenant"])
	})

	t.Run("an update that is declined sends no plugin event", func(t *testing.T) {
		e, _, _ := newUpdateEnv(t, withScopes("read:clients"), withScopes("read:clients", "read:logs"))
		recorder := newEventRecorder(t, e.cli)
		e.answer("n\n")

		events, err := e.tracked(t, recorder, "plugin", "update", "checkmate", "--tenant", pluginTestTenant)

		require.NoError(t, err)
		assert.Equal(t, "1.0.0", e.activeVersion(pluginTestID))
		assert.Equal(t, []string{"CLI - Plugin - Update - Run"}, eventNames(events))
	})

	t.Run("an update that fails sends no plugin event", func(t *testing.T) {
		e, _, _ := newUpdateEnv(t, withAsks, withAsks)
		recorder := newEventRecorder(t, e.cli)
		e.server.Fail(pluginArtifactPath("checkmate", "2.0.0"), 503)

		events, err := e.tracked(t, recorder, "plugin", "update", "checkmate", "--tenant", pluginTestTenant)

		require.Error(t, err)
		assert.Equal(t, []string{"CLI - Plugin - Update - Run"}, eventNames(events))
	})
}

func TestPluginChangeTelemetry_UpdateAll(t *testing.T) {
	t.Run("one event for each plugin that was updated, none for a pinned one", func(t *testing.T) {
		e, _, _ := newPairEnv(t, true, withAsks, withAsks)
		recorder := newEventRecorder(t, e.cli)

		events, err := e.tracked(t, recorder, "plugin", "update", "--all", "--tenant", pluginTestTenant)

		require.NoError(t, err)
		assert.Equal(t, []string{"CLI - Plugin - Update", "CLI - Plugin - Update - Run"}, eventNames(events))

		update := only(t, events, "CLI - Plugin - Update")
		assert.Equal(t, pluginProperties(map[string]string{"plugin": "other", "plugin_version": "2.0.0"}), update.Properties)
		assert.NotContains(t, wireJSON(t, update), pluginTestTenant)
		assert.Equal(t, pluginTestTenant, only(t, events, "CLI - Plugin - Update - Run").Properties["tenant"])
	})

	t.Run("an update that failed or was declined is left out of a mixed result", func(t *testing.T) {
		tests := []struct {
			name  string
			setup func(e *pluginEnv)
			want  []string
		}{
			{
				name:  "the second download fails",
				setup: func(e *pluginEnv) { e.server.Fail(pluginArtifactPath("other", "2.0.0"), 503) },
				want:  []string{"checkmate"},
			},
			{
				name:  "the person declines the first plugin",
				setup: func(e *pluginEnv) { e.answer("n\ny\n") },
				want:  []string{"other"},
			},
			{
				name:  "every plugin was updated",
				setup: func(e *pluginEnv) {},
				want:  []string{"checkmate", "other"},
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				e, _, _ := newPairEnv(t, false, withScopes("read:clients"), withScopes("read:clients", "read:logs"))
				recorder := newEventRecorder(t, e.cli)

				e.answer("y\ny\n")
				test.setup(e)

				events, _ := e.tracked(t, recorder, "plugin", "update", "--all", "--tenant", pluginTestTenant)

				var updated []string

				for _, event := range events {
					if event.Event == "CLI - Plugin - Update" {
						assert.Equal(t, "2.0.0", event.Properties["plugin_version"])
						assert.NotContains(t, event.Properties, "success")

						updated = append(updated, event.Properties["plugin"])
					}
				}

				slices.Sort(updated)

				assert.Equal(t, test.want, updated)
			})
		}
	})

	t.Run("updates whose approval could not be saved send no plugin event", func(t *testing.T) {
		e, _, _ := newPairEnv(t, false, withScopes("read:clients"), withScopes("read:clients", "read:logs"))
		recorder := newEventRecorder(t, e.cli)
		e.answer("y\ny\n")

		// The approvals of the operation go to a directory that can be read and not
		// written, so asking for consent works and recording the approvals fails
		// after the plugins were updated.
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			t.Skip("a directory cannot be made read-only for this user")
		}

		readOnly := t.TempDir()
		require.NoError(t, os.Chmod(readOnly, 0o500))
		t.Cleanup(func() { _ = os.Chmod(readOnly, 0o700) })

		e.svc.approvals = &approvals.Approvals{Dir: readOnly}

		events, err := e.tracked(t, recorder, "plugin", "update", "--all", "--tenant", pluginTestTenant)

		require.Error(t, err)
		assert.Equal(t, "2.0.0", e.activeVersion(pluginTestID))
		assert.Equal(t, []string{"CLI - Plugin - Update - Run"}, eventNames(events))
	})

	t.Run("a plugin that is already current sends no plugin event", func(t *testing.T) {
		e := newPluginEnv(t)
		e.publish(e.index(e.release("1.0.0")))
		e.installPlugin(pluginTestID, "")
		recorder := newEventRecorder(t, e.cli)

		events, err := e.tracked(t, recorder, "plugin", "update", "--all")

		require.NoError(t, err)
		assert.Equal(t, []string{"CLI - Plugin - Update - Run"}, eventNames(events))
	})
}

func TestPluginChangeTelemetry_Uninstall(t *testing.T) {
	newEnv := func(t *testing.T) *pluginEnv {
		t.Helper()

		e, _, _ := newUpdateEnv(t, withAsks, withAsks)
		e.installPlugin(pluginTestID, "2.0.0")

		return e
	}

	t.Run("sends the version that was active before the plugin was removed", func(t *testing.T) {
		e := newEnv(t)
		recorder := newEventRecorder(t, e.cli)

		events, err := e.tracked(t, recorder, "plugin", "uninstall", "checkmate", "--force", "--tenant", pluginTestTenant)

		require.NoError(t, err)
		assert.Empty(t, e.installedIDs())
		assert.Equal(t, []string{"CLI - Plugin - Uninstall", "CLI - Plugin - Uninstall - Run"}, eventNames(events))

		uninstall := only(t, events, "CLI - Plugin - Uninstall")
		assert.Equal(t, pluginProperties(map[string]string{"plugin": "checkmate", "plugin_version": "2.0.0"}), uninstall.Properties)
		assert.NotContains(t, wireJSON(t, uninstall), pluginTestTenant)
		assert.Equal(t, pluginTestTenant, only(t, events, "CLI - Plugin - Uninstall - Run").Properties["tenant"])
	})

	t.Run("a person who does not confirm sends no plugin event", func(t *testing.T) {
		e := newEnv(t)
		recorder := newEventRecorder(t, e.cli)
		e.Terminal = true

		events, err := e.tracked(t, recorder, "plugin", "uninstall", "checkmate")

		require.NoError(t, err)
		assert.Equal(t, []string{pluginTestID}, e.installedIDs())
		assert.Equal(t, []string{"CLI - Plugin - Uninstall - Run"}, eventNames(events))
		assert.Empty(t, only(t, events, "CLI - Plugin - Uninstall - Run").Properties["tenant"])
	})

	t.Run("a plugin that is not installed sends no plugin event", func(t *testing.T) {
		e := newEnv(t)
		recorder := newEventRecorder(t, e.cli)

		events, err := e.tracked(t, recorder, "plugin", "uninstall", "nothing", "--force")

		require.Error(t, err)
		assert.Equal(t, []string{"CLI - Plugin - Uninstall - Run"}, eventNames(events))
	})

	t.Run("env auth mode has no install ID", func(t *testing.T) {
		e := newEnv(t)
		recorder := newEventRecorder(t, e.cli)

		t.Setenv(authModeEnvVar, authModeEnv)
		t.Setenv(envDomain, pluginTestTenant)
		t.Setenv(envAPIToken, "env-token")

		e.cli.Config.InstallID = ""

		events, err := e.tracked(t, recorder, "plugin", "uninstall", "checkmate", "--force")

		require.NoError(t, err)
		assert.Empty(t, e.installedIDs())
		assert.Empty(t, events)
	})
}

func TestPluginGroupFollowsTheTenantPatternOfOtherCommands(t *testing.T) {
	const leak = "leak.us.auth0.com"

	t.Run("a plugin command that does not log in sends the tenant flag", func(t *testing.T) {
		e, _ := newInstallEnv(t)
		recorder := newEventRecorder(t, e.cli)

		events, err := e.tracked(t, recorder, "plugin", "install", "checkmate@3.0.0", "--tenant", leak)

		require.Error(t, err)
		require.Equal(t, []string{"CLI - Plugin - Install - Run"}, eventNames(events))
		assert.Equal(t, leak, events[0].Properties["tenant"])
	})

	t.Run("a core command sends the tenant flag as before", func(t *testing.T) {
		e := newPluginEnv(t)
		recorder := newEventRecorder(t, e.cli)

		events, _ := e.tracked(t, recorder, "apps", "show", "a", "b", "--tenant", leak)

		require.Len(t, events, 1, eventNames(events))
		assert.Equal(t, leak, events[0].Properties["tenant"])
	})
}

// appendToFile adds text to the end of a file, which makes a plugin fail its
// verification.
func appendToFile(path, text string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}

	if _, err := file.WriteString(text); err != nil {
		_ = file.Close()

		return err
	}

	return file.Close()
}

func TestPluginRunTelemetry_LinkedRunsAreNotTracked(t *testing.T) {
	tests := []struct {
		name       string
		unapproved bool
		args       []string
	}{
		{name: "a run that succeeds", args: []string{"checkmate"}},
		{name: "a plugin that exits with a code", args: []string{"checkmate", "--mode=exit", "--code=7"}},
		{name: "not approved", unapproved: true, args: []string{"checkmate"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := newRunEnv(t, fixtureCommands())
			recorder := newEventRecorder(t, r.cli)

			build := r.linkFixture("2.0.0", fixtureCommands())
			if !test.unapproved {
				r.approveBuild(build)
			}

			events, _ := r.tracked(t, recorder, test.args...)

			assert.Empty(t, events, "a build that is not published sends nothing: %v", eventNames(events))
			assert.Empty(t, recorder.raw())
		})
	}

	t.Run("a link that cannot be used", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		recorder := newEventRecorder(t, r.cli)

		build := r.linkFixture("2.0.0", fixtureCommands())
		require.NoError(t, os.Remove(build.Bin))

		events, err := r.tracked(t, recorder, "checkmate")

		require.Error(t, err)
		assert.Empty(t, events)
	})

	t.Run("the installed plugin is still tracked once the link is gone", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		recorder := newEventRecorder(t, r.cli)
		r.approve()

		build := r.linkFixture("2.0.0", fixtureCommands())
		r.approveBuild(build)

		events, err := r.tracked(t, recorder, "checkmate")
		require.NoError(t, err)
		require.Empty(t, events)

		require.NoError(t, devlink.Links{Dir: r.Dir}.Remove(context.Background(), pluginTestID))

		events, err = r.tracked(t, recorder, "checkmate")
		require.NoError(t, err)
		require.Len(t, events, 1)

		assert.Equal(t, "CLI - Plugin - Run", events[0].Event)
		assert.Equal(t, "1.0.0", events[0].Properties["plugin_version"])
	})
}
