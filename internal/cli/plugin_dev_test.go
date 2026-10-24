package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/devlink"
)

// stubExecutable is the executable of a build that is linked or checked but never
// run.
var stubExecutable = []byte("#!/bin/sh\n")

// devBuild is a local build of the test plugin: a directory with its plugin.json
// and its executable.
type devBuild struct {
	Dir      string
	Bin      string
	Manifest plugins.Manifest
}

// writeDevBuild writes a build of the test plugin in a new directory.
func writeDevBuild(t *testing.T, version string, commands []plugins.Command, executable []byte) devBuild {
	t.Helper()

	dir := t.TempDir()

	build := devBuild{
		Dir:      dir,
		Bin:      devlink.DefaultBin(dir, pluginTestBin),
		Manifest: fixtureManifest(plugins.CurrentPlatform(), version, commands),
	}

	build.save(t)
	require.NoError(t, os.WriteFile(build.Bin, executable, 0o755))

	return build
}

// save writes the manifest of the build.
func (b devBuild) save(t *testing.T) {
	t.Helper()

	data, err := json.Marshal(b.Manifest)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(b.Dir, devlink.ManifestName), data, 0o600))
}

// writeManifestText replaces the manifest of the build with the text.
func (b devBuild) writeManifestText(t *testing.T, text string) {
	t.Helper()

	require.NoError(t, os.WriteFile(filepath.Join(b.Dir, devlink.ManifestName), []byte(text), 0o600))
}

// newDevEnv returns an environment with dev mode on.
func newDevEnv(t *testing.T) *pluginEnv {
	t.Helper()

	e := newPluginEnv(t)
	t.Setenv(pluginDevModeEnv, "true")

	return e
}

// link records the build as the link of its plugin, the way the link command does.
func (e *pluginEnv) link(build devBuild) {
	e.t.Helper()

	require.NoError(e.t, devlink.Links{Dir: e.Dir}.Add(context.Background(), build.Manifest.ID, devlink.Link{Path: build.Dir, Bin: build.Bin}))
}

// links returns the links that are recorded.
func (e *pluginEnv) links() map[string]devlink.Link {
	e.t.Helper()

	links, err := devlink.Links{Dir: e.Dir}.Load()
	require.NoError(e.t, err)

	return links
}

// linksPath is where the links are kept.
func (e *pluginEnv) linksPath() string {
	return filepath.Join(e.Dir, devlink.FileName)
}

// approveBuild records the approval of a person for the permissions of the build.
func (e *pluginEnv) approveBuild(build devBuild) {
	e.t.Helper()

	require.NoError(e.t, e.svc.approvals.Approve(context.Background(), pluginTestTenant, build.Manifest.ID, build.Manifest.PermissionsHash()))
}

// samePath fails unless two paths are the same file, whatever links their
// directories go through.
func samePath(t *testing.T, want, got string) {
	t.Helper()

	resolvedWant, err := filepath.EvalSymlinks(want)
	require.NoError(t, err)

	resolvedGot, err := filepath.EvalSymlinks(got)
	require.NoError(t, err)

	assert.Equal(t, resolvedWant, resolvedGot)
}

// devModeValues are values that leave dev mode off.
var devModeValues = []string{"", "1", "True", "TRUE", " true", "true ", "yes", "on", "false"}

func TestPluginDevMode(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"true", true},
		{"", false},
		{"1", false},
		{"True", false},
		{"TRUE", false},
		{" true", false},
		{"true ", false},
		{"yes", false},
		{"false", false},
	}

	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			t.Setenv(pluginDevModeEnv, test.value)

			assert.Equal(t, test.want, pluginDevMode())
			assert.Equal(t, test.want, !pluginDevCmd(&cli{}).Hidden, "the commands show in the help only in dev mode")

			err := requirePluginDevMode()
			if test.want {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)
			assertValidationError(t, err, "plugin_dev_mode_required")
			assert.Contains(t, err.Error(), pluginDevModeEnv+"=true")
		})
	}
}

