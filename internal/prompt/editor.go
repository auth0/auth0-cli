package prompt

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/kballard/go-shellquote"
)

const (
	defaultEditor = "vim"
)

var (
	bom        = []byte{0xef, 0xbb, 0xbf}
	cliEditors = []string{"emacs", "micro", "nano", "vi", "vim", "nvim"}
)

var defaultEditorPrompt = &editorPrompt{cmd: getDefaultEditor()}

// CaptureInputViaEditor is the high level function to use in this package in
// order to capture input from an editor.
//
// The arguments have been tailored for our use of strings mostly in the rest
// of the CLI even though internally we're using []byte.
func CaptureInputViaEditor(contents, pattern string, infoFn func(), fileCreatedFn func(string)) (result string, err error) {
	v, err := defaultEditorPrompt.captureInput([]byte(contents), pattern, infoFn, fileCreatedFn)
	return string(v), err
}

type editorPrompt struct {
	cmd string
}

// openFile opens filename in the preferred text editor, resolving the
// arguments with editor specific logic.
func (p *editorPrompt) openFile(filename string, infoFn func()) error {
	args, err := shellquote.Split(p.cmd)
	if err != nil {
		return err
	}
	args = append(args, filename)

	isCLIEditor := false
	for _, e := range cliEditors {
		if e == args[0] {
			isCLIEditor = true
		}
	}

	editorExe, err := exec.LookPath(args[0])
	if err != nil {
		return err
	}

	cmd := exec.Command(editorExe, args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if !isCLIEditor && infoFn != nil {
		infoFn()
	}

	return cmd.Run()
}

// captureInput opens a temporary file in a text editor and returns
// the written bytes on success or an error on failure. It handles deletion
// of the temporary file behind the scenes.
//
// If given default contents, it will write that to the file before popping
// open the editor.
func (p *editorPrompt) captureInput(contents []byte, pattern string, infoFn func(), fileCreatedFn func(string)) ([]byte, error) {
	dir, err := os.MkdirTemp("", pattern)
	if err != nil {
		return []byte{}, err
	}
	defer os.Remove(dir)

	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return []byte{}, err
	}

	filename := file.Name()

	if fileCreatedFn != nil {
		go fileCreatedFn(filename)
	}

	// Defer removal of the temporary file in case any of the next steps fail.
	defer os.Remove(filename)

	// write utf8 BOM header
	// The reason why we do this is because notepad.exe on Windows determines the
	// encoding of an "empty" text file by the locale, for example, GBK in China,
	// while golang string only handles utf8 well. However, a text file with utf8
	// BOM header is not considered "empty" on Windows, and the encoding will then
	// be determined utf8 by notepad.exe, instead of GBK or other encodings.
	if _, err := file.Write(bom); err != nil {
		return nil, err
	}

	if len(contents) > 0 {
		if _, err := file.Write(contents); err != nil {
			return nil, fmt.Errorf("Failed to write to file: %w", err)
		}
	}

	if err = file.Close(); err != nil {
		return nil, err
	}

	if err = p.openFile(filename, infoFn); err != nil {
		return nil, err
	}

	raw, err := os.ReadFile(filename)
	if err != nil {
		return []byte{}, err
	}

	// strip BOM header
	return bytes.TrimPrefix(raw, bom), nil
}

// getDefaultEditor is taken from https://github.com/cli/cli/blob/trunk/pkg/surveyext/editor_manual.go
// and tries to infer the editor from different heuristics.
func getDefaultEditor() string {
	if runtime.GOOS == "windows" {
		return "notepad"
	} else if g := os.Getenv("GIT_EDITOR"); g != "" {
		return g
	} else if v := os.Getenv("VISUAL"); v != "" {
		return v
	} else if e := os.Getenv("EDITOR"); e != "" {
		return e
	}

	return defaultEditor
}
