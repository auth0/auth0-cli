package openapi

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIPStringFormats checks each value against the format the live API accepted it
// as (probed through the bot detection allowlist on 2026-11-17). Empty means the API
// rejected it.
func TestIPStringFormats(t *testing.T) {
	tests := []struct {
		value  string
		format string
	}{
		{"192.0.2.10", "ipv4"},
		{"192.0.2.0/24", "cidr"},
		{"192.0.2.10/24", "cidr"},
		{"192.0.2.0/0", "cidr"},
		{"2001:db8::1", "ipv6"},
		{"2001:DB8::1", "ipv6"},
		{"::ffff:192.0.2.1", "ipv6"},
		{"fe80::1%eth0", "ipv6"},
		{"2001:db8::/32", "ipv6_cidr"},
		{"2001:db8::1/64", "ipv6_cidr"},
		{"::ffff:192.0.2.0/120", "ipv6_cidr"},
		{"192.0.2.0/33", ""},
		{"2001:db8::/129", ""},
		{"192.168.001.1", ""},
		{"0300.0.2.1", ""},
		{"1.2.3", ""},
		{" 192.0.2.1", ""},
		{"192.0.2.1 ", ""},
		{"192.0.2.1/", ""},
		{"192.0.2.1/024", ""},
		{"not-an-ip", ""},
		{"", ""},
	}

	for _, tc := range tests {
		t.Run(tc.value, func(t *testing.T) {
			for format, validator := range ipStringFormats {
				err := validator.Validate(tc.value)
				if format == tc.format {
					assert.NoError(t, err, "%q must be a valid %s", tc.value, format)
				} else {
					assert.Error(t, err, "%q must not be a valid %s", tc.value, format)
				}
			}
		})
	}
}

// ipFormatsFixtureDoc mirrors the live spec: network ACL match lists are a two-branch
// oneOf (ipv4 or cidr), the bot detection allowlist a four-branch oneOf through $refs,
// and the brute-force allowlist the same branches as an anyOf.
const ipFormatsFixtureDoc = `{
  "openapi": "3.0.0",
  "info": {"title": "fixture", "version": "1.0.0"},
  "paths": {
    "/network-acls": {"post": {"operationId": "post_network_acls", "requestBody": {"content": {"application/json": {"schema": {
      "type": "object", "additionalProperties": false, "properties": {
        "ipv4_cidrs": {"type": "array", "items": {"$ref": "#/components/schemas/NetworkACLMatchIpv4Cidr"}},
        "ipv6_cidrs": {"type": "array", "items": {"$ref": "#/components/schemas/NetworkACLMatchIpv6Cidr"}}}}}}}}},
    "/attack-protection/bot-detection": {"patch": {"operationId": "patch_bot_detection", "requestBody": {"content": {"application/json": {"schema": {
      "type": "object", "properties": {
        "allowlist": {"type": "array", "items": {"$ref": "#/components/schemas/BotDetectionIPAddressOrCidrBlock"}}}}}}}}},
    "/attack-protection/brute-force-protection": {"patch": {"operationId": "patch_brute_force", "requestBody": {"content": {"application/json": {"schema": {
      "type": "object", "properties": {
        "allowlist": {"type": "array", "items": {"type": "string", "anyOf": [
          {"type": "string", "format": "ipv4"}, {"type": "string", "format": "cidr"},
          {"type": "string", "format": "ipv6"}, {"type": "string", "format": "ipv6_cidr"}]}}}}}}}}}
  },
  "components": {"schemas": {
    "NetworkACLMatchIpv4Cidr": {"type": "string", "oneOf": [
      {"type": "string", "format": "ipv4"}, {"type": "string", "format": "cidr"}]},
    "NetworkACLMatchIpv6Cidr": {"type": "string", "oneOf": [
      {"type": "string", "format": "ipv6"}, {"type": "string", "format": "ipv6_cidr"}]},
    "BotDetectionIPAddressOrCidrBlock": {"type": "string", "oneOf": [
      {"$ref": "#/components/schemas/BotDetectionIPv4"},
      {"$ref": "#/components/schemas/BotDetectionIPv6"},
      {"$ref": "#/components/schemas/BotDetectionCidrBlock"},
      {"$ref": "#/components/schemas/BotDetectionIPv6CidrBlock"}]},
    "BotDetectionIPv4": {"type": "string", "format": "ipv4"},
    "BotDetectionIPv6": {"type": "string", "format": "ipv6"},
    "BotDetectionCidrBlock": {"type": "string", "format": "cidr"},
    "BotDetectionIPv6CidrBlock": {"type": "string", "format": "ipv6_cidr"}
  }}
}`

func TestValidateRequestIPFormatOneOf(t *testing.T) {
	doc, err := LoadDocFromData([]byte(ipFormatsFixtureDoc))
	require.NoError(t, err)
	manager := NewSchemaManagerFromDoc(doc)

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantFields []string // Empty means the payload must be valid.
	}{
		{
			name: "network ACL addresses and CIDR blocks", method: "POST", path: "/network-acls",
			body: `{"ipv4_cidrs":["192.0.2.10","192.0.2.0/24"],"ipv6_cidrs":["2001:db8::1","2001:db8::/32"]}`,
		},
		{
			name: "network ACL invalid and wrong-family values", method: "POST", path: "/network-acls",
			body:       `{"ipv4_cidrs":["192.0.2.10","not-an-ip","2001:db8::/32"],"ipv6_cidrs":["192.0.2.0/24"]}`,
			wantFields: []string{"ipv4_cidrs[1]", "ipv4_cidrs[2]", "ipv6_cidrs[0]"},
		},
		{
			name: "bot detection allowlist with every branch", method: "PATCH", path: "/attack-protection/bot-detection",
			body: `{"allowlist":["198.51.100.42","2001:db8::1","198.51.100.0/24","2001:db8::/32"]}`,
		},
		{
			name: "bot detection allowlist with an invalid value", method: "PATCH", path: "/attack-protection/bot-detection",
			body:       `{"allowlist":["198.51.100.42","198.51.100.0/33"]}`,
			wantFields: []string{"allowlist[1]"},
		},
		{
			name: "brute-force anyOf allowlist", method: "PATCH", path: "/attack-protection/brute-force-protection",
			body: `{"allowlist":["198.51.100.42","198.51.100.0/24","2001:db8::1","2001:db8::/32"]}`,
		},
		{
			name: "brute-force anyOf allowlist with an invalid value", method: "PATCH", path: "/attack-protection/brute-force-protection",
			body:       `{"allowlist":["not-an-ip"]}`,
			wantFields: []string{"allowlist[0]"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := manager.ValidateRequest(tc.method, tc.path, []byte(tc.body))
			require.NoError(t, err)

			if len(tc.wantFields) == 0 {
				assert.Equal(t, StatusValid, result.Status, "errors: %v", result.Errors)
				assert.Empty(t, result.Errors)
				return
			}

			// A oneOf reports one error per branch, each naming the format it failed; an
			// anyOf reports one error for the value.
			assert.Equal(t, StatusInvalid, result.Status)
			var fields []string
			for _, fe := range result.FieldErrors {
				if !slices.Contains(fields, fe.Field) {
					fields = append(fields, fe.Field)
				}
				assert.Contains(t, fe.Reason, "doesn't match")
			}
			assert.Equal(t, tc.wantFields, fields)
			assert.NotContains(t, strings.Join(result.Errors, "\n"), "more than one schema",
				"a value must match at most one IP format")
		})
	}
}
