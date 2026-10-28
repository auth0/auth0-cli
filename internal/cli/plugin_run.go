package cli

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/signal"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/buildinfo"
	"github.com/auth0/auth0-cli/internal/config"
	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/approvals"
	"github.com/auth0/auth0-cli/internal/plugins/proxy"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
	"github.com/auth0/auth0-cli/internal/plugins/runner"
	"github.com/auth0/auth0-cli/internal/plugins/store"
)

const (
	// PluginRootAnnotation marks the command of a plugin whose manifest has a root
	// command, so that its usage shows that the plugin can run without a
	// subcommand.
	pluginRootAnnotation = "pluginRoot"

	// PluginCommandAnnotation marks the commands that installed plugins add.
	pluginCommandAnnotation = "plugin"

	// PluginRefreshTimeout bounds the refresh of a stale registry index before a
	// plugin runs, so that a slow network does not hold up a plugin that may run
	// from the saved index.
	pluginRefreshTimeout = 10 * time.Second
)

// pluginOutputFlags are the output flags of the commands. A plugin command can
// support them next to the persistent flags of the root command.
var pluginOutputFlags = [...]string{"json", "json-compact", "csv"}

// registerPluginCommands adds the commands of the installed plugins to the root,
// as auth0 <plugin> and auth0 <plugin> <command>. A CLI build whose version does
// not have the plugin feature adds none.
//
// It runs on every start of the CLI, so it is cheap and never fails: it reads the
// lockfile once and the small manifest of each plugin, checked against the hash in
// the lockfile, and neither hashes the executables nor reaches the network. It
// returns at once when no plugin is installed. A lockfile or manifest that cannot
// be read registers nothing for that plugin, silently, because every other command
// of the CLI must keep working. Whether a plugin may run is decided when it is
// run, see pluginCommand.run.
//
// The core commands win over the plugins. A plugin that has the name of a core
// command or alias is not registered, and the core command says so when it runs,
// see warnShadowedPlugin. The trailing commands are the core commands that are
// added to the root after the plugins.
func registerPluginCommands(rootCmd *cobra.Command, cli *cli, version string, trailing []*cobra.Command) {
	if !plugins.FeatureEnabled(version) {
		return
	}

	installedStore := cli.pluginStore()
	if installedStore == nil {
		return
	}

	manifests, err := installedStore.Manifests()
	if err != nil || len(manifests) == 0 {
		return
	}

	cli.shadowedPlugins = nil

	core := map[string]bool{"help": true, "completion": true}

	for _, command := range slices.Concat(rootCmd.Commands(), trailing) {
		core[command.Name()] = true

		for _, alias := range command.Aliases {
			core[alias] = true
		}
	}

	for _, id := range slices.Sorted(maps.Keys(manifests)) {
		name := plugins.NameOf(id)

		if core[name] {
			if cli.shadowedPlugins == nil {
				cli.shadowedPlugins = map[string]string{}
			}

			cli.shadowedPlugins[name] = id

			continue
		}

		// Two registries could have a plugin of the same name. The first one stays.
		if slices.ContainsFunc(rootCmd.Commands(), func(c *cobra.Command) bool { return c.Name() == name }) {
			continue
		}

		rootCmd.AddCommand(newPluginCommand(cli, rootCmd, id, manifests[id]))
	}
}

// pluginStore returns the store of the installed plugins, for registering their
// commands. It reads no configuration: the plugins directory is a path next to
// the config file, whether or not the config is read, as it is not in env auth
// mode.
func (c *cli) pluginStore() *store.Store {
	if c.plugins != nil {
		svc, err := c.plugins()
		if err != nil {
			return nil
		}

		return svc.store
	}

	dir := c.Config.PluginsDir()

	return &store.Store{Dir: dir, CLIVersion: buildinfo.Version, Approvals: &approvals.Approvals{Dir: dir}}
}

