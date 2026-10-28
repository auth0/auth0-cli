package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gokeyring "github.com/zalando/go-keyring"

	"github.com/auth0/auth0-cli/internal/buildinfo"
	"github.com/auth0/auth0-cli/internal/config"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/keyring"
	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/approvals"
	"github.com/auth0/auth0-cli/internal/plugins/devlink"
	"github.com/auth0/auth0-cli/internal/plugins/proxy"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
	"github.com/auth0/auth0-cli/internal/plugins/registry/registrytest"
	"github.com/auth0/auth0-cli/internal/plugins/runner"
)

// pluginFixtureEnv is the variable that the scan command of the fixture plugin
// declares in its permissions.
const pluginFixtureEnv = "FIXTURE_DECLARED"

// TestMain lets the test binary act as the plugin. A copy of the binary that is
// installed as checkmate runs the fixture instead of the tests, so the commands
// start a real process and no shell script is needed on any system.
func TestMain(m *testing.M) {
	if code, ok := runPluginFixture(os.Args); ok {
		exitFixture(code)
	}

	os.Exit(m.Run())
}

// fixtureReportDoc is what the fixture plugin prints in its report mode.
type fixtureReportDoc struct {
	Args []string          `json:"args"`
	Env  map[string]string `json:"env"`
}

// runPluginFixture runs the fixture plugin when the binary was started under the
// name of the plugin. Its mode comes from --mode=, and the other options from
// --code=, --path= and --marker=. Everything else is only reported, which is how
// a test sees the arguments that the CLI passed on.
func runPluginFixture(args []string) (code int, ok bool) {
	if filepath.Base(args[0]) != registrytest.BinName(plugins.CurrentPlatform(), pluginTestBin) {
		return 0, false
	}

	options := map[string]string{}

	for _, arg := range args[1:] {
		if arg == "--" {
			break
		}

		if name, value, found := strings.Cut(arg, "="); found && strings.HasPrefix(name, "--") {
			options[name] = value
		}
	}

	switch options["--mode"] {
	case "exit":
		code, _ := strconv.Atoi(options["--code"])

		return code, true
	case "sleep":
		_ = os.WriteFile(options["--marker"], []byte(strconv.Itoa(os.Getpid())), 0o600)

		time.Sleep(time.Minute)

		return 0, true
	case "call":
		code := fixtureCall(options["--path"])

		// A call that is followed by a failure, with --code.
		if exit, err := strconv.Atoi(options["--code"]); err == nil && code == 0 {
			code = exit
		}

		return code, true
	default:
		return fixtureReport(args[1:]), true
	}
}

// fixtureReport prints the arguments and the environment of the plugin as JSON on
// stdout, and a line on stderr.
func fixtureReport(args []string) int {
	env := map[string]string{}

	for _, entry := range os.Environ() {
		name, value, _ := strings.Cut(entry, "=")
		env[name] = value
	}

	data, err := json.Marshal(fixtureReportDoc{Args: append([]string{}, args...), Env: env})
	if err != nil {
		return 2
	}

	fmt.Fprintln(os.Stdout, string(data))
	fmt.Fprintln(os.Stderr, "fixture stderr")

	return 0
}

// fixtureCall makes a call to the Management API the way a plugin does, through
// the proxy with the secret of the run, and prints the status and the body.
func fixtureCall(path string) int {
	certificate, err := os.ReadFile(os.Getenv("AUTH0_PLUGIN_CA_CERT"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)

		return 2
	}

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(certificate)

	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}}

	request, err := http.NewRequest(http.MethodGet, "https://"+os.Getenv("AUTH0_PLUGIN_DOMAIN")+path, nil)
	if err != nil {
		return 2
	}

	request.Header.Set("Authorization", "Bearer "+os.Getenv("AUTH0_PLUGIN_MANAGEMENT_TOKEN"))

	response, err := client.Do(request)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)

		return 2
	}

	defer response.Body.Close()

	body, _ := io.ReadAll(response.Body)

	fmt.Fprintf(os.Stdout, "status=%d\n%s\n", response.StatusCode, body)

	return 0
}

// fixtureBinary returns the test binary, which is the executable of every
// fixture plugin.
var fixtureBinary = sync.OnceValues(func() ([]byte, error) {
	path, err := os.Executable()
	if err != nil {
		return nil, err
	}

	return os.ReadFile(path)
})

// fixtureArchive returns a zip archive with the test binary under the given name
// and the manifest. The binary is stored without compression, because it is large
// and compressing it would make every test slow.
func fixtureArchive(t *testing.T, name string, manifest []byte) []byte {
	t.Helper()

	binary, err := fixtureBinary()
	require.NoError(t, err)

	var buffer bytes.Buffer

	buffer.Grow(len(binary) + len(manifest) + 4096)

	archive := zip.NewWriter(&buffer)

	for _, entry := range []struct {
		name string
		body []byte
	}{{name, binary}, {"plugin.json", manifest}} {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Store}
		header.SetMode(0o755)

		writer, err := archive.CreateHeader(header)
		require.NoError(t, err)

		_, err = writer.Write(entry.body)
		require.NoError(t, err)
	}

	require.NoError(t, archive.Close())

	return buffer.Bytes()
}

// fixturePermissions returns permissions with the given scopes, and the
// environment variables that the command may read.
func fixturePermissions(scopes []string, env ...string) *plugins.Permissions {
	permissions := &plugins.Permissions{
		Scopes: scopes,
		Hosts:  []plugins.HostPermission{},
		Files:  []plugins.FilePermission{},
		Env:    []plugins.EnvPermission{},
	}

	for _, name := range env {
		permissions.Env = append(permissions.Env, plugins.EnvPermission{Name: name, Reason: "Read a setting"})
	}

	return permissions
}

// fixtureCommands returns the commands of the fixture plugin: a root command
// that can read clients, scan that can read users and read one variable of the
// environment, and report that supports JSON output only.
func fixtureCommands() []plugins.Command {
	return []plugins.Command{
		{
			Name:        pluginTestBin,
			Description: "Check a tenant",
			Flags:       []plugins.Flag{{Name: "validators", Shorthand: "v", Type: plugins.FlagTypeString, Description: "The validators to run"}},
			Output:      &plugins.Output{JSON: true, CSV: true, AgentMode: true},
			Permissions: fixturePermissions([]string{"read:clients"}),
		},
		{
			Name:        "scan",
			Description: "Scan the users",
			Flags:       []plugins.Flag{{Name: "depth", Type: plugins.FlagTypeInt, Description: "How deep to scan"}},
			Timeout:     "30s",
			Permissions: fixturePermissions([]string{"read:users"}, pluginFixtureEnv),
		},
		{
			Name:        "report",
			Description: "Write a report",
			Output:      &plugins.Output{JSON: true},
		},
	}
}

// fakeTenant stands in for the Management API. It answers every call with a
// list of clients that includes a secret, and remembers what it was asked.
type fakeTenant struct {
	mu       sync.Mutex
	requests []fakeTenantRequest
}

type fakeTenantRequest struct {
	path          string
	authorization string
}

func (f *fakeTenant) RoundTrip(request *http.Request) (*http.Response, error) {
	f.mu.Lock()
	f.requests = append(f.requests, fakeTenantRequest{path: request.URL.Path, authorization: request.Header.Get("Authorization")})
	f.mu.Unlock()

	body := `{"clients":[{"name":"app","client_secret":"shh"}]}`

	return &http.Response{
		StatusCode:    http.StatusOK,
		Header:        http.Header{"Content-Type": {"application/json"}},
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       request,
	}, nil
}

func (f *fakeTenant) seen() []fakeTenantRequest {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.requests)
}

// runEnv is a pluginEnv with the fixture plugin installed and a session that
// needs no tenant: the login of the plugin run is replaced, and so is the tenant
// the proxy calls.
type runEnv struct {
	*pluginEnv

	record registry.VersionRecord
	tenant *fakeTenant

	// Logins counts the sessions that runs opened since the last run started, and
	// LoginErr, when it is set, is what opening one returns.
	logins   int
	loginErr error

	// Token and Domain are the access token and the tenant of the session that a
	// login opens.
	Token  string
	Domain string
}

// newRunEnv returns an environment with version 1.0.0 of the fixture plugin
// installed, which is not approved yet. The changes are applied to the index
// before it is published.
func newRunEnv(t *testing.T, commands []plugins.Command, change ...func(*registrytest.IndexBuilder)) *runEnv {
	t.Helper()

	e := newPluginEnv(t)
	r := &runEnv{pluginEnv: e, tenant: &fakeTenant{}, Token: "session-token-1", Domain: pluginTestTenant}

	// The session is the one of a person who logged in with a browser, which is
	// saved in the config file of the home directory of the test.
	saveTenant(t, config.Tenant{Domain: pluginTestTenant})

	r.record = r.releaseFixture("1.0.0", commands)
	r.publishFixture(change, r.record)
	e.installPlugin(pluginTestID, "1.0.0")

	e.cli.Config.DefaultTenant = pluginTestTenant
	e.svc.transport = r.tenant
	e.svc.login = func(_ context.Context, c *cli) error {
		r.logins++

		if r.loginErr != nil {
			return r.loginErr
		}

		c.accessToken = r.Token
		c.sessionDomain = r.Domain

		return nil
	}

	return r
}

// fixtureManifest returns the manifest of a version of the fixture plugin.
func fixtureManifest(platform plugins.Platform, version string, commands []plugins.Command) plugins.Manifest {
	return plugins.Manifest{
		ID:          pluginTestID,
		Version:     version,
		APIVersion:  1,
		Description: "Checks the tenant",
		Owner:       plugins.Owner{Team: "Test team", Bugs: "https://example.com/bugs"},
		Platforms:   []plugins.Platform{platform},
		Commands:    commands,
	}
}

// releaseFixture builds a version of the fixture plugin with the given commands
// and serves its archive. The record has the permissions and commands of the
// manifest, as the registry requires.
func (r *runEnv) releaseFixture(version string, commands []plugins.Command) registry.VersionRecord {
	r.t.Helper()

	manifest := fixtureManifest(r.platform, version, commands)

	record := registrytest.NewVersion(version)
	record.Platforms = manifest.Platforms
	record.Permissions = manifest.Permissions()
	record.Commands = manifest.ConsentCommands()
	record = registrytest.Seal(record)

	data, err := json.Marshal(manifest)
	require.NoError(r.t, err)

	bin := registrytest.BinName(r.platform, pluginTestBin)
	archive := fixtureArchive(r.t, bin, data)
	path := "/auth0/auth0-cli-plugins/releases/download/v" + version + "/" + pluginTestBin + ".zip"

	record.Artifacts = []registry.Artifact{
		registrytest.NewArtifact(r.platform, "https://github.com"+path, registry.ArtifactFormatZip, bin, archive),
	}

	r.server.Set(path, archive)

	return record
}

// fixtureIndex returns the builder of an index with the records, a scope table
// for two calls and a secret field.
func (r *runEnv) fixtureIndex(records ...registry.VersionRecord) *registrytest.IndexBuilder {
	return r.index(records...).
		SetScopeTable(registry.ScopeTable{FormatVersion: 1, Rules: []registry.ScopeRule{
			{Method: "GET", Path: "/api/v2/clients", AnyOf: []string{"read:clients"}},
			{Method: "GET", Path: "/api/v2/users", AnyOf: []string{"read:users"}},
		}}).
		SetSecretFields(registry.SecretFields{FormatVersion: 1, Names: []string{"client_secret"}})
}

func (r *runEnv) publishFixture(change []func(*registrytest.IndexBuilder), records ...registry.VersionRecord) {
	r.t.Helper()

	builder := r.fixtureIndex(records...)

	for _, apply := range change {
		apply(builder)
	}

	r.publish(builder)
}

// approve approves the installed version of the plugin for the tenant of the
// environment, the way a person does at a terminal.
func (r *runEnv) approve() {
	r.t.Helper()

	require.NoError(r.t, r.svc.approvals.Approve(context.Background(), pluginTestTenant, pluginTestID, r.record.PermissionsHash))
}

// exec runs a command line on a new command tree and returns its error. The
// output of the plugin goes to the stdout buffer and its errors to stderr.
func (r *runEnv) exec(args ...string) error {
	return r.execContext(context.Background(), args...)
}

func (r *runEnv) execContext(ctx context.Context, args ...string) error {
	r.stdout.Reset()
	r.stderr.Reset()

	r.logins = 0
	r.cli.renderer = &display.Renderer{MessageWriter: r.stderr, ResultWriter: r.stdout}

	root := buildRootCmd(r.cli)
	addPersistentFlags(root, r.cli)
	addSubCommands(root, r.cli)
	enforceUnknownSubcommand(root)
	root.SetOut(r.stdout)
	root.SetErr(r.stderr)
	root.SetArgs(args)

	return root.ExecuteContext(ctx)
}

// report returns what the fixture plugin printed in its report mode.
func (r *runEnv) report(t *testing.T) fixtureReportDoc {
	t.Helper()

	var report fixtureReportDoc

	require.NoError(t, json.Unmarshal(r.stdout.Bytes(), &report), r.stdout.String())

	return report
}

// started reports whether the fixture plugin ran in the last run.
func (r *runEnv) started() bool {
	return strings.Contains(r.stderr.String(), "fixture stderr") || strings.Contains(r.stdout.String(), `"args"`)
}

// reasonOf returns the class and the reason that the error has in the JSON error
// envelope.
func reasonOf(err error) (class, reason string) {
	return errorClass(err), errorReason(err)
}

// tamper appends to the executable of a plugin, so that it no longer matches the
// hash that was recorded when it was installed.
func tamper(t *testing.T, bin string) {
	t.Helper()

	file, err := os.OpenFile(bin, os.O_APPEND|os.O_WRONLY, 0)
	require.NoError(t, err)

	_, err = file.WriteString("tampered")
	require.NoError(t, err)
	require.NoError(t, file.Close())
}

// waitForMarker waits for the fixture plugin to write its process ID and returns
// it.
func waitForMarker(t *testing.T, path string) int {
	t.Helper()

	var pid int

	require.Eventually(t, func() bool {
		data, err := os.ReadFile(path)
		if err != nil {
			return false
		}

		pid, err = strconv.Atoi(string(data))

		return err == nil
	}, 30*time.Second, 20*time.Millisecond)

	return pid
}

func TestRegisterPluginCommands_Gate(t *testing.T) {
	e := newPluginEnv(t)
	e.publish(e.index(e.release("1.0.0")))
	e.install("1.0.0")

	tests := []struct {
		version string
		want    bool
	}{
		{"dev", true},
		{"1.39.0", false},
	}

	for _, test := range tests {
		t.Run(test.version, func(t *testing.T) {
			root := &cobra.Command{Use: "auth0"}
			registerPluginCommands(root, e.cli, test.version, nil)

			found, _, err := root.Find([]string{pluginTestBin})
			require.NoError(t, err)
			assert.Equal(t, test.want, found != root)
			assert.Equal(t, test.want, len(root.Commands()) == 1)
		})
	}
}

