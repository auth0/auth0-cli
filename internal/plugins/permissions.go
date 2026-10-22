package plugins

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

const hashPrefix = "sha256:"

// Permissions lists what a plugin command asks the user to allow.
type Permissions struct {
	Scopes []string         `json:"scopes"`
	Hosts  []HostPermission `json:"hosts"`
	Files  []FilePermission `json:"files"`
	Env    []EnvPermission  `json:"env"`
}

// HostPermission asks to contact a host.
type HostPermission struct {
	Host   string `json:"host"`
	Reason string `json:"reason"`
}

// FilePermission asks to read or write a file system location.
type FilePermission struct {
	Path   string     `json:"path"`
	Access FileAccess `json:"access"`
	Reason string     `json:"reason"`
}

// EnvPermission asks to read an environment variable.
type EnvPermission struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// Permissions returns the normalized union of the permissions requested by all
// commands. Scopes are sorted and deduplicated, hosts are sorted by host and
// then reason, files by path, access and reason, and env by name and then
// reason. Exact duplicates are dropped, and no slice is ever nil.
func (m *Manifest) Permissions() Permissions {
	union := Permissions{
		Scopes: []string{},
		Hosts:  []HostPermission{},
		Files:  []FilePermission{},
		Env:    []EnvPermission{},
	}

	for _, command := range m.Commands {
		if command.Permissions == nil {
			continue
		}

		union.Scopes = append(union.Scopes, command.Permissions.Scopes...)
		union.Hosts = append(union.Hosts, command.Permissions.Hosts...)
		union.Files = append(union.Files, command.Permissions.Files...)
		union.Env = append(union.Env, command.Permissions.Env...)
	}

	union.Scopes = sortedUnique(union.Scopes, strings.Compare)
	union.Hosts = sortedUnique(union.Hosts, func(a, b HostPermission) int {
		return cmp.Or(cmp.Compare(a.Host, b.Host), cmp.Compare(a.Reason, b.Reason))
	})
	union.Files = sortedUnique(union.Files, func(a, b FilePermission) int {
		return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Access, b.Access), cmp.Compare(a.Reason, b.Reason))
	})
	union.Env = sortedUnique(union.Env, func(a, b EnvPermission) int {
		return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.Reason, b.Reason))
	})

	return union
}

// sortedUnique sorts items with compare and drops adjacent duplicates.
func sortedUnique[T comparable](items []T, compare func(a, b T) int) []T {
	slices.SortFunc(items, compare)

	return slices.Compact(items)
}

// consentCommand is the per-command part of the consent hash.
type consentCommand struct {
	name        string
	interactive bool
}

// PermissionsHash returns "sha256:" followed by the lowercase hex SHA-256 of
// the canonical JSON of the consent document for the manifest.
//
// The consent document is the permissions union returned by Permissions,
// followed by one entry per command holding the command name and its
// interactive value, sorted by command name so that the command order in the
// manifest does not matter. The canonical JSON has no whitespace, no HTML
// escaping, and this fixed shape:
//
//	{"scopes":[...],"hosts":[{"host":..,"reason":..}],
//	 "files":[{"path":..,"access":..,"reason":..}],
//	 "env":[{"name":..,"reason":..}],
//	 "commands":[{"name":..,"interactive":..}]}
//
// Reasons are part of the hash on purpose, so a changed reason produces a new
// hash and asks the user for consent again. The same goes for flipping the
// interactive value of any command. Everything else, such as descriptions,
// flags, timeouts, output settings and the plugin version, is not part of the
// hash.
//
// The JSON is written by hand rather than by encoding/json. That keeps the hash
// free of an error path, and keeps it from depending on the encoding rules of
// the Go version that built the CLI. The string escaping matches what
// encoding/json writes with HTML escaping turned off.
func (m *Manifest) PermissionsHash() string {
	union := m.Permissions()

	commands := make([]consentCommand, 0, len(m.Commands))
	for _, command := range m.Commands {
		commands = append(commands, consentCommand{name: command.Name, interactive: command.Interactive})
	}
	slices.SortFunc(commands, func(a, b consentCommand) int {
		return cmp.Compare(a.name, b.name)
	})

	document := []byte(`{"scopes":`)
	document = appendList(document, union.Scopes, appendJSONString)
	document = append(document, `,"hosts":`...)
	document = appendList(document, union.Hosts, func(dst []byte, host HostPermission) []byte {
		return appendObject(dst, "host", host.Host, "reason", host.Reason)
	})
	document = append(document, `,"files":`...)
	document = appendList(document, union.Files, func(dst []byte, file FilePermission) []byte {
		return appendObject(dst, "path", file.Path, "access", string(file.Access), "reason", file.Reason)
	})
	document = append(document, `,"env":`...)
	document = appendList(document, union.Env, func(dst []byte, env EnvPermission) []byte {
		return appendObject(dst, "name", env.Name, "reason", env.Reason)
	})
	document = append(document, `,"commands":`...)
	document = appendList(document, commands, func(dst []byte, command consentCommand) []byte {
		dst = append(dst, `{"name":`...)
		dst = appendJSONString(dst, command.name)
		dst = append(dst, `,"interactive":`...)
		dst = strconv.AppendBool(dst, command.interactive)

		return append(dst, '}')
	})
	document = append(document, '}')

	sum := sha256.Sum256(document)

	return hashPrefix + hex.EncodeToString(sum[:])
}

