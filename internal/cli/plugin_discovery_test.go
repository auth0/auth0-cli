package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
	"github.com/auth0/auth0-cli/internal/plugins/registry/registrytest"
)

// execute runs a command line on a new command tree the way Execute does, with
// the flag errors wrapped and the suggestion to install a plugin offered, and
// returns the error that Execute would report.
func (e *pluginEnv) execute(version string, args ...string) error {
	e.t.Helper()

	e.stdout.Reset()
	e.stderr.Reset()

	e.cli.renderer = &display.Renderer{MessageWriter: e.stderr, ResultWriter: e.stdout}

	root := newRootTree(e.cli)
	root.SetFlagErrorFunc(wrapFlagError)
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(args)

	return e.cli.offerPluginInstall(root, version, classifyRequiredFlagError(root.ExecuteContext(context.Background())))
}

func TestOfferPluginInstall(t *testing.T) {
	t.Setenv(agentModeEnvVar, "false")

	// A registry that lists the plugin, whose index the client has saved, and
	// nothing installed.
	catalog := func(t *testing.T) *pluginEnv {
		t.Helper()

		e := newPluginEnv(t)
		e.publish(e.index(e.release("1.0.0")))

		_, _, err := e.svc.client.EnsureFresh(context.Background())
		require.NoError(t, err)

		return e
	}

	unchanged := func(t *testing.T, err error) {
		t.Helper()

		require.Error(t, err)

		_, offered := errors.AsType[pluginNotInstalledError](err)
		assert.False(t, offered, err.Error())
		assert.Equal(t, "usage", errorClass(err))
	}

	t.Run("a plugin that is only in the registry", func(t *testing.T) {
		e := catalog(t)

		err := e.execute("dev", "checkmate")

		require.Error(t, err)
		assert.Equal(t, `Plugin "checkmate" is available. Install it with: auth0 plugin install checkmate`, err.Error())
		assert.Equal(t, exitGeneric, exitCodeForError(err))

		envelope := buildErrorEnvelope(err)

		assert.Equal(t, "not_found", envelope.Error.Code)
		assert.Equal(t, "plugin_not_installed", envelope.Error.Reason)
		assert.Equal(t, err.Error(), envelope.Error.Message)
		assert.JSONEq(t, `{"pluginId":"auth0/checkmate","installCommand":"auth0 plugin install checkmate"}`, string(envelope.Error.Details))
	})

	t.Run("the person sees one line, without a full stop after the command", func(t *testing.T) {
		e := catalog(t)

		renderErrorMessage(e.cli.renderer, e.execute("dev", "checkmate"))

		out := e.stderr.String()

		assert.Contains(t, out, `Plugin "checkmate" is available. Install it with: auth0 plugin install checkmate`+"\n")
		assert.NotContains(t, out, "checkmate.")
		assert.NotContains(t, out, "Did you mean")
	})

	t.Run("with arguments and flags after the name", func(t *testing.T) {
		for _, args := range [][]string{
			{"checkmate", "scan"},
			{"checkmate", "scan", "--json"},
			{"checkmate", "--depth", "3"},
			{"--no-input", "checkmate"},
		} {
			e := catalog(t)

			err := e.execute("dev", args...)

			assert.Equal(t, "plugin_not_installed", errorReason(err), args)
		}
	})

	t.Run("a prerelease build has it too", func(t *testing.T) {
		assert.Equal(t, "plugin_not_installed", errorReason(catalog(t).execute("1.39.0-beta.1", "checkmate")))
	})

	t.Run("an installed plugin is a command", func(t *testing.T) {
		e := catalog(t)
		e.installPlugin(pluginTestID, "1.0.0")

		err := e.execute("dev", "checkmate", "--help")

		assert.NoError(t, err)
	})

	t.Run("a core command stays a core command", func(t *testing.T) {
		e := newPluginEnv(t)

		apps := e.releaseAs("auth0/apps", e.platform, "1.0.0")
		e.publish(registrytest.NewIndexBuilder().AddPlugin(registrytest.NewPlugin("auth0/apps", apps)))

		_, _, err := e.svc.client.EnsureFresh(context.Background())
		require.NoError(t, err)

		// The core command is found, so there is nothing to offer for its name.
		assert.NoError(t, e.execute("dev", "apps", "--help"))

		// Its own unknown subcommand is not at the root.
		err = e.execute("dev", "apps", "bogus")
		unchanged(t, err)
		assert.Contains(t, err.Error(), `unknown command "bogus" for "auth0 apps"`)

		// The same name under another command is not offered either.
		err = e.execute("dev", "tenants", "apps")
		unchanged(t, err)
	})

	t.Run("a name that is not in the registry", func(t *testing.T) {
		e := catalog(t)

		err := e.execute("dev", "nothing")

		unchanged(t, err)
		assert.Contains(t, err.Error(), `unknown command "nothing" for "auth0"`)

		// A near miss is not the plugin.
		unchanged(t, e.execute("dev", "checkmat"))
		unchanged(t, e.execute("dev", "Checkmate"))
		unchanged(t, e.execute("dev", "auth0/checkmate"))
	})

	t.Run("without a saved index", func(t *testing.T) {
		e := newPluginEnv(t)
		e.publish(e.index(e.release("1.0.0")))

		err := e.execute("dev", "checkmate")

		unchanged(t, err)
		assert.Zero(t, e.server.Requests("/index.json"), "the lookup never goes to the network")
		assert.Empty(t, e.stderr.String())
	})

	t.Run("an index that cannot be trusted any more", func(t *testing.T) {
		e := newPluginEnv(t)
		e.publish(e.index(e.release("1.0.0")).SetExpires(e.Now.Add(48 * time.Hour)))

		_, _, err := e.svc.client.EnsureFresh(context.Background())
		require.NoError(t, err)

		e.Now = e.Now.Add(72 * time.Hour)

		_, _, err = e.svc.client.Load(context.Background())
		require.ErrorIs(t, err, registry.ErrExpired)

		unchanged(t, e.execute("dev", "checkmate"))
		assert.Empty(t, e.stderr.String())
	})

	t.Run("a damaged saved index", func(t *testing.T) {
		e := catalog(t)

		files, err := filepath.Glob(filepath.Join(e.Dir, "registries", registry.DefaultRegistryName, "*"))
		require.NoError(t, err)
		require.NotEmpty(t, files)

		for _, file := range files {
			require.NoError(t, os.WriteFile(file, []byte("{broken"), 0o600))
		}

		unchanged(t, e.execute("dev", "checkmate"))
		assert.Empty(t, e.stderr.String())
	})

	t.Run("a saved index that is due for a refresh is not refreshed", func(t *testing.T) {
		e := catalog(t)
		e.Now = e.Now.Add(25 * time.Hour)

		_, status, err := e.svc.client.Load(context.Background())
		require.NoError(t, err)
		require.True(t, status.Stale, "the index must be due for a refresh for this test")

		before := e.server.Requests("/index.json")

		err = e.execute("dev", "checkmate")

		assert.Equal(t, "plugin_not_installed", errorReason(err))
		assert.Equal(t, before, e.server.Requests("/index.json"))
		assert.Empty(t, e.stderr.String())
	})

	t.Run("a build without the plugin feature", func(t *testing.T) {
		unchanged(t, catalog(t).execute("1.39.0", "checkmate"))
	})

	t.Run("plugins are turned off", func(t *testing.T) {
		for _, value := range []string{"true", "1", "perhaps"} {
			t.Run(value, func(t *testing.T) {
				e := catalog(t)
				t.Setenv(pluginsDisabledEnv, value)

				unchanged(t, e.execute("dev", "checkmate"))
			})
		}

		t.Run("false leaves them on", func(t *testing.T) {
			e := catalog(t)
			t.Setenv(pluginsDisabledEnv, "false")

			assert.Equal(t, "plugin_not_installed", errorReason(e.execute("dev", "checkmate")))
		})
	})

	t.Run("a plugin that cannot be installed here", func(t *testing.T) {
		elsewhere := plugins.PlatformDarwinArm64
		if elsewhere == plugins.CurrentPlatform() {
			elsewhere = plugins.PlatformWindowsX64
		}

		tests := []struct {
			name  string
			build func(e *pluginEnv) *registrytest.IndexBuilder
			offer bool
		}{
			{
				name: "a latest version that can be installed",
				build: func(e *pluginEnv) *registrytest.IndexBuilder {
					return e.index(e.release("1.0.0"), e.release("1.1.0"))
				},
				offer: true,
			},
			{
				name: "an older version that is revoked",
				build: func(e *pluginEnv) *registrytest.IndexBuilder {
					return e.index(e.release("1.0.0"), e.release("1.1.0")).Revoke(pluginTestID, "1.0.0", "unsafe")
				},
				offer: true,
			},
			{
				name: "a latest version that is revoked",
				build: func(e *pluginEnv) *registrytest.IndexBuilder {
					return e.index(e.release("1.0.0"), e.release("1.1.0")).Revoke(pluginTestID, "1.1.0", "unsafe")
				},
			},
			{
				name: "a latest version that needs a newer CLI",
				build: func(e *pluginEnv) *registrytest.IndexBuilder {
					return e.index(e.release("1.0.0"), e.release("1.1.0", func(r *registry.VersionRecord) {
						r.Engines = &plugins.Engines{Auth0CLI: ">=9.0.0"}
					}))
				},
			},
			{
				name: "a latest version without a build for this platform",
				build: func(e *pluginEnv) *registrytest.IndexBuilder {
					return e.index(e.release("1.0.0"), e.releaseAs(pluginTestID, elsewhere, "1.1.0"))
				},
			},
			{
				name: "a plugin of another registry",
				build: func(e *pluginEnv) *registrytest.IndexBuilder {
					return registrytest.NewIndexBuilder().AddPlugin(registrytest.NewPlugin("acme/checkmate", e.releaseAs("acme/checkmate", e.platform, "1.0.0")))
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				e := newPluginEnv(t)
				e.publish(tt.build(e))

				_, _, err := e.svc.client.EnsureFresh(context.Background())
				require.NoError(t, err)

				err = e.execute("dev", "checkmate")

				if !tt.offer {
					unchanged(t, err)
					assert.Contains(t, err.Error(), `unknown command "checkmate" for "auth0"`)

					return
				}

				assert.Equal(t, "plugin_not_installed", errorReason(err))
				assert.JSONEq(t, `{"pluginId":"auth0/checkmate","installCommand":"auth0 plugin install checkmate"}`, string(buildErrorEnvelope(err).Error.Details))
			})
		}
	})

	t.Run("a plugin service that cannot be built", func(t *testing.T) {
		e := catalog(t)
		e.cli.plugins = func() (*pluginService, error) { return nil, errors.New("no config") }

		unchanged(t, e.execute("dev", "checkmate"))
	})

	t.Run("an error that is not an unknown command is left alone", func(t *testing.T) {
		e := catalog(t)
		root := newRootTree(e.cli)
		failure := errors.New("boom")

		assert.Same(t, failure, e.cli.offerPluginInstall(root, "dev", failure))
		assert.NoError(t, e.cli.offerPluginInstall(root, "dev", nil))
	})
}

