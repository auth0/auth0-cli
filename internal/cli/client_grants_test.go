package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/auth0/go-auth0/management"
	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
)

func TestClientGrantsPickerOptions(t *testing.T) {
	// The picker reads only the first page, so a page with just Results set is
	// all the fixture needs.
	firstPage := func(grants []*managementv3.ClientGrantResponseContent) *auth0.ClientGrantPage {
		return &auth0.ClientGrantPage{Results: grants}
	}

	tests := []struct {
		name         string
		page         *auth0.ClientGrantPage
		apiError     error
		assertOutput func(t testing.TB, options pickerOptions)
		assertError  func(t testing.TB, err error)
	}{
		{
			name: "happy path",
			page: firstPage([]*managementv3.ClientGrantResponseContent{
				{
					ID:       auth0.String("cgr_1"),
					ClientID: auth0.String("client-id-1"),
					Audience: auth0.String("https://travel0.com/api"),
				},
				{
					ID:       auth0.String("cgr_2"),
					ClientID: auth0.String("client-id-2"),
					Audience: auth0.String("https://travel0.com/api"),
				},
			}),
			assertOutput: func(t testing.TB, options pickerOptions) {
				assert.Len(t, options, 2)
				assert.Equal(t, "client-id-1 (https://travel0.com/api)", options[0].label)
				assert.Equal(t, "cgr_1", options[0].value)
				assert.Equal(t, "client-id-2 (https://travel0.com/api)", options[1].label)
				assert.Equal(t, "cgr_2", options[1].value)
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name: "default_for grant falls back to default_for label",
			page: firstPage([]*managementv3.ClientGrantResponseContent{
				{
					ID:         auth0.String("cgr_3"),
					DefaultFor: managementv3.ClientGrantDefaultForEnumThirdPartyClients.Ptr(),
					Audience:   auth0.String("https://travel0.com/api"),
				},
			}),
			assertOutput: func(t testing.TB, options pickerOptions) {
				assert.Len(t, options, 1)
				assert.Equal(t, "third_party_clients (https://travel0.com/api)", options[0].label)
				assert.Equal(t, "cgr_3", options[0].value)
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name: "no client grants",
			page: firstPage([]*managementv3.ClientGrantResponseContent{}),
			assertOutput: func(t testing.TB, options pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorContains(t, err, "there are currently no client grants to choose from. Create one by running: `auth0 client-grants create`")
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

			clientGrantAPI := mock.NewMockClientGrantAPIV3(ctrl)
			clientGrantAPI.EXPECT().
				List(gomock.Any(), gomock.Any()).
				Return(test.page, test.apiError)

			cli := &cli{
				apiv3: &auth0.APIV3{ClientGrant: clientGrantAPI},
			}

			options, err := cli.clientGrantPickerOptions(context.Background())

			if err != nil {
				test.assertError(t, err)
			} else {
				test.assertOutput(t, options)
			}
		})
	}
}

func TestMutableClientGrantPickerOptions(t *testing.T) {
	page := &auth0.ClientGrantPage{Results: []*managementv3.ClientGrantResponseContent{
		{
			ID:       auth0.String("cgr_1"),
			ClientID: auth0.String("client-id-1"),
			Audience: auth0.String("https://travel0.com/api"),
		},
		{
			ID:       auth0.String("cgr_system"),
			ClientID: auth0.String("client-id-system"),
			Audience: auth0.String("https://travel0.com/api"),
			IsSystem: auth0.Bool(true),
		},
	}}

	t.Run("excludes system grants", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		clientGrantAPI := mock.NewMockClientGrantAPIV3(ctrl)
		clientGrantAPI.EXPECT().
			List(gomock.Any(), gomock.Any()).
			Return(page, nil)

		cli := &cli{apiv3: &auth0.APIV3{ClientGrant: clientGrantAPI}}

		options, err := cli.mutableClientGrantPickerOptions(context.Background())

		assert.NoError(t, err)
		assert.Len(t, options, 1)
		assert.Equal(t, "cgr_1", options[0].value)
	})

	t.Run("errors when only system grants exist", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		clientGrantAPI := mock.NewMockClientGrantAPIV3(ctrl)
		clientGrantAPI.EXPECT().
			List(gomock.Any(), gomock.Any()).
			Return(&auth0.ClientGrantPage{Results: []*managementv3.ClientGrantResponseContent{
				{
					ID:       auth0.String("cgr_system"),
					ClientID: auth0.String("client-id-system"),
					Audience: auth0.String("https://travel0.com/api"),
					IsSystem: auth0.Bool(true),
				},
			}}, nil)

		cli := &cli{apiv3: &auth0.APIV3{ClientGrant: clientGrantAPI}}

		_, err := cli.mutableClientGrantPickerOptions(context.Background())

		assert.ErrorContains(t, err, "there are currently no client grants to choose from")
	})
}

