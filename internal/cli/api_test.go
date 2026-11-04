package cli

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/iostream"
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
		{
			name:          "it fails when the uri escapes the management api path",
			givenArgs:     []string{"get", "../../oauth/token"},
			expectedError: "invalid uri given: \"../../oauth/token\" resolves outside of the Management API (/api/v2)",
		},
		{
			name:          "it fails when a percent-encoded uri escapes the management api path",
			givenArgs:     []string{"get", "%2e%2e/%2e%2e/.well-known/openid-configuration"},
			expectedError: "invalid uri given: \"%2e%2e/%2e%2e/.well-known/openid-configuration\" resolves outside of the Management API (/api/v2)",
		},
		{
			name:          "it fails when an encoded slash escapes the management api path",
			givenArgs:     []string{"get", "..%2F..%2Fuserinfo"},
			expectedError: "invalid uri given: \"..%2F..%2Fuserinfo\" resolves outside of the Management API (/api/v2)",
		},
		{
			name:           "it allows dot segments that stay within the management api path",
			givenArgs:      []string{"get", "users/../clients"},
			expectedMethod: http.MethodGet,
			expectedURL:    "https://" + testDomain + "/api/v2/users/../clients",
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

	// --data @file reads the payload from a file, the common way to send a large
	// body without embedding it on the command line.
	t.Run("--data @file reads from a file", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "body.json")
		require.NoError(t, os.WriteFile(file, []byte(`{"name":"from-file"}`), 0600))

		inputs := &apiCmdInputs{Method: http.MethodPost, RawData: "@" + file}
		require.NoError(t, inputs.validateAndSetData())
		assert.Equal(t, map[string]any{"name": "from-file"}, inputs.Data)
	})

	t.Run("--data @file surfaces a read error for a missing file", func(t *testing.T) {
		inputs := &apiCmdInputs{Method: http.MethodPost, RawData: "@/does/not/exist.json"}
		err := inputs.validateAndSetData()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read data file")
	})

	// --data @- (and -) reads from stdin, letting a script pipe a body while still
	// passing the flag explicitly.
	t.Run("--data @- reads from piped stdin", func(t *testing.T) {
		inputs := &apiCmdInputs{Method: http.MethodPost, RawData: "@-"}
		withPipedStdin(t, `{"name":"from-pipe"}`, func() {
			require.NoError(t, inputs.validateAndSetData())
		})
		assert.Equal(t, map[string]any{"name": "from-pipe"}, inputs.Data)
	})

	t.Run("--data @- errors when stdin is empty", func(t *testing.T) {
		inputs := &apiCmdInputs{Method: http.MethodPost, RawData: "@-"}
		withPipedStdin(t, "", func() {
			err := inputs.validateAndSetData()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "no data received on stdin")
		})
	})
}

func TestAPICmdInputs_FromArgs_SingleArgumentMethod(t *testing.T) {
	const testDomain = "example.auth0.com"

	// A bare single-argument call must default to GET and never touch stdin, so an
	// agent or CI run with an inherited, open, EOF-less stdin pipe cannot block. To
	// send a piped body, a method (or --data @-) is given explicitly.
	t.Run("a single-argument request does not block on an open, EOF-less stdin pipe", func(t *testing.T) {
		r, w, err := os.Pipe()
		require.NoError(t, err)
		defer func() {
			_ = w.Close()
			_ = r.Close()
		}()

		original := iostream.Input
		iostream.Input = r
		defer func() { iostream.Input = original }()

		// The write end is intentionally left open, so stdin never reaches EOF.
		// If parseRaw read stdin to infer the method, this would hang forever.
		inputs := &apiCmdInputs{}
		done := make(chan error, 1)
		go func() { done <- inputs.fromArgs([]string{"clients"}, testDomain) }()

		select {
		case err := <-done:
			require.NoError(t, err)
			assert.Equal(t, http.MethodGet, inputs.Method)
			assert.Equal(t, "https://"+testDomain+"/api/v2/clients", inputs.URL.String())
			assert.Nil(t, inputs.Data)
		case <-time.After(3 * time.Second):
			t.Fatal("fromArgs blocked reading an open, EOF-less stdin pipe")
		}
	})

	t.Run("a single-argument request with --data infers POST", func(t *testing.T) {
		inputs := &apiCmdInputs{RawData: `{"name":"x"}`}
		require.NoError(t, inputs.fromArgs([]string{"clients"}, testDomain))

		assert.Equal(t, http.MethodPost, inputs.Method)
		assert.Equal(t, "https://"+testDomain+"/api/v2/clients", inputs.URL.String())
		assert.Equal(t, map[string]any{"name": "x"}, inputs.Data)
	})
}

