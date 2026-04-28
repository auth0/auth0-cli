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

func TestOrganizationPickerOptionsForGrant(t *testing.T) {
	const audience = "https://cli-demo.us.auth0.com/api/v2/"

	tests := []struct {
		name          string
		orgList       *management.OrganizationList
		apiError      error
		expectedError string
		expectedOpts  pickerOptions
	}{
		{
			name:          "api error fetching organizations",
			apiError:      errors.New("unexpected error"),
			expectedError: "unexpected error",
		},
		{
			name:    "no organizations exist",
			orgList: &management.OrganizationList{},
			expectedError: "the client grant for " + audience + " requires an organization, but no organizations exist on this tenant.\n\n" +
				"Create one by running: 'auth0 orgs create'",
		},
		{
			name: "organizations exist",
			orgList: &management.OrganizationList{
				Organizations: []*management.Organization{
					{ID: auth0.String("org_abc123"), Name: auth0.String("My Org")},
					{ID: auth0.String("org_def456"), Name: auth0.String("Other Org")},
				},
			},
			expectedOpts: pickerOptions{
				{value: "org_abc123", label: "My Org (org_abc123)"},
				{value: "org_def456", label: "Other Org (org_def456)"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			orgAPI := mock.NewMockOrganizationAPI(ctrl)
			orgAPI.EXPECT().
				List(gomock.Any(), gomock.Any()).
				Return(test.orgList, test.apiError)

			cli := &cli{
				api: &auth0.API{Organization: orgAPI},
			}

			opts, err := cli.organizationPickerOptionsForGrant(audience)(context.Background())

			if test.expectedError != "" {
				assert.ErrorContains(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expectedOpts, opts)
			}
		})
	}
}
