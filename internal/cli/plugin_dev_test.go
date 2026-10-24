package cli

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/buildinfo"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/devlink"
	"github.com/auth0/auth0-cli/internal/plugins/devpolicy"
	"github.com/auth0/auth0-cli/internal/plugins/scaffold"
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

// fixturePolicy is a dev policy with the two rules of the scope table that the
// fixture index serves.
const fixturePolicy = `{"rules":[
	{"method":"GET","path":"/api/v2/clients","anyOf":["read:clients"]},
	{"method":"GET","path":"/api/v2/users","anyOf":["read:users"]}
]}`

// writePolicy writes the dev policy of the build, in its directory.
func (b devBuild) writePolicy(t *testing.T, content string) {
	t.Helper()

	require.NoError(t, os.WriteFile(filepath.Join(b.Dir, devpolicy.FileName), []byte(content), 0o600))
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

// approveBuild records the approval of a person for the permissions of the build
// and for the dev policy that it has now.
func (e *pluginEnv) approveBuild(build devBuild) {
	e.t.Helper()

	hash, _ := linkedHash(&devlink.Build{Dir: build.Dir, Manifest: &build.Manifest})

	require.NoError(e.t, e.svc.approvals.Approve(context.Background(), pluginTestTenant, build.Manifest.ID, hash))
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
			created := filepath.Join(t.TempDir(), "demo")

			commands := [][]string{
				{"plugin", "dev", "create", "demo", "--lang", "go", "--author", "Me", "--bugs", "https://example.com/issues", "--dir", created},
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
			assert.NoDirExists(t, created, "nothing is written")
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

	t.Run("asks again when the approval was for the manifest alone", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)
		build.writePolicy(t, fixturePolicy)

		require.NoError(t, e.svc.approvals.Approve(context.Background(), pluginTestTenant, build.Manifest.ID, build.Manifest.PermissionsHash()))

		require.NoError(t, e.run("plugin", "dev", "link", build.Dir, "--tenant", pluginTestTenant, "--json"))
		assert.Equal(t, true, e.jsonObject()["approvalRequired"])
	})

	t.Run("does not ask again when the approval covers the dev policy", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)
		build.writePolicy(t, fixturePolicy)
		e.approveBuild(build)

		require.NoError(t, e.run("plugin", "dev", "link", build.Dir, "--tenant", pluginTestTenant, "--json"))
		assert.Equal(t, false, e.jsonObject()["approvalRequired"])
	})

	t.Run("asks again when the dev policy changed since the approval", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)
		build.writePolicy(t, fixturePolicy)
		e.approveBuild(build)
		build.writePolicy(t, fixturePolicy+"\n")

		require.NoError(t, e.run("plugin", "dev", "link", build.Dir, "--tenant", pluginTestTenant, "--json"))
		assert.Equal(t, true, e.jsonObject()["approvalRequired"])
	})

	t.Run("covers the dev policy that --policy names", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)
		other := filepath.Join(t.TempDir(), "policy.json")
		require.NoError(t, os.WriteFile(other, []byte(fixturePolicy), 0o600))

		file, err := devpolicy.Load(other)
		require.NoError(t, err)

		require.NoError(t, e.svc.approvals.Approve(context.Background(), pluginTestTenant, build.Manifest.ID, devpolicy.ConsentHash(build.Manifest.PermissionsHash(), file)))

		require.NoError(t, e.run("plugin", "dev", "link", build.Dir, "--policy", other, "--tenant", pluginTestTenant, "--json"))
		assert.Equal(t, false, e.jsonObject()["approvalRequired"])
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

