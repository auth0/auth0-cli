package cli

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/auth0/go-auth0/management"
	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/option"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
	"github.com/auth0/auth0-cli/internal/display"
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
				assert.Equal(t, "cgr_1 (client-id-1, https://travel0.com/api)", options[0].label)
				assert.Equal(t, "cgr_1", options[0].value)
				assert.Equal(t, "cgr_2 (client-id-2, https://travel0.com/api)", options[1].label)
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
				assert.Equal(t, "cgr_3 (third_party_clients, https://travel0.com/api)", options[0].label)
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

func TestCreateClientGrantCmd(t *testing.T) {
	t.Run("errors when neither client-id nor default-for is set", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cli := &cli{apiv3: &auth0.APIV3{ClientGrant: mock.NewMockClientGrantAPIV3(ctrl)}}
		cli.noInput = true // Non-interactive mode.

		cmd := createClientGrantCmd(cli)
		cmd.SetArgs([]string{"--audience", "https://travel0.com/api"})

		assert.EqualError(t, cmd.Execute(), "one of --client-id or --default-for must be set")
	})

	t.Run("errors when client-id and default-for are both set", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cli := &cli{apiv3: &auth0.APIV3{ClientGrant: mock.NewMockClientGrantAPIV3(ctrl)}}
		cli.noInput = true // Non-interactive mode.

		cmd := createClientGrantCmd(cli)
		cmd.SetArgs([]string{
			"--audience", "https://travel0.com/api",
			"--client-id", "client-id-1",
			"--default-for", "third_party_clients",
		})

		assert.ErrorContains(t, cmd.Execute(), "[client-id default-for]")
	})

	t.Run("sends default_for when --default-for is set", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		var captured *managementv3.CreateClientGrantRequestContent
		clientGrantAPI := mock.NewMockClientGrantAPIV3(ctrl)
		clientGrantAPI.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, req *managementv3.CreateClientGrantRequestContent, _ ...option.RequestOption) (*managementv3.CreateClientGrantResponseContent, error) {
				captured = req
				return &managementv3.CreateClientGrantResponseContent{ID: auth0.String("cgr_1")}, nil
			})

		cli := &cli{
			apiv3:    &auth0.APIV3{ClientGrant: clientGrantAPI},
			renderer: &display.Renderer{MessageWriter: &bytes.Buffer{}, ResultWriter: &bytes.Buffer{}},
		}
		cli.noInput = true // Non-interactive mode.

		cmd := createClientGrantCmd(cli)
		cmd.SetArgs([]string{
			"--audience", "https://travel0.com/api",
			"--default-for", "third_party_clients",
		})

		assert.NoError(t, cmd.Execute())
		assert.Nil(t, captured.ClientID)
		assert.Equal(t, managementv3.ClientGrantDefaultForEnumThirdPartyClients, *captured.DefaultFor)
		// Organization and subject-type settings do not apply to a default grant
		// and the API rejects them, so they must never be sent.
		assert.Nil(t, captured.SubjectType)
		assert.Nil(t, captured.OrganizationUsage)
		assert.Nil(t, captured.AllowAnyOrganization)
	})

	t.Run("rejects organization flags with --default-for", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cli := &cli{apiv3: &auth0.APIV3{ClientGrant: mock.NewMockClientGrantAPIV3(ctrl)}}
		cli.noInput = true // Non-interactive mode.

		cmd := createClientGrantCmd(cli)
		cmd.SetArgs([]string{
			"--audience", "https://travel0.com/api",
			"--default-for", "third_party_clients",
			"--organization-usage", "allow",
		})

		assert.EqualError(t, cmd.Execute(), "--organization-usage cannot be set with --default-for")
	})

	t.Run("sends authorization_details_types when --authorization-details-types is set", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		var captured *managementv3.CreateClientGrantRequestContent
		clientGrantAPI := mock.NewMockClientGrantAPIV3(ctrl)
		clientGrantAPI.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, req *managementv3.CreateClientGrantRequestContent, _ ...option.RequestOption) (*managementv3.CreateClientGrantResponseContent, error) {
				captured = req
				return &managementv3.CreateClientGrantResponseContent{ID: auth0.String("cgr_1")}, nil
			})

		cli := &cli{
			apiv3:    &auth0.APIV3{ClientGrant: clientGrantAPI},
			renderer: &display.Renderer{MessageWriter: &bytes.Buffer{}, ResultWriter: &bytes.Buffer{}},
		}
		cli.noInput = true // Non-interactive mode.

		cmd := createClientGrantCmd(cli)
		cmd.SetArgs([]string{
			"--client-id", "client-id-1",
			"--audience", "https://travel0.com/api",
			"--authorization-details-types", "payment,transfer",
		})

		assert.NoError(t, cmd.Execute())
		assert.Equal(t, []string{"payment", "transfer"}, captured.AuthorizationDetailsTypes)
	})

	t.Run("does not send organization settings when no organization flags are passed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		var captured *managementv3.CreateClientGrantRequestContent
		clientGrantAPI := mock.NewMockClientGrantAPIV3(ctrl)
		clientGrantAPI.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, req *managementv3.CreateClientGrantRequestContent, _ ...option.RequestOption) (*managementv3.CreateClientGrantResponseContent, error) {
				captured = req
				return &managementv3.CreateClientGrantResponseContent{ID: auth0.String("cgr_1")}, nil
			})

		cli := &cli{
			apiv3:    &auth0.APIV3{ClientGrant: clientGrantAPI},
			renderer: &display.Renderer{MessageWriter: &bytes.Buffer{}, ResultWriter: &bytes.Buffer{}},
		}
		cli.noInput = true // Non-interactive mode.

		cmd := createClientGrantCmd(cli)
		cmd.SetArgs([]string{
			"--client-id", "client-id-1",
			"--audience", "https://travel0.com/api",
		})

		// A grant that never touches organizations must not carry a stray
		// allow_any_organization, which the API rejects for system APIs.
		assert.NoError(t, cmd.Execute())
		assert.Nil(t, captured.OrganizationUsage)
		assert.Nil(t, captured.AllowAnyOrganization)
	})
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

