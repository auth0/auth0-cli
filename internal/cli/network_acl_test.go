package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/golang/mock/gomock"

	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/auth0/mock"
)

func TestNetworkACLPickerOptions(t *testing.T) {
	// Disable colors for consistent test output.
	tests := []struct {
		name         string
		networkACLs  []*management.NetworkACL
		apiError     error
		assertOutput func(t testing.TB, options pickerOptions)
		assertError  func(t testing.TB, err error)
	}{
		{
			name: "happy path",
			networkACLs: []*management.NetworkACL{
				{
					ID:          auth0.String("acl-id-1"),
					Description: auth0.String("Block IPs"),
				},
				{
					ID:          auth0.String("acl-id-2"),
					Description: auth0.String("Allow Countries"),
				},
			},
			assertOutput: func(t testing.TB, options pickerOptions) {
				assert.Len(t, options, 2)
				// Check the value which should not have ANSI formatting.
				assert.Equal(t, "acl-id-1", options[0].value)
				assert.Equal(t, "acl-id-2", options[1].value)

				// For labels, just check that they contain the expected text without worrying about ANSI codes.
				assert.Contains(t, options[0].label, "Block IPs")
				assert.Contains(t, options[0].label, "acl-id-1")
				assert.Contains(t, options[1].label, "Allow Countries")
				assert.Contains(t, options[1].label, "acl-id-2")
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name:        "no network ACLs",
			networkACLs: []*management.NetworkACL{},
			assertOutput: func(t testing.TB, options pickerOptions) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorContains(t, err, "there are currently no network ACLs to choose from")
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

			networkACLAPI := mock.NewMockNetworkACLAPI(ctrl)
			networkACLAPI.EXPECT().
				List(gomock.Any()).
				Return(test.networkACLs, test.apiError)

			cli := &cli{
				api: &auth0.API{NetworkACL: networkACLAPI},
			}

			options, err := cli.networkACLPickerOptions(context.Background())

			if err != nil {
				test.assertError(t, err)
			} else {
				test.assertOutput(t, options)
			}
		})
	}
}

func TestBuildNetworkACLRule_Auth0Managed(t *testing.T) {
	tests := []struct {
		name        string
		inputs      *ruleInputs
		assertRule  func(t testing.TB, rule *management.NetworkACLRule)
		expectError bool
	}{
		{
			name: "auth0_managed on match",
			inputs: &ruleInputs{
				Scope:        "tenant",
				Action:       "block",
				Auth0Managed: []string{"auth0.low_reputation", "auth0.icloud_relay_proxy"},
				IsMatchRule:  true,
			},
			assertRule: func(t testing.TB, rule *management.NetworkACLRule) {
				assert.Nil(t, rule.NotMatch)
				assert.NotNil(t, rule.Match)
				assert.NotNil(t, rule.Match.Auth0Managed)
				assert.Equal(t, []string{"auth0.low_reputation", "auth0.icloud_relay_proxy"}, *rule.Match.Auth0Managed)
			},
		},
		{
			name: "auth0_managed on not_match",
			inputs: &ruleInputs{
				Scope:        "tenant",
				Action:       "block",
				Auth0Managed: []string{"auth0.low_reputation"},
				IsMatchRule:  false,
			},
			assertRule: func(t testing.TB, rule *management.NetworkACLRule) {
				assert.Nil(t, rule.Match)
				assert.NotNil(t, rule.NotMatch)
				assert.NotNil(t, rule.NotMatch.Auth0Managed)
				assert.Equal(t, []string{"auth0.low_reputation"}, *rule.NotMatch.Auth0Managed)
			},
		},
		{
			name: "auth0_managed coexists with other criteria",
			inputs: &ruleInputs{
				Scope:        "tenant",
				Action:       "block",
				IPv4CIDRs:    []string{"192.168.1.0/24"},
				Auth0Managed: []string{"auth0.low_reputation"},
				IsMatchRule:  true,
			},
			assertRule: func(t testing.TB, rule *management.NetworkACLRule) {
				assert.NotNil(t, rule.Match)
				assert.NotNil(t, rule.Match.IPv4Cidrs)
				assert.NotNil(t, rule.Match.Auth0Managed)
				assert.Equal(t, []string{"auth0.low_reputation"}, *rule.Match.Auth0Managed)
			},
		},
		{
			name: "auth0_managed empty is not set",
			inputs: &ruleInputs{
				Scope:       "tenant",
				Action:      "block",
				IsMatchRule: true,
			},
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rule, err := buildNetworkACLRule(test.inputs)

			if test.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			test.assertRule(t, rule)
		})
	}
}

