package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
)

type rawHTTPClientStub struct {
	method   string
	payload  interface{}
	response json.RawMessage
}

func (s *rawHTTPClientStub) NewRequest(
	ctx context.Context,
	method string,
	uri string,
	payload interface{},
	_ ...management.RequestOption,
) (*http.Request, error) {
	s.method = method
	s.payload = payload
	return http.NewRequestWithContext(ctx, method, uri, nil)
}

func (s *rawHTTPClientStub) Do(_ *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(string(s.response))),
	}, nil
}

func (s *rawHTTPClientStub) Request(
	context.Context,
	string,
	string,
	interface{},
	...management.RequestOption,
) error {
	return nil
}

func (s *rawHTTPClientStub) URI(path ...string) string {
	return "https://example.test/api/v2/" + strings.Join(path, "/")
}

func newRawTestCLI(stub *rawHTTPClientStub, stdout *bytes.Buffer) *cli {
	return &cli{
		api: &auth0.API{HTTPClient: stub},
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  stdout,
		},
	}
}

func TestCreateVaultConnectionCmdUsesRawClient(t *testing.T) {
	body := []byte(`{"setup":{"type":"BEARER","token":"secret"}}`)
	path := filepath.Join(t.TempDir(), "conn.json")
	require.NoError(t, os.WriteFile(path, body, 0600))

	stub := &rawHTTPClientStub{
		response: json.RawMessage(`{"id":"ac_1","app_id":"HTTP","name":"Renamed","ready":true}`),
	}
	stdout := &bytes.Buffer{}
	c := newRawTestCLI(stub, stdout)

	cmd := createVaultConnectionCmd(c)
	cmd.SetArgs([]string{"--setup-file", path, "--name", "Renamed", "--app-id", "HTTP"})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, http.MethodPost, stub.method)
	require.IsType(t, json.RawMessage{}, stub.payload)
	sent := string(stub.payload.(json.RawMessage))
	assert.Contains(t, sent, `"Renamed"`)
	assert.Contains(t, sent, `"HTTP"`)
	assert.Contains(t, sent, `"setup"`)
	// The rendered output must never echo the setup secrets back.
	assert.NotContains(t, stdout.String(), "secret")
}
