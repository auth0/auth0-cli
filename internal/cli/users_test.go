package cli

import (
	"errors"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/golang/mock/gomock"

	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
)

func TestConnectionsPickerOptions(t *testing.T) {
	tests := []struct {
		name         string
		connections  []*management.Connection
		apiError     error
		assertOutput func(t testing.TB, options []string)
		assertError  func(t testing.TB, err error)
	}{
		{
			name: "happy path",
			connections: []*management.Connection{
				{
					Name:     auth0.String("some-name-1"),
					Strategy: auth0.String("auth0"),
				},
				{
					Name:     auth0.String("some-name-2"),
					Strategy: auth0.String("auth0"),
				},
			},
			assertOutput: func(t testing.TB, options []string) {
				assert.Len(t, options, 2)
				assert.Equal(t, "some-name-1", options[0])
				assert.Equal(t, "some-name-2", options[1])
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name:        "no connections",
			connections: []*management.Connection{},
			assertOutput: func(t testing.TB, options []string) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorContains(t, err, "There are currently no database connections.")
			},
		},
		{
			name: "no database connections",
			connections: []*management.Connection{
				{
					Name:     auth0.String("some-name-1"),
					Strategy: auth0.String("foo"),
				},
				{
					Name:     auth0.String("some-name-2"),
					Strategy: auth0.String("foo"),
				},
			},
			assertOutput: func(t testing.TB, options []string) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorContains(t, err, "There are currently no database connections.")
			},
		},
		{
			name:     "API error",
			apiError: errors.New("error"),
			assertOutput: func(t testing.TB, options []string) {
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

			connectionAPI := mock.NewMockConnectionAPI(ctrl)
			connectionAPI.EXPECT().
				List(gomock.Any()).
				Return(&management.ConnectionList{
					Connections: test.connections}, test.apiError)

			cli := &cli{
				api: &auth0.API{Connection: connectionAPI},
			}

			options, err := cli.connectionPickerOptions()

			if err != nil {
				test.assertError(t, err)
			} else {
				test.assertOutput(t, options)
			}
		})
	}
}
