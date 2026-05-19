package skills

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// AgentConfig describes a single AI coding agent and where it reads skills from.
type AgentConfig struct {
	ID               string
	DisplayName      string
	GlobalSkillsDir  string
	ProjectSkillsDir string
	DetectMarkers    []string // Paths whose existence indicates the agent is installed (any one match is sufficient).
	DetectBinaries   []string // Binary names to look up in PATH (any one match is sufficient).
}

// IsInstalled reports whether the agent appears to be installed on this machine.
// It returns true if any marker path exists or any binary is found in PATH.
func (a AgentConfig) IsInstalled() bool {
	for _, marker := range a.DetectMarkers {
		if marker == "" {
			continue
		}
		if _, err := os.Stat(marker); err == nil {
			return true
		}
	}
	for _, binary := range a.DetectBinaries {
		if binary == "" {
			continue
		}
		if _, err := exec.LookPath(binary); err == nil {
			return true
		}
	}
	return false
}

// SupportedAgents is the full list of AI coding agents that support the agentskills spec.
var SupportedAgents []AgentConfig

func init() {
	home, _ := os.UserHomeDir()
	if home == "" {
		return
	}

	SupportedAgents = []AgentConfig{
		{
			ID:               "claude-code",
			DisplayName:      "Claude Code",
			GlobalSkillsDir:  filepath.Join(home, ".claude", "skills"),
			ProjectSkillsDir: filepath.Join(".claude", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".claude")},
			DetectBinaries:   []string{"claude"},
		},
		{
			ID:               "cursor",
			DisplayName:      "Cursor",
			GlobalSkillsDir:  filepath.Join(home, ".cursor", "skills"),
			ProjectSkillsDir: filepath.Join(".agents", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".cursor")},
			DetectBinaries:   []string{"cursor"},
		},
		{
			ID:               "github-copilot",
			DisplayName:      "GitHub Copilot",
			GlobalSkillsDir:  filepath.Join(home, ".copilot", "skills"),
			ProjectSkillsDir: filepath.Join(".agents", "skills"),
			DetectMarkers: []string{
				filepath.Join(home, ".copilot"),
				filepath.Join(home, ".config", "github-copilot"),
				filepath.Join(home, ".config", "gh"),
			},
			DetectBinaries: []string{"gh"},
		},
		{
			ID:               "gemini-cli",
			DisplayName:      "Gemini CLI",
			GlobalSkillsDir:  filepath.Join(home, ".gemini", "skills"),
			ProjectSkillsDir: filepath.Join(".agents", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".gemini")},
			DetectBinaries:   []string{"gemini"},
		},
		{
			ID:               "roo",
			DisplayName:      "Roo Code",
			GlobalSkillsDir:  filepath.Join(home, ".roo", "skills"),
			ProjectSkillsDir: filepath.Join(".roo", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".roo")},
			DetectBinaries:   nil,
		},
		{
			ID:               "goose",
			DisplayName:      "Goose",
			GlobalSkillsDir:  filepath.Join(home, ".config", "goose", "skills"),
			ProjectSkillsDir: filepath.Join(".goose", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".config", "goose")},
			DetectBinaries:   nil,
		},
		{
			ID:               "opencode",
			DisplayName:      "OpenCode",
			GlobalSkillsDir:  filepath.Join(home, ".config", "opencode", "skills"),
			ProjectSkillsDir: filepath.Join(".agents", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".config", "opencode")},
			DetectBinaries:   nil,
		},
		{
			ID:               "codex",
			DisplayName:      "Codex (OpenAI)",
			GlobalSkillsDir:  filepath.Join(home, ".codex", "skills"),
			ProjectSkillsDir: filepath.Join(".agents", "skills"),
			DetectMarkers:    []string{os.Getenv("CODEX_HOME")},
			DetectBinaries:   nil,
		},
		{
			ID:               "windsurf",
			DisplayName:      "Windsurf",
			GlobalSkillsDir:  filepath.Join(home, ".windsurf", "skills"),
			ProjectSkillsDir: filepath.Join(".windsurf", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".windsurf")},
			DetectBinaries:   nil,
		},
		{
			ID:               "continue",
			DisplayName:      "Continue",
			GlobalSkillsDir:  filepath.Join(home, ".continue", "skills"),
			ProjectSkillsDir: filepath.Join(".continue", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".continue")},
			DetectBinaries:   nil,
		},
		{
			ID:               "amp",
			DisplayName:      "Amp",
			GlobalSkillsDir:  filepath.Join(home, ".config", "agents", "skills"),
			ProjectSkillsDir: filepath.Join(".agents", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".config", "amp")},
			DetectBinaries:   nil,
		},
		{
			ID:               "junie",
			DisplayName:      "Junie",
			GlobalSkillsDir:  filepath.Join(home, ".junie", "skills"),
			ProjectSkillsDir: filepath.Join(".junie", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".junie")},
			DetectBinaries:   nil,
		},
		{
			ID:               "kiro-cli",
			DisplayName:      "Kiro CLI",
			GlobalSkillsDir:  filepath.Join(home, ".kiro", "skills"),
			ProjectSkillsDir: filepath.Join(".kiro", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".kiro")},
			DetectBinaries:   nil,
		},
		{
			ID:               "cline",
			DisplayName:      "Cline",
			GlobalSkillsDir:  filepath.Join(home, ".agents", "skills"),
			ProjectSkillsDir: filepath.Join(".agents", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".cline")},
			DetectBinaries:   nil,
		},
		{
			ID:               "augment",
			DisplayName:      "Augment",
			GlobalSkillsDir:  filepath.Join(home, ".augment", "skills"),
			ProjectSkillsDir: filepath.Join(".augment", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".augment")},
			DetectBinaries:   nil,
		},
		{
			ID:               "aider-desk",
			DisplayName:      "AiderDesk",
			GlobalSkillsDir:  filepath.Join(home, ".aider-desk", "skills"),
			ProjectSkillsDir: filepath.Join(".aider-desk", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".aider-desk")},
			DetectBinaries:   nil,
		},
		{
			ID:               "warp",
			DisplayName:      "Warp",
			GlobalSkillsDir:  filepath.Join(home, ".config", "agents", "skills"),
			ProjectSkillsDir: filepath.Join(".agents", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".warp")},
			DetectBinaries:   nil,
		},
		{
			ID:               "openhands",
			DisplayName:      "OpenHands",
			GlobalSkillsDir:  filepath.Join(home, ".openhands", "skills"),
			ProjectSkillsDir: filepath.Join(".openhands", "skills"),
			DetectMarkers:    nil,
			DetectBinaries:   nil,
		},
		{
			ID:               "trae",
			DisplayName:      "Trae",
			GlobalSkillsDir:  filepath.Join(home, ".trae", "skills"),
			ProjectSkillsDir: filepath.Join(".trae", "skills"),
			DetectMarkers:    nil,
			DetectBinaries:   nil,
		},
		{
			ID:               "universal",
			DisplayName:      "Universal",
			GlobalSkillsDir:  filepath.Join(home, ".agents", "skills"),
			ProjectSkillsDir: filepath.Join(".agents", "skills"),
			DetectMarkers:    nil,
			DetectBinaries:   nil,
		},
	}
}

