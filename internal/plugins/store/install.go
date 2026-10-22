package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
)

// Install installs a plugin, and makes the version the active one.
//
// The index must be one that registry.Client returned, because the signature,
// the expiry and the schema of the index are not checked again. An empty version
// means the latest one. Asking for an exact version pins the plugin, so that
// UpdateAll leaves it alone.
//
// Install fails, before anything is downloaded, with a *NotAvailableError when
// the registry does not list the plugin or version, a *RevokedError, an
// *EnginesError when the version needs another auth0 CLI, a *NoBuildError when
// it has no build for this machine, or an *UnsupportedFormatError when it is not
// a tar.gz or zip archive. Then opts.Consent is called when the plugin is new or
// its permissions differ from the active version, and a refusal is a
// *DeclinedError. When another auth0 process changed the plugin while the
// consent was being asked, the answer is not applied, and the error is
// ErrChanged.
//
// The rest runs with the store lock held. A version that is already installed
// and verifies is not downloaded again: it is made the active one, which is how
// a rollback works. Otherwise the artifact is downloaded, unpacked, compared
// with the registry record and moved into the store, and the lockfile is
// switched to it in one step. The previous active version stays on disk, and
// older versions are deleted. Any failure leaves the installed plugin as it was,
// also when the version being installed replaces a different build of itself.
func (s *Store) Install(ctx context.Context, index *registry.Index, id, version string, opts Options) (Result, error) {
	return s.install(ctx, index, id, version, opts, modeInstall)
}

// Update moves an installed plugin to the latest version in the index and
// clears its pin. It fails with a *NotInstalledError when the plugin is not
// installed, and otherwise behaves like Install of the latest version, except
// that it never goes back: when the active version is the latest or newer than
// the latest, nothing is downloaded or asked and the active version stays.
func (s *Store) Update(ctx context.Context, index *registry.Index, id string, opts Options) (Result, error) {
	return s.install(ctx, index, id, "", opts, modeUpdate)
}

// mode says which operation an install is part of.
type mode int

const (
	modeInstall mode = iota
	modeUpdate

	// An update of every plugin, which leaves a pinned plugin alone.
	modeUpdateAll
)

// errPinned means an update of every plugin skipped a pinned plugin.
var errPinned = errors.New("the plugin is pinned")

// basis is what the consent of an install was judged against: the active version
// of the plugin and its permissions hash, or nothing when it is not installed.
type basis struct {
	active string
	hash   string
}

// basisOf returns the basis of an entry, which may be nil.
func basisOf(e *entry) basis {
	if e == nil {
		return basis{}
	}

	return basis{active: e.Active, hash: e.Versions[e.Active].PermissionsHash}
}

// checkTarget fails for an update of a plugin that is not installed, and with
// errPinned for an update of every plugin when this one is pinned.
func checkTarget(id string, e *entry, m mode) error {
	switch {
	case m != modeInstall && e == nil:
		return &NotInstalledError{ID: id}
	case m == modeUpdateAll && e.Pinned:
		return errPinned
	}

	return nil
}

// alreadyCurrent reports whether an update has nothing to do, because the active
// version is the version of the plan or a newer one.
func alreadyCurrent(m mode, e *entry, p plan) bool {
	return m != modeInstall && compareVersions(p.version, e.Active) <= 0
}

// install is Install and Update. The lockfile is read twice: before the lock,
// to decide whether to ask for consent, and under the lock, where it is read
// again and the decision is checked.
func (s *Store) install(ctx context.Context, index *registry.Index, id, requested string, opts Options, m mode) (Result, error) {
	if _, err := s.pluginPath(id); err != nil {
		return Result{}, err
	}

	before, err := s.readLockfile()
	if err != nil {
		return Result{}, err
	}

	seen := before.Plugins[id]

	if err := checkTarget(id, seen, m); err != nil {
		return Result{}, err
	}

	p, err := s.resolve(ctx, index, id, requested)
	if err != nil {
		return Result{}, err
	}

	if opts.Consent != nil && !alreadyCurrent(m, seen, p) {
		if err := s.askConsent(ctx, p, seen, opts.Consent); err != nil {
			return Result{}, err
		}
	}

	release, err := s.lock(ctx)
	if err != nil {
		return Result{}, err
	}
	defer release()

	current, err := s.readLockfile()
	if err != nil {
		return Result{}, err
	}

	e := current.Plugins[id]

	if err := checkTarget(id, e, m); err != nil {
		return Result{}, err
	}

	if opts.Consent != nil && basisOf(e) != basisOf(seen) {
		return Result{}, ErrChanged
	}

	if alreadyCurrent(m, e, p) {
		p.version = e.Active

		return s.commit(current, p, nil)
	}

	tmp, err := s.resetTmp()
	if err != nil {
		return Result{}, err
	}

	s.removeAsides(id, e)

	// A version that is already on disk and verifies is not downloaded again. It
	// must be the artifact and the permissions that the registry lists now.
	if e != nil {
		installed := e.Versions[p.version]
		if installed != nil && installed.ArtifactSHA256 == p.artifact.SHA256 && installed.PermissionsHash == p.record.PermissionsHash {
			if _, err := s.verifyVersion(id, e, p.version); err == nil {
				return s.commit(current, p, nil)
			}
		}
	}

	work, err := os.MkdirTemp(tmp, "install-")
	if err != nil {
		return Result{}, fmt.Errorf("failed to create a directory in %s: %w", tmp, err)
	}
	defer os.RemoveAll(work)

	installed, unpacked, err := s.fetch(ctx, p, work)
	if err != nil {
		return Result{}, err
	}

	final := s.versionPath(id, p.version)

	if err := os.MkdirAll(filepath.Dir(final), dirPerm); err != nil {
		return Result{}, fmt.Errorf("failed to create the directory of %s: %w", id, err)
	}

	// Whatever is at the final path stays where it is until the lockfile points at
	// the new directory. It is a version that failed verification, a leftover, or
	// a different build of this version that the registry published again, and
	// the lockfile may still list it, so it is only moved aside.
	aside, err := moveAside(final, p.version)
	if err != nil {
		return Result{}, err
	}

	if err := renameFile(unpacked, final); err != nil {
		return Result{}, putBack(fmt.Errorf("failed to move the plugin into %s: %w", final, err), final, aside)
	}

	result, err := s.commit(current, p, installed)
	if err != nil {
		return Result{}, putBack(err, final, aside)
	}

	if aside != "" {
		_ = os.RemoveAll(aside)
	}

	return result, nil
}

