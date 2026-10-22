// Package registry fetches, verifies and caches the signed index of an Auth0
// CLI plugin registry.
//
// The index lists the plugins of a registry, the versions of each plugin, the
// permissions each version asks for, and where to download it. The CLI only
// ever uses an index whose signature was verified against a key that is
// embedded in the CLI, and it fails closed in every other case.
package registry

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/auth0/auth0-cli/internal/plugins"
)

// supportedFormatVersion is the newest index format this CLI understands.
const supportedFormatVersion = 1

// ArtifactFormat is how a plugin artifact is packaged.
type ArtifactFormat string

// The packaging formats of a plugin artifact.
const (
	ArtifactFormatRaw   ArtifactFormat = "raw"
	ArtifactFormatTarGz ArtifactFormat = "tar.gz"
	ArtifactFormatZip   ArtifactFormat = "zip"
)

// Index is the content of a registry's index.json. Fields that this version of
// the CLI does not know are ignored.
type Index struct {
	FormatVersion int           `json:"formatVersion"`
	Version       int           `json:"version"`
	Expires       time.Time     `json:"expires"`
	Plugins       []Plugin      `json:"plugins,omitempty"`
	Revocations   []Revocation  `json:"revoked,omitempty"`
	Deprecations  []Deprecation `json:"deprecated,omitempty"`
	Policy        Policy        `json:"policy,omitzero"`

	// ScopeTable and SecretFields are the data the plugin proxy enforces. They
	// are zero when the registry publishes none, and the proxy refuses to run a
	// plugin then.
	ScopeTable   ScopeTable   `json:"scopeTable,omitzero"`
	SecretFields SecretFields `json:"secretFields,omitzero"`
}

// ScopeTable lists the Management API calls that the plugin proxy knows a scope
// for. A call that no rule matches is denied.
type ScopeTable struct {
	FormatVersion int         `json:"formatVersion"`
	Rules         []ScopeRule `json:"rules"`
}

// ScopeRule says which scopes allow one Management API call.
type ScopeRule struct {
	// Method is the HTTP method, one of GET, POST, PUT, PATCH and DELETE.
	Method string `json:"method"`

	// Path is the path template, such as /api/v2/clients/{id}. A segment in
	// braces matches any one segment.
	Path string `json:"path"`

	// AnyOf lists the scopes that allow the call. A plugin that declared at least
	// one of them may make it.
	AnyOf []string `json:"anyOf"`
}

// SecretFields lists the names of the fields that the plugin proxy removes from
// every response before the plugin sees it.
type SecretFields struct {
	FormatVersion int      `json:"formatVersion"`
	Names         []string `json:"names"`
}

// Plugin is a plugin listed in the index. The versions of a plugin are either
// listed in the index itself, or in a separate file that Ref points to. Use
// Client.Versions to get them in both cases.
type Plugin struct {
	ID          string          `json:"id"`
	Description string          `json:"description"`
	Owner       plugins.Owner   `json:"owner"`
	Homepage    string          `json:"homepage,omitempty"`
	Latest      string          `json:"latest"`
	Versions    []VersionRecord `json:"versions,omitempty"`
	Ref         *Ref            `json:"ref,omitempty"`
}

// Ref points to a file that holds the versions of a plugin. The file is pinned
// by its hash, so it does not have to be trusted.
type Ref struct {
	// Path is relative to the URL of the index.
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// VersionRecord describes one version of a plugin.
//
// The permissions must be normalized the way plugins.Manifest.Permissions
// returns them, because PermissionsHash is checked against them as they are.
type VersionRecord struct {
	Version         string                   `json:"version"`
	Engines         *plugins.Engines         `json:"engines,omitempty"`
	Platforms       []plugins.Platform       `json:"platforms"`
	PermissionsHash string                   `json:"permissionsHash"`
	Permissions     plugins.Permissions      `json:"permissions"`
	Commands        []plugins.ConsentCommand `json:"commands"`
	Artifacts       []Artifact               `json:"artifacts"`
}

// Artifact is a downloadable build of a plugin version for one platform.
type Artifact struct {
	OS     string         `json:"os"`
	Arch   string         `json:"arch"`
	Libc   string         `json:"libc,omitempty"`
	URL    string         `json:"url"`
	SHA256 string         `json:"sha256"`
	Size   int64          `json:"size"`
	Format ArtifactFormat `json:"format"`

	// Bin is the file name of the executable inside the artifact. For the raw
	// format it is the name the downloaded file is saved as.
	Bin string `json:"bin"`
}

// Revocation marks a plugin version that must no longer run.
type Revocation struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Reason  string `json:"reason"`
}

