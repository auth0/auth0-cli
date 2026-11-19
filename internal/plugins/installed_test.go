package plugins

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseInstalledManifest_Accepts(t *testing.T) {
	t.Run("returns the same result as ParseManifest for a valid manifest", func(t *testing.T) {
		data := readValidManifest(t)

		strict, err := ParseManifest(data)
		require.NoError(t, err)

		installed, err := ParseInstalledManifest(data)
		require.NoError(t, err)

		assert.Equal(t, strict, installed)
		assert.Equal(t, strict.PermissionsHash(), installed.PermissionsHash())
	})

	tests := []struct {
		name   string
		mutate func(manifest map[string]any)
	}{
		{name: "an unknown top-level field", mutate: setValue(true, "futureField")},
		{name: "an unknown owner field", mutate: setValue("#checkmate", "owner", "slack")},
		{name: "an unknown engines field", mutate: setValue(">=20.0.0", "engines", "node")},
		{name: "an unknown command field", mutate: setCommand(true, "hidden")},
		{name: "an unknown output field", mutate: setCommand(true, "output", "xml")},
		{name: "an unknown flag field", mutate: setCommand("all", "flags", 0, "default")},
	}

	for _, test := range tests {
		t.Run("accepts "+test.name+" that ParseManifest rejects", func(t *testing.T) {
			data := mutateManifest(t, test.mutate)

			_, err := ParseManifest(data)
			requireValidationError(t, err, "unknown field")

			installed, err := ParseInstalledManifest(data)
			require.NoError(t, err)

			strict, err := ParseManifest(readValidManifest(t))
			require.NoError(t, err)
			assert.Equal(t, strict, installed)
		})
	}
}

func TestParseInstalledManifest_StillValidates(t *testing.T) {
	tests := []manifestCase{
		{
			name:     "applies the normal validation to known fields",
			mutate:   combine(setValue(true, "futureField"), setCommand("0s", "timeout")),
			wantErrs: []string{"commands[0].timeout:"},
		},
		{name: "rejects a field of the wrong type", mutate: setValue(1, "version"), wantErrs: []string{"version:"}},
		{name: "rejects a missing required field", mutate: removeValue("id"), wantErrs: []string{"id:"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest, err := ParseInstalledManifest(mutateManifest(t, test.mutate))
			assert.Nil(t, manifest)
			requireValidationError(t, err, test.wantErrs...)
		})
	}
}

func TestParseInstalledManifest_FailsClosedOnUnknownPermissions(t *testing.T) {
	tests := []manifestCase{
		{
			name:     "rejects an unknown field under permissions",
			mutate:   setCommand([]any{}, "permissions", "network"),
			wantErrs: []string{"commands[0].permissions.network:"},
		},
		{
			name:     "rejects an unknown key inside a hosts entry",
			mutate:   setCommand([]any{443}, "permissions", "hosts", 0, "ports"),
			wantErrs: []string{"commands[0].permissions.hosts[0].ports:"},
		},
		{
			name:     "rejects an unknown key inside a files entry",
			mutate:   setCommand(true, "permissions", "files", 0, "recursive"),
			wantErrs: []string{"commands[0].permissions.files[0].recursive:"},
		},
		{
			name:     "rejects an unknown key inside an env entry",
			mutate:   setCommand(true, "permissions", "env", 0, "secret"),
			wantErrs: []string{"commands[0].permissions.env[0].secret:"},
		},
		{
			name: "rejects unknown permission fields in every command",
			mutate: setValue([]any{
				map[string]any{"name": "one", "description": "One", "permissions": map[string]any{"camera": true}},
				map[string]any{"name": "two", "description": "Two", "permissions": map[string]any{"microphone": true}},
			}, "commands"),
			wantErrs: []string{"commands[0].permissions.camera:", "commands[1].permissions.microphone:"},
		},
		{
			name: "rejects unknown permission fields under a differently cased permissions key",
			mutate: func(manifest map[string]any) {
				command := manifest["commands"].([]any)[0].(map[string]any)
				command["Permissions"] = command["permissions"]
				delete(command, "permissions")
				command["Permissions"].(map[string]any)["network"] = []any{}
			},
			wantErrs: []string{"commands[0].permissions.network:"},
		},
		{
			name: "rejects unknown permission fields under a differently cased commands key",
			mutate: func(manifest map[string]any) {
				manifest["Commands"] = manifest["commands"]
				delete(manifest, "commands")
				manifest["Commands"].([]any)[0].(map[string]any)["permissions"].(map[string]any)["network"] = []any{}
			},
			wantErrs: []string{"commands[0].permissions.network:"},
		},
		{
			name:     "rejects unknown permission fields when both key cases are present",
			mutate:   setCommand(map[string]any{"network": []any{}}, "PERMISSIONS"),
			wantErrs: []string{"commands[0].permissions.network:"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := mutateManifest(t, test.mutate)

			if _, err := ParseManifest(data); err == nil {
				t.Fatal("ParseManifest must reject it as well")
			}

			manifest, err := ParseInstalledManifest(data)
			assert.Nil(t, manifest)
			requireValidationError(t, err, append(test.wantErrs, "newer auth0 CLI")...)
		})
	}
}

func TestParseInstalledManifest_APIVersion(t *testing.T) {
	tests := []struct {
		name       string
		apiVersion any
	}{
		{name: "rejects apiVersion 2 as needing a newer CLI", apiVersion: 2},
		{name: "rejects a large apiVersion as needing a newer CLI", apiVersion: 99},
		{name: "rejects apiVersion 2.0 as needing a newer CLI", apiVersion: json.Number("2.0")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := mutateManifest(t, combine(setValue(test.apiVersion, "apiVersion"), setValue(true, "futureField")))

			manifest, err := ParseInstalledManifest(data)
			assert.Nil(t, manifest)
			requireValidationError(t, err, "apiVersion:", "newer auth0 CLI", fmt.Sprint(test.apiVersion))
		})
	}

	t.Run("does not report a newer CLI for apiVersion values below 1", func(t *testing.T) {
		_, err := ParseInstalledManifest(mutateManifest(t, setValue(0, "apiVersion")))

		requireValidationError(t, err, "apiVersion:")
		assert.NotContains(t, err.Error(), "newer auth0 CLI")
	})
}
