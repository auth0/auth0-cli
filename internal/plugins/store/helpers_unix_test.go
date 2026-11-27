//go:build !windows

package store_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// symlink makes a symbolic link.
func symlink(t *testing.T, target, link string) {
	t.Helper()

	require.NoError(t, os.Symlink(target, link))
}
