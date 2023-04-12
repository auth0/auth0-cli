package cli

import (
	"context"
	"embed"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"text/template"
	"time"

	"github.com/auth0/go-auth0/management"
	"github.com/fsnotify/fsnotify"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/prompt"
)

const (
	defaultPrimaryColor    = "#0059d6"
	defaultBackgroundColor = "#000000"
	defaultLogoURL         = "https://cdn.auth0.com/manhattan/versions/1.3935.0/assets/badge.png"
)

var (
	errNotAllowed = errors.New(
		"this feature requires at least one custom domain to be set and verified " +
			"for the tenant, use 'auth0 domains create' to create one and 'auth0 domains verify' to have it verified",
	)

	//go:embed data/branding/storybook/*
	templatePreviewAssets embed.FS

	//go:embed data/branding/tenant-data.js
	tenantDataAsset string

	//go:embed data/branding/default-template.liquid
	templateBasic string

	//go:embed data/branding/image-template.liquid
	templateWithImage string

	//go:embed data/branding/footer-template.liquid
	templateWithFooter string

	templateBody = Flag{
		Name:       "Template",
		LongForm:   "template",
		ShortForm:  "t",
		Help:       "Custom page template for Universal Login.",
		IsRequired: true,
	}

	templateOptions = pickerOptions{
		{"Basic template", templateBasic},
		{"Template with login box and background image", templateWithImage},
		{"Template with page footers", templateWithFooter},
	}
)

// TemplateData contains all the variables we project onto our embedded go
// template. These variables largely resemble the same ones in the auth0
// branding template.
type TemplateData struct {
	Filename        string
	Clients         []ClientData
	PrimaryColor    string
	BackgroundColor string
	LogoURL         string
	TenantName      string
	Body            string
	Experience      string
}

// ClientData is a minimal representation of an Auth0 Client as defined in the
// management API. This is used within the branding machinery to populate the
// tenant data.
type ClientData struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	LogoURL string `json:"logo_url,omitempty"`
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

			cli.renderer.Heading("universal login template")

			if currentTemplate == nil {
				cli.renderer.Infof("No custom template found. To set one, run: `auth0 universal-login templates update`.")
			}

			cli.renderer.Output(currentTemplate.GetBody())

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
		Long:  "Update the custom template for the New Universal Login Experience.",
		Example: `  auth0 universal-login templates update
  auth0 ul templates update`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			var templateData *TemplateData
			if err := ansi.Waiting(func() (err error) {
				templateData, err = cli.fetchTemplateData(ctx)
				return err
			}); err != nil {
				return fmt.Errorf("failed to fetch the Universal Login template data: %w", err)
			}

			if templateData.Experience == "classic" {
				cli.renderer.Warnf("The tenant is configured to use the classic Universal Login Experience instead of the new. The template changes won't apply until you select the new Universal Login Experience. You can do so by running: \"auth0 api patch prompts --data '{\"universal_login_experience\":\"new\"}'\"")
			}

			if templateData.Body == "" {
				if err := templateBody.Select(cmd, &templateData.Body, templateOptions.labels(), nil); err != nil {
					return fmt.Errorf("failed to select the desired template: %w", err)
				}
				templateData.Body = templateOptions.getValue(templateData.Body)
			}

			if err := cli.editTemplateAndPreviewChanges(ctx, cmd, templateData); err != nil {
				return fmt.Errorf("failed to edit the template and preview the changes: %w", err)
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

			if err := ansi.Waiting(func() error {
				return cli.api.Branding.SetUniversalLogin(
					&management.BrandingUniversalLogin{
						Body: &templateData.Body,
					},
				)
			}); err != nil {
				return fmt.Errorf("failed to update the template for the New Universal Login Experience: %w", err)
			}

			cli.renderer.Infof("Successfully updated the template for the New Universal Login Experience!")

			return nil
		},
	}

	cmd.Flags().BoolVar(&cli.force, "force", false, "Skip confirmation.")

	return cmd
}

func (cli *cli) fetchTemplateData(ctx context.Context) (*TemplateData, error) {
	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() (err error) {
		return ensureCustomDomainIsEnabled(ctx, cli.api)
	})

	var promptSettings *management.Prompt
	group.Go(func() (err error) {
		promptSettings, err = cli.api.Prompt.Read()
		return err
	})

	var clientList *management.ClientList
	group.Go(func() (err error) {
		clientList, err = cli.api.Client.List(management.Context(ctx), management.PerPage(100)) // Capping the clients retrieved to 100 for now.
		return err
	})

	var brandingSettings *management.Branding
	group.Go(func() (err error) {
		brandingSettings = fetchBrandingSettingsOrUseDefaults(ctx, cli.api)
		return nil
	})

	var currentTemplate *management.BrandingUniversalLogin
	group.Go(func() (err error) {
		currentTemplate = fetchBrandingTemplateOrUseEmpty(ctx, cli.api)
		return nil
	})

	var tenant *management.Tenant
	group.Go(func() (err error) {
		tenant, err = cli.api.Tenant.Read(management.Context(ctx))
		return err
	})

	if err := group.Wait(); err != nil {
		return nil, err
	}

	templateData := &TemplateData{
		PrimaryColor:    brandingSettings.GetColors().GetPrimary(),
		BackgroundColor: brandingSettings.GetColors().GetPageBackground(),
		LogoURL:         brandingSettings.GetLogoURL(),
		TenantName:      tenant.GetFriendlyName(),
		Body:            currentTemplate.GetBody(),
		Experience:      promptSettings.UniversalLoginExperience,
	}

	for _, client := range clientList.Clients {
		templateData.Clients = append(templateData.Clients, ClientData{
			ID:      client.GetClientID(),
			Name:    client.GetName(),
			LogoURL: client.GetLogoURI(),
		})
	}

	return templateData, nil
}

