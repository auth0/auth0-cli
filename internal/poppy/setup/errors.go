package setup

import "fmt"

// InputError is a problem with the inputs or with what the inputs point to. It
// stops the command before anything is changed.
type InputError struct {
	// Reason is a stable, machine-readable identifier.
	Reason  string
	Message string
	// Fix says what to do about it. It can be empty.
	Fix string
	// Candidates are the values the user can choose from, such as the identifiers
	// of the APIs a name could mean. It is empty when there is no choice to make.
	Candidates []string
}

// Error returns the message followed by the FIX line.
func (e *InputError) Error() string {
	if e.Fix == "" {
		return e.Message
	}

	return e.Message + "\n  FIX  " + e.Fix
}

func invalidFlag(flag, format string, args ...any) *InputError {
	return &InputError{Reason: ReasonInvalidFlag, Message: flag + ": " + fmt.Sprintf(format, args...)}
}
