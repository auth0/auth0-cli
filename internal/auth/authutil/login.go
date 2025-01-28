package authutil

import (
	"net/url"
	"strings"
)

// BuildLoginURL constructs a URL + query string that can be used to
// initiate a user-facing login-flow from the CLI.
func BuildLoginURL(domain, clientID, callbackURL, state, connectionName, audience, prompt string, scopes []string, customParams map[string]string) (string, error) {
	q := url.Values{}
	q.Add("client_id", clientID)
	q.Add("response_type", "code")
	q.Add("redirect_uri", callbackURL)
	q.Add("state", state)

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

	if len(customParams) > 0 {
		for k, v := range customParams {
			q.Add(k, v)
		}
	}

	u := &url.URL{
		Scheme:   "https",
		Host:     domain,
		Path:     "/authorize",
		RawQuery: q.Encode(),
	}

	return u.String(), nil
}
