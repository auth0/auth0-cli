package setup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/auth0/go-auth0/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// actionServer serves the part of the Actions API that setup uses, over a list of
// Actions and of post-login bindings. It records the requests, and answers the
// lists page by page like the API. It ignores the actionName filter, as an API
// that matches names loosely would.
type actionServer struct {
	t *testing.T

	mu       sync.Mutex
	actions  []map[string]any
	bindings []map[string]any
	// Requests are "METHOD path" in the order they came.
	requests []string
	// Bodies by "METHOD path".
	bodies map[string][]byte
	// Fail makes the request "METHOD path" answer with the status.
	fail map[string]int
	// BuildChecks is how many reads of an Action answer "building" before "built".
	buildChecks int
	// BuildStatus is the status a read answers once the build is over.
	buildStatus string
	// ActionsTotal and bindingsTotal replace the total a list reports, which is the
	// number of items it has.
	actionsTotal  *int
	bindingsTotal *int
}

// serverPageCap is the most items the API gives on a page, whatever per_page asks.
const serverPageCap = 20

func newActionServer(t *testing.T) (*actionServer, *management.Management) {
	t.Helper()

	s := &actionServer{t: t, bodies: map[string][]byte{}, fail: map[string]int{}, buildStatus: management.ActionStatusBuilt}

	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)

	host, err := url.Parse(srv.URL)
	require.NoError(t, err)

	api, err := management.New(host.Host, management.WithInsecure(), management.WithStaticToken("token"), management.WithNoRetries(), management.WithNoAuth0ClientInfo())
	require.NoError(t, err)

	return s, api
}

// pageOf returns the items of the page the query asks for, like the API does: the
// page size is capped.
func pageOf(query url.Values, items []map[string]any) []map[string]any {
	number, _ := strconv.Atoi(query.Get("page"))
	size, _ := strconv.Atoi(query.Get("per_page"))
	if size <= 0 || size > serverPageCap {
		size = serverPageCap
	}

	from := min(number*size, len(items))
	to := min(from+size, len(items))

	return items[from:to]
}

func (s *actionServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := r.Method + " " + strings.TrimPrefix(r.URL.Path, "/api/v2/actions/")
	s.requests = append(s.requests, key)

	body, _ := io.ReadAll(r.Body)
	s.bodies[key] = body

	reply := func(status int, payload any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(payload)
	}

	if status, found := s.fail[key]; found {
		reply(status, map[string]any{"message": "failed"})

		return
	}

	var request map[string]any
	_ = json.Unmarshal(body, &request)

	switch {
	case key == "GET actions":
		reply(http.StatusOK, map[string]any{"actions": pageOf(r.URL.Query(), s.actions), "total": totalOf(s.actionsTotal, len(s.actions))})
	case key == "POST actions":
		request["id"] = "act_new"
		request["status"] = "pending"
		s.actions = append(s.actions, request)
		reply(http.StatusCreated, request)
	case key == "GET triggers/post-login/bindings":
		reply(http.StatusOK, map[string]any{"bindings": pageOf(r.URL.Query(), s.bindings), "total": totalOf(s.bindingsTotal, len(s.bindings))})
	case key == "PATCH triggers/post-login/bindings":
		s.bindings = nil
		for i, binding := range request["bindings"].([]any) {
			entry := binding.(map[string]any)
			ref := entry["ref"].(map[string]any)
			s.bindings = append(s.bindings, map[string]any{
				"id": fmt.Sprintf("b_%d", i), "display_name": entry["display_name"], "action": map[string]any{"id": ref["value"]},
			})
		}
		reply(http.StatusOK, map[string]any{"bindings": s.bindings})
	case strings.HasPrefix(key, "GET actions/"):
		action := s.find(strings.TrimPrefix(key, "GET actions/"))
		if action == nil {
			reply(http.StatusNotFound, map[string]any{"message": "not found"})

			return
		}

		status := s.buildStatus
		if s.buildChecks > 0 {
			s.buildChecks--
			status = management.ActionStatusBuilding
		}
		action["status"] = status
		reply(http.StatusOK, action)
	case strings.HasPrefix(key, "PATCH actions/"):
		action := s.find(strings.TrimPrefix(key, "PATCH actions/"))
		action["code"] = request["code"]
		reply(http.StatusOK, action)
	case strings.HasSuffix(key, "/deploy"):
		reply(http.StatusOK, map[string]any{"id": "ver_1", "deployed": true})
	default:
		s.t.Errorf("unexpected request %s", key)
		reply(http.StatusTeapot, nil)
	}
}

// totalOf is the total a list reports: the given one, else the real count.
func totalOf(given *int, count int) int {
	if given != nil {
		return *given
	}

	return count
}

func (s *actionServer) find(id string) map[string]any {
	for _, action := range s.actions {
		if action["id"] == id {
			return action
		}
	}

	return nil
}

// stored returns a copy of the requests so far.
func (s *actionServer) stored() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]string{}, s.requests...)
}

// actionNamed is an Action of the tenant as the list shows it.
func actionNamed(id, name, code string, deployed bool) map[string]any {
	action := map[string]any{"id": id, "name": name, "code": code, "all_changes_deployed": deployed}
	if deployed {
		action["deployed_version"] = map[string]any{"id": "ver_0", "code": code, "deployed": true}
	}

	return action
}

