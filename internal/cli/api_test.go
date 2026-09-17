package cli

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPICmdInputs_FromArgs(t *testing.T) {
	const testDomain = "example.auth0.com"
	var testCases = []struct {
		name           string
		givenArgs      []string
		givenDataFlag  string
		expectedMethod string
		expectedURL    string
		expectedData   any
		expectedError  string
	}{
		{
			name:           "it can correctly parse input arguments",
			givenArgs:      []string{"get", "/tenants/settings"},
			expectedMethod: http.MethodGet,
			expectedURL:    "https://" + testDomain + "/api/v2/tenants/settings",
		},
		{
			name:           "it can correctly parse input arguments and data flag",
			givenArgs:      []string{"post", "clients"},
			givenDataFlag:  `{"name":"genericTest"}`,
			expectedMethod: http.MethodPost,
			expectedURL:    "https://" + testDomain + "/api/v2/clients",
			expectedData:   map[string]any{"name": "genericTest"},
		},
		{
			name:           "it can correctly parse input arguments when get method is missing",
			givenArgs:      []string{"tenants/settings"},
			expectedMethod: http.MethodGet,
			expectedURL:    "https://" + testDomain + "/api/v2/tenants/settings",
		},
		{
			name:           "it can correctly parse input arguments and data flag when post method is missing",
			givenArgs:      []string{"/clients"},
			givenDataFlag:  `{"name":"genericTest"}`,
			expectedMethod: http.MethodPost,
			expectedURL:    "https://" + testDomain + "/api/v2/clients",
			expectedData:   map[string]any{"name": "genericTest"},
		},
		{
			name:          "it fails to parse input arguments when method is invalid",
			givenArgs:     []string{"abracadabra", "/clients"},
			expectedError: "invalid method given: ABRACADABRA, accepting only GET, POST, PUT, PATCH, DELETE",
		},
		{
			name:           "it can correctly parse delete with data flag",
			givenArgs:      []string{"delete", "organizations/org_123/members"},
			givenDataFlag:  `{"members":["user_123"]}`,
			expectedMethod: http.MethodDelete,
			expectedURL:    "https://" + testDomain + "/api/v2/organizations/org_123/members",
			expectedData:   map[string]any{"members": []any{"user_123"}},
		},
		{
			name:          "it fails to parse input arguments when data is not a valid JSON",
			givenArgs:     []string{"patch", "clients"},
			givenDataFlag: "{",
			expectedError: "invalid JSON data provided: unexpected end of JSON input",
		},
		{
			name:          "it fails to parse input arguments when uri is invalid",
			givenArgs:     []string{"get", "#$%^&*(#$%%^("},
			expectedError: "invalid uri given: parse \"https://example.auth0.com/api/v2/#$%^&*(#$%%^(\": invalid URL escape \"%^&\"",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if len(testCase.givenArgs) < 1 {
				t.Fatalf("the test cases need to pass at least 1 argument")
			}

			actualInputs := &apiCmdInputs{
				RawData: testCase.givenDataFlag,
			}

			err := actualInputs.fromArgs(testCase.givenArgs, testDomain)

			if testCase.expectedError != "" {
				assert.EqualError(t, err, testCase.expectedError)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedMethod, actualInputs.Method)
			assert.Equal(t, testCase.expectedURL, actualInputs.URL.String())
			if testCase.expectedData != nil {
				assert.Equal(t, testCase.expectedData, actualInputs.Data)
			}
		})
	}
}

func TestAPICmdInputs_ValidateAndSetData(t *testing.T) {
	t.Run("GET ignores data entirely", func(t *testing.T) {
		inputs := &apiCmdInputs{Method: http.MethodGet, RawData: `{"name":"x"}`}
		require.NoError(t, inputs.validateAndSetData())
		assert.Nil(t, inputs.Data)
	})

	// With --data set we use it as-is and never read stdin, so a request with
	// --data never blocks on an open, EOF-less pipe.
	t.Run("--data flag is used as-is over piped stdin", func(t *testing.T) {
		inputs := &apiCmdInputs{Method: http.MethodPost, RawData: `{"name":"from-flag"}`}
		withPipedStdin(t, `{"name":"from-pipe"}`, func() {
			require.NoError(t, inputs.validateAndSetData())
		})
		assert.Equal(t, map[string]any{"name": "from-flag"}, inputs.Data)
	})

	t.Run("piped stdin is used when --data is absent", func(t *testing.T) {
		inputs := &apiCmdInputs{Method: http.MethodPost}
		withPipedStdin(t, `{"name":"from-pipe"}`, func() {
			require.NoError(t, inputs.validateAndSetData())
		})
		assert.Equal(t, map[string]any{"name": "from-pipe"}, inputs.Data)
	})

	t.Run("invalid JSON is rejected", func(t *testing.T) {
		inputs := &apiCmdInputs{Method: http.MethodPost, RawData: "{"}
		err := inputs.validateAndSetData()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid JSON data provided")
	})
}

func TestAPICmd_IsInsufficientScopeError(t *testing.T) {
	var testCases = []struct {
		name              string
		inputStatusCode   int
		inputResponseBody string
		expectedError     string
	}{
		{
			name:            "it does not detect 404 error",
			inputStatusCode: 404,
			inputResponseBody: `{
				"statusCode": 404,
				"error": "Not Found",
				"message": "Not Found"
			}`,
			expectedError: "",
		},
		{
			name:            "it does not detect a 200 HTTP response",
			inputStatusCode: 200,
			inputResponseBody: `{
				"allowed_logout_urls": [],
				"change_password": {
				  "enabled": true,
				  "html": "<html>LOL</html>"
				},
				"default_audience": "",
			}`,
			expectedError: "",
		},
		{
			name:            "it does not detect a 403 that is not an insufficient scope error",
			inputStatusCode: 403,
			inputResponseBody: `{
				"statusCode": 403,
				"error": "Forbidden",
				"message": "Operation not allowed"
			}`,
			expectedError: "",
		},
		{
			name:            "it correctly detects an insufficient scope error",
			inputStatusCode: 403,
			inputResponseBody: `{
				"statusCode": 403,
				"error": "Forbidden",
				"message": "Insufficient scope, expected any of: create:client_grants",
				"errorCode": "insufficient_scope"
			  }`,
			expectedError: "request failed because access token lacks scope: create:client_grants.\n If authenticated via client credentials, add this scope to the designated client. If authenticated as a user, request this scope during login by running `auth0 login --scopes create:client_grants`",
		},
		{
			name:            "it correctly detects an insufficient scope error with multiple scope",
			inputStatusCode: 403,
			inputResponseBody: `{
				"statusCode": 403,
				"error": "Forbidden",
				"message": "Insufficient scope, expected any of: read:clients, read:client_summary",
				"errorCode": "insufficient_scope"
			  }`,
			expectedError: "request failed because access token lacks scope: read:clients.\n If authenticated via client credentials, add this scope to the designated client. If authenticated as a user, request this scope during login by running `auth0 login --scopes read:clients`",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := isInsufficientScopeError(testCase.inputStatusCode, []byte(testCase.inputResponseBody))
			if testCase.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, testCase.expectedError)
			}
		})
	}
}
