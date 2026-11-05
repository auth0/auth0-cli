// Package plugins defines the plugin.json manifest contract that Auth0 CLI
// plugins ship with, and the validation shared by the CLI, the plugin catalog
// and the plugin developer tooling.
package plugins

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// schemaURL identifies the embedded schema. The schema is only ever loaded
// from ManifestSchema and is never fetched from this address.
const schemaURL = "https://raw.githubusercontent.com/auth0/auth0-cli-plugins/main/schema/plugin.v1.schema.json"

// manifestSchema holds the embedded JSON Schema.
//
//go:embed schema/plugin.schema.json
var manifestSchema []byte

// ManifestSchema returns the JSON Schema (draft 2020-12) that describes
// plugin.json. It is embedded so that the CLI, `plugin dev validate` and the
// plugin catalog can all publish and enforce the exact same file. The caller
// gets its own copy of the bytes, so changing them cannot affect validation.
func ManifestSchema() []byte {
	return slices.Clone(manifestSchema)
}

// Platform is a platform that a plugin ships a binary for.
type Platform string

// The platforms a plugin can ship binaries for.
const (
	PlatformDarwinArm64    Platform = "darwin-arm64"
	PlatformDarwinX64      Platform = "darwin-x64"
	PlatformLinuxX64       Platform = "linux-x64"
	PlatformLinuxArm64     Platform = "linux-arm64"
	PlatformLinuxX64Musl   Platform = "linux-x64-musl"
	PlatformLinuxArm64Musl Platform = "linux-arm64-musl"
	PlatformWindowsX64     Platform = "windows-x64"
)

// The C libraries that Libc can report for a Linux platform.
const (
	libcGlibc = "glibc"
	libcMusl  = "musl"
)

// platformInfo describes the parts of a platform.
type platformInfo struct {
	platform Platform
	os       string
	arch     string
	libc     string
}

// knownPlatforms lists every platform in a stable order.
var knownPlatforms = []platformInfo{
	{PlatformDarwinArm64, "darwin", "arm64", ""},
	{PlatformDarwinX64, "darwin", "x64", ""},
	{PlatformLinuxX64, "linux", "x64", libcGlibc},
	{PlatformLinuxArm64, "linux", "arm64", libcGlibc},
	{PlatformLinuxX64Musl, "linux", "x64", libcMusl},
	{PlatformLinuxArm64Musl, "linux", "arm64", libcMusl},
	{PlatformWindowsX64, "windows", "x64", ""},
}

// AllPlatforms returns every platform a plugin can ship a binary for.
func AllPlatforms() []Platform {
	platforms := make([]Platform, 0, len(knownPlatforms))
	for _, info := range knownPlatforms {
		platforms = append(platforms, info.platform)
	}

	return platforms
}

// PlatformFor returns the platform with the given operating system, CPU
// architecture and C library, written the way the manifest names them. The C
// library is empty for every platform that is not a Linux one. It reports false
// when there is no such platform.
func PlatformFor(os, arch, libc string) (Platform, bool) {
	for _, info := range knownPlatforms {
		if info.os == os && info.arch == arch && info.libc == libc {
			return info.platform, true
		}
	}

	return "", false
}

// info returns the parts of the platform, or the zero value for an unknown one.
func (p Platform) info() platformInfo {
	for _, info := range knownPlatforms {
		if info.platform == p {
			return info
		}
	}

	return platformInfo{}
}

// OS returns the operating system of the platform, such as "darwin", "linux" or
// "windows". It is empty for an unknown platform.
func (p Platform) OS() string {
	return p.info().os
}

// Arch returns the CPU architecture of the platform as the manifest names it,
// which is "arm64" or "x64". It is empty for an unknown platform.
func (p Platform) Arch() string {
	return p.info().arch
}

// Libc returns the C library of a Linux platform, either "glibc" or "musl". It
// is empty for every other platform.
func (p Platform) Libc() string {
	return p.info().libc
}

// FlagType is the type of value a plugin flag accepts.
type FlagType string

// The flag types a plugin can declare.
const (
	FlagTypeString      FlagType = "string"
	FlagTypeBool        FlagType = "bool"
	FlagTypeInt         FlagType = "int"
	FlagTypeFloat       FlagType = "float"
	FlagTypeStringArray FlagType = "stringArray"
)

// FileAccess is the kind of access a plugin asks for on a file system location.
type FileAccess string

