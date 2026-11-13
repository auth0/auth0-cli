package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auth0/go-auth0"
	"github.com/auth0/go-auth0/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
	"go.uber.org/mock/gomock"

	"github.com/auth0/auth0-cli/internal/poppy/setup"
)

// poppyActionTenant is the Actions API of a tenant: the Actions, the post-login
// bindings, and the writes made to them.
type poppyActionTenant struct {
	actions  []*management.Action
	bindings []*management.ActionBinding

	// Calls are the writes made, in order.
	calls []string
	// Created is the Action that was created, and patched the list the bindings
	// were replaced with.
	created *management.Action
	updated *management.Action
	patched []*management.ActionBinding
	// OnDeploy runs when the Action is deployed, to change the tenant while setup
	// is running.
	onDeploy func()
}

// poppyOurAction is the Action of setup, with the given code.
func poppyOurAction(code string, deployed bool) *management.Action {
	action := &management.Action{
		ID:                 auth0.String("act_ours"),
		Name:               auth0.String(setup.OfflineAccessActionName),
		Code:               auth0.String(code),
		AllChangesDeployed: deployed,
	}
	if deployed {
		action.DeployedVersion = &management.ActionVersion{Code: auth0.String(code), Deployed: true}
	}

	return action
}

func poppyBinding(actionID, name string) *management.ActionBinding {
	return &management.ActionBinding{
		ID:          auth0.String("bnd_" + actionID),
		DisplayName: auth0.String(name),
		Action:      &management.Action{ID: auth0.String(actionID), Name: auth0.String(name)},
	}
}

// poppyOthers are two Actions of the tenant that run on every sign-in.
func poppyOthers() []*management.ActionBinding {
	return []*management.ActionBinding{poppyBinding("act_a", "Enrich"), poppyBinding("act_b", "Audit")}
}

// patchedFlow returns the Actions the bindings were replaced with, by ID.
func (a *poppyActionTenant) patchedFlow() []string {
	var flow []string
	for _, binding := range a.patched {
		flow = append(flow, binding.Ref.GetValue())
	}

	return flow
}

// serveActions serves the Actions API of the tenant to the commands and records
// its writes. Without it, the harness has no calls on the Actions API.
func (h *poppySetupHarness) serveActions(tenant *poppyActionTenant) {
	h.actions.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(context.Context, ...management.RequestOption) (*management.ActionList, error) {
			return &management.ActionList{List: management.List{Total: len(tenant.actions)}, Actions: tenant.actions}, nil
		}).AnyTimes()
	h.actions.EXPECT().Bindings(gomock.Any(), "post-login", gomock.Any()).DoAndReturn(
		func(context.Context, string, ...management.RequestOption) (*management.ActionBindingList, error) {
			return &management.ActionBindingList{List: management.List{Total: len(tenant.bindings)}, Bindings: tenant.bindings}, nil
		}).AnyTimes()
	h.actions.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, action *management.Action, _ ...management.RequestOption) error {
			tenant.calls = append(tenant.calls, "create")
			action.ID = auth0.String("act_ours")
			tenant.created = action

			return nil
		}).AnyTimes()
	h.actions.EXPECT().Update(gomock.Any(), "act_ours", gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, action *management.Action, _ ...management.RequestOption) error {
			tenant.calls = append(tenant.calls, "update")
			tenant.updated = action

			return nil
		}).AnyTimes()
	h.actions.EXPECT().Read(gomock.Any(), "act_ours", gomock.Any()).Return(
		&management.Action{ID: auth0.String("act_ours"), Status: auth0.String(management.ActionStatusBuilt)}, nil).AnyTimes()
	h.actions.EXPECT().Deploy(gomock.Any(), "act_ours", gomock.Any()).DoAndReturn(
		func(context.Context, string, ...management.RequestOption) (*management.ActionVersion, error) {
			tenant.calls = append(tenant.calls, "deploy")
			if tenant.onDeploy != nil {
				tenant.onDeploy()
			}

			return &management.ActionVersion{Deployed: true}, nil
		}).AnyTimes()
	h.actions.EXPECT().UpdateBindings(gomock.Any(), "post-login", gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, bindings []*management.ActionBinding, _ ...management.RequestOption) error {
			tenant.calls = append(tenant.calls, "bindings")
			tenant.patched = bindings

			return nil
		}).AnyTimes()
}

