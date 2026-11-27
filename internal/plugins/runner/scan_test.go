package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cliFlags are the flags the CLI owns, as the pre-scan sees them.
var cliFlags = []OwnedFlag{
	{Name: "tenant", Short: "t"},
	{Name: "debug", Bool: true},
	{Name: "no-input", Bool: true},
	{Name: "json", Bool: true},
	{Name: "json-compact", Bool: true},
	{Name: "csv", Bool: true},
}

func TestScan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		args   []string
		values map[string]string
		rest   []string
		help   bool
	}{
		{"no arguments", nil, map[string]string{}, []string{}, false},
		{"nothing owned", []string{"scan", "--validators", "a"}, map[string]string{}, []string{"scan", "--validators", "a"}, false},
		{"a long flag with a separate value", []string{"--tenant", "acme", "scan"}, map[string]string{"tenant": "acme"}, []string{"scan"}, false},
		{"a long flag with an equals value", []string{"--tenant=acme", "scan"}, map[string]string{"tenant": "acme"}, []string{"scan"}, false},
		{"a short flag", []string{"-t", "acme", "scan"}, map[string]string{"tenant": "acme"}, []string{"scan"}, false},
		{"a short flag with an equals value", []string{"-t=acme"}, map[string]string{"tenant": "acme"}, []string{}, false},
		{"a value that starts with a dash", []string{"--tenant", "-x"}, map[string]string{"tenant": "-x"}, []string{}, false},
		{"an empty value after an equals sign", []string{"--tenant="}, map[string]string{"tenant": ""}, []string{}, false},
		{"an empty value as a separate argument", []string{"--tenant", "", "scan"}, map[string]string{"tenant": ""}, []string{"scan"}, false},
		{"a bool flag", []string{"--json", "scan"}, map[string]string{"json": "true"}, []string{"scan"}, false},
		{"a bool flag never takes the next argument", []string{"--debug", "false", "scan"}, map[string]string{"debug": "true"}, []string{"false", "scan"}, false},
		{"a bool flag set to true", []string{"--debug=true"}, map[string]string{"debug": "true"}, []string{}, false},
		{"a bool flag set to false", []string{"--debug=false"}, map[string]string{"debug": "false"}, []string{}, false},
		{"a bool value in another spelling", []string{"--debug=1", "--csv=F"}, map[string]string{"debug": "true", "csv": "false"}, []string{}, false},
		{"a flag that repeats takes the last value", []string{"--tenant", "a", "--tenant=b", "--json", "--json=false"}, map[string]string{"tenant": "b", "json": "false"}, []string{}, false},
		{"flags between the arguments of the plugin", []string{"scan", "--tenant", "acme", "--validators", "a", "--json", "b"}, map[string]string{"tenant": "acme", "json": "true"}, []string{"scan", "--validators", "a", "b"}, false},
		{"an unknown flag and its value are untouched", []string{"--unknown", "--tenant", "acme", "value"}, map[string]string{"tenant": "acme"}, []string{"--unknown", "value"}, false},
		{"a flag of the plugin that has an owned name as a prefix", []string{"--tenants", "a", "--json-compacts", "--debugger"}, map[string]string{}, []string{"--tenants", "a", "--json-compacts", "--debugger"}, false},
		{"a short flag that is not owned", []string{"-v", "-x=1"}, map[string]string{}, []string{"-v", "-x=1"}, false},
		{"a short flag in a cluster or with a glued value", []string{"-tacme", "-vt"}, map[string]string{}, []string{"-tacme", "-vt"}, false},
		{"a lone dash", []string{"-", "scan"}, map[string]string{}, []string{"-", "scan"}, false},
		{"a lone equals form", []string{"--=x", "-=x"}, map[string]string{}, []string{"--=x", "-=x"}, false},
		{"a long flag with one dash is not the long flag", []string{"-tenant", "acme"}, map[string]string{}, []string{"-tenant", "acme"}, false},
		{"a short name with two dashes is not the short flag", []string{"--t", "acme"}, map[string]string{}, []string{"--t", "acme"}, false},
		{"a double dash stops the scan and is kept", []string{"scan", "--", "--tenant", "acme"}, map[string]string{}, []string{"scan", "--", "--tenant", "acme"}, false},
		{"flags before a double dash still count", []string{"--tenant", "acme", "--", "--json", "-h"}, map[string]string{"tenant": "acme"}, []string{"--", "--json", "-h"}, false},
		{"a second double dash is kept as is", []string{"--", "--", "x"}, map[string]string{}, []string{"--", "--", "x"}, false},
		{"a flag that looks owned after a double dash", []string{"--", "--debug=false", "-t", "x"}, map[string]string{}, []string{"--", "--debug=false", "-t", "x"}, false},
		{"a short help flag", []string{"-h"}, map[string]string{}, []string{}, true},
		{"a long help flag", []string{"scan", "--help"}, map[string]string{}, []string{"scan"}, true},
		{"help set to false", []string{"--help=false"}, map[string]string{}, []string{}, false},
		{"help after a double dash is the plugin's", []string{"--", "-h", "--help"}, map[string]string{}, []string{"--", "-h", "--help"}, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			original := append([]string(nil), test.args...)

			scanned, err := Scan(test.args, cliFlags)

			require.NoError(t, err)
			assert.Equal(t, test.values, scanned.Values)
			assert.Equal(t, test.rest, scanned.Rest)
			assert.Equal(t, test.help, scanned.Help)
			assert.Equal(t, original, append([]string(nil), test.args...), "the arguments were changed")
		})
	}
}

func TestScan_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"a flag with no value at the end", []string{"scan", "--tenant"}, "the flag --tenant needs a value"},
		{"a short flag with no value at the end", []string{"-t"}, "the flag -t needs a value"},
		{"a flag whose value would be the double dash", []string{"--tenant", "--", "x"}, "the flag --tenant needs a value"},
		{"a bool flag with a value that is not a bool", []string{"--json=maybe"}, `invalid value "maybe" for --json`},
		{"a bool flag with an empty value", []string{"--debug="}, `invalid value "" for --debug`},
		{"help with a value that is not a bool", []string{"--help=please"}, `invalid value "please" for --help`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			scanned, err := Scan(test.args, cliFlags)

			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
			assert.Empty(t, scanned.Rest)
		})
	}
}

func TestScan_TheHelpFlagBelongsToTheCLI(t *testing.T) {
	t.Parallel()

	// An owned flag cannot take over -h or --help.
	scanned, err := Scan([]string{"-h", "--help"}, []OwnedFlag{{Name: "help", Short: "h"}, {Name: "other", Short: "h"}})

	require.NoError(t, err)
	assert.True(t, scanned.Help)
	assert.Empty(t, scanned.Values)
}