// warnShadowedPlugin warns when the command that runs is a core command that an
// installed plugin has the name of, and says how to remove the plugin. It is
// silent in agent mode and for every other command. The name that counts is the
// one that was typed, so an alias of a core command is covered.
func (c *cli) warnShadowedPlugin(cmd *cobra.Command) {
	if len(c.shadowedPlugins) == 0 {
		return
	}

	top := cmd
	for top.HasParent() && top.Parent() != top.Root() {
		top = top.Parent()
	}

	typed := top.CalledAs()

	id, shadowed := c.shadowedPlugins[typed]
	if !shadowed {
		return
	}

	c.renderer.Warnf("The installed plugin %s has the name of the core command %q, so the core command runs and the plugin is not available. Remove the plugin with: auth0 plugin uninstall %s", id, typed, typed)
}

// pluginCommand is an installed plugin as the command tree sees it.
type pluginCommand struct {
	cli *cli

	id      string
	version string

	// Scanned is what the pre-run hook made of the arguments of the command that
	// runs, for the run that follows it.
	scanned runner.Scanned
}

// newPluginCommand returns the command of a plugin, with a subcommand for each
// command of the manifest that is not the root command.
//
// Cobra does not read the flags of these commands. Everything after the name of
// the command goes to the pre-run hook, which takes out the flags of the CLI and
// leaves the rest for the plugin. The flags that the manifest declares are
// registered for help and completion only, and the CLI never checks them.
func newPluginCommand(cli *cli, rootCmd *cobra.Command, id string, manifest *plugins.Manifest) *cobra.Command {
	p := &pluginCommand{cli: cli, id: id, version: manifest.Version}

	rootSpec := manifest.RootCommand()

	top := &cobra.Command{
		Use:                plugins.NameOf(id),
		Short:              plugins.SafeText(manifest.Description),
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		Annotations:        pluginAnnotations(),
		PersistentPreRunE:  p.preRun,
	}

	top.SetUsageTemplate(pluginUsageTemplate())

	if rootSpec != nil {
		top.Short = plugins.SafeText(rootSpec.Description)
		top.Annotations[pluginRootAnnotation] = "true"
		top.RunE = func(cmd *cobra.Command, _ []string) error { return p.run(cmd, *rootSpec, nil) }

		addPluginFlags(top, cli, rootCmd, *rootSpec)
	} else {
		top.RunE = func(cmd *cobra.Command, _ []string) error { return p.namespace(cmd) }

		addPluginFlags(top, cli, rootCmd, plugins.Command{})
	}

	top.Long = pluginLong(id, manifest, rootSpec)

	for _, spec := range manifest.Commands {
		if rootSpec != nil && spec.Name == rootSpec.Name {
			continue
		}

		sub := &cobra.Command{
			Use:                spec.Name,
			Short:              plugins.SafeText(spec.Description),
			Long:               pluginLong(id, manifest, &spec),
			Args:               cobra.ArbitraryArgs,
			DisableFlagParsing: true,
			Annotations:        pluginAnnotations(),
			RunE:               func(cmd *cobra.Command, _ []string) error { return p.run(cmd, spec, []string{spec.Name}) },
		}

		addPluginFlags(sub, cli, rootCmd, spec)
		top.AddCommand(sub)
	}

	return top
}

// pluginAnnotations returns the annotations of a plugin command: it is a plugin
// command, it stays out of the generated docs while the feature is gated, and the
// root pre-run hook does not log in for it.
func pluginAnnotations() map[string]string {
	return map[string]string{
		pluginCommandAnnotation: "true",
		skipDocsAnnotation:      "true",
		skipAuthAnnotation:      "true",
	}
}