// listedBinding is a binding of the post-login trigger as the API lists it.
func listedBinding(actionID, actionName, displayName string) map[string]any {
	return map[string]any{
		"id": "bnd_" + actionID, "trigger_id": "post-login", "display_name": displayName,
		"action": map[string]any{"id": actionID, "name": actionName},
	}
}

func TestOfflineAccessActionSource(t *testing.T) {
	assert.NotEmpty(t, OfflineAccessActionSource)
	assert.Contains(t, OfflineAccessActionSource, "onExecutePostLogin")
	assert.Contains(t, OfflineAccessActionSource, "addTargetScope('offline_access')")
	assert.Contains(t, OfflineAccessActionSource, "event.agent")
}

func TestFlagsParseAlwaysIssueRefreshTokens(t *testing.T) {
	yes, no := true, false

	tests := []struct {
		name string
		flag *bool
		want *bool
	}{
		{name: "not given", flag: nil, want: nil},
		{name: "true", flag: &yes, want: &yes},
		{name: "false", flag: &no, want: &no},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			in, err := Flags{AlwaysIssueRefreshTokens: test.flag}.Parse()

			require.NoError(t, err)
			assert.Equal(t, test.want, in.AlwaysIssueRefreshTokens)
			assert.Equal(t, test.want == nil, in.AsksAlwaysIssueRefreshTokens(), "asked only when the flag was not given")
			assert.Equal(t, test.want != nil && *test.want, in.WantsRefreshTokenAction())

			if test.flag != nil {
				assert.NotSame(t, test.flag, in.AlwaysIssueRefreshTokens, "the inputs hold a copy")
			}
		})
	}
}

func TestCommandAlwaysIssueRefreshTokens(t *testing.T) {
	yes, no := true, false

	tests := []struct {
		name string
		in   Inputs
		opts CommandOptions
		want string
	}{
		{name: "not given", in: Inputs{CompanyDomain: "shop.example"}, want: "auth0 poppy setup --company-domain shop.example"},
		{name: "true", in: Inputs{CompanyDomain: "shop.example", AlwaysIssueRefreshTokens: &yes}, want: "auth0 poppy setup --company-domain shop.example --always-issue-refresh-tokens=true"},
		{name: "false", in: Inputs{CompanyDomain: "shop.example", AlwaysIssueRefreshTokens: &no}, want: "auth0 poppy setup --company-domain shop.example --always-issue-refresh-tokens=false"},
		{name: "the config command has no such flag", in: Inputs{CompanyDomain: "shop.example", AlwaysIssueRefreshTokens: &yes}, opts: CommandOptions{Config: true}, want: "auth0 poppy config --company-domain shop.example"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, strings.Join(Command(test.in, test.opts), " "))
		})
	}
}

func TestReadAction(t *testing.T) {
	ours := OfflineAccessActionName
	code := OfflineAccessActionSource

	tests := []struct {
		name     string
		actions  []map[string]any
		bindings []map[string]any
		want     ActionState
	}{
		{
			name: "no Action and an empty flow",
			want: ActionState{Flow: []FlowAction{}},
		},
		{
			name:    "the Action exists with the code and is deployed",
			actions: []map[string]any{actionNamed("act_1", ours, code, true)},
			want:    ActionState{ID: "act_1", CodeCurrent: true, Deployed: true, Flow: []FlowAction{}},
		},
		{
			name:    "the Action has other code",
			actions: []map[string]any{actionNamed("act_1", ours, "old code", true)},
			want:    ActionState{ID: "act_1", Flow: []FlowAction{}},
		},
		{
			name:    "the code is current but the draft is not deployed",
			actions: []map[string]any{actionNamed("act_1", ours, code, false)},
			want:    ActionState{ID: "act_1", CodeCurrent: true, Flow: []FlowAction{}},
		},
		{
			name:    "an Action with a similar name is not the one",
			actions: []map[string]any{actionNamed("act_9", ours+" (copy)", code, true), actionNamed("act_1", ours, code, true)},
			want:    ActionState{ID: "act_1", CodeCurrent: true, Deployed: true, Flow: []FlowAction{}},
		},
		{
			name:    "only a similar name",
			actions: []map[string]any{actionNamed("act_9", ours+" (copy)", code, true)},
			want:    ActionState{Flow: []FlowAction{}},
		},
		{
			name:     "the flow keeps its order and the display names",
			bindings: []map[string]any{listedBinding("act_a", "Enrich", "Enrich profile"), listedBinding("act_b", "Audit", "")},
			want: ActionState{Flow: []FlowAction{
				{BindingID: "bnd_act_a", RefType: "action_id", RefValue: "act_a", DisplayName: "Enrich profile", Name: "Enrich profile"},
				{BindingID: "bnd_act_b", RefType: "action_id", RefValue: "act_b", Name: "Audit"},
			}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, api := newActionServer(t)
			server.actions, server.bindings = test.actions, test.bindings

			state, err := (&Reader{Actions: api.Action}).ReadAction(t.Context())

			require.NoError(t, err)
			assert.Equal(t, test.want, *state)
		})
	}
}

