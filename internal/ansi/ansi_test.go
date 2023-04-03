package ansi

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldUseColors(t *testing.T) {
	os.Setenv("CLICOLOR_FORCE", "true")
	assert.True(t, shouldUseColors())

	os.Setenv("CLICOLOR_FORCE", "0")
	assert.False(t, shouldUseColors())

	os.Unsetenv("CLI_COLOR_FORCE")

	os.Setenv("CLICOLOR", "0")
	assert.False(t, shouldUseColors())
	os.Unsetenv("CLICOLOR")
}
