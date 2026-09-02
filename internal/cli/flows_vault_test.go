package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestCreateVaultConnectionCmdRejectsNameInSetupFile(t *testing.T) {
	body := []byte(`{"name":"Renamed","setup":{"type":"BEARER","token":"secret"}}`)
	path := filepath.Join(t.TempDir(), "conn.json")
	require.NoError(t, os.WriteFile(path, body, 0600))

	stub := &rawHTTPClientStub{}
	stdout := &bytes.Buffer{}
	c := newRawTestCLI(stub, stdout)

	cmd := createVaultConnectionCmd(c)
	cmd.SetArgs([]string{"--setup-file", path, "--name", "Renamed", "--app-id", "HTTP"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not contain a top-level \"name\" field")
}