func TestReadActionPaginates(t *testing.T) {
	server, api := newActionServer(t)

	for i := range actionPageSize + 3 {
		id := fmt.Sprintf("act_%d", i)
		server.bindings = append(server.bindings, listedBinding(id, "Action "+id, ""))
		server.actions = append(server.actions, actionNamed(id, "Other "+id, "code", true))
	}
	server.actions = append(server.actions, actionNamed("act_ours", OfflineAccessActionName, OfflineAccessActionSource, true))

	state, err := (&Reader{Actions: api.Action}).ReadAction(t.Context())

	require.NoError(t, err)
	assert.Equal(t, "act_ours", state.ID, "the Action is on the second page")
	require.Len(t, state.Flow, actionPageSize+3, "every binding is read, in order")
	assert.Equal(t, "act_0", state.Flow[0].RefValue)
	assert.Equal(t, fmt.Sprintf("act_%d", actionPageSize+2), state.Flow[actionPageSize+2].RefValue)
	assert.Equal(t, []string{
		"GET actions", "GET actions", "GET actions",
		"GET triggers/post-login/bindings", "GET triggers/post-login/bindings", "GET triggers/post-login/bindings",
	}, server.stored(), "the server gives a capped number of items a page, so there are three pages of each")
}

func TestReadActionPagesFollowTheServerCap(t *testing.T) {
	bindingPages := []string{"GET triggers/post-login/bindings", "GET triggers/post-login/bindings", "GET triggers/post-login/bindings"}

	t.Run("a page shorter than the page asked for is not the last", func(t *testing.T) {
		server, api := newActionServer(t)

		for i := range 45 {
			server.bindings = append(server.bindings, listedBinding(fmt.Sprintf("act_%d", i), "A", ""))
		}

		state, err := (&Reader{Actions: api.Action}).ReadAction(t.Context())

		require.NoError(t, err)
		require.Len(t, state.Flow, 45)
		assert.Equal(t, "act_0", state.Flow[0].RefValue)
		assert.Equal(t, "act_44", state.Flow[44].RefValue)
		assert.Equal(t, append([]string{"GET actions"}, bindingPages...), server.stored())
	})

	t.Run("a list with no total ends at an empty page", func(t *testing.T) {
		server, api := newActionServer(t)
		none := 0
		server.bindingsTotal = &none

		for i := range 45 {
			server.bindings = append(server.bindings, listedBinding(fmt.Sprintf("act_%d", i), "A", ""))
		}

		state, err := (&Reader{Actions: api.Action}).ReadAction(t.Context())

		require.NoError(t, err)
		assert.Len(t, state.Flow, 45)
		assert.Equal(t, append(append([]string{"GET actions"}, bindingPages...), "GET triggers/post-login/bindings"), server.stored())
	})

	t.Run("a total that disagrees with the list fails", func(t *testing.T) {
		tests := []struct {
			name  string
			total int
		}{
			{name: "more items reported than listed", total: 50},
			{name: "fewer items reported than listed", total: 30},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				server, api := newActionServer(t)
				server.bindingsTotal = &test.total

				for i := range 45 {
					server.bindings = append(server.bindings, listedBinding(fmt.Sprintf("act_%d", i), "A", ""))
				}

				state, err := (&Reader{Actions: api.Action}).ReadAction(t.Context())

				assert.Nil(t, state)
				assert.ErrorContains(t, err, fmt.Sprintf("says the list has %d", test.total))
			})
		}

		t.Run("the same for the Actions", func(t *testing.T) {
			server, api := newActionServer(t)
			seven := 7
			server.actionsTotal = &seven
			server.actions = []map[string]any{actionNamed("act_1", OfflineAccessActionName, "code", true)}

			_, err := (&Reader{Actions: api.Action}).ReadAction(t.Context())

			assert.ErrorContains(t, err, "says the list has 7")
		})
	})
}

func TestReadActionFailures(t *testing.T) {
	t.Run("a flow that cannot be written back is refused", func(t *testing.T) {
		tests := []struct {
			name    string
			binding map[string]any
			want    string
		}{
			{
				name:    "a binding with secrets",
				binding: map[string]any{"id": "bnd_1", "display_name": "Enrich", "action": map[string]any{"id": "act_a"}, "secrets": []any{map[string]any{"name": "KEY"}}},
				want:    "The binding Enrich of the post-login trigger has secrets",
			},
			{
				name:    "a binding that names no Action",
				binding: map[string]any{"id": "bnd_1", "display_name": "Mystery"},
				want:    "does not say which Action it is",
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				server, api := newActionServer(t)
				server.bindings = []map[string]any{test.binding}

				_, err := (&Reader{Actions: api.Action}).ReadAction(t.Context())

				var input *InputError
				require.ErrorAs(t, err, &input)
				assert.Equal(t, ReasonActionFlowUnsupported, input.Reason)
				assert.Contains(t, input.Message, test.want)
				assert.Contains(t, input.Fix, "--always-issue-refresh-tokens=false")
			})
		}
	})

	t.Run("a binding that refers to its Action by name is kept as it is", func(t *testing.T) {
		server, api := newActionServer(t)
		server.bindings = []map[string]any{{"id": "bnd_1", "display_name": "By name", "ref": map[string]any{"type": "action_name", "value": "Enrich"}}}

		state, err := (&Reader{Actions: api.Action}).ReadAction(t.Context())

		require.NoError(t, err)
		assert.Equal(t, []FlowAction{{BindingID: "bnd_1", RefType: "action_name", RefValue: "Enrich", DisplayName: "By name", Name: "By name"}}, state.Flow)
	})

	t.Run("a failed read is wrapped", func(t *testing.T) {
		tests := map[string]string{
			"GET actions":                      "failed to list the Actions",
			"GET triggers/post-login/bindings": "failed to read the post-login bindings",
		}

		for key, want := range tests {
			server, api := newActionServer(t)
			server.fail[key] = http.StatusForbidden

			_, err := (&Reader{Actions: api.Action}).ReadAction(t.Context())

			assert.ErrorContains(t, err, want)
		}
	})

	t.Run("a list that never ends fails instead of returning part of it", func(t *testing.T) {
		server, api := newActionServer(t)

		for i := range maxActionPages*serverPageCap + 1 {
			server.bindings = append(server.bindings, listedBinding(fmt.Sprintf("act_%d", i), "A", ""))
		}

		_, err := (&Reader{Actions: api.Action}).ReadAction(t.Context())

		assert.ErrorContains(t, err, "did not end after")
	})
}

