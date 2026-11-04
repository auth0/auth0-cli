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

// emailTemplateFixtureSchemaDoc is a minimal OpenAPI document covering the PATCH
// operation the --data test drives. Injecting it keeps the suite deterministic
// and offline instead of fetching the live schema.
const emailTemplateFixtureSchemaDoc = `{
  "openapi": "3.0.0",
  "info": {"title": "fixture", "version": "1.0.0"},
  "paths": {
    "/email-templates/{templateName}": {
      "patch": {
        "operationId": "patch_email_template",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "body": {"type": "string"},
                  "from": {"type": "string"},
                  "subject": {"type": "string"},
                  "enabled": {"type": "boolean"}
                }
              }
            }
          }
        }
      }
    }
  }
}`

// useEmailTemplateFixtureSchema points --data validation at the fixture doc for
// the duration of the test, restoring the real fetching constructor afterward.
// It swaps the package-level newSchemaManager, so tests that call it must not use
// t.Parallel(): the shared global would race across parallel cases.
func useEmailTemplateFixtureSchema(t *testing.T) {
	t.Helper()
	doc, err := openapi.LoadDocFromData([]byte(emailTemplateFixtureSchemaDoc))
	require.NoError(t, err)

	prev := newSchemaManager
	newSchemaManager = func() (*openapi.SchemaManager, error) {
		return openapi.NewSchemaManagerFromDoc(doc), nil
	}
	t.Cleanup(func() { newSchemaManager = prev })
}

// emailTemplateWriteClient captures the method and URI runJSONWrite sends, so a
// --data test can assert the command built the right resource path. Request
// returns nil so runJSONWrite can unmarshal the payload back for rendering.
type emailTemplateWriteClient struct {
	rawHTTPClientStub
	gotMethod string
	gotURI    string
}

func (c *emailTemplateWriteClient) Request(
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

func newEmailTemplateWriteCLI(client *emailTemplateWriteClient, stdout *bytes.Buffer) *cli {
	return &cli{
		api: &auth0.API{HTTPClient: client},
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  stdout,
		},
	}
}

func TestEmailTemplatesUpdateCmdData(t *testing.T) {
	useEmailTemplateFixtureSchema(t)

	t.Run("sends the validated payload to PATCH /email-templates/{templateName}", func(t *testing.T) {
		client := &emailTemplateWriteClient{}
		cmd := updateEmailTemplateCmd(newEmailTemplateWriteCLI(client, &bytes.Buffer{}))
		// The leaf command runs without the root PreRun, so clear the required-flag
		// annotations here (stdin is non-TTY under test) to reach RunE with only --data.
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"welcome", "--data", `{"body":"<html>Welcome!</html>","enabled":true}`})

		require.NoError(t, cmd.Execute())
		assert.Equal(t, http.MethodPatch, client.gotMethod)
		assert.Contains(t, client.gotURI, "/email-templates/welcome_email")
	})

	t.Run("surfaces a schema validation failure before calling the API", func(t *testing.T) {
		client := &emailTemplateWriteClient{}
		cmd := updateEmailTemplateCmd(newEmailTemplateWriteCLI(client, &bytes.Buffer{}))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"welcome", "--data", `{"body":123}`})

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schema validation")
		assert.Empty(t, client.gotMethod, "must not reach the API when validation fails")
	})
}
