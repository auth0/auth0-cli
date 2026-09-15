package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"

	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/prompt"
)

// pluginsCmd groups the (POC) plugin framework commands.
func pluginsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugins",
		Args:  cobra.NoArgs,
		Short: "Run external tools as Auth0 CLI plugins",
		Long: "Run external tools as Auth0 CLI plugins that reuse the CLI's active tenant session " +
			"through a local auth-injecting proxy, so the tool never receives the raw access token.",
	}

	cmd.AddCommand(pluginsAvailableCmd(cli))
	cmd.AddCommand(pluginsInstallCmd(cli))
	cmd.AddCommand(pluginsUpdateCmd(cli))
	cmd.AddCommand(pluginsListCmd(cli))
	cmd.AddCommand(pluginsRemoveCmd(cli))

	return cmd
}

// pluginsAvailableCmd lists the plugins published in the signed registry index.
// It needs no tenant session; it only reads the public registry.
func pluginsAvailableCmd(cli *cli) *cobra.Command {
	return &cobra.Command{
		Use:   "available",
		Args:  cobra.NoArgs,
		Short: "List plugins available in the Auth0 CLI plugin registry",
		Long: "List the plugins published in the signed Auth0 CLI plugin registry. " +
			"The registry index is signature-verified before it is read.",
		Example: `  # List every plugin published in the registry
  auth0 plugins available`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			registry, err := plugins.NewRegistry()
			if err != nil {
				return err
			}

			index, err := registry.Fetch(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to load the plugin registry: %w", err)
			}

			if len(index.Plugins) == 0 {
				cli.renderer.Infof("No plugins are published in the registry yet.")
				return nil
			}

			cli.renderer.Heading("available plugins")
			for _, p := range index.Plugins {
				cli.renderer.Infof("%s (%s) — %s", p.Name, p.Install.Version, p.Description)
			}

			return nil
		},
	}
}

// pluginsInstallCmd installs a plugin from the registry and records it locally.
func pluginsInstallCmd(cli *cli) *cobra.Command {
	return &cobra.Command{
		Use:   "install [name]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Install an Auth0 CLI plugin from the registry",
		Long: "Install an Auth0 CLI plugin from the signed registry. npm plugins are run on " +
			"demand through the pinned npx spec; github-release plugins are downloaded and " +
			"checksum-verified for your OS and architecture.\n\n" +
			"Run without a name to pick a plugin from a searchable list of what the registry offers.",
		Example: `  # Pick a plugin to install from a searchable list
  auth0 plugins install

  # Install a specific plugin by name
  auth0 plugins install checkmate`,
		RunE: func(cmd *cobra.Command, args []string) error {
			registry, err := plugins.NewRegistry()
			if err != nil {
				return err
			}

			index, err := registry.Fetch(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to load the plugin registry: %w", err)
			}

			name, err := pluginToInstall(cmd, index, args)
			if err != nil {
				return err
			}

			plugin, ok := index.Plugin(name)
			if !ok {
				return fmt.Errorf("plugin %q is not in the registry; run `auth0 plugins available` to see the list", name)
			}

			if plugin.Install.Type == plugins.InstallNPM {
				if _, err := exec.LookPath("npx"); err != nil {
					return errors.New("npx not found on PATH; this plugin needs Node.js. Install Node.js and try again")
				}
			}

			record, err := plugins.InstallPlugin(cmd.Context(), plugin)
			if err != nil {
				return fmt.Errorf("failed to install plugin %q: %w", name, err)
			}

			store := plugins.NewStore(plugins.DefaultStorePath())
			if err := store.Add(record); err != nil {
				return fmt.Errorf("failed to record installed plugin: %w", err)
			}

			cli.renderer.Infof("Installed %s (%s). Run it with `auth0 %s`.", record.Name, record.Version, record.Name)
			return nil
		},
	}
}