func TestPluginDevCmds_NeedDevMode(t *testing.T) {
	for _, value := range devModeValues {
		t.Run("dev mode is "+value, func(t *testing.T) {
			e := newPluginEnv(t)
			t.Setenv(pluginDevModeEnv, value)

			build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)

			commands := [][]string{
				{"plugin", "dev", "link", build.Dir},
				{"plugin", "dev", "unlink", "checkmate"},
				{"plugin", "dev", "validate", build.Dir},
			}

			for _, args := range commands {
				err := e.run(args...)

				require.Error(t, err, args)
				assertValidationError(t, err, "plugin_dev_mode_required")
				assert.Contains(t, err.Error(), pluginDevModeEnv+"=true")
				assert.Empty(t, e.stdout.String())
			}

			assert.NoDirExists(t, e.Dir, "nothing is written")
		})
	}
}

func TestLinkPluginCmd(t *testing.T) {
	t.Run("links the directory, tells what every run prints, and asks for the approval", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)

		require.NoError(t, e.run("plugin", "dev", "link", build.Dir, "--tenant", pluginTestTenant))

		assert.Equal(t, map[string]devlink.Link{pluginTestID: {Path: build.Dir, Bin: build.Bin}}, e.links())

		out := e.stderr.String()
		assert.Contains(t, out, "Linked auth0/checkmate 1.0.0 from "+build.Dir+". Run it with: auth0 checkmate")
		assert.Contains(t, out, "Every run prints this line on stderr: Running the linked build of auth0/checkmate 1.0.0 from "+build.Dir+". Its files are not checked against a release.")
		assert.Contains(t, out, "auth0/checkmate is not approved for "+pluginTestTenant+" yet. Approve it in a terminal with: auth0 plugin approve auth0/checkmate --tenant "+pluginTestTenant)
		assert.Empty(t, e.stdout.String())
	})

	t.Run("prints the result as JSON", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)

		require.NoError(t, e.run("plugin", "dev", "link", build.Dir, "--tenant", pluginTestTenant, "--json-compact"))

		assert.JSONEq(t, `{
			"id": "auth0/checkmate",
			"version": "1.0.0",
			"path": `+strconv.Quote(build.Dir)+`,
			"bin": `+strconv.Quote(build.Bin)+`,
			"banner": `+strconv.Quote("Running the linked build of auth0/checkmate 1.0.0 from "+build.Dir+". Its files are not checked against a release.")+`,
			"approvalRequired": true,
			"approveCommand": "auth0 plugin approve auth0/checkmate --tenant `+pluginTestTenant+`"
		}`, e.stdout.String())
	})

	t.Run("does not ask for an approval that is there", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)
		e.approveBuild(build)

		require.NoError(t, e.run("plugin", "dev", "link", build.Dir, "--tenant", pluginTestTenant, "--json"))

		object := e.jsonObject()
		assert.Equal(t, false, object["approvalRequired"])
		assert.NotContains(t, object, "approveCommand")
		assert.NotContains(t, e.stderr.String(), "not approved")
	})

	t.Run("asks again for permissions that changed", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)
		e.approveBuild(build)

		build.Manifest.Commands[0].Permissions = fixturePermissions([]string{"read:clients", "delete:clients"})
		build.save(t)

		require.NoError(t, e.run("plugin", "dev", "link", build.Dir, "--tenant", pluginTestTenant, "--json"))

		assert.Equal(t, true, e.jsonObject()["approvalRequired"])
	})

	t.Run("says how to approve when there is no tenant", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)

		require.NoError(t, e.run("plugin", "dev", "link", build.Dir))

		assert.Contains(t, e.stderr.String(), "auth0/checkmate is not approved on any tenant. Log in with auth0 login, then approve it in a terminal with: auth0 plugin approve auth0/checkmate --tenant <tenant>")
	})

	t.Run("takes the current directory by default", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)

		t.Chdir(build.Dir)

		require.NoError(t, e.run("plugin", "dev", "link"))

		link := e.links()[pluginTestID]
		assert.True(t, filepath.IsAbs(link.Path))
		assert.True(t, filepath.IsAbs(link.Bin))
		samePath(t, build.Dir, link.Path)
		samePath(t, build.Bin, link.Bin)
	})

	t.Run("records an absolute path for a relative one", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)

		t.Chdir(filepath.Dir(build.Dir))

		require.NoError(t, e.run("plugin", "dev", "link", "."+string(filepath.Separator)+filepath.Base(build.Dir)))

		link := e.links()[pluginTestID]
		assert.True(t, filepath.IsAbs(link.Path), link.Path)
		samePath(t, build.Dir, link.Path)
	})

	t.Run("runs the executable that --bin names", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)

		// The conventional file is gone, so only the one that was named can be it.
		require.NoError(t, os.Remove(build.Bin))
		require.NoError(t, os.Mkdir(filepath.Join(build.Dir, "dist"), 0o700))

		other := filepath.Join(build.Dir, "dist", "built")
		require.NoError(t, os.WriteFile(other, stubExecutable, 0o755))

		t.Chdir(build.Dir)

		require.NoError(t, e.run("plugin", "dev", "link", "--bin", filepath.Join("dist", "built")))

		link := e.links()[pluginTestID]
		assert.True(t, filepath.IsAbs(link.Bin), link.Bin)
		samePath(t, other, link.Bin)
	})

	t.Run("linking again replaces the link", func(t *testing.T) {
		e := newDevEnv(t)
		first := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)
		second := writeDevBuild(t, "2.0.0", fixtureCommands(), stubExecutable)

		require.NoError(t, e.run("plugin", "dev", "link", first.Dir))
		require.NoError(t, e.run("plugin", "dev", "link", second.Dir), "the command of the plugin that is linked does not block its own link")

		assert.Equal(t, map[string]devlink.Link{pluginTestID: {Path: second.Dir, Bin: second.Bin}}, e.links())
		assert.Contains(t, e.stderr.String(), "Linked auth0/checkmate 2.0.0 from "+second.Dir)
	})

	t.Run("takes at most one directory", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)

		require.Error(t, e.run("plugin", "dev", "link", build.Dir, build.Dir))
		assert.NoFileExists(t, e.linksPath())
	})
}

