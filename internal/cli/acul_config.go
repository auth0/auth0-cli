package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/auth0/go-auth0/management"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/auth0/auth0-cli/internal/utils"
)

var (
	screenName = Flag{
		Name:       "Screen Name",
		LongForm:   "screen",
		ShortForm:  "s",
		Help:       "Name of the screen to customize.",
		IsRequired: true,
	}
	file = Flag{
		Name:       "File",
		LongForm:   "file",
		ShortForm:  "f",
		Help:       "File to save the rendering configs to.",
		IsRequired: false,
	}
	rendererScript = Flag{
		Name:       "Script",
		LongForm:   "script",
		Help:       "Script contents for the rendering configs.",
		IsRequired: true,
	}
	fieldsFlag = Flag{
		Name:       "Fields",
		LongForm:   "fields",
		Help:       "Comma-separated list of fields to include or exclude in the result (based on value provided for include_fields) ",
		IsRequired: false,
	}
	includeFieldsFlag = Flag{
		Name:       "Include Fields",
		LongForm:   "include-fields",
		Help:       "Whether specified fields are to be included (true) or excluded (false).",
		IsRequired: false,
	}
	includeTotalsFlag = Flag{
		Name:       "Include Totals",
		LongForm:   "include-totals",
		Help:       "Return results inside an object that contains the total result count (true) or as a direct array of results (false).",
		IsRequired: false,
	}
	pageFlag = Flag{
		Name:       "Page",
		LongForm:   "page",
		Help:       "Page index of the results to return. First page is 0.",
		IsRequired: false,
	}
	perPageFlag = Flag{
		Name:       "Per Page",
		LongForm:   "per-page",
		Help:       "Number of results per page. Default value is 50, maximum value is 100.",
		IsRequired: false,
	}
	promptFlag = Flag{
		Name:       "Prompt",
		LongForm:   "prompt",
		Help:       "Filter by the Universal Login prompt.",
		IsRequired: false,
	}
	screenFlag = Flag{
		Name:       "Screen",
		LongForm:   "screen",
		Help:       "Filter by the Universal Login screen.",
		IsRequired: false,
	}
	renderingModeFlag = Flag{
		Name:       "Rendering Mode",
		LongForm:   "rendering-mode",
		Help:       "Filter by the rendering mode (advanced or standard).",
		IsRequired: false,
	}
	queryFlag = Flag{
		Name:       "Query",
		LongForm:   "query",
		ShortForm:  "q",
		Help:       "Advanced query.",
		IsRequired: false,
	}
	ScreenPromptMap = map[string]string{
		"signup-id":                                      "signup-id",
		"signup-password":                                "signup-password",
		"login-id":                                       "login-id",
		"login-password":                                 "login-password",
		"login-passwordless-email-code":                  "login-passwordless",
		"login-passwordless-sms-otp":                     "login-passwordless",
		"phone-identifier-enrollment":                    "phone-identifier-enrollment",
		"phone-identifier-challenge":                     "phone-identifier-challenge",
		"email-identifier-challenge":                     "email-identifier-challenge",
		"passkey-enrollment":                             "passkeys",
		"passkey-enrollment-local":                       "passkeys",
		"interstitial-captcha":                           "captcha",
		"login":                                          "login",
		"signup":                                         "signup",
		"reset-password-request":                         "reset-password",
		"reset-password-email":                           "reset-password",
		"reset-password":                                 "reset-password",
		"reset-password-success":                         "reset-password",
		"reset-password-error":                           "reset-password",
		"reset-password-mfa-email-challenge":             "reset-password",
		"reset-password-mfa-otp-challenge":               "reset-password",
		"reset-password-mfa-push-challenge-push":         "reset-password",
		"reset-password-mfa-sms-challenge":               "reset-password",
		"reset-password-mfa-phone-challenge":             "reset-password",
		"reset-password-mfa-voice-challenge":             "reset-password",
		"reset-password-mfa-recovery-code-challenge":     "reset-password",
		"reset-password-mfa-webauthn-platform-challenge": "reset-password",
		"reset-password-mfa-webauthn-roaming-challenge":  "reset-password",
		"mfa-detect-browser-capabilities":                "mfa",
		"mfa-enroll-result":                              "mfa",
		"mfa-begin-enroll-options":                       "mfa",
		"mfa-login-options":                              "mfa",
		"mfa-email-challenge":                            "mfa-email",
		"mfa-email-list":                                 "mfa-email",
		"mfa-country-codes":                              "mfa-sms",
		"mfa-sms-challenge":                              "mfa-sms",
		"mfa-sms-enrollment":                             "mfa-sms",
		"mfa-sms-list":                                   "mfa-sms",
		"mfa-push-challenge-push":                        "mfa-push",
		"mfa-push-enrollment-qr":                         "mfa-push",
		"mfa-push-list":                                  "mfa-push",
		"mfa-push-welcome":                               "mfa-push",
		"accept-invitation":                              "invitation",
		"organization-selection":                         "organizations",
		"organization-picker":                            "organizations",
		"mfa-otp-challenge":                              "mfa-otp",
		"mfa-otp-enrollment-code":                        "mfa-otp",
		"mfa-otp-enrollment-qr":                          "mfa-otp",
		"device-code-activation":                         "device-flow",
		"device-code-activation-allowed":                 "device-flow",
		"device-code-activation-denied":                  "device-flow",
		"device-code-confirmation":                       "device-flow",
		"mfa-phone-challenge":                            "mfa-phone",
		"mfa-phone-enrollment":                           "mfa-phone",
		"mfa-voice-challenge":                            "mfa-voice",
		"mfa-voice-enrollment":                           "mfa-voice",
		"mfa-recovery-code-challenge":                    "mfa-recovery-code",
		"mfa-recovery-code-enrollment":                   "mfa-recovery-code",
		"mfa-recovery-code-challenge-new-code":           "mfa-recovery-code",
		"redeem-ticket":                                  "common",
		"email-verification-result":                      "email-verification",
		"login-email-verification":                       "login-email-verification",
		"logout":                                         "logout",
		"logout-aborted":                                 "logout",
		"logout-complete":                                "logout",
		"mfa-webauthn-change-key-nickname":               "mfa-webauthn",
		"mfa-webauthn-enrollment-success":                "mfa-webauthn",
		"mfa-webauthn-error":                             "mfa-webauthn",
		"mfa-webauthn-platform-challenge":                "mfa-webauthn",
		"mfa-webauthn-platform-enrollment":               "mfa-webauthn",
		"mfa-webauthn-roaming-challenge":                 "mfa-webauthn",
		"mfa-webauthn-roaming-enrollment":                "mfa-webauthn",
	}
)

