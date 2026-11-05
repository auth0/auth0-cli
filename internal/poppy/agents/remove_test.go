package agents

import (
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
		recordErr error
		grants    []*managementv3.ClientGrantResponseContent
		grantErr  error
		want      *Removal
		notPoppy  bool
		wantError string
	}{
		{
			name:   "Poppy grants only",
			ref:    museURL,
			app:    agentApp(museApp, museURL, museAgent),
			grants: []*managementv3.ClientGrantResponseContent{shopGrant},
			want: &Removal{
				URL: museURL, Name: "Meta Muse", ClientID: museApp, AgentID: museAgent,
				PoppyGrants: []Grant{shop}, OtherGrants: []Grant{},
			},
		},
		{
			name:   "by agent ID, with other grants",
			ref:    museAgent,
			app:    agentApp(museApp, museURL, museAgent),
			grants: []*managementv3.ClientGrantResponseContent{shopGrant, otherGrant},
			want: &Removal{
				URL: museURL, Name: "Meta Muse", ClientID: museApp, AgentID: museAgent,
				PoppyGrants: []Grant{shop}, OtherGrants: []Grant{other},
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
				PoppyGrants: []Grant{shop}, OtherGrants: []Grant{},
			},
		},
		{
			name:     "not an agent app",
			ref:      "agt_plain",
			app:      plain,
			notPoppy: true,
		},
		{
			name:     "no Poppy grant",
			ref:      museURL,
			app:      agentApp(museApp, museURL, museAgent),
			grants:   []*managementv3.ClientGrantResponseContent{otherGrant},
			notPoppy: true,
		},
		{
			name:     "no grant at all",
			ref:      museURL,
			app:      agentApp(museApp, museURL, museAgent),
			notPoppy: true,
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
			name: "gone, by URL",
			ref:  museURL,
			want: &Removal{URL: museURL, Gone: true, PoppyGrants: []Grant{}, OtherGrants: []Grant{}},
		},
		{
			name: "gone, by agent ID",
			ref:  museAgent,
			want: &Removal{Gone: true, PoppyGrants: []Grant{}, OtherGrants: []Grant{}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			expectFind(t, f, test.ref, appsOf(test.app)...)

			if test.app != nil && test.app.GetAgentID() != "" {
				f.agents.EXPECT().Read(gomock.Any(), test.app.GetAgentID()).Return(museRecord, test.recordErr)
			}

			if test.app != nil && test.app != plain && (test.recordErr == nil || statusOf(test.recordErr) == 404) {
				f.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(serverPage(shopServer()), nil)
				f.grants.EXPECT().List(gomock.Any(), &managementv3.ListClientGrantsRequestParameters{ClientID: test.app.ClientID}).Return(grantPage(test.grants...), test.grantErr)
			}

			got, err := f.svc.PlanRemoval(t.Context(), test.ref)

			switch {
			case test.notPoppy:
				var notPoppy *NotPoppyAgentError
				require.ErrorAs(t, err, &notPoppy)
				assert.Equal(t, test.ref, notPoppy.Ref)
				assert.Equal(t, test.ref+" isn't a Poppy agent. Nothing removed.", err.Error())
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

func removalWith(poppy, other int) *Removal {
	removal := &Removal{URL: museURL, Name: "Meta Muse", ClientID: museApp, AgentID: museAgent, PoppyGrants: []Grant{}, OtherGrants: []Grant{}}

	for i := range poppy {
		removal.PoppyGrants = append(removal.PoppyGrants, Grant{ID: "cgr_p" + string(rune('1'+i)), Audience: shopAPI})
	}
	for i := range other {
		removal.OtherGrants = append(removal.OtherGrants, Grant{ID: "cgr_o" + string(rune('1'+i)), Audience: otherAPI})
	}

	return removal
}

func TestDecide(t *testing.T) {
	gone := &Removal{Gone: true, PoppyGrants: []Grant{}, OtherGrants: []Grant{}}

	tests := []struct {
		name        string
		removal     *Removal
		removeAll   bool
		force       bool
		interactive bool
		want        Decision
	}{
		{name: "gone, no terminal", removal: gone, want: Decision{Approval: Proceed}},
		{name: "gone, terminal", removal: gone, interactive: true, want: Decision{Approval: Proceed}},
		{name: "gone, forced", removal: gone, force: true, want: Decision{Approval: Proceed}},
		{name: "no other grants, no terminal", removal: removalWith(1, 0), want: Decision{Approval: Refuse, Everything: true}},
		{name: "no other grants, forced", removal: removalWith(1, 0), force: true, want: Decision{Approval: Proceed, Everything: true}},
		{name: "no other grants, terminal", removal: removalWith(1, 0), interactive: true, want: Decision{Approval: Ask, Everything: true}},
		{name: "no other grants, terminal, remove all", removal: removalWith(1, 0), removeAll: true, interactive: true, want: Decision{Approval: Ask, Everything: true}},
		{name: "other grants, no terminal", removal: removalWith(1, 1), want: Decision{Approval: Refuse}},
		{name: "other grants, no terminal, remove all", removal: removalWith(1, 1), removeAll: true, want: Decision{Approval: Refuse, Everything: true}},
		{name: "other grants, forced", removal: removalWith(2, 1), force: true, want: Decision{Approval: Proceed}},
		{name: "other grants, forced, remove all", removal: removalWith(1, 2), force: true, removeAll: true, want: Decision{Approval: Proceed, Everything: true}},
		{name: "other grants, terminal", removal: removalWith(1, 1), interactive: true, want: Decision{Approval: Choose}},
		{name: "other grants, terminal, remove all", removal: removalWith(1, 1), removeAll: true, interactive: true, want: Decision{Approval: Ask, Everything: true}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, Decide(test.removal, test.removeAll, test.force, test.interactive))
		})
	}
}

func TestPlanMakesNoCalls(t *testing.T) {
	f := newFixture(t)
	removal := removalWith(1, 1)
	got := f.svc.Plan(removal, RemoveOptions{Everything: true})

	assert.True(t, got.DryRun)
	assert.Equal(t, []string{
		"client grant cgr_p1 (" + shopAPI + ")",
		"client grant cgr_o1 (" + otherAPI + ")",
		"agent record " + museAgent,
		"agent app " + museApp,
	}, got.Removed)
	assert.Empty(t, got.Kept)
}

func TestPlanOutcome(t *testing.T) {
	f := newFixture(t)

	got := f.svc.Plan(removalWith(1, 1), RemoveOptions{})

	assert.Equal(t, []string{"client grant cgr_p1 (" + shopAPI + ")"}, got.Removed)
	assert.Equal(t, []string{"agent record " + museAgent, "agent app " + museApp}, got.Kept)
	assert.Equal(t, []Grant{{ID: "cgr_o1", Audience: otherAPI}}, got.RemainingGrants)

	gone := f.svc.Plan(&Removal{URL: museURL, Gone: true, PoppyGrants: []Grant{}, OtherGrants: []Grant{}}, RemoveOptions{Everything: true})

	assert.True(t, gone.Gone)
	assert.Empty(t, gone.Removed)
	assert.Empty(t, gone.Kept)
}

func TestRemove(t *testing.T) {
	t.Run("everything, in order", func(t *testing.T) {
		f := newFixture(t)
		removal := removalWith(1, 1)

		gomock.InOrder(
			f.grants.EXPECT().Delete(gomock.Any(), "cgr_p1").Return(nil),
			f.grants.EXPECT().Delete(gomock.Any(), "cgr_o1").Return(nil),
			f.agents.EXPECT().Delete(gomock.Any(), museAgent).Return(nil),
			f.clients.EXPECT().Delete(gomock.Any(), museApp).Return(nil),
		)

		got, err := f.svc.Remove(t.Context(), removal, RemoveOptions{Everything: true})

		require.NoError(t, err)
		assert.False(t, got.DryRun)
		assert.Len(t, got.Removed, 4)
		assert.Empty(t, got.Kept)
		assert.Empty(t, got.RemainingGrants)
	})

	t.Run("only the Poppy grants", func(t *testing.T) {
		f := newFixture(t)
		removal := removalWith(2, 1)

		f.grants.EXPECT().Delete(gomock.Any(), "cgr_p1").Return(nil)
		f.grants.EXPECT().Delete(gomock.Any(), "cgr_p2").Return(nil)

		got, err := f.svc.Remove(t.Context(), removal, RemoveOptions{})

		require.NoError(t, err)
		assert.Equal(t, []string{"client grant cgr_p1 (" + shopAPI + ")", "client grant cgr_p2 (" + shopAPI + ")"}, got.Removed)
		assert.Equal(t, []string{"agent record " + museAgent, "agent app " + museApp}, got.Kept)
		assert.Equal(t, []Grant{{ID: "cgr_o1", Audience: otherAPI}}, got.RemainingGrants)
	})

	t.Run("an app without a record", func(t *testing.T) {
		f := newFixture(t)
		removal := removalWith(1, 0)
		removal.AgentID = ""

		f.grants.EXPECT().Delete(gomock.Any(), "cgr_p1").Return(nil)
		f.clients.EXPECT().Delete(gomock.Any(), museApp).Return(nil)

		got, err := f.svc.Remove(t.Context(), removal, RemoveOptions{Everything: true})

		require.NoError(t, err)
		assert.Equal(t, []string{"client grant cgr_p1 (" + shopAPI + ")", "agent app " + museApp}, got.Removed)
	})

	t.Run("the agent is already gone", func(t *testing.T) {
		f := newFixture(t)
		removal := &Removal{URL: museURL, Gone: true, PoppyGrants: []Grant{}, OtherGrants: []Grant{}}

		got, err := f.svc.Remove(t.Context(), removal, RemoveOptions{Everything: true})

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
		wantRemaining []string
	}{
		{
			name: "the first grant",
			calls: func(f *fixture) {
				f.grants.EXPECT().Delete(gomock.Any(), "cgr_p1").Return(boom)
			},
			wantError: "failed to delete client grant cgr_p1 (" + shopAPI + "): boom",
		},
		{
			name: "the second grant",
			calls: func(f *fixture) {
				f.grants.EXPECT().Delete(gomock.Any(), "cgr_p1").Return(nil)
				f.grants.EXPECT().Delete(gomock.Any(), "cgr_o1").Return(boom)
			},
			wantApplied:   []string{"delete client grant cgr_p1 (" + shopAPI + ")"},
			wantRemaining: []string{"delete client grant cgr_o1 (" + otherAPI + ")", "delete agent record " + museAgent, "delete agent app " + museApp},
		},
		{
			name: "the agent record",
			calls: func(f *fixture) {
				f.grants.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(nil).Times(2)
				f.agents.EXPECT().Delete(gomock.Any(), museAgent).Return(boom)
			},
			wantApplied: []string{
				"delete client grant cgr_p1 (" + shopAPI + ")",
				"delete client grant cgr_o1 (" + otherAPI + ")",
			},
			wantRemaining: []string{"delete agent record " + museAgent, "delete agent app " + museApp},
		},
		{
			name: "the agent app",
			calls: func(f *fixture) {
				f.grants.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(nil).Times(2)
				f.agents.EXPECT().Delete(gomock.Any(), museAgent).Return(nil)
				f.clients.EXPECT().Delete(gomock.Any(), museApp).Return(boom)
			},
			wantApplied: []string{
				"delete client grant cgr_p1 (" + shopAPI + ")",
				"delete client grant cgr_o1 (" + otherAPI + ")",
				"delete agent record " + museAgent,
			},
			wantRemaining: []string{"delete agent app " + museApp},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			test.calls(f)

			got, err := f.svc.Remove(t.Context(), removalWith(1, 1), RemoveOptions{Everything: true})

			require.Error(t, err)
			assert.Nil(t, got)

			if test.wantError != "" {
				assert.EqualError(t, err, test.wantError)
				return
			}

			var partial *PartialError
			require.ErrorAs(t, err, &partial)
			assert.Equal(t, test.wantApplied, partial.Applied)
			assert.Equal(t, test.wantRemaining, partial.Remaining)
		})
	}
}
