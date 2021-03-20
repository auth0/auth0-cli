package display

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth/authutil"
)

type userInfoAndTokens struct {
	UserInfo *authutil.UserInfo      `json:"user_info"`
	Tokens   *authutil.TokenResponse `json:"tokens"`
}

func isNotZero(v interface{}) bool {
	t := reflect.TypeOf(v)
	if !t.Comparable() {
		// assume non-zero if error
		return true
	}
	return v != reflect.Zero(t).Interface()
}

func (r *Renderer) TryLogin(u *authutil.UserInfo, t *authutil.TokenResponse) {
	r.Heading(ansi.Bold(r.Tenant), "/userinfo\n")

	out := &userInfoAndTokens{UserInfo: u, Tokens: t}
	b, err := json.MarshalIndent(out, "", "    ")
	if err != nil {
		r.Errorf("couldn't marshal results as JSON: %v", err)
		return
	}
	jsonStr := string(b)

	switch r.Format {
	case OutputFormatJSON:
		fmt.Fprint(r.ResultWriter, jsonStr)
	default:
		fmt.Fprintln(r.ResultWriter, ansi.ColorizeJSON(jsonStr, false, os.Stdout))
	}
}
