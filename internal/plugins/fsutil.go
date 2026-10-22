package plugins

import (
	"errors"
	"io"
	"os"
	"runtime"
)

// tempFilePerm is the permissions of a staged file, which only the current user
// needs.
const tempFilePerm os.FileMode = 0o600

// ErrTooLarge is returned when a file or a body is bigger than the limit it was
// read with.
var ErrTooLarge = errors.New("larger than the allowed size")

// ReadCapped reads all of r, and fails with ErrTooLarge if there is more than
// limit bytes.
func ReadCapped(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}

	if int64(len(data)) > limit {
		return nil, ErrTooLarge
	}

	return data, nil
}

// SyncDir syncs a directory so that the files that were renamed in it survive a
// crash. It does nothing on Windows, where a directory cannot be synced, and
// otherwise does what it can, as not every file system supports it.
func SyncDir(dir string) {
	if runtime.GOOS == "windows" {
		return
	}

	handle, err := os.Open(dir)
	if err != nil {
		return
	}

	_ = handle.Sync()
	_ = handle.Close()
}

// StageTempFile writes data to a new private temporary file in dir that is named
// after name, syncs it, and returns its path. Nothing is left behind when it
// fails.
func StageTempFile(dir, name string, data []byte) (path string, err error) {
	temp, err := os.CreateTemp(dir, "."+name+".tmp-*")
	if err != nil {
		return "", err
	}

	defer func() {
		if err != nil {
			_ = temp.Close()
			_ = os.Remove(temp.Name())
		}
	}()

	if err = temp.Chmod(tempFilePerm); err != nil {
		return "", err
	}

	if _, err = temp.Write(data); err != nil {
		return "", err
	}

	if err = temp.Sync(); err != nil {
		return "", err
	}

	if err = temp.Close(); err != nil {
		return "", err
	}

	return temp.Name(), nil
}
