// Package devpolicy reads the dev policy of a linked plugin: the scope rules
// that a local build may use, kept in a dev-policy.json next to its plugin.json.
//
// A published plugin runs against the scope table of the signed registry index.
// A build that is not published is in no registry, and the CLI embeds no root of
// trust yet, so there is no index to ask. The dev policy takes the place of the
// scope table for a linked plugin only. It is not signed, and nothing here
// decides whether it may be used: the caller must have checked that dev mode is
// on. The rules go through the same validators and the same proxy policy as the
// ones of the registry, so a call is allowed by the same logic.
package devpolicy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
)

const (
	// FileName is the name of the dev policy in the directory of a build.
	FileName = "dev-policy.json"

	// The format version of the scope table and of the secret fields that the
	// proxy reads, which is the only one.
	formatVersion = 1

	// The largest dev policy that is read, in bytes.
	maxFileBytes = 1 << 20
)

var (
	// ErrMissing means there is no dev policy at the path.
	ErrMissing = errors.New("there is no dev policy")

	// ErrInvalid means the dev policy cannot be used: it cannot be read, is too
	// large, is not the JSON this CLI reads, or holds rules or names that the
	// registry would refuse.
	ErrInvalid = errors.New("the dev policy is not valid")
)

// DefaultSecretFields are the names of the fields that are removed from every
// response in dev mode, whatever the dev policy lists. They are the names that
// the secret fields of the registry list, at the least, so that a linked build
// never sees a secret that a published one would not.
var DefaultSecretFields = []string{"client_secret", "signing_keys", "encryption_key", "private_key"}

// File is the content of a dev-policy.json.
type File struct {
	// Rules say which scopes allow which Management API calls, in the form of the
	// scope table of the registry. A call that no rule matches is denied.
	Rules []registry.ScopeRule `json:"rules"`

	// SecretFields are names of fields to remove from responses, in addition to
	// DefaultSecretFields.
	SecretFields []string `json:"secretFields,omitempty"`

	// The path is where the file was read from, for messages.
	path string

	// The SHA-256 of the exact bytes that were read, which the consent hash of a
	// linked build covers.
	sum [sha256.Size]byte
}

// PathFor returns where the dev policy of the build in the directory is: the
// override when there is one, and dev-policy.json in the directory otherwise.
func PathFor(linkDir, override string) string {
	if override != "" {
		return override
	}

	return filepath.Join(linkDir, FileName)
}

// Load reads the dev policy at the path. A file that is not there is an error
// that wraps ErrMissing and says how to create one. A file that cannot be read, is
// not a regular file, is too large, is not valid JSON, has a key twice, has a
// field this CLI does not know, or has more than one document is an error that
// wraps ErrInvalid and names the path. Whether the rules are good is checked by
// Index.
func Load(path string) (*File, error) {
	data, err := readRegularFile(path)
	if errors.Is(err, os.ErrNotExist) {
		// A file with another name was given with --policy, so creating the default
		// file next to plugin.json would not help.
		if filepath.Base(path) != FileName {
			return nil, fmt.Errorf("%w at %s. Create the file, or link the plugin again with another one: auth0 plugin dev link --policy <file>", ErrMissing, plugins.SafeText(path))
		}

		return nil, fmt.Errorf("%w at %s. Create a %s next to plugin.json, or give the file with: auth0 plugin dev link --policy <file>", ErrMissing, plugins.SafeText(path), FileName)
	}

	if err != nil {
		return nil, fmt.Errorf("%w (%s: %w)", ErrInvalid, plugins.SafeText(path), err)
	}

	if err := plugins.RejectDuplicateKeys(data); err != nil {
		return nil, fmt.Errorf("%w (%s: %w)", ErrInvalid, plugins.SafeText(path), err)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var file File
	if err := decoder.Decode(&file); err != nil {
		return nil, fmt.Errorf("%w (%s: %w)", ErrInvalid, plugins.SafeText(path), err)
	}

	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w (%s: it has more than one JSON document)", ErrInvalid, plugins.SafeText(path))
	}

	// Decoding matches the name of a field without regard to case, so that "RULES"
	// would be read as "rules" and could hide a rule from a person who reads the
	// file. The file is accepted only if writing what was read gives the same
	// document back.
	canonical, err := json.Marshal(file)
	if err != nil {
		return nil, fmt.Errorf("%w (%s: %w)", ErrInvalid, plugins.SafeText(path), err)
	}

	var got, want any

	if err := json.Unmarshal(data, &got); err != nil {
		return nil, fmt.Errorf("%w (%s: %w)", ErrInvalid, plugins.SafeText(path), err)
	}

	if err := json.Unmarshal(canonical, &want); err != nil {
		return nil, fmt.Errorf("%w (%s: %w)", ErrInvalid, plugins.SafeText(path), err)
	}

	if !reflect.DeepEqual(got, want) {
		return nil, fmt.Errorf("%w (%s: it has a field that is not written in the exact case, or an empty or null field. Use the names rules, method, path, anyOf and secretFields)", ErrInvalid, plugins.SafeText(path))
	}

	file.path = path
	file.sum = sha256.Sum256(data)

	return &file, nil
}

