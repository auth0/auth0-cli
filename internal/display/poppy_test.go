package display

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/poppy/doctor"
	"github.com/auth0/auth0-cli/internal/poppy/setup"
)

const (
	poppyShopAPI   = "https://api.shop.example"
	poppyShopApp   = "W1x9aBcD2eFgH3iJ"
	poppyTenantURL = "tenant.us.auth0.com"
)

var update = flag.Bool("update", false, "rewrite the golden files")

var (
	unicode80 = PoppyLayout{Width: 80, Unicode: true}
	ascii80   = PoppyLayout{Width: 80}
	pipe      = PoppyLayout{Unicode: true}
)

// assertGolden compares got with testdata/<name>.golden.txt, and rewrites the
// file when the tests run with -update.
func assertGolden(t *testing.T, name, got string) {
	t.Helper()

	path := filepath.Join("testdata", name+".golden.txt")
	if *update {
		require.NoError(t, os.MkdirAll("testdata", 0o750))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o600))
	}

	want, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(want), got)
}

// poppyConfiguredState is a tenant that setup has already been run against, with
// the config file it wrote.
func poppyConfiguredState(t *testing.T) *setup.State {
	t.Helper()

	file, err := config.Read(filepath.Join("..", "poppy", "config", "testdata", "auth0-poppy.golden.json"))
	require.NoError(t, err)

	return &setup.State{
		Tenant: setup.Tenant{
			CIMD:                     true,
			IssParameter:             true,
			ResourceParameterProfile: "compatibility",
			DefaultAudience:          poppyShopAPI,
			FriendlyName:             "Shop",
			CompanyDomains:           []string{"https://shop.example"},
		},
		Domain:       poppyTenantURL,
		CustomDomain: "auth.shop.example",
		APIs: []setup.API{{
			ID:         "rs_shop",
			Identifier: poppyShopAPI,
			Name:       "Shop API",
			Scopes: []setup.Scope{
				{Name: "orders:read", Description: "See your orders and their status. Read only."},
				{Name: "checkout", Description: "Place orders and pay with your saved card. Can't be undone."},
				{Name: setup.BrowserSignInScope, Description: setup.BrowserSignInDescription},
			},
			AllowOfflineAccess: true,
			AgentSubjectClaims: "auth0-v1",
			Poppy:              true,
			GuestPolicy:        "require_client_grant",
		}},
		App:           setup.App{ClientID: poppyShopApp, Name: "Shop Web", FirstParty: true, LoginURI: "https://shop.example/login", BrowserJoin: true},
		HasGuestGrant: map[string]bool{poppyShopAPI: true},
		File:          file,
	}
}

func poppyConfiguredInputs() setup.Inputs {
	return setup.Inputs{
		CompanyDomain:  "shop.example",
		APIs:           []string{poppyShopAPI},
		App:            "Shop Web",
		Out:            config.DefaultFileName,
		GuestAPIAccess: true,
		GuestAllScopes: true,
	}
}

// poppyFreshState is a tenant that has never been set up for Poppy.
func poppyFreshState(t *testing.T) (*setup.State, setup.Inputs) {
	t.Helper()

	state := poppyConfiguredState(t)
	state.Tenant = setup.Tenant{ResourceParameterProfile: "audience", FriendlyName: "Shop"}
	state.CustomDomain = ""
	state.APIs[0] = setup.API{
		ID:         "rs_shop",
		Identifier: poppyShopAPI,
		Name:       "Shop API",
		Scopes: []setup.Scope{
			{Name: "orders:read", Description: "See your orders and their status. Read only."},
			{Name: "checkout", Description: "Place orders and pay with your saved card. Can't be undone."},
		},
		AgentSubjectClaims: "off",
	}
	state.App.BrowserJoin = false
	state.HasGuestGrant = map[string]bool{}
	state.File = nil

	in := poppyConfiguredInputs()
	in.APIURLs = map[string]string{poppyShopAPI: poppyShopAPI + "/openapi.json"}

	return state, in
}

func renderWith(t *testing.T, r doctor.Report, l PoppyLayout) string {
	t.Helper()

	return doctorText(l, r)
}

