package cli

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/auth0/go-auth0/management"

	"github.com/gorilla/websocket"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/auth0/auth0-cli/internal/utils"
)

const (
	webServerPort            = "52649"
	webServerHost            = "localhost:" + webServerPort
	webServerURL             = "http://localhost:" + webServerPort
	fetchBrandingMessageType = "FETCH_BRANDING"
	fetchPromptMessageType   = "FETCH_PROMPT"
	saveBrandingMessageType  = "SAVE_BRANDING"
	fetchPartialMessageType  = "FETCH_PARTIAL"
	fetchPartialFeatureFlag  = "FETCH_PARTIALS_FEATURE_FLAG"
	errorMessageType         = "ERROR"
	successMessageType       = "SUCCESS"
	standardMode             = "standard"
	advancedMode             = "advanced"

	sunsetDate        = "2026-06-15" // 6 months from GA.
	warningPeriodDays = 30           // Show urgent warnings 30 days before sunset.
)

var (
	//go:embed data/universal-login/*
	universalLoginPreviewAssets embed.FS

	ErrNoChangesDetected = fmt.Errorf("no changes detected")
)

var (
	renderingMode = Flag{
		Name:      "Rendering Mode",
		LongForm:  "rendering-mode",
		ShortForm: "r",
		Help: fmt.Sprintf(
			"%s\n%s\n",
			"standardMode is recommended for customizating consistent, branded experience for users.",
			"Alternatively, advancedMode is recommended for full customization/granular control of the login experience and to integrate own component design system",
		),
		IsRequired: true,
	}

	promptName = Flag{
		Name:       "Prompt Name",
		LongForm:   "prompt",
		ShortForm:  "p",
		Help:       "Name of the prompt to to switch or customize.",
		IsRequired: true,
	}

	screenNameFlag = Flag{
		Name:       "Screen Name",
		LongForm:   "screen",
		ShortForm:  "s",
		Help:       "Name of the screen to to switch or customize.",
		IsRequired: true,
	}

	fileFlag = Flag{
		Name:       "File",
		LongForm:   "settings-file",
		ShortForm:  "f",
		Help:       "File to save the rendering configs to.",
		IsRequired: false,
	}
)

var allowedPromptsWithPartials = []management.PromptType{
	management.PromptSignup,
	management.PromptSignupID,
	management.PromptSignupPassword,
	management.PromptLogin,
	management.PromptLoginID,
	management.PromptLoginPassword,
	management.PromptLoginPasswordLess,
}

