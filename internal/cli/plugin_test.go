package cli

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/auth"
	"github.com/auth0/auth0-cli/internal/buildinfo"
	"github.com/auth0/auth0-cli/internal/config"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/approvals"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
	"github.com/auth0/auth0-cli/internal/plugins/registry/registrytest"
	"github.com/auth0/auth0-cli/internal/plugins/store"
)

const (
	pluginTestID     = "auth0/checkmate"
	pluginTestBin    = "checkmate"
	pluginTestTenant = "acme.us.auth0.com"
)

// pluginEnv is a CLI with a plugin service that works on a registry served by a
// test server and on a temporary plugins directory. It does not need a build tag
// or a network: the index is signed with a throwaway key that only the client of
// the environment trusts, and downloads are sent to the test server whatever
// host the artifact URL names.
type pluginEnv struct {
	t        *testing.T
	server   *registrytest.Server
	keyID    string
	private  ed25519.PrivateKey
	platform plugins.Platform

	// Dir is the plugins directory.
	Dir string

	// Now is the time the client of the registry sees. A test moves it to make an
	// index stale or expired.
	Now time.Time

	// ManifestDescription, when it is set, is the description in the manifest of
	// the archives that release builds, instead of the one of the test plugin.
	ManifestDescription string

	// IndexVersion is the version of the index that was published last.
	indexVersion int

	// Terminal is true when a person sits at a terminal. The answers of the
	// person are set with answer, and the answer to a confirmation with Confirm.
	Terminal bool
	Confirm  bool

	// Confirmations are the questions that the commands asked the person to
	// confirm, since the last run.
	Confirmations []string

	cli    *cli
	svc    *pluginService
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

// newPluginEnv returns an environment whose registry serves nothing yet. It skips
// the test on a machine that is not one of the plugin platforms.
func newPluginEnv(t *testing.T) *pluginEnv {
	t.Helper()

	// The commands read the config file of the user, and these variables, which
	// the machine of a developer may set.
	t.Setenv("HOME", t.TempDir())

	for _, name := range []string{approvals.PinsEnv, pluginsDisabledEnv, authModeEnvVar, envDomain} {
		t.Setenv(name, "")
	}

	platform, ok := plugins.CurrentPlatform()
	if !ok {
		t.Skip("this machine is not one of the plugin platforms")
	}

	keyID, keys, private := registrytest.NewKeyPair(t)
	server := registrytest.NewServer(t)
	dir := filepath.Join(t.TempDir(), "plugins")

	e := &pluginEnv{
		t:        t,
		server:   server,
		keyID:    keyID,
		private:  private,
		platform: platform,
		Dir:      dir,
		Now:      time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC),

		indexVersion: 1,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}

	client := &registry.Client{
		Registry: server.Registry(),
		Keys:     keys,
		HTTP:     server.Client(),
		Dir:      filepath.Join(dir, "registries", registry.DefaultRegistryName),
		Now:      func() time.Time { return e.Now },
	}
	approved := &approvals.Approvals{Dir: dir, Now: func() time.Time { return e.Now }}

	e.svc = &pluginService{
		client:    client,
		approvals: approved,
		store:     &store.Store{Dir: dir, CLIVersion: "1.39.0", HTTP: server.AnyHostClient(), Approvals: approved, Versions: client},
	}
	e.svc.interactive = func(cmd *cobra.Command) bool {
		noInput, _ := cmd.Root().Flags().GetBool("no-input")

		return e.Terminal && !noInput
	}
	e.svc.confirmer = func(message string) bool {
		e.Confirmations = append(e.Confirmations, message)

		return e.Confirm
	}
	e.cli = &cli{plugins: func() (*pluginService, error) { return e.svc, nil }}

	return e
}

// answer puts a person at a terminal who types the given text, line by line.
func (e *pluginEnv) answer(text string) {
	e.Terminal = true
	e.svc.prompter = newPluginPrompter(strings.NewReader(text), e.stderr)
}

// loginAs records a login session for a tenant in the config file of the CLI.
func (e *pluginEnv) loginAs(tenant config.Tenant) {
	e.cli.Config.Tenants = config.Tenants{tenant.Domain: tenant}
}

// installedIDs returns the IDs of the installed plugins.
func (e *pluginEnv) installedIDs() []string {
	e.t.Helper()

	installed, err := e.svc.store.List()
	require.NoError(e.t, err)

	ids := []string{}
	for _, plugin := range installed {
		ids = append(ids, plugin.ID)
	}

	return ids
}

// activeVersion returns the active version of an installed plugin.
func (e *pluginEnv) activeVersion(id string) string {
	e.t.Helper()

	installed, err := e.svc.store.List()
	require.NoError(e.t, err)

	for _, plugin := range installed {
		if plugin.ID == id {
			return plugin.Active
		}
	}

	return ""
}

// approvedHashes returns the consent hashes that are approved for a plugin, by
// tenant. It reads the approvals file, so pins are not in it.
func (e *pluginEnv) approvedHashes(id string) map[string]string {
	e.t.Helper()

	records, err := e.svc.approvals.List(id)
	require.NoError(e.t, err)

	hashes := map[string]string{}
	for _, record := range records {
		hashes[record.Tenant] = record.PermissionsHash
	}

	return hashes
}

// release builds a version of the test plugin for this machine, serves its
// archive, and returns its registry record. The mutators change the record
// before it is sealed.
func (e *pluginEnv) release(version string, mutate ...func(*registry.VersionRecord)) registry.VersionRecord {
	e.t.Helper()

	return e.releaseAs(pluginTestID, e.platform, version, mutate...)
}

// releaseAs is release for another plugin, or for a build for another platform.
func (e *pluginEnv) releaseAs(id string, platform plugins.Platform, version string, mutate ...func(*registry.VersionRecord)) registry.VersionRecord {
	e.t.Helper()

	name := pluginName(id)

	record := registrytest.NewVersion(version)
	record.Platforms = []plugins.Platform{platform}

	for _, change := range mutate {
		change(&record)
	}

	record = registrytest.Seal(record)

	binary := []byte("#!/bin/sh\necho " + name + " " + version + "\n")

	// A newer major version is bigger, so that the size of the download tells the
	// versions apart. The padding is a comment that does not compress.
	block := sha256.Sum256([]byte(version))
	binary = append(binary, '#')

	for range int(version[0] - '0') {
		binary = append(binary, hex.EncodeToString(block[:])...)
		block = sha256.Sum256(block[:])
	}

	binary = append(binary, '\n')
	archive := registrytest.Package(id, record, registry.ArtifactFormatTarGz, name, binary)

	if e.ManifestDescription != "" {
		description, err := json.Marshal(e.ManifestDescription)
		require.NoError(e.t, err)

		manifest := bytes.Replace(registrytest.Manifest(id, record), []byte(`"description":"Test plugin `+id+`"`), append([]byte(`"description":`), description...), 1)
		archive = registrytest.TarGz(registrytest.ArchiveEntry{Name: name, Body: binary}, registrytest.ArchiveEntry{Name: "plugin.json", Body: manifest})
	}

	path := pluginArtifactPath(name, version)

	record.Artifacts = []registry.Artifact{
		registrytest.NewArtifact(platform, "https://github.com"+path, registry.ArtifactFormatTarGz, name, archive),
	}

	e.server.Set(path, archive)

	return record
}

// pluginArtifactPath is where the archive of a version of a plugin is served.
func pluginArtifactPath(name, version string) string {
	return "/auth0/auth0-cli-plugins/releases/download/v" + version + "/" + name + ".tar.gz"
}

// index returns a builder for an index with the test plugin, whose latest
// version is the last record.
func (e *pluginEnv) index(records ...registry.VersionRecord) *registrytest.IndexBuilder {
	return registrytest.NewIndexBuilder().AddPlugin(registrytest.NewPlugin(pluginTestID, records...))
}

// publish signs the index and serves it.
func (e *pluginEnv) publish(builder *registrytest.IndexBuilder) {
	e.t.Helper()

	e.server.Publish(builder, e.keyID, e.private)
}

// republish serves a newer index a day later, so that the client fetches it.
func (e *pluginEnv) republish(builder *registrytest.IndexBuilder) {
	e.t.Helper()

	e.indexVersion++
	e.Now = e.Now.Add(25 * time.Hour)
	e.publish(builder.SetVersion(e.indexVersion))
}

// install installs a version of the test plugin from the published index,
// without asking for consent.
func (e *pluginEnv) install(version string) store.Result {
	e.t.Helper()

	return e.installPlugin(pluginTestID, version)
}

// installPlugin is install for any plugin of the published index.
func (e *pluginEnv) installPlugin(id, version string) store.Result {
	e.t.Helper()

	index, _, err := e.svc.client.EnsureFresh(context.Background())
	require.NoError(e.t, err)

	result, err := e.svc.store.Install(context.Background(), index, id, version, store.Options{})
	require.NoError(e.t, err)

	return result
}

// run executes a command line on a new command tree, with empty output buffers,
// and returns the error of the command. Colors are off, and no terminal is
// attached, so nothing can prompt.
func (e *pluginEnv) run(args ...string) error {
	e.t.Helper()

	e.stdout.Reset()
	e.stderr.Reset()
	e.Confirmations = nil

	e.cli.renderer = &display.Renderer{MessageWriter: e.stderr, ResultWriter: e.stdout}

	root := buildRootCmd(e.cli)
	addPersistentFlags(root, e.cli)
	addSubCommands(root, e.cli)
	enforceUnknownSubcommand(root)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs(append([]string{"--no-color"}, args...))

	return root.ExecuteContext(context.Background())
}

// pluginWithDescription returns a plugin with the given ID and description, for a
// test that needs more than the test plugin.
func pluginWithDescription(id, description string, records ...registry.VersionRecord) registry.Plugin {
	plugin := registrytest.NewPlugin(id, records...)
	plugin.Description = description

	return plugin
}

// refIndex returns a builder for an index whose test plugin lists its versions
// in a separate file, as large registries do.
func (e *pluginEnv) refIndex(records ...registry.VersionRecord) *registrytest.IndexBuilder {
	return registrytest.NewIndexBuilder().AddRefPlugin(registrytest.NewPlugin(pluginTestID, records...))
}

func TestRegisterPluginCmd(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"", true},
		{"dev", true},
		{"1.39.0-beta.1", true},
		{"v1.39.0-rc.2+build", true},
		{"1.39.0", false},
		{"v1.39.0", false},
		{"1.39.0+build", false},
		{"garbage", false},
	}

	for _, test := range tests {
		t.Run(test.version, func(t *testing.T) {
			root := &cobra.Command{Use: "auth0"}
			registerPluginCmd(root, &cli{}, test.version)

			found, _, err := root.Find([]string{"plugin"})
			require.NoError(t, err)
			assert.Equal(t, test.want, found != root)

			if !test.want {
				// The alias must not leak the feature either.
				found, _, err = root.Find([]string{"plugins"})
				require.NoError(t, err)
				assert.Equal(t, root, found)
			}
		})
	}
}

func TestPluginCmd(t *testing.T) {
	group := pluginCmd(&cli{})

	assert.Equal(t, "plugin", group.Name())
	assert.Equal(t, []string{"plugins"}, group.Aliases)
	assert.Equal(t, "true", group.Annotations[skipDocsAnnotation])
	assert.False(t, group.Hidden, "the group must show in help")
	assert.True(t, strings.HasPrefix(group.Short, "(Experimental)"), group.Short)
	assert.True(t, strings.HasPrefix(group.Long, "(Experimental)"), group.Long)

	var names []string

	for _, sub := range group.Commands() {
		names = append(names, sub.Name())

		assert.NotEmpty(t, sub.Short, sub.Name())
		assert.NotEmpty(t, sub.Example, sub.Name())

		if sub.Name() == "uninstall" {
			assert.Equal(t, []string{"remove"}, sub.Aliases)
		} else {
			assert.Empty(t, sub.Aliases, sub.Name())
		}
	}

	slices.Sort(names)
	assert.Equal(t, []string{"approve", "info", "install", "list", "search", "uninstall", "update"}, names)
}

func TestSearchPluginCmd(t *testing.T) {
	t.Run("lists the plugins of the registry", func(t *testing.T) {
		e := newPluginEnv(t)
		e.publish(e.index(e.release("1.0.0"), e.release("1.1.0")))

		require.NoError(t, e.run("plugin", "search"))

		assert.Regexp(t, `(?m)^\s*NAME\s+VERSION\s+INSTALLED\s+DESCRIPTION\s*$`, e.stdout.String())
		assert.Regexp(t, `(?m)^\s*auth0/checkmate\s+1\.1\.0\s+no\s+Test plugin auth0/checkmate\s*$`, e.stdout.String())
		assert.Empty(t, e.stderr.String())
	})

	t.Run("marks an installed plugin and works through the plugins alias", func(t *testing.T) {
		e := newPluginEnv(t)
		e.publish(e.index(e.release("1.0.0")))
		e.install("1.0.0")

		require.NoError(t, e.run("plugins", "search"))

		assert.Regexp(t, `auth0/checkmate\s+1\.0\.0\s+yes\s`, e.stdout.String())
	})

	t.Run("matches the query against the ID and the description, ignoring case", func(t *testing.T) {
		e := newPluginEnv(t)
		other := pluginWithDescription("auth0/other", "Audits the tenant", e.release("1.0.0"))
		e.publish(e.index(e.release("1.0.0")).AddPlugin(other))

		tests := []struct {
			query string
			want  []string
		}{
			{"", []string{"auth0/checkmate", "auth0/other"}},
			{"CHECK", []string{"auth0/checkmate"}},
			{"audits", []string{"auth0/other"}},
			{"auth0/", []string{"auth0/checkmate", "auth0/other"}},
			{"zzz", nil},
		}

		for _, test := range tests {
			t.Run(test.query, func(t *testing.T) {
				args := []string{"plugin", "search", "--json"}
				if test.query != "" {
					args = append(args, test.query)
				}

				require.NoError(t, e.run(args...))

				var found []map[string]any
				require.NoError(t, json.Unmarshal(e.stdout.Bytes(), &found))

				ids := make([]string, 0, len(found))
				for _, plugin := range found {
					ids = append(ids, plugin["id"].(string))
				}

				assert.ElementsMatch(t, test.want, ids)
			})
		}
	})

	t.Run("says so when nothing matches", func(t *testing.T) {
		e := newPluginEnv(t)
		e.publish(e.index(e.release("1.0.0")))

		require.NoError(t, e.run("plugin", "search", "zzz"))

		assert.Empty(t, e.stdout.String())
		assert.Contains(t, e.stderr.String(), "No plugins available")
	})

	t.Run("prints stable JSON with the deprecation", func(t *testing.T) {
		e := newPluginEnv(t)
		// The registry lists the plugins in no particular order.
		builder := registrytest.NewIndexBuilder().
			AddPlugin(pluginWithDescription("auth0/other", "Fine", e.release("2.0.0"))).
			AddPlugin(registrytest.NewPlugin(pluginTestID, e.release("1.0.0"))).
			Deprecate(registry.Deprecation{ID: pluginTestID, Message: "Use the new one.", Replacement: "auth0/other"})
		e.publish(builder)

		require.NoError(t, e.run("plugin", "search", "--json-compact"))

		assert.JSONEq(t, `[
			{"id":"auth0/checkmate","name":"checkmate","version":"1.0.0","installed":false,"description":"Test plugin auth0/checkmate",
			 "deprecated":{"message":"Use the new one.","replacement":"auth0/other"}},
			{"id":"auth0/other","name":"other","version":"2.0.0","installed":false,"description":"Fine","deprecated":null}
		]`, e.stdout.String())
		assert.Empty(t, e.stderr.String())
	})

	t.Run("marks a deprecated plugin in the table and prints CSV", func(t *testing.T) {
		e := newPluginEnv(t)
		e.publish(e.index(e.release("1.0.0")).Deprecate(registry.Deprecation{ID: pluginTestID, Message: "Old."}))

		require.NoError(t, e.run("plugin", "search"))
		assert.Contains(t, e.stdout.String(), "(deprecated) Test plugin auth0/checkmate")

		require.NoError(t, e.run("plugin", "search", "--csv"))
		assert.Equal(t, "Name,Version,Installed,Description\nauth0/checkmate,1.0.0,no,(deprecated) Test plugin auth0/checkmate\n", e.stdout.String())
	})
}