// actionAnswers are the answers of a person at the terminal.
type actionAnswers struct {
	// Refresh answers the question about the Action, and apply the confirmation.
	refresh, apply bool
}

// runActions runs setup in a mode, with the question about the Action and the
// confirmation of the plan answered as given. It returns the run and how many times
// each was asked.
func (h *poppySetupHarness) runActions(t *testing.T, mode poppyPromptMode, args []string, a actionAnswers) (poppyPromptRun, map[string]int) {
	t.Helper()

	installPoppyAsker(t, mode)

	asked := map[string]int{}
	previous := promptAsk.confirm

	promptAsk.confirm = func(message string, def bool) (bool, error) {
		switch {
		case message == poppyRefreshTokensText:
			asked["refresh"]++

			return a.refresh, nil
		case strings.HasPrefix(message, "Apply "):
			asked["apply"]++

			return a.apply, nil
		}

		return previous(message, def)
	}

	h.agentMode = mode.agentMode

	err := h.run(append(args, mode.flags...)...)

	return poppyPromptRun{out: h.out.String(), errOut: h.messages.String(), err: err}, asked
}

// actionArgs are the arguments of a setup against the configured tenant, whose
// config file is up to date, so that the Action is the only difference.
func actionArgs(t *testing.T, extra ...string) []string {
	t.Helper()

	return append([]string{"setup", "--company-domain", "shop.example", "--api", poppyTestAPI, "--app", poppySetupAppID,
		"--out", goldenPoppyConfig(t), "--skip-doctor"}, extra...)
}

