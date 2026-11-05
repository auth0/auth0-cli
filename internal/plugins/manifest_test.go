package plugins

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// manifestCase mutates the valid example manifest and lists the substrings
// that the resulting error must contain. An empty list means no error.
type manifestCase struct {
	name     string
	mutate   func(manifest map[string]any)
	wantErrs []string
}

func readValidManifest(t *testing.T) []byte {
	t.Helper()

	data, err := os.ReadFile("testdata/valid-manifest.json")
	require.NoError(t, err)

	return data
}

// mutateManifest applies fn to the valid manifest and returns the new JSON.
func mutateManifest(t *testing.T, fn func(manifest map[string]any)) []byte {
	t.Helper()

	var manifest map[string]any
	require.NoError(t, json.Unmarshal(readValidManifest(t), &manifest))

	fn(manifest)

	data, err := json.Marshal(manifest)
	require.NoError(t, err)

	return data
}

func parseMutated(t *testing.T, fn func(manifest map[string]any)) *Manifest {
	t.Helper()

	manifest, err := ParseManifest(mutateManifest(t, fn))
	require.NoError(t, err)

	return manifest
}

// follow walks path from root. A string step selects a key of an object and an
// int step selects an element of an array.
func follow(root any, path []any) any {
	current := root

	for _, step := range path {
		switch key := step.(type) {
		case string:
			current = current.(map[string]any)[key]
		case int:
			current = current.([]any)[key]
		}
	}

	return current
}

// setValue returns a mutation that sets the value at path.
func setValue(value any, path ...any) func(manifest map[string]any) {
	return func(manifest map[string]any) {
		parent := follow(manifest, path[:len(path)-1])

		switch last := path[len(path)-1].(type) {
		case string:
			parent.(map[string]any)[last] = value
		case int:
			parent.([]any)[last] = value
		}
	}
}

// removeValue returns a mutation that deletes the object key at path.
func removeValue(path ...any) func(manifest map[string]any) {
	return func(manifest map[string]any) {
		delete(follow(manifest, path[:len(path)-1]).(map[string]any), path[len(path)-1].(string))
	}
}

// setCommand and removeCommand are setValue and removeValue relative to the
// first command of the manifest.
func setCommand(value any, path ...any) func(manifest map[string]any) {
	return setValue(value, append([]any{"commands", 0}, path...)...)
}

func removeCommand(path ...any) func(manifest map[string]any) {
	return removeValue(append([]any{"commands", 0}, path...)...)
}

// combine returns a mutation that applies all of the given mutations.
func combine(mutations ...func(manifest map[string]any)) func(manifest map[string]any) {
	return func(manifest map[string]any) {
		for _, mutate := range mutations {
			mutate(manifest)
		}
	}
}

// requireValidationError asserts that err is a *ValidationError that contains
// all of the wanted substrings.
func requireValidationError(t *testing.T, err error, wantErrs ...string) {
	t.Helper()

	require.Error(t, err)

	var validationErr *ValidationError
	require.True(t, errors.As(err, &validationErr), "want *ValidationError, got %T", err)
	assert.NotEmpty(t, validationErr.Errors)

	for _, want := range wantErrs {
		assert.Contains(t, err.Error(), want)
	}
}

// schemaEnum returns the enum found at path in the embedded schema.
func schemaEnum(t *testing.T, path ...any) []string {
	t.Helper()

	var schema map[string]any
	require.NoError(t, json.Unmarshal(ManifestSchema(), &schema))

	var values []string
	for _, value := range follow(schema, path).([]any) {
		values = append(values, value.(string))
	}

	return values
}

func TestManifestSchema(t *testing.T) {
	t.Run("compiles", func(t *testing.T) {
		schema, err := compileSchema()
		require.NoError(t, err)
		assert.NotNil(t, schema)
	})

	t.Run("targets draft 2020-12", func(t *testing.T) {
		var schema map[string]any
		require.NoError(t, json.Unmarshal(ManifestSchema(), &schema))
		assert.Equal(t, "https://json-schema.org/draft/2020-12/schema", schema["$schema"])
	})

	t.Run("returns a copy that cannot change the embedded schema", func(t *testing.T) {
		first := ManifestSchema()
		original := string(first)
		first[0] = 'X'

		assert.Equal(t, original, string(ManifestSchema()))
		_, err := ParseManifest(readValidManifest(t))
		assert.NoError(t, err)
	})
}