func TestPluginHintLine(t *testing.T) {
	machine := plugins.CurrentPlatform()
	if machine == "" {
		t.Skip("this machine is not one of the plugin platforms")
	}

	other := plugins.PlatformDarwinArm64
	if machine == other {
		other = plugins.PlatformLinuxX64
	}

	artifact := func(platform plugins.Platform) registry.Artifact {
		return registry.Artifact{OS: platform.OS(), Arch: platform.Arch(), Libc: platform.Libc()}
	}

	record := func(version string, change ...func(*registry.VersionRecord)) registry.VersionRecord {
		r := registry.VersionRecord{Version: version, Artifacts: []registry.Artifact{artifact(machine)}}

		for _, apply := range change {
			apply(&r)
		}

		return r
	}

	index := func(latest string, mutate func(*registry.Index), records ...registry.VersionRecord) *registry.Index {
		i := &registry.Index{Plugins: []registry.Plugin{{ID: pluginTestID, Latest: latest, Versions: records}}}

		if mutate != nil {
			mutate(i)
		}

		return i
	}

	const update = "checkmate 1.1.0 is available. Run: auth0 plugin update checkmate"

	tests := []struct {
		name      string
		index     *registry.Index
		cli       string
		installed string
		want      string
	}{
		{
			name:      "a newer version",
			index:     index("1.1.0", nil, record("1.0.0"), record("1.1.0")),
			installed: "1.0.0",
			want:      update,
		},
		{
			name:      "the same version",
			index:     index("1.0.0", nil, record("1.0.0")),
			installed: "1.0.0",
		},
		{
			name:      "an installed version that is newer than the latest",
			index:     index("1.0.0", nil, record("1.0.0")),
			installed: "1.2.0",
		},
		{
			name:      "a version that is not a version",
			index:     index("latest", nil, record("latest")),
			installed: "1.0.0",
		},
		{
			name:      "a version that is not in the canonical form",
			index:     index("1.01.0", nil, record("1.01.0")),
			installed: "1.0.0",
		},
		{
			name:      "a version with a leading v",
			index:     index("v1.1.0", nil, record("v1.1.0")),
			installed: "1.0.0",
		},
		{
			name:      "a version with build metadata",
			index:     index("1.1.0+build", nil, record("1.1.0+build")),
			installed: "1.0.0",
		},
		{
			name:      "a plugin that the index does not list",
			index:     &registry.Index{},
			installed: "1.0.0",
		},
		{
			name: "a newer version that is revoked",
			index: index("1.1.0", func(i *registry.Index) {
				i.Revocations = []registry.Revocation{{ID: pluginTestID, Version: "1.1.0", Reason: "Leaked."}}
			}, record("1.1.0")),
			installed: "1.0.0",
		},
		{
			name: "a newer version for a newer CLI",
			index: index("1.1.0", nil, record("1.1.0", func(r *registry.VersionRecord) {
				r.Engines = &plugins.Engines{Auth0CLI: ">=1.50.0"}
			})),
			cli:       "1.39.0",
			installed: "1.0.0",
		},
		{
			name: "a newer version that the CLI meets",
			index: index("1.1.0", nil, record("1.1.0", func(r *registry.VersionRecord) {
				r.Engines = &plugins.Engines{Auth0CLI: ">=1.38.0"}
			})),
			cli:       "1.39.0",
			installed: "1.0.0",
			want:      update,
		},
		{
			name: "a newer version with an engines constraint that cannot be read",
			index: index("1.1.0", nil, record("1.1.0", func(r *registry.VersionRecord) {
				r.Engines = &plugins.Engines{Auth0CLI: "newer"}
			})),
			cli:       "1.39.0",
			installed: "1.0.0",
		},
		{
			name: "a newer version without a build for this machine",
			index: index("1.1.0", nil, record("1.1.0", func(r *registry.VersionRecord) {
				r.Artifacts = []registry.Artifact{artifact(other)}
			})),
			installed: "1.0.0",
		},
		{
			name: "a newer version in a separate file",
			index: func() *registry.Index {
				i := index("1.1.0", nil)
				i.Plugins[0].Ref = &registry.Ref{Path: "checkmate.json"}

				return i
			}(),
			installed: "1.0.0",
		},
		{
			name: "deprecated with a newer version",
			index: index("1.1.0", func(i *registry.Index) {
				i.Deprecations = []registry.Deprecation{{ID: pluginTestID, Message: "Moved.", Replacement: "auth0/other"}}
			}, record("1.1.0")),
			installed: "1.0.0",
			want:      update + ". checkmate is deprecated: Moved. Use auth0/other.",
		},
		{
			name: "deprecated with nothing newer",
			index: index("1.0.0", func(i *registry.Index) {
				i.Deprecations = []registry.Deprecation{{ID: pluginTestID, Message: "Moved.", Replacement: "auth0/other"}}
			}, record("1.0.0")),
			installed: "1.0.0",
			want:      "checkmate is deprecated: Moved. Use auth0/other.",
		},
		{
			name: "deprecated without a replacement",
			index: index("1.0.0", func(i *registry.Index) {
				i.Deprecations = []registry.Deprecation{{ID: pluginTestID, Message: "Unmaintained"}}
			}, record("1.0.0")),
			installed: "1.0.0",
			want:      "checkmate is deprecated: Unmaintained.",
		},
		{
			name: "only another version is deprecated",
			index: index("1.0.0", func(i *registry.Index) {
				i.Deprecations = []registry.Deprecation{{ID: pluginTestID, Version: "0.9.0", Message: "Old."}}
			}, record("1.0.0")),
			installed: "1.0.0",
		},
		{
			name: "the installed version is deprecated",
			index: index("1.0.0", func(i *registry.Index) {
				i.Deprecations = []registry.Deprecation{{ID: pluginTestID, Version: "1.0.0", Message: "Broken."}}
			}, record("1.0.0")),
			installed: "1.0.0",
			want:      "checkmate is deprecated: Broken.",
		},
		{
			name: "text from the registry is made safe for a terminal",
			index: index("1.0.0", func(i *registry.Index) {
				i.Deprecations = []registry.Deprecation{{ID: pluginTestID, Message: "Gone\x1b[2J\nnow", Replacement: "auth0/\u202eother"}}
			}, record("1.0.0")),
			installed: "1.0.0",
			want:      "checkmate is deprecated: Gone�[2J�now. Use auth0/�other.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, pluginHintLine(test.index, test.cli, pluginTestID, test.installed))
		})
	}
}

