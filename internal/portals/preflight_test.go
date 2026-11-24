package portals_test

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/portals"
)

// answeringProber answers each check the way the scope gate of the API would, keyed by "METHOD path".
// A check that is not listed succeeds.
type answeringProber struct {
	answers map[string]error
}

func (p answeringProber) Probe(_ context.Context, method, path string, _ any) error {
	return p.answers[method+" "+path]
}

func insufficient(scopes string, requirement string) error {
	return &portals.APIError{
		Status:  http.StatusForbidden,
		Code:    "insufficient_scope",
		Message: "Insufficient scope, expected " + requirement + " of: " + scopes,
	}
}

var probesForTest = []portals.Probe{
	{Name: "create client", Method: http.MethodPost, Path: "clients", Body: map[string]any{}},
	{Name: "import forms", Method: http.MethodPost, Path: "forms/import", Body: map[string]any{}},
	{Name: "create portal", Method: http.MethodPost, Path: "portals", Body: map[string]any{}},
}

func TestRunPreflight(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		answers        map[string]error
		probes         []portals.Probe
		wantGaps       []portals.ScopeGap
		wantUnexpected []portals.UnexpectedProbe
	}{
		{name: "a scope the session has is not reported"},
		{
			name:    "a validation error means the scope is present",
			answers: map[string]error{"POST clients": &portals.APIError{Status: 400, Message: "Payload validation error"}},
		},
		{
			name: "every missing scope is reported at once, with the requirement",
			answers: map[string]error{
				"POST clients":      insufficient("create:clients", "any"),
				"POST forms/import": insufficient("create:forms,create:flows", "all"),
				"POST portals":      insufficient("create:portals", "any"),
			},
			wantGaps: []portals.ScopeGap{
				{Probe: "create client", Requirement: portals.RequirementAny, Scopes: []string{"create:clients"}},
				{Probe: "import forms", Requirement: portals.RequirementAll, Scopes: []string{"create:forms", "create:flows"}},
				{Probe: "create portal", Requirement: portals.RequirementAny, Scopes: []string{"create:portals"}},
			},
		},
		{
			name:    "a 403 without a scope message is not a scope gap",
			answers: map[string]error{"POST clients": &portals.APIError{Status: 403, Message: "Forbidden"}},
			wantUnexpected: []portals.UnexpectedProbe{
				{Probe: "create client", Status: 403, Message: "Forbidden"},
			},
		},
		{
			name: "a 404 means the scope is present only for a probe on a resource that does not exist",
			probes: []portals.Probe{
				{Name: "update client grant", Method: http.MethodPatch, Path: "client-grants/cgr_fake", FakeTarget: true},
				{Name: "list portals", Method: http.MethodGet, Path: "portals"},
			},
			answers: map[string]error{
				"PATCH client-grants/cgr_fake": &portals.APIError{Status: 404, Message: "The grant does not exist"},
				"GET portals":                  &portals.APIError{Status: 404, Message: "Not Found"},
			},
			wantUnexpected: []portals.UnexpectedProbe{
				{Probe: "list portals", Status: 404, Message: "Not Found"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			probes := probesForTest
			if tt.probes != nil {
				probes = tt.probes
			}

			result, err := portals.RunPreflight(context.Background(), answeringProber{answers: tt.answers}, probes, "acme.us.auth0.com")

			require.NoError(t, err)
			assert.Equal(t, tt.wantGaps, result.Gaps)
			assert.Equal(t, tt.wantUnexpected, result.Unexpected)
		})
	}
}

func TestRunPreflight_FeatureNotEnabled(t *testing.T) {
	t.Parallel()

	prober := answeringProber{answers: map[string]error{
		"POST portals": &portals.APIError{Status: 403, Code: "feature_not_enabled", Message: "Feature not enabled"},
	}}

	_, err := portals.RunPreflight(context.Background(), prober, probesForTest, "acme.us.auth0.com")

	var notEnabled *portals.FeatureNotEnabledError
	require.ErrorAs(t, err, &notEnabled)
	assert.Equal(t,
		"Universal Portals is not enabled for acme.us.auth0.com. Ask Auth0 to enable it for this tenant, then run the command again",
		err.Error(),
	)
}

