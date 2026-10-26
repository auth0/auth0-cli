package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeNode writes a stand-in for the node executable. It answers --version, and for a script it
// reports what it was given: the script path, the permissions of the script and its folder, the
// script's text, the arguments, the environment the tool relies on, and optionally its stdin.
func fakeNode(t *testing.T, version string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake node is a shell script")
	}

	script := `#!/bin/sh
if [ "$1" = "--version" ]; then echo "` + version + `"; exit 0; fi
script="$1"
echo "script=$script"
echo "folder_mode=$(ls -ld "$(dirname "$script")" | cut -c1-10)"
echo "file_mode=$(ls -l "$script" | cut -c1-10)"
echo "content=$(cat "$script")"
shift
echo "args=$*"
echo "bin=$AUTH0_CLI_BIN"
echo "tenant=$AUTH0_CLI_TENANT_DOMAIN"
echo "proxy_url=${AUTH0_CLI_PROXY_URL:-unset}"
echo "proxy_token=${AUTH0_CLI_PROXY_TOKEN:-unset}"
if [ -n "$FAKE_NODE_READ_STDIN" ]; then read line; echo "stdin=$line"; fi
echo "to-stderr" >&2
exit "${FAKE_NODE_EXIT:-0}"
`
	path := filepath.Join(t.TempDir(), "node")
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return path
}

// reported turns the fake node's "key=value" lines into a map.
func reported(output string) map[string]string {
	values := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		if key, value, found := strings.Cut(line, "="); found {
			values[key] = value
		}
	}
	return values
}

func TestUniversalPortalsTenantFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "no flag", args: []string{"setup", "--name", "Acme"}, want: ""},
		{name: "separate value", args: []string{"setup", "--tenant", "acme.us.auth0.com"}, want: "acme.us.auth0.com"},
		{name: "equals form", args: []string{"audit", "up_1", "--tenant=acme.eu.auth0.com"}, want: "acme.eu.auth0.com"},
		{name: "the last one wins", args: []string{"--tenant", "a.us.auth0.com", "--tenant", "b.us.auth0.com"}, want: "b.us.auth0.com"},
		{name: "no value at the end", args: []string{"setup", "--tenant"}, want: ""},
		{name: "a flag is not a value", args: []string{"setup", "--tenant", "--json"}, want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, universalPortalsTenantFlag(test.args))
		})
	}
}

func TestCheckUniversalPortalsNodeVersion(t *testing.T) {
	tests := []struct {
		version string
		wantErr string
	}{
		{version: "v22.1.0"},
		{version: "v20.12.0"},
		{version: "v20.19.4"},
		{version: "v21.0.0"},
		{version: "v20.11.1", wantErr: "20.12"},
		{version: "v18.19.0", wantErr: "20.12"},
		{version: "not a version", wantErr: "could not read"},
	}

	for _, test := range tests {
		t.Run(test.version, func(t *testing.T) {
			err := checkUniversalPortalsNodeVersion(fakeNode(t, test.version))
			if test.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.wantErr)
		})
	}
}

func TestRunUniversalPortalsScript(t *testing.T) {
	t.Run("runs the script with the arguments and the environment the tool relies on", func(t *testing.T) {
		// A proxy left in the environment must not switch the tool to another way of calling the API.
		t.Setenv("AUTH0_CLI_PROXY_URL", "https://127.0.0.1:1")
		t.Setenv("AUTH0_CLI_PROXY_TOKEN", "should-not-reach-the-tool")
		var stdout, stderr bytes.Buffer

		code, err := runUniversalPortalsScript(universalPortalsRun{
			Node:   fakeNode(t, "v22.1.0"),
			Script: []byte("console.log('hi')"),
			Args:   []string{"setup", "--json"},
			Env:    []string{"AUTH0_CLI_BIN=/opt/bin/auth0", "AUTH0_CLI_TENANT_DOMAIN=acme.us.auth0.com"},
			Stdout: &stdout,
			Stderr: &stderr,
		})

		require.NoError(t, err)
		assert.Equal(t, 0, code)
		got := reported(stdout.String())
		assert.Equal(t, "setup --json", got["args"])
		assert.Equal(t, "/opt/bin/auth0", got["bin"])
		assert.Equal(t, "acme.us.auth0.com", got["tenant"])
		assert.Equal(t, "unset", got["proxy_url"])
		assert.Equal(t, "unset", got["proxy_token"])
		assert.Equal(t, "console.log('hi')", got["content"])
		assert.Contains(t, stderr.String(), "to-stderr", "the tool's stderr must reach the user")
	})

	t.Run("keeps the script private and removes it afterwards", func(t *testing.T) {
		var stdout bytes.Buffer

		_, err := runUniversalPortalsScript(universalPortalsRun{
			Node:   fakeNode(t, "v22.1.0"),
			Script: []byte("x"),
			Stdout: &stdout,
		})

		require.NoError(t, err)
		got := reported(stdout.String())
		assert.Equal(t, "drwx------", got["folder_mode"], "only the current user may enter the folder")
		assert.Equal(t, "-rw-------", got["file_mode"], "only the current user may read the script")
		assert.NoDirExists(t, filepath.Dir(got["script"]), "the temporary folder must be gone after the run")
	})

	t.Run("hands the terminal's input to the tool", func(t *testing.T) {
		t.Setenv("FAKE_NODE_READ_STDIN", "1")
		var stdout bytes.Buffer

		_, err := runUniversalPortalsScript(universalPortalsRun{
			Node:   fakeNode(t, "v22.1.0"),
			Script: []byte("x"),
			Stdin:  strings.NewReader("an answer\n"),
			Stdout: &stdout,
		})

		require.NoError(t, err)
		assert.Equal(t, "an answer", reported(stdout.String())["stdin"])
	})

	t.Run("returns the tool's exit code, so the CLI can exit with it", func(t *testing.T) {
		t.Setenv("FAKE_NODE_EXIT", "7")

		code, err := runUniversalPortalsScript(universalPortalsRun{Node: fakeNode(t, "v22.1.0"), Script: []byte("x")})

		require.NoError(t, err)
		assert.Equal(t, 7, code)
	})

	t.Run("fails with a clear message when node cannot be started", func(t *testing.T) {
		_, err := runUniversalPortalsScript(universalPortalsRun{
			Node:   filepath.Join(t.TempDir(), "no-such-node"),
			Script: []byte("x"),
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to start Node.js")
	})

	t.Run("reports a tool that Ctrl+C stopped with the code a shell would", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("signals are not delivered the same way on Windows")
		}
		interrupted := filepath.Join(t.TempDir(), "node")
		require.NoError(t, os.WriteFile(interrupted, []byte("#!/bin/sh\nkill -INT $$\nsleep 5\n"), 0o755))

		code, err := runUniversalPortalsScript(universalPortalsRun{Node: interrupted, Script: []byte("x")})

		require.NoError(t, err)
		assert.Equal(t, 130, code, "128 plus the signal number, so scripts can tell it from a failure")
	})

	t.Run("keeps waiting for the tool when Ctrl+C reaches the CLI as well", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("signals are not delivered the same way on Windows")
		}
		// The terminal sends Ctrl+C to both processes. The CLI's own handler would exit at once and
		// cut off the tool's last message, so the runner must outlive the signal and report the
		// tool's exit code.
		slow := filepath.Join(t.TempDir(), "node")
		require.NoError(t, os.WriteFile(slow, []byte("#!/bin/sh\nsleep 1\nexit 3\n"), 0o755))
		time.AfterFunc(200*time.Millisecond, func() {
			process, findErr := os.FindProcess(os.Getpid())
			if findErr == nil {
				_ = process.Signal(syscall.SIGINT)
			}
		})

		code, err := runUniversalPortalsScript(universalPortalsRun{Node: slow, Script: []byte("x")})

		require.NoError(t, err)
		assert.Equal(t, 3, code)
	})
}

