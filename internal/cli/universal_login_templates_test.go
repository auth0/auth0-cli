package cli

import (
	"context"
	"net/http"
	"sync"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/golang/mock/gomock"

	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
)

type mockManagamentError struct {
	error
	status int
}

func (m mockManagamentError) Status() int {
	return m.status
}

func TestEnsureCustomDomainIsEnabled(t *testing.T) {
	tests := []struct {
		name         string
		customDomain []*management.CustomDomain
		apiError     management.Error
		assertOutput func(t testing.TB, err error)
	}{
		{
			name: "happy path",
			customDomain: []*management.CustomDomain{
				{
					Status: auth0.String("foo"),
				},
				{
					Status: auth0.String("ready"),
				},
			},
			assertOutput: func(t testing.TB, err error) {
				assert.Nil(t, err)
			},
		},
		{
			name: "no verified domains",
			customDomain: []*management.CustomDomain{
				{
					Status: auth0.String("foo"),
				},
			},
			assertOutput: func(t testing.TB, err error) {
				assert.ErrorIs(t, errNotAllowed, err)
			},
		},
		{
			name:     "custom domains are not enabled",
			apiError: mockManagamentError{status: http.StatusForbidden},
			assertOutput: func(t testing.TB, err error) {
				assert.ErrorIs(t, errNotAllowed, err)
			},
		},
		{
			name:     "api error",
			apiError: mockManagamentError{status: http.StatusServiceUnavailable},
			assertOutput: func(t testing.TB, err error) {
				assert.Error(t, err)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			customDomainAPI := mock.NewMockCustomDomainAPI(ctrl)
			customDomainAPI.EXPECT().
				List(gomock.Any()).
				Return(test.customDomain, test.apiError)

			ctx := context.Background()
			api := &auth0.API{CustomDomain: customDomainAPI}
			err := ensureCustomDomainIsEnabled(ctx, api)
			test.assertOutput(t, err)
		})
	}
}

func TestFetchBrandingSettingsOrUseDefaults(t *testing.T) {
	tests := []struct {
		name         string
		branding     *management.Branding
		apiError     management.Error
		assertOutput func(t testing.TB, branding *management.Branding)
	}{
		{
			name: "happy path",
			branding: &management.Branding{
				Colors: &management.BrandingColors{
					Primary:        auth0.String("#FF4F40"),
					PageBackground: auth0.String("#2A2E35"),
				},
				LogoURL: auth0.String("https://example.com/logo.png"),
			},
			assertOutput: func(t testing.TB, branding *management.Branding) {
				assert.NotNil(t, branding)
				assert.NotNil(t, branding.Colors)
				assert.Equal(t, branding.Colors.GetPrimary(), "#FF4F40")
				assert.Equal(t, branding.Colors.GetPageBackground(), "#2A2E35")
				assert.Equal(t, branding.GetLogoURL(), "https://example.com/logo.png")
			},
		},
		{
			name:     "no branding settings",
			branding: nil,
			assertOutput: func(t testing.TB, branding *management.Branding) {
				assert.NotNil(t, branding)
				assert.NotNil(t, branding.Colors)
				assert.Equal(t, branding.Colors.GetPrimary(), defaultPrimaryColor)
				assert.Equal(t, branding.Colors.GetPageBackground(), defaultBackgroundColor)
				assert.Equal(t, branding.GetLogoURL(), defaultLogoURL)
			},
		},
		{
			name:     "empty branding settings",
			branding: &management.Branding{},
			assertOutput: func(t testing.TB, branding *management.Branding) {
				assert.NotNil(t, branding)
				assert.NotNil(t, branding.Colors)
				assert.Equal(t, branding.Colors.GetPrimary(), defaultPrimaryColor)
				assert.Equal(t, branding.Colors.GetPageBackground(), defaultBackgroundColor)
				assert.Equal(t, branding.GetLogoURL(), defaultLogoURL)
			},
		},
		{
			name:     "api error",
			apiError: mockManagamentError{status: http.StatusServiceUnavailable},
			assertOutput: func(t testing.TB, branding *management.Branding) {
				assert.NotNil(t, branding)
				assert.Equal(t, branding.Colors.GetPrimary(), defaultPrimaryColor)
				assert.Equal(t, branding.Colors.GetPageBackground(), defaultBackgroundColor)
				assert.Equal(t, branding.GetLogoURL(), defaultLogoURL)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			brandingAPI := mock.NewMockBrandingAPI(ctrl)
			brandingAPI.EXPECT().
				Read(gomock.Any()).
				Return(test.branding, test.apiError)

			ctx := context.Background()
			api := &auth0.API{Branding: brandingAPI}
			branding := fetchBrandingSettingsOrUseDefaults(ctx, api)
			test.assertOutput(t, branding)
		})
	}
}

