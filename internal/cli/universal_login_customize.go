package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/auth0/go-auth0/management"
	"github.com/gorilla/websocket"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
)

const webAppURL = "http://localhost:5173"

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
		Language   string                            `json:"language"`
		Prompt     string                            `json:"prompt"`
		CustomText map[string]map[string]interface{} `json:"custom_text"`
	}

	webSocketHandler struct {
		shutdown     context.CancelFunc
		display      *display.Renderer
		api          *auth0.API
		brandingData *universalLoginBrandingData
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
			ctx := cmd.Context()

			if err := ensureCustomDomainIsEnabled(ctx, cli.api); err != nil {
				return err
			}

			var universalLoginBrandingData *universalLoginBrandingData

			if err := ansi.Spinner("Fetching Universal Login branding data", func() (err error) {
				universalLoginBrandingData, err = fetchUniversalLoginBrandingData(ctx, cli.api, cli.tenant)
				return err
			}); err != nil {
				return err
			}

			return startWebSocketServer(ctx, cli.api, cli.renderer, universalLoginBrandingData)
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
		defaultLanguage := tenant.GetEnabledLocales()[0]

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

func startWebSocketServer(
	ctx context.Context,
	api *auth0.API,
	display *display.Renderer,
	brandingData *universalLoginBrandingData,
) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close()

	handler := &webSocketHandler{
		display:      display,
		api:          api,
		shutdown:     cancel,
		brandingData: brandingData,
	}

	server := &http.Server{
		Handler:      handler,
		ReadTimeout:  time.Minute * 10,
		WriteTimeout: time.Minute * 10,
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Serve(listener)
	}()

	openWebAppInBrowser(display, listener.Addr())

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return server.Close()
	}
}

func openWebAppInBrowser(display *display.Renderer, addr net.Addr) {
	port := addr.(*net.TCPAddr).Port
	webAppURLWithPort := fmt.Sprintf("%s?ws_port=%d", webAppURL, port)

	display.Infof("Perform your changes within the editor: %q", webAppURLWithPort)

	if err := browser.OpenURL(webAppURLWithPort); err != nil {
		display.Warnf("Failed to open the browser. Visit the URL manually.")
	}
}

func (h *webSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: checkOriginFunc,
	}

	connection, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.display.Errorf("error accepting WebSocket connection: %v", err)
		h.shutdown()
		return
	}

	connection.SetReadLimit(1e+6) // 1 MB.

	if err = connection.WriteJSON(h.brandingData); err != nil {
		h.display.Errorf("failed to write json message: %v", err)
		h.shutdown()
		return
	}
}

func checkOriginFunc(r *http.Request) bool {
	origin := r.Header["Origin"]
	if len(origin) == 0 {
		return false
	}

	originURL, err := url.Parse(origin[0])
	if err != nil {
		return false
	}

	return originURL.String() == webAppURL
}