var PromptScreenMap = map[string][]string{
	"signup-id":                   {"signup-id"},
	"signup-password":             {"signup-password"},
	"login-id":                    {"login-id"},
	"login-password":              {"login-password"},
	"login-passwordless":          {"login-passwordless-email-code", "login-passwordless-sms-otp"},
	"phone-identifier-enrollment": {"phone-identifier-enrollment"},
	"phone-identifier-challenge":  {"phone-identifier-challenge"},
	"email-identifier-challenge":  {"email-identifier-challenge"},
	"passkeys":                    {"passkey-enrollment", "passkey-enrollment-local"},
	"captcha":                     {"interstitial-captcha"},
	"login":                       {"login"},
	"signup":                      {"signup"},
	"reset-password": {"reset-password-request", "reset-password-email", "reset-password", "reset-password-success", "reset-password-error",
		"reset-password-mfa-email-challenge", "reset-password-mfa-otp-challenge", "reset-password-mfa-push-challenge-push",
		"reset-password-mfa-sms-challenge", "reset-password-mfa-phone-challenge", "reset-password-mfa-voice-challenge",
		"reset-password-mfa-recovery-code-challenge", "reset-password-mfa-webauthn-platform-challenge", "reset-password-mfa-webauthn-roaming-challenge"},
	"mfa":                      {"mfa-detect-browser-capabilities", "mfa-enroll-result", "mfa-begin-enroll-options", "mfa-login-options"},
	"mfa-email":                {"mfa-email-challenge", "mfa-email-list"},
	"mfa-sms":                  {"mfa-country-codes", "mfa-sms-challenge", "mfa-sms-enrollment", "mfa-sms-list"},
	"mfa-push":                 {"mfa-push-challenge-push", "mfa-push-enrollment-qr", "mfa-push-list", "mfa-push-welcome"},
	"invitation":               {"accept-invitation"},
	"organizations":            {"organization-selection", "organization-picker"},
	"mfa-otp":                  {"mfa-otp-challenge", "mfa-otp-enrollment-code", "mfa-otp-enrollment-qr"},
	"device-flow":              {"device-code-activation", "device-code-activation-allowed", "device-code-activation-denied", "device-code-confirmation"},
	"mfa-phone":                {"mfa-phone-challenge", "mfa-phone-enrollment"},
	"mfa-voice":                {"mfa-voice-challenge", "mfa-voice-enrollment"},
	"mfa-recovery-code":        {"mfa-recovery-code-challenge", "mfa-recovery-code-enrollment", "mfa-recovery-code-challenge-new-code"},
	"common":                   {"redeem-ticket"},
	"email-verification":       {"email-verification-result"},
	"login-email-verification": {"login-email-verification"},
	"logout":                   {"logout", "logout-aborted", "logout-complete"},
	"mfa-webauthn": {"mfa-webauthn-change-key-nickname", "mfa-webauthn-enrollment-success", "mfa-webauthn-error", "mfa-webauthn-platform-challenge",
		"mfa-webauthn-platform-enrollment", "mfa-webauthn-roaming-challenge", "mfa-webauthn-roaming-enrollment", "mfa-webauthn-not-available-error"},
	"consent":             {"consent"},
	"customized-consent":  {"customized-consent"},
	"email-otp-challenge": {"email-otp-challenge"},
}

type partialsData map[string]*management.PromptScreenPartials

type (
	universalLoginBrandingData struct {
		Applications []*applicationData                 `json:"applications"`
		Prompts      []*promptData                      `json:"prompts"`
		Partials     []partialsData                     `json:"partials"`
		Settings     *management.Branding               `json:"settings"`
		Template     *management.BrandingUniversalLogin `json:"template"`
		Theme        *management.BrandingTheme          `json:"theme"`
		Tenant       *tenantData                        `json:"tenant"`
	}

	applicationData struct {
		ID       string                 `json:"id"`
		Name     string                 `json:"name"`
		LogoURL  string                 `json:"logo_url"`
		Metadata map[string]interface{} `json:"metadata"`
	}

	promptData struct {
		Language   string                 `json:"language"`
		Prompt     string                 `json:"prompt"`
		CustomText map[string]interface{} `json:"custom_text,omitempty"`
	}

	partialData struct {
		InsertionPoint string `json:"insertion_point"`
		ScreenName     string `json:"screen_name"`
		PromptName     string `json:"prompt_name"`
	}

	partialFlagData struct {
		FeatureFlag bool `json:"feature_flag"`
	}

	tenantData struct {
		FriendlyName   string   `json:"friendly_name"`
		EnabledLocales []string `json:"enabled_locales"`
		Domain         string   `json:"domain"`
	}

	errorData struct {
		Error string `json:"error"`
	}

	successData struct {
		Success bool `json:"success"`
	}

	webSocketHandler struct {
		shutdown context.CancelFunc
		display  *display.Renderer
		api      *auth0.API
		tenant   string
	}

	webSocketMessage struct {
		Type    string      `json:"type"`
		Payload interface{} `json:"-"`
	}
)

