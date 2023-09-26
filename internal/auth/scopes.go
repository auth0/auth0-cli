package auth

var RequiredScopes = []string{
	"openid",
	"offline_access", // for retrieving refresh token
	"create:clients", "delete:clients", "read:clients", "update:clients",
	"read:client_grants",
	"create:resource_servers", "delete:resource_servers", "read:resource_servers", "update:resource_servers",
	"create:roles", "delete:roles", "read:roles", "update:roles",
	"create:rules", "delete:rules", "read:rules", "update:rules",
	"create:users", "delete:users", "read:users", "update:users",
	"read:branding", "update:branding",
	"read:email_templates", "update:email_templates",
	"read:email_provider",
	"read:connections", "update:connections",
	"read:client_keys", "read:logs", "read:tenant_settings",
	"read:custom_domains", "create:custom_domains", "update:custom_domains", "delete:custom_domains",
	"read:anomaly_blocks", "delete:anomaly_blocks",
	"create:log_streams", "delete:log_streams", "read:log_streams", "update:log_streams",
	"create:actions", "delete:actions", "read:actions", "update:actions",
	"create:organizations", "delete:organizations", "read:organizations", "update:organizations", "read:organization_members", "read:organization_member_roles", "read:organization_connections",
	"read:prompts", "update:prompts",
	"read:attack_protection", "update:attack_protection",
}

// RequiredScopesForClientCreds returns minimum scopes required when authenticating with client credentials.
func RequiredScopesForClientCreds() []string {
	var min []string
	for _, s := range RequiredScopes {
		// Both "offline_access" and "openid" scopes only apply to device-flow authentication
		// and should be ignored when authenticating with client credentials
		if s != "offline_access" && s != "openid" {
			min = append(min, s)
		}
	}
	return min
}
