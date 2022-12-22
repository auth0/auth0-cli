package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/iostream"
)

const (
	textDocsKey         = "__doc__"
	textDocsURL         = "https://auth0.com/docs/customize/universal-login-pages/customize-login-text-prompts"
	textLocalesURL      = "https://cdn.auth0.com/ulp/react-components/development/languages/%s/prompts.json"
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

	textBody = Flag{
		Name:       "Text",
		LongForm:   "text",
		ShortForm:  "t",
		Help:       "Text contents for the branding.",
		IsRequired: true,
	}
)

type promptsTextInput struct {
	Prompt   string
	Language string
	Body     string
}

func universalLoginPromptsTextCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "prompts",
		Short:   "Manage custom text for prompts",
		Long:    fmt.Sprintf("Manage custom [text for prompts](%s).", textDocsURL),
		Aliases: []string{"texts"},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())

	cmd.AddCommand(showPromptsTextCmd(cli))
	cmd.AddCommand(updatePromptsTextCmd(cli))

	return cmd
}

func showPromptsTextCmd(cli *cli) *cobra.Command {
	var inputs promptsTextInput

	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.ExactArgs(1),
		Short: "Show the custom text for a prompt",
		Long:  "Show the custom text for a prompt.",
		Example: `  auth0 universal-login prompts show <prompt> --language es
  auth0 universal-login prompts show <prompt> -l es`,
		RunE: showPromptsText(cli, &inputs),
	}

	textLanguage.RegisterString(cmd, &inputs.Language, textLanguageDefault)

	return cmd
}

func updatePromptsTextCmd(cli *cli) *cobra.Command {
	var inputs promptsTextInput

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.ExactArgs(1),
		Short: "Update the custom text for a prompt",
		Long:  "Update the custom text for a prompt.",
		Example: `  auth0 universal-login prompts update <prompt> --language es
  auth0 universal-login prompts update <prompt> -l es`,
		RunE: updateBrandingText(cli, &inputs),
	}

	textLanguage.RegisterString(cmd, &inputs.Language, textLanguageDefault)

	return cmd
}

func showPromptsText(cli *cli, inputs *promptsTextInput) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		inputs.Prompt = args[0]
		brandingText := make(map[string]interface{})

		if err := ansi.Waiting(func() (err error) {
			brandingText, err = cli.api.Prompt.CustomText(inputs.Prompt, inputs.Language)
			return err
		}); err != nil {
			return fmt.Errorf(
				"unable to fetch custom text for prompt %s and language %s: %w",
				inputs.Prompt,
				inputs.Language,
				err,
			)
		}

		brandingTextJSON, err := json.MarshalIndent(brandingText, "", "    ")
		if err != nil {
			return fmt.Errorf("failed to serialize the prompt custom text to JSON: %w", err)
		}

		cli.renderer.BrandingTextShow(string(brandingTextJSON), inputs.Prompt, inputs.Language)

		return nil
	}
}

func updateBrandingText(cli *cli, inputs *promptsTextInput) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		inputs.Prompt = args[0]
		inputs.Body = string(iostream.PipedInput())

		brandingTextToEdit, err := fetchBrandingTextContentToEdit(cli, inputs)
		if err != nil {
			return fmt.Errorf("failed to fetch branding text content to edit: %w", err)
		}

		editedBrandingText, err := fetchEditedBrandingTextContent(cmd, cli, inputs, brandingTextToEdit)
		if err != nil {
			return fmt.Errorf("failed to fetch edited branding text content: %w", err)
		}

		if err := ansi.Waiting(func() error {
			return cli.api.Prompt.SetCustomText(inputs.Prompt, inputs.Language, editedBrandingText)
		}); err != nil {
			return fmt.Errorf(
				"unable to set custom text for prompt %s and language %s: %w",
				inputs.Prompt,
				inputs.Language,
				err,
			)
		}

		editedBrandingTextJSON, err := json.MarshalIndent(editedBrandingText, "", "    ")
		if err != nil {
			return fmt.Errorf("failed to serialize the prompt custom text to JSON: %w", err)
		}

		cli.renderer.BrandingTextUpdate(string(editedBrandingTextJSON), inputs.Prompt, inputs.Language)

		return nil
	}
}