// Deprecation marks a plugin, or one version of it, as one that should no
// longer be used.
type Deprecation struct {
	ID string `json:"id"`

	// Version is empty when the whole plugin is deprecated.
	Version     string `json:"version,omitempty"`
	Message     string `json:"message"`
	Replacement string `json:"replacement,omitempty"`
}

// The defaults for the settings that a registry leaves out of its policy.
const (
	defaultOfflineGraceDays   = 30
	defaultCheckIntervalHours = 24
	defaultMaxDownloadBytes   = 256 << 20
	defaultCommandTimeout     = "10m"
)

// Policy holds the settings that a registry hands to the CLI. A setting that is
// left out is zero, and WithDefaults replaces it with the default.
type Policy struct {
	// OfflineGraceDays is how many days after the last refresh installed plugins
	// keep running without a reachable registry.
	OfflineGraceDays int `json:"offlineGraceDays,omitempty"`

	// CheckIntervalHours is how many hours after the last refresh the index is
	// considered stale.
	CheckIntervalHours int `json:"checkIntervalHours,omitempty"`

	// MaxDownloadBytes is the largest artifact that the CLI downloads.
	MaxDownloadBytes int64 `json:"maxDownloadBytes,omitempty"`

	// DefaultTimeout is the Go duration that applies to commands that declare
	// no timeout.
	DefaultTimeout string `json:"defaultTimeout,omitempty"`

	// VersionRetention is how many old versions of a plugin to keep on disk,
	// where zero keeps all of them.
	VersionRetention int `json:"versionRetention,omitempty"`
}

// WithDefaults returns the policy with every setting that was left out set to
// its default: 30 days of offline grace, a 24 hour check interval, 256 MiB for
// downloads and a 10 minute command timeout. A version retention of zero is
// kept, as it means keeping all versions.
func (p Policy) WithDefaults() Policy {
	if p.OfflineGraceDays == 0 {
		p.OfflineGraceDays = defaultOfflineGraceDays
	}

	if p.CheckIntervalHours == 0 {
		p.CheckIntervalHours = defaultCheckIntervalHours
	}

	if p.MaxDownloadBytes == 0 {
		p.MaxDownloadBytes = defaultMaxDownloadBytes
	}

	if p.DefaultTimeout == "" {
		p.DefaultTimeout = defaultCommandTimeout
	}

	return p
}

// FindPlugin returns the plugin with the given ID.
func (i *Index) FindPlugin(id string) (Plugin, bool) {
	for _, plugin := range i.Plugins {
		if plugin.ID == id {
			return plugin, true
		}
	}

	return Plugin{}, false
}

// Revoked returns the revocation of the given plugin version, if there is one.
func (i *Index) Revoked(id, version string) (Revocation, bool) {
	for _, revocation := range i.Revocations {
		if revocation.ID == id && revocation.Version == version {
			return revocation, true
		}
	}

	return Revocation{}, false
}

// Deprecation returns the deprecation that applies to the given plugin
// version, if there is one. A deprecation of that exact version wins over a
// deprecation of the whole plugin.
func (i *Index) Deprecation(id, version string) (Deprecation, bool) {
	var whole *Deprecation

	for n, deprecation := range i.Deprecations {
		switch {
		case deprecation.ID != id:
		case deprecation.Version == version:
			return deprecation, true
		case deprecation.Version == "":
			whole = &i.Deprecations[n]
		}
	}

	if whole == nil {
		return Deprecation{}, false
	}

	return *whole, true
}