type aculConfigInput struct {
	screenName string
	filePath   string
}

// ensureConfigFilePath sets a default config file path if none is provided and creates the config directory.
func ensureConfigFilePath(input *aculConfigInput, cli *cli) error {
	if input.filePath == "" {
		input.filePath = fmt.Sprintf("acul_config/%s.json", input.screenName)
		cli.renderer.Warnf("No configuration file path specified. Defaulting to '%s'.", ansi.Green(input.filePath))
	}
	if err := os.MkdirAll("acul_config", 0755); err != nil {
		return fmt.Errorf("could not create config directory: %w", err)
	}
	return nil
}

// Generate default ACUL config stub.
func defaultACULConfig() map[string]interface{} {
	return map[string]interface{}{
		"rendering_mode":             "standard",
		"context_configuration":      []interface{}{},
		"use_page_template":          false,
		"default_head_tags_disabled": false,
		"head_tags":                  []interface{}{},
		"filters":                    map[string]interface{}{},
	}
}

func aculConfigGenerateCmd(cli *cli) *cobra.Command {
	var input aculConfigInput

	cmd := &cobra.Command{
		Use:   "generate",
		Args:  cobra.MaximumNArgs(1),
		Short: "Generate a stub config file for a Universal Login screen.",
		Long: "Generate a stub config file for a Universal Login screen and save it to a file.\n" +
			"If fileName is not provided, it will default to <screen-name>.json in the current directory.",
		Example: `  auth0 acul config generate <screen-name>
  auth0 acul config generate <screen-name> --file settings.json
  auth0 acul config generate signup-id
  auth0 acul config generate login-id --file login-settings.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureACULPrerequisites(cmd.Context(), cli.api); err != nil {
				return err
			}

			screens, err := validateAndSelectScreens(cli, utils.FetchKeys(ScreenPromptMap), args, false)
			if err != nil {
				return err
			}

			input.screenName = screens[0]

			if err := ensureConfigFilePath(&input, cli); err != nil {
				return err
			}

			config := defaultACULConfig()

			// Error handling omitted for brevity.
			data, _ := json.MarshalIndent(config, "", "  ")

			message := fmt.Sprintf("Overwrite file '%s' with default config? : ", ansi.Green(input.filePath))
			if shouldCancelOverwrite(cli, cmd, input.filePath, message) {
				return nil
			}

			if err := os.WriteFile(input.filePath, data, 0644); err != nil {
				return fmt.Errorf("could not write config: %w", err)
			}

			cli.renderer.Infof("Configuration generated at '%s'", ansi.Green(input.filePath))

			cli.renderer.Output("Learn more about configuring ACUL screens https://auth0.com/docs/customize/login-pages/advanced-customizations/getting-started/configure-acul-screens")

			cli.renderer.Output(ansi.Yellow("💡 Tip: Use `auth0 acul config get` to fetch remote rendering settings or `auth0 acul config set` to sync local configs."))
			return nil
		},
	}

	file.RegisterString(cmd, &input.filePath, "")
	return cmd
}

func aculConfigGetCmd(cli *cli) *cobra.Command {
	var input aculConfigInput

	cmd := &cobra.Command{
		Use:   "get",
		Args:  cobra.MaximumNArgs(1),
		Short: "Get the current rendering settings for a specific screen",
		Long:  "Get the current rendering settings for a specific screen.",
		Example: `  auth0 acul config get <screen-name>
  auth0 acul config get <screen-name> --file settings.json
  auth0 acul config get signup-id
  auth0 acul config get login-id -f ./acul_config/login-id.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if err := ensureACULPrerequisites(cmd.Context(), cli.api); err != nil {
				return err
			}

			screens, err := validateAndSelectScreens(cli, utils.FetchKeys(ScreenPromptMap), args, false)
			if err != nil {
				return err
			}

			input.screenName = screens[0]

			existingRenderSettings, err := cli.api.Prompt.ReadRendering(ctx, management.PromptType(ScreenPromptMap[input.screenName]), management.ScreenName(input.screenName))
			if err != nil {
				return fmt.Errorf("failed to fetch the existing render settings: %w", err)
			}

			if existingRenderSettings == nil {
				cli.renderer.Warnf("No rendering settings found for screen '%s' in tenant '%s'.", ansi.Green(input.screenName), ansi.Blue(cli.tenant))
				cli.renderer.Output(ansi.Yellow("💡 Tip: Use `auth0 acul config generate` to generate a stub config file or `auth0 acul config set` to sync local configs."))
				cli.renderer.Output(ansi.Cyan("📖 Customization Guide: https://github.com/auth0/auth0-cli/blob/main/CUSTOMIZATION_GUIDE.md"))
				return nil
			}

			if err := ensureConfigFilePath(&input, cli); err != nil {
				return err
			}

			message := fmt.Sprintf("Overwrite file '%s' with new data from tenant '%s'? : ", ansi.Green(input.filePath), ansi.Blue(cli.tenant))
			if shouldCancelOverwrite(cli, cmd, input.filePath, message) {
				return nil
			}

			data, err := json.MarshalIndent(existingRenderSettings, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal render settings: %w", err)
			}
			if err := os.WriteFile(input.filePath, data, 0644); err != nil {
				return fmt.Errorf("failed to write render settings to file %q: %w", input.filePath, err)
			}

			cli.renderer.Infof("Configuration downloaded and saved at '%s'.", ansi.Green(input.filePath))
			cli.renderer.Output(ansi.Yellow("💡 Tip: Use `auth0 acul config set` to sync local config to remote or `auth0 acul config list` to view all ACUL screens."))
			return nil
		},
	}

	file.RegisterString(cmd, &input.filePath, "")

	return cmd
}

