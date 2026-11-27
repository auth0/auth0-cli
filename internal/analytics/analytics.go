package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/auth0/auth0-cli/internal/buildinfo"
)

const (
	eventNamePrefix   = "CLI"
	analyticsEndpoint = "https://heapanalytics.com/api/track"
	appID             = "1279799279"
	versionKey        = "version"
	osKey             = "os"
	archKey           = "arch"
	pluginKey         = "plugin"
	pluginVersionKey  = "plugin_version"
	successKey        = "success"
	errorReasonKey    = "error_reason"
)

type Tracker struct {
	wg sync.WaitGroup

	// The client that sends the events. When it is nil, a default client is used. Tests
	// replace it, so that no test reaches the analytics endpoint.
	client *http.Client
}

type event struct {
	App        string            `json:"app_id"`
	ID         string            `json:"identity"`
	Event      string            `json:"event"`
	Timestamp  int64             `json:"timestamp"`
	Properties map[string]string `json:"properties"`
}

func NewTracker() *Tracker {
	return &Tracker{}
}

// NewTrackerWithClient returns a tracker that sends its events with the given
// client. It exists so that tests can see what is sent without any request
// reaching the analytics endpoint.
func NewTrackerWithClient(client *http.Client) *Tracker {
	return &Tracker{client: client}
}

func (t *Tracker) TrackFirstLogin(id string, loginType string, properties map[string]string) {
	eventName := fmt.Sprintf("%s - Auth0 - First Login", eventNamePrefix)
	t.track(eventName, id, properties)

	eventName = fmt.Sprintf("%s - Auth0 - First Login - %s", eventNamePrefix, loginType)
	t.track(eventName, id, properties)
}

func (t *Tracker) TrackCommandRun(commandPath string, id string, properties map[string]string) {
	eventName := generateRunEventName(commandPath)
	t.track(eventName, id, properties)
}

// TrackPlugin counts one event about a plugin. The event name comes from the
// closed set of actions, and the properties are built here from the named fields
// of the event, never from a map that the caller fills, so nothing the user typed
// or received can be added to it. An action that is not one of the constants
// sends nothing.
func (t *Tracker) TrackPlugin(action PluginAction, pluginEvent PluginEvent, id string) {
	if !action.valid() {
		return
	}

	properties := map[string]string{
		pluginKey:        pluginEvent.Plugin,
		pluginVersionKey: pluginEvent.Version,
	}

	if action == PluginActionRun {
		properties[successKey] = strconv.FormatBool(pluginEvent.Success)

		if !pluginEvent.Success {
			properties[errorReasonKey] = pluginErrorReason(pluginEvent.ErrorReason)
		}
	}

	t.track(fmt.Sprintf("%s - Plugin - %s", eventNamePrefix, action), id, properties)
}

func (t *Tracker) Wait(ctx context.Context) {
	ch := make(chan struct{})

	go func() {
		t.wg.Wait()
		close(ch)
	}()

	select {
	case <-ch: // Waitgroup is done.
		return
	case <-ctx.Done():
		return
	}
}

func (t *Tracker) track(eventName string, id string, properties map[string]string) {
	if !shouldTrack() {
		return
	}

	event := newEvent(eventName, id, properties)

	t.wg.Add(1)
	go t.sendEvent(event)
}

func (t *Tracker) sendEvent(event *event) {
	jsonEvent, err := json.Marshal(event)
	if err != nil {
		return
	}

	req, err := http.NewRequest("POST", analyticsEndpoint, bytes.NewBuffer(jsonEvent))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")

	client := t.client
	if client == nil {
		client = &http.Client{}
	}

	resp, err := client.Do(req)
	if err != nil {
		println(err.Error())
		return
	}

	// Defers execute in LIFO order.
	defer t.wg.Done()
	defer func() {
		_ = resp.Body.Close()
	}()
}

