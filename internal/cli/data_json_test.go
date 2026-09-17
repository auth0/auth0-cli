package cli

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/iostream"
	"github.com/auth0/auth0-cli/internal/openapi"
)

// newDataCommand builds a minimal create-like command with the flags that matter
// for ResolveData: --data, a granular input flag (--name), and an output flag
// (--json). It mirrors how a real resource command registers these.
func newDataCommand() (*cobra.Command, *struct {
	Data string
	Name string
	JSON bool
}) {
	inputs := &struct {
		Data string
		Name string
		JSON bool
	}{}

	cmd := &cobra.Command{Use: "create", RunE: func(*cobra.Command, []string) error { return nil }}
	dataFlag.RegisterString(cmd, &inputs.Data, "")
	actionName.RegisterString(cmd, &inputs.Name, "")
	cmd.Flags().BoolVar(&inputs.JSON, "json", false, "Output in json format.")

	return cmd, inputs
}

// withPipedStdin swaps iostream.Input for a pipe carrying content (empty content
// means "closed pipe with no data"), runs fn, and restores the original stdin.
// A pipe is a non-terminal file, so iostream.PipedInput() reads from it.
func withPipedStdin(t *testing.T, content string, fn func()) {
	t.Helper()

	r, w, err := os.Pipe()
	require.NoError(t, err)

	original := iostream.Input
	iostream.Input = r
	defer func() { iostream.Input = original }()

	_, err = w.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	fn()
	require.NoError(t, r.Close())
}

func TestReadJSONInput(t *testing.T) {
	handler := &DataJSONHandler{}

	t.Run("inline JSON is returned verbatim", func(t *testing.T) {
		data, err := handler.readJSONInput(`{"name":"x"}`)
		require.NoError(t, err)
		assert.Equal(t, `{"name":"x"}`, string(data))
	})

	t.Run("empty input is an error", func(t *testing.T) {
		_, err := handler.readJSONInput("")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no input provided")
	})

	// "@-" reads the payload from stdin, letting a script pipe a body while still
	// passing the flag explicitly.
	t.Run("@- reads from piped stdin", func(t *testing.T) {
		withPipedStdin(t, `{"name":"from-pipe"}`, func() {
			data, err := handler.readJSONInput("@-")
			require.NoError(t, err)
			assert.Equal(t, `{"name":"from-pipe"}`, string(data))
		})
	})

	t.Run("- reads from piped stdin", func(t *testing.T) {
		withPipedStdin(t, `{"name":"from-pipe"}`, func() {
			data, err := handler.readJSONInput("-")
			require.NoError(t, err)
			assert.Equal(t, `{"name":"from-pipe"}`, string(data))
		})
	})

	t.Run("@- with empty stdin is an error", func(t *testing.T) {
		withPipedStdin(t, "", func() {
			_, err := handler.readJSONInput("@-")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "no data received on stdin")
		})
	})
}

func TestResolveData(t *testing.T) {
	t.Run("explicit --data flag", func(t *testing.T) {
		cmd, _ := newDataCommand()
		require.NoError(t, cmd.ParseFlags([]string{"--data", `{"name":"x"}`}))

		withPipedStdin(t, "", func() {
			payload, provided, err := ResolveData(cmd)
			require.NoError(t, err)
			assert.True(t, provided)
			assert.Equal(t, `{"name":"x"}`, payload)
		})
	})

	// --data wins over piped stdin (like `auth0 api`); the flag value is used.
	t.Run("--data flag takes precedence over piped stdin", func(t *testing.T) {
		cmd, _ := newDataCommand()
		require.NoError(t, cmd.ParseFlags([]string{"--data", `{"name":"from-flag"}`}))

		withPipedStdin(t, `{"name":"from-pipe"}`, func() {
			payload, provided, err := ResolveData(cmd)
			require.NoError(t, err)
			assert.True(t, provided)
			assert.Equal(t, `{"name":"from-flag"}`, payload)
		})
	})

	t.Run("piped stdin, no flags", func(t *testing.T) {
		cmd, _ := newDataCommand()
		require.NoError(t, cmd.ParseFlags([]string{}))

		withPipedStdin(t, `{"name":"from-pipe"}`, func() {
			payload, provided, err := ResolveData(cmd)
			require.NoError(t, err)
			assert.True(t, provided)
			assert.Equal(t, `{"name":"from-pipe"}`, payload)
		})
	})

	// JSON input replaces the individual flags, so piped JSON combined with a
	// granular input flag is a clear error. MarkFlagsMutuallyExclusive cannot see
	// stdin, so ResolveData must reject this itself.
	t.Run("piped stdin combined with input flag is rejected", func(t *testing.T) {
		cmd, _ := newDataCommand()
		require.NoError(t, cmd.ParseFlags([]string{"--name", "from-flag"}))

		withPipedStdin(t, `{"name":"from-pipe"}`, func() {
			payload, provided, err := ResolveData(cmd)
			require.Error(t, err)
			assert.False(t, provided)
			assert.Empty(t, payload)
			assert.Contains(t, err.Error(), "name")
			assert.Contains(t, err.Error(), "cannot combine")
		})
	})

	// Output flags are not input flags, so a pipe may coexist with --json.
	t.Run("piped stdin with output flag is allowed", func(t *testing.T) {
		cmd, _ := newDataCommand()
		require.NoError(t, cmd.ParseFlags([]string{"--json"}))

		withPipedStdin(t, `{"name":"from-pipe"}`, func() {
			payload, provided, err := ResolveData(cmd)
			require.NoError(t, err)
			assert.True(t, provided)
			assert.Equal(t, `{"name":"from-pipe"}`, payload)
		})
	})

	t.Run("no data and no pipe falls through to interactive", func(t *testing.T) {
		cmd, _ := newDataCommand()
		require.NoError(t, cmd.ParseFlags([]string{}))

		withPipedStdin(t, "", func() {
			payload, provided, err := ResolveData(cmd)
			require.NoError(t, err)
			assert.False(t, provided)
			assert.Empty(t, payload)
		})
	})
}

func TestReadAndValidateAttachesStructuredDetails(t *testing.T) {
	manager, err := openapi.NewSchemaManager()
	require.NoError(t, err)
	handler := &DataJSONHandler{cli: &cli{}, manager: manager}

	// A payload missing a required field fails local schema validation. The
	// returned error must classify as validation and carry the field-level
	// failures as JSON details for the error envelope.
	_, _, err = handler.ReadAndValidate(`{"name":"x","code":"module.exports = () => {}"}`, "POST", "/actions/actions")
	require.Error(t, err)
	assert.Equal(t, "validation", errorClass(err))

	details := errorDetails(err)
	require.NotEmpty(t, details, "expected structured details on the validation error")

	var fieldErrors []openapi.FieldError
	require.NoError(t, json.Unmarshal(details, &fieldErrors))
	require.NotEmpty(t, fieldErrors)

	var found bool
	for _, fe := range fieldErrors {
		assert.NotEmpty(t, fe.Reason)
		if fe.Field == "supported_triggers" {
			found = true
		}
	}
	assert.True(t, found, "expected a field error for the missing supported_triggers")
}