func shouldCancelOverwrite(cli *cli, cmd *cobra.Command, filePath, message string) bool {
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return false
	}
	if !cli.force && canPrompt(cmd) {
		if confirmed := prompt.Confirm(message); !confirmed {
			return true
		}
	}

	return false
}

func aculConfigSetCmd(cli *cli) *cobra.Command {
	var input aculConfigInput

	cmd := &cobra.Command{
		Use:   "set",
		Args:  cobra.MaximumNArgs(1),
		Short: "Set the rendering settings for a specific screen",
		Long:  "Set the rendering settings for a specific screen.",
		Example: `  auth0 acul config set <screen-name>
  auth0 acul config set <screen-name> --file settings.json
  auth0 acul config set signup-id --file settings.json
  auth0 acul config set login-id`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureACULPrerequisites(cmd.Context(), cli.api); err != nil {
				return err
			}

			screens, err := validateAndSelectScreens(cli, utils.FetchKeys(ScreenPromptMap), args, false)
			if err != nil {
				return err
			}

			input.screenName = screens[0]

			cli.renderer.Output(ansi.Yellow("📖 Customization Guide: https://github.com/auth0/auth0-cli/blob/main/CUSTOMIZATION_GUIDE.md"))

			return advanceCustomize(cmd, cli, input)
		},
	}

	file.RegisterString(cmd, &input.filePath, "")
	return cmd
}

