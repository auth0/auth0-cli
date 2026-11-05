package setup

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/poppy/config"
)

func TestCommand(t *testing.T) {
	tests := []struct {
		name string
		in   Inputs
		opts CommandOptions
		want string
	}{
		{
			name: "the flags used",
			in: Inputs{
				CompanyDomain: "shop.example", APIs: []string{shopAPI}, App: "Shop Web", Out: config.DefaultFileName,
				APIURLs: map[string]string{shopAPI: shopAPI + "/openapi.json"},
			},
			want: `auth0 poppy setup --company-domain shop.example --api https://api.shop.example --api-url https://api.shop.example=https://api.shop.example/openapi.json --app "Shop Web"`,
		},
		{
			name: "dry run is never in it, yes only when given",
			in:   Inputs{CompanyDomain: "shop.example", APIs: []string{shopAPI}, App: "app1"},
			opts: CommandOptions{Yes: true, SkipDoctor: true},
			want: `auth0 poppy setup --company-domain shop.example --api https://api.shop.example --app app1 --skip-doctor --yes`,
		},
		{
			name: "repeated flags, in a fixed order",
			in: Inputs{
				CompanyDomain: "shop.example", APIs: []string{shopAPI, "rs_pay"}, App: "app1",
				Scopes: map[string]string{"zeta": "Last one.", "alpha": "First one."},
				Hints:  map[string]config.Hints{"checkout": {Changes: "irreversible", PersonalData: true}, "browse": {Changes: "none"}},
			},
			want: `auth0 poppy setup --company-domain shop.example --api https://api.shop.example --api rs_pay --app app1` +
				` --scope "alpha=First one." --scope "zeta=Last one."` +
				` --hint browse=changes:none,personal_data:false --hint checkout=changes:irreversible,personal_data:true`,
		},
		{
			name: "values that need quoting",
			in: Inputs{
				CompanyDomain: "shop.example", APIs: []string{shopAPI}, App: `Shop's "Web" $app`, Name: "Shop Inc",
				Scopes: map[string]string{"a": "it's $5"}, LoginURI: "https://shop.example/login?next=a b", Out: "./config/my poppy.json",
			},
			want: `auth0 poppy setup --company-domain shop.example --api https://api.shop.example --app 'Shop'\''s "Web" $app'` +
				` --scope 'a=it'\''s $5' --name "Shop Inc" --login-uri "https://shop.example/login?next=a b" --out "./config/my poppy.json"`,
		},
		{
			name: "control characters are escaped inside the quotes, never cut",
			in: Inputs{
				CompanyDomain: "shop.example", APIs: []string{shopAPI}, App: "a",
				Scopes: map[string]string{"x": "tab\there and a long " + strings.Repeat("a", 400) + " tail"},
			},
			want: `auth0 poppy setup --company-domain shop.example --api https://api.shop.example --app a` +
				` --scope 'x=tab\x09here and a long ` + strings.Repeat("a", 400) + ` tail'`,
		},
		{
			name: "guest scopes named",
			in:   Inputs{CompanyDomain: "shop.example", APIs: []string{shopAPI}, App: "a", GuestAPIAccess: true, GuestScopeNames: []string{"orders:read", "checkout"}, ForceConfig: true},
			want: `auth0 poppy setup --company-domain shop.example --api https://api.shop.example --app a --guest-api-access --guest-scopes orders:read,checkout --force-config`,
		},
		{
			name: "all guest scopes",
			in:   Inputs{CompanyDomain: "shop.example", APIs: []string{shopAPI}, App: "a", GuestAPIAccess: true, GuestAllScopes: true},
			want: `auth0 poppy setup --company-domain shop.example --api https://api.shop.example --app a --guest-api-access --guest-all-scopes`,
		},
		{
			name: "guest scopes picked in a terminal, the same for every API",
			in: Inputs{
				CompanyDomain: "shop.example", APIs: []string{shopAPI, "rs_pay"}, App: "a", GuestAPIAccess: true,
				GuestScopes: map[string][]string{shopAPI: {"b", "a"}, "https://pay.shop.example": {"a", "b"}},
			},
			want: `auth0 poppy setup --company-domain shop.example --api https://api.shop.example --api rs_pay --app a --guest-api-access --guest-scopes a,b`,
		},
		{
			name: "guest scopes picked differently are left out, so a rerun cannot grant more",
			in: Inputs{
				CompanyDomain: "shop.example", APIs: []string{shopAPI, "rs_pay"}, App: "a", GuestAPIAccess: true,
				GuestScopes: map[string][]string{shopAPI: {"a"}, "https://pay.shop.example": {"b"}},
			},
			want: `auth0 poppy setup --company-domain shop.example --api https://api.shop.example --api rs_pay --app a --guest-api-access`,
		},
		{
			name: "guest scopes picked for only one of two APIs are left out",
			in: Inputs{
				CompanyDomain: "shop.example", APIs: []string{shopAPI, "rs_pay"}, App: "a", GuestAPIAccess: true,
				GuestScopes: map[string][]string{shopAPI: {"a"}},
			},
			want: `auth0 poppy setup --company-domain shop.example --api https://api.shop.example --api rs_pay --app a --guest-api-access`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, strings.Join(Command(test.in, test.opts), " "))
		})
	}
}

func TestFormatCommand(t *testing.T) {
	parts := Command(Inputs{
		CompanyDomain: "shop.example", APIs: []string{shopAPI, "https://pay.shop.example"}, App: "Shop Web",
		Scopes: map[string]string{"orders:read": "See your orders and their status. Read only."},
		Hints:  map[string]config.Hints{"checkout": {Changes: "irreversible", PersonalData: true}},
	}, CommandOptions{Yes: true})

	t.Run("it breaks before a part that passes the width", func(t *testing.T) {
		out := FormatCommand("To apply:  ", parts, 90)

		assert.Contains(t, out, " \\\n")
		for _, line := range strings.Split(out, "\n") {
			assert.LessOrEqual(t, utf8.RuneCountInString(line), 90, line)
		}

		joined := strings.ReplaceAll(strings.ReplaceAll(out, " \\\n", " "), "To apply:  ", "")
		assert.Equal(t, strings.Join(parts, " "), strings.Join(strings.Fields(joined), " "), "nothing is lost")
	})

	t.Run("it never breaks without a width", func(t *testing.T) {
		out := FormatCommand("To apply:  ", parts, 0)

		assert.NotContains(t, out, "\n")
		assert.Equal(t, "To apply:  "+strings.Join(parts, " "), out)
	})

	t.Run("a part longer than the width is kept whole", func(t *testing.T) {
		out := FormatCommand("To apply:  ", []string{"auth0 poppy setup", "--api " + strings.Repeat("x", 80)}, 40)

		assert.Contains(t, out, strings.Repeat("x", 80))
	})
}
