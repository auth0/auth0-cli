package importcmd

import (
	"encoding/json"
	"io/ioutil"
)

type Config struct {
	Auth0Domain                 string                 `json:"AUTH0_DOMAIN"`
	Auth0KeywordReplaceMappings map[string]interface{} `json:"AUTH0_KEYWORD_REPLACE_MAPPINGS"`
	Auth0AllowDelete            bool                   `json:"AUTH0_ALLOW_DELETE"`
}

func GetConfig(path string) (*Config, error) {
	file, error := ioutil.ReadFile(path)

	if error != nil {
		return nil, error
	}

	c := &Config{}
	error = json.Unmarshal([]byte(file), &c)

	if error != nil {
		return nil, error
	}

	return c, nil
}