func TestEnumsMatchSchema(t *testing.T) {
	t.Run("platforms", func(t *testing.T) {
		var got []string
		for _, platform := range AllPlatforms() {
			got = append(got, string(platform))
		}

		assert.Equal(t, schemaEnum(t, "properties", "platforms", "items", "enum"), got)
	})

	t.Run("flag types", func(t *testing.T) {
		got := []string{
			string(FlagTypeString), string(FlagTypeBool), string(FlagTypeInt), string(FlagTypeFloat), string(FlagTypeStringArray),
		}

		assert.Equal(t, schemaEnum(t, "$defs", "flag", "properties", "type", "enum"), got)
	})

	t.Run("file accesses", func(t *testing.T) {
		got := []string{string(FileAccessRead), string(FileAccessWrite)}

		assert.Equal(t, schemaEnum(t, "$defs", "permissions", "properties", "files", "items", "properties", "access", "enum"), got)
	})
}

func TestPlatform(t *testing.T) {
	tests := []struct {
		platform Platform
		os       string
		arch     string
		libc     string
	}{
		{platform: PlatformDarwinArm64, os: "darwin", arch: "arm64"},
		{platform: PlatformDarwinX64, os: "darwin", arch: "x64"},
		{platform: PlatformLinuxX64, os: "linux", arch: "x64", libc: "glibc"},
		{platform: PlatformLinuxArm64, os: "linux", arch: "arm64", libc: "glibc"},
		{platform: PlatformLinuxX64Musl, os: "linux", arch: "x64", libc: "musl"},
		{platform: PlatformLinuxArm64Musl, os: "linux", arch: "arm64", libc: "musl"},
		{platform: PlatformWindowsX64, os: "windows", arch: "x64"},
		{platform: Platform("freebsd-x64")},
	}

	for _, test := range tests {
		t.Run("describes "+string(test.platform), func(t *testing.T) {
			assert.Equal(t, test.os, test.platform.OS())
			assert.Equal(t, test.arch, test.platform.Arch())
			assert.Equal(t, test.libc, test.platform.Libc())
		})
	}

	t.Run("lists all seven platforms in manifest order", func(t *testing.T) {
		assert.Len(t, AllPlatforms(), 7)
		assert.Equal(t, PlatformDarwinArm64, AllPlatforms()[0])
	})

	t.Run("returns a list that callers can change", func(t *testing.T) {
		platforms := AllPlatforms()
		platforms[0] = "changed"

		assert.Equal(t, PlatformDarwinArm64, AllPlatforms()[0])
	})
}

