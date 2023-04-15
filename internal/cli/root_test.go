package cli

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommandRequiresAuthentication(t *testing.T) {
	var testCases = []struct {
		givenCommand                    string
		expectedToRequireAuthentication bool
	}{
		{"auth0 user list", true},
		{"auth0 user create", true},
		{"auth0 api", true},
		{"auth0 apps list", true},
		{"auth0 apps create", true},
		{"auth0 orgs members list", true},
		{"auth0 completion", false},
		{"auth0 help", false},
		{"auth0 login", false},
		{"auth0 logout", false},
		{"auth0 tenants use", false},
		{"auth0 tenants list", false},
	}

	for index, testCase := range testCases {
		t.Run(fmt.Sprintf("TestCase #%d Command: %s", index, testCase.givenCommand), func(t *testing.T) {
			actualAuth := commandRequiresAuthentication(testCase.givenCommand)
			assert.Equal(t, testCase.expectedToRequireAuthentication, actualAuth)
		})
	}
}