func TestSearchPluginCmd_Freshness(t *testing.T) {
	t.Run("keeps the stale warning out of JSON and agent mode output", func(t *testing.T) {
		e := newPluginEnv(t)
		e.publish(e.index(e.release("1.0.0")))
		require.NoError(t, e.run("plugin", "search"))

		e.Now = e.Now.Add(7 * 24 * time.Hour)
		e.server.Fail("/index.json", 503)

		for _, flag := range []string{"--json", "--json-compact", "--agent-mode"} {
			require.NoError(t, e.run("plugin", "search", flag), flag)

			assert.Contains(t, e.stdout.String(), "auth0/checkmate", flag)
			assert.NotContains(t, e.stderr.String(), pluginStaleWarning, flag)
		}
	})

	t.Run("does not use an index that fails its signature", func(t *testing.T) {
		e := newPluginEnv(t)
		e.publish(e.index(e.release("1.0.0")))
		e.server.Set("/index.json.sig", []byte(`{"keyId":"`+e.keyID+`","algorithm":"ed25519","signature":"AAAA"}`))

		err := e.run("plugin", "search")

		require.ErrorIs(t, err, registry.ErrSignatureInvalid)
		assert.Empty(t, e.stdout.String())
	})
}

// pluginStaleWarning is what the registry client says about an index that is a
// week old and could not be refreshed.
const pluginStaleWarning = "using the cached index from 7 days ago"

// TestPluginCommands_IndexFreshness runs every command that reads the registry
// index against a saved index that is stale, expired or missing. Search works
// from a stale copy with a warning. Install and update need a current copy and
// refuse it. No command works from an expired or a missing copy, and each says
// what to run once the registry can be reached.
func TestPluginCommands_IndexFreshness(t *testing.T) {
	// The environment has version 1.0.0 installed and the index cached, and the
	// registry has a newer version that it cannot reach. A positive expiry is
	// relative to the time of the environment.
	newEnv := func(t *testing.T, expiresIn time.Duration) *pluginEnv {
		t.Helper()

		e := newPluginEnv(t)
		builder := e.index(e.release("1.0.0"), e.release("2.0.0"))

		if expiresIn > 0 {
			builder.SetExpires(e.Now.Add(expiresIn))
		}

		e.publish(builder)
		e.installPlugin(pluginTestID, "1.0.0")

		return e
	}

	commands := []struct {
		name string
		args []string
		next string

		// Uses a stale copy is true for the command that works from a stale index.
		usesStale bool
	}{
		{"search", []string{"plugin", "search"}, "Reconnect and run: auth0 plugin search", true},
		{"install", []string{"plugin", "install", "checkmate"}, "Reconnect and run: auth0 plugin install checkmate", false},
		{"update", []string{"plugin", "update", "checkmate"}, "Reconnect and run: auth0 plugin update checkmate", false},
		{"update --all", []string{"plugin", "update", "--all"}, "Reconnect and run: auth0 plugin update --all", false},
	}

	for _, command := range commands {
		t.Run(command.name+" with a stale copy", func(t *testing.T) {
			e := newEnv(t, 0)
			e.Now = e.Now.Add(7 * 24 * time.Hour)
			e.server.Fail("/index.json", 503)

			err := e.run(command.args...)

			assert.Equal(t, "1.0.0", e.activeVersion(pluginTestID))

			if command.usesStale {
				require.NoError(t, err)
				assert.Contains(t, e.stdout.String(), "auth0/checkmate")
				assert.Contains(t, e.stderr.String(), pluginStaleWarning)

				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), "a current registry index")
			assert.Contains(t, err.Error(), e.svc.client.Registry.URL)
			assert.Contains(t, err.Error(), pluginStaleWarning)
			assert.True(t, strings.HasSuffix(err.Error(), command.next), err.Error())
			assert.Empty(t, e.stdout.String())
		})

		t.Run(command.name+" with an expired copy", func(t *testing.T) {
			e := newEnv(t, time.Hour)
			e.Now = e.Now.Add(2 * time.Hour)
			e.server.Fail("/index.json", 503)

			err := e.run(command.args...)

			require.ErrorIs(t, err, registry.ErrExpired)
			assert.True(t, strings.HasSuffix(err.Error(), command.next), err.Error())
			assert.Equal(t, "1.0.0", e.activeVersion(pluginTestID))
			assert.Empty(t, e.stdout.String())
		})

		t.Run(command.name+" with a copy that expires while it is refreshed", func(t *testing.T) {
			e := newEnv(t, 8*24*time.Hour)

			// The copy is stale but valid when the refresh starts, and the refresh
			// takes so long that the copy has expired when it fails.
			e.Now = e.Now.Add(7 * 24 * time.Hour)
			e.server.Fail("/index.json", 503)
			e.server.OnRequest(func(path string, _ int) {
				if path == "/index.json" {
					e.Now = e.Now.Add(2 * 24 * time.Hour)
				}
			})

			err := e.run(command.args...)

			require.ErrorIs(t, err, registry.ErrExpired)
			assert.NotErrorIs(t, err, registry.ErrOffline)
			assert.True(t, strings.HasSuffix(err.Error(), command.next), err.Error())
		})

		t.Run(command.name+" with no copy and no registry", func(t *testing.T) {
			e := newPluginEnv(t)
			e.server.Fail("/index.json", 503)

			err := e.run(command.args...)

			require.ErrorIs(t, err, registry.ErrOffline)
			assert.True(t, strings.HasSuffix(err.Error(), command.next), err.Error())
		})
	}
}

func TestListPluginCmd(t *testing.T) {
	t.Run("says so when nothing is installed", func(t *testing.T) {
		e := newPluginEnv(t)

		require.NoError(t, e.run("plugin", "list"))
		assert.Empty(t, e.stdout.String())
		assert.Contains(t, e.stderr.String(), "No installed plugins available")

		require.NoError(t, e.run("plugin", "list", "--json"))
		assert.JSONEq(t, `[]`, e.stdout.String())
	})

	t.Run("prints stable JSON with the previous version and the size", func(t *testing.T) {
		e := newPluginEnv(t)
		e.publish(e.index(e.release("1.0.0"), e.release("2.0.0")))
		e.install("1.0.0")
		e.install("2.0.0")

		require.NoError(t, e.run("plugin", "list", "--json-compact"))

		var listed []map[string]any
		require.NoError(t, json.Unmarshal(e.stdout.Bytes(), &listed))
		require.Len(t, listed, 1)

		assert.Equal(t, "auth0/checkmate", listed[0]["id"])
		assert.Equal(t, "checkmate", listed[0]["name"])
		assert.Equal(t, "2.0.0", listed[0]["version"])
		assert.Equal(t, "1.0.0", listed[0]["previous"])
		assert.Equal(t, true, listed[0]["pinned"])
		assert.Greater(t, listed[0]["sizeBytes"], float64(0))
		assert.Len(t, listed[0], 6)
	})

	t.Run("prints CSV", func(t *testing.T) {
		e := newPluginEnv(t)
		e.publish(e.index(e.release("1.0.0")))
		e.install("1.0.0")

		require.NoError(t, e.run("plugin", "list", "--csv"))

		assert.Regexp(t, `^Name,Version,Pinned,Previous,Size\nauth0/checkmate,1\.0\.0,yes,,[^,]+\n$`, e.stdout.String())
	})
}

// withAsks gives a version of the test plugin a requirement and permissions.
func withAsks(record *registry.VersionRecord) {
	record.Engines = &plugins.Engines{Auth0CLI: ">=1.38.0"}
	record.Permissions = plugins.Permissions{
		Scopes: []string{"read:clients", "read:users"},
		Hosts:  []plugins.HostPermission{{Host: "api.example.com", Reason: "Reports"}},
		Files:  []plugins.FilePermission{},
		Env:    []plugins.EnvPermission{},
	}
}

func TestInfoPluginCmd(t *testing.T) {
	t.Run("shows the latest version of a plugin that is not installed", func(t *testing.T) {
		e := newPluginEnv(t)
		e.publish(e.index(e.release("1.0.0"), e.release("2.0.0", withAsks)))

		require.NoError(t, e.run("plugin", "info", "checkmate"))

		out := e.stdout.String()
		assert.Regexp(t, `ID\s+auth0/checkmate`, out)
		assert.Regexp(t, `VERSION\s+2\.0\.0`, out)
		assert.Regexp(t, `OWNER\s+Test team https://example\.com/bugs`, out)
		assert.Regexp(t, `REQUIRES\s+auth0-cli >=1\.38\.0`, out)
		assert.Regexp(t, `PERMISSIONS\s+2 scopes, 1 host, 0 file paths, 0 environment variables`, out)
		assert.Regexp(t, `PERMISSIONS HASH\s+sha256:[0-9a-f]{64}`, out)
		assert.Regexp(t, `VERSIONS\s+2\.0\.0 \(latest\), 1\.0\.0`, out)
		assert.Regexp(t, `INSTALLED\s+no`, out)
		assert.Regexp(t, `APPROVED TENANTS\s+none`, out)
		assert.NotContains(t, out, "DEPRECATED")
		assert.Empty(t, e.stderr.String())
	})

	t.Run("prints stable JSON", func(t *testing.T) {
		e := newPluginEnv(t)
		old, latest := e.release("1.0.0"), e.release("2.0.0", withAsks)
		e.publish(e.index(old, latest))

		require.NoError(t, e.run("plugin", "info", "auth0/checkmate", "--json-compact"))

		assert.JSONEq(t, `{
			"id": "auth0/checkmate",
			"version": "2.0.0",
			"description": "Test plugin auth0/checkmate",
			"owner": {"team": "Test team", "bugs": "https://example.com/bugs"},
			"homepage": "",
			"requires": ">=1.38.0",
			"platforms": ["`+string(e.platform)+`"],
			"permissions": {"scopes": 2, "hosts": 1, "files": 0, "env": 0},
			"permissionsHash": "`+latest.PermissionsHash+`",
			"versions": [{"version": "2.0.0", "latest": true, "revoked": false}, {"version": "1.0.0", "latest": false, "revoked": false}],
			"installed": null,
			"deprecated": null,
			"approvedTenants": []
		}`, e.stdout.String())
	})

	t.Run("shows the version that was asked for", func(t *testing.T) {
		e := newPluginEnv(t)
		old := e.release("1.0.0")
		e.publish(e.index(old, e.release("2.0.0", withAsks)))

		require.NoError(t, e.run("plugin", "info", "checkmate@1.0.0", "--json"))

		var shown map[string]any
		require.NoError(t, json.Unmarshal(e.stdout.Bytes(), &shown))

		assert.Equal(t, "1.0.0", shown["version"])
		assert.Equal(t, old.PermissionsHash, shown["permissionsHash"])
		assert.Equal(t, "", shown["requires"])
	})

	t.Run("reads the versions from the file the index points to", func(t *testing.T) {
		e := newPluginEnv(t)
		e.publish(e.refIndex(e.release("1.0.0"), e.release("1.1.0")))

		require.NoError(t, e.run("plugin", "info", "checkmate@1.0.0", "--json"))

		var shown struct {
			Version  string
			Versions []struct{ Version string }
		}
		require.NoError(t, json.Unmarshal(e.stdout.Bytes(), &shown))

		assert.Equal(t, "1.0.0", shown.Version)
		assert.Len(t, shown.Versions, 2)
	})

	t.Run("marks revoked and deprecated versions", func(t *testing.T) {
		e := newPluginEnv(t)
		builder := e.index(e.release("1.0.0"), e.release("2.0.0"))
		builder.Revoke(pluginTestID, "2.0.0", "Leaked a token.")
		builder.Deprecate(registry.Deprecation{ID: pluginTestID, Message: "Moved.", Replacement: "auth0/other"})
		e.publish(builder)

		require.NoError(t, e.run("plugin", "info", "checkmate"))

		assert.Regexp(t, `VERSIONS\s+2\.0\.0 \(latest, revoked\), 1\.0\.0`, e.stdout.String())
		assert.Regexp(t, `DEPRECATED\s+Moved\. \(use auth0/other instead\)`, e.stdout.String())

		require.NoError(t, e.run("plugin", "info", "checkmate", "--json"))
		assert.Contains(t, e.stdout.String(), `"replacement": "auth0/other"`)
		assert.Contains(t, e.stdout.String(), `"revoked": true`)
	})

	t.Run("shows the verified installed copy and the tenants that approved it", func(t *testing.T) {
		e := newPluginEnv(t)
		record := e.release("1.0.0", withAsks)
		e.publish(e.index(record))
		e.install("1.0.0")
		require.NoError(t, e.svc.approvals.Approve(context.Background(), "acme.us.auth0.com", pluginTestID, record.PermissionsHash))

		require.NoError(t, e.run("plugin", "info", "checkmate"))

		assert.Regexp(t, `INSTALLED\s+1\.0\.0 *\n`, e.stdout.String())
		assert.Regexp(t, `APPROVED TENANTS\s+acme\.us\.auth0\.com`, e.stdout.String())

		require.NoError(t, e.run("plugin", "info", "checkmate", "--json"))

		var shown struct {
			Installed       map[string]any
			ApprovedTenants []map[string]any
		}
		require.NoError(t, json.Unmarshal(e.stdout.Bytes(), &shown))

		assert.Equal(t, map[string]any{"version": "1.0.0", "verified": true, "permissionsHash": record.PermissionsHash}, shown.Installed)
		require.Len(t, shown.ApprovedTenants, 1)
		assert.Equal(t, "acme.us.auth0.com", shown.ApprovedTenants[0]["tenant"])
		assert.Equal(t, record.PermissionsHash, shown.ApprovedTenants[0]["permissionsHash"])
	})

	t.Run("still shows a plugin whose installed copy fails verification, with a warning", func(t *testing.T) {
		e := newPluginEnv(t)
		e.publish(e.index(e.release("1.0.0")))
		e.install("1.0.0")

		binary := filepath.Join(e.Dir, "store", "auth0", "checkmate", "1.0.0", pluginTestBin)
		require.NoError(t, os.WriteFile(binary, []byte("tampered"), 0o700))

		require.NoError(t, e.run("plugin", "info", "checkmate"))

		assert.Regexp(t, `INSTALLED\s+1\.0\.0 \(failed verification\)`, e.stdout.String())
		assert.Contains(t, e.stderr.String(), "The installed copy of auth0/checkmate failed verification")
		assert.Contains(t, e.stderr.String(), "auth0 plugin install checkmate")

		require.NoError(t, e.run("plugin", "info", "checkmate", "--json"))

		var shown struct{ Installed map[string]any }
		require.NoError(t, json.Unmarshal(e.stdout.Bytes(), &shown))

		assert.Equal(t, "1.0.0", shown.Installed["version"])
		assert.Equal(t, false, shown.Installed["verified"])
		assert.NotEmpty(t, shown.Installed["error"])
		assert.NotContains(t, shown.Installed, "permissionsHash")
		assert.Empty(t, e.stderr.String())
	})

	t.Run("fails on a damaged approvals file instead of showing none", func(t *testing.T) {
		e := newPluginEnv(t)
		e.publish(e.index(e.release("1.0.0")))
		require.NoError(t, os.MkdirAll(e.Dir, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(e.Dir, "approvals.json"), []byte("{"), 0o600))

		err := e.run("plugin", "info", "checkmate")

		require.Error(t, err)
		assert.Empty(t, e.stdout.String())
	})

	t.Run("orders the versions by number", func(t *testing.T) {
		e := newPluginEnv(t)
		e.publish(e.index(e.release("1.9.0"), e.release("1.2.0"), e.release("1.10.0")))

		require.NoError(t, e.run("plugin", "info", "checkmate", "--json"))

		var shown struct{ Versions []struct{ Version string } }
		require.NoError(t, json.Unmarshal(e.stdout.Bytes(), &shown))

		var versions []string
		for _, version := range shown.Versions {
			versions = append(versions, version.Version)
		}

		assert.Equal(t, []string{"1.10.0", "1.9.0", "1.2.0"}, versions)
	})
}

