package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"testing"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
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

		data, err := fetchImportData(context.Background(), &cli{}, mockFetchers...)
		assert.NoError(t, err)
		assert.Equal(t, expectedData, data)
	})

	t.Run("it deduplicates same-named resources", func(t *testing.T) {
		mockData1 := importDataList{{ResourceName: "auth0_action.same", ImportID: "action-1"}, {ResourceName: "auth0_action.same", ImportID: "action-2"}}
		mockData2 := importDataList{{ResourceName: "auth0_client.same", ImportID: "client-1"}}

		mockFetchers := []resourceDataFetcher{
			&mockFetcher{mockData: mockData1},
			&mockFetcher{mockData: mockData2},
		}

		expectedData := importDataList{
			{ResourceName: "auth0_action.same", ImportID: "action-1"},
			{ResourceName: "auth0_action.same" + "_2", ImportID: "action-2"},
			{ResourceName: "auth0_client.same", ImportID: "client-1"},
		}

		data, err := fetchImportData(context.Background(), &cli{}, mockFetchers...)
		assert.NoError(t, err)
		assert.Equal(t, expectedData, data)
	})

	t.Run("it returns an error when a data fetcher fails", func(t *testing.T) {
		expectedErr := errors.New("failed to list clients")
		mockFetchers := []resourceDataFetcher{
			&mockFetcher{mockErr: expectedErr},
		}

		_, err := fetchImportData(context.Background(), &cli{}, mockFetchers...)
		assert.EqualError(t, err, "failed to list clients")
	})
}

func TestGenerateTerraformImportConfig(t *testing.T) {
	t.Run("it can correctly generate the terraform config files", func(t *testing.T) {
		input, importData := setupTestDIRAndImportData(t)

		err := generateTerraformImportConfig(&input, importData)
		require.NoError(t, err)

		assertTerraformMainFileWasGeneratedCorrectly(t, input.OutputDIR)
		assertTerraformImportFileWasGeneratedCorrectly(t, input.OutputDIR, importData)
	})

	t.Run("it can correctly generate the terraform main config file even if the dir exists", func(t *testing.T) {
		input, importData := setupTestDIRAndImportData(t)

		err := os.MkdirAll(input.OutputDIR, 0755)
		require.NoError(t, err)

		err = generateTerraformImportConfig(&input, importData)
		require.NoError(t, err)

		assertTerraformMainFileWasGeneratedCorrectly(t, input.OutputDIR)
		assertTerraformImportFileWasGeneratedCorrectly(t, input.OutputDIR, importData)
	})

	t.Run("it fails to generate the terraform config files if there's no import data", func(t *testing.T) {
		input, _ := setupTestDIRAndImportData(t)

		err := generateTerraformImportConfig(&input, importDataList{})
		assert.EqualError(t, err, "no import data available")
	})

	t.Run("it fails to create the directory if path is empty", func(t *testing.T) {
		_, importData := setupTestDIRAndImportData(t)

		err := generateTerraformImportConfig(&terraformInputs{OutputDIR: ""}, importData)
		assert.EqualError(t, err, "mkdir : no such file or directory")
	})

	t.Run("it fails to create the main.tf file if file is already created and read only", func(t *testing.T) {
		input, importData := setupTestDIRAndImportData(t)

		err := os.MkdirAll(input.OutputDIR, 0755)
		require.NoError(t, err)

		mainFilePath := path.Join(input.OutputDIR, "auth0_main.tf")
		_, err = os.Create(mainFilePath)
		require.NoError(t, err)

		err = os.Chmod(mainFilePath, 0444)
		require.NoError(t, err)

		err = generateTerraformImportConfig(&input, importData)
		assert.EqualError(t, err, fmt.Sprintf("open %s: permission denied", mainFilePath))
	})

	t.Run("it fails to create the auth0_import.tf file if file is already created and read only", func(t *testing.T) {
		input, importData := setupTestDIRAndImportData(t)

		err := os.MkdirAll(input.OutputDIR, 0755)
		require.NoError(t, err)

		importFilePath := path.Join(input.OutputDIR, "auth0_import.tf")
		_, err = os.Create(importFilePath)
		require.NoError(t, err)

		err = os.Chmod(importFilePath, 0444)
		require.NoError(t, err)

		err = generateTerraformImportConfig(&input, importData)
		assert.EqualError(t, err, fmt.Sprintf("open %s: permission denied", importFilePath))
	})
}

