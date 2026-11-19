package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"

	"github.com/auth0/auth0-cli/internal/auth"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/poppy/doctor"
)

func TestPoppyDoctorSkipsAuthGate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	tests := []struct {
		path    []string
		wantErr bool
	}{
		{path: []string{"poppy", "doctor"}},
		{path: []string{"poppy", "setup"}, wantErr: true},
		{path: []string{"agents", "register"}, wantErr: true},
	}

	for _, test := range tests {
		t.Run(strings.Join(test.path, " "), func(t *testing.T) {
			testCLI := &cli{renderer: display.NewRenderer()}
			root := buildRootCmd(testCLI)
			addSubCommands(root, testCLI)
			cmd, _, err := root.Find(test.path)
			require.NoError(t, err)

			err = root.PersistentPreRunE(cmd, nil)

			if test.wantErr {
				require.Error(t, err, "only doctor runs without a login")
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestPoppyDoctorDomain(t *testing.T) {
	dir := t.TempDir()
	writeConfig := func(t *testing.T, name, domain string) string {
		t.Helper()
		data, err := json.Marshal(config.Config{Organization: config.Organization{Domain: domain}})
		require.NoError(t, err)
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, data, 0o600))
		return path
	}
	broken := filepath.Join(dir, "broken.json")
	require.NoError(t, os.WriteFile(broken, []byte("{"), 0o600))

	tests := []struct {
		name       string
		args       []string
		configFile string
		want       string
		wantErr    string
	}{
		{name: "argument", args: []string{"Shop.Example"}, configFile: "missing.json", want: "shop.example"},
		{name: "argument as URL", args: []string{"https://shop.example/"}, want: "shop.example"},
		{name: "argument wins over the config file", args: []string{"a.example"}, configFile: writeConfig(t, "a.json", "b.example"), want: "a.example"},
		{name: "argument with a path", args: []string{"shop.example/orders"}, wantErr: "path"},
		{name: "argument with a port", args: []string{"shop.example:443"}, wantErr: "port"},
		{name: "config file", configFile: writeConfig(t, "c.json", "shop.example"), want: "shop.example"},
		{name: "no argument and no config file", configFile: filepath.Join(dir, "missing.json"), wantErr: "pass a domain"},
		{name: "config file without a domain", configFile: writeConfig(t, "d.json", ""), wantErr: "organization.domain"},
		{name: "unreadable config file", configFile: broken, wantErr: "failed to parse"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := poppyDoctorDomain(test.args, test.configFile)

			if test.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestRunPoppyDoctor(t *testing.T) {
	tests := []struct {
		name   string
		format display.OutputFormat
		check  func(t *testing.T, out string)
	}{
		{
			name: "text",
			check: func(t *testing.T, out string) {
				assert.Contains(t, out, "FAIL D1 (spec 3) https://127.0.0.1/.well-known/poppy.json returned HTTP 404")
				assert.Contains(t, out, "FIX  Serve the document at https://127.0.0.1/.well-known/poppy.json with status 200.")
				assert.Contains(t, out, "- skipped: poppy.json is not available")
				assert.Contains(t, out, "2 failed, 0 passed, 15 skipped.")
			},
		},
		{
			name:   "json",
			format: display.OutputFormatJSON,
			check: func(t *testing.T, out string) {
				var report doctor.Report
				require.NoError(t, json.Unmarshal([]byte(out), &report))
				assert.Equal(t, "127.0.0.1", report.Domain)
				assert.Equal(t, 1, report.ExitCode())
				assert.False(t, report.TenantChecked, "there is no login")
				assert.Equal(t, doctor.Counts{Fail: 2, Skipped: 15}, report.Totals)

				var raw struct {
					Summary map[string]int `json:"summary"`
					Results []struct {
						Code       string `json:"code"`
						SkipReason string `json:"skip_reason"`
					} `json:"results"`
				}
				require.NoError(t, json.Unmarshal([]byte(out), &raw))
				assert.Equal(t, map[string]int{"pass": 0, "warn": 0, "fail": 2, "skipped": 15}, raw.Summary)
				for _, res := range raw.Results {
					assert.NotEmpty(t, res.Code)
				}
			},
		},
		{
			name:   "json compact",
			format: display.OutputFormatJSONCompact,
			check: func(t *testing.T, out string) {
				assert.Equal(t, 1, strings.Count(out, "\n"), "one line")
				var report doctor.Report
				require.NoError(t, json.Unmarshal([]byte(out), &report))
				assert.Equal(t, "127.0.0.1", report.Domain)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			testCLI := &cli{renderer: &display.Renderer{ResultWriter: &out, MessageWriter: io.Discard, Format: test.format}}

			err := runPoppyDoctor(t.Context(), testCLI, "127.0.0.1", newDoctorTestOptions(t), false)

			require.ErrorIs(t, err, errPoppyDoctorFailed)
			assert.Equal(t, "doctor_checks_failed", errorReason(err))
			assert.Equal(t, "validation", errorClass(err))
			assert.JSONEq(t, `{"failed":2,"warnings":0,"skipped":15}`, string(errorDetails(err)))
			test.check(t, out.String())
		})
	}
}

// poppySessionToken is a well-formed JWT: only its shape is checked.
const poppySessionToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJodHRwczovL2F1dGgwLmF1dGgwLmNvbS8iLCJpYXQiOjE2ODExNDcwNjAsImV4cCI6OTY4MTgzMzQ2MH0.DsEpQkL0MIWcGJOIfEY8vr3MVS_E0GYsachNLQwBu5Q"

// saveSession writes a saved login to the config file under a new home directory.
func saveSession(t *testing.T, defaultTenant string, tenants map[string]any) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(authModeEnvVar, "")
	keyring.MockInit()

	saved, err := json.Marshal(map[string]any{"default_tenant": defaultTenant, "tenants": tenants})
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".config", "auth0"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".config", "auth0", "config.json"), saved, 0o600))
}

func TestOptionalPoppyTenant(t *testing.T) {
	const domain = "tenant.us.auth0.com"

	valid := func(extra map[string]any) map[string]any {
		tenant := map[string]any{
			"domain":       domain,
			"access_token": poppySessionToken,
			"scopes":       auth.RequiredScopes,
			"expires_at":   time.Now().Add(time.Hour).Format(time.RFC3339),
		}
		maps.Copy(tenant, extra)

		return map[string]any{domain: tenant}
	}

	tests := []struct {
		name          string
		setup         func(t *testing.T)
		tenant        string
		wantTenant    bool
		wantErr       string
		wantTenantSet string
	}{
		{
			name:  "no saved login",
			setup: func(t *testing.T) { t.Setenv("HOME", t.TempDir()); t.Setenv(authModeEnvVar, "") },
		},
		{
			name:  "a usable saved login",
			setup: func(t *testing.T) { saveSession(t, domain, valid(nil)) },

			wantTenant:    true,
			wantTenantSet: domain,
		},
		{
			name:   "the tenant flag picks another saved login",
			setup:  func(t *testing.T) { saveSession(t, "other.us.auth0.com", valid(nil)) },
			tenant: domain,

			wantTenant:    true,
			wantTenantSet: domain,
		},
		{
			name:  "no default tenant",
			setup: func(t *testing.T) { saveSession(t, "", valid(nil)) },
		},
		{
			name:   "the tenant flag names a tenant that is not saved",
			setup:  func(t *testing.T) { saveSession(t, domain, valid(nil)) },
			tenant: "missing.us.auth0.com",
		},
		{
			name: "an expired device login, which a command would answer with a device-code login",
			setup: func(t *testing.T) {
				saveSession(t, domain, valid(map[string]any{"expires_at": time.Now().Add(-time.Hour).Format(time.RFC3339)}))
			},
		},
		{
			name: "an expired client-credentials login whose refresh fails",
			setup: func(t *testing.T) {
				saveSession(t, domain, valid(map[string]any{"client_id": "client", "expires_at": time.Now().Add(-time.Hour).Format(time.RFC3339)}))
			},
		},
		{
			name:  "a device login without the required scopes, which a command would answer with a login",
			setup: func(t *testing.T) { saveSession(t, domain, valid(map[string]any{"scopes": []string{"openid"}})) },
		},
		{
			name:  "a login with a corrupted token",
			setup: func(t *testing.T) { saveSession(t, domain, valid(map[string]any{"access_token": "not-a-token"})) },
		},
		{
			name: "a login whose token cannot be read",
			setup: func(t *testing.T) {
				saveSession(t, domain, valid(map[string]any{"access_token": ""}))
			},
		},
		{
			name: "environment credentials",
			setup: func(t *testing.T) {
				t.Setenv("HOME", t.TempDir())
				t.Setenv(authModeEnvVar, authModeEnv)
				t.Setenv(envDomain, domain)
				t.Setenv(envAPIToken, "token")
			},
			wantTenant:    true,
			wantTenantSet: domain,
		},
		{
			name: "environment credentials that are incomplete",
			setup: func(t *testing.T) {
				t.Setenv("HOME", t.TempDir())
				t.Setenv(authModeEnvVar, authModeEnv)
				t.Setenv(envDomain, "")
			},
			wantErr: "requires",
		},
		{
			name: "an unknown auth mode",
			setup: func(t *testing.T) {
				t.Setenv("HOME", t.TempDir())
				t.Setenv(authModeEnvVar, "evn")
			},
			wantErr: "unrecognized",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.setup(t)

			var messages bytes.Buffer
			testCLI := &cli{renderer: &display.Renderer{MessageWriter: &messages, ResultWriter: io.Discard}, tenant: test.tenant}

			got, err := testCLI.optionalPoppyTenant(t.Context())

			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.Empty(t, messages.String(), "nothing is printed, asked or warned about")

			if !test.wantTenant {
				assert.Nil(t, got)
				assert.Nil(t, testCLI.apiv3, "no client is built")
				return
			}

			require.NotNil(t, got)
			assert.NotNil(t, got.State)
			assert.NotNil(t, got.Agents)
			assert.NotNil(t, got.Policies)
			assert.Equal(t, test.wantTenantSet, testCLI.tenant)
		})
	}
}

