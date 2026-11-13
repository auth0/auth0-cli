package doctor

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	managementv3 "github.com/auth0/go-auth0/v3/management"

	"github.com/auth0/auth0-cli/internal/auth0/mock"
	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/poppy/setup"
)

// skippedIDs returns the IDs of the checks that were skipped with the message,
// and fails on any other result that is not a pass.
func skippedIDs(t *testing.T, results []Result, message string) []string {
	t.Helper()

	var ids []string

	for _, res := range results {
		switch res.Level {
		case Pass:
		case Skipped:
			assert.Equal(t, message, res.Message, res.ID)
			ids = append(ids, res.ID)
		default:
			t.Errorf("unexpected result %+v", res)
		}
	}

	return ids
}

func TestTenantChecksWithThePartsOff(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*fakeTenant)
		file    func(*config.Config)
		message string
		want    []string
		absent  []string
	}{
		{
			name:    "web.disabled in the file skips the web app checks",
			mutate:  func(f *fakeTenant) { f.state.App, f.joinApps = setup.App{}, nil },
			file:    func(c *config.Config) { c.Web = config.Web{Disabled: true} },
			message: messageWebOff,
			want:    []string{"D9"},
			absent:  []string{"Shop Web is first-party", "allow_personal_agent_browser_join"},
		},
		{
			name: "auth.disabled in the file skips the API checks",
			mutate: func(f *fakeTenant) {
				f.state.APIs, f.poppyAPIs, f.allAPIs, f.settings.DefaultAudience = nil, nil, nil, ""
			},
			file:    func(c *config.Config) { c.Auth = config.Auth{Disabled: true, DisabledSet: true} },
			message: messageAPIOff,
			want:    []string{"D8", "D11", "D12", "D13"},
			absent:  []string{"use_for_personal_agent_protocol", "the default audience", "granted every Poppy API"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := healthyTenant()
			test.mutate(f)

			results := runTenant(t, f, goldenConfigFile(t, test.file))

			assert.Equal(t, test.want, skippedIDs(t, results, test.message))

			for _, text := range test.absent {
				assert.False(t, slices.ContainsFunc(results, func(res Result) bool { return strings.Contains(res.Message, text) }), "%q in %+v", text, results)
			}

			for _, res := range results {
				if res.ID == "D8" && res.Level == Pass {
					assert.Contains(t, []string{"4", "3.2"}, res.Spec)
				}
			}
		})
	}

	t.Run("the tenant checks that are not about the part still run", func(t *testing.T) {
		f := healthyTenant()
		f.settings.CIMD = false
		f.state.APIs, f.poppyAPIs, f.allAPIs, f.settings.DefaultAudience = nil, nil, nil, ""

		results := runTenant(t, f, goldenConfigFile(t, func(c *config.Config) { c.Auth = config.Auth{Disabled: true, DisabledSet: true} }))

		assert.True(t, slices.ContainsFunc(results, func(res Result) bool {
			return res.ID == "D8" && res.Level == Fail && strings.Contains(res.Message, "client_id_metadata_document_supported is off")
		}), "%+v", results)
	})

	t.Run("a file that does not turn a part off leaves every check on", func(t *testing.T) {
		results := runTenant(t, healthyTenant(), goldenConfigFile(t, nil))

		assert.Empty(t, skippedIDs(t, results, ""))
	})

	t.Run("a file that disagrees with the tenant still fails D14", func(t *testing.T) {
		f := healthyTenant()
		f.state.App, f.joinApps = setup.App{}, nil

		results := runTenant(t, f, goldenConfigFile(t, func(c *config.Config) {
			c.Web = config.Web{Disabled: true}
			c.Organization.Name = "Another shop"
		}))

		assert.True(t, slices.ContainsFunc(results, func(res Result) bool { return res.ID == "D14" && res.Level != Pass }), "%+v", results)
	})
}