func advanceCustomize(cmd *cobra.Command, cli *cli, input aculConfigInput) error {
	currMode := standardMode
	renderSettings, err := fetchRenderSettings(cmd, cli, input)
	if renderSettings != nil && renderSettings.RenderingMode != nil {
		currMode = string(*renderSettings.RenderingMode)
	}

	if errors.Is(err, ErrNoChangesDetected) {
		cli.renderer.Infof("Current rendering mode for Prompt '%s', Screen '%s': %s", ansi.Green(ScreenPromptMap[input.screenName]), ansi.Green(input.screenName), ansi.Green(currMode))
		return nil
	}

	if err != nil {
		return err
	}

	if err = ansi.Waiting(func() error {
		return cli.api.Prompt.UpdateRendering(cmd.Context(), management.PromptType(ScreenPromptMap[input.screenName]), management.ScreenName(input.screenName), renderSettings)
	}); err != nil {
		return fmt.Errorf("failed to set the render settings: %w", err)
	}

	cli.renderer.Infof("Rendering settings updated. Current rendering mode for '%s', Screen '%s': %s", ansi.Green(ScreenPromptMap[input.screenName]), ansi.Green(input.screenName), ansi.Green(currMode))
	cli.renderer.Output(ansi.Yellow("💡 Tip: Use `auth0 acul config get` to fetch remote rendering settings or `auth0 acul config list` to view all ACUL screens."))
	return nil
}

func fetchRenderSettings(cmd *cobra.Command, cli *cli, input aculConfigInput) (*management.PromptRendering, error) {
	var (
		userRenderSettings string
		renderSettings     = &management.PromptRendering{}
		existingSettings   = map[string]interface{}{}
		currentSettings    = map[string]interface{}{}
	)

	if input.filePath != "" {
		// Case 1: File path is provided, use that file's content.
		data, err := os.ReadFile(input.filePath)
		if err != nil {
			return nil, fmt.Errorf("unable to read file %q: %v", input.filePath, err)
		}
		if err := json.Unmarshal(data, &renderSettings); err != nil {
			return nil, fmt.Errorf("file %q contains invalid JSON: %v", input.filePath, err)
		}
		return renderSettings, nil
	}

	// Case 2: No file path provided, default to config/<screen-name>.json.
	defaultFilePath := fmt.Sprintf("acul_config/%s.json", input.screenName)
	data, err := os.ReadFile(defaultFilePath)
	if err == nil {
		cli.renderer.Warnf("No file path specified. Defaulting to '%s'.", ansi.Green(defaultFilePath))
		if !cli.force && canPrompt(cmd) {
			message := fmt.Sprintf("Use file '%s' for updating remote ACUL configs for '%s'? : ", ansi.Green(defaultFilePath), ansi.Blue(input.screenName))
			if confirmed := prompt.Confirm(message); confirmed {
				if err := json.Unmarshal(data, &renderSettings); err != nil {
					return nil, fmt.Errorf("file %s contains invalid JSON: %v", defaultFilePath, err)
				}
				return renderSettings, nil
			}
		}
	}

	// Case 3: No file path provided and default file doesn't exist or user declined to use it, open editor.
	cli.renderer.Infof("Opening editor to update remote ACUL configs for '%s'.", ansi.Green(input.screenName))
	existingRenderSettings, err := cli.api.Prompt.ReadRendering(cmd.Context(), management.PromptType(ScreenPromptMap[input.screenName]), management.ScreenName(input.screenName))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch the existing render settings: %w", err)
	}

	if existingRenderSettings != nil {
		readRenderingJSON, _ := json.MarshalIndent(existingRenderSettings, "", "  ")
		if err := json.Unmarshal(readRenderingJSON, &existingSettings); err != nil {
			cli.renderer.Warnf("Error parsing fetched rendering JSON: %v", err)
		}
	}

	existingSettings["___customization guide___"] = "https://github.com/auth0/auth0-cli/blob/main/CUSTOMIZATION_GUIDE.md"
	// Error handling omitted for brevity.
	finalJSON, _ := json.MarshalIndent(existingSettings, "", "  ")

	err = rendererScript.OpenEditor(cmd, &userRenderSettings, string(finalJSON), input.screenName+".json", cli.customizeEditorHint)
	if err != nil {
		return nil, fmt.Errorf("failed to capture input from the editor: %w", err)
	}

	err = json.Unmarshal([]byte(userRenderSettings), &currentSettings)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON input into a map: %w", err)
	}

	// Compare the existing settings with the updated settings to detect changes.
	if jsonEqual(existingSettings, currentSettings) {
		cli.renderer.Warnf("No changes detected in the customization settings. This could be due to uncommitted configuration changes or no modifications being made to the configurations.")

		return existingRenderSettings, ErrNoChangesDetected
	}

	if err := json.Unmarshal([]byte(userRenderSettings), &renderSettings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON input: %w", err)
	}

	return renderSettings, nil
}

