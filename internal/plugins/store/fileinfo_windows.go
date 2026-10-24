package store

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// unsafeFile says why another local account could change the file after it was
// checked, or returns an empty string when none could: it is a regular file with
// another hard link that can be written through. A directory is never reported,
// and nothing is read for it. The mode bits of Go are made up on Windows and
// access is decided by ACLs, so there is no group or world write bit to read,
// and the ACLs are not checked.
//
// The file is the open file that info describes. The link count is read from that
// handle, so it is the count of the file that is about to be read and not of
// whatever the path names a moment later. A regular file without an open file
// is refused, because its link count cannot be read.
func unsafeFile(file *os.File, info os.FileInfo) string {
	if !info.Mode().IsRegular() {
		return ""
	}

	if file == nil {
		return "has a link count that cannot be read"
	}

	var data windows.ByHandleFileInformation

	if err := windows.GetFileInformationByHandle(windows.Handle(file.Fd()), &data); err != nil {
		return fmt.Sprintf("has a link count that cannot be read (%v)", err)
	}

	if data.NumberOfLinks > 1 {
		return "has more than one hard link"
	}

	return ""
}
