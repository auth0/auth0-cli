package approvals

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/auth0/auth0-cli/internal/plugins"
)

const (
	// The width of the label column of the screen.
	labelWidth = 12

	// The width of what comes before the value of a row: the indent of two columns,
	// the label column and the space after it.
	indentWidth = 2 + labelWidth + 1

	// The most columns that a line of the exact scopes takes.
	maxLineWidth = 80

	// The most runes of one value from a manifest that the screen shows.
	maxValueRunes = 200

	// The line that tells the person what the CLI does not check.
	notEnforced = "Each command runs with only the permissions it needs. Hosts and files are declared by the publisher and are not enforced by the CLI."
)

// screen builds a consent screen.
type screen struct {
	text strings.Builder

	// How many rows were added.
	rows int
}

// row adds one line with a label and a value. The value is cleaned so that text
// from a manifest cannot move the cursor or rewrite earlier lines.
func (s *screen) row(label, value string) {
	fmt.Fprintf(&s.text, "  %-*s %s\n", labelWidth, label, clean(value))

	s.rows++
}

// wrapped adds the rows of a list of items that are shown as they are, with the
// label on the first row only. The items are separated by commas and the list
// wraps between two items, so that a line takes at most maxLineWidth columns and
// no item is split. An item that is wider than a line is alone on its line.
func (s *screen) wrapped(label string, items []string) {
	width := maxLineWidth - indentWidth

	var line string

	for n, item := range items {
		item = clean(item)
		if n < len(items)-1 {
			item += ","
		}

		switch {
		case line == "":
			line = item
		case utf8.RuneCountInString(line)+1+utf8.RuneCountInString(item) <= width:
			line += " " + item
		default:
			s.row(label, line)

			label, line = "", item
		}
	}

	if line != "" {
		s.row(label, line)
	}
}

// pairs adds one row for each pair, with the label on the first row only and
// the reasons aligned.
func (s *screen) pairs(label string, items [][2]string) {
	names := make([]string, len(items))
	width := 0

	for n, item := range items {
		names[n] = clean(item[0])
		width = max(width, utf8.RuneCountInString(names[n]))
	}

	for n, item := range items {
		if n > 0 {
			label = ""
		}

		s.row(label, fmt.Sprintf("%-*s    %s", width, names[n], item[1]))
	}
}

// scopeGroups holds scopes in the words the screen uses.
type scopeGroups struct {
	// The resources that can be read, such as "tenant settings".
	read []string

	// The resources that can be changed with the verbs allowed on each, such as
	// "clients (create, update, delete)".
	change []string

	// The scopes that are not of the form verb:resource, as they are.
	other []string
}

// changeVerbs are the verbs that the screen lists first, in this order.
var changeVerbs = []string{"create", "update", "delete"}

// groupScopes sorts scopes of the form verb:resource into what can be read and
// what can be changed, and keeps the rest as they are. A scope with no colon, or
// with an empty verb or resource, does not parse.
func groupScopes(scopes []string) scopeGroups {
	var groups scopeGroups

	reads := map[string]bool{}
	changes := map[string]map[string]bool{}

	for _, scope := range scopes {
		verb, resource, found := strings.Cut(scope, ":")

		switch {
		case !found || verb == "" || resource == "":
			groups.other = append(groups.other, scope)
		case verb == "read":
			reads[resource] = true
		default:
			if changes[resource] == nil {
				changes[resource] = map[string]bool{}
			}

			changes[resource][verb] = true
		}
	}

	for _, resource := range slices.Sorted(maps.Keys(reads)) {
		groups.read = append(groups.read, resourceText(resource))
	}

	for _, resource := range slices.Sorted(maps.Keys(changes)) {
		groups.change = append(groups.change, resourceText(resource)+" ("+strings.Join(orderVerbs(changes[resource]), ", ")+")")
	}

	slices.Sort(groups.other)
	groups.other = slices.Compact(groups.other)

	return groups
}

// resourceText returns the resource of a scope in words.
func resourceText(resource string) string {
	return strings.ReplaceAll(resource, "_", " ")
}

// orderVerbs returns the verbs as create, update and delete when they are there,
// followed by the others in alphabetical order.
func orderVerbs(verbs map[string]bool) []string {
	var ordered []string

	for _, verb := range changeVerbs {
		if verbs[verb] {
			ordered = append(ordered, verb)
		}
	}

	for _, verb := range slices.Sorted(maps.Keys(verbs)) {
		if !slices.Contains(changeVerbs, verb) {
			ordered = append(ordered, verb)
		}
	}

	return ordered
}

