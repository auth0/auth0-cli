package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/buildinfo"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/devlink"
	"github.com/auth0/auth0-cli/internal/plugins/devpolicy"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
	"github.com/auth0/auth0-cli/internal/plugins/scaffold"
	"github.com/auth0/auth0-cli/internal/plugins/store"
)

// pluginDevModeEnv turns the commands for plugin authors on, and with them the
// links that skip the integrity checks of an installed plugin. Only the exact
// value true does: this variable lets a plugin run without being verified, so
// anything else, such as 1 or True, leaves it off.
const pluginDevModeEnv = "AUTH0_CLI_PLUGIN_DEV_MODE"

var pluginDevBin = Flag{
	Name:     "Bin",
	LongForm: "bin",
	Help:     "The executable of the plugin. By default it is the file named like the plugin in the directory of the plugin.",
}

var pluginDevPolicy = Flag{
	Name:     "Policy",
	LongForm: "policy",
	Help:     "File with the scope rules that this linked build may use. Defaults to " + devpolicy.FileName + " in the plugin directory.",
}

const (
	pluginCreateDefaultVersion     = "0.1.0"
	pluginCreateDefaultDescription = "A plugin for the Auth0 CLI"
	pluginCreateDefaultLicense     = "UNLICENSED"
	pluginCreateDefaultScope       = "read:clients"
)

var (
	pluginCreateName = Argument{
		Name: "Name",
		Help: "Short name of the plugin: lowercase letters, digits and hyphens. The plugin runs as auth0 <name>.",
	}
	pluginCreateLang = Flag{
		Name:       "Language",
		LongForm:   "lang",
		Help:       "Language of the starter project: go or ts.",
		IsRequired: true,
	}
	pluginCreateAuthor = Flag{
		Name:       "Author",
		LongForm:   "author",
		Help:       "Team or person that owns the plugin. It becomes owner.team in plugin.json.",
		IsRequired: true,
	}
	pluginCreateBugs = Flag{
		Name:       "Bugs URL",
		LongForm:   "bugs",
		Help:       "HTTPS URL where people report problems with the plugin. It becomes owner.bugs in plugin.json.",
		IsRequired: true,
	}
	pluginCreateID = Flag{
		Name:     "ID",
		LongForm: "id",
		Help:     "ID of the plugin, " + registry.DefaultRegistryName + "/<name>. The default is " + registry.DefaultRegistryName + "/ followed by the name.",
	}
	pluginCreateVersion = Flag{
		Name:     "Version",
		LongForm: "version",
		Help:     "First version of the plugin, as MAJOR.MINOR.PATCH.",
	}
	pluginCreateDescription = Flag{
		Name:     "Description",
		LongForm: "description",
		Help:     "One line that says what the plugin does, up to 120 characters.",
	}
	pluginCreateCommand = Flag{
		Name:     "Command",
		LongForm: "command",
		Help:     "Name of the first command. When it is the name of the plugin it runs as `auth0 <name>`, otherwise as `auth0 <name> <command>`. The default is the name of the plugin.",
	}
	pluginCreateCommandDescription = Flag{
		Name:     "Command Description",
		LongForm: "command-description",
		Help:     "What the first command does. The default is the description of the plugin.",
	}
	pluginCreateDir = Flag{
		Name:     "Directory",
		LongForm: "dir",
		Help:     "Where to write the project. It may not exist, or may be empty. The default is a directory named like the plugin.",
	}
	pluginCreateLicense = Flag{
		Name:     "License",
		LongForm: "license",
		Help:     "License of the project, written into its files. There is no LICENSE file.",
	}
	pluginCreateScopes = Flag{
		Name:     "Scopes",
		LongForm: "scopes",
		Help:     "Comma-separated Management API scopes that the first command may use.",
	}
	pluginCreateFrom = Flag{
		Name:     "From",
		LongForm: "from",
		Help:     "A directory with your own starter projects, laid out like the built-in ones: go and ts, each a project. A file named X.tmpl is written as X.",
	}
)

// pluginDevMode reports whether dev mode is on. It reads one variable and
// nothing else, so it is cheap to ask on every start of the CLI.
func pluginDevMode() bool {
	return os.Getenv(pluginDevModeEnv) == "true"
}

