package display

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderer_PluginValidation(t *testing.T) {
	invalid := PluginValidation{Problems: []PluginProblem{
		{Path: "commands[0].name", Message: "does not match the pattern"},
		{Path: "id", Message: "is required"},
	}}
	hostile := PluginValidation{Problems: []PluginProblem{{Path: "id", Message: hostilePluginText}}}
	valid := PluginValidation{Valid: true, Problems: []PluginProblem{}}

	tests := []struct {
		name       string
		format     OutputFormat
		result     PluginValidation
		wantStdout string
		wantStderr string
	}{
		{
			name:       "JSON has the result",
			format:     OutputFormatJSONCompact,
			result:     invalid,
			wantStdout: `{"valid":false,"problems":[{"path":"commands[0].name","message":"does not match the pattern"},{"path":"id","message":"is required"}]}`,
		},
		{
			name:       "JSON has an empty list, not null, when the manifest is valid",
			format:     OutputFormatJSONCompact,
			result:     valid,
			wantStdout: `{"valid":true,"problems":[]}`,
		},
		{
			name:       "a person is told when the manifest is valid",
			result:     valid,
			wantStderr: "The plugin manifest is valid.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, stdout, stderr := pluginRenderer(test.format)
			r.PluginValidation(test.result)

			if test.wantStdout != "" {
				assert.JSONEq(t, test.wantStdout, stdout.String())
			} else {
				assert.Empty(t, stdout.String())
			}

			assert.Contains(t, stderr.String(), test.wantStderr)
		})
	}

	t.Run("a person gets the path and the problem, as plain text", func(t *testing.T) {
		r, stdout, _ := pluginRenderer("")
		r.PluginValidation(invalid)

		assert.Contains(t, stdout.String(), "commands[0].name")
		assert.Contains(t, stdout.String(), "does not match the pattern")

		r, stdout, _ = pluginRenderer("")
		r.PluginValidation(hostile)

		assert.Contains(t, stdout.String(), cleanPluginText)
		assert.NotContains(t, stdout.String(), "\u202e")
	})
}

func TestRenderer_PluginLinked(t *testing.T) {
	result := PluginLinkResult{
		ID: "auth0/checkmate", Version: "1.0.0", Path: "/work/checkmate", Bin: "/work/checkmate/checkmate", Banner: "Running the linked build.",
		ApprovalRequired: true, ApproveCommand: "auth0 plugin approve auth0/checkmate --tenant acme.us.auth0.com",
	}

	r, stdout, _ := pluginRenderer(OutputFormatJSONCompact)
	r.PluginLinked(result)
	assert.JSONEq(t, `{"id":"auth0/checkmate","version":"1.0.0","path":"/work/checkmate","bin":"/work/checkmate/checkmate","banner":"Running the linked build.",
		"approvalRequired":true,"approveCommand":"auth0 plugin approve auth0/checkmate --tenant acme.us.auth0.com"}`, stdout.String())

	// A person gets the lines that the command printed, not a table or a CSV file.
	for _, format := range []OutputFormat{"", OutputFormatCSV} {
		r, stdout, stderr := pluginRenderer(format)
		r.PluginLinked(result)

		assert.Empty(t, stdout.String(), format)
		assert.Empty(t, stderr.String(), format)
	}
}
