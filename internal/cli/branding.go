package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/branding"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"gopkg.in/auth0.v5"
	"gopkg.in/auth0.v5/management"
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

	customTemplateOptions = pickerOptions{
		{"Basic", branding.DefaultTemplate},
		{"Login box + image", branding.ImageTemplate},
		{"Page footers", branding.FooterTemplate},
	}

	errNotAllowed = errors.New("This feature requires at least one custom domain to be configured for the tenant.")
)

func brandingCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "branding",
		Short: "Manage branding options",
		Long:  "Manage branding options.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(showBrandingCmd(cli))
	cmd.AddCommand(updateBrandingCmd(cli))
	cmd.AddCommand(templateCmd(cli))
	cmd.AddCommand(customDomainsCmd(cli))
	cmd.AddCommand(emailTemplateCmd(cli))
	return cmd
}

func templateCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "templates",
		Short: "Manage custom page templates",
		Long:  "Manage custom page templates. This requires at least one custom domain to be configured for the tenant.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(showBrandingTemplateCmd(cli))
	cmd.AddCommand(updateBrandingTemplateCmd(cli))
	return cmd
}

func emailTemplateCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "emails",
		Short: "Manage custom email templates",
		Long:  "Manage custom email templates. This requires a custom email provider to be configured for the tenant.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(showEmailTemplateCmd(cli))
	cmd.AddCommand(updateEmailTemplateCmd(cli))
	return cmd
}

func showBrandingCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show",
		Args:    cobra.NoArgs,
		Short:   "Display the custom branding settings for Universal Login",
		Long:    "Display the custom branding settings for Universal Login.",
		Example: "auth0 branding show",
		RunE: func(cmd *cobra.Command, args []string) error {
			var branding *management.Branding // Load app by id

			if err := ansi.Waiting(func() error {
				var err error
				branding, err = cli.api.Branding.Read()
				return err
			}); err != nil {
				return fmt.Errorf("Unable to load branding settings due to an unexpected error: %w", err)
			}

			cli.renderer.BrandingShow(branding)

			return nil
		},
	}

	return cmd
}