func newEvent(eventName string, id string, properties map[string]string) *event {
	eventProperties := map[string]string{
		versionKey: buildinfo.Version,
		osKey:      runtime.GOOS,
		archKey:    runtime.GOARCH,
	}

	for k, v := range properties {
		eventProperties[k] = v
	}

	return &event{
		App:        appID,
		ID:         id,
		Event:      eventName,
		Timestamp:  timestamp(),
		Properties: eventProperties,
	}
}

func generateRunEventName(command string) string {
	return generateEventName(command, "Run")
}

func generateEventName(command string, action string) string {
	commands := strings.Split(command, " ")
	for i := range commands {
		commands[i] = cases.Title(language.English).String(commands[i])
	}

	switch length := len(commands); {
	case length == 1: // The root command.
		return fmt.Sprintf("%s - %s - %s", eventNamePrefix, commands[0], action)
	case length == 2: // A top-level command e.g. auth0 apps.
		return fmt.Sprintf("%s - %s - %s - %s", eventNamePrefix, commands[0], commands[1], action)
	case length >= 3:
		return fmt.Sprintf("%s - %s - %s - %s", eventNamePrefix, commands[1], strings.Join(commands[2:], " "), action)
	default:
		return eventNamePrefix
	}
}

func shouldTrack() bool {
	if os.Getenv("AUTH0_CLI_ANALYTICS") == "false" || buildinfo.Version == "" { // Do not track debug builds.
		return false
	}

	return true
}

func timestamp() int64 {
	t := time.Now()
	s := t.Unix() * 1e3
	ms := int64(t.Nanosecond()) / 1e6
	return s + ms
}

// PluginAction is what happened to a plugin. It is a closed set: the name of a
// plugin event is made from it, so a name chosen by someone else never becomes
// part of an event name.
type PluginAction string

const (
	// PluginActionRun is a run of a command of an installed plugin.
	PluginActionRun PluginAction = "Run"

	// PluginActionInstall is an install that put a plugin version on this machine.
	PluginActionInstall PluginAction = "Install"

	// PluginActionUpdate is an update that moved a plugin to another version.
	PluginActionUpdate PluginAction = "Update"

	// PluginActionUninstall is the removal of an installed plugin.
	PluginActionUninstall PluginAction = "Uninstall"
)

// valid reports whether the action is one of the constants.
func (a PluginAction) valid() bool {
	switch a {
	case PluginActionRun, PluginActionInstall, PluginActionUpdate, PluginActionUninstall:
		return true
	default:
		return false
	}
}

// PluginEvent is everything a plugin event can carry. There is no field for
// arguments, flags, a tenant, an exit code or output, so none can be sent.
type PluginEvent struct {
	// Plugin is the name of the plugin without its registry, such as checkmate.
	Plugin string

	// Version is the version of the plugin the event is about.
	Version string

	// Success and ErrorReason describe the outcome of a run, and are ignored for
	// every other action. ErrorReason is the reason code of the CLI for a failed
	// run, and is sent as it is only when it is one of the known plugin codes.
	Success     bool
	ErrorReason string
}

// otherReason is what a reason that is not on the allowlist is sent as.
const otherReason = "other"

// pluginReasons are the reason codes of the CLI that a failed plugin run can
// have. A reason that is not in the set is sent as "other", so free text can
// never reach an event.
var pluginReasons = map[string]bool{
	"plugin_approval_check":       true,
	"plugin_approvals_damaged":    true,
	"plugin_changed":              true,
	"plugin_command_missing":      true,
	"plugin_exit":                 true,
	"plugin_interactive":          true,
	"plugin_interrupted":          true,
	"plugin_no_tenant":            true,
	"plugin_not_approved":         true,
	"plugin_output_unsupported":   true,
	"plugin_registry_unavailable": true,
	"plugin_revoked":              true,
	"plugin_session_unavailable":  true,
	"plugin_start_failed":         true,
	"plugin_tenant_mismatch":      true,
	"plugin_timeout":              true,
	"plugin_timeout_invalid":      true,
	"plugin_verification_failed":  true,
}

// pluginErrorReason returns the reason if it is a known plugin reason code, and
// "other" otherwise.
func pluginErrorReason(reason string) string {
	if pluginReasons[reason] {
		return reason
	}

	return otherReason
}
