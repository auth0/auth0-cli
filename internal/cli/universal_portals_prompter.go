package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/core"
	"github.com/AlecAivazis/survey/v2/terminal"

	"github.com/auth0/auth0-cli/internal/portals"
	"github.com/auth0/auth0-cli/internal/prompt"
)

// The most options a list shows at once.
const portalsMaxVisibleOptions = 15

// portalsPrompter asks the questions of setup and audit on the terminal. A Ctrl+C is not an exit
// here: it comes back as portals.ErrCancelled, so the command can say what was and was not done.
type portalsPrompter struct{}

var _ portals.Prompter = portalsPrompter{}

const answerKey = "answer"

// ask shows one question and returns the answer.
func (portalsPrompter) ask(question survey.Prompt) (any, error) {
	response := map[string]any{}

	if err := prompt.AskOne(&survey.Question{Name: answerKey, Prompt: question}, &response); err != nil {
		if errors.Is(err, terminal.InterruptErr) {
			return nil, portals.ErrCancelled
		}

		return nil, fmt.Errorf("could not read the answer: %w", err)
	}

	return response[answerKey], nil
}

// Ask implements portals.Prompter.
func (p portalsPrompter) Ask(question, defaultValue, example string) (string, error) {
	message := question
	if defaultValue == "" && example != "" {
		message += " (for example " + example + ")"
	}

	answer, err := p.ask(&survey.Input{Message: message, Default: defaultValue})
	if err != nil {
		return "", err
	}

	text, _ := answer.(string)
	if text = strings.TrimSpace(text); text != "" {
		return text, nil
	}

	return defaultValue, nil
}

// Confirm implements portals.Prompter.
func (p portalsPrompter) Confirm(question string, defaultValue bool) (bool, error) {
	answer, err := p.ask(&survey.Confirm{Message: question, Default: defaultValue})
	if err != nil {
		return false, err
	}

	confirmed, _ := answer.(bool)

	return confirmed, nil
}

// Choose implements portals.Prompter.
func (p portalsPrompter) Choose(question string, options []portals.Choice) (int, error) {
	labels := make([]string, 0, len(options))
	for _, option := range options {
		labels = append(labels, optionLabel(option.Label, option.Hint))
	}

	answer, err := p.ask(&survey.Select{
		Message:  question,
		Options:  labels,
		PageSize: min(len(labels), portalsMaxVisibleOptions),
	})
	if err != nil {
		return 0, err
	}

	picked, ok := answer.(core.OptionAnswer)
	if !ok {
		return 0, fmt.Errorf("unexpected answer %v to %q", answer, question)
	}

	return picked.Index, nil
}

// Checklist implements portals.Prompter.
func (p portalsPrompter) Checklist(question string, items []portals.ChecklistItem) ([]int, error) {
	labels := make([]string, 0, len(items))
	checked := make([]int, 0, len(items))

	for index, item := range items {
		labels = append(labels, optionLabel(item.Label, item.Hint))

		if item.Checked {
			checked = append(checked, index)
		}
	}

	answer, err := p.ask(&survey.MultiSelect{
		Message:  question,
		Options:  labels,
		Default:  checked,
		PageSize: min(len(labels), portalsMaxVisibleOptions),
	})
	if err != nil {
		return nil, err
	}

	picked, ok := answer.([]core.OptionAnswer)
	if !ok {
		return nil, fmt.Errorf("unexpected answer %v to %q", answer, question)
	}

	indexes := make([]int, 0, len(picked))
	for _, option := range picked {
		indexes = append(indexes, option.Index)
	}

	return indexes, nil
}

// optionLabel is the text of an option, with its hint after it.
func optionLabel(label, hint string) string {
	if hint == "" {
		return label
	}

	return label + " (" + hint + ")"
}
