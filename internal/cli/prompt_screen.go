package cli

import (
	"errors"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/prompt"
)

// promptPageSize is how many options a list shows at once before it scrolls, the
// same as the other prompts of the CLI.
const promptPageSize = 13

// promptAsker is how the commands ask questions. Whether a command may ask
// depends on canPrompt alone, whatever the output format. Tests replace the
// fields to run as if in a terminal.
type promptAsker struct {
	canPrompt func(cmd *cobra.Command) bool
	// Interrupt ends the command after Ctrl-C.
	interrupt func(err error) error
	// The four kinds of prompt. A prompt that the user ends with Ctrl-C returns
	// terminal.InterruptErr.
	text       func(label, def string) (string, error)
	confirm    func(message string, def bool) (bool, error)
	choose     func(message string, options []string, def int) (int, error)
	chooseMany func(message string, options []string, defaults []int) ([]int, error)
}

var promptAsk = promptAsker{
	canPrompt:  canPrompt,
	interrupt:  handleInputError,
	text:       terminalPromptText,
	confirm:    terminalPromptConfirm,
	choose:     terminalPromptChoose,
	chooseMany: terminalPromptChooseMany,
}

// questionScreen is what a command shows while it asks questions in a
// terminal: the text of the blocks is in display, and this holds the prompts
// that are asked below them.
type questionScreen struct {
	*display.PromptScreen
	// Asker holds the prompts. It is the one that was set when the screen started.
	asker promptAsker
}

// newPromptScreen starts the screen of a command. The title of the run takes the
// company domain, see ForDomain.
func (c *cli) newPromptScreen(title string) *questionScreen {
	return &questionScreen{
		PromptScreen: c.renderer.NewPromptScreen(title, c.tenant),
		asker:        promptAsk,
	}
}

// ask draws a block with a title and a reason, and runs the questions below it.
// Ctrl-C tells that nothing was changed, and ends the command the way the other
// prompts do.
func (s *questionScreen) ask(title, reason string, questions func() error) error {
	s.Begin(title, reason)

	err := questions()
	if errors.Is(err, terminal.InterruptErr) {
		s.Cancelled()

		return s.asker.interrupt(err)
	}

	return err
}

// askLine asks a question with a title, a reason and a place to type the answer,
// and asks again for an answer that is empty.
func (s *questionScreen) askLine(title, reason, label, def string) (answer string, err error) {
	err = s.ask(title, reason, func() error {
		for {
			var err error
			if answer, err = s.asker.text(label, def); err != nil {
				return err
			}

			if strings.TrimSpace(answer) != "" {
				return nil
			}

			s.Invalid("A value is required.")
		}
	})

	return answer, err
}

// confirmAsk asks a question answered with yes or no, which is no unless the user
// says yes.
func (s *questionScreen) confirmAsk(title, reason, message string) (yes bool, err error) {
	return s.confirmDefault(title, reason, message, false)
}

// confirmDefault asks a question answered with yes or no, which is def when the
// user just presses Enter.
func (s *questionScreen) confirmDefault(title, reason, message string, def bool) (yes bool, err error) {
	err = s.ask(title, reason, func() (err error) {
		yes, err = s.asker.confirm(message, def)

		return err
	})

	return yes, err
}

// askPrompt runs one prompt. Ctrl-C comes back as terminal.InterruptErr, and any
// other failure as an unexpected error.
func askPrompt(question *survey.Question, answer any) error {
	err := prompt.AskOne(question, answer)

	switch {
	case err == nil, errors.Is(err, terminal.InterruptErr):
		return err
	default:
		return unexpectedError(err)
	}
}

// terminalPromptText asks for a line of text, with a default that Enter accepts.
func terminalPromptText(label, def string) (string, error) {
	var answer string

	err := askPrompt(prompt.TextInput("", label, "", def, false), &answer)

	return answer, err
}

// terminalPromptConfirm asks a question answered with yes or no. Enter takes the
// default.
func terminalPromptConfirm(message string, def bool) (bool, error) {
	var yes bool

	err := askPrompt(&survey.Question{Prompt: &survey.Confirm{Message: message, Default: def}}, &yes)

	return yes, err
}

// terminalPromptChoose asks to pick one of the options, which start on the one at
// index def. It returns the index picked. A list longer than a page can be
// filtered by typing.
func terminalPromptChoose(message string, options []string, def int) (int, error) {
	var defaultOption string
	if def >= 0 && def < len(options) {
		defaultOption = options[def]
	}

	question := prompt.SelectInput("", message, "", options, defaultOption, false)
	if len(options) > promptPageSize {
		question = prompt.PaginatedSelectInput("", message, "", options, defaultOption, false)
	}

	var picked int
	err := askPrompt(question, &picked)

	return picked, err
}

// terminalPromptChooseMany asks to pick any of the options, which start with the
// ones at the indexes in defaults chosen. It returns the indexes picked.
func terminalPromptChooseMany(message string, options []string, defaults []int) ([]int, error) {
	var picked []int

	question := &survey.Question{Prompt: &survey.MultiSelect{
		Message:  message,
		Options:  options,
		Default:  defaults,
		PageSize: promptPageSize,
	}}

	err := askPrompt(question, &picked)

	return picked, err
}
