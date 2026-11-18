package cli

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/openapi"
)

const organizationsFixtureSchemaDoc = `{
  "openapi": "3.0.0",
  "info": {"title": "fixture", "version": "1.0.0"},
  "paths": {
    "/organizations": {"post": {"operationId": "post_organizations", "requestBody": {"content": {"application/json": {"schema": {
      "type": "object", "required": ["name"], "properties": {"name": {"type": "string"}, "display_name": {"type": "string"}}}}}}}},
    "/organizations/{id}": {"patch": {"operationId": "patch_organizations_by_id", "requestBody": {"content": {"application/json": {"schema": {
      "type": "object", "properties": {"display_name": {"type": "string"}}}}}}}},
    "/organizations/{id}/invitations": {"post": {"operationId": "post_invitations", "requestBody": {"content": {"application/json": {"schema": {
      "type": "object", "required": ["inviter", "invitee", "client_id"],
      "properties": {"inviter": {"type": "object"}, "invitee": {"type": "object"}, "client_id": {"type": "string"}}}}}}}}
  }
}`

const (
	organizationResponse = `{"id":"org_123","name":"acme","display_name":"Acme"}`
	invitationResponse   = `{"id":"uinv_1","inviter":{"name":"Ann"},"invitee":{"email":"bo@example.com"},"client_id":"cli_1"}`
	invitationPayload    = `{"inviter":{"name":"Ann"},"invitee":{"email":"bo@example.com"},"client_id":"cli_1"}`
)

// useOrganizationsFixtureSchema validates --data against the fixture; callers must not use t.Parallel().
func useOrganizationsFixtureSchema(t *testing.T) {
	t.Helper()
	doc, err := openapi.LoadDocFromData([]byte(organizationsFixtureSchemaDoc))
	require.NoError(t, err)
	prev := newSchemaManager
	newSchemaManager = func() (*openapi.SchemaManager, error) { return openapi.NewSchemaManagerFromDoc(doc), nil }
	t.Cleanup(func() { newSchemaManager = prev })
}

func TestOrganizationsJSONFlagsAreRegistered(t *testing.T) {
	for name, newCmd := range map[string]func(*cli) *cobra.Command{
		"orgs list":         listOrganizationsCmd,
		"orgs members list": listMembersOrganizationCmd,
		"orgs invs list":    listInvitationsOrganizationCmd,
	} {
		t.Run(name, func(t *testing.T) {
			cmd := newCmd(&cli{})
			assert.NotNil(t, cmd.Flags().Lookup("schema"), "missing --schema")
			assert.NotNil(t, cmd.Flags().ShorthandLookup("q"), "missing -q")
			assert.Nil(t, cmd.Flags().Lookup("data"), "list commands take no payload")
		})
	}

	for name, newCmd := range map[string]func(*cli) *cobra.Command{
		"orgs create":      createOrganizationCmd,
		"orgs update":      updateOrganizationCmd,
		"orgs invs create": createInvitationOrganizationCmd,
	} {
		t.Run(name, func(t *testing.T) {
			cmd := newCmd(&cli{})
			assert.NotNil(t, cmd.Flags().Lookup("schema"), "missing --schema")
			assert.NotNil(t, cmd.Flags().Lookup("data"), "missing --data")
			assert.Nil(t, cmd.Flags().Lookup("query"), "write commands take no query")
		})
	}
}

