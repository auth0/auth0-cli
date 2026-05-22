package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeSkillMD(t *testing.T, dir, name, description, body string) {
	t.Helper()
	skillDir := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	content := "---\nname: " + name + "\ndescription: " + description + "\n---\n" + body
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644))
}

func TestParseSkillMeta(t *testing.T) {
	t.Run("parses name and description from valid frontmatter", func(t *testing.T) {
		dir := t.TempDir()
		skillDir := filepath.Join(dir, "auth0-react")
		require.NoError(t, os.MkdirAll(skillDir, 0o755))
		content := "---\nname: auth0-react\ndescription: Auth0 React integration\n---\n\n# Auth0 React\n"
		require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644))

		meta, err := ParseSkillMeta(skillDir)
		require.NoError(t, err)
		assert.Equal(t, "auth0-react", meta.Name)
		assert.Equal(t, "Auth0 React integration", meta.Description)
	})

	t.Run("returns error when SKILL.md does not exist", func(t *testing.T) {
		_, err := ParseSkillMeta(t.TempDir())
		require.Error(t, err)
	})

	t.Run("returns empty meta when no frontmatter delimiters", func(t *testing.T) {
		dir := t.TempDir()
		skillDir := filepath.Join(dir, "no-frontmatter")
		require.NoError(t, os.MkdirAll(skillDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Just a heading\n"), 0o644))

		meta, err := ParseSkillMeta(skillDir)
		require.NoError(t, err)
		assert.Equal(t, SkillMeta{}, meta)
	})

	t.Run("returns empty meta when only one delimiter present", func(t *testing.T) {
		dir := t.TempDir()
		skillDir := filepath.Join(dir, "partial")
		require.NoError(t, os.MkdirAll(skillDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: foo\n"), 0o644))

		meta, err := ParseSkillMeta(skillDir)
		require.NoError(t, err)
		assert.Equal(t, SkillMeta{}, meta)
	})

	t.Run("returns error for invalid YAML in frontmatter", func(t *testing.T) {
		dir := t.TempDir()
		skillDir := filepath.Join(dir, "bad-yaml")
		require.NoError(t, os.MkdirAll(skillDir, 0o755))
		// Indentation error creates invalid YAML
		content := "---\nname: foo\n  bad: indent: here\n---\n"
		require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644))

		_, err := ParseSkillMeta(skillDir)
		require.Error(t, err)
	})

	t.Run("only name is populated when description is absent", func(t *testing.T) {
		dir := t.TempDir()
		skillDir := filepath.Join(dir, "name-only")
		require.NoError(t, os.MkdirAll(skillDir, 0o755))
		content := "---\nname: auth0-vue\n---\n"
		require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644))

		meta, err := ParseSkillMeta(skillDir)
		require.NoError(t, err)
		assert.Equal(t, "auth0-vue", meta.Name)
		assert.Equal(t, "", meta.Description)
	})
}

func TestListAvailableSkills(t *testing.T) {
	t.Run("returns error when directory does not exist", func(t *testing.T) {
		_, err := ListAvailableSkills(filepath.Join(t.TempDir(), "nonexistent"))
		require.Error(t, err)
	})

	t.Run("returns empty slice for empty directory", func(t *testing.T) {
		skills, err := ListAvailableSkills(t.TempDir())
		require.NoError(t, err)
		assert.Empty(t, skills)
	})

	t.Run("returns sorted skills from multiple subdirectories", func(t *testing.T) {
		dir := t.TempDir()
		writeSkillMD(t, dir, "auth0-vue", "Auth0 Vue integration", "")
		writeSkillMD(t, dir, "auth0-nextjs", "Auth0 Next.js integration", "")
		writeSkillMD(t, dir, "auth0-react", "Auth0 React integration", "")

		skills, err := ListAvailableSkills(dir)
		require.NoError(t, err)
		require.Len(t, skills, 3)
		assert.Equal(t, "auth0-nextjs", skills[0].Name)
		assert.Equal(t, "auth0-react", skills[1].Name)
		assert.Equal(t, "auth0-vue", skills[2].Name)
	})

	t.Run("skips non-directory entries", func(t *testing.T) {
		dir := t.TempDir()
		writeSkillMD(t, dir, "auth0-react", "Auth0 React integration", "")
		require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# readme"), 0o644))

		skills, err := ListAvailableSkills(dir)
		require.NoError(t, err)
		require.Len(t, skills, 1)
		assert.Equal(t, "auth0-react", skills[0].Name)
	})

	t.Run("skips subdirectories without SKILL.md", func(t *testing.T) {
		dir := t.TempDir()
		writeSkillMD(t, dir, "auth0-react", "Auth0 React integration", "")
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "not-a-skill"), 0o755))

		skills, err := ListAvailableSkills(dir)
		require.NoError(t, err)
		require.Len(t, skills, 1)
		assert.Equal(t, "auth0-react", skills[0].Name)
	})

	t.Run("description is populated from frontmatter", func(t *testing.T) {
		dir := t.TempDir()
		writeSkillMD(t, dir, "auth0-nextjs", "Next.js with Auth0", "")

		skills, err := ListAvailableSkills(dir)
		require.NoError(t, err)
		require.Len(t, skills, 1)
		assert.Equal(t, "Next.js with Auth0", skills[0].Description)
	})
}