// flowOfOthers is a flow of two Actions of the tenant.
func flowOfOthers() []FlowAction {
	return []FlowAction{
		{BindingID: "bnd_act_a", RefType: "action_id", RefValue: "act_a", DisplayName: "Enrich profile", Name: "Enrich profile"},
		{BindingID: "bnd_act_b", RefType: "action_id", RefValue: "act_b", Name: "Audit"},
	}
}

// bindingsOfOthers are the bindings the API lists for flowOfOthers.
func bindingsOfOthers() []map[string]any {
	return []map[string]any{listedBinding("act_a", "Enrich", "Enrich profile"), listedBinding("act_b", "Audit", "")}
}

func oursFlowAction() FlowAction {
	return FlowAction{BindingID: "bnd_act_1", RefType: "action_id", RefValue: "act_1", DisplayName: OfflineAccessActionName, Name: OfflineAccessActionName}
}

func TestActionChange(t *testing.T) {
	flow := flowOfOthers()
	ours := oursFlowAction()

	tests := []struct {
		name          string
		action        ActionState
		wantFields    []Field
		wantUnchanged []string
	}{
		{
			name:   "no Action: create, deploy and bind it last",
			action: ActionState{Flow: flow},
			wantFields: []Field{
				{Name: fieldAction, Value: "deployed", Before: ValueNone},
				{Name: fieldActionOrder, Value: "last", Before: "not bound", Effect: "runs after: Enrich profile, Audit"},
			},
		},
		{
			name:   "no Action and an empty flow",
			action: ActionState{},
			wantFields: []Field{
				{Name: fieldAction, Value: "deployed", Before: ValueNone},
				{Name: fieldActionOrder, Value: "last", Before: "not bound", Effect: "no other Action runs"},
			},
		},
		{
			name:   "other code: update, deploy, keep the place",
			action: ActionState{ID: "act_1", Flow: append(slicesClone(flow), ours)},
			wantFields: []Field{
				{Name: fieldAction, Value: "deployed", Before: "old code"},
			},
			wantUnchanged: []string{"Action position"},
		},
		{
			name:   "current code that is not deployed",
			action: ActionState{ID: "act_1", CodeCurrent: true, Flow: append(slicesClone(flow), ours)},
			wantFields: []Field{
				{Name: fieldAction, Value: "deployed", Before: "not deployed"},
			},
			wantUnchanged: []string{"Action position"},
		},
		{
			name:   "deployed but not bound",
			action: ActionState{ID: "act_1", CodeCurrent: true, Deployed: true, Flow: flow},
			wantFields: []Field{
				{Name: fieldActionOrder, Value: "last", Before: "not bound", Effect: "runs after: Enrich profile, Audit"},
			},
			wantUnchanged: []string{"refresh token Action"},
		},
		{
			name:   "bound but not last",
			action: ActionState{ID: "act_1", CodeCurrent: true, Deployed: true, Flow: []FlowAction{flow[0], ours, flow[1]}},
			wantFields: []Field{
				{Name: fieldActionOrder, Value: "last", Before: "2 of 3", Effect: "runs after: Enrich profile, Audit"},
			},
			wantUnchanged: []string{"refresh token Action"},
		},
		{
			name:   "bound twice",
			action: ActionState{ID: "act_1", CodeCurrent: true, Deployed: true, Flow: []FlowAction{ours, flow[0], ours}},
			wantFields: []Field{
				{Name: fieldActionOrder, Value: "last", Before: "3 of 3", Effect: "runs after: Enrich profile"},
			},
			wantUnchanged: []string{"refresh token Action"},
		},
		{
			name:          "already bound last: nothing to change",
			action:        ActionState{ID: "act_1", CodeCurrent: true, Deployed: true, Flow: append(slicesClone(flow), ours)},
			wantUnchanged: []string{"refresh token Action", "Action position"},
		},
		{
			name:          "the only Action of the flow",
			action:        ActionState{ID: "act_1", CodeCurrent: true, Deployed: true, Flow: []FlowAction{ours}},
			wantUnchanged: []string{"refresh token Action", "Action position"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			change, unchanged, changed := actionChange(&test.action)

			assert.Equal(t, len(test.wantFields) > 0, changed)
			assert.Equal(t, test.wantUnchanged, unchanged)
			assert.Equal(t, KindAction, change.Kind)
			assert.Equal(t, ImpactTenantWide, change.Impact)
			assert.Equal(t, "post-login Action "+OfflineAccessActionName, change.Description)

			if len(test.wantFields) > 0 {
				assert.Equal(t, test.wantFields, change.Fields)
				assert.Equal(t, len(test.wantFields), change.Count())
			}
		})
	}
}

