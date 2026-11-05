package display

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/poppy/agents"
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

func testAgent() agents.Agent {
	return agents.Agent{
		AgentID:        agentTestID,
		Name:           agentTestName,
		URL:            agentTestURL,
		ClientID:       agentTestApp,
		MetadataStatus: "available",
		DPoPRequired:   agents.Maybe[bool]{Known: true, Value: true},
		APIs:           agents.Maybe[[]agents.Grant]{Known: true, Value: []agents.Grant{{Audience: agentTestAPI, Scopes: []string{"orders:read"}}}},
	}
}

func TestPoppyAgentViewsEscapeUntrustedText(t *testing.T) {
	agent := agents.Agent{
		AgentID:        agentTestID,
		Name:           "Muse\x1b[2J\n✓ forged",
		URL:            "https://muse.example/\x1b]0;x\x07agent.json",
		ClientID:       agentTestApp,
		MetadataStatus: "available",
	}

	row := poppyAgentView{agent}.AsTableRow()
	assert.Equal(t, `Muse\x1b[2J ✓ forged`, row[1])
	assert.Equal(t, `https://muse.example/\x1b]0;x\x07agent.json`, row[2])

	detail := poppyAgentDetailView{&agents.Detail{
		Agent:         agent,
		MetadataCheck: agents.MetadataCheck{Valid: agents.Maybe[bool]{Known: true}, Violations: []string{"redirect \x1b[31mbad"}},
	}}
	got := map[string]string{}
	for _, row := range detail.KeyValues() {
		got[row[0]] = row[1]
	}
	assert.Equal(t, `Muse\x1b[2J ✓ forged`, got["NAME"])
	assert.Equal(t, `https://muse.example/\x1b]0;x\x07agent.json`, got["URL"])
	assert.Equal(t, `invalid: redirect \x1b[31mbad`, got["DOCUMENT"])

	for _, row := range detail.KeyValues() {
		assert.NotContains(t, row[1], "\x1b")
	}
}

func TestDescribePoppyKeys(t *testing.T) {
	assert.Equal(t, "0 ()", describePoppyKeys(nil))
	assert.Equal(t, "3 (PS256, RS256)", describePoppyKeys([]agents.Key{{Alg: "RS256"}, {Alg: "PS256"}, {Alg: "RS256"}}))
}

func TestPoppyAgentList(t *testing.T) {
	setColors(t, false)

	list := []agents.Agent{testAgent()}

	t.Run("a table with one row for each agent", func(t *testing.T) {
		r, out, _ := newAgentRenderer(OutputFormat(""))
		r.PoppyAgentList(list)

		lines := bytes.Split(bytes.TrimSpace(out.Bytes()), []byte("\n"))
		require.Len(t, lines, 2, "the header and the row")
		assert.Regexp(t, `AGENT ID\s+NAME\s+URL\s+METADATA\s+APIS`, string(lines[0]))
		assert.Regexp(t, agentTestID+`\s+`+agentTestName+`\s+`+agentTestURL+`\s+available\s+`+agentTestAPI, string(lines[1]))
	})

	t.Run("a heading with the count goes to the messages", func(t *testing.T) {
		r, _, msg := newAgentRenderer(OutputFormat(""))
		r.PoppyAgentList(list)

		assert.Contains(t, msg.String(), "agents (1)")
	})

	t.Run("json is the agents as they are", func(t *testing.T) {
		r, out, msg := newAgentRenderer(OutputFormatJSON)
		r.PoppyAgentList(list)

		want, err := json.MarshalIndent(list, "", "    ")
		require.NoError(t, err)

		assert.JSONEq(t, string(want), out.String())
		assert.Empty(t, msg.String(), "no decoration beside JSON")
	})

	t.Run("none says how to register one", func(t *testing.T) {
		r, out, msg := newAgentRenderer(OutputFormat(""))
		r.PoppyAgentList(nil)

		assert.Empty(t, out.String())
		assert.Contains(t, msg.String(), "No agents available. Register one with: auth0 poppy agents register <agent-url>")
	})

	t.Run("none is an empty array in json", func(t *testing.T) {
		r, out, _ := newAgentRenderer(OutputFormatJSON)
		r.PoppyAgentList(nil)

		assert.JSONEq(t, "[]", out.String())
	})
}

