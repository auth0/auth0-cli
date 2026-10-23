package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// lookupIn returns a lookup function over a fixed environment.
func lookupIn(env map[string]string) func(string) string {
	return func(name string) string { return env[name] }
}

// userEnv is an environment with everything on the allowlist, and with variables
// that a plugin must never see.
func userEnv() map[string]string {
	env := map[string]string{
		"AUTH0_CLIENT_SECRET": "secret",
		"AUTH0_API_TOKEN":     "token",
		"AUTH0_CLIENT_ID":     "id",
		"AUTH0_DOMAIN":        "acme.auth0.com",
		"auth0_client_secret": "secret",
		"Auth0_Api_Token":     "token",
		"AUTH0_CLI_X":         "x",
		"AWS_PROFILE":         "prod",
		"SSH_AUTH_SOCK":       "/tmp/agent",
		"AUTH0_PLUGIN_TENANT": "forged",
	}

	for _, name := range append(append([]string{}, baseNames...), windowsNames...) {
		env[name] = "value-of-" + name
	}

	return env
}

func TestBaseEnv(t *testing.T) {
	t.Parallel()

	common := []string{
		"PATH=value-of-PATH", "HOME=value-of-HOME", "LANG=value-of-LANG", "LC_ALL=value-of-LC_ALL",
		"LC_CTYPE=value-of-LC_CTYPE", "TERM=value-of-TERM", "TMPDIR=value-of-TMPDIR",
		"HTTP_PROXY=value-of-HTTP_PROXY", "HTTPS_PROXY=value-of-HTTPS_PROXY", "NO_PROXY=value-of-NO_PROXY",
		"http_proxy=value-of-http_proxy", "https_proxy=value-of-https_proxy", "no_proxy=value-of-no_proxy",
	}

	windows := []string{
		"SystemRoot=value-of-SystemRoot", "TEMP=value-of-TEMP", "TMP=value-of-TMP",
		"USERPROFILE=value-of-USERPROFILE", "APPDATA=value-of-APPDATA", "LOCALAPPDATA=value-of-LOCALAPPDATA",
		"PATHEXT=value-of-PATHEXT", "ComSpec=value-of-ComSpec",
	}

	tests := []struct {
		goos string
		want []string
	}{
		{"linux", common},
		{"darwin", common},
		{"windows", append(append([]string{}, common...), windows...)},
	}

	for _, test := range tests {
		t.Run(test.goos, func(t *testing.T) {
			t.Parallel()

			got := BaseEnv(lookupIn(userEnv()), test.goos)

			assert.ElementsMatch(t, test.want, got)

			for _, entry := range got {
				assert.NotContains(t, entry, "AUTH0_", "nothing of the CLI may reach the plugin")
				assert.NotContains(t, entry, "AWS_PROFILE")
				assert.NotContains(t, entry, "SSH_AUTH_SOCK")
			}
		})
	}

	t.Run("leaves out what is not set", func(t *testing.T) {
		t.Parallel()

		got := BaseEnv(lookupIn(map[string]string{"PATH": "/bin", "HOME": "", "AUTH0_CLIENT_SECRET": "s"}), "linux")

		assert.Equal(t, []string{"PATH=/bin"}, got)
	})

	t.Run("is empty for an empty environment", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, BaseEnv(lookupIn(nil), "windows"))
	})
}

func TestDeclaredEnv(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		names []string
		want  []string
	}{
		{"names that are set", []string{"AWS_PROFILE", "SSH_AUTH_SOCK"}, []string{"AWS_PROFILE=prod", "SSH_AUTH_SOCK=/tmp/agent"}},
		{"names that are not set are left out", []string{"AWS_PROFILE", "MISSING"}, []string{"AWS_PROFILE=prod"}},
		{"nothing declared", nil, nil},
		{"names that cannot be variables are left out", []string{"", "A=B", "AWS_PROFILE"}, []string{"AWS_PROFILE=prod"}},
		{"only what is declared passes", []string{"AWS_PROFILE"}, []string{"AWS_PROFILE=prod"}},
		{"AUTH0_CLIENT_SECRET is left out", []string{"AUTH0_CLIENT_SECRET", "AWS_PROFILE"}, []string{"AWS_PROFILE=prod"}},
		{"AUTH0_API_TOKEN is left out", []string{"AUTH0_API_TOKEN"}, nil},
		{"AUTH0_CLIENT_ID is left out", []string{"AUTH0_CLIENT_ID"}, nil},
		{"AUTH0_DOMAIN is left out", []string{"AUTH0_DOMAIN"}, nil},
		{"AUTH0_CLI_X is left out", []string{"AUTH0_CLI_X"}, nil},
		{"AUTH0_PLUGIN_TENANT is left out", []string{"AUTH0_PLUGIN_TENANT"}, nil},
		{"the prefix is read without regard to case", []string{"auth0_client_secret", "Auth0_Api_Token"}, nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := DeclaredEnv(test.names, lookupIn(userEnv()))

			assert.Equal(t, test.want, got)
		})
	}
}

