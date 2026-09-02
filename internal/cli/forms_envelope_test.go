package cli

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/golang/mock/gomock"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
)

func TestIsFormEnvelope(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{name: "envelope with form object", body: `{"version":"4.0.0","form":{"name":"x"}}`, want: true},
		{name: "flat form graph", body: `{"name":"x","nodes":[]}`, want: false},
		{name: "form as non-object is still detected", body: `{"form":{}}`, want: true},
		{name: "invalid json", body: `not-json`, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, isFormEnvelope([]byte(test.body)))
		})
	}
}

func TestSubstituteIDs(t *testing.T) {
	t.Run("replaces exact string matches anywhere in the tree", func(t *testing.T) {
		in := json.RawMessage(`{"flow_id":"fl_1","nested":{"connection_id":"ac_1","keep":"fl_1x"},"list":["fl_1","other"]}`)
		out, err := substituteIDs(in, map[string]string{"fl_1": "#FLOW-1#", "ac_1": "#CONN-1#"})
		require.NoError(t, err)

		var got map[string]interface{}
		require.NoError(t, json.Unmarshal(out, &got))
		assert.Equal(t, "#FLOW-1#", got["flow_id"])
		nested := got["nested"].(map[string]interface{})
		assert.Equal(t, "#CONN-1#", nested["connection_id"])
		assert.Equal(t, "fl_1x", nested["keep"]) // Substring must not be replaced.
		list := got["list"].([]interface{})
		assert.Equal(t, "#FLOW-1#", list[0])
		assert.Equal(t, "other", list[1])
	})

	t.Run("returns the input unchanged when there are no replacements", func(t *testing.T) {
		in := json.RawMessage(`{"a":"b"}`)
		out, err := substituteIDs(in, nil)
		require.NoError(t, err)
		assert.Equal(t, in, out)
	})
}

func TestCollectConnectionIDs(t *testing.T) {
	raw := json.RawMessage(`{
		"name": "flow",
		"actions": [
			{"params": {"connection_id": "ac_2"}},
			{"params": {"connection_id": "ac_1"}},
			{"params": {"connection_id": "ac_1"}},
			{"params": {"other": "x"}}
		]
	}`)

	got, err := collectConnectionIDs(raw)
	require.NoError(t, err)
	assert.Equal(t, []string{"ac_1", "ac_2"}, got) // De-duplicated and sorted.
}

func TestBuildFormEnvelope(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	form := json.RawMessage(`{
		"id": "ap_form1",
		"name": "Test Form",
		"nodes": [
			{"id": "step_1", "type": "STEP", "config": {"next_node": "flow_1", "components": [{"id": "job_title", "type": "TEXT", "category": "FIELD"}]}},
			{"id": "flow_1", "type": "FLOW", "config": {"flow_id": "fl_real", "next_node": "$ending"}}
		],
		"start": {"next_node": "step_1"},
		"ending": {}
	}`)

	flowMock := mock.NewMockFlowAPI(ctrl)
	flowMock.EXPECT().Read(gomock.Any(), "fl_real").Return(&management.Flow{
		Name: auth0.String("My Flow"),
		Actions: []interface{}{
			map[string]interface{}{
				"type":   "AUTH0",
				"params": map[string]interface{}{"connection_id": "ac_real"},
			},
		},
	}, nil)

	connMock := mock.NewMockFlowVaultConnectionAPI(ctrl)
	connMock.EXPECT().GetConnection(gomock.Any(), "ac_real").Return(&management.FlowVaultConnection{
		ID:    auth0.String("ac_real"),
		AppID: auth0.String("AUTH0"),
		Name:  auth0.String("My Connection"),
	}, nil)

	cli := &cli{api: &auth0.API{Flow: flowMock, FlowVaultConnection: connMock}}

	env, err := cli.buildFormEnvelope(context.Background(), form)
	require.NoError(t, err)

	assert.Equal(t, formEnvelopeVersion, env.Version)

	// Connection descriptor keeps the real values under the placeholder key.
	require.Contains(t, env.Connections, "#CONN-1#")
	assert.Equal(t, "ac_real", env.Connections["#CONN-1#"].ID)
	assert.Equal(t, "AUTH0", env.Connections["#CONN-1#"].AppID)
	assert.Equal(t, "My Connection", env.Connections["#CONN-1#"].Name)

	// The flow node's flow_id is replaced with the placeholder, and volatile
	// fields are dropped from the form block.
	var formMap map[string]interface{}
	require.NoError(t, json.Unmarshal(env.Form, &formMap))
	assert.NotContains(t, formMap, "id")
	nodes := formMap["nodes"].([]interface{})
	flowNode := nodes[1].(map[string]interface{})
	flowConfig := flowNode["config"].(map[string]interface{})
	assert.Equal(t, "#FLOW-1#", flowConfig["flow_id"])

	// STEP node config (components) is preserved, not dropped by the SDK's union.
	stepNode := nodes[0].(map[string]interface{})
	stepConfig := stepNode["config"].(map[string]interface{})
	components := stepConfig["components"].([]interface{})
	require.Len(t, components, 1)
	assert.Equal(t, "job_title", components[0].(map[string]interface{})["id"])

	// The flow's connection_id is replaced with the placeholder.
	require.Contains(t, env.Flows, "#FLOW-1#")
	var flowMap map[string]interface{}
	require.NoError(t, json.Unmarshal(env.Flows["#FLOW-1#"], &flowMap))
	action := flowMap["actions"].([]interface{})[0].(map[string]interface{})
	params := action["params"].(map[string]interface{})
	assert.Equal(t, "#CONN-1#", params["connection_id"])
}

func TestResolveFormEnvelope(t *testing.T) {
	envelope := []byte(`{
		"version": "4.0.0",
		"form": {
			"name": "Test Form",
			"nodes": [
				{"id": "flow_1", "type": "FLOW", "config": {"flow_id": "#FLOW-1#"}}
			]
		},
		"flows": {
			"#FLOW-1#": {
				"name": "My Flow",
				"actions": [{"params": {"connection_id": "#CONN-1#"}}]
			}
		},
		"connections": {
			"#CONN-1#": {"id": "ac_placeholder", "app_id": "AUTH0", "name": "REPLACE_WITH_M2M_CONNECTION"}
		}
	}`)

	t.Run("maps connections, creates flows, and substitutes flow IDs", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		flowMock := mock.NewMockFlowAPI(ctrl)
		flowMock.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, r *management.Flow, _ ...management.RequestOption) error {
				// The connection placeholder is resolved before the flow is created.
				action := r.Actions[0].(map[string]interface{})
				params := action["params"].(map[string]interface{})
				assert.Equal(t, "ac_mapped", params["connection_id"])
				r.ID = auth0.String("fl_created")
				return nil
			})

		cli := &cli{api: &auth0.API{Flow: flowMock}}

		body, err := cli.resolveFormEnvelope(&cobra.Command{}, envelope, map[string]string{"#CONN-1#": "ac_mapped"})
		require.NoError(t, err)

		var formMap map[string]interface{}
		require.NoError(t, json.Unmarshal(body, &formMap))
		node := formMap["nodes"].([]interface{})[0].(map[string]interface{})
		config := node["config"].(map[string]interface{})
		assert.Equal(t, "fl_created", config["flow_id"])
	})

	t.Run("errors when a connection cannot be resolved without a terminal", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cli := &cli{api: &auth0.API{Flow: mock.NewMockFlowAPI(ctrl)}}

		_, err := cli.resolveFormEnvelope(&cobra.Command{}, envelope, nil)
		assert.ErrorContains(t, err, "cannot resolve connection #CONN-1#")
	})
}