// Version returns the record of the given version from the versions listed in
// the index. It finds nothing for a plugin whose versions are in a file that Ref
// points to, so use Client.Versions for those.
func (p Plugin) Version(version string) (VersionRecord, bool) {
	for _, record := range p.Versions {
		if record.Version == version {
			return record, true
		}
	}

	return VersionRecord{}, false
}

// Platform returns the platform that the artifact is built for. It reports
// false when the OS, architecture and C library are not a supported platform.
func (a Artifact) Platform() (plugins.Platform, bool) {
	return plugins.PlatformFor(a.OS, a.Arch, a.Libc)
}

// ParseIndex validates data as an index.json and returns it. It does not check
// the signature or the expiry, which Client does.
//
// Validation runs in these steps: the format version check, validation against
// IndexSchema, and the checks that JSON Schema cannot express. An index with a
// format version newer than this CLI understands is rejected with
// ErrUnsupportedFormat. Any other problem is returned as an error that wraps
// ErrInvalidIndex and a *plugins.ValidationError.
func ParseIndex(data []byte) (*Index, error) {
	var header struct {
		FormatVersion int `json:"formatVersion"`
	}

	if json.Unmarshal(data, &header) == nil && header.FormatVersion > supportedFormatVersion {
		return nil, fmt.Errorf("%w (the index has format version %d and this CLI understands version %d)",
			ErrUnsupportedFormat, header.FormatVersion, supportedFormatVersion)
	}

	schemas, err := compileSchemas()
	if err != nil {
		return nil, err
	}

	index, err := decode[Index](schemas.index, data, "index.json")
	if err != nil {
		return nil, err
	}

	if problems := index.semanticErrors(); len(problems) > 0 {
		return nil, newInvalidError("index.json", problems)
	}

	return &index, nil
}

// parseVersionsFile validates data as the file that a Ref points to and returns
// its versions. The latest version of the plugin must be among them.
func parseVersionsFile(data []byte, latest, what string) ([]VersionRecord, error) {
	schemas, err := compileSchemas()
	if err != nil {
		return nil, err
	}

	type versionsFile struct {
		Versions []VersionRecord `json:"versions"`
	}

	file, err := decode[versionsFile](schemas.versionsFile, data, what)
	if err != nil {
		return nil, err
	}

	problems := checkVersions("versions", file.Versions)
	if !slices.ContainsFunc(file.Versions, func(record VersionRecord) bool { return record.Version == latest }) {
		problems.Addf("versions: the latest version %q of the plugin is not listed", latest)
	}

	if len(problems) > 0 {
		return nil, newInvalidError(what, problems)
	}

	return file.Versions, nil
}

// semanticErrors runs the checks that JSON Schema cannot express.
func (i *Index) semanticErrors() plugins.ProblemList {
	var found plugins.ProblemList

	pluginPaths := make(map[string]string, len(i.Plugins))

	for n, plugin := range i.Plugins {
		path := fmt.Sprintf("plugins[%d]", n)

		found.CheckUnique(pluginPaths, plugin.ID, path, "id", "plugin id")

		// The versions of a plugin with a ref are checked once they are fetched.
		if plugin.Ref != nil {
			continue
		}

		found = append(found, checkVersions(path+".versions", plugin.Versions)...)

		if _, ok := plugin.Version(plugin.Latest); !ok {
			found.Addf("%s.latest: version %q is not listed in versions", path, plugin.Latest)
		}
	}

	// The sections are optional in the index, and the zero value stands for a
	// section that is not there. A section that is there has format version 1.
	if i.ScopeTable.FormatVersion != 0 {
		found = append(found, i.ScopeTable.problems()...)
	}

	if i.SecretFields.FormatVersion != 0 {
		found = append(found, i.SecretFields.problems()...)
	}

	if timeout := i.Policy.DefaultTimeout; timeout != "" {
		if err := plugins.CheckTimeout(timeout); err != nil {
			found.Addf("policy.defaultTimeout: %v", err)
		}
	}

	return found
}