func TestPoppySetupRefreshTokens(t *testing.T) {
	code := setup.OfflineAccessActionSource

	tests := []struct {
		name    string
		mode    poppyPromptMode
		flags   []string
		answers actionAnswers
		actions []*management.Action
		// The bindings of the tenant before the run.
		bindings []*management.ActionBinding
		// The writes the run makes, in order, and the flow it ends with.
		wantCalls []string
		wantFlow  []string
		// How many times the question about the Action and the confirmation were asked.
		wantRefresh, wantApply int
	}{
		{
			name: "the flag, no Action yet: create, deploy, bind it last", mode: modePiped, flags: []string{"--always-issue-refresh-tokens", "--force"},
			bindings:  poppyOthers(),
			wantCalls: []string{"create", "deploy", "bindings"},
			wantFlow:  []string{"act_a", "act_b", "act_ours"},
		},
		{
			name: "the flag with an empty flow", mode: modePiped, flags: []string{"--always-issue-refresh-tokens", "--force"},
			wantCalls: []string{"create", "deploy", "bindings"},
			wantFlow:  []string{"act_ours"},
		},
		{
			name: "the Action exists with the code, deployed and bound last: nothing to do", mode: modePiped, flags: []string{"--always-issue-refresh-tokens", "--force"},
			actions:  []*management.Action{poppyOurAction(code, true)},
			bindings: append(poppyOthers(), poppyBinding("act_ours", setup.OfflineAccessActionName)),
		},
		{
			name: "other code: update and deploy, the place is kept", mode: modePiped, flags: []string{"--always-issue-refresh-tokens", "--force"},
			actions:   []*management.Action{poppyOurAction("old code", true)},
			bindings:  append(poppyOthers(), poppyBinding("act_ours", setup.OfflineAccessActionName)),
			wantCalls: []string{"update", "deploy"},
		},
		{
			name: "current code that is not deployed: deploy", mode: modePiped, flags: []string{"--always-issue-refresh-tokens", "--force"},
			actions:   []*management.Action{poppyOurAction(code, false)},
			bindings:  append(poppyOthers(), poppyBinding("act_ours", setup.OfflineAccessActionName)),
			wantCalls: []string{"deploy"},
		},
		{
			name: "bound but not last: moved to last, the others keep their order", mode: modePiped, flags: []string{"--always-issue-refresh-tokens", "--force"},
			actions:   []*management.Action{poppyOurAction(code, true)},
			bindings:  []*management.ActionBinding{poppyBinding("act_a", "Enrich"), poppyBinding("act_ours", setup.OfflineAccessActionName), poppyBinding("act_b", "Audit")},
			wantCalls: []string{"bindings"},
			wantFlow:  []string{"act_a", "act_b", "act_ours"},
		},
		{
			name: "terminal, no flag, yes", mode: modeTerminal, flags: []string{"--force"}, answers: actionAnswers{refresh: true},
			bindings:  poppyOthers(),
			wantCalls: []string{"create", "deploy", "bindings"},
			wantFlow:  []string{"act_a", "act_b", "act_ours"}, wantRefresh: 1,
		},
		{
			name: "terminal, no flag, no", mode: modeTerminal, flags: []string{"--force"}, answers: actionAnswers{refresh: false},
			bindings: poppyOthers(), wantRefresh: 1,
		},
		{
			name: "terminal, the flag answers it", mode: modeTerminal, flags: []string{"--always-issue-refresh-tokens", "--force"}, answers: actionAnswers{refresh: false},
			bindings:  poppyOthers(),
			wantCalls: []string{"create", "deploy", "bindings"},
			wantFlow:  []string{"act_a", "act_b", "act_ours"},
		},
		{
			name: "terminal, =false asks nothing and changes nothing", mode: modeTerminal, flags: []string{"--always-issue-refresh-tokens=false", "--force"}, answers: actionAnswers{refresh: true},
			bindings: poppyOthers(),
		},
		{
			name: "piped without the flag leaves the flow alone", mode: modePiped, flags: []string{"--force"}, answers: actionAnswers{refresh: true},
			bindings: poppyOthers(),
		},
		{
			name: "--no-input without the flag leaves the flow alone", mode: modeTerminalNoInput, flags: []string{"--force"}, answers: actionAnswers{refresh: true},
			bindings: poppyOthers(),
		},
		{
			name: "agent mode without the flag leaves the flow alone", mode: modeAgent, flags: []string{"--force"}, answers: actionAnswers{refresh: true},
			bindings: poppyOthers(),
		},
		{
			name: "agent mode with the flag", mode: modeAgent, flags: []string{"--always-issue-refresh-tokens", "--force"},
			bindings:  poppyOthers(),
			wantCalls: []string{"create", "deploy", "bindings"},
			wantFlow:  []string{"act_a", "act_b", "act_ours"},
		},
		{
			name: "the person declines the plan", mode: modeTerminal, flags: []string{"--always-issue-refresh-tokens"}, answers: actionAnswers{apply: false},
			bindings: poppyOthers(), wantApply: 1,
		},
		{
			name: "the person accepts the plan", mode: modeTerminal, flags: []string{"--always-issue-refresh-tokens"}, answers: actionAnswers{apply: true},
			bindings:  poppyOthers(),
			wantCalls: []string{"create", "deploy", "bindings"},
			wantFlow:  []string{"act_a", "act_b", "act_ours"}, wantApply: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := newPoppySetupHarness(t, configuredPoppyTenant())
			h.allowWrites()

			tenant := &poppyActionTenant{actions: test.actions, bindings: test.bindings}
			h.serveActions(tenant)

			run, asked := h.runActions(t, test.mode, actionArgs(t, test.flags...), test.answers)

			require.NoError(t, run.err)
			assert.Equal(t, test.wantCalls, tenant.calls)
			assert.Equal(t, test.wantFlow, tenant.patchedFlow())
			assert.Equal(t, test.wantRefresh, asked["refresh"], "times the question was asked")
			assert.Equal(t, test.wantApply, asked["apply"], "times the plan was confirmed")
			assert.Empty(t, h.calls, "nothing else of the tenant was changed")
		})
	}
}

func TestPoppySetupRefreshTokensWithoutTheFlagCallsNoActionsAPI(t *testing.T) {
	// The harness has no calls on the Actions API, so any call fails the test.
	for _, mode := range []poppyPromptMode{modePiped, modePipedJSON, modeAgent, modeTerminalNoInput} {
		h := newPoppySetupHarness(t, freshPoppyTenant())
		h.allowWrites()

		run, asked := h.runActions(t, mode, freshSetupArgs(t, "--force", "--skip-doctor"), actionAnswers{refresh: true})

		require.NoError(t, run.err)
		assert.Zero(t, asked["refresh"])
		assert.NotContains(t, run.out, "Post-login", "the plan does not mention the Action")
		assert.NotContains(t, run.errOut, "Post-login")
	}
}

