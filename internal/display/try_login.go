package display

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth"
	"github.com/auth0/auth0-cli/internal/auth0"
)

type userInfoAndTokens struct {
	UserInfo *auth.UserInfo      `json:"user_info"`
	Tokens   *auth.TokenResponse `json:"tokens"`
}

func isNotZero(v interface{}) bool {
	t := reflect.TypeOf(v)
	if !t.Comparable() {
		// assume non-zero if error
		return true
	}
	return v != reflect.Zero(t).Interface()
}

func (r *Renderer) TryLogin(u *auth.UserInfo, t *auth.TokenResponse) {
	r.Heading(ansi.Bold(auth0.StringValue(u.Sub)), "/userinfo\n")

	switch r.Format {
	case OutputFormatJSON:
		out := &userInfoAndTokens{UserInfo: u, Tokens: t}
		b, err := json.MarshalIndent(out, "", "    ")
		if err != nil {
			r.Errorf("couldn't marshal results as JSON: %v", err)
			return
		}
		fmt.Fprint(r.ResultWriter, string(b))
	default:
		rows := make([][]string, 0)

		// TODO: make this less verbose
		if isNotZero(u.Name) {
			rows = append(rows, []string{ansi.Faint("Name"), auth0.StringValue(u.Name)})
		}
		if isNotZero(u.GivenName) {
			rows = append(rows, []string{ansi.Faint("GivenName"), auth0.StringValue(u.GivenName)})
		}
		if isNotZero(u.MiddleName) {
			rows = append(rows, []string{ansi.Faint("MiddleName"), auth0.StringValue(u.MiddleName)})
		}
		if isNotZero(u.FamilyName) {
			rows = append(rows, []string{ansi.Faint("FamilyName"), auth0.StringValue(u.FamilyName)})
		}
		if isNotZero(u.Nickname) {
			rows = append(rows, []string{ansi.Faint("Nickname"), auth0.StringValue(u.Nickname)})
		}
		if isNotZero(u.PreferredUsername) {
			rows = append(rows, []string{ansi.Faint("PreferredUsername"), auth0.StringValue(u.PreferredUsername)})
		}
		if isNotZero(u.Profile) {
			rows = append(rows, []string{ansi.Faint("Profile"), auth0.StringValue(u.Profile)})
		}
		if isNotZero(u.Picture) {
			rows = append(rows, []string{ansi.Faint("Picture"), auth0.StringValue(u.Picture)})
		}
		if isNotZero(u.Website) {
			rows = append(rows, []string{ansi.Faint("Website"), auth0.StringValue(u.Website)})
		}
		if isNotZero(u.PhoneNumber) {
			rows = append(rows, []string{ansi.Faint("PhoneNumber"), auth0.StringValue(u.PhoneNumber)})
		}
		if isNotZero(u.PhoneVerified) {
			rows = append(rows, []string{ansi.Faint("PhoneVerified"), strconv.FormatBool(auth0.BoolValue(u.PhoneVerified))})
		}
		if isNotZero(u.Email) {
			rows = append(rows, []string{ansi.Faint("Email"), auth0.StringValue(u.Email)})
		}
		if isNotZero(u.EmailVerified) {
			rows = append(rows, []string{ansi.Faint("EmailVerified"), strconv.FormatBool(auth0.BoolValue(u.EmailVerified))})
		}
		if isNotZero(u.Gender) {
			rows = append(rows, []string{ansi.Faint("Gender"), auth0.StringValue(u.Gender)})
		}
		if isNotZero(u.BirthDate) {
			rows = append(rows, []string{ansi.Faint("BirthDate"), auth0.StringValue(u.BirthDate)})
		}
		if isNotZero(u.ZoneInfo) {
			rows = append(rows, []string{ansi.Faint("ZoneInfo"), auth0.StringValue(u.ZoneInfo)})
		}
		if isNotZero(u.Locale) {
			rows = append(rows, []string{ansi.Faint("Locale"), auth0.StringValue(u.Locale)})
		}
		if isNotZero(u.UpdatedAt) {
			rows = append(rows, []string{ansi.Faint("UpdatedAt"), auth0.TimeValue(u.UpdatedAt).Format(time.RFC3339)})
		}
		if isNotZero(t.AccessToken) {
			rows = append(rows, []string{ansi.Faint("AccessToken"), t.AccessToken})
		}
		if isNotZero(t.RefreshToken) {
			rows = append(rows, []string{ansi.Faint("RefreshToken"), t.RefreshToken})
		}
		// TODO: This is a long string and it messes up formatting when printed
		// to the table, so need to come back to this one and fix it later.
		// if isNotZero(t.IDToken) {
		// 	rows = append(rows, []string{ansi.Faint("IDToken"), t.IDToken})
		// }
		if isNotZero(t.TokenType) {
			rows = append(rows, []string{ansi.Faint("TokenType"), t.TokenType})
		}
		if isNotZero(t.ExpiresIn) {
			rows = append(rows, []string{ansi.Faint("ExpiresIn"), strconv.FormatInt(t.ExpiresIn, 10)})
		}

		tableHeader := []string{"Field", "Value"}
		writeTable(r.ResultWriter, tableHeader, rows)
	}
}