func TestInfoPluginCmd_Errors(t *testing.T) {
	e := newPluginEnv(t)
	e.publish(e.index(e.release("1.0.0"), e.release("2.0.0")))

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"unknown plugin", []string{"nothing"}, "nothing is not listed in the auth0 registry"},
		{"unknown version", []string{"checkmate@3.0.0"}, "checkmate 3.0.0 is not available in the auth0 registry"},
		{"no argument", nil, "accepts 1 arg(s), received 0"},
		{"other registry", []string{"other/checkmate"}, `"other" is not a registry this CLI knows`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := e.run(append([]string{"plugin", "info"}, test.args...)...)

			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
			assert.Empty(t, e.stdout.String())
		})
	}

	t.Run("lists the versions that exist", func(t *testing.T) {
		var notAvailable *store.NotAvailableError

		require.ErrorAs(t, e.run("plugin", "info", "checkmate@3.0.0"), &notAvailable)
		assert.Equal(t, []string{"2.0.0", "1.0.0"}, notAvailable.Available)
	})
}

func TestNewPluginService(t *testing.T) {
	setup := func(t *testing.T) string {
		t.Helper()

		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv(approvals.PinsEnv, "")
		t.Setenv(authModeEnvVar, "")

		return home
	}

	writeConfig := func(t *testing.T, home, content string) {
		t.Helper()

		dir := filepath.Join(home, ".config", "auth0")
		require.NoError(t, os.MkdirAll(dir, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), []byte(content), 0o600))
	}

	t.Run("works on a machine that never logged in", func(t *testing.T) {
		home := setup(t)

		svc, err := newPluginService(&cli{})
		require.NoError(t, err)

		dir := filepath.Join(home, ".config", "auth0", "plugins")

		assert.Equal(t, dir, svc.store.Dir)
		assert.Equal(t, buildinfo.Version, svc.store.CLIVersion)
		assert.Equal(t, dir, svc.approvals.Dir)
		assert.Same(t, svc.approvals, svc.store.Approvals)
		assert.Same(t, svc.client, svc.store.Versions)
		assert.Equal(t, filepath.Join(dir, "registries", "auth0"), svc.client.Dir)
		assert.Equal(t, registry.Registry{Name: "auth0", URL: registry.DefaultRegistryURL}, svc.client.Registry)
		assert.NotNil(t, svc.prompter)
	})

	t.Run("takes the mirror from the config file", func(t *testing.T) {
		home := setup(t)
		writeConfig(t, home, `{"registries":[{"name":"auth0","url":"https://mirror.example.com/index.json"}]}`)

		svc, err := newPluginService(&cli{})
		require.NoError(t, err)

		assert.Equal(t, "https://mirror.example.com/index.json", svc.client.Registry.URL)
	})

	t.Run("does not read the config file in env auth mode", func(t *testing.T) {
		home := setup(t)
		writeConfig(t, home, `{"registries":[{"name":"other","url":"https://mirror.example.com/index.json"}]}`)
		t.Setenv(authModeEnvVar, authModeEnv)

		svc, err := newPluginService(&cli{})
		require.NoError(t, err)

		assert.Equal(t, registry.DefaultRegistryURL, svc.client.Registry.URL)
	})

	t.Run("does not read the approval pins, whatever they are", func(t *testing.T) {
		setup(t)
		t.Setenv(approvals.PinsEnv, "checkmate")

		svc, err := newPluginService(&cli{})
		require.NoError(t, err)

		assert.Nil(t, svc.approvals.Pins)
	})

	failures := []struct {
		name  string
		setup func(t *testing.T, home string)
		want  error
	}{
		{
			name: "a custom registry",
			setup: func(t *testing.T, home string) {
				writeConfig(t, home, `{"registries":[{"name":"other","url":"https://mirror.example.com/index.json"}]}`)
			},
		},
		{
			name: "an insecure mirror",
			setup: func(t *testing.T, home string) {
				writeConfig(t, home, `{"registries":[{"name":"auth0","url":"http://mirror.example.com/index.json"}]}`)
			},
			want: registry.ErrInsecureURL,
		},
		{
			name:  "a damaged config file",
			setup: func(t *testing.T, home string) { writeConfig(t, home, `{`) },
		},
	}

	for _, test := range failures {
		t.Run("fails on "+test.name, func(t *testing.T) {
			home := setup(t)
			test.setup(t, home)

			svc, err := newPluginService(&cli{})

			require.Error(t, err)
			assert.Nil(t, svc)

			if test.want != nil {
				assert.True(t, errors.Is(err, test.want), err.Error())
			}
		})
	}
}

func TestPluginPrompter(t *testing.T) {
	t.Run("returns the lines one after the other and shows each prompt", func(t *testing.T) {
		var out strings.Builder

		prompter := newPluginPrompter(strings.NewReader("yes\r\nn\nlast"), &out)

		for _, want := range []string{"yes", "n", "last"} {
			got, err := prompter.ReadLine("Approve? ")
			require.NoError(t, err)
			assert.Equal(t, want, got)
		}

		assert.Equal(t, strings.Repeat("Approve? ", 3), out.String())
	})

	t.Run("returns EOF when the input has ended", func(t *testing.T) {
		prompter := newPluginPrompter(strings.NewReader("a\n"), io.Discard)

		_, err := prompter.ReadLine("")
		require.NoError(t, err)

		line, err := prompter.ReadLine("")
		assert.Equal(t, io.EOF, err)
		assert.Empty(t, line)
	})
}