// fetch downloads the artifact of the plan into work, unpacks it, and checks it
// against the registry record. It returns what to record for the version and
// the directory it was unpacked into.
func (s *Store) fetch(ctx context.Context, p plan, work string) (*installedVersion, string, error) {
	archive, err := s.download(ctx, p.artifact, filepath.Join(work, "artifact"))
	if err != nil {
		return nil, "", err
	}
	defer archive.Close()

	unpacked := filepath.Join(work, "unpacked")

	contents, err := extract(archive, p.artifact.Format, p.artifact.Bin, unpacked, p.policy.MaxDownloadBytes)
	if err != nil {
		return nil, "", err
	}

	if err := checkManifest(contents.manifest, p); err != nil {
		return nil, "", err
	}

	return &installedVersion{
		Bin:             p.artifact.Bin,
		BinSHA256:       contents.binSHA256,
		ManifestSHA256:  contents.manifestSHA256,
		ArtifactSHA256:  p.artifact.SHA256,
		PermissionsHash: p.record.PermissionsHash,
		Size:            contents.binSize,
		InstalledAt:     time.Now().UTC(),
	}, unpacked, nil
}

// commit makes the version of the plan the active one in the lockfile, which it
// writes, and then deletes the versions that are neither the active one nor the
// previous one. The installed argument is what to record for a version that was just
// installed, and is nil for a version that was already on disk.
func (s *Store) commit(current *lockfile, p plan, installed *installedVersion) (Result, error) {
	e := current.Plugins[p.id]
	if e == nil {
		e = &entry{Registry: registryName(p.id), Name: plugins.NameOf(p.id), Versions: map[string]*installedVersion{}}
		current.Plugins[p.id] = e
	}

	result := Result{ID: p.id, Version: p.version, Previous: e.Active, Pinned: p.pinned, Action: ActionSwitched}

	switch {
	case installed != nil:
		result.Action = ActionInstalled
	case e.Active == p.version:
		result.Action = ActionUnchanged
	}

	if result.Action == ActionUnchanged && e.Pinned == p.pinned {
		return result, nil
	}

	if installed != nil {
		e.Versions[p.version] = installed
	}

	if e.Active != "" && e.Active != p.version {
		e.Previous = e.Active
	}

	e.Active = p.version
	e.Pinned = p.pinned

	var pruned []string

	for version := range e.Versions {
		if version != e.Active && version != e.Previous {
			delete(e.Versions, version)

			pruned = append(pruned, version)
		}
	}

	if err := s.writeLockfile(current); err != nil {
		return Result{}, err
	}

	// The lockfile no longer lists these, so a failure to delete one only leaves
	// unused files behind.
	for _, version := range pruned {
		_ = os.RemoveAll(s.versionPath(p.id, version))
	}

	return result, nil
}

// askConsent asks for approval of a plugin that is new or whose permissions
// change. The entry is the plugin as the lockfile has it, or nil.
func (s *Store) askConsent(ctx context.Context, p plan, e *entry, consent ConsentFunc) error {
	change := Change{ID: p.id, NewVersion: p.version, New: p.record.Permissions, NewCommands: p.record.Commands}

	if e != nil {
		if e.Versions[e.Active].PermissionsHash == p.record.PermissionsHash {
			return nil
		}

		change.OldVersion = e.Active
		change.Old, change.OldCommands = s.installedPermissions(p.id, e)
	}

	if err := consent(ctx, change); err != nil {
		return &DeclinedError{ID: p.id, Version: p.version, Err: err}
	}

	return nil
}

// installedPermissions reads the permissions and commands of the active version
// of an entry from its manifest, after checking the manifest against the hash in
// the lockfile. It returns nothing when the manifest cannot be read or was
// changed, which shows the user everything the new version asks for.
func (s *Store) installedPermissions(id string, e *entry) (plugins.Permissions, []plugins.ConsentCommand) {
	manifest, err := s.readManifest(id, e, e.Active)
	if err != nil {
		return plugins.Permissions{}, nil
	}

	return manifest.Permissions(), manifest.ConsentCommands()
}
