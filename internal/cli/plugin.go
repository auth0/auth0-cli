package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/buildinfo"
	"github.com/auth0/auth0-cli/internal/config"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/iostream"
	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/approvals"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
	"github.com/auth0/auth0-cli/internal/plugins/store"
	"github.com/auth0/auth0-cli/internal/prompt"
)

// pluginCommandPath is the command path of the plugin group, which every plugin
// command starts with. It is built from the name of the group, never from an
// alias.
const pluginCommandPath = "auth0 plugin"

var pluginUpdateAll = Flag{
	Name:     "All",
	LongForm: "all",
	Help:     "Update every installed plugin that is not pinned.",
}

// registerPluginCmd adds the plugin command to the root when a CLI build with
// the given version has the plugin feature. A stable release carries the code
// but not the feature. Graduating the feature means removing this check.
func registerPluginCmd(rootCmd *cobra.Command, cli *cli, version string) {
	if plugins.FeatureEnabled(version) {
		rootCmd.AddCommand(pluginCmd(cli))
	}
}

func pluginCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "plugin",
		Aliases: []string{"plugins"},
		Short:   "(Experimental) Search, inspect and manage Auth0 CLI plugins",
		Long: "(Experimental) Search the signed Auth0 plugin registry, install, update and remove plugins, and approve what a plugin may do on a tenant." +
			"\n\nPlugin commands do not need you to be logged in." +
			"\n\n**Warning:** This command is experimental and is subject to change in future versions.",
		// The plugin commands stay out of the generated docs until the feature
		// graduates.
		Annotations: map[string]string{skipDocsAnnotation: "true"},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(searchPluginCmd(cli))
	cmd.AddCommand(listPluginCmd(cli))
	cmd.AddCommand(infoPluginCmd(cli))
	cmd.AddCommand(installPluginCmd(cli))
	cmd.AddCommand(updatePluginCmd(cli))
	cmd.AddCommand(uninstallPluginCmd(cli))
	cmd.AddCommand(approvePluginCmd(cli))

	return cmd
}

func searchPluginCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search [query]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Search the plugin registry",
		Long: "Search the signed plugin registry. The query is matched against the plugin ID and description, ignoring case. " +
			"Without a query, every plugin is listed.\n\n" +
			"The registry index is checked for a newer copy when it is due. When the registry cannot be reached, the last verified copy is used.",
		Example: `  auth0 plugin search
  auth0 plugin search security
  auth0 plugin search security --json
  auth0 plugin search --csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := cli.getPlugins()
			if err != nil {
				return err
			}

			index, err := loadPluginIndex(cmd.Context(), cli, svc)
			if err != nil {
				return err
			}

			installed, err := svc.store.List()
			if err != nil {
				return err
			}

			query := ""
			if len(args) == 1 {
				query = strings.ToLower(args[0])
			}

			var results []display.PluginSearchResult

			for _, plugin := range sortedPlugins(index.Plugins) {
				if !strings.Contains(strings.ToLower(plugin.ID), query) && !strings.Contains(strings.ToLower(plugin.Description), query) {
					continue
				}

				view := display.PluginSearchResult{
					ID:          plugin.ID,
					Name:        plugins.NameOf(plugin.ID),
					Version:     plugin.Latest,
					Installed:   isInstalled(installed, plugin.ID),
					Description: plugin.Description,
					Deprecated:  pluginDeprecation(index, plugin.ID, plugin.Latest),
				}

				results = append(results, view)
			}

			cli.renderer.PluginSearchList(results)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	return cmd
}

// pluginDeprecation returns the view of the deprecation of a version of a plugin,
// or nil when the version is not deprecated.
func pluginDeprecation(index *registry.Index, id, version string) *display.PluginDeprecation {
	deprecation, ok := index.Deprecation(id, version)
	if !ok {
		return nil
	}

	return &display.PluginDeprecation{Message: deprecation.Message, Replacement: deprecation.Replacement}
}

// sortedPlugins returns the plugins ordered by ID, without changing the index.
func sortedPlugins(list []registry.Plugin) []registry.Plugin {
	sorted := slices.Clone(list)
	slices.SortFunc(sorted, func(a, b registry.Plugin) int { return strings.Compare(a.ID, b.ID) })

	return sorted
}

func listPluginCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Args:  cobra.NoArgs,
		Short: "List installed plugins",
		Long:  "List the installed plugins with their active version, whether the version is pinned, the version kept for going back, and the space they use on disk.",
		Example: `  auth0 plugin list
  auth0 plugin list --json
  auth0 plugin list --csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := cli.getPlugins()
			if err != nil {
				return err
			}

			installed, err := svc.store.List()
			if err != nil {
				return err
			}

			sizes, err := svc.store.Sizes()
			if err != nil {
				return err
			}

			results := make([]display.PluginListEntry, 0, len(installed))
			for _, plugin := range installed {
				results = append(results, display.PluginListEntry{
					ID:        plugin.ID,
					Name:      plugins.NameOf(plugin.ID),
					Version:   plugin.Active,
					Pinned:    plugin.Pinned,
					Previous:  plugin.Previous,
					SizeBytes: sizes[plugin.ID],
				})
			}

			cli.renderer.PluginList(results)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact", "csv")

	return cmd
}

func infoPluginCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info <plugin>",
		Args:  cobra.ExactArgs(1),
		Short: "Show what a plugin is and what it asks to do",
		Long: "Show the owner, requirements, platforms, permissions and versions of a plugin, whether it is installed, and the tenants where it is approved.\n\n" +
			"The plugin can be given as `name`, `auth0/name` or `name@version`. Without a version, the latest one is shown.",
		Example: `  auth0 plugin info checkmate
  auth0 plugin info auth0/checkmate
  auth0 plugin info checkmate@1.0.0
  auth0 plugin info checkmate --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := parsePluginRef(args[0])
			if err != nil {
				return validationError{err: err, reason: "plugin_reference"}
			}

			svc, err := cli.getPlugins()
			if err != nil {
				return err
			}

			index, err := loadPluginIndex(cmd.Context(), cli, svc)
			if err != nil {
				return err
			}

			view, err := buildPluginInfo(cmd, cli, svc, index, ref)
			if err != nil {
				return err
			}

			cli.renderer.PluginInfo(*view)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	return cmd
}

// buildPluginInfo gathers everything info shows about the plugin of the
// reference: the index, the installed copy and the approvals.
func buildPluginInfo(cmd *cobra.Command, cli *cli, svc *pluginService, index *registry.Index, ref pluginRef) (*display.PluginDetails, error) {
	plugin, ok := index.FindPlugin(ref.ID)
	if !ok {
		return nil, &store.NotAvailableError{ID: ref.ID}
	}

	// A plugin whose versions are in a separate file needs the registry, even when
	// the index came from the cache. There is nothing safe to show without them,
	// so the error says what to run once the registry can be reached.
	records, err := svc.client.Versions(cmd.Context(), plugin)
	if err != nil {
		return nil, withRegistryNextAction(err, "auth0 plugin info "+ref.String())
	}

	sortVersionRecords(records)

	shown := ref.Version
	if shown == "" {
		shown = plugin.Latest
	}

	n := slices.IndexFunc(records, func(r registry.VersionRecord) bool { return r.Version == shown })
	if n < 0 {
		available := make([]string, 0, len(records))
		for _, record := range records {
			available = append(available, record.Version)
		}

		return nil, &store.NotAvailableError{ID: ref.ID, Version: ref.Version, Available: available}
	}

	record := records[n]

	view := &display.PluginDetails{
		ID:          plugin.ID,
		Version:     record.Version,
		Description: plugin.Description,
		Owner:       plugin.Owner,
		Homepage:    plugin.Homepage,
		Platforms:   record.Platforms,
		Permissions: display.PluginPermissions{
			Scopes: len(record.Permissions.Scopes),
			Hosts:  len(record.Permissions.Hosts),
			Files:  len(record.Permissions.Files),
			Env:    len(record.Permissions.Env),
		},
		PermissionsHash: record.PermissionsHash,
		Versions:        make([]display.PluginVersion, 0, len(records)),
		ApprovedTenants: []display.PluginApproval{},
	}

	if record.Engines != nil {
		view.Requires = record.Engines.Auth0CLI
	}

	for _, other := range records {
		_, revoked := index.Revoked(plugin.ID, other.Version)
		view.Versions = append(view.Versions, display.PluginVersion{Version: other.Version, Latest: other.Version == plugin.Latest, Revoked: revoked})
	}

	view.Deprecated = pluginDeprecation(index, plugin.ID, record.Version)

	if view.Installed, err = installedPluginView(cli, svc, plugin.ID); err != nil {
		return nil, err
	}

	approved, err := svc.approvals.List(plugin.ID)
	if err != nil {
		return nil, err
	}

	for _, approval := range approved {
		view.ApprovedTenants = append(view.ApprovedTenants, display.PluginApproval{
			Tenant:          approval.Tenant,
			PermissionsHash: approval.PermissionsHash,
			ApprovedAt:      approval.ApprovedAt,
		})
	}

	return view, nil
}

// installedPluginView describes the installed copy of a plugin, or returns nil
// when it is not installed. A copy that fails verification is reported with the
// reason and a warning, never hidden.
func installedPluginView(cli *cli, svc *pluginService, id string) (*display.PluginInstalled, error) {
	installed, err := svc.store.List()
	if err != nil {
		return nil, err
	}

	entry, ok := findInstalled(installed, id)
	if !ok {
		return nil, nil //nolint:nilnil // A plugin that is not installed has no installed copy to describe.
	}

	verified, err := svc.store.Verify(id)
	if err != nil {
		if !cli.wantsJSONError() {
			cli.renderer.Warnf("The installed copy of %s failed verification: %v", id, err)
			cli.renderer.Warnf("Reinstall it with: auth0 plugin install %s", plugins.NameOf(id))
		}

		return &display.PluginInstalled{Version: entry.Active, Error: err.Error()}, nil
	}

	return &display.PluginInstalled{Version: verified.Version, Verified: true, PermissionsHash: verified.PermissionsHash}, nil
}

// sortVersionRecords orders versions from the highest to the lowest.
func sortVersionRecords(records []registry.VersionRecord) {
	slices.SortStableFunc(records, func(a, b registry.VersionRecord) int {
		left, leftOK := plugins.ParseVersion(a.Version)
		right, rightOK := plugins.ParseVersion(b.Version)

		if leftOK && rightOK {
			return right.Compare(left)
		}

		return strings.Compare(b.Version, a.Version)
	})
}

func installPluginCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <plugin>",
		Args:  cobra.ExactArgs(1),
		Short: "Install a plugin",
		Long: "Install a plugin from the signed registry. The plugin can be given as `name`, `auth0/name` or `name@version`. " +
			"Installing an exact version pins the plugin, so that `auth0 plugin update --all` leaves it alone.\n\n" +
			"In a terminal, the permissions that the plugin asks for are shown first, and the plugin is approved for the tenant when you agree. " +
			"Without a terminal, with `--no-input` and in agent mode, the plugin is installed without an approval, and the command to approve it in a terminal is printed. " +
			"Only a person approves a plugin.\n\n" +
			"Installing needs a current copy of the registry index, so it fails when the registry cannot be reached and the saved copy is out of date.",
		Example: `  auth0 plugin install checkmate
  auth0 plugin install auth0/checkmate
  auth0 plugin install checkmate@1.0.0 --tenant acme.us.auth0.com
  auth0 plugin install checkmate --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := parsePluginRef(args[0])
			if err != nil {
				return validationError{err: err, reason: "plugin_reference"}
			}

			if err := checkPluginNameAvailable(cmd.Root(), ref.Name); err != nil {
				return err
			}

			svc, err := changePluginService(cmd, cli)
			if err != nil {
				return err
			}

			index, err := freshPluginIndex(cmd.Context(), svc, "installing a plugin", "auth0 plugin install "+args[0])
			if err != nil {
				return err
			}

			op := newPluginOperation(cmd, cli, svc, index)

			result, err := svc.store.Install(cmd.Context(), index, ref.ID, ref.Version, store.Options{Consent: op.consent(false)})
			if declinedByPerson(err) {
				cli.renderer.Infof("%s was not approved, so nothing was installed. To review the permissions again, run: auth0 plugin install %s", ref.ID, ref.Name)
				return op.declined(ref.ID)
			}

			if err != nil {
				return err
			}

			view, err := op.finish(cmd.Context(), result)
			if err != nil {
				return err
			}

			op.say(result, view, false)
			cli.renderer.PluginChange(*view)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	return cmd
}

