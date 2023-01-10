package cli

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/iostream"
	"github.com/auth0/auth0-cli/internal/prompt"
)

var (
	templateBody = Flag{
		Name:       "Template",
		LongForm:   "template",
		ShortForm:  "t",
		Help:       "Custom page template for Universal Login.",
		IsRequired: true,
	}

	brandingAccent = Flag{
		Name:         "Accent Color",
		LongForm:     "accent",
		ShortForm:    "a",
		Help:         "Accent color.",
		AlwaysPrompt: true,
	}

	brandingBackground = Flag{
		Name:         "Background Color",
		LongForm:     "background",
		ShortForm:    "b",
		Help:         "Page background color",
		AlwaysPrompt: true,
	}

	brandingLogo = Flag{
		Name:         "Logo URL",
		LongForm:     "logo",
		ShortForm:    "l",
		Help:         "URL for the logo. Must use HTTPS.",
		AlwaysPrompt: true,
	}

	brandingFavicon = Flag{
		Name:         "Favicon URL",
		LongForm:     "favicon",
		ShortForm:    "f",
		Help:         "URL for the favicon. Must use HTTPS.",
		AlwaysPrompt: true,
	}

	brandingFont = Flag{
		Name:         "Custom Font URL",
		LongForm:     "font",
		ShortForm:    "c",
		Help:         "URL for the custom font. The URL must point to a font file and not a stylesheet. Must use HTTPS.",
		AlwaysPrompt: true,
	}

	errNotAllowed = errors.New("this feature requires at least one custom domain to be set and verified for the tenant")
)

func universalLoginCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "universal-login",
		Short: "Manage the Universal Login experience",
		Long: "Manage a consistent, branded Universal Login experience that can " +
			"handle all of your authentication flows.",
		Aliases: []string{"ul"},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())

	cmd.AddCommand(showUniversalLoginCmd(cli))
	cmd.AddCommand(updateUniversalLoginCmd(cli))
	cmd.AddCommand(universalLoginTemplatesCmd(cli))
	cmd.AddCommand(universalLoginPromptsTextCmd(cli))

	return cmd
}

func universalLoginTemplatesCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "templates",
		Short: "Manage custom Universal Login templates",
		Long: `Manage custom [page templates](https://auth0.com/docs/universal-login/new-experience/universal-login-page-templates). This requires a custom domain to be configured for the tenant.

This command will open two windows:

* A browser window with a [Storybook](https://storybook.js.org/) that shows the login page with the page template applied:

![storybook](images/templates-storybook.png)

* The default terminal editor, with the page template code:

![storybook](images/templates-vs-code.png)

You now change the page template code, and the changes will be reflected in the browser window. 

Once you close the window, you’ll be asked if you want to save the template. If you answer Yes, the template will be uploaded to your tenant.
`,
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(showBrandingTemplateCmd(cli))
	cmd.AddCommand(updateBrandingTemplateCmd(cli))

	return cmd
}

func showUniversalLoginCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.NoArgs,
		Short: "Display the custom branding settings for Universal Login",
		Long:  "Display the custom branding settings for Universal Login.",
		Example: `  auth0 universal-login show
  auth0 ul show
  auth0 ul show --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var myBranding *management.Branding

			if err := ansi.Waiting(func() error {
				var err error
				myBranding, err = cli.api.Branding.Read()
				return err
			}); err != nil {
				return fmt.Errorf("unable to load branding settings due to an unexpected error: %w", err)
			}

			cli.renderer.BrandingShow(myBranding)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")

	return cmd
}

func updateUniversalLoginCmd(cli *cli) *cobra.Command {
	var inputs struct {
		AccentColor     string
		BackgroundColor string
		LogoURL         string
		FaviconURL      string
		CustomFontURL   string
	}

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.NoArgs,
		Short: "Update the custom branding settings for Universal Login",
		Long: "Update the custom branding settings for Universal Login.\n\n" +
			"To update the settings for Universal Login interactively, use `auth0 universal-login update` " +
			"with no arguments.\n\n" +
			"To update the settings for Universal Login non-interactively, supply the accent, background and " +
			"logo through the flags.",
		Example: `  auth0 universal-login update
  auth0 ul update --accent "#FF4F40" --background "#2A2E35" --logo "https://example.com/logo.png"
  auth0 ul update -a "#FF4F40" -b "#2A2E35" -l "https://example.com/logo.png"
  auth0 ul update -a "#FF4F40" -b "#2A2E35" -l "https://example.com/logo.png" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var current *management.Branding

			if err := ansi.Waiting(func() error {
				var err error
				current, err = cli.api.Branding.Read()
				return err
			}); err != nil {
				return fmt.Errorf("unable to load branding settings due to an unexpected error: %w", err)
			}

			// Prompt for accent color
			if err := brandingAccent.AskU(cmd, &inputs.AccentColor, auth0.String(current.GetColors().GetPrimary())); err != nil {
				return err
			}

			// Prompt for background color
			if err := brandingBackground.AskU(cmd, &inputs.BackgroundColor, auth0.String(current.GetColors().GetPageBackground())); err != nil {
				return err
			}

			// Load updated values into a fresh branding instance
			b := &management.Branding{}
			isAccentColorSet := len(inputs.AccentColor) > 0
			isBackgroundColorSet := len(inputs.BackgroundColor) > 0
			currentHasColors := current.Colors != nil

			if isAccentColorSet || isBackgroundColorSet || currentHasColors {
				b.Colors = &management.BrandingColors{}

				if isAccentColorSet {
					b.Colors.Primary = &inputs.AccentColor
				} else if currentHasColors {
					b.Colors.Primary = current.Colors.Primary
				}

				if isBackgroundColorSet {
					b.Colors.PageBackground = &inputs.BackgroundColor
				} else if currentHasColors {
					b.Colors.PageBackground = current.Colors.PageBackground
				}
			}

			if len(inputs.LogoURL) == 0 {
				b.LogoURL = current.LogoURL
			} else {
				b.LogoURL = &inputs.LogoURL
			}

			if len(inputs.FaviconURL) == 0 {
				b.FaviconURL = current.FaviconURL
			} else {
				b.FaviconURL = &inputs.FaviconURL
			}

			// API2 will produce an error if we send an empty font struct
			if b.Font == nil && inputs.CustomFontURL != "" {
				b.Font = &management.BrandingFont{URL: &inputs.CustomFontURL}
			}

			if b.Font != nil {
				if len(inputs.CustomFontURL) == 0 {
					b.Font.URL = current.Font.URL
				} else {
					b.Font.URL = &inputs.CustomFontURL
				}
			}

			// Update branding
			if err := ansi.Waiting(func() error {
				return cli.api.Branding.Update(b)
			}); err != nil {
				return fmt.Errorf("unable to update branding settings: %v", err)
			}

			// Render result
			cli.renderer.BrandingUpdate(b)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	brandingAccent.RegisterStringU(cmd, &inputs.AccentColor, "")
	brandingBackground.RegisterStringU(cmd, &inputs.BackgroundColor, "")
	brandingLogo.RegisterStringU(cmd, &inputs.LogoURL, "")
	brandingFavicon.RegisterStringU(cmd, &inputs.FaviconURL, "")
	brandingFont.RegisterStringU(cmd, &inputs.CustomFontURL, "")

	return cmd
}

func showBrandingTemplateCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Args:  cobra.NoArgs,
		Short: "Display the custom template for Universal Login",
		Long:  "Display the custom template for the Universal Login experience.",
		Example: `  auth0 universal-login templates show
  auth0 ul templates show`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var template *management.BrandingUniversalLogin
			if err := ansi.Waiting(func() (err error) {
				template, err = cli.api.Branding.UniversalLogin()
				if err != nil {
					if mErr, ok := err.(management.Error); ok && mErr.Status() == http.StatusNotFound {
						return nil
					}
					return err
				}

				return nil
			}); err != nil {
				return fmt.Errorf("failed to load the Universal Login template: %w", err)
			}

			cli.renderer.Heading("universal login template")

			if template == nil {
				cli.renderer.Infof("No custom template found. To set one, run: `auth0 universal-login templates update`.")
			}

			fmt.Println(template.GetBody())

			return nil
		},
	}

	return cmd
}

func updateBrandingTemplateCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.NoArgs,
		Short: "Update the custom template for Universal Login",
		Long:  "Update the custom template for Universal Login.",
		Example: `  auth0 universal-login templates update
  auth0 ul templates update
  cat path/to/body.html | auth0 ul templates update`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := isCustomDomainEnabled(cli.api); err != nil {
				return err
			}

			var currentTemplate *management.BrandingUniversalLogin
			if err := ansi.Waiting(func() (err error) {
				currentTemplate, err = cli.api.Branding.UniversalLogin()
				if err != nil {
					if mErr, ok := err.(management.Error); ok && mErr.Status() == http.StatusNotFound {
						return nil
					}
					return err
				}

				return nil
			}); err != nil {
				return fmt.Errorf("failed to load the Universal Login template: %w", err)
			}

			onInfo := func() {
				cli.renderer.Infof(
					"%s Once you close the editor, you'll be prompted to save your changes. To cancel, press CTRL+C.",
					ansi.Faint("Hint:"),
				)
			}

			body := string(iostream.PipedInput())
			err := textBody.OpenEditor(cmd, &body, currentTemplate.GetBody(), "ul-template.*.html", onInfo)
			if err != nil {
				return fmt.Errorf("failed to capture input from the editor: %w", err)
			}

			if !cli.force && canPrompt(cmd) {
				var confirmed bool
				if err := prompt.AskBool("Do you want to save the template?", &confirmed, true); err != nil {
					return fmt.Errorf("failed to capture prompt input: %w", err)
				}
				if !confirmed {
					return nil
				}
			}

			if err = ansi.Waiting(func() error {
				return cli.api.Branding.SetUniversalLogin(
					&management.BrandingUniversalLogin{
						Body: &body,
					},
				)
			}); err != nil {
				return fmt.Errorf("failed to update the Universal Login template: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

func isCustomDomainEnabled(api *auth0.API) error {
	domains, err := api.CustomDomain.List()
	if err != nil {
		// 403 is a valid response for free tenants that don't have custom domains enabled
		if mErr, ok := err.(management.Error); ok && mErr.Status() == http.StatusForbidden {
			return errNotAllowed
		}

		return err
	}

	for _, domain := range domains {
		if domain.GetStatus() == "ready" {
			return nil
		}
	}

	return errNotAllowed
}