// addPluginFlags registers the flags of a command of the manifest, and the output
// flags, on the cobra command. They show in the help and in completion and are
// used for nothing else.
//
// The output flags that the command does not support are hidden. They are
// registered all the same, so that the pre-run hook sets every output flag the
// same way and the run can then refuse the ones that are not supported.
func addPluginFlags(cmd *cobra.Command, cli *cli, rootCmd *cobra.Command, spec plugins.Command) {
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.Flags().BoolVar(&cli.csv, "csv", false, "Output in csv format.")

	output := plugins.Output{}
	if spec.Output != nil {
		output = *spec.Output
	}

	for name, supported := range map[string]bool{"json": output.JSON, "json-compact": output.JSON, "csv": output.CSV} {
		if !supported {
			_ = cmd.Flags().MarkHidden(name)
		}
	}

	for _, flag := range spec.Flags {
		// The manifest cannot declare a flag of the CLI. This keeps a manifest that
		// slipped through from making pflag panic when the CLI starts.
		if cmd.Flags().Lookup(flag.Name) != nil || rootCmd.PersistentFlags().Lookup(flag.Name) != nil {
			continue
		}

		short := flag.Shorthand
		if len(short) != 1 || short == "h" || cmd.Flags().ShorthandLookup(short) != nil || rootCmd.PersistentFlags().ShorthandLookup(short) != nil {
			short = ""
		}

		description := plugins.SafeText(flag.Description)

		switch flag.Type {
		case plugins.FlagTypeBool:
			cmd.Flags().BoolP(flag.Name, short, false, description)
		case plugins.FlagTypeInt:
			cmd.Flags().IntP(flag.Name, short, 0, description)
		case plugins.FlagTypeFloat:
			cmd.Flags().Float64P(flag.Name, short, 0, description)
		case plugins.FlagTypeStringArray:
			cmd.Flags().StringArrayP(flag.Name, short, nil, description)
		default:
			cmd.Flags().StringP(flag.Name, short, "", description)
		}
	}
}

// pluginUsageTemplate is the usage of a plugin command. The usage template of the
// root assumes core commands, which have an operation after the resource.
func pluginUsageTemplate() string {
	return fmt.Sprintf(`%s{{if .HasAvailableSubCommands}}{{if eq (index .Annotations "pluginRoot") "true"}}
  {{.UseLine}}{{end}}
  {{.CommandPath}} <command> [flags]{{else}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}

%s{{range .Commands}}{{if .IsAvailableCommand}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

%s
{{WrappedLocalFlagUsages . | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

%s
{{WrappedInheritedFlagUsages . | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} <command> --help" for more information about a command.{{end}}
`,
		ansi.Bold("Usage:"),
		ansi.Bold("Available Commands:"),
		ansi.Bold("Flags:"),
		ansi.Bold("Global Flags:"),
	)
}

// pluginLong returns the long help of a command of a plugin, from its manifest:
// what it does, what it may do, how long it may run, and how its arguments are
// read. The command is nil for the top command of a plugin that has no root
// command, which only groups the others.
func pluginLong(id string, manifest *plugins.Manifest, command *plugins.Command) string {
	var text strings.Builder

	if command == nil {
		text.WriteString(plugins.SafeText(manifest.Description))
		text.WriteString("\n\nThis plugin has no command of its own. Run one of the commands below.")
	} else {
		text.WriteString(plugins.SafeText(command.Description))
		text.WriteString("\n\n")
		text.WriteString(pluginFacts(command))
	}

	fmt.Fprintf(&text, "\n\nThis command comes from the plugin %s %s. It runs only on a tenant where you approved the permissions of the plugin: see them and approve them with `auth0 plugin approve %s`.", id, manifest.Version, plugins.NameOf(id))

	text.WriteString("\n\nThe CLI reads its own flags (--tenant, --debug, --no-input, --no-color, --agent-mode, --json, --json-compact, --csv and --help) wherever they are, " +
		"and passes every other argument to the plugin as it was typed, without checking the flags of the plugin. " +
		"Everything after a bare `--` goes to the plugin untouched. " +
		"Type the name of a subcommand first, directly after the name of the plugin. When a flag comes first, the CLI may not find the subcommand. A call of the command that has the name of the plugin with the name of a subcommand among its arguments is then refused, and so is any call of a plugin that has no such command.")

	return text.String()
}

// pluginFacts describes what a command of a plugin may do, in lines.
func pluginFacts(command *plugins.Command) string {
	scopes := "none, so it cannot call the Management API"

	if command.Permissions != nil && len(command.Permissions.Scopes) > 0 {
		scopes = plugins.SafeText(strings.Join(plugins.NormalizePermissions(*command.Permissions).Scopes, ", "))
	}

	timeout := "the default of the registry"
	if command.Timeout != "" {
		timeout = plugins.SafeText(command.Timeout)
	}

	lines := []string{"Management API scopes: " + scopes, "Timeout: " + timeout}

	if command.Interactive {
		lines = append(lines, "Interactive: it needs a terminal, and does not run with --no-input or in agent mode.")
	}

	if output := command.Output; output != nil {
		var supported []string

		if output.JSON {
			supported = append(supported, "--json", "--json-compact")
		}

		if output.CSV {
			supported = append(supported, "--csv")
		}

		if output.AgentMode {
			supported = append(supported, "agent mode")
		}

		if len(supported) > 0 {
			lines = append(lines, "Output: "+strings.Join(supported, ", "))
		}
	}

	return strings.Join(lines, "\n")
}

