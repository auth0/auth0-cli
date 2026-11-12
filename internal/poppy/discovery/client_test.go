package discovery

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient returns a client for the TLS server and the server itself.
func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()

	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)

	return New(srv.Client()), srv
}

func TestClientGet(t *testing.T) {
	t.Run("returns any status with its headers and body", func(t *testing.T) {
		client, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "auth0-cli-poppy-doctor", r.Header.Get("User-Agent"))
			w.Header().Set("X-Test", "yes")
			w.WriteHeader(http.StatusTeapot)
			fmt.Fprint(w, "short and stout")
		})

		resp, err := client.Get(context.Background(), srv.URL)

		require.NoError(t, err)
		assert.Equal(t, http.StatusTeapot, resp.StatusCode)
		assert.Equal(t, "yes", resp.Header.Get("X-Test"))
		assert.Equal(t, "short and stout", string(resp.Body))
	})

	t.Run("rejects a URL that is not HTTPS before sending anything", func(t *testing.T) {
		client := New(nil)

		_, err := client.Get(context.Background(), "http://shop.example/")

		var insecure *InsecureURLError
		require.ErrorAs(t, err, &insecure)
		assert.Equal(t, "http://shop.example/", insecure.URL)
	})

	t.Run("rejects an invalid URL", func(t *testing.T) {
		_, err := New(nil).Get(context.Background(), "https://shop.example/%zz")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid URL")
	})

	t.Run("reports a transport failure", func(t *testing.T) {
		client, srv := newTestClient(t, func(http.ResponseWriter, *http.Request) {})
		srv.Close()

		_, err := client.Get(context.Background(), srv.URL)

		require.Error(t, err)
	})
}

func TestClientGetFinalURL(t *testing.T) {
	client, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/final?x=1", http.StatusFound)
			return
		}
		fmt.Fprint(w, "done")
	})

	direct, err := client.Get(context.Background(), srv.URL+"/direct")
	require.NoError(t, err)
	assert.Equal(t, srv.URL+"/direct", direct.URL)

	redirected, err := client.Get(context.Background(), srv.URL+"/start")
	require.NoError(t, err)
	assert.Equal(t, srv.URL+"/final?x=1", redirected.URL, "the URL after the redirect")
}

func TestClientGetRedirects(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantStatus int
		wantErr    error
		wantTarget string
	}{
		{
			name: "follows an HTTPS redirect",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/" {
					http.Redirect(w, r, "/final", http.StatusFound)
					return
				}
				fmt.Fprint(w, "done")
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "fails on a redirect to HTTP and names the target",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "http://shop.example/insecure", http.StatusMovedPermanently)
			},
			wantErr:    &InsecureURLError{},
			wantTarget: "http://shop.example/insecure",
		},
		{
			name:       "follows five redirects",
			handler:    redirectChain(5),
			wantStatus: http.StatusOK,
		},
		{
			name:    "stops after more than five redirects",
			handler: redirectChain(6),
			wantErr: ErrTooManyRedirects,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, srv := newTestClient(t, test.handler)

			resp, err := client.Get(context.Background(), srv.URL)

			switch {
			case test.wantErr == nil:
				require.NoError(t, err)
				assert.Equal(t, test.wantStatus, resp.StatusCode)
			case test.wantTarget != "":
				var insecure *InsecureURLError
				require.ErrorAs(t, err, &insecure)
				assert.Equal(t, test.wantTarget, insecure.URL)
				assert.Contains(t, err.Error(), test.wantTarget)
			default:
				require.ErrorIs(t, err, test.wantErr)
			}
		})
	}
}

// redirectChain redirects from / through /1, /2 and so on, count times, then answers 200.
func redirectChain(count int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var hop int
		if r.URL.Path != "/" {
			_, _ = fmt.Sscanf(r.URL.Path, "/%d", &hop)
		}
		if hop < count {
			http.Redirect(w, r, fmt.Sprintf("/%d", hop+1), http.StatusFound)
			return
		}
		fmt.Fprint(w, "done")
	}
}

func TestClientGetBodyCap(t *testing.T) {
	tests := []struct {
		name    string
		size    int
		wantErr error
	}{
		{name: "exactly the cap", size: maxBodyBytes},
		{name: "one byte over the cap", size: maxBodyBytes + 1, wantErr: ErrBodyTooLarge},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, srv := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(strings.Repeat("a", test.size)))
			})

			resp, err := client.Get(context.Background(), srv.URL)

			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Len(t, resp.Body, test.size)
		})
	}
}

func TestClientNeverSendsCredentials(t *testing.T) {
	var seen []http.Header
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Clone())
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "secret", Path: "/"})
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/next", http.StatusFound)
		}
	}))
	t.Cleanup(srv.Close)

	// An injected client with a cookie jar must still send no credentials.
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	base := srv.Client()
	base.Jar = jar
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	jar.SetCookies(u, []*http.Cookie{{Name: "preset", Value: "yes", Path: "/"}})
	client := New(base)

	for range 2 {
		_, err = client.Get(context.Background(), srv.URL)
		require.NoError(t, err)
	}

	require.Len(t, seen, 4)
	for _, header := range seen {
		assert.Empty(t, header.Values("Authorization"))
		assert.Empty(t, header.Values("Cookie"))
	}
}

func TestNewDoesNotChangeTheInjectedClient(t *testing.T) {
	hc := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}}

	client := New(hc)

	assert.Nil(t, hc.CheckRedirect)
	assert.Zero(t, hc.Timeout)
	assert.Equal(t, requestTimeout, client.http.Timeout)
	assert.Same(t, hc.Transport, client.http.Transport)
	assert.NotNil(t, New(nil).http)
}