func slicesClone(flow []FlowAction) []FlowAction { return append([]FlowAction{}, flow...) }

func TestBuildAction(t *testing.T) {
	yes, no := true, false

	t.Run("the option adds the change before the file, with a note about removing it", func(t *testing.T) {
		state, in := freshState(t)
		in.AlwaysIssueRefreshTokens = &yes
		state.Action = &ActionState{Flow: flowOfOthers()}

		plan := buildPlan(t, state, in)

		assert.Equal(t, []Kind{KindTenant, KindAPI, KindApp, KindAction, KindFile}, kinds(plan))
		assert.Contains(t, strings.Join(plan.Warnings, "\n"), "auth0 actions delete")
		assert.Contains(t, strings.Join(plan.Warnings, "\n"), OfflineAccessActionName)
		assert.Equal(t, len(plan.Warnings), len(plan.warningKinds))
	})

	t.Run("the Action alone is a plan", func(t *testing.T) {
		state, in := configuredState(t), configuredInputs()
		in.AlwaysIssueRefreshTokens = &yes
		state.Action = &ActionState{Flow: flowOfOthers()}

		plan := buildPlan(t, state, in)

		assert.Equal(t, []Kind{KindAction}, kinds(plan))
		assert.Equal(t, []string{fieldAction, fieldActionOrder}, fieldNames(plan.Changes[0]))
		assert.Equal(t, 2, plan.Count())
	})

	t.Run("an Action that is in place leaves the plan empty and says so", func(t *testing.T) {
		state, in := configuredState(t), configuredInputs()
		in.AlwaysIssueRefreshTokens = &yes
		state.Action = &ActionState{ID: "act_1", CodeCurrent: true, Deployed: true, Flow: append(flowOfOthers(), oursFlowAction())}

		plan := buildPlan(t, state, in)

		assert.True(t, plan.Empty())
		assert.Contains(t, plan.Unchanged, "refresh token Action")
		assert.Contains(t, plan.Unchanged, "Action position")
	})

	t.Run("the JSON carries the change", func(t *testing.T) {
		state, in := configuredState(t), configuredInputs()
		in.AlwaysIssueRefreshTokens = &yes
		state.Action = &ActionState{Flow: flowOfOthers()}

		raw := string(mustJSON(t, buildPlan(t, state, in)))

		assert.Contains(t, raw, `"kind":"action"`)
		assert.Contains(t, raw, `"impact":"tenant-wide"`)
		assert.Contains(t, raw, `"name":"post_login_order","value":"last","before":"not bound","effect":"runs after: Enrich profile, Audit"`)
	})

	t.Run("without the option nothing changes, even when the state has an Action", func(t *testing.T) {
		for _, answer := range []*bool{nil, &no} {
			state, in := freshState(t)
			in.AlwaysIssueRefreshTokens = answer
			state.Action = &ActionState{Flow: flowOfOthers()}

			plan := buildPlan(t, state, in)

			assert.Equal(t, []Kind{KindTenant, KindAPI, KindApp, KindFile}, kinds(plan))
			assert.NotContains(t, strings.Join(plan.Warnings, "\n"), "auth0 actions delete")
		}
	})

	t.Run("the option without the state is an error", func(t *testing.T) {
		state, in := configuredState(t), configuredInputs()
		in.AlwaysIssueRefreshTokens = &yes

		_, err := Build(state, in)

		assert.EqualError(t, err, "the post-login Action was not read")
	})
}

func TestPlanScopesAction(t *testing.T) {
	plan := Plan{Changes: []Change{{Kind: KindAction}}}

	assert.ElementsMatch(t, append([]string{"read:actions", "create:actions", "update:actions"}, ReadScopes...), plan.Scopes())
	assert.Equal(t, []string{"read:actions"}, ActionReadScopes)
	assert.Equal(t, []string{"create:actions", "update:actions"}, ActionWriteScopes)
}

// applyAction applies a change of the Action over the server.
func applyAction(t *testing.T, api *management.Management, state *ActionState) error {
	t.Helper()

	change, _, changed := actionChange(state)
	require.True(t, changed)

	applier := &Applier{Actions: api.Action, BuildPoll: time.Millisecond}

	return applier.applyAction(t.Context(), change, state)
}

