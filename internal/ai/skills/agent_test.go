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
			assert.NotEmptyf(t, a.GlobalSkillsDir, "agent %s GlobalSkillsDir must not be empty", a.ID)
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

	t.Run("agents with no detection are detectable-never", func(t *testing.T) {
		// Openhands, trae, and universal have nil markers/binaries meaning IsInstalled always
		// returns false for them; they are included through explicit ID checks instead.
		noDetectIDs := []string{"openhands", "trae", "universal"}
		byID := make(map[string]AgentConfig)
		for _, a := range SupportedAgents {
			byID[a.ID] = a
		}
		for _, id := range noDetectIDs {
			a, ok := byID[id]
			require.Truef(t, ok, "agent %s must be in SupportedAgents", id)
			assert.Nilf(t, a.DetectMarkers, "agent %s should have nil DetectMarkers", id)
			assert.Nilf(t, a.DetectBinaries, "agent %s should have nil DetectBinaries", id)
		}
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
