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
	Scope         Scope     `json:"scope"`
}

// openFlockFile opens (or creates) the advisory lock file used to coordinate concurrent access.
func openFlockFile(path string) (*os.File, error) {
	return os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o644)
}

// ReadLock reads the skills-lock.json at path. Returns nil, nil when the file does not exist.
// It acquires a shared file lock for the duration of the read.
func ReadLock(path string) (*Lock, error) {
	fl, err := openFlockFile(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = releaseFlock(fl)
		fl.Close()
	}()
	if err := acquireSharedFlock(fl); err != nil {
		return nil, err
	}

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
// It acquires an exclusive file lock for the duration of the write.
func WriteLock(path string, lock *Lock) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	fl, err := openFlockFile(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = releaseFlock(fl)
		fl.Close()
	}()
	if err := acquireExclusiveFlock(fl); err != nil {
		return err
	}

	if !lock.Scope.Valid() {
		return errors.New("invalid scope: " + string(lock.Scope) + " (must be 'global' or 'local')")
	}
	os.Truncate(path, 0) // Best-effort: ensures a failed write leaves an empty file, not stale data.
	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
