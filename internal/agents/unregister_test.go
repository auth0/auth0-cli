package agents

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPlanRemoval(t *testing.T) {
	boom := errors.New("boom")
	museRecord := agentRecord(museAgent, "Meta Muse", museURL)
	shopGrant := grantOf("cgr_shop", shopAPI, "orders:read")
	otherGrant := grantOf("cgr_other", otherAPI, "refund")
	plain := &managementv3.Client{ClientID: ptr("client_plain"), Name: ptr("Plain"), AgentID: ptr("agt_plain")}

	shop := Grant{ID: "cgr_shop", Audience: shopAPI, Scopes: []string{"orders:read"}}
	other := Grant{ID: "cgr_other", Audience: otherAPI, Scopes: []string{"refund"}}

	tests := []struct {
		name      string
		ref       string
		app       *managementv3.Client
		narrowed  bool
		filterErr error
		recordErr error
		// The agent record found when no app links to the reference.
		orphan    *managementv3.AgentResponseContent
		orphanErr error
		grants    []*managementv3.ClientGrantResponseContent
		grantErr  error
		want      *Removal
		notAgent  *NotAnAgentError
		wantError string
	}{
		{
			name:   "every grant of the app",
			ref:    museURL,
			app:    agentApp(museApp, museURL, museAgent),
			grants: []*managementv3.ClientGrantResponseContent{shopGrant, otherGrant},
			want: &Removal{
				URL: museURL, Name: "Meta Muse", ClientID: museApp, AgentID: museAgent,
				Grants: []Grant{shop, other}, OtherGrants: []Grant{},
			},
		},
		{
			name: "an agent app with no grant at all",
			ref:  museURL,
			app:  agentApp(museApp, museURL, museAgent),
			want: &Removal{
				URL: museURL, Name: "Meta Muse", ClientID: museApp, AgentID: museAgent,
				Grants: []Grant{}, OtherGrants: []Grant{},
			},
		},
		{
			name:   "by agent ID",
			ref:    museAgent,
			app:    agentApp(museApp, museURL, museAgent),
			grants: []*managementv3.ClientGrantResponseContent{shopGrant},
			want: &Removal{
				URL: museURL, Name: "Meta Muse", ClientID: museApp, AgentID: museAgent,
				Grants: []Grant{shop}, OtherGrants: []Grant{},
			},
		},
		{
			name:      "agent record gone",
			ref:       museURL,
			app:       agentApp(museApp, museURL, museAgent),
			recordErr: apiError(404, `{"errorCode":"inexistent_agent"}`),
			grants:    []*managementv3.ClientGrantResponseContent{shopGrant},
			want: &Removal{
				URL: museURL, Name: "App " + museApp, ClientID: museApp,
				Grants: []Grant{shop}, OtherGrants: []Grant{},
			},
		},
		{
			name:     "narrowed, the grants on other APIs are left alone",
			ref:      museURL,
			app:      agentApp(museApp, museURL, museAgent),
			narrowed: true,
			grants:   []*managementv3.ClientGrantResponseContent{shopGrant, otherGrant},
			want: &Removal{
				URL: museURL, Name: "Meta Muse", ClientID: museApp, AgentID: museAgent,
				Grants: []Grant{shop}, OtherGrants: []Grant{other},
			},
		},
		{
			name:     "narrowed, no grant on those APIs",
			ref:      museURL,
			app:      agentApp(museApp, museURL, museAgent),
			narrowed: true,
			grants:   []*managementv3.ClientGrantResponseContent{otherGrant},
			notAgent: &NotAnAgentError{Ref: museURL, NoGrants: true},
		},
		{
			name:     "narrowed, no grant at all, as a removal that stopped partway left it",
			ref:      museURL,
			app:      agentApp(museApp, museURL, museAgent),
			narrowed: true,
			want: &Removal{
				URL: museURL, Name: "Meta Muse", ClientID: museApp, AgentID: museAgent,
				Grants: []Grant{}, OtherGrants: []Grant{},
			},
		},
		{
			name:      "narrowed, the audiences cannot be read",
			ref:       museURL,
			app:       agentApp(museApp, museURL, museAgent),
			narrowed:  true,
			filterErr: boom,
			wantError: "boom",
		},
		{
			name:     "not an agent app",
			ref:      "agt_plain",
			app:      plain,
			notAgent: &NotAnAgentError{Ref: "agt_plain", AgentID: "agt_plain"},
		},
		{
			name:      "grants cannot be listed",
			ref:       museURL,
			app:       agentApp(museApp, museURL, museAgent),
			grantErr:  boom,
			wantError: "failed to list the grants of " + museApp + ": boom",
		},
		{
			name:      "record read fails",
			ref:       museURL,
			app:       agentApp(museApp, museURL, museAgent),
			recordErr: apiError(500, "server error"),
			wantError: "failed to read agent " + museAgent,
		},
		{
			name:   "a record without an app, by URL",
			ref:    museURL,
			orphan: museRecord,
			want:   &Removal{URL: museURL, Name: "Meta Muse", AgentID: museAgent, Grants: []Grant{}, OtherGrants: []Grant{}},
		},
		{
			name:   "a record without an app, by agent ID",
			ref:    museAgent,
			orphan: museRecord,
			want:   &Removal{URL: museURL, Name: "Meta Muse", AgentID: museAgent, Grants: []Grant{}, OtherGrants: []Grant{}},
		},
		{
			name:     "a record without an app, narrowed",
			ref:      museURL,
			narrowed: true,
			orphan:   museRecord,
			want:     &Removal{URL: museURL, Name: "Meta Muse", AgentID: museAgent, Grants: []Grant{}, OtherGrants: []Grant{}},
		},
		{
			name:      "a record without an app, the agents cannot be listed",
			ref:       museURL,
			orphanErr: boom,
			wantError: "failed to list agents: boom",
		},
		{
			name:      "a record without an app, the record cannot be read",
			ref:       museAgent,
			orphanErr: apiError(500, "server error"),
			wantError: "failed to read agent " + museAgent,
		},
		{
			name: "gone, by URL",
			ref:  museURL,
			want: &Removal{URL: museURL, Gone: true, Grants: []Grant{}, OtherGrants: []Grant{}},
		},
		{
			name:      "gone, by agent ID",
			ref:       museAgent,
			orphanErr: apiError(404, `{"errorCode":"inexistent_agent"}`),
			want:      &Removal{Gone: true, Grants: []Grant{}, OtherGrants: []Grant{}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			expectFind(t, f, test.ref, appsOf(test.app)...)

			switch {
			case test.app != nil && test.app.GetAgentID() != "":
				f.agents.EXPECT().Read(gomock.Any(), test.app.GetAgentID()).Return(museRecord, test.recordErr)
			case test.app == nil && IsURL(test.ref):
				// Without an app, the record is looked up in the list of agents.
				var records []*managementv3.AgentResponseContent
				if test.orphan != nil {
					records = append(records, test.orphan)
				}

				f.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(agentPage(records...), test.orphanErr)
			case test.app == nil:
				f.agents.EXPECT().Read(gomock.Any(), test.ref).Return(test.orphan, test.orphanErr)
			}

			// The resource servers are never read here: the filter is the caller's.
			var only AudienceFilter
			filterCalls := 0
			if test.narrowed {
				only = func(context.Context) (map[string]bool, error) {
					filterCalls++

					return map[string]bool{shopAPI: true}, test.filterErr
				}
			}

			reachesGrants := test.app != nil && test.app != plain && test.filterErr == nil && (test.recordErr == nil || statusOf(test.recordErr) == 404)
			if reachesGrants {
				f.grants.EXPECT().List(gomock.Any(), &managementv3.ListClientGrantsRequestParameters{ClientID: test.app.ClientID}).Return(grantPage(test.grants...), test.grantErr)
			}

			got, err := f.svc.PlanRemoval(t.Context(), test.ref, only)

			switch {
			case test.notAgent != nil:
				var notAgent *NotAnAgentError
				require.ErrorAs(t, err, &notAgent)
				assert.Equal(t, test.notAgent, notAgent)
				assert.Nil(t, got)
			case test.wantError != "":
				assert.ErrorContains(t, err, test.wantError)
				assert.Nil(t, got)
			default:
				require.NoError(t, err)
				assert.Equal(t, test.want, got)
			}
		})
	}
}

