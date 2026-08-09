package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/agent/skills"
	"github.com/auth0/auth0-cli/internal/ansi"
)

const (
	skillConfigFileName = "skillConfig.json"
	skillsScopeGlobal   = "global"
)

// skillConfig records the installed state of the auth0 agent-skills, persisted as
// skillConfigFileName and read back to skip re-downloading when the ETag still matches.
type skillConfig struct {
	ETag        string    `json:"etag"`
	InstalledAt time.Time `json:"installedAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Agents      []string  `json:"agents"`
	Scope       string    `json:"scope"`
}

// readSkillConfig reads skillConfig.json at path. Returns nil, nil when the file does not exist.
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

// skillsRootDir holds the downloaded skills/ tree and the skill config file.
func skillsRootDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "agents"), nil
}

func localSkillsDir(rootDir string) string {
	return filepath.Join(rootDir, "skills")
}

func authSkillDir(rootDir string) string {
	return filepath.Join(localSkillsDir(rootDir), "auth0")
}

func skillConfigPath(rootDir string) string {
	return filepath.Join(rootDir, skillConfigFileName)
}

func agentCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage Auth0 AI capabilities",
		Long:  "Manage Auth0 AI capabilities including skills for your AI coding assistants.",
	}

	cmd.AddCommand(agentSkillsCmd(cli))

	return cmd
}

func agentSkillsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Manage Auth0 AI skills for coding assistants",
		Long:  "Manage Auth0 AI skills that provide Auth0-specific guidance to your AI coding assistants.",
	}

	cmd.AddCommand(installCmd(cli))

	return cmd
}

func installCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the Auth0 skill for your AI coding assistants",
		Long: "Download the Auth0 skill and install it globally into every detected AI " +
			"coding assistant on this machine.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstall(cli)
		},
	}

	return cmd
}

// runInstall downloads the "auth0" skill and installs it globally into every detected AI agent.
func runInstall(_ *cli) error {
	rootDir, err := skillsRootDir()
	if err != nil {
		return fmt.Errorf("resolve skills directory: %w", err)
	}

	sourceSkillDir := authSkillDir(rootDir)
	configPath := skillConfigPath(rootDir)

	prev, err := readSkillConfig(configPath)
	if err != nil {
		return fmt.Errorf("read skill config file: %w", err)
	}
	prevETag := ""
	if prev != nil {
		prevETag = prev.ETag
	}

	// Conditionally download: a 304 leaves the local skills untouched.
	var etag string
	if err := ansi.Waiting(func() error {
		etag, _, err = skills.DownloadSkills(localSkillsDir(rootDir), prevETag)
		return err
	}); err != nil {
		return fmt.Errorf("download Auth0 skill: %w", err)
	}

	if _, err = os.Stat(sourceSkillDir); err != nil {
		return fmt.Errorf("skill %q not found in %s", "auth0", filepath.Dir(sourceSkillDir))
	}

	installedAgents := installSkillIntoAgents(sourceSkillDir)

	now := time.Now()
	cfg := &skillConfig{
		ETag:        etag,
		InstalledAt: now,
		UpdatedAt:   now,
		Agents:      installedAgents,
		Scope:       skillsScopeGlobal,
	}
	if writeErr := writeSkillConfig(configPath, cfg); writeErr != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write skill config file: %v\n", writeErr)
	}

	fmt.Fprintf(os.Stdout, "\nInstalled the Auth0 skill for %d agent(s):\n", len(installedAgents))
	for _, agentID := range installedAgents {
		fmt.Fprintf(os.Stdout, "  - %s\n", agentID)
	}

	return nil
}

// installSkillIntoAgents links the skill at sourceSkillDir into every detected AI agent's
// global skills directory, returning the IDs of the agents it was successfully installed into.
func installSkillIntoAgents(sourceSkillDir string) []string {
	var installedAgents []string
	for _, agent := range skills.DetectedAgents() {
		agentSkillsDir, err := agent.ResolvedGlobalSkillsDir()
		if err != nil {
			continue
		}
		if err := skills.CreateSkillLink(sourceSkillDir, agentSkillsDir, "auth0"); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not install skill %q for %s: %v\n", "auth0", agent.DisplayName, err)
			continue
		}
		installedAgents = append(installedAgents, agent.ID)
	}
	return installedAgents
}