// The file accesses a plugin can ask for.
const (
	FileAccessRead  FileAccess = "read"
	FileAccessWrite FileAccess = "write"
)

// reservedFlagNames lists the flags owned by the CLI itself. A plugin command
// cannot declare a flag with one of these names.
//
// The list is static because the manifest validator cannot see the commands of
// the CLI. A test in internal/cli fails when a persistent flag of the root
// command, an output flag or help is missing from it.
var reservedFlagNames = []string{
	"tenant",
	"json",
	"json-compact",
	"csv",
	"agent-mode",
	"no-input",
	"no-color",
	"debug",
	"help",
}

// reservedFlagShorthands lists the single letter flags owned by the CLI
// itself. A plugin command cannot declare a flag with one of these shorthands.
// The CLI's persistent flags have no shorthand, so the only reserved one comes
// from cobra's built-in help flag. A test in internal/cli fails when a flag of
// the CLI gets another shorthand.
var reservedFlagShorthands = []string{
	"h",
}

// reservedEnvPrefixes lists the environment variable prefixes that the CLI
// owns, so a plugin cannot ask for variables that start with them. AUTH0_ covers
// the credentials of the CLI (AUTH0_DOMAIN, AUTH0_CLIENT_ID, AUTH0_CLIENT_SECRET,
// AUTH0_API_TOKEN), the settings it reads (AUTH0_CLI_) and the variables it sets
// for plugins (AUTH0_PLUGIN_). The comparison ignores case.
var reservedEnvPrefixes = []string{
	"AUTH0_",
}

// Manifest is the parsed content of a plugin.json file.
//
// A plugin adds commands to the CLI under its own name. The command whose name
// equals the plugin name (see RootCommand) is the root command and runs as
// `auth0 <plugin>`. Every other command is a subcommand and runs as
// `auth0 <plugin> <name>`.
type Manifest struct {
	Schema      string     `json:"$schema,omitempty"`
	ID          string     `json:"id"`
	Version     string     `json:"version"`
	APIVersion  int        `json:"apiVersion"`
	Description string     `json:"description"`
	Owner       Owner      `json:"owner"`
	Homepage    string     `json:"homepage,omitempty"`
	Engines     *Engines   `json:"engines,omitempty"`
	Platforms   []Platform `json:"platforms"`
	Commands    []Command  `json:"commands"`
}

// Owner identifies who maintains a plugin and where to report problems.
type Owner struct {
	Team string `json:"team"`
	Bugs string `json:"bugs"`
}

// Engines holds version constraints on the environment a plugin runs in.
//
// Auth0CLI is one or more space separated comparators that must all match, for
// example ">=1.39.0 <2.0.0". The grammar is checked here but evaluating the
// constraint is left to the code that installs plugins.
type Engines struct {
	Auth0CLI string `json:"auth0-cli"`
}

// Command is a single command that a plugin adds to the CLI.
type Command struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Flags       []Flag       `json:"flags,omitempty"`
	Interactive bool         `json:"interactive"`
	Timeout     string       `json:"timeout,omitempty"`
	Output      *Output      `json:"output,omitempty"`
	Permissions *Permissions `json:"permissions,omitempty"`
}

// Flag describes a flag that a plugin command accepts.
type Flag struct {
	Name        string   `json:"name"`
	Shorthand   string   `json:"shorthand,omitempty"`
	Type        FlagType `json:"type"`
	Description string   `json:"description"`
}

// Output declares the output modes a plugin command supports.
type Output struct {
	JSON      bool `json:"json"`
	CSV       bool `json:"csv"`
	AgentMode bool `json:"agentMode"`
}

// compileSchema compiles the embedded schema on first use and reuses the result.
var compileSchema = sync.OnceValues(func() (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(manifestSchema))
	if err != nil {
		return nil, fmt.Errorf("failed to read the plugin manifest schema: %w", err)
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(schemaURL, doc); err != nil {
		return nil, fmt.Errorf("failed to load the plugin manifest schema: %w", err)
	}

	schema, err := compiler.Compile(schemaURL)
	if err != nil {
		return nil, fmt.Errorf("failed to compile the plugin manifest schema: %w", err)
	}

	return schema, nil
})

