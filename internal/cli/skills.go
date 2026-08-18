package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/agent/skills"
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
)

const (
	// The single skill published under plugins/auth0/skills in the repo.
	skillName = "auth0"

	skillConfigFileName = "skill-config.json"
	skillsScopeGlobal   = "global"

	// Synthetic first entry in the interactive multi-select.
	allAgentsOption = "All"
)

// skillConfig is the persisted install state (skill-config.json); the ETag gates re-downloads and Skills is a slice for future skills.
type skillConfig struct {
	ETag        string    `json:"etag"`
	Skills      []string  `json:"skills"`
	InstalledAt time.Time `json:"installedAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Agents      []string  `json:"agents"`
	Scope       string    `json:"scope"`
}

// readSkillConfig reads skill-config.json at path. Returns nil, nil when the file does not exist.
func readSkillConfig(path string) (*skillConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var cfg skillConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// writeSkillConfig serialises cfg as JSON and writes it to path, creating parent directories as needed.
func writeSkillConfig(path string, cfg *skillConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// authSkillDir is where the auth0 skill is stored: ~/.agents/skills/auth0 (the universal cross-tool location).
func authSkillDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agents", "skills", skillName), nil
}

func skillConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "auth0", skillConfigFileName), nil
}

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

func installCmd(_ *cli) *cobra.Command {
	var inputs struct {
		Agents []string
		Force  bool
	}

	supportedIDs := make([]string, 0)
	for _, a := range skills.SupportedAgents() {
		supportedIDs = append(supportedIDs, a.ID)
	}

	cmd := &cobra.Command{
		Use:   "install",
		Args:  cobra.NoArgs,
		Short: "Install the Auth0 skill for your AI coding assistants",
		Long: "Download the Auth0 skill and install it into your detected AI coding assistants.\n\n" +
			"With no flags it prompts for which assistants to set up. Use --agent to select " +
			"them non-interactively.\n\n" +
			fmt.Sprintf("Supported assistants (%d): %s.", len(supportedIDs), strings.Join(supportedIDs, ", ")),
		Example: `  # Choose assistants interactively
  auth0 agent skills install

  # Install into specific assistants (comma-separated or repeatable)
  auth0 agent skills install --agent claude-code,cursor
  auth0 agent skills install --agent claude-code --agent cursor

  # Install into every detected assistant
  auth0 agent skills install --agent all

  # Re-download even if already up to date
  auth0 agent skills install --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstall(cmd, inputs.Agents, inputs.Force)
		},
	}

	cmd.Flags().StringSliceVar(&inputs.Agents, "agent", nil,
		"Assistant ID(s) to install into: comma-separated or repeatable, or 'all'. Defaults to prompting.")
	cmd.Flags().BoolVar(&inputs.Force, "force", false,
		"Re-download the skill even if it is already up to date.")

	return cmd
}

// runInstall downloads the Auth0 skill and installs it into the selected AI assistants.
func runInstall(cmd *cobra.Command, agentIDs []string, force bool) error {
	skillDir, err := authSkillDir()
	if err != nil {
		return fmt.Errorf("resolve skill directory: %w", err)
	}
	configPath, err := skillConfigPath()
	if err != nil {
		return fmt.Errorf("resolve skill config path: %w", err)
	}

	targets, err := selectAgents(cmd, agentIDs)
	if err != nil {
		return err
	}

	prev, err := readSkillConfig(configPath)
	if err != nil {
		return fmt.Errorf("read skill config file: %w", err)
	}

	// Conditional request only when the skill is present and --force is off, so a deleted
	// skill (with a stale config) still re-downloads.
	prevETag := ""
	if !force && prev != nil {
		if _, statErr := os.Stat(skillDir); statErr == nil {
			prevETag = prev.ETag
		}
	}

	var (
		etag        string
		notModified bool
	)
	if waitErr := ansi.Waiting(func() error {
		etag, notModified, err = skills.DownloadSkills(skillDir, prevETag)
		return err
	}); waitErr != nil {
		return fmt.Errorf("download Auth0 skill: %w", waitErr)
	}

	if _, err = os.Stat(skillDir); err != nil {
		return fmt.Errorf("skill %q not found in %s after download", skillName, filepath.Dir(skillDir))
	}

	// Preserve InstalledAt; advance UpdatedAt only when the content actually changed.
	now := time.Now()
	installedAt, updatedAt := now, now
	if prev != nil {
		if !prev.InstalledAt.IsZero() {
			installedAt = prev.InstalledAt
		}
		if notModified && !prev.UpdatedAt.IsZero() {
			updatedAt = prev.UpdatedAt
		}
	}

	outcome := installSkillIntoAgents(skillDir, targets)

	cfg := &skillConfig{
		ETag:        etag,
		Skills:      []string{skillName},
		InstalledAt: installedAt,
		UpdatedAt:   updatedAt,
		Agents:      outcome.installed,
		Scope:       skillsScopeGlobal,
	}
	if writeErr := writeSkillConfig(configPath, cfg); writeErr != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write skill config file: %v\n", writeErr)
	}

	reportInstallOutcome(outcome, notModified)

	if len(outcome.installed) == 0 {
		return fmt.Errorf("could not install the Auth0 skill into any assistant")
	}
	return nil
}

