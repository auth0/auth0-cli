package cli

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/iostream"
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
