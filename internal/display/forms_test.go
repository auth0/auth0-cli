package display

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormShowRawPreservesRichGraphJSON(t *testing.T) {
	stdout := &bytes.Buffer{}
	renderer := &Renderer{
		MessageWriter: io.Discard,
		ResultWriter:  stdout,
		Format:        OutputFormatJSON,
	}
	body := json.RawMessage(`{
		"id":"ap_rich",
		"name":"Rich Form",
		"flow_count":2,
		"links":{"self":"https://example.test/forms/ap_rich"},
		"nodes":[
			{"id":"step_1","type":"STEP","config":{"components":[{"id":"field_1","category":"FIELD","type":"TEXT"}]}},
			{"id":"router_1","type":"ROUTER","config":{"rules":[{"id":"rule_1","condition":{"operator":"AND"}}]}}
		]
	}`)

	require.NoError(t, renderer.FormShowRaw(body))

	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
	assert.Equal(t, float64(2), got["flow_count"])
	assert.Contains(t, got, "links")

	nodes := got["nodes"].([]interface{})
	step := nodes[0].(map[string]interface{})
	assert.Contains(t, step["config"].(map[string]interface{}), "components")
	router := nodes[1].(map[string]interface{})
	rules := router["config"].(map[string]interface{})["rules"].([]interface{})
	assert.Contains(t, rules[0].(map[string]interface{}), "condition")
}

func TestFormShowRawRejectsInvalidJSON(t *testing.T) {
	renderer := &Renderer{MessageWriter: io.Discard, ResultWriter: io.Discard}
	err := renderer.FormShowRaw(json.RawMessage(`not-json`))
	assert.ErrorContains(t, err, "failed to parse form response")
}