func TestValidateClientGrantScopes(t *testing.T) {
	apiWithScopes := &management.ResourceServer{
		Identifier: auth0.String("https://travel0.com/api"),
		Scopes: &[]management.ResourceServerScope{
			{Value: auth0.String("read:users")},
			{Value: auth0.String("update:users")},
		},
	}
	apiWithoutScopes := &management.ResourceServer{
		Identifier: auth0.String("https://travel0.us.auth0.com/api/v2/"),
	}

	tests := []struct {
		name           string
		resourceServer *management.ResourceServer
		scopes         []string
		currentScopes  []string
		subjectType    string
		wantErr        string
	}{
		{
			name:           "scopes defined on the API are accepted",
			resourceServer: apiWithScopes,
			scopes:         []string{"read:users", "update:users"},
		},
		{
			name:           "an unknown scope is rejected",
			resourceServer: apiWithScopes,
			scopes:         []string{"read:users", "delete:users"},
			wantErr:        `the following scopes are not defined on the API "https://travel0.com/api": delete:users`,
		},
		{
			name:           "a scope already on the grant is accepted even if the API no longer defines it",
			resourceServer: apiWithScopes,
			scopes:         []string{"legacy:scope"},
			currentScopes:  []string{"legacy:scope"},
		},
		{
			name:           "an API with no scopes rejects specific scopes for the client subject type",
			resourceServer: apiWithoutScopes,
			scopes:         []string{"read:current_user"},
			subjectType:    "client",
			wantErr:        `the API "https://travel0.us.auth0.com/api/v2/" does not define any scopes`,
		},
		{
			name:           "an API with no scopes accepts inline scopes for the user subject type",
			resourceServer: apiWithoutScopes,
			scopes:         []string{"read:current_user"},
			subjectType:    "user",
		},
		{
			name:           "an API with no scopes accepts inline scopes for the anonymous_user subject type",
			resourceServer: apiWithoutScopes,
			scopes:         []string{"read:current_user"},
			subjectType:    "anonymous_user",
		},
		{
			name:           "no scopes passed is always valid",
			resourceServer: apiWithoutScopes,
			subjectType:    "client",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateClientGrantScopes(test.resourceServer, test.scopes, test.currentScopes, test.subjectType)
			if test.wantErr != "" {
				assert.ErrorContains(t, err, test.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateClientGrantScopeValidation(t *testing.T) {
	t.Run("rejects a scope not defined on the API", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		resourceServerAPI := mock.NewMockResourceServerAPI(ctrl)
		resourceServerAPI.EXPECT().
			Read(gomock.Any(), gomock.Any()).
			Return(&management.ResourceServer{
				Identifier: auth0.String("https://travel0.com/api"),
				Scopes: &[]management.ResourceServerScope{
					{Value: auth0.String("read:users")},
				},
			}, nil)

		cli := &cli{
			api:      &auth0.API{ResourceServer: resourceServerAPI},
			apiv3:    &auth0.APIV3{ClientGrant: mock.NewMockClientGrantAPIV3(ctrl)},
			renderer: &display.Renderer{MessageWriter: &bytes.Buffer{}, ResultWriter: &bytes.Buffer{}},
		}
		cli.noInput = true // Non-interactive mode.

		cmd := createClientGrantCmd(cli)
		cmd.SetArgs([]string{
			"--client-id", "client-id-1",
			"--audience", "https://travel0.com/api",
			"--scopes", "read:users,delete:users",
		})

		assert.ErrorContains(t, cmd.Execute(), `the following scopes are not defined on the API "https://travel0.com/api": delete:users`)
	})

	t.Run("sends scopes defined on the API", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		resourceServerAPI := mock.NewMockResourceServerAPI(ctrl)
		resourceServerAPI.EXPECT().
			Read(gomock.Any(), gomock.Any()).
			Return(&management.ResourceServer{
				Identifier: auth0.String("https://travel0.com/api"),
				Scopes: &[]management.ResourceServerScope{
					{Value: auth0.String("read:users")},
					{Value: auth0.String("update:users")},
				},
			}, nil)

		var captured *managementv3.CreateClientGrantRequestContent
		clientGrantAPI := mock.NewMockClientGrantAPIV3(ctrl)
		clientGrantAPI.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, req *managementv3.CreateClientGrantRequestContent, _ ...option.RequestOption) (*managementv3.CreateClientGrantResponseContent, error) {
				captured = req
				return &managementv3.CreateClientGrantResponseContent{ID: auth0.String("cgr_1")}, nil
			})

		cli := &cli{
			api:      &auth0.API{ResourceServer: resourceServerAPI},
			apiv3:    &auth0.APIV3{ClientGrant: clientGrantAPI},
			renderer: &display.Renderer{MessageWriter: &bytes.Buffer{}, ResultWriter: &bytes.Buffer{}},
		}
		cli.noInput = true // Non-interactive mode.

		cmd := createClientGrantCmd(cli)
		cmd.SetArgs([]string{
			"--client-id", "client-id-1",
			"--audience", "https://travel0.com/api",
			"--scopes", "read:users,update:users",
		})

		assert.NoError(t, cmd.Execute())
		assert.Equal(t, []string{"read:users", "update:users"}, captured.Scope)
	})
}

func TestUpdateClientGrantScopes(t *testing.T) {
	t.Run("--no-scopes clears scopes to an empty array", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		var captured *managementv3.UpdateClientGrantRequestContent
		clientGrantAPI := mock.NewMockClientGrantAPIV3(ctrl)
		clientGrantAPI.EXPECT().
			Get(gomock.Any(), "cgr_1").
			Return(&managementv3.GetClientGrantResponseContent{
				ID:       auth0.String("cgr_1"),
				Audience: auth0.String("https://travel0.com/api"),
				Scope:    []string{"read:users"},
			}, nil)
		clientGrantAPI.EXPECT().
			Update(gomock.Any(), "cgr_1", gomock.Any()).
			DoAndReturn(func(_ context.Context, _ string, req *managementv3.UpdateClientGrantRequestContent, _ ...option.RequestOption) (*managementv3.UpdateClientGrantResponseContent, error) {
				captured = req
				return &managementv3.UpdateClientGrantResponseContent{ID: auth0.String("cgr_1")}, nil
			})

		cli := &cli{
			apiv3:    &auth0.APIV3{ClientGrant: clientGrantAPI},
			renderer: &display.Renderer{MessageWriter: &bytes.Buffer{}, ResultWriter: &bytes.Buffer{}},
		}
		cli.noInput = true // Non-interactive mode.

		cmd := updateClientGrantCmd(cli)
		cmd.SetArgs([]string{"cgr_1", "--no-scopes"})

		assert.NoError(t, cmd.Execute())
		assert.NotNil(t, captured.Scope)
		assert.Empty(t, captured.Scope)
	})

	t.Run("rejects a scope not defined on the API", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		clientGrantAPI := mock.NewMockClientGrantAPIV3(ctrl)
		clientGrantAPI.EXPECT().
			Get(gomock.Any(), "cgr_1").
			Return(&managementv3.GetClientGrantResponseContent{
				ID:       auth0.String("cgr_1"),
				Audience: auth0.String("https://travel0.com/api"),
				Scope:    []string{"read:users"},
			}, nil)

		resourceServerAPI := mock.NewMockResourceServerAPI(ctrl)
		resourceServerAPI.EXPECT().
			Read(gomock.Any(), gomock.Any()).
			Return(&management.ResourceServer{
				Identifier: auth0.String("https://travel0.com/api"),
				Scopes: &[]management.ResourceServerScope{
					{Value: auth0.String("read:users")},
				},
			}, nil)

		cli := &cli{
			api:      &auth0.API{ResourceServer: resourceServerAPI},
			apiv3:    &auth0.APIV3{ClientGrant: clientGrantAPI},
			renderer: &display.Renderer{MessageWriter: &bytes.Buffer{}, ResultWriter: &bytes.Buffer{}},
		}
		cli.noInput = true // Non-interactive mode.

		cmd := updateClientGrantCmd(cli)
		cmd.SetArgs([]string{"cgr_1", "--scopes", "read:users,delete:users"})

		assert.ErrorContains(t, cmd.Execute(), `the following scopes are not defined on the API "https://travel0.com/api": delete:users`)
	})
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