// ownedFlags returns the flags that the CLI keeps for itself and takes out of
// the arguments of a plugin: the persistent flags of the root, which are a bool
// or a string, and the output flags. The list is read from the root command, so
// it follows the flags the CLI has.
func ownedFlags(root *cobra.Command) []runner.OwnedFlag {
	var owned []runner.OwnedFlag

	root.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
		if kind := flag.Value.Type(); kind == "bool" || kind == "string" {
			owned = append(owned, runner.OwnedFlag{Name: flag.Name, Short: flag.Shorthand, Bool: kind == "bool"})
		}
	})

	for _, name := range pluginOutputFlags {
		owned = append(owned, runner.OwnedFlag{Name: name, Bool: true})
	}

	return owned
}

// preRun is the pre-run hook of every command of a plugin. Cobra did not read
// the flags, so it takes the flags of the CLI out of the arguments and applies
// them as cobra would have, and then runs the hook of the root, which does not
// log in for these commands.
func (p *pluginCommand) preRun(cmd *cobra.Command, args []string) error {
	scanned, err := runner.Scan(args, ownedFlags(cmd.Root()))
	if err != nil {
		return usageError{err: err}
	}

	p.scanned = scanned

	for name, value := range scanned.Values {
		flags := cmd.Flags()
		if cmd.Root().PersistentFlags().Lookup(name) != nil {
			flags = cmd.Root().PersistentFlags()
		}

		if err := flags.Set(name, value); err != nil {
			return usageError{err: err}
		}
	}

	_, p.cli.tenantExplicit = scanned.Values["tenant"]

	return cmd.Root().PersistentPreRunE(cmd, args)
}

// checkSubcommandOrder refuses a call of the root command of a plugin that has
// the name of one of its subcommands among its arguments before a bare double
// dash. Cobra looks for the subcommand after the flags it knows. An unknown flag
// takes the next word as its value, so in "--flag scan" the word "scan" is not
// found as a subcommand and the root command runs with it, and a plugin could
// then run its scan command with the scopes of the root command. The call is
// refused, as it is not clear which command was meant.
func (p *pluginCommand) checkSubcommandOrder(cmd *cobra.Command) error {
	for _, token := range p.scanned.Rest {
		if token == "--" {
			return nil
		}

		if !slices.ContainsFunc(cmd.Commands(), func(sub *cobra.Command) bool { return sub.Name() == token }) {
			continue
		}

		return usageError{
			err: fmt.Errorf("%q is a command of %s, but it comes after a flag, so it cannot be told from the value of that flag. "+
				"Type the command first (%s %s ...), write the value of the flag as --flag=value, or put the argument after --",
				token, cmd.CommandPath(), cmd.CommandPath(), token),
			reason: "plugin_ambiguous_subcommand",
		}
	}

	return nil
}

// namespace is what the top command of a plugin without a root command does: it
// shows its help, or rejects an argument the way a group of core commands does.
func (p *pluginCommand) namespace(cmd *cobra.Command) error {
	if p.scanned.Help || len(p.scanned.Rest) == 0 {
		return cmd.Help()
	}

	token := p.scanned.Rest[0]

	if strings.HasPrefix(token, "-") {
		return usageError{err: fmt.Errorf("unknown flag %q for %q", token, cmd.CommandPath())}
	}

	return unknownCommandError{token: token, parent: cmd.CommandPath(), suggestions: commandSuggestions(cmd, p.scanned.Rest)}
}

