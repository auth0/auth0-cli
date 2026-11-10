//go:build !windows

package devpolicy

import (
	"os"
	"syscall"
)

// openFile opens the file for reading without waiting for a writer, so that a
// named pipe swapped in after the path was checked cannot block the open.
func openFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
