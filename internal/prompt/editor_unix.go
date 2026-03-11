//go:build !windows
// +build !windows

package prompt

import (
	"github.com/kballard/go-shellquote"
)

// parseEditorArgs parses POSIX shell-style command line arguments
// into a slice of strings suitable for exec.Command.
func parseEditorArgs(cmd string) ([]string, error) {
	return shellquote.Split(cmd)
}
