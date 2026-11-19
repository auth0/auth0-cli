package display

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/poppy/doctor"
)

func TestPoppyDoctorTextGolden(t *testing.T) {
	green := greenReport(t)

	long := shopReport(
		doctor.Result{ID: "D1", Spec: "3", Level: doctor.Pass, Message: "poppy.json is served over HTTPS at /.well-known/poppy.json"},
		doctor.Result{ID: "D7", Spec: "7", Level: doctor.Fail, Message: "https://api.shop.example/poppy/openapi.json returned HTTP 503, and the document must be served with status 200 so that agents can read it", FixHint: "Serve the document at https://api.shop.example/poppy/openapi.json with status 200."},
		doctor.Result{ID: "D6", Spec: "6", Level: doctor.Warn, Message: "web.browser_session_endpoint is not published, so agents cannot join a browser session"},
		tenantSkipped,
	)
	long.Domain, long.Issuer = "a-very-long-company-domain.example.com", "https://login.a-very-long-company-domain.example.com/"

	tests := []struct {
		name   string
		report doctor.Report
		layout Layout
	}{
		{name: "report-green", report: green, layout: unicode80},
		{name: "report-fail", report: shopReport(issuerPass, protectedFail, ps384Fail, tenantSkipped), layout: unicode80},
		{name: "report-warn", report: shopReport(issuerPass, hintWarn), layout: unicode80},
		{name: "report-skipped-tenant", report: shopReport(issuerPass, tenantSkipped), layout: unicode80},
		{
			name: "report-other",
			report: shopReport(issuerPass,
				doctor.Result{ID: "D99", Level: doctor.Pass, Message: "a check this CLI does not know"},
				doctor.Result{Level: doctor.Warn, Message: "a result without an ID"}),
			layout: unicode80,
		},
		{name: "report-no-header-details", report: doctor.Report{Domain: "shop.example", Results: []doctor.Result{issuerPass}}, layout: unicode80},
		{name: "report-ascii", report: shopReport(issuerPass, protectedFail, tenantSkipped), layout: ascii80},
		{name: "report-width-60", report: long, layout: Layout{Width: 60, Unicode: true}},
		{name: "report-width-100", report: long, layout: Layout{Width: 100, Unicode: true}},
		{
			name: "report-escapes",
			report: shopReport(doctor.Result{
				ID: "D5", Level: doctor.Fail,
				Message: "scope \x1b[2J\x1b[31mred\nforged line",
				FixHint: "run \x1b]0;title\x07\nthen \r check",
			}),
			layout: unicode80,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertGolden(t, test.name, renderWith(t, test.report, test.layout))
		})
	}
}

func TestPoppyDoctorTextLines(t *testing.T) {
	tests := []struct {
		name   string
		report doctor.Report
		layout Layout
		want   string
	}{
		{
			name:   "an empty report",
			report: doctor.Report{Domain: "shop.example"},
			layout: unicode80,
			want:   "Doctor for shop.example\n\n0 checks ran.\n",
		},
		{
			name:   "the protocol version sits at the right edge",
			report: doctor.Report{Domain: "shop.example", ProtocolVersion: "1.0", Results: []doctor.Result{{ID: "D1", Level: doctor.Pass, Message: "ok"}}},
			layout: Layout{Width: 40, Unicode: true},
			want: "Doctor for shop.example     protocol 1.0\n\n" +
				"Website files\n  ✓ ok\n\n1 passed.\n",
		},
		{
			name:   "a header wider than the terminal keeps one space",
			report: doctor.Report{Domain: "shop.example", ProtocolVersion: "1.0"},
			layout: Layout{Width: 10, Unicode: true},
			want:   "Doctor for shop.example protocol 1.0\n\n0 checks ran.\n",
		},
		{
			name:   "groups come in a fixed order whatever the result order",
			report: doctor.Report{Domain: "d", Results: []doctor.Result{{ID: "D8", Level: doctor.Pass, Message: "t"}, {ID: "D1", Level: doctor.Pass, Message: "w"}}},
			layout: unicode80,
			want:   "Doctor for d\n\nWebsite files\n  ✓ w\n\nTenant\n  ✓ t\n\n2 passed.\n",
		},
		{
			name: "a fix sits under the message of any level",
			report: doctor.Report{Domain: "d", Results: []doctor.Result{
				{ID: "D8", Level: doctor.Skipped, Message: "cannot check the web apps", FixCommand: "auth0 login --scopes read:clients"},
				{ID: "D4", Level: doctor.Fail, Message: "bad", FixHint: "one\ntwo", PlatformDependency: true},
			}},
			layout: unicode80,
			want: "Doctor for d\n\n" +
				"Auth0 issuer\n  FAIL D4 bad\n       FIX  one\n            two (Auth0 must fix this)\n\n" +
				"Tenant\n  - skipped: cannot check the web apps\n    FIX  auth0 login --scopes read:clients\n\n" +
				"1 failed, 0 passed, 1 skipped.\n",
		},
		{
			name:   "ASCII fallback for the pass mark",
			report: doctor.Report{Domain: "d", Results: []doctor.Result{{ID: "D1", Level: doctor.Pass, Message: "fine"}}},
			layout: ascii80,
			want:   "Doctor for d\n\nWebsite files\n  ok fine\n\n1 passed.\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, renderWith(t, test.report, test.layout))
		})
	}
}

