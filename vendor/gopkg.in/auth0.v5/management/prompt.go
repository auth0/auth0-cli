package management

type Prompt struct {
	// Which login experience to use. Can be `new` or `classic`.
	UniversalLoginExperience string `json:"universal_login_experience,omitempty"`

	// IdentifierFirst determines if the login screen prompts for just the identifier, identifier and password first.
	IdentifierFirst *bool `json:"identifier_first,omitempty"`
}

type PromptManager struct {
	*Management
}

func newPromptManager(m *Management) *PromptManager {
	return &PromptManager{m}
}

// Read retrieves prompts settings.
//
// See: https://auth0.com/docs/api/management/v2#!/Prompts/get_prompts
func (m *PromptManager) Read(opts ...RequestOption) (p *Prompt, err error) {
	err = m.Request("GET", m.URI("prompts"), &p, opts...)
	return
}

// Update prompts settings.
//
// See: https://auth0.com/docs/api/management/v2#!/Prompts/patch_prompts
func (m *PromptManager) Update(p *Prompt, opts ...RequestOption) error {
	return m.Request("PATCH", m.URI("prompts"), p, opts...)
}
