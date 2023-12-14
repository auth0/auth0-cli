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

func TestLogStreamsPickerOptions(t *testing.T) {
	tests := []struct {
		name         string
		logStreams   []*management.LogStream
		apiError     error
		assertOutput func(t testing.TB, options pickerOptions)
		assertError  func(t testing.TB, err error)
	}{
		{
			name: "happy path",
			logStreams: []*management.LogStream{
				{
					ID:   auth0.String("some-id-1"),
					Name: auth0.String("some-name-1"),
				},
				{
					ID:   auth0.String("some-id-2"),
					Name: auth0.String("some-name-2"),
				},
			},
			assertOutput: func(t testing.TB, options pickerOptions) {
				assert.Len(t, options, 2)
				assert.Equal(t, "some-name-1 (some-id-1)", options[0].label)
				assert.Equal(t, "some-id-1", options[0].value)
				assert.Equal(t, "some-name-2 (some-id-2)", options[1].label)
				assert.Equal(t, "some-id-2", options[1].value)
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name:       "no log streams",
			logStreams: []*management.LogStream{},
			assertOutput: func(t testing.TB, options pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorContains(t, err, "there are currently no log streams to choose from. Create one by running: `auth0 logs streams create`")
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

			logStreamAPI := mock.NewMockLogStreamAPI(ctrl)
			logStreamAPI.EXPECT().
				List(gomock.Any()).
				Return(test.logStreams, test.apiError)

			cli := &cli{
				api: &auth0.API{LogStream: logStreamAPI},
			}

			options, err := cli.allLogStreamsPickerOptions(context.Background())

			if err != nil {
				test.assertError(t, err)
			} else {
				test.assertOutput(t, options)
			}
		})
	}
}

func TestLogStreamsPickerOptionsByType(t *testing.T) {
	tests := []struct {
		name           string
		logStreams     []*management.LogStream
		logsStreamType logStreamType
		apiError       error
		assertOutput   func(t testing.TB, options pickerOptions)
		assertError    func(t testing.TB, err error)
	}{
		{
			name: "happy path",
			logStreams: []*management.LogStream{
				{
					ID:   auth0.String("some-id-1"),
					Name: auth0.String("some-name-1"),
					Type: auth0.String("bar"),
				},
				{
					ID:   auth0.String("some-id-2"),
					Name: auth0.String("some-name-2"),
					Type: auth0.String("foo"),
				},
				{
					ID:   auth0.String("some-id-3"),
					Name: auth0.String("some-name-3"),
					Type: auth0.String("foo"),
				},
			},
			logsStreamType: "foo",
			assertOutput: func(t testing.TB, options pickerOptions) {
				assert.Len(t, options, 2)
				assert.Equal(t, "some-name-2 (some-id-2)", options[0].label)
				assert.Equal(t, "some-id-2", options[0].value)
				assert.Equal(t, "some-name-3 (some-id-3)", options[1].label)
				assert.Equal(t, "some-id-3", options[1].value)
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name: "no log streams of type",
			logStreams: []*management.LogStream{
				{
					ID:   auth0.String(""),
					Name: auth0.String(""),
					Type: auth0.String("bar"),
				},
			},
			logsStreamType: "foo",
			assertOutput: func(t testing.TB, options pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorContains(t, err, "there are currently no log streams of type: \"foo\", use 'auth0 logs streams create foo' to create one")
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

			logStreamAPI := mock.NewMockLogStreamAPI(ctrl)
			logStreamAPI.EXPECT().
				List(gomock.Any()).
				Return(test.logStreams, test.apiError)

			cli := &cli{
				api: &auth0.API{LogStream: logStreamAPI},
			}

			pickerOptionsFunc := cli.logStreamPickerOptionsByType(test.logsStreamType)
			options, err := pickerOptionsFunc(context.Background())

			if err != nil {
				test.assertError(t, err)
			} else {
				test.assertOutput(t, options)
			}
		})
	}
}

func TestErrInvalidLogStreamType(t *testing.T) {
	err := errInvalidLogStreamType("some-id", "actual-type", "expected-type")

	assert.ErrorContains(t, err, "The log stream with ID \"some-id\" is of type \"actual-type\" instead of \"expected-type\", "+
		"use 'auth0 logs streams update actual-type' to update it instead.")
}