func render(t *testing.T, r doctor.Report) string {
	t.Helper()

	return renderWith(t, r, unicode80)
}

// greenReport is the report of a site that passes every check, as the doctor
// produced it.
func greenReport(t *testing.T) doctor.Report {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", "report-green.json"))
	require.NoError(t, err)

	var report doctor.Report
	require.NoError(t, json.Unmarshal(data, &report))

	return report
}

func shopReport(results ...doctor.Result) doctor.Report {
	return doctor.Report{Domain: "shop.example", ProtocolVersion: "0.3", Issuer: "https://shop.example/", Results: results}
}

var (
	issuerPass = doctor.Result{ID: "D3", Spec: "3.2", Level: doctor.Pass, Message: "issuer matches https://shop.example/"}

	protectedFail = doctor.Result{
		ID:      "D3",
		Spec:    "3.2",
		Level:   doctor.Fail,
		Message: "protected_resources does not list https://shop.example",
		Fix:     "Run auth0 poppy setup --company-domain shop.example",
	}

	ps384Fail = doctor.Result{
		ID:                 "D4",
		Spec:               "4.1",
		Level:              doctor.Fail,
		Message:            "client assertions: PS384 not accepted by the issuer",
		Fix:                "PS384 needs the HRI SKU. Ask support if you need it",
		PlatformDependency: true,
	}

	hintWarn = doctor.Result{
		ID:      "D5",
		Spec:    "4.7",
		Level:   doctor.Warn,
		Message: "scope_hints missing for checkout",
		Fix:     "auth0 poppy setup --hint checkout=changes:<none|reversible|irreversible>,personal_data:<true|false>",
	}

	tenantSkipped = doctor.Result{
		ID:      "D8",
		Level:   doctor.Skipped,
		Message: "you are logged in to a different tenant than the one that serves shop.example. Log in to that tenant to run these checks.",
	}
)