func updatePluginCmd(cli *cli) *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "update [plugin]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update a plugin, or every plugin",
		Long: "Update an installed plugin to the latest version of the registry, and clear its pin. " +
			"With `--all`, every installed plugin is updated, except the ones that are pinned to a version. " +
			"The pinned ones are listed with the newer version that is available.\n\n" +
			"When the new version asks for other permissions than the installed one, you are asked to approve what changed. " +
			"Without a terminal, with `--no-input` and in agent mode, the installed version stays active, and the command says why. " +
			"The update of a named plugin fails then, and `--all` fails when some plugin could not be updated.\n\n" +
			"Updating needs a current copy of the registry index, so it fails when the registry cannot be reached and the saved copy is out of date.",
		Example: `  auth0 plugin update checkmate
  auth0 plugin update --all
  auth0 plugin update --all --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if all == (len(args) == 1) {
				return validationError{err: errors.New("give the name of one plugin, or use --all to update every plugin"), reason: "plugin_update_target"}
			}

			var ref pluginRef

			if len(args) == 1 {
				var err error
				if ref, err = parsePluginName(args[0], "update"); err != nil {
					return err
				}
			}

			svc, err := changePluginService(cmd, cli)
			if err != nil {
				return err
			}

			retry := "auth0 plugin update --all"
			if !all {
				retry = "auth0 plugin update " + ref.Name
			}

			index, err := freshPluginIndex(cmd.Context(), svc, "updating plugins", retry)
			if err != nil {
				return err
			}

			op := newPluginOperation(cmd, cli, svc, index)
			opts := store.Options{Consent: op.consent(true)}

			if all {
				return op.updateAll(cmd.Context(), opts)
			}

			return op.updateOne(cmd.Context(), ref, opts)
		},
	}

	pluginUpdateAll.RegisterBool(cmd, &all, false)
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	return cmd
}

func uninstallPluginCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "uninstall <plugin>",
		Aliases: []string{"remove"},
		Args:    cobra.ExactArgs(1),
		Short:   "Uninstall a plugin",
		Long: "Uninstall a plugin: every version of it, its entry in the lockfile and the approvals on every tenant are removed, and the space that was freed is printed.\n\n" +
			"This works without the registry. To uninstall without being asked to confirm, use the `--force` flag.",
		Example: `  auth0 plugin uninstall checkmate
  auth0 plugin remove checkmate
  auth0 plugin uninstall checkmate --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := parsePluginName(args[0], "uninstall")
			if err != nil {
				return err
			}

			// Uninstalling must work on a machine where plugins are turned off, where
			// the registry cannot be reached and where the approval pins are wrong.
			svc, err := cli.getPlugins()
			if err != nil {
				return err
			}

			if _, err := requireInstalled(svc.store, ref); err != nil {
				return err
			}

			if !cli.force && cli.agentMode {
				return errDestructiveNoConfirm
			}

			if !cli.force && svc.canPrompt(cmd) {
				if !svc.confirm(fmt.Sprintf("Are you sure you want to uninstall %s? This removes every version and its approvals.", ref.ID)) {
					cli.renderer.Infof("%s was not uninstalled.", ref.ID)
					return nil
				}
			}

			freed, err := svc.store.Remove(cmd.Context(), ref.ID)
			if err != nil {
				return err
			}

			cli.renderer.PluginUninstalled(ref.ID, freed)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

func approvePluginCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "approve <plugin>",
		Args:  cobra.ExactArgs(1),
		Short: "Approve what an installed plugin may do on a tenant",
		Long: "Show the permissions of an installed plugin and, when you agree, approve them for a tenant. " +
			"A plugin runs on a tenant only after a person approved its permissions there. " +
			"The approval holds for later versions that ask for the same permissions.\n\n" +
			"This needs a terminal. It cannot run with `--no-input`, in agent mode or in a script, because only a person approves a plugin. " +
			"To approve a plugin in CI, pin its permissions with the AUTH0_CLI_PLUGIN_APPROVALS environment variable instead.",
		Example: `  auth0 plugin approve checkmate --tenant acme.us.auth0.com
  auth0 plugin approve auth0/checkmate`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := parsePluginName(args[0], "approve")
			if err != nil {
				return err
			}

			svc, err := changePluginService(cmd, cli)
			if err != nil {
				return err
			}

			tenant := pluginTenant(cli)
			if tenant == "" {
				return validationError{err: errPluginNoTenant, reason: "plugin_no_tenant"}
			}

			if _, err := requireInstalled(svc.store, ref); err != nil {
				return err
			}

			verified, err := svc.store.Verify(ref.ID)

			if _, failed := errors.AsType[*store.VerificationError](err); failed {
				return fmt.Errorf("the installed copy of %s failed verification, so it cannot be approved: %w. Reinstall it with: auth0 plugin install %s", ref.ID, err, ref.Name)
			}

			if err != nil {
				return err
			}

			cli.renderer.PluginSummary(verified.ID, verified.Version, verified.Manifest.Description)

			op := newPluginOperation(cmd, cli, svc, nil)
			request := approvals.Request{
				ID:          verified.ID,
				Version:     verified.Version,
				New:         verified.Manifest.Permissions(),
				NewCommands: verified.Manifest.ConsentCommands(),
			}

			err = op.consenter.Ask(cmd.Context(), request)

			switch {
			case errors.Is(err, approvals.ErrAgentMode), errors.Is(err, approvals.ErrNotInteractive):
				return fmt.Errorf("only a person can approve a plugin. Run this in a terminal: %s", approveCommand(ref.ID, tenant))
			case errors.Is(err, approvals.ErrDeclined):
				cli.renderer.Infof("%s was not approved for %s.", ref.ID, tenant)
				return nil
			case err != nil:
				return err
			}

			cli.renderer.Infof("Approved %s %s for %s.", verified.ID, verified.Version, tenant)

			return nil
		},
	}

	return cmd
}

