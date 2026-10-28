package store_test

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// symlink makes a symbolic link, and skips the test on a machine where the
// account may not make one: that takes a privilege that an account without
// developer mode does not have. The CI runner has it.
func symlink(t *testing.T, target, link string) {
	t.Helper()

	err := os.Symlink(target, link)
	if errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
		t.Skip("this account may not create symbolic links")
	}

	require.NoError(t, err)
}
