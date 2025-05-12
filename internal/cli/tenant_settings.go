package cli

import (
	"fmt"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/auth0/go-auth0/management"

	"github.com/spf13/cobra"
)

var (
	supportedFlags = []string{
		"EnableClientConnections",
		"EnableAPIsSection",
		"EnablePipeline2",
		"EnableDynamicClientRegistration",
		"EnableCustomDomainInEmails",
		"EnableSSO",
		"AllowChangingEnableSSO",
		"UniversalLogin",
		"EnableLegacyLogsSearchV2",
		"DisableClickjackProtectionHeaders",
		"EnablePublicSignupUserExistsError",
		"UseScopeDescriptionsForConsent",
		"AllowLegacyDelegationGrantTypes",
		"AllowLegacyROGrantTypes",
		"AllowLegacyTokenInfoEndpoint",
		"EnableLegacyProfile",
		"EnableIDTokenAPI2",
		"NoDisclosureEnterpriseConnections",
		"DisableManagementAPISMSObfuscation",
		"EnableADFSWAADEmailVerification",
		"RevokeRefreshTokenGrant",
		"DashboardLogStreams",
		"DashboardInsightsView",
		"DisableFieldsMapFix",
		"MFAShowFactorListOnEnrollment",
		"RequirePushedAuthorizationRequests",
		"RemoveAlgFromJWKS",
		"CustomizeMFAInPostLoginAction",
		"AllowOrgNameInAuthAPI",
		"PushedAuthorizationRequestsSupported",
		"OIDCResourceProviderLogoutEndSessionEndpointDiscovery",
		"EnableEndpointAliases",
	}
)

func tenantSettingsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tenant-settings",
		Short: "Manage tenant settings",
	}

	cmd.AddCommand(show(cli))
	cmd.AddCommand(update(cli))

	return cmd
}

func show(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show",
		Short:   "Display the current tenant settings",
		Long:    "Display the current tenant settings",
		Example: "auth0 tenant-settings show",
		RunE: func(cmd *cobra.Command, args []string) error {
			tenant, err := cli.api.Tenant.Read(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to fetch tenant settings: %w", err)
			}

			cli.renderer.TenantSettingsShow(tenant)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in JSON format.")

	return cmd
}

func update(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update tenant settings by enabling or disabling flags",
	}

	cmd.AddCommand(set(cli))
	cmd.AddCommand(unset(cli))

	return cmd
}

func set(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Enable tenant setting flags",
		Long: "Enable selected tenant setting flags.\n\n" +
			"To enable interactively, use `auth0 tenant-settings update set` with no arguments.\n\n" +
			"To enable non-interactively, supply the flags.",
		Example: `auth0 tenant-settings update set
auth0 tenant-settings update set <flag1> <flag2> <flag3>
auth0 tenant-settings update set enable_client_connections enable_apis_section enable_pipeline2`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				tenant        = &management.Tenant{}
				tenantFlags   = &management.TenantFlags{}
				selectedFlags []string
				err           error
			)
			if len(args) != 0 {
				selectedFlags = append(selectedFlags, args...)
			} else {
				selectedFlags, err = selectTenantSettingsParams(true)
				if err != nil {
					return err
				}
			}

			setSelectTenantSettings(tenant, selectedFlags, true)
			setSelectedTenantFlags(tenantFlags, selectedFlags, true)
			tenant.Flags = tenantFlags

			if err = cli.api.Tenant.Update(cmd.Context(), tenant); err != nil {
				return err
			}

			cli.renderer.TenantSettingsUpdate(tenant)
			return nil
		},
	}

	return cmd
}

func unset(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unset",
		Short: "Disable tenant setting flags",
		Long: "Disable selected tenant setting flags.\n\n" +
			"To disable interactively, use `auth0 tenant-settings update unset` with no arguments.\n\n" +
			"To disable non-interactively, supply the flags.",
		Example: `auth0 tenant-settings update unset
auth0 tenant-settings update unset <flag1> <flag2> <flag3>
auth0 tenant-settings update unset enable_client_connections enable_apis_section enable_pipeline2`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				tenant        = &management.Tenant{}
				tenantFlags   = &management.TenantFlags{}
				selectedFlags []string
				err           error
			)
			if len(args) != 0 {
				selectedFlags = append(selectedFlags, args...)
			} else {
				selectedFlags, err = selectTenantSettingsParams(false)
				if err != nil {
					return err
				}
			}

			setSelectTenantSettings(tenant, selectedFlags, false)
			setSelectedTenantFlags(tenantFlags, selectedFlags, false)
			tenant.Flags = tenantFlags

			if err := cli.api.Tenant.Update(cmd.Context(), tenant); err != nil {
				return err
			}

			cli.renderer.TenantSettingsUpdate(tenant)
			return nil
		},
	}

	return cmd
}