// reportInstallOutcome prints the installed, skipped, and failed assistants; failures go to stderr.
func reportInstallOutcome(outcome installOutcome, notModified bool) {
	if len(outcome.installed) > 0 {
		header := "Installed the Auth0 skill"
		if notModified {
			header = "The Auth0 skill is already up to date; linked it"
		}
		fmt.Fprintf(os.Stdout, "\n%s for %d assistant(s):\n", header, len(outcome.installed))
		for _, id := range outcome.installed {
			fmt.Fprintf(os.Stdout, "  - %s\n", id)
		}
	}
	if len(outcome.skipped) > 0 {
		fmt.Fprintf(os.Stdout, "\nSkipped %d assistant(s):\n", len(outcome.skipped))
		for _, s := range outcome.skipped {
			fmt.Fprintf(os.Stdout, "  - %s: %s\n", s.agent, s.detail)
		}
	}
	if len(outcome.failed) > 0 {
		fmt.Fprintf(os.Stderr, "\nFailed for %d assistant(s):\n", len(outcome.failed))
		for _, f := range outcome.failed {
			fmt.Fprintf(os.Stderr, "  - %s: %s\n", f.agent, f.detail)
		}
	}
}

// selectAgents resolves the target assistants: a validated --agent list, else an interactive prompt, else all detected.
func selectAgents(cmd *cobra.Command, agentIDs []string) ([]skills.AgentConfig, error) {
	detected := skills.DetectedAgents()

	// Explicit --agent: validate against the full supported set (all-or-nothing). A supported but
	// undetected agent is accepted here; the parent-exists guard skips it later if its dir is absent.
	if len(agentIDs) > 0 {
		supported := skills.SupportedAgents()
		byID := make(map[string]skills.AgentConfig, len(supported))
		supportedIDs := make([]string, 0, len(supported))
		for _, a := range supported {
			byID[a.ID] = a
			supportedIDs = append(supportedIDs, a.ID)
		}

		var selected []skills.AgentConfig
		var unknown []string
		for _, id := range agentIDs {
			if strings.EqualFold(id, allAgentsOption) {
				return detected, nil
			}
			if a, ok := byID[id]; ok {
				selected = append(selected, a)
			} else {
				unknown = append(unknown, id)
			}
		}
		if len(unknown) > 0 {
			return nil, fmt.Errorf(
				"unknown assistant(s): %s (available: %s)",
				strings.Join(unknown, ", "), strings.Join(supportedIDs, ", "))
		}
		return selected, nil
	}

	// The interactive picker and the no-flag default operate on auto-detected agents only.
	detectedIDs := make([]string, 0, len(detected))
	byDetectedID := make(map[string]skills.AgentConfig, len(detected))
	for _, a := range detected {
		detectedIDs = append(detectedIDs, a.ID)
		byDetectedID[a.ID] = a
	}

	// No --agent and not interactive: default to all detected.
	if !canPrompt(cmd) {
		return detected, nil
	}

	// Interactive multi-select: "All" first, at least one required.
	options := append([]string{allAgentsOption}, detectedIDs...)
	var chosen []string
	if err := prompt.AskMultiSelect(
		"Select the AI assistants to install the Auth0 skill into", &chosen, options...,
	); err != nil {
		return nil, err
	}
	if len(chosen) == 0 {
		return nil, errors.New("no assistants selected; select at least one")
	}
	for _, c := range chosen {
		if c == allAgentsOption {
			return detected, nil
		}
	}
	selected := make([]skills.AgentConfig, 0, len(chosen))
	for _, c := range chosen {
		selected = append(selected, byDetectedID[c])
	}
	return selected, nil
}

// agentOutcome pairs an assistant with a human-readable detail (a skip reason or an error).
type agentOutcome struct {
	agent  string
	detail string
}

// installOutcome is the per-assistant result of an install run.
type installOutcome struct {
	installed []string
	skipped   []agentOutcome
	failed    []agentOutcome
}

// installSkillIntoAgents symlinks skillDir into each agent's skills directory, continuing past
// per-agent failures, and returns the installed IDs plus any skips and failures.
func installSkillIntoAgents(skillDir string, agents []skills.AgentConfig) installOutcome {
	var out installOutcome
	seenDir := make(map[string]bool)
	for _, agent := range agents {
		agentSkillsDir, err := agent.ResolvedGlobalSkillsDir()
		if err != nil {
			out.skipped = append(out.skipped, agentOutcome{agent.DisplayName, "no skills directory: " + err.Error()})
			continue
		}
		// Skip when the parent directory is absent, so we never create a dead skills dir.
		if _, statErr := os.Stat(filepath.Dir(agentSkillsDir)); statErr != nil {
			out.skipped = append(out.skipped, agentOutcome{agent.DisplayName, filepath.Dir(agentSkillsDir) + " does not exist"})
			continue
		}
		if seenDir[agentSkillsDir] {
			continue // Directory already linked via another agent.
		}
		seenDir[agentSkillsDir] = true

		// The agent's own skills dir is the store (e.g. universal reads ~/.agents/skills):
		// already present, so record it without a self-referential symlink.
		if filepath.Join(agentSkillsDir, skillName) == skillDir {
			out.installed = append(out.installed, agent.ID)
			continue
		}
		if err := skills.CreateSkillLink(skillDir, agentSkillsDir, skillName); err != nil {
			out.failed = append(out.failed, agentOutcome{agent.DisplayName, err.Error()})
			continue
		}
		out.installed = append(out.installed, agent.ID)
	}
	return out
}