func TestParseManifest_Valid(t *testing.T) {
	t.Run("parses the example manifest", func(t *testing.T) {
		manifest, err := ParseManifest(readValidManifest(t))
		require.NoError(t, err)

		assert.Equal(t, "https://raw.githubusercontent.com/auth0/auth0-cli-plugins/main/schema/plugin.v1.schema.json", manifest.Schema)
		assert.Equal(t, "auth0/checkmate", manifest.ID)
		assert.Equal(t, "checkmate", manifest.Name())
		assert.Equal(t, "1.0.0", manifest.Version)
		assert.Equal(t, 1, manifest.APIVersion)
		assert.Equal(t, "Run a tenant security audit", manifest.Description)
		assert.Equal(t, "CheckMate", manifest.Owner.Team)
		assert.Equal(t, "https://github.com/auth0/auth0-checkmate/issues", manifest.Owner.Bugs)
		assert.Empty(t, manifest.Homepage)
		require.NotNil(t, manifest.Engines)
		assert.Equal(t, ">=1.39.0", manifest.Engines.Auth0CLI)
		assert.Equal(t, AllPlatforms(), manifest.Platforms)

		require.Len(t, manifest.Commands, 1)
		command := manifest.Commands[0]
		assert.Equal(t, "checkmate", command.Name)
		assert.Equal(t, "10m", command.Timeout)
		assert.False(t, command.Interactive)
		assert.Equal(t, []Flag{{Name: "validators", Type: FlagTypeString, Description: "Validators to run"}}, command.Flags)
		assert.Equal(t, &Output{JSON: true, AgentMode: true}, command.Output)
		require.NotNil(t, command.Permissions)
		assert.Equal(t, []string{"read:tenant_settings", "read:clients", "read:connections"}, command.Permissions.Scopes)
		assert.Equal(t, FileAccessWrite, command.Permissions.Files[0].Access)
	})

	flagWithShorthand := func(name, shorthand string) map[string]any {
		return map[string]any{"name": name, "shorthand": shorthand, "type": "string", "description": "A flag"}
	}

	tests := []manifestCase{
		{name: "accepts a prerelease and build metadata version", mutate: setValue("1.0.0-rc.1+build.5", "version")},
		{name: "accepts a manifest without engines", mutate: removeValue("engines")},
		{name: "accepts a homepage", mutate: setValue("https://auth0.com/docs/checkmate?ref=cli#top", "homepage")},
		{name: "accepts a description of exactly 120 characters", mutate: setValue(strings.Repeat("a", 120), "description")},
		{
			name:   "accepts a command with only a name and a description",
			mutate: setValue([]any{map[string]any{"name": "hello", "description": "Say hello"}}, "commands"),
		},
		{name: "accepts a compound timeout", mutate: setCommand("1h30m", "timeout")},
		{name: "accepts the csv output mode", mutate: setCommand(map[string]any{"csv": true}, "output")},
		{name: "accepts empty permissions", mutate: setCommand(map[string]any{}, "permissions")},
		{
			name: "accepts every flag type",
			mutate: setCommand([]any{
				map[string]any{"name": "a", "type": "string", "description": "A flag"},
				map[string]any{"name": "b", "type": "bool", "description": "A flag"},
				map[string]any{"name": "c", "type": "int", "description": "A flag"},
				map[string]any{"name": "d", "type": "float", "description": "A flag"},
				map[string]any{"name": "e", "type": "stringArray", "description": "A flag"},
			}, "flags"),
		},
		{
			name:   "accepts flag shorthands in both cases",
			mutate: setCommand([]any{flagWithShorthand("format", "f"), flagWithShorthand("force", "F")}, "flags"),
		},
		{name: "accepts several engine comparators", mutate: setValue(map[string]any{"auth0-cli": ">=1.39.0 <2.0.0"}, "engines")},
		{
			name:   "accepts every engine comparator",
			mutate: setValue(map[string]any{"auth0-cli": ">1.0.0 >=1.0.0 <3.0.0 <=2.0.0 =1.2.3"}, "engines"),
		},
		{
			name: "accepts wildcard hosts and single label hosts",
			mutate: setCommand([]any{
				map[string]any{"host": "*.amazonaws.com", "reason": "Call AWS"},
				map[string]any{"host": "*.s3.amazonaws.com", "reason": "Call S3"},
				map[string]any{"host": "localhost", "reason": "Local server"},
			}, "permissions", "hosts"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseManifest(mutateManifest(t, test.mutate))
			assert.NoError(t, err)
		})
	}
}

