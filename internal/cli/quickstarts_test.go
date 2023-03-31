package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQuickstartsTypeFor(t *testing.T) {
	assert.Equal(t, qsSpa, quickstartsTypeFor("spa"))
	assert.Equal(t, qsWebApp, quickstartsTypeFor("regular_web"))
	assert.Equal(t, qsWebApp, quickstartsTypeFor("regular_web"))
	assert.Equal(t, qsBackend, quickstartsTypeFor("non_interactive"))
	assert.Equal(t, "generic", quickstartsTypeFor("some-unknown-value"))
}
