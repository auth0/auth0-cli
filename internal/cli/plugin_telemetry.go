package cli

import (
	"strings"

	"github.com/auth0/auth0-cli/internal/analytics"
	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
	"github.com/auth0/auth0-cli/internal/plugins/store"
)

// pluginRunIdentity is the plugin command that is running. The version is empty
// until the installed files of the plugin were verified, and a run that stops
// before that is not counted.
type pluginRunIdentity struct {
	id      string
	version string

	// Linked is true for a plugin that an author runs from a local build. Its
	// name can be private and unreleased, so its runs are never counted.
	linked bool
}

// trackablePlugin reports whether events may name the plugin. Only the plugins of
// the built-in registry do: the names of any other registry, and of a plugin that
// a developer linked, are not ours to collect.
func trackablePlugin(id string) bool {
	return strings.HasPrefix(id, registry.DefaultRegistryName+"/")
}

// trackPluginRun sends the event of a plugin run: the plugin, its version and how
// the run ended. Nothing else about the run is sent. A run that stopped before the
// plugin was verified sends nothing, and neither does a run of a linked plugin.
func trackPluginRun(c *cli, executionErr error) {
	run := c.pluginRun
	if run == nil || run.version == "" || run.linked {
		return
	}

	event := analytics.PluginEvent{Version: run.version, Success: executionErr == nil}

	if executionErr != nil {
		event.ErrorReason = errorReason(executionErr)
	}

	c.trackPlugin(analytics.PluginActionRun, run.id, event)
}

// trackPluginChange sends the event of an install or an update, when it changed
// what is installed. A plugin that was already at that version sends nothing.
func (c *cli) trackPluginChange(action analytics.PluginAction, result store.Result) {
	if result.Action == store.ActionUnchanged {
		return
	}

	c.trackPlugin(action, result.ID, analytics.PluginEvent{Version: result.Version})
}

// trackPlugin sends one plugin event with the install ID, as the other events do.
// It sends nothing for a plugin that is not from the built-in registry, without a
// version, or when there is no install ID, which is the case in env auth mode.
func (c *cli) trackPlugin(action analytics.PluginAction, id string, event analytics.PluginEvent) {
	if c.tracker == nil || !trackablePlugin(id) || event.Version == "" {
		return
	}

	installID := resolveInstallIDForTracking(c)
	if installID == "" {
		return
	}

	event.Plugin = plugins.NameOf(id)

	c.tracker.TrackPlugin(action, event, installID)
}