// requirePluginDevMode fails unless dev mode is on.
func requirePluginDevMode() error {
	if pluginDevMode() {
		return nil
	}

	return validationError{
		err:    fmt.Errorf("the commands for plugin authors are available only in dev mode. Turn it on with: %s=true", pluginDevModeEnv),
		reason: "plugin_dev_mode_required",
	}
}

func pluginDevCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Check and run a plugin you are building",
		Long: "Commands for plugin authors. They are available only in dev mode, which you turn on with " + pluginDevModeEnv + "=true.\n\n" +
			"A linked plugin runs as `auth0 <name>` from the directory of your build, through the same proxy, permissions and approval as an installed plugin. " +
			"Only the checks that make no sense for a build that is not published are skipped: the hashes of its files, the lockfile and the revocation list.",
		Example: `  auth0 plugin dev create checkmate --lang go --author "Platform team" --bugs https://github.com/acme/checkmate/issues
  auth0 plugin dev validate
  auth0 plugin dev link ./checkmate`,
		// The commands for authors show in the help only in dev mode.
		Hidden: !pluginDevMode(),
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(createPluginCmd(cli))
	cmd.AddCommand(linkPluginCmd(cli))
	cmd.AddCommand(unlinkPluginCmd(cli))
	cmd.AddCommand(validatePluginCmd(cli))

	return cmd
}

func createPluginCmd(cli *cli) *cobra.Command {
	var (
		opts    scaffold.Options
		lang    string
		from    string
		version = pluginCreateDefaultVersion
		scopes  []string
	)

	cmd := &cobra.Command{
		Use:   "create [name]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Create a new plugin project",
		Long: "Write a new plugin project into a directory: a plugin.json, and a small starter project in Go or TypeScript that calls the Management API through the CLI. " +
			"The project builds, tests and releases with the tools of its language, and the README says how.\n\n" +
			"The name is the short name of the plugin: lowercase letters, digits and hyphens, starting with a letter. The plugin runs as `auth0 <name>`. " +
			"The directory may not exist or may be empty, and an existing project is never overwritten. " +
			"Missing values are asked for in a terminal and are an error otherwise.\n\n" +
			"With `--from` the starter projects come from a directory of yours instead of the CLI. They may not hold links, more than 64 files, or more than 256 KB.",
		Example: `  auth0 plugin dev create checkmate --lang go --author "Platform team" --bugs https://github.com/acme/checkmate/issues
  auth0 plugin dev create checkmate --lang ts --author "Platform team" --bugs https://github.com/acme/checkmate/issues --scopes read:clients,read:users
  auth0 plugin dev create checkmate --lang go --author "Platform team" --bugs https://github.com/acme/checkmate/issues --dir ./plugins/checkmate --json`,
		// The gate comes before the check of the required flags.
		PreRunE: func(*cobra.Command, []string) error { return requirePluginDevMode() },
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Name = args[0]
			}

			if opts.Name == "" {
				if !canPrompt(cmd) {
					return createInvalid(fmt.Errorf("<name> is required: %s", strings.TrimSuffix(pluginCreateName.Help, ".")))
				}

				if err := pluginCreateName.Ask(cmd, &opts.Name); err != nil {
					return err
				}
			}

			// A name that is typed goes through the same checks as one that is given.
			if err := scaffold.CheckName(opts.Name); err != nil {
				return createInvalid(err)
			}

			if err := checkPluginNameAvailable(cmd.Root(), opts.Name); err != nil {
				return err
			}

			if err := pluginCreateLang.Select(cmd, &lang, []string{string(scaffold.LangGo), string(scaffold.LangTypeScript)}, nil); err != nil {
				return err
			}

			if err := pluginCreateAuthor.Ask(cmd, &opts.Team, nil); err != nil {
				return err
			}

			if err := pluginCreateBugs.Ask(cmd, &opts.Bugs, nil); err != nil {
				return err
			}

			for _, missing := range []struct {
				flag  Flag
				value string
			}{{pluginCreateLang, lang}, {pluginCreateAuthor, opts.Team}, {pluginCreateBugs, opts.Bugs}} {
				if missing.value == "" {
					return createInvalid(fmt.Errorf("--%s is required: %s", missing.flag.LongForm, strings.TrimSuffix(missing.flag.Help, ".")))
				}
			}

			for i := range scopes {
				scopes[i] = strings.TrimSpace(scopes[i])
			}

			opts.Lang = scaffold.Lang(lang)
			opts.Version = version
			opts.Scopes = scopes
			opts.CLIVersion = buildinfo.Version

			// The values that were not given.
			setDefault(&opts.ID, registry.DefaultRegistryName+"/"+opts.Name)
			setDefault(&opts.Description, pluginCreateDefaultDescription)
			setDefault(&opts.Command, opts.Name)
			setDefault(&opts.CommandDescription, opts.Description)
			setDefault(&opts.Dir, opts.Name)
			setDefault(&opts.License, pluginCreateDefaultLicense)

			templates := scaffold.Embedded()

			if from != "" {
				info, err := os.Stat(from)
				if err == nil && !info.IsDir() {
					err = fmt.Errorf("%s is not a directory", from)
				}

				if err != nil {
					return createInvalid(fmt.Errorf("cannot use --from: %w", err))
				}

				templates = os.DirFS(from)
			}

			files, err := scaffold.Create(opts, templates)
			if err != nil {
				return createFailed(err)
			}

			return sayCreated(cli, opts, files)
		},
	}

	pluginCreateLang.RegisterString(cmd, &lang, "")
	pluginCreateAuthor.RegisterString(cmd, &opts.Team, "")
	pluginCreateBugs.RegisterString(cmd, &opts.Bugs, "")
	pluginCreateID.RegisterString(cmd, &opts.ID, "")
	pluginCreateVersion.RegisterString(cmd, &version, pluginCreateDefaultVersion)
	pluginCreateDescription.RegisterString(cmd, &opts.Description, "")
	pluginCreateCommand.RegisterString(cmd, &opts.Command, "")
	pluginCreateCommandDescription.RegisterString(cmd, &opts.CommandDescription, "")
	pluginCreateDir.RegisterString(cmd, &opts.Dir, "")
	pluginCreateLicense.RegisterString(cmd, &opts.License, "")
	pluginCreateScopes.RegisterStringSlice(cmd, &scopes, []string{pluginCreateDefaultScope})
	pluginCreateFrom.RegisterString(cmd, &from, "")
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	return cmd
}