// MarshalJSON implements the json.Marshaler interface.
func (m *webSocketMessage) MarshalJSON() ([]byte, error) {
	type message webSocketMessage
	type messageWrapper struct {
		*message
		RawPayload json.RawMessage `json:"payload"`
	}

	w := &messageWrapper{(*message)(m), nil}

	if m.Payload != nil {
		b, err := json.Marshal(m.Payload)
		if err != nil {
			return nil, err
		}

		w.RawPayload = b
	}

	return json.Marshal(w)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (m *webSocketMessage) UnmarshalJSON(b []byte) error {
	type message webSocketMessage
	type messageWrapper struct {
		*message
		RawPayload json.RawMessage `json:"payload"`
	}

	w := &messageWrapper{(*message)(m), nil}

	if err := json.Unmarshal(b, w); err != nil {
		return err
	}

	var payload interface{}

	switch m.Type {
	case fetchBrandingMessageType, saveBrandingMessageType:
		payload = &universalLoginBrandingData{}
	case fetchPromptMessageType:
		payload = &promptData{}
	case fetchPartialMessageType:
		payload = &partialData{}
	case fetchPartialFeatureFlag:
		payload = &partialFlagData{}
	default:
		payload = make(map[string]interface{})
	}

	if w.RawPayload != nil {
		if err := json.Unmarshal(w.RawPayload, &payload); err != nil {
			return err
		}
	}

	m.Payload = payload

	return nil
}

type promptScreen struct {
	filePath   string
	promptName string
	screenName string
}

func customizeUniversalLoginCmd(cli *cli) *cobra.Command {
	var (
		selectedRenderingMode string
		input                 promptScreen
	)

	cmd := &cobra.Command{
		Use:   "customize",
		Args:  cobra.NoArgs,
		Short: "⚠️ Customize Universal Login (Advanced mode DEPRECATED)",
		Long: "\nCustomize your Universal Login Experience. Note that this requires a custom domain to be configured for the tenant.\n\n" +
			"* Standard mode is recommended for creating a consistent, branded experience for users. Choosing Standard mode will open a webpage\n" +
			"  in your browser where you can edit and preview your branding changes. For a comprehensive list of editable parameters and their values,\n" +
			"  please visit the Management API documentation: https://auth0.com/docs/api/management/v2\n\n" +
			"⚠️  DEPRECATION NOTICE: Advanced mode will be deprecated on " + sunsetDate + "\n" +
			"   For future Advanced Customizations, use: auth0 acul config <command>\n\n" +
			"* Advanced mode is recommended for full customization and granular control of the login experience, allowing integration of your own component design system.\n" +
			"  Choosing Advanced mode will open the default terminal editor with rendering configurations in a settings.json file.\n\n" +
			"  Closing the terminal editor will save the settings to your tenant.",

		Example: `  auth0 universal-login customize
  auth0 ul customize`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := ensureCustomDomainIsEnabled(ctx, cli.api); err != nil {
				return err
			}

			if err := ensureNewUniversalLoginExperienceIsActive(ctx, cli.api); err != nil {
				return err
			}

			if selectedRenderingMode == "" {
				cli.renderer.Infof("Please select a rendering mode to customize:")
				if err := renderingMode.Select(cmd, &selectedRenderingMode, []string{standardMode, advancedMode}, auth0.String(standardMode)); err != nil {
					return err
				}
			}

			if selectedRenderingMode == advancedMode {
				if err := displayDeprecationStatus(cli, true); err != nil {
					return err
				}

				// Add visual separation and brief pause to ensure users see the warning.
				cli.renderer.Output(ansi.Bold("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
				cli.renderer.Output("")

				if !cli.noInput {
					time.Sleep(1 * time.Second) // Brief pause to let users read.
				}

				err := fetchPromptScreenInfo(cmd, cli, &input, "customize")
				if err != nil {
					return err
				}

				return advanceCustomize(cmd, cli, aculConfigInput{
					screenName: input.screenName,
					filePath:   input.filePath,
				})
			}

			// RenderingMode as standard.
			return startWebSocketServer(ctx, cli.api, cli.renderer, cli.tenant)
		},
	}

	renderingMode.RegisterString(cmd, &selectedRenderingMode, "")
	promptName.RegisterString(cmd, &input.promptName, "")
	screenNameFlag.RegisterString(cmd, &input.screenName, "")
	fileFlag.RegisterString(cmd, &input.filePath, "")

	return cmd
}

func fetchPromptScreenInfo(cmd *cobra.Command, cli *cli, input *promptScreen, action string) error {
	if input.promptName == "" {
		cli.renderer.Infof("Please select a prompt to %s its rendering mode:", action)
		if err := promptName.Select(cmd, &input.promptName, utils.FetchKeys(PromptScreenMap), nil); err != nil {
			return handleInputError(err)
		}
	}

	if input.screenName == "" {
		if len(PromptScreenMap[input.promptName]) > 1 {
			cli.renderer.Infof("Please select a screen to %s its rendering mode:", action)
			if err := screenName.Select(cmd, &input.screenName, PromptScreenMap[input.promptName], nil); err != nil {
				return handleInputError(err)
			}
		} else {
			input.screenName = PromptScreenMap[input.promptName][0]
		}
	}

	return nil
}

func ensureNewUniversalLoginExperienceIsActive(ctx context.Context, api *auth0.API) error {
	authenticationProfile, err := api.Prompt.Read(ctx)
	if err != nil {
		return err
	}

	if authenticationProfile.UniversalLoginExperience == "new" {
		return nil
	}

	return fmt.Errorf(
		"this feature requires the new Universal Login experience to be enabled for the tenant, " +
			"use `auth0 api patch prompts --data '{\"universal_login_experience\":\"new\"}'` to enable it",
	)
}

// displayDeprecationStatus displays timeline information.
// The `showCommands` flag controls whether to display helpful usage examples.
func displayDeprecationStatus(cli *cli, showCommands bool) error {
	parsedSunset, _ := time.Parse("2006-01-02", sunsetDate)
	now := time.Now()
	daysUntil := int(parsedSunset.Sub(now).Hours() / 24)
	formattedDate := ansi.Bold(ansi.Cyan(parsedSunset.Format("Jan 2, 2006")))

	switch {
	case daysUntil <= 0:
		cli.renderer.Errorf("Advanced rendering mode was retired on %s", formattedDate)
		cli.renderer.Errorf("   Use instead: %s", ansi.Green("auth0 acul config"))
		if showCommands {
			showNewConfigExamples(cli)
		}
		return fmt.Errorf("advanced rendering mode is no longer supported")

	case daysUntil <= warningPeriodDays:
		cli.renderer.Output("")
		cli.renderer.Output(ansi.Bold(ansi.Red("🚨 URGENT: DEPRECATION NOTICE 🚨")))
		cli.renderer.Warnf("%s will be retired soon (%d days left)",
			ansi.Bold("Advanced rendering mode"), daysUntil)
		cli.renderer.Warnf("   Switch to: %s", ansi.Bold(ansi.Cyan("auth0 acul config")))

		if showCommands {
			cli.renderer.Output("")
			showNewConfigExamples(cli)
		}

		cli.renderer.Warnf("⏳ Proceeding with advanced rendering mode (deprecated)")

	default:
		cli.renderer.Output("")
		cli.renderer.Output(ansi.Bold(ansi.Red("⚠️  DEPRECATION NOTICE ⚠️")))
		cli.renderer.Warnf("%s will be retired on %s (%d days remaining)",
			ansi.Bold("Advanced rendering mode"), formattedDate, daysUntil)
		cli.renderer.Warnf("   Try new commands: %s", ansi.Bold(ansi.Cyan("auth0 acul config")))
		if showCommands {
			cli.renderer.Output("")
			showNewConfigExamples(cli)
		}
	}

	cli.renderer.Output("")
	cli.renderer.Warnf("   For help: %s", ansi.Bold(ansi.Cyan("auth0 acul config --help")))
	cli.renderer.Output("")

	return nil
}

// showNewConfigExamples displays example commands for managing ACUL configurations.
func showNewConfigExamples(cli *cli) {
	cli.renderer.Warnf("  %s - Create config files", ansi.Yellow("auth0 acul config generate <screen>"))
	cli.renderer.Warnf("  %s - Download current settings", ansi.Yellow("auth0 acul config get <screen>"))
	cli.renderer.Warnf("  %s - Upload customizations", ansi.Yellow("auth0 acul config set <screen>"))
	cli.renderer.Warnf("  %s - View available screens", ansi.Yellow("auth0 acul config list"))
	cli.renderer.Output("")
	cli.renderer.Warnf("  %s", ansi.Bold("Quick Start:"))
	cli.renderer.Warnf("  1. %s", ansi.Cyan("auth0 acul config generate login-id"))
	cli.renderer.Warnf("  2. Edit the generated JSON file as needed")
	cli.renderer.Warnf("  3. %s", ansi.Cyan("auth0 acul config set login-id --file login-id.json"))
	cli.renderer.Output("")
}

func startWebSocketServer(
	ctx context.Context,
	api *auth0.API,
	display *display.Renderer,
	tenantDomain string,
) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	listener, err := net.Listen("tcp", webServerHost)
	if err != nil {
		return err
	}
	defer func() {
		_ = listener.Close()
	}()

	handler := &webSocketHandler{
		display:  display,
		api:      api,
		shutdown: cancel,
		tenant:   tenantDomain,
	}

	assetsWithoutPrefix, err := fs.Sub(universalLoginPreviewAssets, "data/universal-login")
	if err != nil {
		return err
	}

	router := http.NewServeMux()
	router.Handle("/", http.FileServer(http.FS(assetsWithoutPrefix)))
	router.Handle("/ws", handler)

	server := &http.Server{
		Handler:      router,
		ReadTimeout:  time.Minute * 10,
		WriteTimeout: time.Minute * 10,
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Serve(listener)
	}()

	openWebAppInBrowser(display)

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return server.Close()
	}
}

