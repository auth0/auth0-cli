// Package scaffold writes a new plugin project: a plugin.json generated from the
// typed manifest, a starter dev-policy.json for the scopes it declares, and the
// files of a starter project for one language.
//
// A template tree holds one project root per language, go/ and ts/. A file named
// X.tmpl is written as X, and a file without the suffix is written as it is. Three
// tokens are replaced in the content of every file: __PLUGIN_NAME__, __COMMAND__
// and __LICENSE__. Only values with a strict pattern are ever substituted, so the
// free text of the author, such as the description, goes into plugin.json through
// the JSON encoder and never into a source file.
package scaffold

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/devpolicy"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
	"github.com/auth0/auth0-cli/internal/plugins/store"
)

// Lang is a language that has a starter project.
type Lang string

// The languages that have a starter project, named like the directories of a
// template tree.
const (
	LangGo         Lang = "go"
	LangTypeScript Lang = "ts"
)

const (
	// The suffix of a template file, which is not part of the name it is written as.
	templateSuffix = ".tmpl"

	// The file that is generated, so a template tree cannot hold it.
	manifestName = "plugin.json"

	// The prefix of the paths of the Management API, which the starter rules of the
	// dev policy are under.
	apiPrefix = "/api/v2/"

	// The limits of one project in a template tree. They are checked before
	// anything is written, and they hold for the tree inside the CLI as well, so a
	// tree from --from cannot be bigger than the one that ships.
	maxFiles      = 64
	maxDirs       = 64
	maxDepth      = 8
	maxFileBytes  = 64 << 10
	maxTotalBytes = 256 << 10

	dirPerm  os.FileMode = 0o755
	filePerm os.FileMode = 0o644
)

var (
	// The pattern of a name that goes into source files and file names. It is
	// stricter than the pattern of a plugin ID, so it is also a command name.
	namePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

	licensePattern = regexp.MustCompile(`^[A-Za-z0-9.+-]+$`)

	// A token that is left after the replacement is a typo in a template.
	leftoverToken = regexp.MustCompile(`__[A-Z][A-Z_]*__`)

	// The pattern of a scope that has starter rules: a verb and a resource.
	scopePattern = regexp.MustCompile(`^(read|create|update|delete):([a-z][a-z0-9_]*)$`)

	// The token of the README section about dev-policy.json, which is replaced by
	// the section when the project has that file, and dropped when it does not.
	devPolicyToken = "__DEV_POLICY_SECTION__"

	// The function that moves the finished directory into place, which tests
	// replace to make it fail.
	renameDir = plugins.Rename
)

// Options describes the plugin to create. Every field is required, apart from
// Scopes, which may be empty, and CLIVersion.
type Options struct {
	// Name is the short name of the plugin. It is the name of the executable and
	// of the module or package, and ID ends with it.
	Name string

	// ID is the plugin ID, registry/name.
	ID string

	Version     string
	Description string

	// Command is the name of the first command. It is the root command, which runs
	// as `auth0 <name>`, when it equals Name.
	Command            string
	CommandDescription string

	// Dir is where the project is written. It may not exist, or may be empty.
	Dir string

	// Team and Bugs become the owner of the plugin.
	Team string
	Bugs string

	// License is written into the files of the project. It is not in the manifest.
	License string

	// Scopes are the Management API scopes of the first command.
	Scopes []string
	Lang   Lang

	// CLIVersion is the version of this CLI. The plugin requires it or a later one.
	// A version that is not MAJOR.MINOR.PATCH, such as dev, sets no requirement.
	CLIVersion string
}

// InputError is a failure caused by what the caller gave: a value that cannot be
// used, a template tree that breaks a rule, or a target directory that cannot be
// written to. Any other error is a failure of the machine, such as a full disk.
type InputError struct {
	err error
}

func (e *InputError) Error() string { return e.err.Error() }

func (e *InputError) Unwrap() error { return e.err }

// inputf returns an InputError with a formatted text.
func inputf(format string, args ...any) error {
	return &InputError{err: fmt.Errorf(format, args...)}
}

// file is a file to write, with a slash separated path inside the project.
type file struct {
	path string
	data []byte
}