func TestRegisterPluginCommands_Shape(t *testing.T) {
	t.Run("a plugin with a root command and subcommands", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())

		root := &cobra.Command{Use: "auth0"}
		addPersistentFlags(root, r.cli)
		registerPluginCommands(root, r.cli, "dev", nil)

		top, _, err := root.Find([]string{pluginTestBin})
		require.NoError(t, err)

		assert.Equal(t, pluginTestBin, top.Name())
		assert.Equal(t, "Check a tenant", top.Short)
		assert.True(t, top.DisableFlagParsing)
		assert.True(t, top.Runnable())
		annotations := maps.Clone(top.Annotations)
		info := annotations[pluginInfoAnnotation]
		delete(annotations, pluginInfoAnnotation)

		assert.Equal(t, map[string]string{
			pluginCommandAnnotation: "true",
			pluginRootAnnotation:    "true",
			skipDocsAnnotation:      "true",
			skipAuthAnnotation:      "true",
		}, annotations)
		assert.JSONEq(t, `{"id":"auth0/checkmate","version":"1.0.0","scopes":["read:clients"],"output":{"json":true,"csv":true,"agentMode":true}}`, info)

		// The root command of the manifest is the top command and not a subcommand.
		var names []string
		for _, sub := range top.Commands() {
			names = append(names, sub.Name())

			assert.True(t, sub.DisableFlagParsing, sub.Name())
			assert.Equal(t, "true", sub.Annotations[skipAuthAnnotation], sub.Name())
			assert.Equal(t, "true", sub.Annotations[skipDocsAnnotation], sub.Name())
			assert.NotContains(t, sub.Annotations, pluginRootAnnotation, sub.Name())
		}

		assert.ElementsMatch(t, []string{"scan", "report"}, names)

		// The flags are there for help and completion.
		validators := top.Flags().Lookup("validators")
		require.NotNil(t, validators)
		assert.Equal(t, "v", validators.Shorthand)
		assert.Equal(t, "The validators to run", validators.Usage)

		scan, _, err := root.Find([]string{pluginTestBin, "scan"})
		require.NoError(t, err)
		assert.NotNil(t, scan.Flags().Lookup("depth"))
		assert.Nil(t, scan.Flags().Lookup("validators"))

		// Only the output flags that the command supports show in the help.
		assert.False(t, top.Flags().Lookup("json").Hidden)
		assert.False(t, top.Flags().Lookup("csv").Hidden)
		assert.True(t, scan.Flags().Lookup("json").Hidden)

		report, _, err := root.Find([]string{pluginTestBin, "report"})
		require.NoError(t, err)
		assert.False(t, report.Flags().Lookup("json").Hidden)
		assert.True(t, report.Flags().Lookup("csv").Hidden)
	})

	t.Run("a plugin without a root command is a namespace", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands()[1:])

		root := &cobra.Command{Use: "auth0"}
		addPersistentFlags(root, r.cli)
		registerPluginCommands(root, r.cli, "dev", nil)

		top, _, err := root.Find([]string{pluginTestBin})
		require.NoError(t, err)

		assert.Equal(t, pluginTestBin, top.Name())
		assert.Equal(t, "Checks the tenant", top.Short)
		assert.NotContains(t, top.Annotations, pluginRootAnnotation)
		assert.Equal(t, "true", top.Annotations[skipAuthAnnotation])
		assert.Len(t, top.Commands(), 2)
		assert.Nil(t, top.Flags().Lookup("validators"))
	})

	t.Run("a manifest cannot make the flags collide", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())

		root := &cobra.Command{Use: "auth0"}
		addPersistentFlags(root, r.cli)

		manifest := &plugins.Manifest{ID: pluginTestID, Version: "1.0.0", Description: "x", Commands: []plugins.Command{{
			Name: pluginTestBin,
			Flags: []plugins.Flag{
				{Name: "tenant", Type: plugins.FlagTypeString},
				{Name: "json", Type: plugins.FlagTypeBool},
				{Name: "mine", Shorthand: "h", Type: plugins.FlagTypeBool},
				{Name: "twice", Shorthand: "t", Type: plugins.FlagTypeBool},
				{Name: "again", Shorthand: "t", Type: plugins.FlagTypeBool},
				{Name: "twice", Type: plugins.FlagTypeBool},
				{Name: "count", Type: plugins.FlagTypeInt},
				{Name: "ratio", Type: plugins.FlagTypeFloat},
				{Name: "items", Type: plugins.FlagTypeStringArray},
			},
		}}}

		assert.NotPanics(t, func() { root.AddCommand(newPluginCommand(r.cli, root, pluginTestID, manifest)) })

		top, _, err := root.Find([]string{pluginTestBin})
		require.NoError(t, err)

		assert.Nil(t, top.Flags().Lookup("tenant"), "a persistent flag of the CLI is not shadowed")
		assert.Equal(t, "Output in json format.", top.Flags().Lookup("json").Usage)
		assert.Equal(t, "", top.Flags().Lookup("mine").Shorthand)
		assert.Equal(t, "t", top.Flags().Lookup("twice").Shorthand)
		assert.Equal(t, "", top.Flags().Lookup("again").Shorthand)
		assert.Equal(t, "int", top.Flags().Lookup("count").Value.Type())
		assert.Equal(t, "float64", top.Flags().Lookup("ratio").Value.Type())
		assert.Equal(t, "stringArray", top.Flags().Lookup("items").Value.Type())
	})
}

func TestRegisterPluginCommands_NeverBreaksTheCLI(t *testing.T) {
	r := newRunEnv(t, fixtureCommands())

	verified, err := r.svc.store.Verify(pluginTestID)
	require.NoError(t, err)

	tests := []struct {
		name   string
		damage func(t *testing.T)
	}{
		{"a lockfile that is not JSON", func(t *testing.T) {
			require.NoError(t, os.WriteFile(filepath.Join(r.Dir, "plugins.lock.json"), []byte("{not json"), 0o600))
		}},
		{"a lockfile that is a directory", func(t *testing.T) {
			path := filepath.Join(r.Dir, "plugins.lock.json")

			require.NoError(t, os.Remove(path))
			require.NoError(t, os.Mkdir(path, 0o700))
		}},
		{"a manifest that no longer matches the lockfile", func(t *testing.T) {
			require.NoError(t, os.WriteFile(verified.ManifestPath, []byte(`{"id":"auth0/checkmate"}`), 0o600))
		}},
		{"a manifest that is missing", func(t *testing.T) {
			require.NoError(t, os.Remove(verified.ManifestPath))
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// A fresh copy of the broken state each time is not needed: each case only
			// has to leave the others working, and they all break the same plugin.
			test.damage(t)

			root := &cobra.Command{Use: "auth0"}

			assert.NotPanics(t, func() { registerPluginCommands(root, r.cli, "dev", nil) })
			assert.Empty(t, root.Commands(), "nothing is registered for a plugin that cannot be read")

			// The rest of the CLI works and says nothing about the plugin.
			require.NoError(t, r.exec("apps", "--help"))
			assert.Empty(t, r.stderr.String())
			assert.NotContains(t, r.stdout.String(), pluginTestBin)
			assert.Zero(t, r.logins)

			require.NoError(t, r.exec("apps"))
			assert.Empty(t, r.stderr.String())
		})
	}
}

// A nil trailing command panics when the names of the core commands are
// collected, so a panic in this test shows that the registration did not return
// before that.
func TestRegisterPluginCommands_ReturnsEarlyWithoutPlugins(t *testing.T) {
	trailing := []*cobra.Command{nil}

	t.Run("nothing is installed", func(t *testing.T) {
		e := newPluginEnv(t)

		root := &cobra.Command{Use: "auth0"}

		assert.NotPanics(t, func() { registerPluginCommands(root, e.cli, "dev", trailing) })
		assert.Empty(t, root.Commands())
		assert.Empty(t, e.cli.shadowedPlugins)
		assert.NoDirExists(t, e.Dir, "nothing is created to look for plugins")
	})

	t.Run("no manifest can be read", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())

		verified, err := r.svc.store.Verify(pluginTestID)
		require.NoError(t, err)
		require.NoError(t, os.Remove(verified.ManifestPath))

		root := &cobra.Command{Use: "auth0"}

		assert.NotPanics(t, func() { registerPluginCommands(root, r.cli, "dev", trailing) })
		assert.Empty(t, root.Commands())
	})
}

// TestPluginCommand_Routing records how cobra finds the command of a plugin when
// the flags are not parsed. The routing is cobra's and depends on which flags it
// knows, so these are the behaviours that the help text warns about.
func TestPluginCommand_Routing(t *testing.T) {
	r := newRunEnv(t, fixtureCommands())

	root := &cobra.Command{Use: "auth0"}
	addPersistentFlags(root, r.cli)
	registerPluginCommands(root, r.cli, "dev", nil)

	tests := []struct {
		name string
		args []string
		want string
		rest []string
	}{
		{"the subcommand first", []string{"checkmate", "scan", "--x"}, "auth0 checkmate scan", []string{"--x"}},
		{"a flag of the root command with its value", []string{"checkmate", "--validators", "a"}, "auth0 checkmate", []string{"--validators", "a"}},
		{"a flag with its value before the subcommand", []string{"checkmate", "--validators", "a", "scan"}, "auth0 checkmate scan", []string{"--validators", "a"}},
		{"a flag with an equals sign before the subcommand", []string{"checkmate", "--validators=a", "scan"}, "auth0 checkmate scan", []string{"--validators=a"}},
		{"a bare double dash", []string{"checkmate", "--", "scan"}, "auth0 checkmate", []string{"--", "scan"}},
		{"an unknown flag swallows the next word", []string{"checkmate", "--x", "scan"}, "auth0 checkmate", []string{"--x", "scan"}},
		{"an unknown short flag swallows the next word", []string{"checkmate", "-z", "scan"}, "auth0 checkmate", []string{"-z", "scan"}},
		{"a flag of the CLI before the subcommand", []string{"checkmate", "--tenant", "t", "scan"}, "auth0 checkmate scan", []string{"--tenant", "t"}},
		{"a boolean flag of the CLI before the subcommand", []string{"checkmate", "--json", "scan"}, "auth0 checkmate scan", []string{"--json"}},
		{"an unknown word", []string{"checkmate", "bogus"}, "auth0 checkmate", []string{"bogus"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			found, rest, err := root.Find(test.args)

			require.NoError(t, err)
			assert.Equal(t, test.want, found.CommandPath())
			assert.Equal(t, test.rest, rest)
		})
	}

	t.Run("the help says that the subcommand comes first", func(t *testing.T) {
		top, _, err := root.Find([]string{"checkmate"})
		require.NoError(t, err)

		assert.Contains(t, top.Long, "Type the name of a subcommand first, directly after the name of the plugin")
	})
}

func TestPluginCommand_Help(t *testing.T) {
	r := newRunEnv(t, fixtureCommands())

	t.Run("the root command, without an approval or a login", func(t *testing.T) {
		require.NoError(t, r.exec("checkmate", "--help"))

		help := r.stdout.String()

		assert.Contains(t, help, "Check a tenant")
		assert.Contains(t, help, "Management API scopes: read:clients")
		assert.Contains(t, help, "Timeout: the default of the registry")
		assert.Contains(t, help, "Output: --json, --json-compact, --csv, agent mode")
		assert.Contains(t, help, "comes from the plugin auth0/checkmate 1.0.0")
		assert.Contains(t, help, "auth0 plugin approve checkmate")
		assert.Contains(t, help, "auth0 checkmate <command> [flags]")
		assert.Contains(t, help, "scan")
		assert.Contains(t, help, "Scan the users")
		assert.Contains(t, help, "--validators string")
		assert.Contains(t, help, "The validators to run")
		assert.Contains(t, help, "--json")
		assert.NotContains(t, help, "fixture")
		assert.Zero(t, r.logins)
		assert.False(t, r.started())
	})

	t.Run("a subcommand", func(t *testing.T) {
		for _, args := range [][]string{
			{"checkmate", "scan", "--help"},
			{"checkmate", "scan", "-h"},
			{"checkmate", "scan", "--depth", "3", "--help"},
		} {
			require.NoError(t, r.exec(args...), args)

			help := r.stdout.String()

			assert.Contains(t, help, "Scan the users", args)
			assert.Contains(t, help, "Management API scopes: read:users", args)
			assert.Contains(t, help, "Timeout: 30s", args)
			assert.Contains(t, help, "--depth int", args)
			assert.NotContains(t, help, "--json ", args, "the output flag is hidden when the command does not declare it")
			assert.Zero(t, r.logins)
			assert.False(t, r.started())
		}
	})

	t.Run("after a double dash the flag is the plugin's", func(t *testing.T) {
		r.approve()

		require.NoError(t, r.exec("checkmate", "--", "--help"))

		assert.Equal(t, []string{"--", "--help"}, r.report(t).Args)
	})

	t.Run("through the help command", func(t *testing.T) {
		require.NoError(t, r.exec("help", "checkmate"))

		assert.Contains(t, r.stdout.String(), "Check a tenant")
	})

	t.Run("a plugin without a root command", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands()[1:])

		for _, args := range [][]string{{"checkmate"}, {"checkmate", "--help"}, {"checkmate", "--debug"}} {
			require.NoError(t, r.exec(args...), args)

			help := r.stdout.String()

			assert.Contains(t, help, "This plugin has no command of its own", args)
			assert.Contains(t, help, "scan", args)
			assert.NotContains(t, help, "auth0 checkmate [flags]", args)
			assert.Zero(t, r.logins)
			assert.False(t, r.started())
		}

		err := r.exec("checkmate", "bogus")
		require.Error(t, err)
		assert.Equal(t, "usage", errorClass(err))
		assert.Contains(t, err.Error(), "bogus")

		err = r.exec("checkmate", "--bogus")
		require.Error(t, err)
		assert.Equal(t, "usage", errorClass(err))
		assert.Zero(t, r.logins)
	})
}