func TestLinkPluginCmd_Refusals(t *testing.T) {
	tests := []struct {
		name string

		// Prepare changes the build, and returns the arguments after the link
		// command.
		prepare func(t *testing.T, build devBuild) []string
		want    []string
	}{
		{
			name: "a directory that does not exist",
			prepare: func(t *testing.T, build devBuild) []string {
				return []string{filepath.Join(build.Dir, "missing")}
			},
			want: []string{"missing"},
		},
		{
			name: "a file and not a directory",
			prepare: func(t *testing.T, build devBuild) []string {
				return []string{filepath.Join(build.Dir, devlink.ManifestName)}
			},
			want: []string{"is not a directory", devlink.ManifestName},
		},
		{
			name: "a directory without a manifest",
			prepare: func(t *testing.T, build devBuild) []string {
				require.NoError(t, os.Remove(filepath.Join(build.Dir, devlink.ManifestName)))

				return []string{build.Dir}
			},
			want: []string{devlink.ManifestName},
		},
		{
			name: "a manifest that is not JSON",
			prepare: func(t *testing.T, build devBuild) []string {
				build.writeManifestText(t, "not json")

				return []string{build.Dir}
			},
			want: []string{"is not valid", "auth0 plugin dev validate {dir}"},
		},
		{
			name: "a manifest with problems",
			prepare: func(t *testing.T, build devBuild) []string {
				build.writeManifestText(t, `{"id":"auth0/checkmate"}`)

				return []string{build.Dir}
			},
			want: []string{"is not valid", "version:", "auth0 plugin dev validate {dir}"},
		},
		{
			name: "a name of a core command",
			prepare: func(t *testing.T, build devBuild) []string {
				build.Manifest.ID = "auth0/apps"
				build.save(t)

				return []string{build.Dir}
			},
			want: []string{"auth0 already has a command named \"apps\""},
		},
		{
			name: "a plugin name that Windows reserves",
			prepare: func(t *testing.T, build devBuild) []string {
				build.Manifest.ID = "auth0/con"
				build.save(t)

				return []string{build.Dir}
			},
			want: []string{`"con" cannot be a directory name`},
		},
		{
			name: "a registry name that Windows reserves",
			prepare: func(t *testing.T, build devBuild) []string {
				build.Manifest.ID = "nul/checkmate"
				build.save(t)

				return []string{build.Dir}
			},
			want: []string{`"nul" cannot be a directory name`},
		},
		{
			name: "a registry that the CLI does not know",
			prepare: func(t *testing.T, build devBuild) []string {
				build.Manifest.ID = "acme/checkmate"
				build.save(t)

				return []string{build.Dir}
			},
			want: []string{"acme/checkmate", "must start with auth0/"},
		},
		{
			name: "an executable that is not there",
			prepare: func(t *testing.T, build devBuild) []string {
				require.NoError(t, os.Remove(build.Bin))

				return []string{build.Dir}
			},
			want: []string{"{bin}", "Build it there, or give another file with --bin"},
		},
		{
			name: "an executable that is a directory",
			prepare: func(t *testing.T, build devBuild) []string {
				require.NoError(t, os.Remove(build.Bin))
				require.NoError(t, os.Mkdir(build.Bin, 0o700))

				return []string{build.Dir}
			},
			want: []string{"{bin} is not a regular file"},
		},
		{
			name: "a file that --bin names and is not there",
			prepare: func(t *testing.T, build devBuild) []string {
				return []string{build.Dir, "--bin", filepath.Join(build.Dir, "nothing")}
			},
			want: []string{filepath.Join("{dir}", "nothing"), "--bin"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := newDevEnv(t)
			build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)

			args := test.prepare(t, build)

			err := e.run(append([]string{"plugin", "dev", "link"}, args...)...)

			require.Error(t, err)
			assertValidationError(t, err, "plugin_link_invalid")

			for _, want := range test.want {
				want = strings.ReplaceAll(want, "{dir}", build.Dir)
				want = strings.ReplaceAll(want, "{bin}", build.Bin)

				assert.Contains(t, err.Error(), want)
			}

			assert.NoFileExists(t, e.linksPath(), "nothing is linked")
			assert.NoDirExists(t, e.Dir, "nothing is written")
			assert.Empty(t, e.stdout.String())
		})
	}

	t.Run("an executable that cannot be run", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows has no executable bit")
		}

		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)
		require.NoError(t, os.Chmod(build.Bin, 0o644))

		err := e.run("plugin", "dev", "link", build.Dir)

		assertValidationError(t, err, "plugin_link_invalid")
		assert.Contains(t, err.Error(), "chmod +x "+build.Bin)
		assert.NoFileExists(t, e.linksPath())
	})

	t.Run("a file of links that is damaged is left as it is", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)

		require.NoError(t, os.MkdirAll(e.Dir, 0o700))
		require.NoError(t, os.WriteFile(e.linksPath(), []byte("{damaged"), 0o600))

		err := e.run("plugin", "dev", "link", build.Dir)

		require.ErrorIs(t, err, devlink.ErrDamaged)
		assertValidationError(t, err, "plugin_link_invalid")
		assert.Contains(t, err.Error(), e.linksPath())
		assert.Contains(t, err.Error(), "Repair it, or delete it to start again with no links")
		assert.Equal(t, "{damaged", readFile(t, e.linksPath()))
	})
}