func TestNotAnAgentError(t *testing.T) {
	assert.EqualError(t, &NotAnAgentError{Ref: museURL}, museURL+" isn't an agent. Nothing removed.")
	assert.EqualError(t, &NotAnAgentError{Ref: museURL, NoGrants: true}, museURL+" has no grant to remove. Nothing removed.")
}

func removalWith(grants, other int) *Removal {
	removal := &Removal{URL: museURL, Name: "Meta Muse", ClientID: museApp, AgentID: museAgent, Grants: []Grant{}, OtherGrants: []Grant{}}

	for i := range grants {
		removal.Grants = append(removal.Grants, Grant{ID: "cgr_g" + string(rune('1'+i)), Audience: shopAPI})
	}
	for i := range other {
		removal.OtherGrants = append(removal.OtherGrants, Grant{ID: "cgr_o" + string(rune('1'+i)), Audience: otherAPI})
	}

	return removal
}

func TestDecide(t *testing.T) {
	gone := &Removal{Gone: true, Grants: []Grant{}, OtherGrants: []Grant{}}

	tests := []struct {
		name        string
		removal     *Removal
		force       bool
		interactive bool
		want        Decision
	}{
		{name: "gone, no terminal", removal: gone, want: Decision{Approval: Proceed}},
		{name: "gone, terminal", removal: gone, interactive: true, want: Decision{Approval: Proceed}},
		{name: "gone, forced", removal: gone, force: true, want: Decision{Approval: Proceed}},
		{name: "no other grants, no terminal", removal: removalWith(1, 0), want: Decision{Approval: Refuse, DeleteApp: true}},
		{name: "no other grants, forced", removal: removalWith(1, 0), force: true, want: Decision{Approval: Proceed, DeleteApp: true}},
		{name: "no other grants, terminal", removal: removalWith(1, 0), interactive: true, want: Decision{Approval: Ask, DeleteApp: true}},
		{name: "no grant to remove, terminal", removal: removalWith(0, 0), interactive: true, want: Decision{Approval: Ask, DeleteApp: true}},
		{name: "other grants, no terminal", removal: removalWith(1, 1), want: Decision{Approval: Refuse}},
		{name: "other grants, forced", removal: removalWith(2, 1), force: true, want: Decision{Approval: Proceed}},
		{name: "other grants, terminal", removal: removalWith(1, 1), interactive: true, want: Decision{Approval: Ask}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, Decide(test.removal, test.force, test.interactive))
		})
	}
}

