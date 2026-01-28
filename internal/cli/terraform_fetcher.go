package cli

import (
	"context"
	"net/http"
	"strings"

	"github.com/auth0/go-auth0/management"
	"github.com/google/uuid"

	"github.com/auth0/auth0-cli/internal/auth0"
)

var (
	defaultResources = []string{"auth0_action", "auth0_attack_protection", "auth0_branding", "auth0_branding_theme", "auth0_phone_provider", "auth0_client", "auth0_client_grant", "auth0_connection", "auth0_custom_domain", "auth0_flow", "auth0_flow_vault_connection", "auth0_form", "auth0_email_provider", "auth0_email_template", "auth0_guardian", "auth0_log_stream", "auth0_network_acl", "auth0_organization", "auth0_pages", "auth0_prompt", "auth0_prompt_custom_text", "auth0_prompt_screen_renderer", "auth0_resource_server", "auth0_role", "auth0_self_service_profile", "auth0_tenant", "auth0_trigger_actions", "auth0_user_attribute_profile", "auth0_prompt_screen_partial"}
)

type (
	importDataList []importDataItem

	importDataItem struct {
		ResourceName string
		ImportID     string
	}

	resourceDataFetcher interface {
		FetchData(ctx context.Context) (importDataList, error)
	}
)

type (
	actionResourceFetcher struct {
		api *auth0.API
	}

	attackProtectionResourceFetcher struct{}

	brandingResourceFetcher struct{}

	brandingThemeResourceFetcher struct {
		api *auth0.API
	}

	phoneProviderResourceFetcher struct {
		api *auth0.API
	}

	clientResourceFetcher struct {
		api *auth0.API
	}

	clientGrantResourceFetcher struct {
		api *auth0.API
	}

	connectionResourceFetcher struct {
		api *auth0.API
	}

	customDomainResourceFetcher struct {
		api *auth0.API
	}

	emailProviderResourceFetcher struct {
		api *auth0.API
	}

	emailTemplateResourceFetcher struct {
		api *auth0.API
	}

	flowResourceFetcher struct {
		api *auth0.API
	}

	flowVaultConnectionResourceFetcher struct {
		api *auth0.API
	}

	formResourceFetcher struct {
		api *auth0.API
	}

	guardianResourceFetcher  struct{}
	logStreamResourceFetcher struct {
		api *auth0.API
	}
	organizationResourceFetcher struct {
		api *auth0.API
	}

	networkACLResourceFetcher struct {
		api *auth0.API
	}

	pagesResourceFetcher          struct{}
	resourceServerResourceFetcher struct {
		api *auth0.API
	}

	promptResourceFetcher               struct{}
	promptScreenRendererResourceFetcher struct {
		api *auth0.API
	}

	promptCustomTextResourceFetcherResourceFetcher struct {
		api *auth0.API
	}

	promptScreenPartialResourceFetcher struct{}

	roleResourceFetcher struct {
		api *auth0.API
	}

	selfServiceProfileFetcher struct {
		api *auth0.API
	}

	tenantResourceFetcher struct{}

	triggerActionsResourceFetcher struct {
		api *auth0.API
	}

	userAttributeProfilesResourceFetcher struct {
		api *auth0.API
	}
)

func (f *attackProtectionResourceFetcher) FetchData(_ context.Context) (importDataList, error) {
	return []importDataItem{
		{
			ResourceName: "auth0_attack_protection.attack_protection",
			ImportID:     uuid.NewString(),
		},
	}, nil
}

func (f *brandingResourceFetcher) FetchData(_ context.Context) (importDataList, error) {
	return []importDataItem{
		{
			ResourceName: "auth0_branding.branding",
			ImportID:     uuid.NewString(),
		},
	}, nil
}
func (f *brandingThemeResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	theme, err := f.api.BrandingTheme.Default(ctx)
	if err != nil {
		if mErr, ok := err.(management.Error); ok && mErr.Status() == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}

	data = append(data, importDataItem{
		ResourceName: "auth0_branding_theme.default",
		ImportID:     theme.GetID(),
	})

	return data, nil
}

