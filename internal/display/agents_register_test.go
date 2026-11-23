package display

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	coreagents "github.com/auth0/auth0-cli/internal/agents"
)

const (
	agentTestURL  = "https://muse.example/agent.json"
	agentTestAPI  = "https://api.shop.example"
	agentTestID   = "agt_1"
	agentTestApp  = "cli_agent"
	agentTestName = "Meta Muse"
)

func newAgentRenderer(format OutputFormat) (*Renderer, *bytes.Buffer, *bytes.Buffer) {
	var out, msg bytes.Buffer

	return &Renderer{ResultWriter: &out, MessageWriter: &msg, Format: format, Tenant: "tenant.example"}, &out, &msg
}

func TestDescribePoppyKeys(t *testing.T) {
	assert.Equal(t, "0 ()", describePoppyKeys(nil))
	assert.Equal(t, "3 (PS256, RS256)", describePoppyKeys([]coreagents.Key{{Alg: "RS256"}, {Alg: "PS256"}, {Alg: "RS256"}}))
}

func TestAgentPreviewAndPlan(t *testing.T) {
	setColors(t, false)

	preview := &coreagents.Preview{
		URL:          agentTestURL,
		Name:         agentTestName,
		RedirectURLs: []string{"https://muse.example/cb"},
		Keys:         []coreagents.Key{{Alg: "PS256"}},
		Warnings:     []string{"No logo."},
		ClientID:     agentTestApp,
	}

	r, out, _ := newAgentRenderer(OutputFormat(""))
	r.AgentPreview(preview)

	assert.Contains(t, out.String(), "Agent metadata  "+agentTestURL+"\n")
	assert.Contains(t, out.String(), "name           "+agentTestName+"\n")
	assert.Contains(t, out.String(), "keys           1 (PS256)\n")
	assert.Contains(t, out.String(), "!  No logo.\n")
	assert.Contains(t, out.String(), "An agent app already exists for this URL ("+agentTestApp+"). It will be updated.")

	out.Reset()
	r.AgentRegisterPlan(preview, "", true, []coreagents.Grant{{Audience: agentTestAPI, Scopes: []string{"orders:read"}}})

	assert.Contains(t, out.String(), "\nThis will\n")
	assert.Contains(t, out.String(), "~  Agent app     update, for "+agentTestURL+"\n")
	assert.Contains(t, out.String(), "+  DPoP          required on the agent app\n")
	assert.Contains(t, out.String(), `+  Agent record  "`+agentTestName+`"`+"\n")
	assert.Contains(t, out.String(), "+  Grant         "+agentTestAPI+": orders:read  (skipped if the agent app already has a grant)\n")
}

func TestAgentRegistration(t *testing.T) {
	setColors(t, false)

	reg := &coreagents.Registered{
		Registration: coreagents.Registration{
			URL:          agentTestURL,
			Name:         agentTestName,
			ClientID:     agentTestApp,
			AppCreated:   true,
			AgentID:      agentTestID,
			AgentCreated: true,
		},
		Grants: []coreagents.GrantResult{{Grant: coreagents.Grant{Audience: agentTestAPI, Scopes: []string{"orders:read"}}, Created: true}},
	}

	t.Run("what was made, and what to do next", func(t *testing.T) {
		r, out, _ := newAgentRenderer(OutputFormat(""))
		r.AgentRegistration(reg)

		assert.Contains(t, out.String(), "ok Agent app     created  "+agentTestApp+"\n")
		assert.Contains(t, out.String(), "ok Agent record  created  "+agentTestID+" (in tokens as "+agentTestURL+")\n")
		assert.Contains(t, out.String(), "ok Grant         "+agentTestAPI+": orders:read\n")
		assert.Contains(t, out.String(), "Next\n  auth0 agents show "+agentTestID+"   to check the agent.\n")
		assert.NotContains(t, out.String(), "FIX")
	})

	t.Run("without an agent ID the next step uses the URL", func(t *testing.T) {
		noID := *reg
		noID.AgentID = ""

		r, out, _ := newAgentRenderer(OutputFormat(""))
		r.AgentRegistration(&noID)

		assert.Contains(t, out.String(), "Next\n  auth0 agents show "+agentTestURL+"   to check the agent.\n")
	})

	t.Run("a grant with other scopes is followed by what differs and the command that sets them", func(t *testing.T) {
		differing := *reg
		differing.AppCreated = false
		differing.Grants = []coreagents.GrantResult{{
			Grant:   coreagents.Grant{Audience: agentTestAPI, Scopes: []string{"orders:read"}},
			Created: false,
			Note:    "The grant has other scopes: checkout.",
			Fix:     "auth0 client-grants update cgr_1 --scopes orders:read",
		}}

		r, out, _ := newAgentRenderer(OutputFormat(""))
		r.AgentRegistration(&differing)

		assert.Contains(t, out.String(), "~  Agent app     updated  "+agentTestApp+"\n")
		assert.Contains(t, out.String(), "ok Grant         "+agentTestAPI+": orders:read  (already granted)\n")
		assert.Contains(t, out.String(), "!  The grant has other scopes: checkout.\n")
		assert.Contains(t, out.String(), "FIX  auth0 client-grants update cgr_1 --scopes orders:read\n")
		assert.Less(t, bytes.Index(out.Bytes(), []byte("(already granted)")), bytes.Index(out.Bytes(), []byte("!  The grant")))
		assert.Less(t, bytes.Index(out.Bytes(), []byte("FIX")), bytes.Index(out.Bytes(), []byte("Next")))
	})
}

