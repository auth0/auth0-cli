package analytics

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/buildinfo"
)

func TestGenerateEventName(t *testing.T) {
	t.Run("generates from root command run", func(t *testing.T) {
		want := "CLI - Auth0 - Action"
		got := generateEventName("auth0", "Action")
		assert.Equal(t, want, got)
	})

	t.Run("generates from top-level command run", func(t *testing.T) {
		want := "CLI - Auth0 - Apps - Action"
		got := generateEventName("auth0 apps", "Action")
		assert.Equal(t, want, got)
	})

	t.Run("generates from subcommand run", func(t *testing.T) {
		want := "CLI - Apps - List - Action"
		got := generateEventName("auth0 apps list", "Action")
		assert.Equal(t, want, got)
	})

	t.Run("generates from deep subcommand run", func(t *testing.T) {
		want := "CLI - Apis - Scopes List - Action"
		got := generateEventName("auth0 apis scopes list", "Action")
		assert.Equal(t, want, got)
	})
}

func TestGenerateRunEventName(t *testing.T) {
	t.Run("generates from root command", func(t *testing.T) {
		want := "CLI - Auth0 - Run"
		got := generateRunEventName("auth0")
		assert.Equal(t, want, got)
	})

	t.Run("generates from top-level command", func(t *testing.T) {
		want := "CLI - Auth0 - Apps - Run"
		got := generateRunEventName("auth0 apps")
		assert.Equal(t, want, got)
	})

	t.Run("generates from subcommand", func(t *testing.T) {
		want := "CLI - Apps - List - Run"
		got := generateRunEventName("auth0 apps list")
		assert.Equal(t, want, got)
	})
}

func TestNewEvent(t *testing.T) {
	t.Run("creates a new event instance", func(t *testing.T) {
		event := newEvent("event", "id", nil)
		// Assert that the interval between the event timestamp and now is within 1 second.
		assert.WithinDuration(t, time.Now(), time.Unix(0, event.Timestamp*int64(1000000)), 1*time.Second)
		assert.Equal(t, event.App, appID)
		assert.Equal(t, event.Event, "event")
		assert.Equal(t, event.ID, "id")
		assert.Equal(t, event.Properties[osKey], runtime.GOOS)
		assert.Equal(t, event.Properties[archKey], runtime.GOARCH)
	})

	t.Run("merges extra properties", func(t *testing.T) {
		event := newEvent("event", "id", map[string]string{"success": "false", "error_class": "auth"})
		assert.Equal(t, "false", event.Properties["success"])
		assert.Equal(t, "auth", event.Properties["error_class"])
		assert.Equal(t, runtime.GOOS, event.Properties[osKey])
	})
}

// recordingTransport keeps the events that the tracker sends instead of sending
// them, so that no test reaches the analytics endpoint.
type recordingTransport struct {
	mu     sync.Mutex
	events []event
	urls   []string
}

func (r *recordingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}

	var sent event
	if err := json.Unmarshal(body, &sent); err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.events = append(r.events, sent)
	r.urls = append(r.urls, request.URL.String())
	r.mu.Unlock()

	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(http.NoBody), Request: request}, nil
}

// newRecordedTracker returns a tracker that records what it sends, for a build
// with a version, so that tracking is on unless the test turns it off.
func newRecordedTracker(t *testing.T) (*Tracker, *recordingTransport) {
	t.Helper()

	previous := buildinfo.Version
	buildinfo.Version = "1.2.3"

	t.Cleanup(func() { buildinfo.Version = previous })
	t.Setenv("AUTH0_CLI_ANALYTICS", "")

	transport := &recordingTransport{}

	return NewTrackerWithClient(&http.Client{Transport: transport}), transport
}

// sent waits for the events in flight and returns what was sent.
func (r *recordingTransport) sent(t *testing.T, tracker *Tracker) []event {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tracker.Wait(ctx)
	require.NoError(t, ctx.Err())

	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]event{}, r.events...)
}

