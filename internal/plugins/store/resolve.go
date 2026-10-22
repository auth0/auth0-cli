package store

import (
	"context"
	"fmt"
	"slices"

	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
)

// plan is what an install is going to do, worked out from the index before
// anything is downloaded.
type plan struct {
	id       string
	version  string
	record   registry.VersionRecord
	artifact registry.Artifact
	policy   registry.Policy

	// True when an exact version was asked for.
	pinned bool
}

// resolve works out what to install from the index, and fails for every case
// that the registry already rules out.
func (s *Store) resolve(ctx context.Context, index *registry.Index, id, requested string) (plan, error) {
	plugin, ok := index.FindPlugin(id)
	if !ok {
		return plan{}, &NotAvailableError{ID: id}
	}

	records, err := s.versions(ctx, plugin)
	if err != nil {
		return plan{}, err
	}

	version := requested
	if version == "" {
		version = plugin.Latest
	}

	// A revoked version is never offered as one to install.
	installable := func(record registry.VersionRecord) bool {
		_, revoked := index.Revoked(id, record.Version)

		return !revoked
	}

	position := slices.IndexFunc(records, func(record registry.VersionRecord) bool { return record.Version == version })
	if position < 0 {
		return plan{}, &NotAvailableError{ID: id, Version: version, Available: versionsWhere(records, installable)}
	}

	record := records[position]

	if !isCanonicalVersion(record.Version) {
		return plan{}, fmt.Errorf("the registry lists %s with the version %q, which is not in the form MAJOR.MINOR.PATCH", id, record.Version)
	}

	if revocation, revoked := index.Revoked(id, version); revoked {
		return plan{}, &RevokedError{ID: id, Version: version, Reason: revocation.Reason}
	}

	if record.Engines != nil {
		met, err := plugins.EnginesSatisfied(record.Engines.Auth0CLI, s.CLIVersion)
		if err != nil {
			return plan{}, fmt.Errorf("%s %s: %w", id, version, err)
		}

		if !met {
			return plan{}, &EnginesError{ID: id, Version: version, Requires: record.Engines.Auth0CLI, CLIVersion: s.CLIVersion}
		}
	}

	// A machine that is not a plugin platform gives an empty platform, which no
	// artifact is built for, so nothing is installed on it.
	machine := currentPlatform()

	artifact, found := artifactFor(record, machine)
	if !found {
		return plan{}, &NoBuildError{
			ID: id, Version: version, Machine: machineName(machine), Platforms: record.Platforms,
			Versions: versionsWhere(records, func(other registry.VersionRecord) bool {
				_, has := artifactFor(other, machine)

				return has && installable(other)
			}),
		}
	}

	if artifact.Format != registry.ArtifactFormatTarGz && artifact.Format != registry.ArtifactFormatZip {
		return plan{}, &UnsupportedFormatError{ID: id, Version: version, Format: string(artifact.Format)}
	}

	policy := index.Policy.WithDefaults()
	if artifact.Size > policy.MaxDownloadBytes {
		return plan{}, fmt.Errorf("%w (%s %s is %d bytes and the registry allows %d)", ErrDownloadTooLarge, plugins.NameOf(id), version, artifact.Size, policy.MaxDownloadBytes)
	}

	return plan{id: id, version: version, record: record, artifact: artifact, policy: policy, pinned: requested != ""}, nil
}

// versions returns the versions of a plugin.
func (s *Store) versions(ctx context.Context, plugin registry.Plugin) ([]registry.VersionRecord, error) {
	if s.Versions != nil {
		return s.Versions.Versions(ctx, plugin)
	}

	if plugin.Ref != nil {
		return nil, fmt.Errorf("the versions of %s are in a separate file, and the store was given no way to read it", plugin.ID)
	}

	return plugin.Versions, nil
}

// artifactFor returns the artifact of the record that is built for the platform.
// The empty platform of an unknown machine matches no artifact.
func artifactFor(record registry.VersionRecord, platform plugins.Platform) (registry.Artifact, bool) {
	for _, artifact := range record.Artifacts {
		if built, ok := artifact.Platform(); ok && built == platform {
			return artifact, true
		}
	}

	return registry.Artifact{}, false
}

// versionsWhere returns the versions of the records that match, latest first.
func versionsWhere(records []registry.VersionRecord, match func(registry.VersionRecord) bool) []string {
	var versions []string

	for _, record := range records {
		if match(record) {
			versions = append(versions, record.Version)
		}
	}

	slices.SortFunc(versions, func(a, b string) int { return -compareVersions(a, b) })

	return versions
}
