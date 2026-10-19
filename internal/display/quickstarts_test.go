package display

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func qsSetupAppOnlyResult() *QuickstartSetupResult {
	return &QuickstartSetupResult{
		App: &QuickstartSetupApp{
			ClientID:  "cid",
			Name:      "My App",
			Type:      "regular_web",
			Callbacks: []string{"http://localhost:3000/callback"},
			// LogoutURLs and WebOrigins intentionally nil.
		},
		ConfigFile: &QuickstartSetupConfigFile{Path: ".env", Format: "dotenv", Action: "created"},
		// KeysWritten and NextSteps intentionally nil.
	}
}

func TestRenderer_QuickstartSetup_JSON(t *testing.T) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	r := &Renderer{ResultWriter: stdout, MessageWriter: stderr, Format: OutputFormatJSON}

	r.QuickstartSetup(qsSetupAppOnlyResult())

	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc), "output: %s", stdout.String())

	assert.Equal(t, "null", string(doc["api"]))
	assert.Equal(t, "null", string(doc["client_grant"]))
	assert.JSONEq(t, `[]`, string(doc["keys_written"]))
	assert.JSONEq(t, `[]`, string(doc["next_steps"]))

	var cfg map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(doc["config_file"], &cfg))
	assert.Equal(t, "null", string(cfg["backup"]))
	assert.JSONEq(t, `".env"`, string(cfg["path"]))
	assert.JSONEq(t, `"dotenv"`, string(cfg["format"]))
	assert.JSONEq(t, `"created"`, string(cfg["action"]))

	var app map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(doc["app"], &app))
	assert.JSONEq(t, `"cid"`, string(app["client_id"]))
	assert.JSONEq(t, `"My App"`, string(app["name"]))
	assert.JSONEq(t, `"regular_web"`, string(app["type"]))
	assert.JSONEq(t, `["http://localhost:3000/callback"]`, string(app["callbacks"]))
	assert.JSONEq(t, `[]`, string(app["logout_urls"]))
	assert.JSONEq(t, `[]`, string(app["web_origins"]))

	assert.Empty(t, stderr.String())
}

func TestRenderer_QuickstartSetup_JSON_FullShape(t *testing.T) {
	stdout := &bytes.Buffer{}
	r := &Renderer{ResultWriter: stdout, MessageWriter: &bytes.Buffer{}, Format: OutputFormatJSON}

	backup := "src/environments/environment.ts.auth0-backup"
	r.QuickstartSetup(&QuickstartSetupResult{
		App:         &QuickstartSetupApp{ClientID: "cid", Name: "n", Type: "spa"},
		API:         &QuickstartSetupAPI{Identifier: "https://my-api", Name: "n-API"},
		ClientGrant: &QuickstartSetupClientGrant{ID: "cgr_1", ClientID: "cid", Audience: "https://my-api"},
		ConfigFile:  &QuickstartSetupConfigFile{Path: "src/environments/environment.ts", Format: "angular-ts", Action: "overwritten", Backup: &backup},
		KeysWritten: []string{"clientId", "domain"},
		NextSteps:   []string{"do x"},
	})

	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc))
	for _, k := range []string{"app", "api", "client_grant", "config_file", "keys_written", "next_steps"} {
		assert.Contains(t, doc, k)
	}
	var api, grant, cfg map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(doc["api"], &api))
	require.NoError(t, json.Unmarshal(doc["client_grant"], &grant))
	require.NoError(t, json.Unmarshal(doc["config_file"], &cfg))
	assert.JSONEq(t, `[]`, string(api["scopes"]), "nil nested scopes normalize to []")
	assert.JSONEq(t, `[]`, string(grant["scopes"]))
	assert.JSONEq(t, `"cgr_1"`, string(grant["id"]))
	assert.JSONEq(t, `"`+backup+`"`, string(cfg["backup"]))
	assert.JSONEq(t, `[]`, string(cfg["keys_replaced"]), "nil keys_replaced normalizes to []")
	assert.NotContains(t, cfg, "notes", "notes feed next_steps and are not part of config_file")
	assert.NotContains(t, cfg, "Notes")
	assert.JSONEq(t, `["clientId","domain"]`, string(doc["keys_written"]))
	assert.JSONEq(t, `["do x"]`, string(doc["next_steps"]))
}

func TestRenderer_QuickstartSetup_KeysReplacedAndNotes(t *testing.T) {
	stdout := &bytes.Buffer{}
	r := &Renderer{ResultWriter: stdout, MessageWriter: &bytes.Buffer{}, Format: OutputFormatJSON}

	r.QuickstartSetup(&QuickstartSetupResult{
		ConfigFile: &QuickstartSetupConfigFile{
			Path: ".env", Format: "dotenv", Action: "merged",
			KeysReplaced: []string{"AUTH0_DOMAIN"}, Notes: []string{"hidden note"},
		},
	})

	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc))
	var cfg map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(doc["config_file"], &cfg))
	assert.JSONEq(t, `["AUTH0_DOMAIN"]`, string(cfg["keys_replaced"]))
	assert.NotContains(t, stdout.String(), "hidden note")
}

func TestRenderer_QuickstartSetup_JSONCompact(t *testing.T) {
	stdout := &bytes.Buffer{}
	r := &Renderer{ResultWriter: stdout, MessageWriter: &bytes.Buffer{}, Format: OutputFormatJSONCompact}

	r.QuickstartSetup(qsSetupAppOnlyResult())

	out := stdout.String()
	assert.True(t, strings.HasSuffix(out, "\n"), "compact output ends with a newline")
	assert.Equal(t, 1, strings.Count(out, "\n"), "compact output is a single line")
	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(out), &doc))
	assert.JSONEq(t, `[]`, string(doc["keys_written"]))
}

func TestRenderer_QuickstartSetup_HumanNoOutput(t *testing.T) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	r := &Renderer{ResultWriter: stdout, MessageWriter: stderr}

	r.QuickstartSetup(qsSetupAppOnlyResult())
	assert.Empty(t, stdout.String(), "human format prints nothing; output was already rendered")
	assert.Empty(t, stderr.String())

	for _, f := range []OutputFormat{"", OutputFormatJSON, OutputFormatJSONCompact} {
		r := &Renderer{ResultWriter: stdout, MessageWriter: stderr, Format: f}
		assert.NotPanics(t, func() { r.QuickstartSetup(nil) })
	}
	assert.Empty(t, stdout.String(), "nil result prints nothing")
}
