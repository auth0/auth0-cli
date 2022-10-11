package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/iostream"
)

const (
	textDocsKey         = "__doc__"
	textDocsURL         = "https://auth0.com/docs/brand-and-customize/text-customization-new-universal-login"
	textLocalesURL      = "https://auth0-ulp.herokuapp.com/static/locales"
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

type brandingTextsInputs struct {
	Language string
}

func textsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "texts",
		Short: "Manage custom text for prompts",
		Long:  "Manage custom text for prompts.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(showBrandingTextCmd(cli))
	cmd.AddCommand(updateBrandingTextCmd(cli))

	return cmd
}

func showBrandingTextCmd(cli *cli) *cobra.Command {
	var inputs brandingTextsInputs

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.ExactArgs(1),
		Short: "Show the custom texts for a prompt",
		Long:  "Show the custom texts for a prompt.",
		Example: `
auth0 branding texts show <prompt> --language es
auth0 branding texts show <prompt> -l es`,
		RunE: showBrandingTexts(cli, &inputs),
	}

	textLanguage.RegisterString(cmd, &inputs.Language, textLanguageDefault)

	return cmd
}

func showBrandingTexts(cli *cli, inputs *brandingTextsInputs) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		var prompt = args[0]
		var brandingText map[string]interface{}

		if err := ansi.Waiting(func() (err error) {
			brandingText, err = cli.api.Prompt.CustomText(prompt, inputs.Language)
			return err
		}); err != nil {
			return fmt.Errorf(
				"unable to load custom text for prompt %s and language %s: %w",
				prompt,
				inputs.Language,
				err,
			)
		}

		brandingTextJSON, err := json.MarshalIndent(brandingText, "", "    ")
		if err != nil {
			return fmt.Errorf("failed to serialize the prompt custom text to JSON: %w", err)
		}

		cli.renderer.BrandingTextShow(string(brandingTextJSON), prompt, inputs.Language)

		return nil
	}
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
			fileName := fmt.Sprintf("%s.%s.json", prompt, inputs.Language)
			currentBody := make(map[string]interface{})
			currentBody[textDocsKey] = brandingTextDocsURL(prompt)

			if err := ansi.Waiting(func() error {
				defaultTranslations := downloadBrandingTextLocale(fileName)

				customTranslations, err := cli.api.Prompt.CustomText(prompt, inputs.Language)
				if err != nil {
					return err
				}

				currentBodyTranslations := mergeBrandingTextLocales(defaultTranslations, customTranslations)
				for k, v := range currentBodyTranslations {
					currentBody[k] = v
				}
				return nil
			}); err != nil {
				return fmt.Errorf("Unable to load custom text for prompt %s and language %s: %w", prompt, inputs.Language, err)
			}

			currentBodyStr, err := marshalBrandingTextBody(currentBody)
			if err != nil {
				return err
			}

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

func brandingTextDocsURL(p string) string {
	return fmt.Sprintf("%s/prompt-%s", textDocsURL, p)
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

func downloadBrandingTextLocale(filename string) map[string]interface{} {
	url := fmt.Sprintf("%s/%s", textLocalesURL, filename)
	resp, err := http.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil
		}
		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err != nil {
			return nil
		}
		return result
	}
	return nil
}

func mergeBrandingTextLocales(d map[string]interface{}, c map[string]interface{}) map[string]map[string]interface{} {
	result := make(map[string]map[string]interface{})

	for p, pv := range d {
		if translations, ok := pv.(map[string]interface{}); ok {
			for k, v := range translations {
				if !strings.HasPrefix(k, "error") && !strings.HasPrefix(k, "devKeys") {
					if _, ok := result[p]; !ok {
						result[p] = make(map[string]interface{})
					}
					if _, ok := result[p][k]; !ok {
						result[p][k] = make(map[string]interface{})
					}
					result[p][k] = v
				}
			}
		}
	}
	for p, pv := range c {
		if translations, ok := pv.(map[string]interface{}); ok {
			for k, v := range translations {
				if _, ok := result[p]; !ok {
					result[p] = make(map[string]interface{})
				}
				if _, ok := result[p][k]; !ok {
					result[p][k] = make(map[string]interface{})
				}
				result[p][k] = v
			}
		}
	}
	return result
}
