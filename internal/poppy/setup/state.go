// Package setup reads the tenant for the Poppy setup and config commands,
// validates their inputs, plans the changes that setup makes and derives the
// values of the config file. It makes no Management API writes and has no
// dependency on Cobra.
package setup

import (
	"cmp"

	"github.com/auth0/auth0-cli/internal/poppy/config"
)

// Tenant holds the tenant settings that Poppy cares about.
type Tenant struct {
	// CIMD tells whether agents identified by URL (client ID metadata
	// documents) are allowed.
	CIMD bool
	// IssParameter tells whether authorization responses carry the iss parameter.
	IssParameter             bool
	ResourceParameterProfile string
	DefaultAudience          string
	FriendlyName             string
	// CompanyDomains holds the entries as the tenant has them. They should be bare
	// hosts such as shop.example, but an https://shop.example origin may be there.
	CompanyDomains []string
}

// Scope is a scope of an API.
type Scope struct {
	Name        string
	Description string
}

// API is a resource server with the settings that Poppy changes.
type API struct {
	ID         string
	Identifier string
	// Name is the name of the API in the tenant. The config file uses it as the
	// description of a new apis entry.
	Name               string
	Scopes             []Scope
	AllowOfflineAccess bool
	AgentSubjectClaims string
	// Poppy tells whether the API has the Personal Agent Protocol on.
	Poppy bool
	// GuestPolicy is the policy for the anonymous_user subject type.
	GuestPolicy string
	// UserPolicy and ClientPolicy are the policies for the user and client
	// subject types, empty when the API has none. Updating the anonymous_user
	// policy sends them back unchanged, so that the update cannot reset them.
	UserPolicy   string
	ClientPolicy string
}

// App is the web app used for the browser join.
type App struct {
	ClientID    string
	Name        string
	FirstParty  bool
	LoginURI    string
	BrowserJoin bool
}

// State is what the tenant holds today, as far as setup and config care.
type State struct {
	Tenant Tenant
	// Domain is the canonical domain of the tenant.
	Domain string
	// CustomDomain is the default verified custom domain, empty when there is none.
	CustomDomain string
	// APIs are the requested APIs, in the order they were requested.
	APIs []API
	App  App
	// File is the existing config file, nil when there is none. The caller
	// fills it.
	File *config.Config
}

// Issuer returns the issuer: the default verified custom domain, else the
// canonical domain of the tenant.
func (s *State) Issuer() string {
	return "https://" + cmp.Or(s.CustomDomain, s.Domain) + "/"
}

// usesCanonicalDomain tells whether the issuer is the Auth0 domain of the tenant.
func (s *State) usesCanonicalDomain() bool {
	return s.CustomDomain == ""
}

// fileEntries returns the indexes of the apis entries of the existing config
// file that belong to an API, which are those at the origin of its identifier.
func (s *State) fileEntries(identifier string) []int {
	if s.File == nil {
		return nil
	}

	return s.File.EntriesFor(identifier)
}