// Create writes the plugin project into opts.Dir and returns the paths of the
// files it wrote, sorted, relative to that directory and separated by slashes.
// The project is built in a temporary directory beside the target and renamed
// into place, so a failure leaves the target as it was. A failure that the caller
// can fix is an *InputError.
func Create(opts Options, templates fs.FS) ([]string, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}

	manifest, err := opts.manifest()
	if err != nil {
		return nil, err
	}

	tree, err := readTree(templates, opts.Lang)
	if err != nil {
		return nil, err
	}

	files, err := opts.render(tree, manifest)
	if err != nil {
		return nil, err
	}

	files, err = opts.withDevPolicy(files)
	if err != nil {
		return nil, err
	}

	target, exists, err := checkTarget(opts.Dir)
	if err != nil {
		return nil, err
	}

	if err := writeTree(target, exists, files); err != nil {
		return nil, fmt.Errorf("cannot create %s: %w", target, err)
	}

	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.path)
	}

	return paths, nil
}

// CheckName fails unless name can be the short name of a plugin: it goes into file
// names and source files, and it is the command that runs the plugin.
func CheckName(name string) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("the name %q must start with a lowercase letter and hold only lowercase letters, digits and hyphens", name)
	}

	return nil
}

// validate checks the values that go into source files and file names, and the
// ones that tie the plugin together. The rest is checked by the manifest.
func (o Options) validate() error {
	if err := o.check(); err != nil {
		return &InputError{err: err}
	}

	return nil
}

func (o Options) check() error {
	if o.Lang != LangGo && o.Lang != LangTypeScript {
		return fmt.Errorf("the language %q is not one of %s and %s", o.Lang, LangGo, LangTypeScript)
	}

	if err := CheckName(o.Name); err != nil {
		return err
	}

	if err := store.CheckID(o.ID); err != nil {
		return err
	}

	if want := registry.DefaultRegistryName + "/" + o.Name; o.ID != want {
		return fmt.Errorf("the ID %q must be %s: the only registry this CLI knows is %q, and the ID ends with the name of the plugin", o.ID, want, registry.DefaultRegistryName)
	}

	if !namePattern.MatchString(o.Command) {
		return fmt.Errorf("the command %q must start with a lowercase letter and hold only lowercase letters, digits and hyphens", o.Command)
	}

	if !licensePattern.MatchString(o.License) {
		return fmt.Errorf("the license %q may hold only letters, digits, dots, plus signs and hyphens", o.License)
	}

	switch {
	case o.Dir == "":
		return errors.New("the directory is required")
	case o.Team == "":
		return errors.New("the team is required")
	case o.Bugs == "":
		return errors.New("the bugs URL is required")
	}

	return nil
}

// manifest returns the content of plugin.json. It is read back with the strict
// parser of the registry before anything is written, so a plugin that this
// command creates is one that it can publish.
func (o Options) manifest() ([]byte, error) {
	// The schema names itself, so that the manifest points at the schema it
	// is checked against.
	var schema struct {
		ID string `json:"$id"`
	}

	if err := json.Unmarshal(plugins.ManifestSchema(), &schema); err != nil {
		return nil, fmt.Errorf("cannot read the manifest schema: %w", err)
	}

	manifest := plugins.Manifest{
		Schema:      schema.ID,
		ID:          o.ID,
		Version:     o.Version,
		APIVersion:  1,
		Description: o.Description,
		Owner:       plugins.Owner{Team: o.Team, Bugs: o.Bugs},
		Engines:     engines(o.CLIVersion),
		Platforms:   plugins.AllPlatforms(),
		Commands: []plugins.Command{{
			Name:        o.Command,
			Description: o.CommandDescription,
			Timeout:     "5m",
			Output:      &plugins.Output{JSON: true, AgentMode: true},
			Permissions: &plugins.Permissions{
				// Empty lists, not null, which the schema does not accept.
				Scopes: append([]string{}, o.Scopes...),
				Hosts:  []plugins.HostPermission{},
				Files:  []plugins.FilePermission{},
				Env:    []plugins.EnvPermission{},
			},
		}},
	}

	var out bytes.Buffer

	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(manifest); err != nil {
		return nil, fmt.Errorf("cannot write %s: %w", manifestName, err)
	}

	if _, err := plugins.ParseManifest(out.Bytes()); err != nil {
		return nil, inputf("%s would not be valid: %s", manifestName, strings.ReplaceAll(err.Error(), "\n", "; "))
	}

	return out.Bytes(), nil
}

