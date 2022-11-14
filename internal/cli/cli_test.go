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

	"github.com/auth0/auth0-cli/internal/display"
)

func TestIsExpired(t *testing.T) {
	t.Run("is expired", func(t *testing.T) {
		d := time.Date(2021, 01, 01, 10, 30, 30, 0, time.UTC)
		if want, got := true, isExpired(d, 1*time.Minute); want != got {
			t.Fatalf("wanted: %v, got %v", want, got)
		}
	})

	t.Run("expired because of the threshold", func(t *testing.T) {
		d := time.Now().Add(-2 * time.Minute)
		if want, got := true, isExpired(d, 5*time.Minute); want != got {
			t.Fatalf("wanted: %v, got %v", want, got)
		}
	})

	t.Run("is not expired", func(t *testing.T) {
		d := time.Now().Add(10 * time.Minute)
		if want, got := false, isExpired(d, 5*time.Minute); want != got {
			t.Fatalf("wanted: %v, got %v", want, got)
		}
	})
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
