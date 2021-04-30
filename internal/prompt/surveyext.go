package prompt

import (
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
)

// EXTENDED to enable different prompting behavior
type Editor struct {
	*survey.Editor
	EditorCommand string
	BlankAllowed  bool
}

func (e *Editor) editorCommand() string {
	if e.EditorCommand == "" {
		return getDefaultEditor()
	}

	return e.EditorCommand
}

// EXTENDED to change prompt text
var EditorQuestionTemplate = `
{{- if .ShowHelp }}{{- color .Config.Icons.Help.Format }}{{ .Config.Icons.Help.Text }} {{ .Help }}{{color "reset"}}{{"\n"}}{{end}}
{{- color .Config.Icons.Question.Format }}{{ .Config.Icons.Question.Text }} {{color "reset"}}
{{- color "default+hb"}}{{ .Message }} {{color "reset"}}
{{- if .ShowAnswer}}
  {{- color "cyan"}}{{.Answer}}{{color "reset"}}{{"\n"}}
{{- else }}
  {{- if and .Help (not .ShowHelp)}}{{color "cyan"}}[{{ .Config.HelpInput }} for help]{{color "reset"}} {{end}}
  {{- if and .Default (not .HideDefault)}}{{color "white"}}({{.Default}}) {{color "reset"}}{{end}}
	{{- color "cyan"}}[(e) to launch {{ .EditorCommand }}{{- if .BlankAllowed }}, enter to skip{{ end }}] {{color "reset"}}
{{- end}}`

// EXTENDED to pass editor name (to use in prompt)
type EditorTemplateData struct {
	survey.Editor
	EditorCommand string
	BlankAllowed  bool
	Answer        string
	ShowAnswer    bool
	ShowHelp      bool
	Config        *survey.PromptConfig
}

// EXTENDED to augment prompt text and keypress handling
func (e *Editor) prompt(initialValue string, config *survey.PromptConfig) (interface{}, error) {
	err := e.Render(
		EditorQuestionTemplate,
		// EXTENDED to support printing editor in prompt and BlankAllowed
		EditorTemplateData{
			Editor:        *e.Editor,
			BlankAllowed:  e.BlankAllowed,
			EditorCommand: filepath.Base(e.editorCommand()),
			Config:        config,
		},
	)
	if err != nil {
		return "", err
	}

	// start reading runes from the standard in
	rr := e.NewRuneReader()
	_ = rr.SetTermMode()
	defer func() { _ = rr.RestoreTermMode() }()

	cursor := e.NewCursor()
	cursor.Hide()
	defer cursor.Show()

	for {
		// EXTENDED to handle the e to edit / enter to skip behavior + BlankAllowed
		r, _, err := rr.ReadRune()
		if err != nil {
			return "", err
		}
		if r == 'e' {
			break
		}
		if r == '\r' || r == '\n' {
			if e.BlankAllowed {
				return "", nil
			} else {
				continue
			}
		}
		if r == terminal.KeyInterrupt {
			return "", terminal.InterruptErr
		}
		if r == terminal.KeyEndTransmission {
			break
		}
		if string(r) == config.HelpInput && e.Help != "" {
			err = e.Render(
				EditorQuestionTemplate,
				EditorTemplateData{
					// EXTENDED to support printing editor in prompt, BlankAllowed
					Editor:        *e.Editor,
					BlankAllowed:  e.BlankAllowed,
					EditorCommand: filepath.Base(e.editorCommand()),
					ShowHelp:      true,
					Config:        config,
				},
			)
			if err != nil {
				return "", err
			}
		}
		continue
	}

	text, err := CaptureInputViaEditor(initialValue, e.FileName, nil, nil)
	if err != nil {
		return "", err
	}

	// check length, return default value on empty
	if len(text) == 0 && !e.AppendDefault {
		return e.Default, nil
	}

	return text, nil
}

// EXTENDED This is straight copypasta from survey to get our overridden prompt called.
func (e *Editor) Prompt(config *survey.PromptConfig) (interface{}, error) {
	initialValue := ""
	if e.Default != "" && e.AppendDefault {
		initialValue = e.Default
	}
	return e.prompt(initialValue, config)
}