// withDevPolicy adds the starter dev-policy.json to the files, unless the template
// tree has a file of that name, which is the author's own and is kept as it is, or
// the scopes give no rule.
func (o Options) withDevPolicy(files []file) ([]file, error) {
	if slices.ContainsFunc(files, func(f file) bool { return strings.EqualFold(f.path, devpolicy.FileName) }) {
		return files, nil
	}

	policy, ok, err := o.devPolicy()
	if err != nil {
		return nil, err
	}

	if !ok {
		return files, nil
	}

	files = append(files, file{path: devpolicy.FileName, data: policy})
	slices.SortFunc(files, func(a, b file) int { return strings.Compare(a.path, b.path) })

	return files, nil
}

// SkippedScopes returns the scopes that get no starter rule in dev-policy.json,
// sorted and without repeats: the ones that are not a verb (read, create, update
// or delete) and a resource of the form read:clients.
func (o Options) SkippedScopes() []string {
	var skipped []string

	for _, scope := range sortedScopes(o.Scopes) {
		if !scopePattern.MatchString(scope) {
			skipped = append(skipped, scope)
		}
	}

	return skipped
}

// sortedScopes returns the scopes sorted and without repeats.
func sortedScopes(scopes []string) []string {
	return slices.Compact(slices.Sorted(slices.Values(scopes)))
}

// devPolicy returns the content of the starter dev-policy.json: for each scope of
// the first command, the calls of the Management API that the scope is named for.
// A scope that is not a verb and a resource of the form read:clients is skipped.
// It reports false, and no content, when no scope gives a rule. The policy is
// checked the way a link checks it before it is returned.
func (o Options) devPolicy() ([]byte, bool, error) {
	var rules []registry.ScopeRule

	for _, scope := range sortedScopes(o.Scopes) {
		match := scopePattern.FindStringSubmatch(scope)
		if match == nil {
			continue
		}

		path := apiPrefix + strings.ReplaceAll(match[2], "_", "-")
		anyOf := []string{scope}

		switch match[1] {
		case "read":
			rules = append(rules,
				registry.ScopeRule{Method: "GET", Path: path, AnyOf: anyOf},
				registry.ScopeRule{Method: "GET", Path: path + "/{id}", AnyOf: anyOf},
			)
		case "create":
			rules = append(rules, registry.ScopeRule{Method: "POST", Path: path, AnyOf: anyOf})
		case "update":
			rules = append(rules, registry.ScopeRule{Method: "PATCH", Path: path + "/{id}", AnyOf: anyOf})
		case "delete":
			rules = append(rules, registry.ScopeRule{Method: "DELETE", Path: path + "/{id}", AnyOf: anyOf})
		}
	}

	if len(rules) == 0 {
		return nil, false, nil
	}

	slices.SortFunc(rules, func(a, b registry.ScopeRule) int {
		return cmp.Or(strings.Compare(a.Path, b.Path), strings.Compare(a.Method, b.Method))
	})

	policy := devpolicy.File{Rules: rules}

	if _, err := policy.Index(); err != nil {
		return nil, false, inputf("%s would not be valid: %s", devpolicy.FileName, strings.ReplaceAll(err.Error(), "\n", "; "))
	}

	var out bytes.Buffer

	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(policy); err != nil {
		return nil, false, fmt.Errorf("cannot write %s: %w", devpolicy.FileName, err)
	}

	return out.Bytes(), true, nil
}

// engines returns the requirement on the CLI that runs the plugin, which is the
// version of this CLI or a later one. It is nil for a build that has no version.
func engines(cliVersion string) *plugins.Engines {
	version, ok := plugins.ParseVersion(cliVersion)
	if !ok {
		return nil
	}

	return &plugins.Engines{Auth0CLI: ">=" + version.String()}
}

