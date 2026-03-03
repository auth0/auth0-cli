//go:build windows
// +build windows

package prompt

import (
	"fmt"
	"syscall"
	"unsafe"
)

// parseEditorArgs parses Windows-style command line arguments
// into a slice of strings suitable for exec.Command.
// Uses the Windows CommandLineToArgvW API to correctly handle
// backslash path separators and quoted paths with spaces.
func parseEditorArgs(cmd string) ([]string, error) {
	if cmd == "" {
		return []string{}, nil
	}

	// Convert the Go string (UTF-8) to a UTF-16 pointer, as required by Windows APIs.
	utf16Cmd, err := syscall.UTF16PtrFromString(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to encode editor command: %w", err)
	}

	// Use the Windows CommandLineToArgvW API to split the command string into arguments.
	// This correctly handles Windows path separators (\) and quoted paths with spaces.
	var argc int32
	argv, err := syscall.CommandLineToArgv(utf16Cmd, &argc)
	if err != nil {
		return nil, fmt.Errorf("failed to parse editor command %q: %w", cmd, err)
	}
	// argv is allocated by the Windows API and must be freed with LocalFree.
	defer syscall.LocalFree(syscall.Handle(unsafe.Pointer(argv)))

	// Convert each UTF-16 encoded argument back to a Go string.
	args := make([]string, argc)
	for i := range args {
		args[i] = syscall.UTF16ToString((*argv[i])[:])
	}

	return args, nil
}
