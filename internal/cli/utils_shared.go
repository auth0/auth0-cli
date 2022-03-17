package cli

import (
	"crypto/rand"
	"fmt"
	"strings"

	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth/authutil"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/pkg/browser"
	"github.com/auth0/go-auth0/management"
)

const (
	cliLoginTestingClientName        string = "CLI Login Testing"
	cliLoginTestingClientDescription string = "A client used for testing logins using the Auth0 CLI."
	cliLoginTestingCallbackAddr      string = "localhost:8484"
	cliLoginTestingCallbackURL       string = "http://localhost:8484"
	cliLoginTestingInitiateLoginURI  string = "https://cli.auth0.com"
	cliLoginTestingStateSize         int    = 64
	manageURL                        string = "https://manage.auth0.com"
)

var (
	cliLoginTestingScopes []string = []string{"openid", "profile"}
)

func BuildOauthTokenURL(domain string) string {
	var path string = "/oauth/token"

	u := &url.URL{
		Scheme: "https",
		Host:   domain,
		Path:   path,
	}

	return u.String()
}

func BuildOauthTokenParams(clientID, clientSecret, audience string) url.Values {
	q := url.Values{
		"audience":      {audience},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"grant_type":    {"client_credentials"},
	}
	return q
}

// runClientCredentialsFlow runs an M2M client credentials flow without opening a browser
func runClientCredentialsFlow(cli *cli, c *management.Client, clientID string, audience string, tenant tenant) (*authutil.TokenResponse, error) {

	var tokenResponse *authutil.TokenResponse

	tokenURL := BuildOauthTokenURL(tenant.Domain)
	payload := BuildOauthTokenParams(clientID, c.GetClientSecret(), audience)

	// TODO: Check if the audience is valid, and suggest a different client if it is wrong.

	err := ansi.Spinner("Waiting for token", func() error {
		res, err := http.PostForm(tokenURL, payload)
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

	if !cli.force {
		if confirmed := prompt.Confirm("Do you wish to proceed?"); !confirmed {
			return false
		}
	}

	fmt.Fprint(cli.renderer.MessageWriter, "\n")

	return true
}

// runLoginFlow initiates a full user-facing login flow, waits for a response
// and returns the retrieved tokens to the caller when done.
func runLoginFlow(cli *cli, t tenant, c *management.Client, connName, audience, prompt string, scopes []string, customDomain string) (*authutil.TokenResponse, error) {
	var tokenResponse *authutil.TokenResponse

	err := ansi.Spinner("Waiting for login flow to complete", func() error {
		callbackAdded, err := addLocalCallbackURLToClient(cli.api.Client, c)
		if err != nil {
			return err
		}

		state, err := generateState(cliLoginTestingStateSize)
		if err != nil {
			return err
		}

		domain := t.Domain
		if customDomain != "" {
			domain = customDomain
		}

		// Build a login URL and initiate login in a browser window.
		loginURL, err := authutil.BuildLoginURL(domain, c.GetClientID(), cliLoginTestingCallbackURL, state, connName, audience, prompt, scopes)
		if err != nil {
			return err
		}

		if err := browser.OpenURL(loginURL); err != nil {
			return err
		}

		// launch a HTTP server to wait for the callback to capture the auth
		// code.
		authCode, authState, err := authutil.WaitForBrowserCallback(cliLoginTestingCallbackAddr)
		if err != nil {
			return err
		}

		if state != authState {
			return fmt.Errorf("unexpected auth state")
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
		defer func() {
			if callbackAdded {
				if err := removeLocalCallbackURLFromClient(cli.api.Client, c); err != nil { // TODO: Make it a warning
					cli.renderer.Errorf("Unable to remove callback URL '%s' from client: %s", cliLoginTestingCallbackURL, err)
				}
			}
		}()

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

// adds the localhost callback URL to a given application
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

// generate state parameter value used to mitigate CSRF attacks
// more: https://auth0.com/docs/protocols/state-parameters
func generateState(size int) (string, error) {
	b := make([]byte, size)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

// check if slice contains a string
func containsStr(s []interface{}, u string) bool {
	for _, a := range s {
		if a == u {
			return true
		}
	}
	return false
}

func openManageURL(cli *cli, tenant string, path string) {
	manageTenantURL := formatManageTenantURL(tenant, cli.config)
	if len(manageTenantURL) == 0 || len(path) == 0 {
		cli.renderer.Warnf("Unable to format the correct URL, please ensure you have run 'auth0 login' and try again.")
		return
	}
	if err := browser.OpenURL(fmt.Sprintf("%s%s", manageTenantURL, path)); err != nil {
		cli.renderer.Warnf("Couldn't open the URL, please do it manually: %s.", manageTenantURL)
	}
}

func formatManageTenantURL(tenant string, cfg config) string {
	if len(tenant) == 0 {
		return ""
	}
	// ex: dev-tti06f6y.us.auth0.com
	s := strings.Split(tenant, ".")

	if len(s) < 3 {
		return ""
	}

	var region string
	if len(s) == 3 { // It's a PUS1 tenant, ex: dev-tti06f6y.auth0.com
		region = "us"
	} else {
		region = s[len(s)-3]
	}

	tenantName := cfg.Tenants[tenant].Name
	if len(tenantName) == 0 {
		return ""
	}
	return fmt.Sprintf("%s/dashboard/%s/%s/",
		manageURL,
		region,
		tenantName,
	)
}
