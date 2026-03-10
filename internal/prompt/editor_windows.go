//go:build windows
// +build windows

package prompt

import (
	"golang.org/x/sys/windows"
)

func parseEditorArgs(cmd string) ([]string, error) {
	return windows.DecomposeCommandLine(cmd)
}