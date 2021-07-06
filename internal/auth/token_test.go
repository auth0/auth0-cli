package auth

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/auth0/auth0-cli/internal/auth/mock"
	"github.com/golang/mock/gomock"
)

// HTTPTransport implements an http.RoundTripper for testing purposes only.
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

		secretsMock := mock.NewMockSecretStore(ctrl)
		secretsMock.EXPECT().Get("auth0-cli", "mytenant").Return("refresh-token-here", nil).Times(1)

		transport := &testTransport{
			withResponse: &http.Response{
				StatusCode: http.StatusOK,
				Body: ioutil.NopCloser(bytes.NewReader([]byte(`{
						"access_token": "access-token-here",
						"id_token": "id-token-here",
						"token_type": "token-type-here",
						"expires_in": 1000
					}`))),
			},
		}

		client := &http.Client{Transport: transport}

		tr := &TokenRetriever{
			Authenticator: &Authenticator{"https://test.com/api/v2/", "client-id", "https://test.com/oauth/device/code", "https://test.com/token"},
			Secrets:       secretsMock,
			Client:        client,
		}

		got, err := tr.Refresh(context.Background(), "mytenant")
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
