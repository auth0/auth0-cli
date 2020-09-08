package management

import (
	"context"
	"net/url"
	"time"
)

type Action struct {
	ID                string    `json:"id,omitempty"`
	Name              string    `json:"name,omitempty"`
	SupportedTriggers []Trigger `json:"supported_triggers,omitempty"`

	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`

	// TODO: add required configuration / secrets
}

type VersionStatus string

const (
	VersionStatusPending  VersionStatus = "pending"
	VersionStatusRetrying VersionStatus = "retrying"
	VersionStatusBuilding VersionStatus = "building"
	VersionStatusBuilt    VersionStatus = "built"

	// TODO(cyx): maybe get rid of this
	VersionStatusPromoted VersionStatus = "promoted"
)

type TriggerID string

const (
	PostLogin         TriggerID = "post-login"
	ClientCredentials TriggerID = "client-credentials"
)

type ActionVersion struct {
	ID           string        `json:"id,omitempty"`
	Action       *Action       `json:"action,omitempty"`
	Code         string        `json:"code"`
	Dependencies []Dependency  `json:"dependencies,omitempty"`
	Runtime      string        `json:"runtime,omitempty"`
	Status       VersionStatus `json:"status,omitempty"`
	Number       int           `json:"number,omitempty"`

	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`

	// TODO: maybe add errors?
}

type Dependency struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	RegistryURL string `json:"registry_url"`
}

type Trigger struct {
	ID      TriggerID `json:"id"`
	Version string    `json:"version"`
	URL     string    `json:"url"`
}

type ActionList struct {
	List
	Actions []*Action `json:"actions"`
}

type ActionVersionList struct {
	List
	Versions []*ActionVersion `json:"versions"`
}

type Object map[string]interface{}

type ActionManager struct {
	*Management
}

func (m *ActionManager) Create(a *Action) error {
	return m.post(m.uri("actions", "actions"), a)
}

func (m *ActionManager) Read(id string) (*Action, error) {
	var a Action
	err := m.get(m.uri("actions", "actions", id), &a)
	return &a, err
}

func (m *ActionManager) Update(id string, a *Action) error {
	// We'll get a 400 if we try to send the ID as part of the payload.
	a.ID = ""
	return m.patch(m.uri("actions", "actions", id), a)
}

func (m *ActionManager) Delete(id string) error {
	return m.delete(m.uri("actions", "actions", id))
}

func WithTriggerID(id TriggerID) ListOption {
	return func(v url.Values) {
		v.Set("triggerId", string(id))
	}
}

// TODO(cyx): do the standard m.q(opts) here supporting per_page, etc.
func (m *ActionManager) List(opts ...ListOption) (*ActionList, error) {
	var list ActionList
	err := m.get(m.uri("actions", "actions")+m.q(opts), &list)
	return &list, err
}

type ActionVersionManager struct {
	*Management
}

func (m *ActionVersionManager) Deploy(actionID string, v *ActionVersion) error {
	if err := m.post(m.uri("actions", "actions", actionID, "versions"), v); err != nil {
		return err
	}

	// Wait up to 1 minute for deploying an action version.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			got, err := m.Read(actionID, v.ID)
			if err != nil {
				return err
			}

			if got.Status == VersionStatusBuilt {
				if _, err := m.Promote(actionID, got.ID); err != nil {
					return err
				}

				// Refresh the final representation of this
				// action version.
				*v = *got
				return nil
			}
		}
	}
}

func (m *ActionVersionManager) Create(actionID string, v *ActionVersion) error {
	return m.post(m.uri("actions", "actions", actionID, "versions"), v)
}

// TODO(cyx): This isn't implemented yet.
func (m *ActionVersionManager) Update(actionID string, v *ActionVersion) error {
	return m.patch(m.uri("actions", "actions", actionID, "versions", "draft"), v)
}

func (m *ActionVersionManager) Read(actionID, id string) (*ActionVersion, error) {
	var v ActionVersion
	err := m.get(m.uri("actions", "actions", actionID, "versions", id), &v)
	return &v, err
}

func (m *ActionVersionManager) Delete(actionID, id string) error {
	return m.delete(m.uri("actions", "actions", actionID, "versions", id))
}

func (m *ActionVersionManager) List(actionID string, opts ...ListOption) (*ActionVersionList, error) {
	var list ActionVersionList
	opts = m.defaults(opts)
	err := m.get(m.uri("actions", "actions", actionID, "versions")+m.q(opts), &list)
	return &list, err
}

// TODO(cyx): might call this `activate` instead later. Still fleshing out the
// name.
func (m *ActionVersionManager) Promote(actionID, id string) (*ActionVersion, error) {
	var v ActionVersion
	err := m.post(m.uri("actions", "actions", actionID, "versions", id, "promote"), &v)
	return &v, err
}

// TODO(cyx): consider how the `draft` test looks like. Will it just use
// `draft` in place of the ID?
func (m *ActionVersionManager) Test(actionID, id string, payload Object) (Object, error) {
	v := Object{"payload": payload}
	err := m.post(m.uri("actions", "actions", actionID, "versions", id, "test"), &v)
	return v, err
}
