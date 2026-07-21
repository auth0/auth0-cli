package cli

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/utils"

	"github.com/auth0/go-auth0/management"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/prompt"

	"github.com/spf13/cobra"
)

var (
	// Country Codes flags.
	countryCodesList = Flag{
		Name:         "Country Codes List",
		LongForm:     "list",
		Help:         "Comma-separated ISO 3166-1 alpha-2 country codes (e.g., US,GB,CA).",
		IsRequired:   false,
		AlwaysPrompt: false,
	}
	countryCodesMode = Flag{
		Name:         "Country Codes Mode",
		LongForm:     "mode",
		Help:         "Filter mode for country codes. One of allow or deny.",
		IsRequired:   false,
		AlwaysPrompt: false,
	}
)

func tenantSettingsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tenant-settings",
		Short: "Manage tenant settings",
	}

	cmd.AddCommand(show(cli))
	cmd.AddCommand(update(cli))
	cmd.AddCommand(countryCodesCmd(cli))

	return cmd
}

func show(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Display the current tenant settings",
		Long:  "Display the current tenant settings",
		Example: `  auth0 tenant-settings show 
  auth0 tenant-settings show --json
  auth0 tenant-settings show --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			tenant, err := cli.api.Tenant.Read(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to fetch tenant settings: %w", err)
			}

			cli.renderer.TenantSettingsShow(tenant)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

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
auth0 tenant-settings update set <setting1> <setting2> <setting3>
auth0 tenant-settings update set flags.enable_client_connections mtls.enable_endpoint_aliases pushed_authorization_requests_supported`,
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
			if *tenantFlags != (management.TenantFlags{}) {
				tenant.Flags = tenantFlags
			}

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
auth0 tenant-settings update unset <setting1> <setting2> <setting3>
auth0 tenant-settings update unset customize_mfa_in_postlogin_action flags.enable_pipeline2`,
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
			if *tenantFlags != (management.TenantFlags{}) {
				tenant.Flags = tenantFlags
			}

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

	if err := prompt.AskMultiSelect(label, &selectedFlags, utils.FetchKeys(display.SupportedTenantSettings)...); err != nil {
		return nil, err
	}

	return selectedFlags, nil
}

func setSelectedTenantFlags(f *management.TenantFlags, selectedFlags []string, isSet bool) {
	val := auth0.Bool(isSet)

	for _, name := range selectedFlags {
		switch name {
		case display.SupportedTenantSettings["EnableClientConnections"], "EnableClientConnections":
			f.EnableClientConnections = val
		case "flags.enable_apis_section", "EnableAPIsSection":
			f.EnableAPIsSection = val
		case "flags.enable_pipeline2", "EnablePipeline2":
			f.EnablePipeline2 = val
		case "flags.enable_dynamic_client_registration", "EnableDynamicClientRegistration":
			f.EnableDynamicClientRegistration = val
		case "flags.enable_custom_domain_in_emails", "EnableCustomDomainInEmails":
			f.EnableCustomDomainInEmails = val
		case "flags.enable_sso", "EnableSSO":
			f.EnableSSO = val
		case "flags.allow_changing_enable_sso", "AllowChangingEnableSSO":
			f.AllowChangingEnableSSO = val
		case "flags.universal_login", "UniversalLogin":
			f.UniversalLogin = val
		case "flags.enable_legacy_logs_search_v2", "EnableLegacyLogsSearchV2":
			f.EnableLegacyLogsSearchV2 = val
		case "flags.disable_clickjack_protection_headers", "DisableClickjackProtectionHeaders":
			f.DisableClickjackProtectionHeaders = val
		case "flags.enable_public_signup_user_exists_error", "EnablePublicSignupUserExistsError":
			f.EnablePublicSignupUserExistsError = val
		case "flags.use_scope_descriptions_for_consent", "UseScopeDescriptionsForConsent":
			f.UseScopeDescriptionsForConsent = val
		case "flags.allow_legacy_delegation_grant_types", "AllowLegacyDelegationGrantTypes":
			f.AllowLegacyDelegationGrantTypes = val
		case "flags.allow_legacy_ro_grant_types", "AllowLegacyROGrantTypes":
			f.AllowLegacyROGrantTypes = val
		case "flags.allow_legacy_tokeninfo_endpoint", "AllowLegacyTokenInfoEndpoint":
			f.AllowLegacyTokenInfoEndpoint = val
		case "flags.enable_legacy_profile", "EnableLegacyProfile":
			f.EnableLegacyProfile = val
		case "flags.enable_idtoken_api2", "EnableIDTokenAPI2":
			f.EnableIDTokenAPI2 = val
		case "flags.no_disclose_enterprise_connections", "NoDisclosureEnterpriseConnections":
			f.NoDisclosureEnterpriseConnections = val
		case "flags.disable_management_api_sms_obfuscation", "DisableManagementAPISMSObfuscation":
			f.DisableManagementAPISMSObfuscation = val
		case "flags.enable_adfs_waad_email_verification", "EnableADFSWAADEmailVerification":
			f.EnableADFSWAADEmailVerification = val
		case "flags.revoke_refresh_token_grant", "RevokeRefreshTokenGrant":
			f.RevokeRefreshTokenGrant = val
		case "flags.dashboard_log_streams_next", "DashboardLogStreams":
			f.DashboardLogStreams = val
		case "flags.dashboard_insights_view", "DashboardInsightsView":
			f.DashboardInsightsView = val
		case "flags.disable_fields_map_fix", "DisableFieldsMapFix":
			f.DisableFieldsMapFix = val
		case "flags.mfa_show_factor_list_on_enrollment", "MFAShowFactorListOnEnrollment":
			f.MFAShowFactorListOnEnrollment = val
		case "flags.remove_alg_from_jwks", "RemoveAlgFromJWKS":
			f.RemoveAlgFromJWKS = val
		}
	}
}

func setSelectTenantSettings(tenant *management.Tenant, selectedFlags []string, isSet bool) {
	val := auth0.Bool(isSet)

	for _, name := range selectedFlags {
		switch name {
		case "customize_mfa_in_postlogin_action", "CustomizeMFAInPostLoginAction":
			tenant.CustomizeMFAInPostLoginAction = val
		case "allow_organization_name_in_authentication_api", "AllowOrgNameInAuthAPI":
			tenant.AllowOrgNameInAuthAPI = val
		case "pushed_authorization_requests_supported", "PushedAuthorizationRequestsSupported":
			tenant.PushedAuthorizationRequestsSupported = val
		case "client_id_metadata_document_supported", "ClientIDMetadataDocumentSupported":
			tenant.ClientIDMetadataDocumentSupported = val
		case "oidc_logout.rp_logout_end_session_endpoint_discovery", "OIDCLogout.RPLogoutEndSessionEndpointDiscovery":
			if tenant.OIDCLogout == nil {
				tenant.OIDCLogout = &management.TenantOIDCLogout{}
			}
			tenant.OIDCLogout.OIDCResourceProviderLogoutEndSessionEndpointDiscovery = val
		case "mtls.enable_endpoint_aliases", "MTLS.EnableEndpointAliases":
			if tenant.MTLS == nil {
				tenant.MTLS = &management.TenantMTLSConfiguration{}
			}
			tenant.MTLS.EnableEndpointAliases = val
		}
	}
}

func countryCodesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "country-codes",
		Aliases: []string{"cc"},
		Short:   "Manage country codes filtering for the tenant",
		Long:    "Manage the country codes filtering configured for the tenant.",
	}

	cmd.AddCommand(showCountryCodesCmd(cli))
	cmd.AddCommand(updateCountryCodesCmd(cli))
	cmd.AddCommand(removeCountryCodesCmd(cli))

	return cmd
}

func removeCountryCodesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove",
		Aliases: []string{"rm"},
		Short:   "Remove country codes filtering from the tenant",
		Long: "Remove country codes filtering from the tenant.\n\n" +
			"This clears any configured allow/deny list by setting country_codes to null.",
		Example: `  auth0 tenant-settings country-codes remove`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Send null via the raw HTTP client, since `omitempty` omits nil pointers and prevents the PATCH API from clearing the filter.
			payload := map[string]interface{}{"country_codes": nil}
			uri := cli.api.HTTPClient.URI("tenants", "settings")

			if err := ansi.Waiting(func() error {
				return cli.api.HTTPClient.Request(cmd.Context(), http.MethodPatch, uri, payload)
			}); err != nil {
				return fmt.Errorf("failed to remove tenant country codes: %w", err)
			}

			cli.renderer.TenantCountryCodesRemove()
			return nil
		},
	}

	return cmd
}

func showCountryCodesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Display the tenant's country codes filtering",
		Long:  "Display the country codes filtering configured for the tenant.",
		Example: `  auth0 tenant-settings country-codes show
  auth0 tenant-settings country-codes show --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var tenant *management.Tenant
			if err := ansi.Waiting(func() error {
				var err error
				tenant, err = cli.api.Tenant.Read(cmd.Context())
				return err
			}); err != nil {
				return fmt.Errorf("failed to fetch tenant settings: %w", err)
			}

			cli.renderer.TenantCountryCodesShow(tenant.CountryCodes)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	return cmd
}