func TestFetchBrandingTemplateOrUseEmpty(t *testing.T) {
	tests := []struct {
		name         string
		brandingUL   *management.BrandingUniversalLogin
		apiError     management.Error
		assertOutput func(t testing.TB, branding *management.BrandingUniversalLogin)
	}{
		{
			name: "happy path",
			brandingUL: &management.BrandingUniversalLogin{
				Body: auth0.String("<html></html>"),
			},
			assertOutput: func(t testing.TB, branding *management.BrandingUniversalLogin) {
				assert.NotNil(t, branding)
				assert.Equal(t, branding.GetBody(), "<html></html>")
			},
		},
		{
			name:     "api error",
			apiError: mockManagamentError{status: http.StatusServiceUnavailable},
			assertOutput: func(t testing.TB, branding *management.BrandingUniversalLogin) {
				assert.NotNil(t, branding)
				assert.Equal(t, branding.GetBody(), "")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			brandingAPI := mock.NewMockBrandingAPI(ctrl)
			brandingAPI.EXPECT().
				UniversalLogin(gomock.Any()).
				Return(test.brandingUL, test.apiError)

			ctx := context.Background()
			api := &auth0.API{Branding: brandingAPI}
			branding := fetchBrandingTemplateOrUseEmpty(ctx, api)
			test.assertOutput(t, branding)
		})
	}
}