// grantItem is the item of a client grant.
func grantItem(id, audience string) Item {
	return Item{Kind: KindClientGrant, ID: id, Audience: audience}
}

var (
	recordItem = Item{Kind: KindAgentRecord, ID: museAgent}
	appItem    = Item{Kind: KindAgentApp, ID: museApp}
)

func TestPlanMakesNoCalls(t *testing.T) {
	// The fixture has no expectations, so any call to the API fails the test.
	f := newFixture(t)
	removal := removalWith(2, 0)
	got := f.svc.Plan(removal, RemoveOptions{DeleteApp: true})

	assert.True(t, got.DryRun)
	assert.Equal(t, []Item{
		grantItem("cgr_g1", shopAPI),
		grantItem("cgr_g2", shopAPI),
		recordItem,
		appItem,
	}, got.Removed)
	assert.Empty(t, got.Kept)
}

func TestPlanOutcome(t *testing.T) {
	f := newFixture(t)

	got := f.svc.Plan(removalWith(1, 1), RemoveOptions{})

	assert.Equal(t, []Item{grantItem("cgr_g1", shopAPI)}, got.Removed)
	assert.Equal(t, []Item{recordItem, appItem}, got.Kept)
	assert.Equal(t, []Grant{{ID: "cgr_o1", Audience: otherAPI}}, got.RemainingGrants)

	gone := f.svc.Plan(&Removal{URL: museURL, Gone: true, Grants: []Grant{}, OtherGrants: []Grant{}}, RemoveOptions{DeleteApp: true})

	assert.True(t, gone.Gone)
	assert.Empty(t, gone.Removed)
	assert.Empty(t, gone.Kept)
}