func TestPoppySetupRefreshTokensCallsTheRightRequests(t *testing.T) {
	h := newPoppySetupHarness(t, configuredPoppyTenant())
	h.allowWrites()

	tenant := &poppyActionTenant{bindings: []*management.ActionBinding{
		{ID: auth0.String("bnd_a"), DisplayName: auth0.String("Enrich profile"), Action: &management.Action{ID: auth0.String("act_a"), Name: auth0.String("Enrich")}},
		{ID: auth0.String("bnd_b"), Action: &management.Action{ID: auth0.String("act_b"), Name: auth0.String("Audit")}},
	}}
	h.serveActions(tenant)

	run, _ := h.runActions(t, modePiped, actionArgs(t, "--always-issue-refresh-tokens", "--force"), actionAnswers{})
	require.NoError(t, run.err)

	require.NotNil(t, tenant.created)
	assert.Equal(t, setup.OfflineAccessActionName, tenant.created.GetName())
	assert.Equal(t, "post-login", tenant.created.SupportedTriggers[0].GetID())
	assert.Equal(t, "v3", tenant.created.SupportedTriggers[0].GetVersion())
	assert.NotEmpty(t, tenant.created.GetCode())

	body, err := json.Marshal(map[string]any{"bindings": tenant.patched})
	require.NoError(t, err)
	assert.JSONEq(t, `{"bindings":[
		{"display_name":"Enrich profile","ref":{"type":"action_id","value":"act_a"}},
		{"ref":{"type":"action_id","value":"act_b"}},
		{"display_name":"Poppy: always issue refresh tokens","ref":{"type":"action_id","value":"act_ours"}}
	]}`, string(body))
}

func TestPoppySetupRefreshTokensFlowChangedWhileRunning(t *testing.T) {
	h := newPoppySetupHarness(t, configuredPoppyTenant())
	h.allowWrites()

	tenant := &poppyActionTenant{bindings: poppyOthers()}
	tenant.onDeploy = func() {
		tenant.bindings = append(tenant.bindings, poppyBinding("act_c", "Added by someone"))
	}
	h.serveActions(tenant)

	run, _ := h.runActions(t, modePiped, actionArgs(t, "--always-issue-refresh-tokens", "--force"), actionAnswers{})

	require.Error(t, run.err)
	assert.Contains(t, run.err.Error(), "The post-login flow changed while setup was running")
	assert.Equal(t, []string{"create", "deploy"}, tenant.calls, "the flow was not overwritten")
	assert.Nil(t, tenant.patched)
}

func TestPoppySetupRefreshTokensDryRun(t *testing.T) {
	t.Run("the plan shows the order, nothing is written", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		tenant := &poppyActionTenant{bindings: poppyOthers()}
		h.serveActions(tenant)

		run, _ := h.runActions(t, modePiped, actionArgs(t, "--always-issue-refresh-tokens", "--dry-run"), actionAnswers{})

		require.NoError(t, run.err)
		assert.Empty(t, tenant.calls)
		assert.Contains(t, run.out, "Post-login flow")
		assert.Contains(t, run.out, "refresh token Action")
		assert.Contains(t, run.out, "none -> deployed", "the plan shows the Action as new")
		assert.Contains(t, run.out, "not bound -> last")
		assert.Contains(t, run.out, "runs after: Enrich, Audit")
		assert.Contains(t, run.out, "auth0 actions delete", "the note says how to remove it")
		assert.Contains(t, run.out, "--always-issue-refresh-tokens=true", "the command to apply carries the flag")
		assert.Contains(t, run.out, "Refresh token always issued, by a post-login Action")
	})

	t.Run("the JSON has the change", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		h.serveActions(&poppyActionTenant{bindings: poppyOthers()})

		run, _ := h.runActions(t, modePipedJSON, actionArgs(t, "--always-issue-refresh-tokens", "--dry-run"), actionAnswers{})

		require.NoError(t, run.err)

		var plan struct {
			Changes []struct {
				Kind   string `json:"kind"`
				Impact string `json:"impact"`
				Fields []struct {
					Name   string `json:"name"`
					Value  string `json:"value"`
					Before string `json:"before"`
					Effect string `json:"effect"`
				} `json:"fields"`
			} `json:"changes"`
		}
		require.NoError(t, json.Unmarshal([]byte(run.out), &plan))
		require.Len(t, plan.Changes, 1)
		assert.Equal(t, "action", plan.Changes[0].Kind)
		assert.Equal(t, "tenant-wide", plan.Changes[0].Impact)
		require.Len(t, plan.Changes[0].Fields, 2)
		assert.Equal(t, "post_login_order", plan.Changes[0].Fields[1].Name)
		assert.Equal(t, "runs after: Enrich, Audit", plan.Changes[0].Fields[1].Effect)
	})

	t.Run("an Action that is in place is a plan with nothing to change", func(t *testing.T) {
		h := newPoppySetupHarness(t, configuredPoppyTenant())
		h.serveActions(&poppyActionTenant{
			actions:  []*management.Action{poppyOurAction(setup.OfflineAccessActionSource, true)},
			bindings: append(poppyOthers(), poppyBinding("act_ours", setup.OfflineAccessActionName)),
		})

		run, _ := h.runActions(t, modePiped, actionArgs(t, "--always-issue-refresh-tokens", "--dry-run"), actionAnswers{})

		require.NoError(t, run.err)
		assert.Contains(t, run.out, "Nothing to change")
	})
}

