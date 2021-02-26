package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/olekukonko/tablewriter"
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