func TestPluginRun_Arguments(t *testing.T) {
	r := newRunEnv(t, fixtureCommands())
	r.approve()

	other := "other.us.auth0.com"

	tests := []struct {
		name string
		args []string
		want []string
		env  map[string]string
	}{
		{
			name: "no arguments",
			args: []string{"checkmate"},
			want: []string{},
			env: map[string]string{
				"AUTH0_PLUGIN_TENANT": pluginTestTenant, "AUTH0_PLUGIN_DEBUG": "false", "AUTH0_PLUGIN_NO_INPUT": "false",
				"AUTH0_PLUGIN_NO_COLOR": "false", "AUTH0_PLUGIN_AGENT_MODE": "false", "AUTH0_PLUGIN_JSON": "false",
				"AUTH0_PLUGIN_JSON_COMPACT": "false", "AUTH0_PLUGIN_CSV": "false",
			},
		},
		{
			name: "the flags of the plugin reach it untouched",
			args: []string{"checkmate", "--validators", "a", "-v=b", "--unknown", "--mode=report", "plain"},
			want: []string{"--validators", "a", "-v=b", "--unknown", "--mode=report", "plain"},
		},
		{
			name: "the flags of the CLI are taken out wherever they are",
			args: []string{"checkmate", "--debug", "--validators", "a", "--tenant", pluginTestTenant, "--json", "last"},
			want: []string{"--validators", "a", "last"},
			env:  map[string]string{"AUTH0_PLUGIN_DEBUG": "true", "AUTH0_PLUGIN_JSON": "true", "AUTH0_PLUGIN_TENANT": pluginTestTenant},
		},
		{
			name: "a flag of the CLI with an equals sign",
			args: []string{"checkmate", "--tenant=" + pluginTestTenant, "--no-input=true", "--no-color"},
			want: []string{},
			env:  map[string]string{"AUTH0_PLUGIN_NO_INPUT": "true", "AUTH0_PLUGIN_NO_COLOR": "true"},
		},
		{
			name: "everything after a double dash is the plugin's",
			args: []string{"checkmate", "--debug", "--", "--tenant", other, "--debug", "--json", "-h", "--agent-mode"},
			want: []string{"--", "--tenant", other, "--debug", "--json", "-h", "--agent-mode"},
			env:  map[string]string{"AUTH0_PLUGIN_DEBUG": "true", "AUTH0_PLUGIN_TENANT": pluginTestTenant, "AUTH0_PLUGIN_JSON": "false", "AUTH0_PLUGIN_AGENT_MODE": "false"},
		},
		{
			name: "the subcommand name goes first to the plugin",
			args: []string{"checkmate", "scan", "--depth", "3", "--debug", "-x"},
			want: []string{"scan", "--depth", "3", "-x"},
			env:  map[string]string{"AUTH0_PLUGIN_DEBUG": "true"},
		},
		{
			name: "a double dash after the subcommand",
			args: []string{"checkmate", "scan", "--", "--tenant", "x"},
			want: []string{"scan", "--", "--tenant", "x"},
			env:  map[string]string{"AUTH0_PLUGIN_TENANT": pluginTestTenant},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.NoError(t, r.exec(test.args...))

			report := r.report(t)

			assert.Equal(t, test.want, report.Args)

			for name, value := range test.env {
				assert.Equal(t, value, report.Env[name], name)
			}

			assert.Equal(t, 1, r.logins)
		})
	}

	t.Run("every flag of the CLI becomes a variable of the plugin", func(t *testing.T) {
		require.NoError(t, r.exec("checkmate", "--tenant", pluginTestTenant, "--debug", "--no-input", "--no-color", "--agent-mode", "--json", "--json-compact", "--csv"))

		assert.Equal(t, map[string]string{
			"AUTH0_PLUGIN_TENANT": pluginTestTenant, "AUTH0_PLUGIN_DEBUG": "true", "AUTH0_PLUGIN_NO_INPUT": "true",
			"AUTH0_PLUGIN_NO_COLOR": "true", "AUTH0_PLUGIN_AGENT_MODE": "true", "AUTH0_PLUGIN_JSON": "true",
			"AUTH0_PLUGIN_JSON_COMPACT": "true", "AUTH0_PLUGIN_CSV": "true",
		}, pluginVariables(r.report(t).Env, "AUTH0_PLUGIN_TENANT", "AUTH0_PLUGIN_DEBUG", "AUTH0_PLUGIN_NO_INPUT", "AUTH0_PLUGIN_NO_COLOR",
			"AUTH0_PLUGIN_AGENT_MODE", "AUTH0_PLUGIN_JSON", "AUTH0_PLUGIN_JSON_COMPACT", "AUTH0_PLUGIN_CSV"))
	})

	t.Run("agent mode switches the defaults", func(t *testing.T) {
		require.NoError(t, r.exec("checkmate", "--agent-mode"))

		env := r.report(t).Env

		assert.Equal(t, "true", env["AUTH0_PLUGIN_AGENT_MODE"])
		assert.Equal(t, "true", env["AUTH0_PLUGIN_NO_INPUT"])
		assert.Equal(t, "true", env["AUTH0_PLUGIN_NO_COLOR"])
		assert.Equal(t, "true", env["AUTH0_PLUGIN_JSON"])
	})

	t.Run("an owned flag without its value is a usage error", func(t *testing.T) {
		err := r.exec("checkmate", "--tenant")

		require.Error(t, err)
		assert.Equal(t, "usage", errorClass(err))
		assert.Zero(t, r.logins)
		assert.False(t, r.started())
	})
}

// pluginVariables returns the named variables of the environment of the plugin.
func pluginVariables(env map[string]string, names ...string) map[string]string {
	found := map[string]string{}

	for _, name := range names {
		if value, ok := env[name]; ok {
			found[name] = value
		}
	}

	return found
}

func TestPluginRun_PassesOutputThrough(t *testing.T) {
	r := newRunEnv(t, fixtureCommands())
	r.approve()

	require.NoError(t, r.exec("checkmate"))

	assert.Contains(t, r.stdout.String(), `"args"`)
	assert.Equal(t, "fixture stderr\n", r.stderr.String(), "the plugin's own stderr and nothing from the CLI")
}

func TestPluginRun_Environment(t *testing.T) {
	r := newRunEnv(t, fixtureCommands())
	r.approve()

	t.Setenv("AUTH0_CLIENT_SECRET", "client-secret-value")
	t.Setenv("AUTH0_API_TOKEN", "api-token-value")
	t.Setenv("AUTH0_CLIENT_ID", "client-id-value")
	t.Setenv("AUTH0_CLI_ANALYTICS", "false")
	t.Setenv("AWS_PROFILE", "production")
	t.Setenv("FIXTURE_UNDECLARED", "undeclared-value")
	t.Setenv(pluginFixtureEnv, "declared-value")
	t.Setenv("LANG", "en_US.UTF-8")

	allowed := func(name string) bool {
		// Windows names are not case sensitive, and a plugin gets the variables that
		// it needs to find its way around the system.
		if runtime.GOOS == "windows" {
			for _, system := range []string{"SystemRoot", "TEMP", "TMP", "USERPROFILE", "APPDATA", "LOCALAPPDATA", "PATHEXT", "ComSpec"} {
				if strings.EqualFold(name, system) {
					return true
				}
			}
		}

		switch name {
		case "PATH", "HOME", "LANG", "LC_ALL", "LC_CTYPE", "TERM", "TMPDIR",
			"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "no_proxy",
			"SSL_CERT_FILE", "NODE_EXTRA_CA_CERTS", pluginFixtureEnv:
			return true
		}

		return strings.HasPrefix(name, "AUTH0_PLUGIN_")
	}

	check := func(t *testing.T, env map[string]string) {
		t.Helper()

		var unexpected []string

		for name, value := range env {
			if !allowed(name) {
				unexpected = append(unexpected, name)
			}

			for _, secret := range []string{"client-secret-value", "api-token-value", "client-id-value", r.Token} {
				assert.NotContains(t, value, secret, "the value of %s", name)
			}
		}

		assert.Empty(t, unexpected, "variables that no rule lets through")

		assert.NotEmpty(t, env["AUTH0_PLUGIN_DOMAIN"])
		assert.NotEmpty(t, env["AUTH0_PLUGIN_MANAGEMENT_TOKEN"], "the secret of the proxy is there, the token is not")
		assert.NotEmpty(t, env["AUTH0_PLUGIN_CA_CERT"])
		assert.Equal(t, os.Getenv("HOME"), env["HOME"])
		assert.Equal(t, "en_US.UTF-8", env["LANG"])
	}

	t.Run("the root command", func(t *testing.T) {
		require.NoError(t, r.exec("checkmate"))

		env := r.report(t).Env

		check(t, env)
		assert.NotContains(t, env, pluginFixtureEnv, "a variable that another command declared")
	})

	t.Run("a command that declares a variable", func(t *testing.T) {
		require.NoError(t, r.exec("checkmate", "scan"))

		env := r.report(t).Env

		check(t, env)
		assert.Equal(t, "declared-value", env[pluginFixtureEnv])
	})

	t.Run("a declared variable that is not set", func(t *testing.T) {
		t.Setenv(pluginFixtureEnv, "")

		require.NoError(t, r.exec("checkmate", "scan"))

		assert.NotContains(t, r.report(t).Env, pluginFixtureEnv)
	})
}

func TestPluginRun_OnlyTheInvokedCommandScopes(t *testing.T) {
	r := newRunEnv(t, fixtureCommands())
	r.approve()

	tests := []struct {
		name string
		args []string
		path string
		want int
	}{
		{"the root command reads clients", []string{"checkmate", "--mode=call", "--path=/api/v2/clients"}, "/api/v2/clients", http.StatusOK},
		{"the root command cannot read users", []string{"checkmate", "--mode=call", "--path=/api/v2/users"}, "/api/v2/users", http.StatusForbidden},
		{"scan reads users", []string{"checkmate", "scan", "--mode=call", "--path=/api/v2/users"}, "/api/v2/users", http.StatusOK},
		{"scan cannot read clients", []string{"checkmate", "scan", "--mode=call", "--path=/api/v2/clients"}, "/api/v2/clients", http.StatusForbidden},
		{"report has no scopes", []string{"checkmate", "report", "--mode=call", "--path=/api/v2/clients"}, "/api/v2/clients", http.StatusForbidden},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			before := len(r.tenant.seen())

			require.NoError(t, r.exec(test.args...))

			assert.Contains(t, r.stdout.String(), "status="+strconv.Itoa(test.want))

			seen := r.tenant.seen()[before:]

			if test.want == http.StatusOK {
				require.Len(t, seen, 1)
				assert.Equal(t, test.path, seen[0].path)
				assert.Equal(t, "Bearer "+r.Token, seen[0].authorization, "the tenant gets the token of the session")
				assert.NotContains(t, r.stdout.String(), "shh", "the secret field is removed")
				assert.Contains(t, r.stdout.String(), `"name":"app"`)

				return
			}

			assert.Empty(t, seen, "a denied call never reaches the tenant")
			assert.Contains(t, r.stderr.String(), "API call was denied", "the summary of the denied calls")
		})
	}

	t.Run("the summary is not printed in agent mode", func(t *testing.T) {
		require.NoError(t, r.exec("checkmate", "--agent-mode", "--mode=call", "--path=/api/v2/users"))

		assert.Contains(t, r.stdout.String(), "status=403")
		assert.Empty(t, r.stderr.String())
	})
}

func TestPluginRun_Refusals(t *testing.T) {
	// A refusal never starts the plugin, so it never starts the proxy either.
	assertRefused := func(t *testing.T, r *runEnv, err error, class, reason string) {
		t.Helper()

		require.Error(t, err)

		gotClass, gotReason := reasonOf(err)

		assert.Equal(t, class, gotClass, err.Error())
		assert.Equal(t, reason, gotReason, err.Error())
		assert.False(t, r.started(), "the plugin must not have run")
		assert.Empty(t, r.tenant.seen())
	}

	t.Run("not approved", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())

		err := r.exec("checkmate")

		assertRefused(t, r, err, "validation", "plugin_not_approved")
		assert.Contains(t, err.Error(), "auth0 plugin approve auth0/checkmate --tenant "+pluginTestTenant)
		assert.Zero(t, r.logins, "an unapproved plugin never starts a login")

		err = r.exec("checkmate", "scan")
		assertRefused(t, r, err, "validation", "plugin_not_approved")
		assert.Zero(t, r.logins)
	})

	t.Run("approved for another tenant", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())

		require.NoError(t, r.svc.approvals.Approve(context.Background(), "other.us.auth0.com", pluginTestID, r.record.PermissionsHash))

		err := r.exec("checkmate")

		assertRefused(t, r, err, "validation", "plugin_not_approved")
		assert.Zero(t, r.logins)
	})

	t.Run("the permissions changed since the approval", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())

		require.NoError(t, r.svc.approvals.Approve(context.Background(), pluginTestTenant, pluginTestID, "sha256:"+strings.Repeat("a", 64)))

		err := r.exec("checkmate")

		assertRefused(t, r, err, "validation", "plugin_not_approved")
		assert.Contains(t, err.Error(), "changed what it asks for")
		assert.Zero(t, r.logins)
	})

	t.Run("a damaged approvals file", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		r.approve()

		require.NoError(t, os.WriteFile(filepath.Join(r.Dir, "approvals.json"), []byte("{broken"), 0o600))

		err := r.exec("checkmate")

		assertRefused(t, r, err, "validation", "plugin_approvals_damaged")
		assert.Zero(t, r.logins)
	})

	t.Run("no tenant", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		r.approve()
		r.cli.Config.DefaultTenant = ""

		// The config file would give the CLI a default tenant when it is read.
		require.NoError(t, os.Remove(filepath.Join(os.Getenv("HOME"), ".config", "auth0", "config.json")))

		err := r.exec("checkmate")

		assertRefused(t, r, err, "validation", "plugin_no_tenant")
		assert.Zero(t, r.logins)
	})

	t.Run("plugins are turned off", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		r.approve()

		t.Setenv(pluginsDisabledEnv, "true")

		err := r.exec("checkmate")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "turned off")
		assert.False(t, r.started())
		assert.Zero(t, r.logins)

		// Only running is refused. The help still reads.
		require.NoError(t, r.exec("checkmate", "--help"))
		assert.Contains(t, r.stdout.String(), "Check a tenant")

		t.Setenv(pluginsDisabledEnv, "perhaps")
		err = r.exec("checkmate")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not true or false")
		assert.False(t, r.started())
	})

	t.Run("a bad pin", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		r.approve()

		t.Setenv(approvals.PinsEnv, "not-a-pin")

		err := r.exec("checkmate")

		require.Error(t, err)
		assert.ErrorIs(t, err, approvals.ErrBadPins)
		assert.False(t, r.started())
		assert.Zero(t, r.logins)
	})

	t.Run("a binary that changed", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		r.approve()

		verified, err := r.svc.store.Verify(pluginTestID)
		require.NoError(t, err)

		tamper(t, verified.Bin)

		err = r.exec("checkmate")

		assertRefused(t, r, err, "validation", "plugin_verification_failed")
		assert.Zero(t, r.logins, "a plugin that failed verification never starts a login")

		// It still shows in the help, which runs nothing.
		require.NoError(t, r.exec("checkmate", "--help"))
	})

	t.Run("a binary that changed while the session was opened", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		r.approve()

		verified, err := r.svc.store.Verify(pluginTestID)
		require.NoError(t, err)

		login := r.svc.login
		r.svc.login = func(ctx context.Context, c *cli) error {
			if err := login(ctx, c); err != nil {
				return err
			}

			tamper(t, verified.Bin)

			return nil
		}

		err = r.exec("checkmate")

		assertRefused(t, r, err, "validation", "plugin_verification_failed")
		assert.Equal(t, 1, r.logins, "the files are checked again after the session is open")
	})

	t.Run("a tenant flag that conflicts with the environment", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		r.approve()

		t.Setenv(authModeEnvVar, authModeEnv)
		t.Setenv(envDomain, "fixed.us.auth0.com")
		t.Setenv(envAPIToken, "env-token")

		err := r.exec("checkmate", "--tenant", pluginTestTenant)

		assertRefused(t, r, err, "auth", "env_auth_tenant_conflict")
		assert.Zero(t, r.logins)
	})

	t.Run("revoked", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		r.approve()

		r.republish(r.fixtureIndex(r.record).Revoke(pluginTestID, "1.0.0", "it leaks tokens."))

		err := r.exec("checkmate")

		assertRefused(t, r, err, "validation", "plugin_revoked")
		assert.Equal(t, "checkmate 1.0.0 was revoked: it leaks tokens. Run: auth0 plugin update checkmate", err.Error())
		assert.Zero(t, r.logins, "a revoked plugin never starts a login")
	})

	t.Run("another version is revoked", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		r.approve()

		r.republish(r.fixtureIndex(r.record).Revoke(pluginTestID, "0.9.0", "old"))

		require.NoError(t, r.exec("checkmate"))
		assert.True(t, r.started())
	})

	t.Run("the tenant of the session is not the one that approved", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		r.approve()
		r.Domain = "other.us.auth0.com"

		err := r.exec("checkmate")

		assertRefused(t, r, err, "auth", "plugin_tenant_mismatch")
		assert.Equal(t, 1, r.logins)
		assert.Contains(t, err.Error(), "other.us.auth0.com")
		assert.Contains(t, err.Error(), "auth0 plugin approve auth0/checkmate --tenant other.us.auth0.com")
	})

	t.Run("the tenant is compared without regard to case", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		r.approve()
		r.Domain = strings.ToUpper(pluginTestTenant)
		saveTenant(t, config.Tenant{Domain: r.Domain})

		require.NoError(t, r.exec("checkmate"))
		assert.True(t, r.started())
	})

	t.Run("the login fails", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		r.approve()
		r.loginErr = authError{err: errors.New("session expired"), reason: "session_expired"}

		err := r.exec("checkmate")

		assertRefused(t, r, err, "auth", "session_expired")
		assert.Equal(t, 1, r.logins)
	})

	t.Run("a command the installed manifest no longer has", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		r.approve()

		top := newPluginCommand(r.cli, &cobra.Command{Use: "auth0"}, pluginTestID, &plugins.Manifest{ID: pluginTestID, Version: "1.0.0", Commands: fixtureCommands()})
		p := &pluginCommand{cli: r.cli, id: pluginTestID, version: "1.0.0"}

		err := p.run(top, plugins.Command{Name: "gone"}, []string{"gone"})

		assertRefused(t, r, err, "validation", "plugin_command_missing")
	})
}