// readTree reads every file of the project of a language, and fails on anything
// that is not a regular file or that goes over a limit. It follows no symlink.
func readTree(templates fs.FS, lang Lang) ([]file, error) {
	root, err := projectRoot(templates, lang)
	if err != nil {
		return nil, err
	}

	var (
		files []file
		dirs  int
		total int
	)

	err = fs.WalkDir(root, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			if path == "." {
				return nil
			}

			if dirs++; dirs > maxDirs {
				return inputf("the %s templates have more than %d directories", lang, maxDirs)
			}

			if strings.Count(path, "/") >= maxDepth {
				return inputf("the %s templates have a directory deeper than %d levels", lang, maxDepth)
			}

			return nil
		}

		if !entry.Type().IsRegular() {
			return inputf("%s in the %s templates is not a regular file (a symlink is not followed)", path, lang)
		}

		if len(files) == maxFiles {
			return inputf("the %s templates have more than %d files", lang, maxFiles)
		}

		data, err := readFile(root, path)
		if err != nil {
			return err
		}

		if total += len(data); total > maxTotalBytes {
			return inputf("the %s templates are larger than %d bytes", lang, maxTotalBytes)
		}

		files = append(files, file{path: path, data: data})

		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

// projectRoot returns the project of a language, which must be a directory of the
// tree and not a symlink to one.
func projectRoot(templates fs.FS, lang Lang) (fs.FS, error) {
	entries, err := fs.ReadDir(templates, ".")
	if err != nil {
		return nil, fmt.Errorf("cannot read the templates: %w", err)
	}

	for _, entry := range entries {
		if entry.Name() != string(lang) {
			continue
		}

		if !entry.IsDir() {
			return nil, inputf("%s in the templates is not a directory (a symlink is not followed)", lang)
		}

		return fs.Sub(templates, string(lang))
	}

	return nil, inputf("the templates have no %s directory", lang)
}

// readFile reads a file of the tree, up to the limit for one file.
func readFile(root fs.FS, path string) ([]byte, error) {
	handle, err := root.Open(path)
	if err != nil {
		return nil, err
	}

	defer handle.Close()

	data, err := plugins.ReadCapped(handle, maxFileBytes)
	if errors.Is(err, plugins.ErrTooLarge) {
		return nil, inputf("%s is larger than %d bytes", path, maxFileBytes)
	}

	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", path, err)
	}

	return data, nil
}

// render replaces the tokens in the files, drops the template suffix from their
// names, adds plugin.json and sorts the result by path.
func (o Options) render(tree []file, manifest []byte) ([]file, error) {
	section, err := o.devPolicySection(tree)
	if err != nil {
		return nil, err
	}

	pairs := []string{
		"__PLUGIN_NAME__", o.Name,
		"__COMMAND__", o.Command,
		"__LICENSE__", o.License,
	}

	if section == "" {
		// The section goes with the blank line after it.
		pairs = append(pairs, devPolicyToken+"\n\n", "")
	}

	pairs = append(pairs, devPolicyToken, section)

	replacer := strings.NewReplacer(pairs...)

	files := make([]file, 0, len(tree)+1)
	files = append(files, file{path: manifestName, data: manifest})

	// The files by lower-case path, since a file system may fold the case of names.
	type origin struct{ from, path string }

	seen := map[string]origin{strings.ToLower(manifestName): {"the generated " + manifestName, manifestName}}

	for _, f := range tree {
		path := strings.TrimSuffix(f.path, templateSuffix)
		if path == "" || strings.HasSuffix(path, "/") {
			return nil, inputf("%s has no name once %s is dropped", f.path, templateSuffix)
		}

		if !filepath.IsLocal(filepath.FromSlash(path)) {
			return nil, inputf("%s is not a path inside the project", f.path)
		}

		if first, duplicate := seen[strings.ToLower(path)]; duplicate {
			if first.path != path {
				return nil, inputf("%s would be written to %s, and %s to %s, which is the same name when the case is ignored", f.path, path, first.from, first.path)
			}

			return nil, inputf("%s would be written to %s, which %s already uses", f.path, path, first.from)
		}

		seen[strings.ToLower(path)] = origin{f.path, path}

		// A name carries no token: nothing is replaced in it.
		if token := leftoverToken.FindString(path); token != "" {
			return nil, inputf("%s has the token %s in its name, and file names are not replaced", f.path, token)
		}

		text := replacer.Replace(string(f.data))
		if token := leftoverToken.FindString(text); token != "" {
			return nil, inputf("%s has the token %s, which is not one of __PLUGIN_NAME__, __COMMAND__, __LICENSE__ and __DEV_POLICY_SECTION__", f.path, token)
		}

		files = append(files, file{path: path, data: []byte(text)})
	}

	slices.SortFunc(files, func(a, b file) int { return strings.Compare(a.path, b.path) })

	return files, nil
}

