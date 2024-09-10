//go:build !windows

package cli

const ErrNotFound = "no such file or directory"
const ErrPermissionDenied = "permission denied"
const ErrFileNotFound = ErrNotFound
const ErrCannotRemoveFile = ErrPermissionDenied
