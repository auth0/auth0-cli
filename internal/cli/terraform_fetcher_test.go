package cli

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
)

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
				ResourceName: "auth0_action.action_1",
				ImportID:     "07898b80-02ba-42ee-82ad-e5b224a9b450",
			},
			{
				ResourceName: "auth0_action.action_2",
				ImportID:     "24118aae-8022-4b94-80c1-e8e28511eb92",
			},
			{
				ResourceName: "auth0_action.action_3",
				ImportID:     "fa04d1ff-fe8d-4662-b7c2-32d212719876",
			},
			{
				ResourceName: "auth0_action.action_4",
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

func TestAttackProtectionResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully generates attack protection import data", func(t *testing.T) {
		fetcher := attackProtectionResourceFetcher{}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Len(t, data, 1)
		assert.Equal(t, data[0].ResourceName, "auth0_attack_protection.attack_protection")
		assert.Greater(t, len(data[0].ImportID), 0)
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

func TestBrandingThemeResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully retrieves branding theme data", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		brandingThemeAPI := mock.NewMockBrandingThemeAPI(ctrl)
		brandingThemeAPI.EXPECT().
			Default(gomock.Any()).
			Return(&management.BrandingTheme{
				ID: auth0.String("theme_123"),
			}, nil)

		fetcher := brandingThemeResourceFetcher{
			api: &auth0.API{
				BrandingTheme: brandingThemeAPI,
			},
		}

		expectedData := importDataList{
			{
				ResourceName: "auth0_branding_theme.default",
				ImportID:     "theme_123",
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, expectedData, data)
	})

	t.Run("it returns an error if api call fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		brandingThemeAPI := mock.NewMockBrandingThemeAPI(ctrl)
		brandingThemeAPI.EXPECT().
			Default(gomock.Any()).
			Return(nil, fmt.Errorf("failed to get default theme"))

		fetcher := brandingThemeResourceFetcher{
			api: &auth0.API{
				BrandingTheme: brandingThemeAPI,
			},
		}

		_, err := fetcher.FetchData(context.Background())
		assert.EqualError(t, err, "failed to get default theme")
	})

	t.Run("it returns nil data if branding theme is not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mErr := mockManagamentError{status: http.StatusNotFound}
		brandingThemeAPI := mock.NewMockBrandingThemeAPI(ctrl)
		brandingThemeAPI.EXPECT().
			Default(gomock.Any()).
			Return(nil, mErr)

		fetcher := brandingThemeResourceFetcher{
			api: &auth0.API{
				BrandingTheme: brandingThemeAPI,
			},
		}

		_, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
	})
}

