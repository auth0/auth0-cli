package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResolveACULConfigData covers the --file/--data value resolution: inline
// JSON is used verbatim, @file.json and a bare path both read the file, and a
// missing file surfaces a read error. This keeps the alias's whole-payload
// contract offline-testable without touching the tenant.
func TestResolveACULConfigData(t *testing.T) {
	dir := t.TempDir()
	fileContent := `{"rendering_mode":"advanced"}`
	filePath := filepath.Join(dir, "settings.json")
	require.NoError(t, os.WriteFile(filePath, []byte(fileContent), 0600))

	t.Run("inline JSON object is used verbatim", func(t *testing.T) {
		data, err := resolveACULConfigData(`{"rendering_mode":"advanced"}`)
		require.NoError(t, err)
		assert.JSONEq(t, fileContent, string(data))
	})

	t.Run("inline JSON with leading whitespace is used verbatim", func(t *testing.T) {
		data, err := resolveACULConfigData("  \n" + fileContent)
		require.NoError(t, err)
		assert.JSONEq(t, fileContent, string(data))
	})

	t.Run("@file.json reads the referenced file", func(t *testing.T) {
		data, err := resolveACULConfigData("@" + filePath)
		require.NoError(t, err)
		assert.JSONEq(t, fileContent, string(data))
	})

	t.Run("bare path reads the file (original --file behavior)", func(t *testing.T) {
		data, err := resolveACULConfigData(filePath)
		require.NoError(t, err)
		assert.JSONEq(t, fileContent, string(data))
	})

	t.Run("missing file surfaces a read error", func(t *testing.T) {
		_, err := resolveACULConfigData(filepath.Join(dir, "does-not-exist.json"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unable to read file")
	})
}
