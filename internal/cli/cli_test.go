package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/olekukonko/tablewriter"
	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth"
	"github.com/auth0/auth0-cli/internal/display"
)

func TestTenant_HasExpiredToken(t *testing.T) {
	var testCases = []struct {
		name                     string
		givenTime                time.Time
		expectedTokenToBeExpired bool
	}{
		{
			name:                     "is expired",
			givenTime:                time.Date(2021, 01, 01, 10, 30, 30, 0, time.UTC),
			expectedTokenToBeExpired: true,
		},
		{
			name:                     "expired because of the threshold",
			givenTime:                time.Now().Add(-2 * time.Minute),
			expectedTokenToBeExpired: true,
		},
		{
			name:                     "is not expired",
			givenTime:                time.Now().Add(10 * time.Minute),
			expectedTokenToBeExpired: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tenant := Tenant{ExpiresAt: testCase.givenTime}
			assert.Equal(t, testCase.expectedTokenToBeExpired, tenant.hasExpiredToken())
		})
	}
}

// TODO(cyx): think about whether we should extract this function in the
// `display` package. For now duplication might be better and less premature.
func expectTable(t testing.TB, got string, header []string, data [][]string) {
	w := &bytes.Buffer{}

	tableString := &strings.Builder{}
	table := tablewriter.NewWriter(tableString)
	table.SetHeader(header)

	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)

	for _, v := range data {
		table.Append(v)
	}

	table.Render()
	fmt.Fprint(w, tableString.String())

	want := w.String()

	if got != want {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestIsLoggedIn(t *testing.T) {
	tests := []struct {
		defaultTenant string
		tenants       map[string]Tenant
		want          bool
		desc          string
	}{
		{"", map[string]Tenant{}, false, "no tenants"},
		{"t0", map[string]Tenant{}, false, "tenant is set but no tenants map"},
		{"t0", map[string]Tenant{"t0": {}}, false, "tenants map set but invalid token"},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			tmpFile, err := os.CreateTemp(os.TempDir(), "isLoggedIn-")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpFile.Name())

			type Config struct {
				DefaultTenant string            `json:"default_tenant"`
				Tenants       map[string]Tenant `json:"tenants"`
			}

			b, err := json.Marshal(&Config{test.defaultTenant, test.tenants})
			if err != nil {
				t.Fatal(err)
			}

			if err = os.WriteFile(tmpFile.Name(), b, 0400); err != nil {
				t.Fatal(err)
			}

			c := cli{renderer: display.NewRenderer(), path: tmpFile.Name()}
			assert.Equal(t, test.want, c.isLoggedIn())
		})
	}
}

func TestTenant_AdditionalRequestedScopes(t *testing.T) {
	var testCases = []struct {
		name           string
		givenScopes    []string
		expectedScopes []string
	}{
		{
			name:           "it can correctly distinguish additionally requested scopes",
			givenScopes:    append(auth.RequiredScopes, "read:stats", "read:client_grants"),
			expectedScopes: []string{"read:stats", "read:client_grants"},
		},
		{
			name:           "it returns an empty string slice if no additional requested scopes were given",
			givenScopes:    auth.RequiredScopes,
			expectedScopes: []string{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tenant := Tenant{Scopes: testCase.givenScopes}
			assert.Equal(t, testCase.expectedScopes, tenant.additionalRequestedScopes())
		})
	}
}
