package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/open"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

const (
	cliLoginTestingClientName        string = "CLI Login Testing"
	cliLoginTestingClientDescription string = "A client used for testing logins using the Auth0 CLI."
	cliLoginTestingCallbackAddr      string = "localhost:8484"
	cliLoginTestingCallbackURL       string = "http://localhost:8484"
	cliLoginTestingInitiateLoginURI  string = "https://cli.auth0.com"
	cliLoginTestingScopes            string = "openid profile"
)

func tryLoginCmd(cli *cli) *cobra.Command {
	var clientID string
	var connectionName string
	var reveal bool

	cmd := &cobra.Command{
		Use:   "try-login",
		Short: "Try out your universal login box",
		Long: `$ auth0 try-login
Launch a browser to try out your universal login box for the given client.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var userInfo *auth.UserInfo
			var tokenResponse *auth.TokenResponse

			err := ansi.Spinner("Trying login", func() error {
				var err error
				tenant, err := cli.getTenant()
				if err != nil {
					return err
				}

				// use the client ID as passed in by the user, or default to the
				// "CLI Login Testing" client if none passed. This client is only
				// used for testing login from the CLI and will be created if it
				// does not exist.
				if clientID == "" {
					client, err := getOrCreateCLITesterClient(cli.api.Client)
					if err != nil {
						return err
					}
					clientID = client.GetClientID()
				}

				client, err := cli.api.Client.Read(clientID)
				if err != nil {
					return err
				}

				// check if the client's initiate_login_uri matches the one for our
				// "CLI Login Testing" app. If so, then initiate the login via the
				// `/authorize` endpoint, if not, open a browser at the client's
				// configured URL. If none is specified, return an error to the
				// caller explaining the problem.
				if client.GetInitiateLoginURI() == "" {
					return fmt.Errorf(
						"client %s does not specify a URL with which to initiate login",
						client.GetClientID(),
					)
				}

				if client.GetInitiateLoginURI() != cliLoginTestingInitiateLoginURI {
					if connectionName != "" {
						cli.renderer.Warnf("Specific connections are not supported when using a non-default client, ignoring.")
						cli.renderer.Warnf("You should ensure the connection you wish to test is enabled for the client you want to use in the Auth0 Dashboard.")
					}
					return open.URL(client.GetInitiateLoginURI())
				}

				// Build a login URL and initiate login in a browser window.
				loginURL, err := buildInitiateLoginURL(tenant.Domain, client.GetClientID(), connectionName)
				if err != nil {
					return err
				}

				if err := open.URL(loginURL); err != nil {
					return err
				}

				// launch a HTTP server to wait for the callback to capture the auth
				// code.
				authCode, err := waitForBrowserCallback()
				if err != nil {
					return err
				}

				// once the callback is received, exchange the code for an access
				// token.
				tokenResponse, err = auth.ExchangeCodeForToken(
					tenant.Domain,
					client.GetClientID(),
					client.GetClientSecret(),
					authCode,
					cliLoginTestingCallbackURL,
				)
				if err != nil {
					return fmt.Errorf("%w", err)
				}

				// Use the access token to fetch user information from the /userinfo
				// endpoint.
				userInfo, err = auth.FetchUserInfo(tenant.Domain, tokenResponse.AccessToken)

				return err
			})

			if err != nil {
				return err
			}

			cli.renderer.TryLogin(userInfo, tokenResponse, reveal)
			return nil
		},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.Flags().StringVarP(&clientID, "client-id", "c", "", "Client ID for which to test login.")
	cmd.Flags().StringVarP(&connectionName, "connection", "", "", "Connection to test during login.")
	cmd.Flags().BoolVarP(&reveal, "reveal", "r", false, "⚠️  Reveal tokens after successful login.")
	return cmd
}

// getOrCreateCLITesterClient uses the manage API to look for an existing client
// named `cliLoginTestingClientName`, and if it doesn't find one creates it with
// default settings.
func getOrCreateCLITesterClient(clientManager auth0.ClientAPI) (*management.Client, error) {
	clients, err := clientManager.List()
	if err != nil {
		return nil, err
	}

	for _, client := range clients.Clients {
		if client.GetName() == cliLoginTestingClientName {
			return client, nil
		}
	}

	// we couldn't find the default client, so let's create it
	client := &management.Client{
		Name:             auth0.String(cliLoginTestingClientName),
		Description:      auth0.String(cliLoginTestingClientDescription),
		Callbacks:        []interface{}{cliLoginTestingCallbackURL},
		InitiateLoginURI: auth0.String(cliLoginTestingInitiateLoginURI),
	}
	return client, clientManager.Create(client)
}

// buildInitiateLoginURL constructs a URL + query string that can be used to
// initiate a login-flow from the CLI.
func buildInitiateLoginURL(domain, clientID, connectionName string) (string, error) {
	var path string = "/authorize"

	q := url.Values{}
	q.Add("client_id", clientID)
	q.Add("response_type", "code")
	q.Add("prompt", "login")
	q.Add("scope", cliLoginTestingScopes)
	q.Add("redirect_uri", cliLoginTestingCallbackURL)

	if connectionName != "" {
		q.Add("connection", connectionName)
	}

	u := &url.URL{
		Scheme:   "https",
		Host:     domain,
		Path:     path,
		RawQuery: q.Encode(),
	}

	return u.String(), nil
}

// waitForBrowserCallback lauches a new HTTP server listening on
// `cliLoginTestingCallbackAddr` and waits for a request. Once received, the
// `code` is extracted from the query string (if any), and returns it to the
// caller.
func waitForBrowserCallback() (string, error) {
	type callback struct {
		code           string
		err            string
		errDescription string
	}

	cbCh := make(chan *callback)
	errCh := make(chan error)

	m := http.NewServeMux()
	s := http.Server{Addr: cliLoginTestingCallbackAddr, Handler: m}

	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		cb := &callback{
			code:           r.URL.Query().Get("code"),
			err:            r.URL.Query().Get("error"),
			errDescription: r.URL.Query().Get("error_description"),
		}

		if cb.code == "" {
			_, _ = w.Write([]byte("<p>&#10060; Unable to extract code from request, please try authenticating again</p>"))
		} else {
			_, _ = w.Write([]byte("<p>&#128075; You can close the window and go back to the CLI to see the user info and tokens</p>"))
		}

		cbCh <- cb
	})

	go func() {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case cb := <-cbCh:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		defer func(c context.Context) { _ = s.Shutdown(ctx) }(ctx)

		var err error
		if cb.err != "" {
			err = fmt.Errorf("%s: %s", cb.err, cb.errDescription)
		}
		return cb.code, err
	case err := <-errCh:
		return "", err
	}
}