func TestUnlinkPluginCmd(t *testing.T) {
	// The other plugin is linked without a build, as unlinking does not need one.
	other := devlink.Link{Path: filepath.Join(t.TempDir(), "other"), Bin: filepath.Join(t.TempDir(), "other", "other")}

	newEnv := func(t *testing.T) (*pluginEnv, devBuild) {
		t.Helper()

		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)

		e.link(build)
		require.NoError(t, devlink.Links{Dir: e.Dir}.Add(context.Background(), "auth0/other", other))

		return e, build
	}

	t.Run("by short name, and the build is left alone", func(t *testing.T) {
		e, build := newEnv(t)

		require.NoError(t, e.run("plugin", "dev", "unlink", "checkmate"))

		assert.Equal(t, map[string]devlink.Link{"auth0/other": other}, e.links())
		assert.Contains(t, e.stderr.String(), "Unlinked auth0/checkmate.")
		assert.FileExists(t, build.Bin)
		assert.FileExists(t, filepath.Join(build.Dir, devlink.ManifestName))
	})

	t.Run("by ID", func(t *testing.T) {
		e, _ := newEnv(t)

		require.NoError(t, e.run("plugin", "dev", "unlink", "auth0/other"))

		assert.Contains(t, e.links(), pluginTestID)
		assert.NotContains(t, e.links(), "auth0/other")
	})

	t.Run("the last link leaves a file with none", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)
		e.link(build)

		require.NoError(t, e.run("plugin", "dev", "unlink", "checkmate"))

		assert.Empty(t, e.links())
	})

	t.Run("a plugin that is not linked names the ones that are", func(t *testing.T) {
		e, _ := newEnv(t)

		err := e.run("plugin", "dev", "unlink", "nothing")

		require.EqualError(t, err, "the plugin auth0/nothing is not linked. Linked plugins: auth0/checkmate, auth0/other")
		assertValidationError(t, err, "plugin_not_linked")
		assert.Len(t, e.links(), 2)
	})

	t.Run("a plugin that is not linked when nothing is", func(t *testing.T) {
		e := newDevEnv(t)

		err := e.run("plugin", "dev", "unlink", "checkmate")

		require.EqualError(t, err, "the plugin auth0/checkmate is not linked, and no plugin is. Link one with: auth0 plugin dev link <path>")
		assertValidationError(t, err, "plugin_not_linked")
	})

	t.Run("takes a name and not a version", func(t *testing.T) {
		e, _ := newEnv(t)

		err := e.run("plugin", "dev", "unlink", "checkmate@1.0.0")

		require.ErrorContains(t, err, "unlink takes the name of a plugin, without a version")
		assert.Len(t, e.links(), 2)
	})

	t.Run("takes one plugin", func(t *testing.T) {
		e, _ := newEnv(t)

		require.Error(t, e.run("plugin", "dev", "unlink"))
		require.Error(t, e.run("plugin", "dev", "unlink", "checkmate", "other"))
		assert.Len(t, e.links(), 2)
	})

	t.Run("a file of links that is damaged is left as it is", func(t *testing.T) {
		e := newDevEnv(t)

		require.NoError(t, os.MkdirAll(e.Dir, 0o700))
		require.NoError(t, os.WriteFile(e.linksPath(), []byte("{damaged"), 0o600))

		err := e.run("plugin", "dev", "unlink", "checkmate")

		require.ErrorIs(t, err, devlink.ErrDamaged)
		assertValidationError(t, err, "plugin_link_invalid")
		assert.Equal(t, "{damaged", readFile(t, e.linksPath()))
	})
}

