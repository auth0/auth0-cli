//go:build windows

package devpolicy

import "os"

// openFile opens the file for reading.
func openFile(path string) (*os.File, error) {
	return os.Open(path)
}