func TestPoppyDoctorTextCollapsesRepeatedSkips(t *testing.T) {
	const reason = "no usable tenant login; run auth0 login to check the tenant"

	report := doctor.Report{Domain: "d", Results: []doctor.Result{
		{ID: "D8", Level: doctor.Skipped, Message: reason},
		{ID: "D9", Level: doctor.Skipped, Message: reason},
		{ID: "D10", Level: doctor.Pass, Message: "fine"},
		{ID: "D11", Level: doctor.Skipped, Message: reason},
		{ID: "D12", Level: doctor.Skipped, Message: reason},
		{ID: "D13", Level: doctor.Skipped, Message: reason},
		{ID: "D14", Level: doctor.Skipped, Message: "another reason"},
	}}

	want := "Doctor for d\n\nTenant\n" +
		"  - skipped (2 checks): " + reason + "\n" +
		"  ✓ fine\n" +
		"  - skipped (3 checks): " + reason + "\n" +
		"  - skipped: another reason\n\n" +
		"1 passed, 6 skipped.\n"

	assert.Equal(t, want, renderWith(t, report, Layout{Unicode: true}), "the summary still counts every check")
	assert.Equal(t, 6, report.Count(doctor.Skipped))
}

func TestPoppyDoctorTextDoesNotWrapWithoutAWidth(t *testing.T) {
	const full = "checkout is in neither auth.scopes nor the scopes_supported of the issuer, and the signed-in browser join needs it"

	report := doctor.Report{Domain: "d", Results: []doctor.Result{{ID: "D5", Level: doctor.Warn, Message: full}}}

	assert.Contains(t, renderWith(t, report, Layout{Unicode: true}), "  WARN D5 "+full+"\n", "a pipe sees the whole line")
	assert.NotContains(t, renderWith(t, report, Layout{Width: 60, Unicode: true}), "  WARN D5 "+full+"\n", "a terminal wraps it")
}

func TestPoppyDoctorProblems(t *testing.T) {
	report := shopReport(issuerPass, protectedFail, hintWarn, tenantSkipped,
		doctor.Result{ID: "D1", Level: doctor.Pass, Message: "poppy.json is served"})

	tests := []struct {
		name   string
		layout Layout
		prefix string
		want   string
	}{
		{
			name:   "only warnings and failures, in their groups, with fixes",
			layout: unicode80,
			prefix: "  ",
			want: "  Auth0 issuer\n" +
				"    FAIL D3 (spec 3.2) protected_resources does not list https://shop.example\n" +
				"         FIX  auth0 poppy setup --company-domain shop.example --force\n" +
				"\n" +
				"  Scopes\n" +
				"    WARN D5 (spec 4.7) scope_hints missing for checkout\n" +
				"         FIX  auth0 poppy setup --hint\n" +
				"              checkout=changes:<none|reversible|irreversible>,personal_data:<tru\n" +
				"              e|false>\n",
		},
		{
			name:   "the prefix narrows the wrapping",
			layout: Layout{Width: 30, Unicode: true},
			prefix: "    ",
			want: "    Auth0 issuer\n" +
				"      FAIL D3 (spec 3.2)\n" +
				"           protected_resources\n" +
				"           does not list\n" +
				"           https://shop.example\n" +
				"           FIX  auth0 poppy setup\n" +
				"                --company-domain\n" +
				"                shop.example --force\n" +
				"\n" +
				"    Scopes\n" +
				"      WARN D5 (spec 4.7)\n" +
				"           scope_hints missing\n" +
				"           for checkout\n" +
				"           FIX  auth0 poppy setup\n" +
				"                --hint\n" +
				"                checkout=changes:<no\n" +
				"                ne|reversible|irreve\n" +
				"                rsible>,personal_dat\n" +
				"                a:<true|false>\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, doctorProblems(test.layout, report, test.prefix))
		})
	}

	assert.Empty(t, doctorProblems(unicode80, shopReport(issuerPass, tenantSkipped), "  "), "nothing to say when nothing warned or failed")
}

func TestPoppyDoctorTextWrapsUnderTheMessage(t *testing.T) {
	report := doctor.Report{Domain: "d", Results: []doctor.Result{
		{ID: "D1", Level: doctor.Pass, Message: strings.Repeat("word ", 30)},
		{ID: "D1", Level: doctor.Fail, Message: strings.Repeat("x", 100)},
	}}

	for _, width := range []int{40, 60, 100} {
		out := renderWith(t, report, Layout{Width: width, Unicode: true})

		for _, line := range strings.Split(out, "\n") {
			assert.LessOrEqual(t, utf8.RuneCountInString(line), width, "line %q at width %d", line, width)
		}
	}

	out := renderWith(t, report, Layout{Width: 40, Unicode: true})
	assert.Contains(t, out, "  ✓ word word word word word word word\n    word word", "continuation lines sit under the message")
	assert.Contains(t, out, "  FAIL D1\n       "+strings.Repeat("x", 33)+"\n       "+strings.Repeat("x", 33)+"\n", "a word longer than a line is cut")
}

