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

type mockManagementError struct {
	statusCode int
	error
}

func (m mockManagementError) Status() int {
	return m.statusCode
}

func TestAPIProvisioningTypeFor(t *testing.T) {
	t.Run("maps the 'auth0' provisioning type", func(t *testing.T) {
		provisioningType := "auth0"
		result := apiProvisioningTypeFor(provisioningType)

		assert.Equal(t, *result, customDomainProvisioningTypeAuth0)
	})

	t.Run("maps the 'self' provisioning type", func(t *testing.T) {
		provisioningType := "self"
		result := apiProvisioningTypeFor(provisioningType)

		assert.Equal(t, *result, customDomainProvisioningTypeSelf)
	})

	t.Run("returns the input string when the provisioning type is unknown", func(t *testing.T) {
		provisioningType := "foo"
		result := apiProvisioningTypeFor(provisioningType)

		assert.Equal(t, *result, provisioningType)
	})
}

func TestAPIPVerificationMethodFor(t *testing.T) {
	t.Run("maps the 'txt' verification method", func(t *testing.T) {
		verificationMethod := "txt"
		result := apiVerificationMethodFor(verificationMethod)

		assert.Equal(t, *result, customDomainVerificationMethodTxt)
	})

	t.Run("returns the input string when the verification method is unknown", func(t *testing.T) {
		verificationMethod := "foo"
		result := apiVerificationMethodFor(verificationMethod)

		assert.Equal(t, *result, verificationMethod)
	})
}

func TestAPITLSPolicyFor(t *testing.T) {
	t.Run("maps the 'recommended' TLS policy", func(t *testing.T) {
		tlsPolicy := "recommended"
		result := apiTLSPolicyFor(tlsPolicy)

		assert.Equal(t, *result, customDomainTLSPolicyRecommended)
	})

	t.Run("maps the 'recommended' TLS policy", func(t *testing.T) {
		tlsPolicy := "compatible"
		result := apiTLSPolicyFor(tlsPolicy)

		assert.Equal(t, *result, customDomainTLSPolicyCompatible)
	})

	t.Run("returns the input string when the TLS policy is unknown", func(t *testing.T) {
		tlsPolicy := "foo"
		result := apiTLSPolicyFor(tlsPolicy)

		assert.Equal(t, *result, tlsPolicy)
	})
}

func TestCustomDomainsPickerOptions(t *testing.T) {
	tests := []struct {
		name          string
		customDomains []*management.CustomDomain
		apiError      error
		assertOutput  func(t testing.TB, options pickerOptions)
		assertError   func(t testing.TB, err error)
	}{
		{
			name: "happy path",
			customDomains: []*management.CustomDomain{
				{
					ID:     auth0.String("some-id-1"),
					Domain: auth0.String("some-domain-1"),
					Status: auth0.String("ready"),
				},
				{
					ID:     auth0.String("some-id-2"),
					Domain: auth0.String("some-domain-2"),
					Status: auth0.String("ready"),
				},
			},
			assertOutput: func(t testing.TB, options pickerOptions) {
				assert.Len(t, options, 2)
				assert.Equal(t, "some-domain-1 (some-id-1)", options[0].label)
				assert.Equal(t, "some-id-1", options[0].value)
				assert.Equal(t, "some-domain-2 (some-id-2)", options[1].label)
				assert.Equal(t, "some-id-2", options[1].value)
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name: "custom domains with a non-ready status",
			customDomains: []*management.CustomDomain{
				{
					ID:     auth0.String("some-id-1"),
					Domain: auth0.String("some-domain-1"),
					Status: auth0.String("foo"),
				},
				{
					ID:     auth0.String("some-id-2"),
					Domain: auth0.String("some-domain-2"),
					Status: auth0.String("ready"),
				},
				{
					ID:     auth0.String("some-id-3"),
					Domain: auth0.String("some-domain-3"),
					Status: auth0.String("bar"),
				},
			},
			assertOutput: func(t testing.TB, options pickerOptions) {
				assert.Len(t, options, 1)
				assert.Equal(t, "some-domain-2 (some-id-2)", options[0].label)
				assert.Equal(t, "some-id-2", options[0].value)
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name:          "no custom domains",
			customDomains: []*management.CustomDomain{},
			assertOutput: func(t testing.TB, options pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorIs(t, err, errNoCustomDomains)
			},
		},
		{
			name:     "custom domains disabled",
			apiError: &mockManagementError{statusCode: http.StatusForbidden},
			assertOutput: func(t testing.TB, options pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorIs(t, err, errNoCustomDomains)
			},
		},
		{
			name:     "API error",
			apiError: &mockManagementError{statusCode: http.StatusServiceUnavailable},
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

			customDomainAPI := mock.NewMockCustomDomainAPI(ctrl)
			customDomainAPI.EXPECT().
				List(gomock.Any()).
				Return(test.customDomains, test.apiError)

			cli := &cli{
				api: &auth0.API{CustomDomain: customDomainAPI},
			}

			options, err := cli.customDomainsPickerOptions(context.Background())

			if err != nil {
				test.assertError(t, err)
			} else {
				test.assertOutput(t, options)
			}
		})
	}
}
