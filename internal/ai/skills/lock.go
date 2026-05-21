package skills

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Lock records the installed state of the auth0 agent-skills plugin.
type Lock struct {
	Repo          string    `json:"repo"`
	Ref           string    `json:"ref"`
	CommitSHA     string    `json:"commitSHA"`
	InstalledAt   time.Time `json:"installedAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	LastCheckedAt time.Time `json:"lastCheckedAt"`
	Skills        []string  `json:"skills"`
	Agents        []string  `json:"agents"`
	Scope         string    `json:"scope"` // "global" or "local".
}

// ReadLock reads the skills-lock.json at path. Returns nil, nil when the file does not exist.
func ReadLock(path string) (*Lock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var lock Lock
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, err
	}
	return &lock, nil
}

// WriteLock serialises lock as JSON and writes it to path, creating parent directories as needed.
func WriteLock(path string, lock *Lock) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