func TestUpdateClientGrantCmd(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		grant         *managementv3.GetClientGrantResponseContent
		expectedError string
	}{
		{
			name: "fails fast on a system grant",
			args: []string{"cgr_system", "--scopes", "read:todos"},
			grant: &managementv3.GetClientGrantResponseContent{
				ID:       auth0.String("cgr_system"),
				Audience: auth0.String("https://travel0.com/api"),
				IsSystem: auth0.Bool(true),
			},
			expectedError: `client grant with ID "cgr_system" is a system grant and cannot be updated`,
		},
		{
			name: "rejects organization settings for a user subject type",
			args: []string{"cgr_user", "--organization-usage", "allow"},
			grant: &managementv3.GetClientGrantResponseContent{
				ID:          auth0.String("cgr_user"),
				Audience:    auth0.String("https://travel0.com/api"),
				SubjectType: managementv3.ClientGrantSubjectTypeEnumUser.Ptr(),
			},
			expectedError: `--organization-usage and --allow-any-organization cannot be set when --subject-type is "user"`,
		},
		{
			name: "rejects allow-any-organization for an anonymous_user subject type",
			args: []string{"cgr_anon", "--allow-any-organization=true"},
			grant: &managementv3.GetClientGrantResponseContent{
				ID:          auth0.String("cgr_anon"),
				Audience:    auth0.String("https://travel0.com/api"),
				SubjectType: managementv3.ClientGrantSubjectTypeEnumAnonymousUser.Ptr(),
			},
			expectedError: `--organization-usage and --allow-any-organization cannot be set when --subject-type is "anonymous_user"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			clientGrantAPI := mock.NewMockClientGrantAPIV3(ctrl)
			clientGrantAPI.EXPECT().
				Get(gomock.Any(), test.grant.GetID()).
				Return(test.grant, nil)

			cli := &cli{apiv3: &auth0.APIV3{ClientGrant: clientGrantAPI}}
			cli.noInput = true // Non-interactive mode.

			cmd := updateClientGrantCmd(cli)
			cmd.SetArgs(test.args)

			assert.EqualError(t, cmd.Execute(), test.expectedError)
		})
	}
}

func TestDeleteClientGrantCmd(t *testing.T) {
	t.Run("fails fast on a system grant", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		clientGrantAPI := mock.NewMockClientGrantAPIV3(ctrl)
		clientGrantAPI.EXPECT().
			Get(gomock.Any(), "cgr_system").
			Return(&managementv3.GetClientGrantResponseContent{
				ID:       auth0.String("cgr_system"),
				IsSystem: auth0.Bool(true),
			}, nil)

		cli := &cli{apiv3: &auth0.APIV3{ClientGrant: clientGrantAPI}}
		cli.noInput = true // Non-interactive mode.

		cmd := deleteClientGrantCmd(cli)
		cmd.SetArgs([]string{"cgr_system", "--force"})

		assert.EqualError(t, cmd.Execute(), `client grant with ID "cgr_system" is a system grant and cannot be deleted`)
	})
}

func TestValidateClientGrantSubjectType(t *testing.T) {
	tests := []struct {
		name                 string
		subjectType          string
		organizationUsage    string
		allowAnyOrganization bool
		wantErr              bool
	}{
		{
			name:        "client subject type with no org settings is valid",
			subjectType: "client",
		},
		{
			name:                 "client subject type with org settings is valid",
			subjectType:          "client",
			organizationUsage:    "allow",
			allowAnyOrganization: true,
		},
		{
			name:        "user subject type with no org settings is valid",
			subjectType: "user",
		},
		{
			name:              "user subject type with organization usage is rejected",
			subjectType:       "user",
			organizationUsage: "allow",
			wantErr:           true,
		},
		{
			name:                 "user subject type with allow-any-organization is rejected",
			subjectType:          "user",
			allowAnyOrganization: true,
			wantErr:              true,
		},
		{
			name:        "anonymous_user subject type with no org settings is valid",
			subjectType: "anonymous_user",
		},
		{
			name:              "anonymous_user subject type with organization usage is rejected",
			subjectType:       "anonymous_user",
			organizationUsage: "allow",
			wantErr:           true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateClientGrantSubjectType(test.subjectType, test.organizationUsage, test.allowAnyOrganization)
			if test.wantErr {
				assert.ErrorContains(t, err, "cannot be set when --subject-type is")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateClientGrantOrganization(t *testing.T) {
	tests := []struct {
		name                 string
		organizationUsage    string
		allowAnyOrganization bool
		wantErr              bool
	}{
		{
			name:                 "allow-any-organization off is always valid",
			organizationUsage:    "",
			allowAnyOrganization: false,
		},
		{
			name:                 "allow-any-organization on with allow usage is valid",
			organizationUsage:    "allow",
			allowAnyOrganization: true,
		},
		{
			name:                 "allow-any-organization on with require usage is valid",
			organizationUsage:    "require",
			allowAnyOrganization: true,
		},
		{
			name:                 "allow-any-organization on with deny usage is rejected",
			organizationUsage:    "deny",
			allowAnyOrganization: true,
			wantErr:              true,
		},
		{
			name:                 "allow-any-organization on with no usage is rejected",
			organizationUsage:    "",
			allowAnyOrganization: true,
			wantErr:              true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateClientGrantOrganization(test.organizationUsage, test.allowAnyOrganization)
			if test.wantErr {
				assert.ErrorContains(t, err, "--allow-any-organization can only be enabled")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestResolveUpdateClientGrantScopes(t *testing.T) {
	tests := []struct {
		name            string
		newScopes       []string
		newAllowAll     bool
		currentScopes   []string
		currentAllowAll bool
		wantScope       []string
		wantAllowAll    *bool
	}{
		{
			name:          "new scopes win over an existing specific grant",
			newScopes:     []string{"read:users"},
			currentScopes: []string{"read:foo"},
			wantScope:     []string{"read:users"},
			wantAllowAll:  nil,
		},
		{
			name:            "switching allow-all to specific clears allow_all_scopes",
			newScopes:       []string{"read:users"},
			currentAllowAll: true,
			wantScope:       []string{"read:users"},
			wantAllowAll:    auth0.Bool(false),
		},
		{
			name:         "explicit allow-all sets allow_all_scopes",
			newAllowAll:  true,
			wantScope:    nil,
			wantAllowAll: auth0.Bool(true),
		},
		{
			name:            "no changes preserves an existing allow-all grant",
			currentAllowAll: true,
			wantScope:       nil,
			wantAllowAll:    auth0.Bool(true),
		},
		{
			name:          "no changes preserves existing specific scopes",
			currentScopes: []string{"read:foo", "read:bar"},
			wantScope:     []string{"read:foo", "read:bar"},
			wantAllowAll:  nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scope, allowAll := resolveUpdateClientGrantScopes(test.newScopes, test.newAllowAll, test.currentScopes, test.currentAllowAll)
			assert.Equal(t, test.wantScope, scope)
			assert.Equal(t, test.wantAllowAll, allowAll)
		})
	}
}

func TestAPIIdentifierPickerOptions(t *testing.T) {
	tests := []struct {
		name         string
		apis         []*management.ResourceServer
		apiError     error
		assertOutput func(t testing.TB, options pickerOptions)
		assertError  func(t testing.TB, err error)
	}{
		{
			name: "picker value is the identifier, not the API id",
			apis: []*management.ResourceServer{
				{
					ID:         auth0.String("api-id-1"),
					Identifier: auth0.String("https://travel0.com/api"),
					Name:       auth0.String("Travel0 API"),
				},
			},
			assertOutput: func(t testing.TB, options pickerOptions) {
				assert.Len(t, options, 1)
				assert.Equal(t, "Travel0 API (https://travel0.com/api)", options[0].label)
				assert.Equal(t, "https://travel0.com/api", options[0].value)
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name: "falls back to a custom API label when the API has no name",
			apis: []*management.ResourceServer{
				{
					ID:         auth0.String("api-id-1"),
					Identifier: auth0.String("https://travel0.com/api"),
				},
			},
			assertOutput: func(t testing.TB, options pickerOptions) {
				assert.Len(t, options, 1)
				assert.Equal(t, "custom API (https://travel0.com/api)", options[0].label)
				assert.Equal(t, "https://travel0.com/api", options[0].value)
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name: "no apis",
			apis: []*management.ResourceServer{},
			assertOutput: func(t testing.TB, options pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorContains(t, err, "there are currently no APIs to choose from. Create one by running: `auth0 apis create`")
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

			apiAPI := mock.NewMockResourceServerAPI(ctrl)
			apiAPI.EXPECT().
				List(gomock.Any()).
				Return(&management.ResourceServerList{ResourceServers: test.apis}, test.apiError)

			cli := &cli{
				api: &auth0.API{ResourceServer: apiAPI},
			}

			options, err := cli.apiIdentifierPickerOptions(context.Background())

			if err != nil {
				test.assertError(t, err)
			} else {
				test.assertOutput(t, options)
			}
		})
	}
}