func TestApplyAction(t *testing.T) {
	ours := OfflineAccessActionName

	tests := []struct {
		name         string
		actions      []map[string]any
		bindings     []map[string]any
		state        ActionState
		wantRequests []string
		// The order of the flow after the apply, as display names or IDs.
		wantFlow []string
	}{
		{
			name:     "no Action: create, wait for the build, deploy, bind it last",
			bindings: []map[string]any{listedBinding("act_a", "Enrich", "Enrich profile")},
			state:    ActionState{Flow: []FlowAction{{BindingID: "bnd_act_a", RefType: "action_id", RefValue: "act_a", DisplayName: "Enrich profile", Name: "Enrich profile"}}},
			wantRequests: []string{
				"POST actions", "GET actions/act_new", "POST actions/act_new/deploy", "GET triggers/post-login/bindings", "PATCH triggers/post-login/bindings",
			},
			wantFlow: []string{"act_a", "act_new"},
		},
		{
			name:         "other code: update, wait, deploy, keep its place",
			actions:      []map[string]any{actionNamed("act_1", ours, "old", true)},
			bindings:     []map[string]any{listedBinding("act_1", ours, ours)},
			state:        ActionState{ID: "act_1", Flow: []FlowAction{oursFlowAction()}},
			wantRequests: []string{"PATCH actions/act_1", "GET actions/act_1", "POST actions/act_1/deploy"},
		},
		{
			name:         "current code that is not deployed: deploy only",
			actions:      []map[string]any{actionNamed("act_1", ours, OfflineAccessActionSource, false)},
			bindings:     []map[string]any{listedBinding("act_1", ours, ours)},
			state:        ActionState{ID: "act_1", CodeCurrent: true, Flow: []FlowAction{oursFlowAction()}},
			wantRequests: []string{"GET actions/act_1", "POST actions/act_1/deploy"},
		},
		{
			name:         "deployed and not last: only the bindings",
			actions:      []map[string]any{actionNamed("act_1", ours, OfflineAccessActionSource, true)},
			bindings:     []map[string]any{listedBinding("act_1", ours, ours), listedBinding("act_a", "Enrich", "")},
			state:        ActionState{ID: "act_1", CodeCurrent: true, Deployed: true, Flow: []FlowAction{oursFlowAction(), {BindingID: "bnd_act_a", RefType: "action_id", RefValue: "act_a", Name: "Enrich"}}},
			wantRequests: []string{"GET triggers/post-login/bindings", "PATCH triggers/post-login/bindings"},
			wantFlow:     []string{"act_a", "act_1"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, api := newActionServer(t)
			server.actions, server.bindings = test.actions, test.bindings

			require.NoError(t, applyAction(t, api, &test.state))

			assert.Equal(t, test.wantRequests, server.stored())

			if test.wantFlow != nil {
				var flow []string
				for _, binding := range server.bindings {
					flow = append(flow, binding["action"].(map[string]any)["id"].(string))
				}
				assert.Equal(t, test.wantFlow, flow)
			}
		})
	}
}

func TestApplyActionCreatesTheAction(t *testing.T) {
	server, api := newActionServer(t)

	require.NoError(t, applyAction(t, api, &ActionState{}))

	var created map[string]any
	require.NoError(t, json.Unmarshal(server.bodies["POST actions"], &created))

	assert.Equal(t, OfflineAccessActionName, created["name"])
	assert.Equal(t, OfflineAccessActionSource, created["code"])
	assert.Equal(t, []any{map[string]any{"id": "post-login", "version": "v3"}}, created["supported_triggers"])
	assert.NotContains(t, created, "deploy", "the deploy is a call of its own, after the build")
	assert.NotContains(t, created, "runtime", "the runtime is the default of the API")
	assert.JSONEq(t, `{"bindings":[{"ref":{"type":"action_id","value":"act_new"},"display_name":"Poppy: always issue refresh tokens"}]}`, string(server.bodies["PATCH triggers/post-login/bindings"]))
}

func TestApplyActionUpdatesTheCode(t *testing.T) {
	server, api := newActionServer(t)
	server.actions = []map[string]any{actionNamed("act_1", OfflineAccessActionName, "old", true)}

	state := &ActionState{ID: "act_1", Flow: []FlowAction{oursFlowAction()}}
	require.NoError(t, applyAction(t, api, state))

	// The API takes the request as the whole Action, so the name and the trigger go with the code.
	assert.JSONEq(t, `{
		"name":"Poppy: always issue refresh tokens",
		"supported_triggers":[{"id":"post-login","version":"v3"}],
		"code":`+strconv.Quote(OfflineAccessActionSource)+`
	}`, string(server.bodies["PATCH actions/act_1"]))
}

func TestApplyActionBindingsBody(t *testing.T) {
	// The flow is written back whole: every entry as an action_id reference, in its
	// order and with its display name, then the Action of setup. The old entry of the
	// Action of setup is dropped from its place.
	server, api := newActionServer(t)
	server.actions = []map[string]any{actionNamed("act_1", OfflineAccessActionName, OfflineAccessActionSource, true)}
	server.bindings = []map[string]any{
		listedBinding("act_a", "Enrich", "Enrich profile"),
		listedBinding("act_1", OfflineAccessActionName, OfflineAccessActionName),
		listedBinding("act_b", "Audit", ""),
		{"id": "bnd_legacy", "display_name": "Legacy by name", "ref": map[string]any{"type": "action_name", "value": "Legacy"}},
	}

	state, err := (&Reader{Actions: api.Action}).ReadAction(t.Context())
	require.NoError(t, err)
	require.True(t, state.CodeCurrent && state.Deployed)

	require.NoError(t, applyAction(t, api, state))

	assert.JSONEq(t, `{"bindings":[
		{"ref":{"type":"action_id","value":"act_a"},"display_name":"Enrich profile"},
		{"ref":{"type":"action_id","value":"act_b"}},
		{"ref":{"type":"action_name","value":"Legacy"},"display_name":"Legacy by name"},
		{"ref":{"type":"action_id","value":"act_1"},"display_name":"Poppy: always issue refresh tokens"}
	]}`, string(server.bodies["PATCH triggers/post-login/bindings"]))
	assert.Equal(t, "PATCH triggers/post-login/bindings", server.stored()[len(server.stored())-1])
}

