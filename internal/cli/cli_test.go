package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jsanda/tablewriter"
)

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
