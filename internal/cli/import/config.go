package cli

// Put here the logic for parsing the JSON config file

type Config struct {
	Auth0Domain                 string   `json:"AUTH0_DOMAIN"`
	Auth0KeywordReplaceMappings struct{} `json:"AUTH0_KEYWORD_REPLACE_MAPPINGS"`
}
