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

func TestOrganizationsPickerOptions(t *testing.T) {
	tests := []struct {
		name          string
		organizations []*management.Organization
		apiError      error
		assertOutput  func(t testing.TB, options pickerOptions)
		assertError   func(t testing.TB, err error)
	}{
		{
			name: "happy path",
			organizations: []*management.Organization{
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
			name:          "no organizations",
			organizations: []*management.Organization{},
			assertOutput: func(t testing.TB, options pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorContains(t, err, "there are currently no organizations to choose from. Create one by running: `auth0 orgs create`")
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

			organizationAPI := mock.NewMockOrganizationAPI(ctrl)
			organizationAPI.EXPECT().
				List(gomock.Any()).
				Return(&management.OrganizationList{
					Organizations: test.organizations}, test.apiError)

			cli := &cli{
				api: &auth0.API{Organization: organizationAPI},
			}

			options, err := cli.organizationPickerOptions(context.Background())

			if err != nil {
				test.assertError(t, err)
			} else {
				test.assertOutput(t, options)
			}
		})
	}
}

func TestInvitationsPickerOptions(t *testing.T) {
	tests := []struct {
		name         string
		invitations  []*management.OrganizationInvitation
		apiError     error
		assertOutput func(t testing.TB, options pickerOptions)
		assertError  func(t testing.TB, err error)
	}{
		{
			name: "happy path",
			invitations: []*management.OrganizationInvitation{
				{
					ID: auth0.String("inv-id-1"),
					Invitee: &management.OrganizationInvitationInvitee{
						Email: auth0.String("user1@example.com"),
					},
				},
				{
					ID: auth0.String("inv-id-2"),
					Invitee: &management.OrganizationInvitationInvitee{
						Email: auth0.String("user2@example.com"),
					},
				},
			},
			assertOutput: func(t testing.TB, options pickerOptions) {
				assert.Len(t, options, 2)
				assert.Equal(t, "user1@example.com (inv-id-1)", options[0].label)
				assert.Equal(t, "inv-id-1", options[0].value)
				assert.Equal(t, "user2@example.com (inv-id-2)", options[1].label)
				assert.Equal(t, "inv-id-2", options[1].value)
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name:        "no invitations",
			invitations: []*management.OrganizationInvitation{},
			assertOutput: func(t testing.TB, options pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.Error(t, err)
				assert.ErrorContains(t, err, "there are currently no invitations to choose from")
			},
		},
		{
			name:     "API error",
			apiError: errors.New("api error"),
			assertOutput: func(t testing.TB, options pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.Error(t, err)
				assert.ErrorContains(t, err, "api error")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			organizationAPI := mock.NewMockOrganizationAPI(ctrl)
			organizationAPI.EXPECT().
				Invitations(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&management.OrganizationInvitationList{
					OrganizationInvitations: test.invitations,
				}, test.apiError)

			cli := &cli{
				api: &auth0.API{Organization: organizationAPI},
			}

			options, err := cli.invitationPickerOptions(context.Background(), "test-org-id")

			if err != nil {
				test.assertError(t, err)
			} else {
				test.assertOutput(t, options)
			}
		})
	}
}
