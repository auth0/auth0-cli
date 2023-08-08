package cli

import (
	"context"
	"errors"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockFetcher struct {
	mockData importDataList
	mockErr  error
}

func (m *mockFetcher) FetchData(context.Context) (importDataList, error) {
	return m.mockData, m.mockErr
}

func TestFetchImportData(t *testing.T) {
	t.Run("it can successfully fetch import data for multiple resources", func(t *testing.T) {
		mockData1 := importDataList{{ResourceName: "Resource1", ImportID: "123"}}
		mockData2 := importDataList{{ResourceName: "Resource2", ImportID: "456"}}
		mockFetchers := []resourceDataFetcher{
			&mockFetcher{mockData: mockData1},
			&mockFetcher{mockData: mockData2},
		}

		expectedData := importDataList{
			{ResourceName: "Resource1", ImportID: "123"},
			{ResourceName: "Resource2", ImportID: "456"},
		}

		data, err := fetchImportData(context.Background(), mockFetchers...)
		assert.NoError(t, err)
		assert.Equal(t, expectedData, data)
	})

	t.Run("it returns an error when a data fetcher fails", func(t *testing.T) {
		expectedErr := errors.New("failed to list clients")
		mockFetchers := []resourceDataFetcher{
			&mockFetcher{mockErr: expectedErr},
		}

		_, err := fetchImportData(context.Background(), mockFetchers...)
		assert.EqualError(t, err, "failed to list clients")
	})
}

func TestGenerateTerraformConfigFiles(t *testing.T) {
	testInputs := terraformInputs{
		OutputDIR: "./terraform/dev",
	}
	defer os.RemoveAll("./terraform")

	t.Run("it can correctly generate the terraform main config file", func(t *testing.T) {
		assertTerraformConfigFilesWereGeneratedWithCorrectContent(t, &testInputs)
	})

	t.Run("it can correctly generate the terraform main config file even if the dir exists", func(t *testing.T) {
		err := os.MkdirAll(testInputs.OutputDIR, 0755)
		require.NoError(t, err)

		assertTerraformConfigFilesWereGeneratedWithCorrectContent(t, &testInputs)
	})

	t.Run("it fails to create the directory if path is empty", func(t *testing.T) {
		testInputs := terraformInputs{
			OutputDIR: "",
		}

		err := generateTerraformConfigFiles(&testInputs)
		assert.EqualError(t, err, "mkdir : no such file or directory")
	})

	t.Run("it fails to create the main.tf file if file is already created and read only", func(t *testing.T) {
		err := os.MkdirAll(testInputs.OutputDIR, 0755)
		require.NoError(t, err)

		mainFilePath := path.Join(testInputs.OutputDIR, "main.tf")
		_, err = os.Create(mainFilePath)
		require.NoError(t, err)

		err = os.Chmod(mainFilePath, 0444)
		require.NoError(t, err)

		err = generateTerraformConfigFiles(&testInputs)
		assert.EqualError(t, err, "open terraform/dev/main.tf: permission denied")
	})
}

func assertTerraformConfigFilesWereGeneratedWithCorrectContent(t *testing.T, testInputs *terraformInputs) {
	err := generateTerraformConfigFiles(testInputs)
	require.NoError(t, err)

	// Assert that the directory was created.
	_, err = os.Stat(testInputs.OutputDIR)
	assert.NoError(t, err)

	// Assert that the main.tf file was created with the correct content.
	mainTerraformConfigFilePath := path.Join(testInputs.OutputDIR, "main.tf")
	_, err = os.Stat(mainTerraformConfigFilePath)
	assert.NoError(t, err)

	expectedContent := `terraform {
  required_version = "~> 1.5.0"
  required_providers {
    auth0 = {
      source  = "auth0/auth0"
      version = "1.0.0-beta.1"
    }
  }
}

provider "auth0" {
  debug         = true
}
`
	// Read the file content and check if it matches the expected content
	content, err := os.ReadFile(mainTerraformConfigFilePath)
	assert.NoError(t, err)
	assert.Equal(t, expectedContent, string(content))
}
