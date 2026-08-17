package skills

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"

	"github.com/auth0/auth0-cli/internal/utils"
)

// copyTree recursively copies the contents of src into dst, creating directories as needed.
func copyTree(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := os.MkdirAll(dstPath, 0o755); err != nil {
				return err
			}
			if err := copyTree(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		if err := utils.CopyFile(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}

type AgentConfig struct {
	ID                    string
	DisplayName           string
	GlobalSkillsDir       string
	GlobalSkillsDirEnvVar string
	DetectMarkers         []string
	DetectMarkerEnvVars   []string
	DetectBinaries        []string
}

func (a AgentConfig) ResolvedGlobalSkillsDir() (string, error) {
	if a.GlobalSkillsDirEnvVar != "" {
		if v := os.Getenv(a.GlobalSkillsDirEnvVar); v != "" {
			return filepath.Join(v, "skills"), nil
		}
	}
	if a.GlobalSkillsDir == "" {
		return "", fmt.Errorf("no skills directory resolved for %q (GlobalSkillsDir or GlobalSkillsDirEnvVar required)", a.ID)
	}
	return a.GlobalSkillsDir, nil
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

func homeDir() string {
	if u, err := user.LookupId(strconv.Itoa(os.Getuid())); err == nil && u.HomeDir != "" {
		return u.HomeDir
	}
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h
	}
	return ""
}

// supportedAgents returns every assistant the CLI knows about, rooted at home.
func supportedAgents(home string) []AgentConfig {
	if home == "" {
		return []AgentConfig{{ID: "universal", DisplayName: "Universal"}}
	}

	return []AgentConfig{
		{
			ID:              "claude-code",
			DisplayName:     "Claude Code",
			GlobalSkillsDir: filepath.Join(home, ".claude", "skills"),
			DetectMarkers:   []string{filepath.Join(home, ".claude")},
			DetectBinaries:  []string{"claude"},
		},
		{
			ID:              "cursor",
			DisplayName:     "Cursor",
			GlobalSkillsDir: filepath.Join(home, ".cursor", "skills"),
			DetectMarkers:   []string{filepath.Join(home, ".cursor")},
			DetectBinaries:  []string{"cursor"},
		},
		{
			ID:              "github-copilot",
			DisplayName:     "GitHub Copilot",
			GlobalSkillsDir: filepath.Join(home, ".copilot", "skills"),
			DetectMarkers: []string{
				filepath.Join(home, ".copilot"),
				filepath.Join(home, ".config", "github-copilot"),
			},
		},
		{
			ID:              "gemini-cli",
			DisplayName:     "Gemini CLI",
			GlobalSkillsDir: filepath.Join(home, ".gemini", "skills"),
			DetectMarkers:   []string{filepath.Join(home, ".gemini")},
			DetectBinaries:  []string{"gemini"},
		},
		{
			ID:              "antigravity",
			DisplayName:     "Antigravity",
			GlobalSkillsDir: filepath.Join(home, ".gemini", "config", "skills"),
			DetectMarkers:   []string{filepath.Join(home, ".gemini", "antigravity")},
		},
		{
			ID:              "roo",
			DisplayName:     "Roo Code",
			GlobalSkillsDir: filepath.Join(home, ".roo", "skills"),
			DetectMarkers:   []string{filepath.Join(home, ".roo")},
		},
		{
			ID:              "goose",
			DisplayName:     "Goose",
			GlobalSkillsDir: filepath.Join(home, ".agents", "skills"),
			DetectMarkers:   []string{filepath.Join(home, ".config", "goose")},
		},
		{
			ID:              "opencode",
			DisplayName:     "OpenCode",
			GlobalSkillsDir: filepath.Join(home, ".config", "opencode", "skills"),
			DetectMarkers:   []string{filepath.Join(home, ".config", "opencode")},
		},
		{
			ID:                    "codex",
			DisplayName:           "Codex (OpenAI)",
			GlobalSkillsDir:       filepath.Join(home, ".codex", "skills"),
			GlobalSkillsDirEnvVar: "CODEX_HOME",
			DetectMarkers:         []string{filepath.Join(home, ".codex"), "/etc/codex"},
			DetectMarkerEnvVars:   []string{"CODEX_HOME"},
		},
		{
			ID:              "windsurf",
			DisplayName:     "Windsurf",
			GlobalSkillsDir: filepath.Join(home, ".windsurf", "skills"),
			DetectMarkers:   []string{filepath.Join(home, ".windsurf")},
		},
		{
			ID:              "continue",
			DisplayName:     "Continue",
			GlobalSkillsDir: filepath.Join(home, ".continue", "skills"),
			DetectMarkers:   []string{filepath.Join(home, ".continue")},
		},
		{
			ID:              "amp",
			DisplayName:     "Amp",
			GlobalSkillsDir: filepath.Join(home, ".config", "agents", "skills"),
			DetectMarkers:   []string{filepath.Join(home, ".config", "amp")},
		},
		{
			ID:              "junie",
			DisplayName:     "Junie",
			GlobalSkillsDir: filepath.Join(home, ".junie", "skills"),
			DetectMarkers:   []string{filepath.Join(home, ".junie")},
		},
		{
			ID:              "kiro-cli",
			DisplayName:     "Kiro CLI",
			GlobalSkillsDir: filepath.Join(home, ".kiro", "skills"),
			DetectMarkers:   []string{filepath.Join(home, ".kiro")},
		},
		{
			ID:              "cline",
			DisplayName:     "Cline",
			GlobalSkillsDir: filepath.Join(home, ".cline", "skills"),
			DetectMarkers:   []string{filepath.Join(home, ".cline")},
		},
		{
			ID:              "augment",
			DisplayName:     "Augment",
			GlobalSkillsDir: filepath.Join(home, ".augment", "skills"),
			DetectMarkers:   []string{filepath.Join(home, ".augment")},
		},
		{
			ID:              "aider-desk",
			DisplayName:     "AiderDesk",
			GlobalSkillsDir: filepath.Join(home, ".aider-desk", "skills"),
			DetectMarkers:   []string{filepath.Join(home, ".aider-desk")},
		},
		{
			ID:              "warp",
			DisplayName:     "Warp",
			GlobalSkillsDir: filepath.Join(home, ".agents", "skills"),
			DetectMarkers:   []string{filepath.Join(home, ".warp")},
		},
		{
			ID:              "devin",
			DisplayName:     "Devin",
			GlobalSkillsDir: filepath.Join(home, ".config", "devin", "skills"),
			DetectMarkers:   []string{filepath.Join(home, ".config", "devin")},
		},
		{
			ID:                    "mistral-vibe",
			DisplayName:           "Mistral Vibe",
			GlobalSkillsDirEnvVar: "VIBE_HOME",
			DetectMarkerEnvVars:   []string{"VIBE_HOME"},
		},
		{
			ID:              "openhands",
			DisplayName:     "OpenHands",
			GlobalSkillsDir: filepath.Join(home, ".agents", "skills"),
		},
		{
			ID:              "trae",
			DisplayName:     "Trae",
			GlobalSkillsDir: filepath.Join(home, ".trae", "skills"),
		},
		{
			ID:              "mux",
			DisplayName:     "Mux",
			GlobalSkillsDir: filepath.Join(home, ".mux", "skills"),
		},
		{
			ID:              "universal",
			DisplayName:     "Universal",
			GlobalSkillsDir: filepath.Join(home, ".agents", "skills"),
		},
	}
}

// DetectedAgents returns the supported assistants installed on this machine, plus universal.
func DetectedAgents() []AgentConfig {
	var detected []AgentConfig
	for _, a := range supportedAgents(homeDir()) {
		if a.ID == "universal" || a.IsInstalled() {
			detected = append(detected, a)
		}
	}
	return detected
}
