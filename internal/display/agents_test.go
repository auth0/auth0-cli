package display

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	management "github.com/auth0/go-auth0/v3/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	coreagents "github.com/auth0/auth0-cli/internal/agents"
)

func testAgentRecord() *management.AgentResponseContent {
	url := agentTestURL

	return &management.AgentResponseContent{
		AgentID:         agentTestID,
		Name:            agentTestName,
		ExternalAgentID: &url,
		Metadata:        management.AgentMetadata{"team": "ai"},
		CreatedAt:       time.Now().Add(-time.Hour),
		UpdatedAt:       time.Now().Add(-time.Minute),
	}
}

func testAgentExtras() coreagents.Extras {
	return coreagents.Extras{
		ClientID:       coreagents.Known(agentTestApp),
		MetadataStatus: coreagents.Known("available"),
		DPoPRequired:   coreagents.Known(true),
		APIs:           coreagents.Known([]coreagents.Grant{{Audience: agentTestAPI, Scopes: []string{"orders:read", "checkout"}}}),
		UnknownFields:  []string{},
	}
}

func testAgentDetail() *coreagents.DetailExtras {
	return &coreagents.DetailExtras{
		Extras:        testAgentExtras(),
		MetadataCheck: coreagents.MetadataCheck{Valid: coreagents.Known(true), Violations: []string{}, Warnings: []string{}},
		KeysCurrent:   coreagents.Known(false),
		Notes:         []string{"Could not read the grants.\nTry again."},
	}
}

// rowsOf reads the key value rows of a rendered agent, in order.
func rowsOf(v *agentView) (keys []string, values map[string]string) {
	values = map[string]string{}
	for _, row := range v.KeyValues() {
		keys = append(keys, row[0])
		values[row[0]] = row[1]
	}

	return keys, values
}

func jsonKeys(t *testing.T, raw []byte) []string {
	t.Helper()

	dec := json.NewDecoder(bytes.NewReader(raw))
	_, err := dec.Token()
	require.NoError(t, err)

	var keys []string
	for dec.More() {
		tok, err := dec.Token()
		require.NoError(t, err)
		keys = append(keys, tok.(string))

		var skip json.RawMessage
		require.NoError(t, dec.Decode(&skip))
	}

	return keys
}

func TestAgentListColumns(t *testing.T) {
	setColors(t, false)

	record := testAgentRecord()

	tests := []struct {
		name       string
		extras     *coreagents.Extras
		wantHeader []string
		wantTail   []string
	}{
		{
			name:       "without extras the columns are the ones of the agents API",
			wantHeader: []string{"ID", "Name", "External ID", "Created At"},
			wantTail:   []string{},
		},
		{
			name:       "extras add their columns after the existing ones",
			extras:     ptrTo(testAgentExtras()),
			wantHeader: []string{"ID", "Name", "External ID", "Created At", "Client ID", "Metadata", "APIs"},
			wantTail:   []string{agentTestApp, "available", agentTestAPI},
		},
		{
			name:       "nothing could be read",
			extras:     &coreagents.Extras{},
			wantHeader: []string{"ID", "Name", "External ID", "Created At", "Client ID", "Metadata", "APIs"},
			wantTail:   []string{"unknown", "unknown", "unknown"},
		},
		{
			name: "no agent app and no API",
			extras: &coreagents.Extras{
				ClientID:       coreagents.Known(""),
				MetadataStatus: coreagents.Known("none"),
				APIs:           coreagents.Known([]coreagents.Grant{}),
			},
			wantHeader: []string{"ID", "Name", "External ID", "Created At", "Client ID", "Metadata", "APIs"},
			wantTail:   []string{"-", "none", "-"},
		},
		{
			name: "several APIs on one row",
			extras: &coreagents.Extras{
				ClientID:       coreagents.Known(agentTestApp),
				MetadataStatus: coreagents.Known("available"),
				APIs:           coreagents.Known([]coreagents.Grant{{Audience: "https://a.example"}, {Audience: "https://b.example"}}),
			},
			wantHeader: []string{"ID", "Name", "External ID", "Created At", "Client ID", "Metadata", "APIs"},
			wantTail:   []string{agentTestApp, "available", "https://a.example, https://b.example"},
		},
		{
			name: "text from the document cannot forge output",
			extras: &coreagents.Extras{
				ClientID:       coreagents.Known("cli\x1b[2J"),
				MetadataStatus: coreagents.Known("available"),
				APIs:           coreagents.Known([]coreagents.Grant{{Audience: "https://a.example/\x1b]0;x\x07"}}),
			},
			wantHeader: []string{"ID", "Name", "External ID", "Created At", "Client ID", "Metadata", "APIs"},
			wantTail:   []string{`cli\x1b[2J`, "available", `https://a.example/\x1b]0;x\x07`},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			v := makeAgentView(record)
			v.extras = test.extras

			assert.Equal(t, test.wantHeader, v.AsTableHeader())

			row := v.AsTableRow()
			require.Len(t, row, len(test.wantHeader))
			assert.Equal(t, []string{agentTestID, agentTestName, agentTestURL}, row[:3])
			assert.Equal(t, test.wantTail, row[4:])
		})
	}
}