// devPolicySection returns the README section about dev-policy.json, or an empty
// text when the project has no such file: the one of the template tree, or the
// starter one that Create writes when a scope gives a rule.
func (o Options) devPolicySection(tree []file) (string, error) {
	own := slices.ContainsFunc(tree, func(f file) bool {
		return strings.EqualFold(strings.TrimSuffix(f.path, templateSuffix), devpolicy.FileName)
	})

	if !own {
		_, ok, err := o.devPolicy()
		if err != nil {
			return "", err
		}

		if !ok {
			return "", nil
		}
	}

	return "## Local development policy\n\n" +
		"`dev-policy.json` lists the Management API calls that each scope allows. It holds starter rules for the scopes the CLI recognizes, which are read, create, update and delete of a resource. A scope it does not recognize needs a rule that you add by hand. Run `auth0 plugin dev validate` to check that every scope in `plugin.json` has a rule. " +
		"The file is not signed, and only a linked build in dev mode reads it, so a published plugin never uses it. Edit it when the plugin calls a route the starter rules do not cover. Any change needs a new `auth0 plugin approve " + o.Name + "`.", nil
}

// checkTarget returns the absolute path of the directory to write, and whether it
// exists. It fails unless the directory is missing, with a parent that exists, or
// is an empty directory.
func checkTarget(dir string) (string, bool, error) {
	target, err := filepath.Abs(dir)
	if err != nil {
		return "", false, fmt.Errorf("cannot find %s: %w", dir, err)
	}

	info, err := os.Lstat(target)
	if errors.Is(err, fs.ErrNotExist) {
		parent, statErr := os.Stat(filepath.Dir(target))
		if statErr != nil || !parent.IsDir() {
			return "", false, inputf("cannot create %s: its parent directory does not exist", target)
		}

		return target, false, nil
	}

	if err != nil {
		return "", false, fmt.Errorf("cannot check %s: %w", target, err)
	}

	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		return "", false, inputf("%s is a symlink, which is not followed. Give a directory that does not exist, or an empty one", target)
	case !info.IsDir():
		return "", false, inputf("%s exists and is not a directory. Give a directory that does not exist, or an empty one", target)
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		return "", false, fmt.Errorf("cannot read %s: %w", target, err)
	}

	if len(entries) > 0 {
		return "", false, inputf("%s is not empty: never overwrite a non-empty directory. Give a new directory with --dir", target)
	}

	// An empty directory is replaced by the finished one, which would leave a
	// shell that is in it in a directory that no longer exists.
	if here, err := os.Stat("."); err == nil && os.SameFile(info, here) {
		return "", false, inputf("%s is the current directory, which cannot be replaced. Run the command from the parent directory, or give a directory that does not exist", target)
	}

	return target, true, nil
}

// writeTree writes the files into a temporary directory beside the target and
// renames it to the target. The temporary directory is removed when anything
// fails. An empty target is removed first, because Windows cannot rename a
// directory over another.
func writeTree(target string, exists bool, files []file) error {
	temp, err := os.MkdirTemp(filepath.Dir(target), "."+filepath.Base(target)+".tmp-")
	if err != nil {
		return err
	}

	done := false

	defer func() {
		if !done {
			_ = os.RemoveAll(temp)
		}
	}()

	for _, f := range files {
		path := filepath.Join(temp, filepath.FromSlash(f.path))

		if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
			return err
		}

		if err := os.WriteFile(path, f.data, filePerm); err != nil {
			return err
		}
	}

	// MkdirTemp makes a private directory, and the project is not.
	if err := os.Chmod(temp, dirPerm); err != nil {
		return err
	}

	if exists {
		if err := os.Remove(target); err != nil {
			return err
		}
	}

	if err := renameDir(temp, target); err != nil {
		if exists {
			// Put the empty directory back, so the target is as it was.
			_ = os.Mkdir(target, dirPerm)
		}

		return err
	}

	done = true

	return nil
}
