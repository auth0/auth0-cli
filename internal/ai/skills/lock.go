package skills

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type Scope string

const (
	ScopeGlobal Scope = "global"
	ScopeLocal  Scope = "local"
)

func (s Scope) Valid() bool {
	return s == ScopeGlobal || s == ScopeLocal
}

// SkillsVersionConfig records the installed state of the auth0 agent-skills plugin.
type SkillsVersionConfig struct {
	Repo          string    `json:"repo"`
	Ref           string    `json:"ref"`
	CommitSHA     string    `json:"commitSHA"`
	InstalledAt   time.Time `json:"installedAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	LastCheckedAt time.Time `json:"lastCheckedAt"`
	Skills        []string  `json:"skills"`
	Agents        []string  `json:"agents"`
	Scope         Scope     `json:"scope"`
}

// ReadLock reads the skills-lock.json at path. Returns nil, nil when the file does not exist.
func ReadLock(path string) (*SkillsVersionConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var cfg SkillsVersionConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// WriteLock serialises cfg as JSON and writes it to path, creating parent directories as needed.
func WriteLock(path string, cfg *SkillsVersionConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if !cfg.Scope.Valid() {
		return errors.New("invalid scope: " + string(cfg.Scope) + " (must be 'global' or 'local')")
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
