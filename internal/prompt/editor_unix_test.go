//go:build !windows
// +build !windows

package prompt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEditorArgs(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string
		expected []string
		wantErr  bool
	}{
		{
			name:     "simple editor",
			cmd:      "vim",
			expected: []string{"vim"},
		},
		{
			name:     "editor with flag",
			cmd:      "code --wait",
			expected: []string{"code", "--wait"},
		},
		{
			name:     "editor with multiple flags",
			cmd:      "subl --wait --new-window",
			expected: []string{"subl", "--wait", "--new-window"},
		},
		{
			name:     "path with spaces in double quotes",
			cmd:      `"/usr/local/bin/my editor" --wait`,
			expected: []string{"/usr/local/bin/my editor", "--wait"},
		},
		{
			name:     "path with spaces in single quotes",
			cmd:      `'/usr/local/bin/my editor' --wait`,
			expected: []string{"/usr/local/bin/my editor", "--wait"},
		},
		{
			name:     "path without spoaces or quotes",
			cmd:      `/usr/local/bin/myeditor --wait`,
			expected: []string{"/usr/local/bin/myeditor", "--wait"},
		},
		{
			name:     "empty command",
			cmd:      ``,
			expected: []string{},
		},
		{
			name:    "unterminated quote",
			cmd:     `"unterminated`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, err := parseEditorArgs(tt.cmd)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, args)
		})
	}
}
