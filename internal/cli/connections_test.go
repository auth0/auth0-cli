package cli

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
)

func TestCollectConnections(t *testing.T) {
	t.Run("pages across responses until exhausted", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		secondPage := &auth0.ConnectionForListPage{
			Results: []*managementv3.ConnectionForList{{ID: auth0.String("con-3"), Name: auth0.String("Conn 3")}},
			NextPageFunc: func(_ context.Context) (*auth0.ConnectionForListPage, error) {
				return nil, core.ErrNoPages
			},
		}
		firstPage := &auth0.ConnectionForListPage{
			Results: []*managementv3.ConnectionForList{
				{ID: auth0.String("con-1"), Name: auth0.String("Conn 1")},
				{ID: auth0.String("con-2"), Name: auth0.String("Conn 2")},
			},
			NextPageFunc: func(_ context.Context) (*auth0.ConnectionForListPage, error) {
				return secondPage, nil
			},
		}

		connAPI := mock.NewMockConnectionAPIV3(ctrl)
		connAPI.EXPECT().List(gomock.Any(), gomock.Any()).Return(firstPage, nil)

		cli := &cli{apiv3: &auth0.APIV3{Connection: connAPI}}

		connections, err := collectConnections(context.Background(), cli, &managementv3.ListConnectionsQueryParameters{}, 0)
		assert.NoError(t, err)
		assert.Len(t, connections, 3)
		assert.Equal(t, "con-3", connections[2].GetID())
	})

	t.Run("stops at the requested limit without paging further", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		firstPage := &auth0.ConnectionForListPage{
			Results: []*managementv3.ConnectionForList{
				{ID: auth0.String("con-1"), Name: auth0.String("Conn 1")},
				{ID: auth0.String("con-2"), Name: auth0.String("Conn 2")},
			},
			NextPageFunc: func(_ context.Context) (*auth0.ConnectionForListPage, error) {
				t.Fatal("should not page past the limit")
				return nil, nil
			},
		}

		connAPI := mock.NewMockConnectionAPIV3(ctrl)
		connAPI.EXPECT().List(gomock.Any(), gomock.Any()).Return(firstPage, nil)

		cli := &cli{apiv3: &auth0.APIV3{Connection: connAPI}}

		connections, err := collectConnections(context.Background(), cli, &managementv3.ListConnectionsQueryParameters{}, 1)
		assert.NoError(t, err)
		assert.Len(t, connections, 1)
	})
}

func TestConnectionPickerOptions(t *testing.T) {
	tests := []struct {
		name         string
		connections  []*managementv3.ConnectionForList
		apiError     error
		assertOutput func(t testing.TB, options pickerOptions)
		assertError  func(t testing.TB, err error)
	}{
		{
			name: "happy path",
			connections: []*managementv3.ConnectionForList{
				{ID: auth0.String("con-1"), Name: auth0.String("conn-1")},
				{ID: auth0.String("con-2"), Name: auth0.String("conn-2")},
			},
			assertOutput: func(t testing.TB, options pickerOptions) {
				assert.Len(t, options, 2)
				assert.Equal(t, "conn-1 (con-1)", options[0].label)
				assert.Equal(t, "con-1", options[0].value)
			},
			assertError: func(t testing.TB, _ error) {
				t.Fail()
			},
		},
		{
			name:        "no connections",
			connections: []*managementv3.ConnectionForList{},
			assertOutput: func(t testing.TB, _ pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorContains(t, err, "there are currently no connections to choose from")
			},
		},
		{
			name:     "API error",
			apiError: errors.New("error"),
			assertOutput: func(t testing.TB, _ pickerOptions) {
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

			connAPI := mock.NewMockConnectionAPIV3(ctrl)
			if test.apiError != nil {
				connAPI.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, test.apiError)
			} else {
				connAPI.EXPECT().List(gomock.Any(), gomock.Any()).Return(
					&auth0.ConnectionForListPage{
						Results: test.connections,
						NextPageFunc: func(_ context.Context) (*auth0.ConnectionForListPage, error) {
							return nil, core.ErrNoPages
						},
					}, nil)
			}

			cli := &cli{apiv3: &auth0.APIV3{Connection: connAPI}}

			options, err := cli.connectionPickerOptions(context.Background())

			if err != nil {
				test.assertError(t, err)
			} else {
				test.assertOutput(t, options)
			}
		})
	}
}

func TestStripConnectionImmutableFields(t *testing.T) {
	body := json.RawMessage(`{"id":"con-1","name":"my-conn","strategy":"auth0","display_name":"My Conn","options":{"foo":"bar"}}`)

	stripped, err := stripConnectionImmutableFields(body)
	assert.NoError(t, err)

	var got map[string]json.RawMessage
	assert.NoError(t, json.Unmarshal(stripped, &got))

	// Immutable fields are removed.
	assert.NotContains(t, got, "id")
	assert.NotContains(t, got, "name")
	assert.NotContains(t, got, "strategy")

	// Mutable fields are preserved.
	assert.Contains(t, got, "display_name")
	assert.Contains(t, got, "options")
}