func linkPluginCmd(cli *cli) *cobra.Command {
	var bin, policy string

	cmd := &cobra.Command{
		Use:   "link [path]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Run a local build of a plugin as an auth0 command",
		Long: "Link the plugin in a directory, so that `auth0 <name>` runs it. The directory must hold a valid plugin.json, and the executable is the file named like the plugin in it, " +
			"with .exe added on Windows, unless you give another with `--bin`. The manifest and the executable are read again on every run, so a rebuild needs no new link.\n\n" +
			"A linked plugin takes the place of an installed plugin with the same name. It is held to the same rules as an installed one, and a person must approve its permissions with `auth0 plugin approve`. " +
			"Linking it again replaces the link.\n\n" +
			"A linked plugin is in no registry, so the Management API calls it may make come from a dev policy: a " + devpolicy.FileName + " next to plugin.json, or the file you give with `--policy`. " +
			"It lists rules in the form of the scope table of the registry, and it is checked when you link. It is not signed and applies to this build only. " +
			"A linked plugin with no dev policy does not run.",
		Example: `  auth0 plugin dev link
  auth0 plugin dev link ./checkmate
  auth0 plugin dev link ./checkmate --bin ./checkmate/dist/checkmate
  auth0 plugin dev link ./checkmate --policy ./checkmate/policy.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requirePluginDevMode(); err != nil {
				return err
			}

			path := "."
			if len(args) == 1 {
				path = args[0]
			}

			dir, err := filepath.Abs(path)
			if err != nil {
				return linkInvalid(err)
			}

			info, err := os.Stat(dir)
			if err != nil {
				return linkInvalid(err)
			}

			if !info.IsDir() {
				return linkInvalid(fmt.Errorf("%s is not a directory. Give the directory that holds %s", dir, devlink.ManifestName))
			}

			manifest, problems, err := checkPluginManifest(cmd.Root(), dir)
			if err != nil {
				return linkInvalid(err)
			}

			if len(problems) > 0 {
				return linkProblems(filepath.Join(dir, devlink.ManifestName), problems)
			}

			if !strings.HasPrefix(manifest.ID, registry.DefaultRegistryName+"/") {
				return linkInvalid(fmt.Errorf("%s names the plugin %s, but the only registry this CLI knows is %q, so the ID must start with %s/", filepath.Join(dir, devlink.ManifestName), manifest.ID, registry.DefaultRegistryName, registry.DefaultRegistryName))
			}

			// The executable is the one the author gave, or the file named like the
			// plugin in the directory of the build.
			if bin == "" {
				bin = devlink.DefaultBin(dir, manifest.Name())
			} else if bin, err = filepath.Abs(bin); err != nil {
				return linkInvalid(err)
			}

			if err := devlink.CheckExecutable(bin); err != nil {
				return linkInvalid(fmt.Errorf("the executable of the plugin cannot be used: %w. Build it there, or give another file with --bin", err))
			}

			// The dev policy is the one that was given, or the file in the directory.
			// Only a file that was given is recorded, so that the file in the
			// directory is found wherever the directory is.
			if policy != "" {
				if policy, err = filepath.Abs(policy); err != nil {
					return linkInvalid(err)
				}
			}

			policyNote, err := checkDevPolicy(dir, policy)
			if err != nil {
				return err
			}

			svc, err := cli.getPlugins()
			if err != nil {
				return err
			}

			links := devlink.Links{Dir: svc.store.Dir}
			if err := links.Add(cmd.Context(), manifest.ID, devlink.Link{Path: dir, Bin: bin, Policy: policy}); err != nil {
				return linksError(err)
			}

			return sayLinked(cmd, cli, svc, manifest, policy, policyNote, display.PluginLinkResult{
				ID:      manifest.ID,
				Version: manifest.Version,
				Path:    dir,
				Bin:     bin,
				Banner:  linkedBanner(manifest.ID, manifest.Version, dir),
			})
		},
	}

	pluginDevBin.RegisterString(cmd, &bin, "")
	pluginDevPolicy.RegisterString(cmd, &policy, "")
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	return cmd
}

// checkDevPolicy checks the dev policy of a build that is being linked, the way a
// run reads it, so that a policy that would be refused is found now. The policy is
// the file given with --policy, as an absolute path, or the one in the directory.
// A file that was given must be there. A directory without one is no failure, and
// the note says that a run needs one and how to make it; the note is empty when
// the policy is fine.
func checkDevPolicy(dir, given string) (string, error) {
	path := devpolicy.PathFor(dir, given)

	file, err := devpolicy.Load(path)
	if errors.Is(err, devpolicy.ErrMissing) && given == "" {
		return fmt.Sprintf("There is no %s in %s, and a linked plugin does not run without one. Create it next to %s with the calls the plugin makes, for example "+
			`{ "rules": [ { "method": "GET", "path": "/api/v2/clients", "anyOf": ["read:clients"] } ] }`+
			", or link again with --policy <file>.", devpolicy.FileName, plugins.SafeText(dir), devlink.ManifestName), nil
	}

	if err != nil {
		return "", linkInvalid(err)
	}

	if _, err := file.Index(); err != nil {
		return "", linkInvalid(err)
	}

	return "", nil
}

// sayLinked tells the person what was linked, what each run prints, and whether
// the plugin still needs an approval, and prints the result for a machine. A
// policy note, when there is one, is a warning about the dev policy.
func sayLinked(cmd *cobra.Command, cli *cli, svc *pluginService, manifest *plugins.Manifest, policy, policyNote string, result display.PluginLinkResult) error {
	op := newPluginOperation(cmd, cli, svc, nil)

	// The approval of a linked build covers its dev policy too, so a person who
	// approved the manifest only is asked again.
	hash, _ := linkedConsent(manifest, result.Path, policy)

	// An approval that cannot be read counts as no approval: the person is asked to
	// approve, and the approval itself reports a damaged file.
	approved := false
	if op.tenant != "" {
		approved, _ = svc.approvals.Approved(op.tenant, manifest.ID, hash)
	}

	result.ApprovalRequired = !approved

	if result.ApprovalRequired && op.tenant != "" {
		result.ApproveCommand = approveCommand(manifest.ID, op.tenant)
	}

	r := cli.renderer
	r.Infof("Linked %s %s from %s. Run it with: auth0 %s", manifest.ID, plugins.SafeText(manifest.Version), plugins.SafeText(result.Path), manifest.Name())
	r.Infof("Every run prints this line on stderr: %s", result.Banner)

	if policyNote != "" {
		r.Warnf("%s", policyNote)
	}

	op.sayApproval(&display.PluginChangeResult{ID: manifest.ID, ApprovalRequired: result.ApprovalRequired, ApproveCommand: result.ApproveCommand})
	r.PluginLinked(result)

	return nil
}

// linkedBanner is the line that every run of a linked plugin prints on stderr.
func linkedBanner(id, version, dir string) string {
	return fmt.Sprintf("Running the linked build of %s %s from %s. Its files are not checked against a release.", id, plugins.SafeText(version), plugins.SafeText(dir))
}

func unlinkPluginCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unlink <plugin>",
		Args:  cobra.ExactArgs(1),
		Short: "Stop running a local build of a plugin",
		Long: "Remove the link of a plugin, so that `auth0 <name>` runs the installed plugin again, or nothing when none is installed. " +
			"The plugin can be given as `name` or `auth0/name`. The build itself is not touched.",
		Example: `  auth0 plugin dev unlink checkmate
  auth0 plugin dev unlink auth0/checkmate`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requirePluginDevMode(); err != nil {
				return err
			}

			ref, err := parsePluginName(args[0], "unlink")
			if err != nil {
				return err
			}

			svc, err := cli.getPlugins()
			if err != nil {
				return err
			}

			links := devlink.Links{Dir: svc.store.Dir}

			err = links.Remove(cmd.Context(), ref.ID)
			if errors.Is(err, devlink.ErrNotLinked) {
				linked, _ := links.Load()

				return notLinkedError(ref, slices.Sorted(maps.Keys(linked)))
			}

			if err != nil {
				return linksError(err)
			}

			cli.renderer.Infof("Unlinked %s.", ref.ID)

			return nil
		},
	}

	return cmd
}

// notLinkedError says that a plugin is not linked, and which plugins are.
func notLinkedError(ref pluginRef, linked []string) error {
	if len(linked) == 0 {
		return validationError{
			err:    fmt.Errorf("the plugin %s is not linked, and no plugin is. Link one with: auth0 plugin dev link <path>", ref.ID),
			reason: "plugin_not_linked",
		}
	}

	return validationError{
		err:    fmt.Errorf("the plugin %s is not linked. Linked plugins: %s", ref.ID, strings.Join(linked, ", ")),
		reason: "plugin_not_linked",
	}
}

func validatePluginCmd(cli *cli) *cobra.Command {
	var bin string

	cmd := &cobra.Command{
		Use:   "validate [path]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Check the manifest of a plugin",
		Long: "Check a plugin.json the way the registry does when a plugin is published, and print every problem with the path of its field. " +
			"The path is a directory that holds plugin.json, or the file itself, and defaults to the current directory. " +
			"It also checks that the name of the plugin is free of core commands. With `--bin`, it checks that the executable can be run too.\n\n" +
			"This works offline. The command fails when the manifest has a problem.",
		Example: `  auth0 plugin dev validate
  auth0 plugin dev validate ./checkmate/plugin.json
  auth0 plugin dev validate ./checkmate --bin ./checkmate/checkmate --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requirePluginDevMode(); err != nil {
				return err
			}

			path := "."
			if len(args) == 1 {
				path = args[0]
			}

			manifest, problems, err := checkPluginManifest(cmd.Root(), path)
			if err != nil {
				return validationError{err: err, reason: "plugin_manifest_invalid"}
			}

			problems = append(problems, checkDevPolicyFile(manifestDir(path), manifest)...)

			if bin != "" {
				if err := devlink.CheckExecutable(bin); err != nil {
					problems = append(problems, display.PluginProblem{Path: "--bin", Message: err.Error()})
				}
			}

			result := display.PluginValidation{Valid: len(problems) == 0, Problems: problems}
			if result.Problems == nil {
				result.Problems = []display.PluginProblem{}
			}

			cli.renderer.PluginValidation(result)

			if !result.Valid {
				return validationError{
					err:     fmt.Errorf("%s has %s", problemSubject(problems), pluralize(len(problems), "problem", "problems")),
					reason:  "plugin_manifest_invalid",
					details: problemDetails(problems),
				}
			}

			return nil
		},
	}

	pluginDevBin.RegisterString(cmd, &bin, "")
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	return cmd
}