func TestFetchTemplateData(t *testing.T) {
	tests := []struct {
		name           string
		brandingUL     *management.BrandingUniversalLogin
		clients        []*management.Client
		customDomain   *management.CustomDomain
		prompt         *management.Prompt
		tenant         *management.Tenant
		clientAPIError management.Error
		promptAPIError management.Error
		tenantAPIError management.Error
		assertOutput   func(t testing.TB, templateData *TemplateData)
		assertError    func(t testing.TB, err error)
	}{
		{
			name: "happy path",
			brandingUL: &management.BrandingUniversalLogin{
				Body: auth0.String("<html></html>"),
			},
			clients: []*management.Client{
				{
					ClientID: auth0.String("some-client-id-1"),
					Name:     auth0.String("some-name-1"),
					LogoURI:  auth0.String("https://example.com/logo-1.png"),
				},
				{
					ClientID: auth0.String("some-client-id-2"),
					Name:     auth0.String("some-name-2"),
					LogoURI:  auth0.String("https://example.com/logo-2.png"),
				},
			},
			customDomain: &management.CustomDomain{
				Status: auth0.String("ready"),
			},
			prompt: &management.Prompt{
				UniversalLoginExperience: "classic",
			},
			tenant: &management.Tenant{
				FriendlyName: auth0.String("some-friendly-name"),
			},
			assertOutput: func(t testing.TB, templateData *TemplateData) {
				assert.NotEmpty(t, templateData.Clients)
				assert.Equal(t, templateData.BackgroundColor, defaultBackgroundColor)
				assert.Equal(t, templateData.PrimaryColor, defaultPrimaryColor)
				assert.Equal(t, templateData.LogoURL, defaultLogoURL)
				assert.Equal(t, templateData.Body, "<html></html>")
				assert.Equal(t, templateData.Clients[0].ID, "some-client-id-1")
				assert.Equal(t, templateData.Clients[0].Name, "some-name-1")
				assert.Equal(t, templateData.Clients[0].LogoURL, "https://example.com/logo-1.png")
				assert.Equal(t, templateData.Clients[1].ID, "some-client-id-2")
				assert.Equal(t, templateData.Clients[1].Name, "some-name-2")
				assert.Equal(t, templateData.Clients[1].LogoURL, "https://example.com/logo-2.png")
				assert.Equal(t, templateData.Experience, "classic")
				assert.Equal(t, templateData.TenantName, "some-friendly-name")
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name: "client api error",
			customDomain: &management.CustomDomain{
				Status: auth0.String("ready"),
			},
			prompt: &management.Prompt{
				UniversalLoginExperience: "",
			},
			tenant: &management.Tenant{
				FriendlyName: auth0.String(""),
			},
			clientAPIError: mockManagamentError{status: http.StatusServiceUnavailable},
			assertOutput: func(t testing.TB, templateData *TemplateData) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.Error(t, err)
			},
		},
		{
			name: "prompt api error",
			clients: []*management.Client{
				{
					ClientID: auth0.String(""),
					Name:     auth0.String(""),
					LogoURI:  auth0.String(""),
				},
			},
			customDomain: &management.CustomDomain{
				Status: auth0.String("ready"),
			},
			tenant: &management.Tenant{
				FriendlyName: auth0.String(""),
			},
			promptAPIError: mockManagamentError{status: http.StatusServiceUnavailable},
			assertOutput: func(t testing.TB, templateData *TemplateData) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.Error(t, err)
			},
		},
		{
			name: "tenant api error",
			clients: []*management.Client{
				{
					ClientID: auth0.String(""),
					Name:     auth0.String(""),
					LogoURI:  auth0.String(""),
				},
			},
			customDomain: &management.CustomDomain{
				Status: auth0.String("ready"),
			},
			prompt: &management.Prompt{
				UniversalLoginExperience: "",
			},
			tenantAPIError: mockManagamentError{status: http.StatusServiceUnavailable},
			assertOutput: func(t testing.TB, templateData *TemplateData) {
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

			wg := sync.WaitGroup{}
			wg.Add(5)

			brandingAPI := mock.NewMockBrandingAPI(ctrl)
			brandingAPI.EXPECT().
				Read(gomock.Any()).
				Return(&management.Branding{}, nil)
			brandingAPI.EXPECT().
				UniversalLogin(gomock.Any()).
				Return(test.brandingUL, nil).
				Do(func(opts ...management.RequestOption) {
					defer wg.Done()
				})

			clientAPI := mock.NewMockClientAPI(ctrl)
			clientAPI.EXPECT().
				List(gomock.All()).
				Return(&management.ClientList{Clients: test.clients}, test.clientAPIError).
				Do(func(opts ...management.RequestOption) {
					defer wg.Done()
				})

			customDomainAPI := mock.NewMockCustomDomainAPI(ctrl)
			customDomainAPI.EXPECT().
				List(gomock.Any()).
				Return([]*management.CustomDomain{test.customDomain}, nil).
				Do(func(opts ...management.RequestOption) {
					defer wg.Done()
				})

			promptAPI := mock.NewMockPromptAPI(ctrl)
			promptAPI.EXPECT().
				Read(gomock.Any()).
				Return(test.prompt, test.promptAPIError).
				Do(func(opts ...management.RequestOption) {
					defer wg.Done()
				})

			tenantAPI := mock.NewMockTenantAPI(ctrl)
			tenantAPI.EXPECT().
				Read(gomock.Any()).
				Return(test.tenant, test.tenantAPIError).
				Do(func(opts ...management.RequestOption) {
					defer wg.Done()
				})

			ctx := context.Background()
			api := &auth0.API{
				Client:       clientAPI,
				CustomDomain: customDomainAPI,
				Branding:     brandingAPI,
				Prompt:       promptAPI,
				Tenant:       tenantAPI,
			}
			cli := &cli{api: api}

			templateData, err := cli.fetchTemplateData(ctx)

			wg.Wait()

			if err != nil {
				test.assertError(t, err)
			} else {
				test.assertOutput(t, templateData)
			}
		})
	}
}
