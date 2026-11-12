package display

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/poppy/setup"
)

// poppyRichTenant is a tenant with something to change in every group: scopes to
// describe and to add, and a login route.
func poppyRichTenant(t *testing.T) (*setup.State, setup.Inputs) {
	t.Helper()

	state, in := poppyFreshState(t)
	state.Tenant.CompanyDomains = []string{"old.example", "https://legacy.example"}
	state.APIs[0].Scopes = append(state.APIs[0].Scopes, setup.Scope{Name: "refunds"})
	state.App.LoginURI = ""

	in.Scopes = map[string]string{"refunds": "Refund an order you placed.", "wishlist": "Keep a list of things you want."}
	in.LoginURI = "https://shop.example/login"

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

	// The tenant is right, so there is nothing to change.
	configured := poppyConfiguredState(t)
	configuredIn := poppyConfiguredInputs()
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
			{Kind: setup.KindFile, Description: "write poppy.json", Impact: setup.ImpactFile, Fields: []setup.Field{}},
		},
	}

	tests := []struct {
		name   string
		plan   setup.Plan
		layout Layout
		opts   PoppySetupOptions
	}{
		{name: "plan-dry-run", plan: rich, layout: unicode80, opts: richOpts},
		{name: "plan-dry-run-ascii", plan: rich, layout: ascii80, opts: richOpts},
		{name: "plan-dry-run-pipe", plan: rich, layout: pipe, opts: richOpts},
		{name: "plan-dry-run-60", plan: rich, layout: Layout{Width: 60, Unicode: true}, opts: richOpts},
		{name: "plan-dry-run-100", plan: rich, layout: Layout{Width: 100, Unicode: true}, opts: richOpts},
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
		out := poppyPlanText(Layout{Width: width, Unicode: true}, plan, PoppySetupOptions{Inputs: in, DryRun: true})

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
		"  Login URI     https://shop.example/login\n",
		"Plan: 15 changes, 5 of them affect every app on the tenant\n",
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
	assert.Contains(t, out, "  shop.example\n", "only the new company domain is shown")
	assert.NotContains(t, out, "old.example")

	order := []string{"Tenant settings", "API " + poppyShopAPI, "App Shop Web", "File\n"}
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

		assert.Equal(t, poppyPlanText(Layout{}, plan, PoppySetupOptions{Inputs: in, DryRun: true}), buf.String())
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
		layout  Layout
		outcome setup.Outcome
	}{
		{name: "apply-success", layout: unicode80, outcome: setup.Outcome{Done: len(plan.Changes)}},
		{name: "apply-success-ascii", layout: ascii80, outcome: setup.Outcome{Done: len(plan.Changes)}},
		{name: "apply-partial-failure", layout: unicode80, outcome: setup.Outcome{Done: 1, Failure: errors.New("failed to update the API " + poppyShopAPI + ": 500 Internal Server Error")}},
		{name: "apply-partial-failure-60", layout: Layout{Width: 60, Unicode: true}, outcome: setup.Outcome{Done: 2, Failure: errors.New("failed to update the app Shop Web: 403 Forbidden\nInsufficient scope")}},
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

	assert.Contains(t, out, "Applying 15 changes\n")
	assert.Contains(t, out, "  ✓ Tenant settings")
	assert.Contains(t, out, "5 changes")
	assert.Contains(t, out, "  ✓ poppy.json")
	assert.Contains(t, out, "written")
	assert.NotContains(t, out, "continues", "a success has no rerun command")
}

func TestPoppySetupNext(t *testing.T) {
	assert.Equal(t, "Next\n"+
		"  1. Deploy poppy.json with your web and API SDKs.\n"+
		"  2. auth0 poppy doctor shop.example   to check the live site.\n"+
		"  3. auth0 agents register <agent-url>   to allow an agent.\n",
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
		got := poppyHintsText(Layout{Width: 24, Unicode: true}, setup.Inputs{Hints: hints("checkout", "orders:read")})

		assertGolden(t, "hints-narrow", got)
	})
}

func TestPoppyWebAnswers(t *testing.T) {
	yes, no := true, false

	tests := []struct {
		name string
		in   setup.Inputs
		want string
	}{
		{name: "not given, no row", in: setup.Inputs{}},
		{name: "true", in: setup.Inputs{WebUnscoped: &yes}, want: "  Web access    web.unscoped true\n"},
		{name: "false", in: setup.Inputs{WebUnscoped: &no}, want: "  Web access    web.unscoped false\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := poppyHintsText(unicode80, test.in)

			if test.want == "" {
				assert.Empty(t, got)

				return
			}

			assert.Equal(t, "Your answers\n"+test.want, got)
		})
	}

	t.Run("the plan lists it among the answers", func(t *testing.T) {
		state, in := poppyRichTenant(t)
		in.WebUnscoped = &yes

		out := poppyPlanText(unicode80, poppyBuildPlan(t, state, in), PoppySetupOptions{Inputs: in, DryRun: true})

		assert.Contains(t, out, "  Web access    web.unscoped true\n")
		assert.Contains(t, out, "--web-unscoped=true", "the command to apply repeats the answer")
	})
}

func TestPoppyPartAnswers(t *testing.T) {
	tests := []struct {
		name string
		in   setup.Inputs
		want []string
	}{
		{name: "both parts on, no row", in: setup.Inputs{}},
		{name: "API off", in: setup.Inputs{NoAPI: true}, want: []string{"API", "off (auth.disabled)"}},
		{name: "web off", in: setup.Inputs{NoWeb: true}, want: []string{"Browser join", "off (web.disabled)"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := poppyHintsText(unicode80, test.in)

			if test.want == nil {
				assert.Empty(t, got)

				return
			}

			assert.True(t, strings.HasPrefix(got, "Your answers\n"))
			assert.Contains(t, got, test.want[0])
			assert.Contains(t, got, test.want[1])
		})
	}

	t.Run("a run with the web part off says so and leaves out the scopes line", func(t *testing.T) {
		state, in := poppyRichTenant(t)
		in.NoWeb, in.App = true, ""
		state.App = setup.App{}

		out := poppyPlanText(unicode80, poppyBuildPlan(t, state, in), PoppySetupOptions{Inputs: in, DryRun: true})

		assert.Contains(t, out, "off (web.disabled)")
		assert.NotContains(t, out, "Web app")
		assert.Contains(t, out, "--no-web", "the command to apply repeats the choice")
	})

	t.Run("a run with the API part off says so and leaves out the scopes line", func(t *testing.T) {
		state, in := poppyRichTenant(t)
		state.APIs = nil
		in.NoAPI, in.APIs, in.APIURLs, in.Scopes, in.Hints = true, nil, nil, nil, nil

		out := poppyPlanText(unicode80, poppyBuildPlan(t, state, in), PoppySetupOptions{Inputs: in, DryRun: true})

		assert.Contains(t, out, "off (auth.disabled)")
		assert.NotContains(t, out, "Scopes")
		assert.Contains(t, out, "--no-api", "the command to apply repeats the choice")
	})
}