// problemSubject says what has the problems: the dev policy when all of them are
// in dev-policy.json, the plugin when some are and some are not, and the plugin
// manifest otherwise.
func problemSubject(problems []display.PluginProblem) string {
	inPolicy := 0

	for _, problem := range problems {
		if problem.Path == devpolicy.FileName {
			inPolicy++
		}
	}

	switch {
	case inPolicy == 0:
		return "the plugin manifest"
	case inPolicy == len(problems):
		return "the dev policy"
	default:
		return "the plugin"
	}
}

// manifestDir returns the directory of the plugin.json at the path, which is a
// directory that holds it or the file itself.
func manifestDir(path string) string {
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return filepath.Dir(path)
	}

	return path
}

// checkDevPolicyFile checks the dev-policy.json in the directory of a manifest, if
// there is one: that it can be used, that it allows every scope the manifest
// declares, and that each of its rules can be used by a scope the manifest declares.
// A directory with no dev policy has no problem here, because a plugin that is not
// meant to be linked needs none. The manifest is nil when it has problems, and then
// only the file itself is checked.
func checkDevPolicyFile(dir string, manifest *plugins.Manifest) []display.PluginProblem {
	file, err := devpolicy.Load(devpolicy.PathFor(dir, ""))
	if errors.Is(err, devpolicy.ErrMissing) {
		return nil
	}

	problem := func(message string) display.PluginProblem {
		return display.PluginProblem{Path: devpolicy.FileName, Message: message}
	}

	if err == nil {
		_, err = file.Index()
	}

	if err != nil {
		return []display.PluginProblem{problem(err.Error())}
	}

	if manifest == nil {
		return nil
	}

	declared := manifest.Permissions().Scopes

	var problems []display.PluginProblem

	for _, scope := range declared {
		allowed := slices.ContainsFunc(file.Rules, func(rule registry.ScopeRule) bool { return slices.Contains(rule.AnyOf, scope) })
		if !allowed {
			problems = append(problems, problem(fmt.Sprintf("scope %s is declared in plugin.json but no rule allows it", plugins.SafeText(scope))))
		}
	}

	for _, rule := range file.Rules {
		used := slices.ContainsFunc(rule.AnyOf, func(scope string) bool { return slices.Contains(declared, scope) })
		if !used {
			problems = append(problems, problem(fmt.Sprintf("rule %s %s allows only scopes that plugin.json does not declare", plugins.SafeText(rule.Method), plugins.SafeText(rule.Path))))
		}
	}

	return problems
}

