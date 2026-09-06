package localserver

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
)

func (s *Server) buildMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", s.handleDiscovery)
	mux.HandleFunc("/.well-known/jwks.json", s.handleJWKS)
	mux.HandleFunc("/authorize", s.handleAuthorize)
	mux.HandleFunc("/oauth/token", s.handleToken)
	mux.HandleFunc("/userinfo", s.handleUserInfo)
	mux.HandleFunc("/v2/logout", s.handleLogout)
	mux.HandleFunc("/__test/last-code", s.handleLastCode)
	return mux
}

var loginPageTmpl = template.Must(template.New("login").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Auth0 Local — Sign In</title>
<style>
  *{box-sizing:border-box;margin:0;padding:0}
  body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;
       background:#f5f5f5;display:flex;align-items:center;justify-content:center;min-height:100vh}
  .card{background:#fff;border-radius:8px;box-shadow:0 2px 12px rgba(0,0,0,.12);
        padding:40px 36px;width:100%;max-width:360px}
  .logo{text-align:center;margin-bottom:24px}
  .logo svg{width:48px;height:48px}
  h1{font-size:20px;font-weight:600;text-align:center;color:#1c1e21;margin-bottom:6px}
  .subtitle{font-size:13px;text-align:center;color:#6b7280;margin-bottom:28px}
  .badge{display:inline-block;background:#fef3c7;color:#92400e;font-size:11px;
         font-weight:600;padding:2px 8px;border-radius:12px;margin-bottom:20px;
         text-align:center;width:100%}
  label{font-size:13px;font-weight:500;color:#374151;display:block;margin-bottom:4px}
  input[type=email],input[type=text]{width:100%;padding:9px 12px;border:1px solid #d1d5db;
    border-radius:6px;font-size:14px;color:#111;outline:none;transition:border .15s}
  input[type=email]:focus,input[type=text]:focus{border-color:#635dff;box-shadow:0 0 0 3px rgba(99,93,255,.15)}
  .field{margin-bottom:18px}
  button{width:100%;padding:10px;background:#635dff;color:#fff;border:none;
         border-radius:6px;font-size:15px;font-weight:600;cursor:pointer;transition:background .15s}
  button:hover{background:#4f46e5}
  .hint{font-size:11px;color:#9ca3af;text-align:center;margin-top:16px}
</style>
</head>
<body>
<div class="card">
  <div class="logo">
    <svg viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg">
      <circle cx="32" cy="32" r="32" fill="#635dff"/>
      <path d="M32 14l14 8v16l-14 8-14-8V22z" fill="white" opacity=".9"/>
    </svg>
  </div>
  <h1>Sign in</h1>
  <p class="subtitle">Local Auth0 Identity Server</p>
  <div class="badge">⚡ Local mode — no real Auth0 account needed</div>
  <form method="POST" action="/authorize">
    <input type="hidden" name="req_id" value="{{.ReqID}}">
    <div class="field">
      <label for="email">Email address</label>
      <input type="email" id="email" name="email" value="{{.Email}}" required autocomplete="email">
    </div>
    <button type="submit">Continue</button>
  </form>
  <p class="hint">Any email works. No password required in local mode.</p>
</div>
</body>
</html>`))

type loginPageData struct {
	ReqID string
	Email string
}

type discoveryDoc struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	UserinfoEndpoint                  string   `json:"userinfo_endpoint"`
	JwksURI                           string   `json:"jwks_uri"`
	EndSessionEndpoint                string   `json:"end_session_endpoint"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	SubjectTypesSupported             []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported  []string `json:"id_token_signing_alg_values_supported"`
	ScopesSupported                   []string `json:"scopes_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	ClaimsSupported                   []string `json:"claims_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
}

func (s *Server) handleDiscovery(w http.ResponseWriter, _ *http.Request) {
	jsonResponse(w, discoveryDoc{
		Issuer:                            s.issuer + "/",
		AuthorizationEndpoint:             s.issuer + "/authorize",
		TokenEndpoint:                     s.issuer + "/oauth/token",
		UserinfoEndpoint:                  s.issuer + "/userinfo",
		JwksURI:                           s.issuer + "/.well-known/jwks.json",
		EndSessionEndpoint:                s.issuer + "/v2/logout",
		ResponseTypesSupported:            []string{"code"},
		SubjectTypesSupported:             []string{"public"},
		IDTokenSigningAlgValuesSupported:  []string{"RS256"},
		ScopesSupported:                   []string{"openid", "profile", "email"},
		TokenEndpointAuthMethodsSupported: []string{"none", "client_secret_post"},
		ClaimsSupported:                   []string{"sub", "email", "name", "email_verified", "iss", "aud", "exp", "iat", "nonce"},
		CodeChallengeMethodsSupported:     []string{"S256", "plain"},
	}, http.StatusOK)
}

func (s *Server) handleJWKS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(s.keys.jwksJSON)
}

// handleAuthorize implements the OIDC authorization endpoint.
//
// GET  — validates params, stores the pending request, renders the login page.
// POST — processes the submitted form, issues an auth code, redirects to redirect_uri.
func (s *Server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.showLoginPage(w, r)
	case http.MethodPost:
		s.completeLogin(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) showLoginPage(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	clientID := q.Get("client_id")
	redirectURI := q.Get("redirect_uri")
	responseType := q.Get("response_type")

	if responseType != "code" {
		oauthError(w, "unsupported_response_type", "only response_type=code is supported", http.StatusBadRequest)
		return
	}
	if clientID == "" || redirectURI == "" {
		oauthError(w, "invalid_request", "client_id and redirect_uri are required", http.StatusBadRequest)
		return
	}

	reqID := uuid.New().String()
	s.store.save(reqID, authRequest{
		clientID:    clientID,
		redirectURI: redirectURI,
		nonce:       q.Get("nonce"),
		scope:       q.Get("scope"),
		challenge:   q.Get("code_challenge"),
		method:      q.Get("code_challenge_method"),
		state:       q.Get("state"),
	})

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = loginPageTmpl.Execute(w, loginPageData{
		ReqID: reqID,
		Email: localUserEmail,
	})
}

func (s *Server) completeLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	reqID := r.FormValue("req_id")
	req, ok := s.store.pop(reqID)
	if !ok {
		http.Error(w, "login session expired or invalid — please try again", http.StatusBadRequest)
		return
	}

	target, err := url.Parse(req.redirectURI)
	if err != nil {
		http.Error(w, "invalid redirect_uri", http.StatusBadRequest)
		return
	}

	code := uuid.New().String()
	s.store.save(code, req)

	params := url.Values{"code": {code}}
	if req.state != "" {
		params.Set("state", req.state)
	}
	target.RawQuery = params.Encode()
	http.Redirect(w, r, target.String(), http.StatusFound)
}

type tokenResponseBody struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token,omitempty"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope,omitempty"`
}

func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	fmt.Println("handleToken called")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		oauthError(w, "invalid_request", "could not parse form body", http.StatusBadRequest)
		return
	}

	if r.FormValue("grant_type") != "authorization_code" {
		oauthError(w, "unsupported_grant_type", "only authorization_code is supported", http.StatusBadRequest)
		return
	}

	code := r.FormValue("code")
	req, ok := s.store.pop(code)
	if !ok {
		oauthError(w, "invalid_grant", "authorization code not found or already used", http.StatusBadRequest)
		return
	}

	if req.challenge != "" && !verifyPKCE(req.method, req.challenge, r.FormValue("code_verifier")) {
		oauthError(w, "invalid_grant", "PKCE verification failed", http.StatusBadRequest)
		return
	}

	now := time.Now()
	exp := now.Add(tokenTTL)

	idToken, err := s.issueIDToken(req, now, exp)
	if err != nil {
		oauthError(w, "server_error", fmt.Sprintf("id_token issue failed: %v", err), http.StatusInternalServerError)
		return
	}

	accessToken, err := s.issueAccessToken(req, now, exp)
	if err != nil {
		oauthError(w, "server_error", fmt.Sprintf("access_token issue failed: %v", err), http.StatusInternalServerError)
		return
	}

	fmt.Println(accessToken)

	jsonResponse(w, tokenResponseBody{
		AccessToken: accessToken,
		IDToken:     idToken,
		TokenType:   "Bearer",
		ExpiresIn:   int(exp.Sub(now).Seconds()),
		Scope:       req.scope,
	}, http.StatusOK)
}

func (s *Server) handleUserInfo(w http.ResponseWriter, _ *http.Request) {
	jsonResponse(w, map[string]interface{}{
		"sub":            localUserSub,
		"email":          localUserEmail,
		"name":           localUserName,
		"email_verified": true,
		"local_only":     true,
	}, http.StatusOK)
}

var loggedOutTmpl = template.Must(template.New("loggedout").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Logged out — Local Auth0</title>
<style>
  *{box-sizing:border-box;margin:0;padding:0}
  body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;
       background:#f5f5f5;display:flex;align-items:center;justify-content:center;min-height:100vh}
  .card{background:#fff;border-radius:8px;box-shadow:0 2px 12px rgba(0,0,0,.12);
        padding:40px 36px;width:100%;max-width:360px;text-align:center}
  .icon{font-size:40px;margin-bottom:16px}
  h1{font-size:20px;font-weight:600;color:#1c1e21;margin-bottom:8px}
  p{font-size:13px;color:#6b7280;margin-bottom:24px;line-height:1.5}
  a{display:inline-block;padding:10px 24px;background:#635dff;color:#fff;
    border-radius:6px;font-size:14px;font-weight:600;text-decoration:none}
  a:hover{background:#4f46e5}
  .hint{font-size:11px;color:#9ca3af;margin-top:16px}
</style>
</head>
<body>
<div class="card">
  <div class="icon">👋</div>
  <h1>You've been logged out</h1>
  <p>Your local session has ended.<br>Navigate back to your app to sign in again.</p>
  {{if .AppURL}}<a href="{{.AppURL}}">Back to app</a>{{end}}
  <p class="hint">Local Auth0 server running on port {{.Port}}</p>
</div>
</body>
</html>`))

// handleLogout implements both Auth0's /v2/logout and the OIDC end_session_endpoint.
// Priority for the redirect target:
//  1. returnTo query param (Auth0 SDKs: nextjs-auth0, auth0-spa-js, auth0-python, etc.)
//  2. post_logout_redirect_uri query param (OIDC middleware: ASP.NET Core, Spring Boot)
//  3. Origin of the Referer header (best-effort when the app omits the param)
//  4. Show a logged-out confirmation page (no redirect — works for any port)
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	target := q.Get("returnTo")
	if target == "" {
		target = q.Get("post_logout_redirect_uri")
	}
	if target == "" {
		if ref := r.Referer(); ref != "" {
			if u, err := url.Parse(ref); err == nil {
				target = u.Scheme + "://" + u.Host
			}
		}
	}

	if target != "" {
		http.Redirect(w, r, target, http.StatusFound)
		return
	}

	// No redirect target available — render a generic logged-out page.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = loggedOutTmpl.Execute(w, map[string]interface{}{
		"AppURL": "",
		"Port":   s.port,
	})
}

// handleLastCode returns the most recent captured OTP code in JSON form.
// This endpoint mirrors Firebase's Auth Emulator pattern so agents can read
// a verification code without needing a terminal or inbox.
func (s *Server) handleLastCode(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	lc := s.lastCode
	s.mu.Unlock()

	if lc == nil {
		jsonResponse(w, map[string]interface{}{
			"code":    nil,
			"message": "no verification code captured yet",
		}, http.StatusOK)
		return
	}
	jsonResponse(w, lc, http.StatusOK)
}

func jsonResponse(w http.ResponseWriter, v interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func oauthError(w http.ResponseWriter, errCode, description string, status int) {
	jsonResponse(w, map[string]string{
		"error":             errCode,
		"error_description": description,
	}, status)
}
