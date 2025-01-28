package authutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildLoginURL(t *testing.T) {
	url, err := BuildLoginURL("cli-demo.us.auth0.com", "some-client-id", "http://localhost:8484", "some-state", "some-conn", "some-aud", "none", []string{"some-scope", "some-other-scope"}, map[string]string{"foo": "bar", "bazz": "buzz"})

	assert.NoError(t, err)
	assert.Equal(t, url, "https://cli-demo.us.auth0.com/authorize?audience=some-aud&bazz=buzz&client_id=some-client-id&connection=some-conn&foo=bar&prompt=none&redirect_uri=http%3A%2F%2Flocalhost%3A8484&response_type=code&scope=some-scope+some-other-scope&state=some-state")
}