// hintEnvOptions changes the environment that newHintEnv returns.
type hintEnvOptions struct {
	// WithoutNewer leaves the newer version out of the saved index, Newer changes
	// it, and Index changes the index before it is published.
	withoutNewer bool
	newer        func(*registry.VersionRecord)
	index        []func(*registrytest.IndexBuilder)
}

// hintWithoutNewer is the option for an index that lists nothing newer than the
// installed version.
func hintWithoutNewer(o *hintEnvOptions) { o.withoutNewer = true }

// hintNewer is the option that changes the newer version in the saved index.
func hintNewer(change func(*registry.VersionRecord)) func(*hintEnvOptions) {
	return func(o *hintEnvOptions) { o.newer = change }
}

// hintIndex is the option that changes the index before it is published.
func hintIndex(change ...func(*registrytest.IndexBuilder)) func(*hintEnvOptions) {
	return func(o *hintEnvOptions) { o.index = change }
}

// newHintEnv returns a runEnv whose plugin is approved and installed in version 1.0.0,
// and whose saved index lists the version 1.1.0 as the latest, unless an option says
// otherwise.
func newHintEnv(t *testing.T, options ...func(*hintEnvOptions)) *runEnv {
	t.Helper()

	var o hintEnvOptions

	for _, apply := range options {
		apply(&o)
	}

	t.Setenv(agentModeEnvVar, "false")
	t.Setenv(pluginUpdateHintEnv, "")

	r := newRunEnv(t, fixtureCommands())
	r.approve()

	records := []registry.VersionRecord{r.record}

	if !o.withoutNewer {
		newer := r.releaseFixture("1.1.0", fixtureCommands())

		if o.newer != nil {
			o.newer(&newer)
		}

		records = append(records, newer)
	}

	r.publishFixture(append([]func(*registrytest.IndexBuilder){func(b *registrytest.IndexBuilder) { b.SetVersion(2) }}, o.index...), records...)

	_, err := r.svc.client.Refresh(context.Background())
	require.NoError(t, err)

	r.svc.outputTerminals = func() bool { return true }

	return r
}

