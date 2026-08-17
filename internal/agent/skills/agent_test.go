package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsInstalled(t *testing.T) {
	t.Run("returns true when marker path exists", func(t *testing.T) {
		dir := t.TempDir()
		a := AgentConfig{DetectMarkers: []string{dir}}
		assert.True(t, a.IsInstalled())
	})

	t.Run("returns false when marker path does not exist", func(t *testing.T) {
		a := AgentConfig{DetectMarkers: []string{"/this/path/definitely/does/not/exist/99999"}}
		assert.False(t, a.IsInstalled())
	})

	t.Run("skips empty marker strings", func(t *testing.T) {
		a := AgentConfig{DetectMarkers: []string{"", "/also/does/not/exist/99999"}}
		assert.False(t, a.IsInstalled())
	})

	t.Run("returns true on first matching marker", func(t *testing.T) {
		dir := t.TempDir()
		a := AgentConfig{DetectMarkers: []string{"/does/not/exist", dir, "/also/does/not/exist"}}
		assert.True(t, a.IsInstalled())
	})

	t.Run("returns true when binary is found in PATH", func(t *testing.T) {
		dir := t.TempDir()
		bin := filepath.Join(dir, "auth0-test-sentinel")
		require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755))
		t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

		a := AgentConfig{DetectBinaries: []string{"auth0-test-sentinel"}}
		assert.True(t, a.IsInstalled())
	})

	t.Run("returns false when binary is not found in PATH", func(t *testing.T) {
		a := AgentConfig{DetectBinaries: []string{"this-binary-does-not-exist-99999"}}
		assert.False(t, a.IsInstalled())
	})

	t.Run("skips empty binary strings", func(t *testing.T) {
		a := AgentConfig{DetectBinaries: []string{"", "also-does-not-exist-99999"}}
		assert.False(t, a.IsInstalled())
	})

	t.Run("returns false with no markers or binaries", func(t *testing.T) {
		a := AgentConfig{}
		assert.False(t, a.IsInstalled())
	})

	t.Run("returns false with nil markers and binaries", func(t *testing.T) {
		a := AgentConfig{DetectMarkers: nil, DetectBinaries: nil}
		assert.False(t, a.IsInstalled())
	})

	t.Run("binary check is tried when markers all miss", func(t *testing.T) {
		dir := t.TempDir()
		bin := filepath.Join(dir, "auth0-fallback-sentinel")
		require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755))
		t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

		a := AgentConfig{
			DetectMarkers:  []string{"/does/not/exist/99999"},
			DetectBinaries: []string{"auth0-fallback-sentinel"},
		}
		assert.True(t, a.IsInstalled())
	})

	t.Run("DetectMarkerEnvVars: returns true when env var points to existing path", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("AUTH0_TEST_DETECT_HOME", dir)
		a := AgentConfig{DetectMarkerEnvVars: []string{"AUTH0_TEST_DETECT_HOME"}}
		assert.True(t, a.IsInstalled())
	})

	t.Run("DetectMarkerEnvVars: returns false when env var is unset", func(t *testing.T) {
		t.Setenv("AUTH0_TEST_DETECT_HOME_UNSET", "")
		a := AgentConfig{DetectMarkerEnvVars: []string{"AUTH0_TEST_DETECT_HOME_UNSET"}}
		assert.False(t, a.IsInstalled())
	})

	t.Run("DetectMarkerEnvVars: returns false when env var points to non-existent path", func(t *testing.T) {
		t.Setenv("AUTH0_TEST_DETECT_HOME", "/does/not/exist/for/sure/99999")
		a := AgentConfig{DetectMarkerEnvVars: []string{"AUTH0_TEST_DETECT_HOME"}}
		assert.False(t, a.IsInstalled())
	})

	t.Run("DetectMarkerEnvVars: skips empty env var names", func(t *testing.T) {
		a := AgentConfig{DetectMarkerEnvVars: []string{"", "ALSO_NOT_SET_SKIPS_99999"}}
		assert.False(t, a.IsInstalled())
	})
}

