package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pkg/browser"
	"os"
	"reflect"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/utils"
	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"
)

var (
	screenName = Flag{
		Name:       "Screen Name",
		LongForm:   "screen",
		ShortForm:  "s",
		Help:       "Name of the screen to to switch or customize.",
		IsRequired: true,
	}

	file = Flag{
		Name:       "File",
		LongForm:   "settings-file",
		ShortForm:  "f",
		Help:       "File to save the rendering configs to.",
		IsRequired: false,
	}

	rendererScript = Flag{
		Name:       "Script",
		LongForm:   "script",
		ShortForm:  "s",
		Help:       "Script contents for the rendering configs.",
		IsRequired: true,
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

func aculCmd(cli *cli) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "acul",
		Short: "Advance Customize the Universal Login experience",
		Long:  `Customize the Universal Login experience. This requires a custom domain to be configured for the tenant.`,
	}

	cmd.AddCommand(aculConfigureCmd(cli))

	return cmd
}

type customizationInputs struct {
	screenName string
	filePath   string
}

func aculConfigureCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configure the Universal Login experience",
		Long:  "Configure the Universal Login experience. This requires a custom domain to be configured for the tenant.",
		Example: `  auth0 acul config
  auth0 acul config
  auth0 acul config --screen login-id --file settings.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return advanceCustomize(cmd, cli, customizationInputs{})
		},
	}

	cmd.AddCommand(aculConfigGenerateCmd(cli))
	cmd.AddCommand(aculConfigGet(cli))
	cmd.AddCommand(aculConfigSet(cli))
	cmd.AddCommand(aculConfigDocsCmd(cli))

	return cmd
}

func aculConfigGenerateCmd(cli *cli) *cobra.Command {
	var input customizationInputs

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a default rendering config for a screen",
		Long:  "Generate a default rendering config for a specific screen and save it to a file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if input.screenName == "" {
				cli.renderer.Infof("Please select a screen")
				if err := screenName.Select(cmd, &input.screenName, utils.FetchKeys(ScreenPromptMap), nil); err != nil {
					return handleInputError(err)
				}
			}
			if input.filePath == "" {
				input.filePath = fmt.Sprintf("%s.json", input.screenName)
			}

			defaultConfig := map[string]interface{}{
				"rendering_mode":             "standard",
				"context_configuration":      []interface{}{},
				"use_page_template":          false,
				"default_head_tags_disabled": false,
				"head_tags":                  []interface{}{},
				"filters":                    []interface{}{},
			}

			data, err := json.MarshalIndent(defaultConfig, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal default config: %w", err)
			}

			if err := os.WriteFile(input.filePath, data, 0644); err != nil {
				return fmt.Errorf("failed to write config file: %w", err)
			}

			cli.renderer.Infof("\nGeneration Message\n\nConfiguration successfully generated!\nYour new config file is located at ./%s\nReview the documentation for configuring screens to use ACUL\nhttps://auth0.com/docs/customize/login-pages/advanced-customizations/getting-started/configure-acul-screens\nGenerated configuration\n", input.filePath)
			return nil
		},
	}

	screenName.RegisterString(cmd, &input.screenName, "")
	file.RegisterString(cmd, &input.filePath, "")

	return cmd
}

func aculConfigGet(cli *cli) *cobra.Command {
	var input customizationInputs

	cmd := &cobra.Command{
		Use:   "get",
		Args:  cobra.MaximumNArgs(1),
		Short: "Get the current rendering settings for a specific screen",
		Long:  "Get the current rendering settings for a specific screen.",
		Example: `  auth0 acul config get signup-id
  auth0 acul config get login-id`,
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) == 0 {
				cli.renderer.Infof("Please select a screen ")
				if err := screenName.Select(cmd, &input.screenName, utils.FetchKeys(ScreenPromptMap), nil); err != nil {
					return handleInputError(err)
				}
			} else {
				input.screenName = args[0]
			}

			// Fetch existing render settings from the API.
			existingRenderSettings, err := cli.api.Prompt.ReadRendering(cmd.Context(), management.PromptType(ScreenPromptMap[input.screenName]), management.ScreenName(input.screenName))
			if err != nil {
				return fmt.Errorf("failed to fetch the existing render settings: %w", err)
			}

			if input.filePath == "" {
				input.filePath = fmt.Sprintf("%s.json", input.screenName)
			}

			data, err := json.MarshalIndent(existingRenderSettings, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal render settings: %w", err)
			}

			if err := os.WriteFile(input.filePath, data, 0644); err != nil {
				return fmt.Errorf("failed to write render settings to file %q: %w", input.filePath, err)
			}

			cli.renderer.Infof("Configuration succcessfully downloaded and saved to %s", ansi.Green(input.filePath))
			return nil

		},
	}

	screenName.RegisterString(cmd, &input.screenName, "")
	file.RegisterString(cmd, &input.filePath, "")

	return cmd
}

func aculConfigSet(cli *cli) *cobra.Command {
	var input customizationInputs

	cmd := &cobra.Command{
		Use:   "set",
		Args:  cobra.MaximumNArgs(1),
		Short: "Set the rendering settings for a specific screen",
		Long:  "Set the rendering settings for a specific screen.",
		Example: `  auth0 acul config set signup-id --file settings.json
  auth0 acul config set login-id --file settings.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return advanceCustomize(cmd, cli, input)
		},
	}

	screenName.RegisterString(cmd, &input.screenName, "")
	file.RegisterString(cmd, &input.filePath, "")

	return cmd
}

