package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/display"
)

func countArg(args []string, want string) int {
	n := 0
	for _, a := range args {
		if a == want {
			n++
		}
	}
	return n
}

func TestBuildSkillsAddArgs(t *testing.T) {
	t.Run("base invocation targets the auth0 subtree, globally, with a pinned skills version", func(t *testing.T) {
		got := buildSkillsAddArgs(nil, false, true)
		assert.Equal(t, []string{
			"--yes", skillsCLISpec, "add", auth0SkillSource, "--global", "--skill", skillName,
		}, got)
		// The skills CLI must be version-pinned (skills@x.y.z), never bare "skills".
		assert.Contains(t, got, skillsCLISpec)
		assert.Contains(t, skillsCLISpec, "@")
		// Interactive, no --agent, no --force: only npx's own --yes, so the picker still runs.
		assert.Equal(t, 1, countArg(got, "--yes"))
	})

	t.Run("explicit agents pass through and force non-interactive", func(t *testing.T) {
		got := buildSkillsAddArgs([]string{"claude-code", "cursor"}, false, true)
		assert.Equal(t, 2, countArg(got, "--agent"))
		assert.Contains(t, got, "claude-code")
		assert.Contains(t, got, "cursor")
		assert.Equal(t, 2, countArg(got, "--yes"), "explicit --agent should add the skills --yes")
	})

	t.Run("all maps to the skills wildcard token", func(t *testing.T) {
		got := buildSkillsAddArgs([]string{"all"}, false, true)
		assert.Contains(t, got, allAgentsToken)
		assert.NotContains(t, got, allAgentsInput)
		assert.Equal(t, 2, countArg(got, "--yes"))
	})

	t.Run("force adds the skills --yes even when interactive", func(t *testing.T) {
		got := buildSkillsAddArgs(nil, true, true)
		assert.Equal(t, 2, countArg(got, "--yes"))
	})

	t.Run("non-interactive session adds the skills --yes", func(t *testing.T) {
		got := buildSkillsAddArgs(nil, false, false)
		assert.Equal(t, 2, countArg(got, "--yes"))
	})
}

func TestSkillsInstallDeprecation(t *testing.T) {
	c := &cli{}

	install, _, err := skillsCmd(c).Find([]string{"install"})
	require.NoError(t, err)
	legacy, _, err := agentCmd(c).Find([]string{"skills", "install"})
	require.NoError(t, err)

	t.Run("new command is not marked deprecated", func(t *testing.T) {
		assert.NotContains(t, install.Short, "DEPRECATED")
		assert.NotContains(t, install.Long, "DEPRECATION")
		assert.NotContains(t, install.Example, "auth0 agent skills install")
	})

	t.Run("legacy command is hidden from help and docs", func(t *testing.T) {
		assert.True(t, agentCmd(c).Hidden)
		assert.False(t, skillsCmd(c).Hidden)
	})

	t.Run("legacy command keeps the same flags and points to the new one", func(t *testing.T) {
		for _, name := range []string{"agent", "force"} {
			assert.NotNil(t, legacy.Flags().Lookup(name), name)
		}
		assert.Contains(t, legacy.Short, "DEPRECATED")
		assert.Contains(t, legacy.Long, "auth0 skills install")
		assert.Contains(t, legacy.Example, "auth0 agent skills install")
	})

	t.Run("legacy command warns with the replacement", func(t *testing.T) {
		var buf bytes.Buffer
		warnAgentSkillsDeprecated(&cli{renderer: &display.Renderer{MessageWriter: &buf}})
		assert.Contains(t, buf.String(), "auth0 agent skills install is deprecated")
		assert.Contains(t, buf.String(), "Use auth0 skills install instead")
	})

	t.Run("legacy warning is suppressed in agent mode", func(t *testing.T) {
		var buf bytes.Buffer
		warnAgentSkillsDeprecated(&cli{renderer: &display.Renderer{MessageWriter: &buf, AgentMode: true}})
		assert.Empty(t, buf.String())
	})
}
