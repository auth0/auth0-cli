package doctor

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func reportOf(levels ...Level) Report {
	r := Report{Domain: "shop.example"}
	for _, l := range levels {
		r.Results = append(r.Results, Result{Level: l})
	}
	return r
}

func TestReportCountAndExitCode(t *testing.T) {
	tests := []struct {
		name     string
		report   Report
		counts   map[Level]int
		exitCode int
	}{
		{
			name:     "all pass",
			report:   reportOf(Pass, Pass, Pass),
			counts:   map[Level]int{Pass: 3},
			exitCode: 0,
		},
		{
			name:     "warn only",
			report:   reportOf(Pass, Warn, Warn),
			counts:   map[Level]int{Pass: 1, Warn: 2},
			exitCode: 0,
		},
		{
			name:     "one fail",
			report:   reportOf(Pass, Warn, Fail, Skipped),
			counts:   map[Level]int{Pass: 1, Warn: 1, Fail: 1, Skipped: 1},
			exitCode: 1,
		},
		{
			name:     "skipped only",
			report:   reportOf(Skipped, Skipped),
			counts:   map[Level]int{Skipped: 2},
			exitCode: 0,
		},
		{
			name:     "empty",
			report:   Report{},
			counts:   map[Level]int{},
			exitCode: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, l := range []Level{Pass, Warn, Fail, Skipped} {
				assert.Equal(t, test.counts[l], test.report.Count(l), l)
			}
			assert.Equal(t, test.exitCode, test.report.ExitCode())
		})
	}
}

func TestReportJSON(t *testing.T) {
	tests := []struct {
		name   string
		report Report
		want   string
	}{
		{
			name: "optional fields are omitted when empty",
			report: Report{
				Domain:  "shop.example",
				Results: []Result{{ID: "D1", Code: "D1.served", Spec: "3", Level: Pass, Message: "served"}},
			},
			want: `{"domain":"shop.example","tenant_checked":false,"summary":{"pass":0,"warn":0,"fail":0,"skipped":0},"results":[{"id":"D1","code":"D1.served","spec":"3","level":"pass","message":"served"}]}`,
		},
		{
			name: "optional fields are present when set",
			report: Report{
				Domain:        "shop.example",
				TenantChecked: true,
				TenantOnly:    true,
				Totals:        Counts{Warn: 1, Skipped: 1},
				Results: []Result{
					{
						ID:                 "D4",
						Code:               "D4.ps384_missing",
						Spec:               "4.1",
						Level:              Warn,
						Message:            "PS384 not accepted",
						FixHint:            "Contact support",
						PlatformDependency: true,
					},
					{
						ID:         "D10",
						Code:       "D10.missing_scope",
						Level:      Skipped,
						SkipReason: SkipMissingScope,
						Message:    "cannot check the MFA policies",
						FixCommand: "auth0 login --scopes read:mfa_policies",
					},
				},
			},
			want: `{"domain":"shop.example","tenant_checked":true,"tenant_only":true,"summary":{"pass":0,"warn":1,"fail":0,"skipped":1},"results":[` +
				`{"id":"D4","code":"D4.ps384_missing","spec":"4.1","level":"warn","message":"PS384 not accepted","fix_hint":"Contact support","platform_dependency":true},` +
				`{"id":"D10","code":"D10.missing_scope","spec":"","level":"skipped","skip_reason":"missing_scope","message":"cannot check the MFA policies","fix_command":"auth0 login --scopes read:mfa_policies"}]}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := json.Marshal(test.report)
			require.NoError(t, err)
			assert.Equal(t, test.want, string(got))
		})
	}
}

func TestSummary(t *testing.T) {
	tests := []struct {
		name    string
		results []Result
		want    string
	}{
		{name: "empty", want: "0 checks ran."},
		{name: "all passed", results: []Result{{Level: Pass}, {Level: Pass}}, want: "2 passed."},
		{name: "zero passed is still shown", results: []Result{{Level: Fail}}, want: "1 failed, 0 passed."},
		{name: "plural warnings", results: []Result{{Level: Warn}, {Level: Warn}}, want: "2 warnings, 0 passed."},
		{
			name:    "every count",
			results: []Result{{Level: Fail}, {Level: Fail}, {Level: Warn}, {Level: Pass}, {Level: Skipped}},
			want:    "2 failed, 1 warning, 1 passed, 1 skipped.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, Report{Results: test.results}.Summary())
		})
	}
}