func TestAPICmdInputs_QueryParams(t *testing.T) {
	const testDomain = "example.auth0.com"

	t.Run("a repeated query param sends every value", func(t *testing.T) {
		inputs := &apiCmdInputs{RawQueryParams: []string{"fields=a", "fields=b"}}
		require.NoError(t, inputs.fromArgs([]string{"get", "clients"}, testDomain))
		assert.Equal(t, "https://"+testDomain+"/api/v2/clients?fields=a&fields=b", inputs.URL.String())
	})

	t.Run("distinct query params are all sent", func(t *testing.T) {
		inputs := &apiCmdInputs{RawQueryParams: []string{"from=20221101", "to=20221118"}}
		require.NoError(t, inputs.fromArgs([]string{"get", "stats/daily"}, testDomain))
		assert.Equal(t, "https://"+testDomain+"/api/v2/stats/daily?from=20221101&to=20221118", inputs.URL.String())
	})

	// Only the first "=" splits key from value, so a value may itself contain
	// "=" (for example a Lucene-style query) and is sent as a single param.
	t.Run("a value may itself contain an equals sign", func(t *testing.T) {
		inputs := &apiCmdInputs{RawQueryParams: []string{"q=name=foo"}}
		require.NoError(t, inputs.fromArgs([]string{"get", "clients"}, testDomain))
		assert.Equal(t, "https://"+testDomain+"/api/v2/clients?q=name%3Dfoo", inputs.URL.String())
	})

	// A comma is a literal value character, so a comma-separated list is sent as
	// a single param the way the Management API expects `fields`, `include_fields`,
	// and similar.
	t.Run("a comma-separated list is sent as a single param value", func(t *testing.T) {
		inputs := &apiCmdInputs{RawQueryParams: []string{"fields=a,b,c"}}
		require.NoError(t, inputs.fromArgs([]string{"get", "clients"}, testDomain))
		assert.Equal(t, "https://"+testDomain+"/api/v2/clients?fields=a%2Cb%2Cc", inputs.URL.String())
	})

	// A value carrying both commas and "=" (a Lucene query with multiple clauses)
	// stays a single param instead of being mis-split on either character.
	t.Run("a comma-and-equals value stays a single param", func(t *testing.T) {
		inputs := &apiCmdInputs{RawQueryParams: []string{"q=name=John,email=jane@x.com"}}
		require.NoError(t, inputs.fromArgs([]string{"get", "users"}, testDomain))
		assert.Equal(t, "https://"+testDomain+"/api/v2/users?q=name%3DJohn%2Cemail%3Djane%40x.com", inputs.URL.String())
	})

	t.Run("a list value and a distinct keyed param coexist", func(t *testing.T) {
		inputs := &apiCmdInputs{RawQueryParams: []string{"fields=a,b,c", "include_fields=true"}}
		require.NoError(t, inputs.fromArgs([]string{"get", "clients"}, testDomain))
		assert.Equal(t, "https://"+testDomain+"/api/v2/clients?fields=a%2Cb%2Cc&include_fields=true", inputs.URL.String())
	})

	// The registered flag delivers a comma-separated value whole (StringArray);
	// validateAndSetEndpoint then keeps it as one param value.
	t.Run("the -q flag delivers a comma-separated value unsplit", func(t *testing.T) {
		cmd := apiCmd(&cli{renderer: &display.Renderer{}})
		require.NoError(t, cmd.Flags().Parse([]string{"-q", "from=1,to=2"}))
		got, err := cmd.Flags().GetStringArray("query")
		require.NoError(t, err)
		assert.Equal(t, []string{"from=1,to=2"}, got)
	})

	t.Run("the -q flag accumulates repeated values", func(t *testing.T) {
		cmd := apiCmd(&cli{renderer: &display.Renderer{}})
		require.NoError(t, cmd.Flags().Parse([]string{"-q", "fields=name", "-q", "fields=email"}))
		got, err := cmd.Flags().GetStringArray("query")
		require.NoError(t, err)
		assert.Equal(t, []string{"fields=name", "fields=email"}, got)
	})

	t.Run("a query param without an equals sign is rejected", func(t *testing.T) {
		inputs := &apiCmdInputs{RawQueryParams: []string{"fields"}}
		err := inputs.fromArgs([]string{"get", "clients"}, testDomain)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `invalid query parameter "fields": expected key=value`)
	})

	// A leading "=" leaves the key empty, which would encode a nameless "?=value"
	// param, so it is rejected the same way a missing "=" is.
	t.Run("a query param with an empty key is rejected", func(t *testing.T) {
		inputs := &apiCmdInputs{RawQueryParams: []string{"=name"}}
		err := inputs.fromArgs([]string{"get", "clients"}, testDomain)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `invalid query parameter "=name": expected key=value`)
	})
}