func selectTenantSettingsParams(isSet bool) ([]string, error) {
	var selectedFlags []string
	label := "Please select the flags you want to "
	if isSet {
		label += "enable (only the selected flags will be changed):"
	} else {
		label += "disable (only the selected flags will be changed):"
	}

	if err := prompt.AskMultiSelect(label, &selectedFlags, supportedFlags...); err != nil {
		return nil, err
	}

	return selectedFlags, nil
}

func setSelectedTenantFlags(f *management.TenantFlags, selectedFlags []string, isSet bool) {
	val := auth0.Bool(isSet)

	for _, name := range selectedFlags {
		switch name {
		case "enable_client_connections", "EnableClientConnections":
			f.EnableClientConnections = val
		case "enable_apis_section", "EnableAPIsSection":
			f.EnableAPIsSection = val
		case "enable_pipeline2", "EnablePipeline2":
			f.EnablePipeline2 = val
		case "enable_dynamic_client_registration", "EnableDynamicClientRegistration":
			f.EnableDynamicClientRegistration = val
		case "enable_custom_domain_in_emails", "EnableCustomDomainInEmails":
			f.EnableCustomDomainInEmails = val
		case "enable_sso", "EnableSSO":
			f.EnableSSO = val
		case "allow_changing_enable_sso", "AllowChangingEnableSSO":
			f.AllowChangingEnableSSO = val
		case "universal_login", "UniversalLogin":
			f.UniversalLogin = val
		case "enable_legacy_logs_search_v2", "EnableLegacyLogsSearchV2":
			f.EnableLegacyLogsSearchV2 = val
		case "disable_clickjack_protection_headers", "DisableClickjackProtectionHeaders":
			f.DisableClickjackProtectionHeaders = val
		case "enable_public_signup_user_exists_error", "EnablePublicSignupUserExistsError":
			f.EnablePublicSignupUserExistsError = val
		case "use_scope_descriptions_for_consent", "UseScopeDescriptionsForConsent":
			f.UseScopeDescriptionsForConsent = val
		case "allow_legacy_delegation_grant_types", "AllowLegacyDelegationGrantTypes":
			f.AllowLegacyDelegationGrantTypes = val
		case "allow_legacy_ro_grant_types", "AllowLegacyROGrantTypes":
			f.AllowLegacyROGrantTypes = val
		case "allow_legacy_tokeninfo_endpoint", "AllowLegacyTokenInfoEndpoint":
			f.AllowLegacyTokenInfoEndpoint = val
		case "enable_legacy_profile", "EnableLegacyProfile":
			f.EnableLegacyProfile = val
		case "enable_idtoken_api2", "EnableIDTokenAPI2":
			f.EnableIDTokenAPI2 = val
		case "no_disclose_enterprise_connections", "NoDisclosureEnterpriseConnections":
			f.NoDisclosureEnterpriseConnections = val
		case "disable_management_api_sms_obfuscation", "DisableManagementAPISMSObfuscation":
			f.DisableManagementAPISMSObfuscation = val
		case "enable_adfs_waad_email_verification", "EnableADFSWAADEmailVerification":
			f.EnableADFSWAADEmailVerification = val
		case "revoke_refresh_token_grant", "RevokeRefreshTokenGrant":
			f.RevokeRefreshTokenGrant = val
		case "dashboard_log_streams_next", "DashboardLogStreams":
			f.DashboardLogStreams = val
		case "dashboard_insights_view", "DashboardInsightsView":
			f.DashboardInsightsView = val
		case "disable_fields_map_fix", "DisableFieldsMapFix":
			f.DisableFieldsMapFix = val
		case "mfa_show_factor_list_on_enrollment", "MFAShowFactorListOnEnrollment":
			f.MFAShowFactorListOnEnrollment = val
		case "require_pushed_authorization_requests", "RequirePushedAuthorizationRequests":
			f.RequirePushedAuthorizationRequests = val
		case "remove_alg_from_jwks", "RemoveAlgFromJWKS":
			f.RemoveAlgFromJWKS = val
		}
	}
}

func setSelectTenantSettings(tenant *management.Tenant, selectedFlags []string, isSet bool) {
	val := auth0.Bool(isSet)

	for _, name := range selectedFlags {
		switch name {
		case "CustomizeMFAInPostLoginAction", "customize_mfa_in_postlogin_action":
			tenant.CustomizeMFAInPostLoginAction = val
		case "AllowOrgNameInAuthAPI", "allow_organization_name_in_authentication_api":
			tenant.AllowOrgNameInAuthAPI = val
		case "PushedAuthorizationRequestsSupported":
			tenant.PushedAuthorizationRequestsSupported = val
		case "OIDCResourceProviderLogoutEndSessionEndpointDiscovery", "rp_logout_end_session_endpoint_discovery":
			tenant.OIDCLogout.OIDCResourceProviderLogoutEndSessionEndpointDiscovery = val
		case "EnableEndpointAliases", "enable_endpoint_aliases":
			tenant.MTLS.EnableEndpointAliases = val
		}
	}
}