func TestValidatePluginCmd(t *testing.T) {
	// Validating needs no registry and no session: the service of the plugins is
	// not even asked for.
	offline := func(e *pluginEnv) {
		e.cli.plugins = func() (*pluginService, error) { return nil, errors.New("the validation must not use the registry") }
	}

	t.Run("a valid manifest in a directory", func(t *testing.T) {
		e := newDevEnv(t)
		offline(e)

		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)

		require.NoError(t, e.run("plugin", "dev", "validate", build.Dir))

		assert.Contains(t, e.stderr.String(), "The plugin manifest is valid.")
		assert.Empty(t, e.stdout.String())
		assert.NoDirExists(t, e.Dir, "nothing is written")
	})

	t.Run("a valid manifest as JSON", func(t *testing.T) {
		e := newDevEnv(t)
		offline(e)

		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)

		require.NoError(t, e.run("plugin", "dev", "validate", filepath.Join(build.Dir, devlink.ManifestName), "--json-compact"))

		assert.JSONEq(t, `{"valid":true,"problems":[]}`, e.stdout.String())
	})

	t.Run("takes the current directory by default", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)

		t.Chdir(build.Dir)

		require.NoError(t, e.run("plugin", "dev", "validate"))
		assert.Contains(t, e.stderr.String(), "The plugin manifest is valid.")
	})

	t.Run("every problem is reported with the path of its field", func(t *testing.T) {
		e := newDevEnv(t)
		offline(e)

		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)
		build.writeManifestText(t, `{"id":"Bad","version":"1","apiVersion":1,"description":"d","owner":{"team":"t","bugs":"https://example.com"},"platforms":["linux-x64"],"commands":[{"name":"a","description":"d"}],"extra":true}`)

		err := e.run("plugin", "dev", "validate", build.Dir, "--json")

		require.EqualError(t, err, "the plugin manifest has 3 problems")
		assertValidationError(t, err, "plugin_manifest_invalid")

		var got display.PluginValidation
		require.NoError(t, json.Unmarshal(e.stdout.Bytes(), &got), e.stdout.String())

		assert.False(t, got.Valid)

		paths := map[string]string{}
		for _, problem := range got.Problems {
			paths[problem.Path] = problem.Message
		}

		assert.Contains(t, paths, "extra")
		assert.Contains(t, paths, "id")
		assert.Len(t, got.Problems, 3)
	})

	t.Run("the problems are a table for a person", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)
		build.writeManifestText(t, `{"id":"auth0/checkmate"}`)

		err := e.run("plugin", "dev", "validate", build.Dir)

		require.Error(t, err)
		assertValidationError(t, err, "plugin_manifest_invalid")
		assert.Contains(t, e.stdout.String(), "PATH")
		assert.Contains(t, e.stdout.String(), "version")
		assert.NotContains(t, e.stderr.String(), "is valid")
	})

	t.Run("a document that is not JSON has the root as its path", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)
		build.writeManifestText(t, "not json")

		err := e.run("plugin", "dev", "validate", build.Dir, "--json")

		require.Error(t, err)

		var got display.PluginValidation
		require.NoError(t, json.Unmarshal(e.stdout.Bytes(), &got), e.stdout.String())

		require.Len(t, got.Problems, 1)
		assert.Equal(t, "(root)", got.Problems[0].Path)
	})

	t.Run("the name of a core command is a problem", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)
		build.Manifest.ID = "auth0/apps"
		build.save(t)

		err := e.run("plugin", "dev", "validate", build.Dir, "--json")

		require.Error(t, err)

		var got display.PluginValidation
		require.NoError(t, json.Unmarshal(e.stdout.Bytes(), &got), e.stdout.String())

		require.Len(t, got.Problems, 1)
		assert.Equal(t, "id", got.Problems[0].Path)
		assert.Contains(t, got.Problems[0].Message, `auth0 already has a command named "apps"`)
	})

	t.Run("an ID that is not safe to use as a path is a problem", func(t *testing.T) {
		for id, part := range map[string]string{"auth0/con": "con", "auth0/nul": "nul", "auth0/com1": "com1", "nul/checkmate": "nul"} {
			e := newDevEnv(t)
			build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)
			build.Manifest.ID = id
			build.save(t)

			err := e.run("plugin", "dev", "validate", build.Dir, "--json")

			require.Error(t, err, id)

			var got display.PluginValidation
			require.NoError(t, json.Unmarshal(e.stdout.Bytes(), &got), e.stdout.String())

			require.Len(t, got.Problems, 1, id)
			assert.Equal(t, "id", got.Problems[0].Path, id)
			assert.Contains(t, got.Problems[0].Message, `"`+part+`" cannot be a directory name`, id)
		}
	})

	t.Run("a file or directory that cannot be read has no result", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)

		for _, path := range []string{filepath.Join(build.Dir, "missing"), t.TempDir()} {
			err := e.run("plugin", "dev", "validate", path)

			require.Error(t, err, path)
			assertValidationError(t, err, "plugin_manifest_invalid")
			assert.Empty(t, e.stdout.String())
		}
	})

	t.Run("checks the executable with --bin", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)

		require.NoError(t, e.run("plugin", "dev", "validate", build.Dir, "--bin", build.Bin))

		missing := filepath.Join(build.Dir, "missing")
		err := e.run("plugin", "dev", "validate", build.Dir, "--bin", missing, "--json-compact")

		require.EqualError(t, err, "the plugin manifest has 1 problem")
		assertValidationError(t, err, "plugin_manifest_invalid")

		var got display.PluginValidation
		require.NoError(t, json.Unmarshal(e.stdout.Bytes(), &got), e.stdout.String())

		require.Len(t, got.Problems, 1)
		assert.Equal(t, "--bin", got.Problems[0].Path)
		assert.Contains(t, got.Problems[0].Message, missing)
	})

	t.Run("takes one output format", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)

		require.Error(t, e.run("plugin", "dev", "validate", build.Dir, "--json", "--json-compact"))
		assert.Empty(t, e.stdout.String())
	})
}

