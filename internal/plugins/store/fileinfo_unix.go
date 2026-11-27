//go:build !windows

package store

import (
	"os"
	"syscall"
)

// unsafeFile says why another local account could change the file or directory
// after it was checked, or returns an empty string when none could: it is
// writable by its group or by everyone, or it is a regular file with another
// hard link that can be written through.
func unsafeFile(info os.FileInfo) string {
	if info.Mode().Perm()&0o022 != 0 {
		return "is writable by other users"
	}

	if !info.Mode().IsRegular() {
		return ""
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "has a link count that cannot be read"
	}

	if uint64(stat.Nlink) > 1 { //nolint:unconvert // Nlink is a different width on each system.
		return "has more than one hard link"
	}

	return ""
}
