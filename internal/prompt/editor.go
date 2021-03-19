package prompt

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const defaultEditor = "vim"

var defaultEditorPrompt = &editorPrompt{defaultEditor: defaultEditor}

// CaptureInputViaEditor is the high level function to use in this package in
// order to capture input from an editor.
//
// The arguments have been tailored for our use of strings mostly in the rest
// of the CLI even though internally we're using []byte.
func CaptureInputViaEditor(contents, pattern string) (result string, err error) {
	v, err := defaultEditorPrompt.captureInput([]byte(contents), pattern)
	return string(v), err
}

type editorPrompt struct {
	defaultEditor string
}

// GetPreferredEditorFromEnvironment returns the user's editor as defined by the
// `$EDITOR` environment variable, or the `defaultEditor` if it is not set.
func (p *editorPrompt) getPreferredEditor() string {
	editor := os.Getenv("EDITOR")

	if editor == "" {
		return p.defaultEditor
	}

	return editor
}

func (p *editorPrompt) resolveEditorArguments(executable string, filename string) []string {
	args := []string{filename}

	if strings.Contains(executable, "Visual Studio Code.app") {
		args = append([]string{"--wait"}, args...)
	}

	// TODO(cyx): add other common editors

	return args
}

// openFile opens filename in the preferred text editor, resolving the
// arguments with editor specific logic.
func (p *editorPrompt) openFile(filename string) error {
	// Get the full executable path for the editor.
	executable, err := exec.LookPath(p.getPreferredEditor())
	if err != nil {
		return err
	}

	cmd := exec.Command(executable, p.resolveEditorArguments(executable, filename)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// captureInput opens a temporary file in a text editor and returns
// the written bytes on success or an error on failure. It handles deletion
// of the temporary file behind the scenes.
//
// If given default contents, it will write that to the file before popping
// open the editor.
func (p *editorPrompt) captureInput(contents []byte, pattern string) ([]byte, error) {
	file, err := os.CreateTemp(os.TempDir(), pattern)
	if err != nil {
		return []byte{}, err
	}

	filename := file.Name()

	if len(contents) > 0 {
		if err := os.WriteFile(filename, contents, 0644); err != nil {
			return nil, fmt.Errorf("Failed to write to file: %w", err)
		}
	}

	// Defer removal of the temporary file in case any of the next steps fail.
	defer os.Remove(filename)

	if err = file.Close(); err != nil {
		return nil, err
	}

	if err = p.openFile(filename); err != nil {
		return nil, err
	}

	bytes, err := os.ReadFile(filename)
	if err != nil {
		return []byte{}, err
	}

	return bytes, nil
}