func TestRunPreflight_AnErrorThatIsNotFromTheAPIStopsTheRun(t *testing.T) {
	t.Parallel()

	failure := errors.New("connection refused")
	prober := answeringProber{answers: map[string]error{"POST forms/import": failure}}

	_, err := portals.RunPreflight(context.Background(), prober, probesForTest, "acme.us.auth0.com")

	require.ErrorIs(t, err, failure)
	assert.Contains(t, err.Error(), `check "import forms"`)
}

// gatedProber holds back the first probe until the last one is answered, so the answers arrive in
// the reverse order of the probes.
type gatedProber struct {
	once        sync.Once
	lastAnswerd chan struct{}
}

func (p *gatedProber) Probe(_ context.Context, _, path string, _ any) error {
	if path == "clients" {
		<-p.lastAnswerd

		return insufficient("create:clients", "any")
	}

	if path == "portals" {
		defer p.once.Do(func() { close(p.lastAnswerd) })

		return insufficient("create:portals", "any")
	}

	return nil
}

func TestRunPreflight_ReportsInTheOrderOfTheProbes(t *testing.T) {
	t.Parallel()

	prober := &gatedProber{lastAnswerd: make(chan struct{})}

	result, err := portals.RunPreflight(context.Background(), prober, probesForTest, "acme.us.auth0.com")

	require.NoError(t, err)
	require.Len(t, result.Gaps, 2)
	assert.Equal(t, "create client", result.Gaps[0].Probe, "the first probe answered last, and is still reported first")
	assert.Equal(t, "create portal", result.Gaps[1].Probe)
}

func TestPreflightResult_Err(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		result      portals.PreflightResult
		wantMessage string
		wantFixes   int
		wantNoError bool
	}{
		{name: "nothing is missing", wantNoError: true},
		{
			name: "a check that failed for another reason does not stop setup",
			result: portals.PreflightResult{
				Unexpected: []portals.UnexpectedProbe{{Probe: "list portals", Status: 404, Message: "Not Found"}},
			},
			wantNoError: true,
		},
		{
			name: "scopes that are all needed, sorted and without duplicates",
			result: portals.PreflightResult{Gaps: []portals.ScopeGap{
				{Probe: "create portal", Requirement: portals.RequirementAny, Scopes: []string{"create:portals"}},
				{Probe: "import forms", Requirement: portals.RequirementAll, Scopes: []string{"create:forms", "create:flows"}},
				{Probe: "create form", Requirement: portals.RequirementAny, Scopes: []string{"create:forms"}},
			}},
			wantMessage: "The CLI session is missing 3 permission(s) setup needs: create:flows, create:forms, create:portals. Nothing was created",
			wantFixes:   2,
		},
		{
			name: "a choice is listed as one of the scopes",
			result: portals.PreflightResult{Gaps: []portals.ScopeGap{
				{Probe: "create client", Requirement: portals.RequirementAny, Scopes: []string{"create:clients"}},
				{Probe: "create portal", Requirement: portals.RequirementAny, Scopes: []string{"create:portals", "create:portals_beta"}},
			}},
			wantMessage: "The CLI session is missing 2 permission(s) setup needs: create:clients, one of create:portals or create:portals_beta. Nothing was created",
			wantFixes:   2,
		},
		{
			name: "a refusal without a reason is its own error",
			result: portals.PreflightResult{
				Unexpected: []portals.UnexpectedProbe{
					{Probe: "create portal", Status: 403, Message: "Forbidden"},
					{Probe: "import forms", Status: 403, Message: "Forbidden"},
				},
				Gaps: []portals.ScopeGap{{Requirement: portals.RequirementAny, Scopes: []string{"create:clients"}}},
			},
			wantMessage: `The Management API refused "create portal", "import forms" with HTTP 403 and gave no reason. Nothing was created. ` +
				`Usually Universal Portals is not enabled for acme.us.auth0.com, or the session is missing a permission`,
			wantFixes: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.result.Err("acme.us.auth0.com")

			if tt.wantNoError {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.Equal(t, tt.wantMessage, err.Error())
			assert.Len(t, portals.FixesOf(err), tt.wantFixes)
		})
	}
}
