package auth0

import (
	"gopkg.in/auth0.v5"
	"gopkg.in/auth0.v5/management"
)

// API mimics `management.Management`s general interface, except it refers to
// the interfaces instead of the concrete structs.
type API struct {
	Action         ActionAPI
	Anomaly        AnomalyAPI
	Branding       BrandingAPI
	Client         ClientAPI
	Connection     ConnectionAPI
	CustomDomain   CustomDomainAPI
	EmailTemplate  EmailTemplateAPI
	Log            LogAPI
	LogStream      LogStreamAPI
	Organization   OrganizationAPI
	ResourceServer ResourceServerAPI
	Role           RoleAPI
	Rule           RuleAPI
	Tenant         TenantAPI
	User           UserAPI
}

func NewAPI(m *management.Management) *API {
	return &API{
		Action:         m.Action,
		Anomaly:        m.Anomaly,
		Branding:       m.Branding,
		Client:         m.Client,
		Connection:     m.Connection,
		CustomDomain:   m.CustomDomain,
		EmailTemplate:  m.EmailTemplate,
		Log:            m.Log,
		LogStream:      m.LogStream,
		Organization:   m.Organization,
		ResourceServer: m.ResourceServer,
		Role:           m.Role,
		Rule:           m.Rule,
		Tenant:         m.Tenant,
		User:           m.User,
	}
}

// Alias all the helper methods so we can keep just typing `auth0.Bool` and the
// compiler can autocomplete our internal package.
var (
	Bool         = auth0.Bool
	BoolValue    = auth0.BoolValue
	String       = auth0.String
	StringValue  = auth0.StringValue
	Int          = auth0.Int
	IntValue     = auth0.IntValue
	Float64      = auth0.Float64
	Float64Value = auth0.Float64Value
	Time         = auth0.Time
	TimeValue    = auth0.TimeValue
)