func TestPoppySetupRefreshTokensRefusesAFlowItCannotRepeat(t *testing.T) {
	h := newPoppySetupHarness(t, configuredPoppyTenant())
	h.allowWrites()

	withSecrets := poppyBinding("act_a", "Enrich")
	withSecrets.Secrets = []*management.ActionSecret{{Name: auth0.String("KEY")}}
	tenant := &poppyActionTenant{bindings: []*management.ActionBinding{withSecrets}}
	h.serveActions(tenant)

	run, _ := h.runActions(t, modePiped, actionArgs(t, "--always-issue-refresh-tokens", "--force"), actionAnswers{})

	reason, details := decodeCLIError(t, run.err)
	assert.Equal(t, setup.ReasonActionFlowUnsupported, reason)
	assert.Contains(t, details["fix"], "--always-issue-refresh-tokens=false")
	assert.Empty(t, tenant.calls, "the flow was not rewritten")
}

func TestPoppySetupRefreshTokensScopes(t *testing.T) {
	// SaveLogin makes the saved login hold the scopes.
	saveLogin := func(t *testing.T, scopes ...string) {
		t.Helper()

		keyring.MockInit()

		home := t.TempDir()
		t.Setenv("HOME", home)

		payload, err := json.Marshal(map[string]string{"scope": strings.Join(scopes, " ")})
		require.NoError(t, err)
		token := "e30." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"

		saved, err := json.Marshal(map[string]any{
			"default_tenant": poppySetupTenant,
			"tenants":        map[string]any{poppySetupTenant: map[string]any{"domain": poppySetupTenant, "access_token": token}},
		})
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Join(home, ".config", "auth0"), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(home, ".config", "auth0", "config.json"), saved, 0o600))
	}

	reads := append(append([]string{}, setup.ReadScopes...), setup.ActionReadScopes...)
	writes := []string{"update:tenant_settings", "update:resource_servers", "update:clients"}

	t.Run("the flag with no update:actions scope fails before anything is asked or changed", func(t *testing.T) {
		saveLogin(t, append(append(append([]string{}, reads...), writes...), "create:actions")...)

		h := newPoppySetupHarness(t, configuredPoppyTenant())
		h.allowWrites()
		tenant := &poppyActionTenant{bindings: poppyOthers()}
		h.serveActions(tenant)

		run, asked := h.runActions(t, modeTerminal, actionArgs(t, "--always-issue-refresh-tokens", "--force"), actionAnswers{})

		require.Error(t, run.err)
		assert.Equal(t, "missing_scopes", errorReason(run.err))
		assert.Contains(t, run.err.Error(), "auth0 login --scopes update:actions")
		assert.Empty(t, asked)
		assert.Empty(t, tenant.calls)
		assert.Empty(t, h.calls)
	})

	t.Run("a dry run only needs read:actions", func(t *testing.T) {
		saveLogin(t, reads...)

		h := newPoppySetupHarness(t, configuredPoppyTenant())
		h.serveActions(&poppyActionTenant{bindings: poppyOthers()})

		run, _ := h.runActions(t, modePiped, actionArgs(t, "--always-issue-refresh-tokens", "--dry-run"), actionAnswers{})

		require.NoError(t, run.err)
	})

	t.Run("a dry run with no read:actions scope is told so", func(t *testing.T) {
		saveLogin(t, setup.ReadScopes...)

		h := newPoppySetupHarness(t, configuredPoppyTenant())
		h.serveActions(&poppyActionTenant{})

		run, _ := h.runActions(t, modePiped, actionArgs(t, "--always-issue-refresh-tokens", "--dry-run"), actionAnswers{})

		require.Error(t, run.err)
		assert.Contains(t, run.err.Error(), "auth0 login --scopes read:actions")
	})

	t.Run("the scopes of the Actions are not asked of a run without the option", func(t *testing.T) {
		saveLogin(t, append(append([]string{}, setup.ReadScopes...), writes...)...)

		h := newPoppySetupHarness(t, configuredPoppyTenant())
		h.allowWrites()

		run, _ := h.runActions(t, modePiped, actionArgs(t, "--force"), actionAnswers{})

		require.NoError(t, run.err)
	})

	t.Run("a yes to the question with no update:actions scope fails before anything is changed", func(t *testing.T) {
		saveLogin(t, append(append([]string{}, reads...), writes...)...)

		h := newPoppySetupHarness(t, configuredPoppyTenant())
		h.allowWrites()
		tenant := &poppyActionTenant{bindings: poppyOthers()}
		h.serveActions(tenant)

		run, asked := h.runActions(t, modeTerminal, actionArgs(t, "--force"), actionAnswers{refresh: true})

		require.Error(t, run.err)
		assert.Equal(t, "missing_scopes", errorReason(run.err))
		assert.Contains(t, run.err.Error(), "create:actions,update:actions")
		assert.Equal(t, 1, asked["refresh"])
		assert.Empty(t, tenant.calls)
		assert.Empty(t, h.calls)
	})

	t.Run("a no to the question needs no scope of the Actions", func(t *testing.T) {
		saveLogin(t, append(append([]string{}, setup.ReadScopes...), writes...)...)

		h := newPoppySetupHarness(t, configuredPoppyTenant())
		h.allowWrites()

		run, asked := h.runActions(t, modeTerminal, actionArgs(t, "--force"), actionAnswers{refresh: false})

		require.NoError(t, run.err)
		assert.Equal(t, 1, asked["refresh"])
	})
}

