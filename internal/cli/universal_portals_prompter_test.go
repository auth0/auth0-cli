package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOptionLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		label string
		hint  string
		want  string
	}{
		{name: "without a hint", label: "Consumer portal", want: "Consumer portal"},
		{name: "with a hint", label: "Fix all errors", hint: "recommended", want: "Fix all errors (recommended)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, optionLabel(tt.label, tt.hint))
		})
	}
}