func TestFormatAPIResponse(t *testing.T) {
	raw := []byte(`{"name":"x","nested":{"a":1}}`)

	t.Run("compact mode emits a single dense line", func(t *testing.T) {
		out, err := formatAPIResponse(display.OutputFormatJSONCompact, raw)
		require.NoError(t, err)
		assert.Equal(t, `{"name":"x","nested":{"a":1}}`, out)
	})

	t.Run("default mode pretty-prints with a 2-space indent", func(t *testing.T) {
		out, err := formatAPIResponse(display.OutputFormatJSON, raw)
		require.NoError(t, err)
		assert.Equal(t, "{\n  \"name\": \"x\",\n  \"nested\": {\n    \"a\": 1\n  }\n}", out)
	})

	t.Run("invalid JSON surfaces a formatting error", func(t *testing.T) {
		_, err := formatAPIResponse(display.OutputFormatJSONCompact, []byte("{"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to prepare json output")
	})
}

func TestAPICmd_RegistersJSONOutputFlags(t *testing.T) {
	// The api command must accept --json/--json-compact so formatAPIResponse can
	// switch between pretty and single-line output. --json-compact sets cli.jsonCompact,
	// which configureRenderer maps to OutputFormatJSONCompact.
	t.Run("--json-compact is accepted and bound to the shared field", func(t *testing.T) {
		c := &cli{renderer: &display.Renderer{}}
		cmd := apiCmd(c)
		require.NoError(t, cmd.Flags().Parse([]string{"--json-compact"}))
		assert.True(t, c.jsonCompact)
		assert.False(t, c.json)
	})

	t.Run("--json is accepted and bound to the shared field", func(t *testing.T) {
		c := &cli{renderer: &display.Renderer{}}
		cmd := apiCmd(c)
		require.NoError(t, cmd.Flags().Parse([]string{"--json"}))
		assert.True(t, c.json)
		assert.False(t, c.jsonCompact)
	})

	t.Run("--json and --json-compact are mutually exclusive", func(t *testing.T) {
		c := &cli{renderer: &display.Renderer{}}
		cmd := apiCmd(c)
		cmd.SetArgs([]string{"get", "tenants/settings", "--json", "--json-compact"})
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "json")
	})
}

func TestAPICmd_WrongMethodHint(t *testing.T) {
	const genericNotFound = `{"statusCode":404,"error":"Not Found","message":"Not Found"}`

	var testCases = []struct {
		name       string
		method     string
		statusCode int
		body       string
		wantHint   bool
	}{
		{
			name:       "generic 404 on PATCH hints at a wrong method",
			method:     http.MethodPatch,
			statusCode: 404,
			body:       genericNotFound,
			wantHint:   true,
		},
		{
			name:       "generic 404 on PATCH with an empty body still hints",
			method:     http.MethodPatch,
			statusCode: 404,
			body:       "",
			wantHint:   true,
		},
		{
			name:       "generic 404 on POST is a wrong path, not a wrong verb",
			method:     http.MethodPost,
			statusCode: 404,
			body:       genericNotFound,
			wantHint:   false,
		},
		{
			name:       "generic 404 on PUT does not hint",
			method:     http.MethodPut,
			statusCode: 404,
			body:       genericNotFound,
			wantHint:   false,
		},
		{
			name:       "generic 404 on DELETE does not hint",
			method:     http.MethodDelete,
			statusCode: 404,
			body:       genericNotFound,
			wantHint:   false,
		},
		{
			name:       "GET 404 is a missing resource, not a wrong verb",
			method:     http.MethodGet,
			statusCode: 404,
			body:       genericNotFound,
			wantHint:   false,
		},
		{
			name:       "resource-specific PATCH 404 with an errorCode is left untouched",
			method:     http.MethodPatch,
			statusCode: 404,
			body:       `{"statusCode":404,"errorCode":"inexistent_connection","message":"The connection does not exist"}`,
			wantHint:   false,
		},
		{
			name:       "descriptive PATCH 404 message is left untouched",
			method:     http.MethodPatch,
			statusCode: 404,
			body:       `{"statusCode":404,"error":"Not Found","message":"The user does not exist."}`,
			wantHint:   false,
		},
		{
			name:       "non-404 status never hints",
			method:     http.MethodPatch,
			statusCode: 400,
			body:       `{"statusCode":400,"error":"Bad Request","message":"Not Found"}`,
			wantHint:   false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.wantHint, isWrongMethod404(testCase.method, testCase.statusCode, []byte(testCase.body)))
		})
	}
}