// appendList appends items as a JSON array, using appendItem for each one.
func appendList[T any](dst []byte, items []T, appendItem func(dst []byte, item T) []byte) []byte {
	dst = append(dst, '[')
	for i, item := range items {
		if i > 0 {
			dst = append(dst, ',')
		}
		dst = appendItem(dst, item)
	}

	return append(dst, ']')
}

// appendObject appends a JSON object with string values from alternating keys
// and values.
func appendObject(dst []byte, keysAndValues ...string) []byte {
	dst = append(dst, '{')
	for i := 0; i < len(keysAndValues); i += 2 {
		if i > 0 {
			dst = append(dst, ',')
		}
		dst = appendJSONString(dst, keysAndValues[i])
		dst = append(dst, ':')
		dst = appendJSONString(dst, keysAndValues[i+1])
	}

	return append(dst, '}')
}

const hexDigits = "0123456789abcdef"

// appendJSONString appends s as a JSON string. It escapes exactly what
// encoding/json escapes when HTML escaping is turned off, which is quotes,
// backslashes, control characters, the line and paragraph separators U+2028 and
// U+2029, and bytes that are not valid UTF-8.
//
// It handles every string so that PermissionsHash works on a Manifest that was
// not built by ParseManifest. A manifest that passed validation never contains
// the characters that other JSON encoders write differently, such as control
// characters, DEL and the separators U+2028 and U+2029, because the schema
// rejects them in every field that feeds the hash.
func appendJSONString(dst []byte, s string) []byte {
	dst = append(dst, '"')

	for i := 0; i < len(s); {
		b := s[i]

		if b < utf8.RuneSelf {
			switch b {
			case '"', '\\':
				dst = append(dst, '\\', b)
			case '\b':
				dst = append(dst, '\\', 'b')
			case '\f':
				dst = append(dst, '\\', 'f')
			case '\n':
				dst = append(dst, '\\', 'n')
			case '\r':
				dst = append(dst, '\\', 'r')
			case '\t':
				dst = append(dst, '\\', 't')
			default:
				if b < 0x20 {
					dst = append(dst, '\\', 'u', '0', '0', hexDigits[b>>4], hexDigits[b&0xF])
				} else {
					dst = append(dst, b)
				}
			}
			i++

			continue
		}

		r, size := utf8.DecodeRuneInString(s[i:])

		switch {
		case r == utf8.RuneError && size == 1:
			dst = append(dst, `\ufffd`...)
		case r == '\u2028' || r == '\u2029':
			dst = append(dst, '\\', 'u', '2', '0', '2', hexDigits[r&0xF])
		default:
			dst = append(dst, s[i:i+size]...)
		}
		i += size
	}

	return append(dst, '"')
}
