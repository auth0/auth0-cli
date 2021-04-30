package cli

import (
	"context"
	"fmt"
	"sync"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/branding"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

var (
	templateBody = Flag{
		Name:       "Template",
		LongForm:   "template",
		ShortForm:  "t",
		Help:       "Custom page template for new universal login.",
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
		Use:   "template",
		Short: "Manage custom page template",
		Long:  "Manage custom page template.",
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
		Short:   "Display the custom template for universal login",
		Long:    "Display the custom template for universal login.",
		Example: "auth0 branding template show",
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
		Short:   "Update the custom template for universal login",
		Long:    "Update the custom template for universal login.",
		Example: `auth0 branding template update`,
		PreRun: func(cmd *cobra.Command, args []string) {
			prepareInteractivity(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var templateData *branding.TemplateData
			err := ansi.Waiting(func() error {
				var err error
				templateData, err = cli.obtainCustomTemplateData()
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
		branding.PreviewCustomTemplate(ctx, templateData)
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

func (cli *cli) obtainCustomTemplateData() (*branding.TemplateData, error) {
	wg := &sync.WaitGroup{}

	errors := make(chan error)
	var clients *management.ClientList
	var brandingInfo *management.Branding
	var template *management.BrandingUniversalLogin
	var tenant *management.Tenant

	wg.Add(4)
	go func() {
		var err error
		clients, err = cli.api.Client.List()
		if err != nil {
			errors <- err
		}
		wg.Done()
	}()

	go func() {
		var err error
		brandingInfo, err = cli.api.Branding.Read()
		if err != nil {
			errors <- err
		}
		defaultPrimaryColor := "#0059d6"
		defaultBackgroundColor := "#000000"
		defaultLogoURL := "https://cdn.auth0.com/manhattan/versions/1.2921.0/assets/badge.png"
		if brandingInfo.GetColors() == nil {
			brandingInfo.Colors = &management.BrandingColors{
				Primary:        &defaultPrimaryColor,
				PageBackground: &defaultBackgroundColor,
			}
		}
		if brandingInfo.LogoURL == nil {
			brandingInfo.LogoURL = &defaultLogoURL
		}
		wg.Done()
	}()

	go func() {
		var err error
		template, err = cli.api.Branding.UniversalLogin()
		if err != nil {
			template = &management.BrandingUniversalLogin{
				Body: nil,
			}
		}
		wg.Done()
	}()

	go func() {
		var err error
		tenant, err = cli.api.Tenant.Read()
		if err != nil {
			errors <- err
		}
		wg.Done()
	}()

	go func() {
		wg.Wait()
		close(errors)
	}()

	for err := range errors {
		return nil, err
	}

	templateData := &branding.TemplateData{
		PrimaryColor:    brandingInfo.GetColors().GetPrimary(),
		BackgroundColor: brandingInfo.GetColors().GetPageBackground(),
		LogoURL:         brandingInfo.GetLogoURL(),
		TenantName:      tenant.GetFriendlyName(),
		Body:            template.GetBody(),
	}

	templateData.Clients = make([]branding.Client, len(clients.Clients))
	for i, client := range clients.Clients {
		templateData.Clients[i] = branding.Client{
			Id:      client.GetClientID(),
			Name:    client.GetName(),
			LogoUrl: client.GetLogoURI(),
		}
	}

	return templateData, nil
}