var (
	detectedAgentsOnce  sync.Once
	detectedAgentsCache []AgentConfig
)

// DetectedAgents returns the subset of SupportedAgents that are installed on this machine.
// The universal agent is always included. Result is cached after the first call.
func DetectedAgents() []AgentConfig {
	detectedAgentsOnce.Do(func() {
		for _, a := range SupportedAgents {
			if a.ID == "universal" || a.IsInstalled() {
				detectedAgentsCache = append(detectedAgentsCache, a)
			}
		}
	})
	return detectedAgentsCache
}

// FastPriorityAgents returns detected agents in the priority order used by --fast mode:
// claude-code, cursor, github-copilot, gemini-cli, then all other detected agents,
// with universal always appended last.
func FastPriorityAgents() []AgentConfig {
	detected := DetectedAgents()

	priority := []string{"claude-code", "cursor", "github-copilot", "gemini-cli"}
	byID := make(map[string]AgentConfig, len(detected))
	for _, a := range detected {
		byID[a.ID] = a
	}

	var result []AgentConfig
	added := make(map[string]bool)

	for _, id := range priority {
		if a, ok := byID[id]; ok && id != "universal" {
			result = append(result, a)
			added[id] = true
		}
	}

	for _, a := range detected {
		if !added[a.ID] && a.ID != "universal" {
			result = append(result, a)
		}
	}

	if a, ok := byID["universal"]; ok {
		result = append(result, a)
	}

	return result
}
