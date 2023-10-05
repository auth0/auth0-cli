package cli

import (
	"context"
	"fmt"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
)

func TestFetchUniversalLoginBrandingData(t *testing.T) {
	const tenantDomain = "tenant-example.auth0.com"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var testCases = []struct {
		name          string
		mockedAPI     func() *auth0.API
		expectedData  *universalLoginBrandingData
		expectedError string
	}{
		{
			name: "it can correctly fetch universal login branding data",
			mockedAPI: func() *auth0.API {
				mockPromptAPI := mock.NewMockPromptAPI(ctrl)
				mockPromptAPI.
					EXPECT().
					Read(gomock.Any()).
					Return(
						&management.Prompt{
							UniversalLoginExperience:    "new",
							IdentifierFirst:             auth0.Bool(true),
							WebAuthnPlatformFirstFactor: auth0.Bool(true),
						},
						nil,
					)

				mockBrandingAPI := mock.NewMockBrandingAPI(ctrl)
				mockBrandingAPI.
					EXPECT().
					Read(gomock.Any()).
					Return(
						&management.Branding{
							Colors: &management.BrandingColors{
								Primary:        auth0.String("#334455"),
								PageBackground: auth0.String("#00AABB"),
							},
							LogoURL: auth0.String("https://some-log.example.com"),
						},
						nil,
					)

				mockBrandingAPI.
					EXPECT().
					UniversalLogin(gomock.Any()).
					Return(
						&management.BrandingUniversalLogin{
							Body: auth0.String("<html></html>"),
						},
						nil,
					)

				mockBrandingThemeAPI := mock.NewMockBrandingThemeAPI(ctrl)
				mockBrandingThemeAPI.
					EXPECT().
					Default(gomock.Any()).
					Return(&management.BrandingTheme{}, nil)

				mockTenantAPI := mock.NewMockTenantAPI(ctrl)
				mockTenantAPI.
					EXPECT().
					Read(gomock.Any()).
					Return(
						&management.Tenant{
							FriendlyName:   auth0.String("My Test Tenant"),
							EnabledLocales: &[]string{"en", "es"},
						},
						nil,
					)

				mockPromptAPI.
					EXPECT().
					CustomText(gomock.Any(), "login", "en").
					Return(
						map[string]interface{}{
							"login": map[string]interface{}{
								"title": "Welcome friend, glad to have you!",
							},
						},
						nil,
					)

				mockAPI := &auth0.API{
					Branding:      mockBrandingAPI,
					BrandingTheme: mockBrandingThemeAPI,
					Prompt:        mockPromptAPI,
					Tenant:        mockTenantAPI,
				}

				return mockAPI
			},
			expectedData: &universalLoginBrandingData{
				AuthenticationProfile: &management.Prompt{
					UniversalLoginExperience:    "new",
					IdentifierFirst:             auth0.Bool(true),
					WebAuthnPlatformFirstFactor: auth0.Bool(true),
				},
				Settings: &management.Branding{
					Colors: &management.BrandingColors{
						Primary:        auth0.String("#334455"),
						PageBackground: auth0.String("#00AABB"),
					},
					LogoURL: auth0.String("https://some-log.example.com"),
				},
				Template: &management.BrandingUniversalLogin{
					Body: auth0.String("<html></html>"),
				},
				Theme: &management.BrandingTheme{},
				Tenant: &tenantData{
					FriendlyName:   "My Test Tenant",
					EnabledLocales: []string{"en", "es"},
					Domain:         "tenant-example.auth0.com",
				},
				Prompt: &promptData{
					Language: "en",
					Prompt:   "login",
					CustomText: map[string]map[string]interface{}{
						"login": {
							"alertListTitle":                "Alerts",
							"auth0-users-validation":        "Something went wrong, please try again later",
							"authentication-failure":        "We are sorry, something went wrong when attempting to login",
							"buttonText":                    "Continue",
							"custom-script-error-code":      "Something went wrong, please try again later.",
							"description":                   "Log in to ${companyName} to continue to ${clientName}.",
							"editEmailText":                 "Edit",
							"emailPlaceholder":              "Email address",
							"federatedConnectionButtonText": "Continue with ${connectionName}",
							"footerLinkText":                "Sign up",
							"footerText":                    "Don't have an account?",
							"forgotPasswordText":            "Forgot password?",
							"hidePasswordText":              "Hide password",
							"invalid-connection":            "Invalid connection",
							"invalid-email-format":          "Email is not valid.",
							"invitationDescription":         "Log in to accept ${inviterName}'s invitation to join ${companyName} on ${clientName}.",
							"invitationTitle":               "You've Been Invited!",
							"ip-blocked":                    "We have detected suspicious login behavior and further attempts will be blocked. Please contact the administrator.",
							"logoAltText":                   "${companyName}",
							"no-db-connection":              "Invalid connection",
							"no-email":                      "Please enter an email address",
							"no-password":                   "Password is required",
							"no-username":                   "Username is required",
							"pageTitle":                     "Log in | ${clientName}",
							"password-breached":             "We have detected a potential security issue with this account. To protect your account, we have prevented this login. Please reset your password to proceed.",
							"passwordPlaceholder":           "Password",
							"same-user-login":               "Too many login attempts for this user. Please wait, and try again later.",
							"separatorText":                 "Or",
							"showPasswordText":              "Show password",
							"signupActionLinkText":          "${footerLinkText}",
							"signupActionText":              "${footerText}",
							"title":                         "Welcome friend, glad to have you!",
							"user-blocked":                  "Your account has been blocked after multiple consecutive login attempts.",
							"usernamePlaceholder":           "Username or email address",
							"wrong-credentials":             "Wrong username or password",
							"wrong-email-credentials":       "Wrong email or password",
						},
					},
				},
			},
		},
		{
			name: "it uses default branding settings if it fails to fetch them",
			mockedAPI: func() *auth0.API {
				mockPromptAPI := mock.NewMockPromptAPI(ctrl)
				mockPromptAPI.
					EXPECT().
					Read(gomock.Any()).
					Return(
						&management.Prompt{
							UniversalLoginExperience:    "new",
							IdentifierFirst:             auth0.Bool(true),
							WebAuthnPlatformFirstFactor: auth0.Bool(true),
						},
						nil,
					)

				mockBrandingAPI := mock.NewMockBrandingAPI(ctrl)
				mockBrandingAPI.
					EXPECT().
					Read(gomock.Any()).
					Return(nil, fmt.Errorf("failed to fetch branding settings"))

				mockBrandingAPI.
					EXPECT().
					UniversalLogin(gomock.Any()).
					Return(
						&management.BrandingUniversalLogin{
							Body: auth0.String("<html></html>"),
						},
						nil,
					)

				mockBrandingThemeAPI := mock.NewMockBrandingThemeAPI(ctrl)
				mockBrandingThemeAPI.
					EXPECT().
					Default(gomock.Any()).
					Return(&management.BrandingTheme{}, nil)

				mockTenantAPI := mock.NewMockTenantAPI(ctrl)
				mockTenantAPI.
					EXPECT().
					Read(gomock.Any()).
					Return(
						&management.Tenant{
							FriendlyName:   auth0.String("My Test Tenant"),
							EnabledLocales: &[]string{"en", "es"},
						},
						nil,
					)

				mockPromptAPI.
					EXPECT().
					CustomText(gomock.Any(), "login", "en").
					Return(
						map[string]interface{}{
							"login": map[string]interface{}{
								"title": "Welcome friend, glad to have you!",
							},
						},
						nil,
					)

				mockAPI := &auth0.API{
					Branding:      mockBrandingAPI,
					BrandingTheme: mockBrandingThemeAPI,
					Prompt:        mockPromptAPI,
					Tenant:        mockTenantAPI,
				}

				return mockAPI
			},
			expectedData: &universalLoginBrandingData{
				AuthenticationProfile: &management.Prompt{
					UniversalLoginExperience:    "new",
					IdentifierFirst:             auth0.Bool(true),
					WebAuthnPlatformFirstFactor: auth0.Bool(true),
				},
				Settings: &management.Branding{
					Colors: &management.BrandingColors{
						Primary:        auth0.String(defaultPrimaryColor),
						PageBackground: auth0.String(defaultBackgroundColor),
					},
					LogoURL: auth0.String(defaultLogoURL),
				},
				Template: &management.BrandingUniversalLogin{
					Body: auth0.String("<html></html>"),
				},
				Theme: &management.BrandingTheme{},
				Tenant: &tenantData{
					FriendlyName:   "My Test Tenant",
					EnabledLocales: []string{"en", "es"},
					Domain:         "tenant-example.auth0.com",
				},
				Prompt: &promptData{
					Language: "en",
					Prompt:   "login",
					CustomText: map[string]map[string]interface{}{
						"login": {
							"alertListTitle":                "Alerts",
							"auth0-users-validation":        "Something went wrong, please try again later",
							"authentication-failure":        "We are sorry, something went wrong when attempting to login",
							"buttonText":                    "Continue",
							"custom-script-error-code":      "Something went wrong, please try again later.",
							"description":                   "Log in to ${companyName} to continue to ${clientName}.",
							"editEmailText":                 "Edit",
							"emailPlaceholder":              "Email address",
							"federatedConnectionButtonText": "Continue with ${connectionName}",
							"footerLinkText":                "Sign up",
							"footerText":                    "Don't have an account?",
							"forgotPasswordText":            "Forgot password?",
							"hidePasswordText":              "Hide password",
							"invalid-connection":            "Invalid connection",
							"invalid-email-format":          "Email is not valid.",
							"invitationDescription":         "Log in to accept ${inviterName}'s invitation to join ${companyName} on ${clientName}.",
							"invitationTitle":               "You've Been Invited!",
							"ip-blocked":                    "We have detected suspicious login behavior and further attempts will be blocked. Please contact the administrator.",
							"logoAltText":                   "${companyName}",
							"no-db-connection":              "Invalid connection",
							"no-email":                      "Please enter an email address",
							"no-password":                   "Password is required",
							"no-username":                   "Username is required",
							"pageTitle":                     "Log in | ${clientName}",
							"password-breached":             "We have detected a potential security issue with this account. To protect your account, we have prevented this login. Please reset your password to proceed.",
							"passwordPlaceholder":           "Password",
							"same-user-login":               "Too many login attempts for this user. Please wait, and try again later.",
							"separatorText":                 "Or",
							"showPasswordText":              "Show password",
							"signupActionLinkText":          "${footerLinkText}",
							"signupActionText":              "${footerText}",
							"title":                         "Welcome friend, glad to have you!",
							"user-blocked":                  "Your account has been blocked after multiple consecutive login attempts.",
							"usernamePlaceholder":           "Username or email address",
							"wrong-credentials":             "Wrong username or password",
							"wrong-email-credentials":       "Wrong email or password",
						},
					},
				},
			},
		},
		{
			name: "it uses an empty branding template if it fails to fetch it",
			mockedAPI: func() *auth0.API {
				mockPromptAPI := mock.NewMockPromptAPI(ctrl)
				mockPromptAPI.
					EXPECT().
					Read(gomock.Any()).
					Return(
						&management.Prompt{
							UniversalLoginExperience:    "new",
							IdentifierFirst:             auth0.Bool(true),
							WebAuthnPlatformFirstFactor: auth0.Bool(true),
						},
						nil,
					)

				mockBrandingAPI := mock.NewMockBrandingAPI(ctrl)
				mockBrandingAPI.
					EXPECT().
					Read(gomock.Any()).
					Return(
						&management.Branding{
							Colors: &management.BrandingColors{
								Primary:        auth0.String("#334455"),
								PageBackground: auth0.String("#00AABB"),
							},
							LogoURL: auth0.String("https://some-log.example.com"),
						},
						nil,
					)

				mockBrandingAPI.
					EXPECT().
					UniversalLogin(gomock.Any()).
					Return(nil, fmt.Errorf("failed to fetch universal login template"))

				mockBrandingThemeAPI := mock.NewMockBrandingThemeAPI(ctrl)
				mockBrandingThemeAPI.
					EXPECT().
					Default(gomock.Any()).
					Return(&management.BrandingTheme{}, nil)

				mockTenantAPI := mock.NewMockTenantAPI(ctrl)
				mockTenantAPI.
					EXPECT().
					Read(gomock.Any()).
					Return(
						&management.Tenant{
							FriendlyName:   auth0.String("My Test Tenant"),
							EnabledLocales: &[]string{"en", "es"},
						},
						nil,
					)

				mockPromptAPI.
					EXPECT().
					CustomText(gomock.Any(), "login", "en").
					Return(
						map[string]interface{}{
							"login": map[string]interface{}{
								"title": "Welcome friend, glad to have you!",
							},
						},
						nil,
					)

				mockAPI := &auth0.API{
					Branding:      mockBrandingAPI,
					BrandingTheme: mockBrandingThemeAPI,
					Prompt:        mockPromptAPI,
					Tenant:        mockTenantAPI,
				}

				return mockAPI
			},
			expectedData: &universalLoginBrandingData{
				AuthenticationProfile: &management.Prompt{
					UniversalLoginExperience:    "new",
					IdentifierFirst:             auth0.Bool(true),
					WebAuthnPlatformFirstFactor: auth0.Bool(true),
				},
				Settings: &management.Branding{
					Colors: &management.BrandingColors{
						Primary:        auth0.String("#334455"),
						PageBackground: auth0.String("#00AABB"),
					},
					LogoURL: auth0.String("https://some-log.example.com"),
				},
				Template: &management.BrandingUniversalLogin{},
				Theme:    &management.BrandingTheme{},
				Tenant: &tenantData{
					FriendlyName:   "My Test Tenant",
					EnabledLocales: []string{"en", "es"},
					Domain:         "tenant-example.auth0.com",
				},
				Prompt: &promptData{
					Language: "en",
					Prompt:   "login",
					CustomText: map[string]map[string]interface{}{
						"login": {
							"alertListTitle":                "Alerts",
							"auth0-users-validation":        "Something went wrong, please try again later",
							"authentication-failure":        "We are sorry, something went wrong when attempting to login",
							"buttonText":                    "Continue",
							"custom-script-error-code":      "Something went wrong, please try again later.",
							"description":                   "Log in to ${companyName} to continue to ${clientName}.",
							"editEmailText":                 "Edit",
							"emailPlaceholder":              "Email address",
							"federatedConnectionButtonText": "Continue with ${connectionName}",
							"footerLinkText":                "Sign up",
							"footerText":                    "Don't have an account?",
							"forgotPasswordText":            "Forgot password?",
							"hidePasswordText":              "Hide password",
							"invalid-connection":            "Invalid connection",
							"invalid-email-format":          "Email is not valid.",
							"invitationDescription":         "Log in to accept ${inviterName}'s invitation to join ${companyName} on ${clientName}.",
							"invitationTitle":               "You've Been Invited!",
							"ip-blocked":                    "We have detected suspicious login behavior and further attempts will be blocked. Please contact the administrator.",
							"logoAltText":                   "${companyName}",
							"no-db-connection":              "Invalid connection",
							"no-email":                      "Please enter an email address",
							"no-password":                   "Password is required",
							"no-username":                   "Username is required",
							"pageTitle":                     "Log in | ${clientName}",
							"password-breached":             "We have detected a potential security issue with this account. To protect your account, we have prevented this login. Please reset your password to proceed.",
							"passwordPlaceholder":           "Password",
							"same-user-login":               "Too many login attempts for this user. Please wait, and try again later.",
							"separatorText":                 "Or",
							"showPasswordText":              "Show password",
							"signupActionLinkText":          "${footerLinkText}",
							"signupActionText":              "${footerText}",
							"title":                         "Welcome friend, glad to have you!",
							"user-blocked":                  "Your account has been blocked after multiple consecutive login attempts.",
							"usernamePlaceholder":           "Username or email address",
							"wrong-credentials":             "Wrong username or password",
							"wrong-email-credentials":       "Wrong email or password",
						},
					},
				},
			},
		},
		{
			name: "it uses a default branding theme if it fails to fetch it",
			mockedAPI: func() *auth0.API {
				mockPromptAPI := mock.NewMockPromptAPI(ctrl)
				mockPromptAPI.
					EXPECT().
					Read(gomock.Any()).
					Return(
						&management.Prompt{
							UniversalLoginExperience:    "new",
							IdentifierFirst:             auth0.Bool(true),
							WebAuthnPlatformFirstFactor: auth0.Bool(true),
						},
						nil,
					)

				mockBrandingAPI := mock.NewMockBrandingAPI(ctrl)
				mockBrandingAPI.
					EXPECT().
					Read(gomock.Any()).
					Return(
						&management.Branding{
							Colors: &management.BrandingColors{
								Primary:        auth0.String("#334455"),
								PageBackground: auth0.String("#00AABB"),
							},
							LogoURL: auth0.String("https://some-log.example.com"),
						},
						nil,
					)

				mockBrandingAPI.
					EXPECT().
					UniversalLogin(gomock.Any()).
					Return(
						&management.BrandingUniversalLogin{
							Body: auth0.String("<html></html>"),
						},
						nil,
					)

				mockBrandingThemeAPI := mock.NewMockBrandingThemeAPI(ctrl)
				mockBrandingThemeAPI.
					EXPECT().
					Default(gomock.Any()).
					Return(nil, fmt.Errorf("failed to fetch branding theme"))

				mockTenantAPI := mock.NewMockTenantAPI(ctrl)
				mockTenantAPI.
					EXPECT().
					Read(gomock.Any()).
					Return(
						&management.Tenant{
							FriendlyName:   auth0.String("My Test Tenant"),
							EnabledLocales: &[]string{"en", "es"},
						},
						nil,
					)

				mockPromptAPI.
					EXPECT().
					CustomText(gomock.Any(), "login", "en").
					Return(
						map[string]interface{}{
							"login": map[string]interface{}{
								"title": "Welcome friend, glad to have you!",
							},
						},
						nil,
					)

				mockAPI := &auth0.API{
					Branding:      mockBrandingAPI,
					BrandingTheme: mockBrandingThemeAPI,
					Prompt:        mockPromptAPI,
					Tenant:        mockTenantAPI,
				}

				return mockAPI
			},
			expectedData: &universalLoginBrandingData{
				AuthenticationProfile: &management.Prompt{
					UniversalLoginExperience:    "new",
					IdentifierFirst:             auth0.Bool(true),
					WebAuthnPlatformFirstFactor: auth0.Bool(true),
				},
				Settings: &management.Branding{
					Colors: &management.BrandingColors{
						Primary:        auth0.String("#334455"),
						PageBackground: auth0.String("#00AABB"),
					},
					LogoURL: auth0.String("https://some-log.example.com"),
				},
				Template: &management.BrandingUniversalLogin{
					Body: auth0.String("<html></html>"),
				},
				Theme: &management.BrandingTheme{
					Borders: management.BrandingThemeBorders{
						ButtonBorderRadius: 3,
						ButtonBorderWeight: 1,
						ButtonsStyle:       "rounded",
						InputBorderRadius:  3,
						InputBorderWeight:  1,
						InputsStyle:        "rounded",
						ShowWidgetShadow:   true,
						WidgetBorderWeight: 0,
						WidgetCornerRadius: 5,
					},
					Colors: management.BrandingThemeColors{
						BaseFocusColor:          auth0.String("#635dff"),
						BaseHoverColor:          auth0.String("#000000"),
						BodyText:                "#1e212a",
						Error:                   "#d03c38",
						Header:                  "#1e212a",
						Icons:                   "#65676e",
						InputBackground:         "#ffffff",
						InputBorder:             "#c9cace",
						InputFilledText:         "#000000",
						InputLabelsPlaceholders: "#65676e",
						LinksFocusedComponents:  "#635dff",
						PrimaryButton:           "#635dff",
						PrimaryButtonLabel:      "#ffffff",
						SecondaryButtonBorder:   "#c9cace",
						SecondaryButtonLabel:    "#1e212a",
						Success:                 "#13a688",
						WidgetBackground:        "#ffffff",
						WidgetBorder:            "#c9cace",
					},
					Fonts: management.BrandingThemeFonts{
						BodyText: management.BrandingThemeText{
							Bold: false,
							Size: 87.5,
						},
						ButtonsText: management.BrandingThemeText{
							Bold: false,
							Size: 100.0,
						},
						FontURL: "",
						InputLabels: management.BrandingThemeText{
							Bold: false,
							Size: 100.0,
						},
						Links: management.BrandingThemeText{
							Bold: true,
							Size: 87.5,
						},
						LinksStyle:        "normal",
						ReferenceTextSize: 16.0,
						Subtitle: management.BrandingThemeText{
							Bold: false,
							Size: 87.5,
						},
						Title: management.BrandingThemeText{
							Bold: false,
							Size: 150.0,
						},
					},
					PageBackground: management.BrandingThemePageBackground{
						BackgroundColor:    "#000000",
						BackgroundImageURL: "",
						PageLayout:         "center",
					},
					Widget: management.BrandingThemeWidget{
						HeaderTextAlignment: "center",
						LogoHeight:          52.0,
						LogoPosition:        "center",
						LogoURL:             "",
						SocialButtonsLayout: "bottom",
					},
				},
				Tenant: &tenantData{
					FriendlyName:   "My Test Tenant",
					EnabledLocales: []string{"en", "es"},
					Domain:         "tenant-example.auth0.com",
				},
				Prompt: &promptData{
					Language: "en",
					Prompt:   "login",
					CustomText: map[string]map[string]interface{}{
						"login": {
							"alertListTitle":                "Alerts",
							"auth0-users-validation":        "Something went wrong, please try again later",
							"authentication-failure":        "We are sorry, something went wrong when attempting to login",
							"buttonText":                    "Continue",
							"custom-script-error-code":      "Something went wrong, please try again later.",
							"description":                   "Log in to ${companyName} to continue to ${clientName}.",
							"editEmailText":                 "Edit",
							"emailPlaceholder":              "Email address",
							"federatedConnectionButtonText": "Continue with ${connectionName}",
							"footerLinkText":                "Sign up",
							"footerText":                    "Don't have an account?",
							"forgotPasswordText":            "Forgot password?",
							"hidePasswordText":              "Hide password",
							"invalid-connection":            "Invalid connection",
							"invalid-email-format":          "Email is not valid.",
							"invitationDescription":         "Log in to accept ${inviterName}'s invitation to join ${companyName} on ${clientName}.",
							"invitationTitle":               "You've Been Invited!",
							"ip-blocked":                    "We have detected suspicious login behavior and further attempts will be blocked. Please contact the administrator.",
							"logoAltText":                   "${companyName}",
							"no-db-connection":              "Invalid connection",
							"no-email":                      "Please enter an email address",
							"no-password":                   "Password is required",
							"no-username":                   "Username is required",
							"pageTitle":                     "Log in | ${clientName}",
							"password-breached":             "We have detected a potential security issue with this account. To protect your account, we have prevented this login. Please reset your password to proceed.",
							"passwordPlaceholder":           "Password",
							"same-user-login":               "Too many login attempts for this user. Please wait, and try again later.",
							"separatorText":                 "Or",
							"showPasswordText":              "Show password",
							"signupActionLinkText":          "${footerLinkText}",
							"signupActionText":              "${footerText}",
							"title":                         "Welcome friend, glad to have you!",
							"user-blocked":                  "Your account has been blocked after multiple consecutive login attempts.",
							"usernamePlaceholder":           "Username or email address",
							"wrong-credentials":             "Wrong username or password",
							"wrong-email-credentials":       "Wrong email or password",
						},
					},
				},
			},
		},
		{
			name: "it fails to fetch branding data if there's an error retrieving tenant data",
			mockedAPI: func() *auth0.API {
				mockPromptAPI := mock.NewMockPromptAPI(ctrl)
				mockPromptAPI.
					EXPECT().
					Read(gomock.Any()).
					Return(
						&management.Prompt{
							UniversalLoginExperience:    "new",
							IdentifierFirst:             auth0.Bool(true),
							WebAuthnPlatformFirstFactor: auth0.Bool(true),
						},
						nil,
					)

				mockBrandingAPI := mock.NewMockBrandingAPI(ctrl)
				mockBrandingAPI.
					EXPECT().
					Read(gomock.Any()).
					Return(
						&management.Branding{
							Colors: &management.BrandingColors{
								Primary:        auth0.String("#334455"),
								PageBackground: auth0.String("#00AABB"),
							},
							LogoURL: auth0.String("https://some-log.example.com"),
						},
						nil,
					)

				mockBrandingAPI.
					EXPECT().
					UniversalLogin(gomock.Any()).
					Return(
						&management.BrandingUniversalLogin{
							Body: auth0.String("<html></html>"),
						},
						nil,
					)

				mockBrandingThemeAPI := mock.NewMockBrandingThemeAPI(ctrl)
				mockBrandingThemeAPI.
					EXPECT().
					Default(gomock.Any()).
					Return(&management.BrandingTheme{}, nil)

				mockTenantAPI := mock.NewMockTenantAPI(ctrl)
				mockTenantAPI.
					EXPECT().
					Read(gomock.Any()).
					Return(nil, fmt.Errorf("failed to fetch tenant data"))

				mockAPI := &auth0.API{
					Branding:      mockBrandingAPI,
					BrandingTheme: mockBrandingThemeAPI,
					Prompt:        mockPromptAPI,
					Tenant:        mockTenantAPI,
				}

				return mockAPI
			},
			expectedError: "failed to fetch tenant data",
		},
		{
			name: "it fails to fetch branding data if there's an error retrieving prompt text data",
			mockedAPI: func() *auth0.API {
				mockPromptAPI := mock.NewMockPromptAPI(ctrl)
				mockPromptAPI.
					EXPECT().
					Read(gomock.Any()).
					Return(
						&management.Prompt{
							UniversalLoginExperience:    "new",
							IdentifierFirst:             auth0.Bool(true),
							WebAuthnPlatformFirstFactor: auth0.Bool(true),
						},
						nil,
					)

				mockBrandingAPI := mock.NewMockBrandingAPI(ctrl)
				mockBrandingAPI.
					EXPECT().
					Read(gomock.Any()).
					Return(
						&management.Branding{
							Colors: &management.BrandingColors{
								Primary:        auth0.String("#334455"),
								PageBackground: auth0.String("#00AABB"),
							},
							LogoURL: auth0.String("https://some-log.example.com"),
						},
						nil,
					)

				mockBrandingAPI.
					EXPECT().
					UniversalLogin(gomock.Any()).
					Return(
						&management.BrandingUniversalLogin{
							Body: auth0.String("<html></html>"),
						},
						nil,
					)

				mockBrandingThemeAPI := mock.NewMockBrandingThemeAPI(ctrl)
				mockBrandingThemeAPI.
					EXPECT().
					Default(gomock.Any()).
					Return(&management.BrandingTheme{}, nil)

				mockTenantAPI := mock.NewMockTenantAPI(ctrl)
				mockTenantAPI.
					EXPECT().
					Read(gomock.Any()).
					Return(
						&management.Tenant{
							FriendlyName:   auth0.String("My Test Tenant"),
							EnabledLocales: &[]string{"en", "es"},
						},
						nil,
					)

				mockPromptAPI.
					EXPECT().
					CustomText(gomock.Any(), "login", "en").
					Return(nil, fmt.Errorf("failed to fetch custom text"))

				mockAPI := &auth0.API{
					Branding:      mockBrandingAPI,
					BrandingTheme: mockBrandingThemeAPI,
					Prompt:        mockPromptAPI,
					Tenant:        mockTenantAPI,
				}

				return mockAPI
			},
			expectedError: "failed to fetch custom text",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			actualData, err := fetchUniversalLoginBrandingData(context.Background(), test.mockedAPI(), tenantDomain)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.expectedData, actualData)
		})
	}
}