func TestSplitProblem(t *testing.T) {
	tests := []struct {
		name    string
		problem string
		want    display.PluginProblem
	}{
		{"a field", "id: required field is missing", display.PluginProblem{Path: "id", Message: "required field is missing"}},
		{"a field of a command", "commands[0].name: must not be empty", display.PluginProblem{Path: "commands[0].name", Message: "must not be empty"}},
		{"a quoted key with a space", `permissions["a b"]: unknown`, display.PluginProblem{Path: `permissions["a b"]`, Message: "unknown"}},
		{"a message with a colon", "id: must match: a-z", display.PluginProblem{Path: "id", Message: "must match: a-z"}},
		{"no path", "the manifest is not valid JSON", display.PluginProblem{Path: "(root)", Message: "the manifest is not valid JSON"}},
		{"words before the colon", "the manifest is not valid JSON: unexpected end", display.PluginProblem{Path: "(root)", Message: "the manifest is not valid JSON: unexpected end"}},
		{"nothing", "", display.PluginProblem{Path: "(root)", Message: ""}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, splitProblem(test.problem))
		})
	}
}

func TestDevLinks(t *testing.T) {
	t.Run("reads nothing when dev mode is off", func(t *testing.T) {
		dir := t.TempDir()

		// A damaged file would be an error if it were read.
		require.NoError(t, os.WriteFile(filepath.Join(dir, devlink.FileName), []byte("{damaged"), 0o600))

		for _, value := range devModeValues {
			t.Setenv(pluginDevModeEnv, value)

			links, err := devLinks(dir)

			require.NoError(t, err, value)
			assert.Nil(t, links, value)
		}
	})

	t.Run("reads the links in dev mode", func(t *testing.T) {
		t.Setenv(pluginDevModeEnv, "true")

		dir := t.TempDir()
		link := devlink.Link{Path: filepath.Join(dir, "build"), Bin: filepath.Join(dir, "build", "checkmate")}
		require.NoError(t, devlink.Links{Dir: dir}.Add(context.Background(), pluginTestID, link))

		links, err := devLinks(dir)

		require.NoError(t, err)
		assert.Equal(t, map[string]devlink.Link{pluginTestID: link}, links)

		require.NoError(t, os.WriteFile(filepath.Join(dir, devlink.FileName), []byte("{damaged"), 0o600))

		_, err = devLinks(dir)
		require.ErrorIs(t, err, devlink.ErrDamaged)
	})
}

