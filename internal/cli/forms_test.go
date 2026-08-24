package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auth0/go-auth0/management"
	managementv3 "github.com/auth0/go-auth0/v3/management"
	"github.com/auth0/go-auth0/v3/management/core"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/iostream"
)

func TestApplyRawFormOverrides(t *testing.T) {
	body := json.RawMessage(`{
		"name":"Original",
		"languages":{"primary":"en","default":"fr"},
		"start":{},
		"nodes":[
			{"id":"step_1","type":"STEP","config":{"components":[{"id":"field_1","category":"FIELD","type":"TEXT"}]}},
			{"id":"router_1","type":"ROUTER","config":{"rules":[{"id":"rule_1","condition":{"operator":"AND"}}]}}
		],
		"ending":null
	}`)

	got, err := applyRawFormOverrides(body, "Renamed", "de", "")
	require.NoError(t, err)

	var form map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(got, &form))

	assert.JSONEq(t, `"Renamed"`, string(form["name"]))
	assert.JSONEq(t, `{"primary":"de","default":"fr"}`, string(form["languages"]))
	assert.JSONEq(t, `{}`, string(form["start"]))
	assert.JSONEq(t, `null`, string(form["ending"]))
	assert.Contains(t, string(form["nodes"]), `"components"`)
	assert.Contains(t, string(form["nodes"]), `"condition"`)
}

func TestApplyRawFormOverridesRejectsNonObject(t *testing.T) {
	_, err := applyRawFormOverrides(json.RawMessage(`[]`), "", "", "")
	assert.ErrorContains(t, err, "cannot unmarshal array")
}

func TestCreateFormCmdUsesRawClientForSimpleScaffold(t *testing.T) {
	httpClient := &formHTTPClientStub{
		response: json.RawMessage(`{"id":"ap_simple","name":"Simple Form","start":{},"nodes":[],"ending":{}}`),
	}
	stdout := &bytes.Buffer{}
	c := &cli{
		api: &auth0.API{HTTPClient: httpClient},
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  stdout,
		},
	}

	cmd := createFormCmd(c)
	cmd.SetArgs([]string{"--name", "Simple Form"})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, http.MethodPost, httpClient.method)
	require.IsType(t, json.RawMessage{}, httpClient.payload)
	assert.JSONEq(t, `{"name":"Simple Form","start":{},"nodes":[],"ending":{}}`, string(httpClient.payload.(json.RawMessage)))
	assert.Contains(t, stdout.String(), "Simple Form")
}

func TestCreateFormCmdUsesRawClientForRichFile(t *testing.T) {
	body := []byte(`{
		"name":"Rich Form",
		"nodes":[{"id":"step_1","type":"STEP","config":{"components":[{"id":"field_1","category":"FIELD","type":"TEXT"}]}}]
	}`)
	path := filepath.Join(t.TempDir(), "form.json")
	require.NoError(t, os.WriteFile(path, body, 0600))

	httpClient := &formHTTPClientStub{
		response: json.RawMessage(`{
			"id":"ap_rich",
			"name":"Rich Form",
			"nodes":[{"id":"step_1","type":"STEP","config":{"components":[{"id":"field_1","category":"FIELD","type":"TEXT"}]}}]
		}`),
	}
	stdout := &bytes.Buffer{}
	c := &cli{
		api: &auth0.API{HTTPClient: httpClient},
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  stdout,
		},
	}

	cmd := createFormCmd(c)
	cmd.SetArgs([]string{"--file", path})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, http.MethodPost, httpClient.method)
	require.IsType(t, json.RawMessage{}, httpClient.payload)
	assert.Contains(t, string(httpClient.payload.(json.RawMessage)), `"components"`)
	assert.Contains(t, stdout.String(), "1 nodes")
}

func TestShowFormCmdUsesRawClient(t *testing.T) {
	httpClient := &formHTTPClientStub{
		response: json.RawMessage(`{
			"id":"ap_rich",
			"name":"Rich Form",
			"flow_count":2,
			"links":{"self":"https://example.test/forms/ap_rich"},
			"nodes":[{"id":"router_1","type":"ROUTER","config":{"rules":[{"id":"rule_1","condition":{"operator":"AND"}}]}}]
		}`),
	}
	stdout := &bytes.Buffer{}
	c := &cli{
		api: &auth0.API{HTTPClient: httpClient},
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  stdout,
		},
	}

	cmd := showFormCmd(c)
	cmd.SetArgs([]string{"ap_rich"})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, http.MethodGet, httpClient.method)
	assert.Contains(t, stdout.String(), "1 nodes")
}