func TestPluginRun_Pins(t *testing.T) {
	r := newRunEnv(t, fixtureCommands())

	t.Run("agent mode without an approval or a pin", func(t *testing.T) {
		err := r.exec("checkmate", "--agent-mode")

		require.Error(t, err)
		assert.Equal(t, "plugin_not_approved", errorReason(err))
		assert.False(t, r.started())
		assert.Zero(t, r.logins)
	})

	t.Run("agent mode with a pin", func(t *testing.T) {
		t.Setenv(approvals.PinsEnv, pluginTestID+"@"+r.record.PermissionsHash)

		require.NoError(t, r.exec("checkmate", "--agent-mode"))

		assert.Equal(t, "true", r.report(t).Env["AUTH0_PLUGIN_AGENT_MODE"])
		assert.Equal(t, 1, r.logins)
		assert.Empty(t, r.approvedHashes(pluginTestID), "a pin is never written down")
	})

	t.Run("a pin for other permissions", func(t *testing.T) {
		t.Setenv(approvals.PinsEnv, pluginTestID+"@sha256:"+strings.Repeat("b", 64))

		err := r.exec("checkmate", "--agent-mode")

		require.Error(t, err)
		assert.Equal(t, "plugin_not_approved", errorReason(err))
		assert.False(t, r.started())
	})
}

func TestPluginRun_Index(t *testing.T) {
	const week = 7 * 24 * time.Hour

	t.Run("a stale index that cannot be refreshed runs with a warning", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		r.approve()

		r.Now = r.Now.Add(week)
		r.server.Fail("/index.json", http.StatusServiceUnavailable)

		require.NoError(t, r.exec("checkmate"))

		assert.True(t, r.started())
		assert.Contains(t, r.stderr.String(), pluginStaleWarning)
	})

	t.Run("the warning is left out of machine output", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		r.approve()

		r.Now = r.Now.Add(week)
		r.server.Fail("/index.json", http.StatusServiceUnavailable)

		require.NoError(t, r.exec("checkmate", "--agent-mode"))

		assert.NotContains(t, r.stderr.String(), pluginStaleWarning)
	})

	t.Run("a stale index that can be refreshed is refreshed", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		r.approve()

		r.Now = r.Now.Add(week)

		before := r.server.Requests("/index.json")

		require.NoError(t, r.exec("checkmate"))

		assert.Greater(t, r.server.Requests("/index.json"), before)
		assert.NotContains(t, r.stderr.String(), pluginStaleWarning)
	})

	t.Run("a fresh index is not fetched again", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		r.approve()

		before := r.server.Requests("/index.json")

		require.NoError(t, r.exec("checkmate"))

		assert.Equal(t, before, r.server.Requests("/index.json"))
	})

	t.Run("an expired index", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands(), func(b *registrytest.IndexBuilder) {
			b.SetExpires(time.Date(2026, time.October, 1, 13, 0, 0, 0, time.UTC))
		})
		r.approve()

		r.Now = r.Now.Add(2 * time.Hour)
		r.server.Fail("/index.json", http.StatusServiceUnavailable)

		err := r.exec("checkmate")

		require.Error(t, err)
		assert.Equal(t, "plugin_registry_unavailable", errorReason(err))
		assert.ErrorIs(t, err, registry.ErrExpired)
		assert.False(t, r.started())
		assert.Zero(t, r.logins)
	})

	t.Run("an index that was not refreshed within the grace period", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands(), func(b *registrytest.IndexBuilder) {
			b.SetExpires(time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC))
		})
		r.approve()

		r.Now = r.Now.Add(40 * 24 * time.Hour)
		r.server.Fail("/index.json", http.StatusServiceUnavailable)

		err := r.exec("checkmate")

		require.Error(t, err)
		assert.Equal(t, "plugin_registry_unavailable", errorReason(err))
		assert.False(t, r.started())
		assert.Zero(t, r.logins)
	})
}

func TestPluginRun_InteractiveAndOutput(t *testing.T) {
	t.Run("an interactive command", func(t *testing.T) {
		commands := fixtureCommands()
		commands[0].Interactive = true

		r := newRunEnv(t, commands)
		r.approve()

		tests := []struct {
			name     string
			terminal bool
			args     []string
			refused  bool
		}{
			{"in agent mode at a terminal", true, []string{"checkmate", "--agent-mode"}, true},
			{"with --no-input at a terminal", true, []string{"checkmate", "--no-input"}, true},
			{"with --no-input=true at a terminal", true, []string{"checkmate", "--no-input=true"}, true},
			{"without a terminal", false, []string{"checkmate"}, true},
			{"at a terminal", true, []string{"checkmate"}, false},
			{"at a terminal with --no-input=false", true, []string{"checkmate", "--no-input=false"}, false},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				r.Terminal = test.terminal

				err := r.exec(test.args...)

				if !test.refused {
					require.NoError(t, err)
					assert.True(t, r.started())

					return
				}

				require.Error(t, err)

				class, reason := reasonOf(err)
				assert.Equal(t, "usage", class)
				assert.Equal(t, "plugin_interactive", reason)
				assert.Contains(t, err.Error(), "auth0 checkmate is interactive")
				assert.False(t, r.started())
				assert.Zero(t, r.logins, "an interactive command that cannot run never starts a login")
			})
		}

		t.Run("a command that is not interactive runs without a terminal", func(t *testing.T) {
			r.Terminal = false

			require.NoError(t, r.exec("checkmate", "scan"))
			assert.True(t, r.started())
		})
	})

	t.Run("the output flags need the declaration", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		r.approve()

		tests := []struct {
			args    []string
			refused string
		}{
			{[]string{"checkmate", "--json"}, ""},
			{[]string{"checkmate", "--json-compact"}, ""},
			{[]string{"checkmate", "--csv"}, ""},
			{[]string{"checkmate", "--agent-mode"}, ""},
			{[]string{"checkmate", "--json=false"}, ""},
			{[]string{"checkmate", "report", "--json"}, ""},
			{[]string{"checkmate", "report", "--json-compact"}, ""},
			{[]string{"checkmate", "report", "--csv"}, "output.csv"},
			{[]string{"checkmate", "report", "--agent-mode"}, "output.agentMode"},
			{[]string{"checkmate", "scan", "--json"}, "output.json"},
			{[]string{"checkmate", "scan", "--json-compact"}, "output.json"},
			{[]string{"checkmate", "scan", "--csv"}, "output.csv"},
			{[]string{"checkmate", "scan", "--agent-mode"}, "output.agentMode"},
			{[]string{"checkmate", "scan", "--agent-mode=false"}, ""},
			{[]string{"checkmate", "scan", "--json=false"}, ""},
			{[]string{"checkmate", "scan", "--", "--json"}, ""},
		}

		for _, test := range tests {
			t.Run(strings.Join(test.args, " "), func(t *testing.T) {
				err := r.exec(test.args...)

				if test.refused == "" {
					require.NoError(t, err)
					assert.True(t, r.started())

					return
				}

				require.Error(t, err)

				class, reason := reasonOf(err)
				assert.Equal(t, "usage", class)
				assert.Equal(t, "plugin_output_unsupported", reason)
				assert.Contains(t, err.Error(), "does not declare "+test.refused)
				assert.False(t, r.started())
				assert.Zero(t, r.logins)
			})
		}
	})
}

func TestPluginRun_Results(t *testing.T) {
	t.Run("a plugin that exits with a code", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		r.approve()

		err := r.exec("checkmate", "--mode=exit", "--code=7")

		require.Error(t, err)

		class, reason := reasonOf(err)
		assert.Equal(t, "plugin", class)
		assert.Equal(t, "plugin_exit", reason)
		assert.Equal(t, exitGeneric, exitCodeForError(err), "the process exits with the generic code")
		assert.Equal(t, "checkmate 1.0.0 exited with code 7", err.Error())

		envelope := buildErrorEnvelope(err)

		assert.Equal(t, "plugin", envelope.Error.Code)
		assert.Equal(t, "plugin_exit", envelope.Error.Reason)
		assert.JSONEq(t, `{"pluginId":"auth0/checkmate","version":"1.0.0","exitCode":7}`, string(envelope.Error.Details))
	})

	t.Run("a plugin that runs out of time", func(t *testing.T) {
		commands := fixtureCommands()
		commands[1].Timeout = "4s"

		r := newRunEnv(t, commands)
		r.approve()

		marker := filepath.Join(t.TempDir(), "pid")
		started := time.Now()

		err := r.exec("checkmate", "scan", "--mode=sleep", "--marker="+marker)

		require.Error(t, err)

		class, reason := reasonOf(err)
		assert.Equal(t, "plugin", class)
		assert.Equal(t, "plugin_timeout", reason)
		assert.Equal(t, "checkmate 1.0.0 did not finish within 4s and was stopped", err.Error())
		assert.Less(t, time.Since(started), 30*time.Second)
		assert.JSONEq(t, `{"pluginId":"auth0/checkmate","version":"1.0.0"}`, string(buildErrorEnvelope(err).Error.Details))

		assert.False(t, processAlive(waitForMarker(t, marker)), "the plugin must be gone")
	})

	t.Run("the timeout comes from the registry when the command has none", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands(), func(b *registrytest.IndexBuilder) {
			b.SetPolicy(registry.Policy{DefaultTimeout: "4s"})
		})
		r.approve()

		marker := filepath.Join(t.TempDir(), "pid")

		err := r.exec("checkmate", "--mode=sleep", "--marker="+marker)

		require.Error(t, err)
		assert.Equal(t, "plugin_timeout", errorReason(err))
		assert.Contains(t, err.Error(), "did not finish within 4s")
	})

	t.Run("the proxy is closed after the run", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		r.approve()

		require.NoError(t, r.exec("checkmate"))

		address := r.report(t).Env["AUTH0_PLUGIN_DOMAIN"]
		require.NotEmpty(t, address)

		_, err := (&http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}).Get("https://" + address + "/")
		assert.Error(t, err, "nothing listens once the plugin has ended")

		certificate := r.report(t).Env["AUTH0_PLUGIN_CA_CERT"]
		_, statErr := os.Stat(certificate)
		assert.True(t, os.IsNotExist(statErr), "the certificate of the run is removed")
	})
}

func TestPluginRun_Interrupt(t *testing.T) {
	r := newRunEnv(t, fixtureCommands())
	r.approve()

	guard := &interruptGuard{}
	ctx, cancel := context.WithCancel(withInterruptGuard(context.Background(), guard))

	defer cancel()

	marker := filepath.Join(t.TempDir(), "pid")
	done := make(chan error, 1)

	go func() { done <- r.execContext(ctx, "checkmate", "--mode=sleep", "--marker="+marker) }()

	pid := waitForMarker(t, marker)

	// The interrupt waits for the run, which stops the plugin before Execute exits.
	require.True(t, guard.interrupt())
	cancel()

	err := <-done

	require.Error(t, err)

	class, reason := reasonOf(err)
	assert.Equal(t, "plugin", class)
	assert.Equal(t, "plugin_interrupted", reason)
	assert.Equal(t, "checkmate 1.0.0 was interrupted", err.Error())
	assert.True(t, interrupted(ctx), "Execute exits with the code of an interrupt")
	assert.False(t, processAlive(pid), "the plugin must be gone")
}

func TestPluginRun_AmbiguousSubcommand(t *testing.T) {
	// The root command declares the boolean flag mode, so that cobra does not take
	// the word after it as its value.
	commands := fixtureCommands()
	commands[0].Flags = append(commands[0].Flags, plugins.Flag{Name: "mode", Type: plugins.FlagTypeBool, Description: "A switch"})

	r := newRunEnv(t, commands)
	r.approve()

	refused := []struct {
		name string
		args []string
	}{
		{"an unknown flag takes the subcommand as its value", []string{"checkmate", "--unknownbool", "scan"}},
		{"a boolean flag followed by words", []string{"checkmate", "--mode", "call", "scan"}},
		{"the word in the middle", []string{"checkmate", "--unknownbool", "scan", "--depth", "3"}},
		{"the word after a flag of the CLI", []string{"checkmate", "--debug", "--unknownbool", "scan"}},
	}

	for _, test := range refused {
		t.Run("refuses "+test.name, func(t *testing.T) {
			err := r.exec(test.args...)

			require.Error(t, err)

			class, reason := reasonOf(err)
			assert.Equal(t, "usage", class)
			assert.Equal(t, "plugin_ambiguous_subcommand", reason)
			assert.Contains(t, err.Error(), `"scan" is a command of auth0 checkmate`)
			assert.Contains(t, err.Error(), "auth0 checkmate scan ...")
			assert.Contains(t, err.Error(), "--flag=value")
			assert.Contains(t, err.Error(), "after --")
			assert.Zero(t, r.logins, "nothing starts before the call is refused")
			assert.False(t, r.started())
			assert.Empty(t, r.tenant.seen())
		})
	}

	allowed := []struct {
		name string
		args []string
		want []string
	}{
		{"the subcommand first", []string{"checkmate", "scan"}, []string{"scan"}},
		{"a flag with an equals sign before the subcommand", []string{"checkmate", "--validators=a", "scan"}, []string{"scan", "--validators=a"}},
		{"a flag and its value before the subcommand", []string{"checkmate", "--validators", "a", "scan"}, []string{"scan", "--validators", "a"}},
		{"a double dash before the word", []string{"checkmate", "--", "scan"}, []string{"--", "scan"}},
		{"the word again after the subcommand", []string{"checkmate", "scan", "--unknownbool", "scan"}, []string{"scan", "--unknownbool", "scan"}},
		{"a value that is not a subcommand", []string{"checkmate", "--unknownbool", "other"}, []string{"--unknownbool", "other"}},
	}

	for _, test := range allowed {
		t.Run("allows "+test.name, func(t *testing.T) {
			require.NoError(t, r.exec(test.args...))

			assert.Equal(t, test.want, r.report(t).Args)
			assert.Equal(t, 1, r.logins)
		})
	}

	t.Run("help is not refused", func(t *testing.T) {
		require.NoError(t, r.exec("checkmate", "--unknownbool", "scan", "--help"))

		assert.Zero(t, r.logins)
		assert.False(t, r.started())
	})

	t.Run("a plugin without subcommands has nothing to mix up", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands()[:1])
		r.approve()

		require.NoError(t, r.exec("checkmate", "--unknownbool", "scan"))

		assert.Equal(t, []string{"--unknownbool", "scan"}, r.report(t).Args)
		assert.Equal(t, 1, r.logins)
	})
}