func TestAgentRemoval(t *testing.T) {
	setColors(t, false)

	result := &coreagents.RemoveResult{
		URL:             agentTestURL,
		Removed:         []coreagents.Item{{Kind: coreagents.KindClientGrant, ID: "cgr_1", Audience: "https://api.shop.example"}},
		Kept:            []coreagents.Item{{Kind: coreagents.KindAgentApp, ID: "app_1"}},
		RemainingGrants: []coreagents.Grant{{ID: "cgr_9", Audience: "https://other.example"}},
	}

	tests := []struct {
		name  string
		stage AgentRemovalStage
		want  []string
	}{
		{name: "dry run", stage: AgentRemovalWould, want: []string{"This would remove:\n", "This would keep:\n"}},
		{name: "waiting for approval", stage: AgentRemovalWill, want: []string{"This will remove:\n", "This will keep:\n"}},
		{name: "done", stage: AgentRemovalDone, want: []string{"Removed:\n  ok client grant cgr_1 (https://api.shop.example)\n", "Kept:\n  -  agent app app_1\n"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, out, _ := newAgentRenderer(OutputFormat(""))
			r.AgentRemoval(test.stage, result)

			for _, want := range test.want {
				assert.Contains(t, out.String(), want)
			}

			assert.Contains(t, out.String(), "Other grants of the agent app stay. Remove them with:\n  auth0 client-grants delete cgr_9   (https://other.example)\n")
		})
	}

	t.Run("a removal that was made says what it does not do", func(t *testing.T) {
		r, out, _ := newAgentRenderer(OutputFormat(""))
		r.AgentRemoved(&coreagents.RemoveResult{Removed: []coreagents.Item{{Kind: coreagents.KindAgentApp, ID: "app_1"}}}, false)

		assert.Contains(t, out.String(), "!  The refresh tokens of the app were revoked with it. Access tokens already issued stay valid until they expire.")
		assert.Contains(t, out.String(), "!  Registering the agent again creates a new app")
	})

	t.Run("a kept app says its refresh tokens are not revoked", func(t *testing.T) {
		r, out, _ := newAgentRenderer(OutputFormat(""))
		r.AgentRemoved(&coreagents.RemoveResult{Removed: []coreagents.Item{{Kind: coreagents.KindClientGrant, ID: "cgr_1", Audience: "https://api.shop.example"}}, Kept: []coreagents.Item{{Kind: coreagents.KindAgentApp, ID: "app_1"}}}, false)

		assert.Contains(t, out.String(), "!  The agent app was kept, so its refresh tokens are not revoked.")
		assert.Contains(t, out.String(), "unregister it without --poppy")
		assert.NotContains(t, out.String(), "were revoked with it")
		assert.NotContains(t, out.String(), "Registering the agent again")
	})

	t.Run("an agent that was gone gets no such notes", func(t *testing.T) {
		r, out, _ := newAgentRenderer(OutputFormat(""))
		r.AgentRemoved(&coreagents.RemoveResult{}, true)

		assert.NotContains(t, out.String(), "revoke")
		assert.NotContains(t, out.String(), "Registering the agent again")
	})

	t.Run("no app found", func(t *testing.T) {
		r, out, _ := newAgentRenderer(OutputFormat(""))
		r.AgentRemovalGone(agentTestURL)

		assert.Equal(t, "  !  No agent app was found for "+agentTestURL+". It may already be unregistered.\n\n", out.String())
	})
}

func TestPoppyTextStream(t *testing.T) {
	setColors(t, false)

	preview := &coreagents.Preview{URL: agentTestURL, Name: agentTestName}

	tests := []struct {
		name         string
		format       OutputFormat
		wantInResult bool
	}{
		{name: "text goes to stdout", format: OutputFormat(""), wantInResult: true},
		{name: "json goes to stderr", format: OutputFormatJSON},
		{name: "compact json goes to stderr", format: OutputFormatJSONCompact},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, out, msg := newAgentRenderer(test.format)

			r.AgentPreview(preview)
			r.AgentRegisterPlan(preview, "", false, nil)
			r.PoppyDeclined("Nothing was registered.")

			text, other := out, msg
			if !test.wantInResult {
				text, other = msg, out
			}

			assert.Contains(t, text.String(), "Agent metadata")
			assert.Contains(t, text.String(), "This will")
			assert.NotContains(t, other.String(), "Agent metadata")
			assert.NotContains(t, other.String(), "This will")

			if !test.wantInResult {
				assert.Empty(t, out.String(), "only the JSON goes to stdout")
			}
		})
	}
}
