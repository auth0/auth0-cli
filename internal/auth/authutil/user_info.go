package authutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"time"
)

// UserInfo contains profile information for a given OIDC user.
type UserInfo struct {
	Sub               *string    `json:"sub,omitempty"`
	Name              *string    `json:"name,omitempty"`
	GivenName         *string    `json:"given_name,omitempty"`
	MiddleName        *string    `json:"middle_name,omitempty"`
	FamilyName        *string    `json:"family_name,omitempty"`
	Nickname          *string    `json:"nickname,omitempty"`
	PreferredUsername *string    `json:"preferred_username,omitempty"`
	Profile           *string    `json:"profile,omitempty"`
	Picture           *string    `json:"picture,omitempty"`
	Website           *string    `json:"website,omitempty"`
	PhoneNumber       *string    `json:"phone_number,omitempty"`
	PhoneVerified     *bool      `json:"phone_verified,omitempty"`
	Email             *string    `json:"email,omitempty"`
	EmailVerified     *bool      `json:"email_verified,omitempty"`
	Gender            *string    `json:"gender,omitempty"`
	BirthDate         *string    `json:"birthdate,omitempty"`
	ZoneInfo          *string    `json:"zoneinfo,omitempty"`
	Locale            *string    `json:"locale,omitempty"`
	UpdatedAt         *time.Time `json:"updated_at,omitempty"`
}

// UnmarshalJSON is a custom deserializer for the UserInfo type.
// A custom solution is necessary due to possible inconsistencies in value types.
func (u *UserInfo) UnmarshalJSON(b []byte) error {
	type userInfo UserInfo
	type userAlias struct {
		*userInfo
		RawEmailVerified interface{} `json:"email_verified,omitempty"`
	}

	alias := &userAlias{(*userInfo)(u), nil}

	err := json.Unmarshal(b, alias)
	if err != nil {
		return err
	}

	if alias.RawEmailVerified != nil {
		var emailVerified bool
		switch rawEmailVerified := alias.RawEmailVerified.(type) {
		case bool:
			emailVerified = rawEmailVerified
		case string:
			emailVerified, err = strconv.ParseBool(rawEmailVerified)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("email_verified field expected to be bool or string, got: %s", reflect.TypeOf(rawEmailVerified))
		}
		alias.EmailVerified = &emailVerified
	}

	return nil
}

// FetchUserInfo fetches and parses user information with the provided access token.
func FetchUserInfo(httpClient *http.Client, baseDomain, token string) (*UserInfo, error) {
	endpoint := url.URL{Scheme: "https", Host: baseDomain, Path: "/userinfo"}

	req, err := http.NewRequest("GET", endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("unable to exchange code for token: %w", err)
	}
	req.Header.Set("authorization", fmt.Sprintf("Bearer %s", token))

	res, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to exchange code for token: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unable to fetch user info: %s", res.Status)
	}

	var u *UserInfo
	err = json.NewDecoder(res.Body).Decode(&u)
	if err != nil {
		return nil, fmt.Errorf("cannot decode response: %w", err)
	}

	return u, nil
}