// run runs a command of the plugin. The checks come first, and every one of them
// fails closed: the plugin is run only when everything holds, in this order.
//
//  1. Plugins are not turned off on this machine.
//  2. The installed files are the ones that were installed (Store.Verify).
//  3. The registry index is current enough, and does not revoke this version.
//  4. The command can run the way it was called: a terminal when it is
//     interactive, and the output it was asked for.
//  5. The tenant approved the permissions of the plugin.
//  6. Only then a session is opened, which can log in or refresh a token, and it
//     must be for the tenant that approved.
//
// The plugin gets the scopes of the command that was called, not of the whole
// plugin. Verify and the start of the process cannot be one step: the files are
// hashed, and the runner then starts what is at the path. A process of the same
// user that can write there in between could swap the file, which the
// permissions that Verify checks make hard but do not rule out. Verify is
// repeated after the session is open, as that can take as long as a person needs
// to log in.
func (p *pluginCommand) run(cmd *cobra.Command, spec plugins.Command, lead []string) error {
	if p.scanned.Help {
		return cmd.Help()
	}

	if lead == nil {
		if err := p.checkSubcommandOrder(cmd); err != nil {
			return err
		}
	}

	c := p.cli
	ctx := cmd.Context()

	if err := checkPluginsEnabled(cmd); err != nil {
		return err
	}

	if err := checkPluginTenantFlag(cmd, c); err != nil {
		return err
	}

	svc, err := c.getPlugins()
	if err != nil {
		return err
	}

	if err := svc.usePins(); err != nil {
		return err
	}

	verified, err := p.verify(svc)
	if err != nil {
		return err
	}

	command, ok := commandOf(verified.Manifest, spec.Name)
	if !ok {
		return validationError{
			err:    fmt.Errorf("%s %s has no command %q any more. Run it again", p.id, verified.Version, spec.Name),
			reason: "plugin_command_missing",
		}
	}

	index, err := p.loadIndex(ctx, svc, verified.Version)
	if err != nil {
		return err
	}

	if revocation, revoked := index.Revoked(p.id, verified.Version); revoked {
		return validationError{
			err: fmt.Errorf("%s %s was revoked: %s. Run: auth0 plugin update %s",
				plugins.NameOf(p.id), verified.Version, strings.TrimRight(plugins.SafeText(revocation.Reason), ". "), plugins.NameOf(p.id)),
			reason: "plugin_revoked",
		}
	}

	if err := p.checkCall(cmd, svc, command); err != nil {
		return err
	}

	tenant := pluginTenant(c)

	if err := p.checkApproval(svc, tenant, verified); err != nil {
		return err
	}

	// The timeout is read again from the second check, below. Reading it here
	// refuses a command whose timeout is not usable before anyone logs in.
	if _, err := pluginTimeout(command, index); err != nil {
		return err
	}

	login := svc.login
	if login == nil {
		login = func(ctx context.Context, c *cli) error { return c.setupWithAuthentication(ctx) }
	}

	if err := login(ctx, c); err != nil {
		return err
	}

	if !strings.EqualFold(c.sessionDomain, tenant) {
		return authError{
			err:    fmt.Errorf("the session is for %s, but %s is approved for %s. Use --tenant %s to run it there, or approve it for %s with: %s", c.sessionDomain, plugins.NameOf(p.id), tenant, tenant, c.sessionDomain, approveCommand(p.id, c.sessionDomain)),
			reason: "plugin_tenant_mismatch",
		}
	}

	session, err := c.newPluginSession()
	if err != nil {
		return err
	}

	again, err := p.verify(svc)
	if err != nil {
		return err
	}

	if again.Version != verified.Version || again.PermissionsHash != verified.PermissionsHash ||
		again.Bin != verified.Bin || again.Dir != verified.Dir || again.ManifestPath != verified.ManifestPath {
		return validationError{err: fmt.Errorf("%s changed while the session was opened. Run it again", p.id), reason: "plugin_changed"}
	}

	// What runs is what the second check found, so the command is taken from its
	// manifest and not from the first one.
	command, ok = commandOf(again.Manifest, spec.Name)
	if !ok {
		return validationError{
			err:    fmt.Errorf("%s %s has no command %q any more. Run it again", p.id, again.Version, spec.Name),
			reason: "plugin_command_missing",
		}
	}

	if err := p.checkCall(cmd, svc, command); err != nil {
		return err
	}

	timeout, err := pluginTimeout(command, index)
	if err != nil {
		return err
	}

	policy, err := proxy.NewPolicy(index)
	if err != nil {
		return err
	}

	// From here on an interrupt stops the plugin and the proxy before the CLI exits.
	release := holdInterrupt(ctx)
	defer release()

	// SIGHUP, which a terminal sends when it is closed, stops a plugin that runs in
	// its own process group, as it would not get the signal from the terminal.
	// A second Ctrl-C does not wait for this: the CLI exits at once, and a plugin
	// that runs in its own process group can be left running.
	runCtx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()

	var scopes []string
	if command.Permissions != nil {
		scopes = command.Permissions.Scopes
	}

	gate, err := proxy.Start(runCtx, proxy.Config{PluginID: p.id, Domain: tenant, Tokens: session, Policy: policy, Scopes: scopes, Transport: svc.transport})
	if err != nil {
		return err
	}

	defer gate.Close()

	err = runner.Run(runCtx, runner.Spec{
		Bin:         again.Bin,
		Args:        slices.Concat(lead, p.scanned.Rest),
		Env:         environment(cmd, command, tenant, gate.Env()),
		Timeout:     timeout,
		Stdin:       cmd.InOrStdin(),
		Stdout:      cmd.OutOrStdout(),
		Stderr:      cmd.ErrOrStderr(),
		Interactive: command.Interactive,
	})

	if summary := gate.Summary(); summary != "" {
		c.renderer.Warnf("%s", summary)
	}

	if err != nil {
		return p.failure(err)
	}

	return nil
}

