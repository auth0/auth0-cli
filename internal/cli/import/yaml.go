package cli

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

// Put here the logic for parsing the yaml file
type TenantConfig struct {
	Clients []struct {
		Name              string   `yaml:"name"`
		AppType           string   `yaml:"app_type"`
		AllowedLogoutUrls []string `yaml:"allowed_logout_urls,omitempty"`
		Callbacks         []string `yaml:"callbacks,omitempty"`
		WebOrigins        []string `yaml:"web_origins,omitempty"`
	} `yaml:"clients"`
	ResourceServers []struct {
		Name               string `yaml:"name"`
		Identifier         string `yaml:"identifier"`
		AllowOfflineAccess bool   `yaml:"allow_offline_access"`
		EnforcePolicies    bool   `yaml:"enforce_policies"`
		Scopes             []struct {
			Value       string `yaml:"value"`
			Description string `yaml:"description"`
		} `yaml:"scopes"`
		SigningAlg                                string `yaml:"signing_alg"`
		SkipConsentForVerifiableFirstPartyClients bool   `yaml:"skip_consent_for_verifiable_first_party_clients"`
		TokenDialect                              string `yaml:"token_dialect"`
		TokenLifetime                             int    `yaml:"token_lifetime"`
		TokenLifetimeForWeb                       int    `yaml:"token_lifetime_for_web"`
		SigningSecret                             string `yaml:"signing_secret,omitempty"`
	} `yaml:"resourceServers"`
	Roles []struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
		Permissions []struct {
			PermissionName           string `yaml:"permission_name"`
			ResourceServerIdentifier string `yaml:"resource_server_identifier"`
		} `yaml:"permissions"`
	} `yaml:"roles"`
}

func (t *TenantConfig) getYAML(yaml string, config string) *TenantConfig {

	// Unmarshall yaml

	// yamlFile, err := ioutil.ReadFile(yaml)
	// if err != nil {
	// 	log.Printf("error reading yaml #%v ", err)
	// }

	// // err = yaml.Unmarshal(yamlFile, t)
	// // if err != nil {
	// // 	log.Fatalf("Unmarshal yaml file: %v", err)
	// // }

	// Unmarshall config json

	configFile, err := ioutil.ReadFile(config)
	if err != nil {
		log.Printf("error reading config json #%v ", err)
	}

	c := &Config{}

	err = json.Unmarshal(configFile, c)
	if err != nil {
		log.Fatalf("Unmarshal config file: %v", err)
	}

	// need to replace updated values from the specified keys in the config file
	// into the UnMarshal'd yaml struct
	return t
}