// checkPluginManifest reads the plugin.json at the path, which is a directory
// that holds it or the file itself, and checks it the way it is checked when a
// plugin is published, that its ID is safe to use as a path, and that its name is
// not one of a core command. It returns the problems it found, and an error when
// the file cannot be read at all. The manifest is nil when it has problems.
func checkPluginManifest(root *cobra.Command, path string) (*plugins.Manifest, []display.PluginProblem, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, err
	}

	if info.IsDir() {
		path = filepath.Join(path, devlink.ManifestName)
	}

	manifest, err := devlink.ReadManifest(path)
	if invalid, ok := errors.AsType[*plugins.ValidationError](err); ok {
		problems := make([]display.PluginProblem, 0, len(invalid.Errors))
		for _, problem := range invalid.Errors {
			problems = append(problems, splitProblem(problem))
		}

		return nil, problems, nil
	}

	if err != nil {
		return nil, nil, err
	}

	// The ID becomes directory names of the store, so a plugin that cannot be
	// installed because of its ID is refused here too.
	err = store.CheckID(manifest.ID)
	if err == nil {
		err = checkPluginNameAvailable(root, manifest.Name())
	}

	if err != nil {
		return nil, []display.PluginProblem{{Path: "id", Message: err.Error()}}, nil
	}

	return manifest, nil, nil
}

