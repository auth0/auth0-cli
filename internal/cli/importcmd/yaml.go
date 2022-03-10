package importcmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	"gopkg.in/yaml.v2"
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

func GetYAML(yamlPath string, config *Config) {

	yamlData, err := ioutil.ReadFile(yamlPath)
	if err != nil {
		log.Printf("error reading yaml #%v ", err)
	}

	t := &TenantConfig{}

	err = yaml.Unmarshal(yamlData, t)
	if err != nil {
		log.Fatalf("Unmarshal yaml file: %v ", err)
	}

	for key, replacement := range config.Auth0KeywordReplaceMappings {

		replacementInYAML := fmt.Sprintf("##%s##", key)

		fmt.Printf("key is: %s\n", key)
		fmt.Printf("replacement is: %s\n", replacement)

		for i, client := range t.Clients {

			if strings.ContainsAny(client.Name, replacementInYAML) {
				fmt.Printf("client.Name is: %s\n", client.Name)
				t.Clients[i].Name = replacement.(string)
				fmt.Printf("client.Name changed is: %s\n", client.Name)
			}

			for j, url := range client.AllowedLogoutUrls {
				if strings.ContainsAny(url, replacementInYAML) {
					t.Clients[i].AllowedLogoutUrls[j] = replacement.(string)
				}
			}
		}
	}

	j, _ := yaml.Marshal(&t)
	fmt.Printf("The yaml is:\n%+v", string(j))

}
