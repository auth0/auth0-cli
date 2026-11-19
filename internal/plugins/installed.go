package plugins

import (
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"slices"
	"strings"
)

// ParseInstalledManifest validates data as the plugin.json of an installed
// plugin, for the CLI runtime. It is forward compatible where that is safe, so
// that a plugin built against a slightly newer manifest still runs on an older
// CLI, and it fails closed where it is not.
//
// It behaves as follows:
//   - An apiVersion greater than 1 is rejected with an error saying that the
//     plugin needs a newer auth0 CLI.
//   - An unknown field inside any permissions object, including the entries of
//     the hosts, files and env lists, is rejected with an error that names the
//     field and says that the plugin needs a newer auth0 CLI. A permission this
//     CLI does not understand must never be silently ignored.
//   - Unknown fields anywhere else are ignored. The manifest is decoded without
//     rejecting unknown fields, encoded again, and the result goes through
//     ParseManifest, so all of the normal validation still applies to every
//     field this version knows about.
//
// Because the second step validates the manifest as re-encoded, a field that is
// missing or has the wrong type may be reported as an invalid value instead of
// a missing field. When the manifest cannot be decoded at all, the errors of
// ParseManifest are returned instead.
//
// Problems are returned as a *ValidationError.
func ParseInstalledManifest(data []byte) (*Manifest, error) {
	document, err := decodeDocument(data)
	if err != nil {
		return nil, err
	}

	root, ok := document.(map[string]any)
	if !ok {
		return ParseManifest(data)
	}

	if version, ok := root["apiVersion"].(json.Number); ok {
		if number, ok := new(big.Rat).SetString(version.String()); ok && number.IsInt() && number.Cmp(big.NewRat(1, 1)) > 0 {
			return nil, &ValidationError{Errors: []string{fmt.Sprintf(
				"apiVersion: the plugin targets plugin API version %s, which needs a newer auth0 CLI", version,
			)}}
		}
	}

	if problems := unknownPermissionFields(root); len(problems) > 0 {
		return nil, &ValidationError{Errors: problems}
	}

	var lenient Manifest
	if err := json.Unmarshal(data, &lenient); err != nil {
		return ParseManifest(data)
	}

	normalized, err := json.Marshal(lenient)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the plugin manifest: %w", err)
	}

	return ParseManifest(normalized)
}

// jsonFieldNames returns the JSON field names of a struct type.
func jsonFieldNames(structType reflect.Type) map[string]bool {
	names := make(map[string]bool, structType.NumField())

	for field := range structType.Fields() {
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name != "" && name != "-" {
			names[name] = true
		}
	}

	return names
}

// unknownKeys reports every key of object that is not in known.
func unknownKeys(path string, object map[string]any, known map[string]bool) []string {
	var problems []string

	for key := range object {
		if !known[key] {
			problems = append(problems, fmt.Sprintf("%s.%s: unknown permission field, the plugin needs a newer auth0 CLI", path, key))
		}
	}

	return problems
}

// unknownPermissionFields reports every field this version does not know inside
// the permissions of any command, including the entries of the hosts, files and
// env lists. The document is the manifest decoded into generic values. The
// problems are sorted so that the result does not depend on map order.
func unknownPermissionFields(root map[string]any) []string {
	permissionFields := jsonFieldNames(reflect.TypeFor[Permissions]())
	entryFields := map[string]map[string]bool{
		"hosts": jsonFieldNames(reflect.TypeFor[HostPermission]()),
		"files": jsonFieldNames(reflect.TypeFor[FilePermission]()),
		"env":   jsonFieldNames(reflect.TypeFor[EnvPermission]()),
	}

	var problems []string

	// The lenient decode matches keys case-insensitively, so the lookups here
	// must as well. Otherwise a key such as "Permissions" would skip this check
	// and still be decoded.
	for _, commandList := range valuesIgnoringCase(root, "commands") {
		commands, _ := commandList.([]any)
		for i, item := range commands {
			command, ok := item.(map[string]any)
			if !ok {
				continue
			}

			for _, value := range valuesIgnoringCase(command, "permissions") {
				permissions, ok := value.(map[string]any)
				if !ok {
					continue
				}

				permissionsPath := itemPath("", "commands", i) + ".permissions"
				problems = append(problems, unknownKeys(permissionsPath, permissions, permissionFields)...)

				for list, fields := range entryFields {
					entries, _ := permissions[list].([]any)
					for j, entry := range entries {
						if object, ok := entry.(map[string]any); ok {
							problems = append(problems, unknownKeys(itemPath(permissionsPath, list, j), object, fields)...)
						}
					}
				}
			}
		}
	}

	slices.Sort(problems)

	return problems
}

// valuesIgnoringCase returns the values of every key of object that equals name
// when case is ignored.
func valuesIgnoringCase(object map[string]any, name string) []any {
	var values []any

	for key, value := range object {
		if strings.EqualFold(key, name) {
			values = append(values, value)
		}
	}

	return values
}