func TestPluginVars(t *testing.T) {
	t.Parallel()

	t.Run("maps every owned flag", func(t *testing.T) {
		t.Parallel()

		got := PluginVars(map[string]string{
			"tenant":       "acme",
			"debug":        "false",
			"no-input":     "true",
			"no-color":     "false",
			"agent-mode":   "true",
			"json":         "true",
			"json-compact": "false",
			"csv":          "false",
		})

		assert.Equal(t, []string{
			"AUTH0_PLUGIN_AGENT_MODE=true",
			"AUTH0_PLUGIN_CSV=false",
			"AUTH0_PLUGIN_DEBUG=false",
			"AUTH0_PLUGIN_JSON=true",
			"AUTH0_PLUGIN_JSON_COMPACT=false",
			"AUTH0_PLUGIN_NO_COLOR=false",
			"AUTH0_PLUGIN_NO_INPUT=true",
			"AUTH0_PLUGIN_TENANT=acme",
		}, got)
	})

	t.Run("keeps a value as it is", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []string{"AUTH0_PLUGIN_TENANT=a b=c"}, PluginVars(map[string]string{"tenant": "a b=c"}))
	})

	t.Run("is empty without flags", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, PluginVars(nil))
	})
}

func TestEnv(t *testing.T) {
	t.Parallel()

	t.Run("combines the sources once per name and sorts them", func(t *testing.T) {
		t.Parallel()

		got := Env("linux", EnvSources{
			Base:     []string{"PATH=/bin", "HOME=/home/u"},
			Declared: []string{"AWS_PROFILE=prod"},
			Proxy:    []string{"AUTH0_PLUGIN_DOMAIN=127.0.0.1:1"},
			Plugin:   []string{"AUTH0_PLUGIN_TENANT=acme"},
		})

		assert.Equal(t, []string{
			"AUTH0_PLUGIN_DOMAIN=127.0.0.1:1",
			"AUTH0_PLUGIN_TENANT=acme",
			"AWS_PROFILE=prod",
			"HOME=/home/u",
			"PATH=/bin",
		}, got)
	})

	t.Run("a name that is declared and also in the base has one entry", func(t *testing.T) {
		t.Parallel()

		got := Env("linux", EnvSources{Base: []string{"PATH=/bin"}, Declared: []string{"PATH=/other"}})

		assert.Equal(t, []string{"PATH=/other"}, got)
	})

	t.Run("the variables of the CLI always win", func(t *testing.T) {
		t.Parallel()

		got := Env("linux", EnvSources{
			Base:     []string{"SSL_CERT_FILE=/base", "AUTH0_PLUGIN_TENANT=base"},
			Declared: []string{"SSL_CERT_FILE=/declared", "AUTH0_PLUGIN_TENANT=declared", "AUTH0_PLUGIN_MANAGEMENT_TOKEN=declared", "NODE_EXTRA_CA_CERTS=/declared"},
			Proxy:    []string{"AUTH0_PLUGIN_MANAGEMENT_TOKEN=proxy", "SSL_CERT_FILE=/proxy", "NODE_EXTRA_CA_CERTS=/proxy"},
			Plugin:   []string{"AUTH0_PLUGIN_TENANT=flag"},
		})

		assert.Equal(t, []string{
			"AUTH0_PLUGIN_MANAGEMENT_TOKEN=proxy",
			"AUTH0_PLUGIN_TENANT=flag",
			"NODE_EXTRA_CA_CERTS=/proxy",
			"SSL_CERT_FILE=/proxy",
		}, got)
	})

	t.Run("a declared name with the prefix of the CLI is dropped even when the CLI sets nothing by that name", func(t *testing.T) {
		t.Parallel()

		got := Env("linux", EnvSources{
			Base:     []string{"auth0_plugin_x=base"},
			Declared: []string{"AUTH0_PLUGIN_JSON=forged", "AUTH0_PLUGIN_ANYTHING=forged", "KEPT=yes"},
		})

		assert.Equal(t, []string{"KEPT=yes"}, got)
	})

	t.Run("a declared name that belongs to the CLI is dropped, whatever its case", func(t *testing.T) {
		t.Parallel()

		got := Env("linux", EnvSources{
			Base: []string{"auth0_client_id=base"},
			Declared: []string{
				"AUTH0_CLIENT_SECRET=s", "AUTH0_API_TOKEN=t", "AUTH0_CLIENT_ID=i", "AUTH0_DOMAIN=d",
				"AUTH0_CLI_X=x", "auth0_client_secret=s", "KEPT=yes",
			},
		})

		assert.Equal(t, []string{"KEPT=yes"}, got)
	})

	t.Run("names are compared without regard to case only on Windows", func(t *testing.T) {
		t.Parallel()

		sources := EnvSources{Base: []string{"Path=/base"}, Declared: []string{"PATH=/declared"}}

		assert.Equal(t, []string{"PATH=/declared"}, Env("windows", sources))
		assert.Equal(t, []string{"PATH=/declared", "Path=/base"}, Env("linux", sources))
	})

	t.Run("drops entries that have no name or no equals sign", func(t *testing.T) {
		t.Parallel()

		got := Env("linux", EnvSources{Base: []string{"NOEQUALS", "=x", "", "OK=1"}, Plugin: []string{"BROKEN"}})

		assert.Equal(t, []string{"OK=1"}, got)
	})

	t.Run("keeps an empty value and a value with an equals sign", func(t *testing.T) {
		t.Parallel()

		got := Env("linux", EnvSources{Declared: []string{"EMPTY=", "A=b=c"}})

		assert.Equal(t, []string{"A=b=c", "EMPTY="}, got)
	})

	t.Run("is empty without sources", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, Env("linux", EnvSources{}))
	})
}

