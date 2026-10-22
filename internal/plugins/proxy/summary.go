package proxy

import (
	"fmt"
	"slices"
	"strings"
	"sync"
)

// The limits on what the proxy remembers and says about denied calls.
const (
	maxRememberedDenials = 20
	maxListedDenials     = 3
)

// denial is a call that the proxy denied, with how often.
type denial struct {
	// The call as text that is safe to print.
	what string

	// The scope is the one that would have allowed the call, or empty.
	scope string

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
}

// deny records a call that the proxy denied. The scope is the one that would
// have allowed it, or empty when the reason is not a missing scope.
func (l *denialLog) deny(scope, method, path string) {
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

	entry := denial{what: what, scope: scope, count: 1}

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

	return fmt.Sprintf("Plugin %s: %s. Review permissions.scopes in plugin.json.", safeText(pluginID), strings.Join(parts, " and "))
}

// plural returns the count with the singular or the plural form of a noun.
func plural(count int, singular, many string) string {
	if count == 1 {
		return "1 " + singular
	}

	return fmt.Sprintf("%d %s", count, many)
}
