package cli

import (
	"fmt"

	"github.com/auth0/go-auth0/management"
	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
)

var (
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

	cmd.AddCommand(customizeUniversalLoginCmd(cli))
	cmd.AddCommand(newUpdateAssetsCmd(cli))
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
  auth0 ul show --json
  auth0 ul show --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var myBranding *management.Branding

			if err := ansi.Waiting(func() error {
				var err error
				myBranding, err = cli.api.Branding.Read(cmd.Context())
				return err
			}); err != nil {
				return fmt.Errorf("failed to read branding settings: %w", err)
			}

			cli.renderer.BrandingShow(myBranding)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")

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
  auth0 ul update -a "#FF4F40" -b "#2A2E35" -l "https://example.com/logo.png" --json
  auth0 ul update -a "#FF4F40" -b "#2A2E35" -l "https://example.com/logo.png" --json-compact`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var current *management.Branding

			if err := ansi.Waiting(func() (err error) {
				current, err = cli.api.Branding.Read(cmd.Context())
				return err
			}); err != nil {
				return fmt.Errorf("failed to read branding settings: %w", err)
			}

			if err := brandingAccent.AskU(cmd, &inputs.AccentColor, auth0.String(current.GetColors().GetPrimary())); err != nil {
				return err
			}

			if err := brandingBackground.AskU(cmd, &inputs.BackgroundColor, auth0.String(current.GetColors().GetPageBackground())); err != nil {
				return err
			}

			b := &management.Branding{}
			isAccentColorSet := len(inputs.AccentColor) > 0
			isBackgroundColorSet := len(inputs.BackgroundColor) > 0
			currentHasColors := current.GetColors() != nil

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

			if len(inputs.LogoURL) != 0 {
				b.LogoURL = &inputs.LogoURL
			}

			if len(inputs.FaviconURL) != 0 {
				b.FaviconURL = &inputs.FaviconURL
			}

			// API2 will produce an error if we send an empty font struct.
			if b.Font == nil && inputs.CustomFontURL != "" {
				b.Font = &management.BrandingFont{URL: &inputs.CustomFontURL}
			}

			// Update branding.
			if err := ansi.Waiting(func() error {
				return cli.api.Branding.Update(cmd.Context(), b)
			}); err != nil {
				return fmt.Errorf("failed to update branding settings: %w", err)
			}

			cli.renderer.BrandingUpdate(b)

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.json, "json", false, "Output in json format.")
	cmd.Flags().BoolVar(&cli.jsonCompact, "json-compact", false, "Output in compact json format.")
	brandingAccent.RegisterStringU(cmd, &inputs.AccentColor, "")
	brandingBackground.RegisterStringU(cmd, &inputs.BackgroundColor, "")
	brandingLogo.RegisterStringU(cmd, &inputs.LogoURL, "")
	brandingFavicon.RegisterStringU(cmd, &inputs.FaviconURL, "")
	brandingFont.RegisterStringU(cmd, &inputs.CustomFontURL, "")

	return cmd
}
