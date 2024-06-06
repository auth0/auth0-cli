package ansi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldUseColors(t *testing.T) {
	t.Setenv("CLICOLOR_FORCE", "true")
	assert.True(t, shouldUseColors())

	t.Setenv("CLICOLOR_FORCE", "0")
	assert.False(t, shouldUseColors())

	t.Setenv("CLICOLOR", "0")
	assert.False(t, shouldUseColors())
}
