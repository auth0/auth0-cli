package cli

import (
	"errors"
	"fmt"

	"github.com/auth0/auth0-cli/internal/prompt"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"
)

func tenantSettingsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tenant-settings",
		Short: "Manage tenant settings",
	}

	cmd.AddCommand(get(cli))
	cmd.AddCommand(update(cli))

	return cmd
}

func get(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Display current tenant settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			var tenant *management.Tenant

			tenant, err := cli.api.Tenant.Read(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to fetch tenant settings : %w", err)
			}

			cli.renderer.SettingShow(tenant)

			return nil
		},
	}
	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

	return cmd
}

func selectTenantSettingsFields() ([]string, error) {
	options := []string{
		"DefaultAudience",
		"DefaultDirectory",
		"FriendlyName",
		"PictureURL",
		"SupportEmail",
		"SupportURL",
		"AllowedLogoutURLs",
		"SessionLifetime",
		"IdleSessionLifetime",
		"SandboxVersion",
		"SandboxVersionAvailable",
		"DefaultRedirectionURI",
		"EnabledLocales",
		"CustomizeMFAInPostLoginAction",
		"AllowOrgNameInAuthAPI",
		"Flags",
	}

	var selected []string
	if err := prompt.AskMultiSelect(
		"Please select the tenant settings fields you want to update:",
		&selected,
		options...,
	); err != nil {
		return nil, err
	}

	if len(selected) == 0 {
		return nil, errors.New("at least one setting must be selected")
	}

	return selected, nil
}

func update(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update current tenant settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Fetch current settings
			current, err := cli.api.Tenant.Read(cmd.Context())
			if err != nil {
				return err
			}

			// Fields , that u want to toggle as true (set to true)

			//auth0 tenant-settings update set flag1,flag2
			//auth0 tenant-settings update unset flag3,flag4

			// Ask user what to update
			selectedFields, err := selectTenantSettingsFields()
			if err != nil {
				return err
			}

			// Fields , that u want to toggle as false

			// Prompt for updates
			if err := askTenantSettingsUpdates(cmd, selectedFields, current); err != nil {
				return err
			}

			// Perform the update
			if err := cli.api.Tenant.Update(cmd.Context(), current); err != nil {
				return err
			}
			cli.renderer.SettingShow(current)
			cli.renderer.Infof("Tenant settings updated successfully.")

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in JSON format")

	return cmd
}

type tenantSettingsInputs struct {
	DefaultAudience               string
	DefaultDirectory              string
	FriendlyName                  string
	PictureURL                    string
	SupportEmail                  string
	SupportURL                    string
	AllowedLogoutURLs             []string
	SessionLifetime               float64
	IdleSessionLifetime           float64
	SandboxVersion                string
	SandboxVersionAvailable       []string
	DefaultRedirectionURI         string
	EnabledLocales                []string
	CustomizeMFAInPostLoginAction bool
	AllowOrgNameInAuthAPI         bool
	FlagsMap                      map[string]bool
}

func askTenantSettingsUpdates(cmd *cobra.Command, selected []string, t *management.Tenant) error {
	inputs := tenantSettingsInputs{}

	for _, field := range selected {
		switch field {
		case "DefaultAudience":
			cmd.Flags().StringVar(&inputs.DefaultAudience, "default-audience", "", "Default Audience")
		case "DefaultDirectory":
			cmd.Flags().StringVar(&inputs.DefaultDirectory, "default-directory", "", "Default Directory")
		case "FriendlyName":
			cmd.Flags().StringVar(&inputs.FriendlyName, "friendly-name", "", "Friendly Name")
		case "PictureURL":
			cmd.Flags().StringVar(&inputs.PictureURL, "picture-url", "", "Picture URL")
		case "SupportEmail":
			cmd.Flags().StringVar(&inputs.SupportEmail, "support-email", "", "Support Email")
		case "SupportURL":
			cmd.Flags().StringVar(&inputs.SupportURL, "support-url", "", "Support URL")
		case "AllowedLogoutURLs":
			cmd.Flags().StringSliceVar(&inputs.AllowedLogoutURLs, "allowed-logout-urls", nil, "Allowed Logout URLs (comma-separated)")
		case "SessionLifetime":
			cmd.Flags().Float64Var(&inputs.SessionLifetime, "session-lifetime", 0, "Session Lifetime (in hours)")
		case "IdleSessionLifetime":
			cmd.Flags().Float64Var(&inputs.IdleSessionLifetime, "idle-session-lifetime", 0, "Idle Session Lifetime (in hours)")
		case "SandboxVersion":
			cmd.Flags().StringVar(&inputs.SandboxVersion, "sandbox-version", "", "Sandbox Version")
		case "SandboxVersionAvailable":
			cmd.Flags().StringSliceVar(&inputs.SandboxVersionAvailable, "sandbox-version-available", nil, "Sandbox Versions Available (comma-separated)")
		case "DefaultRedirectionURI":
			cmd.Flags().StringVar(&inputs.DefaultRedirectionURI, "default-redirection-uri", "", "Default Redirection URI")
		case "EnabledLocales":
			cmd.Flags().StringSliceVar(&inputs.EnabledLocales, "enabled-locales", nil, "Enabled Locales (comma-separated)")
		case "CustomizeMFAInPostLoginAction":
			cmd.Flags().BoolVar(&inputs.CustomizeMFAInPostLoginAction, "customize-mfa-in-postlogin-action", false, "Customize MFA in PostLogin Action")
		case "AllowOrgNameInAuthAPI":
			cmd.Flags().BoolVar(&inputs.AllowOrgNameInAuthAPI, "allow-org-name-in-auth-api", false, "Allow org name in Auth API")
		case "Flags":
			flagsMap, err := selectTenantFlagParams()
			if err != nil {
				return err
			}
			inputs.FlagsMap = flagsMap
		}
	}

	// Assign to the tenant settings struct
	t.DefaultAudience = &inputs.DefaultAudience
	t.DefaultDirectory = &inputs.DefaultDirectory
	t.FriendlyName = &inputs.FriendlyName
	t.PictureURL = &inputs.PictureURL
	t.SupportEmail = &inputs.SupportEmail
	t.SupportURL = &inputs.SupportURL
	t.AllowedLogoutURLs = &inputs.AllowedLogoutURLs
	t.SessionLifetime = &inputs.SessionLifetime
	t.IdleSessionLifetime = &inputs.IdleSessionLifetime
	t.SandboxVersion = &inputs.SandboxVersion
	t.SandboxVersionAvailable = &inputs.SandboxVersionAvailable
	t.DefaultRedirectionURI = &inputs.DefaultRedirectionURI
	t.EnabledLocales = &inputs.EnabledLocales
	t.CustomizeMFAInPostLoginAction = &inputs.CustomizeMFAInPostLoginAction
	t.AllowOrgNameInAuthAPI = &inputs.AllowOrgNameInAuthAPI

	if inputs.FlagsMap != nil {
		flags := &management.TenantFlags{}
		setSelectedTenantFlags(flags, inputs.FlagsMap)
		t.Flags = flags
	}

	return nil
}