func fetchBrandingTextContentToEdit(cli *cli, inputs *promptsTextInput) (string, error) {
	contentToEdit := map[string]interface{}{textDocsKey: textDocsURL}

	if err := ansi.Waiting(func() error {
		defaultTranslations := downloadDefaultBrandingTextTranslations(inputs.Prompt, inputs.Language)

		customTranslations, err := cli.api.Prompt.CustomText(inputs.Prompt, inputs.Language)
		if err != nil {
			return err
		}

		brandingTextTranslations := mergeBrandingTextTranslations(defaultTranslations, customTranslations)

		for key, text := range brandingTextTranslations {
			contentToEdit[key] = text
		}

		return nil
	}); err != nil {
		return "", fmt.Errorf(
			"unable to load custom text for prompt %s and language %s: %w",
			inputs.Prompt,
			inputs.Language,
			err,
		)
	}

	contentToEditJSON, err := json.MarshalIndent(contentToEdit, "", "    ")
	if err != nil {
		return "", fmt.Errorf("failed to serialize the prompt custom text to JSON: %w", err)
	}

	return string(contentToEditJSON), nil
}

// downloadDefaultBrandingTextTranslations will download all the prompt's possible
// screen values. In case of encountering any errors it will simply ignore them
// and let the user define by hand all the values for the screen.
func downloadDefaultBrandingTextTranslations(prompt, language string) map[string]interface{} {
	url := fmt.Sprintf(textLocalesURL, language)

	response, err := http.Get(url)
	if err != nil {
		return nil
	}

	defer response.Body.Close()

	if response.StatusCode == http.StatusOK {
		var allPrompts []map[string]interface{}
		if err := json.NewDecoder(response.Body).Decode(&allPrompts); err != nil {
			return nil
		}

		selectedPrompt := allPrompts[0][prompt].(map[string]interface{})

		return selectedPrompt
	}

	return nil
}

func mergeBrandingTextTranslations(
	defaultTranslations map[string]interface{},
	customTranslations map[string]interface{},
) map[string]map[string]interface{} {
	mergedTranslations := make(map[string]map[string]interface{})

	for screen, keyTextMap := range defaultTranslations {
		translations, ok := keyTextMap.(map[string]interface{})
		if !ok {
			break
		}

		for key, text := range translations {
			if strings.HasPrefix(key, "error") || strings.HasPrefix(key, "devKeys") {
				continue
			}

			if _, ok := mergedTranslations[screen]; !ok {
				mergedTranslations[screen] = make(map[string]interface{})
			}

			if _, ok := mergedTranslations[screen][key]; !ok {
				mergedTranslations[screen][key] = make(map[string]interface{})
			}

			mergedTranslations[screen][key] = text
		}
	}

	for screen, keyTextMap := range customTranslations {
		translations, ok := keyTextMap.(map[string]interface{})
		if !ok {
			break
		}

		for key, text := range translations {
			if _, ok := mergedTranslations[screen]; !ok {
				mergedTranslations[screen] = make(map[string]interface{})
			}

			if _, ok := mergedTranslations[screen][key]; !ok {
				mergedTranslations[screen][key] = make(map[string]interface{})
			}

			mergedTranslations[screen][key] = text
		}
	}

	return mergedTranslations
}

func fetchEditedBrandingTextContent(
	cmd *cobra.Command,
	cli *cli,
	inputs *promptsTextInput,
	brandingTextToEdit string,
) (map[string]interface{}, error) {
	tempFileName := fmt.Sprintf("%s-prompt-%s.json", inputs.Prompt, inputs.Language)

	err := textBody.OpenEditor(cmd, &inputs.Body, brandingTextToEdit, tempFileName, updateBrandingTextHint(cli))
	if err != nil {
		return nil, fmt.Errorf("failed to capture input from the editor: %w", err)
	}

	var editedBrandingText map[string]interface{}
	if err := json.Unmarshal([]byte(inputs.Body), &editedBrandingText); err != nil {
		return nil, err
	}

	delete(editedBrandingText, textDocsKey)

	return editedBrandingText, nil
}

func updateBrandingTextHint(cli *cli) func() {
	return func() {
		cli.renderer.Infof(
			"%s Once you close the editor, the custom text will be saved. To cancel, press CTRL+C.",
			ansi.Faint("Hint:"),
		)
	}
}
