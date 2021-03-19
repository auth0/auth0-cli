package auth

import "testing"

func TestRequiredScopes(t *testing.T) {
	t.Run("verify CRUD", func(t *testing.T) {
		crudResources := []string{
			"clients",
			"resource_servers",
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
			"read:client_keys", "read:logs",
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