func TestAgentListRender(t *testing.T) {
	setColors(t, false)

	records := []*management.AgentResponseContent{testAgentRecord()}
	extras := []coreagents.Extras{testAgentExtras()}

	t.Run("a table", func(t *testing.T) {
		r, out, _ := newAgentRenderer(OutputFormat(""))
		r.AgentList(records, extras)

		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		require.Len(t, lines, 2)
		assert.Regexp(t, `ID\s+NAME\s+EXTERNAL ID\s+CREATED AT\s+CLIENT ID\s+METADATA\s+APIS`, lines[0])
		assert.Regexp(t, agentTestID+`\s+`+agentTestName+`\s+`+agentTestURL+`\s+.*ago\s+`+agentTestApp+`\s+available\s+`+agentTestAPI, lines[1])
	})

	t.Run("long values are cut in the table, kept in csv and json", func(t *testing.T) {
		longURL := "https://gist.example/" + strings.Repeat("a", 80) + "/agent.json"
		longAPIs := []coreagents.Grant{{Audience: "https://api.one.example"}, {Audience: "https://api.two.example"}}

		long := testAgentRecord()
		long.ExternalAgentID = &longURL
		longExtras := testAgentExtras()
		longExtras.APIs = coreagents.Known(longAPIs)

		tests := []struct {
			name     string
			format   OutputFormat
			wantCut  bool
			wantHint bool
		}{
			{name: "table", format: OutputFormat(""), wantCut: true, wantHint: true},
			{name: "csv", format: OutputFormatCSV},
			{name: "json", format: OutputFormatJSON},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				r, out, msg := newAgentRenderer(test.format)
				r.AgentList([]*management.AgentResponseContent{long}, []coreagents.Extras{longExtras})

				if test.wantCut {
					assert.NotContains(t, out.String(), longURL)
					assert.Contains(t, out.String(), longURL[:agentExternalIDWidth-3]+"...")
					assert.Contains(t, out.String(), "https://api.one.example, https://api...")
				} else {
					assert.Contains(t, out.String(), longURL)
					assert.Contains(t, out.String(), "https://api.two.example")
				}

				if test.wantHint {
					assert.Contains(t, msg.String(), "Some values were truncated for display")
					assert.Contains(t, msg.String(), "auth0 agents show "+agentTestID)
				} else {
					assert.NotContains(t, msg.String(), "truncated")
				}
			})
		}
	})

	t.Run("several cut agents get a placeholder in the hint", func(t *testing.T) {
		longURL := "https://gist.example/" + strings.Repeat("b", 80)
		first, second := testAgentRecord(), testAgentRecord()
		first.ExternalAgentID, second.ExternalAgentID = &longURL, &longURL

		r, _, msg := newAgentRenderer(OutputFormat(""))
		r.AgentList([]*management.AgentResponseContent{first, second}, nil)

		assert.Contains(t, msg.String(), "auth0 agents show <agent-id>")
	})

	t.Run("short values print no hint", func(t *testing.T) {
		r, _, msg := newAgentRenderer(OutputFormat(""))
		r.AgentList(records, extras)

		assert.NotContains(t, msg.String(), "truncated")
	})

	t.Run("csv has the same columns", func(t *testing.T) {
		r, out, _ := newAgentRenderer(OutputFormatCSV)
		r.AgentList(records, extras)

		assert.Regexp(t, `^ID,Name,External ID,Created At,Client ID,Metadata,APIs\n`, out.String())
	})

	t.Run("without extras the table is the one of the agents API", func(t *testing.T) {
		r, out, _ := newAgentRenderer(OutputFormat(""))
		r.AgentList(records, nil)

		assert.NotRegexp(t, `CLIENT ID|APIS`, out.String())
	})

	t.Run("none says how to create one", func(t *testing.T) {
		r, out, msg := newAgentRenderer(OutputFormat(""))
		r.AgentList(nil, nil)

		assert.Empty(t, out.String())
		assert.Contains(t, msg.String(), "auth0 agents create")
	})

	t.Run("json keeps the fields of the agent first, then adds the new ones", func(t *testing.T) {
		r, out, _ := newAgentRenderer(OutputFormatJSON)
		r.AgentList(records, extras)

		var got []map[string]any
		require.NoError(t, json.Unmarshal(out.Bytes(), &got))
		require.Len(t, got, 1)

		assert.Equal(t, agentTestID, got[0]["agent_id"])
		assert.Equal(t, map[string]any{"team": "ai"}, got[0]["metadata"])
		assert.Equal(t, agentTestApp, got[0]["client_id"])
		assert.Equal(t, "available", got[0]["metadata_status"])
		assert.Equal(t, true, got[0]["dpop_required"])
		assert.Equal(t, []any{map[string]any{"audience": agentTestAPI, "scopes": []any{"orders:read", "checkout"}}}, got[0]["apis"])

		var list []json.RawMessage
		require.NoError(t, json.Unmarshal(out.Bytes(), &list))
		assert.Equal(t,
			[]string{"agent_id", "name", "external_agent_id", "metadata", "created_at", "updated_at", "client_id", "metadata_status", "dpop_required", "apis", "unknown_fields"},
			jsonKeys(t, list[0]))
	})

	t.Run("json without extras is the agent as it is", func(t *testing.T) {
		r, out, _ := newAgentRenderer(OutputFormatJSON)
		r.AgentList(records, nil)

		want, err := json.MarshalIndent([]*management.AgentResponseContent{records[0]}, "", "    ")
		require.NoError(t, err)
		assert.JSONEq(t, string(want), out.String())
	})

	t.Run("a value that could not be read is null, and named in unknown_fields", func(t *testing.T) {
		unknown := []string{"client_id", "metadata_status", "dpop_required", "apis"}

		r, out, _ := newAgentRenderer(OutputFormatJSONCompact)
		r.AgentList(records, []coreagents.Extras{{UnknownFields: unknown}})

		var got []map[string]any
		require.NoError(t, json.Unmarshal(out.Bytes(), &got))
		for _, key := range unknown {
			value, present := got[0][key]
			assert.True(t, present, key)
			assert.Nil(t, value, key)
		}

		assert.Equal(t, []any{"client_id", "metadata_status", "dpop_required", "apis"}, got[0]["unknown_fields"])
	})
}

