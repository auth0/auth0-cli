package approvals

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/store"
)

// Session says who is behind the command. The zero value is not a person, so it
// cannot approve anything.
type Session struct {
	// Interactive is true when the command runs at a terminal where a person can
	// answer, and --no-input is not set.
	Interactive bool

	// Agent is true in agent mode. An agent never approves a plugin, even at a
	// terminal.
	Agent bool
}

// Prompter reads the answer of the person.
type Prompter interface {
	// ReadLine shows the prompt and returns the line the person typed, without
	// the line break. It returns an error, such as io.EOF, when there is no
	// answer.
	ReadLine(prompt string) (string, error)
}

// Request is what a person is asked to approve.
type Request struct {
	ID      string
	Version string

	// Old holds the permissions and commands that are installed now, and is nil
	// for a first approval. The screen of an update only shows how New differs.
	Old         *plugins.Permissions
	OldCommands []plugins.ConsentCommand

	New         plugins.Permissions
	NewCommands []plugins.ConsentCommand
}

// Consenter asks a person to approve plugins on one tenant. The fields must all
// be set. It is not safe for concurrent use. Use one Consenter for each install
// or update operation, because what the person approved through ConsentFunc is
// only kept until the next Record, which writes it for the tenant and version
// that the person was asked about, and only for what was installed.
type Consenter struct {
	Tenant    string
	Approvals *Approvals
	Session   Session
	Prompter  Prompter

	// Out receives the consent screen.
	Out io.Writer

	// What the person approved through ConsentFunc that Record has not written
	// yet, for each plugin ID.
	granted map[string]grant
}

// grant is what a person approved through ConsentFunc.
type grant struct {
	// The tenant the person was asked about, folded to lower case.
	tenant string

	// The version the person was shown, which Record compares with the version
	// that was installed.
	version string

	// The consent hash of what the person was shown.
	hash string
}

// Ask shows the request to the person and records the approval when they say
// yes. It returns ErrAgentMode or ErrNotInteractive, before it shows or reads
// anything, when the session is not a person, and ErrDeclined when the person
// does not approve.
func (c *Consenter) Ask(ctx context.Context, req Request) error {
	hash, err := c.confirm(req)
	if err != nil {
		return err
	}

	return c.Approvals.Approve(ctx, c.Tenant, req.ID, hash)
}

// ConsentFunc returns the function for store.Options.Consent. It asks nothing
// when the new permissions are already approved on the tenant, and otherwise
// asks the person. It does not record the approval, because the install can
// still fail: after the install or update, call Record with the results of the
// plugins that succeeded. The approval is for the tenant and the version that the
// person was asked about. Calling the function again for a plugin forgets what
// the person approved for it before.
func (c *Consenter) ConsentFunc() store.ConsentFunc {
	return func(_ context.Context, change store.Change) error {
		delete(c.granted, change.ID)

		tenant, err := foldTenant(c.Tenant)
		if err != nil {
			return err
		}

		hash := permissionsHash(change.New, change.NewCommands)

		approved, err := c.Approvals.Approved(c.Tenant, change.ID, hash)
		if err != nil {
			return err
		}

		if approved {
			return nil
		}

		req := Request{ID: change.ID, Version: change.NewVersion, New: change.New, NewCommands: change.NewCommands}
		if change.OldVersion != "" {
			req.Old, req.OldCommands = &change.Old, change.OldCommands
		}

		if _, err = c.confirm(req); err != nil {
			return err
		}

		if c.granted == nil {
			c.granted = map[string]grant{}
		}

		c.granted[change.ID] = grant{tenant: tenant, version: change.NewVersion, hash: hash}

		return nil
	}
}

// Record writes the approvals that the person gave through ConsentFunc, for the
// results that Install or Update returned. An approval is written only for a
// result with the same plugin ID and the same version as the one the person was
// asked about, because another process can change the plugin while the person
// answers, and the version that ends up installed is then not the one that was
// approved. For store.UpdateAll, pass the Result of each entry that has no error
// and was not skipped. A result that is empty, for another version, or for a
// plugin that was not approved is ignored. Every other approval that
// ConsentFunc collected is discarded, so a plugin that failed to install is
// never approved, not even by a later call. It does nothing when there is
// nothing to write.
func (c *Consenter) Record(ctx context.Context, installed ...store.Result) error {
	granted := c.granted
	c.granted = nil

	var errs []error

	for _, result := range installed {
		given, found := granted[result.ID]
		if !found || given.version != result.Version {
			continue
		}

		delete(granted, result.ID)

		errs = append(errs, c.Approvals.Approve(ctx, given.tenant, result.ID, given.hash))
	}

	return errors.Join(errs...)
}

// permissionsHash returns the consent hash of what the person is shown.
func permissionsHash(permissions plugins.Permissions, commands []plugins.ConsentCommand) string {
	return plugins.ConsentHash(plugins.NormalizePermissions(permissions), commands)
}

// confirm shows the request and returns the consent hash of it when the person
// says yes. It is the only place that decides whether a person is there, and
// whether the screen shows the full permissions or only the change since the
// installed version, which it does only when the tenant approved that version and
// the change has something to show.
func (c *Consenter) confirm(req Request) (string, error) {
	switch {
	case c.Session.Agent:
		return "", ErrAgentMode
	case !c.Session.Interactive:
		return "", ErrNotInteractive
	}

	hash := permissionsHash(req.New, req.NewCommands)

	tenant, err := checkApproval(c.Tenant, req.ID, hash)
	if err != nil {
		return "", err
	}

	// The installed version is not known to be approved on this tenant, so a
	// difference to it would not tell the person what they are approving: they
	// see everything.
	if req.Old != nil {
		known, err := c.Approvals.Approved(c.Tenant, req.ID, permissionsHash(*req.Old, req.OldCommands))
		if err != nil {
			return "", err
		}

		if !known {
			req.Old, req.OldCommands = nil, nil
		}
	}

	// A difference that shows no item would tell the person that nothing changed
	// while the hash they approve is another one, so they see everything.
	if req.Old != nil && permissionsHash(*req.Old, req.OldCommands) != hash && !showsChange(req) {
		req.Old, req.OldCommands = nil, nil
	}

	// A blank line sets the screen off from the lines the caller printed before it,
	// such as the name and version of the plugin.
	if _, err := io.WriteString(c.Out, "\n"+render(tenant, req)); err != nil {
		return "", fmt.Errorf("failed to show the permissions: %w", err)
	}

	// The person can ask for the exact scopes once, and only when there are some
	// to show.
	exact := renderExactScopes(req)

	plain := fmt.Sprintf("Approve for %s? [y/N] ", tenant)

	prompt := plain
	if exact != "" {
		prompt = fmt.Sprintf("Approve for %s? [y/N/d]  (d shows the exact scopes) ", tenant)
	}

	for {
		line, err := c.Prompter.ReadLine(prompt)
		if errors.Is(err, io.EOF) {
			return "", ErrDeclined
		}

		if err != nil {
			return "", fmt.Errorf("%w (%w)", ErrDeclined, err)
		}

		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			return hash, nil
		case "d":
			if exact != "" {
				if _, err := io.WriteString(c.Out, exact); err != nil {
					return "", fmt.Errorf("failed to show the permissions: %w", err)
				}

				exact, prompt = "", plain

				continue
			}
		}

		return "", ErrDeclined
	}
}
