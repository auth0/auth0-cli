package auth0

import (
	"github.com/auth0/go-auth0"
	"github.com/auth0/go-auth0/management"
	managementv3 "github.com/auth0/go-auth0/v3/management/client"
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

type APIV3 struct {
	AttackProtectionBotDetection AttackProtectionBotDetectionAPIV3
	ClientGrant                  ClientGrantAPIV3
	ClientGrantOrganization      ClientGrantOrganizationAPIV3
	Events                       EventsAPIV3
	PhoneNotificationTemplate    PhoneNotificationTemplateAPI
	Session                      SessionAPIV3
	RefreshToken                 RefreshTokenAPIV3
	UserSession                  UserSessionAPIV3
	UserRefreshToken             UserRefreshTokenAPIV3
	ActionModule                 ActionModuleAPIV3
	ActionModuleVersion          ActionModuleVersionAPIV3
	NetworkACLKey                NetworkACLKeyAPIV3
}

func NewAPIV3(m *managementv3.Management) *APIV3 {
	return &APIV3{
		AttackProtectionBotDetection: m.AttackProtection.BotDetection,
		ClientGrant:                  m.ClientGrants,
		ClientGrantOrganization:      m.ClientGrants.Organizations,
		Events:                       m.Events,
		PhoneNotificationTemplate:    m.Branding.Phone.Templates,
		Session:                      m.Sessions,
		RefreshToken:                 m.RefreshTokens,
		UserSession:                  m.Users.Sessions,
		UserRefreshToken:             m.Users.RefreshToken,
		ActionModule:                 m.Actions.Modules,
		ActionModuleVersion:          m.Actions.Modules.Versions,
		NetworkACLKey:                m.Keys.NetworkACLs,
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
