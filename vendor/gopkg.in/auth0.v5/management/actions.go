package management

import (
	"net/http"
	"time"
)

const (
	ActionTriggerPostLogin         string = "post-login"
	ActionTriggerClientCredentials string = "client-credentials"
)

type ActionTrigger struct {
	ID      *string `json:"id"`
	Version *string `json:"version"`
	Status  *string `json:"status,omitempty"`
}

type ActionTriggerList struct {
	Triggers []*ActionTrigger `json:"triggers"`
}

type ActionDependency struct {
	Name        *string `json:"name"`
	Version     *string `json:"version,omitempty"`
	RegistryURL *string `json:"registry_url,omitempty"`
}

type ActionSecret struct {
	Name      *string    `json:"name"`
	Value     *string    `json:"value,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

type ActionVersionError struct {
	ID      *string `json:"id"`
	Message *string `json:"msg"`
	Url     *string `json:"url"`
}

const (
	ActionStatusPending  string = "pending"
	ActionStatusBuilding string = "building"
	ActionStatusPackaged string = "packaged"
	ActionStatusBuilt    string = "built"
	ActionStatusRetrying string = "retrying"
	ActionStatusFailed   string = "failed"
)

type Action struct {
	ID   *string `json:"id,omitempty"`
	Name *string `json:"name"`
	Code *string `json:"code,omitempty"` // nil in embedded Action in ActionVersion

	SupportedTriggers []ActionTrigger    `json:"supported_triggers"`
	Dependencies      []ActionDependency `json:"dependencies,omitempty"`
	Secrets           []ActionSecret     `json:"secrets,omitempty"`

	DeployedVersion    *ActionVersion `json:"deployed_version,omitempty"`
	Status             *string        `json:"status,omitempty"`
	AllChangesDeployed bool           `json:"all_changes_deployed,omitempty"`

	BuiltAt   *time.Time `json:"built_at,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

type ActionList struct {
	List
	Actions []*Action `json:"actions"`
}

type ActionVersion struct {
	ID           *string            `json:"id,omitempty"`
	Code         *string            `json:"code"`
	Dependencies []ActionDependency `json:"dependencies,omitempty"`
	Deployed     bool               `json:"deployed"`
	Status       *string            `json:"status,omitempty"`
	Number       int                `json:"number,omitempty"`

	Errors []ActionVersionError `json:"errors,omitempty"`
	Action *Action              `json:"action,omitempty"`

	BuiltAt   *time.Time `json:"built_at,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

type ActionVersionList struct {
	List
	Versions []*ActionVersion `json:"versions"`
}

const (
	ActionBindingReferenceByName string = "action_name"
	ActionBindingReferenceById   string = "action_id"
)

type ActionBindingReference struct {
	Type  *string `json:"type"`
	Value *string `json:"value"`
}

type ActionBinding struct {
	ID          *string `json:"id,omitempty"`
	TriggerID   *string `json:"trigger_id,omitempty"`
	DisplayName *string `json:"display_name,omitempty"`

	Ref     *ActionBindingReference `json:"ref,omitempty"`
	Action  *Action                 `json:"action,omitempty"`
	Secrets []ActionSecret          `json:"secrets,omitempty"`

	CreatedAt *time.Time `json:"created_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

type ActionBindingList struct {
	List
	Bindings []*ActionBinding `json:"bindings"`
}

type actionBindingsPerTrigger struct {
	Bindings []*ActionBinding `json:"bindings"`
}

type ActionTestPayload map[string]interface{}

type ActionTestRequest struct {
	Payload *ActionTestPayload `json:"payload"`
}

type ActionExecutionResult struct {
	ActionName *string            `json:"action_name,omitempty"`
	Error      *map[string]string `json:"error,omitempty"`

	StartedAt *time.Time `json:"started_at,omitempty"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

type ActionExecution struct {
	ID        *string                  `json:"id"`
	TriggerID *string                  `json:"trigger_id"`
	Status    *string                  `json:"status"`
	Results   []*ActionExecutionResult `json:"results"`

	CreatedAt *time.Time `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
}

type ActionManager struct {
	*Management
}

func newActionManager(m *Management) *ActionManager {
	return &ActionManager{m}
}

func applyActionsListDefaults(options []RequestOption) RequestOption {
	return newRequestOption(func(r *http.Request) {
		PerPage(50).apply(r)
		for _, option := range options {
			option.apply(r)
		}
	})
}

// ListTriggers available.
//
// https://auth0.com/docs/api/management/v2/#!/Actions/get_triggers
func (m *ActionManager) ListTriggers(opts ...RequestOption) (l *ActionTriggerList, err error) {
	err = m.Request("GET", m.URI("actions", "triggers"), &l, opts...)
	return
}

// Create a new action.
//
// See: https://auth0.com/docs/api/management/v2#!/Actions/post_action
func (m *ActionManager) Create(a *Action, opts ...RequestOption) error {
	return m.Request("POST", m.URI("actions", "actions"), a, opts...)
}

// Retrieve action details.
//
// See: https://auth0.com/docs/api/management/v2#!/Actions/get_action
func (m *ActionManager) Read(id string, opts ...RequestOption) (a *Action, err error) {
	err = m.Request("GET", m.URI("actions", "actions", id), &a, opts...)
	return
}

// Update an existing action.
//
// See: https://auth0.com/docs/api/management/v2#!/Actions/patch_action
func (m *ActionManager) Update(id string, a *Action, opts ...RequestOption) error {
	return m.Request("PATCH", m.URI("actions", "actions", id), &a, opts...)
}

// Delete an action
//
// See: https://auth0.com/docs/api/management/v2#!/Actions/delete_action
func (m *ActionManager) Delete(id string, opts ...RequestOption) error {
	return m.Request("DELETE", m.URI("actions", "actions", id), nil, opts...)
}

// List all actions.
//
// See: https://auth0.com/docs/api/management/v2#!/Actions/get_actions
func (m *ActionManager) List(opts ...RequestOption) (l *ActionList, err error) {
	err = m.Request("GET", m.URI("actions", "actions"), &l, applyActionsListDefaults(opts))
	return
}

// ReadVersion of an action.
//
// See: https://auth0.com/docs/api/management/v2/#!/Actions/get_action_version
func (m *ActionManager) ReadVersion(id string, versionId string, opts ...RequestOption) (v *ActionVersion, err error) {
	err = m.Request("GET", m.URI("actions", "actions", id, "versions", versionId), &v, opts...)
	return
}

// ListVersions of an action.
//
// See: https://auth0.com/docs/api/management/v2/#!/Actions/get_action_versions
func (m *ActionManager) ListVersions(id string, opts ...RequestOption) (c *ActionVersionList, err error) {
	err = m.Request("GET", m.URI("actions", "actions", id, "versions"), &c, applyActionsListDefaults(opts))
	return
}

// UpdateBindings of a trigger
//
// See: https://auth0.com/docs/api/management/v2/#!/Actions/patch_bindings
func (m *ActionManager) UpdateBindings(triggerID string, b []*ActionBinding, opts ...RequestOption) error {
	bl := &actionBindingsPerTrigger{
		Bindings: b,
	}
	return m.Request("PATCH", m.URI("actions", "triggers", triggerID, "bindings"), &bl, opts...)
}

// ListBindings of a trigger
//
// See: https://auth0.com/docs/api/management/v2/#!/Actions/get_bindings
func (m *ActionManager) ListBindings(triggerID string, opts ...RequestOption) (bl *ActionBindingList, err error) {
	err = m.Request("GET", m.URI("actions", "triggers", triggerID, "bindings"), &bl, applyActionsListDefaults(opts))
	return
}

// Deploy an action
//
// See: https://auth0.com/docs/api/management/v2/#!/Actions/post_deploy_action
func (m *ActionManager) Deploy(id string, opts ...RequestOption) (v *ActionVersion, err error) {
	err = m.Request("POST", m.URI("actions", "actions", id, "deploy"), &v, opts...)
	return
}

// DeployVersion of an action
//
// See: https://auth0.com/docs/api/management/v2/#!/Actions/post_deploy_draft_version
func (m *ActionManager) DeployVersion(id string, versionId string, opts ...RequestOption) (v *ActionVersion, err error) {
	err = m.Request("POST", m.URI("actions", "actions", id, "versions", versionId, "deploy"), &v, opts...)
	return
}

// Test an action
//
// See: https://auth0.com/docs/api/management/v2/#!/Actions/post_test_action
func (m *ActionManager) Test(id string, payload *ActionTestPayload, opts ...RequestOption) (err error) {
	r := &ActionTestRequest{
		Payload: payload,
	}
	err = m.Request("POST", m.URI("actions", "actions", id, "test"), &r, opts...)
	return
}

// ReadExecution of an action
//
// See: https://auth0.com/docs/api/management/v2/#!/Actions/get_execution
func (m *ActionManager) ReadExecution(executionId string, opts ...RequestOption) (v *ActionExecution, err error) {
	err = m.Request("GET", m.URI("actions", "executions", executionId), &v, opts...)
	return
}
