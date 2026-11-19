package doctor

import (
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	managementv3 "github.com/auth0/go-auth0/v3/management"

	coreagents "github.com/auth0/auth0-cli/internal/agents"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
)

var (
	allSiteChecks   = []string{"D1", "D2", "D3", "D4", "D5", "D6", "D7", "D15", "D16", "D17"}
	allTenantChecks = []string{"D8", "D9", "D10", "D11", "D12", "D13", "D14"}
)

// assertStableReport checks what a script can rely on in any report.
func assertStableReport(t *testing.T, report Report, ids []string) {
	t.Helper()

	var seen []string

	for _, res := range report.Results {
		if len(seen) == 0 || seen[len(seen)-1] != res.ID {
			seen = append(seen, res.ID)
		}

		assert.True(t, strings.HasPrefix(res.Code, res.ID+"."), "code %q of %s", res.Code, res.ID)

		if res.Level == Skipped {
			assert.NotEmpty(t, res.SkipReason, "%s: %s", res.Code, res.Message)
		} else {
			assert.Empty(t, res.SkipReason, "%s: %s", res.Code, res.Message)
		}

		if res.Level == Fail {
			assert.NotEmpty(t, res.FixCommand+res.FixHint, "a failure has a fix: %s", res.Message)
		}

		assert.NotContains(t, res.FixCommand, "<", "a command can be run as it is: %s", res.FixCommand)
	}

	assert.Equal(t, ids, seen, "every check appears, in order")
	assert.Equal(t, Counts{
		Pass: report.Count(Pass), Warn: report.Count(Warn), Fail: report.Count(Fail), Skipped: report.Count(Skipped),
	}, report.Totals)
}

func TestRunReportsEveryCheck(t *testing.T) {
	policies := func(t *testing.T) *mock.MockGuardianPolicyAPIV3 {
		p := mock.NewMockGuardianPolicyAPIV3(gomock.NewController(t))
		p.EXPECT().List(gomock.Any()).Return(managementv3.ListGuardianPoliciesResponseContent{}, nil).AnyTimes()

		return p
	}

	tests := []struct {
		name        string
		site        func() *site
		opts        func(t *testing.T) Options
		wantChecked bool
		// Skipped maps the check to the reason it did not run.
		skipped map[string]SkipReason
	}{
		{
			name: "a 0.4 site and no login",
			site: validSite,
			skipped: map[string]SkipReason{
				"D8": SkipNoLogin, "D9": SkipNoLogin, "D10": SkipNoLogin, "D11": SkipNoLogin, "D12": SkipNoLogin, "D13": SkipNoLogin, "D14": SkipNoLogin,
			},
		},
		{
			name: "a 0.3 site and no login",
			site: valid03Site,
			skipped: map[string]SkipReason{
				"D15": SkipProtocolVersion, "D16": SkipProtocolVersion, "D17": SkipProtocolVersion,
				"D8": SkipNoLogin, "D9": SkipNoLogin, "D10": SkipNoLogin, "D11": SkipNoLogin, "D12": SkipNoLogin, "D13": SkipNoLogin, "D14": SkipNoLogin,
			},
		},
		{
			name: "a site without poppy.json",
			site: func() *site {
				s := validSite()
				s.routes[testDomain+"/.well-known/poppy.json"] = func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }

				return s
			},
			skipped: map[string]SkipReason{
				"D3": SkipNotAvailable, "D4": SkipNotAvailable, "D5": SkipNotAvailable, "D6": SkipNotAvailable, "D7": SkipNotAvailable,
				"D15": SkipNotAvailable, "D16": SkipNotAvailable, "D17": SkipNotAvailable,
				"D8": SkipNoLogin, "D9": SkipNoLogin, "D10": SkipNoLogin, "D11": SkipNoLogin, "D12": SkipNoLogin, "D13": SkipNoLogin, "D14": SkipNoLogin,
			},
		},
		{
			name: "a healthy site and tenant",
			site: validSite,
			opts: func(t *testing.T) Options {
				f := healthyTenant()

				return Options{Tenant: &Tenant{State: f, Agents: f, Policies: policies(t)}, ConfigFile: goldenConfigFile(t, nil)}
			},
			wantChecked: true,
			skipped:     map[string]SkipReason{},
		},
		{
			name: "a tenant that does not serve the domain",
			site: validSite,
			opts: func(t *testing.T) Options {
				f := healthyTenant()
				f.hosts = []string{"tenant.us.auth0.com"}
				f.settings.CompanyDomains = nil

				return Options{Tenant: &Tenant{Domain: "tenant.us.auth0.com", State: f, Agents: f, Policies: policies(t)}, ConfigFile: goldenConfigFile(t, nil)}
			},
			skipped: map[string]SkipReason{
				"D8": SkipDifferentTenant, "D9": SkipDifferentTenant, "D10": SkipDifferentTenant, "D11": SkipDifferentTenant,
				"D12": SkipDifferentTenant, "D13": SkipDifferentTenant, "D14": SkipDifferentTenant,
			},
		},
		{
			name: "a tenant and no config file compares the live poppy.json",
			site: validSite,
			opts: func(t *testing.T) Options {
				f := healthyTenant()

				return Options{Tenant: &Tenant{State: f, Agents: f, Policies: policies(t)}, ConfigFile: filepath.Join(t.TempDir(), "missing.json")}
			},
			wantChecked: true,
			skipped:     map[string]SkipReason{},
		},
		{
			name: "a tenant and a config file that was named but is missing",
			site: validSite,
			opts: func(t *testing.T) Options {
				f := healthyTenant()

				return Options{Tenant: &Tenant{State: f, Agents: f, Policies: policies(t)}, ConfigFile: filepath.Join(t.TempDir(), "missing.json"), ConfigFileExplicit: true}
			},
			wantChecked: true,
			skipped:     map[string]SkipReason{"D14": SkipNoConfigFile},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var opts Options
			if test.opts != nil {
				opts = test.opts(t)
			}

			report := test.site().runWith(t, opts)

			assertStableReport(t, report, slices.Concat(allSiteChecks, allTenantChecks))
			assert.Equal(t, test.wantChecked, report.TenantChecked)

			got := map[string]SkipReason{}
			for _, res := range report.Results {
				if res.Level == Skipped {
					got[res.ID] = res.SkipReason
				}
			}
			assert.Equal(t, test.skipped, got)
		})
	}
}