// scopeRows adds a row for each group of scopes that has something in it.
func (s *screen) scopeRows(groups scopeGroups) {
	if len(groups.read) > 0 {
		s.row("Can read", strings.Join(groups.read, ", "))
	}

	if len(groups.change) > 0 {
		s.row("Can change", strings.Join(groups.change, ", "))
	}

	if len(groups.other) > 0 {
		s.row("Other", strings.Join(groups.other, ", "))
	}
}

// render returns the screen for the request, which is the screen of an update
// when the plugin is installed already and the first screen otherwise.
func render(tenant string, req Request) string {
	if req.Old != nil {
		return renderChange(tenant, req)
	}

	return renderFirst(tenant, req)
}

// renderExactScopes returns the block that shows the scopes as they are, for the
// person who wants to see more than the groups of the screen: all the scopes on a
// first screen, and the scopes that were added and removed on the screen of an
// update. It is set off from the lines around it by a blank line before and after,
// and it is empty when there is no scope to show.
func renderExactScopes(req Request) string {
	s := &screen{}
	current := plugins.NormalizePermissions(req.New)

	if req.Old == nil {
		s.wrapped("Exact scopes", current.Scopes)
	} else {
		added, removed, _ := diffBy(plugins.NormalizePermissions(*req.Old).Scopes, current.Scopes, func(scope string) string { return scope })

		if len(added) > 0 {
			s.wrapped("Added", added)
		}

		if len(removed) > 0 {
			s.wrapped("Removed", removed)
		}
	}

	if s.rows == 0 {
		return ""
	}

	return "\n" + s.text.String() + "\n"
}

// renderFirst returns the screen for permissions that were not approved on the
// tenant before.
func renderFirst(tenant string, req Request) string {
	s := &screen{}
	permissions := plugins.NormalizePermissions(req.New)

	fmt.Fprintf(&s.text, "This plugin asks to use your Auth0 tenant %s:\n\n", clean(tenant))

	if len(permissions.Scopes) == 0 {
		s.row("Auth0 API", "none")
	} else {
		s.scopeRows(groupScopes(permissions.Scopes))
	}

	s.pairs("Hosts", pairsOf(permissions.Hosts, hostPair))
	s.pairs("Files", pairsOf(permissions.Files, filePair))
	s.pairs("Environment", pairsOf(permissions.Env, envPair))

	interactive := "no"
	if slices.ContainsFunc(req.NewCommands, func(c plugins.ConsentCommand) bool { return c.Interactive }) {
		interactive = "yes"
	}

	s.row("Interactive", interactive)

	s.text.WriteString("\n" + notEnforced + "\n\n")

	return s.text.String()
}

// renderChange returns the screen for an update: only what was added, removed
// or changed compared with the permissions of the installed version.
func renderChange(tenant string, req Request) string {
	s := &screen{}

	fmt.Fprintf(&s.text, "%s %s asks for different permissions on tenant %s\n", clean(req.ID), clean(req.Version), clean(tenant))

	s.changes(req)

	if s.rows == 0 {
		s.row("", "There is no difference in what the plugin asks for.")
	}

	s.text.WriteString(notEnforced + "\n\n")

	return s.text.String()
}

// showsChange reports whether the screen of an update has anything to show.
func showsChange(req Request) bool {
	s := &screen{}
	s.changes(req)

	return s.rows > 0
}

// changes adds a row for each permission and command that was added, removed or
// changed compared with the installed version.
func (s *screen) changes(req Request) {
	old, current := plugins.NormalizePermissions(*req.Old), plugins.NormalizePermissions(req.New)

	scopesAdded, scopesRemoved, _ := diffBy(old.Scopes, current.Scopes, func(scope string) string { return scope })
	s.scopeChanges("Added", scopesAdded)
	s.scopeChanges("Removed", scopesRemoved)

	changeRows(s, "host", old.Hosts, current.Hosts, func(h plugins.HostPermission) string { return h.Host }, hostPair)
	changeRows(s, "file", old.Files, current.Files, func(f plugins.FilePermission) string { return f.Path + "\x00" + string(f.Access) }, filePair)
	changeRows(s, "variable", old.Env, current.Env, func(e plugins.EnvPermission) string { return e.Name }, envPair)
	commandRows(s, req.OldCommands, req.NewCommands)
}

