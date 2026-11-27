//go:build !windows

package plugins

import "os"

// Rename moves oldpath to newpath and replaces newpath. It is os.Rename here;
// on Windows it also waits out a reader that has the file open.
func Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

// IsInUse reports whether err says that a file or a directory could not be
// changed because another process has it open. It is always false here, because
// only Windows refuses to delete or replace a file that is open.
func IsInUse(error) bool {
	return false
}
