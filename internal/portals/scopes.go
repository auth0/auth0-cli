package portals

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
)

// requiredScopes are the Management API scopes the session of the tool needs. The advice for a user
// login asks for all of them, because `auth0 login --scopes` replaces the scopes of the last login.
var requiredScopes = []string{
	"read:clients",
	"create:clients",
	"update:clients",
	"read:client_keys",
	"read:client_grants",
	"create:client_grants",
	"update:client_grants",
	"delete:client_grants",
	"read:resource_servers",
	"create:resource_servers",
	"read:flows",
	"create:flows",
	"update:flows",
	"read:flows_vault_connections",
	"create:flows_vault_connections",
	"update:flows_vault_connections",
	"read:forms",
	"create:forms",
	"read:portals",
	"create:portals",
	"update:portals",
	"read:custom_domains",
	"read:users",
	"read:organizations",
	"create:organizations",
	"read:organization_members",
	"create:organization_members",
	"read:organization_member_roles",
	"create:organization_member_roles",
	"read:organization_connections",
	"create:organization_connections",
	"read:connections",
	"read:roles",
	"create:roles",
	"update:roles",
}

// RequiredScopes returns the Management API scopes the session of the tool needs.
func RequiredScopes() []string {
	return slices.Clone(requiredScopes)
}

// ScopeFixes tells how to get the missing scopes, for each way the CLI can be signed in. A user
// login asks for every scope the tool uses and not only the missing ones, because the login command
// drops the scopes of the last login. A client-credentials login keeps its grant, so only the
// missing scopes are added to it.
func ScopeFixes(missing []string) []string {
	login := slices.Clone(requiredScopes)
	for _, scope := range missing {
		if !slices.Contains(login, scope) {
			login = append(login, scope)
		}
	}

	grant := fmt.Sprintf("Signed in with client credentials: add %s to the application's Management API grant", strings.Join(missing, ", "))
	if len(missing) == 0 {
		grant = fmt.Sprintf(
			"Signed in with client credentials: check that the application's Management API grant has %s",
			strings.Join(requiredScopes, ", "),
		)
	}

	return []string{
		fmt.Sprintf("Signed in as a user: auth0 login --scopes %s", strings.Join(login, ",")),
		grant,
	}
}

// Requirement tells how many of the scopes of a gap are needed.
type Requirement int

const (
	// RequirementUnknown is the zero value, which no gap has.
	RequirementUnknown Requirement = iota
	// RequirementAny means that one of the scopes is enough.
	RequirementAny
	// RequirementAll means that every one of the scopes is needed.
	RequirementAll
)

// ScopeGap is what a refused request says it needed.
type ScopeGap struct {
	// Probe is the name of the check that was refused, empty for another request.
	Probe       string
	Requirement Requirement
	Scopes      []string
}

// IsChoice reports whether any one of several scopes would do, so none of them is required.
func (g ScopeGap) IsChoice() bool {
	return g.Requirement == RequirementAny && len(g.Scopes) > 1
}

// Required returns the scopes to ask for: all of them, or the first when any one is enough.
func (g ScopeGap) Required() []string {
	if g.Requirement == RequirementAny && len(g.Scopes) > 0 {
		return g.Scopes[:1]
	}

	return g.Scopes
}

// scopeMessage matches the message of the Management API for a missing scope:
// "Insufficient scope, expected any of: a,b" or "Insufficient scope, expected all of: a,b".
var scopeMessage = regexp.MustCompile(`expected (any|all) of:\s*(.+)$`)

// ParseScopeGap reads a refused request. It reports false for any other error.
func ParseScopeGap(probe string, err error) (ScopeGap, bool) {
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok || apiErr.Status != http.StatusForbidden {
		return ScopeGap{}, false
	}

	match := scopeMessage.FindStringSubmatch(apiErr.Message)
	if match == nil {
		return ScopeGap{}, false
	}

	requirement := RequirementAny
	if match[1] == "all" {
		requirement = RequirementAll
	}

	var scopes []string
	for _, scope := range strings.Split(match[2], ",") {
		if scope = strings.TrimSpace(scope); scope != "" {
			scopes = append(scopes, scope)
		}
	}

	return ScopeGap{Probe: probe, Requirement: requirement, Scopes: scopes}, true
}