func updateBrandingCmd(cli *cli) *cobra.Command {
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
		Long:  "Update the custom branding settings for Universal Login.",
		Example: `auth0 branding update
auth0 branding update --accent "#FF4F40" --background "#2A2E35" 
auth0 branding update -a "#FF4F40" -b "#2A2E35" --logo "https://example.com/logo.png"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var current *management.Branding

			if err := ansi.Waiting(func() error {
				var err error
				current, err = cli.api.Branding.Read()
				return err
			}); err != nil {
				return fmt.Errorf("Unable to load branding settings due to an unexpected error: %w", err)
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
				return fmt.Errorf("Unable to update branding settings: %v", err)
			}

			// Render result
			cli.renderer.BrandingUpdate(b)

			return nil
		},
	}

	brandingAccent.RegisterStringU(cmd, &inputs.AccentColor, "")
	brandingBackground.RegisterStringU(cmd, &inputs.BackgroundColor, "")
	brandingLogo.RegisterStringU(cmd, &inputs.LogoURL, "")
	brandingFavicon.RegisterStringU(cmd, &inputs.FaviconURL, "")
	brandingFont.RegisterStringU(cmd, &inputs.CustomFontURL, "")

	return cmd
}

func showBrandingTemplateCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show",
		Args:    cobra.NoArgs,
		Short:   "Display the custom template for Universal Login",
		Long:    "Display the custom template for Universal Login.",
		Example: "auth0 branding templates show",
		RunE: func(cmd *cobra.Command, args []string) error {
			var template *management.BrandingUniversalLogin // Load app by id

			if err := ansi.Waiting(func() error {
				var err error
				template, err = cli.api.Branding.UniversalLogin()
				return err
			}); err != nil {
				return fmt.Errorf("Unable to load the Universal Login template due to an unexpected error: %w", err)
			}

			cli.renderer.Heading("template")
			fmt.Println(*template.Body)

			return nil
		},
	}

	return cmd
}

func updateBrandingTemplateCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "update",
		Args:    cobra.NoArgs,
		Short:   "Update the custom template for Universal Login",
		Long:    "Update the custom template for Universal Login.",
		Example: "auth0 branding templates update",
		RunE: func(cmd *cobra.Command, args []string) error {
			var templateData *branding.TemplateData
			err := ansi.Waiting(func() error {
				var err error
				templateData, err = cli.obtainCustomTemplateData(cmd.Context())
				return err
			})
			if err != nil {
				return err
			}

			if templateData.Body == "" {
				if err := templateBody.Select(cmd, &templateData.Body, customTemplateOptions.labels(), nil); err != nil {
					return err
				}
				templateData.Body = customTemplateOptions.getValue(templateData.Body)
			}

			err = cli.customTemplateEditorPromptWithPreview(
				cmd,
				&templateData.Body,
				*templateData,
			)
			if err != nil {
				return fmt.Errorf("Failed to capture input from the editor: %w", err)
			}

			var confirmed bool
			if err := prompt.AskBool("Do you want to save the template?", &confirmed, true); err != nil {
				return fmt.Errorf("Failed to capture prompt input: %w", err)
			}

			if !confirmed {
				return nil
			}

			err = ansi.Waiting(func() error {
				return cli.api.Branding.SetUniversalLogin(&management.BrandingUniversalLogin{
					Body: &templateData.Body,
				})
			})

			if err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

func (cli *cli) customTemplateEditorPromptWithPreview(cmd *cobra.Command, body *string, templateData branding.TemplateData) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	onInfo := func() {
		cli.renderer.Infof("%s once you close the editor, you'll be prompted to save your changes. To cancel, CTRL+C.", ansi.Faint("Hint:"))
	}

	onFileCreated := func(filename string) {
		templateData.Filename = filename

		if err := branding.PreviewCustomTemplate(ctx, templateData); err != nil {
			cli.renderer.Errorf("Unexpected error while previewing custom template: %w", err)
		}
	}

	return templateBody.EditorPromptW(
		cmd,
		body,
		templateData.Body,
		"custom-template.*.html",
		onInfo,
		onFileCreated,
	)
}

const (
	defaultPrimaryColor    = "#0059d6"
	defaultBackgroundColor = "#000000"
	defaultLogoURL         = "https://cdn.auth0.com/manhattan/versions/1.2921.0/assets/badge.png"
)

func (cli *cli) obtainCustomTemplateData(ctx context.Context) (*branding.TemplateData, error) {
	g, ctx := errgroup.WithContext(ctx)

	var (
		clients      *management.ClientList
		brandingInfo *management.Branding
		template     *management.BrandingUniversalLogin
		tenant       *management.Tenant
	)

	g.Go(func() error {
		var err error
		domains, err := cli.api.CustomDomain.List()
		if err != nil {
			errStatus := err.(management.Error)
			// 403 is a valid response for free tenants that don't have
			// custom domains enabled
			if errStatus != nil && errStatus.Status() == 403 {
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
	})

	g.Go(func() error {
		var err error
		clients, err = cli.api.Client.List(management.Context(ctx))
		return err
	})

	g.Go(func() error {
		var err error
		brandingInfo, err = cli.api.Branding.Read(management.Context(ctx))
		if err != nil {
			brandingInfo = &management.Branding{}
		}

		if brandingInfo.GetColors() == nil {
			brandingInfo.Colors = &management.BrandingColors{
				Primary:        auth0.String(defaultPrimaryColor),
				PageBackground: auth0.String(defaultBackgroundColor),
			}
		}
		if brandingInfo.LogoURL == nil {
			brandingInfo.LogoURL = auth0.String(defaultLogoURL)
		}

		return nil
	})

	g.Go(func() error {
		var err error
		template, err = cli.api.Branding.UniversalLogin(management.Context(ctx))
		if err != nil {
			template = &management.BrandingUniversalLogin{Body: nil}
		}

		return nil
	})

	g.Go(func() error {
		var err error
		tenant, err = cli.api.Tenant.Read(management.Context(ctx))
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	templateData := &branding.TemplateData{
		PrimaryColor:    brandingInfo.GetColors().GetPrimary(),
		BackgroundColor: brandingInfo.GetColors().GetPageBackground(),
		LogoURL:         brandingInfo.GetLogoURL(),
		TenantName:      tenant.GetFriendlyName(),
		Body:            template.GetBody(),
	}

	for _, client := range clients.Clients {
		templateData.Clients = append(templateData.Clients, branding.Client{
			ID:      client.GetClientID(),
			Name:    client.GetName(),
			LogoURL: client.GetLogoURI(),
		})
	}

	return templateData, nil
}
