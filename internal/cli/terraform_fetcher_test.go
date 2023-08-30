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

func TestTeantResourceFetcher_FetchData(t *testing.T) {
	t.Run("it successfully generates tenant import data", func(t *testing.T) {
		fetcher := tenantResourceFetcher{}

		data, err := fetcher.FetchData(context.Background())
		assert.NoError(t, err)
		assert.Len(t, data, 1)
		assert.Equal(t, data[0].ResourceName, "auth0_tenant.tenant")
		assert.Greater(t, len(data[0].ImportID), 0)
	})
}