func TestExemptLoopback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		goos string
		env  []string
		want []string
	}{
		{
			name: "sets both variables when the user has none",
			goos: "linux",
			env:  []string{"PATH=/bin"},
			want: []string{"NO_PROXY=127.0.0.1,localhost,::1", "PATH=/bin", "no_proxy=127.0.0.1,localhost,::1"},
		},
		{
			name: "adds to the value of the user",
			goos: "linux",
			env:  []string{"NO_PROXY=internal.example.com,10.0.0.0/8", "no_proxy=other.example.com"},
			want: []string{"NO_PROXY=internal.example.com,10.0.0.0/8,127.0.0.1,localhost,::1", "no_proxy=other.example.com,127.0.0.1,localhost,::1"},
		},
		{
			name: "adds only what is missing, in any case, once",
			goos: "linux",
			env:  []string{"NO_PROXY=LocalHost, ::1 ,a.com", "no_proxy=127.0.0.1,localhost,::1"},
			want: []string{"NO_PROXY=LocalHost, ::1 ,a.com,127.0.0.1", "no_proxy=127.0.0.1,localhost,::1"},
		},
		{
			name: "sets only the variable that is missing",
			goos: "linux",
			env:  []string{"NO_PROXY=a.com"},
			want: []string{"NO_PROXY=a.com,127.0.0.1,localhost,::1", "no_proxy=127.0.0.1,localhost,::1"},
		},
		{
			name: "does not double a trailing comma or an empty value",
			goos: "linux",
			env:  []string{"NO_PROXY=a.com,", "no_proxy="},
			want: []string{"NO_PROXY=a.com,127.0.0.1,localhost,::1", "no_proxy=127.0.0.1,localhost,::1"},
		},
		{
			name: "has one variable on Windows, whatever its case",
			goos: "windows",
			env:  []string{"No_Proxy=a.com"},
			want: []string{"No_Proxy=a.com,127.0.0.1,localhost,::1"},
		},
		{
			name: "sets one variable on Windows when the user has none",
			goos: "windows",
			want: []string{"NO_PROXY=127.0.0.1,localhost,::1"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			before := append([]string(nil), test.env...)

			assert.Equal(t, test.want, ExemptLoopback(test.env, test.goos))
			assert.Equal(t, before, test.env, "the argument is not changed")
		})
	}
}