func hintPath(r *runEnv) string {
	return filepath.Join(r.Dir, pluginHintFile)
}

func writeHint(t *testing.T, r *runEnv, contents string) {
	t.Helper()

	require.NoError(t, os.WriteFile(hintPath(r), []byte(contents), 0o600))
}

const hintLine = "checkmate 1.1.0 is available. Run: auth0 plugin update checkmate"

func TestPluginRun_UpdateHint(t *testing.T) {
	t.Run("a plugin that is pinned gets it once, after it ran", func(t *testing.T) {
		r := newHintEnv(t)

		installed, err := r.svc.store.List()
		require.NoError(t, err)
		require.Len(t, installed, 1)
		require.True(t, installed[0].Pinned, "an install of a version is a pin")

		require.NoError(t, r.exec("checkmate"))

		assert.Equal(t, 1, strings.Count(r.stderr.String(), hintLine), r.stderr.String())
		assert.Equal(t, 1, strings.Count(r.stderr.String(), "available"), "one line, and not one for every plugin")
		assert.Less(t, strings.Index(r.stderr.String(), "fixture stderr"), strings.Index(r.stderr.String(), hintLine), "after the plugin")

		// The plugin was run, and its output is untouched.
		assert.Equal(t, []string{}, r.report(t).Args)

		data, err := os.ReadFile(hintPath(r))
		require.NoError(t, err)

		var hint pluginHint

		require.NoError(t, json.Unmarshal(data, &hint))
		assert.WithinDuration(t, time.Now(), hint.Shown, time.Minute)

		if runtime.GOOS != "windows" {
			info, err := os.Stat(hintPath(r))
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		}

		entries, err := os.ReadDir(r.Dir)
		require.NoError(t, err)

		for _, entry := range entries {
			assert.False(t, strings.HasPrefix(entry.Name(), ".hint-"), "no temporary file is left: %s", entry.Name())
		}
	})

	t.Run("a command of the plugin gets it as well", func(t *testing.T) {
		r := newHintEnv(t)

		require.NoError(t, r.exec("checkmate", "scan"))

		assert.Contains(t, r.stderr.String(), hintLine)
	})

	t.Run("not again within a day, and again after it", func(t *testing.T) {
		r := newHintEnv(t)

		require.NoError(t, r.exec("checkmate"))
		require.Contains(t, r.stderr.String(), hintLine)

		first, err := os.ReadFile(hintPath(r))
		require.NoError(t, err)

		require.NoError(t, r.exec("checkmate"))
		assert.NotContains(t, r.stderr.String(), hintLine)

		again, err := os.ReadFile(hintPath(r))
		require.NoError(t, err)
		assert.Equal(t, string(first), string(again), "a run without a hint does not touch the file")

		for _, ago := range []time.Duration{time.Minute, 23 * time.Hour} {
			writeHint(t, r, `{"shown":"`+time.Now().Add(-ago).UTC().Format(time.RFC3339)+`"}`)

			require.NoError(t, r.exec("checkmate"))
			assert.NotContains(t, r.stderr.String(), hintLine, ago.String())
		}

		stale := time.Now().Add(-25 * time.Hour).UTC().Format(time.RFC3339)
		writeHint(t, r, `{"shown":"`+stale+`"}`)

		require.NoError(t, r.exec("checkmate"))
		assert.Contains(t, r.stderr.String(), hintLine)

		data, err := os.ReadFile(hintPath(r))
		require.NoError(t, err)
		assert.NotContains(t, string(data), stale, "the time of the new hint is saved")
	})

	t.Run("a hint in the future means no hint was shown", func(t *testing.T) {
		r := newHintEnv(t)
		writeHint(t, r, `{"shown":"`+time.Now().Add(72*time.Hour).UTC().Format(time.RFC3339)+`"}`)

		require.NoError(t, r.exec("checkmate"))

		assert.Contains(t, r.stderr.String(), hintLine)
	})

	t.Run("a hint file that cannot be written never fails the command", func(t *testing.T) {
		r := newHintEnv(t)

		// A directory that is not empty cannot be replaced by the file.
		require.NoError(t, os.MkdirAll(filepath.Join(hintPath(r), "inside"), 0o700))

		require.NoError(t, r.exec("checkmate"))

		assert.Contains(t, r.stderr.String(), hintLine)
		assert.Equal(t, []string{}, r.report(t).Args)

		entries, err := os.ReadDir(r.Dir)
		require.NoError(t, err)

		for _, entry := range entries {
			assert.False(t, strings.HasPrefix(entry.Name(), ".hint-"), "no temporary file is left: %s", entry.Name())
		}
	})

	t.Run("a failed run gets none", func(t *testing.T) {
		r := newHintEnv(t)

		require.Error(t, r.exec("checkmate", "--mode=exit", "--code=2"))

		assert.NotContains(t, r.stderr.String(), hintLine)
		assert.NoFileExists(t, hintPath(r))
	})

	t.Run("a refused run and the help get none", func(t *testing.T) {
		r := newHintEnv(t)

		require.NoError(t, r.exec("checkmate", "--help"))
		assert.NotContains(t, r.stderr.String(), hintLine)
		assert.NotContains(t, r.stdout.String(), hintLine)

		t.Setenv(pluginsDisabledEnv, "true")

		require.Error(t, r.exec("checkmate"))
		assert.NotContains(t, r.stderr.String(), hintLine)
		assert.NoFileExists(t, hintPath(r))
	})

	// Each of these turns the hint off, and nothing else changes.
	off := []struct {
		name  string
		args  []string
		setup func(t *testing.T, r *runEnv)
	}{
		{name: "stdout or stderr is not a terminal", setup: func(_ *testing.T, r *runEnv) { r.svc.outputTerminals = func() bool { return false } }},
		{name: "agent mode", args: []string{"--agent-mode"}},
		{name: "agent mode alone", args: []string{"--agent-mode", "--json=false", "--no-input=false"}},
		{name: "no input", args: []string{"--no-input"}},
		{name: "json", args: []string{"--json"}},
		{name: "compact json", args: []string{"--json-compact"}},
		{name: "csv", args: []string{"--csv"}},
		{name: "the switch is off", setup: func(t *testing.T, _ *runEnv) { t.Setenv(pluginUpdateHintEnv, "false") }},
		{name: "the last hint is recent", setup: func(t *testing.T, r *runEnv) {
			writeHint(t, r, `{"shown":"`+time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)+`"}`)
		}},
	}

	for _, test := range off {
		t.Run("off when "+test.name, func(t *testing.T) {
			r := newHintEnv(t)

			if test.setup != nil {
				test.setup(t, r)
			}

			before, _ := os.ReadFile(hintPath(r))

			require.NoError(t, r.exec(append([]string{"checkmate"}, test.args...)...))

			assert.NotContains(t, r.stderr.String(), "available", r.stderr.String())
			assert.NotContains(t, r.stdout.String(), "available")

			after, _ := os.ReadFile(hintPath(r))
			assert.Equal(t, string(before), string(after), "a hint that is not shown is not recorded")
		})
	}

	t.Run("the switch set to true leaves it on", func(t *testing.T) {
		r := newHintEnv(t)
		t.Setenv(pluginUpdateHintEnv, "true")

		require.NoError(t, r.exec("checkmate"))

		assert.Contains(t, r.stderr.String(), hintLine)
	})

	t.Run("nothing to say when the plugin is current", func(t *testing.T) {
		r := newHintEnv(t, hintWithoutNewer)

		require.NoError(t, r.exec("checkmate"))

		assert.NotContains(t, r.stderr.String(), "available")
		assert.NoFileExists(t, hintPath(r), "nothing was shown, so nothing is recorded")
	})

	t.Run("a deprecated plugin that has no newer version still says so", func(t *testing.T) {
		r := newHintEnv(t, hintWithoutNewer, hintIndex(func(b *registrytest.IndexBuilder) {
			b.Deprecate(registry.Deprecation{ID: pluginTestID, Message: "Moved.", Replacement: "auth0/other"})
		}))

		require.NoError(t, r.exec("checkmate"))

		assert.Contains(t, r.stderr.String(), "checkmate is deprecated: Moved. Use auth0/other.\n")
		assert.NotContains(t, r.stderr.String(), "is available")

		// It keeps running, and it is said once a day.
		assert.Equal(t, []string{}, r.report(t).Args)

		require.NoError(t, r.exec("checkmate"))
		assert.NotContains(t, r.stderr.String(), "deprecated")
	})

	t.Run("a newer version that cannot run here is not offered", func(t *testing.T) {
		r := newHintEnv(t, hintNewer(func(newer *registry.VersionRecord) {
			newer.Engines = &plugins.Engines{Auth0CLI: ">=9.0.0"}
		}))

		require.NoError(t, r.exec("checkmate"))

		assert.NotContains(t, r.stderr.String(), "available")
	})
}