// decodeDocument decodes data into generic values, with numbers kept as
// json.Number, or reports that data is not valid JSON or has a duplicate key.
func decodeDocument(data []byte) (any, error) {
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, &ValidationError{Errors: []string{fmt.Sprintf("manifest is not valid JSON: %v", err)}}
	}

	if err := RejectDuplicateKeys(data); err != nil {
		return nil, &ValidationError{Errors: []string{err.Error()}}
	}

	return document, nil
}

// ParseManifest validates data as a plugin.json manifest and returns it.
//
// It is strict: any field that this version of the package does not know, and
// any apiVersion other than 1, is rejected. Use it where a manifest is being
// authored or published, such as `plugin dev validate` and the plugin catalog.
// Use ParseInstalledManifest to read a manifest that is already installed.
//
// Validation runs in three steps: a JSON syntax check, validation against
// ManifestSchema, and the checks that JSON Schema cannot express (positive
// timeouts, unique names, and names reserved by the CLI). Problems are returned
// as a *ValidationError that lists all of the problems found in the schema step
// and, when the schema step passes, all of the Go-only problems.
func ParseManifest(data []byte) (*Manifest, error) {
	instance, err := decodeDocument(data)
	if err != nil {
		return nil, err
	}

	schema, err := compileSchema()
	if err != nil {
		return nil, err
	}

	if err := schema.Validate(instance); err != nil {
		if problems := SchemaProblems(err); problems != nil {
			return nil, &ValidationError{Errors: problems}
		}

		return nil, err
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, &ValidationError{Errors: []string{fmt.Sprintf("manifest could not be decoded: %v", err)}}
	}

	if problems := manifest.semanticErrors(); len(problems) > 0 {
		return nil, &ValidationError{Errors: problems}
	}

	return &manifest, nil
}

// RootCommand returns the command whose name equals the plugin name, or nil if
// the manifest has no such command. The root command runs as `auth0 <plugin>`,
// and every other command is a subcommand that runs as `auth0 <plugin> <name>`.
func (m *Manifest) RootCommand() *Command {
	name := m.Name()

	for i := range m.Commands {
		if m.Commands[i].Name == name {
			return &m.Commands[i]
		}
	}

	return nil
}

// NameOf returns the plugin name of a plugin ID, which is the part after the
// registry. An ID without a slash is returned unchanged.
func NameOf(id string) string {
	_, name, found := strings.Cut(id, "/")
	if !found {
		return id
	}

	return name
}

// Name returns the plugin name, which is the part of the ID after the registry.
func (m *Manifest) Name() string {
	return NameOf(m.ID)
}

// semanticErrors runs the checks that JSON Schema cannot express.
func (m *Manifest) semanticErrors() []string {
	var found ProblemList

	commandNames := make(map[string]string, len(m.Commands))

	for i, command := range m.Commands {
		commandPath := itemPath("", "commands", i)

		found.CheckUnique(commandNames, command.Name, commandPath, "name", "command name")

		if command.Timeout != "" {
			if err := CheckTimeout(command.Timeout); err != nil {
				found.Addf("%s.timeout: %v", commandPath, err)
			}
		}

		flagNames := make(map[string]string, len(command.Flags))
		flagShorthands := make(map[string]string, len(command.Flags))

		for j, flag := range command.Flags {
			flagPath := itemPath(commandPath, "flags", j)

			if slices.Contains(reservedFlagNames, flag.Name) {
				found.Addf("%s.name: flag %q collides with a flag owned by the CLI", flagPath, flag.Name)
			}
			found.CheckUnique(flagNames, flag.Name, flagPath, "name", "flag name")

			if flag.Shorthand == "" {
				continue
			}

			if slices.Contains(reservedFlagShorthands, flag.Shorthand) {
				found.Addf("%s.shorthand: shorthand %q collides with a shorthand owned by the CLI", flagPath, flag.Shorthand)
			}
			found.CheckUnique(flagShorthands, flag.Shorthand, flagPath, "shorthand", "shorthand")
		}

		if command.Permissions == nil {
			continue
		}

		for j, env := range command.Permissions.Env {
			reserved := slices.ContainsFunc(reservedEnvPrefixes, func(prefix string) bool {
				return strings.HasPrefix(strings.ToUpper(env.Name), prefix)
			})
			if reserved {
				found.Addf("%s.name: %q uses a prefix reserved by the CLI (%s)",
					itemPath(commandPath+".permissions", "env", j), env.Name, strings.Join(reservedEnvPrefixes, ", "))
			}
		}
	}

	return found
}
