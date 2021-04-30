package cli

import (
	"context"
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

	customTemplateOptions = pickerOptions{
		{"Basic", branding.DefaultTemplate},
		{"Login box + image", branding.ImageTemplate},
		{"Page footers", branding.FooterTemplate},
	}
)

func brandingCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "branding",
		Short: "Manage branding options",
		Long:  "Manage branding options.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(templateCmd(cli))
	return cmd
}

func templateCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "templates",
		Short: "Manage custom page templates",
		Long:  "Manage custom page templates.",
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(showBrandingTemplateCmd(cli))
	cmd.AddCommand(updateBrandingTemplateCmd(cli))
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
			template, err := cli.api.Branding.UniversalLogin()
			if err != nil {
				return fmt.Errorf("Unable to load tenants due to an unexpected error: %w", err)
			}

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
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
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
		clients, err = cli.api.Client.List()
		return err
	})

	g.Go(func() error {
		var err error
		brandingInfo, err = cli.api.Branding.Read(management.Context(ctx))
		if err != nil {
			return err
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
		return err
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
