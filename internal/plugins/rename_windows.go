package plugins

import (
	"os"
	"time"
)

// Rename moves oldpath to newpath and replaces newpath, like os.Rename. On
// Windows a rename fails while another process, such as a virus scanner or an
// indexer, has the file open, because Go does not open files in a way that
// lets them be deleted. The failure is brief in practice, so Rename tries
// again for about a second before it gives up and returns the error.
func Rename(oldpath, newpath string) error {
	return retryRename(os.Rename, time.Sleep, oldpath, newpath)
}

// IsInUse reports whether err says that a file or a directory could not be
// changed because another process, such as a running plugin, has it open. On
// Windows a file that is open cannot be deleted or replaced.
func IsInUse(err error) bool {
	return isRetryable(err)
}
