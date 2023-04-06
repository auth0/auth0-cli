package cli

import (
	"context"
	"net/http"
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
		name          string
		customDomains []*management.CustomDomain
		apiError      management.Error
		assertOutput  func(t testing.TB, err error)
	}{
		{
			name: "happy path",
			customDomains: []*management.CustomDomain{
				{
					Status:   auth0.String("foo"),
				},
				{
					Status:   auth0.String("ready"),
				},
			},
			assertOutput: func(t testing.TB, err error) {
				assert.Nil(t, err)
			},
		},
		{
			name: "no verified domains",
			customDomains: []*management.CustomDomain{
				{
					Status:   auth0.String("foo"),
				},
			},
			assertOutput: func(t testing.TB, err error) {
				assert.ErrorIs(t, errNotAllowed, err)
			},
		},
		{
			name: "custom domains are not enabled",
			apiError: mockManagamentError{status: http.StatusForbidden},
			assertOutput: func(t testing.TB, err error) {
				assert.ErrorIs(t, errNotAllowed, err)
			},
		},
		{
			name: "api error",
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
				Return(test.customDomains, test.apiError)

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
					Primary: auth0.String("#FF4F40"),
					PageBackground: auth0.String("#2A2E35"),
				},
				LogoURL: auth0.String("https://example.com/logo-updated-json.png"),
			},
			assertOutput: func(t testing.TB, branding *management.Branding) {
				assert.NotNil(t, branding)
				assert.NotNil(t, branding.Colors)
				assert.Equal(t, branding.Colors.GetPrimary(), "#FF4F40")
				assert.Equal(t, branding.Colors.GetPageBackground(), "#2A2E35")
				assert.Equal(t, branding.GetLogoURL(), "https://example.com/logo-updated-json.png")
			},
		},
		{
			name: "no branding settings",
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
			name: "api error",
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
			name: "api error",
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