func TestBuildNetworkACLRule_HTTPMessageSignature(t *testing.T) {
	tests := []struct {
		name        string
		inputs      *ruleInputs
		assertRule  func(t testing.TB, rule *management.NetworkACLRule)
		expectError bool
	}{
		{
			name: "signature keys on match",
			inputs: &ruleInputs{
				Scope:           "tenant",
				Action:          "block",
				SignatureKeyIDs: []string{"key_abc", "key_def"},
				IsMatchRule:     true,
			},
			assertRule: func(t testing.TB, rule *management.NetworkACLRule) {
				assert.Nil(t, rule.NotMatch)
				assert.NotNil(t, rule.Match)
				assert.NotNil(t, rule.Match.HTTPMessageSignature)
				assert.Equal(t, []string{"key_abc", "key_def"}, signatureKeyIDs(rule.Match))
			},
		},
		{
			name: "signature keys on not_match",
			inputs: &ruleInputs{
				Scope:           "tenant",
				Action:          "block",
				SignatureKeyIDs: []string{"key_abc"},
				IsMatchRule:     false,
			},
			assertRule: func(t testing.TB, rule *management.NetworkACLRule) {
				assert.Nil(t, rule.Match)
				assert.NotNil(t, rule.NotMatch)
				assert.Equal(t, []string{"key_abc"}, signatureKeyIDs(rule.NotMatch))
			},
		},
		{
			name: "signature keys coexist with other criteria",
			inputs: &ruleInputs{
				Scope:           "tenant",
				Action:          "block",
				IPv4CIDRs:       []string{"192.168.1.0/24"},
				SignatureKeyIDs: []string{"key_abc"},
				IsMatchRule:     true,
			},
			assertRule: func(t testing.TB, rule *management.NetworkACLRule) {
				assert.NotNil(t, rule.Match)
				assert.NotNil(t, rule.Match.IPv4Cidrs)
				assert.Equal(t, []string{"key_abc"}, signatureKeyIDs(rule.Match))
			},
		},
		{
			name: "no criteria is an error",
			inputs: &ruleInputs{
				Scope:       "tenant",
				Action:      "block",
				IsMatchRule: true,
			},
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rule, err := buildNetworkACLRule(test.inputs)

			if test.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			test.assertRule(t, rule)
		})
	}
}

// signatureKeyIDs is a test helper that flattens the referenced key ids of a match.
func signatureKeyIDs(match *management.NetworkACLRuleMatch) []string {
	if match == nil || match.HTTPMessageSignature == nil {
		return nil
	}
	ids := make([]string, 0, len(match.HTTPMessageSignature.Keys))
	for _, k := range match.HTTPMessageSignature.Keys {
		if k.ID != nil {
			ids = append(ids, *k.ID)
		}
	}
	return ids
}

func TestExtractCurrentRuleDefaults_HTTPMessageSignature(t *testing.T) {
	tests := []struct {
		name     string
		acl      *management.NetworkACL
		wantKeys []string
	}{
		{
			name: "extracts signature keys from match",
			acl: &management.NetworkACL{
				Rule: &management.NetworkACLRule{
					Match: &management.NetworkACLRuleMatch{
						HTTPMessageSignature: &management.NetworkACLHTTPMessageSignature{
							Keys: []*management.NetworkACLHTTPMessageSignatureKey{
								{ID: auth0StringPtr("key_abc")},
								{ID: auth0StringPtr("key_def")},
							},
						},
					},
				},
			},
			wantKeys: []string{"key_abc", "key_def"},
		},
		{
			name: "extracts signature keys from not_match",
			acl: &management.NetworkACL{
				Rule: &management.NetworkACLRule{
					NotMatch: &management.NetworkACLRuleMatch{
						HTTPMessageSignature: &management.NetworkACLHTTPMessageSignature{
							Keys: []*management.NetworkACLHTTPMessageSignatureKey{
								{ID: auth0StringPtr("key_abc")},
							},
						},
					},
				},
			},
			wantKeys: []string{"key_abc"},
		},
		{
			name: "no signature keys set",
			acl: &management.NetworkACL{
				Rule: &management.NetworkACLRule{
					Match: &management.NetworkACLRuleMatch{
						IPv4Cidrs: &[]string{"192.168.1.0/24"},
					},
				},
			},
			wantKeys: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defaults := extractCurrentRuleDefaults(test.acl)
			assert.Equal(t, test.wantKeys, defaults.SignatureKeyIDs)
		})
	}
}

func auth0StringPtr(s string) *string { return &s }

func TestExtractCurrentRuleDefaults_Auth0Managed(t *testing.T) {
	tests := []struct {
		name             string
		acl              *management.NetworkACL
		wantAuth0Managed []string
	}{
		{
			name: "extracts auth0_managed from match",
			acl: &management.NetworkACL{
				Rule: &management.NetworkACLRule{
					Match: &management.NetworkACLRuleMatch{
						Auth0Managed: &[]string{"auth0.low_reputation", "auth0.icloud_relay_proxy"},
					},
				},
			},
			wantAuth0Managed: []string{"auth0.low_reputation", "auth0.icloud_relay_proxy"},
		},
		{
			name: "extracts auth0_managed from not_match",
			acl: &management.NetworkACL{
				Rule: &management.NetworkACLRule{
					NotMatch: &management.NetworkACLRuleMatch{
						Auth0Managed: &[]string{"auth0.low_reputation"},
					},
				},
			},
			wantAuth0Managed: []string{"auth0.low_reputation"},
		},
		{
			name: "no auth0_managed set",
			acl: &management.NetworkACL{
				Rule: &management.NetworkACLRule{
					Match: &management.NetworkACLRuleMatch{
						IPv4Cidrs: &[]string{"192.168.1.0/24"},
					},
				},
			},
			wantAuth0Managed: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defaults := extractCurrentRuleDefaults(test.acl)
			assert.Equal(t, test.wantAuth0Managed, defaults.Auth0Managed)
		})
	}
}
