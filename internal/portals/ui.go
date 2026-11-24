package portals

import "errors"

// ErrCancelled is returned by a Prompter when the person at the terminal cancels a question.
var ErrCancelled = errors.New("cancelled")

// Choice is one option of a question with a single answer.
type Choice struct {
	Label string
	Hint  string
}

// ChecklistItem is one option of a question with several answers.
type ChecklistItem struct {
	Label string
	Hint  string
	// Checked starts the item selected.
	Checked bool
}

// Prompter asks the person who runs the command. Setup and audit take a Prompter, so tests can
// answer without a terminal.
type Prompter interface {
	// Ask returns the answer, or defaultValue when the answer is empty. The example is shown
	// when there is no default.
	Ask(question, defaultValue, example string) (string, error)
	Confirm(question string, defaultValue bool) (bool, error)
	// Choose returns the index of the option picked.
	Choose(question string, options []Choice) (int, error)
	// Checklist returns the indexes of the items left selected, in order.
	Checklist(question string, items []ChecklistItem) ([]int, error)
}

// Reporter shows the progress of the work. Everything it shows is for a person: the result of a
// command that was asked for JSON goes elsewhere.
//
// Output is sequential and each line stays where it was printed. A section starts a block, its
// items follow, and its result closes it.
type Reporter interface {
	// Banner shows the title and the tenant every change will go to, before any question.
	Banner(title, tenantDomain string)
	// Section starts a block of items.
	Section(title string)
	// Item is one line of a section. The outcome, such as "created", is shown dimmed after it.
	Item(text, outcome string)
	// Fixed is a change the command made, shown with a check mark.
	Fixed(text, summary string)
	// Result closes a section.
	Result(text string)
	Info(text string)
	Warn(text string)
	Error(text string)
	// Detail is a dimmed line.
	Detail(text string)
	// Reference shows an ID that links to its page in the dashboard.
	Reference(label, id, url string)
	// Ready shows the URL of the portal that was set up.
	Ready(url string)
	// Cancelled closes the output when the person cancels.
	Cancelled(message string)
	// Newline leaves a blank line.
	Newline()
	// Table shows rows under a header. The first column is a symbol.
	Table(headers []string, rows [][]string)
	// Task runs work behind a spinner when there is a terminal.
	Task(title string, work func() error) error
}