func TestParseManifest_Invalid(t *testing.T) {
	flag := func(key string) []any { return []any{"commands", 0, "flags", 0, key} }
	host := func(key string) []any { return []any{"commands", 0, "permissions", "hosts", 0, key} }
	file := func(key string) []any { return []any{"commands", 0, "permissions", "files", 0, key} }
	env := func(key string) []any { return []any{"commands", 0, "permissions", "env", 0, key} }
	engines := func(constraint string) func(manifest map[string]any) {
		return setValue(map[string]any{"auth0-cli": constraint}, "engines")
	}

	tests := []manifestCase{
		// Top level.
		{name: "rejects an unknown top-level field", mutate: setValue(true, "extra"), wantErrs: []string{"extra: unknown field"}},
		{name: "rejects a missing id", mutate: removeValue("id"), wantErrs: []string{"id: required field is missing"}},
		{name: "rejects an id without a registry", mutate: setValue("checkmate", "id"), wantErrs: []string{"id:"}},
		{name: "rejects an id with uppercase letters", mutate: setValue("Auth0/CheckMate", "id"), wantErrs: []string{"id:"}},
		{name: "rejects an id with more than one slash", mutate: setValue("auth0/check/mate", "id"), wantErrs: []string{"id:"}},
		{name: "rejects a missing version", mutate: removeValue("version"), wantErrs: []string{"version: required field is missing"}},
		{name: "rejects a version with a leading v", mutate: setValue("v1.0.0", "version"), wantErrs: []string{"version:"}},
		{name: "rejects a version that is not full semver", mutate: setValue("1.0", "version"), wantErrs: []string{"version:"}},
		{name: "rejects a version with leading zeros", mutate: setValue("01.0.0", "version"), wantErrs: []string{"version:"}},
		{name: "rejects a missing apiVersion", mutate: removeValue("apiVersion"), wantErrs: []string{"apiVersion: required field is missing"}},
		{name: "rejects an unsupported apiVersion", mutate: setValue(2, "apiVersion"), wantErrs: []string{"apiVersion:"}},
		{name: "rejects an apiVersion that is a string", mutate: setValue("1", "apiVersion"), wantErrs: []string{"apiVersion:"}},
		{name: "rejects a $schema that is not a string", mutate: setValue(1, "$schema"), wantErrs: []string{"$schema:"}},
		{name: "rejects a homepage that is not https", mutate: setValue("http://example.com", "homepage"), wantErrs: []string{"homepage:"}},
		{name: "rejects a homepage that is not a URL", mutate: setValue("example", "homepage"), wantErrs: []string{"homepage:"}},

		// Description.
		{name: "rejects a missing description", mutate: removeValue("description"), wantErrs: []string{"description: required field is missing"}},
		{name: "rejects an empty description", mutate: setValue("", "description"), wantErrs: []string{"description:"}},
		{
			name:     "rejects a description longer than 120 characters",
			mutate:   setValue(strings.Repeat("a", 121), "description"),
			wantErrs: []string{"description:"},
		},
		{
			name:     "rejects a description with a newline",
			mutate:   setValue("First line\nSecond line", "description"),
			wantErrs: []string{"description:"},
		},

		// Owner.
		{name: "rejects a missing owner", mutate: removeValue("owner"), wantErrs: []string{"owner: required field is missing"}},
		{name: "rejects an empty owner team", mutate: setValue("", "owner", "team"), wantErrs: []string{"owner.team:"}},
		{name: "rejects a missing owner team", mutate: removeValue("owner", "team"), wantErrs: []string{"owner.team: required field is missing"}},
		{name: "rejects a missing owner bugs URL", mutate: removeValue("owner", "bugs"), wantErrs: []string{"owner.bugs: required field is missing"}},
		{
			name:     "rejects an owner bugs URL that is not https",
			mutate:   setValue("http://example.com/issues", "owner", "bugs"),
			wantErrs: []string{"owner.bugs:"},
		},
		{name: "rejects an owner bugs value that is not a URL", mutate: setValue("not a url", "owner", "bugs"), wantErrs: []string{"owner.bugs:"}},
		{name: "rejects an unknown owner field", mutate: setValue("a@example.com", "owner", "email"), wantErrs: []string{"owner.email: unknown field"}},

		// Engines.
		{name: "rejects empty engines", mutate: setValue(map[string]any{}, "engines"), wantErrs: []string{"engines.auth0-cli: required field is missing"}},
		{name: "rejects an empty engine constraint", mutate: engines(""), wantErrs: []string{"engines.auth0-cli:"}},
		{name: "rejects an engine constraint without a comparator", mutate: engines("1.39.0"), wantErrs: []string{"engines.auth0-cli:"}},
		{name: "rejects an engine constraint without a patch version", mutate: engines(">=1.39"), wantErrs: []string{"engines.auth0-cli:"}},
		{name: "rejects a caret engine constraint", mutate: engines("^1.39.0"), wantErrs: []string{"engines.auth0-cli:"}},
		{name: "rejects an engine constraint with an or operator", mutate: engines(">=1.39.0 || <1.0.0"), wantErrs: []string{"engines.auth0-cli:"}},
		{name: "rejects an engine constraint with a v prefix", mutate: engines(">=v1.39.0"), wantErrs: []string{"engines.auth0-cli:"}},
		{name: "rejects an engine constraint with a prerelease", mutate: engines(">=1.39.0-beta.1"), wantErrs: []string{"engines.auth0-cli:"}},
		{name: "rejects an engine constraint with a trailing space", mutate: engines(">=1.39.0 "), wantErrs: []string{"engines.auth0-cli:"}},
		{name: "rejects an unknown engine", mutate: setValue(">=20", "engines", "node"), wantErrs: []string{"engines.node: unknown field"}},

		// Platforms.
		{name: "rejects a missing platforms list", mutate: removeValue("platforms"), wantErrs: []string{"platforms: required field is missing"}},
		{name: "rejects an empty platforms list", mutate: setValue([]any{}, "platforms"), wantErrs: []string{"platforms:"}},
		{name: "rejects an unknown platform", mutate: setValue([]any{"linux-x64", "freebsd-x64"}, "platforms"), wantErrs: []string{"platforms[1]:"}},
		{name: "rejects duplicate platforms", mutate: setValue([]any{"linux-x64", "linux-x64"}, "platforms"), wantErrs: []string{"platforms:"}},

		// Commands.
		{name: "rejects a missing commands list", mutate: removeValue("commands"), wantErrs: []string{"commands: required field is missing"}},
		{name: "rejects an empty commands list", mutate: setValue([]any{}, "commands"), wantErrs: []string{"commands:"}},
		{name: "rejects a command name with uppercase letters", mutate: setCommand("CheckMate", "name"), wantErrs: []string{"commands[0].name:"}},
		{name: "rejects a command name that starts with a digit", mutate: setCommand("1check", "name"), wantErrs: []string{"commands[0].name:"}},
		{name: "rejects an empty command description", mutate: setCommand("", "description"), wantErrs: []string{"commands[0].description:"}},
		{
			name:     "rejects a missing command description",
			mutate:   removeCommand("description"),
			wantErrs: []string{"commands[0].description: required field is missing"},
		},
		{name: "rejects an interactive value that is not a boolean", mutate: setCommand("yes", "interactive"), wantErrs: []string{"commands[0].interactive:"}},
		{name: "rejects an unknown command field", mutate: setCommand(true, "hidden"), wantErrs: []string{"commands[0].hidden: unknown field"}},
		{
			name: "rejects two commands with the same name",
			mutate: setValue([]any{
				map[string]any{"name": "audit", "description": "First"},
				map[string]any{"name": "audit", "description": "Second"},
			}, "commands"),
			wantErrs: []string{`commands[1].name: duplicate command name "audit", already used by commands[0]`},
		},

		// Flags.
		{name: "rejects a flag with an unknown type", mutate: setValue("duration", flag("type")...), wantErrs: []string{"commands[0].flags[0].type:"}},
		{name: "rejects a flag name with uppercase letters", mutate: setValue("Validators", flag("name")...), wantErrs: []string{"commands[0].flags[0].name:"}},
		{name: "rejects a flag name with a leading dash", mutate: setValue("--validators", flag("name")...), wantErrs: []string{"commands[0].flags[0].name:"}},
		{name: "rejects an empty flag description", mutate: setValue("", flag("description")...), wantErrs: []string{"commands[0].flags[0].description:"}},
		{name: "rejects an unknown flag field", mutate: setValue("all", flag("default")...), wantErrs: []string{"commands[0].flags[0].default: unknown field"}},
		{
			name: "rejects two flags with the same name on one command",
			mutate: setCommand([]any{
				map[string]any{"name": "format", "type": "string", "description": "First"},
				map[string]any{"name": "format", "type": "string", "description": "Second"},
			}, "flags"),
			wantErrs: []string{`commands[0].flags[1].name: duplicate flag name "format", already used by commands[0].flags[0]`},
		},
		{name: "rejects a shorthand with more than one letter", mutate: setValue("ab", flag("shorthand")...), wantErrs: []string{"commands[0].flags[0].shorthand:"}},
		{name: "rejects a shorthand that is a digit", mutate: setValue("1", flag("shorthand")...), wantErrs: []string{"commands[0].flags[0].shorthand:"}},
		{name: "rejects an empty shorthand", mutate: setValue("", flag("shorthand")...), wantErrs: []string{"commands[0].flags[0].shorthand:"}},
		{name: "rejects a shorthand with a leading dash", mutate: setValue("-v", flag("shorthand")...), wantErrs: []string{"commands[0].flags[0].shorthand:"}},
		{
			name: "rejects two flags with the same shorthand on one command",
			mutate: setCommand([]any{
				map[string]any{"name": "format", "shorthand": "f", "type": "string", "description": "First"},
				map[string]any{"name": "filter", "shorthand": "f", "type": "string", "description": "Second"},
			}, "flags"),
			wantErrs: []string{`commands[0].flags[1].shorthand: duplicate shorthand "f", already used by commands[0].flags[0]`},
		},
		{
			name:     "rejects a shorthand that collides with a CLI-owned shorthand",
			mutate:   setValue("h", flag("shorthand")...),
			wantErrs: []string{`commands[0].flags[0].shorthand: shorthand "h" collides with a shorthand owned by the CLI`},
		},

		// Timeout and output.
		{name: "rejects a timeout that is not a duration", mutate: setCommand("soon", "timeout"), wantErrs: []string{"commands[0].timeout:"}},
		{name: "rejects a zero timeout", mutate: setCommand("0s", "timeout"), wantErrs: []string{"commands[0].timeout:", "greater than zero"}},
		{name: "rejects a bare zero timeout", mutate: setCommand("0", "timeout"), wantErrs: []string{"commands[0].timeout:"}},
		{name: "rejects a negative timeout", mutate: setCommand("-5s", "timeout"), wantErrs: []string{"commands[0].timeout:"}},
		{name: "rejects a timeout that overflows a duration", mutate: setCommand("99999999999h", "timeout"), wantErrs: []string{"commands[0].timeout:"}},
		{name: "rejects a timeout that is a number", mutate: setCommand(30, "timeout"), wantErrs: []string{"commands[0].timeout:"}},
		{
			name:     "rejects an output flag that is not a boolean",
			mutate:   setCommand(map[string]any{"json": "yes"}, "output"),
			wantErrs: []string{"commands[0].output.json:"},
		},
		{
			name:     "rejects a csv output flag that is not a boolean",
			mutate:   setCommand(map[string]any{"csv": "yes"}, "output"),
			wantErrs: []string{"commands[0].output.csv:"},
		},
		{
			name:     "rejects an unknown output field",
			mutate:   setCommand(map[string]any{"xml": true}, "output"),
			wantErrs: []string{"commands[0].output.xml: unknown field"},
		},

		// Permissions.
		{
			name:     "rejects an unknown permissions field",
			mutate:   setCommand(true, "permissions", "network"),
			wantErrs: []string{"commands[0].permissions.network: unknown field"},
		},
		{
			name:     "rejects a scope without a colon",
			mutate:   setCommand([]any{"read:clients", "clients"}, "permissions", "scopes"),
			wantErrs: []string{"commands[0].permissions.scopes[1]:"},
		},
		{
			name:     "rejects a scope with uppercase letters",
			mutate:   setCommand([]any{"Read:clients"}, "permissions", "scopes"),
			wantErrs: []string{"commands[0].permissions.scopes[0]:"},
		},
		{
			name:     "rejects duplicate scopes",
			mutate:   setCommand([]any{"read:clients", "read:clients"}, "permissions", "scopes"),
			wantErrs: []string{"commands[0].permissions.scopes:"},
		},
		{
			name: "rejects a host with an empty reason",
			mutate: setCommand([]any{
				map[string]any{"host": "api.github.com", "reason": "Check advisories"},
				map[string]any{"host": "example.com", "reason": ""},
			}, "permissions", "hosts"),
			wantErrs: []string{"commands[0].permissions.hosts[1].reason:"},
		},
		{
			name:     "rejects a host without a reason",
			mutate:   setCommand([]any{map[string]any{"host": "example.com"}}, "permissions", "hosts"),
			wantErrs: []string{"commands[0].permissions.hosts[0].reason: required field is missing"},
		},
		{name: "rejects an unknown host field", mutate: setValue(true, host("extra")...), wantErrs: []string{"commands[0].permissions.hosts[0].extra: unknown field"}},
		{name: "rejects a file access that is not read or write", mutate: setValue("execute", file("access")...), wantErrs: []string{"commands[0].permissions.files[0].access:"}},
		{name: "rejects an empty file path", mutate: setValue("", file("path")...), wantErrs: []string{"commands[0].permissions.files[0].path:"}},
		{name: "rejects an empty file reason", mutate: setValue("", file("reason")...), wantErrs: []string{"commands[0].permissions.files[0].reason:"}},
		{
			name:     "rejects an unknown nested field in a file permission",
			mutate:   setValue(true, file("recursive")...),
			wantErrs: []string{"commands[0].permissions.files[0].recursive: unknown field"},
		},
		{name: "rejects a lowercase env name", mutate: setValue("aws_profile", env("name")...), wantErrs: []string{"commands[0].permissions.env[0].name:"}},
		{name: "rejects an env name that starts with a digit", mutate: setValue("1PROFILE", env("name")...), wantErrs: []string{"commands[0].permissions.env[0].name:"}},
		{
			name:     "rejects an env permission without a reason",
			mutate:   removeValue(env("reason")...),
			wantErrs: []string{"commands[0].permissions.env[0].reason: required field is missing"},
		},
		{
			name:     "rejects an env name with the reserved plugin prefix",
			mutate:   setValue("AUTH0_PLUGIN_X", env("name")...),
			wantErrs: []string{"commands[0].permissions.env[0].name:", "AUTH0_PLUGIN_X", "reserved"},
		},
		{
			name:     "rejects an env name with the reserved CLI prefix",
			mutate:   setValue("AUTH0_CLI_ANALYTICS", env("name")...),
			wantErrs: []string{"commands[0].permissions.env[0].name:", "AUTH0_CLI_ANALYTICS", "reserved"},
		},

		// Several problems at once.
		{
			name: "reports every schema problem in one pass",
			mutate: combine(
				setValue("v1", "version"),
				setValue(true, "extra"),
				setCommand("Bad", "name"),
				setValue("", host("reason")...),
			),
			wantErrs: []string{"version:", "extra: unknown field", "commands[0].name:", "commands[0].permissions.hosts[0].reason:"},
		},
	}

	hostTests := map[string]string{
		"a scheme":                 "https://api.github.com",
		"a path":                   "api.github.com/v3",
		"a port":                   "api.github.com:443",
		"uppercase letters":        "API.github.com",
		"a bare wildcard":          "*",
		"a wildcard over a TLD":    "*.com",
		"a wildcard in the middle": "foo.*.com",
		"a double wildcard":        "**.x.com",
		"a wildcard without a dot": "*github.com",
		"a wildcard with a port":   "*.github.com:443",
	}
	for name, value := range hostTests {
		tests = append(tests, manifestCase{
			name:     "rejects a host with " + name,
			mutate:   setValue(value, host("host")...),
			wantErrs: []string{"commands[0].permissions.hosts[0].host:"},
		})
	}

	textFields := []struct {
		name string
		path []any
		want string
	}{
		{name: "a host reason", path: host("reason"), want: "commands[0].permissions.hosts[0].reason:"},
		{name: "a file reason", path: file("reason"), want: "commands[0].permissions.files[0].reason:"},
		{name: "a file path", path: file("path"), want: "commands[0].permissions.files[0].path:"},
		{name: "an env reason", path: env("reason"), want: "commands[0].permissions.env[0].reason:"},
	}
	unsafeCharacters := []struct{ name, value string }{
		{name: "a tab", value: "a\tb"},
		{name: "a newline", value: "a\nb"},
		{name: "a carriage return", value: "a\rb"},
		{name: "a null character", value: "a\x00b"},
		{name: "a delete character", value: "a\u007fb"},
		{name: "a line separator", value: "a\u2028b"},
		{name: "a paragraph separator", value: "a\u2029b"},
	}
	for _, field := range textFields {
		for _, character := range unsafeCharacters {
			tests = append(tests, manifestCase{
				name:     "rejects " + field.name + " with " + character.name,
				mutate:   setValue(character.value, field.path...),
				wantErrs: []string{field.want},
			})
		}
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest, err := ParseManifest(mutateManifest(t, test.mutate))
			assert.Nil(t, manifest)
			requireValidationError(t, err, test.wantErrs...)
		})
	}
}

