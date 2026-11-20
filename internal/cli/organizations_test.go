package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestOrganizationsPickerOptions(t *testing.T) {
	tests := []struct {
		name          string
		organizations []*management.Organization
		apiError      error
		assertOutput  func(t testing.TB, options pickerOptions)
		assertError   func(t testing.TB, err error)
	}{
		{
			name: "happy path",
			organizations: []*management.Organization{
				{
					ID:   auth0.String("some-id-1"),
					Name: auth0.String("some-name-1"),
				},
				{
					ID:   auth0.String("some-id-2"),
					Name: auth0.String("some-name-2"),
				},
			},
			assertOutput: func(t testing.TB, options pickerOptions) {
				assert.Len(t, options, 2)
				assert.Equal(t, "some-name-1 (some-id-1)", options[0].label)
				assert.Equal(t, "some-id-1", options[0].value)
				assert.Equal(t, "some-name-2 (some-id-2)", options[1].label)
				assert.Equal(t, "some-id-2", options[1].value)
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name:          "no organizations",
			organizations: []*management.Organization{},
			assertOutput: func(t testing.TB, options pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorContains(t, err, "there are currently no organizations to choose from. Create one by running: `auth0 orgs create`")
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

			organizationAPI := mock.NewMockOrganizationAPI(ctrl)
			organizationAPI.EXPECT().
				List(gomock.Any()).
				Return(&management.OrganizationList{
					Organizations: test.organizations}, test.apiError)

			cli := &cli{
				api: &auth0.API{Organization: organizationAPI},
			}

			options, err := cli.organizationPickerOptions(context.Background())

			if err != nil {
				test.assertError(t, err)
			} else {
				test.assertOutput(t, options)
			}
		})
	}
}

func TestInvitationsPickerOptions(t *testing.T) {
	tests := []struct {
		name         string
		invitations  []*management.OrganizationInvitation
		apiError     error
		assertOutput func(t testing.TB, options pickerOptions)
		assertError  func(t testing.TB, err error)
	}{
		{
			name: "happy path",
			invitations: []*management.OrganizationInvitation{
				{
					ID: auth0.String("inv-id-1"),
					Invitee: &management.OrganizationInvitationInvitee{
						Email: auth0.String("user1@example.com"),
					},
				},
				{
					ID: auth0.String("inv-id-2"),
					Invitee: &management.OrganizationInvitationInvitee{
						Email: auth0.String("user2@example.com"),
					},
				},
			},
			assertOutput: func(t testing.TB, options pickerOptions) {
				assert.Len(t, options, 2)
				assert.Equal(t, "user1@example.com (inv-id-1)", options[0].label)
				assert.Equal(t, "inv-id-1", options[0].value)
				assert.Equal(t, "user2@example.com (inv-id-2)", options[1].label)
				assert.Equal(t, "inv-id-2", options[1].value)
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name:        "no invitations",
			invitations: []*management.OrganizationInvitation{},
			assertOutput: func(t testing.TB, options pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.Error(t, err)
				assert.ErrorContains(t, err, "there are currently no invitations to choose from")
			},
		},
		{
			name:     "API error",
			apiError: errors.New("api error"),
			assertOutput: func(t testing.TB, options pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.Error(t, err)
				assert.ErrorContains(t, err, "api error")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			organizationAPI := mock.NewMockOrganizationAPI(ctrl)
			organizationAPI.EXPECT().
				Invitations(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&management.OrganizationInvitationList{
					OrganizationInvitations: test.invitations,
				}, test.apiError)

			cli := &cli{
				api: &auth0.API{Organization: organizationAPI},
			}

			options, err := cli.invitationPickerOptions(context.Background(), "test-org-id")

			if err != nil {
				test.assertError(t, err)
			} else {
				test.assertOutput(t, options)
			}
		})
	}
}

// newOrganizationSearchPages chains pages of search results, ending with core.ErrNoPages.
func newOrganizationSearchPages(pages ...[]*managementv3.SearchOrganization) *auth0.OrganizationSearchPage {
	var build func(i int) *auth0.OrganizationSearchPage
	build = func(i int) *auth0.OrganizationSearchPage {
		return &auth0.OrganizationSearchPage{
			Results: pages[i],
			NextPageFunc: func(context.Context) (*auth0.OrganizationSearchPage, error) {
				if i+1 >= len(pages) {
					return nil, core.ErrNoPages
				}
				return build(i + 1), nil
			},
		}
	}

	return build(0)
}

// searchOrganizations returns one search result per organization ID.
func searchOrganizations(ids ...string) []*managementv3.SearchOrganization {
	organizations := make([]*managementv3.SearchOrganization, 0, len(ids))
	for _, id := range ids {
		organizations = append(organizations, &managementv3.SearchOrganization{ID: auth0.String(id)})
	}

	return organizations
}

func TestOrganizationSortField(t *testing.T) {
	tests := []struct {
		input   string
		want    managementv3.OrganizationSortFieldEnum
		wantErr string
	}{
		{input: "name", want: managementv3.OrganizationSortFieldEnumName},
		{input: "display_name", want: managementv3.OrganizationSortFieldEnumDisplayName},
		{input: "created_at", want: managementv3.OrganizationSortFieldEnumCreatedAt},
		{input: "name:1", want: managementv3.OrganizationSortFieldEnumName},
		{input: "name:-1", wantErr: `invalid --sort "name:-1": the search endpoint only sorts ascending, use 'field' or 'field:1'`},
		{input: "updated_at", wantErr: `invalid --sort "updated_at": must be one of name,display_name,created_at`},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := organizationSortField(test.input)
			if test.wantErr != "" {
				assert.EqualError(t, err, test.wantErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestBuildOrganizationSearchRequest(t *testing.T) {
	tests := []struct {
		name    string
		inputs  searchInputs
		want    *managementv3.SearchOrganizationsRequestParameters
		wantErr string
	}{
		{
			name:   "defaults to lucene and omits unset params",
			inputs: searchInputs{Number: 50},
			want: &managementv3.SearchOrganizationsRequestParameters{
				Parser: managementv3.SearchParserEnumLucene.Ptr(),
				Take:   auth0.Int(50),
			},
		},
		{
			name:   "maps every flag onto the request",
			inputs: searchInputs{Filter: `display_name co "Acme"`, Parser: "scim", Sort: "created_at:1", Number: 20},
			want: &managementv3.SearchOrganizationsRequestParameters{
				Q:      auth0.String(`display_name co "Acme"`),
				Parser: managementv3.SearchParserEnumSCIM.Ptr(),
				Sort:   managementv3.OrganizationSortFieldEnumCreatedAt.Ptr(),
				Take:   auth0.Int(20),
			},
		},
		{
			name:   "caps the page size at 100",
			inputs: searchInputs{Number: 1000},
			want: &managementv3.SearchOrganizationsRequestParameters{
				Parser: managementv3.SearchParserEnumLucene.Ptr(),
				Take:   auth0.Int(100),
			},
		},
		{
			name:    "rejects an invalid sort",
			inputs:  searchInputs{Sort: "updated_at", Number: 50},
			wantErr: `invalid --sort "updated_at": must be one of name,display_name,created_at`,
		},
		{
			name:    "rejects invalid shared inputs",
			inputs:  searchInputs{Parser: "sql", Number: 50},
			wantErr: `invalid --parser "sql": must be 'lucene' or 'scim'`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := buildOrganizationSearchRequest(test.inputs)
			if test.wantErr != "" {
				assert.EqualError(t, err, test.wantErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestSearchOrganizationsCmd(t *testing.T) {
	thousandIDs := make([]string, 1000)
	for i := range thousandIDs {
		thousandIDs[i] = fmt.Sprintf("org_%d", i)
	}

	tests := []struct {
		name          string
		args          []string
		searchPage    *auth0.OrganizationSearchPage
		searchErr     error
		serverStatus  int
		serverBody    string
		assertRequest func(t testing.TB, req *managementv3.SearchOrganizationsRequestParameters)
		assertHTTP    func(t testing.TB, r *http.Request)
		assertOutput  func(t testing.TB, stdout, stderr string)
		wantErr       string
	}{
		{
			name:       "sends the flags to the search endpoint",
			args:       []string{"--filter", `name:"acme"`, "-s", "display_name", "-n", "10"},
			searchPage: newOrganizationSearchPages(searchOrganizations("org_1")),
			assertRequest: func(t testing.TB, req *managementv3.SearchOrganizationsRequestParameters) {
				assert.Equal(t, `name:"acme"`, *req.Q)
				assert.Equal(t, managementv3.SearchParserEnumLucene, *req.Parser)
				assert.Equal(t, managementv3.OrganizationSortFieldEnumDisplayName, *req.Sort)
				assert.Equal(t, 10, *req.Take)
			},
		},
		{
			name: "renders the orgs list columns",
			searchPage: newOrganizationSearchPages([]*managementv3.SearchOrganization{
				{ID: auth0.String("org_1"), Name: auth0.String("acme"), DisplayName: auth0.String("Acme Inc")},
			}),
			assertOutput: func(t testing.TB, stdout, stderr string) {
				expectTable(t, stdout,
					[]string{"ID", "NAME", "DISPLAY NAME"},
					[][]string{{"org_1", "acme", "Acme Inc"}},
				)
				assert.Contains(t, stderr, "organizations (1)")
			},
		},
		{
			name:       "collects results across pages up to --number",
			args:       []string{"--filter", "name:*", "-n", "3", "--json"},
			searchPage: newOrganizationSearchPages(searchOrganizations("org_1", "org_2"), searchOrganizations("org_3", "org_4")),
			assertOutput: func(t testing.TB, stdout, stderr string) {
				assert.Contains(t, stdout, "org_3")
				assert.NotContains(t, stdout, "org_4")
				assert.Contains(t, stderr, "Results may be limited by --number; more may match.")
			},
		},
		{
			name:       "does not suggest raising --number when it is already at the maximum",
			args:       []string{"-n", "1000", "--json"},
			searchPage: newOrganizationSearchPages(searchOrganizations(thousandIDs...)),
			assertOutput: func(t testing.TB, stdout, stderr string) {
				assert.Contains(t, stderr, "more may match. Refine --filter.")
				assert.NotContains(t, stderr, "raise --number")
			},
		},
		{
			name:       "renders an empty result",
			args:       []string{"--filter", `name:"missing"`},
			searchPage: newOrganizationSearchPages(searchOrganizations()),
			assertOutput: func(t testing.TB, stdout, stderr string) {
				assert.Contains(t, stderr, "organizations (0)")
				assert.Contains(t, stderr, "Try a broader --filter")
			},
		},
		{
			name:       "sends --query JSON params to the search endpoint",
			args:       []string{"-q", `{"q":"name sw \"acme\"","parser":"scim"}`},
			serverBody: `{"organizations":[{"id":"org_1"}]}`,
			assertHTTP: func(t testing.TB, r *http.Request) {
				assert.Equal(t, "/api/v2/organizations/search", r.URL.Path)
				assert.Equal(t, `name sw "acme"`, r.URL.Query().Get("q"))
				assert.Equal(t, "scim", r.URL.Query().Get("parser"))
			},
			assertOutput: func(t testing.TB, stdout, stderr string) {
				assert.Contains(t, stdout, "org_1")
			},
		},
		{
			name:       "allows --number with --query, which ignores it",
			args:       []string{"-q", `{"q":"name:*"}`, "-n", "5"},
			serverBody: `{"organizations":[]}`,
			assertHTTP: func(t testing.TB, r *http.Request) {
				assert.Empty(t, r.URL.Query().Get("take"))
			},
			assertOutput: func(t testing.TB, stdout, stderr string) {
				assert.Contains(t, stderr, `--number is ignored with --query; set "take" (max 100) in the JSON instead.`)
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
			name:    "does not offer --fields, as the endpoint has no field selection",
			args:    []string{"--fields", "id"},
			wantErr: "unknown flag: --fields",
		},
		{
			name:    "does not offer --exclude-fields",
			args:    []string{"--exclude-fields"},
			wantErr: "unknown flag: --exclude-fields",
		},
		{
			name:    "rejects an invalid --sort before calling the API",
			args:    []string{"-s", "updated_at"},
			wantErr: `invalid --sort "updated_at": must be one of name,display_name,created_at`,
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
			wantErr:   "failed to search organizations: boom",
		},
		{
			name:      "explains a search timeout",
			args:      []string{"--filter", "name:*"},
			searchErr: core.NewAPIError(http.StatusGatewayTimeout, nil, errors.New("timeout")),
			wantErr:   "failed to search organizations: the search timed out, simplify your query and try again: 504: timeout",
		},
		{
			name:      "explains an unavailable Early Access endpoint",
			args:      []string{"--filter", "name:*"},
			searchErr: core.NewAPIError(http.StatusNotFound, nil, errors.New("not found")),
			wantErr:   "failed to search organizations: the search endpoint is in Early Access and may not be enabled for this tenant: 404: not found",
		},
		{
			name:         "explains a search timeout in --query mode",
			args:         []string{"-q", `{"q":"name:*"}`},
			serverStatus: http.StatusGatewayTimeout,
			wantErr:      "failed to search organizations: the search timed out, simplify your query and try again: 504: API request failed: Gateway Timeout",
		},
		{
			name:         "explains an unavailable Early Access endpoint in --query mode",
			args:         []string{"-q", `{"q":"name:*"}`},
			serverStatus: http.StatusNotFound,
			wantErr:      "failed to search organizations: the search endpoint is in Early Access and may not be enabled for this tenant: 404: API request failed: Not Found",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var captured *managementv3.SearchOrganizationsRequestParameters
			searchAPI := mock.NewMockOrganizationAPIV3(ctrl)
			if test.searchPage != nil || test.searchErr != nil {
				searchAPI.EXPECT().
					Search(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req *managementv3.SearchOrganizationsRequestParameters, _ ...option.RequestOption) (*auth0.OrganizationSearchPage, error) {
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
				apiv3:    &auth0.APIV3{OrganizationV3: searchAPI},
				renderer: renderer,
			}

			cmd := searchOrganizationsCmd(cli)
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