func TestPluginRun_NoLoginBeforeApproval(t *testing.T) {
	// The login of the CLI is the real one, in the mode that needs neither the
	// keychain nor the network, so the root hook is the only thing that could start
	// it before the plugin was checked.
	gokeyring.MockInit()

	r := newRunEnv(t, fixtureCommands())
	r.svc.login = nil

	t.Setenv(authModeEnvVar, authModeEnv)
	t.Setenv(envDomain, pluginTestTenant)
	t.Setenv(envAPIToken, "offline-token")
	t.Setenv(envClientID, "")
	t.Setenv(envClientSecret, "")
	t.Setenv("BROWSER", "true")

	// Without the env mode a login could fall through to the saved one, which
	// opens a browser. Nothing below may run unless the mode is on.
	require.True(t, envAuthEnabled(os.Getenv), "the test must stay in the offline mode")

	noSession := func(t *testing.T) {
		t.Helper()

		assert.Empty(t, r.cli.accessToken, "no login may have started")
		assert.Empty(t, r.cli.sessionDomain, "no login may have started")
		assert.False(t, r.started())
	}

	reset := func() {
		r.cli.accessToken, r.cli.sessionDomain = "", ""
	}

	t.Run("not approved", func(t *testing.T) {
		err := r.exec("checkmate", "scan")

		require.Error(t, err)
		assert.Equal(t, "plugin_not_approved", errorReason(err))
		noSession(t)
	})

	t.Run("an approved run opens the session", func(t *testing.T) {
		reset()
		r.approve()

		require.NoError(t, r.exec("checkmate", "scan"))

		assert.Equal(t, "offline-token", r.cli.accessToken)
		assert.Equal(t, pluginTestTenant, r.cli.sessionDomain)
		assert.True(t, r.started())
	})

	t.Run("help", func(t *testing.T) {
		reset()

		require.NoError(t, r.exec("checkmate", "scan", "--help"))
		noSession(t)
	})
}

func TestPluginRun_ChangedDuringLogin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the store is copied with cp")
	}

	r := newRunEnv(t, fixtureCommands())
	r.approve()

	// While the person logs in, the plugin is replaced by an exact copy of the
	// same version in another place. The version and the permissions are the same,
	// but what would run is not what was checked.
	login := r.svc.login
	r.svc.login = func(ctx context.Context, c *cli) error {
		if err := login(ctx, c); err != nil {
			return err
		}

		moved := filepath.Join(t.TempDir(), "plugins")

		if output, err := exec.Command("cp", "-Rp", r.Dir, moved).CombinedOutput(); err != nil {
			return fmt.Errorf("%w: %s", err, output)
		}

		r.svc.store.Dir = moved

		return nil
	}

	err := r.exec("checkmate")

	require.Error(t, err)
	assert.Equal(t, "plugin_changed", errorReason(err))
	assert.Equal(t, 1, r.logins)
	assert.False(t, r.started())
}

func TestPluginEnvironment_KeepsTheCredentialsOfTheCLI(t *testing.T) {
	r := newRunEnv(t, fixtureCommands())

	names := []string{
		"AUTH0_CLIENT_SECRET", "AUTH0_API_TOKEN", "AUTH0_CLIENT_ID", "AUTH0_DOMAIN",
		"AUTH0_CLI_X", "auth0_client_secret",
	}

	// The manifest is built in memory, as the validator of the manifest would not
	// let it through.
	command := plugins.Command{Name: pluginTestBin, Permissions: fixturePermissions(nil, append([]string{"AWS_PROFILE"}, names...)...)}
	manifest := &plugins.Manifest{ID: pluginTestID, Version: "1.0.0", Commands: []plugins.Command{command}}

	root := &cobra.Command{Use: "auth0"}
	addPersistentFlags(root, r.cli)

	top := newPluginCommand(r.cli, root, pluginTestID, manifest)
	root.AddCommand(top)

	for _, name := range names {
		t.Setenv(name, "secret-of-"+name)
	}

	t.Setenv("AWS_PROFILE", "production")

	env := environment(top, command, pluginTestTenant, []string{"AUTH0_PLUGIN_DOMAIN=127.0.0.1:1"})

	assert.Contains(t, env, "AWS_PROFILE=production", "a variable that is not the CLI's still passes when declared")

	for _, entry := range env {
		name, value, _ := strings.Cut(entry, "=")

		assert.False(t, strings.HasPrefix(strings.ToUpper(name), "AUTH0_") && !strings.HasPrefix(name, "AUTH0_PLUGIN_"), "%s must not reach the plugin", name)
		assert.NotContains(t, value, "secret-of-", name)
	}
}

func TestPluginEnvironment_LoopbackIsNotProxied(t *testing.T) {
	r := newRunEnv(t, fixtureCommands())
	r.approve()

	t.Setenv("NO_PROXY", "internal.example.com")
	t.Setenv("HTTPS_PROXY", "http://proxy.example.com:3128")

	// On Windows this is the same variable as NO_PROXY, which would be cleared.
	if runtime.GOOS != "windows" {
		t.Setenv("no_proxy", "")
	}

	require.NoError(t, r.exec("checkmate"))

	env := r.report(t).Env

	// On Windows a name has one spelling in the environment of the plugin, and it is
	// not necessarily the one that was set.
	lookup := func(name string) string {
		for key, value := range env {
			if key == name || (runtime.GOOS == "windows" && strings.EqualFold(key, name)) {
				return value
			}
		}

		return ""
	}

	assert.Equal(t, "internal.example.com,127.0.0.1,localhost,::1", lookup("NO_PROXY"))
	assert.Equal(t, "http://proxy.example.com:3128", lookup("HTTPS_PROXY"), "the proxy of the user is kept")

	if runtime.GOOS != "windows" {
		assert.Equal(t, "127.0.0.1,localhost,::1", env["no_proxy"])
	}
}

func TestRenderNamespaceJSONHelp_LeavesPluginCommandsAlone(t *testing.T) {
	r := newRunEnv(t, fixtureCommands())

	root := &cobra.Command{Use: "auth0"}
	addPersistentFlags(root, r.cli)
	registerPluginCommands(root, r.cli, "dev", nil)

	// The plugin has subcommands, so it looks like a namespace. A JSON flag on it
	// is for the plugin, and must not turn the run into a help request.
	for _, args := range [][]string{
		{"checkmate", "--json"},
		{"checkmate", "--json-compact"},
		{"checkmate", "--validators", "a", "--json"},
	} {
		assert.False(t, renderNamespaceJSONHelp(root, args), args)
	}
}

func TestPluginHelp_UsesSafeText(t *testing.T) {
	manifest := &plugins.Manifest{ID: pluginTestID, Version: "1.0.0", Description: "bad\x1b[2Jtext", Commands: []plugins.Command{
		{Name: pluginTestBin, Description: "root\u202ecommand", Flags: []plugins.Flag{{Name: "f", Type: plugins.FlagTypeBool, Description: "flag\x07"}}},
	}}

	root := &cobra.Command{Use: "auth0"}
	top := newPluginCommand(&cli{}, root, pluginTestID, manifest)

	for _, text := range []string{top.Short, top.Long, top.Flags().Lookup("f").Usage} {
		for _, r := range text {
			assert.False(t, r < ' ' && r != '\n', "control character %q in %q", r, text)
			assert.NotEqual(t, '\u202e', r)
		}
	}
}

func TestCheckPluginNameAvailable_IgnoresPluginCommands(t *testing.T) {
	r := newRunEnv(t, fixtureCommands())

	root := &cobra.Command{Use: "auth0"}
	addPersistentFlags(root, r.cli)
	root.AddCommand(&cobra.Command{Use: "apps", Aliases: []string{"clients"}})
	registerPluginCommands(root, r.cli, "dev", nil)

	require.NoError(t, checkPluginNameAvailable(root, pluginTestBin), "an installed plugin can be installed again")

	err := checkPluginNameAvailable(root, "apps")
	require.Error(t, err)
	assert.Equal(t, "plugin_name_reserved", errorReason(err))

	require.Error(t, checkPluginNameAvailable(root, "clients"))
}

func TestPluginReinstall(t *testing.T) {
	r := newRunEnv(t, fixtureCommands())
	r.approve()

	// The command tree has the plugin when `plugin install` for it runs.
	r.Terminal = false

	require.NoError(t, r.exec("plugin", "install", "checkmate"))
	assert.Contains(t, r.stderr.String(), "checkmate")
}

func TestShadowedPlugin(t *testing.T) {
	e := newPluginEnv(t)

	apps := e.releaseAs("auth0/apps", e.platform, "1.0.0")
	clients := e.releaseAs("auth0/clients", e.platform, "1.0.0")
	fine := e.releaseAs("auth0/pluginless", e.platform, "1.0.0")

	e.publish(registrytest.NewIndexBuilder().
		AddPlugin(registrytest.NewPlugin("auth0/apps", apps)).
		AddPlugin(registrytest.NewPlugin("auth0/clients", clients)).
		AddPlugin(registrytest.NewPlugin("auth0/pluginless", fine)))

	e.installPlugin("auth0/apps", "1.0.0")
	e.installPlugin("auth0/clients", "1.0.0")
	e.installPlugin("auth0/pluginless", "1.0.0")

	t.Run("the core command is the one registered", func(t *testing.T) {
		root := buildRootCmd(e.cli)
		addPersistentFlags(root, e.cli)
		addSubCommands(root, e.cli)

		for _, name := range []string{"apps", "clients"} {
			found, _, err := root.Find([]string{name})
			require.NoError(t, err)

			assert.NotContains(t, found.Annotations, pluginCommandAnnotation, name)
			assert.Equal(t, "apps", found.Name(), name)
		}

		found, _, err := root.Find([]string{"pluginless"})
		require.NoError(t, err)
		assert.Equal(t, "true", found.Annotations[pluginCommandAnnotation])
	})

	t.Run("the warning names the plugin and how to remove it", func(t *testing.T) {
		require.NoError(t, e.run("apps"))

		assert.Contains(t, e.stderr.String(), `The installed plugin auth0/apps has the name of the core command "apps"`)
		assert.Contains(t, e.stderr.String(), "so the core command runs and the plugin is not available")
		assert.Contains(t, e.stderr.String(), "Remove the plugin with: auth0 plugin uninstall apps")
		assert.Equal(t, 1, strings.Count(e.stderr.String(), "The installed plugin"))
	})

	t.Run("an alias of a core command counts", func(t *testing.T) {
		require.NoError(t, e.run("clients"))

		assert.Contains(t, e.stderr.String(), `The installed plugin auth0/clients has the name of the core command "clients"`)
		assert.Contains(t, e.stderr.String(), "auth0 plugin uninstall clients")
		assert.NotContains(t, e.stderr.String(), "auth0/apps")
	})

	t.Run("no warning for another command", func(t *testing.T) {
		require.NoError(t, e.run("tenants"))

		assert.NotContains(t, e.stderr.String(), "The installed plugin")
	})

	t.Run("no warning in agent mode", func(t *testing.T) {
		require.NoError(t, e.run("--agent-mode", "apps"))

		assert.Empty(t, e.stderr.String())
	})
}

func TestPluginReservedFlagsMatchTheCLI(t *testing.T) {
	root := buildRootCmd(&cli{})
	addPersistentFlags(root, &cli{})

	// Every flag the CLI owns, as the pre-scan sees them, plus the help flag.
	names := []string{"help"}
	shorthands := []string{"h"}

	for _, flag := range ownedFlags(root) {
		names = append(names, flag.Name)

		if flag.Short != "" {
			shorthands = append(shorthands, flag.Short)
		}
	}

	root.InitDefaultHelpFlag()
	root.Flags().VisitAll(func(flag *pflag.Flag) { names = append(names, flag.Name) })
	root.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
		names = append(names, flag.Name)

		if flag.Shorthand != "" {
			shorthands = append(shorthands, flag.Shorthand)
		}
	})

	for _, name := range pluginOutputFlags {
		names = append(names, name)
	}

	manifest := func(flag string) []byte {
		return fmt.Appendf(nil, `{"id":"auth0/checkmate","version":"1.0.0","apiVersion":1,"description":"x",
			"owner":{"team":"t","bugs":"https://example.com"},"platforms":["darwin-arm64"],
			"commands":[{"name":"checkmate","description":"x","interactive":false,"flags":[%s]}]}`, flag)
	}

	slices.Sort(names)
	names = slices.Compact(names)

	for _, name := range names {
		t.Run("the flag "+name, func(t *testing.T) {
			_, err := plugins.ParseManifest(manifest(fmt.Sprintf(`{"name":%q,"type":"string","description":"x"}`, name)))

			require.Error(t, err, "a plugin must not be able to declare the flag %s, which the CLI owns", name)
			assert.Contains(t, err.Error(), "collides with a flag owned by the CLI")
		})
	}

	slices.Sort(shorthands)
	shorthands = slices.Compact(shorthands)

	for _, shorthand := range shorthands {
		t.Run("the shorthand "+shorthand, func(t *testing.T) {
			_, err := plugins.ParseManifest(manifest(fmt.Sprintf(`{"name":"mine","shorthand":%q,"type":"string","description":"x"}`, shorthand)))

			require.Error(t, err, "a plugin must not be able to declare the shorthand %s, which the CLI owns", shorthand)
			assert.Contains(t, err.Error(), "collides with a shorthand owned by the CLI")
		})
	}

	t.Run("a flag of the plugin is fine", func(t *testing.T) {
		_, err := plugins.ParseManifest(manifest(`{"name":"mine","shorthand":"m","type":"string","description":"x"}`))

		require.NoError(t, err)
	})
}

func TestOwnedFlags(t *testing.T) {
	root := buildRootCmd(&cli{})
	addPersistentFlags(root, &cli{})

	owned := map[string]bool{}

	for _, flag := range ownedFlags(root) {
		owned[flag.Name] = flag.Bool
	}

	assert.Equal(t, map[string]bool{
		"tenant": false, "debug": true, "no-input": true, "no-color": true, "agent-mode": true,
		"json": true, "json-compact": true, "csv": true,
	}, owned)
}

func TestPluginError(t *testing.T) {
	err := pluginError{err: errors.New("it failed"), reason: "plugin_exit", pluginID: pluginTestID, version: "1.0.0", exitCode: 3}

	assert.Equal(t, "it failed", err.Error())
	assert.Equal(t, "plugin", errorClass(err))
	assert.Equal(t, "plugin_exit", errorReason(err))
	assert.Equal(t, exitGeneric, exitCodeForError(err))
	assert.JSONEq(t, `{"pluginId":"auth0/checkmate","version":"1.0.0","exitCode":3}`, string(err.ErrorDetails()))

	wrapped := fmt.Errorf("wrap: %w", err)

	assert.Equal(t, "plugin", errorClass(wrapped))
	assert.Equal(t, "plugin_exit", errorReason(wrapped))

	timeout := pluginError{err: errors.New("slow"), reason: "plugin_timeout", pluginID: pluginTestID, version: "1.0.0"}

	assert.JSONEq(t, `{"pluginId":"auth0/checkmate","version":"1.0.0"}`, string(timeout.ErrorDetails()), "no exit code when the plugin did not exit")
}

