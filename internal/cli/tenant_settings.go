package cli

import (
	"fmt"

	"github.com/auth0/go-auth0/management"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/prompt"

	"github.com/spf13/cobra"
)

var (
	flags = []string{
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
		Use:   "show",
		Short: "Display current tenant settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			var tenant *management.Tenant

			tenant, err := cli.api.Tenant.Read(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to fetch tenant settings : %w", err)
			}

			cli.renderer.TenantSettingsShow(tenant)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

	return cmd
}

func update(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update tenant settings by setting and unsetting the flags",
	}

	cmd.AddCommand(set(cli))
	cmd.AddCommand(unset(cli))

	return cmd
}

func set(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Toggles on the tenant settings flags that are parsed",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Fields , that u want to toggle as true (set to true)

			// auth0 tenant-settings update set flag1,flag2

			// Prompt for updating the fields , that u want to toggle as true
			tenant, err := askTenantSettingsUpdates(true)
			if err != nil {
				return err
			}

			// Perform the update
			if err := cli.api.Tenant.Update(cmd.Context(), tenant); err != nil {
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
		Short: "Toggles off the tenant settings flags that are parsed",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Fields , that u want to toggle as false (set to false)

			// auth0 tenant-settings update unset flag3,flag4

			// Prompt for updating the fields , that u want to toggle as false
			tenant, err := askTenantSettingsUpdates(false)
			if err != nil {
				return err
			}

			// Perform the update
			if err := cli.api.Tenant.Update(cmd.Context(), tenant); err != nil {
				return err
			}

			cli.renderer.TenantSettingsUpdate(tenant)

			return nil
		},
	}

	return cmd
}

func askTenantSettingsUpdates(isSet bool) (*management.Tenant, error) {
	var (
		tenantFlags = &management.TenantFlags{}
		tenant      = &management.Tenant{}
	)

	settingsMap, err := selectTenantSettingsParams(isSet)
	if err != nil {
		return nil, err
	}

	setSelectTenantSettings(tenant, settingsMap)

	setSelectedTenantFlags(tenantFlags, settingsMap)
	tenant.Flags = tenantFlags

	return tenant, nil
}

func selectTenantSettingsParams(isSet bool) (map[string]*bool, error) {
	var selected []string
	if err := prompt.AskMultiSelect(
		"Please select the flags to enable (only the selected would be change):",
		&selected,
		flags...,
	); err != nil {
		return nil, err
	}

	// Convert to lookup map
	selectedMap := make(map[string]*bool)

	for _, opt := range selected {
		selectedMap[opt] = auth0.Bool(isSet)
	}

	return selectedMap, nil
}

func setSelectedTenantFlags(f *management.TenantFlags, flags map[string]*bool) {
	for name, enabled := range flags {
		switch name {
		case "EnableClientConnections":
			f.EnableClientConnections = enabled
		case "EnableAPIsSection":
			f.EnableAPIsSection = enabled
		case "EnablePipeline2":
			f.EnablePipeline2 = enabled
		case "EnableDynamicClientRegistration":
			f.EnableDynamicClientRegistration = enabled
		case "EnableCustomDomainInEmails":
			f.EnableCustomDomainInEmails = enabled
		case "EnableSSO":
			f.EnableSSO = enabled
		case "AllowChangingEnableSSO":
			f.AllowChangingEnableSSO = enabled
		case "UniversalLogin":
			f.UniversalLogin = enabled
		case "EnableLegacyLogsSearchV2":
			f.EnableLegacyLogsSearchV2 = enabled
		case "DisableClickjackProtectionHeaders":
			f.DisableClickjackProtectionHeaders = enabled
		case "EnablePublicSignupUserExistsError":
			f.EnablePublicSignupUserExistsError = enabled
		case "UseScopeDescriptionsForConsent":
			f.UseScopeDescriptionsForConsent = enabled
		case "AllowLegacyDelegationGrantTypes":
			f.AllowLegacyDelegationGrantTypes = enabled
		case "AllowLegacyROGrantTypes":
			f.AllowLegacyROGrantTypes = enabled
		case "AllowLegacyTokenInfoEndpoint":
			f.AllowLegacyTokenInfoEndpoint = enabled
		case "EnableLegacyProfile":
			f.EnableLegacyProfile = enabled
		case "EnableIDTokenAPI2":
			f.EnableIDTokenAPI2 = enabled
		case "NoDisclosureEnterpriseConnections":
			f.NoDisclosureEnterpriseConnections = enabled
		case "DisableManagementAPISMSObfuscation":
			f.DisableManagementAPISMSObfuscation = enabled
		case "EnableADFSWAADEmailVerification":
			f.EnableADFSWAADEmailVerification = enabled
		case "RevokeRefreshTokenGrant":
			f.RevokeRefreshTokenGrant = enabled
		case "DashboardLogStreams":
			f.DashboardLogStreams = enabled
		case "DashboardInsightsView":
			f.DashboardInsightsView = enabled
		case "DisableFieldsMapFix":
			f.DisableFieldsMapFix = enabled
		case "MFAShowFactorListOnEnrollment":
			f.MFAShowFactorListOnEnrollment = enabled
		case "RequirePushedAuthorizationRequests":
			f.RequirePushedAuthorizationRequests = enabled
		case "RemoveAlgFromJWKS":
			f.RemoveAlgFromJWKS = enabled
		}
	}
}

func setSelectTenantSettings(tenant *management.Tenant, flags map[string]*bool) {
	for name, enabled := range flags {
		switch name {
		case "CustomizeMFAInPostLoginAction":
			tenant.CustomizeMFAInPostLoginAction = enabled
		case "AllowOrgNameInAuthAPI":
			tenant.AllowOrgNameInAuthAPI = enabled
		case "PushedAuthorizationRequestsSupported":
			tenant.PushedAuthorizationRequestsSupported = enabled
		case "OIDCResourceProviderLogoutEndSessionEndpointDiscovery":
			tenant.OIDCLogout.OIDCResourceProviderLogoutEndSessionEndpointDiscovery = enabled
		case "EnableEndpointAliases":
			tenant.MTLS.EnableEndpointAliases = enabled
		}
	}
}