func TestAgentShowRows(t *testing.T) {
	setColors(t, false)

	t.Run("without detail the rows are the ones of the agents API", func(t *testing.T) {
		keys, _ := rowsOf(makeAgentView(testAgentRecord()))

		assert.Equal(t, []string{"ID", "NAME", "EXTERNAL ID", "METADATA", "CREATED AT", "UPDATED AT"}, keys)
	})

	t.Run("the detail rows come after the existing ones", func(t *testing.T) {
		v := makeAgentView(testAgentRecord())
		v.detail = testAgentDetail()

		keys, values := rowsOf(v)

		assert.Equal(t, []string{
			"ID", "NAME", "EXTERNAL ID", "METADATA", "CREATED AT", "UPDATED AT",
			"CLIENT ID", "METADATA STATUS", "DPOP REQUIRED", "APIS", "METADATA CHECK", "KEYS",
		}, keys)
		assert.Equal(t, agentTestApp, values["CLIENT ID"])
		assert.Equal(t, "available", values["METADATA STATUS"])
		assert.Equal(t, "required", values["DPOP REQUIRED"])
		assert.Equal(t, agentTestAPI+": orders:read checkout", values["APIS"])
		assert.Equal(t, "valid", values["METADATA CHECK"])
		assert.Equal(t, "changed since registration", values["KEYS"])
		assert.Equal(t, `{"team":"ai"}`, values["METADATA"], "the metadata of the agent keeps its meaning")
	})

	tests := []struct {
		name   string
		mutate func(d *coreagents.DetailExtras)
		want   map[string]string
	}{
		{
			name:   "nothing could be read",
			mutate: func(d *coreagents.DetailExtras) { *d = coreagents.DetailExtras{} },
			want: map[string]string{
				"CLIENT ID": "unknown", "METADATA STATUS": "unknown", "DPOP REQUIRED": "unknown",
				"APIS": "unknown", "METADATA CHECK": "unknown", "KEYS": "unknown",
			},
		},
		{
			name: "no agent app",
			mutate: func(d *coreagents.DetailExtras) {
				*d = coreagents.DetailExtras{Extras: coreagents.Extras{
					ClientID:       coreagents.Known(""),
					MetadataStatus: coreagents.Known("none"),
					APIs:           coreagents.Known([]coreagents.Grant{}),
				}}
			},
			want: map[string]string{
				"CLIENT ID": "-", "METADATA STATUS": "none", "DPOP REQUIRED": "-",
				"APIS": "-", "METADATA CHECK": "-", "KEYS": "-",
			},
		},
		{
			name:   "DPoP is not required",
			mutate: func(d *coreagents.DetailExtras) { d.DPoPRequired = coreagents.Known(false) },
			want:   map[string]string{"DPOP REQUIRED": "not required"},
		},
		{
			name: "the document is invalid",
			mutate: func(d *coreagents.DetailExtras) {
				d.MetadataCheck = coreagents.MetadataCheck{Valid: coreagents.Known(false), Violations: []string{"redirect \x1b[31mbad", "name is empty"}}
			},
			want: map[string]string{"METADATA CHECK": `invalid: redirect \x1b[31mbad; name is empty`},
		},
		{
			name:   "the keys are current",
			mutate: func(d *coreagents.DetailExtras) { d.KeysCurrent = coreagents.Known(true) },
			want:   map[string]string{"KEYS": "current"},
		},
		{
			name:   "the grants could not be read",
			mutate: func(d *coreagents.DetailExtras) { d.APIs = coreagents.Maybe[[]coreagents.Grant]{} },
			want:   map[string]string{"APIS": "unknown", "CLIENT ID": agentTestApp},
		},
		{
			name: "one line for each API",
			mutate: func(d *coreagents.DetailExtras) {
				d.APIs = coreagents.Known([]coreagents.Grant{{Audience: "https://a.example", Scopes: []string{"a"}}, {Audience: "https://b.example"}})
			},
			want: map[string]string{"APIS": "https://a.example: a\nhttps://b.example: no scopes"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			detail := testAgentDetail()
			test.mutate(detail)

			v := makeAgentView(testAgentRecord())
			v.detail = detail

			_, values := rowsOf(v)
			for key, want := range test.want {
				assert.Equal(t, want, values[key], key)
			}

			for _, value := range values {
				assert.NotContains(t, value, "\x1b")
			}
		})
	}
}

