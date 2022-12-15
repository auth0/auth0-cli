package cli

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAPICmdInputs_FromArgs(t *testing.T) {
	const testDomain = "example.auth0.com"
	var testCases = []struct {
		name           string
		givenArgs      []string
		givenDataFlag  string
		expectedMethod string
		expectedURL    string
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
		},
		{
			name:          "it fails to parse input arguments when method is invalid",
			givenArgs:     []string{"abracadabra", "/clients"},
			expectedError: "invalid method given: ABRACADABRA, accepting only GET, POST, PUT, PATCH, DELETE",
		},
		{
			name:          "it fails to parse input arguments when data is not a valid JSON",
			givenArgs:     []string{"patch", "clients"},
			givenDataFlag: "{",
			expectedError: "invalid json data given: {",
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
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, testCase.expectedMethod, actualInputs.Method)
			assert.Equal(t, testCase.expectedURL, actualInputs.URL.String())
		})
	}
}

func TestAPICmd_IsInsufficientScopeError(t *testing.T) {
	var testCases = []struct {
		name              string
		inputStatusCode   int
		inputResponseBody string
		expectedResult    bool
		expectedScope     string
	}{
		{
			name:            "it does not detect 404 error",
			inputStatusCode: 404,
			inputResponseBody: `{
				"statusCode": 404,
				"error": "Not Found",
				"message": "Not Found"
			}`,
			expectedResult: false,
			expectedScope:  "",
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
			expectedResult: false,
			expectedScope:  "",
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
			expectedResult: true,
			expectedScope:  "create:client_grants",
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
			expectedResult: true,
			expectedScope:  "read:clients",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {

			input := http.Response{
				Body:       ioutil.NopCloser(bytes.NewReader([]byte(testCase.inputResponseBody))),
				StatusCode: testCase.inputStatusCode,
			}

			actualRespBool, actualScope := isInsufficientScopeError(&input)

			assert.Equal(t, testCase.expectedResult, actualRespBool)
			assert.Equal(t, testCase.expectedScope, actualScope)
		})
	}
}
