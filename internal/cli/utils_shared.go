package cli

import (
	"fmt"

	"encoding/json"
	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth/authutil"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/open"
	"github.com/auth0/auth0-cli/internal/prompt"
	"gopkg.in/auth0.v5/management"
	"net/http"
	"net/url"
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

func BuildOauthTokenURL(domain, clientID, clientSecret, audience string) string {
	var path string = "/oauth/token"

	q := url.Values{}
	q.Add("grant_type", "client_credentials")
	q.Add("client_id", clientID)
	q.Add("client_secret", clientSecret)
	q.Add("audience", audience)

	u := &url.URL{
		Scheme:   "https",
		Host:     domain,
		Path:     path,
		RawQuery: q.Encode(),
	}

	return u.String()
}

// runClientCredentialsFlow runs an M2M client credentials flow without opening a browser
func runClientCredentialsFlow(cli *cli, c *management.Client, clientID string, audience string, tenant tenant) (*authutil.TokenResponse, error) {

	var tokenResponse *authutil.TokenResponse

	tokenURL := BuildOauthTokenURL(tenant.Domain, clientID, c.GetClientSecret(), audience)

	// TODO: Check if the audience is valid, and suggest a different client if it is wrong.

	err := ansi.Spinner("Waiting for token", func() error {
		req, _ := http.NewRequest("POST", tokenURL, nil)

		req.Header.Add("content-type", "application/x-www-form-urlencoded")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()

		err = json.NewDecoder(res.Body).Decode(&tokenResponse)
		if err != nil {
			return fmt.Errorf("cannot decode response: %w", err)
		}
		return nil
	})

	return tokenResponse, err
}

// runLoginFlowPreflightChecks checks if we need to make any updates to the
// client being tested in order to log in successfully. If so, it asks the user
// to confirm whether to proceed.
func runLoginFlowPreflightChecks(cli *cli, c *management.Client) (abort bool) {
	cli.renderer.Infof("A browser window will open to begin this client's login flow.")
	cli.renderer.Infof("Once login is complete, you can return to the CLI to view user profile information and tokens.\n")

	// check if the chosen client includes our local callback URL in its
	// allowed list. If not we'll need to add it (after asking the user
	// for permission).
	if !hasLocalCallbackURL(c) {
		cli.renderer.Warnf("The client you are using does not currently allow callbacks to localhost.")
		cli.renderer.Warnf("To complete the login flow the CLI needs to redirect logins to a local server and record the result.\n")
		cli.renderer.Warnf("The client will be modified to update the allowed callback URLs, we'll remove them when done.")
		cli.renderer.Warnf("If you do not wish to modify the client, you can abort now.\n")
	}

	if confirmed := prompt.Confirm("Do you wish to proceed?"); !confirmed {
		return false
	}
	fmt.Fprint(cli.renderer.MessageWriter, "\n")

	return true
}

// runLoginFlow initiates a full user-facing login flow, waits for a response
// and returns the retrieved tokens to the caller when done.
func runLoginFlow(cli *cli, t tenant, c *management.Client, connName, audience, prompt string, scopes []string) (*authutil.TokenResponse, error) {
	var tokenResponse *authutil.TokenResponse

	err := ansi.Spinner("Waiting for login flow to complete", func() error {
		callbackAdded, err := addLocalCallbackURLToClient(cli.api.Client, c)
		if err != nil {
			return err
		}

		// Build a login URL and initiate login in a browser window.
		loginURL, err := authutil.BuildLoginURL(t.Domain, c.GetClientID(), cliLoginTestingCallbackURL, connName, audience, prompt, scopes)
		if err != nil {
			return err
		}

		if err := open.URL(loginURL); err != nil {
			return err
		}

		// launch a HTTP server to wait for the callback to capture the auth
		// code.
		authCode, err := authutil.WaitForBrowserCallback(cliLoginTestingCallbackAddr)
		if err != nil {
			return err
		}

		// once the callback is received, exchange the code for an access
		// token.
		tokenResponse, err = authutil.ExchangeCodeForToken(
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
		if callbackAdded {
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
func hasLocalCallbackURL(client *management.Client) bool {
	for _, rawCallbackURL := range client.Callbacks {
		callbackURL := rawCallbackURL.(string)
		if callbackURL == cliLoginTestingCallbackURL {
			return true
		}
	}

	return false
}

// adds the localhost callback URL to a given client
func addLocalCallbackURLToClient(clientManager auth0.ClientAPI, client *management.Client) (bool, error) {
	for _, rawCallbackURL := range client.Callbacks {
		callbackURL := rawCallbackURL.(string)
		if callbackURL == cliLoginTestingCallbackURL {
			return false, nil
		}
	}

	updatedClient := &management.Client{
		Callbacks: append(client.Callbacks, cliLoginTestingCallbackURL),
	}
	// reflect the changes in the original client instance so when we check it
	// later it has the proper values in Callbacks
	client.Callbacks = updatedClient.Callbacks
	return true, clientManager.Update(client.GetClientID(), updatedClient)
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
