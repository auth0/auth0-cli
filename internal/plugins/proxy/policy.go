package proxy

import (
	"net/http"
	"slices"
	"strings"
	"unicode"

	"github.com/auth0/auth0-cli/internal/plugins/registry"
)

// managementPrefix is the path every call to the Management API starts with.
const managementPrefix = "/api/v2/"

// allowedMethods are the methods that the proxy forwards.
var allowedMethods = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}

// segment is one segment of a path template.
type segment struct {
	// The literal is the text a path segment must equal, empty for a parameter.
	literal string

	// A parameter matches any one non-empty segment.
	parameter bool
}

// rule is a compiled scope rule.
type rule struct {
	method   string
	path     string
	segments []segment
	anyOf    []string
}

// Policy holds the scope rules and the secret field names of a registry index,
// compiled for the proxy. It never changes after NewPolicy returns it, so it can
// be shared.
type Policy struct {
	// The rules are sorted so that the first rule that matches a call is the most
	// specific one.
	rules []rule

	// The secrets are the folded names of the fields removed from responses.
	secrets map[string]struct{}
}

// NewPolicy compiles the scope table and the secret fields of an index. It
// returns ErrNoScopeTable when the index has no rule and ErrNoSecretFields when
// it lists no name, so that a registry that does not publish them cannot make the
// proxy forward calls without limits or without removing secrets. It checks both
// with the validators of the registry package, because the index may not have
// come from ParseIndex.
func NewPolicy(index *registry.Index) (*Policy, error) {
	if index == nil || index.ScopeTable.FormatVersion != 1 || len(index.ScopeTable.Rules) == 0 {
		return nil, ErrNoScopeTable
	}

	if index.SecretFields.FormatVersion != 1 || len(index.SecretFields.Names) == 0 {
		return nil, ErrNoSecretFields
	}

	if err := index.ScopeTable.Validate(); err != nil {
		return nil, err
	}

	if err := index.SecretFields.Validate(); err != nil {
		return nil, err
	}

	policy := &Policy{
		rules:   make([]rule, 0, len(index.ScopeTable.Rules)),
		secrets: make(map[string]struct{}, len(index.SecretFields.Names)),
	}

	for _, source := range index.ScopeTable.Rules {
		policy.rules = append(policy.rules, compileRule(source))
	}

	slices.SortStableFunc(policy.rules, compareRules)

	for _, name := range index.SecretFields.Names {
		policy.secrets[foldName(name)] = struct{}{}
	}

	return policy, nil
}

// compileRule compiles the path of a rule that the registry package validated:
// it starts with the Management API prefix and every segment is a literal or a
// parameter in braces.
func compileRule(source registry.ScopeRule) rule {
	var segments []segment

	for part := range strings.SplitSeq(strings.TrimPrefix(source.Path, managementPrefix), "/") {
		if strings.HasPrefix(part, "{") {
			segments = append(segments, segment{parameter: true})
		} else {
			segments = append(segments, segment{literal: part})
		}
	}

	return rule{method: source.Method, path: source.Path, segments: segments, anyOf: slices.Clone(source.AnyOf)}
}

// compareRules orders rules so that, of two rules that match the same call, the
// more specific one comes first: at the first segment where they differ, a
// literal comes before a parameter. Rules for paths of different lengths never
// match the same call, so their order does not matter.
func compareRules(a, b rule) int {
	if len(a.segments) != len(b.segments) {
		return len(a.segments) - len(b.segments)
	}

	for n := range a.segments {
		x, y := a.segments[n], b.segments[n]

		switch {
		case x.parameter && !y.parameter:
			return 1
		case !x.parameter && y.parameter:
			return -1
		case x.literal != y.literal:
			return strings.Compare(x.literal, y.literal)
		}
	}

	return strings.Compare(a.method, b.method)
}

// match returns the most specific rule for the method and the segments of a
// path that come after the Management API prefix.
func (p *Policy) match(method string, segments []string) (rule, bool) {
	return p.find(method, segments, func(a, b string) bool { return a == b })
}

// ambiguous reports whether the call could be routed to another rule than the
// one that matched it. A server that ignores the case of a path would route the
// call by the rule that matches when the case of the literals is ignored, and
// the scope check must not have looked at a different rule. The matched rule is
// the one that match returned for the same method and segments.
func (p *Policy) ambiguous(method string, segments []string, matched rule) bool {
	folded, ok := p.find(method, segments, strings.EqualFold)

	return !ok || folded.path != matched.path
}

// find returns the first rule, which is the most specific, whose literals equal
// the segments of the call according to equal.
func (p *Policy) find(method string, segments []string, equal func(a, b string) bool) (rule, bool) {
	for n := range p.rules {
		if candidate := &p.rules[n]; candidate.method == method && candidate.matches(segments, equal) {
			return *candidate, true
		}
	}

	return rule{}, false
}

// matches reports whether the segments are a call that the rule describes.
func (r *rule) matches(segments []string, equal func(a, b string) bool) bool {
	if len(segments) != len(r.segments) {
		return false
	}

	for n, want := range r.segments {
		switch {
		case segments[n] == "":
			return false
		case !want.parameter && !equal(segments[n], want.literal):
			return false
		}
	}

	return true
}

// allowedBy reports whether the command declared at least one of the scopes that
// allow the call.
func (r *rule) allowedBy(declared map[string]struct{}) bool {
	return slices.ContainsFunc(r.anyOf, func(scope string) bool {
		_, ok := declared[scope]

		return ok
	})
}

// isSecret reports whether a field name is one that responses must not carry.
func (p *Policy) isSecret(name string) bool {
	_, ok := p.secrets[foldName(name)]

	return ok
}

// foldName returns the name with every letter replaced by the smallest letter
// that is equal to it when case is ignored. Two names get the same result when
// strings.EqualFold says they are equal, which covers letters that ToLower keeps
// apart, such as the long s and the Kelvin sign.
func foldName(name string) string {
	return strings.Map(func(r rune) rune {
		smallest := r
		for next := unicode.SimpleFold(r); next != r; next = unicode.SimpleFold(next) {
			smallest = min(smallest, next)
		}

		return smallest
	}, name)
}

// usable reports why a policy cannot be used to start a proxy, which is the case
// for the zero value and for nil.
func (p *Policy) usable() error {
	switch {
	case p == nil || len(p.rules) == 0:
		return ErrNoScopeTable
	case len(p.secrets) == 0:
		return ErrNoSecretFields
	}

	return nil
}
