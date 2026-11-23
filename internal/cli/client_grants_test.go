package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/auth0/go-auth0/management"
	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

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

// newClientGrantSearchPages chains pages of search results, ending with core.ErrNoPages.
func newClientGrantSearchPages(pages ...[]*managementv3.ClientGrantResponseContent) *auth0.ClientGrantSearchPage {
	var build func(i int) *auth0.ClientGrantSearchPage
	build = func(i int) *auth0.ClientGrantSearchPage {
		return &auth0.ClientGrantSearchPage{
			Results: pages[i],
			NextPageFunc: func(context.Context) (*auth0.ClientGrantSearchPage, error) {
				if i+1 >= len(pages) {
					return nil, core.ErrNoPages
				}
				return build(i + 1), nil
			},
		}
	}

	return build(0)
}

// searchClientGrants returns one search result per grant ID.
func searchClientGrants(ids ...string) []*managementv3.ClientGrantResponseContent {
	grants := make([]*managementv3.ClientGrantResponseContent, 0, len(ids))
	for _, id := range ids {
		grants = append(grants, &managementv3.ClientGrantResponseContent{ID: auth0.String(id)})
	}

	return grants
}

func TestClientGrantSortField(t *testing.T) {
	tests := []struct {
		input   string
		want    managementv3.ClientGrantSortFieldEnum
		wantErr string
	}{
		{input: "audience", want: managementv3.ClientGrantSortFieldEnumAudience},
		{input: "updated_at", want: managementv3.ClientGrantSortFieldEnumUpdatedAt},
		{input: "audience:1", want: managementv3.ClientGrantSortFieldEnumAudience},
		{input: "audience:-1", wantErr: `invalid --sort "audience:-1": the search endpoint only sorts ascending, use 'field' or 'field:1'`},
		{input: "client_id", wantErr: `invalid --sort "client_id": must be one of audience,updated_at`},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := clientGrantSortField(test.input)
			if test.wantErr != "" {
				assert.EqualError(t, err, test.wantErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestPairClientGrantSearchFields(t *testing.T) {
	tests := []struct {
		name   string
		fields string
		want   string
	}{
		{name: "leaves an empty value empty", fields: "", want: ""},
		{name: "leaves fields without a pair unchanged", fields: "id,audience", want: "id,audience"},
		{name: "adds allow_all_scopes to scope", fields: "id,scope", want: "id,scope,allow_all_scopes"},
		{name: "adds scope to allow_all_scopes", fields: "allow_all_scopes", want: "allow_all_scopes,scope"},
		{name: "adds default_for to client_id", fields: "client_id", want: "client_id,default_for"},
		{name: "adds client_id to default_for", fields: "default_for", want: "default_for,client_id"},
		{name: "keeps a complete pair as is", fields: "scope,allow_all_scopes", want: "scope,allow_all_scopes"},
		{name: "pairs both pairs and trims the list", fields: " scope , ,client_id ", want: "scope,client_id,default_for,allow_all_scopes"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, pairClientGrantSearchFields(test.fields))
		})
	}
}

func TestBuildClientGrantSearchRequest(t *testing.T) {
	tests := []struct {
		name    string
		inputs  searchInputs
		want    *managementv3.SearchClientGrantsRequestParameters
		wantErr string
	}{
		{
			name:   "defaults to lucene and omits unset params",
			inputs: searchInputs{Number: 50},
			want: &managementv3.SearchClientGrantsRequestParameters{
				Parser: managementv3.SearchParserEnumLucene.Ptr(),
				Take:   auth0.Int(50),
			},
		},
		{
			name:   "maps every flag onto the request",
			inputs: searchInputs{Filter: `audience co "example"`, Parser: "scim", Sort: "updated_at:1", Fields: "id,audience", Number: 20},
			want: &managementv3.SearchClientGrantsRequestParameters{
				Q:      auth0.String(`audience co "example"`),
				Parser: managementv3.SearchParserEnumSCIM.Ptr(),
				Sort:   managementv3.ClientGrantSortFieldEnumUpdatedAt.Ptr(),
				Fields: auth0.String("id,audience"),
				Take:   auth0.Int(20),
			},
		},
		{
			name:   "sends include_fields=false with --exclude-fields",
			inputs: searchInputs{Fields: "scope", ExcludeFields: true, Number: 50},
			want: &managementv3.SearchClientGrantsRequestParameters{
				Fields:        auth0.String("scope"),
				IncludeFields: auth0.Bool(false),
				Parser:        managementv3.SearchParserEnumLucene.Ptr(),
				Take:          auth0.Int(50),
			},
		},
		{
			name:   "caps the page size at 100",
			inputs: searchInputs{Number: 1000},
			want: &managementv3.SearchClientGrantsRequestParameters{
				Parser: managementv3.SearchParserEnumLucene.Ptr(),
				Take:   auth0.Int(100),
			},
		},
		{
			name:    "rejects an invalid sort",
			inputs:  searchInputs{Sort: "client_id", Number: 50},
			wantErr: `invalid --sort "client_id": must be one of audience,updated_at`,
		},
		{
			name:    "rejects invalid shared inputs",
			inputs:  searchInputs{ExcludeFields: true, Number: 50},
			wantErr: "--exclude-fields requires --fields",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := buildClientGrantSearchRequest(test.inputs)
			if test.wantErr != "" {
				assert.EqualError(t, err, test.wantErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestSearchClientGrantsCmd(t *testing.T) {
	thousandIDs := make([]string, 1000)
	for i := range thousandIDs {
		thousandIDs[i] = fmt.Sprintf("cgr_%d", i)
	}

	allowAll := true
	tests := []struct {
		name          string
		args          []string
		searchPage    *auth0.ClientGrantSearchPage
		searchErr     error
		serverStatus  int
		serverBody    string
		assertRequest func(t testing.TB, req *managementv3.SearchClientGrantsRequestParameters)
		assertHTTP    func(t testing.TB, r *http.Request)
		assertOutput  func(t testing.TB, stdout, stderr string)
		wantErr       string
	}{
		{
			name:       "sends the flags to the search endpoint",
			args:       []string{"--filter", `audience:"https://api.example.com"`, "-s", "audience", "--fields", "id,audience", "-n", "10"},
			searchPage: newClientGrantSearchPages(searchClientGrants("cgr_1")),
			assertRequest: func(t testing.TB, req *managementv3.SearchClientGrantsRequestParameters) {
				assert.Equal(t, `audience:"https://api.example.com"`, *req.Q)
				assert.Equal(t, managementv3.SearchParserEnumLucene, *req.Parser)
				assert.Equal(t, managementv3.ClientGrantSortFieldEnumAudience, *req.Sort)
				assert.Equal(t, "id,audience", *req.Fields)
				assert.Equal(t, 10, *req.Take)
			},
		},
		{
			name: "renders the client-grants list columns",
			searchPage: newClientGrantSearchPages([]*managementv3.ClientGrantResponseContent{
				{ID: auth0.String("cgr_1"), ClientID: auth0.String("client_1"), Audience: auth0.String("https://api.example.com"), Scope: []string{"read:a", "write:a"}},
				{ID: auth0.String("cgr_2"), DefaultFor: managementv3.ClientGrantDefaultForEnumThirdPartyClients.Ptr(), Audience: auth0.String("https://other"), AllowAllScopes: &allowAll},
			}),
			assertOutput: func(t testing.TB, stdout, stderr string) {
				expectTable(t, stdout,
					[]string{"ID", "CLIENT ID", "AUDIENCE", "SCOPES"},
					[][]string{
						{"cgr_1", "client_1", "https://api.example.com", "2"},
						{"cgr_2", "third_party_clients", "https://other", "all"},
					},
				)
			},
		},
		{
			name:       "sends the partner of each paired field",
			args:       []string{"--fields", "id,scope,client_id"},
			searchPage: newClientGrantSearchPages(searchClientGrants("cgr_1")),
			assertRequest: func(t testing.TB, req *managementv3.SearchClientGrantsRequestParameters) {
				assert.Equal(t, "id,scope,client_id,default_for,allow_all_scopes", *req.Fields)
			},
		},
		{
			name:       "keeps a column when only the partner field is requested",
			args:       []string{"--fields", "default_for,allow_all_scopes"},
			searchPage: newClientGrantSearchPages(searchClientGrants("cgr_1")),
			assertRequest: func(t testing.TB, req *managementv3.SearchClientGrantsRequestParameters) {
				assert.Equal(t, "default_for,allow_all_scopes,client_id,scope", *req.Fields)
			},
			assertOutput: func(t testing.TB, stdout, stderr string) {
				assert.Contains(t, stdout, "CLIENT ID")
				assert.Contains(t, stdout, "SCOPES")
				assert.NotContains(t, stdout, "AUDIENCE")
				assert.NotContains(t, stderr, "No table columns")
			},
		},
		{
			name:       "excludes both fields of a pair with --exclude-fields",
			args:       []string{"--fields", "scope", "--exclude-fields"},
			searchPage: newClientGrantSearchPages(searchClientGrants("cgr_1")),
			assertRequest: func(t testing.TB, req *managementv3.SearchClientGrantsRequestParameters) {
				assert.Equal(t, "scope,allow_all_scopes", *req.Fields)
				assert.False(t, *req.IncludeFields)
			},
			assertOutput: func(t testing.TB, stdout, stderr string) {
				assert.Contains(t, stdout, "AUDIENCE")
				assert.NotContains(t, stdout, "SCOPES")
			},
		},
		{
			name:       "sends --query fields unchanged",
			args:       []string{"-q", `{"fields":"scope"}`},
			serverBody: `{"client_grants":[]}`,
			assertHTTP: func(t testing.TB, r *http.Request) {
				assert.Equal(t, "scope", r.URL.Query().Get("fields"))
			},
		},
		{
			name:       "sends --exclude-fields as include_fields=false",
			args:       []string{"--fields", "audience", "--exclude-fields"},
			searchPage: newClientGrantSearchPages(searchClientGrants("cgr_1")),
			assertRequest: func(t testing.TB, req *managementv3.SearchClientGrantsRequestParameters) {
				assert.Equal(t, "audience", *req.Fields)
				assert.False(t, *req.IncludeFields)
			},
			assertOutput: func(t testing.TB, stdout, stderr string) {
				assert.Contains(t, stdout, "CLIENT ID")
				assert.NotContains(t, stdout, "AUDIENCE")
			},
		},
		{
			name:       "shows JSON when --fields leaves no table column",
			args:       []string{"--fields", "subject_type"},
			searchPage: newClientGrantSearchPages(searchClientGrants("cgr_1")),
			assertOutput: func(t testing.TB, stdout, stderr string) {
				assert.Contains(t, stderr, "No table columns (id, client_id, audience, scope) are left in the response; showing JSON instead.")
				assert.Contains(t, stdout, `"id": "cgr_1"`)
			},
		},
		{
			name:       "collects results across pages up to --number",
			args:       []string{"--filter", "audience:*", "-n", "3", "--json"},
			searchPage: newClientGrantSearchPages(searchClientGrants("cgr_1", "cgr_2"), searchClientGrants("cgr_3", "cgr_4")),
			assertOutput: func(t testing.TB, stdout, stderr string) {
				assert.Contains(t, stdout, "cgr_3")
				assert.NotContains(t, stdout, "cgr_4")
				assert.Contains(t, stderr, "Results may be limited by --number; more may match.")
			},
		},
		{
			name:       "does not suggest raising --number when it is already at the maximum",
			args:       []string{"-n", "1000", "--json"},
			searchPage: newClientGrantSearchPages(searchClientGrants(thousandIDs...)),
			assertOutput: func(t testing.TB, stdout, stderr string) {
				assert.Contains(t, stderr, "more may match. Refine --filter.")
				assert.NotContains(t, stderr, "raise --number")
			},
		},
		{
			name:       "renders an empty result",
			args:       []string{"--filter", `audience:"missing"`},
			searchPage: newClientGrantSearchPages(searchClientGrants()),
			assertOutput: func(t testing.TB, stdout, stderr string) {
				assert.Contains(t, stderr, "client grants (0)")
				assert.Contains(t, stderr, "Try a broader --filter")
			},
		},
		{
			name:       "sends --query JSON params to the search endpoint",
			args:       []string{"-q", `{"q":"audience co \"example\"","parser":"scim"}`},
			serverBody: `{"client_grants":[{"id":"cgr_1"}]}`,
			assertHTTP: func(t testing.TB, r *http.Request) {
				assert.Equal(t, "/api/v2/client-grants/search", r.URL.Path)
				assert.Equal(t, `audience co "example"`, r.URL.Query().Get("q"))
				assert.Equal(t, "scim", r.URL.Query().Get("parser"))
			},
			assertOutput: func(t testing.TB, stdout, stderr string) {
				assert.Contains(t, stdout, "cgr_1")
			},
		},
		{
			name:       "allows --number with --query, which ignores it",
			args:       []string{"-q", `{"q":"audience:*"}`, "-n", "5"},
			serverBody: `{"client_grants":[]}`,
			assertHTTP: func(t testing.TB, r *http.Request) {
				assert.Empty(t, r.URL.Query().Get("take"))
			},
			assertOutput: func(t testing.TB, stdout, stderr string) {
				assert.Contains(t, stderr, `--number is ignored with --query; set "take" (max 100) in the JSON instead.`)
			},
		},
		{
			name:    "rejects --query combined with --filter",
			args:    []string{"-q", `{"q":"audience:*"}`, "--filter", "audience:*"},
			wantErr: "if any flags in the group [query filter] are set none of the others can be; [filter query] were all set",
		},
		{
			name:    "rejects --query combined with --parser",
			args:    []string{"-q", `{"q":"audience:*"}`, "--parser", "scim"},
			wantErr: "if any flags in the group [query parser] are set none of the others can be; [parser query] were all set",
		},
		{
			name:    "rejects --query combined with --sort",
			args:    []string{"-q", `{"q":"audience:*"}`, "--sort", "audience"},
			wantErr: "if any flags in the group [query sort] are set none of the others can be; [query sort] were all set",
		},
		{
			name:    "rejects --query combined with --fields",
			args:    []string{"-q", `{"q":"audience:*"}`, "--fields", "id"},
			wantErr: "if any flags in the group [query fields] are set none of the others can be; [fields query] were all set",
		},
		{
			name:    "rejects --query combined with --exclude-fields",
			args:    []string{"-q", `{"q":"audience:*"}`, "--exclude-fields"},
			wantErr: "if any flags in the group [query exclude-fields] are set none of the others can be; [exclude-fields query] were all set",
		},
		{
			name:    "rejects --csv when --fields leaves no table column",
			args:    []string{"--fields", "subject_type", "--csv"},
			wantErr: "--csv needs at least one table column (id, client_id, audience, scope) left by --fields",
		},
		{
			name:    "rejects an invalid --sort before calling the API",
			args:    []string{"-s", "client_id"},
			wantErr: `invalid --sort "client_id": must be one of audience,updated_at`,
		},
		{
			name:    "rejects an out-of-range --number before calling the API",
			args:    []string{"--filter", "audience:*", "-n", "1001"},
			wantErr: "number flag invalid, please pass a number between 1 and 1000",
		},
		{
			name:    "does not offer the list filter flags",
			args:    []string{"--client-id", "client_1"},
			wantErr: "unknown flag: --client-id",
		},
		{
			name:      "wraps a generic API error",
			args:      []string{"--filter", "audience:*"},
			searchErr: errors.New("boom"),
			wantErr:   "failed to search client grants: boom",
		},
		{
			name:      "explains a search timeout",
			args:      []string{"--filter", "audience:*"},
			searchErr: core.NewAPIError(http.StatusGatewayTimeout, nil, errors.New("timeout")),
			wantErr:   "failed to search client grants: the search timed out, simplify your query and try again: 504: timeout",
		},
		{
			name:      "explains an unavailable Early Access endpoint",
			args:      []string{"--filter", "audience:*"},
			searchErr: core.NewAPIError(http.StatusNotFound, nil, errors.New("not found")),
			wantErr:   "failed to search client grants: the search endpoint is in Early Access and may not be enabled for this tenant: 404: not found",
		},
		{
			name:         "explains a search timeout in --query mode",
			args:         []string{"-q", `{"q":"audience:*"}`},
			serverStatus: http.StatusGatewayTimeout,
			wantErr:      "failed to search client grants: the search timed out, simplify your query and try again: 504: API request failed: Gateway Timeout",
		},
		{
			name:         "explains an unavailable Early Access endpoint in --query mode",
			args:         []string{"-q", `{"q":"audience:*"}`},
			serverStatus: http.StatusNotFound,
			wantErr:      "failed to search client grants: the search endpoint is in Early Access and may not be enabled for this tenant: 404: API request failed: Not Found",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var captured *managementv3.SearchClientGrantsRequestParameters
			grantAPI := mock.NewMockClientGrantAPIV3(ctrl)
			if test.searchPage != nil || test.searchErr != nil {
				grantAPI.EXPECT().
					Search(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req *managementv3.SearchClientGrantsRequestParameters, _ ...option.RequestOption) (*auth0.ClientGrantSearchPage, error) {
						captured = req
						return test.searchPage, test.searchErr
					})
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if test.assertHTTP != nil {
					test.assertHTTP(t, r)
				}
				if test.serverStatus != 0 {
					w.WriteHeader(test.serverStatus)
					_, _ = w.Write([]byte(http.StatusText(test.serverStatus)))
					return
				}
				_, _ = w.Write([]byte(test.serverBody))
			}))
			defer server.Close()

			renderer := testRenderer()
			cli := &cli{
				api:      &auth0.API{HTTPClient: &mockHTTPClientAPI{baseURL: server.URL}},
				apiv3:    &auth0.APIV3{ClientGrant: grantAPI},
				renderer: renderer,
			}

			cmd := searchClientGrantsCmd(cli)
			cmd.SetArgs(test.args)
			err := cmd.Execute()

			if test.wantErr != "" {
				assert.EqualError(t, err, test.wantErr)
				return
			}

			assert.NoError(t, err)
			if test.assertRequest != nil {
				test.assertRequest(t, captured)
			}
			if test.assertOutput != nil {
				test.assertOutput(t, fmt.Sprint(renderer.ResultWriter), fmt.Sprint(renderer.MessageWriter))
			}
		})
	}
}
