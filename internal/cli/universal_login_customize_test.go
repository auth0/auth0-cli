package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
)

func TestEnsureNewUniversalLoginExperienceIsActive(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var testCases = []struct {
		name          string
		mockedAPI     func() *auth0.API
		expectedError string
	}{
		{
			name: "it returns nil if new ul is active",
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
				return &auth0.API{
					Prompt: mockPromptAPI,
				}
			},
		},
		{
			name: "it returns an error if there is an api error",
			mockedAPI: func() *auth0.API {
				mockPromptAPI := mock.NewMockPromptAPI(ctrl)
				mockPromptAPI.
					EXPECT().
					Read(gomock.Any()).
					Return(nil, fmt.Errorf("api error"))
				return &auth0.API{
					Prompt: mockPromptAPI,
				}
			},
			expectedError: "api error",
		},
		{
			name: "it returns an error if classic UL is enabled",
			mockedAPI: func() *auth0.API {
				mockPromptAPI := mock.NewMockPromptAPI(ctrl)
				mockPromptAPI.
					EXPECT().
					Read(gomock.Any()).
					Return(
						&management.Prompt{
							UniversalLoginExperience:    "classic",
							IdentifierFirst:             auth0.Bool(true),
							WebAuthnPlatformFirstFactor: auth0.Bool(true),
						},
						nil,
					)

				return &auth0.API{
					Prompt: mockPromptAPI,
				}
			},
			expectedError: "this feature requires the new Universal Login experience to be enabled for the tenant, use `auth0 api patch prompts --data '{\"universal_login_experience\":\"new\"}'` to enable it",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			err := ensureNewUniversalLoginExperienceIsActive(context.Background(), test.mockedAPI())

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestGetPartials(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var testCases = []struct {
		name           string
		mockedAPI      func() *auth0.API
		promptData     *partialData
		expectedResult *management.PromptScreenPartials
		expectedError  string
	}{
		{
			name: "it returns partials successfully",
			mockedAPI: func() *auth0.API {
				mockPromptAPI := mock.NewMockPromptAPI(ctrl)
				mockPromptAPI.
					EXPECT().
					GetPartials(gomock.Any(), management.PromptType("login")).
					Return(&management.PromptScreenPartials{
						management.ScreenLogin: {
							management.InsertionPointFormContentStart: "start",
							management.InsertionPointFormContentEnd:   "end",
						},
					}, nil)
				return &auth0.API{
					Prompt: mockPromptAPI,
				}
			},
			promptData: &partialData{
				PromptName: "login",
				ScreenName: "login",
			},
			expectedResult: &management.PromptScreenPartials{
				management.ScreenName("login"): {
					management.InsertionPointFormContentStart: "start",
					management.InsertionPointFormContentEnd:   "end",
				},
			},
			expectedError: "",
		},
		{
			name: "it returns an error if there is an api error",
			mockedAPI: func() *auth0.API {
				mockPromptAPI := mock.NewMockPromptAPI(ctrl)
				mockPromptAPI.
					EXPECT().
					GetPartials(gomock.Any(), management.PromptType("login")).
					Return(nil, fmt.Errorf("api error"))
				return &auth0.API{
					Prompt: mockPromptAPI,
				}
			},
			promptData: &partialData{
				PromptName: "login",
				ScreenName: "login",
			},
			expectedResult: nil,
			expectedError:  "api error",
		},
		{
			name: "it returns an error if featureflag is not enabled",
			mockedAPI: func() *auth0.API {
				mockPromptAPI := mock.NewMockPromptAPI(ctrl)
				mockPromptAPI.
					EXPECT().
					GetPartials(gomock.Any(), management.PromptType("login")).
					Return(nil, fmt.Errorf("failed to read partials: 403 forbidden: this feature is not available for your plan. To create or modify prompt templates, please upgrade your account to a professional or enterprise plan"))
				return &auth0.API{
					Prompt: mockPromptAPI,
				}
			},
			promptData: &partialData{
				PromptName: "login",
				ScreenName: "login",
			},
			expectedResult: nil,
			expectedError:  "this feature is not available for your plan. To create or modify prompt templates, please upgrade your account to a professional or enterprise plan",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			partials, err := fetchPartial(context.Background(), test.mockedAPI(), test.promptData)

			if test.expectedError != "" {
				if test.name == "it returns an error if featureflag is not enabled" {
					assert.ErrorContains(t, err, test.expectedError)
				} else {
					assert.EqualError(t, err, test.expectedError)
				}
				assert.Nil(t, partials)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.expectedResult, partials)
		})
	}
}

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

				mockPromptAPI := mock.NewMockPromptAPI(ctrl)
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
				for _, promptType := range allowedPromptsWithPartials {
					mockPromptAPI.EXPECT().
						GetPartials(gomock.Any(), promptType).
						Return(&management.PromptScreenPartials{
							management.ScreenName(promptType): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						}, nil)
				}

				mockClientAPI := mock.NewMockClientAPI(ctrl)
				mockClientAPI.
					EXPECT().
					List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&management.ClientList{
						Clients: []*management.Client{
							{
								ClientID:       auth0.String("1"),
								Name:           auth0.String("My App"),
								LogoURI:        auth0.String("https://my-app.example.com/image.png"),
								ClientMetadata: &map[string]interface{}{"meta": "meta"},
							},
						},
					}, nil)

				mockAPI := &auth0.API{
					Client:        mockClientAPI,
					Branding:      mockBrandingAPI,
					BrandingTheme: mockBrandingThemeAPI,
					Prompt:        mockPromptAPI,
					Tenant:        mockTenantAPI,
				}

				return mockAPI
			},
			expectedData: &universalLoginBrandingData{
				Applications: []*applicationData{
					{
						ID:       "1",
						Name:     "My App",
						LogoURL:  "https://my-app.example.com/image.png",
						Metadata: map[string]interface{}{"meta": "meta"},
					},
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
				Partials: []partialsData{
					{
						"signup": {
							management.ScreenName("signup"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
					{
						"signup-id": {
							management.ScreenName("signup-id"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
					{
						"signup-password": {
							management.ScreenName("signup-password"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
					{
						"login": {
							management.ScreenName("login"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
					{
						"login-id": {
							management.ScreenName("login-id"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
					{
						"login-password": {
							management.ScreenName("login-password"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
					{
						"login-passwordless": {
							management.ScreenName("login-passwordless"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
				},
				Prompts: []*promptData{
					{
						Language: "en",
						Prompt:   "login",
						CustomText: map[string]interface{}{
							"login": map[string]interface{}{
								"alertListTitle":                         "Alerts",
								"auth0-users-validation":                 "Something went wrong, please try again later",
								"authentication-failure":                 "We are sorry, something went wrong when attempting to log in",
								"buttonText":                             "Continue",
								"custom-script-error-code":               "Something went wrong, please try again later.",
								"description":                            "Log in to ${companyName} to continue to ${clientName}.",
								"editEmailText":                          "Edit",
								"emailPlaceholder":                       "Email address",
								"federatedConnectionButtonText":          "Continue with ${connectionName}",
								"footerLinkText":                         "Sign up",
								"footerText":                             "Don't have an account?",
								"forgotPasswordText":                     "Forgot password?",
								"hidePasswordText":                       "Hide password",
								"invalid-connection":                     "Invalid connection",
								"invitationDescription":                  "Log in to accept ${inviterName}'s invitation to join ${companyName} on ${clientName}.",
								"invitationTitle":                        "You've Been Invited!",
								"ip-blocked":                             "We have detected suspicious login behavior and further attempts will be blocked. Please contact the administrator.",
								"logoAltText":                            "${companyName}",
								"no-db-connection":                       "Invalid connection",
								"no-email":                               "Please enter an email address",
								"no-password":                            "Password is required",
								"no-username":                            "Username is required",
								"pageTitle":                              "Log in | ${clientName}",
								"password-breached":                      "We have detected a potential security issue with this account. To protect your account, we have prevented this login. Please reset your password to proceed.",
								"passwordPlaceholder":                    "Password",
								"phoneOrEmailPlaceholder":                "Phone number or Email address",
								"phoneOrUsernameOrEmailPlaceholder":      "Phone or Username or Email",
								"phoneOrUsernamePlaceholder":             "Phone Number or Username",
								"phonePlaceholder":                       "Phone number",
								"same-user-login":                        "Too many login attempts for this user. Please wait, and try again later.",
								"separatorText":                          "Or",
								"showPasswordText":                       "Show password",
								"signupActionLinkText":                   "${footerLinkText}",
								"signupActionText":                       "${footerText}",
								"title":                                  "Welcome friend, glad to have you!",
								"user-blocked":                           "Your account has been blocked after multiple consecutive login attempts.",
								"usernamePlaceholder":                    "Username or email address",
								"usernameOnlyPlaceholder":                "Username",
								"usernameOrEmailPlaceholder":             "Username or Email address",
								"wrong-credentials":                      "Wrong username or password",
								"wrong-email-credentials":                "Wrong email or password",
								"wrong-email-phone-credentials":          "Incorrect email address, phone number, or password. Phone numbers must include the country code.",
								"wrong-email-phone-username-credentials": " Incorrect email address, phone number, username, or password. Phone numbers must include the country code.",
								"wrong-email-username-credentials":       "Incorrect email address, username, or password",
								"wrong-phone-credentials":                "Incorrect phone number or password",
								"wrong-phone-username-credentials":       "Incorrect phone number, username or password. Phone numbers must include the country code.",
								"wrong-username-credentials":             "Incorrect username or password",
							},
						},
					},
				},
			},
		},
		{
			name: "it uses default branding settings if it fails to fetch them",
			mockedAPI: func() *auth0.API {
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

				mockPromptAPI := mock.NewMockPromptAPI(ctrl)
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
				for _, promptType := range allowedPromptsWithPartials {
					mockPromptAPI.EXPECT().
						GetPartials(gomock.Any(), promptType).
						Return(&management.PromptScreenPartials{
							management.ScreenName(promptType): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						}, nil)
				}
				mockClientAPI := mock.NewMockClientAPI(ctrl)
				mockClientAPI.
					EXPECT().
					List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&management.ClientList{
						Clients: []*management.Client{
							{
								ClientID:       auth0.String("1"),
								Name:           auth0.String("My App"),
								LogoURI:        auth0.String("https://my-app.example.com/image.png"),
								ClientMetadata: &map[string]interface{}{"meta": "meta"},
							},
						},
					}, nil)

				mockAPI := &auth0.API{
					Client:        mockClientAPI,
					Branding:      mockBrandingAPI,
					BrandingTheme: mockBrandingThemeAPI,
					Prompt:        mockPromptAPI,
					Tenant:        mockTenantAPI,
				}

				return mockAPI
			},
			expectedData: &universalLoginBrandingData{
				Applications: []*applicationData{
					{
						ID:       "1",
						Name:     "My App",
						LogoURL:  "https://my-app.example.com/image.png",
						Metadata: map[string]interface{}{"meta": "meta"},
					},
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
				Partials: []partialsData{
					{
						"signup": {
							management.ScreenName("signup"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
					{
						"signup-id": {
							management.ScreenName("signup-id"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
					{
						"signup-password": {
							management.ScreenName("signup-password"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
					{
						"login": {
							management.ScreenName("login"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
					{
						"login-id": {
							management.ScreenName("login-id"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
					{
						"login-password": {
							management.ScreenName("login-password"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
					{
						"login-passwordless": {
							management.ScreenName("login-passwordless"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
				},
				Prompts: []*promptData{
					{
						Language: "en",
						Prompt:   "login",
						CustomText: map[string]interface{}{
							"login": map[string]interface{}{
								"alertListTitle":                         "Alerts",
								"auth0-users-validation":                 "Something went wrong, please try again later",
								"authentication-failure":                 "We are sorry, something went wrong when attempting to log in",
								"buttonText":                             "Continue",
								"custom-script-error-code":               "Something went wrong, please try again later.",
								"description":                            "Log in to ${companyName} to continue to ${clientName}.",
								"editEmailText":                          "Edit",
								"emailPlaceholder":                       "Email address",
								"federatedConnectionButtonText":          "Continue with ${connectionName}",
								"footerLinkText":                         "Sign up",
								"footerText":                             "Don't have an account?",
								"forgotPasswordText":                     "Forgot password?",
								"hidePasswordText":                       "Hide password",
								"invalid-connection":                     "Invalid connection",
								"invitationDescription":                  "Log in to accept ${inviterName}'s invitation to join ${companyName} on ${clientName}.",
								"invitationTitle":                        "You've Been Invited!",
								"ip-blocked":                             "We have detected suspicious login behavior and further attempts will be blocked. Please contact the administrator.",
								"logoAltText":                            "${companyName}",
								"no-db-connection":                       "Invalid connection",
								"no-email":                               "Please enter an email address",
								"no-password":                            "Password is required",
								"no-username":                            "Username is required",
								"pageTitle":                              "Log in | ${clientName}",
								"password-breached":                      "We have detected a potential security issue with this account. To protect your account, we have prevented this login. Please reset your password to proceed.",
								"passwordPlaceholder":                    "Password",
								"phoneOrEmailPlaceholder":                "Phone number or Email address",
								"phoneOrUsernameOrEmailPlaceholder":      "Phone or Username or Email",
								"phoneOrUsernamePlaceholder":             "Phone Number or Username",
								"phonePlaceholder":                       "Phone number",
								"same-user-login":                        "Too many login attempts for this user. Please wait, and try again later.",
								"separatorText":                          "Or",
								"showPasswordText":                       "Show password",
								"signupActionLinkText":                   "${footerLinkText}",
								"signupActionText":                       "${footerText}",
								"title":                                  "Welcome friend, glad to have you!",
								"user-blocked":                           "Your account has been blocked after multiple consecutive login attempts.",
								"usernamePlaceholder":                    "Username or email address",
								"usernameOnlyPlaceholder":                "Username",
								"usernameOrEmailPlaceholder":             "Username or Email address",
								"wrong-credentials":                      "Wrong username or password",
								"wrong-email-credentials":                "Wrong email or password",
								"wrong-email-phone-credentials":          "Incorrect email address, phone number, or password. Phone numbers must include the country code.",
								"wrong-email-phone-username-credentials": " Incorrect email address, phone number, username, or password. Phone numbers must include the country code.",
								"wrong-email-username-credentials":       "Incorrect email address, username, or password",
								"wrong-phone-credentials":                "Incorrect phone number or password",
								"wrong-phone-username-credentials":       "Incorrect phone number, username or password. Phone numbers must include the country code.",
								"wrong-username-credentials":             "Incorrect username or password",
							},
						},
					},
				},
			},
		},
		{
			name: "it uses an empty branding template if it fails to fetch it",
			mockedAPI: func() *auth0.API {
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

				mockPromptAPI := mock.NewMockPromptAPI(ctrl)
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
				for _, promptType := range allowedPromptsWithPartials {
					mockPromptAPI.EXPECT().
						GetPartials(gomock.Any(), promptType).
						Return(&management.PromptScreenPartials{
							management.ScreenName(promptType): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						}, nil)
				}
				mockClientAPI := mock.NewMockClientAPI(ctrl)
				mockClientAPI.
					EXPECT().
					List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&management.ClientList{
						Clients: []*management.Client{
							{
								ClientID:       auth0.String("1"),
								Name:           auth0.String("My App"),
								LogoURI:        auth0.String("https://my-app.example.com/image.png"),
								ClientMetadata: &map[string]interface{}{"meta": "meta"},
							},
						},
					}, nil)

				mockAPI := &auth0.API{
					Client:        mockClientAPI,
					Branding:      mockBrandingAPI,
					BrandingTheme: mockBrandingThemeAPI,
					Prompt:        mockPromptAPI,
					Tenant:        mockTenantAPI,
				}

				return mockAPI
			},
			expectedData: &universalLoginBrandingData{
				Applications: []*applicationData{
					{
						ID:       "1",
						Name:     "My App",
						LogoURL:  "https://my-app.example.com/image.png",
						Metadata: map[string]interface{}{"meta": "meta"},
					},
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
				Partials: []partialsData{
					{
						"signup": {
							management.ScreenName("signup"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
					{
						"signup-id": {
							management.ScreenName("signup-id"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
					{
						"signup-password": {
							management.ScreenName("signup-password"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
					{
						"login": {
							management.ScreenName("login"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
					{
						"login-id": {
							management.ScreenName("login-id"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
					{
						"login-password": {
							management.ScreenName("login-password"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
					{
						"login-passwordless": {
							management.ScreenName("login-passwordless"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
				},
				Prompts: []*promptData{
					{
						Language: "en",
						Prompt:   "login",
						CustomText: map[string]interface{}{
							"login": map[string]interface{}{
								"alertListTitle":                         "Alerts",
								"auth0-users-validation":                 "Something went wrong, please try again later",
								"authentication-failure":                 "We are sorry, something went wrong when attempting to log in",
								"buttonText":                             "Continue",
								"custom-script-error-code":               "Something went wrong, please try again later.",
								"description":                            "Log in to ${companyName} to continue to ${clientName}.",
								"editEmailText":                          "Edit",
								"emailPlaceholder":                       "Email address",
								"federatedConnectionButtonText":          "Continue with ${connectionName}",
								"footerLinkText":                         "Sign up",
								"footerText":                             "Don't have an account?",
								"forgotPasswordText":                     "Forgot password?",
								"hidePasswordText":                       "Hide password",
								"invalid-connection":                     "Invalid connection",
								"invitationDescription":                  "Log in to accept ${inviterName}'s invitation to join ${companyName} on ${clientName}.",
								"invitationTitle":                        "You've Been Invited!",
								"ip-blocked":                             "We have detected suspicious login behavior and further attempts will be blocked. Please contact the administrator.",
								"logoAltText":                            "${companyName}",
								"no-db-connection":                       "Invalid connection",
								"no-email":                               "Please enter an email address",
								"no-password":                            "Password is required",
								"no-username":                            "Username is required",
								"pageTitle":                              "Log in | ${clientName}",
								"password-breached":                      "We have detected a potential security issue with this account. To protect your account, we have prevented this login. Please reset your password to proceed.",
								"passwordPlaceholder":                    "Password",
								"phoneOrEmailPlaceholder":                "Phone number or Email address",
								"phoneOrUsernameOrEmailPlaceholder":      "Phone or Username or Email",
								"phoneOrUsernamePlaceholder":             "Phone Number or Username",
								"phonePlaceholder":                       "Phone number",
								"same-user-login":                        "Too many login attempts for this user. Please wait, and try again later.",
								"separatorText":                          "Or",
								"showPasswordText":                       "Show password",
								"signupActionLinkText":                   "${footerLinkText}",
								"signupActionText":                       "${footerText}",
								"title":                                  "Welcome friend, glad to have you!",
								"user-blocked":                           "Your account has been blocked after multiple consecutive login attempts.",
								"usernamePlaceholder":                    "Username or email address",
								"usernameOnlyPlaceholder":                "Username",
								"usernameOrEmailPlaceholder":             "Username or Email address",
								"wrong-credentials":                      "Wrong username or password",
								"wrong-email-credentials":                "Wrong email or password",
								"wrong-email-phone-credentials":          "Incorrect email address, phone number, or password. Phone numbers must include the country code.",
								"wrong-email-phone-username-credentials": " Incorrect email address, phone number, username, or password. Phone numbers must include the country code.",
								"wrong-email-username-credentials":       "Incorrect email address, username, or password",
								"wrong-phone-credentials":                "Incorrect phone number or password",
								"wrong-phone-username-credentials":       "Incorrect phone number, username or password. Phone numbers must include the country code.",
								"wrong-username-credentials":             "Incorrect username or password",
							},
						},
					},
				},
			},
		},
		{
			name: "it uses a default branding theme if it fails to fetch it",
			mockedAPI: func() *auth0.API {
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

				mockPromptAPI := mock.NewMockPromptAPI(ctrl)
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
				for _, promptType := range allowedPromptsWithPartials {
					mockPromptAPI.EXPECT().
						GetPartials(gomock.Any(), promptType).
						Return(&management.PromptScreenPartials{
							management.ScreenName(promptType): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						}, nil)
				}
				mockClientAPI := mock.NewMockClientAPI(ctrl)
				mockClientAPI.
					EXPECT().
					List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&management.ClientList{
						Clients: []*management.Client{
							{
								ClientID:       auth0.String("1"),
								Name:           auth0.String("My App"),
								LogoURI:        auth0.String("https://my-app.example.com/image.png"),
								ClientMetadata: &map[string]interface{}{"meta": "meta"},
							},
						},
					}, nil)

				mockAPI := &auth0.API{
					Client:        mockClientAPI,
					Branding:      mockBrandingAPI,
					BrandingTheme: mockBrandingThemeAPI,
					Prompt:        mockPromptAPI,
					Tenant:        mockTenantAPI,
				}

				return mockAPI
			},
			expectedData: &universalLoginBrandingData{
				Applications: []*applicationData{
					{
						ID:       "1",
						Name:     "My App",
						LogoURL:  "https://my-app.example.com/image.png",
						Metadata: map[string]interface{}{"meta": "meta"},
					},
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
				Partials: []partialsData{
					{
						"signup": {
							management.ScreenName("signup"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
					{
						"signup-id": {
							management.ScreenName("signup-id"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
					{
						"signup-password": {
							management.ScreenName("signup-password"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
					{
						"login": {
							management.ScreenName("login"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
					{
						"login-id": {
							management.ScreenName("login-id"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
					{
						"login-password": {
							management.ScreenName("login-password"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
					{
						"login-passwordless": {
							management.ScreenName("login-passwordless"): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						},
					},
				},
				Prompts: []*promptData{
					{
						Language: "en",
						Prompt:   "login",
						CustomText: map[string]interface{}{
							"login": map[string]interface{}{
								"alertListTitle":                         "Alerts",
								"auth0-users-validation":                 "Something went wrong, please try again later",
								"authentication-failure":                 "We are sorry, something went wrong when attempting to log in",
								"buttonText":                             "Continue",
								"custom-script-error-code":               "Something went wrong, please try again later.",
								"description":                            "Log in to ${companyName} to continue to ${clientName}.",
								"editEmailText":                          "Edit",
								"emailPlaceholder":                       "Email address",
								"federatedConnectionButtonText":          "Continue with ${connectionName}",
								"footerLinkText":                         "Sign up",
								"footerText":                             "Don't have an account?",
								"forgotPasswordText":                     "Forgot password?",
								"hidePasswordText":                       "Hide password",
								"invalid-connection":                     "Invalid connection",
								"invitationDescription":                  "Log in to accept ${inviterName}'s invitation to join ${companyName} on ${clientName}.",
								"invitationTitle":                        "You've Been Invited!",
								"ip-blocked":                             "We have detected suspicious login behavior and further attempts will be blocked. Please contact the administrator.",
								"logoAltText":                            "${companyName}",
								"no-db-connection":                       "Invalid connection",
								"no-email":                               "Please enter an email address",
								"no-password":                            "Password is required",
								"no-username":                            "Username is required",
								"pageTitle":                              "Log in | ${clientName}",
								"password-breached":                      "We have detected a potential security issue with this account. To protect your account, we have prevented this login. Please reset your password to proceed.",
								"passwordPlaceholder":                    "Password",
								"phoneOrEmailPlaceholder":                "Phone number or Email address",
								"phoneOrUsernameOrEmailPlaceholder":      "Phone or Username or Email",
								"phoneOrUsernamePlaceholder":             "Phone Number or Username",
								"phonePlaceholder":                       "Phone number",
								"same-user-login":                        "Too many login attempts for this user. Please wait, and try again later.",
								"separatorText":                          "Or",
								"showPasswordText":                       "Show password",
								"signupActionLinkText":                   "${footerLinkText}",
								"signupActionText":                       "${footerText}",
								"title":                                  "Welcome friend, glad to have you!",
								"user-blocked":                           "Your account has been blocked after multiple consecutive login attempts.",
								"usernamePlaceholder":                    "Username or email address",
								"usernameOnlyPlaceholder":                "Username",
								"usernameOrEmailPlaceholder":             "Username or Email address",
								"wrong-credentials":                      "Wrong username or password",
								"wrong-email-credentials":                "Wrong email or password",
								"wrong-email-phone-credentials":          "Incorrect email address, phone number, or password. Phone numbers must include the country code.",
								"wrong-email-phone-username-credentials": " Incorrect email address, phone number, username, or password. Phone numbers must include the country code.",
								"wrong-email-username-credentials":       "Incorrect email address, username, or password",
								"wrong-phone-credentials":                "Incorrect phone number or password",
								"wrong-phone-username-credentials":       "Incorrect phone number, username or password. Phone numbers must include the country code.",
								"wrong-username-credentials":             "Incorrect username or password",
							},
						},
					},
				},
			},
		},
		{
			name: "it fails to fetch branding data if there's an error retrieving tenant data",
			mockedAPI: func() *auth0.API {
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

				mockPromptAPI := mock.NewMockPromptAPI(ctrl)
				mockTenantAPI := mock.NewMockTenantAPI(ctrl)
				mockTenantAPI.
					EXPECT().
					Read(gomock.Any()).
					Return(nil, fmt.Errorf("failed to fetch tenant data"))
				for _, promptType := range allowedPromptsWithPartials {
					mockPromptAPI.EXPECT().
						GetPartials(gomock.Any(), promptType).
						Return(&management.PromptScreenPartials{
							management.ScreenName(promptType): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						}, nil)
				}
				mockClientAPI := mock.NewMockClientAPI(ctrl)
				mockClientAPI.
					EXPECT().
					List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&management.ClientList{
						Clients: []*management.Client{
							{
								ClientID:       auth0.String("1"),
								Name:           auth0.String("My App"),
								LogoURI:        auth0.String("https://my-app.example.com/image.png"),
								ClientMetadata: &map[string]interface{}{"meta": "meta"},
							},
						},
					}, nil)

				mockAPI := &auth0.API{
					Client:        mockClientAPI,
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

				mockPromptAPI := mock.NewMockPromptAPI(ctrl)
				mockPromptAPI.
					EXPECT().
					CustomText(gomock.Any(), "login", "en").
					Return(nil, fmt.Errorf("failed to fetch custom text"))
				for _, promptType := range allowedPromptsWithPartials {
					mockPromptAPI.EXPECT().
						GetPartials(gomock.Any(), promptType).
						Return(&management.PromptScreenPartials{
							management.ScreenName(promptType): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						}, nil)
				}
				mockClientAPI := mock.NewMockClientAPI(ctrl)
				mockClientAPI.
					EXPECT().
					List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&management.ClientList{
						Clients: []*management.Client{
							{
								ClientID:       auth0.String("1"),
								Name:           auth0.String("My App"),
								LogoURI:        auth0.String("https://my-app.example.com/image.png"),
								ClientMetadata: &map[string]interface{}{"meta": "meta"},
							},
						},
					}, nil)

				mockAPI := &auth0.API{
					Client:        mockClientAPI,
					Branding:      mockBrandingAPI,
					BrandingTheme: mockBrandingThemeAPI,
					Prompt:        mockPromptAPI,
					Tenant:        mockTenantAPI,
				}

				return mockAPI
			},
			expectedError: "failed to fetch custom text",
		},
		{
			name: "it fails to fetch branding data if there's an error retrieving client data",
			mockedAPI: func() *auth0.API {
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

				mockPromptAPI := mock.NewMockPromptAPI(ctrl)
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
				for _, promptType := range allowedPromptsWithPartials {
					mockPromptAPI.EXPECT().
						GetPartials(gomock.Any(), promptType).
						Return(&management.PromptScreenPartials{
							management.ScreenName(promptType): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						}, nil)
				}
				mockClientAPI := mock.NewMockClientAPI(ctrl)
				mockClientAPI.
					EXPECT().
					List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, fmt.Errorf("failed to fetch client data"))
				mockAPI := &auth0.API{
					Client:        mockClientAPI,
					Branding:      mockBrandingAPI,
					BrandingTheme: mockBrandingThemeAPI,
					Prompt:        mockPromptAPI,
					Tenant:        mockTenantAPI,
				}

				return mockAPI
			},
			expectedError: "failed to fetch client data",
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

func TestCheckOriginFunc(t *testing.T) {
	var testCases = []struct {
		testName string
		request  *http.Request
		expected bool
	}{
		{
			testName: "No Origin Header",
			request: &http.Request{
				Header: http.Header{},
			},
			expected: false,
		},
		{
			testName: "Valid Origin",
			request: &http.Request{
				Header: http.Header{
					"Origin": []string{webServerURL},
				},
			},
			expected: true,
		},
		{
			testName: "Invalid Origin",
			request: &http.Request{
				Header: http.Header{
					"Origin": []string{"https://invalid.com"},
				},
			},
			expected: false,
		},
		{
			testName: "Malformed Origin - Invalid URL",
			request: &http.Request{
				Header: http.Header{
					"Origin": []string{"http://:80"}, // Incomplete URL with missing host.
				},
			},
			expected: false,
		},
		{
			testName: "Malformed Origin - Invalid Scheme",
			request: &http.Request{
				Header: http.Header{
					"Origin": []string{"ftp://example.com"}, // Unsupported scheme.
				},
			},
			expected: false,
		},
		{
			testName: "Malformed Origin - Invalid URL Encoding",
			request: &http.Request{
				Header: http.Header{
					"Origin": []string{"http://%zz%zz"}, // Invalid percent encoding.
				},
			},
			expected: false,
		},
		{
			testName: "Empty Origin URL",
			request: &http.Request{
				Header: http.Header{
					"Origin": []string{"http://"}, // Valid scheme but empty path.
				},
			},
			expected: false,
		},
	}

	for _, test := range testCases {
		t.Run(test.testName, func(t *testing.T) {
			actual := checkOriginFunc(test.request)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestWebSocketMessage_MarshalJSON(t *testing.T) {
	var testCases = []struct {
		name     string
		input    *webSocketMessage
		expected string
	}{
		{
			name: "it can marshal a fetch prompt data message",
			input: &webSocketMessage{
				Type: "FETCH_PROMPT",
				Payload: &promptData{
					Language:   "en",
					Prompt:     "login",
					CustomText: map[string]interface{}{"key": "value"},
				},
			},
			expected: `{"type":"FETCH_PROMPT","payload":{"language":"en","prompt":"login","custom_text":{"key":"value"}}}`,
		},
		{
			name: "it can marshal a fetch partial data message",
			input: &webSocketMessage{
				Type: "FETCH_PARTIAL",
				Payload: &partialData{
					InsertionPoint: "form-content-start",
					ScreenName:     "login",
					PromptName:     "login",
				},
			},
			expected: `{"type":"FETCH_PARTIAL","payload":{"insertion_point":"form-content-start","screen_name":"login","prompt_name":"login"}}`,
		},
		{
			name: "it can marshal a fetch branding data message",
			input: &webSocketMessage{
				Type:    "FETCH_BRANDING",
				Payload: &universalLoginBrandingData{},
			},
			expected: `{"type":"FETCH_BRANDING","payload":{"applications":null,"prompts":null,"partials":null,"settings":null,"template":null,"theme":null,"tenant":null}}`,
		},
		{
			name: "it can marshal a message with an empty payload",
			input: &webSocketMessage{
				Type: "FETCH_BRANDING",
			},
			expected: `{"type":"FETCH_BRANDING","payload":null}`,
		},
		{
			name: "it can marshal a message without payload",
			input: &webSocketMessage{
				Type: "FETCH_PARTIALS_FEATURE_FLAG",
			},
			expected: `{"type":"FETCH_PARTIALS_FEATURE_FLAG","payload":null}`,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			actual, err := json.Marshal(test.input)
			assert.NoError(t, err)
			assert.Equal(t, test.expected, string(actual))
		})
	}
}

func TestWebSocketMessage_UnmarshalJSON(t *testing.T) {
	var testCases = []struct {
		name     string
		input    []byte
		expected *webSocketMessage
	}{
		{
			name:  "it can unmarshal a fetch prompt data message",
			input: []byte(`{"type":"FETCH_PROMPT","payload":{"language":"en","prompt":"login","custom_text":{"key":"value"}}}`),
			expected: &webSocketMessage{
				Type: "FETCH_PROMPT",
				Payload: &promptData{
					Language:   "en",
					Prompt:     "login",
					CustomText: map[string]interface{}{"key": "value"},
				},
			},
		},
		{
			name:  "it can unmarshal a fetch partial data message",
			input: []byte(`{"type":"FETCH_PARTIAL","payload":{"insertion_point":"form-content-start","prompt_name":"login"}}`),
			expected: &webSocketMessage{
				Type: "FETCH_PARTIAL",
				Payload: &partialData{
					InsertionPoint: "form-content-start",
					PromptName:     "login",
				},
			},
		},
		{
			name:  "it can unmarshal a fetch partials feature flag message",
			input: []byte(`{"type":"FETCH_PARTIALS_FEATURE_FLAG", "payload": {"feature_flag": false}}`),
			expected: &webSocketMessage{
				Type: "FETCH_PARTIALS_FEATURE_FLAG",
				Payload: &partialFlagData{
					FeatureFlag: false,
				},
			},
		},
		{
			name:  "it can unmarshal a fetch branding data message",
			input: []byte(`{"type":"FETCH_BRANDING","payload":{"applications":null,"prompts":null,"settings":null,"template":null,"theme":null,"tenant":null}}`),
			expected: &webSocketMessage{
				Type:    "FETCH_BRANDING",
				Payload: &universalLoginBrandingData{},
			},
		},
		{
			name:  "it can unmarshal a message with an empty payload",
			input: []byte(`{"type":"FETCH_BRANDING","payload":null}`),
			expected: &webSocketMessage{
				Type: "FETCH_BRANDING",
			},
		},
		{
			name:  "it can unmarshal a message with an unknown payload",
			input: []byte(`{"type":"UNKNOWN","payload":{"key":"value"}}`),
			expected: &webSocketMessage{
				Type:    "UNKNOWN",
				Payload: map[string]interface{}{"key": "value"},
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			var actual webSocketMessage
			err := json.Unmarshal(test.input, &actual)
			assert.NoError(t, err)
			assert.Equal(t, test.expected, &actual)
		})
	}
}

func TestSaveUniversalLoginBrandingData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var testCases = []struct {
		name          string
		input         *universalLoginBrandingData
		expectedError string
		mockedAPI     func() *auth0.API
	}{
		{
			name: "it can correctly save all of the universal login branding data",
			input: &universalLoginBrandingData{
				Settings: &management.Branding{
					Colors: &management.BrandingColors{
						Primary:        auth0.String("#33ddff"),
						PageBackground: auth0.String("#99aacc"),
					},
				},
				Template: &management.BrandingUniversalLogin{
					Body: auth0.String("<html></html>"),
				},
				Theme: &management.BrandingTheme{},
				Partials: []partialsData{
					{
						"login": {
							management.ScreenName("login"): {
								management.InsertionPointFormContentStart: "<div>Updated Form Content Start</div>",
							},
						},
					},
				},
				Prompts: []*promptData{
					{
						Language:   "en",
						Prompt:     "login",
						CustomText: map[string]interface{}{"key": "value"},
					},
				},
			},
			mockedAPI: func() *auth0.API {
				mockBrandingAPI := mock.NewMockBrandingAPI(ctrl)
				mockBrandingAPI.EXPECT().
					Update(gomock.Any(), &management.Branding{
						Colors: &management.BrandingColors{
							Primary:        auth0.String("#33ddff"),
							PageBackground: auth0.String("#99aacc"),
						},
					}).
					Return(nil)
				mockBrandingAPI.EXPECT().
					SetUniversalLogin(gomock.Any(), &management.BrandingUniversalLogin{
						Body: auth0.String("<html></html>"),
					}).
					Return(nil)

				mockBrandingThemeAPI := mock.NewMockBrandingThemeAPI(ctrl)
				mockBrandingThemeAPI.EXPECT().
					Default(gomock.Any()).
					Return(&management.BrandingTheme{
						ID: auth0.String("111"),
					}, nil)
				mockBrandingThemeAPI.EXPECT().
					Update(gomock.Any(), "111", &management.BrandingTheme{}).
					Return(nil)

				mockPromptAPI := mock.NewMockPromptAPI(ctrl)
				mockPromptAPI.EXPECT().
					SetCustomText(gomock.Any(), "login", "en", map[string]interface{}{"key": "value"}).
					Return(nil)
				mockPromptAPI.EXPECT().
					SetPartials(gomock.Any(), management.PromptLogin, &management.PromptScreenPartials{
						management.ScreenLogin: {
							management.InsertionPointFormContentStart: "<div>Updated Form Content Start</div>",
						},
					}).
					Return(nil)
				mockAPI := &auth0.API{
					Branding:      mockBrandingAPI,
					BrandingTheme: mockBrandingThemeAPI,
					Prompt:        mockPromptAPI,
				}

				return mockAPI
			},
		},
		{
			name: "it fails to save the universal login branding data if the branding api returns an error",
			input: &universalLoginBrandingData{
				Settings: &management.Branding{
					Colors: &management.BrandingColors{
						Primary:        auth0.String("#33ddff"),
						PageBackground: auth0.String("#99aacc"),
					},
				},
				Template: &management.BrandingUniversalLogin{
					Body: auth0.String("<html></html>"),
				},
				Theme: &management.BrandingTheme{},
				Partials: []partialsData{
					{
						"login": {
							management.ScreenName("login"): {
								management.InsertionPointFormContentStart: "<div>Updated Form Content Start</div>",
							},
						},
					},
				},
				Prompts: []*promptData{
					{
						Language:   "en",
						Prompt:     "login",
						CustomText: map[string]interface{}{"key": "value"},
					},
				},
			},
			expectedError: "branding api failure",
			mockedAPI: func() *auth0.API {
				mockBrandingAPI := mock.NewMockBrandingAPI(ctrl)
				mockBrandingAPI.EXPECT().
					Update(gomock.Any(), &management.Branding{
						Colors: &management.BrandingColors{
							Primary:        auth0.String("#33ddff"),
							PageBackground: auth0.String("#99aacc"),
						},
					}).
					Return(fmt.Errorf("branding api failure"))
				mockBrandingAPI.EXPECT().
					SetUniversalLogin(gomock.Any(), &management.BrandingUniversalLogin{
						Body: auth0.String("<html></html>"),
					}).
					Return(nil)

				mockBrandingThemeAPI := mock.NewMockBrandingThemeAPI(ctrl)
				mockBrandingThemeAPI.EXPECT().
					Default(gomock.Any()).
					Return(&management.BrandingTheme{
						ID: auth0.String("111"),
					}, nil)
				mockBrandingThemeAPI.EXPECT().
					Update(gomock.Any(), "111", &management.BrandingTheme{}).
					Return(nil)

				mockPromptAPI := mock.NewMockPromptAPI(ctrl)
				mockPromptAPI.EXPECT().
					SetCustomText(gomock.Any(), "login", "en", map[string]interface{}{"key": "value"}).
					Return(nil)
				mockPromptAPI.EXPECT().
					SetPartials(gomock.Any(), management.PromptLogin, &management.PromptScreenPartials{
						management.ScreenLogin: {
							management.InsertionPointFormContentStart: "<div>Updated Form Content Start</div>",
						},
					}).
					Return(nil)
				mockAPI := &auth0.API{
					Branding:      mockBrandingAPI,
					BrandingTheme: mockBrandingThemeAPI,
					Prompt:        mockPromptAPI,
				}

				return mockAPI
			},
		},
		{
			name: "it creates the theme if not found",
			input: &universalLoginBrandingData{
				Settings: &management.Branding{
					Colors: &management.BrandingColors{
						Primary:        auth0.String("#33ddff"),
						PageBackground: auth0.String("#99aacc"),
					},
				},
				Template: &management.BrandingUniversalLogin{
					Body: auth0.String("<html></html>"),
				},
				Theme: &management.BrandingTheme{},
				Partials: []partialsData{
					{
						"login": {
							management.ScreenName("login"): {
								management.InsertionPointFormContentStart: "<div>Updated Form Content Start</div>",
							},
						},
					},
				},
				Prompts: []*promptData{
					{
						Language:   "en",
						Prompt:     "login",
						CustomText: map[string]interface{}{"key": "value"},
					},
				},
			},
			mockedAPI: func() *auth0.API {
				mockBrandingAPI := mock.NewMockBrandingAPI(ctrl)
				mockBrandingAPI.EXPECT().
					Update(gomock.Any(), &management.Branding{
						Colors: &management.BrandingColors{
							Primary:        auth0.String("#33ddff"),
							PageBackground: auth0.String("#99aacc"),
						},
					}).
					Return(nil)
				mockBrandingAPI.EXPECT().
					SetUniversalLogin(gomock.Any(), &management.BrandingUniversalLogin{
						Body: auth0.String("<html></html>"),
					}).
					Return(nil)

				mockBrandingThemeAPI := mock.NewMockBrandingThemeAPI(ctrl)
				mockBrandingThemeAPI.EXPECT().
					Default(gomock.Any()).
					Return(&management.BrandingTheme{}, fmt.Errorf("failed to find theme"))
				mockBrandingThemeAPI.EXPECT().
					Create(gomock.Any(), &management.BrandingTheme{}).
					Return(nil)

				mockPromptAPI := mock.NewMockPromptAPI(ctrl)
				mockPromptAPI.EXPECT().
					SetCustomText(gomock.Any(), "login", "en", map[string]interface{}{"key": "value"}).
					Return(nil)
				mockPromptAPI.EXPECT().
					SetPartials(gomock.Any(), management.PromptLogin, &management.PromptScreenPartials{
						management.ScreenLogin: {
							management.InsertionPointFormContentStart: "<div>Updated Form Content Start</div>",
						},
					}).
					Return(nil)
				mockAPI := &auth0.API{
					Branding:      mockBrandingAPI,
					BrandingTheme: mockBrandingThemeAPI,
					Prompt:        mockPromptAPI,
				}

				return mockAPI
			},
		},
		{
			name: "it ignores errors of partial prompts for specific prompts error",
			input: &universalLoginBrandingData{
				Settings: &management.Branding{
					Colors: &management.BrandingColors{
						Primary:        auth0.String("#33ddff"),
						PageBackground: auth0.String("#99aacc"),
					},
				},
				Template: &management.BrandingUniversalLogin{
					Body: auth0.String("<html></html>"),
				},
				Theme: &management.BrandingTheme{},
				Partials: []partialsData{
					{
						"login": {
							management.ScreenName("login"): {
								management.InsertionPointFormContentStart: "<div>Updated Form Content Start</div>",
							},
						},
					},
				},
				Prompts: []*promptData{
					{
						Language:   "en",
						Prompt:     "login",
						CustomText: map[string]interface{}{"key": "value"},
					},
				},
			},
			mockedAPI: func() *auth0.API {
				mockBrandingAPI := mock.NewMockBrandingAPI(ctrl)
				mockBrandingAPI.EXPECT().
					Update(gomock.Any(), &management.Branding{
						Colors: &management.BrandingColors{
							Primary:        auth0.String("#33ddff"),
							PageBackground: auth0.String("#99aacc"),
						},
					}).
					Return(nil)
				mockBrandingAPI.EXPECT().
					SetUniversalLogin(gomock.Any(), &management.BrandingUniversalLogin{
						Body: auth0.String("<html></html>"),
					}).
					Return(nil)

				mockBrandingThemeAPI := mock.NewMockBrandingThemeAPI(ctrl)
				mockBrandingThemeAPI.EXPECT().
					Default(gomock.Any()).
					Return(&management.BrandingTheme{
						ID: auth0.String("111"),
					}, nil)
				mockBrandingThemeAPI.EXPECT().
					Update(gomock.Any(), "111", &management.BrandingTheme{}).
					Return(nil)

				mockPromptAPI := mock.NewMockPromptAPI(ctrl)
				mockPromptAPI.EXPECT().
					SetCustomText(gomock.Any(), "login", "en", map[string]interface{}{"key": "value"}).
					Return(nil)
				mockPromptAPI.EXPECT().
					SetPartials(gomock.Any(), management.PromptLogin, &management.PromptScreenPartials{
						management.ScreenLogin: {
							management.InsertionPointFormContentStart: "<div>Updated Form Content Start</div>",
						},
					}).
					Return(fmt.Errorf("Your account does not have custom prompts"))
				mockAPI := &auth0.API{
					Branding:      mockBrandingAPI,
					BrandingTheme: mockBrandingThemeAPI,
					Prompt:        mockPromptAPI,
				}

				return mockAPI
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			err := saveUniversalLoginBrandingData(context.Background(), test.mockedAPI(), test.input)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestFetchAllPartials(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var testCases = []struct {
		name          string
		expectedData  []partialsData
		expectedError string
		mockedAPI     func() *auth0.API
	}{
		{
			name: "it can fetch all partials",
			expectedData: []partialsData{
				{
					"signup": {
						management.ScreenName("signup"): {
							management.InsertionPointFormContentEnd: "<form>",
						},
					},
				},
				{
					"signup-id": {
						management.ScreenName("signup-id"): {
							management.InsertionPointFormContentEnd: "<form>",
						},
					},
				},
				{
					"signup-password": {
						management.ScreenName("signup-password"): {
							management.InsertionPointFormContentEnd: "<form>",
						},
					},
				},
				{
					"login": {
						management.ScreenName("login"): {
							management.InsertionPointFormContentEnd: "<form>",
						},
					},
				},
				{
					"login-id": {
						management.ScreenName("login-id"): {
							management.InsertionPointFormContentEnd: "<form>",
						},
					},
				},
				{
					"login-password": {
						management.ScreenName("login-password"): {
							management.InsertionPointFormContentEnd: "<form>",
						},
					},
				},
				{
					"login-passwordless": {
						management.ScreenName("login-passwordless"): {
							management.InsertionPointFormContentEnd: "<form>",
						},
					},
				},
			},
			mockedAPI: func() *auth0.API {
				mockPromptAPI := mock.NewMockPromptAPI(ctrl)
				for _, promptType := range allowedPromptsWithPartials {
					mockPromptAPI.EXPECT().
						GetPartials(gomock.Any(), promptType).
						Return(&management.PromptScreenPartials{
							management.ScreenName(promptType): {
								management.InsertionPointFormContentEnd: "<form>",
							},
						}, nil)
				}

				mockAPI := &auth0.API{
					Prompt: mockPromptAPI,
				}

				return mockAPI
			},
		},
		{
			name:          "it fails to fetch partials if there's an error retrieving them",
			expectedError: "failed to fetch partials",
			mockedAPI: func() *auth0.API {
				mockPromptAPI := mock.NewMockPromptAPI(ctrl)
				mockPromptAPI.EXPECT().
					GetPartials(gomock.Any(), gomock.Any()).
					Return(nil, fmt.Errorf("failed to fetch partials"))

				mockAPI := &auth0.API{
					Prompt: mockPromptAPI,
				}

				return mockAPI
			},
		},
		{
			name: "it doesn't fail if feature flag is disabled",
			expectedData: []partialsData{
				{
					"signup": {},
				},
				{
					"signup-id": {},
				},
				{
					"signup-password": {},
				},
				{
					"login": {},
				},
				{
					"login-id": {},
				},
				{
					"login-password": {},
				},
				{
					"login-passwordless": {},
				},
			},
			mockedAPI: func() *auth0.API {
				mockPromptAPI := mock.NewMockPromptAPI(ctrl)
				for _, prompt := range allowedPromptsWithPartials {
					mockPromptAPI.EXPECT().
						GetPartials(gomock.Any(), prompt).
						Return(nil, fmt.Errorf("feature is not available for your plan")).
						Times(1)
				}
				return &auth0.API{
					Prompt: mockPromptAPI,
				}
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			actualData, actualError := fetchAllPartials(context.Background(), test.mockedAPI())

			if test.expectedError != "" {
				assert.Error(t, actualError)
				assert.EqualError(t, actualError, test.expectedError)
			} else {
				assert.NoError(t, actualError)
				assert.Equal(t, test.expectedData, actualData)
			}
		})
	}
}