// verify checks the installed files of the plugin against the lockfile.
func (p *pluginCommand) verify(svc *pluginService) (store.Verified, error) {
	verified, err := svc.store.Verify(p.id)
	if err == nil {
		return verified, nil
	}

	if _, failed := errors.AsType[*store.VerificationError](err); failed {
		return store.Verified{}, validationError{err: err, reason: "plugin_verification_failed"}
	}

	return store.Verified{}, err
}

// commandOf returns the command of the manifest with the given name.
func commandOf(manifest *plugins.Manifest, name string) (plugins.Command, bool) {
	n := slices.IndexFunc(manifest.Commands, func(c plugins.Command) bool { return c.Name == name })
	if n < 0 {
		return plugins.Command{}, false
	}

	return manifest.Commands[n], true
}

// loadIndex returns the registry index that the plugin runs against. A plugin
// runs only with an index that was verified and has not expired, and whose age is
// within the grace of its policy. An index that is due for a refresh is
// refreshed for a few seconds, and when that fails the saved copy is used with a
// warning, as long as it is still good enough.
func (p *pluginCommand) loadIndex(ctx context.Context, svc *pluginService, version string) (*registry.Index, error) {
	unusable := func(why error) error {
		return validationError{
			err:    fmt.Errorf("%s %s was not run, because the registry index cannot be trusted: %w. Reconnect and run: auth0 plugin search", plugins.NameOf(p.id), version, why),
			reason: "plugin_registry_unavailable",
		}
	}

	index, status, err := svc.client.Load(ctx)

	if err == nil && status.Stale {
		fresh, cancel := context.WithTimeout(ctx, pluginRefreshTimeout)
		index, status, err = svc.client.EnsureFresh(fresh)

		cancel()
	}

	if err != nil {
		return nil, unusable(err)
	}

	if !status.Runnable {
		return nil, unusable(errors.New(strings.TrimRight(status.Warning, ". ")))
	}

	if status.Warning != "" && !p.cli.wantsJSONError() {
		p.cli.renderer.Warnf("%s", status.Warning)
	}

	return index, nil
}