// pluginsDisabledEnv turns off the plugin commands that bring plugins onto a
// machine or approve them, for managed machines where plugins are not wanted.
const pluginsDisabledEnv = "AUTH0_CLI_PLUGINS_DISABLED"

// errPluginNoTenant is what a command says when it has no tenant to approve a
// plugin for.
var errPluginNoTenant = errors.New("there is no tenant to approve the plugin for. Pass --tenant <tenant>, or log in with: auth0 login")

// changePluginService returns the plugin service for a command that installs,
// updates or approves a plugin. Those commands fail when plugins are turned off,
// when --tenant contradicts the tenant that env auth mode fixes, when the
// approval pins are not valid, and when the tenant or the approvals file cannot
// be used. All of that is checked before the registry is asked for anything, so
// a command that fails leaves nothing installed. The commands that only read or
// only remove need none of it.
func changePluginService(cmd *cobra.Command, cli *cli) (*pluginService, error) {
	if err := checkPluginsEnabled(cmd); err != nil {
		return nil, err
	}

	if err := checkPluginTenantFlag(cmd, cli); err != nil {
		return nil, err
	}

	svc, err := cli.getPlugins()
	if err != nil {
		return nil, err
	}

	if err := svc.usePins(); err != nil {
		return nil, err
	}

	if err := svc.checkApprovals(pluginTenant(cli)); err != nil {
		return nil, err
	}

	return svc, nil
}

// checkPluginsEnabled returns an error when plugins are turned off by
// AUTH0_CLI_PLUGINS_DISABLED. The variable is read as a boolean. A value that is
// not one turns plugins off as well, because a managed machine that mistyped the
// switch must not end up with plugins on.
func checkPluginsEnabled(cmd *cobra.Command) error {
	value := strings.TrimSpace(os.Getenv(pluginsDisabledEnv))
	if value == "" {
		return nil
	}

	disabled, err := strconv.ParseBool(value)

	switch {
	case err != nil:
		return fmt.Errorf("%s is set to %q, which is not true or false, so plugins are turned off on this machine and %s is not available. Set it to true to keep plugins off, or unset it to allow them. Searching, listing and uninstalling still work",
			pluginsDisabledEnv, value, cmd.CommandPath())
	case disabled:
		return fmt.Errorf("plugins are turned off on this machine by %s=true, so %s is not available. Searching, listing and uninstalling still work", pluginsDisabledEnv, cmd.CommandPath())
	}

	return nil
}

// checkPluginTenantFlag fails when env auth mode fixes the tenant with
// AUTH0_DOMAIN and --tenant names another one, as the commands that call the API
// do, rather than approving a plugin for a tenant that was not meant.
func checkPluginTenantFlag(cmd *cobra.Command, cli *cli) error {
	if !envAuthEnabled(os.Getenv) || !cmd.Flags().Changed("tenant") {
		return nil
	}

	domain := strings.TrimSpace(os.Getenv(envDomain))
	if domain == "" || strings.EqualFold(strings.TrimSpace(cli.tenant), domain) {
		return nil
	}

	return envTenantConflictError(cli.tenant, domain)
}

// parsePluginName reads the name of an installed plugin, which has no version.
func parsePluginName(value, command string) (pluginRef, error) {
	ref, err := parsePluginRef(value)
	if err != nil {
		return pluginRef{}, validationError{err: err, reason: "plugin_reference"}
	}

	if ref.Version != "" {
		return pluginRef{}, validationError{
			err:    fmt.Errorf("%s takes the name of a plugin, without a version. To install an exact version, run: auth0 plugin install %s@%s", command, ref.Name, ref.Version),
			reason: "plugin_reference",
		}
	}

	return ref, nil
}

// freshPluginIndex returns the verified registry index for a command that
// changes what is installed. Such a command needs an index that is current, so
// a saved copy that could not be refreshed is not good enough. The retry
// command is what to run once the registry can be reached.
func freshPluginIndex(ctx context.Context, svc *pluginService, what, retry string) (*registry.Index, error) {
	index, status, err := svc.client.EnsureFresh(ctx)
	if err != nil {
		return nil, withRegistryNextAction(err, retry)
	}

	if status.Stale {
		return nil, fmt.Errorf("%s needs a current registry index, but the registry at %s could not be refreshed: %s. Reconnect and run: %s",
			what, svc.client.Registry.URL, status.Warning, retry)
	}

	return index, nil
}

// pluginTenant returns the tenant a plugin is approved for: the one that was
// asked for with --tenant, else AUTH0_DOMAIN in env auth mode, else the default
// tenant. It is trimmed and in lower case, the form that the config, the
// approvals and the printed commands use. It needs no session, and it is empty
// when there is none of them.
func pluginTenant(cli *cli) string {
	tenant := strings.TrimSpace(cli.tenant)

	switch {
	case tenant != "":
	case envAuthEnabled(os.Getenv):
		tenant = os.Getenv(envDomain)
	default:
		tenant = cli.Config.DefaultTenant
	}

	return strings.ToLower(strings.TrimSpace(tenant))
}

// declinedByPerson reports whether an install or update stopped because the
// person answered no at the consent screen, which is not a failure.
func declinedByPerson(err error) bool {
	declined, ok := errors.AsType[*store.DeclinedError](err)

	return ok && errors.Is(declined.Err, approvals.ErrDeclined)
}

