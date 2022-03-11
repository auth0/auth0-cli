package cli

import (
	"encoding/json"
	"io/ioutil"
)

type ImportConfig struct {
	Auth0KeywordReplaceMappings map[string]interface{} `json:"AUTH0_KEYWORD_REPLACE_MAPPINGS"`
	Auth0AllowDelete            bool                   `json:"AUTH0_ALLOW_DELETE,omitempty"`
}

func GetConfig(path string) (*ImportConfig, error) {
	file, error := ioutil.ReadFile(path)

	if error != nil {
		return nil, error
	}

	c := &ImportConfig{}
	error = json.Unmarshal([]byte(file), &c)

	if error != nil {
		return nil, error
	}

	return c, nil
}
