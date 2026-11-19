package agents

import (
	"errors"
	"testing"

	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func appsOf(app *managementv3.Client) []*managementv3.Client {
	if app == nil {
		return nil
	}

	return []*managementv3.Client{app}
}

func TestGrantsOfReadsTheSubjectType(t *testing.T) {
	f := newFixture(t)
	f.grants.EXPECT().
		List(gomock.Any(), &managementv3.ListClientGrantsRequestParameters{ClientID: ptr(museApp)}).
		Return(grantPage(userGrantOf("cgr_user", shopAPI, "orders:read"), clientGrantOf("cgr_m2m", shopAPI, "orders:read"), grantOf("cgr_none", shopAPI)), nil)

	got, err := f.svc.GrantsOf(t.Context(), museApp)

	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.True(t, got[0].IsUserGrant())
	assert.False(t, got[1].IsUserGrant())
	assert.False(t, got[2].IsUserGrant())
}

func TestGrantsOfDropsTheGrantsOfOtherApps(t *testing.T) {
	f := newFixture(t)
	f.grants.EXPECT().
		List(gomock.Any(), &managementv3.ListClientGrantsRequestParameters{ClientID: ptr(museApp)}).
		Return(grantPage(
			grantOf("cgr_mine", shopAPI, "orders:read"),
			grantFor("client_other", grantOf("cgr_theirs", shopAPI, "orders:read")),
			&managementv3.ClientGrantResponseContent{ID: ptr("cgr_unowned"), Audience: ptr(shopAPI)},
		), nil)

	got, err := f.svc.GrantsOf(t.Context(), museApp)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "cgr_mine", got[0].ID)
}

func TestGrantsByClient(t *testing.T) {
	f := newFixture(t)
	f.grants.EXPECT().
		List(gomock.Any(), gomock.Any()).
		Return(grantPage(
			grantOf("cgr_a", shopAPI),
			grantOf("cgr_b", otherAPI),
			grantFor("client_other", grantOf("cgr_c", shopAPI)),
		), nil).
		Times(1)

	got, err := f.svc.GrantsByClient(t.Context())

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Len(t, got[museApp], 2)
	assert.Len(t, got["client_other"], 1)
}

func TestAPIs(t *testing.T) {
	t.Run("every resource server, with its scopes", func(t *testing.T) {
		f := newFixture(t)
		f.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(serverPage(otherServer(), shopServer()), nil)

		apis, err := f.svc.APIs(t.Context())

		require.NoError(t, err)
		assert.Equal(t, []API{{ID: "rs_other", Identifier: otherAPI}, shopAPIValue()}, apis)
	})

	t.Run("the list fails", func(t *testing.T) {
		f := newFixture(t)
		f.servers.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))

		_, err := f.svc.APIs(t.Context())

		assert.EqualError(t, err, "failed to list APIs: boom")
	})
}

func TestPickAPIs(t *testing.T) {
	other := API{ID: "rs_other", Identifier: otherAPI}
	all := []API{shopAPIValue(), other}

	tests := []struct {
		name      string
		picked    []string
		want      []API
		wantError string
	}{
		{name: "by identifier", picked: []string{otherAPI}, want: []API{other}},
		{name: "by id", picked: []string{"rs_shop"}, want: []API{shopAPIValue()}},
		{name: "in the order picked", picked: []string{otherAPI, shopAPI}, want: []API{other, shopAPIValue()}},
		{name: "picked twice", picked: []string{"rs_shop", shopAPI}, want: []API{shopAPIValue()}},
		{name: "nothing picked", want: nil},
		{name: "an unknown API", picked: []string{shopAPI, "https://nope.example"}, wantError: `"https://nope.example" is not an API of this tenant`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := PickAPIs(all, test.picked)

			if test.wantError != "" {
				assert.EqualError(t, err, test.wantError)

				var unknown *UnknownAPIError
				assert.ErrorAs(t, err, &unknown)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}
