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

func TestDefaultCallbackURLFor(t *testing.T) {
	assert.Equal(t, "http://localhost:3000/api/auth/callback", defaultCallbackURLFor("next.js"))
	assert.Equal(t, "http://localhost:3000", defaultCallbackURLFor("all-other-quickstart-application-types"))
}

func TestDefaultURLFor(t *testing.T) {
	assert.Equal(t, "http://localhost:4200", defaultURLFor("angular"))
	assert.Equal(t, "http://localhost:3000", defaultURLFor("all-other-quickstart-application-types"))
}

func TestUrlPromptFor(t *testing.T) {
	assert.Equal(t, "Quickstarts use localhost, do you want to add http://localhost:3000/api/auth/callback to the list\n of allowed callback URLs and http://localhost:3000 to the list of allowed logout URLs?", urlPromptFor("generic", "Next.js"))
	assert.Equal(t, "Quickstarts use localhost, do you want to add http://localhost:3000 to the list\n of allowed callback URLs and logout URLs?", urlPromptFor("generic", "Laravel API"))
}