func TestPoppyAgentShow(t *testing.T) {
	setColors(t, false)

	detail := &agents.Detail{
		Agent:         testAgent(),
		MetadataCheck: agents.MetadataCheck{Valid: agents.Maybe[bool]{Known: true, Value: true}},
		KeysCurrent:   agents.Maybe[bool]{Known: true},
		Notes:         []string{"Could not read the grants.\nTry again."},
	}

	t.Run("key value rows, then the notes and the warning about keys", func(t *testing.T) {
		r, out, msg := newAgentRenderer(OutputFormat(""))
		r.PoppyAgentShow(detail)

		for label, value := range map[string]string{
			"AGENT ID":  agentTestID,
			"NAME":      agentTestName,
			"URL":       agentTestURL,
			"AGENT APP": agentTestApp,
			"METADATA":  "available",
			"DPOP":      "required",
			"APIS":      agentTestAPI,
			"DOCUMENT":  "valid",
			"KEYS":      "changed since registration",
		} {
			assert.Regexp(t, label+`\s+`+value, out.String())
		}

		assert.Contains(t, msg.String(), "Could not read the grants. Try again.")
		assert.Contains(t, msg.String(), "Run `auth0 poppy agents register "+agentTestURL+"` again.")
	})

	t.Run("json is the detail as it is", func(t *testing.T) {
		r, out, _ := newAgentRenderer(OutputFormatJSON)
		r.PoppyAgentShow(detail)

		want, err := json.MarshalIndent(detail, "", "    ")
		require.NoError(t, err)

		assert.JSONEq(t, string(want), out.String())
	})
}

func TestPoppyAgentPreviewAndPlan(t *testing.T) {
	setColors(t, false)

	preview := &agents.Preview{
		URL:          agentTestURL,
		Name:         agentTestName,
		RedirectURLs: []string{"https://muse.example/cb"},
		Keys:         []agents.Key{{Alg: "PS256"}},
		Warnings:     []string{"No logo."},
		ClientID:     agentTestApp,
	}

	r, out, _ := newAgentRenderer(OutputFormat(""))
	r.PoppyAgentPreview(preview)

	assert.Contains(t, out.String(), "Agent metadata  "+agentTestURL+"\n")
	assert.Contains(t, out.String(), "name           "+agentTestName+"\n")
	assert.Contains(t, out.String(), "keys           1 (PS256)\n")
	assert.Contains(t, out.String(), "!  No logo.\n")
	assert.Contains(t, out.String(), "An agent app already exists for this URL ("+agentTestApp+"). It will be updated.")

	out.Reset()
	r.PoppyRegisterPlan(preview, "", true, []agents.Grant{{Audience: agentTestAPI, Scopes: []string{"orders:read"}}})

	assert.Contains(t, out.String(), "\nThis will\n")
	assert.Contains(t, out.String(), "~  Agent app     update, for "+agentTestURL+"\n")
	assert.Contains(t, out.String(), "+  DPoP          required on the agent app\n")
	assert.Contains(t, out.String(), `+  Agent record  "`+agentTestName+`"`+"\n")
	assert.Contains(t, out.String(), "+  Grant         "+agentTestAPI+": orders:read  (skipped if the agent app already has a grant)\n")
}

