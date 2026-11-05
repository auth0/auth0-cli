package portals

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

// Page is a page of a portal.
type Page struct {
	Title      string      `json:"title"`
	Slug       string      `json:"slug"`
	Components []Component `json:"components,omitempty"`
}

// Component is a block of a page. The config depends on the type, and a section holds its
// blocks in config.children.
type Component struct {
	Type   string         `json:"type"`
	Config map[string]any `json:"config,omitempty"`
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