func TestItemJSON(t *testing.T) {
	got, err := json.Marshal(RemoveResult{
		URL:             museURL,
		Removed:         []Item{grantItem("cgr_g1", shopAPI), recordItem},
		Kept:            []Item{appItem},
		RemainingGrants: []Grant{},
	})

	require.NoError(t, err)
	assert.JSONEq(t, `{
		"url": "`+museURL+`",
		"dry_run": false,
		"gone": false,
		"removed": [
			{"kind": "client_grant", "id": "cgr_g1", "audience": "`+shopAPI+`"},
			{"kind": "agent_record", "id": "`+museAgent+`"}
		],
		"kept": [{"kind": "agent_app", "id": "`+museApp+`"}],
		"remaining_grants": []
	}`, string(got))

	assert.Equal(t, "client grant cgr_g1 ("+shopAPI+")", grantItem("cgr_g1", shopAPI).String())
	assert.Equal(t, "agent record "+museAgent, recordItem.String())
	assert.Equal(t, "agent app "+museApp, appItem.String())
}

// TestRemove asserts exactly which Delete calls happen, in which order. The
// fixture's mocks fail the test on any call that is not expected.
func TestRemove(t *testing.T) {
	t.Run("the grants, the record and the app, in order", func(t *testing.T) {
		f := newFixture(t)
		removal := removalWith(2, 0)

		gomock.InOrder(
			f.grants.EXPECT().Delete(gomock.Any(), "cgr_g1").Return(nil),
			f.grants.EXPECT().Delete(gomock.Any(), "cgr_g2").Return(nil),
			f.agents.EXPECT().Delete(gomock.Any(), museAgent).Return(nil),
			f.clients.EXPECT().Delete(gomock.Any(), museApp).Return(nil),
		)

		got, err := f.svc.Remove(t.Context(), removal, RemoveOptions{DeleteApp: true})

		require.NoError(t, err)
		assert.False(t, got.DryRun)
		assert.Equal(t, []Item{grantItem("cgr_g1", shopAPI), grantItem("cgr_g2", shopAPI), recordItem, appItem}, got.Removed)
		assert.Empty(t, got.Kept)
		assert.Empty(t, got.RemainingGrants)
	})

	t.Run("only the grants, when other grants remain", func(t *testing.T) {
		f := newFixture(t)
		removal := removalWith(2, 1)

		// No Delete on the agent record, the app or the other grant.
		f.grants.EXPECT().Delete(gomock.Any(), "cgr_g1").Return(nil)
		f.grants.EXPECT().Delete(gomock.Any(), "cgr_g2").Return(nil)

		got, err := f.svc.Remove(t.Context(), removal, RemoveOptions{})

		require.NoError(t, err)
		assert.Equal(t, []Item{grantItem("cgr_g1", shopAPI), grantItem("cgr_g2", shopAPI)}, got.Removed)
		assert.Equal(t, []Item{recordItem, appItem}, got.Kept)
		assert.Equal(t, []Grant{{ID: "cgr_o1", Audience: otherAPI}}, got.RemainingGrants)
	})

	t.Run("an app without a record", func(t *testing.T) {
		f := newFixture(t)
		removal := removalWith(1, 0)
		removal.AgentID = ""

		gomock.InOrder(
			f.grants.EXPECT().Delete(gomock.Any(), "cgr_g1").Return(nil),
			f.clients.EXPECT().Delete(gomock.Any(), museApp).Return(nil),
		)

		got, err := f.svc.Remove(t.Context(), removal, RemoveOptions{DeleteApp: true})

		require.NoError(t, err)
		assert.Equal(t, []Item{grantItem("cgr_g1", shopAPI), appItem}, got.Removed)
	})

	t.Run("an app without a grant, as a removal that stopped partway left it", func(t *testing.T) {
		f := newFixture(t)

		gomock.InOrder(
			f.agents.EXPECT().Delete(gomock.Any(), museAgent).Return(nil),
			f.clients.EXPECT().Delete(gomock.Any(), museApp).Return(nil),
		)

		got, err := f.svc.Remove(t.Context(), removalWith(0, 0), RemoveOptions{DeleteApp: true})

		require.NoError(t, err)
		assert.Equal(t, []Item{recordItem, appItem}, got.Removed)
	})

	t.Run("a record without an app", func(t *testing.T) {
		f := newFixture(t)
		removal := &Removal{URL: museURL, Name: "Meta Muse", AgentID: museAgent, Grants: []Grant{}, OtherGrants: []Grant{}}

		// Only the record goes: there is no app and no grant to delete.
		f.agents.EXPECT().Delete(gomock.Any(), museAgent).Return(nil)

		got, err := f.svc.Remove(t.Context(), removal, RemoveOptions{DeleteApp: true})

		require.NoError(t, err)
		assert.Equal(t, []Item{recordItem}, got.Removed)
		assert.Empty(t, got.Kept)
	})

	t.Run("the agent is already gone", func(t *testing.T) {
		f := newFixture(t)
		removal := &Removal{URL: museURL, Gone: true, Grants: []Grant{}, OtherGrants: []Grant{}}

		// No Delete call at all.
		got, err := f.svc.Remove(t.Context(), removal, RemoveOptions{DeleteApp: true})

		require.NoError(t, err)
		assert.True(t, got.Gone)
		assert.Empty(t, got.Removed)
	})
}

