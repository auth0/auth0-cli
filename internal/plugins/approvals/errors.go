package approvals

import (
	"fmt"
	"strings"
)

// approvalsError is an error that is a constant, so that the sentinel errors
// below cannot be changed. The messages of the sentinels are sentences that are
// shown to users as they are.
type approvalsError string

// Error returns the message of the error.
func (e approvalsError) Error() string {
	return string(e)
}

// The errors that approving and checking plugins can return. Use errors.Is to
// tell them apart, as some of them wrap more detail.
const (
	// ErrDamaged means approvals.json cannot be trusted: it cannot be read, is
	// too large, is not a regular file, or does not follow the format. The file
	// is never replaced automatically, because that would drop approvals
	// without telling the user.
	ErrDamaged = approvalsError("The plugin approvals file is damaged.")

	// ErrAgentMode means an agent tried to approve a plugin. Only a person can.
	ErrAgentMode = approvalsError("A plugin cannot be approved in agent mode. Approve it in a terminal, or pin it with " + PinsEnv + ".")

	// ErrNotInteractive means the session has no person to ask. Only a person
	// can approve a plugin.
	ErrNotInteractive = approvalsError("A plugin can only be approved in an interactive terminal. Approve it in a terminal, or pin it with " + PinsEnv + ".")

	// ErrDeclined means the person did not approve the plugin.
	ErrDeclined = approvalsError("The plugin was not approved.")

	// ErrBadPins means the value of PinsEnv is not a list of plugin pins.
	ErrBadPins = approvalsError("The value of " + PinsEnv + " is not valid.")

	// ErrBusy means another auth0 process is changing the approvals and did not
	// finish in time.
	ErrBusy = approvalsError("Another auth0 process is changing the plugin approvals.")
)

// NotApprovedError means a plugin is not approved to run on a tenant.
type NotApprovedError struct {
	Tenant string
	ID     string

	// Changed is true when the tenant approved another permission set of the
	// plugin earlier, so the plugin now asks for something that was not approved.
	Changed bool
}

// Error returns the message and the command that approves the plugin.
func (e *NotApprovedError) Error() string {
	name := e.ID
	if _, after, found := strings.Cut(e.ID, "/"); found {
		name = after
	}

	state := "is not approved for " + e.Tenant
	if e.Changed {
		state = "changed what it asks for since you approved it for " + e.Tenant
	}

	return fmt.Sprintf("%s %s. Run: auth0 plugin approve %s --tenant %s", name, state, e.ID, e.Tenant)
}
