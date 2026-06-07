package skills

import (
	"os"
	"path/filepath"
	"testing"
)

// makeSkillDir creates skillDir/SKILL.md with a frontmatter block.
func makeSkillDir(t *testing.T, base, skillName string) string {
	t.Helper()
	dir := filepath.Join(base, skillName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + skillName + "\ndescription: test skill\n---\n# body\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestValidateInstall_OK(t *testing.T) {
	tmp := t.TempDir()
	sourcePluginDir := filepath.Join(tmp, "plugins")
	agentSkillsDir := filepath.Join(tmp, "agent", "skills")

	makeSkillDir(t, sourcePluginDir, "auth0-react")

	if err := CreateSkillLink(
		filepath.Join(sourcePluginDir, "auth0-react"),
		agentSkillsDir, "auth0-react", false,
	); err != nil {
		t.Fatalf("CreateSkillLink: %v", err)
	}

	statuses := ValidateInstall("claude-code", agentSkillsDir, sourcePluginDir, []string{"auth0-react"})
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	s := statuses[0]
	if s.Status != "ok" {
		t.Errorf("expected ok, got %q (err: %s)", s.Status, s.Error)
	}
	if s.SkillName != "auth0-react" {
		t.Errorf("unexpected SkillName: %q", s.SkillName)
	}
	if s.AgentID != "claude-code" {
		t.Errorf("unexpected AgentID: %q", s.AgentID)
	}
}

func TestValidateInstall_Missing(t *testing.T) {
	tmp := t.TempDir()
	sourcePluginDir := filepath.Join(tmp, "plugins")
	agentSkillsDir := filepath.Join(tmp, "agent", "skills")
	if err := os.MkdirAll(agentSkillsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	statuses := ValidateInstall("cursor", agentSkillsDir, sourcePluginDir, []string{"auth0-nextjs"})
	if len(statuses) != 1 {
		t.Fatalf("expected 1, got %d", len(statuses))
	}
	if statuses[0].Status != "missing" {
		t.Errorf("expected missing, got %q", statuses[0].Status)
	}
}

func TestValidateInstall_BrokenSymlink(t *testing.T) {
	tmp := t.TempDir()
	sourcePluginDir := filepath.Join(tmp, "plugins")
	agentSkillsDir := filepath.Join(tmp, "agent", "skills")
	if err := os.MkdirAll(agentSkillsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a symlink pointing to a non-existent path inside tmp (portable).
	linkPath := filepath.Join(agentSkillsDir, "auth0-vue")
	if err := os.Symlink(filepath.Join(tmp, "nonexistent", "auth0-vue"), linkPath); err != nil {
		t.Fatal(err)
	}

	statuses := ValidateInstall("gemini-cli", agentSkillsDir, sourcePluginDir, []string{"auth0-vue"})
	if statuses[0].Status != "broken_symlink" {
		t.Errorf("expected broken_symlink, got %q", statuses[0].Status)
	}
	if statuses[0].Error == "" {
		t.Error("expected non-empty Error for broken_symlink")
	}
}

func TestValidateInstall_WrongTarget(t *testing.T) {
	tmp := t.TempDir()
	sourcePluginDir := filepath.Join(tmp, "plugins")
	wrongSourceDir := filepath.Join(tmp, "other")
	agentSkillsDir := filepath.Join(tmp, "agent", "skills")

	makeSkillDir(t, sourcePluginDir, "auth0-react")
	makeSkillDir(t, wrongSourceDir, "auth0-react")

	// Link points to wrong source.
	if err := CreateSkillLink(
		filepath.Join(wrongSourceDir, "auth0-react"),
		agentSkillsDir, "auth0-react", false,
	); err != nil {
		t.Fatalf("CreateSkillLink: %v", err)
	}

	statuses := ValidateInstall("cursor", agentSkillsDir, sourcePluginDir, []string{"auth0-react"})
	if statuses[0].Status != "broken_symlink" {
		t.Errorf("expected broken_symlink, got %q", statuses[0].Status)
	}
	if statuses[0].Error == "" {
		t.Error("expected non-empty Error for wrong_target")
	}
}

func TestValidateInstall_InvalidSkill_MissingFile(t *testing.T) {
	tmp := t.TempDir()
	sourcePluginDir := filepath.Join(tmp, "plugins")
	agentSkillsDir := filepath.Join(tmp, "agent", "skills")

	// Create source dir without SKILL.md.
	skillSrc := filepath.Join(sourcePluginDir, "auth0-spa")
	if err := os.MkdirAll(skillSrc, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := CreateSkillLink(skillSrc, agentSkillsDir, "auth0-spa", false); err != nil {
		t.Fatalf("CreateSkillLink: %v", err)
	}

	statuses := ValidateInstall("claude-code", agentSkillsDir, sourcePluginDir, []string{"auth0-spa"})
	if statuses[0].Status != "invalid_skill" {
		t.Errorf("expected invalid_skill, got %q", statuses[0].Status)
	}
	if statuses[0].Error == "" {
		t.Error("expected non-empty Error field")
	}
}

func TestValidateInstall_InvalidSkill_NameMismatch(t *testing.T) {
	tmp := t.TempDir()
	sourcePluginDir := filepath.Join(tmp, "plugins")
	agentSkillsDir := filepath.Join(tmp, "agent", "skills")

	// Create skill dir with mismatched name in frontmatter.
	skillSrc := filepath.Join(sourcePluginDir, "auth0-angular")
	if err := os.MkdirAll(skillSrc, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: totally-different\ndescription: x\n---\n"
	if err := os.WriteFile(filepath.Join(skillSrc, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := CreateSkillLink(skillSrc, agentSkillsDir, "auth0-angular", false); err != nil {
		t.Fatalf("CreateSkillLink: %v", err)
	}

	statuses := ValidateInstall("claude-code", agentSkillsDir, sourcePluginDir, []string{"auth0-angular"})
	if statuses[0].Status != "invalid_skill" {
		t.Errorf("expected invalid_skill, got %q", statuses[0].Status)
	}
}

func TestValidateInstall_CopyMode(t *testing.T) {
	tmp := t.TempDir()
	sourcePluginDir := filepath.Join(tmp, "plugins")
	agentSkillsDir := filepath.Join(tmp, "agent", "skills")

	makeSkillDir(t, sourcePluginDir, "auth0-nextjs")

	if err := CreateSkillLink(
		filepath.Join(sourcePluginDir, "auth0-nextjs"),
		agentSkillsDir, "auth0-nextjs", true,
	); err != nil {
		t.Fatalf("CreateSkillLink (copy): %v", err)
	}

	statuses := ValidateInstall("cursor", agentSkillsDir, sourcePluginDir, []string{"auth0-nextjs"})
	if statuses[0].Status != "copy" {
		t.Errorf("expected copy, got %q", statuses[0].Status)
	}
}

func TestValidateInstall_MultipleSkills(t *testing.T) {
	tmp := t.TempDir()
	sourcePluginDir := filepath.Join(tmp, "plugins")
	agentSkillsDir := filepath.Join(tmp, "agent", "skills")

	makeSkillDir(t, sourcePluginDir, "skill-a")
	makeSkillDir(t, sourcePluginDir, "skill-b")

	if err := CreateSkillLink(filepath.Join(sourcePluginDir, "skill-a"), agentSkillsDir, "skill-a", false); err != nil {
		t.Fatal(err)
	}
	// skill-b intentionally not installed.

	statuses := ValidateInstall("claude-code", agentSkillsDir, sourcePluginDir, []string{"skill-a", "skill-b"})
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}
	statusMap := map[string]string{}
	for _, s := range statuses {
		statusMap[s.SkillName] = s.Status
	}
	if statusMap["skill-a"] != "ok" {
		t.Errorf("skill-a: expected ok, got %q", statusMap["skill-a"])
	}
	if statusMap["skill-b"] != "missing" {
		t.Errorf("skill-b: expected missing, got %q", statusMap["skill-b"])
	}
}

func TestValidateInstall_LinkPathField(t *testing.T) {
	tmp := t.TempDir()
	sourcePluginDir := filepath.Join(tmp, "plugins")
	agentSkillsDir := filepath.Join(tmp, "agent", "skills")
	if err := os.MkdirAll(agentSkillsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	statuses := ValidateInstall("claude-code", agentSkillsDir, sourcePluginDir, []string{"auth0-react"})
	expected := filepath.Join(agentSkillsDir, "auth0-react")
	if statuses[0].LinkPath != expected {
		t.Errorf("expected LinkPath %q, got %q", expected, statuses[0].LinkPath)
	}
}

func TestValidateInstall_EmptySkillsList(t *testing.T) {
	tmp := t.TempDir()
	statuses := ValidateInstall("claude-code", tmp, tmp, []string{})
	if len(statuses) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(statuses))
	}
}

func TestValidateInstall_AbsentAgentSkillsDir(t *testing.T) {
	tmp := t.TempDir()
	sourcePluginDir := filepath.Join(tmp, "plugins")
	// agentSkillsDir does not exist — all requested skills should return "missing".
	agentSkillsDir := filepath.Join(tmp, "nonexistent", "agent", "skills")

	statuses := ValidateInstall("claude-code", agentSkillsDir, sourcePluginDir, []string{"auth0-react", "auth0-nextjs"})
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}
	for _, s := range statuses {
		if s.Status != "missing" {
			t.Errorf("skill %q: expected missing when agentSkillsDir absent, got %q", s.SkillName, s.Status)
		}
	}
}

func TestValidateInstall_CopyMode_InvalidSkillMd(t *testing.T) {
	tmp := t.TempDir()
	sourcePluginDir := filepath.Join(tmp, "plugins")
	agentSkillsDir := filepath.Join(tmp, "agent", "skills")

	// Create source dir with a SKILL.md that has no frontmatter.
	skillSrc := filepath.Join(sourcePluginDir, "auth0-spa")
	if err := os.MkdirAll(skillSrc, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillSrc, "SKILL.md"), []byte("no frontmatter here"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := CreateSkillLink(skillSrc, agentSkillsDir, "auth0-spa", true); err != nil {
		t.Fatalf("CreateSkillLink (copy): %v", err)
	}

	statuses := ValidateInstall("cursor", agentSkillsDir, sourcePluginDir, []string{"auth0-spa"})
	if statuses[0].Status != "invalid_skill" {
		t.Errorf("expected invalid_skill for copy with no frontmatter, got %q", statuses[0].Status)
	}
	if statuses[0].Error == "" {
		t.Error("expected non-empty Error field")
	}
}

func TestValidateInstall_RegularFileAtLinkPath(t *testing.T) {
	tmp := t.TempDir()
	sourcePluginDir := filepath.Join(tmp, "plugins")
	agentSkillsDir := filepath.Join(tmp, "agent", "skills")
	if err := os.MkdirAll(agentSkillsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Place a regular file where a skill directory should be.
	linkPath := filepath.Join(agentSkillsDir, "auth0-react")
	if err := os.WriteFile(linkPath, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	statuses := ValidateInstall("claude-code", agentSkillsDir, sourcePluginDir, []string{"auth0-react"})
	if statuses[0].Status != "invalid_skill" {
		t.Errorf("expected invalid_skill for regular file at linkPath, got %q", statuses[0].Status)
	}
	if statuses[0].Error == "" {
		t.Error("expected non-empty Error field")
	}
}

func TestValidateInstall_NoFrontmatter_ClearError(t *testing.T) {
	tmp := t.TempDir()
	sourcePluginDir := filepath.Join(tmp, "plugins")
	agentSkillsDir := filepath.Join(tmp, "agent", "skills")

	// Skill dir has SKILL.md but with no --- delimiters.
	skillSrc := filepath.Join(sourcePluginDir, "auth0-vue")
	if err := os.MkdirAll(skillSrc, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillSrc, "SKILL.md"), []byte("# Just a heading, no frontmatter\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CreateSkillLink(skillSrc, agentSkillsDir, "auth0-vue", false); err != nil {
		t.Fatalf("CreateSkillLink: %v", err)
	}

	statuses := ValidateInstall("claude-code", agentSkillsDir, sourcePluginDir, []string{"auth0-vue"})
	if statuses[0].Status != "invalid_skill" {
		t.Errorf("expected invalid_skill, got %q", statuses[0].Status)
	}
	// Error should mention missing frontmatter, not a name mismatch.
	if statuses[0].Error == "" {
		t.Error("expected non-empty Error")
	}
	const wantSubstr = "no frontmatter"
	if !contains(statuses[0].Error, wantSubstr) {
		t.Errorf("expected error to contain %q, got %q", wantSubstr, statuses[0].Error)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
