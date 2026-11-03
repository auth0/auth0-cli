package cli

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
)

func TestLogsListCmdInvalidQuery(t *testing.T) {
	cli := &cli{
		renderer: &display.Renderer{MessageWriter: io.Discard, ResultWriter: io.Discard},
		api:      &auth0.API{},
	}

	cmd := listLogsCmd(cli)
	cmd.SetArgs([]string{"--query", "not-valid-json"})

	assert.ErrorContains(t, cmd.Execute(), "invalid --query value")
}