func TestPoppyDoctorTextCapsAHugeMessage(t *testing.T) {
	huge := "scope " + strings.Repeat("x", 1<<20)

	out := render(t, doctor.Report{Domain: "d", Results: []doctor.Result{{ID: "D5", Level: doctor.Fail, Message: huge}, {ID: "D5", Level: doctor.Pass, Message: "ok"}}})

	assert.Less(t, len(out), 1000)
	assert.Contains(t, out, "…\n")
	assert.Equal(t, 193, strings.Count(out, "x"), "the message is cut to 200 characters")
}

func TestPoppyDoctorTextColors(t *testing.T) {
	setColors(t, true)

	out := render(t, doctor.Report{Domain: "d", Results: []doctor.Result{
		{ID: "D1", Level: doctor.Pass, Message: "fine"},
		{ID: "D1", Level: doctor.Warn, Message: "hmm"},
		{ID: "D1", Level: doctor.Fail, Message: "bad"},
		{ID: "D1", Level: doctor.Skipped, Message: "later"},
	}})

	assert.Contains(t, out, ansi.Green("✓"))
	assert.Contains(t, out, ansi.Yellow("WARN"))
	assert.Contains(t, out, ansi.Red("FAIL"))
	assert.Contains(t, out, ansi.Faint("-"))
	assert.Contains(t, out, ansi.Bold("Website files"))

	setColors(t, false)
	assert.NotContains(t, render(t, doctor.Report{Domain: "d", Results: []doctor.Result{{ID: "D1", Level: doctor.Pass, Message: "fine"}}}), "\x1b")
}

func TestPoppyDoctorEveryCheckHasAGroup(t *testing.T) {
	for i := 1; i <= 17; i++ {
		id := fmt.Sprintf("D%d", i)
		assert.NotEqual(t, doctorOtherGroup, doctorGroupOf(id), id)
	}

	assert.Equal(t, doctorOtherGroup, doctorGroupOf("D99"))
	assert.Equal(t, doctorOtherGroup, doctorGroupOf(""))

	seen := map[string]string{}
	for _, g := range doctorGroups {
		for _, id := range g.ids {
			assert.NotContains(t, seen, id, "%s is in two groups", id)
			seen[id] = g.name
		}
	}
}

func TestPoppyDoctorAfterSetup(t *testing.T) {
	failed := shopReport(doctor.Result{ID: "D8", Spec: "4", Level: doctor.Fail, Message: "the tenant has no Poppy API", FixCommand: "auth0 poppy setup --force"})

	tenantOnly := failed
	tenantOnly.TenantOnly = true

	tests := []struct {
		name     string
		report   doctor.Report
		contains []string
		excludes []string
	}{
		{
			name:   "the full check points at the config file",
			report: failed,
			contains: []string{
				"1 check failed. Deploy the config file with your app and run `auth0 poppy doctor shop.example` again.",
				"Run auth0 poppy doctor shop.example to see every check.",
				"FAIL D8 (spec 4) the tenant has no Poppy API",
			},
			excludes: []string{"Only the checks of your tenant ran"},
		},
		{
			name:   "the tenant checks say how to run the rest",
			report: tenantOnly,
			contains: []string{
				"1 check failed. Follow the FIX lines above and run `auth0 poppy doctor shop.example` again.",
				"Only the checks of your tenant ran, because your website is usually not deployed yet.",
				"run `auth0 poppy doctor shop.example` to check everything.",
			},
			excludes: []string{"Deploy the config file", "to see every check"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out := strings.Join(strings.Fields(poppyDoctorAfterSetup(Layout{Unicode: true}, test.report)), " ")

			for _, want := range test.contains {
				assert.Contains(t, out, want)
			}
			for _, unwanted := range test.excludes {
				assert.NotContains(t, out, unwanted)
			}
		})
	}
}

func TestPoppyDoctorFixLines(t *testing.T) {
	both := doctorEntry{Result: doctor.Result{
		Level:      doctor.Fail,
		FixHint:    "Turn the setting on.",
		FixCommand: "auth0 poppy setup --company-domain shop.example --force",
	}}

	assert.Equal(t, []string{
		"Turn the setting on.",
		"auth0 poppy setup --company-domain shop.example --force",
	}, doctorFixLines(Layout{Unicode: true}, both, 10), "the sentence comes first, then the command")

	assert.Equal(t, []string{
		"Turn the setting on.",
		"auth0 poppy setup",
		"--company-domain",
		"shop.example --force",
	}, doctorFixLines(Layout{Width: 30, Unicode: true}, both, 14), "both wrap to the layout")

	assert.Empty(t, doctorFixLines(Layout{Unicode: true}, doctorEntry{}, 10), "no fix, no lines")

	platform := doctorEntry{Result: doctor.Result{FixHint: "Ask support.", PlatformDependency: true}}
	assert.Equal(t, []string{"Ask support. (Auth0 must fix this)"}, doctorFixLines(Layout{Unicode: true}, platform, 10))
}