func setupTestDIRAndImportData(t *testing.T) (terraformInputs, importDataList) {
	dirPath, err := os.MkdirTemp("", "terraform-*")
	require.NoError(t, err)

	t.Cleanup(func() {
		err := os.RemoveAll(dirPath)
		require.NoError(t, err)
	})

	outputDIR := path.Join(dirPath, "dev")
	importData := importDataList{
		{
			ResourceName: "auth0_client.MyTestClient1",
			ImportID:     "clientID_1",
		},
		{
			ResourceName: "auth0_client.MyTestClient2",
			ImportID:     "clientID_2",
		},
		{
			ResourceName: "auth0_action.MyTestAction",
			ImportID:     "actionID_1",
		},
		{
			ResourceName: "auth0_action.MyTestAction", // NOTE: duplicate name.
			ImportID:     "actionID_2",
		},
	}

	input := terraformInputs{
		outputDIR,
		nil,
		"1.5.0",
	}
	return input, importData
}

func assertTerraformMainFileWasGeneratedCorrectly(t *testing.T, outputDIR string) {
	// Assert that the directory was created.
	_, err := os.Stat(outputDIR)
	assert.NoError(t, err)

	// Assert that the main.tf file was created with the correct content.
	filePath := path.Join(outputDIR, "auth0_main.tf")
	_, err = os.Stat(filePath)
	assert.NoError(t, err)

	expectedContent := `terraform {
  required_version = ">= 1.5.0"
  required_providers {
    auth0 = {
      source  = "auth0/auth0"
      version = ">= 1.0.0"
    }
  }
}

provider "auth0" {
  debug = true
}
`
	// Read the file content and check if it matches the expected content.
	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Equal(t, expectedContent, string(content))
}

func assertTerraformImportFileWasGeneratedCorrectly(t *testing.T, outputDIR string, data importDataList) {
	// Assert that the directory was created.
	_, err := os.Stat(outputDIR)
	assert.NoError(t, err)

	// Assert that the auth0_import.tf file was created with the correct content.
	filePath := path.Join(outputDIR, "auth0_import.tf")
	_, err = os.Stat(filePath)
	assert.NoError(t, err)

	contentTemplate := `# This file is automatically generated via the Auth0 CLI.
# It can be safely removed after the successful generation
# of Terraform resource definition files.
{{range .}}
import {
  id = "{{ .ImportID }}"
  to = {{ .ResourceName }}
}
{{end}}
`

	tmpl, err := template.New("terraform").Parse(contentTemplate)
	require.NoError(t, err)

	var expectedContent bytes.Buffer
	err = tmpl.Execute(&expectedContent, data)
	require.NoError(t, err)

	// Read the file content and check if it matches the expected content.
	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Equal(t, expectedContent.String(), string(content))
}

func TestTerraformProviderCredentialsAreAvailable(t *testing.T) {
	testCases := []struct {
		description  string
		domain       string
		clientID     string
		clientSecret string
		apiToken     string
		expected     bool
	}{
		{
			description:  "All credentials are available",
			domain:       "example.com",
			clientID:     "client123",
			clientSecret: "secret123",
			apiToken:     "token123",
			expected:     true,
		},
		{
			description: "Only domain and API token are available",
			domain:      "example.com",
			apiToken:    "token123",
			expected:    true,
		},
		{
			description: "Only domain is available",
			domain:      "example.com",
			expected:    false,
		},
		{
			description: "No credentials are available",
			expected:    false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			t.Setenv("AUTH0_DOMAIN", testCase.domain)
			t.Setenv("AUTH0_CLIENT_ID", testCase.clientID)
			t.Setenv("AUTH0_CLIENT_SECRET", testCase.clientSecret)
			t.Setenv("AUTH0_API_TOKEN", testCase.apiToken)

			assert.Equal(t, testCase.expected, terraformProviderCredentialsAreAvailable())
		})
	}
}