func TestLinkedPlugin(t *testing.T) {
	t.Run("a plugin that is linked is not linked when dev mode is off", func(t *testing.T) {
		e := newPluginEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)
		e.link(build)

		for _, value := range devModeValues {
			t.Setenv(pluginDevModeEnv, value)

			got, linked, err := linkedPlugin(e.cli, e.svc, pluginTestID)

			require.NoError(t, err, value)
			assert.False(t, linked, value)
			assert.Nil(t, got, value)
		}
	})

	t.Run("the build as it is on disk now", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)
		e.link(build)

		got, linked, err := linkedPlugin(e.cli, e.svc, pluginTestID)

		require.NoError(t, err)
		require.True(t, linked)
		assert.Equal(t, "1.0.0", got.Manifest.Version)

		build.Manifest.Version = "1.1.0"
		build.save(t)

		got, _, err = linkedPlugin(e.cli, e.svc, pluginTestID)

		require.NoError(t, err)
		assert.Equal(t, "1.1.0", got.Manifest.Version, "an edit shows without linking again")

		_, linked, err = linkedPlugin(e.cli, e.svc, "auth0/other")
		require.NoError(t, err)
		assert.False(t, linked)
	})

	t.Run("a link that cannot be used is an error and not a plugin that is not linked", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)
		e.link(build)
		require.NoError(t, os.Remove(build.Bin))

		_, linked, err := linkedPlugin(e.cli, e.svc, pluginTestID)

		require.Error(t, err)
		assert.True(t, linked)
		assertValidationError(t, err, "plugin_link_invalid")
		assert.Contains(t, err.Error(), "the linked plugin auth0/checkmate cannot be used")
		assert.Contains(t, err.Error(), build.Bin)
		assert.Contains(t, err.Error(), "auth0 plugin dev validate "+build.Dir)
		assert.Contains(t, err.Error(), "auth0 plugin dev unlink checkmate")
	})

	t.Run("a file of links that is damaged is a warning and no link", func(t *testing.T) {
		e := newDevEnv(t)
		e.cli.renderer = &display.Renderer{MessageWriter: e.stderr, ResultWriter: e.stdout}

		require.NoError(t, os.MkdirAll(e.Dir, 0o700))
		require.NoError(t, os.WriteFile(e.linksPath(), []byte("{damaged"), 0o600))

		_, linked, err := linkedPlugin(e.cli, e.svc, pluginTestID)

		require.NoError(t, err)
		assert.False(t, linked)
		assert.Contains(t, e.stderr.String(), "No plugin is linked")
		assert.Contains(t, e.stderr.String(), e.linksPath())
	})
}

