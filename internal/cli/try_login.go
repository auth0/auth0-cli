package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/auth0/auth0-cli/internal/auth"
	"github.com/auth0/auth0-cli/internal/open"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5"
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
	cmd := &cobra.Command{
		Use:   "try-login",
		Short: "Try out your universal login box",
		Long: `$ auth0 try-login
Launch a browser to try out your universal login box for the given client.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			tenant, err := cli.getTenant()
			if err != nil {
				return err
			}

			// use the client ID as passed in by the user, or default to the
			// "CLI Login Testing" client if none passed. This client is only
			// used for testing login from the CLI and will be created if it
			// does not exist.
			clientID, _ := cmd.Flags().GetString("client-id")
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
				return open.URL(client.GetInitiateLoginURI())
			}

			// Build a login URL and initiate login in a browser window.
			loginURL, err := buildInitiateLoginURL(tenant.Domain, client.GetClientID())
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
			tokenResponse, err := auth.ExchangeCodeForToken(
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
			userInfo, err := auth.FetchUserInfo(tenant.Domain, tokenResponse.AccessToken)
			if err != nil {
				return err
			}

			reveal, _ := cmd.Flags().GetBool("reveal")
			cli.renderer.TryLogin(userInfo, tokenResponse, reveal)
			return nil
		},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.Flags().StringP("client-id", "c", "", "Client ID for which to test login.")
	cmd.Flags().BoolP("reveal", "r", false, "⚠️  Reveal tokens after successful login.")
	return cmd
}

// getOrCreateCLITesterClient uses the manage API to look for an existing client
// named `cliLoginTestingClientName`, and if it doesn't find one creates it with
// default settings.
func getOrCreateCLITesterClient(clientManager *management.ClientManager) (*management.Client, error) {
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
func buildInitiateLoginURL(domain, clientID string) (string, error) {
	var path string = "/authorize"

	q := url.Values{}
	q.Add("client_id", clientID)
	q.Add("response_type", "code")
	q.Add("prompt", "login")
	q.Add("scope", cliLoginTestingScopes)
	q.Add("redirect_uri", cliLoginTestingCallbackURL)

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
	codeCh := make(chan string)
	errCh := make(chan error)

	m := http.NewServeMux()
	s := http.Server{Addr: cliLoginTestingCallbackAddr, Handler: m}

	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		authCode := r.URL.Query().Get("code")
		if authCode == "" {
			_, _ = w.Write([]byte("<p>&#10060; Unable to extract code from request, please try authenticating again</p>"))
		} else {
			_, _ = w.Write([]byte("<p>&#128075; You can close the window and go back to the CLI to see the user info and tokens</p>"))
		}
		codeCh <- authCode
	})

	go func() {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case code := <-codeCh:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := s.Shutdown(ctx)
		return code, err
	case err := <-errCh:
		return "", err
	}
}
