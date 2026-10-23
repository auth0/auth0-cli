// Package approvals records which permissions of which plugin a person approved
// on which tenant, and answers whether a plugin is approved to run.
//
// An approval is keyed on the consent hash of the plugin, so a release that asks
// for the same permissions needs no new approval and one that asks for anything
// else does. Everything fails closed: a file that cannot be trusted is an error,
// never an empty list of approvals, and only a person at an interactive
// terminal can create an approval.
package approvals

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"
	"time"
)

// Approvals is the set of approvals in one plugins directory.
type Approvals struct {
	// Dir is the plugins directory, which holds approvals.json.
	Dir string

	// Pins approve plugins on every tenant, for CI. It is what ParsePins returns
	// for the value of PinsEnv. A nil value means there are no pins.
	Pins Pins

	// Now returns the current time. When it is nil, time.Now is used.
	Now func() time.Time
}

// Record is one approval, as List returns it.
type Record struct {
	Tenant          string
	PluginID        string
	PermissionsHash string
	ApprovedAt      time.Time
}

// foldTenant returns the tenant in the form it is stored in, which is lower case,
// or an error when it is not a tenant domain.
func foldTenant(tenant string) (string, error) {
	if !tenantPattern.MatchString(tenant) {
		return "", fmt.Errorf("%q is not a tenant domain", tenant)
	}

	return strings.ToLower(tenant), nil
}

// checkID returns an error when the plugin ID is not in the form registry/name.
func checkID(pluginID string) error {
	if !idPattern.MatchString(pluginID) {
		return fmt.Errorf("%q is not a plugin ID in the form registry/name", pluginID)
	}

	return nil
}

// checkApproval validates the arguments of Approve, Approved and Check, and
// returns the tenant in the form it is stored in.
func checkApproval(tenant, pluginID, hash string) (string, error) {
	folded, err := foldTenant(tenant)
	if err != nil {
		return "", err
	}

	if err := checkID(pluginID); err != nil {
		return "", err
	}

	if !hashPattern.MatchString(hash) {
		return "", fmt.Errorf("%q is not a consent hash, sha256: followed by a lowercase hex SHA-256", hash)
	}

	return folded, nil
}

// Approve records that the permissions with the given consent hash of the plugin
// are approved for the tenant. It replaces what was approved for that plugin on
// that tenant before, so an earlier permission set stops being approved. This is
// the only function that creates an approval, and it does not check who is
// asking: the Consenter does that.
func (a *Approvals) Approve(ctx context.Context, tenant, pluginID, hash string) error {
	folded, err := checkApproval(tenant, pluginID, hash)
	if err != nil {
		return err
	}

	release, err := a.lock(ctx)
	if err != nil {
		return err
	}
	defer release()

	current, err := a.load()
	if err != nil {
		return err
	}

	if current[folded] == nil {
		current[folded] = map[string]approval{}
	}

	now := time.Now
	if a.Now != nil {
		now = a.Now
	}

	current[folded][pluginID] = approval{PermissionsHash: hash, ApprovedAt: now().UTC()}

	return a.write(current)
}

// Approved reports whether the permissions with the given consent hash of the
// plugin are approved for the tenant, either by a pin or by a recorded approval.
func (a *Approvals) Approved(tenant, pluginID, hash string) (bool, error) {
	err := a.Check(tenant, pluginID, hash)

	var notApproved *NotApprovedError
	if errors.As(err, &notApproved) {
		return false, nil
	}

	return err == nil, err
}

// Check returns nil when the plugin is approved for the tenant at the given
// consent hash. Otherwise it returns a *NotApprovedError, or an error that wraps
// ErrDamaged when the approvals file cannot be trusted.
func (a *Approvals) Check(tenant, pluginID, hash string) error {
	folded, err := checkApproval(tenant, pluginID, hash)
	if err != nil {
		return err
	}

	if a.Pins.approves(pluginID, hash) {
		return nil
	}

	current, err := a.load()
	if err != nil {
		return err
	}

	recorded, found := current[folded][pluginID]
	if found && recorded.PermissionsHash == hash {
		return nil
	}

	return &NotApprovedError{Tenant: folded, ID: pluginID, Changed: found}
}

// List returns the approvals of a plugin on every tenant, sorted by tenant.
func (a *Approvals) List(pluginID string) ([]Record, error) {
	if err := checkID(pluginID); err != nil {
		return nil, err
	}

	current, err := a.load()
	if err != nil {
		return nil, err
	}

	var records []Record

	for tenant, plugs := range current {
		if stored, found := plugs[pluginID]; found {
			records = append(records, Record{
				Tenant:          tenant,
				PluginID:        pluginID,
				PermissionsHash: stored.PermissionsHash,
				ApprovedAt:      stored.ApprovedAt,
			})
		}
	}

	slices.SortFunc(records, func(x, y Record) int { return strings.Compare(x.Tenant, y.Tenant) })

	return records, nil
}

// RemoveAll removes the approvals of a plugin on every tenant. It satisfies
// store.Approvals. Nothing is created when there is no approvals file.
func (a *Approvals) RemoveAll(pluginID string) error {
	if !idPattern.MatchString(pluginID) {
		return nil // Nothing can be stored under it.
	}

	return a.remove(func(current table) bool {
		var changed bool

		for tenant, plugs := range current {
			if _, found := plugs[pluginID]; found {
				delete(plugs, pluginID)

				changed = true
			}

			if len(plugs) == 0 {
				delete(current, tenant)
			}
		}

		return changed
	})
}

// RemoveTenant removes the approvals of every plugin on the tenant. Nothing is
// created when there is no approvals file.
func (a *Approvals) RemoveTenant(tenant string) error {
	folded, err := foldTenant(tenant)
	if err != nil {
		return nil // Nothing can be stored under it.
	}

	return a.remove(func(current table) bool {
		_, found := current[folded]
		delete(current, folded)

		return found
	})
}

// remove changes the approvals with drop, which reports whether it changed
// anything. It does nothing when there is no approvals file. A file that cannot be trusted is deleted instead: consent can only
// shrink here, and removing a plugin or logging out must not be blocked by a bad
// file. A directory that has content, in the place of the file, is not deleted
// recursively, so removing it fails with the error of the operating system, and
// the CLI never empties a directory it did not create.
func (a *Approvals) remove(drop func(current table) bool) error {
	if err := a.checkDir(); err != nil {
		return err
	}

	// With no plugins directory, or a file in its place, there is nothing to
	// remove, and the directory is not created for it. Approve still reports the
	// file in the way.
	if info, err := os.Stat(a.Dir); errors.Is(err, fs.ErrNotExist) || (err == nil && !info.IsDir()) {
		return nil
	}

	if _, err := os.Lstat(a.path()); errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	release, err := a.lock(context.Background())
	if err != nil {
		return err
	}
	defer release()

	current, err := a.load()
	if errors.Is(err, ErrDamaged) {
		if err := os.Remove(a.path()); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("failed to remove the damaged plugin approvals file: %w", err)
		}

		return nil
	}

	if err != nil {
		return err
	}

	if !drop(current) {
		return nil
	}

	return a.write(current)
}
