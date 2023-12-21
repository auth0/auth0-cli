package display

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/stretchr/testify/assert"
)

func TestRenderer_MembersList_json(t *testing.T) {
	var input []management.OrganizationMember
	for _, id := range []string{"1234", "5678"} {
		nid := id
		input = append(input, management.OrganizationMember{UserID: &nid})
	}

	var exp []interface{}
	raw, _ := json.Marshal(input)
	_ = json.Unmarshal(raw, &exp)

	r := NewRenderer()
	r.Format = OutputFormatJSON

	out := bytes.NewBuffer([]byte{})
	r.ResultWriter = out
	r.MembersList(input)
	buf, _ := io.ReadAll(out)

	var got []interface{}
	_ = json.Unmarshal(buf, &got)

	assert.Equal(t, exp, got)
}
