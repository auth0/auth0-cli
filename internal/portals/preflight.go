package portals

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
)

// PreflightResult is what the checks found.
type PreflightResult struct {
	// Gaps are the checks the API refused for a missing scope, in the order of the checks.
	Gaps []ScopeGap
	// Unexpected are the checks that failed for another reason.
	Unexpected []UnexpectedProbe
}

// UnexpectedProbe is a check that failed for a reason other than a missing scope.
type UnexpectedProbe struct {
	Probe   string
	Status  int
	Message string
}

// RunPreflight sends the checks together and reads the answers in the order of the checks, so the
// report and the first error do not depend on which answer came first.
func RunPreflight(ctx context.Context, prober Prober, probes []Probe, tenantDomain string) (PreflightResult, error) {
	failures := make([]error, len(probes))

	forEachParallel(len(probes), func(index int) {
		probe := probes[index]
		failures[index] = prober.Probe(ctx, probe.Method, probe.Path, probe.Body)
	})

	var result PreflightResult

	for index, failure := range failures {
		if failure == nil {
			continue
		}

		probe := probes[index]

		apiErr, ok := errors.AsType[*APIError](failure)
		if !ok {
			return PreflightResult{}, fmt.Errorf("check %q: %w", probe.Name, failure)
		}

		switch {
		case apiErr.Status == http.StatusBadRequest:
			// The API rejected the empty body, so the scope to write is present.
		case apiErr.Status == http.StatusNotFound && probe.FakeTarget:
			// The resource does not exist, so the scope to change it is present.
		case apiErr.Code == "feature_not_enabled":
			return PreflightResult{}, &FeatureNotEnabledError{Tenant: tenantDomain}
		default:
			if gap, isGap := ParseScopeGap(probe.Name, failure); isGap {
				result.Gaps = append(result.Gaps, gap)
				continue
			}

			result.Unexpected = append(result.Unexpected, UnexpectedProbe{
				Probe:   probe.Name,
				Status:  apiErr.Status,
				Message: apiErr.Message,
			})
		}
	}

	return result, nil
}

// RequiredScopes returns the scopes the session must have, sorted. A gap where any one of several
// scopes would do is not listed: it is a choice.
func (r PreflightResult) RequiredScopes() []string {
	var required []string

	for _, gap := range r.Gaps {
		if gap.IsChoice() {
			continue
		}

		for _, scope := range gap.Scopes {
			if !slices.Contains(required, scope) {
				required = append(required, scope)
			}
		}
	}

	slices.Sort(required)

	return required
}

// Choices returns the gaps where any one of several scopes would do.
func (r PreflightResult) Choices() []ScopeGap {
	var choices []ScopeGap

	for _, gap := range r.Gaps {
		if gap.IsChoice() {
			choices = append(choices, gap)
		}
	}

	return choices
}

// Err returns the error that stops setup, or nil when the session has every permission. A refusal
// without a reason is its own error, because it usually means Universal Portals is not enabled.
func (r PreflightResult) Err(tenantDomain string) error {
	var refused []string

	for _, unexpected := range r.Unexpected {
		if unexpected.Status == http.StatusForbidden {
			refused = append(refused, unexpected.Probe)
		}
	}

	if len(refused) > 0 {
		return &PermissionCheckError{Probes: refused, Tenant: tenantDomain}
	}

	required, choices := r.RequiredScopes(), r.Choices()
	if len(required) > 0 || len(choices) > 0 {
		return &MissingScopesError{Scopes: required, Choices: choices}
	}

	return nil
}