func TestDeduplicatedResourceNames(t *testing.T) {
	t.Run("it deduplicates identical resource names", func(t *testing.T) {
		sameNameAction := "auth0_action.same_name"
		sameNameClient := "auth0_client.same_name"

		mockData := importDataList{
			{ResourceName: sameNameAction, ImportID: "id-1"},
			{ResourceName: sameNameAction, ImportID: "id-2"},
			{ResourceName: sameNameAction, ImportID: "id-3"},
			{ResourceName: sameNameAction, ImportID: "id-4"},
			{ResourceName: sameNameAction, ImportID: "id-5"},
			{ResourceName: sameNameClient, ImportID: "id-6"},
			{ResourceName: sameNameClient, ImportID: "id-7"},
			{ResourceName: sameNameClient, ImportID: "id-8"},
		}

		expectedData := importDataList{
			{ResourceName: "auth0_action.same_name", ImportID: "id-1"},
			{ResourceName: "auth0_action.same_name_2", ImportID: "id-2"},
			{ResourceName: "auth0_action.same_name_3", ImportID: "id-3"},
			{ResourceName: "auth0_action.same_name_4", ImportID: "id-4"},
			{ResourceName: "auth0_action.same_name_5", ImportID: "id-5"},
			{ResourceName: "auth0_client.same_name", ImportID: "id-6"},
			{ResourceName: "auth0_client.same_name_2", ImportID: "id-7"},
			{ResourceName: "auth0_client.same_name_3", ImportID: "id-8"},
		}

		assert.Equal(t, expectedData, deduplicateResourceNames(mockData))
	})

	t.Run("it does not modify import list if no duplicates exist", func(t *testing.T) {
		mockData := importDataList{
			{ResourceName: "auth0_action.example_a", ImportID: "action-id-1"},
			{ResourceName: "auth0_action.example_b", ImportID: "action-id-2"},
			{ResourceName: "auth0_action.example_c", ImportID: "action-id-3"},
			{ResourceName: "auth0_connection.example_a", ImportID: "conn-id-1"},
			{ResourceName: "auth0_connection.example_b", ImportID: "conn-id-2"},
			{ResourceName: "auth0_client.example_a", ImportID: "client-id-1"},
			{ResourceName: "auth0_client.example_b", ImportID: "client-id-2"},
		}

		assert.Equal(t, mockData, deduplicateResourceNames(mockData))
	})
}

func TestCheckOutputDirectoryIsEmpty(t *testing.T) {
	t.Run("it returns true if the directory is empty", func(t *testing.T) {
		tempDIR := t.TempDir()

		isEmpty := checkOutputDirectoryIsEmpty(&cli{}, &cobra.Command{}, tempDIR)
		assert.True(t, isEmpty)
	})

	t.Run("it returns true if the directory doesn't exist", func(t *testing.T) {
		isEmpty := checkOutputDirectoryIsEmpty(&cli{}, &cobra.Command{}, "")
		assert.True(t, isEmpty)
	})

	t.Run("it returns true if the directory is not empty but we're forcing the command", func(t *testing.T) {
		tempDIR := t.TempDir()
		files := []string{"auth0_main.tf", "auth0_import.tf", "auth0_generated.tf"}

		for _, file := range files {
			filePath := path.Join(tempDIR, file)
			_, err := os.Create(filePath)
			require.NoError(t, err)
		}

		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: stdout,
				ResultWriter:  stdout,
			},
			force:   true,
			noInput: true,
		}

		isEmpty := checkOutputDirectoryIsEmpty(cli, &cobra.Command{}, tempDIR)
		assert.True(t, isEmpty)
		assert.Contains(t, stdout.String(), "Proceeding will overwrite the auth0_main.tf, auth0_import.tf and auth0_generated.tf files.")
	})
}

func TestCleanOutputDirectory(t *testing.T) {
	t.Run("it can successfully clean the output directory from all generated files", func(t *testing.T) {
		tempDIR := t.TempDir()
		files := []string{"auth0_main.tf", "auth0_import.tf", "auth0_generated.tf"}

		for _, file := range files {
			filePath := path.Join(tempDIR, file)
			_, err := os.Create(filePath)
			require.NoError(t, err)
		}

		err := cleanOutputDirectory(tempDIR)
		assert.NoError(t, err)

		for _, file := range files {
			filePath := path.Join(tempDIR, file)
			_, err := os.Stat(filePath)
			assert.ErrorContains(t, err, "no such file or directory")
		}
	})

	t.Run("it returns an error if it can't remove a file", func(t *testing.T) {
		files := []string{"auth0_main.tf", "auth0_import.tf", "auth0_generated.tf"}

		for _, file := range files {
			t.Run(file, func(t *testing.T) {
				tempDIR := t.TempDir()

				filePath := path.Join(tempDIR, file)
				_, err := os.Create(filePath)
				require.NoError(t, err)

				err = os.Chmod(tempDIR, 0444)
				require.NoError(t, err)

				t.Cleanup(func() {
					err = os.Chmod(tempDIR, 0755)
					require.NoError(t, err)
				})

				err = cleanOutputDirectory(tempDIR)
				assert.ErrorContains(t, err, "permission denied")
			})
		}
	})
}