func (f *phoneProviderResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	phoneProvidersList, err := f.api.Branding.ListPhoneProviders(ctx)
	if err != nil {
		return nil, err
	}

	if len(phoneProvidersList.Providers) == 0 {
		return nil, nil
	}

	for _, provider := range phoneProvidersList.Providers {
		data = append(data, importDataItem{
			ResourceName: "auth0_phone_provider." + sanitizeResourceName(provider.GetName()),
			ImportID:     provider.GetID(),
		})
	}

	return data, nil
}

func (f *clientResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	var page int
	for {
		clients, err := f.api.Client.List(
			ctx,
			management.Page(page),
			management.Parameter("is_global", "false"),
			management.IncludeFields("client_id", "name"),
		)
		if err != nil {
			return nil, err
		}

		for _, client := range clients.Clients {
			data = append(data, importDataItem{
				ResourceName: "auth0_client." + sanitizeResourceName(client.GetName()),
				ImportID:     client.GetClientID(),
			})

			data = append(data, importDataItem{
				ResourceName: "auth0_client_credentials." + sanitizeResourceName(client.GetName()),
				ImportID:     client.GetClientID(),
			})
		}

		if !clients.HasNext() {
			break
		}

		page++
	}

	return data, nil
}

func (f *clientGrantResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	var page int
	for {
		grants, err := f.api.ClientGrant.List(
			ctx,
			management.Page(page),
		)
		if err != nil {
			return nil, err
		}

		for _, grant := range grants.ClientGrants {
			data = append(data, importDataItem{
				ResourceName: "auth0_client_grant." + sanitizeResourceName(grant.GetClientID()+"_"+grant.GetAudience()),
				ImportID:     grant.GetID(),
			})
		}

		if !grants.HasNext() {
			break
		}

		page++
	}

	return data, nil
}

func (f *connectionResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	var page int
	for {
		connections, err := f.api.Connection.List(
			ctx,
			management.Page(page),
			management.IncludeFields("id", "name"),
		)
		if err != nil {
			return nil, err
		}

		for _, connection := range connections.Connections {
			data = append(data,
				importDataItem{
					ResourceName: "auth0_connection." + sanitizeResourceName(connection.GetName()),
					ImportID:     connection.GetID(),
				},
				importDataItem{
					ResourceName: "auth0_connection_clients." + sanitizeResourceName(connection.GetName()),
					ImportID:     connection.GetID(),
				},
			)
		}

		if !connections.HasNext() {
			break
		}

		page++
	}

	return data, nil
}

func (f *customDomainResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	customDomains, err := f.api.CustomDomain.List(ctx)
	if err != nil {
		errNotEnabled := []string{
			"The account is not allowed to perform this operation, please contact our support team",
			"There must be a verified credit card on file to perform this operation",
		}

		for _, e := range errNotEnabled {
			if strings.Contains(err.Error(), e) {
				return data, nil
			}
		}
		return nil, err
	}

	for _, domain := range customDomains {
		data = append(data, importDataItem{
			ResourceName: "auth0_custom_domain." + sanitizeResourceName(domain.GetDomain()),
			ImportID:     domain.GetID(),
		})
	}

	return data, nil
}

func (f *emailProviderResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	_, err := f.api.EmailProvider.Read(ctx)
	if err != nil {
		if mErr, ok := err.(management.Error); ok && mErr.Status() == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}

	return []importDataItem{
		{
			ResourceName: "auth0_email_provider.email_provider",
			ImportID:     uuid.NewString(),
		},
	}, nil
}

func (f *emailTemplateResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	templates := []string{`verify_email`, `reset_email`, `welcome_email`, `blocked_account`, `stolen_credentials`, `enrollment_email`, `mfa_oob_code`, `change_password`, `password_reset`, `verify_email_by_code`, `reset_email_by_code`, `user_invitation`, `async_approval`}

	for _, template := range templates {
		emailTemplate, err := f.api.EmailTemplate.Read(ctx, template)
		if err != nil {
			if mErr, ok := err.(management.Error); ok && (mErr.Status() == http.StatusNotFound || mErr.Status() == http.StatusForbidden) {
				continue
			}
			return nil, err
		}

		data = append(data, importDataItem{
			ResourceName: "auth0_email_template." + sanitizeResourceName(emailTemplate.GetTemplate()),
			ImportID:     sanitizeResourceName(emailTemplate.GetTemplate()),
		})
	}

	return data, nil
}