func TestParseManifest_ReservedFlagNames(t *testing.T) {
	for _, name := range reservedFlagNames {
		t.Run("rejects the CLI-owned flag "+name, func(t *testing.T) {
			_, err := ParseManifest(mutateManifest(t, setCommand(name, "flags", 0, "name")))

			requireValidationError(t, err, "commands[0].flags[0].name:", "collides with a flag owned by the CLI")
		})
	}
}

func TestParseManifest_MalformedJSON(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "rejects an empty document", data: ""},
		{name: "rejects truncated JSON", data: `{"id": "auth0/checkmate"`},
		{name: "rejects trailing data after the manifest", data: `{} {}`},
		{name: "rejects a JSON document that is not an object", data: `[]`},
	}

	parsers := map[string]func([]byte) (*Manifest, error){
		"ParseManifest":          ParseManifest,
		"ParseInstalledManifest": ParseInstalledManifest,
	}

	for parserName, parse := range parsers {
		for _, test := range tests {
			t.Run(parserName+" "+test.name, func(t *testing.T) {
				manifest, err := parse([]byte(test.data))
				assert.Nil(t, manifest)
				requireValidationError(t, err)
			})
		}
	}
}

func TestParseManifest_DecodeFailure(t *testing.T) {
	t.Run("rejects an apiVersion that is an integer in JSON Schema but not in Go", func(t *testing.T) {
		data := strings.Replace(string(readValidManifest(t)), `"apiVersion": 1,`, `"apiVersion": 1.0,`, 1)

		_, err := ParseManifest([]byte(data))
		requireValidationError(t, err, "manifest could not be decoded")
	})
}