// checkCall refuses a command that cannot run the way it was called: an
// interactive command needs a person at a terminal, and the output flags and
// agent mode need the matching declaration in the manifest.
func (p *pluginCommand) checkCall(cmd *cobra.Command, svc *pluginService, command plugins.Command) error {
	c := p.cli
	name := cmd.CommandPath()

	if command.Interactive && (c.agentMode || c.noInput || !svc.canPrompt(cmd)) {
		return usageError{
			err:    fmt.Errorf("%s is interactive, so it needs a terminal. It cannot run in agent mode, with --no-input, or when input or output is not a terminal", name),
			reason: "plugin_interactive",
		}
	}

	output := plugins.Output{}
	if command.Output != nil {
		output = *command.Output
	}

	unsupported := func(flag, declaration string) error {
		return usageError{
			err:    fmt.Errorf("%s does not support %s, because its manifest does not declare %s", name, flag, declaration),
			reason: "plugin_output_unsupported",
		}
	}

	for _, flag := range []struct {
		name        string
		declaration string
		supported   bool
	}{
		{"json", "output.json", output.JSON},
		{"json-compact", "output.json", output.JSON},
		{"csv", "output.csv", output.CSV},
	} {
		if p.scanned.Values[flag.name] == "true" && !flag.supported {
			return unsupported("--"+flag.name, flag.declaration)
		}
	}

	if c.agentMode && !output.AgentMode {
		return unsupported("agent mode (turn it off with --agent-mode=false)", "output.agentMode")
	}

	return nil
}

// checkApproval fails unless the permissions of the plugin are approved for the
// tenant, by a recorded approval or by a pin. A damaged approvals file is an
// error and is never read as no approvals.
func (p *pluginCommand) checkApproval(svc *pluginService, tenant string, verified store.Verified) error {
	if tenant == "" {
		return validationError{err: errPluginNoTenant, reason: "plugin_no_tenant"}
	}

	err := svc.approvals.Check(tenant, p.id, verified.PermissionsHash)

	if _, notApproved := errors.AsType[*approvals.NotApprovedError](err); notApproved {
		return validationError{err: err, reason: "plugin_not_approved"}
	}

	if errors.Is(err, approvals.ErrDamaged) {
		return validationError{err: err, reason: "plugin_approvals_damaged"}
	}

	if err != nil {
		return validationError{err: err, reason: "plugin_approval_check"}
	}

	return nil
}

// pluginTimeout returns how long the command may run: its own timeout, or the
// default of the registry. A value that is not a timeout is an error rather than
// no limit.
func pluginTimeout(command plugins.Command, index *registry.Index) (time.Duration, error) {
	value := command.Timeout
	if value == "" {
		value = index.Policy.WithDefaults().DefaultTimeout
	}

	if err := plugins.CheckTimeout(value); err != nil {
		return 0, validationError{err: fmt.Errorf("the timeout of %s is not usable: %w", command.Name, err), reason: "plugin_timeout_invalid"}
	}

	timeout, _ := time.ParseDuration(value)

	return timeout, nil
}

// environment returns the environment of the plugin, and nothing else is
// inherited: a short allowlist of the environment of the user, the variables
// that the command declared, the proxy, and the CLI-owned flags. The variables of
// the CLI itself, which start with AUTH0_, are never passed on, even when the
// command declared them. The loopback addresses are always exempt from the proxy
// of the user, so a plugin behind a corporate proxy still reaches the proxy of the
// CLI directly. The environment of the proxy of the CLI is proxyEnv.
func environment(cmd *cobra.Command, command plugins.Command, tenant string, proxyEnv []string) []string {
	var declared []string
	if command.Permissions != nil {
		for _, variable := range command.Permissions.Env {
			declared = append(declared, variable.Name)
		}
	}

	owned := map[string]string{}

	for _, flag := range ownedFlags(cmd.Root()) {
		if found := cmd.Flags().Lookup(flag.Name); found != nil {
			owned[flag.Name] = found.Value.String()
		}
	}

	owned["tenant"] = tenant

	return runner.Env(runtime.GOOS, runner.EnvSources{
		Base:           runner.BaseEnv(os.Getenv, runtime.GOOS),
		Declared:       runner.DeclaredEnv(declared, os.Getenv),
		Proxy:          proxyEnv,
		Plugin:         runner.PluginVars(owned),
		ExemptLoopback: true,
	})
}