// scopeChanges adds the rows for scopes that were added or removed, which are
// the same groups as on the first screen.
func (s *screen) scopeChanges(label string, scopes []string) {
	groups := groupScopes(scopes)

	if len(groups.read) > 0 {
		s.row(label, "can read "+strings.Join(groups.read, ", "))
	}

	if len(groups.change) > 0 {
		s.row(label, "can change "+strings.Join(groups.change, ", "))
	}

	if len(groups.other) > 0 {
		s.row(label, "other scopes "+strings.Join(groups.other, ", "))
	}
}

// changeRows adds a row for each item that was added, removed or changed. Two
// items are the same item when they have the same key, and the item changed
// when anything else about it differs.
func changeRows[T comparable](s *screen, kind string, old, current []T, key func(T) string, pair func(T) [2]string) {
	added, removed, changed := diffBy(old, current, key)

	for _, group := range []struct {
		label string
		items []T
	}{{"Added", added}, {"Removed", removed}, {"Changed", changed}} {
		for _, item := range group.items {
			text := pair(item)

			s.row(group.label, strings.TrimSpace(fmt.Sprintf("%s %s    %s", kind, text[0], text[1])))
		}
	}
}

// commandRows adds a row for each command that was added or removed, or whose
// interactive value changed.
func commandRows(s *screen, old, current []plugins.ConsentCommand) {
	added, removed, changed := diffBy(old, current, func(c plugins.ConsentCommand) string { return c.Name })

	for _, command := range added {
		s.row("Added", "command "+command.Name+" ("+interactiveText(command.Interactive)+")")
	}

	for _, command := range removed {
		s.row("Removed", "command "+command.Name)
	}

	for _, command := range changed {
		s.row("Changed", "command "+command.Name+" is now "+interactiveText(command.Interactive))
	}
}

// diffBy compares two lists of items. An item that is only in current was added,
// one that is only in old was removed, and one that is in both under the same
// key but is not equal changed, in which case the new item is returned.
func diffBy[T comparable](old, current []T, key func(T) string) (added, removed, changed []T) {
	before := make(map[string]T, len(old))
	for _, item := range old {
		before[key(item)] = item
	}

	after := make(map[string]bool, len(current))

	for _, item := range current {
		after[key(item)] = true

		previous, found := before[key(item)]

		switch {
		case !found:
			added = append(added, item)
		case previous != item:
			changed = append(changed, item)
		}
	}

	for _, item := range old {
		if !after[key(item)] {
			removed = append(removed, item)
		}
	}

	return added, removed, changed
}

func hostPair(h plugins.HostPermission) [2]string {
	return [2]string{h.Host, freeText(h.Reason)}
}

func filePair(f plugins.FilePermission) [2]string {
	return [2]string{fmt.Sprintf("%s (%s)", freeText(f.Path), f.Access), freeText(f.Reason)}
}

func envPair(e plugins.EnvPermission) [2]string {
	return [2]string{e.Name, freeText(e.Reason)}
}

// pairsOf converts items to the pairs that screen.pairs shows.
func pairsOf[T any](items []T, pair func(T) [2]string) [][2]string {
	pairs := make([][2]string, 0, len(items))
	for _, item := range items {
		pairs = append(pairs, pair(item))
	}

	return pairs
}

func interactiveText(interactive bool) string {
	if interactive {
		return "interactive"
	}

	return "not interactive"
}

// clean replaces the control, format and line or paragraph separator characters
// of text that comes from a manifest. They could move the cursor, rewrite lines
// that were printed before, reorder the text with bidirectional controls, or hide
// text with zero-width characters.
func clean(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.In(r, unicode.Cc, unicode.Cf, unicode.Zl, unicode.Zp) {
			return unicode.ReplacementChar
		}

		return r
	}, text)
}

// freeText cleans a value that is free text in a manifest, such as a reason or a
// file path, so that it takes a single short line. It collapses every run of
// white space to one space, because a long run could push the rows that follow
// off the screen, or end in text that looks like a row of its own, and it cuts
// the value after maxValueRunes runes.
func freeText(text string) string {
	text = strings.Join(strings.Fields(clean(text)), " ")

	if utf8.RuneCountInString(text) <= maxValueRunes {
		return text
	}

	return string([]rune(text)[:maxValueRunes]) + "…"
}