func (f *flowResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	flowList, err := f.api.Flow.List(ctx)
	if err != nil {
		return data, err
	}

	for _, flow := range flowList.Flows {
		data = append(data, importDataItem{
			ResourceName: "auth0_flow." + sanitizeResourceName(flow.GetName()),
			ImportID:     flow.GetID(),
		})
	}

	return data, nil
}

func (f *flowVaultConnectionResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	flowVaultConnectionList, err := f.api.FlowVaultConnection.GetConnectionList(ctx)
	if err != nil {
		return data, err
	}

	for _, flowVaultConnection := range flowVaultConnectionList.Connections {
		data = append(data, importDataItem{
			ResourceName: "auth0_flow_vault_connection." + sanitizeResourceName(flowVaultConnection.GetName()),
			ImportID:     flowVaultConnection.GetID(),
		})
	}

	return data, nil
}

func (f *formResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	forms, err := f.api.Form.List(ctx)
	if err != nil {
		return data, err
	}

	for _, form := range forms.Forms {
		data = append(data, importDataItem{
			ResourceName: "auth0_form." + sanitizeResourceName(form.GetName()),
			ImportID:     form.GetID(),
		})
	}

	return data, nil
}

func (f *guardianResourceFetcher) FetchData(_ context.Context) (importDataList, error) {
	return []importDataItem{
		{
			ResourceName: "auth0_guardian.guardian",
			ImportID:     uuid.NewString(),
		},
	}, nil
}

func (f *logStreamResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	logStreams, err := f.api.LogStream.List(ctx)
	if err != nil {
		return data, err
	}

	for _, log := range logStreams {
		data = append(data, importDataItem{
			ResourceName: "auth0_log_stream." + sanitizeResourceName(log.GetName()),
			ImportID:     log.GetID(),
		})
	}

	return data, nil
}

func (f *organizationResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	orgs, err := getWithPagination(
		100,
		func(opts ...management.RequestOption) (result []interface{}, hasNext bool, err error) {
			res, err := f.api.Organization.List(ctx, opts...)
			if err != nil {
				return nil, false, err
			}

			for _, item := range res.Organizations {
				result = append(result, item)
			}

			return result, res.HasNext(), nil
		},
	)
	if err != nil {
		return data, err
	}

	for _, org := range orgs {
		organization := org.(*management.Organization)
		data = append(data, importDataItem{
			ResourceName: "auth0_organization." + sanitizeResourceName(organization.GetName()),
			ImportID:     organization.GetID(),
		})

		conns, err := f.api.Organization.Connections(ctx, organization.GetID())
		if err != nil {
			return data, err
		}
		if len(conns.OrganizationConnections) > 0 {
			data = append(data, importDataItem{
				ResourceName: "auth0_organization_connections." + sanitizeResourceName(organization.GetName()),
				ImportID:     organization.GetID(),
			})
		}

		discoveryDomains, err := f.api.Organization.DiscoveryDomains(ctx, organization.GetID())
		if err != nil {
			return data, err
		}
		if len(discoveryDomains.Domains) > 0 {
			data = append(data, importDataItem{
				ResourceName: "auth0_organization_discovery_domains." + sanitizeResourceName(organization.GetName()),
				ImportID:     organization.GetID(),
			})
		}
	}

	return data, nil
}

func (f *networkACLResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	networkACLs, err := f.api.NetworkACL.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, networkACL := range networkACLs {
		data = append(data, importDataItem{
			ResourceName: "auth0_network_acl." + sanitizeResourceName(networkACL.GetID()),
			ImportID:     networkACL.GetID(),
		})
	}

	return data, nil
}

func (f *pagesResourceFetcher) FetchData(_ context.Context) (importDataList, error) {
	return []importDataItem{
		{
			ResourceName: "auth0_pages.pages",
			ImportID:     uuid.NewString(),
		},
	}, nil
}