// approveCommand returns the command that approves a plugin on a tenant.
func approveCommand(id, tenant string) string {
	return fmt.Sprintf("auth0 plugin approve %s --tenant %s", id, tenant)
}

// findInstalled returns the installed plugin with the ID.
func findInstalled(installed []store.Installed, id string) (store.Installed, bool) {
	n := slices.IndexFunc(installed, func(i store.Installed) bool { return i.ID == id })
	if n < 0 {
		return store.Installed{}, false
	}

	return installed[n], true
}

// isInstalled reports whether the plugin with the ID is installed.
func isInstalled(installed []store.Installed, id string) bool {
	_, ok := findInstalled(installed, id)

	return ok
}

// requireInstalled returns the installed plugin of the reference, or the error
// that says it is not installed and which plugins are.
func requireInstalled(st *store.Store, ref pluginRef) (store.Installed, error) {
	installed, err := st.List()
	if err != nil {
		return store.Installed{}, err
	}

	entry, ok := findInstalled(installed, ref.ID)
	if !ok {
		return store.Installed{}, notInstalledError(ref, installed)
	}

	return entry, nil
}

// notInstalledError says that a plugin is not installed, and which plugins are.
func notInstalledError(ref pluginRef, installed []store.Installed) error {
	if len(installed) == 0 {
		return validationError{
			err:    fmt.Errorf("the plugin %s is not installed, and no plugin is. Find one with: auth0 plugin search", ref.ID),
			reason: "plugin_not_installed",
		}
	}

	ids := make([]string, 0, len(installed))
	for _, plugin := range installed {
		ids = append(ids, plugin.ID)
	}

	return validationError{
		err:    fmt.Errorf("the plugin %s is not installed. Installed plugins: %s", ref.ID, strings.Join(ids, ", ")),
		reason: "plugin_not_installed",
	}
}

// pluginOperation is one install, update or approval for one tenant. It holds
// the consenter of the operation, which remembers what the person approved until
// the plugins are installed.
type pluginOperation struct {
	cli   *cli
	svc   *pluginService
	index *registry.Index

	// Tenant is the tenant the plugins are approved for, or empty.
	tenant string

	// Asks is true when a person at a terminal can be asked to approve. An agent
	// is never asked, even at a terminal.
	asks bool

	consenter *approvals.Consenter
}

func newPluginOperation(cmd *cobra.Command, cli *cli, svc *pluginService, index *registry.Index) *pluginOperation {
	tenant := pluginTenant(cli)
	interactive := svc.canPrompt(cmd)

	return &pluginOperation{
		cli:    cli,
		svc:    svc,
		index:  index,
		tenant: tenant,
		asks:   interactive && !cli.agentMode,
		consenter: &approvals.Consenter{
			Tenant:    tenant,
			Approvals: svc.approvals,
			Session:   approvals.Session{Interactive: interactive, Agent: cli.agentMode},
			Prompter:  svc.prompter,
			Out:       cli.renderer.MessageWriter,
		},
	}
}

// consent returns the Consent of the install or update. It names the plugin
// before anything else is shown, because the consent screen does not, and checks
// the scopes of the session once the person agreed.
//
// An install of a plugin the person cannot approve now goes ahead without an
// approval, which is recorded by nobody. An update must not: new permissions
// need an approval first, so when the person cannot give it the update fails and
// the installed version stays active.
func (o *pluginOperation) consent(mustApprove bool) store.ConsentFunc {
	var approve store.ConsentFunc

	switch {
	case o.tenant == "" && mustApprove:
		approve = func(context.Context, store.Change) error { return errPluginNoTenant }
	case o.tenant != "" && (o.asks || mustApprove):
		approve = o.consenter.ConsentFunc()
	}

	return func(ctx context.Context, change store.Change) error {
		o.announce(ctx, change)

		if approve != nil {
			if err := approve(ctx, change); err != nil {
				return err
			}
		}

		o.warnScopes(change.New.Scopes)

		return nil
	}
}

// announce prints the plugin line: its ID, version and description, and the size
// of the download for this machine.
func (o *pluginOperation) announce(ctx context.Context, change store.Change) {
	plugin, ok := o.index.FindPlugin(change.ID)
	if !ok {
		return
	}

	o.cli.renderer.PluginSummary(change.ID, change.NewVersion, plugin.Description)

	records, err := o.svc.client.Versions(ctx, plugin)
	if err != nil {
		return
	}

	machine := plugins.CurrentPlatform()

	n := slices.IndexFunc(records, func(r registry.VersionRecord) bool { return r.Version == change.NewVersion })
	if n < 0 {
		return
	}

	for _, artifact := range records[n].Artifacts {
		if built, ok := artifact.Platform(); ok && built == machine {
			o.cli.renderer.PluginDownload(artifact.Size, machine)
		}
	}
}

// warnScopes warns when the session of the tenant may not have the scopes that
// the plugin asks for. It never stops the install and never starts a login.
func (o *pluginOperation) warnScopes(declared []string) {
	if len(declared) == 0 || o.tenant == "" {
		return
	}

	list := strings.Join(declared, ", ")
	tenant, recorded := o.cli.Config.Tenants[o.tenant]

	switch {
	case envAuthEnabled(os.Getenv) || !recorded:
		o.cli.renderer.Warnf("The plugin asks for the scopes %s. The scopes of your session could not be checked, so make sure it has them.", list)
	case tenant.IsAuthenticatedWithClientCredentials():
		o.cli.renderer.Warnf("The plugin asks for the scopes %s. The application you are logged in with needs a grant for them on the Auth0 Management API, or the calls of the plugin are denied.", list)
	default:
		missing := slices.DeleteFunc(slices.Clone(declared), func(scope string) bool { return slices.Contains(tenant.Scopes, scope) })
		if len(missing) == 0 {
			return
		}

		// A login asks for exactly the scopes it is given, so the extra scopes
		// that the session has now must be asked for again.
		request := slices.Concat(tenant.GetExtraRequestedScopes(), missing)
		slices.Sort(request)

		o.cli.renderer.Warnf("The plugin asks for scopes that your login does not have: %s. Its calls that need them are denied until you log in again with: auth0 login --domain %s --scopes %q",
			strings.Join(missing, ", "), o.tenant, strings.Join(request, ","))
	}
}