func TestPoppyDoctorTextGolden(t *testing.T) {
	green := greenReport(t)

	long := shopReport(
		doctor.Result{ID: "D1", Spec: "3", Level: doctor.Pass, Message: "poppy.json is served over HTTPS at /.well-known/poppy.json"},
		doctor.Result{ID: "D7", Spec: "7", Level: doctor.Fail, Message: "https://api.shop.example/poppy/openapi.json returned HTTP 503, and the document must be served with status 200 so that agents can read it", Fix: "Serve the document at https://api.shop.example/poppy/openapi.json with status 200."},
		doctor.Result{ID: "D6", Spec: "6", Level: doctor.Warn, Message: "web.browser_session_endpoint is not published, so agents cannot join a browser session"},
		tenantSkipped,
	)
	long.Domain, long.Issuer = "a-very-long-company-domain.example.com", "https://login.a-very-long-company-domain.example.com/"

	tests := []struct {
		name   string
		report doctor.Report
		layout PoppyLayout
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
		{name: "report-width-60", report: long, layout: PoppyLayout{Width: 60, Unicode: true}},
		{name: "report-width-100", report: long, layout: PoppyLayout{Width: 100, Unicode: true}},
		{
			name: "report-escapes",
			report: shopReport(doctor.Result{
				ID: "D5", Level: doctor.Fail,
				Message: "scope \x1b[2J\x1b[31mred\nforged line",
				Fix:     "run \x1b]0;title\x07\nthen \r check",
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
		layout PoppyLayout
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
			report: doctor.Report{Domain: "shop.example", ProtocolVersion: "0.4", Results: []doctor.Result{{ID: "D1", Level: doctor.Pass, Message: "ok"}}},
			layout: PoppyLayout{Width: 40, Unicode: true},
			want: "Doctor for shop.example     protocol 0.4\n\n" +
				"Website files\n  ✓ ok\n\n1 passed.\n",
		},
		{
			name:   "a header wider than the terminal keeps one space",
			report: doctor.Report{Domain: "shop.example", ProtocolVersion: "0.4"},
			layout: PoppyLayout{Width: 10, Unicode: true},
			want:   "Doctor for shop.example protocol 0.4\n\n0 checks ran.\n",
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
				{ID: "D8", Level: doctor.Skipped, Message: "cannot check the web apps", Fix: "auth0 login --scopes read:clients"},
				{ID: "D4", Level: doctor.Fail, Message: "bad", Fix: "one\ntwo", PlatformDependency: true},
			}},
			layout: unicode80,
			want: "Doctor for d\n\n" +
				"Auth0 issuer\n  FAIL bad\n       FIX  one\n            two (Auth0 must fix this)\n\n" +
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

	assert.Equal(t, want, renderWith(t, report, PoppyLayout{Unicode: true}), "the summary still counts every check")
	assert.Equal(t, 6, report.Count(doctor.Skipped))
}

func TestPoppyDoctorTextDoesNotWrapWithoutAWidth(t *testing.T) {
	const full = "checkout is in neither auth.scopes nor the scopes_supported of the issuer, and the signed-in browser join needs it"

	report := doctor.Report{Domain: "d", Results: []doctor.Result{{ID: "D5", Level: doctor.Warn, Message: full}}}

	assert.Contains(t, renderWith(t, report, PoppyLayout{Unicode: true}), "  WARN "+full+"\n", "a pipe sees the whole line")
	assert.NotContains(t, renderWith(t, report, PoppyLayout{Width: 60, Unicode: true}), "  WARN "+full+"\n", "a terminal wraps it")
}

func TestPoppyDoctorProblems(t *testing.T) {
	report := shopReport(issuerPass, protectedFail, hintWarn, tenantSkipped,
		doctor.Result{ID: "D1", Level: doctor.Pass, Message: "poppy.json is served"})

	tests := []struct {
		name   string
		layout PoppyLayout
		prefix string
		want   string
	}{
		{
			name:   "only warnings and failures, in their groups, with fixes",
			layout: unicode80,
			prefix: "  ",
			want: "  Auth0 issuer\n" +
				"    FAIL protected_resources does not list https://shop.example\n" +
				"         FIX  Run auth0 poppy setup --company-domain shop.example\n" +
				"\n" +
				"  Scopes\n" +
				"    WARN scope_hints missing for checkout\n" +
				"         FIX  auth0 poppy setup --hint checkout=changes:<none|reversible|irreversible>,personal_data:<true|false>\n",
		},
		{
			name:   "the prefix narrows the wrapping",
			layout: PoppyLayout{Width: 30, Unicode: true},
			prefix: "    ",
			want: "    Auth0 issuer\n" +
				"      FAIL protected_resources\n" +
				"           does not list\n" +
				"           https://shop.example\n" +
				"           FIX  Run auth0 poppy setup --company-domain shop.example\n" +
				"\n" +
				"    Scopes\n" +
				"      WARN scope_hints missing\n" +
				"           for checkout\n" +
				"           FIX  auth0 poppy setup --hint checkout=changes:<none|reversible|irreversible>,personal_data:<true|false>\n",
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
		out := renderWith(t, report, PoppyLayout{Width: width, Unicode: true})

		for _, line := range strings.Split(out, "\n") {
			assert.LessOrEqual(t, utf8.RuneCountInString(line), width, "line %q at width %d", line, width)
		}
	}

	out := renderWith(t, report, PoppyLayout{Width: 40, Unicode: true})
	assert.Contains(t, out, "  ✓ word word word word word word word\n    word word", "continuation lines sit under the message")
	assert.Contains(t, out, "  FAIL "+strings.Repeat("x", 33)+"\n       "+strings.Repeat("x", 33)+"\n", "a word longer than a line is cut")
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

// poppyRichTenant is a tenant with something to change in every group: scopes to
// describe and to add, a login route, and a guest grant.
func poppyRichTenant(t *testing.T) (*setup.State, setup.Inputs) {
	t.Helper()

	state, in := poppyFreshState(t)
	state.Tenant.CompanyDomains = []string{"https://old.example"}
	state.APIs[0].Scopes = append(state.APIs[0].Scopes, setup.Scope{Name: "refunds"})
	state.App.LoginURI = ""

	in.Scopes = map[string]string{"refunds": "Refund an order you placed.", "wishlist": "Keep a list of things you want."}
	in.LoginURI = "https://shop.example/login"
	in.GuestAllScopes, in.GuestScopes = false, map[string][]string{poppyShopAPI: {"orders:read", "checkout"}}

	return state, in
}

func poppyBuildPlan(t *testing.T, state *setup.State, in setup.Inputs) setup.Plan {
	t.Helper()

	plan, err := setup.Build(state, in)
	require.NoError(t, err)

	return plan
}

func TestPoppyPlanTextGolden(t *testing.T) {
	state, in := poppyRichTenant(t)
	rich := poppyBuildPlan(t, state, in)
	richOpts := PoppySetupOptions{Inputs: in, DryRun: true}

	// The tenant is right but for the guest grant and the file.
	configured := poppyConfiguredState(t)
	configuredIn := poppyConfiguredInputs()
	configuredIn.GuestAPIAccess = false
	empty := poppyBuildPlan(t, configured, configuredIn)

	// Settings that are right are collapsed into one line.
	partial := poppyConfiguredState(t)
	partial.Tenant.CIMD = false
	partial.App.BrowserJoin = false
	partialPlan := poppyBuildPlan(t, partial, configuredIn)

	// One API, no custom domain: the issuer warning.
	freshState, freshIn := poppyFreshState(t)
	freshPlan := poppyBuildPlan(t, freshState, freshIn)

	// Several APIs.
	many, manyIn := freshState, freshIn
	many.APIs = append(many.APIs, setup.API{ID: "rs_pay", Identifier: "https://pay.shop.example", Name: "Pay", AgentSubjectClaims: "off",
		Scopes: []setup.Scope{{Name: "pay", Description: "Pay for an order."}}})
	manyIn.APIs = append(manyIn.APIs, "https://pay.shop.example")
	manyIn.APIURLs = map[string]string{poppyShopAPI: poppyShopAPI + "/openapi.json", "https://pay.shop.example": "https://pay.shop.example/openapi.json"}
	manyPlan := poppyBuildPlan(t, many, manyIn)

	// A value that could not be read, and a setting that does not exist.
	unknown := setup.Plan{
		Tenant:        poppyTenantURL,
		Issuer:        "https://" + poppyTenantURL + "/",
		CompanyDomain: "shop.example",
		APIs:          []string{poppyShopAPI},
		App:           "Shop Web",
		AppFirstParty: true,
		Warnings:      []string{"Something else to look at."},
		Changes: []setup.Change{
			{Kind: setup.KindApp, Impact: setup.ImpactApp, Fields: []setup.Field{
				{Name: "initiate_login_uri", Value: "https://shop.example/login", Before: "none"},
				{Name: "allow_personal_agent_browser_join", Value: "true"},
			}},
			{Kind: setup.KindFile, Description: "write auth0-poppy.json", Impact: setup.ImpactFile, Fields: []setup.Field{}},
		},
	}

	tests := []struct {
		name   string
		plan   setup.Plan
		layout PoppyLayout
		opts   PoppySetupOptions
	}{
		{name: "plan-dry-run", plan: rich, layout: unicode80, opts: richOpts},
		{name: "plan-dry-run-ascii", plan: rich, layout: ascii80, opts: richOpts},
		{name: "plan-dry-run-pipe", plan: rich, layout: pipe, opts: richOpts},
		{name: "plan-dry-run-60", plan: rich, layout: PoppyLayout{Width: 60, Unicode: true}, opts: richOpts},
		{name: "plan-dry-run-100", plan: rich, layout: PoppyLayout{Width: 100, Unicode: true}, opts: richOpts},
		{name: "plan-before-apply", plan: rich, layout: unicode80, opts: PoppySetupOptions{Inputs: in}},
		{name: "plan-nothing-to-change", plan: empty, layout: unicode80, opts: PoppySetupOptions{Inputs: configuredIn, DryRun: true}},
		{name: "plan-no-custom-domain", plan: freshPlan, layout: unicode80, opts: PoppySetupOptions{Inputs: freshIn, DryRun: true}},
		{name: "plan-unchanged-collapsed", plan: partialPlan, layout: unicode80, opts: PoppySetupOptions{Inputs: configuredIn, DryRun: true}},
		{name: "plan-several-apis", plan: manyPlan, layout: unicode80, opts: PoppySetupOptions{Inputs: manyIn, DryRun: true}},
		{name: "plan-unknown-before", plan: unknown, layout: unicode80, opts: PoppySetupOptions{DryRun: true}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertGolden(t, test.name, poppyPlanText(test.layout, test.plan, test.opts))
		})
	}
}

func TestPoppyPlanTextWarningsWrap(t *testing.T) {
	state, in := poppyFreshState(t)
	plan := poppyBuildPlan(t, state, in)

	for _, width := range []int{60, 100} {
		out := poppyPlanText(PoppyLayout{Width: width, Unicode: true}, plan, PoppySetupOptions{Inputs: in, DryRun: true})

		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, poppyToApplyLabel) || strings.HasPrefix(line, strings.Repeat(" ", len(poppyToApplyLabel)+2)) {
				continue
			}

			assert.LessOrEqual(t, utf8.RuneCountInString(line), width, "line %q at width %d", line, width)
		}

		assert.Contains(t, strings.Join(strings.Fields(out), " "), plan.Warnings[0], "the warning is whole, only wrapped")
	}

	assert.Contains(t, poppyPlanText(pipe, plan, PoppySetupOptions{Inputs: in}), plan.Warnings[0], "a pipe gets the whole warning on one line")
}

func TestPoppyPlanTextLines(t *testing.T) {
	state, in := poppyRichTenant(t)
	plan := poppyBuildPlan(t, state, in)

	out := poppyPlanText(unicode80, plan, PoppySetupOptions{Inputs: in, DryRun: true})

	for _, want := range []string{
		"Poppy setup for shop.example",
		"tenant " + poppyTenantURL + "\n",
		"Dry run: nothing will be changed.\n",
		"  ✓ API       " + poppyShopAPI + "\n",
		"  ✓ Web app   Shop Web (first party)\n",
		"  ✓ Scopes    all described  (refunds, wishlist from your --scope)\n",
		"  ! Issuer    https://" + poppyTenantURL + "/\n",
		"  Login route   https://shop.example/login\n",
		"  Guest access  2 scopes on https://api.shop.example\n",
		"                orders:read, checkout\n",
		"Plan: 16 changes, 5 of them affect every app on the tenant\n",
		"⚠ affects all apps\n",
		"  ~ iss in sign-in redirects",
		"off → on",
		"every app's redirects",
	} {
		assert.Contains(t, out, want)
	}

	assert.NotContains(t, out, "+ scope refunds", "a scope that only gets a description is not shown as new")

	assert.Contains(t, out, `~ scope refunds`)
	assert.Contains(t, out, `none → "Refund an order you placed."`)
	assert.Contains(t, out, `+ scope wishlist`)
	assert.Contains(t, out, "  + company domain", "the company domain is added")
	assert.Contains(t, out, "  https://shop.example\n", "only the new company domain is shown")
	assert.NotContains(t, out, "old.example")

	order := []string{"Tenant settings", "API " + poppyShopAPI, "App Shop Web", "Guest grant", "File\n"}
	last := -1
	for _, heading := range order {
		at := strings.Index(out, "\n"+heading)
		require.Greater(t, at, last, "%q comes after the one before it", heading)
		last = at
	}
}

func TestPoppyPlanTextNothingToChange(t *testing.T) {
	state := poppyConfiguredState(t)
	in := poppyConfiguredInputs()
	in.GuestAPIAccess = false
	plan := poppyBuildPlan(t, state, in)

	// The file differs from what is derived only when the tenant changed, so the
	// plan is empty here.
	require.True(t, plan.Empty())

	out := poppyPlanText(unicode80, plan, PoppySetupOptions{Inputs: in, DryRun: true})

	assert.Contains(t, out, "Nothing to change. Everything already matches.\n")
	assert.NotContains(t, out, "To apply")
	assert.NotContains(t, out, "Your answers")
	assert.NotContains(t, out, "Dry run")
	assert.True(t, strings.HasPrefix(out, "Poppy setup for shop.example"))
}

func TestPoppyPlanTextDoesNotApplyWithoutDryRun(t *testing.T) {
	state, in := poppyRichTenant(t)
	plan := poppyBuildPlan(t, state, in)

	out := poppyPlanText(unicode80, plan, PoppySetupOptions{Inputs: in})

	assert.NotContains(t, out, "To apply")
	assert.NotContains(t, out, "Dry run")
}

func TestRendererPoppyPlan(t *testing.T) {
	state, in := poppyRichTenant(t)
	plan := poppyBuildPlan(t, state, in)

	t.Run("a writer that is not a terminal gets the whole lines", func(t *testing.T) {
		setColors(t, false)

		var buf bytes.Buffer
		r := &Renderer{ResultWriter: &buf}
		r.PoppyPlan(plan, PoppySetupOptions{Inputs: in, DryRun: true})

		assert.Equal(t, poppyPlanText(PoppyLayout{}, plan, PoppySetupOptions{Inputs: in, DryRun: true}), buf.String())
		assert.NotContains(t, buf.String(), "→", "ASCII without colors")
		assert.Contains(t, buf.String(), "->")
		assert.NotContains(t, buf.String(), "\x1b")
	})
}

func TestPoppyPlanTextEscapesTenantText(t *testing.T) {
	state, in := poppyFreshState(t)
	state.App.Name = "Shop\x1b[2J Web\nforged"
	plan := poppyBuildPlan(t, state, in)

	out := poppyPlanText(unicode80, plan, PoppySetupOptions{Inputs: in})

	assert.NotContains(t, out, "\x1b")
	assert.Contains(t, out, `Shop\x1b[2J Web forged`)
}

func TestPoppyApplyTextGolden(t *testing.T) {
	state, in := poppyRichTenant(t)
	plan := poppyBuildPlan(t, state, in)
	opts := PoppySetupOptions{Inputs: in, Command: setup.CommandOptions{Yes: true}}

	tests := []struct {
		name    string
		layout  PoppyLayout
		outcome setup.Outcome
	}{
		{name: "apply-success", layout: unicode80, outcome: setup.Outcome{Done: len(plan.Changes)}},
		{name: "apply-success-ascii", layout: ascii80, outcome: setup.Outcome{Done: len(plan.Changes)}},
		{name: "apply-partial-failure", layout: unicode80, outcome: setup.Outcome{Done: 1, Failure: errors.New("failed to update the API " + poppyShopAPI + ": 500 Internal Server Error")}},
		{name: "apply-partial-failure-60", layout: PoppyLayout{Width: 60, Unicode: true}, outcome: setup.Outcome{Done: 2, Failure: errors.New("failed to update the app Shop Web: 403 Forbidden\nInsufficient scope")}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertGolden(t, test.name, poppyApplyText(test.layout, plan, test.outcome, opts))
		})
	}
}

