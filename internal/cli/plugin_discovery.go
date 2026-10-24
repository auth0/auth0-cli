package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/iostream"
	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
)

const (
	// PluginUpdateHintEnv turns the hint about plugin updates off when it is set to
	// a false value or to a value that is not a boolean.
	pluginUpdateHintEnv = "AUTH0_CLI_PLUGIN_UPDATE_HINT"

	// PluginHintFile is the file in the plugins directory that has the time of the
	// last hint.
	pluginHintFile = "hint.json"

	// PluginHintInterval is how long the hint stays quiet after it was shown.
	pluginHintInterval = 24 * time.Hour

	// PluginHintMaxBytes bounds what is read from the hint file, which is a few
	// dozen bytes when the CLI wrote it.
	pluginHintMaxBytes = 4096

	// PluginLookupTimeout bounds the read of the saved registry index when a
	// command is not found. It reads a few files, and never goes to the network.
	pluginLookupTimeout = 2 * time.Second
)

// offerPluginInstall turns the failure of a command that is not found into the
// suggestion to install a plugin, when the registry has a plugin of that name.
// Every other error is returned as it is.
//
// It only reads the saved index of the registry, and it fails silently: the saved
// index may not exist, may not be trusted any more, or the plugins may be off, and
// the CLI then says what it always said, that the command is unknown. It does
// nothing in a build without the plugin feature and when plugins are turned off
// with AUTH0_CLI_PLUGINS_DISABLED.
func (c *cli) offerPluginInstall(rootCmd *cobra.Command, version string, err error) error {
	unknown, ok := errors.AsType[unknownCommandError](err)
	if !ok || unknown.parent != rootCmd.CommandPath() || !plugins.FeatureEnabled(version) || checkPluginsEnabled(rootCmd) != nil {
		return err
	}

	svc, svcErr := c.getPlugins()
	if svcErr != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), pluginLookupTimeout)
	defer cancel()

	index, _, loadErr := svc.client.Load(ctx)
	if loadErr != nil {
		return err
	}

	// The suggestion is only made for a plugin of the registry of the bare name that
	// `auth0 plugin install` would install here. The schema of the index only allows
	// IDs that the bare name is read back as, so the bare name is the command.
	id := registry.DefaultRegistryName + "/" + unknown.token

	if _, runnable := runnableLatest(index, svc.store.CLIVersion, id); !runnable {
		return err
	}

	return pluginNotInstalledError{id: id, name: unknown.token, command: "auth0 plugin install " + unknown.token}
}

// hintUpdate prints one line on stderr after a plugin ran, when a newer version of
// the plugin can be installed or the installed one is deprecated, and does so at
// most once a day. It is for a person at a terminal: nothing is printed in agent
// mode, with --no-input, with an output format for machines, when stdout or stderr
// is not a terminal, or when AUTH0_CLI_PLUGIN_UPDATE_HINT turns it off. It reads
// the saved index only. It never fails and never changes the result of the
// command, so a hint file that cannot be read or written is ignored.
func (p *pluginCommand) hintUpdate(svc *pluginService, index *registry.Index, installed string) {
	c := p.cli

	if c.agentMode || c.noInput || c.json || c.jsonCompact || c.csv || !pluginHintEnabled() || !svc.terminals() {
		return
	}

	file := filepath.Join(svc.store.Dir, pluginHintFile)
	now := time.Now()

	if last, ok := readPluginHint(file); ok && !last.After(now) && now.Sub(last) <= pluginHintInterval {
		return
	}

	line := pluginHintLine(index, svc.store.CLIVersion, p.id, installed)
	if line == "" {
		return
	}

	c.renderer.Infof("%s", line)

	writePluginHint(file, now)
}

// terminals reports whether stdout and stderr are terminals.
func (s *pluginService) terminals() bool {
	if s.outputTerminals != nil {
		return s.outputTerminals()
	}

	return isatty.IsTerminal(iostream.Output.Fd()) && isatty.IsTerminal(iostream.Messages.Fd())
}

// pluginHintEnabled reports whether the hint is not turned off. A value that is
// not a boolean turns it off, as it does for the other switches.
func pluginHintEnabled() bool {
	value := strings.TrimSpace(os.Getenv(pluginUpdateHintEnv))
	if value == "" {
		return true
	}

	enabled, err := strconv.ParseBool(value)

	return err == nil && enabled
}