func TestRunTenantOnlyChecksOnlyTheTenant(t *testing.T) {
	policies := mock.NewMockGuardianPolicyAPIV3(gomock.NewController(t))
	policies.EXPECT().List(gomock.Any()).Return(managementv3.ListGuardianPoliciesResponseContent{}, nil)

	f := healthyTenant()
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Errorf("the website was fetched: %s", r.URL)

		return nil, assert.AnError
	})}

	report := Run(t.Context(), testDomain, Options{
		HTTPClient: client,
		Tenant:     &Tenant{State: f, Agents: f, Policies: policies},
		ConfigFile: goldenConfigFile(t, nil),
		TenantOnly: true,
	})

	assertStableReport(t, report, allTenantChecks)
	assert.True(t, report.TenantOnly)
	assert.True(t, report.TenantChecked)
	assert.Empty(t, report.ProtocolVersion)
	assert.Zero(t, report.Count(Skipped), "%+v", nonPass(report))
	assert.Zero(t, report.ExitCode())
}

func TestRunTenantOnlyWithoutLogin(t *testing.T) {
	report := Run(t.Context(), testDomain, Options{TenantOnly: true})

	assertStableReport(t, report, allTenantChecks)
	assert.False(t, report.TenantChecked)
	assert.Equal(t, 7, report.Totals.Skipped)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRunSkipsTheChecksThatDoNotApply(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(s *site)
		want   map[string]SkipReason
	}{
		{
			name: "web.disabled",
			mutate: func(s *site) {
				obj(s.poppy, "web")["disabled"] = true
				delete(obj(s.poppy, "web"), "browser_session_endpoint")
			},
			want: map[string]SkipReason{"D16": SkipWebOff},
		},
		{
			name:   "auth.disabled",
			mutate: func(s *site) { obj(s.poppy, "auth")["disabled"] = true },
			want:   map[string]SkipReason{"D7": SkipAPIOff, "D16": SkipAPIOff},
		},
		{
			name:   "no browser session endpoint",
			mutate: func(s *site) { delete(obj(s.poppy, "web"), "browser_session_endpoint") },
			want:   map[string]SkipReason{"D16": SkipNotAvailable},
		},
		{
			name:   "no API",
			mutate: func(s *site) { delete(s.poppy, "apis") },
			want:   map[string]SkipReason{"D7": SkipNotAvailable},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := validSite()
			test.mutate(s)

			report := s.run(t)

			got := map[string]SkipReason{}
			for _, res := range report.Results {
				if res.Level == Skipped {
					got[res.ID] = res.SkipReason
				}
			}
			assert.Equal(t, test.want, got)
		})
	}
}