// finish records the approval the person gave for what was installed, and works
// out whether the plugin still needs one. It is called only after the install
// succeeded.
func (o *pluginOperation) finish(ctx context.Context, result store.Result) (*display.PluginChangeResult, error) {
	if err := o.consenter.Record(ctx, result); err != nil {
		return nil, fmt.Errorf("%s %s was installed, but its approval could not be saved: %w", result.ID, result.Version, err)
	}

	return o.changeView(result)
}

// changeView returns the view of what was installed, with whether the plugin
// still needs an approval.
func (o *pluginOperation) changeView(result store.Result) (*display.PluginChangeResult, error) {
	required, err := o.approvalRequired(result.ID)
	if err != nil {
		return nil, err
	}

	view := &display.PluginChangeResult{
		ID:               result.ID,
		Version:          result.Version,
		PreviousVersion:  result.Previous,
		Action:           string(result.Action),
		Pinned:           result.Pinned,
		ApprovalRequired: required,
	}

	// The command is data for the agent that cannot approve. Without a tenant,
	// there is no command to give.
	if required && o.tenant != "" {
		view.ApproveCommand = approveCommand(result.ID, o.tenant)
	}

	return view, nil
}

// approvalRequired reports whether the active version of the plugin is not
// approved for the tenant, by an approval or by a pin. Without a tenant it is
// always not approved.
func (o *pluginOperation) approvalRequired(id string) (bool, error) {
	if o.tenant == "" {
		return true, nil
	}

	verified, err := o.svc.store.Verify(id)
	if err != nil {
		return false, fmt.Errorf("%s was installed, but it failed verification: %w", id, err)
	}

	approved, err := o.svc.approvals.Approved(o.tenant, id, verified.PermissionsHash)
	if err != nil {
		return false, err
	}

	return !approved, nil
}

// say tells the person what the install or update did, and how to approve the
// plugin when it still needs an approval.
func (o *pluginOperation) say(result store.Result, view *display.PluginChangeResult, updated bool) {
	r := o.cli.renderer

	switch {
	case result.Action == store.ActionUnchanged && updated:
		r.Infof("%s %s is up to date.", result.ID, result.Version)
	case result.Action == store.ActionUnchanged:
		r.Infof("%s %s is already installed.", result.ID, result.Version)
	case result.Previous == "":
		r.Infof("Installed %s %s.", result.ID, result.Version)
	case updated:
		r.Infof("Updated %s from %s to %s.", result.ID, result.Previous, result.Version)
	default:
		r.Infof("Switched %s from %s to %s.", result.ID, result.Previous, result.Version)
	}

	if result.Pinned {
		r.Infof("%s is pinned to %s, so `auth0 plugin update --all` leaves it alone. Move it to the latest version with: auth0 plugin update %s", result.ID, result.Version, plugins.NameOf(result.ID))
	}

	o.sayApproval(view)
}

// sayApproval prints the command that approves the plugin of the view, when it
// needs an approval.
func (o *pluginOperation) sayApproval(view *display.PluginChangeResult) {
	switch {
	case !view.ApprovalRequired:
	case o.tenant == "":
		o.cli.renderer.Infof("%s is not approved on any tenant. Log in with auth0 login, then approve it in a terminal with: auth0 plugin approve %s --tenant <tenant>", view.ID, view.ID)
	default:
		o.cli.renderer.Infof("%s is not approved for %s yet. Approve it in a terminal with: %s", view.ID, o.tenant, view.ApproveCommand)
	}
}

// declined prints the view of an install or update that the person declined, as
// update --all does for a plugin it left alone: the version that stays active,
// which is empty when nothing is installed, and the action declined. The human
// message was printed already, and declining is not a failure.
func (o *pluginOperation) declined(id string) error {
	installed, err := o.svc.store.List()
	if err != nil {
		return err
	}

	view := &display.PluginChangeResult{ID: id, Action: "declined"}

	if entry, ok := findInstalled(installed, id); ok {
		view.Version, view.Pinned = entry.Active, entry.Pinned
	}

	o.cli.renderer.PluginChange(*view)

	return nil
}

// updateOne updates the plugin of the reference.
func (o *pluginOperation) updateOne(ctx context.Context, ref pluginRef, opts store.Options) error {
	installed, err := requireInstalled(o.svc.store, ref)
	if err != nil {
		return err
	}

	result, err := o.svc.store.Update(ctx, o.index, ref.ID, opts)
	if declinedByPerson(err) {
		o.cli.renderer.Infof("%s was not updated, because the new permissions were not approved. %s stays active.", ref.ID, installed.Active)
		return o.declined(ref.ID)
	}

	if err != nil {
		return notUpdatedError(err, installed.Active)
	}

	view, err := o.finish(ctx, result)
	if err != nil {
		return err
	}

	o.say(result, view, true)
	o.cli.renderer.PluginChange(*view)

	return nil
}