// Validate checks the scope table with the schema of the index and with the
// checks that the schema cannot express, which are the checks that ParseIndex
// makes. It returns an error that wraps ErrInvalidIndex and names the JSON path
// of every problem, as seen from the index. Code that gets a table from
// anywhere but ParseIndex must call it before it relies on the table.
func (t ScopeTable) Validate() error {
	if problems := t.problems(); len(problems) > 0 {
		return newInvalidError("scopeTable", problems)
	}

	return nil
}

// problems returns the problems found by the schema or, when the schema has
// none, by the checks that it cannot express.
func (t ScopeTable) problems() plugins.ProblemList {
	found, err := schemaProblems("scopeTable", func(s schemas) *jsonschema.Schema { return s.scopeTable }, t)
	if err != nil {
		return plugins.ProblemList{err.Error()}
	}

	if len(found) > 0 {
		return found
	}

	return t.extraProblems()
}

// extraProblems runs the checks on the scope table that JSON Schema cannot
// express: no dot segments, and no two rules for the same call.
func (t ScopeTable) extraProblems() plugins.ProblemList {
	var found plugins.ProblemList

	// Rules that only differ in the names of their parameters match the same
	// calls, so they count as the same rule.
	shapes := make(map[string]string, len(t.Rules))

	for n, rule := range t.Rules {
		rulePath := fmt.Sprintf("scopeTable.rules[%d]", n)

		segments := strings.Split(strings.TrimPrefix(rule.Path, "/api/v2/"), "/")
		for m, segment := range segments {
			if segment == "." || segment == ".." {
				found.Addf("%s.path: segment %d must not be %q", rulePath, m+1, segment)
			}

			if strings.HasPrefix(segment, "{") {
				segments[m] = "{}"
			}
		}

		shape := rule.Method + " " + strings.Join(segments, "/")
		if first, duplicate := shapes[shape]; duplicate {
			found.Addf("%s: duplicate rule for %s %s, already defined by %s", rulePath, rule.Method, rule.Path, first)
		}

		shapes[shape] = rulePath
	}

	return found
}

// Validate checks the secret fields with the schema of the index and with the
// checks that the schema cannot express, which are the checks that ParseIndex
// makes. It returns an error that wraps ErrInvalidIndex and names the JSON path
// of every problem, as seen from the index.
func (f SecretFields) Validate() error {
	if problems := f.problems(); len(problems) > 0 {
		return newInvalidError("secretFields", problems)
	}

	return nil
}

// problems returns the problems found by the schema or, when the schema has
// none, by the checks that it cannot express.
func (f SecretFields) problems() plugins.ProblemList {
	found, err := schemaProblems("secretFields", func(s schemas) *jsonschema.Schema { return s.secretFields }, f)
	if err != nil {
		return plugins.ProblemList{err.Error()}
	}

	if len(found) > 0 {
		return found
	}

	return f.extraProblems()
}

// extraProblems runs the checks on the secret field names that JSON Schema
// cannot express: names are compared without regard to case, so two names that
// only differ in case are duplicates.
func (f SecretFields) extraProblems() plugins.ProblemList {
	var found plugins.ProblemList

	names := make(map[string]string, len(f.Names))

	for n, name := range f.Names {
		namePath := fmt.Sprintf("secretFields.names[%d]", n)

		if first, duplicate := names[strings.ToLower(name)]; duplicate {
			found.Addf("%s: duplicate name %q, already used by %s", namePath, name, first)
		}

		names[strings.ToLower(name)] = namePath
	}

	return found
}

// checkVersions runs the checks that JSON Schema cannot express on the version
// records found at path.
func checkVersions(path string, versions []VersionRecord) plugins.ProblemList {
	var found plugins.ProblemList

	versionPaths := make(map[string]string, len(versions))

	for n, record := range versions {
		recordPath := fmt.Sprintf("%s[%d]", path, n)

		found.CheckUnique(versionPaths, record.Version, recordPath, "version", "version")
		found = append(found, checkArtifacts(recordPath, record)...)
		found = append(found, checkCommands(recordPath, record.Commands)...)

		normalized := plugins.NormalizePermissions(record.Permissions)
		permissionsPath := recordPath + ".permissions"
		checkNormalized(&found, permissionsPath+".scopes", record.Permissions.Scopes, normalized.Scopes)
		checkNormalized(&found, permissionsPath+".hosts", record.Permissions.Hosts, normalized.Hosts)
		checkNormalized(&found, permissionsPath+".files", record.Permissions.Files, normalized.Files)
		checkNormalized(&found, permissionsPath+".env", record.Permissions.Env, normalized.Env)

		if want := plugins.ConsentHash(record.Permissions, record.Commands); record.PermissionsHash != want {
			found.Addf("%s.permissionsHash: %q does not match %q, the hash of permissions and commands", recordPath, record.PermissionsHash, want)
		}
	}

	return found
}