func selectTenantFlagParams() (map[string]bool, error) {
	options := []string{
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
	}

	var selected []string
	if err := prompt.AskMultiSelect(
		"Please select the flags to enable (unselected will be set to false):",
		&selected,
		options...,
	); err != nil {
		return nil, err
	}

	// Convert to lookup map
	selectedMap := make(map[string]bool)
	for _, opt := range options {
		selectedMap[opt] = false
	}

	for _, opt := range selected {
		selectedMap[opt] = true
	}

	return selectedMap, nil
}

func setSelectedTenantFlags(f *management.TenantFlags, flags map[string]bool) {
	for name, enabled := range flags {
		switch name {
		case "EnableClientConnections":
			f.EnableClientConnections = &enabled
		case "EnableAPIsSection":
			f.EnableAPIsSection = &enabled
		case "EnablePipeline2":
			f.EnablePipeline2 = &enabled
		case "EnableDynamicClientRegistration":
			f.EnableDynamicClientRegistration = &enabled
		case "EnableCustomDomainInEmails":
			f.EnableCustomDomainInEmails = &enabled
		case "EnableSSO":
			f.EnableSSO = &enabled
		case "AllowChangingEnableSSO":
			f.AllowChangingEnableSSO = &enabled
		case "UniversalLogin":
			f.UniversalLogin = &enabled
		case "EnableLegacyLogsSearchV2":
			f.EnableLegacyLogsSearchV2 = &enabled
		case "DisableClickjackProtectionHeaders":
			f.DisableClickjackProtectionHeaders = &enabled
		case "EnablePublicSignupUserExistsError":
			f.EnablePublicSignupUserExistsError = &enabled
		case "UseScopeDescriptionsForConsent":
			f.UseScopeDescriptionsForConsent = &enabled
		case "AllowLegacyDelegationGrantTypes":
			f.AllowLegacyDelegationGrantTypes = &enabled
		case "AllowLegacyROGrantTypes":
			f.AllowLegacyROGrantTypes = &enabled
		case "AllowLegacyTokenInfoEndpoint":
			f.AllowLegacyTokenInfoEndpoint = &enabled
		case "EnableLegacyProfile":
			f.EnableLegacyProfile = &enabled
		case "EnableIDTokenAPI2":
			f.EnableIDTokenAPI2 = &enabled
		case "NoDisclosureEnterpriseConnections":
			f.NoDisclosureEnterpriseConnections = &enabled
		case "DisableManagementAPISMSObfuscation":
			f.DisableManagementAPISMSObfuscation = &enabled
		case "EnableADFSWAADEmailVerification":
			f.EnableADFSWAADEmailVerification = &enabled
		case "RevokeRefreshTokenGrant":
			f.RevokeRefreshTokenGrant = &enabled
		case "DashboardLogStreams":
			f.DashboardLogStreams = &enabled
		case "DashboardInsightsView":
			f.DashboardInsightsView = &enabled
		case "DisableFieldsMapFix":
			f.DisableFieldsMapFix = &enabled
		case "MFAShowFactorListOnEnrollment":
			f.MFAShowFactorListOnEnrollment = &enabled
		case "RequirePushedAuthorizationRequests":
			f.RequirePushedAuthorizationRequests = &enabled
		case "RemoveAlgFromJWKS":
			f.RemoveAlgFromJWKS = &enabled
		}
	}
}