func TestPoppyApplyTextCounts(t *testing.T) {
	state, in := poppyRichTenant(t)
	plan := poppyBuildPlan(t, state, in)

	out := poppyApplyText(unicode80, plan, setup.Outcome{Done: len(plan.Changes)}, PoppySetupOptions{Inputs: in})

	assert.Contains(t, out, "Applying 16 changes\n")
	assert.Contains(t, out, "  ✓ Tenant settings")
	assert.Contains(t, out, "5 changes")
	assert.Contains(t, out, "  ✓ auth0-poppy.json")
	assert.Contains(t, out, "written")
	assert.NotContains(t, out, "continues", "a success has no rerun command")
}

func TestPoppySetupNext(t *testing.T) {
	assert.Equal(t, "Next\n"+
		"  1. Deploy auth0-poppy.json with your web and API SDKs.\n"+
		"  2. auth0 poppy doctor shop.example   to check the live site.\n"+
		"  3. auth0 poppy agents register <agent-url>   to allow an agent.\n",
		poppySetupNext(unicode80, "shop.example", "", 0))

	assert.Contains(t, poppySetupNext(unicode80, "shop.example", "./out/poppy.json", 0), "Deploy ./out/poppy.json with")

	for _, tc := range []struct {
		unhinted int
		want     string
	}{
		{1, "Set the hints of 1 scope with --hint"},
		{4, "Set the hints of 4 scopes with --hint"},
	} {
		out := poppySetupNext(unicode80, "shop.example", "", tc.unhinted)
		assert.Contains(t, out, tc.want)
		assert.Contains(t, out, "<scope>=changes:<none|reversible|irreversible>,personal_data:<true|false>")
		assert.Contains(t, out, "  3. auth0 poppy doctor shop.example", "the hint step goes before the check")
	}

	assert.NotContains(t, poppySetupNext(unicode80, "shop.example", "", 0), "--hint")
}

