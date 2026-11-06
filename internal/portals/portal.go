package portals

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
)

// PortalSummary is a portal in the list of the tenant.
type PortalSummary struct {
	ID   string
	Slug string
	Name string
}

// Portal is a portal as the Management API returns it.
type Portal struct {
	ID         string        `json:"id"`
	Slug       string        `json:"slug"`
	Name       string        `json:"name,omitempty"`
	Client     *PortalClient `json:"client,omitempty"`
	Navigation *Navigation   `json:"navigation,omitempty"`
	Pages      *Pages        `json:"pages,omitempty"`
	CustomCode *CustomCode   `json:"custom_code,omitempty"`
}

// PortalClient is the application a portal signs users in with.
type PortalClient struct {
	ClientID                string `json:"client_id"`
	ClientSecret            string `json:"client_secret,omitempty"`
	TokenEndpointAuthMethod string `json:"token_endpoint_auth_method,omitempty"`
}

// Navigation is the navigation of a portal.
type Navigation struct {
	Sidebar *Sidebar `json:"sidebar,omitempty"`
}

// Sidebar is the sidebar of a portal.
type Sidebar struct {
	Components []SidebarComponent `json:"components,omitempty"`
}

// SidebarComponent is a link in the sidebar. The config depends on the type.
type SidebarComponent struct {
	Type   string         `json:"type"`
	Config map[string]any `json:"config,omitempty"`
}

// Pages are the pages of a portal.
type Pages struct {
	// Default is the slug of the home page.
	Default  string `json:"default,omitempty"`
	Standard []Page `json:"standard,omitempty"`
}

// Page is a page of a portal. A fix sends the pages back whole, so the fields the package does not
// know are kept and sent as they were.
type Page struct {
	Title      string
	Slug       string
	Components []Component

	extra map[string]json.RawMessage
}

// Component is a block of a page. The config depends on the type, and a section holds its
// blocks in config.children. The fields the package does not know are kept.
type Component struct {
	Type   string
	Config map[string]any

	extra map[string]json.RawMessage
}

// UnmarshalJSON implements json.Unmarshaler.
func (p *Page) UnmarshalJSON(data []byte) error {
	known, extra, err := splitFields(data, "title", "slug", "components")
	if err != nil {
		return err
	}

	*p = Page{extra: extra}

	return decodeFields(known, map[string]any{"title": &p.Title, "slug": &p.Slug, "components": &p.Components})
}

// MarshalJSON implements json.Marshaler.
func (p Page) MarshalJSON() ([]byte, error) {
	fields := map[string]any{"title": p.Title, "slug": p.Slug}
	if p.Components != nil {
		fields["components"] = p.Components
	}

	return joinFields(fields, p.extra)
}

// UnmarshalJSON implements json.Unmarshaler.
func (c *Component) UnmarshalJSON(data []byte) error {
	known, extra, err := splitFields(data, "type", "config")
	if err != nil {
		return err
	}

	*c = Component{extra: extra}

	return decodeFields(known, map[string]any{"type": &c.Type, "config": &c.Config})
}

// MarshalJSON implements json.Marshaler.
func (c Component) MarshalJSON() ([]byte, error) {
	fields := map[string]any{"type": c.Type}
	if c.Config != nil {
		fields["config"] = c.Config
	}

	return joinFields(fields, c.extra)
}

// splitFields separates the fields of an object that the package knows from the others.
func splitFields(data []byte, names ...string) (known, extra map[string]json.RawMessage, err error) {
	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, nil, err
	}

	known = map[string]json.RawMessage{}

	for _, name := range names {
		if value, found := all[name]; found {
			known[name] = value

			delete(all, name)
		}
	}

	if len(all) == 0 {
		all = nil
	}

	return known, all, nil
}

func decodeFields(fields map[string]json.RawMessage, targets map[string]any) error {
	// The fields are read in the order of their names, so the error is always the same one.
	for _, name := range slices.Sorted(maps.Keys(targets)) {
		if value, found := fields[name]; found {
			if err := json.Unmarshal(value, targets[name]); err != nil {
				return fmt.Errorf("read the %q of the portal: %w", name, err)
			}
		}
	}

	return nil
}

// joinFields encodes the fields of the package together with the ones it kept.
func joinFields(fields map[string]any, extra map[string]json.RawMessage) ([]byte, error) {
	merged := make(map[string]any, len(fields)+len(extra))

	for name, value := range extra {
		merged[name] = value
	}

	for name, value := range fields {
		merged[name] = value
	}

	return json.Marshal(merged)
}

// CustomCode is the custom code of a portal.
type CustomCode struct {
	Head string `json:"head,omitempty"`
}

// CreatePortalRequest describes a portal to create.
type CreatePortalRequest struct {
	Slug       string        `json:"slug"`
	Name       string        `json:"name"`
	Client     *PortalClient `json:"client"`
	Navigation *Navigation   `json:"navigation,omitempty"`
	Pages      *Pages        `json:"pages,omitempty"`
}

// PortalPatch changes the parts of a portal that are set, and keeps the others.
type PortalPatch struct {
	Client     *PortalClient `json:"client,omitempty"`
	Navigation *Navigation   `json:"navigation,omitempty"`
	Pages      *Pages        `json:"pages,omitempty"`
}