func TestParsePluginRef(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  pluginRef
		fails string
	}{
		{name: "plain", value: "checkmate", want: pluginRef{ID: "auth0/checkmate", Name: "checkmate"}},
		{name: "with the registry", value: "auth0/checkmate", want: pluginRef{ID: "auth0/checkmate", Name: "checkmate"}},
		{name: "with a version", value: "checkmate@1.0.0", want: pluginRef{ID: "auth0/checkmate", Name: "checkmate", Version: "1.0.0"}},
		{name: "with the registry and a version", value: "auth0/checkmate@10.20.30", want: pluginRef{ID: "auth0/checkmate", Name: "checkmate", Version: "10.20.30"}},
		{name: "digits and hyphens", value: "a-1", want: pluginRef{ID: "auth0/a-1", Name: "a-1"}},
		{name: "empty", value: "", fails: "a plugin name is required"},
		{name: "empty name", value: "@1.0.0", fails: "not a valid plugin name"},
		{name: "empty name after the registry", value: "auth0/", fails: "not a valid plugin name"},
		{name: "other registry", value: "other/checkmate", fails: `"other" is not a registry this CLI knows: the only available registry is "auth0"`},
		{name: "uppercase name", value: "Checkmate", fails: "not a valid plugin name"},
		{name: "uppercase registry", value: "Auth0/checkmate", fails: "not a registry this CLI knows"},
		{name: "parent directory", value: "auth0/..", fails: "not a valid plugin name"},
		{name: "parent directory in the middle", value: "auth0/../checkmate", fails: "not a valid plugin name"},
		{name: "extra path part", value: "auth0/checkmate/extra", fails: "not a valid plugin name"},
		{name: "backslash", value: `auth0\checkmate`, fails: "not a valid plugin name"},
		{name: "leading hyphen", value: "-checkmate", fails: "not a valid plugin name"},
		{name: "space", value: "check mate", fails: "not a valid plugin name"},
		{name: "second at sign", value: "checkmate@1.0.0@2.0.0", fails: "not a valid version"},
		{name: "missing version", value: "checkmate@", fails: "not a valid version"},
		{name: "leading v", value: "checkmate@v1.0.0", fails: "not a valid version"},
		{name: "prerelease", value: "checkmate@1.0.0-beta.1", fails: "not a valid version"},
		{name: "build metadata", value: "checkmate@1.0.0+build", fails: "not a valid version"},
		{name: "leading zero", value: "checkmate@01.0.0", fails: "not a valid version"},
		{name: "two parts", value: "checkmate@1.0", fails: "not a valid version"},
		{name: "latest", value: "checkmate@latest", fails: "not a valid version"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parsePluginRef(test.value)

			if test.fails != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.fails)
				assert.Equal(t, pluginRef{}, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestCheckPluginNameAvailable(t *testing.T) {
	testCLI := &cli{renderer: display.NewRenderer()}
	root := buildRootCmd(testCLI)
	addSubCommands(root, testCLI)
	root.AddCommand(&cobra.Command{Use: "shadow", Aliases: []string{"darkness"}, Hidden: true})

	tests := []struct {
		name     string
		reserved bool
	}{
		{"apps", true},
		{"tenants", true},
		{"orgs", true},
		{"organizations", true},
		{"tf", true},
		{"login", true},
		{"plugin", true},
		{"plugins", true},
		{"shadow", true},
		{"darkness", true},
		{"help", true},
		{"completion", true},
		{"checkmate", false},
		{"apps-plus", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := checkPluginNameAvailable(root, test.name)

			if !test.reserved {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.Equal(t, `auth0 already has a command named "`+test.name+`". Plugin names cannot shadow core commands`, err.Error())
		})
	}
}

// withScopes gives a version of the test plugin permissions to the given scopes,
// which must be sorted.
func withScopes(scopes ...string) func(*registry.VersionRecord) {
	return func(record *registry.VersionRecord) {
		record.Permissions = plugins.Permissions{
			Scopes: scopes,
			Hosts:  []plugins.HostPermission{},
			Files:  []plugins.FilePermission{},
			Env:    []plugins.EnvPermission{},
		}
	}
}

// newInstallEnv returns an environment whose registry serves version 1.0.0 of the
// test plugin, which asks for scopes, and the record of the version.
func newInstallEnv(t *testing.T) (*pluginEnv, registry.VersionRecord) {
	t.Helper()

	e := newPluginEnv(t)
	record := e.release("1.0.0", withAsks)
	e.publish(e.index(record))

	return e, record
}

// assertValidationError fails when the error is not a validation error with the
// reason, which is what the JSON error envelope has as its code and reason.
func assertValidationError(t *testing.T, err error, reason string) {
	t.Helper()

	assert.Equal(t, "validation", errorClass(err))
	assert.Equal(t, reason, errorReason(err))
}

// jsonObject returns the standard output of the last run, which must be one JSON
// object and nothing else.
func (e *pluginEnv) jsonObject() map[string]any {
	e.t.Helper()

	var object map[string]any
	require.NoError(e.t, json.Unmarshal(e.stdout.Bytes(), &object), e.stdout.String())

	return object
}

func TestInstallPluginCmd(t *testing.T) {
	t.Run("names the plugin, shows the permissions, installs and then records the approval", func(t *testing.T) {
		e, record := newInstallEnv(t)
		e.answer("y\n")

		require.NoError(t, e.run("plugin", "install", "checkmate", "--tenant", pluginTestTenant))

		out := e.stderr.String()
		line := strings.Index(out, "auth0/checkmate 1.0.0  Test plugin auth0/checkmate")
		download := strings.Index(out, "Download: ")
		screen := strings.Index(out, "This plugin asks to use your Auth0 tenant "+pluginTestTenant)
		question := strings.Index(out, "Approve for "+pluginTestTenant+"? [y/N/d]  (d shows the exact scopes) ")
		installed := strings.Index(out, "Installed auth0/checkmate 1.0.0.")

		assert.True(t, 0 <= line && line < download && download < screen && screen < question && question < installed, out)
		assert.Contains(t, out, "("+string(e.platform)+")")
		assert.NotContains(t, out, "not approved")
		assert.Empty(t, e.stdout.String())
		assert.Equal(t, []string{pluginTestID}, e.installedIDs())
		assert.Equal(t, map[string]string{pluginTestTenant: record.PermissionsHash}, e.approvedHashes(pluginTestID))
	})

	t.Run("asks, and installs only for a clear yes", func(t *testing.T) {
		tests := []struct {
			name      string
			input     string
			installed bool
			screens   int
		}{
			{"y", "y\n", true, 1},
			{"yes in capitals", " YES \n", true, 1},
			{"the exact scopes, then yes", "d\ny\n", true, 1},
			{"n", "n\n", false, 1},
			{"nothing typed", "\n", false, 1},
			{"input that ended", "", false, 1},
			{"something else", "sure\n", false, 1},
			{"the exact scopes, then no", "d\nn\n", false, 1},
			{"the exact scopes twice", "d\nd\n", false, 1},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				e := newPluginEnv(t)
				e.publish(e.index(e.release("1.0.0", withScopes("read:actions", "read:clients", "read:logs", "read:roles", "read:users"))))
				e.answer(test.input)

				require.NoError(t, e.run("plugin", "install", "checkmate", "--tenant", pluginTestTenant, "--json"))

				out := e.stderr.String()
				assert.Equal(t, test.screens, strings.Count(out, "This plugin asks to use your Auth0 tenant"), out)
				assert.Equal(t, strings.Contains(test.input, "d"), strings.Contains(out, "Exact scopes"), out)

				if test.installed {
					assert.Equal(t, "installed", e.jsonObject()["action"])
					assert.Equal(t, []string{pluginTestID}, e.installedIDs())
					assert.Len(t, e.approvedHashes(pluginTestID), 1)

					return
				}

				// The view says what happened, as update --all does for a declined plugin.
				assert.Equal(t, map[string]any{"id": pluginTestID, "version": "", "previousVersion": "", "action": "declined", "pinned": false, "approvalRequired": false}, e.jsonObject())
				assert.Empty(t, e.installedIDs())
				assert.Empty(t, e.approvedHashes(pluginTestID))
				assert.Contains(t, out, "auth0/checkmate was not approved, so nothing was installed.")
				assert.NotContains(t, out, "Installed")
			})
		}
	})

	t.Run("installs without an approval where nobody can approve, and says how to approve", func(t *testing.T) {
		tests := []struct {
			name  string
			setup func(e *pluginEnv)
			args  []string
		}{
			{"no terminal", func(*pluginEnv) {}, nil},
			{"--no-input at a terminal", func(e *pluginEnv) { e.answer("y\n") }, []string{"--no-input"}},
			{"a tenant in capitals", func(*pluginEnv) {}, []string{"--tenant", "ACME.us.auth0.com"}},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				e, _ := newInstallEnv(t)
				test.setup(e)

				require.NoError(t, e.run(append([]string{"plugin", "install", "checkmate", "--tenant", pluginTestTenant}, test.args...)...))

				out := e.stderr.String()
				assert.Contains(t, out, "Installed auth0/checkmate 1.0.0.")
				assert.Contains(t, out, "Approve it in a terminal with: auth0 plugin approve auth0/checkmate --tenant acme.us.auth0.com")
				assert.NotContains(t, out, "This plugin asks to use your Auth0 tenant")
				assert.NotContains(t, out, "Approve for")
				assert.Equal(t, []string{pluginTestID}, e.installedIDs())
				assert.Empty(t, e.approvedHashes(pluginTestID))
			})
		}
	})

	t.Run("prints one JSON object and no human text on standard output", func(t *testing.T) {
		for _, flag := range []string{"--json", "--json-compact"} {
			e, _ := newInstallEnv(t)

			require.NoError(t, e.run("plugin", "install", "checkmate", "--tenant", pluginTestTenant, flag), flag)

			assert.JSONEq(t, `{"id":"auth0/checkmate","version":"1.0.0","previousVersion":"","action":"installed","pinned":false,"approvalRequired":true,
				"approveCommand":"auth0 plugin approve auth0/checkmate --tenant acme.us.auth0.com"}`, e.stdout.String(), flag)
			assert.Contains(t, e.stderr.String(), "Installed auth0/checkmate 1.0.0.", flag)
		}
	})

	t.Run("installs in agent mode without an approval, even at a terminal", func(t *testing.T) {
		e, _ := newInstallEnv(t)
		e.answer("y\n")

		require.NoError(t, e.run("plugin", "install", "checkmate", "--tenant", pluginTestTenant, "--agent-mode", "--no-input=false"))

		assert.Equal(t, true, e.jsonObject()["approvalRequired"])
		assert.Equal(t, "auth0 plugin approve auth0/checkmate --tenant acme.us.auth0.com", e.jsonObject()["approveCommand"])
		assert.Empty(t, e.stderr.String())
		assert.Equal(t, []string{pluginTestID}, e.installedIDs())
		assert.Empty(t, e.approvedHashes(pluginTestID))
	})

	t.Run("installs without consent when there is no tenant, and says to log in", func(t *testing.T) {
		e, _ := newInstallEnv(t)
		e.answer("y\n")

		require.NoError(t, e.run("plugin", "install", "checkmate"))

		out := e.stderr.String()
		assert.Contains(t, out, "Installed auth0/checkmate 1.0.0.")
		assert.Contains(t, out, "Log in with auth0 login, then approve it in a terminal with: auth0 plugin approve auth0/checkmate --tenant <tenant>")
		assert.NotContains(t, out, "This plugin asks to use your Auth0 tenant")
		assert.NotContains(t, out, "The plugin asks for")
		assert.Empty(t, e.approvedHashes(pluginTestID))

		require.NoError(t, e.run("plugin", "install", "checkmate", "--json"))
		assert.Equal(t, true, e.jsonObject()["approvalRequired"])
		assert.NotContains(t, e.jsonObject(), "approveCommand", "there is no tenant to name in a command")
	})

	t.Run("needs no approval for permissions that are approved already", func(t *testing.T) {
		tests := []struct {
			name     string
			approve  func(t *testing.T, e *pluginEnv, record registry.VersionRecord)
			terminal bool
		}{
			{
				name: "by a recorded approval",
				approve: func(t *testing.T, e *pluginEnv, record registry.VersionRecord) {
					require.NoError(t, e.svc.approvals.Approve(context.Background(), pluginTestTenant, pluginTestID, record.PermissionsHash))
				},
			},
			{
				name: "by a pin",
				approve: func(t *testing.T, _ *pluginEnv, record registry.VersionRecord) {
					t.Setenv(approvals.PinsEnv, pluginTestID+"@"+record.PermissionsHash)
				},
			},
			{
				name: "by a recorded approval, at a terminal where nothing is asked",
				approve: func(t *testing.T, e *pluginEnv, record registry.VersionRecord) {
					require.NoError(t, e.svc.approvals.Approve(context.Background(), pluginTestTenant, pluginTestID, record.PermissionsHash))
				},
				terminal: true,
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				e, record := newInstallEnv(t)
				test.approve(t, e, record)

				if test.terminal {
					e.answer("")
				}

				require.NoError(t, e.run("plugin", "install", "checkmate", "--tenant", pluginTestTenant, "--json"))

				assert.Equal(t, false, e.jsonObject()["approvalRequired"])
				assert.NotContains(t, e.jsonObject(), "approveCommand")
				assert.NotContains(t, e.stderr.String(), "approve")
				assert.NotContains(t, e.stderr.String(), "This plugin asks to use your Auth0 tenant")
				assert.Equal(t, []string{pluginTestID}, e.installedIDs())
			})
		}
	})

	t.Run("reports what it did, and pins an exact version", func(t *testing.T) {
		e := newPluginEnv(t)
		e.publish(e.index(e.release("1.0.0"), e.release("2.0.0")))

		steps := []struct {
			ref, version, action, previous, said string
			pinned                               bool
		}{
			{"checkmate@1.0.0", "1.0.0", "installed", "", "Installed auth0/checkmate 1.0.0.", true},
			{"checkmate@1.0.0", "1.0.0", "unchanged", "1.0.0", "auth0/checkmate 1.0.0 is already installed.", true},
			{"checkmate@2.0.0", "2.0.0", "installed", "1.0.0", "Switched auth0/checkmate from 1.0.0 to 2.0.0.", true},
			{"checkmate@1.0.0", "1.0.0", "switched", "2.0.0", "Switched auth0/checkmate from 2.0.0 to 1.0.0.", true},
			{"checkmate", "2.0.0", "switched", "1.0.0", "Switched auth0/checkmate from 1.0.0 to 2.0.0.", false},
		}

		for _, step := range steps {
			require.NoError(t, e.run("plugin", "install", step.ref, "--json"), step.ref)

			assert.Equal(t, step.version, e.jsonObject()["version"], step.ref)
			assert.Equal(t, step.action, e.jsonObject()["action"], step.ref)
			assert.Equal(t, step.previous, e.jsonObject()["previousVersion"], step.ref)
			assert.Equal(t, step.pinned, e.jsonObject()["pinned"], step.ref)
			assert.Contains(t, e.stderr.String(), step.said, step.ref)
			assert.Equal(t, step.pinned, strings.Contains(e.stderr.String(), "auth0/checkmate is pinned to "+step.version), step.ref)
		}
	})
}

func TestInstallPluginCmd_Refusals(t *testing.T) {
	elsewhere := plugins.PlatformDarwinArm64
	if elsewhere == mustPlatform(t) {
		elsewhere = plugins.PlatformWindowsX64
	}

	tests := []struct {
		name  string
		setup func(e *pluginEnv)
		ref   string
		want  string
		is    func(error) bool
	}{
		{
			name: "a revoked version",
			setup: func(e *pluginEnv) {
				e.publish(e.index(e.release("1.0.0"), e.release("2.0.0")).Revoke(pluginTestID, "2.0.0", "Leaked a token."))
			},
			ref:  "checkmate@2.0.0",
			want: "checkmate 2.0.0 was revoked: Leaked a token. Install a newer version with: auth0 plugin update checkmate",
			is:   func(err error) bool { var target *store.RevokedError; return errors.As(err, &target) },
		},
		{
			name: "a version with no build for this machine",
			setup: func(e *pluginEnv) {
				e.publish(e.index(e.release("1.0.0"), e.releaseAs(pluginTestID, elsewhere, "2.0.0")))
			},
			ref:  "checkmate@2.0.0",
			want: "checkmate 2.0.0 has no build for ",
			is:   func(err error) bool { var target *store.NoBuildError; return errors.As(err, &target) },
		},
		{
			name: "a version that needs a newer CLI",
			setup: func(e *pluginEnv) {
				e.publish(e.index(e.release("1.0.0", func(r *registry.VersionRecord) { r.Engines = &plugins.Engines{Auth0CLI: ">=9.0.0"} })))
			},
			ref:  "checkmate",
			want: "checkmate 1.0.0 requires auth0-cli >=9.0.0. You have 1.39.0.",
			is:   func(err error) bool { var target *store.EnginesError; return errors.As(err, &target) },
		},
		{
			name:  "a version the registry does not list",
			setup: func(e *pluginEnv) { e.publish(e.index(e.release("1.0.0"))) },
			ref:   "checkmate@3.0.0",
			want:  "checkmate 3.0.0 is not available in the auth0 registry.\nAvailable versions: 1.0.0 (latest)",
			is:    func(err error) bool { var target *store.NotAvailableError; return errors.As(err, &target) },
		},
		{
			name:  "a plugin the registry does not list",
			setup: func(e *pluginEnv) { e.publish(e.index(e.release("1.0.0"))) },
			ref:   "nothing",
			want:  "nothing is not listed in the auth0 registry.",
			is:    func(err error) bool { var target *store.NotAvailableError; return errors.As(err, &target) },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := newPluginEnv(t)
			test.setup(e)
			e.answer("y\n")

			err := e.run("plugin", "install", test.ref, "--tenant", pluginTestTenant)

			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
			assert.True(t, test.is(err), err.Error())
			assert.Empty(t, e.installedIDs())
			assert.Empty(t, e.approvedHashes(pluginTestID))
			assert.NotContains(t, e.stderr.String(), "This plugin asks to use your Auth0 tenant")
			assert.Empty(t, e.stdout.String())
		})
	}

	t.Run("a plugin named like a core command, without touching the registry", func(t *testing.T) {
		for _, name := range []string{"apps", "orgs", "tf", "plugin", "plugins", "help", "completion"} {
			e := newPluginEnv(t)

			err := e.run("plugin", "install", name)

			require.Error(t, err, name)
			assertValidationError(t, err, "plugin_name_reserved")
			assert.Equal(t, `auth0 already has a command named "`+name+`". Plugin names cannot shadow core commands`, err.Error())
			assert.Zero(t, e.server.Requests("/index.json"), name)
			assert.Empty(t, e.installedIDs())
		}
	})

	t.Run("a reference that is not one", func(t *testing.T) {
		for _, ref := range []string{"other/checkmate", "Checkmate", "auth0/../x", "checkmate@latest", "checkmate@v1.0.0", ""} {
			e, _ := newInstallEnv(t)

			var invalid validationError

			require.ErrorAs(t, e.run("plugin", "install", ref), &invalid, ref)
			assert.Zero(t, e.server.Requests("/index.json"), ref)
			assert.Empty(t, e.installedIDs())
		}
	})
}

