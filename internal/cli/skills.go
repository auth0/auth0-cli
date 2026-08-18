package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

const (
	skillName        = "auth0"
	auth0SkillSource = "https://github.com/auth0/agent-skills/tree/main/plugins/auth0/skills/auth0"
	allAgentsInput   = "all"
	allAgentsToken   = "*"
)

// agentCmd groups the Auth0 AI capability commands.
func agentCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Args:  cobra.NoArgs,
		Short: "Manage Auth0 AI capabilities",
		Long:  "Manage Auth0 AI capabilities including skills for your AI coding assistants.",
	}

	cmd.AddCommand(agentSkillsCmd(cli))

	return cmd
}

// agentSkillsCmd groups the Auth0 skill-management commands.
func agentSkillsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Args:  cobra.NoArgs,
		Short: "Manage Auth0 AI skills for coding assistants",
		Long:  "Manage Auth0 AI skills that provide Auth0-specific guidance to your AI coding assistants.",
	}

	cmd.AddCommand(installCmd(cli))

	return cmd
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
		Long: "Install the Auth0 skill into your AI coding assistants.\n\n" +
			"Delegates to the skills CLI (https://github.com/vercel-labs/skills) via npx, so it " +
			"requires Node.js (with npx) on your PATH. With no flags it opens the interactive " +
			"picker; use --agent to target assistants non-interactively.",
		Example: `  # Choose assistants interactively
  auth0 agent skills install

  # Install into specific assistants (comma-separated or repeatable)
  auth0 agent skills install --agent claude-code,cursor
  auth0 agent skills install --agent claude-code --agent cursor

  # Install into every supported assistant
  auth0 agent skills install --agent all

  # Reinstall without prompting
  auth0 agent skills install --force`,
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
				"  npx skills add " + auth0SkillSource + " --global --skill " + skillName)
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
	args := []string{"--yes", "skills", "add", auth0SkillSource, "--global", "--skill", skillName}

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
