package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCountryCodesList(t *testing.T) {
	tests := []struct {
		name string
		list string
		want []string
	}{
		{
			name: "empty string yields nil",
			list: "",
			want: nil,
		},
		{
			name: "single code",
			list: "US",
			want: []string{"US"},
		},
		{
			name: "multiple codes",
			list: "US,GB,CA",
			want: []string{"US", "GB", "CA"},
		},
		{
			name: "trims surrounding whitespace",
			list: " US , GB , CA ",
			want: []string{"US", "GB", "CA"},
		},
		{
			name: "drops empty entries",
			list: "US,,GB, ,CA",
			want: []string{"US", "GB", "CA"},
		},
		{
			name: "only separators yields nil",
			list: " , , ",
			want: nil,
		},
		{
			name: "preserves case and duplicates as-is (server dedupes/validates)",
			list: "us,US,US",
			want: []string{"us", "US", "US"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseCountryCodesList(tt.list))
		})
	}
}