func TestUpdateFormCmdUsesRawClientForScalarUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	formAPI := mock.NewMockFormAPIV3(ctrl)
	formAPI.EXPECT().
		Get(gomock.Any(), "ap_simple", gomock.Any()).
		Return(&managementv3.GetFormResponseContent{
			ID:   "ap_simple",
			Name: "Original",
			Languages: &managementv3.FormLanguages{
				Primary: auth0.String("en"),
				Default: auth0.String("fr"),
			},
		}, nil)
	httpClient := &formHTTPClientStub{
		response: json.RawMessage(`{"id":"ap_simple","name":"Renamed","languages":{"primary":"de","default":"fr"}}`),
	}

	stdout := &bytes.Buffer{}
	c := &cli{
		api:   &auth0.API{HTTPClient: httpClient},
		apiv3: &auth0.APIV3{Form: formAPI},
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  stdout,
		},
	}

	cmd := updateFormCmd(c)
	cmd.SetArgs([]string{"ap_simple", "--name", "Renamed", "--language-primary", "de"})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, http.MethodPatch, httpClient.method)
	require.IsType(t, json.RawMessage{}, httpClient.payload)
	assert.JSONEq(t, `{"name":"Renamed","languages":{"primary":"de","default":"fr"}}`, string(httpClient.payload.(json.RawMessage)))
	assert.Contains(t, stdout.String(), "Renamed")
}

func TestUpdateFormCmdUsesRawClientForRichFile(t *testing.T) {
	body := []byte(`{
		"id":"ap_rich",
		"name":"Rich Form",
		"nodes":[{"id":"router_1","type":"ROUTER","config":{"rules":[{"id":"rule_1","condition":{"operator":"AND"}}]}}],
		"ending":null,
		"created_at":"2026-08-24T00:00:00Z",
		"updated_at":"2026-08-24T00:00:00Z",
		"flow_count":0,
		"links":{}
	}`)
	path := filepath.Join(t.TempDir(), "form.json")
	require.NoError(t, os.WriteFile(path, body, 0600))

	httpClient := &formHTTPClientStub{
		response: json.RawMessage(`{
			"id":"ap_rich",
			"name":"Rich Form",
			"nodes":[{"id":"router_1","type":"ROUTER","config":{"rules":[{"id":"rule_1","condition":{"operator":"AND"}}]}}],
			"ending":null
		}`),
	}
	stdout := &bytes.Buffer{}
	c := &cli{
		api: &auth0.API{HTTPClient: httpClient},
		renderer: &display.Renderer{
			MessageWriter: io.Discard,
			ResultWriter:  stdout,
		},
	}

	cmd := updateFormCmd(c)
	cmd.SetArgs([]string{"ap_rich", "--file", path})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, http.MethodPatch, httpClient.method)
	require.IsType(t, json.RawMessage{}, httpClient.payload)
	payload := httpClient.payload.(json.RawMessage)
	assert.Contains(t, string(payload), `"condition"`)
	assert.Contains(t, string(payload), `"ending":null`)
	var form map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(payload, &form))
	assert.NotContains(t, form, "id")
	assert.NotContains(t, form, "created_at")
	assert.NotContains(t, form, "updated_at")
	assert.NotContains(t, form, "flow_count")
	assert.NotContains(t, form, "links")
	assert.Contains(t, stdout.String(), "1 nodes")
}

func TestReadBodyInput(t *testing.T) {
	t.Run("reads from a file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "form.json")
		want := []byte(`{"name":"My Form"}`)
		assert.NoError(t, os.WriteFile(path, want, 0600))

		got, err := readBodyInput(path, "form")
		assert.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("errors on a missing file", func(t *testing.T) {
		_, err := readBodyInput(filepath.Join(t.TempDir(), "missing.json"), "form")
		assert.ErrorContains(t, err, "failed to read form file")
	})

	t.Run("reads from stdin when file is '-'", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "stdin.json")
		want := []byte(`{"name":"Piped Form"}`)
		assert.NoError(t, os.WriteFile(path, want, 0600))

		f, err := os.Open(path)
		assert.NoError(t, err)
		defer f.Close()

		original := iostream.Input
		iostream.Input = f
		defer func() { iostream.Input = original }()

		got, err := readBodyInput("-", "form")
		assert.NoError(t, err)
		assert.Equal(t, want, got)
	})
}

