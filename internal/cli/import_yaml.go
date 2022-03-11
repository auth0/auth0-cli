package cli

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/hashicorp/go-multierror"
	"gopkg.in/yaml.v2"
)

// Put here the logic for parsing the yaml file
type TenantConfig struct {
	Clients []struct {
		Name                           string   `yaml:"name"`
		AppType                        string   `yaml:"app_type"`
		Description                    string   `yaml:"description,omitempty"`
		TokenEndpointAuthMethod        string   `yaml:"token_endpoint_auth_method,omitempty"`
		AllowedLogoutURLs              []string `yaml:"allowed_logout_urls,omitempty"`
		Callbacks                      []string `yaml:"callbacks,omitempty"`
		WebOrigins                     []string `yaml:"web_origins,omitempty"`
		AllowedOrigins                 []string `yaml:"allowed_origins,omitempty"`
		GrantTypes                     []string `yaml:"grant_types,omitempty"`
		CrossOriginAuth                bool     `yaml:"cross_origin_auth,omitempty"`
		CustomLoginPageOn              bool     `yaml:"custom_login_page_on,omitempty"`
		IsFirstParty                   bool     `yaml:"is_first_party,omitempty"`
		IsTokenEndpointIPHeaderTrusted bool     `yaml:"is_token_endpoint_ip_header_trusted,omitempty"`
		OIDCConformant                 bool     `yaml:"oidc_conformant,omitempty"`
		SSODisabled                    bool     `yaml:"sso_disabled,omitempty"`
	} `yaml:"clients,omitempty"`
	ResourceServers []struct {
		Name               string `yaml:"name"`
		Identifier         string `yaml:"identifier"`
		AllowOfflineAccess bool   `yaml:"allow_offline_access,omitempty"`
		EnforcePolicies    bool   `yaml:"enforce_policies,omitempty"`
		Scopes             []struct {
			Value       string `yaml:"value"`
			Description string `yaml:"description"`
		} `yaml:"scopes,omitempty"`
		SigningAlg                                string `yaml:"signing_alg,omitempty"`
		SkipConsentForVerifiableFirstPartyClients bool   `yaml:"skip_consent_for_verifiable_first_party_clients,omitempty"`
		TokenDialect                              string `yaml:"token_dialect,omitempty"`
		TokenLifetime                             int    `yaml:"token_lifetime,omitempty"`
		TokenLifetimeForWeb                       int    `yaml:"token_lifetime_for_web,omitempty"`
		SigningSecret                             string `yaml:"signing_secret,omitempty"`
	} `yaml:"resourceServers,omitempty"`
	Roles []struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
		Permissions []struct {
			PermissionName           string `yaml:"permission_name"`
			ResourceServerIdentifier string `yaml:"resource_server_identifier"`
		} `yaml:"permissions,omitempty"`
	} `yaml:"roles,omitempty"`
}

func ParseYAML(yamlPath string, config *ImportConfig) (*TenantConfig, error) {
	yamlData, err := ioutil.ReadFile(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("error reading yaml file: %v ", err)
	}

	t := &TenantConfig{}

	err = yaml.Unmarshal(yamlData, t)
	if err != nil {
		return nil, fmt.Errorf("error Unmarshaling yaml: %v ", err)
	}

	if err = t.CheckForDuplicateNames(); err != nil {
		return nil, err
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

		if strings.ContainsAny(client.Description, key) {
			str := strings.ReplaceAll(client.Description, fmt.Sprintf("##%s##", key), replacement.(string))
			t.Clients[i].Description = str
		}

		if strings.ContainsAny(client.TokenEndpointAuthMethod, key) {
			str := strings.ReplaceAll(client.TokenEndpointAuthMethod, fmt.Sprintf("##%s##", key), replacement.(string))
			t.Clients[i].TokenEndpointAuthMethod = str
		}

		for j, item := range client.AllowedLogoutURLs {
			if strings.ContainsAny(item, key) {
				str := strings.ReplaceAll(item, fmt.Sprintf("##%s##", key), replacement.(string))
				t.Clients[i].AllowedLogoutURLs[j] = str
			}
		}

		for j, item := range client.Callbacks {
			if strings.ContainsAny(item, key) {
				str := strings.ReplaceAll(item, fmt.Sprintf("##%s##", key), replacement.(string))
				t.Clients[i].Callbacks[j] = str
			}
		}

		for j, item := range client.WebOrigins {
			if strings.ContainsAny(item, key) {
				str := strings.ReplaceAll(item, fmt.Sprintf("##%s##", key), replacement.(string))
				t.Clients[i].WebOrigins[j] = str
			}
		}

		for j, item := range client.AllowedOrigins {
			if strings.ContainsAny(item, key) {
				str := strings.ReplaceAll(item, fmt.Sprintf("##%s##", key), replacement.(string))
				t.Clients[i].AllowedOrigins[j] = str
			}
		}

		for j, item := range client.GrantTypes {
			if strings.ContainsAny(item, key) {
				str := strings.ReplaceAll(item, fmt.Sprintf("##%s##", key), replacement.(string))
				t.Clients[i].GrantTypes[j] = str
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

		if strings.ContainsAny(rs.Identifier, key) {
			str := strings.ReplaceAll(rs.Identifier, fmt.Sprintf("##%s##", key), replacement.(string))
			t.ResourceServers[i].Identifier = str
		}

		if strings.ContainsAny(rs.SigningAlg, key) {
			str := strings.ReplaceAll(rs.SigningAlg, fmt.Sprintf("##%s##", key), replacement.(string))
			t.ResourceServers[i].SigningAlg = str
		}

		if strings.ContainsAny(rs.TokenDialect, key) {
			str := strings.ReplaceAll(rs.TokenDialect, fmt.Sprintf("##%s##", key), replacement.(string))
			t.ResourceServers[i].TokenDialect = str
		}

		if strings.ContainsAny(rs.SigningSecret, key) {
			str := strings.ReplaceAll(rs.SigningSecret, fmt.Sprintf("##%s##", key), replacement.(string))
			t.ResourceServers[i].SigningSecret = str
		}

		for j, s := range rs.Scopes {
			if strings.ContainsAny(s.Description, key) {
				str := strings.ReplaceAll(s.Description, fmt.Sprintf("##%s##", key), replacement.(string))
				t.ResourceServers[i].Scopes[j].Description = str
			}
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

func (t *TenantConfig) CheckForDuplicateNames() error {
	var result *multierror.Error

	clientNames := make(map[string]bool)

	for _, c := range t.Clients {
		if clientNames[c.Name] {
			errMsg := fmt.Sprintf("found duplicate name in client: %s", c.Name)
			result = multierror.Append(result, errors.New(errMsg))
		}
		clientNames[c.Name] = true
	}

	resourceServerNames := make(map[string]bool)
	for _, rs := range t.ResourceServers {
		if resourceServerNames[rs.Name] {
			errMsg := fmt.Sprintf("found duplicate name in resourceServer: %s", rs.Name)
			result = multierror.Append(result, errors.New(errMsg))
		}
		resourceServerNames[rs.Name] = true
	}

	roleNames := make(map[string]bool)
	for _, r := range t.Roles {
		if roleNames[r.Name] {
			errMsg := fmt.Sprintf("found duplicate name in role: %s", r.Name)
			result = multierror.Append(result, errors.New(errMsg))
		}
		roleNames[r.Name] = true
	}

	return result.ErrorOrNil()
}