func TestEmbeddedUniversalPortalsScriptRuns(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	if checkUniversalPortalsNodeVersion(node) != nil {
		t.Skip("node is older than the tool supports")
	}
	require.NotEmpty(t, universalPortalsScript, "the bundle is not embedded")
	var stdout, stderr bytes.Buffer

	code, err := runUniversalPortalsScript(universalPortalsRun{
		Node:   node,
		Script: universalPortalsScript,
		Args:   []string{"--help"},
		Stdout: &stdout,
		Stderr: &stderr,
	})

	require.NoError(t, err)
	assert.Equal(t, 0, code, stderr.String())
	assert.Contains(t, stdout.String(), "Usage: auth0 universal-portals setup")
	assert.Contains(t, stdout.String(), "audit")
}

func TestUniversalPortalsCommand(t *testing.T) {
	cmd := universalPortalsCmd(&cli{})

	assert.Equal(t, "universal-portals", cmd.Name())
	assert.Contains(t, cmd.Aliases, "up")

	names := []string{}
	for _, sub := range cmd.Commands() {
		names = append(names, sub.Name())
		// Every flag belongs to the tool, so the CLI must hand all arguments over unparsed.
		assert.True(t, sub.DisableFlagParsing, "%s must pass its arguments through", sub.Name())
		assert.NotNil(t, sub.RunE, sub.Name())
	}
	assert.ElementsMatch(t, []string{"setup", "audit"}, names)
	assert.False(t, cmd.DisableFlagParsing, "the group itself still shows help")
}

func TestUniversalPortalsRunFor(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		args       []string
		tenant     string
		wantArgs   []string
		wantTenant string
	}{
		{
			name:       "the logged-in tenant is used when no flag names one",
			command:    "setup",
			args:       []string{"--template", "business"},
			tenant:     "acme.us.auth0.com",
			wantArgs:   []string{"setup", "--template", "business"},
			wantTenant: "acme.us.auth0.com",
		},
		{
			name:       "--tenant in the arguments wins",
			command:    "audit",
			args:       []string{"up_1", "--tenant", "other.eu.auth0.com"},
			tenant:     "acme.us.auth0.com",
			wantArgs:   []string{"audit", "up_1", "--tenant", "other.eu.auth0.com"},
			wantTenant: "other.eu.auth0.com",
		},
		{
			name:     "no tenant at all leaves the choice to the tool",
			command:  "setup",
			wantArgs: []string{"setup"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			run := universalPortalsRunFor(test.command, test.args, test.tenant, "/opt/bin/auth0", "/usr/bin/node")

			assert.Equal(t, test.wantArgs, run.Args)
			assert.Equal(t, "/usr/bin/node", run.Node)
			assert.NotEmpty(t, run.Script)
			assert.Contains(t, run.Env, "AUTH0_CLI_BIN=/opt/bin/auth0")
			if test.wantTenant == "" {
				for _, entry := range run.Env {
					assert.False(t, strings.HasPrefix(entry, "AUTH0_CLI_TENANT_DOMAIN="), entry)
				}
				return
			}
			assert.Contains(t, run.Env, "AUTH0_CLI_TENANT_DOMAIN="+test.wantTenant)
		})
	}
}

func TestRunUniversalPortalsWithoutNode(t *testing.T) {
	// Nothing is on the PATH, so nothing can be started and the user is told what to install.
	t.Setenv("PATH", t.TempDir())

	err := runUniversalPortals(&cli{}, "setup", []string{"--json"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "node is required but was not found")
	assert.Contains(t, err.Error(), "Node.js 20.12 or newer", "the message must name the version this command needs")
}
