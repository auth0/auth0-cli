package skills

import (
	"fmt"
	"os"
	"path/filepath"
)

// SkillInstallStatus reports the installation state of one (agent, skill) pair.
type SkillInstallStatus struct {
	SkillName string
	AgentID   string
	LinkPath  string
	Status    string // "ok" | "missing" | "broken_symlink" | "invalid_skill" | "copy" | "unknown"
	Error     string
}

// ValidateInstall checks each skill in skills for the given agent.
// sourcePluginDir is the directory containing skill subdirectories (pluginDir/skills/).
func ValidateInstall(agentID, agentSkillsDir, sourcePluginDir string, skills []string) []SkillInstallStatus {
	out := make([]SkillInstallStatus, 0, len(skills))
	for _, skillName := range skills {
		expectedSource := filepath.Join(sourcePluginDir, skillName)
		linkPath := filepath.Join(agentSkillsDir, skillName)

		s := SkillInstallStatus{
			SkillName: skillName,
			AgentID:   agentID,
			LinkPath:  linkPath,
		}

		switch CheckSkillLink(agentSkillsDir, skillName, expectedSource) {
		case "missing":
			s.Status = "missing"
		case "broken":
			s.Status = "broken_symlink"
			s.Error = "symlink target does not exist or is inaccessible"
		case "wrong_target":
			s.Status = "broken_symlink"
			s.Error = "symlink points to wrong target"
		case "ok":
			if err := checkSkillMeta(expectedSource, skillName); err != nil {
				s.Status = "invalid_skill"
				s.Error = err.Error()
			} else {
				s.Status = "ok"
			}
		case "copy":
			fi, statErr := os.Stat(linkPath)
			if statErr != nil || !fi.IsDir() {
				s.Status = "invalid_skill"
				s.Error = fmt.Sprintf("%s is a regular file, not a skill directory", linkPath)
			} else if err := checkSkillMeta(linkPath, skillName); err != nil {
				s.Status = "invalid_skill"
				s.Error = err.Error()
			} else {
				s.Status = "copy"
			}
		default:
			s.Status = "unknown"
			s.Error = "unexpected link state"
		}

		out = append(out, s)
	}
	return out
}

// checkSkillMeta verifies that skillDir contains a readable SKILL.md whose name field matches skillName.
func checkSkillMeta(skillDir, skillName string) error {
	meta, err := ParseSkillMeta(skillDir)
	if err != nil {
		return fmt.Errorf("read SKILL.md: %w", err)
	}
	if meta.Name == "" {
		return fmt.Errorf("SKILL.md has no frontmatter or empty name field")
	}
	if meta.Name != skillName {
		return fmt.Errorf("SKILL.md name %q does not match directory name %q", meta.Name, skillName)
	}
	return nil
}
