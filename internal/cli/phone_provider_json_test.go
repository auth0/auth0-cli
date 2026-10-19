package cli

import (
	"io"
	"net/http"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
)

const phoneProviderResponse = `{"id":"pp_123","name":"twilio","disabled":false}`

func TestPhoneProviderListCmdInvalidQuery(t *testing.T) {
	c := &cli{
		renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: io.Discard},
		api:      &auth0.API{},
	}

	cmd := listBrandingPhoneProviderCmd(c)
	cmd.SetArgs([]string{"--query", "not-valid-json"})

	assert.ErrorContains(t, cmd.Execute(), "invalid --query value")
}

func TestPhoneProviderCreateCmdData(t *testing.T) {
	t.Run("sends the validated payload to POST /branding/phone/providers", func(t *testing.T) {
		stub := &jsonWriteStub{response: phoneProviderResponse}
		cmd := createBrandingPhoneProviderCmd(newJSONWriteCLI(stub))
		// The leaf command runs without the root PreRun, so clear the required-flag
		// annotations here (stdin is non-TTY under test) to reach RunE with only --data.
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"name":"twilio","credentials":{"auth_token":"tok"},"configuration":{"delivery_methods":["text"]}}`})

		require.NoError(t, cmd.Execute())
		assert.Equal(t, http.MethodPost, stub.gotMethod)
		assert.Contains(t, stub.gotURI, "/branding/phone/providers")
	})

	t.Run("surfaces a schema validation failure before calling the API", func(t *testing.T) {
		stub := &jsonWriteStub{}
		cmd := createBrandingPhoneProviderCmd(newJSONWriteCLI(stub))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"name":123}`})

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schema validation")
		assert.Empty(t, stub.gotMethod, "must not reach the API when validation fails")
	})
}

func TestPhoneProviderUpdateCmdData(t *testing.T) {
	stub := &jsonWriteStub{response: phoneProviderResponse}
	cmd := updateBrandingPhoneProviderCmd(newJSONWriteCLI(stub))
	prepareInteractivity(cmd)
	cmd.SetArgs([]string{"pp_123", "--data", `{"credentials":{"auth_token":"newTok"}}`})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, http.MethodPatch, stub.gotMethod)
	assert.Contains(t, stub.gotURI, "/branding/phone/providers/pp_123")
}

func TestPhoneProviderUpdateCmdDataRejectsInvalidSchema(t *testing.T) {
	stub := &jsonWriteStub{}
	cmd := updateBrandingPhoneProviderCmd(newJSONWriteCLI(stub))
	prepareInteractivity(cmd)
	cmd.SetArgs([]string{"pp_123", "--data", `{"disabled":"yes"}`})

	assert.ErrorContains(t, cmd.Execute(), "schema validation")
	assert.Empty(t, stub.gotMethod, "must not reach the API when validation fails")
}

func TestPhoneProviderDataConflictsWithFlags(t *testing.T) {
	commands := map[string]struct {
		newCmd func(*cli) *cobra.Command
		args   []string
	}{
		"create": {createBrandingPhoneProviderCmd, []string{"--data", `{"name":"twilio"}`, "--provider", "twilio"}},
		"update": {updateBrandingPhoneProviderCmd, []string{"pp_123", "--data", `{"disabled":true}`, "--disabled"}},
	}

	for name, tc := range commands {
		t.Run(name, func(t *testing.T) {
			stub := &jsonWriteStub{}
			cmd := tc.newCmd(newJSONWriteCLI(stub))
			prepareInteractivity(cmd)
			cmd.SetArgs(tc.args)

			assert.ErrorContains(t, cmd.Execute(), "none of the others can be")
			assert.Empty(t, stub.gotMethod, "must not reach the API when flags conflict")
		})
	}
}
