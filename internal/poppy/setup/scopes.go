package setup

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// BrowserSignInScope is the reserved scope the signed-in browser join needs.
// Setup adds it to every API that lacks it.
const BrowserSignInScope = "poppy:browser-sign-in"

// BrowserSignInDescription is the description setup gives BrowserSignInScope when
// the scope is missing or has none, unless --scope gives another.
const BrowserSignInDescription = "Use the website signed in, with the other access you approve"

// maxListedScopeFixes is the most --scope examples the FIX line of a missing
// scope description spells out.
const maxListedScopeFixes = 5

// apiScopes holds the scopes of an API once the --scope descriptions are added.
type apiScopes struct {
	All []Scope
	// Changed lists the scopes that get a description or are added, as the plan
	// shows them: a scope that is added reads "<name> (new)", and one that only
	// gets a description reads "<name> (description)".
	Changed []string
	// Details has the same scopes as Changed, with their descriptions.
	Details []ScopeChange
}

func newScope(name string) string { return name + " (new)" }

func describedScope(name string) string { return name + " (description)" }

// resolveScopes adds the --scope descriptions to the scopes of each API, by API
// identifier. A description only goes to a scope that has none: setup never edits
// one. A scope that no API has is added when there is exactly one API. The
// reserved scope BrowserSignInScope is the exception: every API gets it when it
// lacks it, and a missing description is filled in with the default or the one
// given, except when the web part is off, as the scope exists for the browser
// sign-in only. It stops when a scope offered to agents is left without a
// description.
func resolveScopes(state *State, in Inputs) (map[string]apiScopes, error) {
	return resolveScopesOf(state, in, true)
}

// resolveScopesOf is resolveScopes. With addReserved false it leaves
// BrowserSignInScope as the tenant has it, for the commands that describe the
// tenant instead of changing it.
func resolveScopesOf(state *State, in Inputs, addReserved bool) (map[string]apiScopes, error) {
	resolved, undescribed, err := collectScopes(state, in, addReserved)
	if err != nil {
		return nil, err
	}

	if len(undescribed) > 0 {
		return nil, scopeDescriptionRequired(undescribed)
	}

	return resolved, nil
}

// collectScopes does the work of resolveScopesOf and returns the scopes left
// without a description instead of stopping for them.
func collectScopes(state *State, in Inputs, addReserved bool) (map[string]apiScopes, []undescribedScope, error) {
	// The reserved scope exists for the browser sign-in only.
	addReserved = addReserved && !in.NoWeb

	resolved := make(map[string]apiScopes, len(state.APIs))
	seen := map[string]bool{}

	var undescribed []undescribedScope

	for _, api := range state.APIs {
		var res apiScopes

		for _, scope := range api.Scopes {
			if description, given := in.Scopes[scope.Name]; given {
				seen[scope.Name] = true

				switch {
				case scope.Description == description:
				case scope.Description != "":
					return nil, nil, &InputError{
						Reason:  ReasonScopeDescriptionExists,
						Message: fmt.Sprintf("Scope %s of %s already has a description, and setup never edits one.", scope.Name, api.Identifier),
						Fix:     fmt.Sprintf("Remove --scope %s, or change the description of the scope in the Auth0 Dashboard.", scope.Name),
					}
				default:
					scope.Description = description
					res.Changed = append(res.Changed, describedScope(scope.Name))
					res.Details = append(res.Details, ScopeChange{Name: scope.Name, Description: description})
				}
			}

			if addReserved && scope.Description == "" && scope.Name == BrowserSignInScope {
				scope.Description = BrowserSignInDescription
				res.Changed = append(res.Changed, describedScope(scope.Name))
				res.Details = append(res.Details, ScopeChange{Name: scope.Name, Description: BrowserSignInDescription})
			}

			if scope.Description == "" {
				undescribed = append(undescribed, undescribedScope{name: scope.Name, api: api.Identifier})
			}

			res.All = append(res.All, scope)
		}

		if addReserved && !slices.ContainsFunc(api.Scopes, func(s Scope) bool { return s.Name == BrowserSignInScope }) {
			seen[BrowserSignInScope] = true

			description := cmp.Or(in.Scopes[BrowserSignInScope], BrowserSignInDescription)
			res.All = append(res.All, Scope{Name: BrowserSignInScope, Description: description})
			res.Changed = append(res.Changed, newScope(BrowserSignInScope))
			res.Details = append(res.Details, ScopeChange{Name: BrowserSignInScope, Description: description, New: true})
		}

		resolved[api.Identifier] = res
	}

	for _, name := range slices.Sorted(maps.Keys(in.Scopes)) {
		if seen[name] {
			continue
		}

		if len(state.APIs) != 1 {
			return nil, nil, &InputError{
				Reason:  ReasonScopeNotFound,
				Message: fmt.Sprintf("Scope %s is not a scope of any of the APIs, and with several APIs there is no way to tell which one gets it.", name),
				Fix:     "Add the scope to its API first.",
			}
		}

		identifier := state.APIs[0].Identifier
		res := resolved[identifier]
		res.All = append(res.All, Scope{Name: name, Description: in.Scopes[name]})
		res.Changed = append(res.Changed, newScope(name))
		res.Details = append(res.Details, ScopeChange{Name: name, Description: in.Scopes[name], New: true})
		resolved[identifier] = res
	}

	return resolved, undescribed, nil
}