func TestPoppyHintsText(t *testing.T) {
	setColors(t, false)

	hints := func(names ...string) map[string]config.Hints {
		all := map[string]config.Hints{
			"orders:read": {Changes: "none"},
			"profile":     {Changes: "none", PersonalData: true},
			"cart":        {Changes: "reversible"},
			"wishlist":    {Changes: "reversible", PersonalData: true},
			"checkout":    {Changes: "irreversible", PersonalData: true},
			"refund":      {Changes: "irreversible"},
			"delete":      {Changes: "irreversible", PersonalData: true},
			"export":      {Changes: "none", PersonalData: true},
		}

		picked := map[string]config.Hints{}
		for _, name := range names {
			picked[name] = all[name]
		}

		return picked
	}

	tests := []struct {
		name  string
		hints map[string]config.Hints
		empty bool
	}{
		{name: "no hints", empty: true},
		{name: "three hints", hints: hints("checkout", "orders:read", "cart")},
		{name: "five hints are all shown", hints: hints("checkout", "orders:read", "cart", "profile", "refund")},
		{name: "eight hints show three and count the rest", hints: hints("checkout", "orders:read", "cart", "profile", "refund", "wishlist", "delete", "export")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := poppyHintsText(unicode80, setup.Inputs{Hints: test.hints})

			if test.empty {
				assert.Empty(t, got, "no hints, no block")

				return
			}

			assertGolden(t, "hints-"+strings.ReplaceAll(test.name, " ", "-"), got)
		})
	}

	t.Run("a narrow terminal wraps on the label column", func(t *testing.T) {
		got := poppyHintsText(PoppyLayout{Width: 24, Unicode: true}, setup.Inputs{Hints: hints("checkout", "orders:read")})

		assertGolden(t, "hints-narrow", got)
	})
}

