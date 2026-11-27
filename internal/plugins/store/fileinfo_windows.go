package store

import "os"

// unsafeFile reports nothing on Windows, where the mode bits of Go are made up
// and access is decided by ACLs, so there is no group or world write bit to read.
func unsafeFile(os.FileInfo) string {
	return ""
}
