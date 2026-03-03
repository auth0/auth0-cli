//go:build windows
// +build windows

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
			cmd:      "notepad",
			expected: []string{"notepad"},
		},
		{
			name:     "editor with flag",
			cmd:      "code --wait",
			expected: []string{"code", "--wait"},
		},
		{
			name:     "windows path with backslashes",
			cmd:      `C:\Windows\notepad.exe`,
			expected: []string{`C:\Windows\notepad.exe`},
		},
		{
			name:     "quoted path with spaces",
			cmd:      `"C:\Program Files\Notepad++\notepad++.exe" --wait`,
			expected: []string{`C:\Program Files\Notepad++\notepad++.exe`, "--wait"},
		},
		{
			name:     "quoted path with spaces and multiple flags",
			cmd:      `"C:\Program Files\Microsoft VS Code\code.exe" --wait --new-window`,
			expected: []string{`C:\Program Files\Microsoft VS Code\code.exe`, "--wait", "--new-window"},
		},
		{
			name:     "path without spaces or quotes",
			cmd:      `C:\tools\vim.exe -u C:\Users\me\.vimrc`,
			expected: []string{`C:\tools\vim.exe`, "-u", `C:\Users\me\.vimrc`},
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