func TestOptionalPoppyTenantDoesNotChangeTheConfig(t *testing.T) {
	// A saved login without a default tenant is repaired by a command that needs
	// a login, which writes the config. The doctor leaves the file alone.
	saveSession(t, "", map[string]any{"a.us.auth0.com": map[string]any{"domain": "a.us.auth0.com", "access_token": poppySessionToken}})

	path := filepath.Join(os.Getenv("HOME"), ".config", "auth0", "config.json")
	before, err := os.ReadFile(path)
	require.NoError(t, err)

	got, err := (&cli{renderer: display.NewRenderer()}).optionalPoppyTenant(t.Context())

	require.NoError(t, err)
	assert.Nil(t, got)

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after))
}

func TestPoppyDoctorCommandWithoutLogin(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(authModeEnvVar, "")

	var out bytes.Buffer
	testCLI := &cli{
		renderer: &display.Renderer{ResultWriter: &out, MessageWriter: io.Discard},
	}
	useDoctorTestOptions(t)

	root := buildRootCmd(testCLI)
	root.PersistentPreRunE = nil
	root.SilenceErrors, root.SilenceUsage = true, true
	addSubCommands(root, testCLI)
	root.SetArgs([]string{"poppy", "doctor", "shop.example", "--config", filepath.Join(t.TempDir(), "missing.json")})

	err := root.ExecuteContext(t.Context())

	require.ErrorIs(t, err, errPoppyDoctorFailed, "the website cannot be reached")
	text := out.String()
	assert.Contains(t, text, "FAIL D1 (spec 3) https://shop.example/.well-known/poppy.json could not be fetched")
	assert.Equal(t, 1, strings.Count(text, "skipped (7 checks): no usable tenant login, so the tenant checks did not run"), "the same reason is said once")
	assert.Contains(t, text, "FIX  auth0 login")
	assert.Nil(t, testCLI.apiv3, "no client is built")
}