func TestPluginFailure(t *testing.T) {
	p := &pluginCommand{id: pluginTestID, version: "1.0.0"}

	tests := []struct {
		name   string
		err    error
		reason string
		code   int
	}{
		{"an exit", &runner.ExitError{Code: 9}, "plugin_exit", 9},
		{"a timeout", &runner.TimeoutError{After: time.Second}, "plugin_timeout", 0},
		{"an interruption", &runner.InterruptedError{}, "plugin_interrupted", 0},
		{"a plugin that did not start", errors.New("no such file"), "plugin_start_failed", 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := p.failure(test.err, "")

			var failed pluginError

			require.ErrorAs(t, err, &failed)
			assert.Equal(t, test.reason, failed.reason)
			assert.Equal(t, test.code, failed.exitCode)
			assert.Equal(t, pluginTestID, failed.pluginID)
			assert.Equal(t, "1.0.0", failed.version)
		})
	}
}

// saveTenant writes a tenant to the config file in the home directory of the test
// through a config of its own, because a config remembers that its file was
// missing when it first looked.
func saveTenant(t *testing.T, tenant config.Tenant) {
	t.Helper()

	var saved config.Config

	require.NoError(t, saved.AddTenant(tenant))
}

func TestPluginSession(t *testing.T) {
	setup := func(t *testing.T) *cli {
		t.Helper()

		gokeyring.MockInit()
		t.Setenv("HOME", t.TempDir())

		for _, name := range []string{authModeEnvVar, envDomain, envAPIToken, envClientID, envClientSecret} {
			t.Setenv(name, "")
		}

		return &cli{renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: io.Discard}}
	}

	t.Run("there is no session", func(t *testing.T) {
		c := setup(t)

		_, err := c.newPluginSession()

		require.Error(t, err)
		assert.Equal(t, "plugin_session_unavailable", errorReason(err))
	})

	t.Run("a token from the environment cannot be renewed", func(t *testing.T) {
		c := setup(t)
		c.accessToken, c.sessionDomain = "env-token", pluginTestTenant

		t.Setenv(authModeEnvVar, authModeEnv)
		t.Setenv(envAPIToken, "env-token")

		session, err := c.newPluginSession()
		require.NoError(t, err)

		token, err := session.Token(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "env-token", token)

		_, err = session.Refresh(context.Background())
		assert.ErrorIs(t, err, proxy.ErrReloginRequired)
	})

	t.Run("client credentials from the environment can be renewed", func(t *testing.T) {
		c := setup(t)
		c.accessToken, c.sessionDomain = "first", pluginTestTenant

		t.Setenv(authModeEnvVar, authModeEnv)
		t.Setenv(envClientID, "id")
		t.Setenv(envClientSecret, "secret")

		session, err := c.newPluginSession()
		require.NoError(t, err)

		require.NotNil(t, session.renew)

		session.renew = func(context.Context) (string, error) { return "second", nil }

		token, err := session.Refresh(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "second", token)

		token, err = session.Token(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "second", token)
	})

	t.Run("a user who logged in with a device code cannot be renewed", func(t *testing.T) {
		c := setup(t)
		c.accessToken, c.sessionDomain = "user-token", pluginTestTenant
		saveTenant(t, config.Tenant{Domain: pluginTestTenant})

		session, err := c.newPluginSession()
		require.NoError(t, err)

		token, err := session.Token(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "user-token", token)

		_, err = session.Refresh(context.Background())
		assert.ErrorIs(t, err, proxy.ErrReloginRequired)
	})

	t.Run("saved client credentials can be renewed and the new token is saved", func(t *testing.T) {
		c := setup(t)
		c.accessToken, c.sessionDomain = "old-token", pluginTestTenant
		saveTenant(t, config.Tenant{Domain: pluginTestTenant, ClientID: "client"})

		session, err := c.newPluginSession()
		require.NoError(t, err)
		require.NotNil(t, session.renew)

		tenant, err := c.Config.GetTenant(pluginTestTenant)
		require.NoError(t, err)

		session.renew = c.savedCredentialsRenewal(tenant, func(_ context.Context, tenant *config.Tenant) error {
			tenant.ExpiresAt = time.Now().Add(time.Hour)

			return keyring.StoreAccessToken(tenant.Domain, "new-token")
		})

		token, err := session.Refresh(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "new-token", token)

		token, err = session.Token(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "new-token", token)

		saved, err := c.Config.GetTenant(pluginTestTenant)
		require.NoError(t, err)
		assert.True(t, saved.ExpiresAt.After(time.Now()), "the expiry of the new token is saved")
	})

	t.Run("a renewal that fails is an auth error and keeps the old token", func(t *testing.T) {
		c := setup(t)
		c.accessToken, c.sessionDomain = "old-token", pluginTestTenant
		saveTenant(t, config.Tenant{Domain: pluginTestTenant, ClientID: "client"})

		session, err := c.newPluginSession()
		require.NoError(t, err)

		tenant, err := c.Config.GetTenant(pluginTestTenant)
		require.NoError(t, err)

		session.renew = c.savedCredentialsRenewal(tenant, func(context.Context, *config.Tenant) error { return errors.New("rotated") })

		_, err = session.Refresh(context.Background())

		require.Error(t, err)
		assert.Equal(t, "credentials_refresh_failed", errorReason(err))
		assert.NotContains(t, err.Error(), "old-token")

		token, err := session.Token(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "old-token", token)
	})

	t.Run("a session of a tenant that is not saved", func(t *testing.T) {
		c := setup(t)
		c.accessToken, c.sessionDomain = "token", pluginTestTenant

		_, err := c.newPluginSession()

		require.Error(t, err)
		assert.Equal(t, "plugin_session_unavailable", errorReason(err))
	})

	t.Run("the context is honoured", func(t *testing.T) {
		c := setup(t)
		c.accessToken, c.sessionDomain = "token", pluginTestTenant

		t.Setenv(authModeEnvVar, authModeEnv)
		t.Setenv(envClientID, "id")
		t.Setenv(envClientSecret, "secret")

		session, err := c.newPluginSession()
		require.NoError(t, err)

		called := false
		session.renew = func(context.Context) (string, error) { called = true; return "new", nil }

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err = session.Token(ctx)
		assert.ErrorIs(t, err, context.Canceled)

		_, err = session.Refresh(ctx)
		assert.ErrorIs(t, err, context.Canceled)
		assert.False(t, called, "a renewal is not started for a caller that is gone")
	})

	t.Run("one renewal at a time, and a waiting caller can give up", func(t *testing.T) {
		c := setup(t)
		c.accessToken, c.sessionDomain = "token", pluginTestTenant

		t.Setenv(authModeEnvVar, authModeEnv)
		t.Setenv(envClientID, "id")
		t.Setenv(envClientSecret, "secret")

		session, err := c.newPluginSession()
		require.NoError(t, err)

		started := make(chan struct{})
		release := make(chan struct{})

		session.renew = func(context.Context) (string, error) {
			close(started)
			<-release

			return "new", nil
		}

		first := make(chan error, 1)

		go func() {
			_, err := session.Refresh(context.Background())
			first <- err
		}()

		<-started

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err = session.Refresh(ctx)
		assert.ErrorIs(t, err, context.DeadlineExceeded)

		close(release)
		require.NoError(t, <-first)
	})
}

func TestInterruptGuard(t *testing.T) {
	t.Run("without a holder the interrupt ends the process", func(t *testing.T) {
		guard := &interruptGuard{}

		assert.False(t, guard.interrupt())
		assert.False(t, guard.wasInterrupted())
	})

	t.Run("a holder takes the interrupt", func(t *testing.T) {
		guard := &interruptGuard{}
		ctx := withInterruptGuard(context.Background(), guard)

		release := holdInterrupt(ctx)

		assert.True(t, guard.interrupt())
		assert.True(t, guard.wasInterrupted())
		assert.True(t, interrupted(ctx))

		release()
		release()

		assert.False(t, guard.interrupt(), "nothing is held any more")
	})

	t.Run("a context without a guard has nothing to hold", func(t *testing.T) {
		release := holdInterrupt(context.Background())

		assert.NotPanics(t, release)
		assert.False(t, interrupted(context.Background()))
	})
}

// tree returns the command tree the way Execute builds it, so that the help and
// the completion are those of the real CLI.
func (r *runEnv) tree() *cobra.Command {
	return newRootTree(r.cli)
}

// newRootTree returns the command tree the way Execute builds it.
func newRootTree(c *cli) *cobra.Command {
	root := buildRootCmd(c)
	root.SetUsageTemplate(namespaceUsageTemplate())

	addPersistentFlags(root, c)
	addSubCommands(root, c)
	enforceUnknownSubcommand(root)
	overrideHelpAndVersionFlagText(root)

	return root
}

func TestRegisterPluginCommands_HelpGroups(t *testing.T) {
	t.Setenv(agentModeEnvVar, "false")

	// The part of the usage of the root that lists the commands.
	section := func(usage, heading string) string {
		start := strings.Index(usage, heading)
		require.GreaterOrEqual(t, start, 0, "no %q in\n%s", heading, usage)

		rest := usage[start+len(heading):]
		end := strings.Index(rest, "\n\n")
		require.GreaterOrEqual(t, end, 0)

		return rest[:end]
	}

	t.Run("without a plugin the help is the one it was", func(t *testing.T) {
		e := newPluginEnv(t)
		root := newRootTree(e.cli)

		assert.Empty(t, root.Groups())
		assert.Equal(t, namespaceUsageTemplate(), root.UsageTemplate())

		for _, command := range root.Commands() {
			assert.Empty(t, command.GroupID, command.Name())
		}

		usage := root.UsageString()

		assert.Contains(t, usage, "Available Resources:")
		assert.NotContains(t, usage, "Plugin Commands:")
	})

	t.Run("a build without the plugin feature has no heading either", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())

		original := buildinfo.Version
		buildinfo.Version = "1.39.0"

		t.Cleanup(func() { buildinfo.Version = original })

		root := r.tree()

		assert.Empty(t, root.Groups())
		assert.NotContains(t, root.UsageString(), "Plugin Commands:")
		assert.NotContains(t, root.UsageString(), "checkmate")
	})

	t.Run("a plugin has its own heading after the core commands", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		root := r.tree()

		require.Len(t, root.Groups(), 2)
		assert.Equal(t, coreGroupID, root.Groups()[0].ID)
		assert.Equal(t, pluginGroupID, root.Groups()[1].ID)

		usage := root.UsageString()

		require.Less(t, strings.Index(usage, "Available Resources:"), strings.Index(usage, "Plugin Commands:"))

		core := section(usage, "Available Resources:")
		pluginSection := section(usage, "Plugin Commands:")

		for _, name := range []string{"login", "apps", "plugin", "docs", "commands", "completion"} {
			assert.Contains(t, core, name)
			assert.NotContains(t, pluginSection, name+" ")
		}

		assert.NotContains(t, core, "checkmate")
		assert.Regexp(t, `^\s+checkmate\s+Check a tenant\s*$`, pluginSection)

		for _, command := range root.Commands() {
			want := coreGroupID
			if command.Name() == pluginTestBin {
				want = pluginGroupID
			}

			assert.Equal(t, want, command.GroupID, command.Name())
		}

		// The help renders for a person the same way.
		var help bytes.Buffer

		root.SetOut(&help)
		require.NoError(t, root.Help())
		assert.Contains(t, help.String(), "Plugin Commands:")
	})

	t.Run("a plugin that a core command shadows is not listed", func(t *testing.T) {
		e := newPluginEnv(t)

		apps := e.releaseAs("auth0/apps", e.platform, "1.0.0")
		fine := e.releaseAs("auth0/pluginless", e.platform, "1.0.0")

		e.publish(registrytest.NewIndexBuilder().
			AddPlugin(registrytest.NewPlugin("auth0/apps", apps)).
			AddPlugin(registrytest.NewPlugin("auth0/pluginless", fine)))

		e.installPlugin("auth0/apps", "1.0.0")

		// Alone, the shadowed plugin adds no heading.
		assert.Empty(t, newRootTree(e.cli).Groups())

		e.installPlugin("auth0/pluginless", "1.0.0")

		usage := newRootTree(e.cli).UsageString()

		assert.Regexp(t, `^\s+pluginless\s`, section(usage, "Plugin Commands:"))
		assert.NotContains(t, strings.TrimSpace(section(usage, "Plugin Commands:")), "\n", "one plugin is listed")
		assert.Contains(t, section(usage, "Available Resources:"), "apps")
	})
}

func TestPluginCommand_Completion(t *testing.T) {
	t.Setenv(agentModeEnvVar, "false")

	r := newRunEnv(t, fixtureCommands())

	// Complete runs the hidden command of Cobra and returns the lines it printed
	// before the directive.
	complete := func(args ...string) []string {
		t.Helper()

		require.NoError(t, r.exec(append([]string{cobra.ShellCompRequestCmd}, args...)...))

		var lines []string

		for _, line := range strings.Split(strings.TrimSpace(r.stdout.String()), "\n") {
			if strings.HasPrefix(line, ":") || strings.HasPrefix(line, "Completion ended") {
				continue
			}

			lines = append(lines, line)
		}

		assert.False(t, r.started(), "completing never runs the plugin")
		assert.Zero(t, r.logins)

		return lines
	}

	t.Run("the name of the plugin with its description", func(t *testing.T) {
		assert.Contains(t, complete(""), "checkmate\tCheck a tenant")
		assert.Equal(t, []string{"checkmate\tCheck a tenant"}, complete("check"))
	})

	t.Run("the subcommands", func(t *testing.T) {
		lines := complete("checkmate", "")

		assert.Contains(t, lines, "scan\tScan the users")
		assert.Contains(t, lines, "report\tWrite a report")

		for _, line := range lines {
			assert.False(t, strings.HasPrefix(line, "--validators") || strings.HasPrefix(line, "--json") || strings.HasPrefix(line, "--csv"), "the flags of the plugin wait for a dash: %s", line)
		}

		for _, line := range complete("checkmate", "scan", "") {
			assert.False(t, strings.HasPrefix(line, "--depth"), "the flags of the plugin wait for a dash: %s", line)
		}
	})

	t.Run("the flags of the manifest", func(t *testing.T) {
		assert.Contains(t, complete("checkmate", "scan", "--"), "--depth\tHow deep to scan")
		assert.Contains(t, complete("checkmate", "scan", "--de"), "--depth\tHow deep to scan")
		assert.NotContains(t, complete("checkmate", "scan", "--va"), "--validators\tThe validators to run", "the flags of another command")

		lines := complete("checkmate", "--")

		assert.Contains(t, lines, "--validators\tThe validators to run")
		assert.Contains(t, lines, "--json\tOutput in json format.")
		assert.Contains(t, lines, "--csv\tOutput in csv format.")
		assert.NotContains(t, lines, "--depth\tHow deep to scan")
	})

	t.Run("an output flag that the command does not support", func(t *testing.T) {
		for _, line := range complete("checkmate", "scan", "--") {
			assert.False(t, strings.HasPrefix(line, "--json"), line)
			assert.False(t, strings.HasPrefix(line, "--csv"), line)
		}

		assert.Contains(t, complete("checkmate", "report", "--"), "--json\tOutput in json format.")
	})

	t.Run("each flag once, whatever has been typed", func(t *testing.T) {
		tree := r.tree()

		for _, path := range [][]string{{"checkmate"}, {"checkmate", "scan"}, {"checkmate", "report"}} {
			command, _, err := tree.Find(path)
			require.NoError(t, err)

			// The flags of the CLI and of the command that are not hidden.
			var names []string

			for _, flags := range []*pflag.FlagSet{command.InheritedFlags(), command.LocalFlags()} {
				flags.VisitAll(func(flag *pflag.Flag) {
					if !flag.Hidden && flag.Name != "help" {
						names = append(names, "--"+flag.Name)
					}
				})
			}

			for _, typed := range []string{"-", "--", "--d", "--de", "--dep", "--depth", "--j", "--js", "--json", "--json-c", "--v", "--n", "--no", "--c", "--cs", "--t", "--a", "--z"} {
				var want []string

				for _, name := range names {
					if strings.HasPrefix(name, typed) {
						want = append(want, name)
					}
				}

				var got []string

				for _, line := range complete(append(path, typed)...) {
					got = append(got, strings.SplitN(line, "\t", 2)[0])
				}

				assert.ElementsMatch(t, want, got, "%v %s", path, typed)
			}
		}
	})

	t.Run("no value and no completion after a flag is written", func(t *testing.T) {
		assert.Empty(t, complete("checkmate", "scan", "--depth="))
		assert.Empty(t, complete("checkmate", "scan", "x"))
	})
}

