package cli

import (
	"errors"
	"net/http"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const emailProviderResponse = `{"name":"sendgrid","enabled":true}`

func TestEmailProviderCreateCmdData(t *testing.T) {
	t.Run("sends the payload to POST /emails/provider", func(t *testing.T) {
		stub := &jsonWriteStub{response: emailProviderResponse}
		cmd := createEmailProviderCmd(newJSONWriteCLI(stub))
		// The leaf command runs without the root PreRun, so clear the required-flag
		// annotations here (stdin is non-TTY under test) to reach RunE with only --data.
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"name":"sendgrid","credentials":{"api_key":"key"},"enabled":true}`})

		require.NoError(t, cmd.Execute())
		assert.Equal(t, http.MethodPost, stub.gotMethod)
		assert.Contains(t, stub.gotURI, "/emails/provider")
	})

	for name, data := range map[string]string{
		"custom with an empty credentials object":  `{"name":"custom","credentials":{},"enabled":true}`,
		"custom without credentials (API decides)": `{"name":"custom","enabled":true}`,
	} {
		t.Run("sends "+name+" as-is", func(t *testing.T) {
			stub := &jsonWriteStub{response: emailProviderResponse}
			cmd := createEmailProviderCmd(newJSONWriteCLI(stub))
			prepareInteractivity(cmd)
			cmd.SetArgs([]string{"--data", data})

			require.NoError(t, cmd.Execute())
			assert.Equal(t, data, stub.gotBody)
		})
	}

	t.Run("passes the API error through without a schema dump", func(t *testing.T) {
		stub := &jsonWriteStub{err: errors.New("400 Bad Request: Payload validation error: 'Missing required property: credentials'")}
		cmd := createEmailProviderCmd(newJSONWriteCLI(stub))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"name":"custom","enabled":true}`})

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Missing required property: credentials")
		assert.NotContains(t, err.Error(), "Expected Request Schema")
	})
}

func TestEmailProviderUpdateCmdData(t *testing.T) {
	stub := &jsonWriteStub{response: emailProviderResponse}
	cmd := updateEmailProviderCmd(newJSONWriteCLI(stub))
	prepareInteractivity(cmd)
	cmd.SetArgs([]string{"--data", `{"credentials":{"api_key":"NewKey"}}`})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, http.MethodPatch, stub.gotMethod)
	assert.Contains(t, stub.gotURI, "/emails/provider")
	assert.Equal(t, `{"credentials":{"api_key":"NewKey"}}`, stub.gotBody)
}

func TestEmailProviderDataRejectsBadPayload(t *testing.T) {
	commands := map[string]func(*cli) *cobra.Command{
		"create": createEmailProviderCmd,
		"update": updateEmailProviderCmd,
	}
	payloads := map[string]struct{ data, wantErr string }{
		"syntax error keeps the parse detail": {`{"name":"sendgrid",`, "unexpected end of JSON input"},
		"array is not an object":              {`["custom"]`, "must be a JSON object"},
		"null is not an object":               {`null`, "must be a JSON object"},
		"empty --data":                        {``, "no input provided"},
	}

	for cmdName, newCmd := range commands {
		for name, tc := range payloads {
			t.Run(cmdName+" rejects "+name, func(t *testing.T) {
				stub := &jsonWriteStub{}
				cmd := newCmd(newJSONWriteCLI(stub))
				prepareInteractivity(cmd)
				cmd.SetArgs([]string{"--data", tc.data})

				assert.ErrorContains(t, cmd.Execute(), tc.wantErr)
				assert.Empty(t, stub.gotMethod, "must not reach the API on a bad payload")
			})
		}
	}
}

func TestEmailProviderDataConflictsWithFlags(t *testing.T) {
	commands := map[string]func(*cli) *cobra.Command{
		"create": createEmailProviderCmd,
		"update": updateEmailProviderCmd,
	}

	for name, newCmd := range commands {
		t.Run(name, func(t *testing.T) {
			stub := &jsonWriteStub{}
			cmd := newCmd(newJSONWriteCLI(stub))
			prepareInteractivity(cmd)
			cmd.SetArgs([]string{"--data", `{"name":"sendgrid","credentials":{}}`, "--provider", "sendgrid"})

			assert.ErrorContains(t, cmd.Execute(), "none of the others can be")
			assert.Empty(t, stub.gotMethod, "must not reach the API when flags conflict")
		})
	}
}