// pluginToInstall resolves which plugin to install. With a name argument it
// returns that name unchanged (the non-interactive path). With no argument and
// an interactive session it presents a searchable list of registry plugins and
// returns the chosen one; in non-interactive mode it errors, since there is
// nothing to select from.
func pluginToInstall(cmd *cobra.Command, index *plugins.Index, args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}

	if !canPrompt(cmd) {
		return "", errors.New("a plugin name is required in non-interactive mode; run `auth0 plugins available` to see the list")
	}

	if len(index.Plugins) == 0 {
		return "", errors.New("no plugins are published in the registry yet")
	}

	// Build a searchable label per plugin and map it back to the plugin name.
	labels := make([]string, 0, len(index.Plugins))
	nameByLabel := make(map[string]string, len(index.Plugins))
	for _, p := range index.Plugins {
		label := fmt.Sprintf("%s (%s) — %s", p.Name, p.Install.Version, p.Description)
		labels = append(labels, label)
		nameByLabel[label] = p.Name
	}

	var selected string
	question := prompt.SelectInput("plugin", "Select a plugin to install:", "Type to filter the list.", labels, labels[0], true)
	if err := prompt.AskOne(question, &selected); err != nil {
		return "", err
	}

	return nameByLabel[selected], nil
}

// pluginsUpdateCmd updates installed plugins to the version published in the
// registry. With no name it updates all installed plugins (or, interactively,
// lets the user pick one); with a name it updates that plugin. By default a
// plugin is only updated when the registry version is newer than what is
// installed; --force reinstalls at the registry version regardless.
func pluginsUpdateCmd(cli *cli) *cobra.Command {
	var all bool
	var force bool

	cmd := &cobra.Command{
		Use:   "update [name]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Update installed Auth0 CLI plugins",
		Long: "Update installed Auth0 CLI plugins to the version published in the signed " +
			"registry. By default a plugin is updated only when the registry offers a newer " +
			"version.\n\n" +
			"Run without a name to update every installed plugin, or pick one from a " +
			"searchable list. Pass --all to update every installed plugin non-interactively, " +
			"or --force to reinstall at the registry version even when a plugin is already " +
			"up to date.",
		Example: `  # Pick an installed plugin to update from a searchable list
  auth0 plugins update

  # Update a specific plugin by name
  auth0 plugins update checkmate

  # Update every installed plugin
  auth0 plugins update --all

  # Reinstall a plugin at the registry version even if it is up to date
  auth0 plugins update checkmate --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if all && len(args) == 1 {
				return errors.New("pass a plugin name or --all, not both")
			}

			store := plugins.NewStore(plugins.DefaultStorePath())
			installed, err := store.List()
			if err != nil {
				return fmt.Errorf("failed to list installed plugins: %w", err)
			}

			targets, err := pluginsToUpdate(cmd, installed, args, all)
			if err != nil {
				return err
			}

			registry, err := plugins.NewRegistry()
			if err != nil {
				return err
			}

			index, err := registry.Fetch(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to load the plugin registry: %w", err)
			}

			updated := 0
			for _, target := range targets {
				plugin, ok := index.Plugin(target.Name)
				if !ok {
					cli.renderer.Warnf("%s: no longer published in the registry, skipping.", target.Name)
					continue
				}

				if !force && !versionIsNewer(plugin.Install.Version, target.Version) {
					cli.renderer.Infof("%s: already up to date (%s).", target.Name, target.Version)
					continue
				}

				record, err := plugins.InstallPlugin(cmd.Context(), plugin)
				if err != nil {
					return fmt.Errorf("failed to update plugin %q: %w", target.Name, err)
				}
				if err := store.Add(record); err != nil {
					return fmt.Errorf("failed to record updated plugin: %w", err)
				}

				if target.Version == record.Version {
					cli.renderer.Infof("%s: reinstalled (%s).", target.Name, record.Version)
				} else {
					cli.renderer.Infof("%s: %s -> %s (updated).", target.Name, target.Version, record.Version)
				}
				updated++
			}

			if updated == 0 {
				cli.renderer.Infof("Nothing to update.")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Update every installed plugin.")
	cmd.Flags().BoolVar(&force, "force", false, "Reinstall at the registry version even if the plugin is already up to date.")

	return cmd
}

// pluginsToUpdate resolves which installed plugins to update. A name argument
// selects that plugin (erroring if it is not installed); --all selects every
// installed plugin; with neither, an interactive session offers a searchable
// picker while a non-interactive one errors.
func pluginsToUpdate(cmd *cobra.Command, installed []plugins.InstalledPlugin, args []string, all bool) ([]plugins.InstalledPlugin, error) {
	if len(installed) == 0 {
		return nil, errors.New("no plugins are installed; run `auth0 plugins available` to see what you can install")
	}

	if len(args) == 1 {
		for _, p := range installed {
			if p.Name == args[0] {
				return []plugins.InstalledPlugin{p}, nil
			}
		}
		return nil, fmt.Errorf("plugin %q is not installed; run `auth0 plugins list` to see what is installed", args[0])
	}

	if all {
		return installed, nil
	}

	if !canPrompt(cmd) {
		return nil, errors.New("a plugin name or --all is required in non-interactive mode; run `auth0 plugins list` to see what is installed")
	}

	// Build a searchable label per plugin and map it back to the record.
	labels := make([]string, 0, len(installed))
	byLabel := make(map[string]plugins.InstalledPlugin, len(installed))
	for _, p := range installed {
		label := fmt.Sprintf("%s (%s) [%s]", p.Name, p.Version, p.InstallType)
		labels = append(labels, label)
		byLabel[label] = p
	}

	var selected string
	question := prompt.SelectInput("plugin", "Select a plugin to update:", "Type to filter the list.", labels, labels[0], true)
	if err := prompt.AskOne(question, &selected); err != nil {
		return nil, err
	}

	return []plugins.InstalledPlugin{byLabel[selected]}, nil
}

// versionIsNewer reports whether the registry version is strictly newer than the
// installed one. When both parse as semver it compares them semantically;
// otherwise it falls back to treating any difference as newer, so a re-tag still
// triggers an update.
func versionIsNewer(registryVersion, installedVersion string) bool {
	rv, iv := "v"+strings.TrimPrefix(registryVersion, "v"), "v"+strings.TrimPrefix(installedVersion, "v")
	if semver.IsValid(rv) && semver.IsValid(iv) {
		return semver.Compare(rv, iv) > 0
	}
	return registryVersion != installedVersion
}

// pluginsListCmd lists the plugins the user has installed.
func pluginsListCmd(cli *cli) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Args:  cobra.NoArgs,
		Short: "List installed Auth0 CLI plugins",
		Long:  "List the Auth0 CLI plugins installed on this machine.",
		Example: `  # List the plugins installed on this machine
  auth0 plugins list`,
		RunE: func(_ *cobra.Command, _ []string) error {
			store := plugins.NewStore(plugins.DefaultStorePath())
			installed, err := store.List()
			if err != nil {
				return fmt.Errorf("failed to list installed plugins: %w", err)
			}

			if len(installed) == 0 {
				cli.renderer.Infof("No plugins installed. Run `auth0 plugins available` to see what you can install.")
				return nil
			}

			cli.renderer.Heading("installed plugins")
			for _, p := range installed {
				cli.renderer.Infof("%s (%s) [%s]", p.Name, p.Version, p.InstallType)
			}

			return nil
		},
	}
}

// pluginsRemoveCmd removes an installed plugin and any downloaded binary.
func pluginsRemoveCmd(cli *cli) *cobra.Command {
	return &cobra.Command{
		Use:   "remove [name]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Remove an installed Auth0 CLI plugin",
		Long: "Remove an installed Auth0 CLI plugin from this machine.\n\n" +
			"Run without a name to pick a plugin to remove from a searchable list of what is installed.",
		Example: `  # Pick an installed plugin to remove from a searchable list
  auth0 plugins remove

  # Remove a specific plugin by name
  auth0 plugins remove checkmate`,
		RunE: func(cmd *cobra.Command, args []string) error {
			store := plugins.NewStore(plugins.DefaultStorePath())

			installed, err := store.List()
			if err != nil {
				return fmt.Errorf("failed to list installed plugins: %w", err)
			}

			name, err := pluginToRemove(cmd, installed, args)
			if err != nil {
				return err
			}

			plugin, ok, err := store.Get(name)
			if err != nil {
				return fmt.Errorf("failed to look up plugin %q: %w", name, err)
			}
			if !ok {
				return fmt.Errorf("plugin %q is not installed", name)
			}

			if plugin.BinaryPath != "" {
				if err := os.RemoveAll(filepath.Dir(plugin.BinaryPath)); err != nil {
					return fmt.Errorf("failed to remove plugin binary: %w", err)
				}
			}

			if err := store.Remove(name); err != nil {
				return fmt.Errorf("failed to remove plugin %q: %w", name, err)
			}

			cli.renderer.Infof("Removed %s.", name)
			return nil
		},
	}
}

// pluginToRemove resolves which installed plugin to remove. With a name argument
// it returns that name unchanged (the non-interactive path). With no argument and
// an interactive session it presents a searchable list of installed plugins and
// returns the chosen one; in non-interactive mode it errors, since there is
// nothing to select from.
func pluginToRemove(cmd *cobra.Command, installed []plugins.InstalledPlugin, args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}

	if !canPrompt(cmd) {
		return "", errors.New("a plugin name is required in non-interactive mode; run `auth0 plugins list` to see what is installed")
	}

	if len(installed) == 0 {
		return "", errors.New("no plugins are installed; run `auth0 plugins available` to see what you can install")
	}

	// Build a searchable label per plugin and map it back to the plugin name.
	labels := make([]string, 0, len(installed))
	nameByLabel := make(map[string]string, len(installed))
	for _, p := range installed {
		label := fmt.Sprintf("%s (%s) [%s]", p.Name, p.Version, p.InstallType)
		labels = append(labels, label)
		nameByLabel[label] = p.Name
	}

	var selected string
	question := prompt.SelectInput("plugin", "Select a plugin to remove:", "Type to filter the list.", labels, labels[0], true)
	if err := prompt.AskOne(question, &selected); err != nil {
		return "", err
	}

	return nameByLabel[selected], nil
}

// runBehindProxy starts the auth-injecting proxy for the active tenant and runs
// the given command with the proxy env contract injected. The first element of
// argv is the command (resolved on PATH) and the rest are its arguments. The
// declaredScopes argument bounds the plugin at the proxy and drives scope-gap
// re-authorization. It backs every dynamically-registered plugin command.
func runBehindProxy(cmd *cobra.Command, cli *cli, argv []string, declaredScopes []string) error {
	if len(argv) == 0 {
		return errors.New("no command to run")
	}

	tenant, err := cli.Config.GetTenant(cli.tenant)
	if err != nil {
		return fmt.Errorf("failed to resolve the active tenant: %w", err)
	}

	// Re-authorize when the plugin declares scopes the active user session lacks.
	// Only device-code (user) sessions can grow their scopes by re-logging in;
	// machine sessions have fixed grant scopes, so the proxy simply bounds them.
	if tenant.IsAuthenticatedWithDeviceCodeFlow() {
		if missing := missingScopes(tenant.Scopes, declaredScopes); len(missing) > 0 {
			if cli.noInput {
				return fmt.Errorf(
					"the %s plugin needs additional scopes (%s) and --no-input is set; run `auth0 login --scopes %s` first",
					argv[0], strings.Join(missing, ", "), strings.Join(missing, ","),
				)
			}
			cli.renderer.Warnf("The %s plugin needs additional scopes (%s). Re-authorizing.", argv[0], strings.Join(missing, ", "))
			tenant, err = RunLoginAsUser(cmd.Context(), cli, missing, tenant.Domain)
			if err != nil {
				return fmt.Errorf("failed to authorize the plugin's required scopes: %w", err)
			}
		}
	}

	if tenant.GetAccessToken() == "" {
		return errors.New("no access token found for the active tenant; run `auth0 login` first")
	}

	provider := &cliTokenProvider{cli: cli, tenantName: tenant.Domain}
	proxy, err := plugins.StartProxy(cmd.Context(), tenant.Domain, provider, declaredScopes)
	if err != nil {
		return fmt.Errorf("failed to start the plugin proxy: %w", err)
	}
	defer proxy.Close()

	binaryPath, err := exec.LookPath(argv[0])
	if err != nil {
		return fmt.Errorf("command not found on PATH: %s", argv[0])
	}

	plugin := exec.CommandContext(cmd.Context(), binaryPath, argv[1:]...)
	plugin.Stdin, plugin.Stdout, plugin.Stderr = os.Stdin, os.Stdout, os.Stderr
	plugin.Env = append(os.Environ(), proxy.Env()...)

	if err := plugin.Run(); err != nil {
		return fmt.Errorf("plugin command failed: %w", err)
	}

	return nil
}

// cliTokenProvider adapts the CLI's tenant session to the proxy's TokenProvider.
// Token reads the current access token; Refresh re-mints it for machine
// (client-credentials) sessions when the Management API returns 401. User
// (device-code) sessions cannot be refreshed silently, so Refresh reports an
// error and the proxy surfaces the original 401.
type cliTokenProvider struct {
	cli        *cli
	tenantName string
}

// Token returns the active tenant's current access token, or an empty string if
// it cannot be read (which the proxy treats as an unauthenticated upstream call).
func (p *cliTokenProvider) Token() string {
	tenant, err := p.cli.Config.GetTenant(p.tenantName)
	if err != nil {
		return ""
	}
	return tenant.GetAccessToken()
}

// Refresh mints a fresh access token for machine sessions and persists it. It
// errors for user sessions, which must be refreshed by re-running `auth0 login`.
func (p *cliTokenProvider) Refresh(ctx context.Context) (string, error) {
	tenant, err := p.cli.Config.GetTenant(p.tenantName)
	if err != nil {
		return "", err
	}

	if tenant.IsAuthenticatedWithDeviceCodeFlow() {
		return "", errors.New("user session token expired; run `auth0 login` to re-authenticate")
	}

	if err := tenant.RegenerateAccessToken(ctx); err != nil {
		return "", err
	}
	if err := p.cli.Config.AddTenant(tenant); err != nil {
		return "", err
	}

	return tenant.GetAccessToken(), nil
}

// missingScopes returns the required scopes not present in the granted set,
// preserving the order they were declared.
func missingScopes(granted, required []string) []string {
	var missing []string
	for _, scope := range required {
		if !slices.Contains(granted, scope) {
			missing = append(missing, scope)
		}
	}
	return missing
}

// pluginInvocation returns the argv used to launch an installed plugin: the
// pinned npx spec for npm plugins, or the downloaded binary path for
// github-release plugins.
func pluginInvocation(p plugins.InstalledPlugin) ([]string, error) {
	switch p.InstallType {
	case plugins.InstallNPM:
		return []string{"npx", "--yes", p.Package}, nil
	case plugins.InstallGitHubRelease:
		return []string{p.BinaryPath}, nil
	default:
		return nil, fmt.Errorf("plugin %q has an unsupported install type %q", p.Name, p.InstallType)
	}
}

// registerInstalledPlugins adds an `auth0 <name>` command for each installed
// plugin, so plugins are invoked as first-class, discoverable subcommands that
// show up in `auth0 --help`. A plugin whose name collides with an existing
// command is skipped. Flag parsing is disabled so every argument is passed
// straight through to the plugin.
func registerInstalledPlugins(rootCmd *cobra.Command, cli *cli) {
	store := plugins.NewStore(plugins.DefaultStorePath())
	installed, err := store.List()
	if err != nil {
		return // A broken store must not prevent the CLI from starting.
	}

	existing := make(map[string]bool)
	for _, c := range rootCmd.Commands() {
		existing[c.Name()] = true
	}

	for _, p := range installed {
		if existing[p.Name] {
			continue
		}

		plugin := p // Capture for the closure.
		rootCmd.AddCommand(&cobra.Command{
			Use:                p.Name,
			Short:              pluginShort(plugin),
			DisableFlagParsing: true,
			RunE: func(cmd *cobra.Command, args []string) error {
				invocation, err := pluginInvocation(plugin)
				if err != nil {
					return err
				}
				// Flag parsing is disabled, so --help is forwarded to the plugin
				// rather than handled by Cobra. Make it clear the help that
				// follows is the plugin's own, not the CLI's.
				if wantsHelp(args) {
					cli.renderer.Infof("Showing %s's own help (it is an Auth0 CLI plugin):", plugin.Name)
				}
				return runBehindProxy(cmd, cli, append(invocation, args...), plugin.RequiredScopes)
			},
		})
	}
}

// pluginShort returns the help text shown for a plugin's `auth0 <name>` command.
// It uses the description carried from the registry, falling back to a generic
// line for plugins installed before descriptions were persisted.
func pluginShort(p plugins.InstalledPlugin) string {
	if p.Description != "" {
		return p.Description
	}
	return fmt.Sprintf("Run the %s plugin", p.Name)
}

// wantsHelp reports whether the plugin arguments request help, so the CLI can
// note that the help output is the plugin's own before forwarding it.
func wantsHelp(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}