func TestApplyActionWaitsForTheBuild(t *testing.T) {
	server, api := newActionServer(t)
	server.buildChecks = 3

	require.NoError(t, applyAction(t, api, &ActionState{}))

	assert.Equal(t, []string{
		"POST actions", "GET actions/act_new", "GET actions/act_new", "GET actions/act_new", "GET actions/act_new",
		"POST actions/act_new/deploy", "GET triggers/post-login/bindings", "PATCH triggers/post-login/bindings",
	}, server.stored())
}

func TestApplyActionStaleFlow(t *testing.T) {
	ours := OfflineAccessActionName

	// The flow when setup reads it: the Action of setup is bound, but not last.
	start := func() []map[string]any {
		return []map[string]any{listedBinding("act_1", ours, ours), listedBinding("act_a", "Enrich", ""), listedBinding("act_b", "Audit", "")}
	}

	tests := []struct {
		name string
		// Change is what happens to the bindings between the read and the apply.
		change    func(bindings []map[string]any) []map[string]any
		wantStale bool
	}{
		{name: "unchanged", change: func(b []map[string]any) []map[string]any { return b }},
		{
			name:      "an entry added at the end",
			change:    func(b []map[string]any) []map[string]any { return append(b, listedBinding("act_c", "New", "")) },
			wantStale: true,
		},
		{
			name: "an entry added at the start",
			change: func(b []map[string]any) []map[string]any {
				return append([]map[string]any{listedBinding("act_c", "New", "")}, b...)
			},
			wantStale: true,
		},
		{
			name:      "an entry removed",
			change:    func(b []map[string]any) []map[string]any { return append(b[:1:1], b[2:]...) },
			wantStale: true,
		},
		{
			name:      "entries reordered",
			change:    func(b []map[string]any) []map[string]any { return []map[string]any{b[0], b[2], b[1]} },
			wantStale: true,
		},
		{
			name: "an entry bound again, with a new binding ID",
			change: func(b []map[string]any) []map[string]any {
				b[1] = map[string]any{"id": "bnd_new", "action": map[string]any{"id": "act_a", "name": "Enrich"}}

				return b
			},
			wantStale: true,
		},
		{
			name: "an entry renamed",
			change: func(b []map[string]any) []map[string]any {
				b[1] = listedBinding("act_a", "Enrich", "Renamed")

				return b
			},
			wantStale: true,
		},
		{
			name: "an entry that setup could not write back",
			change: func(b []map[string]any) []map[string]any {
				return append(b, map[string]any{"id": "bnd_x", "display_name": "X"})
			},
			wantStale: true,
		},
		{name: "all removed", change: func([]map[string]any) []map[string]any { return nil }, wantStale: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, api := newActionServer(t)
			server.actions = []map[string]any{actionNamed("act_1", ours, OfflineAccessActionSource, true)}
			server.bindings = start()

			state, err := (&Reader{Actions: api.Action}).ReadAction(t.Context())
			require.NoError(t, err)

			server.bindings = test.change(server.bindings)
			before := len(server.stored())

			err = applyAction(t, api, state)

			requests := server.stored()[before:]

			if !test.wantStale {
				require.NoError(t, err)
				assert.Equal(t, []string{"GET triggers/post-login/bindings", "PATCH triggers/post-login/bindings"}, requests)

				return
			}

			var input *InputError
			require.ErrorAs(t, err, &input)
			assert.Equal(t, ReasonActionFlowChanged, input.Reason)
			assert.NotContains(t, requests, "PATCH triggers/post-login/bindings", "the flow is not overwritten")
		})
	}

	t.Run("a change on the last page of a long flow is seen", func(t *testing.T) {
		server, api := newActionServer(t)
		server.actions = []map[string]any{actionNamed("act_1", ours, OfflineAccessActionSource, true)}
		server.bindings = start()

		for i := range 40 {
			server.bindings = append(server.bindings, listedBinding(fmt.Sprintf("act_x%d", i), "X", ""))
		}

		state, err := (&Reader{Actions: api.Action}).ReadAction(t.Context())
		require.NoError(t, err)

		server.bindings[len(server.bindings)-1] = listedBinding("act_other", "Other", "")

		err = applyAction(t, api, state)

		var input *InputError
		require.ErrorAs(t, err, &input)
		assert.Equal(t, ReasonActionFlowChanged, input.Reason)
		assert.NotContains(t, server.stored(), "PATCH triggers/post-login/bindings")
	})

	t.Run("a failed read of the bindings stops the update", func(t *testing.T) {
		server, api := newActionServer(t)
		server.actions = []map[string]any{actionNamed("act_1", ours, OfflineAccessActionSource, true)}
		server.bindings = start()

		state, err := (&Reader{Actions: api.Action}).ReadAction(t.Context())
		require.NoError(t, err)

		server.fail["GET triggers/post-login/bindings"] = http.StatusForbidden

		err = applyAction(t, api, state)

		assert.ErrorContains(t, err, "failed to read the post-login bindings")
		assert.NotContains(t, server.stored(), "PATCH triggers/post-login/bindings")
	})
}

