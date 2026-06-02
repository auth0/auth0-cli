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
		assert.EqualError(t, err, "GlobalSkillsDirEnvVar must be set for: test-agent")
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
		assert.EqualError(t, err, "GlobalSkillsDirEnvVar must be set for: mistral-vibe")
	})
}

func TestSupportedAgents(t *testing.T) {
	t.Run("is non-empty", func(t *testing.T) {
		assert.NotEmpty(t, SupportedAgents)
	})

	t.Run("all agents have non-empty ID and DisplayName", func(t *testing.T) {
		for _, a := range SupportedAgents {
			assert.NotEmptyf(t, a.ID, "agent ID must not be empty")
			assert.NotEmptyf(t, a.DisplayName, "agent %s DisplayName must not be empty", a.ID)
		}
	})

	t.Run("all agents have non-empty skill dirs", func(t *testing.T) {
		for _, a := range SupportedAgents {
			hasGlobalDir := a.GlobalSkillsDir != "" || a.GlobalSkillsDirEnvVar != ""
			assert.Truef(t, hasGlobalDir, "agent %s must have GlobalSkillsDir or GlobalSkillsDirEnvVar", a.ID)
			assert.NotEmptyf(t, a.ProjectSkillsDir, "agent %s ProjectSkillsDir must not be empty", a.ID)
		}
	})

	t.Run("all agent IDs are unique", func(t *testing.T) {
		seen := make(map[string]bool)
		for _, a := range SupportedAgents {
			assert.Falsef(t, seen[a.ID], "duplicate agent ID: %s", a.ID)
			seen[a.ID] = true
		}
	})

	t.Run("universal agent is present", func(t *testing.T) {
		found := false
		for _, a := range SupportedAgents {
			if a.ID == "universal" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("required agents are present", func(t *testing.T) {
		required := []string{
			"claude-code", "cursor", "github-copilot", "gemini-cli",
			"antigravity", "devin", "mistral-vibe", "mux",
			"codex", "universal",
		}
		byID := make(map[string]bool, len(SupportedAgents))
		for _, a := range SupportedAgents {
			byID[a.ID] = true
		}
		for _, id := range required {
			assert.Truef(t, byID[id], "agent %s must be in SupportedAgents", id)
		}
	})

	t.Run("agents with no detection are detectable-never", func(t *testing.T) {
		// openhands, trae, mux, and universal have nil markers/binaries meaning IsInstalled
		// always returns false; they are included via explicit ID checks or --agent flag.
		noDetectIDs := []string{"openhands", "trae", "mux", "universal"}
		byID := make(map[string]AgentConfig)
		for _, a := range SupportedAgents {
			byID[a.ID] = a
		}
		for _, id := range noDetectIDs {
			a, ok := byID[id]
			require.Truef(t, ok, "agent %s must be in SupportedAgents", id)
			assert.Nilf(t, a.DetectMarkers, "agent %s should have nil DetectMarkers", id)
			assert.Nilf(t, a.DetectBinaries, "agent %s should have nil DetectBinaries", id)
			assert.Nilf(t, a.DetectMarkerEnvVars, "agent %s should have nil DetectMarkerEnvVars", id)
		}
	})

	t.Run("codex uses CODEX_HOME env var for detection and skills dir", func(t *testing.T) {
		byID := make(map[string]AgentConfig)
		for _, a := range SupportedAgents {
			byID[a.ID] = a
		}
		codex := byID["codex"]
		assert.Equal(t, "CODEX_HOME", codex.GlobalSkillsDirEnvVar)
		assert.Contains(t, codex.DetectMarkerEnvVars, "CODEX_HOME")
		assert.Contains(t, codex.DetectMarkers, "/etc/codex")
	})

	t.Run("github-copilot does not use gh binary for detection", func(t *testing.T) {
		byID := make(map[string]AgentConfig)
		for _, a := range SupportedAgents {
			byID[a.ID] = a
		}
		copilot := byID["github-copilot"]
		for _, b := range copilot.DetectBinaries {
			assert.NotEqual(t, "gh", b, "gh is the GitHub CLI, not Copilot; must not be used as a detection proxy")
		}
	})

	t.Run("mistral-vibe uses VIBE_HOME env var", func(t *testing.T) {
		byID := make(map[string]AgentConfig)
		for _, a := range SupportedAgents {
			byID[a.ID] = a
		}
		mv := byID["mistral-vibe"]
		assert.Equal(t, "VIBE_HOME", mv.GlobalSkillsDirEnvVar)
		assert.Contains(t, mv.DetectMarkerEnvVars, "VIBE_HOME")
	})
}

func TestDetectedAgents(t *testing.T) {
	t.Run("always includes universal", func(t *testing.T) {
		detected := DetectedAgents()
		found := false
		for _, a := range detected {
			if a.ID == "universal" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("returns consistent results on repeated calls", func(t *testing.T) {
		first := DetectedAgents()
		second := DetectedAgents()
		assert.Equal(t, first, second)
	})

	t.Run("all returned agents come from SupportedAgents", func(t *testing.T) {
		supported := make(map[string]bool, len(SupportedAgents))
		for _, a := range SupportedAgents {
			supported[a.ID] = true
		}
		for _, a := range DetectedAgents() {
			assert.Truef(t, supported[a.ID], "detected agent %s is not in SupportedAgents", a.ID)
		}
	})
}

func TestResetDetectedAgentsCache(t *testing.T) {
	t.Run("subsequent call after reset re-evaluates detection", func(t *testing.T) {
		// Prime the cache.
		first := DetectedAgents()
		require.NotNil(t, first)

		// Reset should clear the cached result.
		ResetDetectedAgentsCache()

		// A second call after reset should return a fresh (equal) result.
		second := DetectedAgents()
		assert.Equal(t, first, second)
	})

	t.Run("reset allows new filesystem state to be detected", func(t *testing.T) {
		// Temporarily inject a fake agent that detects a temp dir.
		dir := t.TempDir()
		fake := AgentConfig{
			ID:              "test-reset-agent",
			DisplayName:     "Test Reset Agent",
			GlobalSkillsDir: filepath.Join(dir, "skills"),
			ProjectSkillsDir: filepath.Join(".agents", "skills"),
			DetectMarkers:   []string{filepath.Join(dir, "marker")},
		}
		original := SupportedAgents
		t.Cleanup(func() {
			SupportedAgents = original
			ResetDetectedAgentsCache()
		})

		// Without the marker, fake agent should not be detected.
		ResetDetectedAgentsCache()
		SupportedAgents = append(SupportedAgents, fake)
		withoutMarker := DetectedAgents()
		for _, a := range withoutMarker {
			assert.NotEqual(t, "test-reset-agent", a.ID)
		}

		// Create the marker and reset — fake agent should now be detected.
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "marker"), 0o755))
		ResetDetectedAgentsCache()
		withMarker := DetectedAgents()
		found := false
		for _, a := range withMarker {
			if a.ID == "test-reset-agent" {
				found = true
			}
		}
		assert.True(t, found, "agent should be detected after marker is created and cache is reset")
	})
}

func TestFastPriorityAgents(t *testing.T) {
	t.Run("universal is always last", func(t *testing.T) {
		result := FastPriorityAgents()
		require.NotEmpty(t, result)
		assert.Equal(t, "universal", result[len(result)-1].ID)
	})

	t.Run("no duplicates", func(t *testing.T) {
		seen := make(map[string]bool)
		for _, a := range FastPriorityAgents() {
			assert.Falsef(t, seen[a.ID], "duplicate agent %s in FastPriorityAgents", a.ID)
			seen[a.ID] = true
		}
	})

	t.Run("contains all detected agents", func(t *testing.T) {
		resultIDs := make(map[string]bool)
		for _, a := range FastPriorityAgents() {
			resultIDs[a.ID] = true
		}
		for _, a := range DetectedAgents() {
			assert.Truef(t, resultIDs[a.ID], "detected agent %s missing from FastPriorityAgents", a.ID)
		}
	})

	t.Run("priority agents appear before non-priority agents", func(t *testing.T) {
		result := FastPriorityAgents()
		prioritySet := map[string]bool{
			"claude-code":    true,
			"cursor":         true,
			"github-copilot": true,
			"gemini-cli":     true,
		}

		lastPriorityIdx := -1
		firstNonPriorityIdx := -1
		for i, a := range result {
			if a.ID == "universal" {
				continue
			}
			if prioritySet[a.ID] {
				lastPriorityIdx = i
			} else if firstNonPriorityIdx == -1 {
				firstNonPriorityIdx = i
			}
		}

		if lastPriorityIdx != -1 && firstNonPriorityIdx != -1 {
			assert.Less(t, lastPriorityIdx, firstNonPriorityIdx,
				"all priority agents must appear before any non-priority agent")
		}
	})

	t.Run("result length equals detected agents count", func(t *testing.T) {
		assert.Len(t, FastPriorityAgents(), len(DetectedAgents()))
	})
}