func ensureCustomDomainIsEnabled(ctx context.Context, api *auth0.API) error {
	domains, err := api.CustomDomain.List(management.Context(ctx))
	if err != nil {
		if mErr, ok := err.(management.Error); ok && mErr.Status() == http.StatusForbidden {
			return errNotAllowed // 403 is a valid response for free tenants that don't have custom domains enabled
		}

		return err
	}

	const domainIsVerified = "ready"
	for _, domain := range domains {
		if domain.GetStatus() == domainIsVerified {
			return nil
		}
	}

	return errNotAllowed
}

func fetchBrandingSettingsOrUseDefaults(ctx context.Context, api *auth0.API) *management.Branding {
	brandingSettings, _ := api.Branding.Read(management.Context(ctx))
	if brandingSettings == nil {
		brandingSettings = &management.Branding{}
	}

	if brandingSettings.Colors == nil {
		brandingSettings.Colors = &management.BrandingColors{
			Primary:        auth0.String(defaultPrimaryColor),
			PageBackground: auth0.String(defaultBackgroundColor),
		}
	}

	if brandingSettings.LogoURL == nil {
		brandingSettings.LogoURL = auth0.String(defaultLogoURL)
	}

	return brandingSettings
}

func fetchBrandingTemplateOrUseEmpty(ctx context.Context, api *auth0.API) *management.BrandingUniversalLogin {
	currentTemplate, err := api.Branding.UniversalLogin(management.Context(ctx))
	if err != nil {
		currentTemplate = &management.BrandingUniversalLogin{}
	}

	return currentTemplate
}

func (cli *cli) editTemplateAndPreviewChanges(ctx context.Context, cmd *cobra.Command, templateData *TemplateData) error {
	onInfo := func() {
		cli.renderer.Infof("%s Once you close the editor, you'll be prompted to save your changes. To cancel, press CTRL+C.", ansi.Faint("Hint:"))
	}

	onFileCreated := func(filename string) {
		templateData.Filename = filename
		if err := previewTemplate(ctx, templateData); err != nil {
			cli.renderer.Errorf("failed to preview the universal login template: %w", err)
		}
	}

	return templateBody.OpenEditorW(
		cmd,
		&templateData.Body,
		templateData.Body,
		"universal-login-template.*.html",
		onInfo,
		onFileCreated,
	)
}

func previewTemplate(ctx context.Context, data *TemplateData) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close()

	changesChan, err := broadcastTemplateChanges(ctx, data.Filename)
	if err != nil {
		return err
	}

	requestTimeout := 10 * time.Minute
	server := &http.Server{
		Handler:      buildRoutes(requestTimeout, data, changesChan),
		ReadTimeout:  requestTimeout + time.Minute,
		WriteTimeout: requestTimeout + time.Minute,
	}
	defer server.Close()

	go func() {
		if err = server.Serve(listener); err != http.ErrServerClosed {
			cancel()
		}
	}()

	storybookURL := &url.URL{
		Scheme:   "http",
		Host:     listener.Addr().String(),
		Path:     "/data/branding/storybook/",
		RawQuery: (url.Values{"path": []string{"/story/universal-login--prompts"}}).Encode(),
	}

	if err := browser.OpenURL(storybookURL.String()); err != nil {
		return err
	}

	// Wait until the file is closed or input is cancelled
	<-ctx.Done()
	return nil
}

func buildRoutes(
	requestTimeout time.Duration,
	data *TemplateData,
	changesChan chan bool,
) *http.ServeMux {
	router := http.NewServeMux()

	router.HandleFunc("/dynamic/events", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		writeStatus := func(w http.ResponseWriter, code int) {
			msg := fmt.Sprintf("%d - %s", code, http.StatusText(http.StatusGone))
			http.Error(w, msg, code)
		}

		select {
		case <-ctx.Done():
			writeStatus(w, http.StatusGone)
		case <-time.After(requestTimeout):
			writeStatus(w, http.StatusRequestTimeout)
		case <-changesChan:
			writeStatus(w, http.StatusOK)
		}
	})

	router.HandleFunc("/dynamic/template", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, data.Filename)
	})

	javascriptTemplate := template.Must(template.New("tenant-data.js").Funcs(template.FuncMap{
		"toJSON": func(v interface{}) string {
			data, _ := json.Marshal(v)
			return string(data)
		},
	}).Parse(tenantDataAsset))

	router.HandleFunc("/dynamic/tenant-data", func(w http.ResponseWriter, r *http.Request) {
		if err := javascriptTemplate.Execute(w, data); err != nil {
			http.Error(w, err.Error(), 500)
		}
	})

	router.Handle("/", http.FileServer(http.FS(templatePreviewAssets)))
	return router
}

func broadcastTemplateChanges(ctx context.Context, filename string) (chan bool, error) {
	changesChan := make(chan bool)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					changesChan <- true
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			}
		}
	}()

	go func() {
		<-ctx.Done()
		watcher.Close()
		close(changesChan)
	}()

	if err := watcher.Add(filepath.Dir(filename)); err != nil {
		return nil, err
	}

	return changesChan, nil
}