// jsonEqual ignores the special "___customization guide___" key used for user reference.
func jsonEqual(a, b map[string]interface{}) bool {
	copyA := make(map[string]interface{}, len(a))
	copyB := make(map[string]interface{}, len(b))
	for k, v := range a {
		if k != "___customization guide___" {
			copyA[k] = v
		}
	}
	for k, v := range b {
		if k != "___customization guide___" {
			copyB[k] = v
		}
	}

	aj, err1 := json.Marshal(copyA)
	bj, err2 := json.Marshal(copyB)
	if err1 != nil || err2 != nil {
		return false
	}
	return bytes.Equal(aj, bj)
}

func aculConfigListCmd(cli *cli) *cobra.Command {
	var (
		fields        string
		includeFields bool
		includeTotals bool
		page          int
		perPage       int
		promptName    string
		screen        string
		renderingMode string
		query         string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List Universal Login rendering configurations",
		Long:    "List Universal Login rendering configurations with optional filters and pagination.",
		Example: `  auth0 acul config list --prompt reset-password
  auth0 acul config list --rendering-mode advanced --include-fields true --fields head_tags,context_configuration`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if err := ensureACULPrerequisites(ctx, cli.api); err != nil {
				return err
			}

			params := []management.RequestOption{
				management.Parameter("page", strconv.Itoa(page)),
				management.Parameter("per_page", strconv.Itoa(perPage)),
			}

			if query != "" {
				params = append(params, management.Parameter("q", query))
			}
			if includeFields && fields != "" {
				params = append(params, management.IncludeFields(fields))
			}
			if !includeFields && fields != "" {
				params = append(params, management.ExcludeFields(fields))
			}
			if screen != "" {
				params = append(params, management.Parameter("screen", screen))
			}
			if promptName != "" {
				params = append(params, management.Parameter("prompt", promptName))
			}
			if renderingMode != "" {
				params = append(params, management.Parameter("rendering_mode", renderingMode))
			}

			var results *management.PromptRenderingList
			if err := ansi.Waiting(func() (err error) {
				results, err = cli.api.Prompt.ListRendering(cmd.Context(), params...)
				return err
			}); err != nil {
				cli.renderer.Errorf("Failed to list rendering configurations: %v", err)
				return err
			}

			cli.renderer.ACULConfigList(results)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	cmd.MarkFlagsMutuallyExclusive("json", "json-compact")

	fieldsFlag.RegisterString(cmd, &fields, "")
	includeFieldsFlag.RegisterBool(cmd, &includeFields, true)
	includeTotalsFlag.RegisterBool(cmd, &includeTotals, false)
	pageFlag.RegisterInt(cmd, &page, 0)
	perPageFlag.RegisterInt(cmd, &perPage, 50)
	promptFlag.RegisterString(cmd, &promptName, "")
	screenFlag.RegisterString(cmd, &screen, "")
	renderingModeFlag.RegisterString(cmd, &renderingMode, "")
	queryFlag.RegisterString(cmd, &query, "")

	return cmd
}

func aculConfigDocsCmd(cli *cli) *cobra.Command {
	return &cobra.Command{
		Use:     "docs",
		Short:   "Open the ACUL configuration documentation",
		Long:    "Open the documentation for configuring Advanced Customizations for Universal Login screens.",
		Example: `  auth0 acul config docs`,
		RunE: func(cmd *cobra.Command, args []string) error {
			url := "https://auth0.com/docs/customize/login-pages/advanced-customizations/getting-started/configure-acul-screens"
			cli.renderer.Infof("Opening documentation: %s", url)
			return browser.OpenURL(url)
		},
	}
}

func (c *cli) customizeEditorHint() {
	c.renderer.Infof("%s Once you close the editor, the shown settings will be saved. To cancel, press CTRL+C.", ansi.Faint("Hint:"))
}
