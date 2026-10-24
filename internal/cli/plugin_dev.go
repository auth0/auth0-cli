package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/devlink"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
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
		Example: `  auth0 plugin dev validate
  auth0 plugin dev link ./checkmate`,
		// The commands for authors show in the help only in dev mode.
		Hidden: !pluginDevMode(),
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(linkPluginCmd(cli))
	cmd.AddCommand(unlinkPluginCmd(cli))
	cmd.AddCommand(validatePluginCmd(cli))

	return cmd
}

func linkPluginCmd(cli *cli) *cobra.Command {
	var bin string

	cmd := &cobra.Command{
		Use:   "link [path]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Run a local build of a plugin as an auth0 command",
		Long: "Link the plugin in a directory, so that `auth0 <name>` runs it. The directory must hold a valid plugin.json, and the executable is the file named like the plugin in it, " +
			"with .exe added on Windows, unless you give another with `--bin`. The manifest and the executable are read again on every run, so a rebuild needs no new link.\n\n" +
			"A linked plugin takes the place of an installed plugin with the same name. It is held to the same rules as an installed one, and a person must approve its permissions with `auth0 plugin approve`. " +
			"Linking it again replaces the link.",
		Example: `  auth0 plugin dev link
  auth0 plugin dev link ./checkmate
  auth0 plugin dev link ./checkmate --bin ./checkmate/dist/checkmate`,
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

			svc, err := cli.getPlugins()
			if err != nil {
				return err
			}

			links := devlink.Links{Dir: svc.store.Dir}
			if err := links.Add(cmd.Context(), manifest.ID, devlink.Link{Path: dir, Bin: bin}); err != nil {
				return linksError(err)
			}

			return sayLinked(cmd, cli, svc, manifest, display.PluginLinkResult{
				ID:      manifest.ID,
				Version: manifest.Version,
				Path:    dir,
				Bin:     bin,
				Banner:  linkedBanner(manifest.ID, manifest.Version, dir),
			})
		},
	}

	pluginDevBin.RegisterString(cmd, &bin, "")
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	return cmd
}

// sayLinked tells the person what was linked, what each run prints, and whether
// the plugin still needs an approval, and prints the result for a machine.
func sayLinked(cmd *cobra.Command, cli *cli, svc *pluginService, manifest *plugins.Manifest, result display.PluginLinkResult) error {
	op := newPluginOperation(cmd, cli, svc, nil)

	// An approval that cannot be read counts as no approval: the person is asked to
	// approve, and the approval itself reports a damaged file.
	approved := false
	if op.tenant != "" {
		approved, _ = svc.approvals.Approved(op.tenant, manifest.ID, manifest.PermissionsHash())
	}

	result.ApprovalRequired = !approved

	if result.ApprovalRequired && op.tenant != "" {
		result.ApproveCommand = approveCommand(manifest.ID, op.tenant)
	}

	r := cli.renderer
	r.Infof("Linked %s %s from %s. Run it with: auth0 %s", manifest.ID, plugins.SafeText(manifest.Version), plugins.SafeText(result.Path), manifest.Name())
	r.Infof("Every run prints this line on stderr: %s", result.Banner)

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

			_, problems, err := checkPluginManifest(cmd.Root(), path)
			if err != nil {
				return validationError{err: err, reason: "plugin_manifest_invalid"}
			}

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
					err:     fmt.Errorf("the plugin manifest has %s", pluralize(len(problems), "problem", "problems")),
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
// lockfile are skipped, and the permissions hash is that of the manifest just read.
func verifiedLinked(build *devlink.Build) store.Verified {
	return store.Verified{
		ID:              build.Manifest.ID,
		Version:         build.Manifest.Version,
		Dir:             build.Dir,
		Bin:             build.Bin,
		ManifestPath:    build.ManifestPath,
		Manifest:        build.Manifest,
		PermissionsHash: build.Manifest.PermissionsHash(),
	}
}
