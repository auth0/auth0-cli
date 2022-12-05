package auth

import "testing"

func TestRequiredScopes(t *testing.T) {
	t.Run("Verify CRUD scopes", func(t *testing.T) {
		crudResources := []string{
			"clients",
			"client_grants",
			"connections",
			"log_streams",
			"resource_servers",
			"roles",
			"rules",
			"users",
			"actions",
			"hooks",
			"organizations",
			"organization_connections",
			"custom_domains",
			"email_provider",
			"shields",
			"users_app_metadata",
		}
		crudPrefixes := []string{"create:", "delete:", "read:", "update:"}

		for _, resource := range crudResources {
			for _, prefix := range crudPrefixes {
				scope := prefix + resource

				if !strInArray(requiredScopes, scope) {
					t.Fatalf("wanted scope: %q, list: %+v", scope, requiredScopes)
				}
			}
		}
	})

	t.Run("Verify special scopes", func(t *testing.T) {
		list := []string{
			"read:branding", "update:branding", "delete:branding",
			"read:triggers", "update:triggers",
			"read:client_keys",
			"read:logs",
			"read:tenant_settings", "update:tenant_settings",
			"read:anomaly_blocks", "delete:anomaly_blocks",
			"read:attack_protection", "update:attack_protection",
			"read:prompts", "update:prompts",
			"read:stats",
			"read:insights",
			"create:user_tickets",
			"blacklist:tokens",
			"read:grants", "delete:grants",
			"read:mfa_policies", "update:mfa_policies",
			"read:guardian_factors", "update:guardian_factors",
			"read:guardian_enrollments", "delete:guardian_enrollments",
			"create:guardian_enrollment_tickets",
			"read:user_idp_tokens",
			"create:passwords_checking_job", "delete:passwords_checking_job",
			"read:limits", "update:limits",
			"read:entitlements",
		}

		for _, v := range list {
			if !strInArray(requiredScopes, v) {
				t.Fatalf("wanted scope: %q, list: %+v", v, requiredScopes)
			}
		}
	})
}

func strInArray(haystack []string, needle string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}

	return false
}
