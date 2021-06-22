package display

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/auth0/auth0-cli/internal/auth/authutil"
	"github.com/auth0/auth0-cli/internal/auth0"
	"gopkg.in/auth0.v5/management"
)

func (r *Renderer) GetToken(c *management.Client, t *authutil.TokenResponse) {
	// pass the access token to the pipe and exit
	if isOutputPiped() {
		fmt.Fprint(r.ResultWriter, t.AccessToken)
		return
	}

	fmt.Fprint(r.ResultWriter, "\n")
	r.Heading(fmt.Sprintf("token for %s", auth0.StringValue(c.Name)))

	switch r.Format {
	case OutputFormatJSON:
		b, err := json.MarshalIndent(t, "", "    ")
		if err != nil {
			r.Errorf("couldn't marshal results as JSON: %v", err)
			return
		}
		fmt.Fprint(r.ResultWriter, string(b))
	default:
		rows := make([][]string, 0)

		if isNotZero(t.AccessToken) {
			rows = append(rows, []string{"ACCESS TOKEN", t.AccessToken})
		}
		if isNotZero(t.RefreshToken) {
			rows = append(rows, []string{"REFRESH TOKEN", t.RefreshToken})
		}
		// TODO: This is a long string and it messes up formatting when printed
		// to the table, so need to come back to this one and fix it later.
		// if isNotZero(t.IDToken) {
		// 	rows = append(rows, []string{ansi.Faint("IDToken"), t.IDToken})
		// }
		if isNotZero(t.TokenType) {
			rows = append(rows, []string{"TOKEN TYPE", t.TokenType})
		}
		if isNotZero(t.ExpiresIn) {
			rows = append(rows, []string{"EXPIRES IN", strconv.FormatInt(t.ExpiresIn, 10)})
		}

		tableHeader := []string{"", ""}
		writeTable(r.ResultWriter, tableHeader, rows)
	}
}
