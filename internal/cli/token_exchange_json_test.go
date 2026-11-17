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
	"github.com/auth0/auth0-cli/internal/openapi"
)

const tokenExchangeFixtureSchemaDoc = `{
  "openapi": "3.0.0",
  "info": {"title": "fixture", "version": "1.0.0"},
  "paths": {
    "/token-exchange-profiles": {"post": {"operationId": "post_token_exchange_profile", "requestBody": {"content": {"application/json": {"schema": {
      "type": "object", "required": ["name", "subject_token_type", "action_id", "type"],
      "properties": {"name": {"type": "string"}, "subject_token_type": {"type": "string"}, "action_id": {"type": "string"}, "type": {"type": "string"}}}}}}}},
    "/token-exchange-profiles/{id}": {"patch": {"operationId": "patch_token_exchange_profile", "requestBody": {"content": {"application/json": {"schema": {
      "type": "object", "properties": {"name": {"type": "string"}, "subject_token_type": {"type": "string"}}}}}}}}
  }
}`

const tokenExchangeProfileResponse = `{"id":"tep_123","name":"p","subject_token_type":"urn:acme:legacy-token","action_id":"act_1","type":"custom_authentication"}`

// useTokenExchangeFixtureSchema validates --data against the fixture; callers must not use t.Parallel().
func useTokenExchangeFixtureSchema(t *testing.T) {
	t.Helper()
	doc, err := openapi.LoadDocFromData([]byte(tokenExchangeFixtureSchemaDoc))
	require.NoError(t, err)
	prev := newSchemaManager
	newSchemaManager = func() (*openapi.SchemaManager, error) { return openapi.NewSchemaManagerFromDoc(doc), nil }
	t.Cleanup(func() { newSchemaManager = prev })
}

func TestTokenExchangeJSONFlagsAreRegistered(t *testing.T) {
	list := listTokenExchangeProfilesCmd(&cli{})
	assert.NotNil(t, list.Flags().Lookup("schema"), "missing --schema")
	assert.NotNil(t, list.Flags().ShorthandLookup("q"), "missing -q")
	assert.Nil(t, list.Flags().Lookup("data"), "list commands take no payload")

	for name, newCmd := range map[string]func(*cli) *cobra.Command{
		"create": createTokenExchangeProfileCmd,
		"update": updateTokenExchangeProfileCmd,
	} {
		t.Run(name, func(t *testing.T) {
			cmd := newCmd(&cli{})
			assert.NotNil(t, cmd.Flags().Lookup("schema"), "missing --schema")
			assert.NotNil(t, cmd.Flags().Lookup("data"), "missing --data")
			assert.Nil(t, cmd.Flags().Lookup("query"), "write commands take no query")
		})
	}
}

func TestTokenExchangeCmdData(t *testing.T) {
	useTokenExchangeFixtureSchema(t)

	t.Run("create sends the payload to POST /token-exchange-profiles", func(t *testing.T) {
		stub := &jsonWriteStub{response: tokenExchangeProfileResponse}
		cmd := createTokenExchangeProfileCmd(newJSONWriteCLI(stub))
		// The leaf command runs without the root PreRun, so clear the required-flag
		// annotations here (stdin is non-TTY under test) to reach RunE with only --data.
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"name":"p","subject_token_type":"urn:acme:legacy-token","action_id":"act_1","type":"custom_authentication"}`})

		require.NoError(t, cmd.Execute())
		assert.Equal(t, http.MethodPost, stub.gotMethod)
		assert.Contains(t, stub.gotURI, "/token-exchange-profiles")
	})

	t.Run("update sends the payload to PATCH /token-exchange-profiles/{id}", func(t *testing.T) {
		stub := &jsonWriteStub{response: tokenExchangeProfileResponse}
		cmd := updateTokenExchangeProfileCmd(newJSONWriteCLI(stub))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"tep_123", "--data", `{"name":"renamed"}`})

		require.NoError(t, cmd.Execute())
		assert.Equal(t, http.MethodPatch, stub.gotMethod)
		assert.Contains(t, stub.gotURI, "/token-exchange-profiles/tep_123")
		assert.JSONEq(t, `{"name":"renamed"}`, stub.gotBody)
	})

	t.Run("create rejects a payload missing required fields", func(t *testing.T) {
		stub := &jsonWriteStub{}
		cmd := createTokenExchangeProfileCmd(newJSONWriteCLI(stub))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"name":"p"}`})

		assert.ErrorContains(t, cmd.Execute(), "schema validation")
		assert.Empty(t, stub.gotMethod, "must not reach the API when validation fails")
	})

	for name, tc := range map[string]struct {
		newCmd func(*cli) *cobra.Command
		args   []string
	}{
		"create": {createTokenExchangeProfileCmd, []string{"--data", `{"name":"p"}`, "--name", "p"}},
		"update": {updateTokenExchangeProfileCmd, []string{"tep_123", "--data", `{"name":"p"}`, "--name", "p"}},
	} {
		t.Run(name+" rejects --data combined with --name", func(t *testing.T) {
			stub := &jsonWriteStub{}
			cmd := tc.newCmd(newJSONWriteCLI(stub))
			prepareInteractivity(cmd)
			cmd.SetArgs(tc.args)

			assert.ErrorContains(t, cmd.Execute(), "none of the others can be")
			assert.Empty(t, stub.gotMethod)
		})
	}
}

func TestTokenExchangeListCmdInvalidQuery(t *testing.T) {
	cmd := listTokenExchangeProfilesCmd(&cli{
		renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: io.Discard},
		api:      &auth0.API{},
	})
	cmd.SetArgs([]string{"-q", "not-valid-json"})

	assert.ErrorContains(t, cmd.Execute(), "invalid --query value")
}
