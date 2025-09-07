package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"

	"github.com/pkg/browser"

	"github.com/auth0/go-auth0/management"
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
	fieldsFlag = Flag{
		Name:       "Fields",
		LongForm:   "fields",
		Help:       "Comma-separated list of fields to include or exclude in the result (based on value provided for include_fields) ",
		IsRequired: false,
	}
	includeFieldsFlag = Flag{
		Name:       "Include Fields",
		LongForm:   "include-fields",
		Help:       "Whether specified fields are to be included (default: true) or excluded (false).",
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
	cmd.AddCommand(aculConfigListCmd(cli))
	cmd.AddCommand(aculConfigDocsCmd(cli))

	return cmd
}

func aculConfigGenerateCmd(cli *cli) *cobra.Command {
	var input customizationInputs

	cmd := &cobra.Command{
		Use:   "generate",
		Args:  cobra.MaximumNArgs(1),
		Short: "Generate a default rendering config for a screen",
		Long:  "Generate a default rendering config for a specific screen and save it to a file.",
		Example: `  auth0 acul config generate signup-id
  auth0 acul config generate login-id --file login-settings.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				cli.renderer.Infof("Please select a screen ")
				if err := screenName.Select(cmd, &screenName, utils.FetchKeys(ScreenPromptMap), nil); err != nil {
					return handleInputError(err)
				}
			} else {
				input.screenName = args[0]
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

			cli.renderer.Infof("Configuration successfully generated!\n"+
				"      Your new config file is located at ./%s\n"+
				"      Review the documentation for configuring screens to use ACUL\n"+
				"      https://auth0.com/docs/customize/login-pages/advanced-customizations/getting-started/configure-acul-screens\n", ansi.Green(input.filePath))
			return nil
		},
	}

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

			if input.filePath != "" {
				if isFileExists(cli, cmd, input.filePath, input.screenName) {
					return nil
				}
			} else {
				cli.renderer.Warnf("No configuration file exists for %s on %s", ansi.Green(input.screenName), ansi.Blue(input.filePath))

				if !cli.force && canPrompt(cmd) {
					message := "Would you like to generate a local config file instead? (Y/n)"
					if confirmed := prompt.Confirm(message); !confirmed {
						return nil
					}
				}

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

func isFileExists(cli *cli, cmd *cobra.Command, filePath, screen string) bool {
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return false
	}

	cli.renderer.Warnf("A configuration file for %s already exists at %s", ansi.Green(screen), ansi.Blue(filePath))

	if !cli.force && canPrompt(cmd) {
		message := fmt.Sprintf("Overwrite this file with the data from %s? (y/N): ", ansi.Blue(cli.tenant))
		if confirmed := prompt.Confirm(message); !confirmed {
			return true
		}
	}

	return false
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
		Example: `  auth0 acul config list --prompt login-id --screen login --rendering-mode advanced --include-fields true --fields head_tags,context_configuration`,
		RunE: func(cmd *cobra.Command, args []string) error {
			params := []management.RequestOption{
				management.Parameter("page", strconv.Itoa(page)),
				management.Parameter("per_page", strconv.Itoa(perPage)),
			}

			if query != "" {
				params = append(params, management.Parameter("q", query))
			}

			if includeFields {
				if fields != "" {
					params = append(params, management.IncludeFields(fields))
				}
			} else {
				if fields != "" {
					params = append(params, management.ExcludeFields(fields))
				}
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
				return err
			}

			fmt.Printf("Results : %v\n", results)

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