func openWebAppInBrowser(display *display.Renderer) {
	display.Infof("Perform your changes within the editor: %q", webServerURL)

	if err := browser.OpenURL(webServerURL); err != nil {
		display.Warnf("Failed to open the browser. Visit the URL manually.")
	}
}

func (h *webSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: checkOriginFunc,
	}

	connection, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.display.Errorf("Failed to upgrade the connection to the WebSocket protocol: %v", err)
		h.display.Warnf("Try restarting the command.")
		h.shutdown()
		return
	}
	defer func() {
		_ = connection.Close()
	}()

	connection.SetReadLimit(1e+6) // 1 MB.

	for {
		var message webSocketMessage
		if err := connection.ReadJSON(&message); err != nil {
			break
		}

		switch message.Type {
		case fetchBrandingMessageType:
			brandingData, err := fetchUniversalLoginBrandingData(r.Context(), h.api, h.tenant)
			if err != nil {
				h.display.Errorf("Failed to fetch Universal Login branding data: %v", err)

				errorMsg := webSocketMessage{
					Type: errorMessageType,
					Payload: &errorData{
						Error: err.Error(),
					},
				}

				if err := connection.WriteJSON(&errorMsg); err != nil {
					h.display.Errorf("Failed to send error message: %v", err)
				}

				continue
			}

			loadBrandingMsg := webSocketMessage{
				Type:    fetchBrandingMessageType,
				Payload: brandingData,
			}

			if err = connection.WriteJSON(&loadBrandingMsg); err != nil {
				h.display.Errorf("Failed to send branding data message: %v", err)
			}
		case fetchPromptMessageType:
			promptToFetch, ok := message.Payload.(*promptData)
			if !ok {
				h.display.Errorf("Invalid payload type: %T", message.Payload)
				continue
			}

			promptToSend, err := fetchPromptCustomTextWithDefaults(
				r.Context(),
				h.api,
				promptToFetch.Prompt,
				promptToFetch.Language,
			)
			if err != nil {
				h.display.Errorf("Failed to fetch custom text for prompt: %v", err)

				errorMsg := webSocketMessage{
					Type: errorMessageType,
					Payload: &errorData{
						Error: err.Error(),
					},
				}

				if err := connection.WriteJSON(&errorMsg); err != nil {
					h.display.Errorf("Failed to send error message: %v", err)
				}

				continue
			}

			fetchPromptMsg := webSocketMessage{
				Type:    fetchPromptMessageType,
				Payload: promptToSend,
			}

			if err = connection.WriteJSON(&fetchPromptMsg); err != nil {
				h.display.Errorf("Failed to send prompt data message: %v", err)
				continue
			}
		case saveBrandingMessageType:
			saveBrandingMsg, ok := message.Payload.(*universalLoginBrandingData)
			if !ok {
				h.display.Errorf("Invalid payload type: %T", message.Payload)
				continue
			}

			if err := saveUniversalLoginBrandingData(r.Context(), h.api, saveBrandingMsg); err != nil {
				h.display.Errorf("Failed to save branding data: %v", err)

				errorMsg := webSocketMessage{
					Type: errorMessageType,
					Payload: &errorData{
						Error: err.Error(),
					},
				}

				if err := connection.WriteJSON(&errorMsg); err != nil {
					h.display.Errorf("Failed to send error message: %v", err)
				}

				continue
			}

			successMsg := webSocketMessage{
				Type: successMessageType,
				Payload: &successData{
					Success: true,
				},
			}

			if err := connection.WriteJSON(&successMsg); err != nil {
				h.display.Errorf("Failed to send success message: %v", err)
			}
		case fetchPartialFeatureFlag:
			partial := &partialData{
				ScreenName: "login",
				PromptName: "login",
			}
			_, err := fetchPartial(r.Context(), h.api, partial)
			if err != nil && (strings.Contains(err.Error(), "feature is not available for your plan") || strings.Contains(err.Error(), "Your account does not have custom prompts")) {
				fetchPartialFlagMsg := webSocketMessage{
					Type:    fetchPartialFeatureFlag,
					Payload: &partialFlagData{FeatureFlag: false},
				}
				if err = connection.WriteJSON(&fetchPartialFlagMsg); err != nil {
					h.display.Errorf("Failed to send partial flag data message: %v", err)
					continue
				}
			} else {
				fetchPartialFlagMsg := webSocketMessage{
					Type:    fetchPartialFeatureFlag,
					Payload: &partialFlagData{FeatureFlag: true},
				}

				if err = connection.WriteJSON(&fetchPartialFlagMsg); err != nil {
					h.display.Errorf("Failed to send partial flag data message: %v", err)
					continue
				}
			}

		case fetchPartialMessageType:
			partialToFetch, ok := message.Payload.(*partialData)

			if !ok {
				h.display.Errorf("Invalid payload type: %T", message.Payload)
				continue
			}

			partialToSend, err := fetchPartial(r.Context(), h.api, partialToFetch)

			if err != nil {
				if strings.Contains(err.Error(), "feature is not available for your plan") || strings.Contains(err.Error(), "Your account does not have custom prompts") {
					partialToSend = &management.PromptScreenPartials{}
				} else {
					h.display.Errorf("Failed to fetch partial for prompt: %v", err)
					errorMsg := webSocketMessage{
						Type: errorMessageType,
						Payload: &errorData{
							Error: err.Error(),
						},
					}

					if err := connection.WriteJSON(&errorMsg); err != nil {
						h.display.Errorf("Failed to send error message: %v", err)
					}

					continue
				}
			}

			fetchPartialMsg := webSocketMessage{
				Type:    fetchPartialMessageType,
				Payload: partialToSend,
			}

			if err = connection.WriteJSON(&fetchPartialMsg); err != nil {
				h.display.Errorf("Failed to send prompt data message: %v", err)
				continue
			}
		}
	}
}

