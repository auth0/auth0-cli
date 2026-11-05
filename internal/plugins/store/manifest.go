package store

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/auth0/auth0-cli/internal/plugins"
)

// supportedAPIVersion is the plugin API version that this CLI runs.
const supportedAPIVersion = 1

// checkManifest checks the plugin.json that came out of the archive, and
// compares it with the registry record of the version. The manifest decides what
// the plugin can do, so it must be the one the registry showed the user.
func checkManifest(data []byte, p plan) error {
	var header struct {
		APIVersion int `json:"apiVersion"`
	}

	if json.Unmarshal(data, &header) == nil && header.APIVersion > supportedAPIVersion {
		return &UnsupportedAPIVersionError{ID: p.id, Version: p.version, APIVersion: header.APIVersion}
	}

	manifest, err := plugins.ParseInstalledManifest(data)
	if err != nil {
		return fmt.Errorf("%w (%s is not valid: %w)", ErrInvalidArchive, manifestFileName, err)
	}

	mismatch := func(format string, args ...any) error {
		return fmt.Errorf("%w (%s)", ErrArtifactMismatch, fmt.Sprintf(format, args...))
	}

	record := p.record

	switch {
	case manifest.ID != p.id:
		return mismatch("%s has the id %q and the registry lists %q", manifestFileName, manifest.ID, p.id)
	case manifest.Version != record.Version:
		return mismatch("%s has the version %q and the registry lists %q", manifestFileName, manifest.Version, record.Version)
	case !samePlatforms(manifest.Platforms, record.Platforms):
		return mismatch("%s lists the platforms %v and the registry lists %v", manifestFileName, manifest.Platforms, record.Platforms)
	case !sameEngines(manifest.Engines, record.Engines):
		return mismatch("%s has other engines than the registry lists", manifestFileName)
	case !sameCommands(manifest.Commands, record.Commands):
		return mismatch("%s has other commands, or other interactive values, than the registry lists", manifestFileName)
	case manifest.PermissionsHash() != record.PermissionsHash:
		return mismatch("%s asks for other permissions than the registry lists", manifestFileName)
	}

	return nil
}

// samePlatforms reports whether two lists hold the same platforms.
func samePlatforms(a, b []plugins.Platform) bool {
	return slices.Equal(slices.Sorted(slices.Values(a)), slices.Sorted(slices.Values(b)))
}

// sameEngines reports whether two engines are both absent or both the same.
func sameEngines(a, b *plugins.Engines) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}

// sameCommands reports whether the commands of the manifest are those of the
// record, with the same interactive values.
func sameCommands(commands []plugins.Command, consent []plugins.ConsentCommand) bool {
	if len(commands) != len(consent) {
		return false
	}

	for _, command := range commands {
		found := slices.ContainsFunc(consent, func(c plugins.ConsentCommand) bool {
			return c.Name == command.Name && c.Interactive == command.Interactive
		})
		if !found {
			return false
		}
	}

	return true
}