func TestTrackPlugin(t *testing.T) {
	base := func(extra map[string]string) map[string]string {
		properties := map[string]string{
			versionKey: "1.2.3",
			osKey:      runtime.GOOS,
			archKey:    runtime.GOARCH,
		}

		for k, v := range extra {
			properties[k] = v
		}

		return properties
	}

	tests := []struct {
		name       string
		action     PluginAction
		event      PluginEvent
		wantName   string
		wantProps  map[string]string
		wantNoSend bool
	}{
		{
			name:      "a successful run",
			action:    PluginActionRun,
			event:     PluginEvent{Plugin: "checkmate", Version: "1.0.0", Success: true},
			wantName:  "CLI - Plugin - Run",
			wantProps: base(map[string]string{"plugin": "checkmate", "plugin_version": "1.0.0", "success": "true"}),
		},
		{
			name:      "a failed run with a known reason",
			action:    PluginActionRun,
			event:     PluginEvent{Plugin: "checkmate", Version: "1.0.0", ErrorReason: "plugin_exit"},
			wantName:  "CLI - Plugin - Run",
			wantProps: base(map[string]string{"plugin": "checkmate", "plugin_version": "1.0.0", "success": "false", "error_reason": "plugin_exit"}),
		},
		{
			name:      "a failed run with free text as the reason",
			action:    PluginActionRun,
			event:     PluginEvent{Plugin: "checkmate", Version: "1.0.0", ErrorReason: "open /Users/jane/acme.us.auth0.com: denied"},
			wantName:  "CLI - Plugin - Run",
			wantProps: base(map[string]string{"plugin": "checkmate", "plugin_version": "1.0.0", "success": "false", "error_reason": "other"}),
		},
		{
			name:      "a failed run without a reason",
			action:    PluginActionRun,
			event:     PluginEvent{Plugin: "checkmate", Version: "1.0.0"},
			wantName:  "CLI - Plugin - Run",
			wantProps: base(map[string]string{"plugin": "checkmate", "plugin_version": "1.0.0", "success": "false", "error_reason": "other"}),
		},
		{
			name:      "an install ignores the outcome fields",
			action:    PluginActionInstall,
			event:     PluginEvent{Plugin: "checkmate", Version: "1.0.0", Success: true, ErrorReason: "plugin_exit"},
			wantName:  "CLI - Plugin - Install",
			wantProps: base(map[string]string{"plugin": "checkmate", "plugin_version": "1.0.0"}),
		},
		{
			name:      "an update",
			action:    PluginActionUpdate,
			event:     PluginEvent{Plugin: "checkmate", Version: "2.0.0"},
			wantName:  "CLI - Plugin - Update",
			wantProps: base(map[string]string{"plugin": "checkmate", "plugin_version": "2.0.0"}),
		},
		{
			name:      "an uninstall",
			action:    PluginActionUninstall,
			event:     PluginEvent{Plugin: "checkmate", Version: "2.0.0"},
			wantName:  "CLI - Plugin - Uninstall",
			wantProps: base(map[string]string{"plugin": "checkmate", "plugin_version": "2.0.0"}),
		},
		{
			name:       "an action that is not one of the constants",
			action:     PluginAction("Run - Everything"),
			event:      PluginEvent{Plugin: "checkmate", Version: "1.0.0"},
			wantNoSend: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tracker, transport := newRecordedTracker(t)

			tracker.TrackPlugin(test.action, test.event, "install-id")

			sent := transport.sent(t, tracker)

			if test.wantNoSend {
				assert.Empty(t, sent)
				return
			}

			require.Len(t, sent, 1)
			assert.Equal(t, test.wantName, sent[0].Event)
			assert.Equal(t, "install-id", sent[0].ID)
			assert.Equal(t, appID, sent[0].App)
			assert.Equal(t, test.wantProps, sent[0].Properties)
			assert.Equal(t, []string{analyticsEndpoint}, transport.urls)
		})
	}
}

func TestTrackPlugin_OptOut(t *testing.T) {
	t.Run("AUTH0_CLI_ANALYTICS=false sends nothing", func(t *testing.T) {
		tracker, transport := newRecordedTracker(t)
		t.Setenv("AUTH0_CLI_ANALYTICS", "false")

		for _, action := range []PluginAction{PluginActionRun, PluginActionInstall, PluginActionUpdate, PluginActionUninstall} {
			tracker.TrackPlugin(action, PluginEvent{Plugin: "checkmate", Version: "1.0.0", Success: true}, "install-id")
		}

		assert.Empty(t, transport.sent(t, tracker))
	})

	t.Run("a build without a version sends nothing", func(t *testing.T) {
		tracker, transport := newRecordedTracker(t)
		buildinfo.Version = ""

		tracker.TrackPlugin(PluginActionRun, PluginEvent{Plugin: "checkmate", Version: "1.0.0", Success: true}, "install-id")

		assert.Empty(t, transport.sent(t, tracker))
	})
}

func TestPluginErrorReason(t *testing.T) {
	for _, reason := range []string{"", "other", "none", "unclassified", "auth_failed", "PLUGIN_EXIT", "plugin_exit ", "plugin_unknown", "exit status 3"} {
		assert.Equal(t, "other", pluginErrorReason(reason), reason)
	}
}