func TestRemoveStopsAtTheFirstFailure(t *testing.T) {
	boom := errors.New("boom")

	tests := []struct {
		name          string
		calls         func(f *fixture)
		wantError     string
		wantApplied   []string
		wantFailed    string
		wantRemaining []string
	}{
		{
			name: "the first grant",
			calls: func(f *fixture) {
				// Nothing else is deleted.
				f.grants.EXPECT().Delete(gomock.Any(), "cgr_g1").Return(boom)
			},
			wantError: "failed to delete client grant cgr_g1 (" + shopAPI + "): boom",
		},
		{
			name: "the second grant",
			calls: func(f *fixture) {
				gomock.InOrder(
					f.grants.EXPECT().Delete(gomock.Any(), "cgr_g1").Return(nil),
					f.grants.EXPECT().Delete(gomock.Any(), "cgr_g2").Return(boom),
				)
			},
			wantApplied:   []string{"delete client grant cgr_g1 (" + shopAPI + ")"},
			wantFailed:    "delete client grant cgr_g2 (" + shopAPI + ")",
			wantRemaining: []string{"delete agent record " + museAgent, "delete agent app " + museApp},
		},
		{
			name: "the agent record",
			calls: func(f *fixture) {
				f.grants.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(nil).Times(2)
				f.agents.EXPECT().Delete(gomock.Any(), museAgent).Return(boom)
			},
			wantApplied: []string{
				"delete client grant cgr_g1 (" + shopAPI + ")",
				"delete client grant cgr_g2 (" + shopAPI + ")",
			},
			wantFailed:    "delete agent record " + museAgent,
			wantRemaining: []string{"delete agent app " + museApp},
		},
		{
			name: "the agent app",
			calls: func(f *fixture) {
				f.grants.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(nil).Times(2)
				f.agents.EXPECT().Delete(gomock.Any(), museAgent).Return(nil)
				f.clients.EXPECT().Delete(gomock.Any(), museApp).Return(boom)
			},
			wantApplied: []string{
				"delete client grant cgr_g1 (" + shopAPI + ")",
				"delete client grant cgr_g2 (" + shopAPI + ")",
				"delete agent record " + museAgent,
			},
			wantFailed:    "delete agent app " + museApp,
			wantRemaining: []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			test.calls(f)

			got, err := f.svc.Remove(t.Context(), removalWith(2, 0), RemoveOptions{DeleteApp: true})

			require.Error(t, err)
			assert.Nil(t, got)

			if test.wantError != "" {
				assert.EqualError(t, err, test.wantError)
				return
			}

			var partial *PartialError
			require.ErrorAs(t, err, &partial)
			assert.Equal(t, test.wantApplied, partial.Applied)
			assert.Equal(t, test.wantFailed, partial.Failed)
			assert.Equal(t, test.wantRemaining, partial.Remaining)
		})
	}
}

