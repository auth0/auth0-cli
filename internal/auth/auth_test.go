package auth

import "testing"

func TestRequiredScopes(t *testing.T) {
	t.Run("verify CRUD", func(t *testing.T) {
		crudResources := []string{
			"clients",
			"log_streams",
			"resource_servers",
			"roles",
			"rules",
			"users",
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

	t.Run("verify special scopes", func(t *testing.T) {
		list := []string{
			"read:branding", "update:branding",
			"read:connections", "update:connections",
			"read:custom_domains", "create:custom_domains", "update:custom_domains", "delete:custom_domains",
			"read:client_keys", "read:logs", "read:tenant_settings",
			"read:anomaly_blocks", "delete:anomaly_blocks",
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