func TestLinkPluginCmd_DevPolicy(t *testing.T) {
	missingNote := "There is no dev-policy.json in "

	t.Run("a dev policy in the directory is checked and needs no note", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)
		build.writePolicy(t, fixturePolicy)

		require.NoError(t, e.run("plugin", "dev", "link", build.Dir, "--tenant", pluginTestTenant))

		assert.NotContains(t, e.stderr.String(), missingNote)
		assert.Equal(t, map[string]devlink.Link{pluginTestID: {Path: build.Dir, Bin: build.Bin}}, e.links(), "the file in the directory is not recorded")
	})

	t.Run("without a dev policy the link is made and the note says how to make one", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)

		require.NoError(t, e.run("plugin", "dev", "link", build.Dir, "--tenant", pluginTestTenant))

		out := e.stderr.String()
		assert.Contains(t, out, missingNote+build.Dir)
		assert.Contains(t, out, "a linked plugin does not run without one")
		assert.Contains(t, out, `"rules"`)
		assert.Contains(t, out, "--policy <file>")
		assert.Equal(t, map[string]devlink.Link{pluginTestID: {Path: build.Dir, Bin: build.Bin}}, e.links())
		assert.Empty(t, e.stdout.String())
	})

	t.Run("a policy file that was given is recorded as an absolute path", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)

		other := filepath.Join(build.Dir, "rules", "policy.json")
		require.NoError(t, os.MkdirAll(filepath.Dir(other), 0o700))
		require.NoError(t, os.WriteFile(other, []byte(fixturePolicy), 0o600))

		t.Chdir(build.Dir)

		require.NoError(t, e.run("plugin", "dev", "link", "--policy", filepath.Join("rules", "policy.json")))

		link := e.links()[pluginTestID]
		assert.True(t, filepath.IsAbs(link.Policy), link.Policy)
		samePath(t, other, link.Policy)
		assert.NotContains(t, e.stderr.String(), missingNote, "a file that was given needs no note")
	})

	t.Run("a policy file that was given wins over the one in the directory", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)
		build.writePolicy(t, `{"rules":`)

		other := filepath.Join(t.TempDir(), "policy.json")
		require.NoError(t, os.WriteFile(other, []byte(fixturePolicy), 0o600))

		require.NoError(t, e.run("plugin", "dev", "link", build.Dir, "--policy", other))

		assert.Equal(t, map[string]devlink.Link{pluginTestID: {Path: build.Dir, Bin: build.Bin, Policy: other}}, e.links())
	})

	t.Run("linking again without --policy drops the policy that was named", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)

		other := filepath.Join(t.TempDir(), "policy.json")
		require.NoError(t, os.WriteFile(other, []byte(fixturePolicy), 0o600))

		require.NoError(t, e.run("plugin", "dev", "link", build.Dir, "--policy", other))
		require.NoError(t, e.run("plugin", "dev", "link", build.Dir))

		assert.Empty(t, e.links()[pluginTestID].Policy)
	})

	t.Run("the JSON result has no policy", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)
		build.writePolicy(t, fixturePolicy)

		require.NoError(t, e.run("plugin", "dev", "link", build.Dir, "--tenant", pluginTestTenant, "--json"))

		object := e.jsonObject()
		assert.NotContains(t, object, "policy")
		assert.ElementsMatch(t, []string{"id", "version", "path", "bin", "banner", "approvalRequired", "approveCommand"}, slices.Collect(maps.Keys(object)))
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
			name: "a dev policy in the directory that is not JSON",
			prepare: func(t *testing.T, build devBuild) []string {
				build.writePolicy(t, `{"rules":`)

				return []string{build.Dir}
			},
			want: []string{"the dev policy is not valid", filepath.Join("{dir}", devpolicy.FileName)},
		},
		{
			name: "a dev policy in the directory with an unknown field",
			prepare: func(t *testing.T, build devBuild) []string {
				build.writePolicy(t, `{"rules":[{"method":"GET","path":"/api/v2/clients","anyOf":["read:clients"]}],"extra":1}`)

				return []string{build.Dir}
			},
			want: []string{"the dev policy is not valid", "extra"},
		},
		{
			name: "a dev policy in the directory with no rules",
			prepare: func(t *testing.T, build devBuild) []string {
				build.writePolicy(t, `{"rules":[]}`)

				return []string{build.Dir}
			},
			want: []string{"the dev policy is not valid", "no rule"},
		},
		{
			name: "a dev policy in the directory with a rule the registry would refuse",
			prepare: func(t *testing.T, build devBuild) []string {
				build.writePolicy(t, `{"rules":[{"method":"GET","path":"/clients","anyOf":["read:clients"]}]}`)

				return []string{build.Dir}
			},
			want: []string{"the dev policy is not valid", "scopeTable.rules[0].path"},
		},
		{
			name: "a policy file that was given and is not there",
			prepare: func(t *testing.T, build devBuild) []string {
				return []string{build.Dir, "--policy", filepath.Join(build.Dir, "missing.json")}
			},
			want: []string{"there is no dev policy", "missing.json"},
		},
		{
			name: "a policy file that was given and is not valid",
			prepare: func(t *testing.T, build devBuild) []string {
				other := filepath.Join(build.Dir, "other.json")
				require.NoError(t, os.WriteFile(other, []byte(`{"rules":[]}`), 0o600))

				return []string{build.Dir, "--policy", other}
			},
			want: []string{"the dev policy is not valid", "other.json"},
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

func TestValidatePluginCmd_DevPolicy(t *testing.T) {
	// The fixture plugin declares read:clients and read:users.
	tests := []struct {
		name   string
		policy string // Empty for no file.
		want   []display.PluginProblem
	}{
		{name: "a dev policy that covers the scopes", policy: fixturePolicy},
		{name: "no dev policy"},
		{
			name:   "a dev policy that cannot be used",
			policy: `{"rules":`,
			want:   []display.PluginProblem{{Path: "dev-policy.json", Message: "the dev policy is not valid"}},
		},
		{
			name:   "a dev policy with no rule",
			policy: `{"rules":[]}`,
			want:   []display.PluginProblem{{Path: "dev-policy.json", Message: "rules lists no rule"}},
		},
		{
			name:   "a scope that no rule allows",
			policy: `{"rules":[{"method":"GET","path":"/api/v2/clients","anyOf":["read:clients"]}]}`,
			want:   []display.PluginProblem{{Path: "dev-policy.json", Message: "scope read:users is declared in plugin.json but no rule allows it"}},
		},
		{
			name: "a rule that no declared scope uses",
			policy: `{"rules":[
				{"method":"GET","path":"/api/v2/clients","anyOf":["read:clients"]},
				{"method":"GET","path":"/api/v2/users","anyOf":["read:users"]},
				{"method":"DELETE","path":"/api/v2/clients/{id}","anyOf":["delete:clients"]}
			]}`,
			want: []display.PluginProblem{{Path: "dev-policy.json", Message: "rule DELETE /api/v2/clients/{id} allows only scopes that plugin.json does not declare"}},
		},
		{
			name:   "a rule that one declared scope uses is enough",
			policy: `{"rules":[{"method":"GET","path":"/api/v2/clients","anyOf":["read:clients","read:users"]}]}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := newDevEnv(t)
			build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)

			if test.policy != "" {
				build.writePolicy(t, test.policy)
			}

			// The policy is looked for next to the manifest, whether the path names the
			// directory or the file.
			for _, path := range []string{build.Dir, filepath.Join(build.Dir, devlink.ManifestName)} {
				e.stdout.Reset()
				e.stderr.Reset()

				err := e.run("plugin", "dev", "validate", path, "--json")

				if len(test.want) == 0 {
					require.NoError(t, err)
					assert.JSONEq(t, `{"valid":true,"problems":[]}`, e.stdout.String())

					continue
				}

				require.Error(t, err)
				assertValidationError(t, err, "plugin_manifest_invalid")

				var got display.PluginValidation
				require.NoError(t, json.Unmarshal(e.stdout.Bytes(), &got), e.stdout.String())

				assert.False(t, got.Valid)
				require.Len(t, got.Problems, len(test.want))

				for n, want := range test.want {
					assert.Equal(t, want.Path, got.Problems[n].Path)
					assert.Contains(t, got.Problems[n].Message, want.Message)
				}
			}
		})
	}

	t.Run("problems only in the dev policy are not blamed on the manifest", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)
		build.writePolicy(t, `{"rules":`)

		err := e.run("plugin", "dev", "validate", build.Dir, "--json")

		require.EqualError(t, err, "the dev policy has 1 problem")
		assertValidationError(t, err, "plugin_manifest_invalid")
	})

	t.Run("the problems of the manifest and of the dev policy are listed together", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "1.0.0", fixtureCommands(), stubExecutable)
		build.writePolicy(t, `{"rules":`)
		build.writeManifestText(t, `{"id":"Bad","version":"1","apiVersion":1,"description":"d","owner":{"team":"t","bugs":"https://example.com"},"platforms":["linux-x64"],"commands":[{"name":"a","description":"d"}],"extra":true}`)

		err := e.run("plugin", "dev", "validate", build.Dir, "--json")

		require.EqualError(t, err, "the plugin has 4 problems")

		var got display.PluginValidation
		require.NoError(t, json.Unmarshal(e.stdout.Bytes(), &got), e.stdout.String())

		paths := []string{}
		for _, problem := range got.Problems {
			paths = append(paths, problem.Path)
		}

		assert.Contains(t, paths, "dev-policy.json")
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
	assert.Equal(t, build.Manifest.PermissionsHash(), verified.PermissionsHash, "without a dev policy the hash is that of the manifest just read")

	t.Run("the hash covers the dev policy", func(t *testing.T) {
		build.writePolicy(t, fixturePolicy)

		file, err := devpolicy.Load(filepath.Join(build.Dir, devpolicy.FileName))
		require.NoError(t, err)

		assert.Equal(t, devpolicy.ConsentHash(build.Manifest.PermissionsHash(), file), verifiedLinked(opened).PermissionsHash)
	})

	t.Run("the hash covers the dev policy that the link names", func(t *testing.T) {
		other := filepath.Join(t.TempDir(), "policy.json")
		require.NoError(t, os.WriteFile(other, []byte(fixturePolicy+"\n"), 0o600))

		named, err := devlink.Link{Path: build.Dir, Bin: build.Bin, Policy: other}.Open(pluginTestID)
		require.NoError(t, err)

		file, err := devpolicy.Load(other)
		require.NoError(t, err)

		assert.Equal(t, devpolicy.ConsentHash(build.Manifest.PermissionsHash(), file), verifiedLinked(named).PermissionsHash)
		assert.NotEqual(t, verifiedLinked(opened).PermissionsHash, verifiedLinked(named).PermissionsHash)
	})

	t.Run("a dev policy that cannot be read leaves the hash of the manifest", func(t *testing.T) {
		build.writePolicy(t, `{"rules":`)

		assert.Equal(t, build.Manifest.PermissionsHash(), verifiedLinked(opened).PermissionsHash)
	})

	t.Run("a dev policy that loads but that the registry would refuse leaves the hash of the manifest", func(t *testing.T) {
		// The path has no /api/v2 prefix, so the file is read but the scope table is not valid.
		build.writePolicy(t, `{"rules":[{"method":"GET","path":"/clients","anyOf":["read:clients"]}]}`)

		file, err := devpolicy.Load(filepath.Join(build.Dir, devpolicy.FileName))
		require.NoError(t, err)

		_, err = file.Index()
		require.Error(t, err, "the file must load and be refused by Index for this case to mean anything")

		hash, used := linkedHash(opened)

		assert.Equal(t, build.Manifest.PermissionsHash(), hash)
		assert.Nil(t, used)
		assert.Equal(t, build.Manifest.PermissionsHash(), verifiedLinked(opened).PermissionsHash)
	})
}

// createArgs returns the arguments that create the plugin demo in a Go project
// in dir. A flag that is given again later in more wins.
func createArgs(dir string, more ...string) []string {
	args := []string{"plugin", "dev", "create", "demo", "--lang", "go", "--author", "Platform team", "--bugs", "https://example.com/issues", "--dir", dir}

	return append(args, more...)
}

// createdManifest reads and parses the plugin.json of a created project.
func createdManifest(t *testing.T, dir string) *plugins.Manifest {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(dir, devlink.ManifestName))
	require.NoError(t, err)

	manifest, err := plugins.ParseManifest(data)
	require.NoError(t, err)

	return manifest
}

func TestCreatePluginCmd(t *testing.T) {
	tests := []struct {
		lang    string
		build   string
		release string
	}{
		{"go", "go mod tidy && go build -o demo .", "make release BUMP=minor"},
		{"ts", "bun install && bun run build", "bun run release minor"},
	}

	for _, test := range tests {
		t.Run("writes a "+test.lang+" project, says what to do next, and keeps the next steps out of the JSON", func(t *testing.T) {
			e := newDevEnv(t)
			dir := filepath.Join(t.TempDir(), "demo")

			require.NoError(t, e.run(createArgs(dir, "--lang", test.lang)...))

			assert.Empty(t, e.stdout.String())

			out := e.stderr.String()
			assert.Contains(t, out, "Created auth0/demo in "+dir)
			assert.Contains(t, out, "cd "+dir)
			assert.Contains(t, out, test.build)
			assert.Contains(t, out, "AUTH0_CLI_PLUGIN_DEV_MODE=true auth0 plugin dev link")
			assert.Contains(t, out, "auth0 plugin approve demo")
			assert.Contains(t, out, "\n  auth0 demo\n")
			assert.Contains(t, out, "the first push to main releases version 0.1.0. Release again with: "+test.release)
			assert.NotContains(t, out, "declares no scopes")

			manifest := createdManifest(t, dir)
			assert.Equal(t, "auth0/demo", manifest.ID)
			assert.Equal(t, "Platform team", manifest.Owner.Team)
			assert.Equal(t, "https://example.com/issues", manifest.Owner.Bugs)
			assert.Equal(t, []string{"read:clients"}, manifest.Commands[0].Permissions.Scopes)
			assert.Nil(t, manifest.Engines, "a build with no version requires nothing")

			assert.NoDirExists(t, e.Dir, "the plugins directory is not touched")
		})

		t.Run("prints the result as JSON for "+test.lang, func(t *testing.T) {
			e := newDevEnv(t)
			dir := filepath.Join(t.TempDir(), "demo")

			require.NoError(t, e.run(createArgs(dir, "--lang", test.lang, "--json-compact")...))

			object := e.jsonObject()
			assert.Len(t, object, 4, "dir, files, id and lang")
			assert.Equal(t, dir, object["dir"])
			assert.Equal(t, "auth0/demo", object["id"])
			assert.Equal(t, test.lang, object["lang"])

			list, ok := object["files"].([]any)
			require.True(t, ok)

			files := make([]string, 0, len(list))
			for _, file := range list {
				files = append(files, file.(string))
				assert.FileExists(t, filepath.Join(dir, filepath.FromSlash(file.(string))))
			}

			assert.Contains(t, files, "plugin.json")
			assert.Contains(t, files, "README.md")
			assert.IsIncreasing(t, files)

			assert.NotContains(t, e.stdout.String(), "go mod tidy", "the next steps are for a person")
			assert.Contains(t, e.stderr.String(), test.build)
		})
	}

	t.Run("takes every value from the flags", func(t *testing.T) {
		e := newDevEnv(t)
		dir := filepath.Join(t.TempDir(), "elsewhere")

		require.NoError(t, e.run(createArgs(dir,
			"--version", "1.2.3",
			"--description", "Counts things",
			"--command", "lister",
			"--command-description", "Lists them",
			"--license", "Apache-2.0",
			"--scopes", "read:clients, read:users",
		)...))

		manifest := createdManifest(t, dir)
		assert.Equal(t, "1.2.3", manifest.Version)
		assert.Equal(t, "Counts things", manifest.Description)
		assert.Equal(t, "lister", manifest.Commands[0].Name)
		assert.Equal(t, "Lists them", manifest.Commands[0].Description)
		assert.Equal(t, []string{"read:clients", "read:users"}, manifest.Commands[0].Permissions.Scopes)

		readme, err := os.ReadFile(filepath.Join(dir, "README.md"))
		require.NoError(t, err)
		assert.Contains(t, string(readme), "Apache-2.0")

		out := e.stderr.String()
		assert.Contains(t, out, "auth0 demo lister", "a command that is not the root command runs under the name of the plugin")
		assert.Contains(t, out, "version 1.2.3")
	})

	t.Run("the command and its description follow the name and the description", func(t *testing.T) {
		e := newDevEnv(t)
		dir := filepath.Join(t.TempDir(), "demo")

		require.NoError(t, e.run(createArgs(dir, "--description", "Counts things")...))

		manifest := createdManifest(t, dir)
		assert.Equal(t, "demo", manifest.Commands[0].Name)
		assert.Equal(t, "Counts things", manifest.Commands[0].Description)
		assert.NotNil(t, manifest.RootCommand())
	})

	t.Run("the directory is named like the plugin by default", func(t *testing.T) {
		e := newDevEnv(t)
		parent := t.TempDir()
		t.Chdir(parent)

		args := createArgs("")
		require.NoError(t, e.run(args[:len(args)-2]...))

		assert.FileExists(t, filepath.Join(parent, "demo", devlink.ManifestName))
		assert.Contains(t, e.stderr.String(), "cd demo\n")
	})

	t.Run("no scopes is allowed and the person is told", func(t *testing.T) {
		e := newDevEnv(t)
		dir := filepath.Join(t.TempDir(), "demo")

		require.NoError(t, e.run(createArgs(dir, "--scopes", "")...))

		assert.Empty(t, createdManifest(t, dir).Commands[0].Permissions.Scopes)
		assert.Contains(t, e.stderr.String(), "The first command declares no scopes, so the Management API call of the sample is denied")
	})

	t.Run("no scopes is a warning in the JSON result, and a project with scopes has none", func(t *testing.T) {
		e := newDevEnv(t)

		require.NoError(t, e.run(createArgs(filepath.Join(t.TempDir(), "demo"), "--scopes", "", "--json-compact")...))

		assert.Equal(t, []any{"The first command declares no scopes, so the Management API call of the sample is denied. Add the scopes it needs under permissions.scopes in plugin.json."}, e.jsonObject()["warnings"])

		require.NoError(t, e.run(createArgs(filepath.Join(t.TempDir(), "demo"), "--json-compact")...))

		assert.NotContains(t, e.jsonObject(), "warnings")
	})

	t.Run("requires the version of this CLI, which has the plugin commands only as a prerelease", func(t *testing.T) {
		original := buildinfo.Version
		t.Cleanup(func() { buildinfo.Version = original })

		buildinfo.Version = "1.40.0-beta.1"

		e := newDevEnv(t)
		dir := filepath.Join(t.TempDir(), "demo")

		require.NoError(t, e.run(createArgs(dir)...))

		assert.Equal(t, ">=1.40.0", createdManifest(t, dir).Engines.Auth0CLI)
	})

	t.Run("fills an empty directory", func(t *testing.T) {
		e := newDevEnv(t)
		dir := t.TempDir()

		require.NoError(t, e.run(createArgs(dir)...))

		assert.FileExists(t, filepath.Join(dir, devlink.ManifestName))
	})

	t.Run("uses the starter projects of a directory given with --from", func(t *testing.T) {
		e := newDevEnv(t)
		from := t.TempDir()
		dir := filepath.Join(t.TempDir(), "demo")

		require.NoError(t, os.MkdirAll(filepath.Join(from, "go"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(from, "go", "main.go.tmpl"), []byte("// __PLUGIN_NAME__\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(from, "go", "plain.txt"), []byte("as it is\n"), 0o644))

		require.NoError(t, e.run(createArgs(dir, "--from", from, "--json-compact")...))

		assert.Equal(t, []any{"main.go", "plain.txt", "plugin.json"}, e.jsonObject()["files"])

		main, err := os.ReadFile(filepath.Join(dir, "main.go"))
		require.NoError(t, err)
		assert.Equal(t, "// demo\n", string(main))
	})
}

func TestCreatePluginCmd_Refusals(t *testing.T) {
	// A tree for --from with a symlink in it.
	symlinkTree := func(t *testing.T) string {
		t.Helper()

		from := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(from, "go"), 0o755))

		if err := os.Symlink(from, filepath.Join(from, "go", "link")); err != nil {
			t.Skipf("cannot create a symlink here: %v", err)
		}

		return from
	}

	tests := []struct {
		name string

		// Args returns the arguments after "plugin dev create", given the directory
		// to create the project in.
		args   func(t *testing.T, dir string) []string
		reason string
		want   []string
	}{
		{
			name:   "a name that is a core command",
			args:   func(t *testing.T, dir string) []string { return append([]string{"apps"}, createArgs(dir)[4:]...) },
			reason: "plugin_name_reserved",
			want:   []string{`already has a command named "apps"`},
		},
		{
			name:   "a name that is not a name",
			args:   func(t *testing.T, dir string) []string { return append([]string{"My_Plugin"}, createArgs(dir)[4:]...) },
			reason: "plugin_create_invalid",
			want:   []string{"My_Plugin", "lowercase"},
		},
		{
			name: "no name and no terminal to ask in",
			args: func(t *testing.T, dir string) []string {
				return append([]string{}, createArgs(dir)[4:]...)
			},
			reason: "plugin_create_invalid",
			want:   []string{"<name> is required"},
		},
		{
			name:   "no language",
			args:   func(t *testing.T, dir string) []string { return withoutFlag(createArgs(dir)[3:], "--lang") },
			reason: "plugin_create_invalid",
			want:   []string{"--lang is required"},
		},
		{
			name:   "another language",
			args:   func(t *testing.T, dir string) []string { return append(createArgs(dir)[3:], "--lang", "rust") },
			reason: "plugin_create_invalid",
			want:   []string{`"rust"`, "go and ts"},
		},
		{
			name:   "no author and no terminal to ask in",
			args:   func(t *testing.T, dir string) []string { return withoutFlag(createArgs(dir)[3:], "--author") },
			reason: "plugin_create_invalid",
			want:   []string{"--author is required"},
		},
		{
			name:   "no bugs URL and no terminal to ask in",
			args:   func(t *testing.T, dir string) []string { return withoutFlag(createArgs(dir)[3:], "--bugs") },
			reason: "plugin_create_invalid",
			want:   []string{"--bugs is required"},
		},
		{
			name: "a bugs URL without TLS",
			args: func(t *testing.T, dir string) []string {
				return append(createArgs(dir)[3:], "--bugs", "http://example.com")
			},
			reason: "plugin_create_invalid",
			want:   []string{"owner.bugs"},
		},
		{
			name:   "a version that is not semver",
			args:   func(t *testing.T, dir string) []string { return append(createArgs(dir)[3:], "--version", "one") },
			reason: "plugin_create_invalid",
			want:   []string{"version"},
		},
		{
			name: "a description of 121 characters",
			args: func(t *testing.T, dir string) []string {
				return append(createArgs(dir)[3:], "--description", strings.Repeat("x", 121))
			},
			reason: "plugin_create_invalid",
			want:   []string{"description"},
		},
		{
			name:   "a scope that is not one",
			args:   func(t *testing.T, dir string) []string { return append(createArgs(dir)[3:], "--scopes", "everything") },
			reason: "plugin_create_invalid",
			want:   []string{"scopes"},
		},
		{
			name:   "a license that is code",
			args:   func(t *testing.T, dir string) []string { return append(createArgs(dir)[3:], "--license", "MIT;ls") },
			reason: "plugin_create_invalid",
			want:   []string{"license"},
		},
		{
			name:   "a command that is not a name",
			args:   func(t *testing.T, dir string) []string { return append(createArgs(dir)[3:], "--command", "List All") },
			reason: "plugin_create_invalid",
			want:   []string{"command"},
		},
		{
			name:   "an ID of another registry",
			args:   func(t *testing.T, dir string) []string { return append(createArgs(dir)[3:], "--id", "acme/demo") },
			reason: "plugin_create_invalid",
			want:   []string{"auth0/demo"},
		},
		{
			name: "--from that does not exist",
			args: func(t *testing.T, dir string) []string {
				return append(createArgs(dir)[3:], "--from", filepath.Join(dir, "nope"))
			},
			reason: "plugin_create_invalid",
			want:   []string{"cannot use --from"},
		},
		{
			name: "--from that is a file",
			args: func(t *testing.T, dir string) []string {
				file := filepath.Join(t.TempDir(), "file")
				require.NoError(t, os.WriteFile(file, []byte("x"), 0o644))

				return append(createArgs(dir)[3:], "--from", file)
			},
			reason: "plugin_create_invalid",
			want:   []string{"is not a directory"},
		},
		{
			name:   "--from with a symlink",
			args:   func(t *testing.T, dir string) []string { return append(createArgs(dir)[3:], "--from", symlinkTree(t)) },
			reason: "plugin_create_invalid",
			want:   []string{"link", "symlink is not followed"},
		},
		{
			name: "--from with too many files",
			args: func(t *testing.T, dir string) []string {
				from := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(from, "go"), 0o755))

				for i := range 65 {
					require.NoError(t, os.WriteFile(filepath.Join(from, "go", "f"+strconv.Itoa(i)), []byte("x"), 0o644))
				}

				return append(createArgs(dir)[3:], "--from", from)
			},
			reason: "plugin_create_invalid",
			want:   []string{"more than 64 files"},
		},
		{
			name: "--from with a file that is too big",
			args: func(t *testing.T, dir string) []string {
				from := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(from, "go"), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(from, "go", "big"), []byte(strings.Repeat("x", 64<<10+1)), 0o644))

				return append(createArgs(dir)[3:], "--from", from)
			},
			reason: "plugin_create_invalid",
			want:   []string{"big is larger than 65536 bytes"},
		},
		{
			name: "--from with a token that is not in the set",
			args: func(t *testing.T, dir string) []string {
				from := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(from, "go"), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(from, "go", "a.tmpl"), []byte("__AUTHOR__"), 0o644))

				return append(createArgs(dir)[3:], "--from", from)
			},
			reason: "plugin_create_invalid",
			want:   []string{"__AUTHOR__"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := newDevEnv(t)
			dir := filepath.Join(t.TempDir(), "demo")

			args := test.args(t, dir)
			if args[0] != "plugin" {
				args = append([]string{"plugin", "dev", "create"}, args...)
			}

			err := e.run(args...)

			require.Error(t, err)
			assertValidationError(t, err, test.reason)

			for _, want := range test.want {
				assert.Contains(t, err.Error(), want)
			}

			assert.Empty(t, e.stdout.String())
			assert.NoDirExists(t, dir, "nothing is written")
			assert.NoDirExists(t, e.Dir)
		})
	}

	t.Run("a directory that is not empty is left as it is", func(t *testing.T) {
		e := newDevEnv(t)
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "mine.txt"), []byte("mine"), 0o644))

		err := e.run(createArgs(dir)...)

		require.Error(t, err)
		assertValidationError(t, err, "plugin_create_invalid")
		assert.Contains(t, err.Error(), "never overwrite a non-empty directory")
		assert.Contains(t, err.Error(), dir)

		list, readErr := os.ReadDir(dir)
		require.NoError(t, readErr)
		require.Len(t, list, 1)
		assert.Equal(t, "mine.txt", list[0].Name())
	})

	t.Run("a failure that is not the input is a plain error that names the path", func(t *testing.T) {
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			t.Skip("the mode of a directory does not stop this user")
		}

		e := newDevEnv(t)
		parent := t.TempDir()
		require.NoError(t, os.Chmod(parent, 0o500))
		t.Cleanup(func() { _ = os.Chmod(parent, 0o700) })

		dir := filepath.Join(parent, "demo")

		err := e.run(createArgs(dir)...)

		require.Error(t, err)
		assert.ErrorContains(t, err, "cannot create "+dir)
		assert.NotEqual(t, "validation", errorClass(err))
		assert.NotEqual(t, "plugin_create_invalid", errorReason(err))
	})

	t.Run("the path of a file that --from holds is safe to print", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows has no file names with control characters")
		}

		e := newDevEnv(t)
		from := symlinkTree(t)
		require.NoError(t, os.Symlink(from, filepath.Join(from, "go", "evil\x1b[31m")))

		err := e.run(createArgs(filepath.Join(t.TempDir(), "demo"), "--from", from)...)

		require.Error(t, err)
		assert.NotContains(t, err.Error(), "\x1b")
	})
}

// withoutFlag returns the arguments without a flag and its value.
func withoutFlag(args []string, flag string) []string {
	var kept []string

	for i := 0; i < len(args); i++ {
		if args[i] == flag {
			i++

			continue
		}

		kept = append(kept, args[i])
	}

	return kept
}

func TestCreateNextSteps(t *testing.T) {
	opts := scaffold.Options{Name: "demo", Command: "demo", Dir: "demo", Lang: scaffold.LangGo}

	tests := []struct {
		name   string
		change func(*scaffold.Options)
		want   []string
	}{
		{
			name: "go",
			want: []string{"cd demo", "go mod tidy && go build -o demo .", "AUTH0_CLI_PLUGIN_DEV_MODE=true auth0 plugin dev link", "auth0 plugin approve demo", "auth0 demo"},
		},
		{
			name:   "typescript",
			change: func(o *scaffold.Options) { o.Lang = scaffold.LangTypeScript },
			want:   []string{"cd demo", "bun install && bun run build", "AUTH0_CLI_PLUGIN_DEV_MODE=true auth0 plugin dev link", "auth0 plugin approve demo", "auth0 demo"},
		},
		{
			name:   "another command",
			change: func(o *scaffold.Options) { o.Command = "lister" },
			want:   []string{"cd demo", "go mod tidy && go build -o demo .", "AUTH0_CLI_PLUGIN_DEV_MODE=true auth0 plugin dev link", "auth0 plugin approve demo", "auth0 demo lister"},
		},
		{
			name:   "a directory with a space",
			change: func(o *scaffold.Options) { o.Dir = "my plugins/demo" },
			want:   []string{"cd " + shellWord("my plugins/demo"), "go mod tidy && go build -o demo .", "AUTH0_CLI_PLUGIN_DEV_MODE=true auth0 plugin dev link", "auth0 plugin approve demo", "auth0 demo"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			o := opts
			if test.change != nil {
				test.change(&o)
			}

			assert.Equal(t, test.want, createNextSteps(o))
		})
	}
}

func TestQuoteWord(t *testing.T) {
	tests := []struct{ path, posix, windows string }{
		{"demo", "demo", "demo"},
		{"./plugins/demo", "./plugins/demo", "./plugins/demo"},
		{`C:\work\demo`, `'C:\work\demo'`, `"C:\work\demo"`},
		{"my dir", "'my dir'", `"my dir"`},
		{"it's", `'it'\''s'`, `"it's"`},
		{"a$b", "'a$b'", `"a$b"`},
		{"a;b", "'a;b'", `"a;b"`},
		{"a\x1bb", "a\ufffdb", "a\ufffdb"},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			assert.Equal(t, test.posix, quoteWord(test.path, false), "POSIX shells")
			assert.Equal(t, test.windows, quoteWord(test.path, true), "Windows shells")
		})
	}
}

func TestCreateFailed(t *testing.T) {
	err := createFailed(errors.New("cannot create /tmp/x\x1b[31m: no space left on device"))

	assert.EqualError(t, err, "cannot create /tmp/x\ufffd[31m: no space left on device")
	assert.NotEqual(t, "validation", errorClass(err))
}