// splitProblem splits a problem of a manifest, which starts with the JSON path of
// the field and a colon, into the path and the message. A problem that has no path,
// such as a document that is not JSON, gets the root.
func splitProblem(problem string) display.PluginProblem {
	path, message, found := strings.Cut(problem, ": ")

	// A path has no space unless it quotes a key.
	if !found || (strings.Contains(path, " ") && !strings.Contains(path, `["`)) {
		return display.PluginProblem{Path: "(root)", Message: problem}
	}

	return display.PluginProblem{Path: path, Message: message}
}

// problemDetails returns the problems as the details of the JSON error.
func problemDetails(problems []display.PluginProblem) json.RawMessage {
	details, err := json.Marshal(struct {
		Problems []display.PluginProblem `json:"problems"`
	}{problems})
	if err != nil {
		return nil
	}

	return details
}

// linkInvalid is the failure of a link that cannot be made or used.
func linkInvalid(err error) error {
	return validationError{err: err, reason: "plugin_link_invalid"}
}

// linkProblems is the failure of a link whose manifest has problems.
func linkProblems(manifestPath string, problems []display.PluginProblem) error {
	lines := make([]string, 0, len(problems))
	for _, problem := range problems {
		lines = append(lines, problem.Path+": "+problem.Message)
	}

	return validationError{
		err:     fmt.Errorf("%s is not valid: %s. Check it with: auth0 plugin dev validate %s", manifestPath, strings.Join(lines, "; "), filepath.Dir(manifestPath)),
		reason:  "plugin_link_invalid",
		details: problemDetails(problems),
	}
}

