package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/golang/mock/gomock"

	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
)

func TestNetworkACLPickerOptions(t *testing.T) {
	// Disable colors for consistent test output.
	tests := []struct {
		name         string
		networkACLs  []*management.NetworkACL
		apiError     error
		assertOutput func(t testing.TB, options pickerOptions)
		assertError  func(t testing.TB, err error)
	}{
		{
			name: "happy path",
			networkACLs: []*management.NetworkACL{
				{
					ID:          auth0.String("acl-id-1"),
					Description: auth0.String("Block IPs"),
				},
				{
					ID:          auth0.String("acl-id-2"),
					Description: auth0.String("Allow Countries"),
				},
			},
			assertOutput: func(t testing.TB, options pickerOptions) {
				assert.Len(t, options, 2)
				// Check the value which should not have ANSI formatting.
				assert.Equal(t, "acl-id-1", options[0].value)
				assert.Equal(t, "acl-id-2", options[1].value)

				// For labels, just check that they contain the expected text without worrying about ANSI codes.
				assert.Contains(t, options[0].label, "Block IPs")
				assert.Contains(t, options[0].label, "acl-id-1")
				assert.Contains(t, options[1].label, "Allow Countries")
				assert.Contains(t, options[1].label, "acl-id-2")
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name:        "no network ACLs",
			networkACLs: []*management.NetworkACL{},
			assertOutput: func(t testing.TB, options pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorContains(t, err, "there are currently no network ACLs to choose from")
			},
		},
		{
			name:     "API error",
			apiError: errors.New("error"),
			assertOutput: func(t testing.TB, options pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.Error(t, err)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			networkACLAPI := mock.NewMockNetworkACLAPI(ctrl)
			networkACLAPI.EXPECT().
				List(gomock.Any()).
				Return(test.networkACLs, test.apiError)

			cli := &cli{
				api: &auth0.API{NetworkACL: networkACLAPI},
			}

			options, err := cli.networkACLPickerOptions(context.Background())

			if err != nil {
				test.assertError(t, err)
			} else {
				test.assertOutput(t, options)
			}
		})
	}
}