func TestPluginHintEnabled(t *testing.T) {
	for value, want := range map[string]bool{
		"":      true,
		" ":     true,
		"true":  true,
		"1":     true,
		" true": true,
		"false": false,
		"0":     false,
		"never": false,
	} {
		t.Run("the switch is "+strconv.Quote(value), func(t *testing.T) {
			t.Setenv(pluginUpdateHintEnv, value)

			assert.Equal(t, want, pluginHintEnabled())
		})
	}
}

func TestReadPluginHint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, pluginHintFile)

	t.Run("a missing file", func(t *testing.T) {
		_, ok := readPluginHint(path)
		assert.False(t, ok)
	})

	t.Run("what the CLI wrote", func(t *testing.T) {
		shown := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)

		writePluginHint(path, shown)

		got, ok := readPluginHint(path)
		require.True(t, ok)
		assert.True(t, shown.Equal(got))
	})

	t.Run("a file that is not what the CLI wrote", func(t *testing.T) {
		now := time.Now().UTC()

		for name, contents := range map[string]string{
			"empty":         "",
			"broken":        "{",
			"null":          "null",
			"not an object": "[1]",
			"not a time":    `{"shown":"yesterday"}`,
			"zero time":     `{"shown":"0001-01-01T00:00:00Z"}`,
			"a number":      `{"shown":42}`,
			"too big":       `{"shown":"` + now.Format(time.RFC3339) + `","padding":"` + strings.Repeat("a", pluginHintMaxBytes) + `"}`,
			"padded":        `{"shown":"` + now.Add(-time.Hour).Format(time.RFC3339) + `"}` + strings.Repeat(" ", pluginHintMaxBytes),
		} {
			t.Run(name, func(t *testing.T) {
				damaged := filepath.Join(t.TempDir(), pluginHintFile)
				require.NoError(t, os.WriteFile(damaged, []byte(contents), 0o600))

				_, ok := readPluginHint(damaged)
				assert.False(t, ok)
			})
		}
	})

	t.Run("a directory", func(t *testing.T) {
		directory := filepath.Join(dir, "directory")
		require.NoError(t, os.Mkdir(directory, 0o700))

		_, ok := readPluginHint(directory)
		assert.False(t, ok)
	})

	t.Run("a named pipe does not hold the command up", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("no named pipes here")
		}

		pipe := filepath.Join(dir, "pipe")
		require.NoError(t, syscall.Mkfifo(pipe, 0o600))

		done := make(chan bool, 1)

		go func() {
			_, ok := readPluginHint(pipe)
			done <- ok
		}()

		select {
		case ok := <-done:
			assert.False(t, ok)
		case <-time.After(10 * time.Second):
			t.Fatal("reading the hint file blocked on a named pipe")
		}
	})

	t.Run("a symbolic link is not followed", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("no symbolic links without a privilege")
		}

		target := filepath.Join(dir, "target.json")
		writePluginHint(target, time.Now())

		link := filepath.Join(dir, "link.json")
		require.NoError(t, os.Symlink(target, link))

		_, ok := readPluginHint(link)
		assert.False(t, ok)
	})

	t.Run("writing where there is no directory does nothing", func(t *testing.T) {
		assert.NotPanics(t, func() { writePluginHint(filepath.Join(dir, "missing", pluginHintFile), time.Now()) })
		assert.NoDirExists(t, filepath.Join(dir, "missing"))
	})
}