func TestOrganizationsCmdData(t *testing.T) {
	useOrganizationsFixtureSchema(t)

	for _, tc := range []struct {
		name       string
		newCmd     func(*cli) *cobra.Command
		args       []string
		response   string
		wantMethod string
		wantURI    string
	}{
		{"orgs create", createOrganizationCmd, []string{"--data", `{"name":"acme"}`}, organizationResponse, http.MethodPost, "/organizations"},
		{"orgs update", updateOrganizationCmd, []string{"org_123", "--data", `{"display_name":"Acme"}`}, organizationResponse, http.MethodPatch, "/organizations/org_123"},
		{"orgs invs create keeps --org-id", createInvitationOrganizationCmd, []string{"--org-id", "org_123", "--data", invitationPayload}, invitationResponse, http.MethodPost, "/organizations/org_123/invitations"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &jsonWriteStub{response: tc.response}
			cmd := tc.newCmd(newJSONWriteCLI(stub))
			// The leaf command runs without the root PreRun, so clear the required-flag
			// annotations here (stdin is non-TTY under test) to reach RunE with only --data.
			prepareInteractivity(cmd)
			cmd.SetArgs(tc.args)

			require.NoError(t, cmd.Execute())
			assert.Equal(t, tc.wantMethod, stub.gotMethod)
			assert.Contains(t, stub.gotURI, tc.wantURI)
		})
	}

	t.Run("orgs invs create rejects a payload missing required fields", func(t *testing.T) {
		stub := &jsonWriteStub{}
		cmd := createInvitationOrganizationCmd(newJSONWriteCLI(stub))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--org-id", "org_123", "--data", `{"client_id":"cli_1"}`})

		assert.ErrorContains(t, cmd.Execute(), "schema validation")
		assert.Empty(t, stub.gotMethod, "must not reach the API when validation fails")
	})

	for _, tc := range []struct {
		name   string
		newCmd func(*cli) *cobra.Command
		args   []string
	}{
		{"orgs create", createOrganizationCmd, []string{"--data", `{"name":"acme"}`, "--name", "acme"}},
		{"orgs update", updateOrganizationCmd, []string{"org_123", "--data", `{"display_name":"x"}`, "--display", "x"}},
		{"orgs invs create", createInvitationOrganizationCmd, []string{"--org-id", "org_123", "--data", invitationPayload, "--invitee-email", "bo@example.com"}},
	} {
		t.Run(tc.name+" rejects --data combined with a payload flag", func(t *testing.T) {
			stub := &jsonWriteStub{}
			cmd := tc.newCmd(newJSONWriteCLI(stub))
			prepareInteractivity(cmd)
			cmd.SetArgs(tc.args)

			assert.ErrorContains(t, cmd.Execute(), "none of the others can be")
			assert.Empty(t, stub.gotMethod)
		})
	}
}

func TestOrganizationsCmdQuery(t *testing.T) {
	for _, tc := range []struct {
		name     string
		newCmd   func(*cli) *cobra.Command
		args     []string
		wantPath string
	}{
		{"orgs list", listOrganizationsCmd, []string{"-q", `{"per_page":5}`}, "/api/v2/organizations"},
		{"orgs members list", listMembersOrganizationCmd, []string{"org_123", "-q", `{"per_page":5}`}, "/api/v2/organizations/org_123/members"},
		{"orgs invs list", listInvitationsOrganizationCmd, []string{"--org-id", "org_123", "--query", `{"per_page":5}`}, "/api/v2/organizations/org_123/invitations"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath, gotQuery string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
				_, _ = w.Write([]byte(`[]`))
			}))
			t.Cleanup(server.Close)

			cmd := tc.newCmd(&cli{
				api:      &auth0.API{HTTPClient: &mockHTTPClientAPI{baseURL: server.URL}},
				renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: &bytes.Buffer{}},
			})
			prepareInteractivity(cmd)
			cmd.SetArgs(tc.args)

			require.NoError(t, cmd.Execute())
			assert.Equal(t, tc.wantPath, gotPath)
			assert.Equal(t, "per_page=5", gotQuery)
		})
	}
}

func TestOrganizationsInvitationsCreatePipedData(t *testing.T) {
	useOrganizationsFixtureSchema(t)

	t.Run("piped JSON combines with --org-id", func(t *testing.T) {
		stub := &jsonWriteStub{response: invitationResponse}
		cmd := createInvitationOrganizationCmd(newJSONWriteCLI(stub))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--org-id", "org_123"})

		withPipedStdin(t, invitationPayload, func() {
			require.NoError(t, cmd.Execute())
		})
		assert.Equal(t, http.MethodPost, stub.gotMethod)
		assert.Contains(t, stub.gotURI, "/organizations/org_123/invitations")
	})

	t.Run("piped JSON still rejects payload flags", func(t *testing.T) {
		stub := &jsonWriteStub{}
		cmd := createInvitationOrganizationCmd(newJSONWriteCLI(stub))
		prepareInteractivity(cmd)
		cmd.SetArgs([]string{"--org-id", "org_123", "--invitee-email", "bo@example.com"})

		withPipedStdin(t, invitationPayload, func() {
			assert.ErrorContains(t, cmd.Execute(), "cannot combine piped JSON input with individual flags (invitee-email)")
		})
		assert.Empty(t, stub.gotMethod)
	})
}
