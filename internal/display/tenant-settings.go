package display

import (
	"github.com/auth0/go-auth0/management"
)

type tenantSettingsView struct {
	Tenant *management.Tenant
	raw    interface{}
}

func (v *tenantSettingsView) AsTableHeader() []string {
	return []string{"Setting", "Value"}
}

func (v *tenantSettingsView) AsTableRow() []string {
	return []string{} // Not used in list views.
}

func (v *tenantSettingsView) KeyValues() [][]string {
	var rows [][]string

	appendBool := func(name string, value *bool) {
		if value != nil {
			rows = append(rows, []string{name, boolean(*value)})
		}
	}

	appendBool("CustomizeMFAInPostLoginAction", v.Tenant.CustomizeMFAInPostLoginAction)
	appendBool("AllowOrgNameInAuthAPI", v.Tenant.AllowOrgNameInAuthAPI)
	appendBool("PushedAuthorizationRequestsSupported", v.Tenant.PushedAuthorizationRequestsSupported)

	if v.Tenant.OIDCLogout != nil {
		appendBool("OIDCLogout.OIDCResourceProviderLogoutEndSessionEndpointDiscovery", v.Tenant.OIDCLogout.OIDCResourceProviderLogoutEndSessionEndpointDiscovery)
	}

	if v.Tenant.MTLS != nil {
		appendBool("MTLS.EnableEndpointAliases", v.Tenant.MTLS.EnableEndpointAliases)
	}

	if v.Tenant.Flags != nil {
		flags := v.Tenant.Flags
		appendBool("Flags.EnableClientConnections", flags.EnableClientConnections)
		appendBool("Flags.EnableAPIsSection", flags.EnableAPIsSection)
		appendBool("Flags.EnablePipeline2", flags.EnablePipeline2)
		appendBool("Flags.EnableDynamicClientRegistration", flags.EnableDynamicClientRegistration)
		appendBool("Flags.EnableCustomDomainInEmails", flags.EnableCustomDomainInEmails)
		appendBool("Flags.EnableSSO", flags.EnableSSO)
		appendBool("Flags.AllowChangingEnableSSO", flags.AllowChangingEnableSSO)
		appendBool("Flags.UniversalLogin", flags.UniversalLogin)
		appendBool("Flags.EnableLegacyLogsSearchV2", flags.EnableLegacyLogsSearchV2)
		appendBool("Flags.DisableClickjackProtectionHeaders", flags.DisableClickjackProtectionHeaders)
		appendBool("Flags.EnablePublicSignupUserExistsError", flags.EnablePublicSignupUserExistsError)
		appendBool("Flags.UseScopeDescriptionsForConsent", flags.UseScopeDescriptionsForConsent)
		appendBool("Flags.AllowLegacyDelegationGrantTypes", flags.AllowLegacyDelegationGrantTypes)
		appendBool("Flags.AllowLegacyROGrantTypes", flags.AllowLegacyROGrantTypes)
		appendBool("Flags.AllowLegacyTokenInfoEndpoint", flags.AllowLegacyTokenInfoEndpoint)
		appendBool("Flags.EnableLegacyProfile", flags.EnableLegacyProfile)
		appendBool("Flags.EnableIDTokenAPI2", flags.EnableIDTokenAPI2)
		appendBool("Flags.NoDisclosureEnterpriseConnections", flags.NoDisclosureEnterpriseConnections)
		appendBool("Flags.DisableManagementAPISMSObfuscation", flags.DisableManagementAPISMSObfuscation)
		appendBool("Flags.EnableADFSWAADEmailVerification", flags.EnableADFSWAADEmailVerification)
		appendBool("Flags.RevokeRefreshTokenGrant", flags.RevokeRefreshTokenGrant)
		appendBool("Flags.DashboardLogStreams", flags.DashboardLogStreams)
		appendBool("Flags.DashboardInsightsView", flags.DashboardInsightsView)
		appendBool("Flags.DisableFieldsMapFix", flags.DisableFieldsMapFix)
		appendBool("Flags.MFAShowFactorListOnEnrollment", flags.MFAShowFactorListOnEnrollment)
		appendBool("Flags.RequirePushedAuthorizationRequests", flags.RequirePushedAuthorizationRequests)
		appendBool("Flags.RemoveAlgFromJWKS", flags.RemoveAlgFromJWKS)
	}

	return rows
}

func (v *tenantSettingsView) Object() interface{} {
	return v.raw
}

func (r *Renderer) TenantSettingsShow(tenant *management.Tenant) {
	r.Heading("tenant settings")
	r.Result(makeTenantSettingView(tenant))
}

func (r *Renderer) TenantSettingsUpdate(tenant *management.Tenant) {
	r.Heading("tenant settings updated")
	r.Result(makeTenantSettingView(tenant))
}

func makeTenantSettingView(tenant *management.Tenant) *tenantSettingsView {
	return &tenantSettingsView{
		Tenant: tenant,
		raw:    tenant,
	}
}