func TestApplyActionFailures(t *testing.T) {
	tests := []struct {
		name         string
		fail         map[string]int
		buildStatus  string
		wantErr      string
		wantRequests []string
	}{
		{
			name:         "create fails",
			fail:         map[string]int{"POST actions": http.StatusForbidden},
			wantErr:      "failed to create the Action",
			wantRequests: []string{"POST actions"},
		},
		{
			name:         "the build fails",
			buildStatus:  management.ActionStatusFailed,
			wantErr:      "the Action failed to build",
			wantRequests: []string{"POST actions", "GET actions/act_new"},
		},
		{
			name:         "the deploy fails",
			fail:         map[string]int{"POST actions/act_new/deploy": http.StatusBadRequest},
			wantErr:      "failed to deploy the Action",
			wantRequests: []string{"POST actions", "GET actions/act_new", "POST actions/act_new/deploy"},
		},
		{
			name:         "the bindings update fails",
			fail:         map[string]int{"PATCH triggers/post-login/bindings": http.StatusForbidden},
			wantErr:      "failed to bind the Action to the post-login trigger",
			wantRequests: []string{"POST actions", "GET actions/act_new", "POST actions/act_new/deploy", "GET triggers/post-login/bindings", "PATCH triggers/post-login/bindings"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, api := newActionServer(t)
			server.fail = test.fail
			if test.buildStatus != "" {
				server.buildStatus = test.buildStatus
			}

			err := applyAction(t, api, &ActionState{})

			assert.ErrorContains(t, err, test.wantErr)
			assert.Equal(t, test.wantRequests, server.stored())
		})
	}

	t.Run("a build that does not end is given up on when the context ends", func(t *testing.T) {
		server, api := newActionServer(t)
		server.actions = []map[string]any{actionNamed("act_1", OfflineAccessActionName, OfflineAccessActionSource, false)}
		server.buildChecks = 1000

		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()

		state := &ActionState{ID: "act_1", CodeCurrent: true, Flow: []FlowAction{oursFlowAction()}}
		change, _, _ := actionChange(state)
		err := (&Applier{Actions: api.Action, BuildPoll: time.Hour}).applyAction(ctx, change, state)

		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})
}

func TestApplyPlanWithTheAction(t *testing.T) {
	yes := true
	state, in := configuredState(t), configuredInputs()
	in.AlwaysIssueRefreshTokens = &yes
	in.Out = t.TempDir() + "/poppy.json"
	in.APIURLs = map[string]string{shopAPI: shopAPI + "/openapi.json"}
	state.Action = &ActionState{Flow: flowOfOthers()}
	state.File = nil

	plan := buildPlan(t, state, in)
	require.Equal(t, []Kind{KindAction, KindFile}, kinds(plan))

	t.Run("the Action is applied before the file is written", func(t *testing.T) {
		server, api := newActionServer(t)
		server.bindings = bindingsOfOthers()
		applier := &Applier{Actions: api.Action, BuildPoll: time.Millisecond}

		applied, err := applier.Apply(t.Context(), state, in, plan)

		require.NoError(t, err)
		assert.Equal(t, []string{plan.Changes[0].Description, plan.Changes[1].Description}, applied)
		assert.FileExists(t, in.Out)
		assert.Len(t, server.stored(), 5)
	})

	t.Run("a failure of the Action is the first failure and writes no file", func(t *testing.T) {
		server, api := newActionServer(t)
		server.fail["POST actions"] = http.StatusForbidden
		out := t.TempDir() + "/poppy.json"
		failing := in
		failing.Out = out

		_, err := (&Applier{Actions: api.Action, BuildPoll: time.Millisecond}).Apply(t.Context(), state, failing, buildPlan(t, state, failing))

		assert.ErrorContains(t, err, "failed to create the Action")
		assert.NotContains(t, err.Error(), "stopped partway")
		assert.NoFileExists(t, out)
	})

	t.Run("a failure after the Action was created says it may exist and to run setup again", func(t *testing.T) {
		server, api := newActionServer(t)
		server.bindings = bindingsOfOthers()
		server.fail["POST actions/act_new/deploy"] = http.StatusBadRequest
		failing := in
		failing.Out = t.TempDir() + "/poppy.json"

		_, err := (&Applier{Actions: api.Action, BuildPoll: time.Millisecond}).Apply(t.Context(), state, failing, buildPlan(t, state, failing))

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to deploy the Action")
		assert.ErrorContains(t, err, "The Action may exist already. Running the command again finds it and continues from here.")
		assert.NotContains(t, err.Error(), "stopped partway")
		assert.Contains(t, server.stored(), "POST actions")
		assert.NoFileExists(t, failing.Out)
	})

	t.Run("a flow that changed is not overwritten and the error says to run setup again", func(t *testing.T) {
		server, api := newActionServer(t)
		server.bindings = append(bindingsOfOthers(), listedBinding("act_c", "Added by someone", ""))
		failing := in
		failing.Out = t.TempDir() + "/poppy.json"

		_, err := (&Applier{Actions: api.Action, BuildPoll: time.Millisecond}).Apply(t.Context(), state, failing, buildPlan(t, state, failing))

		var input *InputError
		require.ErrorAs(t, err, &input)
		assert.Equal(t, ReasonActionFlowChanged, input.Reason)
		assert.Contains(t, input.Message, "The post-login flow changed while setup was running, so setup left it as it is.")
		assert.Contains(t, input.Fix, "Run the command again.")
		assert.NotContains(t, server.stored(), "PATCH triggers/post-login/bindings")
		assert.NoFileExists(t, failing.Out)
	})
}