// TestAPICmd_ResponseHintError checks that a hinted 404 keeps the terse API
// message in the envelope while carrying the hint in the details field, and
// unwraps to the underlying API error so status classification is unchanged.
func TestAPICmd_ResponseHintError(t *testing.T) {
	apiErr := core.NewAPIError(http.StatusNotFound, http.Header{}, fmt.Errorf("API request failed: Not Found"))
	err := apiResponseHintError{err: apiErr, hint: apiWrongMethodHintText}

	t.Run("Error() stays the terse API message, not the hint", func(t *testing.T) {
		assert.Equal(t, apiErr.Error(), err.Error())
		assert.NotContains(t, err.Error(), apiWrongMethodHintText)
	})

	t.Run("ErrorDetails() carries the hint", func(t *testing.T) {
		require.NotNil(t, err.ErrorDetails())
		assert.JSONEq(t, `{"hint":`+strconv.Quote(apiWrongMethodHintText)+`}`, string(err.ErrorDetails()))
	})

	t.Run("the envelope keeps a terse message and a hinted details field", func(t *testing.T) {
		envelope := buildErrorEnvelope(err)
		assert.Equal(t, apiErr.Error(), envelope.Error.Message)
		assert.NotContains(t, envelope.Error.Message, apiWrongMethodHintText)
		assert.Equal(t, http.StatusNotFound, envelope.Error.Status)
		assert.JSONEq(t, `{"hint":`+strconv.Quote(apiWrongMethodHintText)+`}`, string(envelope.Error.Details))
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

func TestMaskAPISecrets(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		want       string
		wantMasked bool
	}{
		{
			name: "body without secrets is returned untouched",
			body: `{"name":"b","app_type":"spa"}`,
			want: `{"name":"b","app_type":"spa"}`,
		},
		{
			name:       "client_secret and signing_keys are removed",
			body:       `{"name":"a","client_secret":"s3cret","signing_keys":[{"cert":"x"}]}`,
			want:       `{"name":"a"}`,
			wantMasked: true,
		},
		{
			name:       "secrets are removed from a list of clients",
			body:       `[{"client_id":"1","client_secret":"a"},{"client_id":"2","client_secret":"b"}]`,
			want:       `[{"client_id":"1"},{"client_id":"2"}]`,
			wantMasked: true,
		},
		{
			name:       "nested secrets are removed",
			body:       `{"options":{"client_secret":"a","client_id":"1"}}`,
			want:       `{"options":{"client_id":"1"}}`,
			wantMasked: true,
		},
		{
			name: "a secret key mentioned only inside a value is not a field",
			body: `{"note":"the \"client_secret\" is hidden"}`,
			want: `{"note":"the \"client_secret\" is hidden"}`,
		},
		{
			name:       "numbers and html characters are preserved",
			body:       `{"a":12345678901234567890,"b":"<x>&","client_secret":"s"}`,
			want:       `{"a":12345678901234567890,"b":"<x>&"}`,
			wantMasked: true,
		},
		{
			name: "invalid json is returned untouched",
			body: `{"client_secret":`,
			want: `{"client_secret":`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, masked, err := maskAPISecrets([]byte(test.body))

			require.NoError(t, err)
			assert.Equal(t, test.wantMasked, masked)
			if test.wantMasked {
				assert.JSONEq(t, test.want, string(got))
				assert.NotContains(t, string(got), "s3cret")
			} else {
				assert.Equal(t, test.want, string(got))
			}
		})
	}
}

func TestAPICmd_RegistersRevealSecretsFlag(t *testing.T) {
	cmd := apiCmd(&cli{renderer: display.NewRenderer()})

	flag := cmd.Flags().Lookup("reveal-secrets")
	require.NotNil(t, flag)
	assert.Equal(t, "false", flag.DefValue)
	assert.Equal(t, "r", flag.Shorthand)
}
