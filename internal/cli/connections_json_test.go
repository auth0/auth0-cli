package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/openapi"
)

// connectionFixtureSchemaDoc is a minimal OpenAPI document covering the connection
// write operations the --data tests drive. Injecting it keeps the suite
// deterministic and offline: validation no longer depends on fetching the live
// schema.
const connectionFixtureSchemaDoc = `{
  "openapi": "3.0.0",
  "info": {"title": "fixture", "version": "1.0.0"},
  "paths": {
    "/connections": {
      "post": {
        "operationId": "post_connection",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["name", "strategy"],
                "properties": {
                  "name": {"type": "string"},
                  "strategy": {"type": "string"},
                  "display_name": {"type": "string"}
                }
              }
            }
          }
        }
      }
    },
    "/connections/{id}": {
      "patch": {
        "operationId": "patch_connection",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "display_name": {"type": "string"}
                }
              }
            }
          }
        }
      }
    }
  }
}`

// useConnectionFixtureSchema points --data validation at connectionFixtureSchemaDoc
// for the duration of the test, restoring the real fetching constructor afterward.
// It swaps the package-level newSchemaManager, so tests that call it must not use
// t.Parallel(): the shared global would race across parallel cases.
func useConnectionFixtureSchema(t *testing.T) {
	t.Helper()
	doc, err := openapi.LoadDocFromData([]byte(connectionFixtureSchemaDoc))
	require.NoError(t, err)

	prev := newSchemaManager
	newSchemaManager = func() (*openapi.SchemaManager, error) {
		return openapi.NewSchemaManagerFromDoc(doc), nil
	}
	t.Cleanup(func() { newSchemaManager = prev })
}

// capturedConnectionRequest records what the raw HTTP write path sent, so a --data
// test can assert the command built the right method and resource path.
type capturedConnectionRequest struct {
	called bool
	method string
	path   string
}

// newConnectionWriteCLI wires a cli whose raw HTTP writes hit a capturing test
// server. The server echoes a connection body so ConnectionCreateRaw/UpdateRaw can
// render, and records the method and path for assertions.
func newConnectionWriteCLI(t *testing.T) (*cli, *capturedConnectionRequest) {
	t.Helper()
	captured := &capturedConnectionRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.called = true
		captured.method = r.Method
		captured.path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"con_123","name":"my-db","strategy":"auth0"}`))
	}))
	t.Cleanup(server.Close)

	c := &cli{
		api: &auth0.API{HTTPClient: &mockHTTPClientAPI{baseURL: server.URL}},
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  io.Discard,
		},
	}
	return c, captured
}

func TestConnectionsListCmdInvalidQuery(t *testing.T) {
	cli := &cli{
		renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: io.Discard},
		api:      &auth0.API{},
	}

	cmd := listConnectionsCmd(cli)
	cmd.SetArgs([]string{"--query", "not-valid-json"})

	assert.ErrorContains(t, cmd.Execute(), "invalid --query value")
}

func TestConnectionsCreateCmdData(t *testing.T) {
	useConnectionFixtureSchema(t)

	t.Run("sends the validated payload to POST /connections", func(t *testing.T) {
		c, captured := newConnectionWriteCLI(t)
		cmd := createConnectionCmd(c)
		// Clear the required-flag annotations (as the root PreRun does for
		// non-TTY input) so --data alone satisfies the command.
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"name":"my-db","strategy":"auth0"}`})

		require.NoError(t, cmd.Execute())
		assert.Equal(t, http.MethodPost, captured.method)
		assert.Contains(t, captured.path, "/connections")
	})

	t.Run("surfaces a schema validation failure before calling the API", func(t *testing.T) {
		c, captured := newConnectionWriteCLI(t)
		cmd := createConnectionCmd(c)
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--data", `{"name":123}`})

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schema validation")
		assert.False(t, captured.called, "must not reach the API when validation fails")
	})
}

func TestConnectionsUpdateCmdData(t *testing.T) {
	useConnectionFixtureSchema(t)

	c, captured := newConnectionWriteCLI(t)
	cmd := updateConnectionCmd(c)
	prepareInteractivity(cmd)
	cmd.SetArgs([]string{"con_123", "--data", `{"display_name":"x"}`})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, http.MethodPatch, captured.method)
	assert.Contains(t, captured.path, "/connections/con_123")
}
