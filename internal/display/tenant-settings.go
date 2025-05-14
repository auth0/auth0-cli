package display

import (
	"github.com/auth0/go-auth0"
	"github.com/auth0/go-auth0/management"
)

var SupportedTenantSettings = map[string]string{
	"EnableClientConnections":                        "flags.enable_client_connections",
	"EnableAPIsSection":                              "flags.enable_apis_section",
	"EnablePipeline2":                                "flags.enable_pipeline2",
	"EnableDynamicClientRegistration":                "flags.enable_dynamic_client_registration",
	"EnableCustomDomainInEmails":                     "flags.enable_custom_domain_in_emails",
	"EnableSSO":                                      "flags.enable_sso",
	"AllowChangingEnableSSO":                         "flags.allow_changing_enable_sso",
	"UniversalLogin":                                 "flags.universal_login",
	"EnableLegacyLogsSearchV2":                       "flags.enable_legacy_logs_search_v2",
	"DisableClickjackProtectionHeaders":              "flags.disable_clickjack_protection_headers",
	"EnablePublicSignupUserExistsError":              "flags.enable_public_signup_user_exists_error",
	"UseScopeDescriptionsForConsent":                 "flags.use_scope_descriptions_for_consent",
	"AllowLegacyDelegationGrantTypes":                "flags.allow_legacy_delegation_grant_types",
	"AllowLegacyROGrantTypes":                        "flags.allow_legacy_ro_grant_types",
	"AllowLegacyTokenInfoEndpoint":                   "flags.allow_legacy_tokeninfo_endpoint",
	"EnableLegacyProfile":                            "flags.enable_legacy_profile",
	"EnableIDTokenAPI2":                              "flags.enable_idtoken_api2",
	"NoDisclosureEnterpriseConnections":              "flags.no_disclose_enterprise_connections",
	"DisableManagementAPISMSObfuscation":             "flags.disable_management_api_sms_obfuscation",
	"EnableADFSWAADEmailVerification":                "flags.enable_adfs_waad_email_verification",
	"RevokeRefreshTokenGrant":                        "flags.revoke_refresh_token_grant",
	"DashboardLogStreams":                            "flags.dashboard_log_streams_next",
	"DashboardInsightsView":                          "flags.dashboard_insights_view",
	"DisableFieldsMapFix":                            "flags.disable_fields_map_fix",
	"MFAShowFactorListOnEnrollment":                  "flags.mfa_show_factor_list_on_enrollment",
	"RequirePushedAuthorizationRequests":             "flags.require_pushed_authorization_requests",
	"RemoveAlgFromJWKS":                              "flags.remove_alg_from_jwks",
	"CustomizeMFAInPostLoginAction":                  "customize_mfa_in_postlogin_action",
	"AllowOrgNameInAuthAPI":                          "allow_organization_name_in_authentication_api",
	"PushedAuthorizationRequestsSupported":           "pushed_authorization_requests_supported",
	"OIDCLogout.RPLogoutEndSessionEndpointDiscovery": "oidc_logout.rp_logout_end_session_endpoint_discovery",
	"MTLS.EnableEndpointAliases":                     "mtls.enable_endpoint_aliases",
}

type TenantSettingsView struct {
	SettingName      string
	FriendlyFlagName string
	Enabled          *bool
	raw              interface{}
}

func (v *TenantSettingsView) AsTableHeader() []string {
	return []string{"Setting-Name", "Friendly-Name", "Enabled"}
}

func (v *TenantSettingsView) AsTableRow() []string {
	return []string{v.SettingName, v.FriendlyFlagName, boolean(auth0.BoolValue(v.Enabled))} // Not used in list views.
}

func (v *TenantSettingsView) KeyValues() [][]string {
	return [][]string{}
}

func (v *TenantSettingsView) Object() interface{} {
	return v.raw
}

func (r *Renderer) TenantSettingsShow(tenant *management.Tenant) {
	r.Heading("tenant settings")
	r.Results(makeTenantSettings(tenant))
}

func (r *Renderer) TenantSettingsUpdate(tenant *management.Tenant) {
	r.Heading("tenant settings updated")
	r.Results(makeTenantSettings(tenant))
}

