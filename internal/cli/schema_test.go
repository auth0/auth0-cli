package cli

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaRequestedAndRequiredBypass(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantSchema    bool
		wantBypass    bool
		withoutSchema bool
	}{
		{name: "no flags", args: nil},
		{name: "--schema", args: []string{"--schema"}, wantSchema: true, wantBypass: true},
		{name: "--schema=true", args: []string{"--schema=true"}, wantSchema: true, wantBypass: true},
		{name: "--schema=false", args: []string{"--schema=false"}},
		{name: "--schema=false with --data", args: []string{"--schema=false", "--data", "{}"}, wantBypass: true},
		{name: "--data", args: []string{"--data", "{}"}, wantBypass: true},
		{name: "--query", args: []string{"--query", "{}"}, wantBypass: true},
		{name: "command without --schema", args: nil, withoutSchema: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var schema bool
			var data, query string
			cmd := &cobra.Command{Use: "test"}
			if !tc.withoutSchema {
				schemaFlag.RegisterBool(cmd, &schema, false)
			}
			dataFlag.RegisterString(cmd, &data, "")
			listQueryFlag.RegisterString(cmd, &query, "")
			require.NoError(t, cmd.ParseFlags(tc.args))

			assert.Equal(t, tc.wantSchema, schemaRequested(cmd))
			assert.Equal(t, tc.wantBypass, hasRequiredBypassFlag(cmd))
		})
	}
}

func TestTenantSettingsUpdateSetCmdSchemaFalseRunsTheCommand(t *testing.T) {
	useTenantSettingsFixtureSchema(t)

	stub := &jsonWriteStub{response: tenantSettingsResponse}
	cmd := set(newJSONWriteCLI(stub))
	prepareInteractivity(cmd)
	cmd.SetArgs([]string{"--schema=false", "--data", tenantSettingsData})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, tenantSettingsData, stub.gotBody, "--schema=false must send the payload")
}