// linksError tells a person what to do about a change of the links that failed.
func linksError(err error) error {
	if errors.Is(err, devlink.ErrDamaged) {
		return linkInvalid(fmt.Errorf("%w. Repair it, or delete it to start again with no links", err))
	}

	return err
}

// devLinks returns the links of the plugins directory. Links count only in dev
// mode, so it returns none, and reads nothing, when dev mode is off.
func devLinks(dir string) (map[string]devlink.Link, error) {
	if !pluginDevMode() {
		return nil, nil //nolint:nilnil // With dev mode off there are no links, and no file is read.
	}

	return devlink.Links{Dir: dir}.Load()
}

// linkedPlugin returns the build of a linked plugin as it is now. It reports false
// when dev mode is off or the plugin is not linked. A file of links that cannot be
// read counts as no links, as it does when the commands are registered, and a
// warning says so. A plugin that is linked but whose link cannot be used is an
// error, and is never read as not being linked.
func linkedPlugin(cli *cli, svc *pluginService, id string) (*devlink.Build, bool, error) {
	links, err := devLinks(svc.store.Dir)
	if err != nil && !cli.wantsJSONError() {
		cli.renderer.Warnf("No plugin is linked: %v", linksError(err))
	}

	link, found := links[id]
	if !found {
		return nil, false, nil
	}

	build, err := link.Open(id)
	if err != nil {
		return nil, true, unusableLink(id, link, err)
	}

	return build, true, nil
}

// unusableLink is the failure of a run or a command that found a link it cannot
// use: it says why, and what to do about it.
func unusableLink(id string, link devlink.Link, err error) error {
	why := err.Error()
	if invalid, ok := errors.AsType[*plugins.ValidationError](err); ok {
		why = strings.Join(invalid.Errors, "; ")
	}

	// The ID is the key of a file on disk, so it is as untrusted as the path.
	id = plugins.SafeText(id)

	return validationError{
		err: fmt.Errorf("the linked plugin %s cannot be used: %s. Check it with: auth0 plugin dev validate %s, or remove the link with: auth0 plugin dev unlink %s",
			id, plugins.SafeText(why), plugins.SafeText(link.Path), plugins.NameOf(id)),
		reason: "plugin_link_invalid",
	}
}

// verifiedLinked describes a linked build the way the checks of a run see an
// installed plugin that was verified. Nothing was verified: the hashes and the
// lockfile are skipped, and the permissions hash is the one that the approval of a
// linked build covers, see linkedHash.
func verifiedLinked(build *devlink.Build) store.Verified {
	hash, _ := linkedHash(build)

	return verifiedLinkedWith(build, hash)
}

// verifiedLinkedWith is verifiedLinked for a hash that the caller made, so that a
// caller that also shows the dev policy hashes the same file that it shows.
func verifiedLinkedWith(build *devlink.Build, hash string) store.Verified {
	return store.Verified{
		ID:              build.Manifest.ID,
		Version:         build.Manifest.Version,
		Dir:             build.Dir,
		Bin:             build.Bin,
		ManifestPath:    build.ManifestPath,
		Manifest:        build.Manifest,
		PermissionsHash: hash,
	}
}

// linkedHash returns the hash that the approval of a linked build covers, and the
// dev policy it was made with. The dev policy decides which Management API calls
// each scope of plugin.json allows, so the hash covers its exact bytes as well as
// the permissions of the manifest. A dev policy that is missing or cannot be read
// gives the hash of the manifest alone and no file: such a build does not run, and
// the run says why.
func linkedHash(build *devlink.Build) (string, *devpolicy.File) {
	return linkedConsent(build.Manifest, build.Dir, build.Policy)
}