func newTestPoppyScreen(erase bool) (*PoppyScreen, *bytes.Buffer) {
	var out bytes.Buffer

	r := &Renderer{ResultWriter: &out}

	return r.NewPoppyScreen("Poppy setup", "mock.auth0.test", PoppyLayout{Width: 80, Unicode: true}, erase), &out
}

func TestPoppyScreenTitle(t *testing.T) {
	ask := func(s *PoppyScreen) {
		s.Begin("Question", "Why.")
		require.NoError(t, s.End())
	}

	t.Run("the title of the run is printed once, above the first question", func(t *testing.T) {
		screen, out := newTestPoppyScreen(false)
		screen.ForDomain("shop.example")

		ask(screen)
		ask(screen)

		assert.Equal(t, 1, strings.Count(out.String(), "Poppy setup for shop.example"))
		assert.True(t, strings.HasPrefix(out.String(), "Poppy setup for shop.example"))
		assert.True(t, screen.HeaderShown())
	})

	t.Run("no title until the company domain is known", func(t *testing.T) {
		screen, out := newTestPoppyScreen(false)

		ask(screen)

		assert.NotContains(t, out.String(), "Poppy setup")
		assert.False(t, screen.HeaderShown())
	})

	t.Run("a run that prints nothing after its questions takes the title back", func(t *testing.T) {
		screen, out := newTestPoppyScreen(true)
		screen.ForDomain("shop.example")

		ask(screen)
		screen.DropHeader()

		assert.True(t, strings.HasSuffix(out.String(), "\r\x1b[1A\x1b[J"), "one row, the title")
		assert.False(t, screen.HeaderShown())
	})

	t.Run("the title stays once text was printed below it", func(t *testing.T) {
		screen, out := newTestPoppyScreen(true)
		screen.ForDomain("shop.example")

		ask(screen)
		screen.Printed()
		screen.DropHeader()

		assert.NotContains(t, out.String(), "\x1b[1A")
		assert.True(t, screen.HeaderShown())
	})

	t.Run("a note stays too", func(t *testing.T) {
		screen, out := newTestPoppyScreen(true)
		screen.ForDomain("shop.example")

		ask(screen)
		screen.Note("Nothing to %s.", "choose")
		screen.DropHeader()

		assert.Contains(t, out.String(), "Nothing to choose.")
		assert.True(t, screen.HeaderShown())
	})

	t.Run("an answer and a refusal are lines of the block", func(t *testing.T) {
		screen, out := newTestPoppyScreen(true)

		screen.Answered("Does it change anything?", "none")
		screen.Invalid("Answer y or n.")

		assert.Equal(t, "  ✓ Does it change anything?  none\n  ✗ Answer y or n.\n", out.String())
		assert.Equal(t, 2, screen.BlockRows())
	})

	t.Run("Ctrl-C says nothing was changed", func(t *testing.T) {
		screen, out := newTestPoppyScreen(true)

		screen.Cancelled()

		assert.Equal(t, PoppyCancelled+"\n", out.String())
	})
}