func TestRunWithThePartsOffInTheConfigFile(t *testing.T) {
	tests := []struct {
		name    string
		file    func(*config.Config)
		mutate  func(*fakeTenant)
		site    func(*site)
		message string
		want    []string
	}{
		{
			name:    "web.disabled skips D6, D9 and the browser sign-in rules of D16",
			file:    func(c *config.Config) { c.Web = config.Web{Disabled: true} },
			mutate:  func(f *fakeTenant) { f.state.App, f.joinApps = setup.App{}, nil },
			site:    func(s *site) { s.poppy["web"] = map[string]any{} },
			message: messageWebOff,
			want:    []string{"D6", "D16", "D9"},
		},
		{
			name: "auth.disabled skips D7 and the API checks of the tenant",
			file: func(c *config.Config) { c.Auth = config.Auth{Disabled: true, DisabledSet: true} },
			mutate: func(f *fakeTenant) {
				f.state.APIs, f.poppyAPIs, f.allAPIs, f.settings.DefaultAudience = nil, nil, nil, ""
			},
			message: messageAPIOff,
			want:    []string{"D7", "D8", "D11", "D12", "D13"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policies := mock.NewMockGuardianPolicyAPIV3(gomock.NewController(t))
			policies.EXPECT().List(gomock.Any()).Return(managementv3.ListGuardianPoliciesResponseContent{}, nil)

			f := healthyTenant()
			test.mutate(f)

			opts := Options{Tenant: &Tenant{State: f, Agents: f, Policies: policies}, ConfigFile: goldenConfigFile(t, test.file)}

			s := validSite()
			if test.site != nil {
				test.site(s)
			}

			report := s.runWith(t, opts)

			var skipped []string
			for _, res := range report.Results {
				switch res.Level {
				case Pass:
				case Skipped:
					assert.Equal(t, test.message, res.Message, res.ID)

					if !slices.Contains(skipped, res.ID) {
						skipped = append(skipped, res.ID)
					}
				default:
					t.Errorf("unexpected result %+v", res)
				}
			}

			assert.ElementsMatch(t, test.want, skipped)
			assert.Zero(t, report.ExitCode())
		})
	}

	t.Run("a join address the site still publishes is still checked", func(t *testing.T) {
		policies := mock.NewMockGuardianPolicyAPIV3(gomock.NewController(t))
		policies.EXPECT().List(gomock.Any()).Return(managementv3.ListGuardianPoliciesResponseContent{}, nil)

		f := healthyTenant()
		f.state.App, f.joinApps = setup.App{}, nil

		report := validSite().runWith(t, Options{
			Tenant:     &Tenant{State: f, Agents: f, Policies: policies},
			ConfigFile: goldenConfigFile(t, func(c *config.Config) { c.Web = config.Web{Disabled: true} }),
		})

		d6 := slices.IndexFunc(report.Results, func(res Result) bool { return res.ID == "D6" })
		require.GreaterOrEqual(t, d6, 0)
		assert.Equal(t, Pass, report.Results[d6].Level)
	})

	t.Run("without a config file nothing is skipped", func(t *testing.T) {
		report := validSite().run(t)

		assert.Zero(t, report.Count(Skipped))
	})

	t.Run("a config file that cannot be read skips nothing", func(t *testing.T) {
		report := validSite().runWith(t, Options{ConfigFile: writeFile(t, "{")})

		require.Zero(t, report.Count(Fail), "%+v", nonPass(report))
		assert.NotContains(t, skippedIDs(t, filterNoLogin(report.Results), messageWebOff), "D6")
	})
}

func TestD1WithAuthDisabled(t *testing.T) {
	policies := func(t *testing.T) *mock.MockGuardianPolicyAPIV3 {
		p := mock.NewMockGuardianPolicyAPIV3(gomock.NewController(t))
		p.EXPECT().List(gomock.Any()).Return(managementv3.ListGuardianPoliciesResponseContent{}, nil).AnyTimes()

		return p
	}

	t.Run("the config file also says so, so it is a pass", func(t *testing.T) {
		f := healthyTenant()
		f.state.APIs, f.poppyAPIs, f.allAPIs, f.settings.DefaultAudience = nil, nil, nil, ""

		s := validSite()
		obj(s.poppy, "auth")["disabled"] = true

		report := s.runWith(t, Options{
			Tenant:     &Tenant{State: f, Agents: f, Policies: policies(t)},
			ConfigFile: goldenConfigFile(t, func(c *config.Config) { c.Auth = config.Auth{Disabled: true, DisabledSet: true} }),
		})

		assert.Zero(t, report.Count(Warn), "%+v", nonPass(report))
		assert.True(t, slices.ContainsFunc(report.Results, func(res Result) bool {
			return res.ID == "D1" && res.Level == Pass && res.Message == "API access is turned off (auth.disabled is true)"
		}), "%+v", report.Results)
		assert.True(t, slices.ContainsFunc(report.Results, func(res Result) bool { return res.ID == "D7" && res.Level == Skipped }))
	})

	t.Run("only the live file says so, so it is a warning", func(t *testing.T) {
		s := validSite()
		obj(s.poppy, "auth")["disabled"] = true

		report := s.run(t)

		assert.True(t, slices.ContainsFunc(report.Results, func(res Result) bool {
			return res.ID == "D1" && res.Level == Warn && strings.Contains(res.Message, "agent sign-in is disabled")
		}), "%+v", report.Results)
	})
}

// filterNoLogin drops the results of the tenant checks that ran without a login.
func filterNoLogin(results []Result) []Result {
	return slices.DeleteFunc(slices.Clone(results), func(res Result) bool { return res.Level == Skipped && res.Message == noLogin })
}
