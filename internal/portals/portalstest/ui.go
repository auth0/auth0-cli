package portalstest

import (
	"fmt"
	"strings"
	"sync"

	"github.com/auth0/auth0-cli/internal/portals"
)

// Prompter answers the questions of a test from functions it sets. A question with no function
// to answer it fails, so a test that asks more than it expects does not pass by accident.
type Prompter struct {
	AskFunc       func(question, defaultValue, example string) (string, error)
	ConfirmFunc   func(question string, defaultValue bool) (bool, error)
	ChooseFunc    func(question string, options []portals.Choice) (int, error)
	ChecklistFunc func(question string, items []portals.ChecklistItem) ([]int, error)

	mu        sync.Mutex
	questions []string
}

var _ portals.Prompter = (*Prompter)(nil)

// Questions returns the questions asked so far, in order.
func (p *Prompter) Questions() []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	return append([]string(nil), p.questions...)
}

func (p *Prompter) record(question string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.questions = append(p.questions, question)
}

// Ask implements portals.Prompter.
func (p *Prompter) Ask(question, defaultValue, example string) (string, error) {
	p.record(question)

	if p.AskFunc == nil {
		return "", fmt.Errorf("portalstest: unexpected question %q", question)
	}

	return p.AskFunc(question, defaultValue, example)
}

// Confirm implements portals.Prompter.
func (p *Prompter) Confirm(question string, defaultValue bool) (bool, error) {
	p.record(question)

	if p.ConfirmFunc == nil {
		return false, fmt.Errorf("portalstest: unexpected question %q", question)
	}

	return p.ConfirmFunc(question, defaultValue)
}

// Choose implements portals.Prompter.
func (p *Prompter) Choose(question string, options []portals.Choice) (int, error) {
	p.record(question)

	if p.ChooseFunc == nil {
		return 0, fmt.Errorf("portalstest: unexpected question %q", question)
	}

	return p.ChooseFunc(question, options)
}

// Checklist implements portals.Prompter.
func (p *Prompter) Checklist(question string, items []portals.ChecklistItem) ([]int, error) {
	p.record(question)

	if p.ChecklistFunc == nil {
		return nil, fmt.Errorf("portalstest: unexpected question %q", question)
	}

	return p.ChecklistFunc(question, items)
}

// Reporter records what a flow shows, one line for each call. A line starts with the kind of the
// call, such as "section" or "warn", then a colon and the text. It runs the work of a task at once.
type Reporter struct {
	mu    sync.Mutex
	lines []string
}

var _ portals.Reporter = (*Reporter)(nil)

// Lines returns what was shown, in order.
func (r *Reporter) Lines() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]string(nil), r.lines...)
}

// Contains reports whether a line of the given kind has the text in it.
func (r *Reporter) Contains(kind, text string) bool {
	for _, line := range r.Lines() {
		if strings.HasPrefix(line, kind+": ") && strings.Contains(line, text) {
			return true
		}
	}

	return false
}

func (r *Reporter) add(kind string, parts ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.lines = append(r.lines, kind+": "+strings.Join(parts, " | "))
}

// Banner implements portals.Reporter.
func (r *Reporter) Banner(title, tenantDomain string) { r.add("banner", title, tenantDomain) }

// Section implements portals.Reporter.
func (r *Reporter) Section(title string) { r.add("section", title) }

// Item implements portals.Reporter.
func (r *Reporter) Item(text, outcome string) { r.add("item", text, outcome) }

// Fixed implements portals.Reporter.
func (r *Reporter) Fixed(text, summary string) { r.add("fixed", text, summary) }

// Result implements portals.Reporter.
func (r *Reporter) Result(text string) { r.add("result", text) }

// Info implements portals.Reporter.
func (r *Reporter) Info(text string) { r.add("info", text) }

// Warn implements portals.Reporter.
func (r *Reporter) Warn(text string) { r.add("warn", text) }

// Error implements portals.Reporter.
func (r *Reporter) Error(text string) { r.add("error", text) }

// Detail implements portals.Reporter.
func (r *Reporter) Detail(text string) { r.add("detail", text) }

// Reference implements portals.Reporter.
func (r *Reporter) Reference(label, id, url string) { r.add("reference", label, id, url) }

// Ready implements portals.Reporter.
func (r *Reporter) Ready(url string) { r.add("ready", url) }

// Cancelled implements portals.Reporter.
func (r *Reporter) Cancelled(message string) { r.add("cancelled", message) }

// Newline implements portals.Reporter.
func (r *Reporter) Newline() { r.add("newline") }

// Table implements portals.Reporter.
func (r *Reporter) Table(headers []string, rows [][]string) {
	r.add("table", strings.Join(headers, ","))

	for _, row := range rows {
		r.add("row", strings.Join(row, ","))
	}
}

// Task implements portals.Reporter.
func (r *Reporter) Task(title string, work func() error) error {
	r.add("task", title)

	return work()
}