func Test_phoneProviderResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully retrieves twilio's phone providers data", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		brandingAPI := mock.NewMockBrandingAPI(ctrl)
		brandingAPI.EXPECT().
			ListPhoneProviders(gomock.Any(), gomock.Any()).
			Return(
				&management.BrandingPhoneProviderList{
					Providers: []*management.BrandingPhoneProvider{
						{
							ID:   auth0.String("pro_epg5EAGoQydDkAdgWMiMLz"),
							Name: auth0.String("twilio"),
						},
					},
				},
				nil,
			)

		fetcher := phoneProviderResourceFetcher{
			api: &auth0.API{
				Branding: brandingAPI,
			},
		}

		expectedData := importDataList{
			{
				ResourceName: "auth0_phone_provider.twilio",
				ImportID:     "pro_epg5EAGoQydDkAdgWMiMLz",
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, expectedData, data)
	})

	t.Run("it successfully retrieves custom's phone providers data", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		brandingAPI := mock.NewMockBrandingAPI(ctrl)
		brandingAPI.EXPECT().
			ListPhoneProviders(gomock.Any(), gomock.Any()).
			Return(
				&management.BrandingPhoneProviderList{
					Providers: []*management.BrandingPhoneProvider{
						{
							ID:   auth0.String("pro_epg5EAGoQydDkAdgWMiMLa"),
							Name: auth0.String("custom"),
						},
					},
				},
				nil,
			)

		fetcher := phoneProviderResourceFetcher{
			api: &auth0.API{
				Branding: brandingAPI,
			},
		}

		expectedData := importDataList{
			{
				ResourceName: "auth0_phone_provider.custom",
				ImportID:     "pro_epg5EAGoQydDkAdgWMiMLa",
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, expectedData, data)
	})

	t.Run("it returns an error if api call fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		brandingAPI := mock.NewMockBrandingAPI(ctrl)
		brandingAPI.EXPECT().
			ListPhoneProviders(gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("failed to list phone providers"))

		fetcher := phoneProviderResourceFetcher{
			api: &auth0.API{
				Branding: brandingAPI,
			},
		}

		_, err := fetcher.FetchData(context.Background())
		assert.EqualError(t, err, "failed to list phone providers")
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
				ResourceName: "auth0_client.my_test_client_1",
				ImportID:     "clientID_1",
			},
			{
				ResourceName: "auth0_client_credentials.my_test_client_1",
				ImportID:     "clientID_1",
			},
			{
				ResourceName: "auth0_client.my_test_client_2",
				ImportID:     "clientID_2",
			},
			{
				ResourceName: "auth0_client_credentials.my_test_client_2",
				ImportID:     "clientID_2",
			},
			{
				ResourceName: "auth0_client.my_test_client_3",
				ImportID:     "clientID_3",
			},
			{
				ResourceName: "auth0_client_credentials.my_test_client_3",
				ImportID:     "clientID_3",
			},
			{
				ResourceName: "auth0_client.my_test_client_4",
				ImportID:     "clientID_4",
			},
			{
				ResourceName: "auth0_client_credentials.my_test_client_4",
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
				ResourceName: "auth0_client_grant.client_id_1_https_travel0_com_api",
				ImportID:     "cgr_1",
			},
			{
				ResourceName: "auth0_client_grant.client_id_2_https_travel0_com_api",
				ImportID:     "cgr_2",
			},
			{
				ResourceName: "auth0_client_grant.client_id_1_https_travel0_us_auth0_com_api_v2",
				ImportID:     "cgr_3",
			},
			{
				ResourceName: "auth0_client_grant.client_id_2_https_travel0_us_auth0_com_api_v2",
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
				ResourceName: "auth0_connection.connection_1",
				ImportID:     "con_id1",
			},
			{
				ResourceName: "auth0_connection_clients.connection_1",
				ImportID:     "con_id1",
			},
			{
				ResourceName: "auth0_connection.connection_2",
				ImportID:     "con_id2",
			},
			{
				ResourceName: "auth0_connection_clients.connection_2",
				ImportID:     "con_id2",
			},
			{
				ResourceName: "auth0_connection.connection_3",
				ImportID:     "con_id3",
			},
			{
				ResourceName: "auth0_connection_clients.connection_3",
				ImportID:     "con_id3",
			},
			{
				ResourceName: "auth0_connection.connection_4",
				ImportID:     "con_id4",
			},
			{
				ResourceName: "auth0_connection_clients.connection_4",
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
				ResourceName: "auth0_custom_domain.travel0_com",
				ImportID:     "cd_XDVfBNsfL2vj7Wm1",
			},
			{
				ResourceName: "auth0_custom_domain.enterprise_travel0_com",
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

	t.Run("it returns empty set error if unsupported feature error occurs", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		customDomainAPI := mock.NewMockCustomDomainAPI(ctrl)
		customDomainAPI.EXPECT().
			List(gomock.Any()).
			Return(nil, fmt.Errorf("403 Forbidden: The account is not allowed to perform this operation, please contact our support team"))

		fetcher := customDomainResourceFetcher{
			api: &auth0.API{
				CustomDomain: customDomainAPI,
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Len(t, data, 0)
	})

	t.Run("it returns empty set error if no verified CC error occurs", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		customDomainAPI := mock.NewMockCustomDomainAPI(ctrl)
		customDomainAPI.EXPECT().
			List(gomock.Any()).
			Return(nil, fmt.Errorf("403 Forbidden: There must be a verified credit card on file to perform this operation"))

		fetcher := customDomainResourceFetcher{
			api: &auth0.API{
				CustomDomain: customDomainAPI,
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Len(t, data, 0)
	})
}

func TestNetworkACLResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully retrieves network ACL data", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		networkACLAPI := mock.NewMockNetworkACLAPI(ctrl)
		networkACLAPI.EXPECT().
			List(gomock.Any()).
			Return(
				[]*management.NetworkACL{
					{
						ID: auth0.String("acl_1"),
					},
					{
						ID: auth0.String("acl_2"),
					},
				},
				nil,
			)

		fetcher := networkACLResourceFetcher{
			api: &auth0.API{
				NetworkACL: networkACLAPI,
			},
		}

		expectedData := importDataList{
			{
				ResourceName: "auth0_network_acl.acl_1",
				ImportID:     "acl_1",
			},
			{
				ResourceName: "auth0_network_acl.acl_2",
				ImportID:     "acl_2",
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, expectedData, data)
	})

	t.Run("it handles error, even if tenant does not have Tenant ACL enabled", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		networkACLAPI := mock.NewMockNetworkACLAPI(ctrl)
		networkACLAPI.EXPECT().
			List(gomock.Any()).
			Return(nil, fmt.Errorf("403 Forbidden: Please upgrade your subscription to enable Tenant ACL Management"))

		fetcher := networkACLResourceFetcher{
			api: &auth0.API{
				NetworkACL: networkACLAPI,
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.EqualError(t, err, "403 Forbidden: Please upgrade your subscription to enable Tenant ACL Management")
		assert.Len(t, data, 0)
	})

	t.Run("it returns an error if api call fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		networkACLAPI := mock.NewMockNetworkACLAPI(ctrl)
		networkACLAPI.EXPECT().
			List(gomock.Any()).
			Return(nil, fmt.Errorf("failed to list network ACLs"))

		fetcher := networkACLResourceFetcher{
			api: &auth0.API{
				NetworkACL: networkACLAPI,
			},
		}

		_, err := fetcher.FetchData(context.Background())
		assert.EqualError(t, err, "failed to list network ACLs")
	})
}

func TestFormResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully generates form import data", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		formAPI := mock.NewMockFormAPI(ctrl)
		formAPI.EXPECT().
			List(gomock.Any(), gomock.Any()).Return(
			&management.FormList{
				List: management.List{
					Start: 0,
					Limit: 1,
					Total: 2,
				},
				Forms: []*management.Form{
					{
						ID:   auth0.String("form_id1"),
						Name: auth0.String("Form 1"),
					},
					{
						ID:   auth0.String("form_id2"),
						Name: auth0.String("Form 2"),
					},
				},
			}, nil)

		fetcher := formResourceFetcher{
			api: &auth0.API{
				Form: formAPI,
			},
		}

		expectedData := importDataList{
			{
				ResourceName: "auth0_form.form_1",
				ImportID:     "form_id1",
			},
			{
				ResourceName: "auth0_form.form_2",
				ImportID:     "form_id2",
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, expectedData, data)
	})

	t.Run("it successfully returns empty import list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		formAPI := mock.NewMockFormAPI(ctrl)
		formAPI.EXPECT().
			List(gomock.Any(), gomock.Any()).Return(
			&management.FormList{
				List: management.List{
					Start: 0,
					Limit: 0,
					Total: 0,
				},
				Forms: []*management.Form{},
			}, nil)

		fetcher := formResourceFetcher{
			api: &auth0.API{
				Form: formAPI,
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Nil(t, data)
	})

	t.Run("it returns an error if api call fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		formAPI := mock.NewMockFormAPI(ctrl)
		formAPI.EXPECT().
			List(gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("failed to read form"))

		fetcher := formResourceFetcher{
			api: &auth0.API{
				Form: formAPI,
			},
		}

		_, err := fetcher.FetchData(context.Background())
		assert.EqualError(t, err, "failed to read form")
	})
}

func TestFlowResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully generates form import data", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		flowAPI := mock.NewMockFlowAPI(ctrl)
		flowAPI.EXPECT().
			List(gomock.Any(), gomock.Any()).Return(
			&management.FlowList{
				List: management.List{
					Start: 0,
					Limit: 1,
					Total: 2,
				},
				Flows: []*management.Flow{
					{
						ID:   auth0.String("flow_id1"),
						Name: auth0.String("Flow 1"),
					},
					{
						ID:   auth0.String("flow_id2"),
						Name: auth0.String("Flow 2"),
					},
				},
			}, nil)

		fetcher := flowResourceFetcher{
			api: &auth0.API{
				Flow: flowAPI,
			},
		}

		expectedData := importDataList{
			{
				ResourceName: "auth0_flow.flow_1",
				ImportID:     "flow_id1",
			},
			{
				ResourceName: "auth0_flow.flow_2",
				ImportID:     "flow_id2",
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, expectedData, data)
	})

	t.Run("it successfully returns empty import list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		flowAPI := mock.NewMockFlowAPI(ctrl)
		flowAPI.EXPECT().
			List(gomock.Any(), gomock.Any()).Return(
			&management.FlowList{
				List: management.List{
					Start: 0,
					Limit: 0,
					Total: 0,
				},
				Flows: []*management.Flow{},
			}, nil)

		fetcher := flowResourceFetcher{
			api: &auth0.API{
				Flow: flowAPI,
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Nil(t, data)
	})

	t.Run("it returns an error if api call fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		flowAPI := mock.NewMockFlowAPI(ctrl)
		flowAPI.EXPECT().
			List(gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("failed to read flow"))

		fetcher := flowResourceFetcher{
			api: &auth0.API{
				Flow: flowAPI,
			},
		}

		_, err := fetcher.FetchData(context.Background())
		assert.EqualError(t, err, "failed to read flow")
	})
}

func TestFlowVaultConnectionResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully generates form import data", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		flowVaultAPI := mock.NewMockFlowVaultConnectionAPI(ctrl)
		flowVaultAPI.EXPECT().
			GetConnectionList(gomock.Any()).Return(
			&management.FlowVaultConnectionList{
				List: management.List{
					Start: 0,
					Limit: 1,
					Total: 2,
				},
				Connections: []*management.FlowVaultConnection{
					{
						ID:   auth0.String("flow_vault_connection1"),
						Name: auth0.String("Flow Vault Connection 1"),
					},
					{
						ID:   auth0.String("flow_vault_connection2"),
						Name: auth0.String("Flow Vault Connection 2"),
					},
				},
			}, nil)

		fetcher := flowVaultConnectionResourceFetcher{
			api: &auth0.API{
				FlowVaultConnection: flowVaultAPI,
			},
		}

		expectedData := importDataList{
			{
				ResourceName: "auth0_flow_vault_connection.flow_vault_connection_1",
				ImportID:     "flow_vault_connection1",
			},
			{
				ResourceName: "auth0_flow_vault_connection.flow_vault_connection_2",
				ImportID:     "flow_vault_connection2",
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, expectedData, data)
	})

	t.Run("it successfully returns empty import list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		flowVaultAPI := mock.NewMockFlowVaultConnectionAPI(ctrl)
		flowVaultAPI.EXPECT().
			GetConnectionList(gomock.Any()).Return(
			&management.FlowVaultConnectionList{
				List: management.List{
					Start: 0,
					Limit: 0,
					Total: 0,
				},
				Connections: []*management.FlowVaultConnection{},
			}, nil)

		fetcher := flowVaultConnectionResourceFetcher{
			api: &auth0.API{
				FlowVaultConnection: flowVaultAPI,
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Nil(t, data)
	})

	t.Run("it returns an error if api call fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		flowVaultConnectionAPI := mock.NewMockFlowVaultConnectionAPI(ctrl)
		flowVaultConnectionAPI.EXPECT().
			GetConnectionList(gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("failed to read flow connection"))

		fetcher := flowVaultConnectionResourceFetcher{
			api: &auth0.API{
				FlowVaultConnection: flowVaultConnectionAPI,
			},
		}

		_, err := fetcher.FetchData(context.Background())
		assert.EqualError(t, err, "failed to read flow connection")
	})
}

func TestGuardianResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully generates pages guardian data", func(t *testing.T) {
		fetcher := guardianResourceFetcher{}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Len(t, data, 1)
		assert.Equal(t, data[0].ResourceName, "auth0_guardian.guardian")
		assert.Greater(t, len(data[0].ImportID), 0)
	})
}

func TestEmailProviderResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully generates email provider import data", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		emailProviderAPI := mock.NewMockEmailProviderAPI(ctrl)

		emailProviderAPI.EXPECT().
			Read(gomock.Any(), gomock.Any()).
			Return(nil, nil)

		fetcher := emailProviderResourceFetcher{
			api: &auth0.API{
				EmailProvider: emailProviderAPI,
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Len(t, data, 1)
		assert.Equal(t, data[0].ResourceName, "auth0_email_provider.email_provider")
		assert.Greater(t, len(data[0].ImportID), 0)
	})

	t.Run("it does not generate email provider import data if email provider does not exist", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mErr := mockManagamentError{status: http.StatusNotFound}

		emailProviderAPI := mock.NewMockEmailProviderAPI(ctrl)
		emailProviderAPI.EXPECT().
			Read(gomock.Any(), gomock.Any()).
			Return(nil, mErr)

		fetcher := emailProviderResourceFetcher{
			api: &auth0.API{
				EmailProvider: emailProviderAPI,
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Len(t, data, 0)
	})

	t.Run("it returns an error if api call fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		emailProviderAPI := mock.NewMockEmailProviderAPI(ctrl)
		emailProviderAPI.EXPECT().
			Read(gomock.Any()).
			Return(nil, fmt.Errorf("failed to read email provider"))

		fetcher := emailProviderResourceFetcher{
			api: &auth0.API{
				EmailProvider: emailProviderAPI,
			},
		}

		_, err := fetcher.FetchData(context.Background())
		assert.EqualError(t, err, "failed to read email provider")
	})
}
func TestEmailTemplateResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully retrieves email templates data", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		emailTemplateAPI := mock.NewMockEmailTemplateAPI(ctrl)
		templates := []string{`verify_email`, `reset_email`, `welcome_email`, `blocked_account`, `stolen_credentials`, `enrollment_email`,
			`mfa_oob_code`, `change_password`, `password_reset`, `verify_email_by_code`, `reset_email_by_code`, `user_invitation`, `async_approval`}

		for _, tmpl := range templates {
			emailTemplateAPI.EXPECT().
				Read(gomock.Any(), tmpl).
				Return(&management.EmailTemplate{Template: auth0.String(tmpl)}, nil)
		}

		fetcher := emailTemplateResourceFetcher{
			api: &auth0.API{
				EmailTemplate: emailTemplateAPI,
			},
		}

		expectedData := importDataList{
			{
				ResourceName: "auth0_email_template.verify_email",
				ImportID:     "verify_email",
			},
			{
				ResourceName: "auth0_email_template.reset_email",
				ImportID:     "reset_email",
			},
			{
				ResourceName: "auth0_email_template.welcome_email",
				ImportID:     "welcome_email",
			},
			{
				ResourceName: "auth0_email_template.blocked_account",
				ImportID:     "blocked_account",
			},
			{
				ResourceName: "auth0_email_template.stolen_credentials",
				ImportID:     "stolen_credentials",
			},
			{
				ResourceName: "auth0_email_template.enrollment_email",
				ImportID:     "enrollment_email",
			},
			{
				ResourceName: "auth0_email_template.mfa_oob_code",
				ImportID:     "mfa_oob_code",
			},
			{
				ResourceName: "auth0_email_template.change_password",
				ImportID:     "change_password",
			},
			{
				ResourceName: "auth0_email_template.password_reset",
				ImportID:     "password_reset",
			},
			{
				ResourceName: "auth0_email_template.verify_email_by_code",
				ImportID:     "verify_email_by_code",
			},
			{
				ResourceName: "auth0_email_template.reset_email_by_code",
				ImportID:     "reset_email_by_code",
			},
			{
				ResourceName: "auth0_email_template.user_invitation",
				ImportID:     "user_invitation",
			},
			{
				ResourceName: "auth0_email_template.async_approval",
				ImportID:     "async_approval",
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, expectedData, data)
	})

	t.Run("it does not generate email template import data if email template does not exist", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mErr := mockManagamentError{status: http.StatusNotFound}
		emailTemplateAPI := mock.NewMockEmailTemplateAPI(ctrl)
		templates := []string{`verify_email`, `reset_email`, `welcome_email`, `blocked_account`, `stolen_credentials`, `enrollment_email`,
			`mfa_oob_code`, `change_password`, `password_reset`, `verify_email_by_code`, `reset_email_by_code`, `user_invitation`, `async_approval`}

		for _, tmpl := range templates {
			emailTemplateAPI.EXPECT().
				Read(gomock.Any(), tmpl).
				Return(nil, mErr)
		}

		fetcher := emailTemplateResourceFetcher{
			api: &auth0.API{
				EmailTemplate: emailTemplateAPI,
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Len(t, data, 0)
	})

	t.Run("it returns an error if api call fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		emailTemplateAPI := mock.NewMockEmailTemplateAPI(ctrl)
		emailTemplateAPI.EXPECT().
			Read(gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("failed to read email template"))

		fetcher := emailTemplateResourceFetcher{
			api: &auth0.API{
				EmailTemplate: emailTemplateAPI,
			},
		}

		_, err := fetcher.FetchData(context.Background())
		assert.EqualError(t, err, "failed to read email template")
	})
}

func TestLogStreamResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully retrieves log streams data", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		logAPI := mock.NewMockLogStreamAPI(ctrl)
		logAPI.EXPECT().
			List(gomock.Any()).
			Return(
				[]*management.LogStream{
					{
						ID:   auth0.String("lst_0000000000014444"),
						Name: auth0.String("DataDog"),
					},
					{
						ID:   auth0.String("lst_0000000000015555"),
						Name: auth0.String("HTTP Logs"),
					},
				},
				nil,
			)

		fetcher := logStreamResourceFetcher{
			api: &auth0.API{
				LogStream: logAPI,
			},
		}

		expectedData := importDataList{
			{
				ResourceName: "auth0_log_stream.datadog",
				ImportID:     "lst_0000000000014444",
			},
			{
				ResourceName: "auth0_log_stream.http_logs",
				ImportID:     "lst_0000000000015555",
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, expectedData, data)
	})

	t.Run("it returns an error if api call fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		logAPI := mock.NewMockLogStreamAPI(ctrl)
		logAPI.EXPECT().
			List(gomock.Any()).
			Return(nil, fmt.Errorf("failed to list log streams"))

		fetcher := logStreamResourceFetcher{
			api: &auth0.API{
				LogStream: logAPI,
			},
		}

		_, err := fetcher.FetchData(context.Background())
		assert.EqualError(t, err, "failed to list log streams")
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
			Connections(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(
				&management.OrganizationConnectionList{
					OrganizationConnections: []*management.OrganizationConnection{
						{
							ConnectionID: auth0.String("conn-1"),
						},
					},
				},
				nil,
			)
		orgAPI.EXPECT().
			Connections(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(
				&management.OrganizationConnectionList{
					OrganizationConnections: []*management.OrganizationConnection{
						{
							ConnectionID: auth0.String("conn-1"),
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
							Name: auth0.String("Organization 4 - NO CONNECTIONS!"),
						},
					},
				},
				nil,
			)
		orgAPI.EXPECT().
			Connections(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(
				&management.OrganizationConnectionList{
					OrganizationConnections: []*management.OrganizationConnection{
						{
							ConnectionID: auth0.String("conn-1"),
						},
					},
				},
				nil,
			)
		orgAPI.EXPECT().
			Connections(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(
				&management.OrganizationConnectionList{
					OrganizationConnections: []*management.OrganizationConnection{},
				},
				nil,
			)

		orgAPI.EXPECT().
			DiscoveryDomains(gomock.Any(), gomock.Any()).
			Return(
				&management.DiscoveryDomainList{
					Domains: []*management.OrganizationDiscoveryDomain{
						{
							Domain: auth0.String("example.com"),
						},
					},
				},
				nil,
			).Times(4)

		fetcher := organizationResourceFetcher{
			api: &auth0.API{
				Organization: orgAPI,
			},
		}

		expectedData := importDataList{
			{
				ResourceName: "auth0_organization.organization_1",
				ImportID:     "org_1",
			},
			{
				ResourceName: "auth0_organization_connections.organization_1",
				ImportID:     "org_1",
			},
			{
				ResourceName: "auth0_organization_discovery_domains.organization_1",
				ImportID:     "org_1",
			},
			{
				ResourceName: "auth0_organization.organization_2",
				ImportID:     "org_2",
			},
			{
				ResourceName: "auth0_organization_connections.organization_2",
				ImportID:     "org_2",
			},
			{
				ResourceName: "auth0_organization_discovery_domains.organization_2",
				ImportID:     "org_2",
			},
			{
				ResourceName: "auth0_organization.organization_3",
				ImportID:     "org_3",
			},
			{
				ResourceName: "auth0_organization_connections.organization_3",
				ImportID:     "org_3",
			},
			{
				ResourceName: "auth0_organization_discovery_domains.organization_3",
				ImportID:     "org_3",
			},
			{
				ResourceName: "auth0_organization.organization_4_no_connections",
				ImportID:     "org_4",
			},
			{
				ResourceName: "auth0_organization_discovery_domains.organization_4_no_connections",
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

func TestPagesResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully generates pages import data", func(t *testing.T) {
		fetcher := pagesResourceFetcher{}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Len(t, data, 1)
		assert.Equal(t, data[0].ResourceName, "auth0_pages.pages")
		assert.Greater(t, len(data[0].ImportID), 0)
	})
}

func TestPromptProviderResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully generates prompts import data", func(t *testing.T) {
		fetcher := promptResourceFetcher{}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Len(t, data, 1)
		assert.Equal(t, data[0].ResourceName, "auth0_prompt.prompts")
		assert.Greater(t, len(data[0].ImportID), 0)
	})
}

func TestPromptScreenRendererResourceFetcher_FetchData(t *testing.T) {
	var settingList = &management.PromptRenderingList{
		PromptRenderings: []*management.PromptRendering{
			{
				Prompt: (*management.PromptType)(auth0.String("login")),
				Screen: (*management.ScreenName)(auth0.String("login")),
			},
			{
				Prompt: (*management.PromptType)(auth0.String("login-password")),
				Screen: (*management.ScreenName)(auth0.String("login-password")),
			},
		},
	}

	t.Run("it successfully renders the prompts & screen settings import data", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		promptAPI := mock.NewMockPromptAPI(ctrl)
		promptAPI.EXPECT().ListRendering(gomock.Any()).
			Return(settingList, nil)

		fetcher := promptScreenRendererResourceFetcher{
			api: &auth0.API{
				Prompt: promptAPI,
			},
		}

		expectedData := importDataList{}
		for _, settings := range settingList.PromptRenderings {
			promptType := string(*settings.Prompt)
			screenName := string(*settings.Screen)
			expectedData = append(expectedData, importDataItem{
				ResourceName: "auth0_prompt_screen_renderer." + sanitizeResourceName(promptType+"_"+screenName),
				ImportID:     promptType + ":" + screenName,
			})
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.ElementsMatch(t, expectedData, data)
	})
	t.Run("it handles error, even if tenant does not have ACUL enabled", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		promptAPI := mock.NewMockPromptAPI(ctrl)
		promptAPI.EXPECT().ListRendering(gomock.Any()).
			Return(nil, fmt.Errorf("403 Forbidden: This tenant does not have Advanced Customizations enabled"))

		fetcher := promptScreenRendererResourceFetcher{
			api: &auth0.API{
				Prompt: promptAPI,
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.EqualError(t, err, "403 Forbidden: This tenant does not have Advanced Customizations enabled")
		assert.Len(t, data, 0)
	})
	t.Run("it returns error, if the API call fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		promptAPI := mock.NewMockPromptAPI(ctrl)
		promptAPI.EXPECT().ListRendering(gomock.Any()).
			Return(nil, fmt.Errorf("failed to read rendering settings"))

		fetcher := promptScreenRendererResourceFetcher{
			api: &auth0.API{
				Prompt: promptAPI,
			},
		}

		_, err := fetcher.FetchData(context.Background())
		assert.EqualError(t, err, "failed to read rendering settings")
	})
}

func TestPromptCustomTextResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully retrieves custom text prompts data", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockEnabledLocales := []string{"es", "fr", "en"}

		tenantAPI := mock.NewMockTenantAPI(ctrl)
		tenantAPI.EXPECT().
			Read(gomock.Any()).
			Return(
				&management.Tenant{
					EnabledLocales: &mockEnabledLocales,
				},
				nil,
			)

		fetcher := promptCustomTextResourceFetcherResourceFetcher{
			api: &auth0.API{
				Tenant: tenantAPI,
			},
		}

		promptTypes := []string{"login", "login-id", "login-password", "login-email-verification", "signup", "signup-id", "signup-password", "reset-password", "consent", "mfa-push", "mfa-otp", "mfa-voice", "mfa-phone", "mfa-webauthn", "mfa-sms", "mfa-email", "mfa-recovery-code", "mfa", "status", "device-flow", "email-verification", "email-otp-challenge", "organizations", "invitation", "common", "email-identifier-challenge", "passkeys", "login-passwordless", "phone-identifier-enrollment", "phone-identifier-challenge", "custom-form", "customized-consent", "logout", "captcha", "brute-force-protection"}

		expectedData := importDataList{}
		for _, enabledLocale := range mockEnabledLocales {
			for _, promptType := range promptTypes {
				expectedData = append(expectedData, importDataItem{
					ResourceName: fmt.Sprintf("auth0_prompt_custom_text.%s_%s", enabledLocale, strings.ReplaceAll(promptType, "-", "_")),
					ImportID:     fmt.Sprintf("%s::%s", promptType, enabledLocale),
				})
			}
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, expectedData, data)
	})

	t.Run("it returns an error if api call fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		tenantAPI := mock.NewMockTenantAPI(ctrl)
		tenantAPI.EXPECT().
			Read(gomock.Any()).
			Return(nil, fmt.Errorf("failed to read tenant"))

		fetcher := promptCustomTextResourceFetcherResourceFetcher{
			api: &auth0.API{
				Tenant: tenantAPI,
			},
		}

		_, err := fetcher.FetchData(context.Background())
		assert.EqualError(t, err, "failed to read tenant")
	})
}

func TestResourceServerResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully retrieves resource server data", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockScopes := []management.ResourceServerScope{
			{Value: auth0.String("read:user")},
			{Value: auth0.String("create:user")},
		}
		mockScopesEmpty := []management.ResourceServerScope{}

		resourceServerAPI := mock.NewMockResourceServerAPI(ctrl)
		resourceServerAPI.EXPECT().
			List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(
				&management.ResourceServerList{
					List: management.List{
						Start: 0,
						Limit: 2,
						Total: 4,
					},
					ResourceServers: []*management.ResourceServer{
						{
							ID:     auth0.String("610e04b71f71b9003a7eb3df"),
							Name:   auth0.String("Auth0 Management API"),
							Scopes: &mockScopes,
						},
						{
							ID:     auth0.String("6358fed7b77d3c391dd78a40"),
							Name:   auth0.String("Payments API"),
							Scopes: &mockScopes,
						},
					},
				},
				nil,
			)
		resourceServerAPI.EXPECT().
			List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(
				&management.ResourceServerList{
					List: management.List{
						Start: 2,
						Limit: 4,
						Total: 4,
					},
					ResourceServers: []*management.ResourceServer{
						{
							ID:     auth0.String("66ef6f9c435cab03def5fa16"),
							Name:   auth0.String("Blog API"),
							Scopes: &mockScopes,
						},
						{
							ID:     auth0.String("63bf6f9b0e025715cb91ce7c"),
							Name:   auth0.String("API with no scopes"),
							Scopes: &mockScopesEmpty,
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
				ResourceName: "auth0_resource_server.auth0_management_api",
				ImportID:     "610e04b71f71b9003a7eb3df",
			},
			{
				ResourceName: "auth0_resource_server_scopes.auth0_management_api",
				ImportID:     "610e04b71f71b9003a7eb3df",
			},
			{
				ResourceName: "auth0_resource_server.payments_api",
				ImportID:     "6358fed7b77d3c391dd78a40",
			},
			{
				ResourceName: "auth0_resource_server_scopes.payments_api",
				ImportID:     "6358fed7b77d3c391dd78a40",
			},
			{
				ResourceName: "auth0_resource_server.blog_api",
				ImportID:     "66ef6f9c435cab03def5fa16",
			},
			{
				ResourceName: "auth0_resource_server_scopes.blog_api",
				ImportID:     "66ef6f9c435cab03def5fa16",
			},
			{
				ResourceName: "auth0_resource_server.api_with_no_scopes",
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
			List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
							Name: auth0.String("Role 1 - No Permissions"),
						},
						{
							ID:   auth0.String("rol_2"),
							Name: auth0.String("Role 2"),
						},
					},
				},
				nil,
			)
		roleAPI.EXPECT().Permissions(gomock.Any(), gomock.Any()).Return(
			&management.PermissionList{
				List: management.List{
					Start: 0,
					Limit: 0,
					Total: 0,
				},
				Permissions: []*management.Permission{},
			},
			nil,
		)
		roleAPI.EXPECT().Permissions(gomock.Any(), gomock.Any()).Return(
			&management.PermissionList{
				List: management.List{
					Start: 0,
					Limit: 1,
					Total: 1,
				},
				Permissions: []*management.Permission{
					{
						Name: auth0.String("Permission-1"),
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
		roleAPI.EXPECT().Permissions(gomock.Any(), gomock.Any()).Return(
			&management.PermissionList{
				List: management.List{
					Start: 0,
					Limit: 4,
					Total: 4,
				},
				Permissions: []*management.Permission{
					{
						Name: auth0.String("Permission-1"),
					},
				},
			},
			nil,
		)
		roleAPI.EXPECT().Permissions(gomock.Any(), gomock.Any()).Return(
			&management.PermissionList{
				List: management.List{
					Start: 0,
					Limit: 4,
					Total: 4,
				},
				Permissions: []*management.Permission{
					{
						Name: auth0.String("Permission-2"),
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
				ResourceName: "auth0_role.role_1_no_permissions",
				ImportID:     "rol_1",
			},
			{
				ResourceName: "auth0_role.role_2",
				ImportID:     "rol_2",
			},
			{
				ResourceName: "auth0_role_permissions.role_2",
				ImportID:     "rol_2",
			},
			{
				ResourceName: "auth0_role.role_3",
				ImportID:     "rol_3",
			},
			{
				ResourceName: "auth0_role_permissions.role_3",
				ImportID:     "rol_3",
			},
			{
				ResourceName: "auth0_role.role_4",
				ImportID:     "rol_4",
			},
			{
				ResourceName: "auth0_role_permissions.role_4",
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

func TestSelfServiceProfileFetcher(t *testing.T) {
	t.Run("it successfully generates self service profile import data with custom text", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		selfServiceProfileAPI := mock.NewMockSelfServiceProfileAPI(ctrl)

		selfServiceProfileAPI.EXPECT().
			List(gomock.Any(), gomock.Any()).
			Return(&management.SelfServiceProfileList{
				SelfServiceProfile: []*management.SelfServiceProfile{
					{
						ID:   auth0.String("ss_profile_1"),
						Name: auth0.String("Self Service Profile 1"),
					},
				},
			}, nil)

		selfServiceProfileAPI.EXPECT().
			GetCustomText(gomock.Any(), "ss_profile_1", "en", "get-started").
			Return(map[string]interface{}{
				"introduction": "hello, world",
			}, nil)

		fetcher := selfServiceProfileFetcher{
			api: &auth0.API{
				SelfServiceProfile: selfServiceProfileAPI,
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Len(t, data, 2)
		assert.Equal(t, data[0].ResourceName, "auth0_self_service_profile.self_service_profile_1")
		assert.Greater(t, len(data[0].ImportID), 0)
		assert.Equal(t, data[1].ResourceName, "auth0_self_service_profile_custom_text.self_service_profile_1_en_get_started")
		assert.Greater(t, len(data[1].ImportID), 0)
	})

	t.Run("it successfully generates self service profile import data without custom text", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		selfServiceProfileAPI := mock.NewMockSelfServiceProfileAPI(ctrl)

		selfServiceProfileAPI.EXPECT().
			List(gomock.Any(), gomock.Any()).
			Return(&management.SelfServiceProfileList{
				SelfServiceProfile: []*management.SelfServiceProfile{
					{
						ID:   auth0.String("ss_profile_1"),
						Name: auth0.String("Self Service Profile 1"),
					},
				},
			}, nil)

		selfServiceProfileAPI.EXPECT().
			GetCustomText(gomock.Any(), "ss_profile_1", "en", "get-started").
			Return(map[string]interface{}{}, nil)

		fetcher := selfServiceProfileFetcher{
			api: &auth0.API{
				SelfServiceProfile: selfServiceProfileAPI,
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Len(t, data, 1)
		assert.Equal(t, data[0].ResourceName, "auth0_self_service_profile.self_service_profile_1")
		assert.Greater(t, len(data[0].ImportID), 0)
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

func TestTriggerActionsResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully retrieves trigger actions data", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		actionAPI := mock.NewMockActionAPI(ctrl)

		for _, trigger := range []string{"post-login", "credentials-exchange", "pre-user-registration", "post-user-registration", "post-change-password", "send-phone-message", "password-reset-post-challenge", "custom-email-provider", "custom-phone-provider"} {
			bindings := []*management.ActionBinding{}

			if trigger == "pre-user-registration" {
				bindings = []*management.ActionBinding{
					{
						ID: auth0.String("action1"),
					},
					{
						ID: auth0.String("action2"),
					},
				}
			}

			actionAPI.EXPECT().
				Bindings(gomock.Any(), gomock.Any()).
				Return(
					&management.ActionBindingList{
						Bindings: bindings,
					},
					nil,
				)
		}

		fetcher := triggerActionsResourceFetcher{
			api: &auth0.API{
				Action: actionAPI,
			},
		}

		expectedData := importDataList{
			{
				ResourceName: "auth0_trigger_actions.pre_user_registration",
				ImportID:     "pre-user-registration",
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
			Bindings(gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("failed to list action triggers"))

		fetcher := triggerActionsResourceFetcher{
			api: &auth0.API{
				Action: actionAPI,
			},
		}

		_, err := fetcher.FetchData(context.Background())
		assert.EqualError(t, err, "failed to list action triggers")
	})
}

func TestUserAttributeProfileResourceFetcher(t *testing.T) {
	t.Run("it successfully generates user attribute profile import data", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		userAttributeProfileAPI := mock.NewMockUserAttributeProfilesAPI(ctrl)

		userAttributeProfileAPI.EXPECT().
			List(gomock.Any(), gomock.Any()).
			Return(&management.UserAttributeProfileList{
				UserAttributeProfiles: []*management.UserAttributeProfile{
					{
						ID:   auth0.String("uap_123456"),
						Name: auth0.String("User Attribute Profile 1"),
					}}}, nil)

		fetcher := userAttributeProfilesResourceFetcher{
			api: &auth0.API{
				UserAttributeProfile: userAttributeProfileAPI,
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Len(t, data, 1)
		assert.Equal(t, data[0].ResourceName, "auth0_user_attribute_profile.user_attribute_profile_1")
		assert.Greater(t, len(data[0].ImportID), 0)
	})
}

func Test_promptScreenPartialResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully retrieves screen partial prompts data", func(t *testing.T) {
		fetcher := promptScreenPartialResourceFetcher{}

		original := screenPartialPromptTypeToScreenMap
		defer func() { screenPartialPromptTypeToScreenMap = original }()

		screenPartialPromptTypeToScreenMap = map[string][]string{
			"test1": {"test1"},
			"test2": {"test2-a", "test2-b"},
		}

		expectedData := importDataList{
			{
				ResourceName: "auth0_prompt_screen_partial.test1_test1",
				ImportID:     "test1:test1",
			},
			{
				ResourceName: "auth0_prompt_screen_partial.test2_test2_a",
				ImportID:     "test2:test2-a",
			},
			{
				ResourceName: "auth0_prompt_screen_partial.test2_test2_b",
				ImportID:     "test2:test2-b",
			},
		}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, expectedData, data)
	})
}
