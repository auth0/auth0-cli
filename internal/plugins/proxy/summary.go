package proxy

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/auth0/auth0-cli/internal/plugins"
)

// The limits on what the proxy remembers and says about denied calls.
const (
	maxRememberedDenials = 20
	maxListedDenials     = 3

	// The longest path that a suggested rule names, in bytes.
	maxRulePathBytes = 256
)

// denial is a call that the proxy denied, with how often.
type denial struct {
	// The call as text that is safe to print.
	what string

	// The scope is the one that would have allowed the call, or empty.
	scope string

	// The method and the path of the call, as text that is safe to print.
	method string
	path   string

	// The path as it is, set only when a rule of the dev policy can hold it: it is
	// at most maxRulePathBytes long and made of the characters of a path that a
	// rule may name.
	rulePath string

	// Unmatched is set when no rule of the scope table matched the call, as
	// opposed to a rule that matched while the plugin lacked the scope.
	unmatched bool

	count int
}

// denialLog records the calls that were denied so that the caller can say so
// after the plugin ran. Only the first maxRememberedDenials different calls are
// kept, and the rest are only counted. A call that was denied for a missing scope
// is kept in preference to a call that was not.
type denialLog struct {
	mu sync.Mutex

	// The kept denials are the different ones, in the order they first happened.
	kept []denial

	// The total is how many calls the proxy denied.
	total int

	// The refused calls are those that Auth0 refused with a 403, which means the
	// session lacks a scope that the plugin declared.
	refused int

	// The dev policy file is set only for a linked build, and makes the summary
	// say how to fix a denial in that file. It is read-only after Start.
	devPolicyFile string
}

// deny records a call that the proxy denied. The scope is the one that would
// have allowed it, or empty when the reason is not a missing scope.
func (l *denialLog) deny(scope, method, path string) {
	l.record(scope, method, path, false)
}

// denyUnmatched records a call that no rule of the scope table matched.
func (l *denialLog) denyUnmatched(method, path string) {
	l.record("", method, path, true)
}

// record records a denied call, see deny.
func (l *denialLog) record(scope, method, path string, unmatched bool) {
	what := safeText(method) + " " + safeText(path)
	if scope != "" {
		what = safeText(scope) + " for " + what
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.total++

	for n := range l.kept {
		if l.kept[n].what == what {
			l.kept[n].count++

			return
		}
	}

	entry := denial{what: what, scope: scope, method: safeText(method), path: safeText(path), unmatched: unmatched, count: 1}
	if unmatched && fitsRule(path) {
		entry.rulePath = path
	}

	if len(l.kept) < maxRememberedDenials {
		l.kept = append(l.kept, entry)

		return
	}

	// A scope that is not kept yet takes the place of a denial that the summary
	// would not list first, so that a plugin cannot push the missing scope out of
	// the summary by making many other calls first.
	if scope == "" || slices.ContainsFunc(l.kept, func(d denial) bool { return d.scope == scope }) {
		return
	}

	for n := len(l.kept) - 1; n >= 0; n-- {
		if l.kept[n].scope == "" || slices.ContainsFunc(l.kept[:n], func(d denial) bool { return d.scope == l.kept[n].scope }) {
			l.kept[n] = entry

			return
		}
	}
}

// listed returns the denials that the summary names: the first one of every
// scope, then the ones without a scope, at most maxListedDenials. The scope is
// what the user has to add to plugin.json, so it comes before anything else.
func (l *denialLog) listed() []denial {
	var scoped, unscoped []denial

	seen := make(map[string]struct{})

	for _, d := range l.kept {
		if d.scope == "" {
			unscoped = append(unscoped, d)

			continue
		}

		if _, ok := seen[d.scope]; !ok {
			seen[d.scope] = struct{}{}
			scoped = append(scoped, d)
		}
	}

	scoped = append(scoped, unscoped...)

	return scoped[:min(len(scoped), maxListedDenials)]
}

// refuse records a call that Auth0 refused with a 403.
func (l *denialLog) refuse() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.refused++
}

// summary returns one line that says what was denied, or an empty string when
// nothing was.
func (l *denialLog) summary(pluginID string) string {
	l.mu.Lock()
	defer l.mu.Unlock()

	var parts []string

	if l.total > 0 {
		listed := l.listed()
		covered := 0

		names := make([]string, 0, len(listed)+1)
		for _, d := range listed {
			names = append(names, d.what)
			covered += d.count
		}

		if other := l.total - covered; other > 0 {
			names = append(names, fmt.Sprintf("%d other", other))
		}

		parts = append(parts, fmt.Sprintf("%s denied (%s)", plural(l.total, "API call was", "API calls were"), strings.Join(names, ", ")))
	}

	if l.refused > 0 {
		parts = append(parts, fmt.Sprintf("Auth0 refused %s because your session lacks a scope", plural(l.refused, "call", "calls")))
	}

	if len(parts) == 0 {
		return ""
	}

	line := fmt.Sprintf("Plugin %s: %s. Review permissions.scopes in plugin.json.", safeText(pluginID), strings.Join(parts, " and "))

	if hints := l.devHints(); hints != "" {
		line += " " + hints
	}

	return line
}

// devHints says how to fix the denials that the summary lists, for a linked build:
// a call that no rule matched needs a rule in the dev policy, and a call whose
// rule matched needs the scope in plugin.json. It returns an empty string when
// the build has no dev policy, so that the summary of an installed plugin is not
// touched. The caller holds the lock.
func (l *denialLog) devHints() string {
	if l.devPolicyFile == "" {
		return ""
	}

	var hints []string

	for _, d := range l.listed() {
		switch {
		case d.unmatched:
			hints = append(hints, ruleHint(plugins.SafeText(l.devPolicyFile), d))
		case d.scope != "":
			hints = append(hints, fmt.Sprintf("Declare %q under permissions.scopes in plugin.json.", d.scope))
		}
	}

	return strings.Join(hints, " ")
}

// ruleHint says how to allow an unmatched call. It suggests a rule only for a path
// that a rule can hold as it is, because a path that was cut or changed to be safe
// to print would give a rule that does not match the call.
func ruleHint(file string, d denial) string {
	if d.rulePath == "" {
		return fmt.Sprintf("Add a rule to %s for %s %s. Use {id} for the parts that change.", file, d.method, d.path)
	}

	return fmt.Sprintf(`Add to %s: { "method": %q, "path": %q, "anyOf": ["<scope>"] } (use {id} where the path has an id)`, file, d.method, d.rulePath)
}

// fitsRule reports whether the path is short enough for a rule and holds only
// letters, digits and the characters . _ ~ / and -.
func fitsRule(path string) bool {
	if path == "" || len(path) > maxRulePathBytes {
		return false
	}

	for i := range len(path) {
		c := path[i]

		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '.', c == '_', c == '~', c == '/', c == '-':
		default:
			return false
		}
	}

	return true
}

// plural returns the count with the singular or the plural form of a noun.
func plural(count int, singular, many string) string {
	if count == 1 {
		return "1 " + singular
	}

	return fmt.Sprintf("%d %s", count, many)
}