func (f *promptResourceFetcher) FetchData(_ context.Context) (importDataList, error) {
	return []importDataItem{
		{
			ResourceName: "auth0_prompt.prompts",
			ImportID:     uuid.NewString(),
		},
	}, nil
}

// Referred from 'prompt' path options in: https://auth0.com/docs/api/management/v2/prompts/get-custom-text-by-language
var customTextPromptTypes = []string{"login", "login-id", "login-password", "login-email-verification", "signup", "signup-id", "signup-password", "reset-password", "consent", "mfa-push", "mfa-otp", "mfa-voice", "mfa-phone", "mfa-webauthn", "mfa-sms", "mfa-email", "mfa-recovery-code", "mfa", "status", "device-flow", "email-verification", "email-otp-challenge", "organizations", "invitation", "common", "email-identifier-challenge", "passkeys", "login-passwordless", "phone-identifier-enrollment", "phone-identifier-challenge", "custom-form", "customized-consent", "logout", "captcha", "brute-force-protection"}

func (f *promptCustomTextResourceFetcherResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	tenant, err := f.api.Tenant.Read(ctx)
	if err != nil {
		return nil, err
	}

	var data importDataList
	for _, language := range tenant.GetEnabledLocales() {
		for _, promptType := range customTextPromptTypes {
			data = append(data, importDataItem{
				ResourceName: "auth0_prompt_custom_text." + sanitizeResourceName(language+"_"+promptType),
				ImportID:     promptType + "::" + language,
			})
		}
	}

	return data, nil
}

// Referred from: https://tus.auth0.com/docs/customize/login-pages/universal-login/customize-signup-and-login-prompts#terminology
// Referred from prompt 'path' options in: https://auth0.com/docs/api/management/v2/prompts/get-partials
var screenPartialPromptTypeToScreenMap = map[string][]string{
	"login":              {"login"},
	"login-id":           {"login-id"},
	"login-password":     {"login-password"},
	"login-passwordless": {"login-passwordless-sms-otp", "login-passwordless-email-code"},
	"signup":             {"signup"},
	"signup-id":          {"signup-id"},
	"signup-password":    {"signup-password"},
	"customized-consent": {"customized-consent"},
}

func (f *promptScreenPartialResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	for promptType, screens := range screenPartialPromptTypeToScreenMap {
		for _, screen := range screens {
			data = append(data, importDataItem{
				ResourceName: "auth0_prompt_screen_partial." + sanitizeResourceName(promptType+"_"+screen),
				ImportID:     promptType + ":" + screen,
			})
		}
	}

	return data, nil
}

func (f *promptScreenRendererResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	screenSettingList, err := f.api.Prompt.ListRendering(ctx)
	if err != nil {
		return nil, err
	}

	var data importDataList

	for _, screenSetting := range screenSettingList.PromptRenderings {
		data = append(data, importDataItem{
			ResourceName: "auth0_prompt_screen_renderer." + sanitizeResourceName(string(*screenSetting.Prompt)+"_"+string(*screenSetting.Screen)),
			ImportID:     string(*screenSetting.Prompt) + ":" + string(*screenSetting.Screen),
		})
	}

	return data, nil
}

func (f *resourceServerResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	var page int
	for {
		apis, err := f.api.ResourceServer.List(
			ctx,
			management.Page(page),
			management.IncludeFields("id", "name", "scopes"),
			management.PerPage(100),
		)
		if err != nil {
			return nil, err
		}

		for _, api := range apis.ResourceServers {
			data = append(data, importDataItem{
				ResourceName: "auth0_resource_server." + sanitizeResourceName(api.GetName()),
				ImportID:     api.GetID(),
			})

			if len(api.GetScopes()) > 0 {
				data = append(data, importDataItem{
					ResourceName: "auth0_resource_server_scopes." + sanitizeResourceName(api.GetName()),
					ImportID:     api.GetID(),
				})
			}
		}

		if !apis.HasNext() {
			break
		}

		page++
	}

	return data, nil
}

