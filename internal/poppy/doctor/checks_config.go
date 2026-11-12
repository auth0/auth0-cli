package doctor

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/auth0/auth0-cli/internal/poppy/config"
	"github.com/auth0/auth0-cli/internal/poppy/setup"
)

// configSource is the document that D14 compares with the tenant.
type configSource struct {
	// Name says which document it is, in the messages: the config file or the
	// poppy.json the website serves.
	name string
	// Repair is how to bring the document back in line with the tenant.
	repair fix
}

// checkConfigFile runs D14: every protocol value of the config file must match
// the one derived from the tenant. The file names no API identifier and no web
// app, so the APIs are the Poppy APIs of the tenant that the apis entries are at,
// and the app is the only one that has the browser join on.
func (r *runner) checkConfigFile(ctx context.Context, t *Tenant, src configSource, file *config.Config, fileErr error, c *collector) {
	name, repair := src.name, src.repair

	if fileErr != nil {
		c.warn("D14.unreadable_file", "", fmt.Sprintf("cannot compare %s with the tenant: %v", name, fileErr), repair)
		return
	}

	// A part that the file turns off is not read from the tenant.
	in := setup.Inputs{ForceConfig: true, NoAPI: file.Auth.Disabled, NoWeb: file.Web.Disabled}

	var (
		identifiers []string
		app         string
	)

	if !in.NoAPI {
		poppy, err := t.State.ListPoppyAPIs(ctx)
		if err != nil {
			r.unreadable(c, "D14", "", name, scopeConfigFile, err)
			return
		}

		if identifiers, err = setup.ResolveAPIs(file, poppy); err != nil {
			var input *setup.InputError
			if errors.As(err, &input) && input.Reason == setup.ReasonAPINotFound && len(file.APIs) > 0 && len(poppy) > 0 {
				// Naming the API would not help: the next run could not find the entry either.
				c.warn("D14.entry_not_found", "", fmt.Sprintf("cannot compare %s with the tenant: no apis entry is on the host of a Poppy API", name),
					hint("The CLI finds the entry of an API in poppy.json by the host of its identifier. Serve the OpenAPI document of each Poppy API on the host of the API."))
				return
			}

			r.warnCannotCompare(c, name, err, repair)
			return
		}
	}

	if !in.NoWeb {
		apps, err := t.State.BrowserJoinApps(ctx)
		if err != nil {
			r.unreadable(c, "D14", "", name, scopeConfigFile, err)
			return
		}

		if app, err = setup.ResolveApp(apps); err != nil {
			r.warnCannotCompare(c, name, err, repair)
			return
		}
	}

	state, err := t.State.Read(ctx, identifiers, app)
	if err != nil {
		r.unreadable(c, "D14", "", name, scopeConfigFile, err)
		return
	}

	state.File = file

	// The issuer is compared, not enforced, so a file of another issuer shows up as a difference.
	derived, err := setup.Derive(state, in)
	if err != nil {
		r.warnCannotCompare(c, name, err, repair)
		return
	}

	// A file of a newer minor version is compatible with the one this CLI writes, so
	// its version is not a difference.
	changes := slices.DeleteFunc(config.Diff(file, derived), func(change config.Change) bool {
		return change.Path == "protocol_version" && change.Kind == config.ChangeChanged &&
			config.Understands(change.Old) && !slices.Contains(config.SupportedVersions, change.Old)
	})

	for _, change := range changes {
		// An older version is understood, so it is only stale, not a mismatch.
		if change.Path == "protocol_version" && change.Kind == config.ChangeChanged && slices.Contains(config.SupportedVersions, change.Old) {
			c.warn("D14.stale_version", "", fmt.Sprintf("%s is a protocol %s file, which is stale: this CLI writes %s", name, change.Old, change.New), repair)
			continue
		}

		switch change.Kind {
		case config.ChangeChanged:
			c.warn("D14.value_differs", "", fmt.Sprintf("%s is %q in %s, and the tenant has %q", change.Path, change.Old, name, change.New), repair)
		case config.ChangeAdded:
			c.warn("D14.value_missing", "", fmt.Sprintf("%s is missing from %s, and the tenant has %q", change.Path, name, change.New), repair)
		case config.ChangeRemoved:
			c.warn("D14.value_extra", "", fmt.Sprintf("%s is %q in %s, and the tenant has no such value", change.Path, change.Old, name), repair)
		}
	}

	if len(changes) == 0 {
		c.pass("D14.matches", "", name+" matches the tenant")
	}
}

// warnCannotCompare reports that the file cannot be compared with the tenant. An
// error that carries its own FIX line (the API or the web app is not known) gives
// that line as the hint next to the command, since it says how to name the
// missing one.
func (r *runner) warnCannotCompare(c *collector, name string, err error, repair fix) {
	var input *setup.InputError
	if errors.As(err, &input) {
		c.warn("D14.cannot_compare", "", fmt.Sprintf("cannot compare %s with the tenant: %s", name, input.Message),
			fix{command: repair.command, hint: strings.TrimSpace(input.Fix)})
		return
	}

	c.warn("D14.cannot_compare", "", fmt.Sprintf("cannot compare %s with the tenant: %v", name, err), repair)
}