func isSupportedPartial(givenPrompt management.PromptType) bool {
	for _, prompt := range allowedPromptsWithPartials {
		if givenPrompt == prompt {
			return true
		}
	}

	return false
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

	return originURL.String() == webServerURL
}

func fetchUniversalLoginBrandingData(
	ctx context.Context,
	api *auth0.API,
	tenantDomain string,
) (*universalLoginBrandingData, error) {
	group, ctx := errgroup.WithContext(ctx)

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

	var applications []*applicationData
	group.Go(func() (err error) {
		applications, err = fetchAllApplications(ctx, api)
		return err
	})

	var partials []partialsData
	group.Go(func() (err error) {
		partials, err = fetchAllPartials(ctx, api)
		return err
	})

	if err := group.Wait(); err != nil {
		return nil, err
	}

	return &universalLoginBrandingData{
		Applications: applications,
		Settings:     brandingSettings,
		Template:     currentTemplate,
		Theme:        currentTheme,
		Tenant: &tenantData{
			FriendlyName:   tenant.GetFriendlyName(),
			EnabledLocales: tenant.GetEnabledLocales(),
			Domain:         tenantDomain,
		},
		Prompts:  []*promptData{prompt},
		Partials: partials,
	}, nil
}

func fetchBrandingThemeOrUseDefault(ctx context.Context, api *auth0.API) *management.BrandingTheme {
	currentTheme, err := api.BrandingTheme.Default(ctx)
	if err == nil {
		currentTheme.ID = nil
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
			CaptchaWidgetTheme:      "auto",
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

	customText := make(map[string]interface{})
	for key, value := range brandingTextTranslations {
		customText[key] = value
	}

	return &promptData{
		Language:   language,
		Prompt:     promptName,
		CustomText: customText,
	}, nil
}

func fetchAllApplications(ctx context.Context, api *auth0.API) ([]*applicationData, error) {
	var applications []*applicationData
	var page int
	for {
		clientList, err := api.Client.List(
			ctx,
			management.Page(page),
			management.PerPage(100),
			management.Parameter("is_global", "false"),
			management.IncludeFields("client_id", "name", "logo_uri", "client_metadata"),
		)
		if err != nil {
			return nil, err
		}

		for _, client := range clientList.Clients {
			applications = append(applications, &applicationData{
				ID:       client.GetClientID(),
				Name:     client.GetName(),
				LogoURL:  client.GetLogoURI(),
				Metadata: client.GetClientMetadata(),
			})
		}

		if !clientList.HasNext() {
			break
		}

		page++
	}

	return applications, nil
}

func fetchPartial(ctx context.Context, api *auth0.API, prompt *partialData) (*management.PromptScreenPartials, error) {
	var filteredPartials = management.PromptScreenPartials{}

	if !isSupportedPartial(management.PromptType(prompt.PromptName)) {
		return &management.PromptScreenPartials{}, nil
	}

	partial, err := api.Prompt.GetPartials(ctx, management.PromptType(prompt.PromptName))
	if err != nil {
		return nil, err
	}

	if partial == nil {
		return &management.PromptScreenPartials{}, nil
	}

	if screenPartials, ok := (*partial)[management.ScreenName(prompt.ScreenName)]; ok {
		filteredPartials[management.ScreenName(prompt.ScreenName)] = screenPartials
	}

	return &filteredPartials, nil
}

func fetchAllPartials(ctx context.Context, api *auth0.API) ([]partialsData, error) {
	var partialsList []partialsData

	for _, prompt := range allowedPromptsWithPartials {
		partial, err := api.Prompt.GetPartials(ctx, prompt)
		if err != nil {
			if strings.Contains(err.Error(), "feature is not available for your plan") ||
				strings.Contains(err.Error(), "Your account does not have custom prompts") {
				constructedPartial := partialsData{
					string(prompt): &management.PromptScreenPartials{},
				}
				partialsList = append(partialsList, constructedPartial)
				continue
			}
			return nil, err
		}

		constructedPartial := partialsData{
			string(prompt): partial,
		}
		partialsList = append(partialsList, constructedPartial)
	}

	return partialsList, nil
}

func saveUniversalLoginBrandingData(ctx context.Context, api *auth0.API, data *universalLoginBrandingData) error {
	group, ctx := errgroup.WithContext(ctx)

	if data.Settings != nil && data.Settings.String() != "{}" {
		group.Go(func() error {
			return api.Branding.Update(ctx, data.Settings)
		})
	}

	if data.Template != nil && data.Template.String() != "{}" {
		group.Go(func() error {
			return api.Branding.SetUniversalLogin(ctx, data.Template)
		})
	}

	if data.Theme != nil && data.Theme.String() != "{}" {
		group.Go(func() error {
			existingTheme, err := api.BrandingTheme.Default(ctx)
			if err == nil {
				return api.BrandingTheme.Update(ctx, existingTheme.GetID(), data.Theme)
			}
			return api.BrandingTheme.Create(ctx, data.Theme)
		})
	}

	for _, prompt := range data.Prompts {
		prompt := prompt
		group.Go(func() error {
			return api.Prompt.SetCustomText(ctx, prompt.Prompt, prompt.Language, prompt.CustomText)
		})
	}

	for _, partials := range data.Partials {
		for promptName, screenPartials := range partials {
			if screenPartials != nil {
				promptName := promptName
				group.Go(func() error {
					err := api.Prompt.SetPartials(ctx, management.PromptType(promptName), screenPartials)
					if err != nil && (strings.Contains(err.Error(), "feature is not available for your plan") || strings.Contains(err.Error(), "Your account does not have custom prompts")) {
						return nil
					}
					return err
				})
			}
		}
	}

	return group.Wait()
}

func switchUniversalLoginRendererModeCmd(cli *cli) *cobra.Command {
	var (
		selectedRenderingMode string
		input                 promptScreen
	)

	cmd := &cobra.Command{
		Use:   "switch",
		Args:  cobra.NoArgs,
		Short: "⚠️ Switch rendering mode (DEPRECATED)",
		Long: `Switch the rendering mode for Universal Login. Note that this requires a custom domain to be configured for the tenant.

🚨 DEPRECATION WARNING: The 'auth0 ul switch' command will be DEPRECATED on June 15, 2026
        
✅ For Advanced Customizations, Use new ACUL config commands:
  • auth0 acul config generate <screen>
  • auth0 acul config get <screen>  
  • auth0 acul config set <screen>
  • auth0 acul config list`,
		Example: `  auth0 universal-login switch
  auth0 universal-login switch --prompt login-id --screen login-id --rendering-mode standard
  auth0 ul switch --prompt login-id --screen login-id --rendering-mode advanced
  auth0 ul switch -p login-id -s login-id -r standard`,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := displayDeprecationStatus(cli, false)
			if err != nil {
				return err
			}

			err = fetchPromptScreenInfo(cmd, cli, &input, "switch")
			if err != nil {
				return err
			}

			if selectedRenderingMode == "" {
				cli.renderer.Infof("Please select a rendering mode to switch:")
				if err = renderingMode.Select(cmd, &selectedRenderingMode, []string{advancedMode, standardMode}, nil); err != nil {
					return err
				}
			}

			if err = ansi.Waiting(func() error {
				rendererMode := management.RenderingMode(selectedRenderingMode)
				return cli.api.Prompt.UpdateRendering(cmd.Context(), management.PromptType(input.promptName), management.ScreenName(input.screenName), &management.PromptRendering{RenderingMode: &rendererMode})
			}); err != nil {
				return fmt.Errorf("failed to switch the rendering mode for the prompt - %s, screen - %s : %w", ansi.Green(input.promptName), ansi.Green(input.screenName), err)
			}

			cli.renderer.Infof(
				"Successfully switched the rendering mode to %s for Prompt: %s and Screen: %s\n",
				ansi.Green(selectedRenderingMode),
				ansi.Green(input.promptName),
				ansi.Green(input.screenName),
			)

			cli.renderer.Infof("Use `auth0 universal-login customize` to customize the Universal Login Experience\n")

			return nil
		},
	}

	promptName.RegisterString(cmd, &input.promptName, "")
	screenNameFlag.RegisterString(cmd, &input.screenName, "")
	renderingMode.RegisterString(cmd, &selectedRenderingMode, "")

	return cmd
}
