package cli

import (
	"context"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
)

type (
	universalLoginBrandingData struct {
		AuthenticationProfile *management.Prompt                 `json:"auth_profile"`
		Settings              *management.Branding               `json:"settings"`
		Template              *management.BrandingUniversalLogin `json:"template"`
		Theme                 *management.BrandingTheme          `json:"theme"`
		Tenant                *tenantData                        `json:"tenant"`
		Prompt                *promptData                        `json:"prompt"`
	}

	tenantData struct {
		FriendlyName   string   `json:"friendly_name"`
		EnabledLocales []string `json:"enabled_locales"`
		Domain         string   `json:"domain"`
	}

	promptData struct {
		Language   string      `json:"language"`
		Prompt     string      `json:"prompt"`
		CustomText interface{} `json:"custom_text"`
	}
)

func customizeUniversalLoginCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "customize",
		Args:  cobra.NoArgs,
		Short: "Customize the Universal Login experience",
		Long: "Customize and preview changes to the Universal Login experience. This command will open a webpage " +
			"within your browser where you can edit and preview your branding changes. For a comprehensive list of " +
			"editable parameters and their values please visit the " +
			"[Management API Documentation](https://auth0.com/docs/api/management/v2).",
		Example: `  auth0 universal-login customize
  auth0 ul customize`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var universalLoginBrandingData *universalLoginBrandingData

			if err := ansi.Spinner("Fetching Universal Login branding data", func() (err error) {
				universalLoginBrandingData, err = fetchUniversalLoginBrandingData(cmd.Context(), cli.api, cli.tenant)
				return err
			}); err != nil {
				return err
			}

			cli.renderer.JSONResult(universalLoginBrandingData)

			return nil
		},
	}

	return cmd
}

func fetchUniversalLoginBrandingData(
	ctx context.Context,
	api *auth0.API,
	tenantDomain string,
) (*universalLoginBrandingData, error) {
	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() (err error) {
		return ensureCustomDomainIsEnabled(ctx, api)
	})

	var authenticationProfile *management.Prompt
	group.Go(func() (err error) {
		authenticationProfile, err = api.Prompt.Read(ctx)
		return err
	})

	var brandingSettings *management.Branding
	group.Go(func() (err error) {
		brandingSettings = fetchBrandingSettingsOrUseDefaults(ctx, api)
		return nil
	})

	var currentTemplate *management.BrandingUniversalLogin
	group.Go(func() (err error) {
		currentTemplate = fetchBrandingTemplateOrUseEmpty(ctx, api)
		return nil
	})

	var currentTheme *management.BrandingTheme
	group.Go(func() (err error) {
		currentTheme = fetchBrandingThemeOrUseDefault(ctx, api)
		return nil
	})

	var tenant *management.Tenant
	var prompt *promptData
	group.Go(func() (err error) {
		tenant, err = api.Tenant.Read(ctx)
		if err != nil {
			return err
		}

		defaultPrompt := "login"
		defaultLanguage := "en"
		if len(tenant.GetEnabledLocales()) > 0 {
			defaultLanguage = tenant.GetEnabledLocales()[0]
		}

		prompt, err = fetchPromptCustomTextWithDefaults(ctx, api, defaultPrompt, defaultLanguage)
		return err
	})

	if err := group.Wait(); err != nil {
		return nil, err
	}

	return &universalLoginBrandingData{
		AuthenticationProfile: authenticationProfile,
		Settings:              brandingSettings,
		Template:              currentTemplate,
		Theme:                 currentTheme,
		Tenant: &tenantData{
			FriendlyName:   tenant.GetFriendlyName(),
			EnabledLocales: tenant.GetEnabledLocales(),
			Domain:         tenantDomain,
		},
		Prompt: prompt,
	}, nil
}

func fetchBrandingThemeOrUseDefault(ctx context.Context, api *auth0.API) *management.BrandingTheme {
	currentTheme, err := api.BrandingTheme.Default(ctx)
	if err == nil {
		return currentTheme
	}

	return &management.BrandingTheme{
		Borders: management.BrandingThemeBorders{
			ButtonBorderRadius: 3,
			ButtonBorderWeight: 1,
			ButtonsStyle:       "rounded",
			InputBorderRadius:  3,
			InputBorderWeight:  1,
			InputsStyle:        "rounded",
			ShowWidgetShadow:   true,
			WidgetBorderWeight: 0,
			WidgetCornerRadius: 5,
		},
		Colors: management.BrandingThemeColors{
			BaseFocusColor:          auth0.String("#635dff"),
			BaseHoverColor:          auth0.String("#000000"),
			BodyText:                "#1e212a",
			Error:                   "#d03c38",
			Header:                  "#1e212a",
			Icons:                   "#65676e",
			InputBackground:         "#ffffff",
			InputBorder:             "#c9cace",
			InputFilledText:         "#000000",
			InputLabelsPlaceholders: "#65676e",
			LinksFocusedComponents:  "#635dff",
			PrimaryButton:           "#635dff",
			PrimaryButtonLabel:      "#ffffff",
			SecondaryButtonBorder:   "#c9cace",
			SecondaryButtonLabel:    "#1e212a",
			Success:                 "#13a688",
			WidgetBackground:        "#ffffff",
			WidgetBorder:            "#c9cace",
		},
		Fonts: management.BrandingThemeFonts{
			BodyText: management.BrandingThemeText{
				Bold: false,
				Size: 87.5,
			},
			ButtonsText: management.BrandingThemeText{
				Bold: false,
				Size: 100.0,
			},
			FontURL: "",
			InputLabels: management.BrandingThemeText{
				Bold: false,
				Size: 100.0,
			},
			Links: management.BrandingThemeText{
				Bold: true,
				Size: 87.5,
			},
			LinksStyle:        "normal",
			ReferenceTextSize: 16.0,
			Subtitle: management.BrandingThemeText{
				Bold: false,
				Size: 87.5,
			},
			Title: management.BrandingThemeText{
				Bold: false,
				Size: 150.0,
			},
		},
		PageBackground: management.BrandingThemePageBackground{
			BackgroundColor:    "#000000",
			BackgroundImageURL: "",
			PageLayout:         "center",
		},
		Widget: management.BrandingThemeWidget{
			HeaderTextAlignment: "center",
			LogoHeight:          52.0,
			LogoPosition:        "center",
			LogoURL:             "",
			SocialButtonsLayout: "bottom",
		},
	}
}

func fetchPromptCustomTextWithDefaults(
	ctx context.Context,
	api *auth0.API,
	promptName string,
	language string,
) (*promptData, error) {
	customTranslations, err := api.Prompt.CustomText(ctx, promptName, language)
	if err != nil {
		return nil, err
	}

	defaultTranslations := downloadDefaultBrandingTextTranslations(promptName, language)

	brandingTextTranslations := mergeBrandingTextTranslations(defaultTranslations, customTranslations)

	return &promptData{
		Language:   language,
		Prompt:     promptName,
		CustomText: brandingTextTranslations,
	}, nil
}