// updateAll updates every plugin that is not pinned and reports each of them. It
// fails when some update failed, but not for a pinned plugin or a person who
// declined the new permissions of one.
func (o *pluginOperation) updateAll(ctx context.Context, opts store.Options) error {
	results, err := o.svc.store.UpdateAll(ctx, o.index, opts)

	// Only the plugins that were updated can be approved: never a skipped one,
	// a declined one or one that failed. All of them are recorded in one call,
	// because a call discards the approvals of the plugins that it is not given.
	var done []store.Result

	for _, entry := range results {
		if entry.Err == nil && !entry.Skipped {
			done = append(done, entry.Result)
		}
	}

	recordErr := o.consenter.Record(ctx, done...)

	var (
		views  []display.PluginChangeResult
		failed []string
	)

	for _, entry := range results {
		view, failure := o.reportEntry(entry)
		views = append(views, view)

		if failure {
			failed = append(failed, plugins.NameOf(entry.ID))
		}
	}

	if len(views) == 0 {
		o.cli.renderer.EmptyState("installed plugins", "Find one with: auth0 plugin search")
	} else {
		o.cli.renderer.PluginChanges(views)
	}

	switch {
	case err != nil:
		return err
	case recordErr != nil:
		return fmt.Errorf("the plugins were updated, but their approvals could not be saved: %w", recordErr)
	case len(failed) > 0:
		return fmt.Errorf("%s could not be updated: %s", pluralize(len(failed), "plugin", "plugins"), strings.Join(failed, ", "))
	}

	return nil
}

// reportEntry prints the line of one plugin of update --all and returns its
// result. The second value is true when the update failed.
func (o *pluginOperation) reportEntry(entry store.UpdateResult) (display.PluginChangeResult, bool) {
	r := o.cli.renderer

	view := display.PluginChangeResult{ID: entry.ID, Version: entry.Active}

	var failure error

	switch {
	case entry.Skipped:
		view.Action, view.Pinned, view.Available = "skipped", true, entry.Available

		if entry.Available == "" {
			r.Infof("%s is pinned to %s, which is the newest version.", entry.ID, entry.Active)
		} else {
			r.Infof("%s is pinned to %s and was skipped. Version %s is available: auth0 plugin update %s", entry.ID, entry.Active, entry.Available, plugins.NameOf(entry.ID))
		}
	case declinedByPerson(entry.Err):
		view.Action = "declined"
		r.Infof("%s was not updated, because the new permissions were not approved. %s stays active.", entry.ID, entry.Active)
	case entry.Err != nil:
		failure = notUpdatedError(entry.Err, entry.Active)
	default:
		done, err := o.changeView(entry.Result)
		failure = err

		if err == nil {
			view = *done
			o.say(entry.Result, done, true)
		}
	}

	if failure != nil {
		view.Action, view.Error = "failed", failure.Error()
		r.Errorf("%s: %v", entry.ID, failure)

		return view, true
	}

	return view, false
}

// notUpdatedError says why an update did not happen when the person could not
// approve the new permissions, and leaves every other error as it is.
func notUpdatedError(err error, active string) error {
	declined, ok := errors.AsType[*store.DeclinedError](err)
	if !ok {
		return err
	}

	why := declined.Err.Error()

	switch {
	case errors.Is(declined.Err, approvals.ErrAgentMode):
		why = "an agent cannot approve plugins"
	case errors.Is(declined.Err, approvals.ErrNotInteractive):
		why = "there is no terminal to ask in"
	case errors.Is(declined.Err, errPluginNoTenant):
		why = "there is no tenant to approve them for"
	}

	name := plugins.NameOf(declined.ID)

	return fmt.Errorf("%s %s asks for permissions that are not approved, and %s. %s was not updated, and %s stays active. Run this in a terminal to review them: auth0 plugin update %s",
		name, declined.Version, strings.TrimRight(why, "."), name, active, name)
}

// pluginService is what the plugin commands work with: the registry client, the
// store of installed plugins, the approvals, and the way to ask the person.
type pluginService struct {
	client    *registry.Client
	store     *store.Store
	approvals *approvals.Approvals
	prompter  approvals.Prompter

	// Interactive says whether a person can answer at the terminal of the
	// command. When it is nil, canPrompt decides. Tests set it, because they have
	// no terminal.
	interactive func(cmd *cobra.Command) bool

	// Confirmer asks the person a yes or no question. When it is nil, the prompt
	// of the CLI is used. Tests set it for the same reason.
	confirmer func(message string) bool

	// Login opens the session that a plugin runs with. When it is nil, the
	// authentication of the CLI is used. Tests set it, because they have no tenant
	// to log in to.
	login func(ctx context.Context, c *cli) error

	// Transport carries the calls that the proxy of a plugin makes to the tenant.
	// When it is nil, the default transport is used. Tests set it, because they
	// have no tenant.
	transport http.RoundTripper
}

// canPrompt reports whether a person can answer at the terminal of the command.
func (s *pluginService) canPrompt(cmd *cobra.Command) bool {
	if s.interactive != nil {
		return s.interactive(cmd)
	}

	return canPrompt(cmd)
}

// confirm asks the person to confirm something.
func (s *pluginService) confirm(message string) bool {
	if s.confirmer != nil {
		return s.confirmer(message)
	}

	return prompt.Confirm(message)
}

// usePins reads the approval pins from AUTH0_CLI_PLUGIN_APPROVALS. Only the
// commands that approve, install and update need them, so a value that cannot be
// read fails those and leaves the commands that only read or remove working. It
// is an error and is never ignored.
func (s *pluginService) usePins() error {
	pins, err := approvals.ParsePins(os.Getenv(approvals.PinsEnv))
	if err != nil {
		return validationError{err: err}
	}

	s.approvals.Pins = pins

	return nil
}

// checkApprovals fails when the approvals cannot be used for the tenant: the
// tenant is not a tenant domain, or the approvals file is damaged. It runs before
// anything is downloaded, so an install or update that would fail at the end
// fails at the start and leaves nothing behind. Without a tenant nothing is
// approved, so there is nothing to check.
func (s *pluginService) checkApprovals(tenant string) error {
	if tenant == "" {
		return nil
	}

	return s.approvals.Usable(tenant)
}

// getPlugins returns the plugin service. It is built when a command asks for it,
// not when the CLI starts, so that commands that never ask read no configuration.
// Each call builds a new service, unless one was injected.
func (c *cli) getPlugins() (*pluginService, error) {
	if c.plugins != nil {
		return c.plugins()
	}

	return newPluginService(c)
}