func TestUnusableLink(t *testing.T) {
	tests := []struct {
		name string
		id   string
		path string
		want []string
	}{
		{"plain text", "auth0/checkmate", "/work/checkmate", []string{"the linked plugin auth0/checkmate cannot be used: it broke.", "validate /work/checkmate,", "unlink checkmate"}},
		{"a control character in the path", "auth0/checkmate", "/work/\x1b[31mevil", []string{"validate /work/\ufffd[31mevil,"}},
		{"a control character in the ID", "auth0/check\x1bmate", "/work/checkmate", []string{"plugin auth0/check\ufffdmate cannot", "unlink check\ufffdmate"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := unusableLink(test.id, devlink.Link{Path: test.path, Bin: test.path}, errors.New("it broke"))

			assertValidationError(t, err, "plugin_link_invalid")
			assert.NotContains(t, err.Error(), "\x1b")

			for _, want := range test.want {
				assert.Contains(t, err.Error(), want)
			}
		})
	}
}

func TestVerifiedLinked(t *testing.T) {
	build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)

	opened, err := devlink.Link{Path: build.Dir, Bin: build.Bin}.Open(pluginTestID)
	require.NoError(t, err)

	verified := verifiedLinked(opened)

	assert.Equal(t, pluginTestID, verified.ID)
	assert.Equal(t, "1.0.0", verified.Version)
	assert.Equal(t, build.Dir, verified.Dir)
	assert.Equal(t, build.Bin, verified.Bin)
	assert.Equal(t, filepath.Join(build.Dir, devlink.ManifestName), verified.ManifestPath)
	assert.Equal(t, build.Manifest.PermissionsHash(), verified.PermissionsHash, "the hash is that of the manifest just read")
}
