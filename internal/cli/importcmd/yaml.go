package importcmd

import (
	"fmt"
	"io/ioutil"
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

func ParseYAML(yamlPath string, config *Config) (*TenantConfig, error) {

	yamlData, err := ioutil.ReadFile(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("error reading yaml file: %v ", err)
	}

	t := &TenantConfig{}

	err = yaml.Unmarshal(yamlData, t)
	if err != nil {
		return nil, fmt.Errorf("error Unmarshaling yaml: %v ", err)
	}

	for key, replacement := range config.Auth0KeywordReplaceMappings {
		t.replaceClientConfig(key, replacement)
		t.replaceResourceServersConfig(key, replacement)
		t.replaceRolesConfig(key, replacement)
	}

	return t, nil
}

func (t *TenantConfig) replaceClientConfig(key string, replacement interface{}) {
	for i, client := range t.Clients {

		if strings.ContainsAny(client.Name, key) {
			str := strings.ReplaceAll(client.Name, fmt.Sprintf("##%s##", key), replacement.(string))
			t.Clients[i].Name = str

		}

		for j, url := range client.AllowedLogoutUrls {
			if strings.ContainsAny(url, key) {
				str := strings.ReplaceAll(url, fmt.Sprintf("##%s##", key), replacement.(string))
				t.Clients[i].AllowedLogoutUrls[j] = str
			}
		}

		for j, url := range client.Callbacks {
			if strings.ContainsAny(url, key) {
				str := strings.ReplaceAll(url, fmt.Sprintf("##%s##", key), replacement.(string))
				t.Clients[i].Callbacks[j] = str
			}
		}

		for j, url := range client.WebOrigins {
			if strings.ContainsAny(url, key) {
				str := strings.ReplaceAll(url, fmt.Sprintf("##%s##", key), replacement.(string))
				t.Clients[i].WebOrigins[j] = str
			}
		}
	}
}

func (t *TenantConfig) replaceResourceServersConfig(key string, replacement interface{}) {
	for i, rs := range t.ResourceServers {
		if strings.ContainsAny(rs.Name, key) {
			str := strings.ReplaceAll(rs.Name, fmt.Sprintf("##%s##", key), replacement.(string))
			t.ResourceServers[i].Name = str
		}
	}
}

func (t *TenantConfig) replaceRolesConfig(key string, replacement interface{}) {
	for i, r := range t.Roles {
		for j, p := range r.Permissions {
			if strings.ContainsAny(p.ResourceServerIdentifier, key) {
				str := strings.ReplaceAll(p.ResourceServerIdentifier, fmt.Sprintf("##%s##", key), replacement.(string))
				t.Roles[i].Permissions[j].ResourceServerIdentifier = str
			}
		}
	}
}