// newPluginService builds the plugin service for the plugins directory next to
// the config file. It does not read the approval pins, see usePins.
func newPluginService(c *cli) (*pluginService, error) {
	// Env auth mode promises not to read the config file, so it is left out and
	// the built-in registry is used.
	if !envAuthEnabled(os.Getenv) {
		if err := c.Config.Initialize(); err != nil && !errors.Is(err, config.ErrConfigFileMissing) {
			return nil, fmt.Errorf("failed to read the config: %w", err)
		}
	}

	configured, err := registry.ResolveRegistries(c.Config.Registries)
	if err != nil {
		return nil, validationError{err: err}
	}

	dir := c.Config.PluginsDir()

	client := registry.NewClient(configured, filepath.Join(dir, "registries", configured.Name))
	approved := &approvals.Approvals{Dir: dir}

	return &pluginService{
		client:    client,
		approvals: approved,
		store:     &store.Store{Dir: dir, CLIVersion: buildinfo.Version, Approvals: approved, Versions: client},
		prompter:  newPluginPrompter(iostream.Input, iostream.Messages),
	}, nil
}

// loadPluginIndex returns the verified index of the registry for a command that
// only reads it. The index is refreshed when it is due, and a copy that could
// not be refreshed is used with a warning, unless the output is for a machine.
// When the registry cannot be used the error says what to run next.
func loadPluginIndex(ctx context.Context, cli *cli, svc *pluginService) (*registry.Index, error) {
	index, status, err := svc.client.EnsureFresh(ctx)
	if err != nil {
		return nil, withRegistryNextAction(err, "auth0 plugin search")
	}

	if status.Warning != "" && !cli.wantsJSONError() {
		cli.renderer.Warnf("%s", status.Warning)
	}

	return index, nil
}

// withRegistryNextAction adds what to do next to the registry errors that
// reconnecting fixes.
func withRegistryNextAction(err error, command string) error {
	if errors.Is(err, registry.ErrExpired) || errors.Is(err, registry.ErrOffline) {
		return fmt.Errorf("%w. Reconnect and run: %s", err, command)
	}

	return err
}

// pluginPrompter reads the answers of the person from one reader, so that
// nothing that was buffered for one question is lost before the next.
type pluginPrompter struct {
	in  *bufio.Reader
	out io.Writer
}

func newPluginPrompter(in io.Reader, out io.Writer) *pluginPrompter {
	return &pluginPrompter{in: bufio.NewReader(in), out: out}
}

// ReadLine writes the prompt and returns the next line without its line break.
// It returns io.EOF when the input ends before anything was typed. A last line
// that has no line break is still an answer.
func (p *pluginPrompter) ReadLine(prompt string) (string, error) {
	if _, err := io.WriteString(p.out, prompt); err != nil {
		return "", err
	}

	line, err := p.in.ReadString('\n')
	if err != nil && (!errors.Is(err, io.EOF) || line == "") {
		return "", err
	}

	return strings.TrimRight(line, "\r\n"), nil
}

// pluginIDPattern is the form of a plugin ID, registry/name. It is a copy of the
// idPattern of the store package, which is unexported, and it is what makes a
// name safe to use as a path. Keep the two in step.
var pluginIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*/[a-z0-9][a-z0-9-]*$`)

// pluginRef is a reference to a plugin, as typed on the command line.
type pluginRef struct {
	// ID is the plugin ID, such as auth0/checkmate.
	ID string

	// Name is the part of the ID after the registry, which is the name of the
	// command that the plugin adds.
	Name string

	// Version is the exact version that was asked for, or empty.
	Version string
}

// String returns the reference the way it is typed in a command: the name, and
// the version after an at sign when there is one.
func (r pluginRef) String() string {
	if r.Version == "" {
		return r.Name
	}

	return r.Name + "@" + r.Version
}

// parsePluginRef reads [registry/]name[@version]. The registry can only be the
// built-in one, the name must be a valid plugin name, and the version must be
// MAJOR.MINOR.PATCH, written the way the registry writes it.
func parsePluginRef(value string) (pluginRef, error) {
	if value == "" {
		return pluginRef{}, errors.New("a plugin name is required, for example: auth0 plugin info checkmate")
	}

	rest, version, hasVersion := strings.Cut(value, "@")

	registryName, name, hasRegistry := strings.Cut(rest, "/")
	if !hasRegistry {
		registryName, name = registry.DefaultRegistryName, rest
	}

	if registryName != registry.DefaultRegistryName {
		return pluginRef{}, fmt.Errorf("%q is not a registry this CLI knows: the only available registry is %q", registryName, registry.DefaultRegistryName)
	}

	id := registryName + "/" + name
	if !pluginIDPattern.MatchString(id) {
		return pluginRef{}, fmt.Errorf("%q is not a valid plugin name: use lowercase letters, digits and hyphens, starting with a letter or digit", name)
	}

	if hasVersion {
		parsed, ok := plugins.ParseVersion(version)
		if !ok || parsed.String() != version {
			return pluginRef{}, fmt.Errorf("%q is not a valid version: use MAJOR.MINOR.PATCH, for example %s@1.0.0", version, name)
		}
	}

	return pluginRef{ID: id, Name: name, Version: version}, nil
}

// checkPluginNameAvailable refuses a plugin name that the running CLI already
// uses for one of its own commands. The names come from the command tree of the
// root at run time, hidden commands and aliases included, plus the help and
// completion commands that Cobra adds itself.
func checkPluginNameAvailable(root *cobra.Command, name string) error {
	reserved := name == "help" || name == "completion"

	for _, command := range root.Commands() {
		// The commands of installed plugins do not count: a plugin can be installed
		// again, and an update replaces it.
		if command.Annotations[pluginCommandAnnotation] == "true" {
			continue
		}

		if command.Name() == name || slices.Contains(command.Aliases, name) {
			reserved = true
		}
	}

	if reserved {
		// The error renderer adds the final period.
		return validationError{
			err:    fmt.Errorf("auth0 already has a command named %q. Plugin names cannot shadow core commands", name),
			reason: "plugin_name_reserved",
		}
	}

	return nil
}