func (f *roleResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	var page int
	for {
		roles, err := f.api.Role.List(
			ctx,
			management.Page(page),
			management.IncludeFields("id", "name"),
		)
		if err != nil {
			return nil, err
		}

		for _, role := range roles.Roles {
			data = append(data,
				importDataItem{
					ResourceName: "auth0_role." + sanitizeResourceName(role.GetName()),
					ImportID:     role.GetID(),
				},
			)

			rolePerms, err := f.api.Role.Permissions(ctx, role.GetID())
			if err != nil {
				return data, nil
			}
			if len(rolePerms.Permissions) > 0 {
				// `permissions` block a required field for TF Provider; cannot have empty permissions.
				data = append(data, importDataItem{
					ResourceName: "auth0_role_permissions." + sanitizeResourceName(role.GetName()),
					ImportID:     role.GetID(),
				})
			}
		}

		if !roles.HasNext() {
			break
		}

		page++
	}

	return data, nil
}

var selfServiceProfileLanguages = []string{"en"}
var selfServiceProfilePages = []string{"get-started"}

func (f *selfServiceProfileFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	var page int
	for {
		profiles, err := f.api.SelfServiceProfile.List(ctx, management.Page(page))
		if err != nil {
			return nil, err
		}

		for _, profile := range profiles.SelfServiceProfile {
			data = append(data, importDataItem{
				ResourceName: "auth0_self_service_profile." + sanitizeResourceName(profile.GetName()),
				ImportID:     profile.GetID(),
			})

			for _, lang := range selfServiceProfileLanguages {
				for _, page := range selfServiceProfilePages {
					customText, err := f.api.SelfServiceProfile.GetCustomText(ctx, profile.GetID(), lang, page)
					if err != nil {
						return nil, err
					}
					if len(customText) == 0 {
						continue
					}

					data = append(data, importDataItem{
						ResourceName: "auth0_self_service_profile_custom_text." + sanitizeResourceName(profile.GetName()+"_"+lang+"_"+page),
						ImportID:     profile.GetID() + "::" + lang + "::" + page,
					})
				}
			}
		}

		if !profiles.HasNext() {
			break
		}
	}

	return data, nil
}

func (f *tenantResourceFetcher) FetchData(_ context.Context) (importDataList, error) {
	return []importDataItem{
		{
			ResourceName: "auth0_tenant.tenant",
			ImportID:     uuid.NewString(),
		},
	}, nil
}

func (f *triggerActionsResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList
	triggers := []string{"post-login", "credentials-exchange", "pre-user-registration", "post-user-registration", "post-change-password", "send-phone-message", "password-reset-post-challenge", "custom-email-provider", "custom-phone-provider"}

	for _, trigger := range triggers {
		res, err := f.api.Action.Bindings(ctx, trigger)
		if err != nil {
			return nil, err
		}
		if len(res.Bindings) > 0 {
			data = append(data, importDataItem{
				ResourceName: "auth0_trigger_actions." + sanitizeResourceName(trigger),
				ImportID:     trigger,
			})
		}
	}

	return data, nil
}

func (f *userAttributeProfilesResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	from := ""
	for {
		profiles, err := f.api.UserAttributeProfile.List(ctx, management.From(from))
		if err != nil {
			return nil, err
		}

		for _, profile := range profiles.UserAttributeProfiles {
			data = append(data, importDataItem{
				ResourceName: "auth0_user_attribute_profile." + sanitizeResourceName(profile.GetName()),
				ImportID:     profile.GetID(),
			})
		}

		if !profiles.HasNext() {
			break
		}

		from = profiles.Next
	}

	return data, nil
}

func (f *actionResourceFetcher) FetchData(ctx context.Context) (importDataList, error) {
	var data importDataList

	var page int
	for {
		actions, err := f.api.Action.List(
			ctx,
			management.Page(page),
		)
		if err != nil {
			return nil, err
		}

		for _, action := range actions.Actions {
			data = append(data, importDataItem{
				ResourceName: "auth0_action." + sanitizeResourceName(action.GetName()),
				ImportID:     action.GetID(),
			})
		}

		if !actions.HasNext() {
			break
		}

		page++
	}

	return data, nil
}
