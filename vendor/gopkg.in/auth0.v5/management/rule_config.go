package management

type RuleConfig struct {

	// The key for a RuleConfigs config
	Key *string `json:"key,omitempty"`

	// The value for the rules config
	Value *string `json:"value,omitempty"`
}

type RuleConfigManager struct {
	*Management
}

func newRuleConfigManager(m *Management) *RuleConfigManager {
	return &RuleConfigManager{m}
}

// Upsert sets a rule configuration variable.
//
// See: https://auth0.com/docs/api/management/v2#!/Rules_Configs/put_rules_configs_by_key
func (m *RuleConfigManager) Upsert(key string, r *RuleConfig, opts ...RequestOption) (err error) {
	return m.Request("PUT", m.URI("rules-configs", key), r, opts...)
}

// Read a rule configuration variable by key.
//
// Note: For security, config variable values cannot be retrieved outside rule
// execution.
//
// See: https://auth0.com/docs/api/management/v2#!/Rules_Configs/get_rules_configs
func (m *RuleConfigManager) Read(key string, opts ...RequestOption) (*RuleConfig, error) {
	rs, err := m.List(opts...)
	if err != nil {
		return nil, err
	}
	for _, r := range rs {
		if r.GetKey() == key {
			return r, nil
		}
	}
	return nil, &managementError{404, "Not Found", "Rule config not found"}
}

// Delete a rule configuration variable identified by its key.
//
// See: https://auth0.com/docs/api/management/v2#!/Rules_Configs/delete_rules_configs_by_key
func (m *RuleConfigManager) Delete(key string, opts ...RequestOption) (err error) {
	return m.Request("DELETE", m.URI("rules-configs", key), nil, opts...)
}

// List all rule configuration variables.
//
// See: https://auth0.com/docs/api/management/v2#!/Rules_Configs/get_rules_configs
func (m *RuleConfigManager) List(opts ...RequestOption) (r []*RuleConfig, err error) {
	err = m.Request("GET", m.URI("rules-configs"), &r, applyListDefaults(opts))
	return
}
