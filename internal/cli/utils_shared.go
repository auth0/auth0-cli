package cli

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/auth0/go-auth0/management"
	"github.com/pkg/browser"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth/authutil"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/config"
	"github.com/auth0/auth0-cli/internal/prompt"
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

var cliLoginTestingScopes = []string{"openid", "profile"}

func BuildOauthTokenURL(domain string) string {
	var path = "/oauth/token"

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

// runClientCredentialsFlow runs an M2M client
// credentials flow without opening a browser.
func runClientCredentialsFlow(
	ctx context.Context,
	cli *cli,
	client *management.Client,
	audience string,
	tenantDomain string,
) (*authutil.TokenResponse, error) {
	if err := checkClientIsAuthorizedForAPI(ctx, cli, client, audience); err != nil {
		return nil, err
	}

	tokenURL := BuildOauthTokenURL(tenantDomain)
	payload := BuildOauthTokenParams(client.GetClientID(), client.GetClientSecret(), audience)

	var tokenResponse *authutil.TokenResponse
	err := ansi.Spinner("Waiting for token", func() error {
		response, err := http.PostForm(tokenURL, payload)
		if err != nil {
			return err
		}
		defer func() {
			_ = response.Body.Close()
		}()

		if err = json.NewDecoder(response.Body).Decode(&tokenResponse); err != nil {
			return fmt.Errorf("failed to decode the response: %w", err)
		}

		return nil
	})

	return tokenResponse, err
}

// runLoginFlowPreflightChecks checks if we need to make any updates
// to the client being tested in order to log in successfully.
// If so, it asks the user to confirm whether to proceed.
func runLoginFlowPreflightChecks(cli *cli, c *management.Client) (abort bool) {
	if !cli.noInput {
		cli.renderer.Infof("A browser window needs to be opened to complete this client's login flow.")
		cli.renderer.Infof("Once login is complete, you can return to the CLI to view user profile information and tokens.")
		cli.renderer.Newline()
	}

	// Check if the chosen client includes our local callback URL in its allowed list.
	// If not we'll need to add it (after asking the user for permission).
	if !hasLocalCallbackURL(c) {
		cli.renderer.Warnf("The client you are using does not currently allow callbacks to localhost.")
		cli.renderer.Warnf("To complete the login flow the CLI needs to redirect logins to a local server and record the result.\n")
		cli.renderer.Warnf("The client will be modified to update the allowed callback URLs, we'll remove them when done.")
		cli.renderer.Warnf("If you do not wish to modify the client, you can abort now.")
		cli.renderer.Newline()
	}

	if !cli.force && !cli.noInput {
		if confirmed := prompt.Confirm("Do you wish to proceed?"); !confirmed {
			return false
		}
	}

	cli.renderer.Newline()

	return true
}

// runLoginFlow initiates a full user-facing login flow, waits for a response
// and returns the retrieved tokens to the caller when done.
func runLoginFlow(ctx context.Context, cli *cli, c *management.Client, connName, audience, prompt string, scopes []string, customDomain string, customParams map[string]string) (*authutil.TokenResponse, error) {
	var tokenResponse *authutil.TokenResponse

	err := ansi.Spinner("Waiting for login flow to complete", func() error {
		callbackAdded, err := addLocalCallbackURLToClient(ctx, cli.api.Client, c)
		if err != nil {
			return err
		}

		state, err := generateState(cliLoginTestingStateSize)
		if err != nil {
			return err
		}

		domain := cli.tenant
		if customDomain != "" {
			domain = customDomain
		}

		// Build a login URL and initiate login in a browser window.
		loginURL, err := authutil.BuildLoginURL(domain, c.GetClientID(), cliLoginTestingCallbackURL, state, connName, audience, prompt, scopes, customParams)
		if err != nil {
			return err
		}

		if cli.noInput {
			cli.renderer.Infof("Open the following URL in a browser: %s\n", loginURL)
		} else {
			if err := browser.OpenURL(loginURL); err != nil {
				return err
			}
		}

		// Launch a HTTP server to wait for the callback to capture the auth
		// code.
		authCode, authState, err := authutil.WaitForBrowserCallback(cliLoginTestingCallbackAddr)
		if err != nil {
			return err
		}

		if state != authState {
			return fmt.Errorf("unexpected auth state")
		}

		// Once the callback is received, exchange the code for an access
		// token.
		tokenResponse, err = authutil.ExchangeCodeForToken(
			http.DefaultClient,
			cli.tenant,
			c.GetClientID(),
			c.GetClientSecret(),
			authCode,
			cliLoginTestingCallbackURL,
		)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		// If we added the local callback URL to the client then we need to
		// remove it when we're done.
		defer func() {
			if callbackAdded {
				if err := removeLocalCallbackURLFromClient(ctx, cli.api.Client, c); err != nil { // TODO: Make it a warning.
					cli.renderer.Errorf("failed to remove callback URL '%s' from client: %s", cliLoginTestingCallbackURL, err)
				}
			}
		}()

		return nil
	})

	return tokenResponse, err
}

// check if a client is already configured with our local callback URL.
func hasLocalCallbackURL(client *management.Client) bool {
	for _, callbackURL := range client.GetCallbacks() {
		if callbackURL == cliLoginTestingCallbackURL {
			return true
		}
	}

	return false
}

// adds the localhost callback URL to a given application.
func addLocalCallbackURLToClient(ctx context.Context, clientManager auth0.ClientAPI, client *management.Client) (bool, error) {
	for _, callbackURL := range client.GetCallbacks() {
		if callbackURL == cliLoginTestingCallbackURL {
			return false, nil
		}
	}

	callbacks := append(client.GetCallbacks(), cliLoginTestingCallbackURL)
	updatedClient := &management.Client{
		Callbacks: &callbacks,
	}
	// Reflect the changes in the original client instance so when we check it
	// later it has the proper values in Callbacks.
	client.Callbacks = updatedClient.Callbacks
	return true, clientManager.Update(ctx, client.GetClientID(), updatedClient)
}

func removeLocalCallbackURLFromClient(ctx context.Context, clientManager auth0.ClientAPI, client *management.Client) error {
	callbacks := make([]string, 0)
	for _, callbackURL := range client.GetCallbacks() {
		if callbackURL != cliLoginTestingCallbackURL {
			callbacks = append(callbacks, callbackURL)
		}
	}

	// No callback URLs to remove, so don't attempt to do so.
	if len(client.GetCallbacks()) == len(callbacks) {
		return nil
	}

	// Can't update a client to have 0 callback URLs, so don't attempt it.
	if len(callbacks) == 0 {
		return nil
	}

	updatedClient := &management.Client{
		Callbacks: &callbacks,
	}
	return clientManager.Update(ctx, client.GetClientID(), updatedClient)
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

// check if slice contains a string.
func containsStr(s []string, u string) bool {
	for _, a := range s {
		if a == u {
			return true
		}
	}
	return false
}

func openManageURL(cli *cli, tenant string, path string) {
	manageTenantURL := formatManageTenantURL(tenant, &cli.Config)
	if len(manageTenantURL) == 0 || len(path) == 0 {
		cli.renderer.Warnf("Failed to format the correct URL, please ensure you have run 'auth0 login' and try again.")
		return
	}

	settingsURL := fmt.Sprintf("%s%s", manageTenantURL, path)

	if cli.noInput {
		cli.renderer.Infof("Open the following URL in a browser: %s", settingsURL)
		return
	}

	if err := browser.OpenURL(settingsURL); err != nil {
		cli.renderer.Warnf("Couldn't open the URL, please do it manually: %s", settingsURL)
	}
}

func formatManageTenantURL(tenant string, cfg *config.Config) string {
	if len(tenant) == 0 {
		return ""
	}
	// ex: dev-tti06f6y.us.auth0.com
	s := strings.Split(tenant, ".")

	if len(s) < 3 {
		return ""
	}

	var region string
	if len(s) == 3 { // It's a PUS1 tenant, ex: dev-tti06f6y.auth0.com.
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

// stringPtr converts a pointer of string-derived type to pointer of string.
// Returns nil if the input pointer is nil.
func stringPtr[strPtrType ~string](ptr *strPtrType) *string {
	if ptr == nil {
		return nil
	}
	s := string(*ptr)
	return &s
}

func parseFlexibleDate(input string) (string, error) {
	now := time.Now().UTC()
	input = strings.TrimSpace(input)

	// Try full RFC3339 first.
	if t, err := time.Parse(time.RFC3339, input); err == nil {
		return t.Format(time.RFC3339), nil
	}

	// Try "YYYY-MM-DD" date only.
	if t, err := time.Parse("2006-01-02", input); err == nil {
		return t.UTC().Format(time.RFC3339), nil
	}

	input = strings.ToLower(input)
	// Keywords.
	switch input {
	case "yesterday":
		return now.AddDate(0, 0, -1).Format(time.RFC3339), nil
	case "today":
		t := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		return t.Format(time.RFC3339), nil
	}

	if strings.HasSuffix(input, "d") {
		dayStr := strings.TrimSuffix(input, "d")
		if n, err := strconv.Atoi(dayStr); err == nil {
			return now.AddDate(0, 0, n).Format(time.RFC3339), nil
		}
	}

	if strings.HasSuffix(input, "h") {
		hourStr := strings.TrimSuffix(input, "h")
		if n, err := strconv.Atoi(hourStr); err == nil {
			return now.Add(time.Duration(n) * time.Hour).Format(time.RFC3339), nil
		}
	}

	return "", fmt.Errorf("invalid date format: use RFC3339, 'YYYY-MM-DD', or formats like 'yesterday', '-2d'")
}