// consentDomain separates the consent hash of a linked build from every other hash
// that is made of the same bytes.
const consentDomain = "auth0-dev-policy/v1\n"

// ConsentHash returns the hash that the approval of a linked build covers. The
// permissions of plugin.json do not say which Management API calls a scope allows
// for a linked build, because the dev policy does, so the approval must cover both.
// It is the manifest hash for a build with no dev policy, and otherwise a hash of
// the manifest hash and of the exact bytes of the dev policy, so that any change of
// the file needs a new approval.
func ConsentHash(manifestHash string, f *File) string {
	if f == nil {
		return manifestHash
	}

	sum := sha256.Sum256([]byte(consentDomain + manifestHash + "\n" + hex.EncodeToString(f.sum[:])))

	return "sha256:" + hex.EncodeToString(sum[:])
}

// CopyRules returns a copy of the rules, so that the caller can show them without
// changing the file. It is not called Rules, because the file has a field of that
// name.
func (f *File) CopyRules() []registry.ScopeRule {
	rules := make([]registry.ScopeRule, len(f.Rules))

	for i, rule := range f.Rules {
		rule.AnyOf = slices.Clone(rule.AnyOf)
		rules[i] = rule
	}

	return rules
}

// BroadRules returns a copy of the rules whose path has no literal segment after
// the /api/v2 prefix, such as /api/v2/{a}/{b}. Every segment of such a path matches
// anything, so the rule can match a call on any resource.
func (f *File) BroadRules() []registry.ScopeRule {
	var broad []registry.ScopeRule

	for _, rule := range f.CopyRules() {
		if isBroad(rule.Path) {
			broad = append(broad, rule)
		}
	}

	return broad
}

// WriteWithReadScopes returns a copy of the rules that allow a POST, PUT, PATCH or
// DELETE call with scopes that all start with "read:". Such a rule lets a plugin
// change something while it holds only read access, which is rarely what the
// person who wrote it meant.
func (f *File) WriteWithReadScopes() []registry.ScopeRule {
	var flagged []registry.ScopeRule

	for _, rule := range f.CopyRules() {
		switch rule.Method {
		case "POST", "PUT", "PATCH", "DELETE":
		default:
			continue
		}

		if len(rule.AnyOf) == 0 {
			continue
		}

		if slices.ContainsFunc(rule.AnyOf, func(scope string) bool { return !strings.HasPrefix(scope, "read:") }) {
			continue
		}

		flagged = append(flagged, rule)
	}

	return flagged
}

// isBroad reports whether the path starts with /api/v2/ and every segment after it
// is a parameter.
func isBroad(path string) bool {
	rest, found := strings.CutPrefix(path, "/api/v2/")
	if !found || rest == "" {
		return false
	}

	for _, segment := range strings.Split(rest, "/") {
		if !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}") {
			return false
		}
	}

	return true
}

// Index returns the registry index that the proxy policy is built from: the rules
// as the scope table, and DefaultSecretFields followed by the extra names as the
// secret fields. The index has no plugins and a zero policy, which takes the
// defaults of the registry. It fails with an error that wraps ErrInvalid when there
// is no rule, or when the registry would refuse the scope table or the names.
func (f *File) Index() (*registry.Index, error) {
	invalid := func(why error) error {
		if f.path == "" {
			return fmt.Errorf("%w (%w)", ErrInvalid, why)
		}

		return fmt.Errorf("%w (%s: %w)", ErrInvalid, plugins.SafeText(f.path), why)
	}

	if len(f.Rules) == 0 {
		return nil, invalid(errors.New("rules lists no rule, so every call would be denied"))
	}

	table := registry.ScopeTable{FormatVersion: formatVersion, Rules: f.Rules}
	if err := table.Validate(); err != nil {
		return nil, invalid(err)
	}

	fields := registry.SecretFields{FormatVersion: formatVersion, Names: secretNames(f.SecretFields)}
	if err := fields.Validate(); err != nil {
		return nil, invalid(err)
	}

	return &registry.Index{FormatVersion: formatVersion, ScopeTable: table, SecretFields: fields}, nil
}

// secretNames returns the default names followed by the extra ones, without a
// name that an earlier one equals when case is ignored.
func secretNames(extra []string) []string {
	names := make([]string, 0, len(DefaultSecretFields)+len(extra))
	seen := make(map[string]struct{}, cap(names))

	for _, name := range slices.Concat(DefaultSecretFields, extra) {
		key := strings.ToLower(name)
		if _, found := seen[key]; found {
			continue
		}

		seen[key] = struct{}{}
		names = append(names, name)
	}

	return names
}

// readRegularFile reads a file of at most maxFileBytes that is a regular file
// once links are followed. The path is checked before it is opened, because
// opening a named pipe waits for a writer. The file is opened without waiting, so
// that a pipe swapped in after that check cannot block the open, and the opened
// file is checked again.
func readRegularFile(path string) ([]byte, error) {
	before, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !before.Mode().IsRegular() {
		return nil, errors.New("it is not a regular file")
	}

	file, err := openFile(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if !info.Mode().IsRegular() {
		return nil, errors.New("it is not a regular file")
	}

	data, err := plugins.ReadCapped(file, maxFileBytes)
	if err != nil {
		return nil, err
	}

	return data, nil
}