func TestStrictExitCode(t *testing.T) {
	tests := []struct {
		name    string
		results []Result
		want    int
		strict  int
	}{
		{name: "all passed", results: []Result{{Level: Pass}}},
		{name: "a warning", results: []Result{{Level: Warn}, {Level: Pass}}},
		{name: "a skipped check", results: []Result{{Level: Pass}, {Level: Skipped}}, strict: 1},
		{name: "a failure", results: []Result{{Level: Fail}}, want: 1, strict: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report := Report{Results: test.results}

			assert.Equal(t, test.want, report.ExitCode())
			assert.Equal(t, test.strict, report.StrictExitCode())
		})
	}
}

func TestFixCommands(t *testing.T) {
	tests := []struct {
		name  string
		r     runner
		check func(r *runner) string
		want  string
	}{
		{
			name:  "setup",
			r:     runner{domain: "shop.example", configFile: "poppy.json"},
			check: func(r *runner) string { return r.setupCommand() },
			want:  "auth0 poppy setup --company-domain shop.example --force",
		},
		{
			name:  "setup with an API, a config file and a tenant",
			r:     runner{domain: "shop.example", configFile: "./config/poppy.json", tenantFlag: "dev.us.auth0.com"},
			check: func(r *runner) string { return r.setupCommand("--api", quoteArg("https://api.shop.example")) },
			want:  "auth0 poppy setup --company-domain shop.example --api https://api.shop.example --out ./config/poppy.json --force --tenant dev.us.auth0.com",
		},
		{
			name:  "a config file with a space",
			r:     runner{domain: "shop.example", configFile: "my config/poppy.json"},
			check: func(r *runner) string { return r.configCommand() },
			want:  "auth0 poppy config --company-domain shop.example --out 'my config/poppy.json'",
		},
		{
			name:  "a quote in a word",
			r:     runner{domain: "shop.example", configFile: "it's.json"},
			check: func(r *runner) string { return r.configCommand() },
			want:  `auth0 poppy config --company-domain shop.example --out 'it'\''s.json'`,
		},
		{
			name:  "no config file name",
			r:     runner{domain: "shop.example"},
			check: func(r *runner) string { return r.configCommand() },
			want:  "auth0 poppy config --company-domain shop.example",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, test.check(&test.r))
		})
	}
}

func TestFixCommandsOfTheTenantChecks(t *testing.T) {
	f := healthyTenant()
	f.settings.CIMD = false
	f.apis = append(f.apis, coreagents.API{ID: "rs_other", Identifier: "https://api.other.example"})
	f.settings.DefaultAudience = ""
	path := goldenConfigFile(t, nil)

	policies := mock.NewMockGuardianPolicyAPIV3(gomock.NewController(t))
	policies.EXPECT().List(gomock.Any()).Return(managementv3.ListGuardianPoliciesResponseContent{}, nil)

	report := Run(t.Context(), testDomain, Options{
		Tenant:     &Tenant{State: f, Agents: f, Policies: policies},
		ConfigFile: path,
		TenantFlag: "dev.us.auth0.com",
		TenantOnly: true,
	})

	byCode := map[string]Result{}
	for _, res := range report.Results {
		byCode[res.Code] = res
	}

	assert.Equal(t, "auth0 poppy setup --company-domain shop.example --out "+quoteArg(path)+" --force --tenant dev.us.auth0.com", byCode["D8.cimd_off"].FixCommand)
	assert.Equal(t, "auth0 agents register "+museURL+" --api https://api.other.example --tenant dev.us.auth0.com", byCode["D13.partially_granted"].FixCommand)
	assert.Empty(t, byCode["D13.partially_granted"].FixHint)
}

func TestWarningsAndConfigFile(t *testing.T) {
	// A config file the doctor cannot read is a warning with the command that writes it again.
	f := healthyTenant()
	policies := mock.NewMockGuardianPolicyAPIV3(gomock.NewController(t))
	policies.EXPECT().List(gomock.Any()).Return(managementv3.ListGuardianPoliciesResponseContent{}, nil)

	path := writeFile(t, "{")
	report := Run(t.Context(), testDomain, Options{
		Tenant:     &Tenant{State: f, Agents: f, Policies: policies},
		ConfigFile: path,
		TenantOnly: true,
	})

	i := slices.IndexFunc(report.Results, func(res Result) bool { return res.Code == "D14.unreadable_file" })
	require.GreaterOrEqual(t, i, 0, "%+v", report.Results)
	assert.Equal(t, Warn, report.Results[i].Level)
	assert.Equal(t, "auth0 poppy config --company-domain shop.example --out "+quoteArg(path), report.Results[i].FixCommand)
}

func TestScopesOfTheConfigFileCheck(t *testing.T) {
	assert.NotContains(t, scopeConfigFile, "read:client_grants", "the file is derived without reading grants")
	assert.Subset(t, scopeConfigFile, []string{"read:tenant_settings", "read:custom_domains", "read:resource_servers", "read:clients"})
}