func TestFormPickerOptions(t *testing.T) {
	tests := []struct {
		name         string
		forms        []*managementv3.FormSummary
		apiError     error
		assertOutput func(t testing.TB, options pickerOptions)
		assertError  func(t testing.TB, err error)
	}{
		{
			name: "happy path",
			forms: []*managementv3.FormSummary{
				{ID: "some-id-1", Name: "some-name-1"},
				{ID: "some-id-2", Name: "some-name-2"},
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
			name:  "no forms",
			forms: []*managementv3.FormSummary{},
			assertOutput: func(t testing.TB, options pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorContains(t, err, "there are currently no forms to choose from. Create one by running: `auth0 forms create`")
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

			formAPI := mock.NewMockFormAPIV3(ctrl)
			if test.apiError != nil {
				formAPI.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, test.apiError)
			} else {
				formAPI.EXPECT().List(gomock.Any(), gomock.Any()).Return(
					&auth0.FormSummaryPage{
						Results: test.forms,
						NextPageFunc: func(_ context.Context) (*auth0.FormSummaryPage, error) {
							return nil, core.ErrNoPages
						},
					}, nil)
			}

			cli := &cli{
				apiv3: &auth0.APIV3{Form: formAPI},
			}

			options, err := cli.formPickerOptions(context.Background())

			if err != nil {
				test.assertError(t, err)
			} else {
				test.assertOutput(t, options)
			}
		})
	}
}

func TestCollectForms(t *testing.T) {
	t.Run("pages across responses until exhausted", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		secondPage := &auth0.FormSummaryPage{
			Results: []*managementv3.FormSummary{{ID: "id-3", Name: "Form 3"}},
			NextPageFunc: func(_ context.Context) (*auth0.FormSummaryPage, error) {
				return nil, core.ErrNoPages
			},
		}
		firstPage := &auth0.FormSummaryPage{
			Results: []*managementv3.FormSummary{
				{ID: "id-1", Name: "Form 1"},
				{ID: "id-2", Name: "Form 2"},
			},
			NextPageFunc: func(_ context.Context) (*auth0.FormSummaryPage, error) {
				return secondPage, nil
			},
		}

		formAPI := mock.NewMockFormAPIV3(ctrl)
		formAPI.EXPECT().List(gomock.Any(), gomock.Any()).Return(firstPage, nil)

		cli := &cli{apiv3: &auth0.APIV3{Form: formAPI}}

		forms, err := collectForms(context.Background(), cli, &managementv3.ListFormsRequestParameters{}, 0)
		assert.NoError(t, err)
		assert.Len(t, forms, 3)
		assert.Equal(t, "id-3", forms[2].GetID())
	})

	t.Run("stops at the requested limit without paging further", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		firstPage := &auth0.FormSummaryPage{
			Results: []*managementv3.FormSummary{
				{ID: "id-1", Name: "Form 1"},
				{ID: "id-2", Name: "Form 2"},
			},
			NextPageFunc: func(_ context.Context) (*auth0.FormSummaryPage, error) {
				t.Fatal("should not page past the limit")
				return nil, nil
			},
		}

		formAPI := mock.NewMockFormAPIV3(ctrl)
		formAPI.EXPECT().List(gomock.Any(), gomock.Any()).Return(firstPage, nil)

		cli := &cli{apiv3: &auth0.APIV3{Form: formAPI}}

		forms, err := collectForms(context.Background(), cli, &managementv3.ListFormsRequestParameters{}, 1)
		assert.NoError(t, err)
		assert.Len(t, forms, 1)
		assert.Equal(t, "id-1", forms[0].GetID())
	})

	t.Run("returns the list error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		formAPI := mock.NewMockFormAPIV3(ctrl)
		formAPI.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))

		cli := &cli{apiv3: &auth0.APIV3{Form: formAPI}}

		_, err := collectForms(context.Background(), cli, &managementv3.ListFormsRequestParameters{}, 0)
		assert.EqualError(t, err, "boom")
	})
}

type formHTTPClientStub struct {
	method   string
	payload  interface{}
	response json.RawMessage
}

func (s *formHTTPClientStub) NewRequest(
	ctx context.Context,
	method string,
	uri string,
	payload interface{},
	_ ...management.RequestOption,
) (*http.Request, error) {
	s.method = method
	s.payload = payload
	return http.NewRequestWithContext(ctx, method, uri, nil)
}

func (s *formHTTPClientStub) Do(_ *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(string(s.response))),
	}, nil
}

func (s *formHTTPClientStub) Request(
	context.Context,
	string,
	string,
	interface{},
	...management.RequestOption,
) error {
	return nil
}

func (s *formHTTPClientStub) URI(path ...string) string {
	return "https://example.test/api/v2/" + strings.Join(path, "/")
}
