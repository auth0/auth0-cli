package cli

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/iostream"
	"github.com/spf13/cobra"
)

const (
	textDocsKey = "__doc__"
	textDocsURL = "https://auth0.com/docs/brand-and-customize/text-customization-new-universal-login/prompt-"
	textLanguageDefault = "en"
)

var (
	textLanguage = Flag{
		Name:       "Language",
		LongForm:   "language",
		ShortForm:  "l",
		Help:       "Language of the custom text.",
		IsRequired: true,
	}
)

func textsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "texts",
		Short: "Manage custom texts for prompts",
		Long:  "Manage custom texts for prompts.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(showBrandingTextCmd(cli))
	cmd.AddCommand(updateBrandingTextCmd(cli))

	return cmd
}

func showBrandingTextCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Language string
	}

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.ExactArgs(1),
		Short: "Show the custom texts for a prompt",
		Long:  "Show the custom texts for a prompt.",
		Example: `
auth0 branding texts show <prompt> --language es
auth0 branding texts show <prompt> -l es`,
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt := args[0]
			var body map[string]interface{}

			if err := ansi.Waiting(func() error {
				var err error
				body, err = cli.api.Prompt.CustomText(prompt, inputs.Language)
				return err
			}); err != nil {
				return fmt.Errorf("Unable to load custom text for prompt %s and language %s: %w", prompt, inputs.Language, err)
			}

			bodyStr, err := marshalBrandingTextBody(body)
			if err != nil {
				return err
			}
			cli.renderer.BrandingTextShow(bodyStr)
			return nil
		},
	}

	textLanguage.RegisterString(cmd, &inputs.Language, textLanguageDefault)
	return cmd
}

func updateBrandingTextCmd(cli *cli) *cobra.Command {
	var inputs struct {
		Language string
		Body     string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.ExactArgs(1),
		Short: "Update the custom texts for a prompt",
		Long:  "Update the custom texts for a prompt.",
		Example: `
auth0 branding texts update <prompt> --language es
auth0 branding texts update <prompt> -l es`,
		RunE: func(cmd *cobra.Command, args []string) error {
			inputs.Body = string(iostream.PipedInput())
			prompt := args[0]
			var currentBody map[string]interface{}

			if err := ansi.Waiting(func() error {
				var err error
				currentBody, err = cli.api.Prompt.CustomText(prompt, inputs.Language)
				return err
			}); err != nil {
				return fmt.Errorf("Unable to load custom text for prompt %s and language %s: %w", prompt, inputs.Language, err)
			}

			currentBody[textDocsKey] = textDocsURL + prompt

			currentBodyStr, err := marshalBrandingTextBody(currentBody)
			if err != nil {
				return err
			}
			fileName := fmt.Sprintf("%s.%s.json", prompt, inputs.Language)

			if err := ruleScript.OpenEditor(
				cmd,
				&inputs.Body,
				currentBodyStr,
				fileName,
				cli.promptTextEditorHint,
			); err != nil {
				return fmt.Errorf("Failed to capture input from the editor: %w", err)
			}

			var body map[string]interface{}
			if err := json.Unmarshal([]byte(inputs.Body), &body); err != nil {
				return err
			}

			delete(body, textDocsKey)

			if err := ansi.Waiting(func() error {
				return cli.api.Prompt.SetCustomText(prompt, inputs.Language, body)
			}); err != nil {
				return fmt.Errorf("Unable to set custom text for prompt %s and language %s: %w", prompt, inputs.Language, err)
			}

			bodyStr, err := marshalBrandingTextBody(body)
			if err != nil {
				return err
			}
			cli.renderer.BrandingTextUpdate(bodyStr)
			return nil
		},
	}

	textLanguage.RegisterString(cmd, &inputs.Language, textLanguageDefault)
	return cmd
}

func (c *cli) promptTextEditorHint() {
	c.renderer.Infof("%s once you close the editor, the custom text will be saved. To cancel, CTRL+C.", ansi.Faint("Hint:"))
}

func marshalBrandingTextBody(b map[string]interface{}) (string, error) {
	bodyBytes, err := json.Marshal(b)
	if err != nil {
		return "", fmt.Errorf("Failed to serialize the custom texts to JSON: %w", err)
	}

	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(bodyBytes), "", "    "); err != nil {
		return "", fmt.Errorf("Failed to format the custom texts JSON: %w", err)
	}
	return buf.String(), nil
}