// mustPlatform returns the platform of this machine.
func mustPlatform(t *testing.T) plugins.Platform {
	t.Helper()

	platform, ok := plugins.CurrentPlatform()
	if !ok {
		t.Skip("this machine is not one of the plugin platforms")
	}

	return platform
}

func TestInstallPluginCmd_WarnsAboutScopes(t *testing.T) {
	declared := withScopes("read:clients", "read:hooks", "read:signing_keys")
	session := func(extra ...string) config.Tenant {
		return config.Tenant{Domain: pluginTestTenant, Scopes: append(slices.Clone(auth.RequiredScopes), extra...)}
	}

	tests := []struct {
		name     string
		setup    func(t *testing.T, e *pluginEnv)
		args     []string
		want     string
		terminal bool
	}{
		{
			name:     "a login that lacks scopes is told to log in again, keeping the scopes it has",
			setup:    func(_ *testing.T, e *pluginEnv) { e.loginAs(session("read:signing_keys")) },
			want:     `have: read:hooks. Its calls that need them are denied until you log in again with: auth0 login --domain acme.us.auth0.com --scopes "read:hooks,read:signing_keys"`,
			terminal: true,
		},
		{
			name:  "a login that lacks scopes is told so without a terminal as well",
			setup: func(_ *testing.T, e *pluginEnv) { e.loginAs(session()) },
			want:  `auth0 login --domain acme.us.auth0.com --scopes "read:hooks,read:signing_keys"`,
		},
		{
			name:  "a tenant typed in capitals is the tenant of the login",
			setup: func(_ *testing.T, e *pluginEnv) { e.loginAs(session()) },
			args:  []string{"--tenant", "ACME.us.auth0.com"},
			want:  `auth0 login --domain acme.us.auth0.com --scopes "read:hooks,read:signing_keys"`,
		},
		{
			name:     "a login that has every scope is told nothing",
			setup:    func(_ *testing.T, e *pluginEnv) { e.loginAs(session("read:hooks", "read:signing_keys")) },
			terminal: true,
		},
		{
			name: "client credentials are told to grant the scopes",
			setup: func(_ *testing.T, e *pluginEnv) {
				e.loginAs(config.Tenant{Domain: pluginTestTenant, ClientID: "client"})
			},
			want:     "The plugin asks for the scopes read:clients, read:hooks, read:signing_keys. The application you are logged in with needs a grant for them",
			terminal: true,
		},
		{
			name: "env auth mode says the scopes could not be checked",
			setup: func(t *testing.T, _ *pluginEnv) {
				t.Setenv(authModeEnvVar, authModeEnv)
				t.Setenv(envDomain, pluginTestTenant)
			},
			args:     []string{},
			want:     "The scopes of your session could not be checked",
			terminal: true,
		},
		{
			name:     "a tenant that is not in the config says the scopes could not be checked",
			setup:    func(_ *testing.T, e *pluginEnv) { e.loginAs(session()) },
			args:     []string{"--tenant", "other.us.auth0.com"},
			want:     "The scopes of your session could not be checked",
			terminal: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := newPluginEnv(t)
			e.publish(e.index(e.release("1.0.0", declared)))
			test.setup(t, e)

			args := []string{"plugin", "install", "checkmate"}
			if test.args == nil {
				args = append(args, "--tenant", pluginTestTenant)
			}

			args = append(args, test.args...)

			if test.terminal {
				e.answer("y\n")
			}

			require.NoError(t, e.run(args...))

			out := e.stderr.String()
			assert.Equal(t, []string{pluginTestID}, e.installedIDs(), "a warning never stops the install")

			if test.want == "" {
				assert.NotContains(t, out, "The plugin asks for")
				return
			}

			assert.Contains(t, out, test.want)

			if test.terminal {
				question, warning, done := strings.Index(out, "Approve for"), strings.Index(out, "The plugin asks for"), strings.Index(out, "Installed")
				assert.True(t, 0 <= question && question < warning && warning < done, out)
			}
		})
	}

	t.Run("says nothing for a plugin that asks for no scopes", func(t *testing.T) {
		e := newPluginEnv(t)
		e.publish(e.index(e.release("1.0.0", withScopes([]string{}...))))
		e.answer("y\n")

		require.NoError(t, e.run("plugin", "install", "checkmate", "--tenant", "other.us.auth0.com"))

		assert.NotContains(t, e.stderr.String(), "The plugin asks for")
		assert.Equal(t, []string{pluginTestID}, e.installedIDs())
	})

	t.Run("says nothing in agent mode", func(t *testing.T) {
		e := newPluginEnv(t)
		e.publish(e.index(e.release("1.0.0", declared)))
		e.loginAs(session())

		require.NoError(t, e.run("plugin", "install", "checkmate", "--tenant", pluginTestTenant, "--agent-mode"))

		assert.Empty(t, e.stderr.String())
	})
}

func TestPluginsDisabled(t *testing.T) {
	disabled := [][]string{
		{"plugin", "install", "checkmate"},
		{"plugin", "update", "checkmate"},
		{"plugin", "update", "--all"},
		{"plugin", "approve", "checkmate", "--tenant", pluginTestTenant},
	}

	notABool := `AUTH0_CLI_PLUGINS_DISABLED is set to %q, which is not true or false, so plugins are turned off on this machine`

	tests := []struct {
		value string
		off   bool
		want  string
	}{
		{"true", true, "AUTH0_CLI_PLUGINS_DISABLED=true"},
		{"TRUE", true, "AUTH0_CLI_PLUGINS_DISABLED=true"},
		{" True ", true, "AUTH0_CLI_PLUGINS_DISABLED=true"},
		{"1", true, "AUTH0_CLI_PLUGINS_DISABLED=true"},
		{"yes", true, fmt.Sprintf(notABool, "yes")},
		{"ttrue", true, fmt.Sprintf(notABool, "ttrue")},
		{"on", true, fmt.Sprintf(notABool, "on")},
		{"false", false, ""},
		{"0", false, ""},
		{"", false, ""},
	}

	for _, test := range tests {
		t.Run(strconv.Quote(test.value), func(t *testing.T) {
			e, _ := newInstallEnv(t)
			t.Setenv(pluginsDisabledEnv, test.value)

			if !test.off {
				require.NoError(t, e.run("plugin", "install", "checkmate"))
				assert.Equal(t, []string{pluginTestID}, e.installedIDs())

				return
			}

			e.install("1.0.0")
			requests := e.server.Requests("/index.json")
			e.answer("y\n")

			for _, args := range disabled {
				err := e.run(args...)

				require.Error(t, err, args)
				assert.Contains(t, err.Error(), test.want)
				assert.Contains(t, err.Error(), "auth0 plugin "+args[1])

				if !strings.Contains(test.want, "=true") {
					assert.Contains(t, err.Error(), "Set it to true to keep plugins off, or unset it to allow them")
				}
			}

			assert.Equal(t, requests, e.server.Requests("/index.json"), "nothing is fetched")
			assert.Empty(t, e.approvedHashes(pluginTestID))
		})
	}
}

// TestPluginCommands_ReadingIsNotBlocked checks that a machine where plugins are
// turned off, or where the approval pins are wrong, can still search, list and
// inspect plugins. Uninstalling is checked with the registry down, in
// TestPluginCommands_WorkWithoutTheRegistry.
func TestPluginCommands_ReadingIsNotBlocked(t *testing.T) {
	for name, value := range map[string]string{pluginsDisabledEnv: "yes", approvals.PinsEnv: "checkmate"} {
		t.Run(name, func(t *testing.T) {
			e, _ := newInstallEnv(t)
			e.install("1.0.0")
			t.Setenv(name, value)

			for _, args := range [][]string{{"plugin", "search"}, {"plugin", "list"}, {"plugin", "info", "checkmate"}} {
				require.NoError(t, e.run(args...), args)
			}
		})
	}
}

// TestPluginApprovalPins_FailTheCommandsThatUseThem checks that a bad value of
// the approval pins fails install, update and approve before they do anything.
func TestPluginApprovalPins_FailTheCommandsThatUseThem(t *testing.T) {
	tests := []struct {
		args      []string
		installed bool
	}{
		{[]string{"plugin", "install", "checkmate"}, false},
		{[]string{"plugin", "install", "checkmate@1.0.0"}, true},
		{[]string{"plugin", "update", "checkmate"}, true},
		{[]string{"plugin", "update", "--all"}, true},
		{[]string{"plugin", "approve", "checkmate", "--tenant", pluginTestTenant}, true},
	}

	for _, test := range tests {
		e, _ := newInstallEnv(t)

		active := ""
		if test.installed {
			e.install("1.0.0")

			active = "1.0.0"
		}

		t.Setenv(approvals.PinsEnv, "checkmate")
		e.answer("y\n")
		requests := e.server.Requests("/index.json")

		err := e.run(test.args...)

		var invalid validationError

		require.ErrorAs(t, err, &invalid, test.args)
		require.ErrorIs(t, err, approvals.ErrBadPins, test.args)
		assert.Contains(t, err.Error(), approvals.PinsEnv)
		assert.Equal(t, requests, e.server.Requests("/index.json"), test.args)
		assert.Equal(t, active, e.activeVersion(pluginTestID), test.args)
		assert.Empty(t, e.approvedHashes(pluginTestID), test.args)
	}
}

func TestPluginTenant(t *testing.T) {
	tests := []struct {
		name          string
		flag          string
		defaultTenant string
		env           map[string]string
		want          string
	}{
		{name: "none"},
		{name: "the flag", flag: " acme.us.auth0.com ", want: "acme.us.auth0.com"},
		{name: "the flag in capitals", flag: " ACME.us.Auth0.com ", want: "acme.us.auth0.com"},
		{name: "the default tenant in capitals", defaultTenant: "Default.us.auth0.com", want: "default.us.auth0.com"},
		{name: "the domain in capitals in env auth mode", env: map[string]string{authModeEnvVar: authModeEnv, envDomain: "ENV.us.auth0.com"}, want: "env.us.auth0.com"},
		{name: "the default tenant", defaultTenant: "default.us.auth0.com", want: "default.us.auth0.com"},
		{name: "the flag wins over the default tenant", flag: "flag.us.auth0.com", defaultTenant: "default.us.auth0.com", want: "flag.us.auth0.com"},
		{name: "the domain in env auth mode", env: map[string]string{authModeEnvVar: authModeEnv, envDomain: " env.us.auth0.com "}, want: "env.us.auth0.com"},
		{name: "the flag wins over the domain in env auth mode", flag: "flag.us.auth0.com", env: map[string]string{authModeEnvVar: authModeEnv, envDomain: "env.us.auth0.com"}, want: "flag.us.auth0.com"},
		{name: "the domain is ignored outside env auth mode", defaultTenant: "default.us.auth0.com", env: map[string]string{envDomain: "env.us.auth0.com"}, want: "default.us.auth0.com"},
		{name: "env auth mode without a domain", env: map[string]string{authModeEnvVar: authModeEnv}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(authModeEnvVar, "")
			t.Setenv(envDomain, "")

			for name, value := range test.env {
				t.Setenv(name, value)
			}

			c := &cli{tenant: test.flag}
			c.Config.DefaultTenant = test.defaultTenant

			assert.Equal(t, test.want, pluginTenant(c))
		})
	}
}

// newUpdateEnv returns an environment that has version 1.0.0 of the test plugin
// installed, not pinned, whose registry serves version 2.0.0 as well, which the
// client sees after a day. The mutators change the two versions.
func newUpdateEnv(t *testing.T, old, newer func(*registry.VersionRecord)) (*pluginEnv, registry.VersionRecord, registry.VersionRecord) {
	t.Helper()

	e := newPluginEnv(t)
	before, after := e.release("1.0.0", old), e.release("2.0.0", newer)
	e.publish(e.index(before))
	e.installPlugin(pluginTestID, "")
	e.republish(e.index(before, after))

	return e, before, after
}

