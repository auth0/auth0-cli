package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/config"
)

func TestQuickstartsTypeFor(t *testing.T) {
	assert.Equal(t, qsSpa, quickstartsTypeFor("spa"))
	assert.Equal(t, qsWebApp, quickstartsTypeFor("regular_web"))
	assert.Equal(t, qsWebApp, quickstartsTypeFor("regular_web"))
	assert.Equal(t, qsBackend, quickstartsTypeFor("non_interactive"))
	assert.Equal(t, "generic", quickstartsTypeFor("some-unknown-value"))
}

func TestDefaultCallbackURLFor(t *testing.T) {
	assert.Equal(t, "http://localhost:3000/api/auth/callback", defaultCallbackURLFor("next.js"))
	assert.Equal(t, "http://localhost:3000", defaultCallbackURLFor("all-other-quickstart-application-types"))
}

func TestDefaultURLFor(t *testing.T) {
	assert.Equal(t, "http://localhost:4200", defaultURLFor("angular"))
	assert.Equal(t, "http://localhost:3000", defaultURLFor("all-other-quickstart-application-types"))
}

func TestUrlPromptFor(t *testing.T) {
	assert.Equal(t, "Quickstarts use localhost, do you want to add http://localhost:3000/api/auth/callback to the list\n of allowed callback URLs and http://localhost:3000 to the list of allowed logout URLs?", urlPromptFor("generic", "Next.js"))
	assert.Equal(t, "Quickstarts use localhost, do you want to add http://localhost:3000 to the list\n of allowed callback URLs and logout URLs?", urlPromptFor("generic", "Laravel API"))
}

func TestQuickstartStrategy(t *testing.T) {
	tenant := config.Tenant{Domain: "test.auth0.com"}

	t.Run("vite strategy", func(t *testing.T) {
		strategy, err := quickstartStrategy("vite")
		assert.NoError(t, err)
		assert.NotNil(t, strategy)
		assert.Equal(t, 5173, strategy.GetDefaultPort())
		assert.Equal(t, "My App", strategy.GetDefaultAppName())

		vite := strategy.(*ViteSetupStrategy)
		vite.createdAppId = "test-client-id"
		envFileName, envContent, err := vite.BuildEnvFileContent(nil, QuickstartSetupInputs{Port: 5173}, tenant)
		assert.NoError(t, err)
		assert.Equal(t, ".env", envFileName)
		assert.Contains(t, envContent, "VITE_AUTH0_DOMAIN=test.auth0.com")
		assert.Contains(t, envContent, "VITE_AUTH0_CLIENT_ID=test-client-id")
	})

	t.Run("nextjs strategy", func(t *testing.T) {
		strategy, err := quickstartStrategy("nextjs")
		assert.NoError(t, err)
		assert.NotNil(t, strategy)
		assert.Equal(t, 3000, strategy.GetDefaultPort())
		assert.Equal(t, "My App", strategy.GetDefaultAppName())

		nextjs := strategy.(*NextjsSetupStrategy)
		nextjs.createdAppId = "test-client-id"
		nextjs.createdClientSecret = "test-client-secret"
		envFileName, envContent, err := nextjs.BuildEnvFileContent(nil, QuickstartSetupInputs{Port: 3000}, tenant)
		assert.NoError(t, err)
		assert.Equal(t, ".env.local", envFileName)
		assert.Contains(t, envContent, "AUTH0_DOMAIN=test.auth0.com")
		assert.Contains(t, envContent, "AUTH0_CLIENT_ID=test-client-id")
		assert.Contains(t, envContent, "AUTH0_CLIENT_SECRET=test-client-secret")
		assert.Contains(t, envContent, "APP_BASE_URL=http://localhost:3000")
	})

	t.Run("jhipster strategy", func(t *testing.T) {
		strategy, err := quickstartStrategy("jhipster")
		assert.NoError(t, err)
		assert.NotNil(t, strategy)
		assert.Equal(t, 8080, strategy.GetDefaultPort())
		assert.Equal(t, "JHipster", strategy.GetDefaultAppName())

		jhipster := strategy.(*JHipsterSetupStrategy)
		jhipster.createdAppId = "test-client-id"
		jhipster.createdClientSecret = "test-client-secret"
		envFileName, envContent, err := jhipster.BuildEnvFileContent(nil, QuickstartSetupInputs{Port: 8080}, tenant)
		assert.NoError(t, err)
		assert.Equal(t, ".auth0.env", envFileName)
		assert.Contains(t, envContent, "SPRING_SECURITY_OAUTH2_CLIENT_PROVIDER_OIDC_ISSUER_URI=\"https://test.auth0.com/\"")
		assert.Contains(t, envContent, "SPRING_SECURITY_OAUTH2_CLIENT_REGISTRATION_OIDC_CLIENT_ID=\"test-client-id\"")
		assert.Contains(t, envContent, "SPRING_SECURITY_OAUTH2_CLIENT_REGISTRATION_OIDC_CLIENT_SECRET=\"test-client-secret\"")
		assert.Contains(t, envContent, "JHIPSTER_SECURITY_OAUTH2_AUDIENCE=\"https://test.auth0.com/api/v2/\"")
	})

	t.Run("case insensitive", func(t *testing.T) {
		strategy, err := quickstartStrategy("VITE")
		assert.NoError(t, err)
		assert.Equal(t, 5173, strategy.GetDefaultPort())

		strategy, err = quickstartStrategy("NextJS")
		assert.NoError(t, err)
		assert.Equal(t, 3000, strategy.GetDefaultPort())

		strategy, err = quickstartStrategy("JHipster")
		assert.NoError(t, err)
		assert.Equal(t, 8080, strategy.GetDefaultPort())
	})

	t.Run("unsupported type", func(t *testing.T) {
		strategy, err := quickstartStrategy("unknown")
		assert.Error(t, err)
		assert.Nil(t, strategy)
		assert.Contains(t, err.Error(), "unsupported quickstart type")
	})
}

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name        string
		port        int
		expectError bool
	}{
		{"port too low", 1023, true},
		{"minimum valid port", 1024, false},
		{"vite default", 5173, false},
		{"nextjs default", 3000, false},
		{"maximum valid port", 65535, false},
		{"port too high", 65536, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePort(tt.port)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid port number")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