func TestCompletePluginFlags_NotTheHelpFlag(t *testing.T) {
	t.Setenv(agentModeEnvVar, "false")

	r := newRunEnv(t, fixtureCommands())
	tree := r.tree()

	scan, _, err := tree.Find([]string{"checkmate", "scan"})
	require.NoError(t, err)

	// A command that has run before has the help flag, and a flag of the CLI that
	// is marked the way the CLI marks them makes cobra leave the flags of the plugin
	// out.
	scan.InitDefaultHelpFlag()
	require.NoError(t, tree.PersistentFlags().SetAnnotation("debug", cobra.BashCompOneRequiredFlag, []string{"false"}))

	completions, directive := completePluginFlags(scan, nil, "--")

	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
	assert.Contains(t, completions, "--depth\tHow deep to scan")

	for _, completion := range completions {
		assert.False(t, strings.HasPrefix(completion, "--help"), completion)
	}
}

func TestCommands_PluginSource(t *testing.T) {
	t.Setenv(agentModeEnvVar, "false")

	commands := fixtureCommands()
	commands[2].Interactive = true

	r := newRunEnv(t, commands)
	root := r.tree()

	var find func(nodes []commandNode, path string) (commandNode, bool)

	find = func(nodes []commandNode, path string) (commandNode, bool) {
		for _, node := range nodes {
			if node.Path == path {
				return node, true
			}

			if found, ok := find(node.Subcommands, path); ok {
				return found, true
			}
		}

		return commandNode{}, false
	}

	// Every node of the plugin, whatever the form of the list, says where it comes
	// from, and no other node does.
	checkSource := func(t *testing.T, nodes []commandNode) {
		t.Helper()

		seen := 0

		var walk func(nodes []commandNode)

		walk = func(nodes []commandNode) {
			for _, node := range nodes {
				if node.Path == "auth0 checkmate" || strings.HasPrefix(node.Path, "auth0 checkmate ") {
					seen++

					assert.Equal(t, "plugin", node.Source, node.Path)
					assert.Equal(t, "auth0/checkmate", node.PluginID, node.Path)
					assert.Equal(t, "1.0.0", node.PluginVersion, node.Path)
				} else {
					assert.Empty(t, node.Source, node.Path)
					assert.Empty(t, node.PluginID, node.Path)
					assert.Empty(t, node.PluginVersion, node.Path)
					assert.Empty(t, node.RequiredScopes, node.Path)
					assert.False(t, node.Interactive, node.Path)
					assert.Nil(t, node.Output, node.Path)
				}

				walk(node.Subcommands)
			}
		}

		walk(nodes)

		assert.Positive(t, seen)
	}

	t.Run("the tree", func(t *testing.T) {
		for _, detailed := range []bool{false, true} {
			tree := buildCommandTree(root, 0, detailed)

			checkSource(t, tree)

			top, ok := find(tree, "auth0 checkmate")
			require.True(t, ok)
			assert.Equal(t, []string{"read:clients"}, top.RequiredScopes)
			assert.Equal(t, &commandOutput{JSON: true, CSV: true, AgentMode: true}, top.Output)
			assert.True(t, top.RequiresAuth)

			scan, ok := find(tree, "auth0 checkmate scan")
			require.True(t, ok)
			assert.Equal(t, []string{"read:users"}, scan.RequiredScopes)
			assert.Equal(t, &commandOutput{}, scan.Output, "a command without output declares none")
			assert.False(t, scan.Interactive)
			assert.True(t, scan.RequiresAuth)

			report, ok := find(tree, "auth0 checkmate report")
			require.True(t, ok)
			assert.Empty(t, report.RequiredScopes)
			assert.True(t, report.Interactive)
			assert.Equal(t, &commandOutput{JSON: true}, report.Output)
		}
	})

	t.Run("the flat list", func(t *testing.T) {
		for _, detailed := range []bool{false, true} {
			nodes := flattenCommands(root, false, detailed)

			checkSource(t, nodes)

			scan, ok := find(nodes, "auth0 checkmate scan")
			require.True(t, ok)
			assert.Equal(t, []string{"read:users"}, scan.RequiredScopes)
		}
	})

	t.Run("one command", func(t *testing.T) {
		scan, _, err := root.Find([]string{"checkmate", "scan"})
		require.NoError(t, err)

		checkSource(t, []commandNode{buildNode(scan, 1, 1, true)})

		nodes := flattenCommands(scan, true, true)
		require.Len(t, nodes, 1)
		assert.Equal(t, "plugin", nodes[0].Source)
	})

	t.Run("the command prints it", func(t *testing.T) {
		var nodes []commandNode

		out := captureOutput(t, func() { require.NoError(t, r.exec("commands", "checkmate", "scan", "--json", "--detailed")) })
		require.NoError(t, json.Unmarshal([]byte(out), &nodes), out)
		require.Len(t, nodes, 1)

		assert.Equal(t, "plugin", nodes[0].Source)
		assert.Equal(t, []string{"read:users"}, nodes[0].RequiredScopes)
		assert.Contains(t, out, `"source": "plugin"`)
		assert.Contains(t, out, `"requiredScopes"`)

		out = captureOutput(t, func() { require.NoError(t, r.exec("commands", "checkmate", "--flat", "--json")) })
		require.NoError(t, json.Unmarshal([]byte(out), &nodes), out)

		checkSource(t, nodes)

		out = captureOutput(t, func() { require.NoError(t, r.exec("commands", "checkmate")) })
		assert.Contains(t, out, "scan")
		assert.Contains(t, out, "Scan the users")
	})

	t.Run("a core command is the same with and without plugins", func(t *testing.T) {
		// Both trees are new, because listing a tree changes how a command writes
		// its usage line.
		with, without := buildCommandTree(r.tree(), 0, true), buildCommandTree(newRootTree(&cli{}), 0, true)

		for _, node := range with {
			if node.Source != "" {
				continue
			}

			expected, ok := find(without, node.Path)
			require.True(t, ok, node.Path)

			got, err := json.Marshal(node)
			require.NoError(t, err)

			want, err := json.Marshal(expected)
			require.NoError(t, err)

			assert.Equal(t, string(want), string(got), node.Path)
		}
	})

	t.Run("a core node has no new field", func(t *testing.T) {
		data, err := json.Marshal(commandNode{Path: "auth0 login", Name: "login", Short: "Authenticate", Runnable: true, RequiresAuth: false})
		require.NoError(t, err)

		assert.JSONEq(t, `{"path":"auth0 login","name":"login","short":"Authenticate","runnable":true,"requiresAuth":false}`, string(data))
	})
}

func TestPluginRun_DeniedCallsInTheEnvelope(t *testing.T) {
	t.Setenv(agentModeEnvVar, "false")

	denied := func(t *testing.T, err error) string {
		t.Helper()

		var details struct {
			PluginID    string `json:"pluginId"`
			ExitCode    int    `json:"exitCode"`
			DeniedCalls string `json:"deniedCalls"`
		}

		require.NoError(t, json.Unmarshal(buildErrorEnvelope(err).Error.Details, &details))
		assert.Equal(t, "auth0/checkmate", details.PluginID)

		return details.DeniedCalls
	}

	t.Run("a failed run in agent mode carries the summary and prints nothing", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		r.approve()

		err := r.exec("checkmate", "--agent-mode", "--mode=call", "--path=/api/v2/users", "--code=3")

		require.Error(t, err)
		assert.Equal(t, "plugin_exit", errorReason(err))
		assert.Contains(t, denied(t, err), "API call was denied")
		assert.NotContains(t, r.stderr.String(), "API call was denied", "stderr stays free of the human line")
		assert.Empty(t, r.stderr.String())
	})

	t.Run("a failed run with a person still gets the line, and the envelope has it too", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		r.approve()

		err := r.exec("checkmate", "--mode=call", "--path=/api/v2/users", "--code=3")

		require.Error(t, err)
		assert.Contains(t, r.stderr.String(), "API call was denied")
		assert.Contains(t, denied(t, err), "API call was denied")
	})

	t.Run("a run that ends well adds nothing", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		r.approve()

		require.NoError(t, r.exec("checkmate", "--agent-mode", "--mode=call", "--path=/api/v2/users"))

		assert.Contains(t, r.stdout.String(), "status=403")
		assert.Empty(t, r.stderr.String())
	})

	t.Run("a failed run without a denied call adds nothing", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		r.approve()

		err := r.exec("checkmate", "--agent-mode", "--mode=call", "--path=/api/v2/clients", "--code=3")

		require.Error(t, err)
		assert.JSONEq(t, `{"pluginId":"auth0/checkmate","version":"1.0.0","exitCode":3}`, string(buildErrorEnvelope(err).Error.Details))
	})

	t.Run("every way a run ends badly carries it", func(t *testing.T) {
		p := &pluginCommand{id: pluginTestID, version: "1.0.0"}

		for _, failure := range []error{
			&runner.ExitError{Code: 2},
			&runner.TimeoutError{After: time.Second},
			&runner.InterruptedError{},
			errors.New("could not start"),
		} {
			err := p.failure(failure, "1 API call was denied.")

			assert.Equal(t, "1 API call was denied.", denied(t, err), failure)
			assert.NotContains(t, string(buildErrorEnvelope(p.failure(failure, "")).Error.Details), "deniedCalls", failure)
		}
	})
}

// linkFixture links a build of the fixture plugin whose executable is the test
// binary, with dev mode on.
func (r *runEnv) linkFixture(version string, commands []plugins.Command) devBuild {
	r.t.Helper()

	r.t.Setenv(pluginDevModeEnv, "true")

	binary, err := fixtureBinary()
	require.NoError(r.t, err)

	build := writeDevBuild(r.t, version, commands, binary)
	r.link(build)

	return build
}