func makeTenantSettings(tenant *management.Tenant) []View {
	views := make([]View, 0, len(SupportedTenantSettings))

	addSetting := func(settingName string, friendlyFlagName string, enabled *bool) {
		views = append(views, &TenantSettingsView{
			SettingName:      settingName,
			FriendlyFlagName: friendlyFlagName,
			Enabled:          enabled,
		})
	}

	// Iterate over the supportedFlags map and create a TenantSettingView for each flag.
	for settingName, friendlyFlagName := range SupportedTenantSettings {
		switch settingName {
		case "EnableClientConnections":
			addSetting(settingName, friendlyFlagName, tenant.Flags.EnableClientConnections)
		case "EnableAPIsSection":
			addSetting(settingName, friendlyFlagName, tenant.Flags.EnableAPIsSection)
		case "EnablePipeline2":
			addSetting(settingName, friendlyFlagName, tenant.Flags.EnablePipeline2)
		case "EnableDynamicClientRegistration":
			addSetting(settingName, friendlyFlagName, tenant.Flags.EnableDynamicClientRegistration)
		case "EnableCustomDomainInEmails":
			addSetting(settingName, friendlyFlagName, tenant.Flags.EnableCustomDomainInEmails)
		case "EnableSSO":
			addSetting(settingName, friendlyFlagName, tenant.Flags.EnableSSO)
		case "AllowChangingEnableSSO":
			addSetting(settingName, friendlyFlagName, tenant.Flags.AllowChangingEnableSSO)
		case "UniversalLogin":
			addSetting(settingName, friendlyFlagName, tenant.Flags.UniversalLogin)
		case "EnableLegacyLogsSearchV2":
			addSetting(settingName, friendlyFlagName, tenant.Flags.EnableLegacyLogsSearchV2)
		case "DisableClickjackProtectionHeaders":
			addSetting(settingName, friendlyFlagName, tenant.Flags.DisableClickjackProtectionHeaders)
		case "EnablePublicSignupUserExistsError":
			addSetting(settingName, friendlyFlagName, tenant.Flags.EnablePublicSignupUserExistsError)
		case "UseScopeDescriptionsForConsent":
			addSetting(settingName, friendlyFlagName, tenant.Flags.UseScopeDescriptionsForConsent)
		case "AllowLegacyDelegationGrantTypes":
			addSetting(settingName, friendlyFlagName, tenant.Flags.AllowLegacyDelegationGrantTypes)
		case "AllowLegacyROGrantTypes":
			addSetting(settingName, friendlyFlagName, tenant.Flags.AllowLegacyROGrantTypes)
		case "AllowLegacyTokenInfoEndpoint":
			addSetting(settingName, friendlyFlagName, tenant.Flags.AllowLegacyTokenInfoEndpoint)
		case "EnableLegacyProfile":
			addSetting(settingName, friendlyFlagName, tenant.Flags.EnableLegacyProfile)
		case "EnableIDTokenAPI2":
			addSetting(settingName, friendlyFlagName, tenant.Flags.EnableIDTokenAPI2)
		case "NoDisclosureEnterpriseConnections":
			addSetting(settingName, friendlyFlagName, tenant.Flags.NoDisclosureEnterpriseConnections)
		case "DisableManagementAPISMSObfuscation":
			addSetting(settingName, friendlyFlagName, tenant.Flags.DisableManagementAPISMSObfuscation)
		case "EnableADFSWAADEmailVerification":
			addSetting(settingName, friendlyFlagName, tenant.Flags.EnableADFSWAADEmailVerification)
		case "RevokeRefreshTokenGrant":
			addSetting(settingName, friendlyFlagName, tenant.Flags.RevokeRefreshTokenGrant)
		case "DashboardLogStreams":
			addSetting(settingName, friendlyFlagName, tenant.Flags.DashboardLogStreams)
		case "DashboardInsightsView":
			addSetting(settingName, friendlyFlagName, tenant.Flags.DashboardInsightsView)
		case "DisableFieldsMapFix":
			addSetting(settingName, friendlyFlagName, tenant.Flags.DisableFieldsMapFix)
		case "MFAShowFactorListOnEnrollment":
			addSetting(settingName, friendlyFlagName, tenant.Flags.MFAShowFactorListOnEnrollment)
		case "RequirePushedAuthorizationRequests":
			addSetting(settingName, friendlyFlagName, tenant.Flags.RequirePushedAuthorizationRequests)
		case "RemoveAlgFromJWKS":
			addSetting(settingName, friendlyFlagName, tenant.Flags.RemoveAlgFromJWKS)
		case "CustomizeMFAInPostLoginAction":
			addSetting(settingName, friendlyFlagName, tenant.CustomizeMFAInPostLoginAction)
		case "AllowOrgNameInAuthAPI":
			addSetting(settingName, friendlyFlagName, tenant.AllowOrgNameInAuthAPI)
		case "PushedAuthorizationRequestsSupported":
			addSetting(settingName, friendlyFlagName, tenant.PushedAuthorizationRequestsSupported)
		case "OIDCLogout.RPLogoutEndSessionEndpointDiscovery":
			if tenant.OIDCLogout != nil {
				addSetting(settingName, friendlyFlagName, tenant.OIDCLogout.OIDCResourceProviderLogoutEndSessionEndpointDiscovery)
			}
		case "MTLS.EnableEndpointAliases":
			if tenant.MTLS != nil {
				addSetting(settingName, friendlyFlagName, tenant.MTLS.EnableEndpointAliases)
			}

			return views
		}
	}

	return views
}
