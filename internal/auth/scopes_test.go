package auth

import "testing"

func TestRequiredScopes(t *testing.T) {
	t.Run("Verify CRUD scopes", func(t *testing.T) {
		crudResources := []string{
			"clients",
			"client_grants",
			"log_streams",
			"resource_servers",
			"roles",
			"rules",
			"users",
			"custom_domains",
			"log_streams",
			"actions",
			"organizations",
		}
		crudPrefixes := []string{"create:", "delete:", "read:", "update:"}

		for _, resource := range crudResources {
			for _, prefix := range crudPrefixes {
				scope := prefix + resource

				if !strInArray(RequiredScopes, scope) {
					t.Fatalf("wanted scope: %q, list: %+v", scope, RequiredScopes)
				}
			}
		}
	})

	t.Run("Verify special scopes", func(t *testing.T) {
		list := []string{
			"read:branding", "update:branding",
			"read:flows", "create:flows", "update:flows", "delete:flows", "read:flows_executions",
			"read:flows_vault_connections", "create:flows_vault_connections", "update:flows_vault_connections", "delete:flows_vault_connections",
			"read:connections", "update:connections", "read:connections_options", "update:connections_options",
			"read:email_templates", "update:email_templates",
			"read:custom_domains", "create:custom_domains", "update:custom_domains", "delete:custom_domains",
			"read:client_keys", "read:logs", "read:tenant_settings",
			"read:anomaly_blocks", "delete:anomaly_blocks",
			"read:organization_members", "read:organization_member_roles", "read:organization_client_grants",
			"read:prompts", "update:prompts",
			"read:attack_protection", "update:attack_protection",
			"read:sessions", "update:sessions", "delete:sessions",
			"read:refresh_tokens", "update:refresh_tokens", "delete:refresh_tokens",
		}

		for _, v := range list {
			if !strInArray(RequiredScopes, v) {
				t.Fatalf("wanted scope: %q, list: %+v", v, RequiredScopes)
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
