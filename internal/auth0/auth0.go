package auth0

import (
	"github.com/auth0/go-auth0"
	"github.com/auth0/go-auth0/management"
)

// API mimics `management.Management`s general interface, except it refers to
// the interfaces instead of the concrete structs.
type API struct {
	Action               ActionAPI
	Anomaly              AnomalyAPI
	AttackProtection     AttackProtectionAPI
	Branding             BrandingAPI
	BrandingTheme        BrandingThemeAPI
	Client               ClientAPI
	ClientGrant          ClientGrantAPI
	Connection           ConnectionAPI
	CustomDomain         CustomDomainAPI
	EmailTemplate        EmailTemplateAPI
	EmailProvider        EmailProviderAPI
	EventStream          EventStreamAPI
	Flow                 FlowAPI
	FlowVaultConnection  FlowVaultConnectionAPI
	Form                 FormAPI
	Log                  LogAPI
	LogStream            LogStreamAPI
	Organization         OrganizationAPI
	NetworkACL           NetworkACLAPI
	Prompt               PromptAPI
	ResourceServer       ResourceServerAPI
	Role                 RoleAPI
	Rule                 RuleAPI
	Tenant               TenantAPI
	TokenExchange        TokenExchangeAPI
	User                 UserAPI
	Jobs                 JobsAPI
	SelfServiceProfile   SelfServiceProfileAPI
	UserAttributeProfile UserAttributeProfilesAPI

	HTTPClient HTTPClientAPI
}

func NewAPI(m *management.Management) *API {
	return &API{
		Action:               m.Action,
		Anomaly:              m.Anomaly,
		AttackProtection:     m.AttackProtection,
		Branding:             m.Branding,
		BrandingTheme:        m.BrandingTheme,
		Client:               m.Client,
		ClientGrant:          m.ClientGrant,
		Connection:           m.Connection,
		CustomDomain:         m.CustomDomain,
		EmailTemplate:        m.EmailTemplate,
		EmailProvider:        m.EmailProvider,
		EventStream:          m.EventStream,
		Flow:                 m.Flow,
		FlowVaultConnection:  m.Flow.Vault,
		Form:                 m.Form,
		Log:                  m.Log,
		LogStream:            m.LogStream,
		Organization:         m.Organization,
		NetworkACL:           m.NetworkACL,
		Prompt:               m.Prompt,
		ResourceServer:       m.ResourceServer,
		Role:                 m.Role,
		Rule:                 m.Rule,
		Tenant:               m.Tenant,
		TokenExchange:        m.TokenExchangeProfile,
		User:                 m.User,
		Jobs:                 m.Job,
		SelfServiceProfile:   m.SelfServiceProfile,
		UserAttributeProfile: m.UserAttributeProfile,
		HTTPClient:           m,
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