func TestConfirmPoppyReasonWithTheAction(t *testing.T) {
	in := setup.Inputs{Out: "poppy.json"}

	tests := []struct {
		name  string
		kinds []setup.Kind
		want  string
	}{
		{name: "no wide change", kinds: []setup.Kind{setup.KindAPI}, want: "This changes your Auth0 tenant and writes poppy.json."},
		{name: "tenant settings", kinds: []setup.Kind{setup.KindTenant}, want: "This changes your Auth0 tenant and writes poppy.json. The tenant settings affect every app."},
		{name: "the Action", kinds: []setup.Kind{setup.KindAction}, want: "This changes your Auth0 tenant and writes poppy.json. The post-login Action runs on every sign-in."},
		{
			name: "both", kinds: []setup.Kind{setup.KindTenant, setup.KindAction},
			want: "This changes your Auth0 tenant and writes poppy.json. The tenant settings affect every app. The post-login Action runs on every sign-in.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var plan setup.Plan
			for _, kind := range test.kinds {
				impact := setup.ImpactAPIWide
				if kind == setup.KindTenant || kind == setup.KindAction {
					impact = setup.ImpactTenantWide
				}
				plan.Changes = append(plan.Changes, setup.Change{Kind: kind, Impact: impact})
			}

			assert.Equal(t, test.want, confirmPoppyReason(plan, in))
		})
	}
}

func TestPoppyAlwaysIssueRefreshTokensWording(t *testing.T) {
	assert.Contains(t, poppyRefreshTokensReason, "can add offline access when a Poppy agent signs in")
	assert.Contains(t, poppyRefreshTokensReason, "last Action")
	assert.Contains(t, poppyRefreshTokensReason, "Saying no does not remove an Action that is already there.")
	assert.True(t, strings.HasSuffix(poppyRefreshTokensText, "?"))
	assert.Equal(t, []string{"read:actions"}, poppyActionScopes(true))
	assert.Equal(t, []string{"read:actions", "create:actions", "update:actions"}, poppyActionScopes(false))
}