func TestPoppyScreenRows(t *testing.T) {
	tests := []struct {
		name string
		line string
		want int
	}{
		{name: "one row", line: "  Question", want: 1},
		{name: "a long line wraps", line: "    " + strings.Repeat("x", 100), want: 2},
		{name: "an empty line takes a row", want: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			screen, out := newTestPoppyScreen(true)

			assert.Equal(t, test.want, screen.Rows(test.line))

			screen.AddRows(screen.Rows(test.line))

			assert.Equal(t, test.want, screen.BlockRows())
			assert.Empty(t, out.String(), "counting writes nothing")
		})
	}
}

func TestPoppyHintNote(t *testing.T) {
	tests := []struct {
		name  string
		hints *config.Hints
		want  string
	}{
		{name: "no hints", want: ""},
		{name: "changes nothing", hints: &config.Hints{Changes: "none"}, want: "[changes: none]"},
		{name: "personal data", hints: &config.Hints{Changes: "reversible", PersonalData: true}, want: "[changes: reversible, personal data]"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, poppyHintNote(test.hints))
		})
	}
}

func TestPoppyScopeRows(t *testing.T) {
	scopes := []PoppyScopeChoice{
		{Name: "orders:read", Description: "See your orders and their status", Hints: &config.Hints{Changes: "none"}},
		{Name: "checkout", Description: "Place orders and pay with your saved card"},
		{Name: "poppy:browser-sign-in", Description: "Use the website signed in, with the other access you approve"},
	}

	rowsAt := func(width int) []string {
		var out bytes.Buffer

		r := &Renderer{ResultWriter: &out}

		return r.NewPoppyScreen("", "", PoppyLayout{Width: width, Unicode: true}, false).ScopeRows(scopes)
	}

	t.Run("names are aligned and the descriptions follow", func(t *testing.T) {
		assert.Equal(t, []string{
			"orders:read            See your orders and their status [changes: none]",
			"checkout               Place orders and pay with your saved card",
			"poppy:browser-sign-in  Use the website signed in, with the other access you approve",
		}, rowsAt(120))
	})

	t.Run("a description that does not fit is cut, never wrapped", func(t *testing.T) {
		for _, row := range rowsAt(60) {
			assert.LessOrEqual(t, len([]rune(row))+PoppyListMarks, 60, row)
		}

		assert.Contains(t, rowsAt(60)[2], "…")
	})

	t.Run("the hints stay when the description is cut", func(t *testing.T) {
		assert.True(t, strings.HasSuffix(rowsAt(60)[0], "[changes: none]"), rowsAt(60)[0])
	})
}

func TestPoppyChangeCount(t *testing.T) {
	assert.Equal(t, "1 change", PoppyChangeCount(1))
	assert.Equal(t, "3 changes", PoppyChangeCount(3))
}