func TestNonSystemAPIIdentifierPickerOptions(t *testing.T) {
	apis := []*management.ResourceServer{
		{
			ID:         auth0.String("api-id-1"),
			Identifier: auth0.String("https://travel0.com/api"),
			Name:       auth0.String("Travel0 API"),
		},
		{
			ID:         auth0.String("api-id-mgmt"),
			Identifier: auth0.String("https://travel0.us.auth0.com/api/v2/"),
			Name:       auth0.String("Auth0 Management API"),
			IsSystem:   auth0.Bool(true),
		},
	}

	t.Run("excludes system APIs", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		apiAPI := mock.NewMockResourceServerAPI(ctrl)
		apiAPI.EXPECT().
			List(gomock.Any()).
			Return(&management.ResourceServerList{ResourceServers: apis}, nil)

		cli := &cli{api: &auth0.API{ResourceServer: apiAPI}}

		options, err := cli.nonSystemAPIIdentifierPickerOptions(context.Background())

		assert.NoError(t, err)
		assert.Len(t, options, 1)
		assert.Equal(t, "https://travel0.com/api", options[0].value)
	})

	t.Run("errors when only system APIs exist", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		apiAPI := mock.NewMockResourceServerAPI(ctrl)
		apiAPI.EXPECT().
			List(gomock.Any()).
			Return(&management.ResourceServerList{ResourceServers: []*management.ResourceServer{
				{
					ID:         auth0.String("api-id-mgmt"),
					Identifier: auth0.String("https://travel0.us.auth0.com/api/v2/"),
					Name:       auth0.String("Auth0 Management API"),
					IsSystem:   auth0.Bool(true),
				},
			}}, nil)

		cli := &cli{api: &auth0.API{ResourceServer: apiAPI}}

		_, err := cli.nonSystemAPIIdentifierPickerOptions(context.Background())

		assert.ErrorContains(t, err, "there are currently no APIs to choose from")
	})
}
