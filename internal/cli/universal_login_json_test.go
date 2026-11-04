package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/openapi"
)

// universalLoginFixtureSchemaDoc is a minimal OpenAPI document covering the
// operations the --data tests drive. Injecting it keeps the suite deterministic
// and offline instead of fetching the live schema.
const universalLoginFixtureSchemaDoc = `{
  "openapi": "3.0.0",
  "info": {"title": "fixture", "version": "1.0.0"},
  "paths": {
    "/branding": {
      "patch": {
        "operationId": "patch_branding",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "logo_url": {"type": "string"},
                  "favicon_url": {"type": "string"},
                  "colors": {"type": "object"}
                }
              }
            }
          }
        }
      }
    },
    "/prompts/{prompt}/custom-text/{language}": {
      "put": {
        "operationId": "put_prompt_custom_text",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {"type": "object"}
            }
          }
        }
      }
    }
  }
}`

// useUniversalLoginFixtureSchema points --data validation at the fixture doc for
// the duration of the test, restoring the real fetching constructor afterward.
// It swaps the package-level newSchemaManager, so tests that call it must not use
// t.Parallel(): the shared global would race across parallel cases.
func useUniversalLoginFixtureSchema(t *testing.T) {
	t.Helper()
	doc, err := openapi.LoadDocFromData([]byte(universalLoginFixtureSchemaDoc))
	require.NoError(t, err)

	prev := newSchemaManager
	newSchemaManager = func() (*openapi.SchemaManager, error) {
		return openapi.NewSchemaManagerFromDoc(doc), nil
	}
	t.Cleanup(func() { newSchemaManager = prev })
}

// brandingWriteClient captures the method and URI runJSONWrite sends, so a --data
// test can assert the command built the right resource path. Request returns nil
// so runJSONWrite can unmarshal the payload back for rendering.
type brandingWriteClient struct {
	rawHTTPClientStub
	gotMethod string
	gotURI    string
}

func (c *brandingWriteClient) Request(
	_ context.Context,
	method string,
	uri string,
	_ interface{},
	_ ...management.RequestOption,
) error {
	c.gotMethod = method
	c.gotURI = uri
	return nil
}

func newBrandingWriteCLI(client *brandingWriteClient, stdout *bytes.Buffer) *cli {
	return &cli{
		api: &auth0.API{HTTPClient: client},
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  stdout,
		},
	}
}

func TestUniversalLoginUpdateCmdData(t *testing.T) {
	useUniversalLoginFixtureSchema(t)

	t.Run("sends the validated payload to PATCH /branding", func(t *testing.T) {
		client := &brandingWriteClient{}
		cmd := updateUniversalLoginCmd(newBrandingWriteCLI(client, &bytes.Buffer{}))
		// The leaf command runs without the root PreRun, so clear the required-flag
		// annotations here (stdin is non-TTY under test) to reach RunE with only --data.
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"logo_url":"https://example.com/logo.png"}`})

		require.NoError(t, cmd.Execute())
		assert.Equal(t, http.MethodPatch, client.gotMethod)
		assert.Contains(t, client.gotURI, "/branding")
	})

	t.Run("surfaces a schema validation failure before calling the API", func(t *testing.T) {
		client := &brandingWriteClient{}
		cmd := updateUniversalLoginCmd(newBrandingWriteCLI(client, &bytes.Buffer{}))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"logo_url":123}`})

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schema validation")
		assert.Empty(t, client.gotMethod, "must not reach the API when validation fails")
	})
}

func TestUniversalLoginPromptsUpdateCmdData(t *testing.T) {
	useUniversalLoginFixtureSchema(t)

	client := &brandingWriteClient{}
	cmd := updatePromptsTextCmd(newBrandingWriteCLI(client, &bytes.Buffer{}))
	prepareInteractivity(cmd)
	cmd.SetArgs([]string{"login", "--language", "en", "--data", `{"login":{"title":"Sign In"}}`})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, http.MethodPut, client.gotMethod)
	assert.Contains(t, client.gotURI, "/prompts/login/custom-text/en")
}