func updateCountryCodesCmd(cli *cli) *cobra.Command {
	var inputs struct {
		List string
		Mode string
	}

	cmd := &cobra.Command{
		Use:     "update",
		Short:   "Set country codes filtering for the tenant",
		Long:    "Set country codes filtering for the tenant.\n\nTo set country codes interactively, omit the flags.",
		Example: `  auth0 tenant-settings country-codes update --list US,GB,CA --mode allow`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := countryCodesList.Ask(cmd, &inputs.List, nil); err != nil {
				return err
			}
			if err := countryCodesMode.Ask(cmd, &inputs.Mode, nil); err != nil {
				return err
			}

			countryCodes := &management.TenantCountryCodes{
				List: parseCountryCodesList(inputs.List),
				Mode: inputs.Mode,
			}

			if err := ansi.Waiting(func() error {
				return cli.api.Tenant.Update(cmd.Context(), &management.Tenant{
					CountryCodes: countryCodes,
				})
			}); err != nil {
				return fmt.Errorf("failed to update tenant country codes: %w", err)
			}

			cli.renderer.TenantCountryCodesUpdate(countryCodes)
			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	countryCodesList.RegisterString(cmd, &inputs.List, "")
	countryCodesMode.RegisterString(cmd, &inputs.Mode, "")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")
	return cmd
}

// parseCountryCodesList splits a comma-separated list of country codes into a
// slice, trimming whitespace and dropping empty entries.
func parseCountryCodesList(list string) []string {
	var codes []string
	for _, code := range strings.Split(list, ",") {
		if c := strings.TrimSpace(code); c != "" {
			codes = append(codes, c)
		}
	}
	return codes
}