func TestUpdatePluginCmd(t *testing.T) {
	newEnv := newUpdateEnv

	t.Run("updates without a word about permissions that did not change", func(t *testing.T) {
		e, _, _ := newEnv(t, withAsks, withAsks)

		require.NoError(t, e.run("plugin", "update", "checkmate", "--tenant", pluginTestTenant, "--json"))

		assert.JSONEq(t, `{"id":"auth0/checkmate","version":"2.0.0","previousVersion":"1.0.0","action":"installed","pinned":false,"approvalRequired":true,
			"approveCommand":"auth0 plugin approve auth0/checkmate --tenant acme.us.auth0.com"}`, e.stdout.String())
		assert.Contains(t, e.stderr.String(), "Updated auth0/checkmate from 1.0.0 to 2.0.0.")
		assert.NotContains(t, e.stderr.String(), "Permissions")
		assert.NotContains(t, e.stderr.String(), "different permissions")
		assert.Equal(t, "2.0.0", e.activeVersion(pluginTestID))
	})

	t.Run("clears the pin of an exact install", func(t *testing.T) {
		e := newPluginEnv(t)
		e.publish(e.index(e.release("1.0.0"), e.release("2.0.0")))
		e.install("1.0.0")

		require.NoError(t, e.run("plugin", "update", "checkmate", "--json"))

		assert.Equal(t, false, e.jsonObject()["pinned"])
		assert.Equal(t, "2.0.0", e.jsonObject()["version"])
	})

	t.Run("needs no approval when the tenant approved the new permissions already", func(t *testing.T) {
		e, _, after := newEnv(t, withAsks, withAsks)
		require.NoError(t, e.svc.approvals.Approve(context.Background(), pluginTestTenant, pluginTestID, after.PermissionsHash))

		require.NoError(t, e.run("plugin", "update", "checkmate", "--tenant", pluginTestTenant, "--json"))

		assert.Equal(t, false, e.jsonObject()["approvalRequired"])
		assert.NotContains(t, e.stderr.String(), "approve")
	})

	t.Run("shows what changed when the tenant approved the installed version, and everything when not", func(t *testing.T) {
		tests := []struct {
			name        string
			approvedOld bool
			want        string
			notWant     string
		}{
			{"the installed version was approved", true, "auth0/checkmate 2.0.0 asks for different permissions on tenant acme.us.auth0.com\n  Added        can read logs", "This plugin asks to use your Auth0 tenant"},
			{"the installed version was not approved", false, "This plugin asks to use your Auth0 tenant acme.us.auth0.com", "asks for different permissions"},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				e, before, after := newEnv(t, withScopes("read:clients"), withScopes("read:clients", "read:logs"))
				e.answer("y\n")

				if test.approvedOld {
					require.NoError(t, e.svc.approvals.Approve(context.Background(), pluginTestTenant, pluginTestID, before.PermissionsHash))
				}

				require.NoError(t, e.run("plugin", "update", "checkmate", "--tenant", pluginTestTenant))

				out := e.stderr.String()
				assert.Contains(t, out, "auth0/checkmate 2.0.0  Test plugin auth0/checkmate")
				// The test archives are under a kilobyte, which prints as bytes.
				require.Less(t, after.Artifacts[0].Size, int64(1000))
				assert.Contains(t, out, fmt.Sprintf("Download: %d B (%s)", after.Artifacts[0].Size, e.platform))
				assert.NotEqual(t, before.Artifacts[0].Size, after.Artifacts[0].Size)
				assert.Contains(t, out, test.want)
				assert.NotContains(t, out, test.notWant)
				assert.Contains(t, out, "Updated auth0/checkmate from 1.0.0 to 2.0.0.")
				assert.Equal(t, "2.0.0", e.activeVersion(pluginTestID))
				assert.Equal(t, map[string]string{pluginTestTenant: after.PermissionsHash}, e.approvedHashes(pluginTestID))
			})
		}
	})

	t.Run("keeps the installed version when the person says no", func(t *testing.T) {
		for _, input := range []string{"n\n", ""} {
			e, before, _ := newEnv(t, withScopes("read:clients"), withScopes("read:clients", "read:logs"))
			require.NoError(t, e.svc.approvals.Approve(context.Background(), pluginTestTenant, pluginTestID, before.PermissionsHash))
			e.answer(input)

			require.NoError(t, e.run("plugin", "update", "checkmate", "--tenant", pluginTestTenant, "--json"))

			// The view says what happened, as update --all does for a declined plugin.
			assert.Equal(t, map[string]any{"id": pluginTestID, "version": "1.0.0", "previousVersion": "", "action": "declined", "pinned": false, "approvalRequired": false}, e.jsonObject())
			assert.Contains(t, e.stderr.String(), "auth0/checkmate was not updated, because the new permissions were not approved. 1.0.0 stays active.")
			assert.Equal(t, "1.0.0", e.activeVersion(pluginTestID))
			assert.Equal(t, map[string]string{pluginTestTenant: before.PermissionsHash}, e.approvedHashes(pluginTestID))
		}
	})

	t.Run("keeps the installed version, says why and fails where nobody can approve", func(t *testing.T) {
		tests := []struct {
			name  string
			setup func(e *pluginEnv)
			args  []string
			want  string
		}{
			{"no terminal", func(*pluginEnv) {}, nil, "there is no terminal to ask in"},
			{"--no-input at a terminal", func(e *pluginEnv) { e.answer("y\n") }, []string{"--no-input"}, "there is no terminal to ask in"},
			{"agent mode at a terminal", func(e *pluginEnv) { e.answer("y\n") }, []string{"--agent-mode", "--no-input=false"}, "an agent cannot approve plugins"},
			{"no tenant", func(e *pluginEnv) { e.answer("y\n") }, []string{"--tenant", ""}, "there is no tenant to approve them for"},
		}

		for _, test := range tests {
			for _, target := range [][]string{{"checkmate"}, {"--all"}} {
				t.Run(test.name+" "+strings.Join(target, ""), func(t *testing.T) {
					e, before, _ := newEnv(t, withScopes("read:clients"), withScopes("read:clients", "read:logs"))
					require.NoError(t, e.svc.approvals.Approve(context.Background(), pluginTestTenant, pluginTestID, before.PermissionsHash))
					test.setup(e)

					args := append([]string{"plugin", "update"}, target...)
					if !slices.Contains(test.args, "--tenant") {
						args = append(args, "--tenant", pluginTestTenant)
					}

					err := e.run(append(args, test.args...)...)

					require.Error(t, err)
					assert.Contains(t, e.stdout.String()+e.stderr.String()+err.Error(), test.want)
					assert.Equal(t, "1.0.0", e.activeVersion(pluginTestID))
					assert.Equal(t, map[string]string{pluginTestTenant: before.PermissionsHash}, e.approvedHashes(pluginTestID))

					if target[0] == "checkmate" {
						assert.Contains(t, err.Error(), "checkmate 2.0.0 asks for permissions that are not approved, and "+test.want+". checkmate was not updated, and 1.0.0 stays active. Run this in a terminal to review them: auth0 plugin update checkmate")
					} else {
						assert.Contains(t, err.Error(), "1 plugin could not be updated: checkmate")
					}
				})
			}
		}
	})

	t.Run("is a no-op for a plugin that is up to date", func(t *testing.T) {
		e, _, _ := newEnv(t, withAsks, withAsks)

		require.NoError(t, e.run("plugin", "update", "checkmate"))
		require.NoError(t, e.run("plugin", "update", "checkmate", "--json"))

		assert.Equal(t, "unchanged", e.jsonObject()["action"])
		assert.Contains(t, e.stderr.String(), "auth0/checkmate 2.0.0 is up to date.")
		assert.NotContains(t, e.stderr.String(), "already installed")
	})

	t.Run("says how to install a plugin when none is installed", func(t *testing.T) {
		e, _ := newInstallEnv(t)

		err := e.run("plugin", "update", "checkmate")

		var missing *store.NotInstalledError

		require.ErrorAs(t, err, &missing)
		assert.Contains(t, err.Error(), "Install it with: auth0 plugin install checkmate")
	})

	t.Run("asks for exactly one target", func(t *testing.T) {
		e, _, _ := newEnv(t, withAsks, withAsks)

		tests := []struct {
			args []string
			want string
		}{
			{[]string{"plugin", "update"}, "give the name of one plugin, or use --all"},
			{[]string{"plugin", "update", "checkmate", "--all"}, "give the name of one plugin, or use --all"},
			{[]string{"plugin", "update", "checkmate@2.0.0"}, "update takes the name of a plugin, without a version. To install an exact version, run: auth0 plugin install checkmate@2.0.0"},
			{[]string{"plugin", "update", "other/checkmate"}, "is not a registry this CLI knows"},
			{[]string{"plugin", "update", "nothing"}, "nothing is not installed. Install it with: auth0 plugin install nothing"},
		}

		for _, test := range tests {
			err := e.run(test.args...)

			require.Error(t, err, test.args)
			assert.Contains(t, err.Error(), test.want)
			assert.Equal(t, "1.0.0", e.activeVersion(pluginTestID))
		}
	})
}

// pluginOtherID is a second plugin, for the tests that work on more than one.
const pluginOtherID = "auth0/other"

// newPairEnv returns an environment that has auth0/checkmate and auth0/other
// installed at 1.0.0, both with the given permissions, and whose registry serves
// version 2.0.0 of both with other permissions, which the client sees after a
// day. The test plugin is pinned to 1.0.0 when pinned is true, and the other one
// never is. The records returned are the 2.0.0 ones.
func newPairEnv(t *testing.T, pinned bool, old, newer func(*registry.VersionRecord)) (*pluginEnv, registry.VersionRecord, registry.VersionRecord) {
	t.Helper()

	e := newPluginEnv(t)
	first1, first2 := e.release("1.0.0", old), e.release("2.0.0", newer)
	other1, other2 := e.releaseAs(pluginOtherID, e.platform, "1.0.0", old), e.releaseAs(pluginOtherID, e.platform, "2.0.0", newer)

	e.publish(e.index(first1).AddPlugin(registrytest.NewPlugin(pluginOtherID, other1)))

	version := ""
	if pinned {
		version = "1.0.0"
	}

	e.installPlugin(pluginTestID, version)
	e.installPlugin(pluginOtherID, "")
	e.republish(e.index(first1, first2).AddPlugin(registrytest.NewPlugin(pluginOtherID, other1, other2)))

	return e, first2, other2
}

func TestUpdatePluginCmd_AllFlag(t *testing.T) {
	flag := updatePluginCmd(&cli{}).Flags().Lookup("all")

	require.NotNil(t, flag)
	assert.Equal(t, "Update every installed plugin that is not pinned.", flag.Usage)
	assert.Equal(t, "false", flag.DefValue)
	assert.Equal(t, "bool", flag.Value.Type())
	assert.Empty(t, flag.Shorthand)
}

func TestUpdatePluginCmd_All(t *testing.T) {
	const otherID = pluginOtherID

	// The environment has auth0/checkmate pinned to 1.0.0, and auth0/other not
	// pinned at 1.0.0. The registry serves version 2.0.0 of both after a day.
	newEnv := func(t *testing.T, old, newer func(*registry.VersionRecord)) *pluginEnv {
		t.Helper()

		e, _, _ := newPairEnv(t, true, old, newer)

		return e
	}

	t.Run("skips the pinned plugins, says which version is available, and updates the others", func(t *testing.T) {
		e := newEnv(t, withAsks, withAsks)

		require.NoError(t, e.run("plugin", "update", "--all", "--tenant", pluginTestTenant))

		out := e.stderr.String()
		assert.Contains(t, out, "auth0/checkmate is pinned to 1.0.0 and was skipped. Version 2.0.0 is available: auth0 plugin update checkmate")
		assert.Contains(t, out, "Updated auth0/other from 1.0.0 to 2.0.0.")
		assert.Empty(t, e.stdout.String())
		assert.Equal(t, "1.0.0", e.activeVersion(pluginTestID))
		assert.Equal(t, "2.0.0", e.activeVersion(otherID))
	})

	t.Run("prints one JSON entry per plugin", func(t *testing.T) {
		e := newEnv(t, withAsks, withAsks)

		require.NoError(t, e.run("plugin", "update", "--all", "--tenant", pluginTestTenant, "--json-compact"))

		assert.JSONEq(t, `[
			{"id":"auth0/checkmate","version":"1.0.0","previousVersion":"","action":"skipped","pinned":true,"approvalRequired":false,"available":"2.0.0"},
			{"id":"auth0/other","version":"2.0.0","previousVersion":"1.0.0","action":"installed","pinned":false,"approvalRequired":true,
			 "approveCommand":"auth0 plugin approve auth0/other --tenant acme.us.auth0.com"}
		]`, e.stdout.String())
	})

	t.Run("says so when a pinned plugin is on the newest version", func(t *testing.T) {
		e := newPluginEnv(t)
		e.publish(e.index(e.release("1.0.0")))
		e.install("1.0.0")

		require.NoError(t, e.run("plugin", "update", "--all", "--json"))

		assert.Contains(t, e.stderr.String(), "auth0/checkmate is pinned to 1.0.0, which is the newest version.")
		assert.JSONEq(t, `[{"id":"auth0/checkmate","version":"1.0.0","previousVersion":"","action":"skipped","pinned":true,"approvalRequired":false}]`, e.stdout.String())
	})

	t.Run("prints a list, not an object, for one plugin", func(t *testing.T) {
		e := newPluginEnv(t)
		e.publish(e.index(e.release("1.0.0")))
		e.installPlugin(pluginTestID, "")

		require.NoError(t, e.run("plugin", "update", "--all", "--json"))

		var entries []map[string]any
		require.NoError(t, json.Unmarshal(e.stdout.Bytes(), &entries))
		assert.Len(t, entries, 1)
		assert.Equal(t, "unchanged", entries[0]["action"])
		assert.Contains(t, e.stderr.String(), "auth0/checkmate 1.0.0 is up to date.")
	})

	t.Run("says so when nothing is installed", func(t *testing.T) {
		e, _ := newInstallEnv(t)

		require.NoError(t, e.run("plugin", "update", "--all"))
		assert.Contains(t, e.stderr.String(), "No installed plugins available")

		require.NoError(t, e.run("plugin", "update", "--all", "--json"))
		assert.JSONEq(t, `[]`, e.stdout.String())
	})

	t.Run("does not fail for a plugin it skipped or a person who declined", func(t *testing.T) {
		e := newEnv(t, withScopes("read:clients"), withScopes("read:clients", "read:logs"))
		e.answer("n\n")

		require.NoError(t, e.run("plugin", "update", "--all", "--tenant", pluginTestTenant, "--json"))

		var entries []map[string]any
		require.NoError(t, json.Unmarshal(e.stdout.Bytes(), &entries))
		require.Len(t, entries, 2)
		assert.Equal(t, "skipped", entries[0]["action"])
		assert.Equal(t, "declined", entries[1]["action"])
		assert.Equal(t, "1.0.0", e.activeVersion(otherID))
		assert.Empty(t, e.approvedHashes(otherID))
	})

	t.Run("records the approvals of the plugins that were updated, and fails only when an update failed", func(t *testing.T) {
		tests := []struct {
			name string

			// FailOther makes the download of the second plugin fail, after the person
			// approved both.
			failOther bool
		}{
			{"every plugin was updated", false},
			{"the second download fails", true},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				e, first, other := newPairEnv(t, false, withScopes("read:clients"), withScopes("read:clients", "read:logs"))
				e.answer("y\ny\n")

				if test.failOther {
					e.server.Fail(pluginArtifactPath("other", "2.0.0"), 503)
				}

				err := e.run("plugin", "update", "--all", "--tenant", pluginTestTenant, "--json")

				var entries []map[string]any
				require.NoError(t, json.Unmarshal(e.stdout.Bytes(), &entries))
				require.Len(t, entries, 2)

				assert.Equal(t, 2, strings.Count(e.stderr.String(), "Approve for"))
				assert.Equal(t, map[string]string{pluginTestTenant: first.PermissionsHash}, e.approvedHashes(pluginTestID))
				assert.Equal(t, "2.0.0", e.activeVersion(pluginTestID))

				if test.failOther {
					require.EqualError(t, err, "1 plugin could not be updated: other")
					assert.Equal(t, "failed", entries[1]["action"])
					assert.Contains(t, entries[1]["error"], "could not be downloaded")
					assert.Empty(t, e.approvedHashes(otherID))
					assert.Equal(t, "1.0.0", e.activeVersion(otherID))

					return
				}

				require.NoError(t, err)
				assert.Equal(t, "installed", entries[1]["action"])
				assert.Equal(t, map[string]string{pluginTestTenant: other.PermissionsHash}, e.approvedHashes(otherID))
			})
		}
	})
}