// pluginHintLine returns the hint about the installed version of a plugin, or an
// empty string when there is nothing to say. The hint comes from the registry, so
// its text is made safe for a terminal.
func pluginHintLine(index *registry.Index, cliVersion, id, installed string) string {
	name := plugins.NameOf(id)

	var sentences []string

	if newer := newerRunnableVersion(index, cliVersion, id, installed); newer != "" {
		sentences = append(sentences, fmt.Sprintf("%s %s is available. Run: auth0 plugin update %s", name, newer, name))
	}

	if deprecation, deprecated := index.Deprecation(id, installed); deprecated {
		sentence := fmt.Sprintf("%s is deprecated: %s.", name, strings.TrimRight(plugins.SafeText(deprecation.Message), ". "))

		if deprecation.Replacement != "" {
			sentence += fmt.Sprintf(" Use %s.", strings.TrimRight(plugins.SafeText(deprecation.Replacement), ". "))
		}

		sentences = append(sentences, sentence)
	}

	if len(sentences) == 0 {
		return ""
	}

	// The first sentence has no full stop, so that the line reads the same with and
	// without the second one.
	if len(sentences) == 2 {
		sentences[0] += "."
	}

	return strings.Join(sentences, " ")
}

// newerRunnableVersion returns the version that `auth0 plugin update` would
// install, when it is newer than the installed one and can run here: it is not
// revoked, the CLI meets its engines, and it has a build for this machine. The
// versions that are listed in a separate file are not read, because that is a
// download, so those plugins get no hint about a new version.
func newerRunnableVersion(index *registry.Index, cliVersion, id, installed string) string {
	latest, runnable := runnableLatest(index, cliVersion, id)
	if !runnable {
		return ""
	}

	newest, _ := plugins.ParseVersion(latest)

	if current, valid := plugins.ParseVersion(installed); !valid || newest.Compare(current) <= 0 {
		return ""
	}

	return latest
}

// runnableLatest returns the latest version of the plugin, when `auth0 plugin
// install` would install it here: it is a version, it is not revoked, the CLI
// meets its engines, and it has a build for this machine. The versions that are
// listed in a separate file are not read, because that is a download, so those
// plugins have no runnable latest version here.
func runnableLatest(index *registry.Index, cliVersion, id string) (string, bool) {
	listed, found := index.FindPlugin(id)
	if !found {
		return "", false
	}

	latest, valid := plugins.ParseVersion(listed.Latest)
	if !valid || latest.String() != listed.Latest {
		return "", false
	}

	if _, revoked := index.Revoked(id, listed.Latest); revoked {
		return "", false
	}

	record, found := listed.Version(listed.Latest)
	if !found {
		return "", false
	}

	if record.Engines != nil {
		if met, err := plugins.EnginesSatisfied(record.Engines.Auth0CLI, cliVersion); err != nil || !met {
			return "", false
		}
	}

	machine := plugins.CurrentPlatform()

	for _, artifact := range record.Artifacts {
		if built, ok := artifact.Platform(); ok && built == machine {
			return listed.Latest, true
		}
	}

	return "", false
}

// pluginHint is the content of the hint file.
type pluginHint struct {
	Shown time.Time `json:"shown"`
}

// readPluginHint returns the time of the last hint. A file that is missing, is not
// a regular file, is too big or is not what the CLI wrote means no hint was shown,
// and a hint cannot block a command: only a regular file is opened, so a named pipe
// does not hold the command up.
func readPluginHint(path string) (time.Time, bool) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return time.Time{}, false
	}

	file, err := os.Open(path)
	if err != nil {
		return time.Time{}, false
	}

	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, pluginHintMaxBytes+1))
	if err != nil || len(data) > pluginHintMaxBytes {
		return time.Time{}, false
	}

	var hint pluginHint
	if json.Unmarshal(data, &hint) != nil || hint.Shown.IsZero() {
		return time.Time{}, false
	}

	return hint.Shown, true
}

// writePluginHint saves the time of a hint. The file is replaced as a whole, so a
// reader never sees half of it, and a failure is ignored.
func writePluginHint(path string, shown time.Time) {
	data, err := json.Marshal(pluginHint{Shown: shown.UTC()})
	if err != nil {
		return
	}

	temporary, err := os.CreateTemp(filepath.Dir(path), ".hint-*.tmp")
	if err != nil {
		return
	}

	_, writeErr := temporary.Write(data)
	closeErr := temporary.Close()

	if writeErr != nil || closeErr != nil || os.Rename(temporary.Name(), path) != nil {
		_ = os.Remove(temporary.Name())
	}
}