// checkArtifacts checks that every artifact of a version record is for a
// supported platform that the record lists, and that no platform has two
// artifacts. The record is at recordPath.
func checkArtifacts(recordPath string, record VersionRecord) plugins.ProblemList {
	var found plugins.ProblemList

	artifactPaths := make(map[plugins.Platform]string, len(record.Artifacts))

	for n, artifact := range record.Artifacts {
		artifactPath := fmt.Sprintf("%s.artifacts[%d]", recordPath, n)

		platform, ok := artifact.Platform()

		switch first, duplicate := artifactPaths[platform]; {
		case !ok:
			found.Addf("%s: os %q, arch %q and libc %q are not a supported platform", artifactPath, artifact.OS, artifact.Arch, artifact.Libc)
		case !slices.Contains(record.Platforms, platform):
			found.Addf("%s: platform %q is not listed in platforms", artifactPath, platform)
		case duplicate:
			found.Addf("%s: duplicate artifact for platform %q, already used by %s", artifactPath, platform, first)
		}

		artifactPaths[platform] = artifactPath
	}

	return found
}

// checkCommands checks that the commands of a version record have different
// names. The record is at recordPath.
func checkCommands(recordPath string, commands []plugins.ConsentCommand) plugins.ProblemList {
	var found plugins.ProblemList

	names := make(map[string]bool, len(commands))

	for n, command := range commands {
		if names[command.Name] {
			found.Addf("%s.commands[%d].name: duplicate command name %q", recordPath, n, command.Name)
		}

		names[command.Name] = true
	}

	return found
}

// checkNormalized adds a problem when a list of permissions is not the same as
// its normalized form, which is sorted and free of duplicates. The consent hash
// is calculated from the normalized form, so the list that people are shown must
// be the same list.
func checkNormalized[T comparable](found *plugins.ProblemList, path string, got, normalized []T) {
	if !slices.Equal(got, normalized) {
		found.Addf("%s: must be sorted and free of duplicates, as plugins.Manifest.Permissions returns them", path)
	}
}

// rejectCaseVariants reports every object key that is only a case variant of a
// field of typ, sorted. The standard library decoder matches field names without
// regard to case while the schema does not, so a key such as "Homepage" would be
// decoded without ever being validated. Fields that typ does not know at all
// are left alone, because the index ignores them on purpose.
func rejectCaseVariants(path string, value any, typ reflect.Type) []string {
	var problems []string

	collectCaseVariants(&problems, path, value, typ)
	slices.Sort(problems)

	return problems
}

// collectCaseVariants adds the problems that rejectCaseVariants reports to
// problems.
func collectCaseVariants(problems *[]string, path string, value any, typ reflect.Type) {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	switch value := value.(type) {
	case []any:
		if typ.Kind() != reflect.Slice {
			return
		}

		for n, item := range value {
			collectCaseVariants(problems, fmt.Sprintf("%s[%d]", path, n), item, typ.Elem())
		}
	case map[string]any:
		if typ.Kind() != reflect.Struct {
			return
		}

		fields := make(map[string]reflect.Type, typ.NumField())
		for field := range typ.Fields() {
			name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
			if name != "" && name != "-" {
				fields[name] = field.Type
			}
		}

		for key, child := range value {
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}

			if fieldType, ok := fields[key]; ok {
				collectCaseVariants(problems, childPath, child, fieldType)
				continue
			}

			for name := range fields {
				if strings.EqualFold(name, key) {
					*problems = append(*problems, fmt.Sprintf("%s: field name must be written as %q", childPath, name))
				}
			}
		}
	}
}
