package display

import (
	"fmt"
	"io"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/text"
)

// PromptCancelled is what a Poppy command says when the user ends a question with
// Ctrl-C. The questions come before anything is changed.
const PromptCancelled = "Cancelled. Nothing was changed."

// PromptScreen is what a Poppy command shows while it asks questions in a
// terminal. A question draws a block of its own, a title and a reason, above the
// prompt, and the block stays on the screen with the answer. The title of the run
// is printed once, above the first question.
//
// The blocks go to the message writer (stderr), like the prompts. The title of the
// run goes where the text of the command goes, see poppyOut.
type PromptScreen struct {
	// Out is where the blocks are written, and layout is the layout of it.
	out    io.Writer
	layout Layout
	// Head is where the title of the run is written, and headLayout is the layout
	// of it.
	head       io.Writer
	headLayout Layout

	title, domain, tenant string
	// Shown tells that the title line is on the screen.
	shown bool
}

// NewPromptScreen starts the screen of a command. The title of the run takes the
// company domain, see ForDomain.
func (r *Renderer) NewPromptScreen(title, tenant string) *PromptScreen {
	head := r.poppyOut()

	return &PromptScreen{
		out:        r.MessageWriter,
		layout:     LayoutOf(r.MessageWriter),
		head:       head,
		headLayout: LayoutOf(head),
		title:      title,
		tenant:     tenant,
	}
}

// ForDomain says which company domain the run is for. The title is printed above
// the first question, with the domain when it is known by then.
func (s *PromptScreen) ForDomain(domain string) { s.domain = domain }

// HeaderShown tells whether the title line of the run is on the screen, in which
// case the text that follows must not print it again.
func (s *PromptScreen) HeaderShown() bool { return s.shown }

// Printed tells the screen that the text of the command was printed with the title
// of the run, such as the plan, so that a question that follows does not print it
// again.
func (s *PromptScreen) Printed() { s.shown = true }

// Note prints a line that stays on the screen, such as a decision that was made
// for the user.
func (s *PromptScreen) Note(format string, args ...any) {
	s.say(s.layout.entries([]poppyEntry{{mark: markWarning, value: text.Line(fmt.Sprintf(format, args...))}}))
}

// Begin draws a block with a title and a reason, below the title of the run, which
// it prints first when it is not on the screen yet, without the company domain
// when that is not known yet, so that the title stays at the top of the run. A
// screen without a title has no title line.
func (s *PromptScreen) Begin(title, reason string) {
	if !s.shown && s.title != "" {
		s.shown = true

		_, _ = io.WriteString(s.head, poppyHeader(s.headLayout, s.title, s.domain, s.tenant))
	}

	s.say("\n")
	s.lines(ansi.Bold, title)
	s.lines(ansi.Faint, reason)
}

// Cancelled tells that the user ended the questions with Ctrl-C.
func (s *PromptScreen) Cancelled() {
	_, _ = fmt.Fprintln(s.out, PromptCancelled)
}

// say writes to the blocks.
func (s *PromptScreen) say(line string) {
	_, _ = io.WriteString(s.out, line)
}

// lines writes a text, wrapped, in a style. An empty text writes nothing.
func (s *PromptScreen) lines(style func(string) string, line string) {
	if line == "" {
		return
	}

	for _, wrapped := range s.layout.wrap(text.Line(line), 0) {
		s.say(style(wrapped) + "\n")
	}
}

// Invalid writes a line that says an answer was not accepted.
func (s *PromptScreen) Invalid(reason string) {
	s.say(s.layout.entries([]poppyEntry{{mark: markFailed, value: text.Line(reason)}}))
}