func TestResolvedGlobalSkillsDir(t *testing.T) {
	t.Run("returns GlobalSkillsDir when env var is unset", func(t *testing.T) {
		t.Setenv("AUTH0_TEST_SKILLS_HOME", "")
		a := AgentConfig{
			GlobalSkillsDir:       "/fallback/skills",
			GlobalSkillsDirEnvVar: "AUTH0_TEST_SKILLS_HOME",
		}
		got, err := a.ResolvedGlobalSkillsDir()
		assert.NoError(t, err)
		assert.Equal(t, "/fallback/skills", got)
	})

	t.Run("returns env var path when set", func(t *testing.T) {
		t.Setenv("AUTH0_TEST_SKILLS_HOME", "/custom/home")
		a := AgentConfig{
			GlobalSkillsDir:       "/fallback/skills",
			GlobalSkillsDirEnvVar: "AUTH0_TEST_SKILLS_HOME",
		}
		got, err := a.ResolvedGlobalSkillsDir()
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join("/custom/home", "skills"), got)
	})

	t.Run("returns GlobalSkillsDir when GlobalSkillsDirEnvVar is empty", func(t *testing.T) {
		a := AgentConfig{GlobalSkillsDir: "/fallback/skills"}
		got, err := a.ResolvedGlobalSkillsDir()
		assert.NoError(t, err)
		assert.Equal(t, "/fallback/skills", got)
	})

	t.Run("returns error when GlobalSkillsDir is empty and env var unset", func(t *testing.T) {
		a := AgentConfig{ID: "test-agent"}
		_, err := a.ResolvedGlobalSkillsDir()
		assert.EqualError(t, err, `no skills directory resolved for "test-agent" (GlobalSkillsDir or GlobalSkillsDirEnvVar required)`)
	})

	t.Run("returns env var path when GlobalSkillsDir is empty but env var is set", func(t *testing.T) {
		t.Setenv("AUTH0_TEST_SKILLS_HOME", "/custom/home")
		a := AgentConfig{
			ID:                    "test-agent",
			GlobalSkillsDirEnvVar: "AUTH0_TEST_SKILLS_HOME",
		}
		got, err := a.ResolvedGlobalSkillsDir()
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join("/custom/home", "skills"), got)
	})

	t.Run("mistral-vibe returns error when VIBE_HOME is not set", func(t *testing.T) {
		t.Setenv("VIBE_HOME", "")
		a := AgentConfig{
			ID:                    "mistral-vibe",
			GlobalSkillsDirEnvVar: "VIBE_HOME",
		}
		_, err := a.ResolvedGlobalSkillsDir()
		assert.EqualError(t, err, `no skills directory resolved for "mistral-vibe" (GlobalSkillsDir or GlobalSkillsDirEnvVar required)`)
	})
}

