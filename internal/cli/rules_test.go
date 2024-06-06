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

func TestRulesPickerOptions(t *testing.T) {
	tests := []struct {
		name         string
		rules        []*management.Rule
		apiError     error
		assertOutput func(t testing.TB, options pickerOptions)
		assertError  func(t testing.TB, err error)
	}{
		{
			name: "happy path",
			rules: []*management.Rule{
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
				assert.Equal(t, "some-name-1", options[0].label)
				assert.Equal(t, "some-id-1", options[0].value)
				assert.Equal(t, "some-name-2", options[1].label)
				assert.Equal(t, "some-id-2", options[1].value)
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name:  "no rules",
			rules: []*management.Rule{},
			assertOutput: func(t testing.TB, options pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorContains(t, err, "there are currently no rules to choose from")
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

			ruleAPI := mock.NewMockRuleAPI(ctrl)
			ruleAPI.EXPECT().
				List(gomock.Any()).
				Return(&management.RuleList{
					Rules: test.rules}, test.apiError)

			cli := &cli{
				api: &auth0.API{Rule: ruleAPI},
			}

			options, err := cli.rulePickerOptions(context.Background())

			if err != nil {
				test.assertError(t, err)
			} else {
				test.assertOutput(t, options)
			}
		})
	}
}
