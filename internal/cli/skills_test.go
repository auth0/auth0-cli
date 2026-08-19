package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