func TestTerraformInputs_ParseResourceFetchers(t *testing.T) {
	api := &auth0.API{}

	var testCases = []struct {
		name                 string
		input                terraformInputs
		expectedError        string
		expectedDataFetchers []resourceDataFetcher
	}{
		{
			name: "it can successfully parse resources: auth0_client",
			input: terraformInputs{
				Resources: []string{"auth0_client"},
			},
			expectedDataFetchers: []resourceDataFetcher{
				&clientResourceFetcher{api},
			},
		},
		{
			name: "it can successfully parse resources: auth0_client, auth0_connection",
			input: terraformInputs{
				Resources: []string{"auth0_client", "auth0_connection"},
			},
			expectedDataFetchers: []resourceDataFetcher{
				&clientResourceFetcher{api},
				&connectionResourceFetcher{api},
			},
		},
		{
			name: "it fails to parse unsupported resources: auth0_technology",
			input: terraformInputs{
				Resources: []string{"auth0_technology"},
			},
			expectedError: "unsupported resource type: auth0_technology",
		},
		{
			name: "it fails to parse unsupported resources even if combined with supported resources: auth0_client, auth0_technology",
			input: terraformInputs{
				Resources: []string{"auth0_client", "auth0_technology"},
			},
			expectedError: "unsupported resource type: auth0_technology",
		},
		{
			name: "it fails to parse unsupported resources and raises the error for all of them: auth0_metrics, auth0_technology",
			input: terraformInputs{
				Resources: []string{"auth0_metrics", "auth0_technology"},
			},
			expectedError: "unsupported resource type: auth0_metrics\nunsupported resource type: auth0_technology",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual, err := testCase.input.parseResourceFetchers(api)

			if testCase.expectedError != "" {
				assert.EqualError(t, err, testCase.expectedError)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedDataFetchers, actual)
		})
	}
}

func TestSanitizeResourceName(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		// Test cases with valid names.
		{"ValidName123", "validname123"},
		{"_Another_Valid-Name", "another_valid_name"},
		{"name_with_123", "name_with_123"},
		{"name-with-dashes", "name_with_dashes"},
		{"_starts_and_ends_with_underscore_", "starts_and_ends_with_underscore"},
		{"multiple          spaces       between", "multiple_spaces_between"},
		{"https://travel0.us.auth0.com/api/v2/", "https_travel0_us_auth0_com_api_v2"},

		// Test cases with invalid names to be sanitized.
		{"Invalid@Name", "invalid_name"},
		{"Invalid Name", "invalid_name"},
		{"123 Starts With Number", "starts_with_number"},
		{"-Starts With Dash", "starts_with_dash"},
		{"", ""},
	}

	for _, testCase := range testCases {
		t.Run(testCase.input, func(t *testing.T) {
			sanitized := sanitizeResourceName(testCase.input)
			assert.Equal(t, testCase.expected, sanitized)
		})
	}
}

func TestCheckTerraformProviderAndCLIDomainsMatch(t *testing.T) {
	t.Run("it should return no error if provided domain and TF provider env var domain match", func(t *testing.T) {
		domain := "travel0.us.auth0.com"

		t.Setenv("AUTH0_DOMAIN", domain)
		err := checkTerraformProviderAndCLIDomainsMatch(domain)
		assert.NoError(t, err)
	})

	t.Run("it should return an error if provided domain and TF provider env var domain do not match", func(t *testing.T) {
		t.Setenv("AUTH0_DOMAIN", "different-tenant.eu.auth0.com")
		err := checkTerraformProviderAndCLIDomainsMatch("travel0.us.auth0.com")
		assert.Error(t, err)
		assert.Equal(t, err.Error(), "terraform provider tenant domain \"different-tenant.eu.auth0.com\" does not match current CLI tenant \"travel0.us.auth0.com\"")
	})
}
