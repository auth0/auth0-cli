package display

import (
	"fmt"
	"strconv"

	"github.com/auth0/go-auth0/management"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth/authutil"
)

type userInfoAndTokens struct {
	UserInfo *authutil.UserInfo      `json:"user_info"`
	Tokens   *authutil.TokenResponse `json:"tokens"`
}

func (r *Renderer) TestLogin(u *authutil.UserInfo, t *authutil.TokenResponse, clientID string) {
	r.Heading("user metadata and token")

	data := &userInfoAndTokens{UserInfo: u, Tokens: t}
	r.JSONResult(data)

	r.Newline()
	r.Newline()
	r.Infof(
		"%s Login flow is working! If this is the first time you are testing the login flow for this "+
			"application, consider downloading and running a quickstart next by running %s",
		ansi.Faint("Hint:"),
		fmt.Sprintf("`auth0 quickstarts download %s`", clientID),
	)
}

func (r *Renderer) TestToken(client *management.Client, t *authutil.TokenResponse) {
	r.Heading(fmt.Sprintf("token for %s", ansi.Bold(client.GetName())))

	switch r.Format {
	case OutputFormatJSON:
		r.JSONResult(t)
	case OutputFormatJSONCompact:
		r.JSONCompactResult(t)
	default:
		if t.TokenType != "" {
			r.Output("  TOKEN    TYPE   " + t.TokenType)
			r.Newline()
		}
		if t.ExpiresIn != 0 {
			r.Output("  EXPIRES    IN   " + strconv.FormatInt(t.ExpiresIn/60, 10) + " minute(s)")
			r.Newline()
		}
		if t.RefreshToken != "" {
			r.Output("  REFRESH TOKEN   " + t.RefreshToken)
			r.Newline()
		}
		if t.IDToken != "" {
			r.Output("  ID      TOKEN   " + t.IDToken)
			r.Newline()
		}
		if t.AccessToken != "" {
			r.Output("  ACCESS  TOKEN   " + t.AccessToken)
			r.Newline()
		}
	}
}