func TestAgentShowRender(t *testing.T) {
	setColors(t, false)

	record := testAgentRecord()

	t.Run("rows, then the notes and the warning about keys", func(t *testing.T) {
		r, out, msg := newAgentRenderer(OutputFormat(""))
		r.AgentShow(record, testAgentDetail())

		assert.Regexp(t, `CLIENT ID\s+`+agentTestApp, out.String())
		assert.Regexp(t, `APIS\s+`+agentTestAPI+`: orders:read checkout`, out.String())
		assert.Contains(t, msg.String(), "Could not read the grants. Try again.")
		assert.Contains(t, msg.String(), "Run `auth0 agents register "+agentTestURL+"` again.")
	})

	t.Run("no warning about keys while they are current or unknown", func(t *testing.T) {
		for _, keys := range []coreagents.Maybe[bool]{coreagents.Known(true), {}} {
			detail := testAgentDetail()
			detail.KeysCurrent, detail.Notes = keys, nil

			r, _, msg := newAgentRenderer(OutputFormat(""))
			r.AgentShow(record, detail)

			assert.NotContains(t, msg.String(), "register")
		}
	})

	t.Run("without detail it is the agent as it is", func(t *testing.T) {
		r, out, msg := newAgentRenderer(OutputFormat(""))
		r.AgentShow(record, nil)

		assert.NotContains(t, out.String(), "CLIENT ID")
		assert.NotContains(t, msg.String(), "register")
	})

	t.Run("json keeps the fields of the agent first, then adds the new ones", func(t *testing.T) {
		r, out, msg := newAgentRenderer(OutputFormatJSON)
		r.AgentShow(record, testAgentDetail())

		assert.Empty(t, msg.String(), "no decoration beside JSON")

		var got map[string]any
		require.NoError(t, json.Unmarshal(out.Bytes(), &got))
		assert.Equal(t, agentTestApp, got["client_id"])
		assert.Equal(t, false, got["keys_current"])
		assert.Equal(t, map[string]any{"valid": true, "violations": []any{}, "warnings": []any{}}, got["metadata_check"])
		assert.Equal(t, []any{"Could not read the grants.\nTry again."}, got["notes"])

		assert.Equal(t,
			[]string{
				"agent_id", "name", "external_agent_id", "metadata", "created_at", "updated_at",
				"client_id", "metadata_status", "dpop_required", "apis", "unknown_fields", "metadata_check", "keys_current", "notes",
			},
			jsonKeys(t, out.Bytes()))
	})

	t.Run("json without detail is the agent as it is", func(t *testing.T) {
		r, out, _ := newAgentRenderer(OutputFormatJSON)
		r.AgentShow(record, nil)

		want, err := json.MarshalIndent(record, "", "    ")
		require.NoError(t, err)
		assert.JSONEq(t, string(want), out.String())
	})
}

func TestJSONWithFields(t *testing.T) {
	type fields struct {
		A string `json:"a"`
	}

	tests := []struct {
		name    string
		base    any
		want    string
		wantErr bool
	}{
		{name: "fields are added after the object's own", base: map[string]int{"x": 1}, want: `{"x":1,"a":"b"}`},
		{name: "an empty object", base: struct{}{}, want: `{"a":"b"}`},
		{name: "not an object", base: []int{1}, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := json.Marshal(jsonWithFields{base: test.base, fields: fields{A: "b"}})

			if test.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, string(got))
		})
	}
}

func ptrTo[T any](value T) *T { return &value }