// undescribedScope is a scope of an API that has no description.
type undescribedScope struct {
	name string
	api  string
}

// scopeDescriptionRequired builds the error for scopes that agents would be
// offered without a description. The FIX line is a ready-to-paste --scope flag
// for each missing scope, up to maxListedScopeFixes of them.
func scopeDescriptionRequired(undescribed []undescribedScope) *InputError {
	labels := make([]string, len(undescribed))
	for i, scope := range undescribed {
		labels[i] = scope.name + " (" + scope.api + ")"
	}

	var names []string

	for _, scope := range undescribed {
		if !slices.Contains(names, scope.name) {
			names = append(names, scope.name)
		}
	}

	flag := func(name string) string { return "--scope '" + name + "=<what this lets an agent do>'" }

	fix := "Pass " + flag(names[0]) + "."

	if len(names) > 1 {
		var b strings.Builder

		b.WriteString("Pass --scope for each of them:")

		for i, name := range names {
			if i == maxListedScopeFixes {
				fmt.Fprintf(&b, "\n         and %d more", len(names)-i)

				break
			}

			b.WriteString("\n         ")
			b.WriteString(flag(name))
		}

		fix = b.String()
	}

	return &InputError{
		Reason:  ReasonScopeDescriptionRequired,
		Message: "Every scope offered to agents needs a description. These have none: " + strings.Join(labels, ", ") + ".",
		Fix:     fix,
	}
}

// HintScope is a scope that agents are offered and that has no hints yet.
type HintScope struct {
	Name        string
	Description string
	// APIs are the identifiers of the APIs that have the scope.
	APIs []string
}

// HintScopes returns the scopes of the APIs that have no hints, in the order of
// the APIs. A scope has hints when --hint gave it some or the existing config file
// has them. The reserved BrowserSignInScope never needs them: it has a default.
// With addReserved false the scopes are the ones the tenant has, as for a command
// that does not change it. It returns nothing when the scopes cannot be resolved,
// which the command reports in its own way.
func HintScopes(state *State, in Inputs, addReserved bool) []HintScope {
	scopes, err := resolveScopesOf(state, in, addReserved)
	if err != nil {
		return nil
	}

	var found []HintScope

	for _, api := range state.APIs {
		for _, scope := range scopes[api.Identifier].All {
			if scope.Name == BrowserSignInScope || hasHints(state, in, scope.Name) {
				continue
			}

			if i := slices.IndexFunc(found, func(h HintScope) bool { return h.Name == scope.Name }); i >= 0 {
				found[i].APIs = append(found[i].APIs, api.Identifier)

				continue
			}

			found = append(found, HintScope{Name: scope.Name, Description: scope.Description, APIs: []string{api.Identifier}})
		}
	}

	return found
}

func hasHints(state *State, in Inputs, scope string) bool {
	if _, found := in.Hints[scope]; found {
		return true
	}

	return state.File != nil && state.File.Scopes[scope].Hints != nil
}
