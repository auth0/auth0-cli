package auth

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/golang/mock/gomock"
	goKeyring "github.com/zalando/go-keyring"

	"github.com/auth0/auth0-cli/internal/keyring"
)

// HTTPTransport implements a http.RoundTripper for testing purposes only.
type testTransport struct {
	withResponse *http.Response
	withError    error
	requests     []*http.Request
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.requests = append(t.requests, req)
	return t.withResponse, t.withError
}

func TestTokenRetriever_Refresh(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		oldCreds := credentials
		defer func() { credentials = oldCreds }()
		credentials = &Credentials{
			Audience:           "https://test.com/api/v2/",
			ClientID:           "client-id",
			OauthTokenEndpoint: "https://test.com/oauth/device/code",
			DeviceCodeEndpoint: "https://test.com/token",
		}

		testTenantName := "auth0-cli-test.us.auth0.com"

		goKeyring.MockInit()
		err := keyring.StoreRefreshToken(testTenantName, "refresh-token-here")
		if err != nil {
			t.Fatal(err)
		}

		transport := &testTransport{
			withResponse: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(`{
						"access_token": "access-token-here",
						"id_token": "id-token-here",
						"token_type": "token-type-here",
						"expires_in": 1000
					}`))),
			},
		}

		client := &http.Client{Transport: transport}

		got, err := RefreshAccessToken(client, testTenantName)
		if err != nil {
			t.Fatal(err)
		}

		want := TokenResponse{
			AccessToken: "access-token-here",
			IDToken:     "id-token-here",
			TokenType:   "token-type-here",
			ExpiresIn:   1000,
		}

		if want != got {
			t.Fatalf("wanted: %v, got: %v", want, got)
		}

		req := transport.requests[0]
		err = req.ParseForm()
		if err != nil {
			t.Fatal(err)
		}

		t.Log("num reqs", len(transport.requests))

		if want, got := "https://test.com/token", req.URL.String(); want != got {
			t.Fatalf("wanted request URL: %v, got: %v", want, got)
		}
		if want, got := "refresh_token", req.Form["grant_type"][0]; want != got {
			t.Fatalf("wanted grant_type: %v, got: %v", want, got)
		}
		if want, got := "client-id", req.Form["client_id"][0]; want != got {
			t.Fatalf("wanted grant_type: %v, got: %v", want, got)
		}
		if want, got := "refresh-token-here", req.Form["refresh_token"][0]; want != got {
			t.Fatalf("wanted grant_type: %v, got: %v", want, got)
		}
	})
}