func TestPoppyDoctorError(t *testing.T) {
	passed := doctor.Result{ID: "D1", Level: doctor.Pass}
	skipped := doctor.Result{ID: "D8", Level: doctor.Skipped}
	warned := doctor.Result{ID: "D5", Level: doctor.Warn}
	failed := doctor.Result{ID: "D3", Level: doctor.Fail}

	tests := []struct {
		name        string
		results     []doctor.Result
		strict      bool
		want        error
		wantDetails string
	}{
		{name: "everything passed", results: []doctor.Result{passed}},
		{name: "a warning does not fail", results: []doctor.Result{passed, warned}, strict: true},
		{name: "a skipped check does not fail", results: []doctor.Result{passed, skipped}},
		{
			name:        "a failed check fails",
			results:     []doctor.Result{passed, failed, warned, skipped, skipped},
			want:        errPoppyDoctorFailed,
			wantDetails: `{"failed":1,"warnings":1,"skipped":2}`,
		},
		{
			name:        "a failed check fails with --strict too",
			results:     []doctor.Result{failed, skipped},
			strict:      true,
			want:        errPoppyDoctorFailed,
			wantDetails: `{"failed":1,"warnings":0,"skipped":1}`,
		},
		{
			name:        "a skipped check fails with --strict",
			results:     []doctor.Result{passed, skipped},
			strict:      true,
			want:        errPoppyDoctorIncomplete,
			wantDetails: `{"failed":0,"warnings":0,"skipped":1}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := poppyDoctorError(doctor.Report{Results: test.results}, test.strict)

			if test.want == nil {
				require.NoError(t, err)
				return
			}

			require.ErrorIs(t, err, test.want)
			assert.Equal(t, "doctor_checks_failed", errorReason(err))
			assert.Equal(t, "validation", errorClass(err))
			assert.JSONEq(t, test.wantDetails, string(errorDetails(err)))
		})
	}
}

func TestPoppyDoctorCommandStrict(t *testing.T) {
	// A failed check is what the error reports, whether or not --strict is set.
	t.Setenv("HOME", t.TempDir())
	t.Setenv(authModeEnvVar, "")
	useDoctorTestOptions(t)

	run := func(t *testing.T, args ...string) (string, error) {
		t.Helper()

		var out bytes.Buffer
		testCLI := &cli{renderer: &display.Renderer{ResultWriter: &out, MessageWriter: io.Discard, Format: display.OutputFormatJSON}}

		root := buildRootCmd(testCLI)
		root.PersistentPreRunE = nil
		root.SilenceErrors, root.SilenceUsage = true, true
		addSubCommands(root, testCLI)
		root.SetArgs(append([]string{"poppy", "doctor", "shop.example", "--config", filepath.Join(t.TempDir(), "missing.json")}, args...))

		err := root.ExecuteContext(t.Context())

		return out.String(), err
	}

	out, err := run(t, "--strict")
	require.ErrorIs(t, err, errPoppyDoctorFailed, "a failed check is reported before skipped ones")
	assert.JSONEq(t, `{"failed":2,"warnings":0,"skipped":15}`, string(errorDetails(err)))

	var report doctor.Report
	require.NoError(t, json.Unmarshal([]byte(out), &report))
	assert.False(t, report.TenantChecked)
}
