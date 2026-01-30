package cli

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	sampleContentWithSensitive = `
resource "auth0_example" "example" {
  sensitive_field = null # sensitive
  regular_field   = "value"
}
`

	sampleContentWithoutSensitive = `
resource "auth0_example" "example" {
  regular_field = "value"
}
`
)

func createDirWithTempTerraformFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	filePath := dir + "/" + generatedTFFileName
	assert.NoError(t, os.WriteFile(filePath, []byte(content), 0644))
	return dir
}

func Test_hasSensitiveNullValues(t *testing.T) {
	t.Run("returns true when sensitive null values are present", func(t *testing.T) {
		dir := createDirWithTempTerraformFile(t, sampleContentWithSensitive)
		assert.True(t, hasSensitiveNullValues(dir))
	})

	t.Run("returns false when no sensitive null values are present", func(t *testing.T) {
		dir := createDirWithTempTerraformFile(t, sampleContentWithoutSensitive)
		assert.False(t, hasSensitiveNullValues(dir))
	})

	t.Run("returns false when file does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		assert.False(t, hasSensitiveNullValues(tmpDir))
	})
}

func Test_processSensitiveFieldsInConfig(t *testing.T) {
	t.Run("replaces sensitive null values with empty strings and TODO comments", func(t *testing.T) {
		expectedContent := `
resource "auth0_example" "example" {
  sensitive_field = "" # TODO: Add sensitive value for 'sensitive_field'
  regular_field   = "value"
}
`
		dir := createDirWithTempTerraformFile(t, sampleContentWithSensitive)
		assert.NoError(t, processSensitiveFieldsInConfig(dir))

		updatedContent, err := os.ReadFile(dir + "/" + generatedTFFileName)
		assert.NoError(t, err)
		assert.Equal(t, expectedContent, string(updatedContent))
	})
	t.Run("does nothing when no sensitive null values are present", func(t *testing.T) {
		dir := createDirWithTempTerraformFile(t, sampleContentWithoutSensitive)
		assert.NoError(t, processSensitiveFieldsInConfig(dir))

		updatedContent, err := os.ReadFile(dir + "/" + generatedTFFileName)
		assert.NoError(t, err)
		assert.Equal(t, sampleContentWithoutSensitive, string(updatedContent))
	})
	t.Run("returns error when file cannot be read", func(t *testing.T) {
		tmpDir := t.TempDir()
		assert.Error(t, processSensitiveFieldsInConfig(tmpDir))
	})
}