func TestUninstallPluginCmd(t *testing.T) {
	// The environment has the test plugin installed, approved for a tenant.
	newEnv := func(t *testing.T) *pluginEnv {
		t.Helper()

		e, record := newInstallEnv(t)
		e.install("1.0.0")
		require.NoError(t, e.svc.approvals.Approve(context.Background(), pluginTestTenant, pluginTestID, record.PermissionsHash))

		return e
	}

	t.Run("asks, removes every trace and says how much space it freed", func(t *testing.T) {
		e := newEnv(t)
		e.Terminal, e.Confirm = true, true

		installed, err := e.svc.store.List()
		require.NoError(t, err)

		size := installed[0].Size

		require.NoError(t, e.run("plugin", "uninstall", "checkmate"))

		assert.Equal(t, []string{"Are you sure you want to uninstall auth0/checkmate? This removes every version and its approvals."}, e.Confirmations)
		// The size is under a kilobyte, which prints as bytes.
		require.Less(t, size, int64(1000))
		assert.Contains(t, e.stderr.String(), fmt.Sprintf("Uninstalled auth0/checkmate and freed %d B.", size))
		assert.Empty(t, e.installedIDs())
		assert.Empty(t, e.approvedHashes(pluginTestID))
		assert.NoDirExists(t, filepath.Join(e.Dir, "store", "auth0", "checkmate"))
		assert.NotContains(t, readFile(t, filepath.Join(e.Dir, "plugins.lock.json")), "checkmate")
	})

	t.Run("does nothing when the person does not confirm", func(t *testing.T) {
		e := newEnv(t)
		e.Terminal = true

		require.NoError(t, e.run("plugin", "uninstall", "checkmate"))

		assert.Len(t, e.Confirmations, 1)
		assert.Contains(t, e.stderr.String(), "auth0/checkmate was not uninstalled.")
		assert.Equal(t, []string{pluginTestID}, e.installedIDs())
		assert.Len(t, e.approvedHashes(pluginTestID), 1)
	})

	t.Run("asks nothing with --force, at a terminal, with the alias, and where nobody can answer", func(t *testing.T) {
		tests := []struct {
			name     string
			args     []string
			terminal bool
		}{
			{"--force", []string{"plugin", "uninstall", "checkmate", "--force"}, true},
			{"the remove alias", []string{"plugin", "remove", "checkmate", "--force"}, false},
			{"--force in agent mode", []string{"plugin", "uninstall", "checkmate", "--force", "--agent-mode"}, false},
			{"no terminal", []string{"plugin", "uninstall", "checkmate"}, false},
			{"--no-input at a terminal", []string{"plugin", "uninstall", "checkmate", "--no-input"}, true},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				e := newEnv(t)
				e.Terminal = test.terminal

				require.NoError(t, e.run(test.args...))

				assert.Empty(t, e.Confirmations)
				assert.Empty(t, e.installedIDs())
				assert.Empty(t, e.approvedHashes(pluginTestID))
			})
		}
	})

	t.Run("refuses in agent mode without --force", func(t *testing.T) {
		e := newEnv(t)
		e.Terminal, e.Confirm = true, true

		err := e.run("plugin", "uninstall", "checkmate", "--agent-mode")

		require.ErrorIs(t, err, errDestructiveNoConfirm)
		assert.Empty(t, e.Confirmations)
		assert.Equal(t, []string{pluginTestID}, e.installedIDs())
	})

	t.Run("removes a damaged approvals file with the plugin", func(t *testing.T) {
		e := newEnv(t)
		require.NoError(t, os.WriteFile(filepath.Join(e.Dir, "approvals.json"), []byte("{"), 0o600))

		require.NoError(t, e.run("plugin", "uninstall", "checkmate", "--force"))

		assert.Empty(t, e.installedIDs())
		assert.Empty(t, e.approvedHashes(pluginTestID))
	})

	t.Run("lists the installed plugins when the plugin is not installed", func(t *testing.T) {
		e := newEnv(t)
		e.republish(e.index(e.release("1.0.0")).AddPlugin(registrytest.NewPlugin("auth0/other", e.releaseAs("auth0/other", e.platform, "1.0.0"))))
		e.installPlugin("auth0/other", "")
		e.Terminal, e.Confirm = true, true

		err := e.run("plugin", "uninstall", "nothing")

		require.EqualError(t, err, "the plugin auth0/nothing is not installed. Installed plugins: auth0/checkmate, auth0/other")
		assertValidationError(t, err, "plugin_not_installed")
		assert.Empty(t, e.Confirmations)
	})

	t.Run("says so when no plugin is installed", func(t *testing.T) {
		e := newPluginEnv(t)

		err := e.run("plugin", "uninstall", "checkmate")

		require.EqualError(t, err, "the plugin auth0/checkmate is not installed, and no plugin is. Find one with: auth0 plugin search")
		assertValidationError(t, err, "plugin_not_installed")
	})

	t.Run("takes a name, not a version or another registry", func(t *testing.T) {
		e := newEnv(t)

		require.ErrorContains(t, e.run("plugin", "uninstall", "checkmate@1.0.0", "--force"), "uninstall takes the name of a plugin, without a version")
		require.ErrorContains(t, e.run("plugin", "uninstall", "other/checkmate", "--force"), "is not a registry this CLI knows")
		assert.Equal(t, []string{pluginTestID}, e.installedIDs())
	})
}

func TestUninstallPluginCmd_RealService(t *testing.T) {
	// The environment serves the plugin, and the home directory is a temporary
	// one, so the real service works in a temporary plugins directory.
	e, record := newInstallEnv(t)
	index, _, err := e.svc.client.EnsureFresh(context.Background())
	require.NoError(t, err)

	svc, err := newPluginService(&cli{})
	require.NoError(t, err)

	svc.store.HTTP = e.server.AnyHostClient()
	_, err = svc.store.Install(context.Background(), index, pluginTestID, "1.0.0", store.Options{})
	require.NoError(t, err)
	require.NoError(t, svc.approvals.Approve(context.Background(), pluginTestTenant, pluginTestID, record.PermissionsHash))

	// The command builds its own service, as it does outside the tests, and
	// an uninstall does not read the approval pins.
	t.Setenv(approvals.PinsEnv, "checkmate")

	c := &cli{renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: io.Discard}}
	root := buildRootCmd(c)
	addPersistentFlags(root, c)
	addSubCommands(root, c)
	enforceUnknownSubcommand(root)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--no-color", "plugin", "uninstall", "checkmate", "--force"})

	require.NoError(t, root.ExecuteContext(context.Background()))

	installed, err := svc.store.List()
	require.NoError(t, err)
	assert.Empty(t, installed)

	approved, err := svc.approvals.List(pluginTestID)
	require.NoError(t, err)
	assert.Empty(t, approved)
}

// readFile returns the content of a file.
func readFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	return string(content)
}

func TestApprovePluginCmd(t *testing.T) {
	// The environment has the test plugin installed without an approval.
	newEnv := func(t *testing.T) (*pluginEnv, registry.VersionRecord) {
		t.Helper()

		e, record := newInstallEnv(t)
		e.install("1.0.0")

		return e, record
	}

	t.Run("names the plugin, shows the permissions and records the approval of a person", func(t *testing.T) {
		e, record := newEnv(t)
		e.answer("y\n")

		require.NoError(t, e.run("plugin", "approve", "checkmate", "--tenant", pluginTestTenant))

		out := e.stderr.String()
		line := strings.Index(out, "auth0/checkmate 1.0.0  Test plugin auth0/checkmate")
		screen := strings.Index(out, "This plugin asks to use your Auth0 tenant "+pluginTestTenant)
		question := strings.Index(out, "Approve for "+pluginTestTenant+"? [y/N/d]  (d shows the exact scopes) ")
		done := strings.Index(out, "Approved auth0/checkmate 1.0.0 for "+pluginTestTenant+".")

		assert.True(t, 0 <= line && line < screen && screen < question && question < done, out)
		assert.Equal(t, map[string]string{pluginTestTenant: record.PermissionsHash}, e.approvedHashes(pluginTestID))
		assert.Empty(t, e.stdout.String())
	})

	t.Run("takes the tenant from the default tenant, and approves one tenant at a time", func(t *testing.T) {
		e, record := newEnv(t)
		e.cli.Config.DefaultTenant = "default.us.auth0.com"
		e.answer("y\ny\n")

		require.NoError(t, e.run("plugin", "approve", "auth0/checkmate"))
		require.NoError(t, e.run("plugin", "approve", "checkmate", "--tenant", pluginTestTenant))

		assert.Equal(t, map[string]string{"default.us.auth0.com": record.PermissionsHash, pluginTestTenant: record.PermissionsHash}, e.approvedHashes(pluginTestID))
	})

	t.Run("records nothing when the person says no", func(t *testing.T) {
		for _, input := range []string{"n\n", "", "\n", "d\nn\n"} {
			e, _ := newEnv(t)
			e.answer(input)

			require.NoError(t, e.run("plugin", "approve", "checkmate", "--tenant", pluginTestTenant), input)

			assert.Contains(t, e.stderr.String(), "auth0/checkmate was not approved for "+pluginTestTenant+".", input)
			assert.Empty(t, e.approvedHashes(pluginTestID), input)
		}
	})

	t.Run("works with approval pins for other plugins and writes none to disk", func(t *testing.T) {
		e, record := newEnv(t)
		t.Setenv(approvals.PinsEnv, "auth0/other@sha256:"+strings.Repeat("a", 64))
		e.answer("y\n")

		require.NoError(t, e.run("plugin", "approve", "checkmate", "--tenant", pluginTestTenant))

		assert.Equal(t, map[string]string{pluginTestTenant: record.PermissionsHash}, e.approvedHashes(pluginTestID))
		assert.NotContains(t, readFile(t, filepath.Join(e.Dir, "approvals.json")), strings.Repeat("a", 64))
	})

	t.Run("is a person's decision: never in agent mode and never without a terminal", func(t *testing.T) {
		tests := []struct {
			name     string
			args     []string
			terminal bool
		}{
			{"agent mode at a terminal", []string{"--agent-mode", "--no-input=false"}, true},
			{"agent mode", []string{"--agent-mode"}, false},
			{"--no-input at a terminal", []string{"--no-input"}, true},
			{"no terminal", nil, false},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				e, _ := newEnv(t)
				e.answer("y\n")
				e.Terminal = test.terminal

				err := e.run(append([]string{"plugin", "approve", "checkmate", "--tenant", pluginTestTenant}, test.args...)...)

				require.EqualError(t, err, "only a person can approve a plugin. Run this in a terminal: auth0 plugin approve auth0/checkmate --tenant acme.us.auth0.com")
				assert.NotContains(t, e.stderr.String(), "This plugin asks to use your Auth0 tenant")
				assert.Empty(t, e.approvedHashes(pluginTestID))
			})
		}
	})

	t.Run("needs a tenant", func(t *testing.T) {
		e, _ := newEnv(t)
		e.answer("y\n")

		err := e.run("plugin", "approve", "checkmate")

		require.ErrorIs(t, err, errPluginNoTenant)
		assertValidationError(t, err, "plugin_no_tenant")
		assert.Empty(t, e.approvedHashes(pluginTestID))
	})

	t.Run("fails when the approvals cannot be trusted", func(t *testing.T) {
		e, _ := newEnv(t)
		e.answer("y\n")
		require.NoError(t, os.WriteFile(filepath.Join(e.Dir, "approvals.json"), []byte("{"), 0o600))

		require.ErrorIs(t, e.run("plugin", "approve", "checkmate", "--tenant", pluginTestTenant), approvals.ErrDamaged)
	})

	t.Run("needs an installed plugin that passes verification", func(t *testing.T) {
		e, _ := newEnv(t)
		e.answer("y\ny\n")

		err := e.run("plugin", "approve", "nothing", "--tenant", pluginTestTenant)
		require.EqualError(t, err, "nothing is not installed. Install it with: auth0 plugin install nothing")

		require.NoError(t, os.WriteFile(filepath.Join(e.Dir, "store", "auth0", "checkmate", "1.0.0", pluginTestBin), []byte("tampered"), 0o700))

		err = e.run("plugin", "approve", "checkmate", "--tenant", pluginTestTenant)

		var failed *store.VerificationError
		require.ErrorAs(t, err, &failed)
		assert.Contains(t, err.Error(), "the installed copy of auth0/checkmate failed verification, so it cannot be approved")
		assert.Contains(t, err.Error(), "Reinstall it with: auth0 plugin install checkmate")
		assert.Empty(t, e.approvedHashes(pluginTestID))
		assert.NotContains(t, e.stderr.String(), "This plugin asks to use your Auth0 tenant")
	})

	t.Run("takes a name, not a version", func(t *testing.T) {
		e, _ := newEnv(t)
		e.answer("y\n")

		require.ErrorContains(t, e.run("plugin", "approve", "checkmate@1.0.0", "--tenant", pluginTestTenant), "approve takes the name of a plugin, without a version")
		assert.Empty(t, e.approvedHashes(pluginTestID))
	})
}

// TestPluginCommands_RefuseTwoOutputFormats gives each command that prints a
// result two output flags at once, and expects it to refuse before it does
// anything.
func TestPluginCommands_RefuseTwoOutputFormats(t *testing.T) {
	tests := []struct {
		name    string
		command []string
		flags   []string

		// ReadOnly is true for a command that can be run with the first flag alone.
		readOnly bool

		// Active is the version of the test plugin that is installed afterwards.
		active string
	}{
		{"search", []string{"plugin", "search"}, []string{"--json", "--csv"}, true, ""},
		{"info", []string{"plugin", "info", "checkmate"}, []string{"--json", "--json-compact"}, true, ""},
		{"install", []string{"plugin", "install", "checkmate"}, []string{"--json", "--json-compact"}, false, ""},
		{"update", []string{"plugin", "update", "checkmate"}, []string{"--json", "--json-compact"}, false, "1.0.0"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e, _ := newInstallEnv(t)

			if test.active != "" {
				e.install(test.active)
			}

			if test.readOnly {
				require.NoError(t, e.run(slices.Concat(test.command, test.flags[:1])...))
			}

			require.Error(t, e.run(slices.Concat(test.command, test.flags)...))
			assert.Equal(t, test.active, e.activeVersion(pluginTestID))
		})
	}
}