// failure turns how the plugin ended into the error of the command. The plugin
// did not end well: err is not nil.
func (p *pluginCommand) failure(err error) error {
	name := plugins.NameOf(p.id)
	failed := pluginError{err: err, pluginID: p.id, version: p.version}

	if exit, ok := errors.AsType[*runner.ExitError](err); ok {
		failed.reason, failed.exitCode = "plugin_exit", exit.Code
		failed.err = fmt.Errorf("%s %s exited with code %d", name, p.version, exit.Code)

		return failed
	}

	if timeout, ok := errors.AsType[*runner.TimeoutError](err); ok {
		failed.reason = "plugin_timeout"
		failed.err = fmt.Errorf("%s %s did not finish within %s and was stopped", name, p.version, timeout.After)

		return failed
	}

	if _, ok := errors.AsType[*runner.InterruptedError](err); ok {
		failed.reason = "plugin_interrupted"
		failed.err = fmt.Errorf("%s %s was interrupted", name, p.version)

		return failed
	}

	failed.reason = "plugin_start_failed"
	failed.err = fmt.Errorf("%s %s could not be run: %w", name, p.version, err)

	return failed
}

// pluginSession is the session of a plugin run as the proxy sees it. It holds the
// access token that setupWithAuthentication resolved, and can renew it where the
// session allows. The token is never printed or logged.
type pluginSession struct {
	mu    sync.RWMutex
	token string

	// Renew gets a new token, or is nil when the session cannot be renewed
	// without the user.
	renew func(ctx context.Context) (string, error)

	// Renewing lets one renewal run at a time, and lets a caller give up on
	// waiting for it when its context ends.
	renewing chan struct{}
}

// newPluginSession returns the session that setupWithAuthentication opened. A
// session logged in with a user, and a token given in AUTH0_API_TOKEN, cannot be
// renewed silently. A client credentials login, saved or from the environment,
// can.
func (c *cli) newPluginSession() (*pluginSession, error) {
	if c.accessToken == "" {
		return nil, authError{err: errors.New("there is no session to run the plugin with. Run: auth0 login"), reason: "plugin_session_unavailable"}
	}

	session := &pluginSession{token: c.accessToken, renewing: make(chan struct{}, 1)}

	domain := c.sessionDomain

	if envAuthEnabled(os.Getenv) {
		apiToken := strings.TrimSpace(os.Getenv(envAPIToken))
		clientID := strings.TrimSpace(os.Getenv(envClientID))
		clientSecret := strings.TrimSpace(os.Getenv(envClientSecret))

		if apiToken == "" && clientID != "" && clientSecret != "" {
			session.renew = func(ctx context.Context) (string, error) {
				return c.resolveEnvAccessToken(ctx, domain, "", clientID, clientSecret)
			}
		}

		return session, nil
	}

	tenant, err := c.Config.GetTenant(domain)
	if err != nil {
		return nil, authError{err: err, reason: "plugin_session_unavailable"}
	}

	if tenant.IsAuthenticatedWithClientCredentials() {
		session.renew = c.savedCredentialsRenewal(tenant, func(ctx context.Context, tenant *config.Tenant) error {
			return tenant.RegenerateAccessToken(ctx)
		})
	}

	return session, nil
}

// savedCredentialsRenewal returns how to renew the token of a tenant that is
// logged in with client credentials: it gets a new token with the saved secret
// and saves it, as setupWithAuthentication does when the token has expired.
func (c *cli) savedCredentialsRenewal(tenant config.Tenant, regenerate func(ctx context.Context, tenant *config.Tenant) error) func(ctx context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		if err := regenerate(ctx, &tenant); err != nil {
			return "", authError{err: fmt.Errorf("failed to fetch access token using client credentials: %w", err), reason: "credentials_refresh_failed"}
		}

		if err := c.Config.AddTenant(tenant); err != nil {
			return "", configPersistError(err)
		}

		token := tenant.GetAccessToken()
		if token == "" {
			return "", authError{err: errors.New("the new access token could not be read back"), reason: "stored_token_unavailable"}
		}

		return token, nil
	}
}

// Token returns the current access token.
func (s *pluginSession) Token(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.token, nil
}

// Refresh gets a new access token after the current one was rejected. It returns
// proxy.ErrReloginRequired for a session that only the user can renew.
func (s *pluginSession) Refresh(ctx context.Context) (string, error) {
	if s.renew == nil {
		return "", proxy.ErrReloginRequired
	}

	select {
	case s.renewing <- struct{}{}:
		defer func() { <-s.renewing }()
	case <-ctx.Done():
		return "", ctx.Err()
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}

	token, err := s.renew(ctx)
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	s.token = token
	s.mu.Unlock()

	return token, nil
}
