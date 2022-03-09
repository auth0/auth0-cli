package cli

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

type Config struct {
	Auth0Domain                 string   `json:"AUTH0_DOMAIN"`
	Auth0KeywordReplaceMappings struct{} `json:"AUTH0_KEYWORD_REPLACE_MAPPINGS"`
}

func getConfig(path string) *Config {
	file, error := ioutil.ReadFile(path)

	if error != nil {
		log.Printf("Error reading config: #%v ", error)
	}

	c := &Config{}
	error = json.Unmarshal([]byte(file), &c)

	if error != nil {
		log.Printf("Error processing config structure: #%v ", error)
	}

	return c
}