// linkedConsent is linkedHash for a manifest, the directory of the build and the
// dev policy that was given for it, if any.
func linkedConsent(manifest *plugins.Manifest, dir, policy string) (string, *devpolicy.File) {
	hash := manifest.PermissionsHash()

	file, err := devpolicy.Load(devpolicy.PathFor(dir, policy))
	if err != nil {
		return hash, nil
	}

	// A policy that the run would refuse is not part of the hash, as it is not when
	// the person approves it.
	if _, err := file.Index(); err != nil {
		return hash, nil
	}

	return devpolicy.ConsentHash(hash, file), file
}

// setDefault sets a value that was not given.
func setDefault(value *string, fallback string) {
	if *value == "" {
		*value = fallback
	}
}

// createInvalid is the failure of an input or a target that cannot be used. The
// text may hold a path or a name from a directory given with --from, so it is
// made safe to print.
func createInvalid(err error) error {
	return validationError{err: errors.New(plugins.SafeText(err.Error())), reason: "plugin_create_invalid"}
}

// createFailed is the failure of scaffold.Create. Only a failure that the person
// can fix by changing what they gave has the reason of an invalid input, and any
// other, such as a full disk, is a plain error that names the path. The text may
// hold a path or a name from a directory given with --from, so it is made safe to
// print.
func createFailed(err error) error {
	var input *scaffold.InputError
	if errors.As(err, &input) {
		return createInvalid(err)
	}

	return errors.New(plugins.SafeText(err.Error()))
}

// sayCreated tells the person what was written and what to do next, and prints
// the result for a machine.
func sayCreated(cli *cli, opts scaffold.Options, files []string) error {
	dir, err := filepath.Abs(opts.Dir)
	if err != nil {
		return createInvalid(err)
	}

	r := cli.renderer

	r.Infof("Created %s in %s (%s).", opts.ID, plugins.SafeText(dir), pluralize(len(files), "file", "files"))

	var warnings []string

	if len(opts.Scopes) == 0 {
		warnings = append(warnings, "The first command declares no scopes, so the Management API call of the sample is denied. Add the scopes it needs under permissions.scopes in plugin.json.")
	}

	for _, warning := range warnings {
		r.Warnf("%s", warning)
	}

	r.Infof("Next:")

	for _, step := range createNextSteps(opts) {
		r.Detailf("%s", step)
	}

	r.Infof("To publish it, push the project to a new GitHub repository: the first push to main releases version %s. Release again with: %s", opts.Version, createReleaseCommand(opts.Lang))

	r.PluginCreated(display.PluginCreateResult{Dir: dir, Files: files, ID: opts.ID, Lang: string(opts.Lang), Warnings: warnings})

	return nil
}

// createNextSteps returns the commands to run after the project is created, in
// order. They are the ones in the README of the project.
func createNextSteps(opts scaffold.Options) []string {
	build := "go mod tidy && go build -o " + opts.Name + " ."
	if opts.Lang == scaffold.LangTypeScript {
		build = "bun install && bun run build"
	}

	run := "auth0 " + opts.Name
	if opts.Command != opts.Name {
		run += " " + opts.Command
	}

	return []string{
		"cd " + shellWord(opts.Dir),
		build,
		pluginDevModeEnv + "=true auth0 plugin dev link",
		"auth0 plugin approve " + opts.Name,
		run,
	}
}

// createReleaseCommand returns the command of the project that releases a new
// version.
func createReleaseCommand(lang scaffold.Lang) string {
	if lang == scaffold.LangTypeScript {
		return "bun run release minor"
	}

	return "make release BUMP=minor"
}

// shellWord returns a path as one word of a shell command line.
func shellWord(path string) string {
	return quoteWord(path, runtime.GOOS == "windows")
}

// quoteWord quotes a path that has a character the shell treats as special. The
// Windows shells do not read single quotes, and a Windows path holds no double
// quote, so it is wrapped in double quotes as it is.
func quoteWord(path string, windows bool) string {
	path = plugins.SafeText(path)
	if !strings.ContainsAny(path, " \t'\"$`\\&;|<>()*?[]{}!#~") {
		return path
	}

	if windows {
		return `"` + path + `"`
	}

	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}