func advanceCustomize(cmd *cobra.Command, cli *cli, input customizationInputs) error {
	var currMode = standardMode

	renderSettings, err := fetchRenderSettings(cmd, cli, input)
	if renderSettings != nil && renderSettings.RenderingMode != nil {
		currMode = string(*renderSettings.RenderingMode)
	}

	if errors.Is(err, ErrNoChangesDetected) {
		cli.renderer.Infof("Current rendering mode for Prompt '%s' and Screen '%s': %s",
			ansi.Green(ScreenPromptMap[input.screenName]), ansi.Green(input.screenName), ansi.Green(currMode))
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

	cli.renderer.Infof(
		"Successfully updated the rendering settings.\n Current rendering mode for Prompt '%s' and Screen '%s': %s",
		ansi.Green(ScreenPromptMap[input.screenName]),
		ansi.Green(input.screenName),
		ansi.Green(currMode),
	)

	return nil
}

func fetchRenderSettings(cmd *cobra.Command, cli *cli, input customizationInputs) (*management.PromptRendering, error) {
	var (
		userRenderSettings string
		renderSettings     = &management.PromptRendering{}
		existingSettings   = map[string]interface{}{}
		currentSettings    = map[string]interface{}{}
	)

	if input.filePath != "" {
		data, err := os.ReadFile(input.filePath)
		if err != nil {
			return nil, fmt.Errorf("unable to read file %q: %v", input.filePath, err)
		}

		// Validate JSON content.
		if err := json.Unmarshal(data, &renderSettings); err != nil {
			return nil, fmt.Errorf("file %q contains invalid JSON: %v", input.filePath, err)
		}

		return renderSettings, nil
	}

	// Fetch existing render settings from the API.
	existingRenderSettings, err := cli.api.Prompt.ReadRendering(cmd.Context(), management.PromptType(ScreenPromptMap[input.screenName]), management.ScreenName(input.screenName))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch the existing render settings: %w", err)
	}

	// Marshal existing render settings into JSON and parse into a map if it's not nil.
	if existingRenderSettings != nil {
		readRenderingJSON, _ := json.MarshalIndent(existingRenderSettings, "", "  ")
		if err := json.Unmarshal(readRenderingJSON, &existingSettings); err != nil {
			fmt.Println("Error parsing readRendering JSON:", err)
		}
	}

	existingSettings["___customization guide___"] = "https://github.com/auth0/auth0-cli/blob/main/CUSTOMIZATION_GUIDE.md"

	// Marshal final JSON.
	finalJSON, err := json.MarshalIndent(existingSettings, "", "  ")
	if err != nil {
		fmt.Println("Error generating final JSON:", err)
	}

	err = rendererScript.OpenEditor(cmd, &userRenderSettings, string(finalJSON), input.screenName+".json", cli.customizeEditorHint)
	if err != nil {
		return nil, fmt.Errorf("failed to capture input from the editor: %w", err)
	}

	// Unmarshal user-provided JSON into a map for comparison.
	err = json.Unmarshal([]byte(userRenderSettings), &currentSettings)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON input into a map: %w", err)
	}

	// Compare the existing settings with the updated settings to detect changes.
	if reflect.DeepEqual(existingSettings, currentSettings) {
		cli.renderer.Warnf("No changes detected in the customization settings. This could be due to uncommitted configuration changes or no modifications being made to the configurations.")

		return existingRenderSettings, ErrNoChangesDetected
	}

	if err := json.Unmarshal([]byte(userRenderSettings), &renderSettings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON input: %w", err)
	}

	return renderSettings, nil
}

func aculConfigDocsCmd(cli *cli) *cobra.Command {
	return &cobra.Command{
		Use:   "docs",
		Short: "Open the ACUL configuration documentation",
		Long:  "Open the documentation for configuring Advanced Customizations for Universal Login screens.",
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
