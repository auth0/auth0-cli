//go:build !windows

package plugins

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsInUse(t *testing.T) {
	// The Win32 codes are other errors here, and nothing is ever in use.
	assert.False(t, IsInUse(&os.LinkError{Op: "rename", Err: errorSharingViolation}))
}