func TestPluginRun_Linked(t *testing.T) {
	// A refusal never starts the plugin, so it never starts the proxy either.
	assertRefused := func(t *testing.T, r *runEnv, err error, reason string) {
		t.Helper()

		require.Error(t, err)
		assertValidationError(t, err, reason)
		assert.False(t, r.started(), "the plugin must not have run")
		assert.Empty(t, r.tenant.seen())
	}

	// An approved linked build.
	newLinked := func(t *testing.T) (*runEnv, devBuild) {
		t.Helper()

		r := newRunEnv(t, fixtureCommands())
		build := r.linkFixture("2.0.0", fixtureCommands())
		r.approveBuild(build)

		return r, build
	}

	t.Run("runs through the proxy with the scopes of its command", func(t *testing.T) {
		r, build := newLinked(t)

		require.NoError(t, r.exec("checkmate", "--mode=call", "--path=/api/v2/clients"))

		assert.Contains(t, r.stdout.String(), "status=200")
		assert.NotContains(t, r.stdout.String(), "shh", "the secret field is removed")
		require.Len(t, r.tenant.seen(), 1)
		assert.Equal(t, "Bearer "+r.Token, r.tenant.seen()[0].authorization)
		assert.Equal(t, 1, strings.Count(r.stderr.String(), linkedBanner(pluginTestID, "2.0.0", build.Dir)), "one banner for one run")

		require.NoError(t, r.exec("checkmate", "--mode=call", "--path=/api/v2/users"))

		assert.Contains(t, r.stdout.String(), "status=403", "the root command cannot read users")
		assert.Contains(t, r.stderr.String(), "API call was denied")
		assert.Len(t, r.tenant.seen(), 1, "a denied call never reaches the tenant")

		require.NoError(t, r.exec("checkmate", "scan", "--mode=call", "--path=/api/v2/users"))

		assert.Contains(t, r.stdout.String(), "status=200", "scan has the scopes of its own command")
	})

	t.Run("takes the place of the installed plugin of the same name", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		r.approve()

		build := r.linkFixture("2.0.0", fixtureCommands())
		r.approveBuild(build)

		require.NoError(t, r.exec("checkmate"))
		assert.Contains(t, r.stderr.String(), linkedBanner(pluginTestID, "2.0.0", build.Dir), "the linked build ran")

		require.NoError(t, devlink.Links{Dir: r.Dir}.Remove(context.Background(), pluginTestID))
		require.NoError(t, r.exec("checkmate"))

		assert.NotContains(t, r.stderr.String(), "linked build", "the installed plugin runs again")
		assert.True(t, r.started())
	})

	t.Run("runs although the installed copy fails verification", func(t *testing.T) {
		r, build := newLinked(t)

		verified, err := r.svc.store.Verify(pluginTestID)
		require.NoError(t, err)

		tamper(t, verified.Bin)

		require.NoError(t, r.exec("checkmate"))
		assert.Contains(t, r.stderr.String(), linkedBanner(pluginTestID, "2.0.0", build.Dir))
		assert.True(t, r.started())
	})

	t.Run("reads the build as it is on every run", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("a running executable cannot be replaced")
		}

		r, build := newLinked(t)

		require.NoError(t, r.exec("checkmate"))

		// A rebuild: a new executable in the place of the old one, and an edit of the
		// manifest that changes nothing that was approved.
		binary, err := fixtureBinary()
		require.NoError(t, err)

		staged := build.Bin + ".new"
		require.NoError(t, os.WriteFile(staged, binary, 0o755))
		require.NoError(t, os.Rename(staged, build.Bin))

		build.Manifest.Version = "2.1.0"
		build.Manifest.Description = "Checks the tenant again"
		build.save(t)

		require.NoError(t, r.exec("checkmate"))

		assert.Contains(t, r.stderr.String(), linkedBanner(pluginTestID, "2.1.0", build.Dir), "no new link and no new approval")
		assert.True(t, r.started())
	})

	t.Run("still needs the approval of a person", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		r.approve()

		// The approval of the installed copy does not count for a build.
		r.linkFixture("2.0.0", []plugins.Command{{Name: pluginTestBin, Description: "Check a tenant", Permissions: fixturePermissions([]string{"read:clients", "read:users"})}})

		err := r.exec("checkmate")

		assertRefused(t, r, err, "plugin_not_approved")
		assert.Contains(t, err.Error(), "auth0 plugin approve auth0/checkmate --tenant "+pluginTestTenant)
		assert.Zero(t, r.logins, "an unapproved build never starts a login")
	})

	t.Run("needs a new approval when the permissions changed", func(t *testing.T) {
		r, build := newLinked(t)

		require.NoError(t, r.exec("checkmate"))

		build.Manifest.Commands[0].Permissions = fixturePermissions([]string{"read:clients", "delete:clients"})
		build.save(t)

		err := r.exec("checkmate")

		assertRefused(t, r, err, "plugin_not_approved")
		assert.Zero(t, r.logins)

		r.approveBuild(build)
		require.NoError(t, r.exec("checkmate"))
		assert.True(t, r.started())
	})

	t.Run("fails when the manifest changed while the session was opened", func(t *testing.T) {
		r, build := newLinked(t)

		login := r.svc.login
		r.svc.login = func(ctx context.Context, c *cli) error {
			if err := login(ctx, c); err != nil {
				return err
			}

			build.Manifest.Commands[0].Permissions = fixturePermissions([]string{"read:clients", "delete:clients"})
			build.save(t)

			return nil
		}

		err := r.exec("checkmate")

		require.Error(t, err)
		assert.Equal(t, "plugin_changed", errorReason(err))
		assert.Equal(t, 1, r.logins, "the build is read again after the session is open")
		assert.False(t, r.started())
		assert.Empty(t, r.tenant.seen())
	})

	t.Run("fails when the executable went away while the session was opened", func(t *testing.T) {
		r, build := newLinked(t)

		login := r.svc.login
		r.svc.login = func(ctx context.Context, c *cli) error {
			if err := login(ctx, c); err != nil {
				return err
			}

			return os.Remove(build.Bin)
		}

		err := r.exec("checkmate")

		require.Error(t, err)
		assertValidationError(t, err, "plugin_link_invalid")
		assert.False(t, r.started())
	})

	t.Run("keeps the index of the registry", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands(), func(b *registrytest.IndexBuilder) {
			b.SetExpires(time.Date(2026, time.October, 1, 13, 0, 0, 0, time.UTC))
		})
		build := r.linkFixture("2.0.0", fixtureCommands())
		r.approveBuild(build)

		r.Now = r.Now.Add(2 * time.Hour)
		r.server.Fail("/index.json", http.StatusServiceUnavailable)

		err := r.exec("checkmate")

		require.Error(t, err)
		assert.Equal(t, "plugin_registry_unavailable", errorReason(err))
		assert.False(t, r.started())
		assert.Zero(t, r.logins)
	})

	t.Run("keeps the rules of the tenant", func(t *testing.T) {
		r, _ := newLinked(t)

		t.Setenv(authModeEnvVar, authModeEnv)
		t.Setenv(envDomain, "fixed.us.auth0.com")
		t.Setenv(envAPIToken, "env-token")

		err := r.exec("checkmate", "--tenant", pluginTestTenant)

		require.Error(t, err)
		assert.Equal(t, "env_auth_tenant_conflict", errorReason(err))
		assert.False(t, r.started())
		assert.Zero(t, r.logins)
	})

	t.Run("keeps the tenant of the session", func(t *testing.T) {
		r, _ := newLinked(t)
		r.Domain = "other.us.auth0.com"

		err := r.exec("checkmate")

		require.Error(t, err)
		assert.Equal(t, "plugin_tenant_mismatch", errorReason(err))
		assert.False(t, r.started())
	})

	t.Run("passes only the environment the command declared", func(t *testing.T) {
		r, _ := newLinked(t)

		t.Setenv("AUTH0_CLIENT_SECRET", "the-secret-of-the-cli")
		t.Setenv(pluginFixtureEnv, "declared-value")
		t.Setenv("UNDECLARED_VARIABLE", "undeclared")

		require.NoError(t, r.exec("checkmate", "scan"))

		env := r.report(t).Env

		assert.Equal(t, "declared-value", env[pluginFixtureEnv])
		assert.NotContains(t, env, "AUTH0_CLIENT_SECRET")
		assert.NotContains(t, env, "UNDECLARED_VARIABLE")
		assert.NotContains(t, env, pluginDevModeEnv)
	})

	t.Run("stops a command that runs out of time", func(t *testing.T) {
		commands := fixtureCommands()
		commands[1].Timeout = "1s"

		r := newRunEnv(t, commands)
		build := r.linkFixture("2.0.0", commands)
		r.approveBuild(build)

		err := r.exec("checkmate", "scan", "--mode=sleep", "--marker="+filepath.Join(t.TempDir(), "pid"))

		require.Error(t, err)
		assert.Equal(t, "plugin_timeout", errorReason(err))
	})

	t.Run("refuses an output the command does not support", func(t *testing.T) {
		r, _ := newLinked(t)

		err := r.exec("checkmate", "scan", "--json")

		require.Error(t, err)
		assert.Equal(t, "plugin_output_unsupported", errorReason(err))
		assert.False(t, r.started())
		assert.Empty(t, r.tenant.seen())
	})

	t.Run("prints no banner in machine output", func(t *testing.T) {
		for _, flag := range []string{"--json", "--json-compact", "--agent-mode"} {
			r, _ := newLinked(t)

			require.NoError(t, r.exec("checkmate", flag, "--mode=call", "--path=/api/v2/clients"), flag)

			assert.Contains(t, r.stdout.String(), "status=200", flag)
			assert.Empty(t, r.stderr.String(), flag)
		}
	})

	t.Run("is not revoked by the registry", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		build := r.linkFixture("1.0.0", fixtureCommands())
		r.approveBuild(build)
		r.approve()

		r.republish(r.fixtureIndex(r.record).Revoke(pluginTestID, "1.0.0", "it leaks tokens."))

		require.NoError(t, r.exec("checkmate"))
		assert.True(t, r.started())

		// The installed copy of the same version is revoked, so the revocation is read.
		require.NoError(t, devlink.Links{Dir: r.Dir}.Remove(context.Background(), pluginTestID))

		err := r.exec("checkmate")

		assertRefused(t, r, err, "plugin_revoked")
	})

	t.Run("does not hint at an update of the installed copy", func(t *testing.T) {
		r := newHintEnv(t)
		build := r.linkFixture("1.0.0", fixtureCommands())
		r.approveBuild(build)

		require.NoError(t, r.exec("checkmate"))

		assert.NotContains(t, r.stderr.String(), hintLine)
		assert.NoFileExists(t, hintPath(r))

		require.NoError(t, devlink.Links{Dir: r.Dir}.Remove(context.Background(), pluginTestID))
		require.NoError(t, r.exec("checkmate"))

		assert.Contains(t, r.stderr.String(), hintLine, "the installed copy still gets it")
	})
}

func TestPluginRun_LinkedNeedsDevMode(t *testing.T) {
	for _, value := range devModeValues {
		t.Run("the links are ignored with dev mode "+value, func(t *testing.T) {
			r := newRunEnv(t, fixtureCommands())
			r.approve()

			build := r.linkFixture("2.0.0", fixtureCommands())
			r.approveBuild(build)

			t.Setenv(pluginDevModeEnv, value)

			require.NoError(t, r.exec("checkmate"))

			assert.NotContains(t, r.stderr.String(), "linked build", "the installed plugin ran")
			assert.True(t, r.started())

			// A tampered copy is refused, and the link does not stand in for it.
			verified, err := r.svc.store.Verify(pluginTestID)
			require.NoError(t, err)

			tamper(t, verified.Bin)

			err = r.exec("checkmate")

			require.Error(t, err)
			assert.Equal(t, "plugin_verification_failed", errorReason(err))
			assert.False(t, r.started())
		})
	}

	t.Run("a run does not start when dev mode went off after the commands were made", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		build := r.linkFixture("2.0.0", fixtureCommands())
		r.approveBuild(build)

		r.cli.renderer = &display.Renderer{MessageWriter: r.stderr, ResultWriter: r.stdout}

		root := buildRootCmd(r.cli)
		addPersistentFlags(root, r.cli)
		addSubCommands(root, r.cli)
		enforceUnknownSubcommand(root)
		root.SetOut(r.stdout)
		root.SetErr(r.stderr)

		// The command of the linked build is in the tree.
		found, _, err := root.Find([]string{pluginTestBin})
		require.NoError(t, err)
		require.NotEqual(t, root, found)

		t.Setenv(pluginDevModeEnv, "")

		root.SetArgs([]string{"checkmate"})

		err = root.ExecuteContext(context.Background())

		require.Error(t, err)
		assertValidationError(t, err, "plugin_dev_mode_required")
		assert.False(t, r.started())
		assert.Zero(t, r.logins)
		assert.Empty(t, r.tenant.seen())
	})

	t.Run("verify of a linked plugin needs dev mode", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		build := r.linkFixture("2.0.0", fixtureCommands())

		link := devlink.Link{Path: build.Dir, Bin: build.Bin}
		linked := &pluginCommand{cli: r.cli, id: pluginTestID, link: &link}

		verified, err := linked.verify(r.svc)

		require.NoError(t, err)
		assert.Equal(t, "2.0.0", verified.Version)

		t.Setenv(pluginDevModeEnv, "")

		_, err = linked.verify(r.svc)

		require.Error(t, err)
		assertValidationError(t, err, "plugin_dev_mode_required")
	})
}

func TestRegisterPluginCommands_Linked(t *testing.T) {
	t.Run("a linked plugin comes first, and takes the name of an installed one", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		build := r.linkFixture("2.0.0", fixtureCommands())

		root := &cobra.Command{Use: "auth0"}
		addPersistentFlags(root, r.cli)
		registerPluginCommands(root, r.cli, "dev", nil)

		require.Len(t, root.Commands(), 1, "one command for the name")

		var info struct {
			Version string `json:"version"`
		}

		require.NoError(t, json.Unmarshal([]byte(root.Commands()[0].Annotations[pluginInfoAnnotation]), &info))
		assert.Equal(t, build.Manifest.Version, info.Version, "the command is the one of the linked build")
	})

	t.Run("a linked plugin with no installed plugin", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "2.0.0", fixtureCommands(), stubExecutable)
		e.link(build)

		root := &cobra.Command{Use: "auth0"}
		addPersistentFlags(root, e.cli)
		registerPluginCommands(root, e.cli, "dev", nil)

		require.Len(t, root.Commands(), 1)
		assert.Equal(t, pluginTestBin, root.Commands()[0].Name())
	})

	t.Run("a name of a core command is not registered", func(t *testing.T) {
		e := newDevEnv(t)
		build := writeDevBuild(t, "2.0.0", fixtureCommands(), stubExecutable)
		e.link(build)

		root := &cobra.Command{Use: "auth0"}
		root.AddCommand(&cobra.Command{Use: pluginTestBin})

		registerPluginCommands(root, e.cli, "dev", nil)

		assert.Len(t, root.Commands(), 1)
	})

	t.Run("nothing is registered or read when dev mode is off", func(t *testing.T) {
		for _, value := range devModeValues {
			e := newPluginEnv(t)
			t.Setenv(pluginDevModeEnv, value)

			// A file that would be an error if it were read.
			require.NoError(t, os.MkdirAll(e.Dir, 0o700))
			require.NoError(t, os.WriteFile(e.linksPath(), []byte("{damaged"), 0o600))

			build := writeDevBuild(t, "2.0.0", fixtureCommands(), stubExecutable)
			require.NoError(t, os.WriteFile(e.linksPath(), []byte(`{"formatVersion":1,"links":{"auth0/checkmate":{"path":`+strconv.Quote(build.Dir)+`,"bin":`+strconv.Quote(build.Bin)+`}}}`), 0o600))

			root := &cobra.Command{Use: "auth0"}

			assert.NotPanics(t, func() { registerPluginCommands(root, e.cli, "dev", []*cobra.Command{nil}) })
			assert.Empty(t, root.Commands(), value)
		}
	})

	t.Run("a file of links that is damaged registers no link and breaks nothing", func(t *testing.T) {
		r := newRunEnv(t, fixtureCommands())
		r.approve()

		t.Setenv(pluginDevModeEnv, "true")

		require.NoError(t, os.WriteFile(filepath.Join(r.Dir, devlink.FileName), []byte("{damaged"), 0o600))

		root := &cobra.Command{Use: "auth0"}
		addPersistentFlags(root, r.cli)

		assert.NotPanics(t, func() { registerPluginCommands(root, r.cli, "dev", nil) })

		require.Len(t, root.Commands(), 1, "the installed plugin is registered as it is without links")

		require.NoError(t, r.exec("apps", "--help"))
		assert.Empty(t, r.stderr.String())

		require.NoError(t, r.exec("checkmate"))
		assert.NotContains(t, r.stderr.String(), "linked build")
		assert.True(t, r.started())
	})

	t.Run("a damaged file with nothing installed registers nothing", func(t *testing.T) {
		e := newDevEnv(t)

		require.NoError(t, os.MkdirAll(e.Dir, 0o700))
		require.NoError(t, os.WriteFile(e.linksPath(), []byte("{damaged"), 0o600))

		root := &cobra.Command{Use: "auth0"}

		assert.NotPanics(t, func() { registerPluginCommands(root, e.cli, "dev", []*cobra.Command{nil}) })
		assert.Empty(t, root.Commands())
	})
}

func TestPluginRun_LinkedThatCannotBeUsed(t *testing.T) {
	tests := []struct {
		name   string
		damage func(t *testing.T, build devBuild)
		want   []string
	}{
		{
			name:   "the executable is gone",
			damage: func(t *testing.T, build devBuild) { require.NoError(t, os.Remove(build.Bin)) },
			want:   []string{"the linked plugin auth0/checkmate cannot be used", "{bin}"},
		},
		{
			name: "the manifest is gone",
			damage: func(t *testing.T, build devBuild) {
				require.NoError(t, os.Remove(filepath.Join(build.Dir, devlink.ManifestName)))
			},
			want: []string{"the linked plugin auth0/checkmate cannot be used", devlink.ManifestName},
		},
		{
			name:   "the manifest has a problem",
			damage: func(t *testing.T, build devBuild) { build.writeManifestText(t, `{"id":"auth0/checkmate"}`) },
			want:   []string{"cannot be used", "version:"},
		},
		{
			name: "the manifest is for another plugin",
			damage: func(t *testing.T, build devBuild) {
				build.Manifest.ID = "auth0/other"
				build.save(t)
			},
			want: []string{"is for the plugin auth0/other, not auth0/checkmate"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := newRunEnv(t, fixtureCommands())
			r.approve()

			build := r.linkFixture("2.0.0", fixtureCommands())
			r.approveBuild(build)

			test.damage(t, build)

			for _, args := range [][]string{{"checkmate"}, {"checkmate", "scan"}, {"checkmate", "--agent-mode"}} {
				err := r.exec(args...)

				require.Error(t, err, args)
				assertValidationError(t, err, "plugin_link_invalid")

				for _, want := range test.want {
					assert.Contains(t, err.Error(), strings.ReplaceAll(want, "{bin}", build.Bin))
				}

				assert.Contains(t, err.Error(), "auth0 plugin dev validate "+build.Dir)
				assert.Contains(t, err.Error(), "auth0 plugin dev unlink checkmate")
				assert.False(t, r.started(), "neither the build nor the installed plugin runs")
				assert.Empty(t, r.tenant.seen())
				assert.Zero(t, r.logins)
			}
		})
	}
}
