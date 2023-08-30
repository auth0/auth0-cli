package cli

import (
	"context"
	"fmt"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
)

func TestSanitizeResourceName(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		// Test cases with valid names
		{"ValidName123", "ValidName123"},
		{"_Another_Valid-Name", "_Another_Valid-Name"},
		{"name_with_123", "name_with_123"},
		{"_start_with_underscore", "_start_with_underscore"},

		// Test cases with invalid names to be sanitized
		{"Invalid@Name", "InvalidName"},
		{"Invalid Name", "InvalidName"},
		{"123StartWithNumber", "StartWithNumber"},
		{"-StartWithDash", "StartWithDash"},
		{"", ""},
	}

	for _, testCase := range testCases {
		t.Run(testCase.input, func(t *testing.T) {
			sanitized := sanitizeResourceName(testCase.input)
			assert.Equal(t, testCase.expected, sanitized)
		})
	}
}

func TestActionResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully retrieves actions data", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		actionAPI := mock.NewMockActionAPI(ctrl)
		actionAPI.EXPECT().
			List(gomock.Any(), gomock.Any()).
			Return(
				&management.ActionList{
					List: management.List{
						Start: 0,
						Limit: 2,
						Total: 4,
					},
					Actions: []*management.Action{
						{
							ID:   auth0.String("07898b80-02ba-42ee-82ad-e5b224a9b450"),
							Name: auth0.String("Action 1"),
						},
						{
							ID:   auth0.String("24118aae-8022-4b94-80c1-e8e28511eb92"),
							Name: auth0.String("Action 2"),
						},
					},
				},
				nil,
			)
		actionAPI.EXPECT().
			List(gomock.Any(), gomock.Any()).
			Return(
				&management.ActionList{
					List: management.List{
						Start: 2,
						Limit: 4,
						Total: 4,
					},
					Actions: []*management.Action{
						{
							ID:   auth0.String("fa04d1ff-fe8d-4662-b7c2-32d212719876"),
							Name: auth0.String("Action 3"),
						},
						{
							ID:   auth0.String("9cb897b9-c25c-47be-b5aa-e03e31af2e44"),
							Name: auth0.String("Action 4"),
						},
					},
				},
				nil,
			)

		fetcher := actionResourceFetcher{
			api: &auth0.API{
				Action: actionAPI,
			},
		}

		expectedData := importDataList{
			{
				ResourceName: "auth0_action.Action1",
				ImportID:     "07898b80-02ba-42ee-82ad-e5b224a9b450",
			},
			{
				ResourceName: "auth0_action.Action2",
				ImportID:     "24118aae-8022-4b94-80c1-e8e28511eb92",
			},
			{
				ResourceName: "auth0_action.Action3",
				ImportID:     "fa04d1ff-fe8d-4662-b7c2-32d212719876",
			},
			{
				ResourceName: "auth0_action.Action4",
				ImportID:     "9cb897b9-c25c-47be-b5aa-e03e31af2e44",
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, expectedData, data)
	})

	t.Run("it returns an error if api call fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		actionAPI := mock.NewMockActionAPI(ctrl)
		actionAPI.EXPECT().
			List(gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("failed to list actions"))

		fetcher := actionResourceFetcher{
			api: &auth0.API{
				Action: actionAPI,
			},
		}

		_, err := fetcher.FetchData(context.Background())
		assert.EqualError(t, err, "failed to list actions")
	})
}

func TestBrandingResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully generates branding import data", func(t *testing.T) {
		fetcher := brandingResourceFetcher{}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Len(t, data, 1)
		assert.Equal(t, data[0].ResourceName, "auth0_branding.branding")
		assert.Greater(t, len(data[0].ImportID), 0)
	})
}

func TestClientResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully retrieves client data", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		clientAPI := mock.NewMockClientAPI(ctrl)
		clientAPI.EXPECT().
			List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(
				&management.ClientList{
					List: management.List{
						Start: 0,
						Limit: 2,
						Total: 4,
					},
					Clients: []*management.Client{
						{
							ClientID: auth0.String("clientID_1"),
							Name:     auth0.String("My Test Client 1"),
						},
						{
							ClientID: auth0.String("clientID_2"),
							Name:     auth0.String("My Test Client 2"),
						},
					},
				},
				nil,
			)
		clientAPI.EXPECT().
			List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(
				&management.ClientList{
					List: management.List{
						Start: 2,
						Limit: 4,
						Total: 4,
					},
					Clients: []*management.Client{
						{
							ClientID: auth0.String("clientID_3"),
							Name:     auth0.String("My Test Client 3"),
						},
						{
							ClientID: auth0.String("clientID_4"),
							Name:     auth0.String("My Test Client 4"),
						},
					},
				},
				nil,
			)

		fetcher := clientResourceFetcher{
			api: &auth0.API{
				Client: clientAPI,
			},
		}

		expectedData := importDataList{
			{
				ResourceName: "auth0_client.MyTestClient1",
				ImportID:     "clientID_1",
			},
			{
				ResourceName: "auth0_client.MyTestClient2",
				ImportID:     "clientID_2",
			},
			{
				ResourceName: "auth0_client.MyTestClient3",
				ImportID:     "clientID_3",
			},
			{
				ResourceName: "auth0_client.MyTestClient4",
				ImportID:     "clientID_4",
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, expectedData, data)
	})

	t.Run("it returns an error if api call fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		clientAPI := mock.NewMockClientAPI(ctrl)
		clientAPI.EXPECT().
			List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("failed to list clients"))

		fetcher := clientResourceFetcher{
			api: &auth0.API{
				Client: clientAPI,
			},
		}

		_, err := fetcher.FetchData(context.Background())
		assert.EqualError(t, err, "failed to list clients")
	})
}

func TestClientGrantResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully retrieves client grant data", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		clientGrantAPI := mock.NewMockClientGrantAPI(ctrl)
		clientGrantAPI.EXPECT().
			List(gomock.Any(), gomock.Any()).
			Return(
				&management.ClientGrantList{
					List: management.List{
						Start: 0,
						Limit: 2,
						Total: 4,
					},
					ClientGrants: []*management.ClientGrant{
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
					},
				},
				nil,
			)
		clientGrantAPI.EXPECT().
			List(gomock.Any(), gomock.Any()).
			Return(
				&management.ClientGrantList{
					List: management.List{
						Start: 2,
						Limit: 4,
						Total: 4,
					},
					ClientGrants: []*management.ClientGrant{
						{
							ID:       auth0.String("cgr_3"),
							ClientID: auth0.String("client-id-1"),
							Audience: auth0.String("https://travel0.us.auth0.com/api/v2/"),
						},
						{
							ID:       auth0.String("cgr_4"),
							ClientID: auth0.String("client-id-2"),
							Audience: auth0.String("https://travel0.us.auth0.com/api/v2/"),
						},
					},
				},
				nil,
			)

		fetcher := clientGrantResourceFetcher{
			api: &auth0.API{
				ClientGrant: clientGrantAPI,
			},
		}

		expectedData := importDataList{
			{
				ResourceName: "auth0_client_grant.client-id-1_httpstravel0comapi",
				ImportID:     "cgr_1",
			},
			{
				ResourceName: "auth0_client_grant.client-id-2_httpstravel0comapi",
				ImportID:     "cgr_2",
			},
			{
				ResourceName: "auth0_client_grant.client-id-1_httpstravel0usauth0comapiv2",
				ImportID:     "cgr_3",
			},
			{
				ResourceName: "auth0_client_grant.client-id-2_httpstravel0usauth0comapiv2",
				ImportID:     "cgr_4",
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, expectedData, data)
	})

	t.Run("it returns an error if api call fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		clientGrantAPI := mock.NewMockClientGrantAPI(ctrl)
		clientGrantAPI.EXPECT().
			List(gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("failed to list clients"))

		fetcher := clientGrantResourceFetcher{
			api: &auth0.API{
				ClientGrant: clientGrantAPI,
			},
		}

		_, err := fetcher.FetchData(context.Background())
		assert.EqualError(t, err, "failed to list clients")
	})
}

func TestConnectionResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully retrieves connections data", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		connAPI := mock.NewMockConnectionAPI(ctrl)
		connAPI.EXPECT().
			List(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(
				&management.ConnectionList{
					List: management.List{
						Start: 0,
						Limit: 2,
						Total: 4,
					},
					Connections: []*management.Connection{
						{
							ID:   auth0.String("con_id1"),
							Name: auth0.String("Connection 1"),
						},
						{
							ID:   auth0.String("con_id2"),
							Name: auth0.String("Connection 2"),
						},
					},
				},
				nil,
			)
		connAPI.EXPECT().
			List(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(
				&management.ConnectionList{
					List: management.List{
						Start: 2,
						Limit: 4,
						Total: 4,
					},
					Connections: []*management.Connection{
						{
							ID:   auth0.String("con_id3"),
							Name: auth0.String("Connection 3"),
						},
						{
							ID:   auth0.String("con_id4"),
							Name: auth0.String("Connection 4"),
						},
					},
				},
				nil,
			)

		fetcher := connectionResourceFetcher{
			api: &auth0.API{
				Connection: connAPI,
			},
		}

		expectedData := importDataList{
			{
				ResourceName: "auth0_connection.Connection1",
				ImportID:     "con_id1",
			},
			{
				ResourceName: "auth0_connection.Connection2",
				ImportID:     "con_id2",
			},
			{
				ResourceName: "auth0_connection.Connection3",
				ImportID:     "con_id3",
			},
			{
				ResourceName: "auth0_connection.Connection4",
				ImportID:     "con_id4",
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, expectedData, data)
	})

	t.Run("it returns an error if api call fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		connAPI := mock.NewMockConnectionAPI(ctrl)
		connAPI.EXPECT().
			List(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("failed to list connections"))

		fetcher := connectionResourceFetcher{
			api: &auth0.API{
				Connection: connAPI,
			},
		}

		_, err := fetcher.FetchData(context.Background())
		assert.EqualError(t, err, "failed to list connections")
	})
}

func TestCustomDomainResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully retrieves custom domains data", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		customDomainAPI := mock.NewMockCustomDomainAPI(ctrl)
		customDomainAPI.EXPECT().
			List(gomock.Any()).
			Return(
				[]*management.CustomDomain{
					{
						ID:     auth0.String("cd_XDVfBNsfL2vj7Wm1"),
						Domain: auth0.String("travel0.com"),
					},
					{
						ID:     auth0.String("cd_XDVfBNsfL2vj7Wm1"),
						Domain: auth0.String("enterprise.travel0.com"),
					},
				},
				nil,
			)

		fetcher := customDomainResourceFetcher{
			api: &auth0.API{
				CustomDomain: customDomainAPI,
			},
		}

		expectedData := importDataList{
			{
				ResourceName: "auth0_custom_domain.travel0com",
				ImportID:     "cd_XDVfBNsfL2vj7Wm1",
			},
			{
				ResourceName: "auth0_custom_domain.enterprisetravel0com",
				ImportID:     "cd_XDVfBNsfL2vj7Wm1",
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, expectedData, data)
	})

	t.Run("it returns an error if api call fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		customDomainAPI := mock.NewMockCustomDomainAPI(ctrl)
		customDomainAPI.EXPECT().
			List(gomock.Any()).
			Return(nil, fmt.Errorf("failed to list custom domains"))

		fetcher := customDomainResourceFetcher{
			api: &auth0.API{
				CustomDomain: customDomainAPI,
			},
		}

		_, err := fetcher.FetchData(context.Background())
		assert.EqualError(t, err, "failed to list custom domains")
	})
}

func TestOrganizationResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully retrieves organizations data", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		orgAPI := mock.NewMockOrganizationAPI(ctrl)
		orgAPI.EXPECT().
			List(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(
				&management.OrganizationList{
					List: management.List{
						Start: 0,
						Limit: 2,
						Total: 4,
					},
					Organizations: []*management.Organization{
						{
							ID:   auth0.String("org_1"),
							Name: auth0.String("Organization 1"),
						},
						{
							ID:   auth0.String("org_2"),
							Name: auth0.String("Organization 2"),
						},
					},
				},
				nil,
			)
		orgAPI.EXPECT().
			List(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(
				&management.OrganizationList{
					List: management.List{
						Start: 2,
						Limit: 4,
						Total: 4,
					},
					Organizations: []*management.Organization{
						{
							ID:   auth0.String("org_3"),
							Name: auth0.String("Organization 3"),
						},
						{
							ID:   auth0.String("org_4"),
							Name: auth0.String("Organization 4"),
						},
					},
				},
				nil,
			)

		fetcher := organizationResourceFetcher{
			api: &auth0.API{
				Organization: orgAPI,
			},
		}

		expectedData := importDataList{
			{
				ResourceName: "auth0_organization.Organization1",
				ImportID:     "org_1",
			},
			{
				ResourceName: "auth0_organization.Organization2",
				ImportID:     "org_2",
			},
			{
				ResourceName: "auth0_organization.Organization3",
				ImportID:     "org_3",
			},
			{
				ResourceName: "auth0_organization.Organization4",
				ImportID:     "org_4",
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, expectedData, data)
	})

	t.Run("it returns an error if api call fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		orgAPI := mock.NewMockOrganizationAPI(ctrl)
		orgAPI.EXPECT().
			List(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("failed to list organizations"))

		fetcher := organizationResourceFetcher{
			api: &auth0.API{
				Organization: orgAPI,
			},
		}

		_, err := fetcher.FetchData(context.Background())
		assert.EqualError(t, err, "failed to list organizations")
	})
}

func TestResourceServerResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully retrieves resource server data", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		resourceServerAPI := mock.NewMockResourceServerAPI(ctrl)
		resourceServerAPI.EXPECT().
			List(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(
				&management.ResourceServerList{
					List: management.List{
						Start: 0,
						Limit: 2,
						Total: 4,
					},
					ResourceServers: []*management.ResourceServer{
						{
							ID:   auth0.String("610e04b71f71b9003a7eb3df"),
							Name: auth0.String("Auth0 Management API"),
						},
						{
							ID:   auth0.String("6358fed7b77d3c391dd78a40"),
							Name: auth0.String("Payments API"),
						},
					},
				},
				nil,
			)
		resourceServerAPI.EXPECT().
			List(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(
				&management.ResourceServerList{
					List: management.List{
						Start: 2,
						Limit: 4,
						Total: 4,
					},
					ResourceServers: []*management.ResourceServer{
						{
							ID:   auth0.String("66ef6f9c435cab03def5fa16"),
							Name: auth0.String("Blog API"),
						},
						{
							ID:   auth0.String("63bf6f9b0e025715cb91ce7c"),
							Name: auth0.String("User API"),
						},
					},
				},
				nil,
			)

		fetcher := resourceServerResourceFetcher{
			api: &auth0.API{
				ResourceServer: resourceServerAPI,
			},
		}

		expectedData := importDataList{
			{
				ResourceName: "auth0_resource_server.Auth0ManagementAPI",
				ImportID:     "610e04b71f71b9003a7eb3df",
			},
			{
				ResourceName: "auth0_resource_server_scopes.Auth0ManagementAPI",
				ImportID:     "610e04b71f71b9003a7eb3df",
			},
			{
				ResourceName: "auth0_resource_server.PaymentsAPI",
				ImportID:     "6358fed7b77d3c391dd78a40",
			},
			{
				ResourceName: "auth0_resource_server_scopes.PaymentsAPI",
				ImportID:     "6358fed7b77d3c391dd78a40",
			},
			{
				ResourceName: "auth0_resource_server.BlogAPI",
				ImportID:     "66ef6f9c435cab03def5fa16",
			},
			{
				ResourceName: "auth0_resource_server_scopes.BlogAPI",
				ImportID:     "66ef6f9c435cab03def5fa16",
			},
			{
				ResourceName: "auth0_resource_server.UserAPI",
				ImportID:     "63bf6f9b0e025715cb91ce7c",
			},
			{
				ResourceName: "auth0_resource_server_scopes.UserAPI",
				ImportID:     "63bf6f9b0e025715cb91ce7c",
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, expectedData, data)
	})

	t.Run("it returns an error if api call fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		resourceServerAPI := mock.NewMockResourceServerAPI(ctrl)
		resourceServerAPI.EXPECT().
			List(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("failed to list resource servers"))

		fetcher := resourceServerResourceFetcher{
			api: &auth0.API{
				ResourceServer: resourceServerAPI,
			},
		}

		_, err := fetcher.FetchData(context.Background())
		assert.EqualError(t, err, "failed to list resource servers")
	})
}

func TestRoleResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully retrieves roles data", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		roleAPI := mock.NewMockRoleAPI(ctrl)
		roleAPI.EXPECT().
			List(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(
				&management.RoleList{
					List: management.List{
						Start: 0,
						Limit: 2,
						Total: 4,
					},
					Roles: []*management.Role{
						{
							ID:   auth0.String("rol_1"),
							Name: auth0.String("Role 1"),
						},
						{
							ID:   auth0.String("rol_2"),
							Name: auth0.String("Role 2"),
						},
					},
				},
				nil,
			)
		roleAPI.EXPECT().
			List(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(
				&management.RoleList{
					List: management.List{
						Start: 2,
						Limit: 4,
						Total: 4,
					},
					Roles: []*management.Role{
						{
							ID:   auth0.String("rol_3"),
							Name: auth0.String("Role 3"),
						},
						{
							ID:   auth0.String("rol_4"),
							Name: auth0.String("Role 4"),
						},
					},
				},
				nil,
			)

		fetcher := roleResourceFetcher{
			api: &auth0.API{
				Role: roleAPI,
			},
		}

		expectedData := importDataList{
			{
				ResourceName: "auth0_role.Role1",
				ImportID:     "rol_1",
			},
			{
				ResourceName: "auth0_role.Role2",
				ImportID:     "rol_2",
			},
			{
				ResourceName: "auth0_role.Role3",
				ImportID:     "rol_3",
			},
			{
				ResourceName: "auth0_role.Role4",
				ImportID:     "rol_4",
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, expectedData, data)
	})

	t.Run("it returns an error if api call fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		roleAPI := mock.NewMockRoleAPI(ctrl)
		roleAPI.EXPECT().
			List(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("failed to list roles"))

		fetcher := roleResourceFetcher{
			api: &auth0.API{
				Role: roleAPI,
			},
		}

		_, err := fetcher.FetchData(context.Background())
		assert.EqualError(t, err, "failed to list roles")
	})
}

func TestTenantResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully generates tenant import data", func(t *testing.T) {
		fetcher := tenantResourceFetcher{}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Len(t, data, 1)
		assert.Equal(t, data[0].ResourceName, "auth0_tenant.tenant")
		assert.Greater(t, len(data[0].ImportID), 0)
	})
}
