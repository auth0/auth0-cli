package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/auth0/go-auth0/management"
	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/auth0/go-auth0/v3/management/option"
	"go.uber.org/mock/gomock"

	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
)

func TestAPIsPickerOptions(t *testing.T) {
	tests := []struct {
		name         string
		apis         []*management.ResourceServer
		apiError     error
		assertOutput func(t testing.TB, options pickerOptions)
		assertError  func(t testing.TB, err error)
	}{
		{
			name: "happy path",
			apis: []*management.ResourceServer{
				{
					ID:         auth0.String("some-id-1"),
					Identifier: auth0.String("some-audience-1"),
					Name:       auth0.String("some-name-1"),
				},
				{
					ID:         auth0.String("some-id-2"),
					Identifier: auth0.String("some-audience-2"),
					Name:       auth0.String("some-name-2"),
				},
			},
			assertOutput: func(t testing.TB, options pickerOptions) {
				assert.Len(t, options, 2)
				assert.Equal(t, "some-name-1 (some-audience-1)", options[0].label)
				assert.Equal(t, "some-id-1", options[0].value)
				assert.Equal(t, "some-name-2 (some-audience-2)", options[1].label)
				assert.Equal(t, "some-id-2", options[1].value)
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name: "APIs with subject type authorization",
			apis: []*management.ResourceServer{
				{
					ID:         auth0.String("api-id-1"),
					Identifier: auth0.String("https://api.example.com"),
					Name:       auth0.String("Example API"),
					SubjectTypeAuthorization: &management.ResourceServerSubjectTypeAuthorization{
						User: &management.ResourceServerSubjectTypeAuthorizationUser{
							Policy: auth0.String("allow_all"),
						},
						Client: &management.ResourceServerSubjectTypeAuthorizationClient{
							Policy: auth0.String("deny_all"),
						},
					},
				},
				{
					ID:         auth0.String("api-id-2"),
					Identifier: auth0.String("https://secure-api.example.com"),
					Name:       auth0.String("Secure API"),
					SubjectTypeAuthorization: &management.ResourceServerSubjectTypeAuthorization{
						User: &management.ResourceServerSubjectTypeAuthorizationUser{
							Policy: auth0.String("require_client_grant"),
						},
						Client: &management.ResourceServerSubjectTypeAuthorizationClient{
							Policy: auth0.String("deny_all"),
						},
					},
				},
			},
			assertOutput: func(t testing.TB, options pickerOptions) {
				assert.Len(t, options, 2)
				assert.Equal(t, "Example API (https://api.example.com)", options[0].label)
				assert.Equal(t, "api-id-1", options[0].value)
				assert.Equal(t, "Secure API (https://secure-api.example.com)", options[1].label)
				assert.Equal(t, "api-id-2", options[1].value)
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name: "no apis",
			apis: []*management.ResourceServer{},
			assertOutput: func(t testing.TB, options pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorContains(t, err, "there are currently no APIs to choose from. Create one by running: `auth0 apis create`")
			},
		},
		{
			name:     "API error",
			apiError: errors.New("error"),
			assertOutput: func(t testing.TB, options pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.Error(t, err)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			apiAPI := mock.NewMockResourceServerAPI(ctrl)
			apiAPI.EXPECT().
				List(gomock.Any()).
				Return(&management.ResourceServerList{
					ResourceServers: test.apis}, test.apiError)

			cli := &cli{
				api: &auth0.API{ResourceServer: apiAPI},
			}

			options, err := cli.apiPickerOptions(context.Background())

			if err != nil {
				test.assertError(t, err)
			} else {
				test.assertOutput(t, options)
			}
		})
	}
}

// newResourceServerSearchPages chains pages of search results, ending with core.ErrNoPages.
func newResourceServerSearchPages(pages ...[]string) *auth0.ResourceServerSearchPage {
	var build func(i int) *auth0.ResourceServerSearchPage
	build = func(i int) *auth0.ResourceServerSearchPage {
		var results []*managementv3.ResourceServerSearchResponse
		for _, id := range pages[i] {
			results = append(results, &managementv3.ResourceServerSearchResponse{ID: auth0.String(id)})
		}

		return &auth0.ResourceServerSearchPage{
			Results: results,
			NextPageFunc: func(context.Context) (*auth0.ResourceServerSearchPage, error) {
				if i+1 >= len(pages) {
					return nil, core.ErrNoPages
				}
				return build(i + 1), nil
			},
		}
	}

	return build(0)
}

func TestResourceServerSortField(t *testing.T) {
	tests := []struct {
		input   string
		want    managementv3.ResourceServerSortFieldEnum
		wantErr string
	}{
		{input: "name", want: managementv3.ResourceServerSortFieldEnumName},
		{input: "identifier", want: managementv3.ResourceServerSortFieldEnumIdentifier},
		{input: "updated_at", want: managementv3.ResourceServerSortFieldEnumUpdatedAt},
		{input: "name:1", want: managementv3.ResourceServerSortFieldEnumName},
		{input: "name:-1", wantErr: `invalid --sort "name:-1": the search endpoint only sorts ascending, use 'field' or 'field:1'`},
		{input: "created_at", wantErr: `invalid --sort "created_at": must be 'name', 'identifier' or 'updated_at'`},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := resourceServerSortField(test.input)
			if test.wantErr != "" {
				assert.EqualError(t, err, test.wantErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestBuildResourceServerSearchRequest(t *testing.T) {
	tests := []struct {
		name    string
		inputs  apiSearchInputs
		want    *managementv3.SearchResourceServersRequestParameters
		wantErr string
	}{
		{
			name:   "defaults to lucene and omits unset params",
			inputs: apiSearchInputs{Number: 50},
			want: &managementv3.SearchResourceServersRequestParameters{
				Parser: managementv3.SearchParserEnumLucene.Ptr(),
				Take:   auth0.Int(50),
			},
		},
		{
			name:   "maps every flag onto the request",
			inputs: apiSearchInputs{Filter: `name co "billing"`, Parser: "scim", Sort: "updated_at:1", Fields: "id,name", Number: 20},
			want: &managementv3.SearchResourceServersRequestParameters{
				Q:      auth0.String(`name co "billing"`),
				Parser: managementv3.SearchParserEnumSCIM.Ptr(),
				Sort:   managementv3.ResourceServerSortFieldEnumUpdatedAt.Ptr(),
				Fields: auth0.String("id,name"),
				Take:   auth0.Int(20),
			},
		},
		{
			name:   "caps the page size at 100",
			inputs: apiSearchInputs{Number: 1000},
			want: &managementv3.SearchResourceServersRequestParameters{
				Parser: managementv3.SearchParserEnumLucene.Ptr(),
				Take:   auth0.Int(100),
			},
		},
		{
			name:   "trims and normalizes the fields list",
			inputs: apiSearchInputs{Fields: " id, name ,,identifier ", Number: 50},
			want: &managementv3.SearchResourceServersRequestParameters{
				Fields: auth0.String("id,name,identifier"),
				Parser: managementv3.SearchParserEnumLucene.Ptr(),
				Take:   auth0.Int(50),
			},
		},
		{
			name:    "rejects an unknown parser",
			inputs:  apiSearchInputs{Parser: "sql", Number: 50},
			wantErr: `invalid --parser "sql": must be 'lucene' or 'scim'`,
		},
		{
			name:    "rejects an invalid sort",
			inputs:  apiSearchInputs{Sort: "name:-1", Number: 50},
			wantErr: `invalid --sort "name:-1": the search endpoint only sorts ascending, use 'field' or 'field:1'`,
		},
		{
			name:   "sends include_fields=false with --exclude-fields",
			inputs: apiSearchInputs{Fields: "scopes", ExcludeFields: true, Number: 50},
			want: &managementv3.SearchResourceServersRequestParameters{
				Fields:        auth0.String("scopes"),
				IncludeFields: auth0.Bool(false),
				Parser:        managementv3.SearchParserEnumLucene.Ptr(),
				Take:          auth0.Int(50),
			},
		},
		{
			name:    "rejects --exclude-fields without --fields",
			inputs:  apiSearchInputs{ExcludeFields: true, Number: 50},
			wantErr: "--exclude-fields requires --fields",
		},
		{
			name:    "rejects a --filter over 1000 characters",
			inputs:  apiSearchInputs{Filter: strings.Repeat("a", 1001), Number: 50},
			wantErr: "--filter must be at most 1000 characters",
		},
		{
			name:   "counts --filter length in characters, not bytes",
			inputs: apiSearchInputs{Filter: strings.Repeat("界", 400), Number: 50},
			want: &managementv3.SearchResourceServersRequestParameters{
				Q:      auth0.String(strings.Repeat("界", 400)),
				Parser: managementv3.SearchParserEnumLucene.Ptr(),
				Take:   auth0.Int(50),
			},
		},
		{
			name:   "measures --fields after trimming",
			inputs: apiSearchInputs{Fields: strings.Repeat(" ,", 600) + "id", Number: 50},
			want: &managementv3.SearchResourceServersRequestParameters{
				Fields: auth0.String("id"),
				Parser: managementv3.SearchParserEnumLucene.Ptr(),
				Take:   auth0.Int(50),
			},
		},
		{
			name:   "treats a blank --fields as unset",
			inputs: apiSearchInputs{Fields: " , ,", Number: 50},
			want: &managementv3.SearchResourceServersRequestParameters{
				Parser: managementv3.SearchParserEnumLucene.Ptr(),
				Take:   auth0.Int(50),
			},
		},
		{
			name:    "rejects --exclude-fields with a blank --fields",
			inputs:  apiSearchInputs{Fields: " , ", ExcludeFields: true, Number: 50},
			wantErr: "--exclude-fields requires --fields",
		},
		{
			name:    "rejects --fields over 1000 characters",
			inputs:  apiSearchInputs{Fields: strings.Repeat("a", 1001), Number: 50},
			wantErr: "--fields must be at most 1000 characters",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := buildResourceServerSearchRequest(test.inputs)
			if test.wantErr != "" {
				assert.EqualError(t, err, test.wantErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestSearchApisCmd(t *testing.T) {
	thousandIDs := make([]string, 1000)
	for i := range thousandIDs {
		thousandIDs[i] = fmt.Sprintf("rs_%d", i)
	}

	tests := []struct {
		name          string
		args          []string
		searchPage    *auth0.ResourceServerSearchPage
		searchErr     error
		serverStatus  int
		serverBody    string
		assertRequest func(t testing.TB, req *managementv3.SearchResourceServersRequestParameters)
		assertHTTP    func(t testing.TB, r *http.Request)
		assertOutput  func(t testing.TB, stdout, stderr string)
		wantErr       string
	}{
		{
			name:       "sends the flags to the search endpoint",
			args:       []string{"--filter", `name:"My API"`, "-s", "name", "--fields", "id,name", "-n", "10"},
			searchPage: newResourceServerSearchPages([]string{"rs_1"}),
			assertRequest: func(t testing.TB, req *managementv3.SearchResourceServersRequestParameters) {
				assert.Equal(t, `name:"My API"`, *req.Q)
				assert.Equal(t, managementv3.SearchParserEnumLucene, *req.Parser)
				assert.Equal(t, managementv3.ResourceServerSortFieldEnumName, *req.Sort)
				assert.Equal(t, "id,name", *req.Fields)
				assert.Equal(t, 10, *req.Take)
			},
		},
		{
			name:       "sends --exclude-fields as include_fields=false",
			args:       []string{"--fields", "scopes", "--exclude-fields"},
			searchPage: newResourceServerSearchPages([]string{"rs_1"}),
			assertRequest: func(t testing.TB, req *managementv3.SearchResourceServersRequestParameters) {
				assert.Equal(t, "scopes", *req.Fields)
				assert.False(t, *req.IncludeFields)
			},
			assertOutput: func(t testing.TB, stdout, stderr string) {
				assert.Contains(t, stdout, "IDENTIFIER")
				assert.NotContains(t, stdout, "SCOPES")
			},
		},
		{
			name:       "collects results across pages up to --number",
			args:       []string{"--filter", "name:*", "-n", "3", "--json"},
			searchPage: newResourceServerSearchPages([]string{"rs_1", "rs_2"}, []string{"rs_3", "rs_4"}),
			assertOutput: func(t testing.TB, stdout, stderr string) {
				assert.Contains(t, stdout, "rs_3")
				assert.NotContains(t, stdout, "rs_4")
				assert.Contains(t, stderr, "Results may be limited by --number; more may match.")
			},
		},
		{
			name:       "does not suggest raising --number when it is already at the maximum",
			args:       []string{"-n", "1000", "--json"},
			searchPage: newResourceServerSearchPages(thousandIDs),
			assertOutput: func(t testing.TB, stdout, stderr string) {
				assert.Contains(t, stderr, "more may match. Refine --filter.")
				assert.NotContains(t, stderr, "raise --number")
			},
		},
		{
			name:       "renders an empty result",
			args:       []string{"--filter", `name:"missing"`},
			searchPage: newResourceServerSearchPages([]string{}),
		},
		{
			name:       "sends --query JSON params to the search endpoint",
			args:       []string{"-q", `{"q":"name co \"billing\"","parser":"scim"}`},
			serverBody: `{"resource_servers":[{"id":"rs_1"}]}`,
			assertHTTP: func(t testing.TB, r *http.Request) {
				assert.Equal(t, "/api/v2/resource-servers/search", r.URL.Path)
				assert.Equal(t, `name co "billing"`, r.URL.Query().Get("q"))
				assert.Equal(t, "scim", r.URL.Query().Get("parser"))
			},
			assertOutput: func(t testing.TB, stdout, stderr string) {
				assert.Contains(t, stdout, "rs_1")
			},
		},
		{
			name:       "allows --number with --query, which ignores it",
			args:       []string{"-q", `{"q":"name:*"}`, "-n", "5"},
			serverBody: `{"resource_servers":[]}`,
			assertHTTP: func(t testing.TB, r *http.Request) {
				assert.Empty(t, r.URL.Query().Get("take"))
			},
		},
		{
			name:    "rejects --query combined with --filter",
			args:    []string{"-q", `{"q":"name:*"}`, "--filter", "name:*"},
			wantErr: "if any flags in the group [query filter] are set none of the others can be; [filter query] were all set",
		},
		{
			name:    "rejects --query combined with --parser",
			args:    []string{"-q", `{"q":"name:*"}`, "--parser", "scim"},
			wantErr: "if any flags in the group [query parser] are set none of the others can be; [parser query] were all set",
		},
		{
			name:    "rejects --query combined with --sort",
			args:    []string{"-q", `{"q":"name:*"}`, "--sort", "name"},
			wantErr: "if any flags in the group [query sort] are set none of the others can be; [query sort] were all set",
		},
		{
			name:    "rejects --query combined with --fields",
			args:    []string{"-q", `{"q":"name:*"}`, "--fields", "id"},
			wantErr: "if any flags in the group [query fields] are set none of the others can be; [fields query] were all set",
		},
		{
			name:       "warns that --number is ignored with --query",
			args:       []string{"-q", `{"q":"name:*"}`, "-n", "500"},
			serverBody: `{"resource_servers":[]}`,
			assertOutput: func(t testing.TB, stdout, stderr string) {
				assert.Contains(t, stderr, `--number is ignored with --query; set "take" (max 100) in the JSON instead.`)
			},
		},
		{
			name:    "rejects --csv when --fields leaves no table column",
			args:    []string{"--fields", "token_lifetime", "--csv"},
			wantErr: "--csv needs at least one table column (id, name, identifier, scopes) left by --fields",
		},
		{
			name:    "rejects --query combined with --exclude-fields",
			args:    []string{"-q", `{"q":"name:*"}`, "--exclude-fields"},
			wantErr: "if any flags in the group [query exclude-fields] are set none of the others can be; [exclude-fields query] were all set",
		},
		{
			name:    "rejects an out-of-range --number before calling the API",
			args:    []string{"--filter", "name:*", "-n", "1001"},
			wantErr: "number flag invalid, please pass a number between 1 and 1000",
		},
		{
			name:      "wraps a generic API error",
			args:      []string{"--filter", "name:*"},
			searchErr: errors.New("boom"),
			wantErr:   "failed to search APIs: boom",
		},
		{
			name:      "explains a search timeout",
			args:      []string{"--filter", "name:*"},
			searchErr: core.NewAPIError(http.StatusGatewayTimeout, nil, errors.New("timeout")),
			wantErr:   "failed to search APIs: the search timed out, simplify your query and try again: 504: timeout",
		},
		{
			name:      "explains an unavailable Early Access endpoint",
			args:      []string{"--filter", "name:*"},
			searchErr: core.NewAPIError(http.StatusNotFound, nil, errors.New("not found")),
			wantErr:   "failed to search APIs: the search endpoint is in Early Access and may not be enabled for this tenant: 404: not found",
		},
		{
			name:         "explains a search timeout in --query mode",
			args:         []string{"-q", `{"q":"name:*"}`},
			serverStatus: http.StatusGatewayTimeout,
			wantErr:      "failed to search APIs: the search timed out, simplify your query and try again: 504: API request failed: Gateway Timeout",
		},
		{
			name:         "explains an unavailable Early Access endpoint in --query mode",
			args:         []string{"-q", `{"q":"name:*"}`},
			serverStatus: http.StatusNotFound,
			wantErr:      "failed to search APIs: the search endpoint is in Early Access and may not be enabled for this tenant: 404: API request failed: Not Found",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var captured *managementv3.SearchResourceServersRequestParameters
			searchAPI := mock.NewMockResourceServerAPIV3(ctrl)
			if test.searchPage != nil || test.searchErr != nil {
				searchAPI.EXPECT().
					Search(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req *managementv3.SearchResourceServersRequestParameters, _ ...option.RequestOption) (*auth0.ResourceServerSearchPage, error) {
						captured = req
						return test.searchPage, test.searchErr
					})
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if test.assertHTTP != nil {
					test.assertHTTP(t, r)
				}
				if test.serverStatus != 0 {
					w.WriteHeader(test.serverStatus)
					_, _ = w.Write([]byte(http.StatusText(test.serverStatus)))
					return
				}
				_, _ = w.Write([]byte(test.serverBody))
			}))
			defer server.Close()

			renderer := testRenderer()
			cli := &cli{
				api:      &auth0.API{HTTPClient: &mockHTTPClientAPI{baseURL: server.URL}},
				apiv3:    &auth0.APIV3{ResourceServerV3: searchAPI},
				renderer: renderer,
			}

			cmd := searchApisCmd(cli)
			cmd.SetArgs(test.args)
			err := cmd.Execute()

			if test.wantErr != "" {
				assert.EqualError(t, err, test.wantErr)
				return
			}

			assert.NoError(t, err)
			if test.assertRequest != nil {
				test.assertRequest(t, captured)
			}
			if test.assertOutput != nil {
				test.assertOutput(t, fmt.Sprint(renderer.ResultWriter), fmt.Sprint(renderer.MessageWriter))
			}
		})
	}
}