func TestValidationError(t *testing.T) {
	t.Run("joins errors one per line", func(t *testing.T) {
		err := &ValidationError{Errors: []string{"a: bad", "b: worse"}}
		assert.Equal(t, "a: bad\nb: worse", err.Error())
	})
}

func TestItemPath(t *testing.T) {
	assert.Equal(t, "commands[2]", itemPath("", "commands", 2))
	assert.Equal(t, "commands[0].flags[1]", itemPath("commands[0]", "flags", 1))
}

func TestNameOf(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want string
	}{
		{name: "returns the part after the slash", id: "auth0/checkmate", want: "checkmate"},
		{name: "returns a hyphenated name", id: "acme/log-viewer", want: "log-viewer"},
		{name: "returns the whole id when there is no slash", id: "checkmate", want: "checkmate"},
		{name: "returns the part after the first slash only", id: "acme/a/b", want: "a/b"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, NameOf(test.id))
			assert.Equal(t, test.want, (&Manifest{ID: test.id}).Name())
		})
	}
}

func TestManifest_RootCommand(t *testing.T) {
	tests := []struct {
		name     string
		commands []string
		wantRoot string
	}{
		{name: "returns the command named like the plugin", commands: []string{"report", "checkmate"}, wantRoot: "checkmate"},
		{name: "returns nil when no command is named like the plugin", commands: []string{"report", "run"}},
		{name: "does not match on the registry part of the id", commands: []string{"auth0"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := &Manifest{ID: "auth0/checkmate"}
			for _, name := range test.commands {
				manifest.Commands = append(manifest.Commands, Command{Name: name})
			}

			root := manifest.RootCommand()
			if test.wantRoot == "" {
				assert.Nil(t, root)
				return
			}

			require.NotNil(t, root)
			assert.Equal(t, test.wantRoot, root.Name)
		})
	}
}
