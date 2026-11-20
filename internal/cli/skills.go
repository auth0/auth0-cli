package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
)

const (
	skillName        = "auth0"
	auth0SkillSource = "https://github.com/auth0/agent-skills/tree/main/plugins/auth0/skills/auth0"
	allAgentsInput   = "all"
	allAgentsToken   = "*"

	// Pinned Vercel `skills` npm package version so npx never pulls an untested/compromised release.
	skillsCLIVersion = "1.5.23"
	skillsCLISpec    = "skills@" + skillsCLIVersion
)

// agentSkillsDeprecationText points users of the deprecated `auth0 agent` commands to `auth0 skills`.
const agentSkillsDeprecationText = "⚠️  DEPRECATION NOTICE: `auth0 agent skills install` is deprecated and will be removed in a future release.\n" +
	"   Use `auth0 skills install` instead; it accepts the same flags.\n\n"

// skillsCmd groups the Auth0 skill-management commands.
func skillsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Args:  cobra.NoArgs,
		Short: "Manage Auth0 skills for AI coding assistants",
		Long:  "Manage Auth0 skills that provide Auth0-specific guidance to your AI coding assistants.",
	}

	cmd.AddCommand(installCmd(cli))

	return cmd
}

// agentCmd is the deprecated parent of `auth0 agent skills install`, hidden from help and docs
// but kept runnable so existing workflows don't break.
func agentCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "agent",
		Args:   cobra.NoArgs,
		Hidden: true,
		Short:  "⚠️ Manage Auth0 AI capabilities (DEPRECATED, use `auth0 skills`)",
		Long:   agentSkillsDeprecationText + "Manage Auth0 AI capabilities including skills for your AI coding assistants.",
	}

	cmd.AddCommand(agentSkillsCmd(cli))

	return cmd
}

// agentSkillsCmd is the deprecated `auth0 agent skills` group.
func agentSkillsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Args:  cobra.NoArgs,
		Short: "⚠️ Manage Auth0 AI skills for coding assistants (DEPRECATED, use `auth0 skills`)",
		Long:  agentSkillsDeprecationText + "Manage Auth0 AI skills that provide Auth0-specific guidance to your AI coding assistants.",
	}

	cmd.AddCommand(deprecatedInstallCmd(cli))

	return cmd
}

// deprecatedInstallCmd is `auth0 agent skills install`: it warns, then runs `auth0 skills install`.
func deprecatedInstallCmd(cli *cli) *cobra.Command {
	cmd := installCmd(cli)
	cmd.Short = "⚠️ Install the Auth0 skill (DEPRECATED, use `auth0 skills install`)"
	cmd.Long = agentSkillsDeprecationText + cmd.Long
	cmd.Example = strings.ReplaceAll(cmd.Example, "auth0 skills install", "auth0 agent skills install")

	runE := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		warnAgentSkillsDeprecated(cli)
		return runE(cmd, args)
	}

	return cmd
}

// warnAgentSkillsDeprecated points users to `auth0 skills install`; like all warnings, it is suppressed in agent mode.
func warnAgentSkillsDeprecated(cli *cli) {
	cli.renderer.Warnf("%s %s is deprecated and will be removed in a future release. Use %s instead (same flags).",
		ansi.Bold(ansi.Yellow("DEPRECATED:")), ansi.Bold("auth0 agent skills install"), ansi.Bold(ansi.Cyan("auth0 skills install")))
}

// installCmd installs the Auth0 skill into the user's AI coding assistants.
func installCmd(_ *cli) *cobra.Command {
	var inputs struct {
		Agents []string
		Force  bool
	}

	cmd := &cobra.Command{
		Use:   "install",
		Args:  cobra.NoArgs,
		Short: "Install the Auth0 skill for your AI coding assistants",
		Long: "Install the Auth0 skill into your AI coding assistants via the pinned skills CLI (" +
			skillsCLISpec + "), run through npx (requires Node.js).",
		Example: `  # Choose assistants interactively
  auth0 skills install

  # Install into specific assistants (comma-separated or repeatable)
  auth0 skills install --agent claude-code,cursor
  auth0 skills install --agent claude-code --agent cursor

  # Install into every supported assistant
  auth0 skills install --agent all

  # Reinstall without prompting
  auth0 skills install --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstall(cmd, inputs.Agents, inputs.Force)
		},
	}

	cmd.Flags().StringSliceVar(&inputs.Agents, "agent", nil,
		"Assistant ID(s) to install into: comma-separated or repeatable, or 'all'. Defaults to prompting.")
	cmd.Flags().BoolVar(&inputs.Force, "force", false,
		"Reinstall without prompting (skills always fetches the latest).")

	return cmd
}

// runInstall installs the Auth0 skill by shelling out to skills(1) via npx.
func runInstall(cmd *cobra.Command, agents []string, force bool) error {
	npxPath, err := exec.LookPath("npx")
	if err != nil {
		return errors.New(
			"npx not found on PATH; installing the Auth0 skill needs Node.js (>= 22.20). " +
				"Install Node.js, or run it directly:\n" +
				"  npx " + skillsCLISpec + " add " + auth0SkillSource + " --global --skill " + skillName)
	}

	args := buildSkillsAddArgs(agents, force, canPrompt(cmd))

	install := exec.Command(npxPath, args...)
	install.Stdin, install.Stdout, install.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := install.Run(); err != nil {
		return fmt.Errorf("failed to install the Auth0 skill via skills: %w", err)
	}
	return nil
}

// buildSkillsAddArgs builds the npx args for `skills add`. The leading --yes is npx's own; a
// second --yes is added when the run must not prompt (explicit --agent, --force, or non-interactive).
func buildSkillsAddArgs(agents []string, force, interactive bool) []string {
	args := []string{"--yes", skillsCLISpec, "add", auth0SkillSource, "--global", "--skill", skillName}

	nonInteractive := force || !interactive
	for _, agent := range agents {
		if strings.EqualFold(agent, allAgentsInput) {
			args = append(args, "--agent", allAgentsToken)
			nonInteractive = true
			continue
		}
		args = append(args, "--agent", agent)
		nonInteractive = true
	}

	if nonInteractive {
		args = append(args, "--yes")
	}
	return args
}
