package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/open"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

const (
	cliLoginTestingClientName        string = "CLI Login Testing"
	cliLoginTestingClientDescription string = "A client used for testing logins using the Auth0 CLI."
	cliLoginTestingCallbackAddr      string = "localhost:8484"
	cliLoginTestingCallbackURL       string = "http://localhost:8484"
	cliLoginTestingInitiateLoginURI  string = "https://cli.auth0.com"
)

var (
	cliLoginTestingScopes []string = []string{"openid", "profile"}
)

func tryLoginCmd(cli *cli) *cobra.Command {
	var clientID string
	var connectionName string

	cmd := &cobra.Command{
		Use:   "try-login",
		Short: "Try out your universal login box",
		Long: `auth0 try-login
Launch a browser to try out your universal login box for the given client.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var userInfo *auth.UserInfo

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

			abort, needsLocalCallbackURL := runLoginFlowPreflightChecks(cli, client)
			if abort {
				return nil
			}

			tokenResponse, err := runLoginFlow(
				cli,
				tenant,
				client,
				connectionName,
				needsLocalCallbackURL,
				"",      // audience is only supported for get-token
				"login", // force a login page when using try-login
				cliLoginTestingScopes,
			)
			if err != nil {
				return err
			}

			if err := ansi.Spinner("Fetching user metadata", func() error {
				// Use the access token to fetch user information from the /userinfo
				// endpoint.
				userInfo, err = auth.FetchUserInfo(tenant.Domain, tokenResponse.AccessToken)
				return err
			}); err != nil {
				return err
			}

			fmt.Fprint(cli.renderer.MessageWriter, "\n")
			cli.renderer.TryLogin(userInfo, tokenResponse)
			return nil
		},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.Flags().StringVarP(&clientID, "client-id", "c", "", "Client Id for which to test login.")
	cmd.Flags().StringVarP(&connectionName, "connection", "", "", "Connection to test during login.")
	return cmd
}

// runLoginFlowPreflightChecks checks if we need to make any updates to the
// client being tested, and asks the user to confirm whether to proceed.
func runLoginFlowPreflightChecks(cli *cli, c *management.Client) (bool, bool) {
	cli.renderer.Infof("A browser window will open to begin this client's login flow.")
	cli.renderer.Infof("Once login is complete, you can return to the CLI to view user profile information and tokens.\n")

	// check if the chosen client includes our local callback URL in its
	// allowed list. If not we'll need to add it (after asking the user
	// for permission).
	needsLocalCallbackURL := !checkForLocalCallbackURL(c)
	if needsLocalCallbackURL {
		cli.renderer.Warnf("The client you are using does not currently allow callbacks to localhost.")
		cli.renderer.Warnf("To complete the login flow the CLI needs to redirect logins to a local server and record the result.\n")
		cli.renderer.Warnf("The client will be modified to update the allowed callback URLs, we'll remove them when done.")
		cli.renderer.Warnf("If you do not wish to modify the client, you can abort now.\n")
	}

	if confirmed := prompt.Confirm("Do you wish to proceed?"); !confirmed {
		return true, needsLocalCallbackURL
	}
	fmt.Fprint(cli.renderer.MessageWriter, "\n")

	return false, needsLocalCallbackURL
}

// runLoginFlow initiates a full user-facing login flow, waits for a response
// and returns the retrieved tokens to the caller when done.
func runLoginFlow(cli *cli, t tenant, c *management.Client, connName string, needsLocalCallbackURL bool, audience, prompt string, scopes []string) (*auth.TokenResponse, error) {
	var tokenResponse *auth.TokenResponse

	err := ansi.Spinner("Waiting for login flow to complete", func() error {
		if needsLocalCallbackURL {
			if err := addLocalCallbackURLToClient(cli.api.Client, c); err != nil {
				return err
			}
		}

		// Build a login URL and initiate login in a browser window.
		loginURL, err := buildInitiateLoginURL(t.Domain, c.GetClientID(), connName, audience, prompt, scopes)
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
			t.Domain,
			c.GetClientID(),
			c.GetClientSecret(),
			authCode,
			cliLoginTestingCallbackURL,
		)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		// if we added the local callback URL to the client then we need to
		// remove it when we're done
		if needsLocalCallbackURL {
			if err := removeLocalCallbackURLFromClient(cli.api.Client, c); err != nil {
				return err
			}
		}

		return nil
	})

	return tokenResponse, err
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

// check if a client is already configured with our local callback URL
func checkForLocalCallbackURL(client *management.Client) bool {
	for _, rawCallbackURL := range client.Callbacks {
		callbackURL := rawCallbackURL.(string)
		if callbackURL == cliLoginTestingCallbackURL {
			return true
		}
	}

	return false
}

// adds the localhost callback URL to a given client
func addLocalCallbackURLToClient(clientManager auth0.ClientAPI, client *management.Client) error {
	for _, rawCallbackURL := range client.Callbacks {
		callbackURL := rawCallbackURL.(string)
		if callbackURL == cliLoginTestingCallbackURL {
			return nil
		}
	}

	updatedClient := &management.Client{
		Callbacks: append(client.Callbacks, cliLoginTestingCallbackURL),
	}
	// reflect the changes in the original client instance so when we check it
	// later it has the proper values in Callbacks
	client.Callbacks = updatedClient.Callbacks
	return clientManager.Update(client.GetClientID(), updatedClient)
}

func removeLocalCallbackURLFromClient(clientManager auth0.ClientAPI, client *management.Client) error {
	callbacks := []interface{}{}
	for _, rawCallbackURL := range client.Callbacks {
		callbackURL := rawCallbackURL.(string)
		if callbackURL != cliLoginTestingCallbackURL {
			callbacks = append(callbacks, callbackURL)
		}
	}

	// no callback URLs to remove, so don't attempt to do so
	if len(client.Callbacks) == len(callbacks) {
		return nil
	}

	// can't update a client to have 0 callback URLs, so don't attempt it
	if len(callbacks) == 0 {
		return nil
	}

	updatedClient := &management.Client{
		Callbacks: callbacks,
	}
	return clientManager.Update(client.GetClientID(), updatedClient)

}

// buildInitiateLoginURL constructs a URL + query string that can be used to
// initiate a login-flow from the CLI.
func buildInitiateLoginURL(domain, clientID, connectionName, audience, prompt string, scopes []string) (string, error) {
	var path string = "/authorize"

	q := url.Values{}
	q.Add("client_id", clientID)
	q.Add("response_type", "code")
	q.Add("redirect_uri", cliLoginTestingCallbackURL)

	if prompt != "" {
		q.Add("prompt", prompt)
	}

	if connectionName != "" {
		q.Add("connection", connectionName)
	}

	if audience != "" {
		q.Add("audience", audience)
	}

	if len(scopes) > 0 {
		q.Add("scope", strings.Join(scopes, " "))
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