func TestCandidates(t *testing.T) {
	muse := agentApp(museApp, museURL, museAgent)
	shopApp := agentApp("client_shop", "https://shop.example/agent.json", "agt_shop")
	m2m := &managementv3.Client{ClientID: ptr("client_m2m"), Name: ptr("M2M")}

	grantsOfApp := func(clientID, id, audience string) *managementv3.ClientGrantResponseContent {
		return grantFor(clientID, grantOf(id, audience, "x"))
	}

	t.Run("every agent app, whatever its grants", func(t *testing.T) {
		f := newFixture(t)
		f.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(clientPage(muse, m2m, shopApp), nil)
		f.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(agentPage(agentRecord(museAgent, "Meta Muse", museURL)), nil)

		got, err := f.svc.Candidates(t.Context(), nil)

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, museApp, got[0].Client.GetClientID())
		assert.Equal(t, museAgent, got[0].Record.GetAgentID())
		assert.Equal(t, "client_shop", got[1].Client.GetClientID())
		assert.Nil(t, got[1].Record)
	})

	t.Run("narrowed, leaves out the agents whose grants are all on other APIs", func(t *testing.T) {
		f := newFixture(t)
		f.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(clientPage(muse, shopApp), nil)
		f.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(agentPage(), nil)
		// One call for all the agents.
		f.grants.EXPECT().List(gomock.Any(), gomock.Any()).Return(grantPage(grantsOfApp(museApp, "cgr_a", otherAPI)), nil).Times(1)

		only := func(context.Context) (map[string]bool, error) { return map[string]bool{shopAPI: true}, nil }

		got, err := f.svc.Candidates(t.Context(), only)

		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "client_shop", got[0].Client.GetClientID())
	})

	t.Run("narrowed, keeps the agents with a grant on those APIs", func(t *testing.T) {
		f := newFixture(t)
		f.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(clientPage(muse, shopApp), nil)
		f.agents.EXPECT().List(gomock.Any(), gomock.Any()).Return(agentPage(), nil)
		f.grants.EXPECT().List(gomock.Any(), gomock.Any()).Return(grantPage(
			grantsOfApp(museApp, "cgr_a", shopAPI),
			grantsOfApp(museApp, "cgr_b", otherAPI),
		), nil)

		only := func(context.Context) (map[string]bool, error) { return map[string]bool{shopAPI: true}, nil }

		got, err := f.svc.Candidates(t.Context(), only)

		require.NoError(t, err)
		require.Len(t, got, 2)
	})

	t.Run("the clients cannot be listed", func(t *testing.T) {
		f := newFixture(t)
		f.clients.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))

		got, err := f.svc.Candidates(t.Context(), nil)

		assert.ErrorContains(t, err, "failed to list the agent apps: boom")
		assert.Nil(t, got)
	})
}