// TestPluginCommands_AgentModePrintsOnlyJSON checks that the commands that print
// a result put JSON on standard output in agent mode, and nothing on standard
// error.
func TestPluginCommands_AgentModePrintsOnlyJSON(t *testing.T) {
	for _, command := range [][]string{{"plugin", "search"}, {"plugin", "info", "checkmate"}} {
		t.Run(command[1], func(t *testing.T) {
			e, _ := newInstallEnv(t)

			require.NoError(t, e.run(append(command, "--agent-mode")...))

			assert.True(t, json.Valid(e.stdout.Bytes()), e.stdout.String())
			assert.Empty(t, e.stderr.String())
		})
	}
}

// TestPluginCommands_FailOnDamagedLockfile checks that every command that reads
// the lockfile fails when it is damaged, instead of treating it as empty.
func TestPluginCommands_FailOnDamagedLockfile(t *testing.T) {
	for _, command := range [][]string{{"plugin", "search"}, {"plugin", "list"}, {"plugin", "info", "checkmate"}, {"plugin", "update", "--all"}} {
		t.Run(strings.Join(command[1:], " "), func(t *testing.T) {
			e, _ := newInstallEnv(t)
			require.NoError(t, os.MkdirAll(e.Dir, 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(e.Dir, "plugins.lock.json"), []byte("{"), 0o600))

			err := e.run(command...)

			require.ErrorIs(t, err, store.ErrLockfileDamaged)
			assert.Empty(t, e.stdout.String())
		})
	}
}

// TestPluginCommands_RecordNoApprovalWhenTheDownloadFails checks that a person
// who approved a plugin at the consent screen has no approval recorded when the
// download then fails, and that the installed version stays active.
func TestPluginCommands_RecordNoApprovalWhenTheDownloadFails(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		fails  string
		active string
	}{
		{"install", []string{"plugin", "install", "checkmate"}, "1.0.0", ""},
		{"update", []string{"plugin", "update", "checkmate"}, "2.0.0", "1.0.0"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// An update starts from version 1.0.0, and needs new permissions to ask for.
			e, _ := newInstallEnv(t)
			if test.active != "" {
				e, _, _ = newUpdateEnv(t, withScopes("read:clients"), withScopes("read:clients", "read:logs"))
			}

			e.server.Fail(pluginArtifactPath("checkmate", test.fails), 503)
			e.answer("y\n")

			err := e.run(append(test.args, "--tenant", pluginTestTenant)...)

			require.ErrorIs(t, err, store.ErrDownloadFailed)
			assert.Contains(t, e.stderr.String(), "Approve for")
			assert.Equal(t, test.active, e.activeVersion(pluginTestID))
			assert.Empty(t, e.approvedHashes(pluginTestID))
		})
	}
}

// TestPluginCommands_WorkWithoutTheRegistry checks that the commands that only
// need what is on the machine still work when the registry cannot be reached,
// and that they do not even try. Uninstall also works where plugins are turned
// off and the approval pins are wrong.
func TestPluginCommands_WorkWithoutTheRegistry(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		env    map[string]string
		answer string
		check  func(t *testing.T, e *pluginEnv)
	}{
		{
			name: "list",
			args: []string{"plugin", "list"},
			check: func(t *testing.T, e *pluginEnv) {
				assert.Regexp(t, `(?m)^\s*NAME\s+VERSION\s+PINNED\s+PREVIOUS\s+SIZE\s*$`, e.stdout.String())
				assert.Regexp(t, `auth0/checkmate\s+1\.0\.0\s+yes\s+\d+(\.\d)? (B|kB|MB)\s*$`, e.stdout.String())
			},
		},
		{
			name: "uninstall, with plugins turned off and bad approval pins",
			args: []string{"plugin", "uninstall", "checkmate", "--force"},
			env:  map[string]string{pluginsDisabledEnv: "true", approvals.PinsEnv: "checkmate"},
			check: func(t *testing.T, e *pluginEnv) {
				assert.Empty(t, e.installedIDs())
			},
		},
		{
			name:   "approve",
			args:   []string{"plugin", "approve", "checkmate", "--tenant", pluginTestTenant},
			answer: "y\n",
			check: func(t *testing.T, e *pluginEnv) {
				assert.Len(t, e.approvedHashes(pluginTestID), 1)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e, _ := newInstallEnv(t)
			e.install("1.0.0")

			// The saved index is due for a refresh, so a command that reads it asks the
			// registry.
			e.Now = e.Now.Add(25 * time.Hour)
			requests := e.server.Requests("/index.json")
			e.server.Fail("/index.json", 503)

			for name, value := range test.env {
				t.Setenv(name, value)
			}

			if test.answer != "" {
				e.answer(test.answer)
			}

			require.NoError(t, e.run(test.args...))

			assert.Equal(t, requests, e.server.Requests("/index.json"))
			test.check(t, e)
		})
	}
}

// TestPluginCommands_FailBeforeDownloading checks that a tenant that cannot be
// used and an approvals file that cannot be trusted stop install, update and
// approve before anything is downloaded, installed or shown, instead of after.
func TestPluginCommands_FailBeforeDownloading(t *testing.T) {
	tests := []struct {
		name    string
		tenant  string
		damaged bool
		want    string
	}{
		{"a URL", "https://acme.us.auth0.com", false, `"https://acme.us.auth0.com" is not a tenant domain`},
		{"a trailing dot", "acme.us.auth0.com.", false, `"acme.us.auth0.com." is not a tenant domain`},
		{"a space", "acme us", false, `"acme us" is not a tenant domain`},
		{"a damaged approvals file", pluginTestTenant, true, "The plugin approvals file is damaged"},
	}

	commands := [][]string{
		{"plugin", "install", "checkmate"},
		{"plugin", "update", "checkmate"},
		{"plugin", "update", "--all"},
		{"plugin", "approve", "checkmate"},
	}

	for _, test := range tests {
		for _, command := range commands {
			t.Run(test.name+" "+strings.Join(command[1:], " "), func(t *testing.T) {
				// Version 1.0.0 is installed, and the registry has 2.0.0, which an install
				// or an update would download.
				e, _, _ := newUpdateEnv(t, withAsks, withAsks)
				e.answer("y\n")

				if test.damaged {
					require.NoError(t, os.WriteFile(filepath.Join(e.Dir, "approvals.json"), []byte("{"), 0o600))
				}

				requests := e.server.Requests("/index.json")

				err := e.run(append(slices.Clone(command), "--tenant", test.tenant)...)

				require.Error(t, err)
				assert.Contains(t, err.Error(), test.want)
				assert.Equal(t, requests, e.server.Requests("/index.json"), "the registry is not asked")
				assert.Zero(t, e.server.Requests(pluginArtifactPath("checkmate", "2.0.0")), "nothing is downloaded")
				assert.Equal(t, "1.0.0", e.activeVersion(pluginTestID))
				assert.NotContains(t, e.stderr.String(), "Test plugin", "nothing is shown about the plugin")
				assert.Empty(t, e.stdout.String())
			})
		}
	}
}

func TestInfoPluginCmd_ListsVersionsFromTheRegistry(t *testing.T) {
	// The versions of a plugin that the index points to in a file are not in the
	// index, so the saved index is not enough to show the plugin.
	for _, ref := range []string{"checkmate", "checkmate@1.0.0"} {
		t.Run(ref, func(t *testing.T) {
			e := newPluginEnv(t)
			e.publish(e.refIndex(e.release("1.0.0"), e.release("1.1.0")))
			require.NoError(t, e.run("plugin", "search"))

			e.server.Fail("/plugins/checkmate.json", 503)

			err := e.run("plugin", "info", ref)

			require.ErrorIs(t, err, registry.ErrOffline)
			assert.True(t, strings.HasSuffix(err.Error(), "Reconnect and run: auth0 plugin info "+ref), err.Error())
			assert.Empty(t, e.stdout.String())
		})
	}
}

func TestInfoPluginCmd_ApprovalsOfOtherPermissions(t *testing.T) {
	e := newPluginEnv(t)
	old, newer := e.release("1.0.0", withScopes("read:clients")), e.release("2.0.0", withScopes("read:clients", "read:logs"))
	e.publish(e.index(old, newer))
	e.install("1.0.0")

	// One tenant approved the old permissions and another the new ones.
	require.NoError(t, e.svc.approvals.Approve(context.Background(), pluginTestTenant, pluginTestID, old.PermissionsHash))
	require.NoError(t, e.svc.approvals.Approve(context.Background(), "other.us.auth0.com", pluginTestID, newer.PermissionsHash))

	require.NoError(t, e.run("plugin", "info", "checkmate"))
	assert.Regexp(t, `APPROVED TENANTS\s+acme\.us\.auth0\.com \(different permissions\), other\.us\.auth0\.com\s*\n`, e.stdout.String())

	require.NoError(t, e.run("plugin", "info", "checkmate@1.0.0"))
	assert.Regexp(t, `APPROVED TENANTS\s+acme\.us\.auth0\.com, other\.us\.auth0\.com \(different permissions\)\s*\n`, e.stdout.String())

	// The JSON has the hashes, which say the same, and no label.
	require.NoError(t, e.run("plugin", "info", "checkmate", "--json"))

	var shown struct{ ApprovedTenants []map[string]any }
	require.NoError(t, json.Unmarshal(e.stdout.Bytes(), &shown))
	require.Len(t, shown.ApprovedTenants, 2)
	assert.Equal(t, "acme.us.auth0.com", shown.ApprovedTenants[0]["tenant"])
	assert.Equal(t, old.PermissionsHash, shown.ApprovedTenants[0]["permissionsHash"])
}

const (
	// HostileText has a bidirectional override and a zero width joiner, which the
	// schema of the registry allows in its text, and which could make a line read
	// differently than it is written. The control characters and the line and
	// paragraph separators are refused by the schema, and TestPluginText covers
	// them.
	hostileText = "a\u202eb\u200dc"

	// CleanText is what a terminal gets instead of the hostile text.
	cleanText = "a\ufffdb\ufffdc"
)

// assertNoHostileText fails when the output has any character of the hostile
// text.
func assertNoHostileText(t *testing.T, out, what string) {
	t.Helper()

	for _, hostile := range []string{"\u202e", "\u200d"} {
		assert.NotContains(t, out, hostile, what)
	}
}

func TestPluginCommands_PrintRegistryTextSafely(t *testing.T) {
	// The registry has hostile text, and so has the manifest of the installed copy.
	e := newPluginEnv(t)
	e.ManifestDescription = hostileText

	plugin := registrytest.NewPlugin(pluginTestID, e.release("1.0.0", withAsks))
	plugin.Description = hostileText
	plugin.Owner = plugins.Owner{Team: hostileText, Bugs: "https://example.com/" + hostileText}
	plugin.Homepage = "https://example.com/" + hostileText

	e.publish(registrytest.NewIndexBuilder().AddPlugin(plugin).Deprecate(registry.Deprecation{ID: pluginTestID, Message: hostileText, Replacement: "auth0/other"}))
	e.answer("y\ny\n")

	for _, args := range [][]string{
		{"search"},
		{"search", "--csv"},
		{"info", "checkmate"},
		{"install", "checkmate", "--tenant", pluginTestTenant},
		{"approve", "checkmate", "--tenant", "other.us.auth0.com"},
	} {
		require.NoError(t, e.run(append([]string{"plugin"}, args...)...), args)

		out := e.stdout.String() + e.stderr.String()
		assertNoHostileText(t, out, out)
		assert.Contains(t, out, cleanText, args)
	}

	require.NoError(t, e.run("plugin", "info", "checkmate"))

	for label, want := range map[string]string{
		"DESCRIPTION": cleanText,
		"OWNER":       cleanText + " https://example.com/" + cleanText,
		"HOMEPAGE":    "https://example.com/" + cleanText,
		"DEPRECATED":  cleanText + " (use auth0/other instead)",
	} {
		assert.Regexp(t, label+`\s+`+regexp.QuoteMeta(want), e.stdout.String())
	}

	// JSON is data, so it keeps the text as the registry has it.
	require.NoError(t, e.run("plugin", "info", "checkmate", "--json"))

	var shown struct {
		Description, Homepage string
		Owner                 struct{ Team string }
		Deprecated            struct{ Message string }
	}
	require.NoError(t, json.Unmarshal(e.stdout.Bytes(), &shown))
	assert.Equal(t, []string{hostileText, hostileText, "https://example.com/" + hostileText, hostileText},
		[]string{shown.Description, shown.Owner.Team, shown.Homepage, shown.Deprecated.Message})
}

// TestPluginCommands_EnvAuthModeFixesTheTenant checks that in env auth mode a
// --tenant that names another tenant than AUTH0_DOMAIN is refused, like the
// commands that call the API do, before anything happens.
func TestPluginCommands_EnvAuthModeFixesTheTenant(t *testing.T) {
	setup := func(t *testing.T) *pluginEnv {
		t.Helper()

		e, _ := newInstallEnv(t)
		t.Setenv(authModeEnvVar, authModeEnv)
		t.Setenv(envDomain, pluginTestTenant)
		e.answer("y\n")

		return e
	}

	commands := [][]string{
		{"plugin", "install", "checkmate"},
		{"plugin", "update", "checkmate"},
		{"plugin", "update", "--all"},
		{"plugin", "approve", "checkmate"},
	}

	for _, command := range commands {
		t.Run("refuses another tenant: "+strings.Join(command[1:], " "), func(t *testing.T) {
			e := setup(t)

			err := e.run(append(slices.Clone(command), "--tenant", "other.us.auth0.com")...)

			require.Error(t, err)
			assert.Equal(t, "auth", errorClass(err))
			assert.Equal(t, "env_auth_tenant_conflict", errorReason(err))
			assert.Contains(t, err.Error(), `--tenant "other.us.auth0.com" conflicts with AUTH0_DOMAIN="acme.us.auth0.com"`)
			assert.Zero(t, e.server.Requests("/index.json"))
			assert.Empty(t, e.installedIDs())
		})
	}

	for _, args := range [][]string{nil, {"--tenant", "ACME.us.auth0.com"}} {
		t.Run("accepts the tenant of AUTH0_DOMAIN: "+strings.Join(args, " "), func(t *testing.T) {
			e := setup(t)

			require.NoError(t, e.run(append([]string{"plugin", "install", "checkmate"}, args...)...))
			assert.Equal(t, []string{pluginTestID}, e.installedIDs())
		})
	}
}
