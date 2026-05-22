package skills

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// SkillMeta holds the name and description extracted from a SKILL.md frontmatter.
type SkillMeta struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// ParseSkillMeta reads SKILL.md from skillDir and extracts the YAML frontmatter.
// Returns an empty SkillMeta (no error) when the file has no valid frontmatter delimiters.
func ParseSkillMeta(skillDir string) (SkillMeta, error) {
	data, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		return SkillMeta{}, err
	}

	parts := strings.SplitN(string(data), "---", 3)
	if len(parts) < 3 {
		return SkillMeta{}, nil
	}

	var meta SkillMeta
	if err := yaml.Unmarshal([]byte(parts[1]), &meta); err != nil {
		return SkillMeta{}, err
	}
	return meta, nil
}

// ListAvailableSkills walks pluginSkillsDir and returns SkillMeta for every
// subdirectory that contains a valid SKILL.md, sorted alphabetically by name.
func ListAvailableSkills(pluginSkillsDir string) ([]SkillMeta, error) {
	entries, err := os.ReadDir(pluginSkillsDir)
	if err != nil {
		return nil, err
	}

	var skills []SkillMeta
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		meta, err := ParseSkillMeta(filepath.Join(pluginSkillsDir, entry.Name()))
		if err != nil {
			continue
		}
		skills = append(skills, meta)
	}

	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})

	return skills, nil
}
