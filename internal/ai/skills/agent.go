package skills

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

type AgentConfig struct {
	ID                   string
	DisplayName          string
	GlobalSkillsDir       string
	GlobalSkillsDirEnvVar string
	ProjectSkillsDir      string
	DetectMarkers         []string
	DetectMarkerEnvVars   []string
	DetectBinaries        []string
}

func (a AgentConfig) ResolvedGlobalSkillsDir() string {
	if a.GlobalSkillsDirEnvVar != "" {
		if v := os.Getenv(a.GlobalSkillsDirEnvVar); v != "" {
			return filepath.Join(v, "skills")
		}
	}
	return a.GlobalSkillsDir
}

func (a AgentConfig) IsInstalled() bool {
	for _, marker := range a.DetectMarkers {
		if marker == "" {
			continue
		}
		if _, err := os.Stat(marker); err == nil {
			return true
		}
	}
	for _, envVar := range a.DetectMarkerEnvVars {
		if envVar == "" {
			continue
		}
		if v := os.Getenv(envVar); v != "" {
			if _, err := os.Stat(v); err == nil {
				return true
			}
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

var SupportedAgents []AgentConfig

func init() {
	home, _ := os.UserHomeDir()
	if home == "" {
		SupportedAgents = []AgentConfig{
			{ID: "universal", DisplayName: "Universal", ProjectSkillsDir: filepath.Join(".agents", "skills")},
		}
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
			},
			DetectBinaries: []string{"code"},
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
			ID:               "antigravity",
			DisplayName:      "Antigravity",
			GlobalSkillsDir:  filepath.Join(home, ".gemini", "antigravity", "skills"),
			ProjectSkillsDir: filepath.Join(".agents", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".gemini", "antigravity")},
		},
		{
			ID:               "roo",
			DisplayName:      "Roo Code",
			GlobalSkillsDir:  filepath.Join(home, ".roo", "skills"),
			ProjectSkillsDir: filepath.Join(".roo", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".roo")},
		},
		{
			ID:               "goose",
			DisplayName:      "Goose",
			GlobalSkillsDir:  filepath.Join(home, ".config", "goose", "skills"),
			ProjectSkillsDir: filepath.Join(".goose", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".config", "goose")},
		},
		{
			ID:               "opencode",
			DisplayName:      "OpenCode",
			GlobalSkillsDir:  filepath.Join(home, ".config", "opencode", "skills"),
			ProjectSkillsDir: filepath.Join(".agents", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".config", "opencode")},
		},
		{
			ID:                   "codex",
			DisplayName:          "Codex (OpenAI)",
			GlobalSkillsDir:      filepath.Join(home, ".codex", "skills"),
			GlobalSkillsDirEnvVar: "CODEX_HOME",
			ProjectSkillsDir:     filepath.Join(".agents", "skills"),
			DetectMarkers:        []string{"/etc/codex"},
			DetectMarkerEnvVars:  []string{"CODEX_HOME"},
		},
		{
			ID:               "windsurf",
			DisplayName:      "Windsurf",
			GlobalSkillsDir:  filepath.Join(home, ".windsurf", "skills"),
			ProjectSkillsDir: filepath.Join(".windsurf", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".windsurf")},
		},
		{
			ID:               "continue",
			DisplayName:      "Continue",
			GlobalSkillsDir:  filepath.Join(home, ".continue", "skills"),
			ProjectSkillsDir: filepath.Join(".continue", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".continue")},
		},
		{
			ID:               "amp",
			DisplayName:      "Amp",
			GlobalSkillsDir:  filepath.Join(home, ".config", "agents", "skills"),
			ProjectSkillsDir: filepath.Join(".agents", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".config", "amp")},
		},
		{
			ID:               "junie",
			DisplayName:      "Junie",
			GlobalSkillsDir:  filepath.Join(home, ".junie", "skills"),
			ProjectSkillsDir: filepath.Join(".junie", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".junie")},
		},
		{
			ID:               "kiro-cli",
			DisplayName:      "Kiro CLI",
			GlobalSkillsDir:  filepath.Join(home, ".kiro", "skills"),
			ProjectSkillsDir: filepath.Join(".kiro", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".kiro")},
		},
		{
			ID:               "cline",
			DisplayName:      "Cline",
			GlobalSkillsDir:  filepath.Join(home, ".agents", "skills"),
			ProjectSkillsDir: filepath.Join(".agents", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".cline")},
		},
		{
			ID:               "augment",
			DisplayName:      "Augment",
			GlobalSkillsDir:  filepath.Join(home, ".augment", "skills"),
			ProjectSkillsDir: filepath.Join(".augment", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".augment")},
		},
		{
			ID:               "aider-desk",
			DisplayName:      "AiderDesk",
			GlobalSkillsDir:  filepath.Join(home, ".aider-desk", "skills"),
			ProjectSkillsDir: filepath.Join(".aider-desk", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".aider-desk")},
		},
		{
			ID:               "warp",
			DisplayName:      "Warp",
			GlobalSkillsDir:  filepath.Join(home, ".config", "agents", "skills"),
			ProjectSkillsDir: filepath.Join(".agents", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".warp")},
		},
		{
			ID:               "devin",
			DisplayName:      "Devin",
			GlobalSkillsDir:  filepath.Join(home, ".config", "devin", "skills"),
			ProjectSkillsDir: filepath.Join(".agents", "skills"),
			DetectMarkers:    []string{filepath.Join(home, ".config", "devin")},
		},
		{
			ID:                   "mistral-vibe",
			DisplayName:          "Mistral Vibe",
			GlobalSkillsDir:      filepath.Join(home, ".mistral-vibe", "skills"),
			GlobalSkillsDirEnvVar: "VIBE_HOME",
			ProjectSkillsDir:     filepath.Join(".agents", "skills"),
			DetectMarkerEnvVars:  []string{"VIBE_HOME"},
		},
		{
			ID:               "openhands",
			DisplayName:      "OpenHands",
			GlobalSkillsDir:  filepath.Join(home, ".openhands", "skills"),
			ProjectSkillsDir: filepath.Join(".openhands", "skills"),
		},
		{
			ID:               "trae",
			DisplayName:      "Trae",
			GlobalSkillsDir:  filepath.Join(home, ".trae", "skills"),
			ProjectSkillsDir: filepath.Join(".trae", "skills"),
		},
		{
			ID:               "mux",
			DisplayName:      "Mux",
			GlobalSkillsDir:  filepath.Join(home, ".mux", "skills"),
			ProjectSkillsDir: filepath.Join(".mux", "skills"),
		},
		{
			ID:               "universal",
			DisplayName:      "Universal",
			GlobalSkillsDir:  filepath.Join(home, ".agents", "skills"),
			ProjectSkillsDir: filepath.Join(".agents", "skills"),
		},
	}
}

var (
	detectedAgentsOnce  sync.Once
	detectedAgentsCache []AgentConfig
)

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

func ResetDetectedAgentsCache() {
	detectedAgentsOnce = sync.Once{}
	detectedAgentsCache = nil
}

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
