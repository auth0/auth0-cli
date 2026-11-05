package config

import (
	"maps"
	"slices"
	"strconv"
)

// ChangeKind says how a value differs between two configs.
type ChangeKind string

// The kinds of change reported by Diff.
const (
	ChangeAdded   ChangeKind = "added"
	ChangeRemoved ChangeKind = "removed"
	ChangeChanged ChangeKind = "changed"
)

// Change is one difference between two configs.
type Change struct {
	Path string
	Kind ChangeKind
	Old  string
	New  string
}

// Diff lists what differs between a and b, as the changes needed to go from a
// to b. A nil config counts as empty. The changes are sorted by path. Equal
// configs give an empty, non-nil slice.
func Diff(a, b *Config) []Change {
	oldValues, newValues := flatten(a), flatten(b)

	paths := maps.Clone(oldValues)
	maps.Copy(paths, newValues)

	out := []Change{}
	for _, path := range slices.Sorted(maps.Keys(paths)) {
		oldValue, inOld := oldValues[path]
		newValue, inNew := newValues[path]

		switch {
		case !inOld:
			out = append(out, Change{Path: path, Kind: ChangeAdded, New: newValue})
		case !inNew:
			out = append(out, Change{Path: path, Kind: ChangeRemoved, Old: oldValue})
		case oldValue != newValue:
			out = append(out, Change{Path: path, Kind: ChangeChanged, Old: oldValue, New: newValue})
		}
	}

	return out
}

// flatten maps every value of the config to its path in the file. A path is
// present only when its value is not empty, except for the hints of a scope,
// which are present whenever the scope has hints. The entries of apis are
// numbered, because the file has no key for them.
func flatten(c *Config) map[string]string {
	m := map[string]string{}
	if c == nil {
		return m
	}

	set := func(path, value string) {
		if value != "" {
			m[path] = value
		}
	}

	set("protocol_version", c.ProtocolVersion)
	set("organization.name", c.Organization.Name)
	set("organization.domain", c.Organization.Domain)
	set("organization.logo_uri", c.Organization.LogoURI)
	set("auth.issuer", c.Issuer)

	for name, scope := range c.Scopes {
		set("auth.scopes."+name, scope.Description)

		if scope.Hints != nil {
			m["auth.scope_hints."+name+".changes"] = scope.Hints.Changes
			m["auth.scope_hints."+name+".personal_data"] = strconv.FormatBool(scope.Hints.PersonalData)
		}
	}

	for i, api := range c.APIs {
		prefix := "apis[" + strconv.Itoa(i) + "]"
		set(prefix+".type", api.Type)
		set(prefix+".url", api.URL)
		set(prefix+".description", api.Description)
	}

	set("web.browser_session_endpoint", c.Web.BrowserSessionEndpoint)

	if c.Web.Unscoped {
		set("web.unscoped", "true")
	}

	if c.Web.Disabled {
		set("web.disabled", "true")
	}

	return m
}