func TestPoppyRegistration(t *testing.T) {
	setColors(t, false)

	reg := &agents.Registration{
		URL:          agentTestURL,
		Name:         agentTestName,
		ClientID:     agentTestApp,
		AppCreated:   true,
		AgentID:      agentTestID,
		AgentCreated: true,
		Grants:       []agents.GrantResult{{Grant: agents.Grant{Audience: agentTestAPI, Scopes: []string{"orders:read"}}, Created: true}},
	}

	t.Run("what was made, and what to do next", func(t *testing.T) {
		r, out, _ := newAgentRenderer(OutputFormat(""))
		r.PoppyRegistration(reg)

		assert.Contains(t, out.String(), "ok Agent app     created  "+agentTestApp+"\n")
		assert.Contains(t, out.String(), "ok Agent record  created  "+agentTestID+" (in tokens as "+agentTestURL+")\n")
		assert.Contains(t, out.String(), "ok Grant         "+agentTestAPI+": orders:read\n")
		assert.Contains(t, out.String(), "Next\n  auth0 poppy agents show "+agentTestURL+"   to check the agent.\n")
		assert.NotContains(t, out.String(), "FIX")
	})

	t.Run("a grant with other scopes is followed by what differs and the command that sets them", func(t *testing.T) {
		differing := *reg
		differing.AppCreated = false
		differing.Grants = []agents.GrantResult{{
			Grant:   agents.Grant{Audience: agentTestAPI, Scopes: []string{"orders:read"}},
			Created: false,
			Note:    "The grant has other scopes: checkout.",
			Fix:     "auth0 client-grants update cgr_1 --scopes orders:read",
		}}

		r, out, _ := newAgentRenderer(OutputFormat(""))
		r.PoppyRegistration(&differing)

		assert.Contains(t, out.String(), "~  Agent app     updated  "+agentTestApp+"\n")
		assert.Contains(t, out.String(), "ok Grant         "+agentTestAPI+": orders:read  (already granted)\n")
		assert.Contains(t, out.String(), "!  The grant has other scopes: checkout.\n")
		assert.Contains(t, out.String(), "FIX  auth0 client-grants update cgr_1 --scopes orders:read\n")
		assert.Less(t, bytes.Index(out.Bytes(), []byte("(already granted)")), bytes.Index(out.Bytes(), []byte("!  The grant")))
		assert.Less(t, bytes.Index(out.Bytes(), []byte("FIX")), bytes.Index(out.Bytes(), []byte("Next")))
	})
}

func TestPoppyRemoval(t *testing.T) {
	setColors(t, false)

	result := &agents.RemoveResult{
		URL:             agentTestURL,
		Removed:         []string{"the grant of https://api.shop.example"},
		Kept:            []string{"the agent app"},
		RemainingGrants: []agents.Grant{{ID: "cgr_9", Audience: "https://other.example"}},
	}

	tests := []struct {
		name  string
		stage PoppyRemovalStage
		want  []string
	}{
		{name: "dry run", stage: PoppyRemovalWould, want: []string{"This would remove:\n", "This would keep:\n"}},
		{name: "waiting for approval", stage: PoppyRemovalWill, want: []string{"This will remove:\n", "This will keep:\n"}},
		{name: "done", stage: PoppyRemovalDone, want: []string{"Removed:\n  ok the grant of https://api.shop.example\n", "Kept:\n  -  the agent app\n"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, out, _ := newAgentRenderer(OutputFormat(""))
			r.PoppyRemoval(test.stage, result)

			for _, want := range test.want {
				assert.Contains(t, out.String(), want)
			}

			assert.Contains(t, out.String(), "Other grants of the agent app stay. Remove them with:\n  auth0 client-grants delete cgr_9   (https://other.example)\n")
		})
	}

	t.Run("a removal that was made says what it does not do", func(t *testing.T) {
		r, out, _ := newAgentRenderer(OutputFormat(""))
		r.PoppyRemoved(&agents.RemoveResult{Removed: []string{"the agent app"}}, false)

		assert.Contains(t, out.String(), "!  This command does not revoke tokens already issued")
		assert.Contains(t, out.String(), "!  Registering the agent again creates a new app")
	})

	t.Run("an agent that was gone gets no such notes", func(t *testing.T) {
		r, out, _ := newAgentRenderer(OutputFormat(""))
		r.PoppyRemoved(&agents.RemoveResult{}, true)

		assert.NotContains(t, out.String(), "revoke")
		assert.NotContains(t, out.String(), "Registering the agent again")
	})

	t.Run("no app found", func(t *testing.T) {
		r, out, _ := newAgentRenderer(OutputFormat(""))
		r.PoppyRemovalGone(agentTestURL)

		assert.Equal(t, "  !  No agent app was found for "+agentTestURL+". It may already be removed.\n\n", out.String())
	})
}
