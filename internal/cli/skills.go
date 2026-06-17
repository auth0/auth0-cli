package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/iostream"

	"github.com/auth0/auth0-cli/internal/ai/skills"
)

const (
	skillsSentinelPath = ".config/auth0/agents/.post-install-ran"
	skillsInstallTip   = "Tip: run 'auth0 ai skills install' to set up Auth0 skills for your AI assistant."

	skillsPluginRepo = "https://github.com/auth0/agent-skills"
	skillsPluginRef  = "main"
)

var postInstallHookAuto = Flag{
	Name:     "Auto",
	LongForm: "auto",
	Help:     "Skip the interactive prompt and install all skills automatically.",
}

func pluginTargetDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "auth0", "agents", "plugins", "auth0"), nil
}

func globalLockPath(targetDir string) string {
	return filepath.Join(targetDir, "skills-lock.json")
}

func skillsSentinel() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, skillsSentinelPath)
}

func writeSkillsSentinel() error {
	sentinel := skillsSentinel()
	if err := os.MkdirAll(filepath.Dir(sentinel), 0o755); err != nil {
		return fmt.Errorf("create sentinel directory %s: %w", filepath.Dir(sentinel), err)
	}
	if err := os.WriteFile(sentinel, []byte{}, 0o644); err != nil {
		return fmt.Errorf("write sentinel %s: %w", sentinel, err)
	}
	return nil
}

func skillsSentinelExists() bool {
	_, err := os.Stat(skillsSentinel())
	return err == nil
}

func aiCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ai",
		Short: "Manage Auth0 AI capabilities",
		Long:  "Manage Auth0 AI capabilities including skills for your AI coding assistants.",
	}

	cmd.AddCommand(aiSkillsCmd(cli))

	return cmd
}

func aiSkillsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Manage Auth0 AI skills for coding assistants",
		Long:  "Manage Auth0 AI skills that provide Auth0-specific guidance to your AI coding assistants.",
	}

	cmd.AddCommand(postInstallHookCmd(cli))

	return cmd
}

func postInstallHookCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Auto bool
	}

	cmd := &cobra.Command{
		Use:    "post-install-hook",
		Hidden: true,
		Short:  "Run post-install setup for Auth0 AI skills",
		RunE: func(cmd *cobra.Command, args []string) error {
			if skillsSentinelExists() {
				return nil
			}

			if inputs.Auto {
				if err := runInstallFast(cli); err != nil {
					return err
				}
				return writeSkillsSentinel()
			}

			if !iostream.IsInputTerminal() || !iostream.IsOutputTerminal() {
				fmt.Fprintln(os.Stderr, skillsInstallTip)
				return nil
			}

			const (
				choiceInstall = "Install   — Detect installed AI agents and install all skills globally"
				choiceSkip    = "Skip    — I will run 'auth0 ai skills install' later"
			)

			fmt.Fprintln(os.Stdout, "\nAuth0 AI skills add Auth0-specific guidance to your AI coding assistant.")
			fmt.Fprintln(os.Stdout, "")

			var choice string
			prompt := &survey.Select{
				Message: "How would you like to install them?",
				Options: []string{choiceInstall, choiceSkip},
				Default: choiceInstall,
			}

			if err := survey.AskOne(prompt, &choice); err != nil {
				// User pressed Ctrl+C or closed the terminal — skip gracefully.
				fmt.Fprintln(os.Stderr, skillsInstallTip)
				return nil
			}

			switch choice {
			case choiceInstall:
				if err := runInstallFast(cli); err != nil {
					return err
				}
			default:
				fmt.Fprintln(os.Stderr, skillsInstallTip)
				return nil
			}

			return writeSkillsSentinel()
		},
	}

	postInstallHookAuto.RegisterBool(cmd, &inputs.Auto, false)

	return cmd
}

// runInstallFast detects all installed AI agents and installs all available Auth0
// skills globally into each one. Equivalent to `auth0 ai skills install --fast`.
func runInstallFast(_ *cli) error {
	targetDir, err := pluginTargetDir()
	if err != nil {
		return fmt.Errorf("resolve plugin directory: %w", err)
	}

	lockPath := globalLockPath(targetDir)

	// Download (or skip if already up-to-date).
	var commitSHA string
	if err := ansi.Waiting(func() error {
		commitSHA, err = downloadSkillsIfNeeded(targetDir, lockPath)
		return err
	}); err != nil {
		return fmt.Errorf("download Auth0 skills: %w", err)
	}

	// List skills that were downloaded.
	skillsDir := filepath.Join(targetDir, "skills")
	available, err := skills.ListAvailableSkills(skillsDir)
	if err != nil || len(available) == 0 {
		return fmt.Errorf("no skills found in %s", skillsDir)
	}

	skillNames := make([]string, len(available))
	for i, s := range available {
		skillNames[i] = s.Name
	}

	// Install into every detected agent.
	agents := skills.FastPriorityAgents()
	var installedAgents []string
	installedSkills := make(map[string]struct{})

	for _, agent := range agents {
		agentSkillsDir, resolveErr := agent.ResolvedGlobalSkillsDir()
		if resolveErr != nil {
			continue
		}
		var linked int
		for _, skillName := range skillNames {
			sourceSkillDir := filepath.Join(skillsDir, skillName)
			if linkErr := skills.CreateSkillLink(sourceSkillDir, agentSkillsDir, skillName, false); linkErr != nil {
				fmt.Fprintf(os.Stderr, "warning: could not install skill %q for %s: %v\n", skillName, agent.DisplayName, linkErr)
			} else {
				linked++
				installedSkills[skillName] = struct{}{}
			}
		}
		if linked > 0 {
			installedAgents = append(installedAgents, agent.ID)
		}
	}

	// Write the global lock file.
	now := time.Now()
	versionConfig := &skills.VersionConfig{
		Repo:          skillsPluginRepo,
		Ref:           skillsPluginRef,
		CommitSHA:     commitSHA,
		InstalledAt:   now,
		UpdatedAt:     now,
		LastCheckedAt: now,
		Skills:        skillNames,
		Agents:        installedAgents,
		Scope:         skills.ScopeGlobal,
	}
	if writeErr := skills.WriteLock(lockPath, versionConfig); writeErr != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write lock file: %v\n", writeErr)
	}

	fmt.Fprintf(os.Stdout, "\nInstalled %d Auth0 skill(s) for %d agent(s).\n", len(installedSkills), len(installedAgents))

	fmt.Fprintf(os.Stdout, "\nAGENTS: \n")

	for _, agentID := range installedAgents {
		fmt.Fprintf(os.Stdout, "  - %s\n", agentID)
	}

	fmt.Fprintf(os.Stdout, "\nSKILLS: \n")

	for _, skillName := range skillNames {
		if _, ok := installedSkills[skillName]; ok {
			fmt.Fprintf(os.Stdout, "  - %s\n", skillName)
		}
	}

	return nil
}

// downloadSkillsIfNeeded downloads the skills plugin if the lock file is absent or
// the local commit SHA differs from the remote HEAD of main. Returns the commit SHA in use.
func downloadSkillsIfNeeded(targetDir, lockPath string) (string, error) {
	remoteSHA, err := skills.FetchCommitSHA(skillsPluginRef)
	if err != nil {
		return "", fmt.Errorf("fetch remote commit SHA: %w", err)
	}

	lock, err := skills.ReadLock(lockPath)
	if err != nil {
		return "", fmt.Errorf("read lock file: %w", err)
	}

	if lock != nil && lock.CommitSHA == remoteSHA {
		return remoteSHA, nil
	}

	return skills.DownloadPlugin(targetDir, skillsPluginRef)
}