func TestSupportedAgents(t *testing.T) {
	agents := supportedAgents(t.TempDir())

	t.Run("is non-empty", func(t *testing.T) {
		assert.NotEmpty(t, agents)
	})

	t.Run("all agents have non-empty ID and DisplayName", func(t *testing.T) {
		for _, a := range agents {
			assert.NotEmptyf(t, a.ID, "agent ID must not be empty")
			assert.NotEmptyf(t, a.DisplayName, "agent %s DisplayName must not be empty", a.ID)
		}
	})

	t.Run("all agents have non-empty skill dirs", func(t *testing.T) {
		for _, a := range agents {
			hasGlobalDir := a.GlobalSkillsDir != "" || a.GlobalSkillsDirEnvVar != ""
			assert.Truef(t, hasGlobalDir, "agent %s must have GlobalSkillsDir or GlobalSkillsDirEnvVar", a.ID)
		}
	})

	t.Run("all agent IDs are unique", func(t *testing.T) {
		seen := make(map[string]bool)
		for _, a := range agents {
			assert.Falsef(t, seen[a.ID], "duplicate agent ID: %s", a.ID)
			seen[a.ID] = true
		}
	})

	t.Run("required agents are present", func(t *testing.T) {
		required := []string{
			"claude-code", "cursor", "github-copilot", "gemini-cli",
			"antigravity", "devin", "mistral-vibe", "mux",
			"codex", "universal",
		}
		byID := make(map[string]bool, len(agents))
		for _, a := range agents {
			byID[a.ID] = true
		}
		for _, id := range required {
			assert.Truef(t, byID[id], "agent %s must be supported", id)
		}
	})

	t.Run("every non-universal agent has a detection signal", func(t *testing.T) {
		// The universal agent is force-included by DetectedAgents, so it needs no signal; every other
		// agent must declare at least one, or it can never be auto-detected (the openhands/trae/mux gap).
		for _, a := range agents {
			if a.ID == "universal" {
				continue
			}
			hasSignal := len(a.DetectMarkers) > 0 || len(a.DetectMarkerEnvVars) > 0 || len(a.DetectBinaries) > 0
			assert.Truef(t, hasSignal, "agent %s must declare at least one detection signal", a.ID)
		}
	})

	t.Run("codex uses CODEX_HOME env var for detection and skills dir", func(t *testing.T) {
		byID := make(map[string]AgentConfig)
		for _, a := range agents {
			byID[a.ID] = a
		}
		codex := byID["codex"]
		assert.Equal(t, "CODEX_HOME", codex.GlobalSkillsDirEnvVar)
		assert.Contains(t, codex.DetectMarkerEnvVars, "CODEX_HOME")
		assert.Contains(t, codex.DetectMarkers, "/etc/codex")
	})

	t.Run("github-copilot does not use gh binary for detection", func(t *testing.T) {
		byID := make(map[string]AgentConfig)
		for _, a := range agents {
			byID[a.ID] = a
		}
		copilot := byID["github-copilot"]
		for _, b := range copilot.DetectBinaries {
			assert.NotEqual(t, "gh", b, "gh is the GitHub CLI, not Copilot; must not be used as a detection proxy")
		}
	})

	t.Run("mistral-vibe uses VIBE_HOME env var", func(t *testing.T) {
		byID := make(map[string]AgentConfig)
		for _, a := range agents {
			byID[a.ID] = a
		}
		mv := byID["mistral-vibe"]
		assert.Equal(t, "VIBE_HOME", mv.GlobalSkillsDirEnvVar)
		assert.Contains(t, mv.DetectMarkerEnvVars, "VIBE_HOME")
	})

	t.Run("falls back to only universal when home is empty", func(t *testing.T) {
		fallback := supportedAgents("")
		require.Len(t, fallback, 1)
		assert.Equal(t, "universal", fallback[0].ID)
	})
}

func TestDetectedAgents(t *testing.T) {
	t.Run("always includes universal", func(t *testing.T) {
		found := false
		for _, a := range DetectedAgents() {
			if a.ID == "universal" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("returns consistent results on repeated calls", func(t *testing.T) {
		assert.Equal(t, DetectedAgents(), DetectedAgents())
	})

	t.Run("all detected agents are supported", func(t *testing.T) {
		supported := make(map[string]bool)
		for _, a := range supportedAgents(homeDir()) {
			supported[a.ID] = true
		}
		for _, a := range DetectedAgents() {
			assert.Truef(t, supported[a.ID], "detected agent %s is not supported", a.ID)
		}
	})
}

func TestSupportedAgentsAccessor(t *testing.T) {
	ids := make(map[string]bool)
	for _, a := range SupportedAgents() {
		ids[a.ID] = true
	}
	// The three agents that were previously unreachable must now be in the supported set.
	for _, id := range []string{"openhands", "trae", "mux", "universal", "claude-code"} {
		assert.Truef(t, ids[id], "SupportedAgents() must include %s", id)
	}
}

func TestCopyTree(t *testing.T) {
	t.Run("copies regular files", func(t *testing.T) {
		src := t.TempDir()
		dst := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(src, "file.txt"), []byte("hello"), 0o644))

		require.NoError(t, copyTree(src, dst))

		data, err := os.ReadFile(filepath.Join(dst, "file.txt"))
		require.NoError(t, err)
		assert.Equal(t, "hello", string(data))
	})

	t.Run("recurses into subdirectories", func(t *testing.T) {
		src := t.TempDir()
		dst := t.TempDir()
		sub := filepath.Join(src, "sub")
		require.NoError(t, os.MkdirAll(sub, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(sub, "nested.txt"), []byte("nested"), 0o644))

		require.NoError(t, copyTree(src, dst))

		data, err := os.ReadFile(filepath.Join(dst, "sub", "nested.txt"))
		require.NoError(t, err)
		assert.Equal(t, "nested", string(data))
	})

	t.Run("returns error when src does not exist", func(t *testing.T) {
		err := copyTree(filepath.Join(t.TempDir(), "missing"), t.TempDir())
		require.Error(t, err)
	})
}
