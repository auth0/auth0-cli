package cli

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/auth0/auth0-cli/internal/portals"
)

// portalsResumeHint tells how to finish a setup that stopped half way.
const portalsResumeHint = "Run the same command again to resume. Existing resources are reused."

// portalsFixError carries the steps that resolve an error. They are shown after the message on a
// terminal, through the hint block of the root command, and listed in the details of the JSON error.
type portalsFixError struct {
	err   error
	fixes []string
	// resources are what a setup made before it failed.
	resources []portals.Resource
}

func (e portalsFixError) Error() string { return e.err.Error() }

func (e portalsFixError) Unwrap() error { return e.err }

func (e portalsFixError) ErrorDetails() json.RawMessage {
	raw, err := json.Marshal(struct {
		Fixes     []string           `json:"fixes,omitempty"`
		Resources []portals.Resource `json:"resources,omitempty"`
	}{Fixes: e.fixes, Resources: e.resources})
	if err != nil {
		return nil
	}

	return raw
}

// portalsCommandError gives an error of setup or audit the class and the reason the CLI reports, and
// the steps that resolve it. When wrote is true, a request changed the tenant before the error, so a
// rerun resumes. The tenant names the one a request was refused for when the feature is not enabled.
func portalsCommandError(err error, tenant string, wrote bool) error {
	if err == nil {
		return nil
	}

	// Any request tells it, not only the checks of setup, so the audit reports it the same way.
	if apiErr, ok := errors.AsType[*portals.APIError](err); ok && apiErr.Code == "feature_not_enabled" {
		err = &portals.FeatureNotEnabledError{Tenant: tenant}
	}

	classified := classifyPortalsError(err)
	fixes := portals.FixesOf(err)

	// A slug taken while setup runs is not resumed by a rerun with the same name.
	if _, race := errors.AsType[*portals.SlugRaceError](err); wrote && !race {
		fixes = append(fixes, portalsResumeHint)
	}

	// A dry run records what it would make, and made nothing, so only a real write has resources to show.
	var resources []portals.Resource
	if wrote {
		resources = portals.ResourcesOf(err)
	}

	if len(fixes) == 0 && len(resources) == 0 {
		return classified
	}

	return portalsFixError{
		err:       apiResponseHintError{err: classified, hint: strings.Join(fixes, "\n")},
		fixes:     fixes,
		resources: resources,
	}
}

func classifyPortalsError(err error) error {
	if _, ok := errors.AsType[*portals.UsageError](err); ok {
		return usageError{err: err, reason: "invalid_input"}
	}

	if _, ok := errors.AsType[*portals.BusinessAccessError](err); ok {
		return validationError{err: err, reason: "business_access"}
	}

	if _, ok := errors.AsType[*portals.NameInUseError](err); ok {
		return validationError{err: err, reason: "name_in_use"}
	}

	if _, ok := errors.AsType[*portals.SlugTakenError](err); ok {
		return validationError{err: err, reason: "slug_taken"}
	}

	if _, ok := errors.AsType[*portals.SlugRaceError](err); ok {
		return validationError{err: err, reason: "slug_race"}
	}

	if _, ok := errors.AsType[*portals.MissingScopesError](err); ok {
		return authError{err: err, reason: "missing_scopes"}
	}

	if _, ok := errors.AsType[*portals.ClientSecretUnreadableError](err); ok {
		return authError{err: err, reason: "missing_scopes"}
	}

	if _, ok := errors.AsType[*portals.PermissionCheckError](err); ok {
		return authError{err: err, reason: "permission_check_failed"}
	}

	if _, ok := errors.AsType[*portals.FeatureNotEnabledError](err); ok {
		return authError{err: err, reason: "feature_not_enabled"}
	}

	// A request refused for a missing scope is fixed the same way whichever command made it.
	if _, isGap := portals.ParseScopeGap("", err); isGap {
		return authError{err: err, reason: "insufficient_scope"}
	}

	if portals.IsStatus(err, http.StatusForbidden) {
		return authError{err: err, reason: "insufficient_scope"}
	}

	return err
}
